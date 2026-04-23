/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-23 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-23 00:15:56
 * @FilePath: \go-rpc-gateway\middleware\struct_tag_validator.go
 * @Description: 基于 go-playground/validator 的 struct tag gRPC 校验拦截器
 *               配合 protoc-go-inject-tag 在 pb 生成代码字段上注入 `validate:"..."` 标签，
 *               无需业务方在 service 中手写 `if req.GetXxx() == ""` 之类的参数校验
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"github.com/go-playground/validator/v10"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
	"sync"
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

// toValidationErrors 执行一次断言，避免 fmt 依赖
func toValidationErrors(err error, out *validator.ValidationErrors) bool {
	ve, ok := err.(validator.ValidationErrors)
	if !ok {
		return false
	}
	*out = ve
	return true
}
