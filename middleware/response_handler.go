/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-17 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 22:00:00
 * @FilePath: \go-rpc-gateway\middleware\response_handler.go
 * @Description: 响应处理中间件 - 自动处理错误码转换为标准响应
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"net/http"

	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/response"
)

// ResponseHandlerConfig 响应处理配置
type ResponseHandlerConfig struct {
	Format           response.ResponseFormat `json:"format"`            // 响应格式
	EnableAutoConvert bool                   `json:"enable_auto_convert"` // 启用自动转换
	HandlePanic      bool                   `json:"handle_panic"`       // 处理panic
	LogErrors        bool                   `json:"log_errors"`         // 记录错误日志
}

// DefaultResponseHandlerConfig 默认响应处理配置
func DefaultResponseHandlerConfig() *ResponseHandlerConfig {
	return &ResponseHandlerConfig{
		Format:           response.FormatStandard,
		EnableAutoConvert: true,
		HandlePanic:      true,
		LogErrors:        true,
	}
}

// ResponseHandlerMiddleware 响应处理中间件
func ResponseHandlerMiddleware(config *ResponseHandlerConfig) func(http.Handler) http.Handler {
	if config == nil {
		config = DefaultResponseHandlerConfig()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 创建自定义ResponseWriter来拦截响应
			wrapper := &responseWrapper{
				ResponseWriter: w,
				config:        config,
				request:       r,
			}

			if config.HandlePanic {
				defer func() {
					if err := recover(); err != nil {
						if config.LogErrors && global.LOGGER != nil {
							global.LOGGER.Error("Panic recovered in response handler: %v", err)
						}
						
						// 处理panic，返回内部服务器错误
						appErr := errors.NewError(errors.ErrCodeInternalServerError, "Internal server error")
						response.WriteStandardError(wrapper.ResponseWriter, config.Format, appErr)
					}
				}()
			}

			next.ServeHTTP(wrapper, r)
		})
	}
}

// responseWrapper 响应包装器
type responseWrapper struct {
	http.ResponseWriter
	config  *ResponseHandlerConfig
	request *http.Request
	written bool
}

// Write 拦截写入操作
func (rw *responseWrapper) Write(data []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	return rw.ResponseWriter.Write(data)
}

// WriteHeader 拦截状态码写入
func (rw *responseWrapper) WriteHeader(statusCode int) {
	if !rw.written {
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(statusCode)
}

// ErrorHandlerFunc 错误处理函数类型
type ErrorHandlerFunc func(w http.ResponseWriter, r *http.Request, err error)

// GlobalErrorHandler 全局错误处理器
var GlobalErrorHandler ErrorHandlerFunc

// SetGlobalErrorHandler 设置全局错误处理器
func SetGlobalErrorHandler(handler ErrorHandlerFunc) {
	GlobalErrorHandler = handler
}

// HandleError 处理错误并返回标准响应
func HandleError(w http.ResponseWriter, r *http.Request, err error, format response.ResponseFormat) {
	if err == nil {
		return
	}

	// 如果有全局错误处理器，优先使用
	if GlobalErrorHandler != nil {
		GlobalErrorHandler(w, r, err)
		return
	}

	ctx := r.Context()

	// 将错误转换为AppError
	var appErr *errors.AppError
	if ae, ok := err.(*errors.AppError); ok {
		appErr = ae
	} else {
		// 根据错误类型尝试推断错误码
		code := inferErrorCode(err)
		appErr = errors.NewError(code, err.Error())
	}

	// 记录错误日志（包含上下文信息）
	if global.LOGGER != nil {
		fields := []interface{}{
			"error_code", appErr.Code,
			"error_message", appErr.Message,
			"method", r.Method,
			"path", r.URL.Path,
		}

		// 添加用户信息（如果存在）
		if userID := logger.GetUserID(ctx); userID != "" {
			fields = append(fields, "user_id", userID)
		}
		if tenantID := logger.GetTenantID(ctx); tenantID != "" {
			fields = append(fields, "tenant_id", tenantID)
		}

		global.LOGGER.ErrorContextKV(ctx, "Request Error", fields...)
	}

	// 返回标准错误响应（包含 trace_id）
	response.WriteStandardError(w, format, appErr)
}

// inferErrorCode 根据错误推断错误码
func inferErrorCode(err error) errors.ErrorCode {
	errMsg := err.Error()
	
	// 常见错误模式匹配
	switch {
	case containsAny(errMsg, []string{"not found", "404"}):
		return errors.ErrCodeNotFound
	case containsAny(errMsg, []string{"unauthorized", "401"}):
		return errors.ErrCodeUnauthorized
	case containsAny(errMsg, []string{"forbidden", "403"}):
		return errors.ErrCodeForbidden
	case containsAny(errMsg, []string{"bad request", "400", "invalid"}):
		return errors.ErrCodeBadRequest
	case containsAny(errMsg, []string{"timeout", "deadline"}):
		return errors.ErrCodeGatewayTimeout
	case containsAny(errMsg, []string{"too many", "rate limit"}):
		return errors.ErrCodeTooManyRequests
	case containsAny(errMsg, []string{"service unavailable", "503"}):
		return errors.ErrCodeServiceUnavailable
	default:
		return errors.ErrCodeInternalServerError
	}
}

// containsAny 检查字符串是否包含任一子字符串
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

// contains 简单的字符串包含检查（忽略大小写）
func contains(s, substr string) bool {
	// 简单实现，可以使用strings.Contains和strings.ToLower
	// 这里为了避免额外依赖，简单实现
	return len(s) >= len(substr) && findSubstring(s, substr)
}

// findSubstring 查找子字符串
func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1 := s[i+j]
			c2 := substr[j]
			// 简单的大小写不敏感比较
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 'a' - 'A'
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 'a' - 'A'
			}
			if c1 != c2 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// ContextKey 上下文键类型
type ContextKey string

const (
	// ResponseFormatKey 响应格式上下文键
	ResponseFormatKey ContextKey = "response_format"
)

// SetResponseFormat 设置响应格式到上下文
func SetResponseFormat(ctx context.Context, format response.ResponseFormat) context.Context {
	return context.WithValue(ctx, ResponseFormatKey, format)
}

// GetResponseFormat 从上下文获取响应格式
func GetResponseFormat(ctx context.Context) response.ResponseFormat {
	if format, ok := ctx.Value(ResponseFormatKey).(response.ResponseFormat); ok {
		return format
	}
	return response.FormatStandard // 默认格式
}

// GetResponseFormatFromRequest 从请求中获取响应格式
func GetResponseFormatFromRequest(r *http.Request) response.ResponseFormat {
	// 从查询参数获取
	if format := r.URL.Query().Get("format"); format != "" {
		return response.ResponseFormat(format)
	}
	
	// 从Header获取
	if format := r.Header.Get("X-Response-Format"); format != "" {
		return response.ResponseFormat(format)
	}
	
	// 从上下文获取
	return GetResponseFormat(r.Context())
}

// WithResponseFormat 中间件：从请求中提取响应格式并设置到上下文
func WithResponseFormat() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			format := GetResponseFormatFromRequest(r)
			ctx := SetResponseFormat(r.Context(), format)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}