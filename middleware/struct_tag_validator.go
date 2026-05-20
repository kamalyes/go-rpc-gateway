/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-23 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-23 00:15:56
 * @FilePath: \go-rpc-gateway\middleware\struct_tag_validator.go
 * @Description: 基于 go-argus 的 struct tag 校验拦截器
 *               配合 protoc-go-inject-tag 在 pb 生成代码字段上注入 `validate:"..."` 标签，
 *               无需业务方在 service 中手写 `if req.GetXxx() == ""` 之类的参数校验
 *               同时支持 gRPC 拦截器和 grpc-gateway HTTP 中间件两种模式，
 *               确保本地 Handler（RegisterXxxHandlerServer）模式下 HTTP 请求也能走校验
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	validator "github.com/kamalyes/go-argus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	structTagValidatorOnce sync.Once           // 确保线程安全
	structTagValidator     *validator.Validate // 全局共享的 validator 实例
)

// getStructTagValidator 返回进程内共享的 validator 实例（并发安全）
func getStructTagValidator() *validator.Validate {
	structTagValidatorOnce.Do(func() {
		structTagValidator = validator.New(validator.WithRequiredStructEnabled())
	})
	return structTagValidator
}

// StructTagValidatorUnaryInterceptor 基于 struct tag 的 gRPC Unary 校验拦截器
// 对每个入参 req 做校验
// 工作方式：对每个入参 req 调用 `validator.Struct(req)`若 pb 消息字段通过
// protoc-go-inject-tag 注入了 `validate:"required,min=1,..."` 之类的标签，校验失败
// 时返回 codes.InvalidArgument；字段未注入标签时 validator 会跳过，不会产生误报
func StructTagValidatorUnaryInterceptor() GRPCInterceptor {
	v := getStructTagValidator()
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if req == nil {
			return handler(ctx, req)
		}
		if err := v.Struct(req); err != nil {
			return nil, status.Error(codes.InvalidArgument, formatStructTagValidationError(err))
		}
		return handler(ctx, req)
	}
}

// StructTagValidatorStreamInterceptor 基于 struct tag 的 gRPC Stream 校验拦截器
// 对每条流消息 RecvMsg 做校验
func StructTagValidatorStreamInterceptor() grpc.StreamServerInterceptor {
	v := getStructTagValidator()
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		wrapped := &structTagValidatingStream{ServerStream: ss, validate: v}
		return handler(srv, wrapped)
	}
}

// structTagValidatingStream 基于 struct tag 的 gRPC Stream 校验拦截器
type structTagValidatingStream struct {
	grpc.ServerStream
	validate *validator.Validate
}

// RecvMsg 接收流消息，校验 struct tag
func (s *structTagValidatingStream) RecvMsg(m interface{}) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	if m == nil {
		return nil
	}
	if err := s.validate.Struct(m); err != nil {
		return status.Error(codes.InvalidArgument, formatStructTagValidationError(err))
	}
	return nil
}

// formatStructTagValidationError 将 validator 的错误格式化为更易读的消息
func formatStructTagValidationError(err error) string {
	var fieldErrs validator.ValidationErrors
	if ok := toValidationErrors(err, &fieldErrs); !ok {
		return err.Error()
	}
	if len(fieldErrs) == 0 {
		return err.Error()
	}
	parts := make([]string, 0, len(fieldErrs))
	for _, fe := range fieldErrs {
		parts = append(parts, fe.Namespace()+": "+fe.Tag())
	}
	return "invalid argument: " + strings.Join(parts, ", ")
}

// ────────────────────────────────────────
// proto 消息类型注册表
// ────────────────────────────────────────

// gatewayMessageTypeRegistry 全局注册表：HTTP 方法+路径模式 → 消息工厂
// 用于本地 Handler 模式下，在 runtime.Middleware 层反序列化请求体并做 struct tag 校验
var gatewayMessageTypeRegistry struct {
	mu   sync.RWMutex
	data map[string]func() any // key = "METHOD /path/pattern"
}

