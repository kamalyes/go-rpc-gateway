/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-13 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 23:55:55
 * @FilePath: \go-rpc-gateway\middleware\pb_validation.go
 * @Description: 通用参数验证中间件 - 基于 go-pbmo Validator
 * 支持规则注册、自动类型识别、HTTP/gRPC 双协议
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	gopbmo "github.com/kamalyes/go-pbmo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PBValidationMiddleware PB参数验证中间件
// 用于验证HTTP请求体中的参数是否符合指定规则
type PBValidationMiddleware struct {
	validator     *gopbmo.Validator
	enabled       bool
	skipPaths     []string
	typeResolvers map[string]TypeResolverFunc
}

// TypeResolverFunc 类型解析函数
// 用于根据路径前缀动态解析请求体为指定结构体
type TypeResolverFunc func(body []byte) (interface{}, error)

// PBValidationError PB验证错误结构
type PBValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// PBValidationResponse PB验证响应结构
// 包含验证结果、错误信息和状态码
type PBValidationResponse struct {
	Success bool                `json:"success"`
	Message string              `json:"message"`
	Errors  []PBValidationError `json:"errors,omitempty"`
	Code    int                 `json:"code"`
}

// NewPBValidationMiddleware 创建PB验证中间件（默认开启）
func NewPBValidationMiddleware() *PBValidationMiddleware {
	return &PBValidationMiddleware{
		validator:     gopbmo.NewValidator(),
		enabled:       true,
		skipPaths:     []string{"/health", "/metrics", "/swagger", "/debug"},
		typeResolvers: make(map[string]TypeResolverFunc),
	}
}

// RegisterRules 注册验证规则
// 用于为指定结构体注册验证规则
// 可以调用多次，每个结构体可以有多个规则
// 规则格式：FieldRule{Field: "field_name", Tags: []string{"required"}}
// 例如：FieldRule{Field: "name", Tags: []string{"required"}}
func (m *PBValidationMiddleware) RegisterRules(structName string, rules ...gopbmo.FieldRule) {
	m.validator.RegisterRules(structName, rules...)
}

// RegisterBatch 注册批量验证规则
// 用于批量注册多个结构体的验证规则
// 格式：map[string][]FieldRule
// 例如：map[string][]FieldRule{"User": {FieldRule{Field: "name", Tags: []string{"required"}}}}
func (m *PBValidationMiddleware) RegisterBatch(rulesMap map[string][]gopbmo.FieldRule) {
	m.validator.RegisterBatch(rulesMap)
}

// RegisterTypeResolver 注册类型解析函数
// 用于根据路径前缀动态解析请求体为指定结构体
//
//	例如：RegisterTypeResolver("/user", func(body []byte) (interface{}, error) {
//	    return &User{}, nil
//	})
func (m *PBValidationMiddleware) RegisterTypeResolver(pathPrefix string, resolver TypeResolverFunc) {
	m.typeResolvers[pathPrefix] = resolver
}

// AddSkipPaths 添加跳过路径
// 用于指定哪些路径不进行参数验证
// 例如：AddSkipPaths("/health", "/metrics", "/swagger", "/debug")
// 例如：AddSkipPaths("/user")
func (m *PBValidationMiddleware) AddSkipPaths(paths ...string) {
	m.skipPaths = append(m.skipPaths, paths...)
}

// SetEnabled 设置中间件是否启用
// 用于控制中间件是否在请求处理中生效
func (m *PBValidationMiddleware) SetEnabled(enabled bool) {
	m.enabled = enabled
}

// GetValidator 获取验证器实例
func (m *PBValidationMiddleware) GetValidator() *gopbmo.Validator {
	return m.validator
}

// Validate 验证数据是否符合规则
func (m *PBValidationMiddleware) Validate(data interface{}) error {
	return m.validator.Validate(data)
}

