/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-13 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 23:55:55
 * @FilePath: \go-rpc-gateway\middleware\pb_validation.go
 * @Description: PB参数验证中间件
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

	"github.com/go-playground/validator/v10"
	"github.com/kamalyes/go-rpc-gateway/pbmo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PBValidationMiddleware PB参数验证中间件
type PBValidationMiddleware struct {
	validator *validator.Validate
	enabled   bool
}

// PBValidationError PB验证错误
type PBValidationError struct {
	Field   string      `json:"field"`
	Tag     string      `json:"tag"`
	Value   interface{} `json:"value"`
	Message string      `json:"message"`
}

// PBValidationResponse PB验证错误响应
type PBValidationResponse struct {
	Success bool                `json:"success"`
	Message string              `json:"message"`
	Errors  []PBValidationError `json:"errors,omitempty"`
	Code    int                 `json:"code"`
}

// NewPBValidationMiddleware 创建PB验证中间件（默认开启）
func NewPBValidationMiddleware() *PBValidationMiddleware {
	// 确保PBValidator已初始化
	if pbmo.PBValidator == nil {
		pbmo.PBValidator = validator.New()
	}

	validatorInstance := pbmo.PBValidator

	// 注册自定义验证规则
	registerCustomValidationRules(validatorInstance)

	return &PBValidationMiddleware{
		validator: validatorInstance,
		enabled:   true, // 默认开启
	}
}

// HTTPMiddleware HTTP中间件实现
func (m *PBValidationMiddleware) HTTPMiddleware() MiddlewareFunc {
	if !m.enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 只处理POST/PUT/PATCH请求
			if !m.shouldValidate(r) {
				next.ServeHTTP(w, r)
				return
			}

			// 读取请求体
			body, err := io.ReadAll(r.Body)
			if err != nil {
				m.writeErrorResponse(w, "无法读取请求体", nil, http.StatusBadRequest)
				return
			}
			r.Body.Close()

			// 恢复请求体，供后续处理使用
			r.Body = io.NopCloser(bytes.NewReader(body))

			// 尝试解析并验证PB结构
			if err := m.validateRequestBody(body, r.URL.Path); err != nil {
				m.writeErrorResponse(w, "参数验证失败", err, http.StatusBadRequest)
				return
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
		// 验证请求参数
		if err := m.validatePBStruct(req, info.FullMethod); err != nil {
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
		// 对于流式调用，我们包装ServerStream来验证每个消息
		wrappedStream := &validatingServerStream{
			ServerStream: ss,
			validator:    m,
			methodName:   info.FullMethod,
		}

		return handler(srv, wrappedStream)
	}
}

// shouldValidate 判断是否应该验证请求
func (m *PBValidationMiddleware) shouldValidate(r *http.Request) bool {
	method := strings.ToUpper(r.Method)
	return method == "POST" || method == "PUT" || method == "PATCH"
}

// validateRequestBody 验证HTTP请求体
func (m *PBValidationMiddleware) validateRequestBody(body []byte, path string) error {
	if len(body) == 0 {
		return nil
	}

	// 尝试识别PB结构类型并验证
	pbStruct, err := m.identifyPBStruct(body, path)
	if err != nil {
		return nil // 如果无法识别PB类型，跳过验证
	}

	return m.validatePBStruct(pbStruct, path)
}

// validatePBStruct 验证PB结构体
func (m *PBValidationMiddleware) validatePBStruct(pb interface{}, methodName string) error {
	if pb == nil {
		return nil
	}

	// 跳过不需要验证的方法
	if m.shouldSkipValidation(methodName) {
		return nil
	}

	err := m.validator.Struct(pb)
	if err != nil {
		return m.formatValidationError(err)
	}

	return nil
}

// identifyPBStruct 根据路径和请求体识别PB结构类型
func (m *PBValidationMiddleware) identifyPBStruct(body []byte, path string) (interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	// 根据路径判断PB类型
	switch {
	case strings.Contains(path, "/users") && strings.Contains(path, "/create"):
		var user pbmo.User
		if err := json.Unmarshal(body, &user); err != nil {
			return nil, err
		}
		return &user, nil

	case strings.Contains(path, "/users") && strings.Contains(path, "/get"):
		var req pbmo.GetUserRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		return &req, nil

	case strings.Contains(path, "/users") && strings.Contains(path, "/list"):
		var req pbmo.ListUsersRequest
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, err
		}
		return &req, nil

	case strings.Contains(path, "/products"):
		var product pbmo.Product
		if err := json.Unmarshal(body, &product); err != nil {
			return nil, err
		}
		return &product, nil

	case strings.Contains(path, "/orders"):
		var order pbmo.Order
		if err := json.Unmarshal(body, &order); err != nil {
			return nil, err
		}
		return &order, nil
	}

	return nil, fmt.Errorf("unknown PB type for path: %s", path)
}