// RegisterGatewayMessageType 注册某个 HTTP 路由对应的请求消息工厂函数
// 业务方在调用 RegisterXxxHandlerServer 的同时，应调用此函数注册请求消息类型，例如：
//
//	tenantpb.RegisterPlatformServiceHandlerServer(ctx, mux, g.platformSvc)
//	middleware.RegisterGatewayMessageType(http.MethodPost, "/v1/platforms", func() any {
//	    return &tenantpb.CreatePlatformRequest{}
//	})
//
// 只有注册了消息类型的路由才会触发 struct tag 校验，未注册的路由会被跳过
func RegisterGatewayMessageType(method string, pathPattern string, newMsg func() any) {
	gatewayMessageTypeRegistry.mu.Lock()
	defer gatewayMessageTypeRegistry.mu.Unlock()
	if gatewayMessageTypeRegistry.data == nil {
		gatewayMessageTypeRegistry.data = make(map[string]func() any)
	}
	key := method + " " + pathPattern
	gatewayMessageTypeRegistry.data[key] = newMsg
}

// lookupGatewayMessageType 根据 HTTP 方法和路径查找注册的消息工厂
func lookupGatewayMessageType(method string, path string) (func() any, bool) {
	gatewayMessageTypeRegistry.mu.RLock()
	defer gatewayMessageTypeRegistry.mu.RUnlock()
	if gatewayMessageTypeRegistry.data == nil {
		return nil, false
	}
	// 先尝试精确匹配
	key := method + " " + path
	if fn, ok := gatewayMessageTypeRegistry.data[key]; ok {
		return fn, true
	}
	// 再尝试前缀匹配（路径可能包含路径参数的实际值，如 /v1/platforms/123）
	for k, fn := range gatewayMessageTypeRegistry.data {
		if strings.HasPrefix(key, k) || strings.HasPrefix(key+" ", k) {
			return fn, true
		}
	}
	return nil, false
}

// StructTagValidatorGatewayMiddleware 基于 struct tag 的 grpc-gateway HTTP 校验中间件
// 用于本地 Handler 模式（RegisterXxxHandlerServer），HTTP 请求绕过 gRPC 拦截器链，
// 需要通过 runtime.WithMiddlewares 注入此中间件才能触发校验
//
// 工作原理：
//  1. 在 runtime.Middleware 层拦截 HTTP 请求
//  2. 读取请求体，查找注册的 proto 消息类型
//  3. 反序列化为 proto 消息后调用 go-argus 的 Struct 校验
//  4. 校验通过后回放请求体，交给后续 handler 处理
//  5. 未注册消息类型的路由直接放行
func StructTagValidatorGatewayMiddleware() runtime.Middleware {
	v := getStructTagValidator()
	return func(next runtime.HandlerFunc) runtime.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
			// 查找该路由对应的 proto 消息类型
			newMsg, found := lookupGatewayMessageType(r.Method, r.URL.Path)
			if !found {
				next(w, r, pathParams)
				return
			}

			// 读取请求体
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read request body", http.StatusBadRequest)
				return
			}

			// 回放请求体，确保后续 handler 仍可读取
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			// 空请求体跳过校验（GET 请求等）
			if len(bodyBytes) == 0 {
				next(w, r, pathParams)
				return
			}

			// 创建 proto 消息实例并反序列化
			msg := newMsg()
			inboundMarshaler, _ := runtime.MarshalerForRequest(runtime.NewServeMux(), r)
			if err := inboundMarshaler.NewDecoder(bytes.NewReader(bodyBytes)).Decode(msg); err != nil {
				// 反序列化失败，交给后续 handler 处理（让它返回标准错误）
				next(w, r, pathParams)
				return
			}

			// 执行 struct tag 校验
			if err := v.Struct(msg); err != nil {
				runtime.HTTPError(r.Context(), runtime.NewServeMux(), inboundMarshaler, w, r,
					status.Error(codes.InvalidArgument, formatStructTagValidationError(err)))
				return
			}

			next(w, r, pathParams)
		}
	}
}

// toValidationErrors 执行一次断言，避免 fmt 依赖
func toValidationErrors(err error, out *validator.ValidationErrors) bool {
	ve, ok := err.(validator.ValidationErrors)
	if !ok {
		return false
	}
	*out = ve
	return true
}
