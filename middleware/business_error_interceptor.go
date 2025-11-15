/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-15 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\business_error_interceptor.go
 * @Description: 业务错误拦截器 - 自动处理业务服务返回的错误码
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/kamalyes/go-rpc-gateway/response"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// BusinessErrorInterceptor gRPC业务错误拦截器
// 自动将业务服务返回的错误码转换为统一格式
type BusinessErrorInterceptor struct {
	// 可选：错误码映射配置
	codeMapping map[int32]int32
}

// NewBusinessErrorInterceptor 创建业务错误拦截器
func NewBusinessErrorInterceptor() *BusinessErrorInterceptor {
	return &BusinessErrorInterceptor{
		codeMapping: make(map[int32]int32),
	}
}

// WithCodeMapping 设置错误码映射
// 用于将业务服务的错误码映射为标准错误码
func (b *BusinessErrorInterceptor) WithCodeMapping(mapping map[int32]int32) *BusinessErrorInterceptor {
	b.codeMapping = mapping
	return b
}

// UnaryClientInterceptor 一元调用客户端拦截器
func (b *BusinessErrorInterceptor) UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		// 调用远程服务
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			// 尝试提取业务错误并转换
			return b.transformError(err)
		}

		// 检查响应中是否包含业务错误码
		// 某些业务可能在正常响应中也返回错误码
		if bizErr := b.extractErrorFromResponse(reply); bizErr != nil {
			return bizErr
		}

		return nil
	}
}

// StreamClientInterceptor 流式调用客户端拦截器
func (b *BusinessErrorInterceptor) StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		clientStream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			return nil, b.transformError(err)
		}
		return &businessErrorStream{ClientStream: clientStream, interceptor: b}, nil
	}
}

// businessErrorStream 包装的客户端流
type businessErrorStream struct {
	grpc.ClientStream
	interceptor *BusinessErrorInterceptor
}

func (s *businessErrorStream) RecvMsg(m interface{}) error {
	err := s.ClientStream.RecvMsg(m)
	if err != nil {
		return s.interceptor.transformError(err)
	}
	
	// 检查消息中的业务错误
	if bizErr := s.interceptor.extractErrorFromResponse(m); bizErr != nil {
		return bizErr
	}
	
	return nil
}

// transformError 转换错误
func (b *BusinessErrorInterceptor) transformError(err error) error {
	bizErr := response.ExtractBusinessError(err)
	if bizErr == nil {
		return err
	}

	// 应用错误码映射
	code := bizErr.GetCode()
	if mappedCode, ok := b.codeMapping[code]; ok {
		return response.NewBusinessErrorWithDetails(
			mappedCode,
			bizErr.GetMessage(),
			bizErr.GetDetails(),
		)
	}

	return bizErr
}

// extractErrorFromResponse 从响应中提取业务错误
// 适用于业务在正常响应体中返回错误码的场景
func (b *BusinessErrorInterceptor) extractErrorFromResponse(resp interface{}) error {
	// 1. 检查是否实现了 BusinessErrorResponse 接口
	if errResp, ok := resp.(BusinessErrorResponse); ok {
		if errResp.GetErrorCode() != 0 {
			return response.NewBusinessError(
				errResp.GetErrorCode(),
				errResp.GetErrorMessage(),
			)
		}
	}

	// 2. 使用反射检查常见字段名
	// 例如: Code, ErrorCode, ErrCode 等
	// 这里使用类型断言，避免使用反射带来的性能损耗
	
	return nil
}

// BusinessErrorResponse 业务错误响应接口
// 业务服务的响应消息可以实现此接口
type BusinessErrorResponse interface {
	GetErrorCode() int32
	GetErrorMessage() string
}

// HTTPBusinessErrorMiddleware HTTP业务错误处理中间件
// 用于HTTP层面统一处理业务错误
func HTTPBusinessErrorMiddleware() HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 创建自定义ResponseWriter来捕获响应
			recorder := &businessErrorRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// 调用下一个处理器
			next.ServeHTTP(recorder, r)

			// 如果有业务错误，优先处理
			if recorder.businessError != nil {
				response.WriteBusinessErrorResponse(w, recorder.businessError)
				return
			}
		})
	}
}

// businessErrorRecorder 捕获业务错误的ResponseWriter
type businessErrorRecorder struct {
	http.ResponseWriter
	statusCode    int
	businessError error
}

func (r *businessErrorRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	// 暂不写入，等判断是否有业务错误
}

func (r *businessErrorRecorder) Write(data []byte) (int, error) {
	// 尝试解析业务错误
	var resp response.Response
	if err := json.Unmarshal(data, &resp); err == nil {
		if resp.Code != 0 {
			r.businessError = response.NewBusinessErrorWithDetails(
				resp.Code,
				resp.Message,
				resp.Details,
			)
			return len(data), nil
		}
	}

	// 没有业务错误，正常写入
	if r.statusCode != 0 {
		r.ResponseWriter.WriteHeader(r.statusCode)
	}
	return r.ResponseWriter.Write(data)
}

// InjectBusinessErrorToMetadata 将业务错误注入到gRPC metadata
// 业务服务可以使用这个函数将错误码注入到metadata
func InjectBusinessErrorToMetadata(ctx context.Context, code int32, message string) context.Context {
	md := metadata.Pairs(
		"x-business-error-code", string(rune(code)),
		"x-business-error-message", message,
	)
	return metadata.NewOutgoingContext(ctx, md)
}

// ExtractBusinessErrorFromMetadata 从metadata提取业务错误
func ExtractBusinessErrorFromMetadata(ctx context.Context) *response.StandardBusinessError {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}

	codes := md.Get("x-business-error-code")
	messages := md.Get("x-business-error-message")
	
	if len(codes) == 0 || len(messages) == 0 {
		return nil
	}

	code := int32(codes[0][0]) // 简化处理
	return response.NewBusinessError(code, messages[0])
}