// shouldSkipValidation 判断是否跳过验证
func (m *PBValidationMiddleware) shouldSkipValidation(methodName string) bool {
	// 定义需要跳过验证的路径
	skipPaths := []string{
		"/health",
		"/metrics",
		"/swagger",
		"/debug",
	}

	for _, skipPath := range skipPaths {
		if strings.Contains(methodName, skipPath) {
			return true
		}
	}

	return false
} // formatValidationError 格式化验证错误
func (m *PBValidationMiddleware) formatValidationError(err error) error {
	if validatorErrs, ok := err.(validator.ValidationErrors); ok {
		var errors []PBValidationError
		for _, fieldErr := range validatorErrs {
			errors = append(errors, PBValidationError{
				Field:   fieldErr.Field(),
				Tag:     fieldErr.Tag(),
				Value:   fieldErr.Value(),
				Message: m.getValidationMessage(fieldErr),
			})
		}

		errBytes, _ := json.Marshal(errors)
		return fmt.Errorf("validation failed: %s", string(errBytes))
	}

	return err
}

// getValidationMessage 获取验证错误的友好消息
func (m *PBValidationMiddleware) getValidationMessage(fieldErr validator.FieldError) string {
	field := fieldErr.Field()
	tag := fieldErr.Tag()
	param := fieldErr.Param()

	switch tag {
	case "required":
		return fmt.Sprintf("%s 是必填字段", field)
	case "min":
		return fmt.Sprintf("%s 的值必须大于等于 %s", field, param)
	case "max":
		return fmt.Sprintf("%s 的值必须小于等于 %s", field, param)
	case "email":
		return fmt.Sprintf("%s 必须是有效的邮箱地址", field)
	case "alphanum":
		return fmt.Sprintf("%s 只能包含字母和数字", field)
	case "len":
		return fmt.Sprintf("%s 的长度必须是 %s", field, param)
	case "oneof":
		return fmt.Sprintf("%s 必须是以下值之一: %s", field, param)
	case "pbmo_status":
		return fmt.Sprintf("%s 必须是有效的状态值(0-3)", field)
	case "pbmo_priority":
		return fmt.Sprintf("%s 必须是有效的优先级值(0-3)", field)
	default:
		return fmt.Sprintf("%s 验证失败: %s", field, tag)
	}
}

// writeErrorResponse 写入错误响应
func (m *PBValidationMiddleware) writeErrorResponse(w http.ResponseWriter, message string, validationErr error, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	var errors []PBValidationError
	if validationErr != nil {
		if validatorErrs, ok := validationErr.(validator.ValidationErrors); ok {
			for _, fieldErr := range validatorErrs {
				errors = append(errors, PBValidationError{
					Field:   fieldErr.Field(),
					Tag:     fieldErr.Tag(),
					Value:   fieldErr.Value(),
					Message: m.getValidationMessage(fieldErr),
				})
			}
		}
	}

	response := PBValidationResponse{
		Success: false,
		Message: message,
		Errors:  errors,
		Code:    code,
	}

	json.NewEncoder(w).Encode(response)
}

// validatingServerStream 验证流包装器
type validatingServerStream struct {
	grpc.ServerStream
	validator  *PBValidationMiddleware
	methodName string
}

func (s *validatingServerStream) RecvMsg(m interface{}) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}

	// 验证接收的消息
	if err := s.validator.validatePBStruct(m, s.methodName); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	return nil
}

// registerCustomValidationRules 注册自定义验证规则
func registerCustomValidationRules(v *validator.Validate) {
	// 注册状态枚举验证
	v.RegisterValidation("pbmo_status", func(fl validator.FieldLevel) bool {
		status := fl.Field().Int()
		return status >= 0 && status <= 3 // STATUS_UNKNOWN到STATUS_PENDING
	})

	// 注册优先级枚举验证
	v.RegisterValidation("pbmo_priority", func(fl validator.FieldLevel) bool {
		priority := fl.Field().Int()
		return priority >= 0 && priority <= 3 // PRIORITY_LOW到PRIORITY_CRITICAL
	})

	// 注册自定义字段验证
	v.RegisterValidation("pbmo_user_id", func(fl validator.FieldLevel) bool {
		id := fl.Field().Int()
		return id > 0 // 用户ID必须大于0
	})
}