// HTTPMiddleware 应用HTTP中间件链
func (m *PBValidationMiddleware) HTTPMiddleware() MiddlewareFunc {
	if !m.enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !m.shouldValidateHTTP(r) {
				next.ServeHTTP(w, r)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				m.writeErrorResponse(w, "无法读取请求体", nil, http.StatusBadRequest)
				return
			}
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(body))

			if len(body) > 0 {
				if err := m.validateHTTPBody(body, r.URL.Path); err != nil {
					m.writeErrorResponse(w, "参数验证失败", err, http.StatusBadRequest)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GRPCUnaryInterceptor gRPC一元拦截器
func (m *PBValidationMiddleware) GRPCUnaryInterceptor() grpc.UnaryServerInterceptor {
	if !m.enabled {
		return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			return handler(ctx, req)
		}
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if m.shouldSkipPath(info.FullMethod) {
			return handler(ctx, req)
		}

		if err := m.validator.Validate(req); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		return handler(ctx, req)
	}
}

// GRPCStreamInterceptor gRPC流拦截器
func (m *PBValidationMiddleware) GRPCStreamInterceptor() grpc.StreamServerInterceptor {
	if !m.enabled {
		return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, ss)
		}
	}

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		wrappedStream := &validatingServerStream{
			ServerStream: ss,
			validator:    m,
			methodName:   info.FullMethod,
		}
		return handler(srv, wrappedStream)
	}
}

// shouldValidateHTTP 判断是否需要验证HTTP请求体
func (m *PBValidationMiddleware) shouldValidateHTTP(r *http.Request) bool {
	method := strings.ToUpper(r.Method)
	return method == "POST" || method == "PUT" || method == "PATCH"
}

// shouldSkipPath 判断是否需要跳过参数验证
func (m *PBValidationMiddleware) shouldSkipPath(path string) bool {
	for _, skipPath := range m.skipPaths {
		if strings.Contains(path, skipPath) {
			return true
		}
	}
	return false
}

// validateHTTPBody 验证HTTP请求体
func (m *PBValidationMiddleware) validateHTTPBody(body []byte, path string) error {
	resolved, err := m.resolveType(body, path)
	if err != nil || resolved == nil {
		return nil
	}

	return m.validator.Validate(resolved)
}

// resolveType 解析请求体为指定结构体
func (m *PBValidationMiddleware) resolveType(body []byte, path string) (interface{}, error) {
	for prefix, resolver := range m.typeResolvers {
		if strings.Contains(path, prefix) {
			return resolver(body)
		}
	}
	return nil, nil
}

// formatValidationError 格式化验证错误
func (m *PBValidationMiddleware) formatValidationError(err error) error {
	if validationErrs, ok := err.(gopbmo.ValidationErrors); ok {
		var errors []PBValidationError
		for _, fieldErr := range validationErrs {
			errors = append(errors, PBValidationError{
				Field:   fieldErr.Field,
				Message: fieldErr.Message,
			})
		}
		errBytes, _ := json.Marshal(errors)
		return fmt.Errorf("validation failed: %s", string(errBytes))
	}
	return err
}

// writeErrorResponse 写入错误响应
func (m *PBValidationMiddleware) writeErrorResponse(w http.ResponseWriter, message string, validationErr error, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	var errs []PBValidationError
	if validationErr != nil {
		if validationErrs, ok := validationErr.(gopbmo.ValidationErrors); ok {
			for _, fieldErr := range validationErrs {
				errs = append(errs, PBValidationError{
					Field:   fieldErr.Field,
					Message: fieldErr.Message,
				})
			}
		}
	}

	response := PBValidationResponse{
		Success: false,
		Message: message,
		Errors:  errs,
		Code:    code,
	}

	json.NewEncoder(w).Encode(response)
}

// validatingServerStream 验证gRPC流请求参数
type validatingServerStream struct {
	grpc.ServerStream
	validator  *PBValidationMiddleware
	methodName string
}

// RecvMsg 接收gRPC流消息
func (s *validatingServerStream) RecvMsg(m interface{}) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}

	if err := s.validator.validator.Validate(m); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	return nil
}
