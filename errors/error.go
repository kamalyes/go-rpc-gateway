/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 06:43:40
 * @FilePath: \go-rpc-gateway\errors\error.go
 * @Description: 统一的错误定义和管理
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package errors

import (
	"fmt"
	"net/http"

	commonapis "github.com/kamalyes/go-rpc-gateway/proto"
)

// errorMessages 错误消息映射
var errorMessages = map[ErrorCode]string{
	ErrCodeOK:                     "OK",
	ErrCodeGatewayNotInitialized:  "Gateway not initialized",
	ErrCodeInvalidConfiguration:   "Invalid configuration",
	ErrCodeServiceUnavailable:     "Service unavailable",
	ErrCodeInternalServerError:    "Internal server error",
	ErrCodeGatewayTimeout:         "Gateway timeout",
	ErrCodeUnauthorized:           "Unauthorized",
	ErrCodeForbidden:              "Forbidden",
	ErrCodeInvalidToken:           "Invalid token",
	ErrCodeTokenExpired:           "Token expired",
	ErrCodeInvalidCredentials:     "Invalid credentials",
	ErrCodeCSRFTokenInvalid:       "CSRF token validation failed",
	ErrCodeBadRequest:             "Bad request",
	ErrCodeNotFound:               "Not found",
	ErrCodeMethodNotAllowed:       "Method not allowed",
	ErrCodeInvalidContentType:     "Invalid content type",
	ErrCodeRequestTooLarge:        "Request too large",
	ErrCodeInvalidParameter:       "Invalid parameter",
	ErrCodeMissingParameter:       "Missing parameter",
	ErrCodeTooManyRequests:        "Too many requests",
	ErrCodeRateLimitExceeded:      "Rate limit exceeded",
	ErrCodeCircuitBreakerOpen:     "Circuit breaker open",
	ErrCodeServiceDegraded:        "Service degraded",
	ErrCodeMiddlewareError:        "Middleware error",
	ErrCodeRecoveryError:          "Recovery error",
	ErrCodeLoggingError:           "Logging error",
	ErrCodeTracingError:           "Tracing error",
	ErrCodeMetricsError:           "Metrics error",
	ErrCodeSecurityError:          "Security error",
	ErrCodeSignatureInvalid:       "Invalid signature",
	ErrCodeGRPCConnectionFailed:   "gRPC connection failed",
	ErrCodeGRPCServiceNotFound:    "gRPC service not found",
	ErrCodeGRPCMethodNotFound:     "gRPC method not found",
	ErrCodeGRPCTimeout:            "gRPC timeout",
	ErrCodeGRPCCanceled:           "gRPC canceled",
	ErrCodeHealthCheckFailed:      "Health check failed",
	ErrCodeHealthCheckTimeout:     "Health check timeout",
	ErrCodeHealthCheckUnavailable: "Health check unavailable",
	ErrCodeSwaggerNotFound:        "Swagger JSON not found",
	ErrCodeSwaggerLoadFailed:      "Failed to load Swagger",
	ErrCodeSwaggerRenderFailed:    "Failed to render Swagger UI",
	ErrCodeUnknown:                "Unknown error",
	ErrCodeInternal:               "Internal error",
	ErrCodeOperationFailed:        "Operation failed",
	ErrCodeResourceNotFound:       "Resource not found",
	ErrCodeConflict:               "Conflict",
}

// httpStatusMapping 错误码到HTTP状态码的映射
var httpStatusMapping = map[ErrorCode]int{
	ErrCodeOK:                     http.StatusOK,
	ErrCodeGatewayNotInitialized:  http.StatusInternalServerError,
	ErrCodeInvalidConfiguration:   http.StatusInternalServerError,
	ErrCodeServiceUnavailable:     http.StatusServiceUnavailable,
	ErrCodeInternalServerError:    http.StatusInternalServerError,
	ErrCodeGatewayTimeout:         http.StatusGatewayTimeout,
	ErrCodeUnauthorized:           http.StatusUnauthorized,
	ErrCodeForbidden:              http.StatusForbidden,
	ErrCodeInvalidToken:           http.StatusUnauthorized,
	ErrCodeTokenExpired:           http.StatusUnauthorized,
	ErrCodeInvalidCredentials:     http.StatusUnauthorized,
	ErrCodeCSRFTokenInvalid:       http.StatusForbidden,
	ErrCodeBadRequest:             http.StatusBadRequest,
	ErrCodeNotFound:               http.StatusNotFound,
	ErrCodeMethodNotAllowed:       http.StatusMethodNotAllowed,
	ErrCodeInvalidContentType:     http.StatusBadRequest,
	ErrCodeRequestTooLarge:        http.StatusRequestEntityTooLarge,
	ErrCodeInvalidParameter:       http.StatusBadRequest,
	ErrCodeMissingParameter:       http.StatusBadRequest,
	ErrCodeTooManyRequests:        http.StatusTooManyRequests,
	ErrCodeRateLimitExceeded:      http.StatusTooManyRequests,
	ErrCodeCircuitBreakerOpen:     http.StatusServiceUnavailable,
	ErrCodeServiceDegraded:        http.StatusServiceUnavailable,
	ErrCodeMiddlewareError:        http.StatusInternalServerError,
	ErrCodeRecoveryError:          http.StatusInternalServerError,
	ErrCodeLoggingError:           http.StatusInternalServerError,
	ErrCodeTracingError:           http.StatusInternalServerError,
	ErrCodeMetricsError:           http.StatusInternalServerError,
	ErrCodeSecurityError:          http.StatusForbidden,
	ErrCodeSignatureInvalid:       http.StatusUnauthorized,
	ErrCodeGRPCConnectionFailed:   http.StatusBadGateway,
	ErrCodeGRPCServiceNotFound:    http.StatusNotFound,
	ErrCodeGRPCMethodNotFound:     http.StatusNotFound,
	ErrCodeGRPCTimeout:            http.StatusGatewayTimeout,
	ErrCodeGRPCCanceled:           http.StatusRequestTimeout,
	ErrCodeHealthCheckFailed:      http.StatusServiceUnavailable,
	ErrCodeHealthCheckTimeout:     http.StatusGatewayTimeout,
	ErrCodeHealthCheckUnavailable: http.StatusServiceUnavailable,
	ErrCodeSwaggerNotFound:        http.StatusNotFound,
	ErrCodeSwaggerLoadFailed:      http.StatusInternalServerError,
	ErrCodeSwaggerRenderFailed:    http.StatusInternalServerError,
	ErrCodeUnknown:                http.StatusInternalServerError,
	ErrCodeInternal:               http.StatusInternalServerError,
	ErrCodeOperationFailed:        http.StatusInternalServerError,
	ErrCodeResourceNotFound:       http.StatusNotFound,
	ErrCodeConflict:               http.StatusConflict,
}

// statusCodeMapping 错误码到gRPC状态码的映射
var statusCodeMapping = map[ErrorCode]commonapis.StatusCode{
	ErrCodeOK:                     commonapis.StatusCode_OK,
	ErrCodeGatewayNotInitialized:  commonapis.StatusCode_Internal,
	ErrCodeInvalidConfiguration:   commonapis.StatusCode_Internal,
	ErrCodeServiceUnavailable:     commonapis.StatusCode_Unavailable,
	ErrCodeInternalServerError:    commonapis.StatusCode_Internal,
	ErrCodeGatewayTimeout:         commonapis.StatusCode_DeadlineExceeded,
	ErrCodeUnauthorized:           commonapis.StatusCode_Unauthenticated,
	ErrCodeForbidden:              commonapis.StatusCode_PermissionDenied,
	ErrCodeInvalidToken:           commonapis.StatusCode_Unauthenticated,
	ErrCodeTokenExpired:           commonapis.StatusCode_Unauthenticated,
	ErrCodeInvalidCredentials:     commonapis.StatusCode_Unauthenticated,
	ErrCodeCSRFTokenInvalid:       commonapis.StatusCode_PermissionDenied,
	ErrCodeBadRequest:             commonapis.StatusCode_InvalidArgument,
	ErrCodeNotFound:               commonapis.StatusCode_NotFound,
	ErrCodeMethodNotAllowed:       commonapis.StatusCode_Unimplemented,
	ErrCodeInvalidContentType:     commonapis.StatusCode_InvalidArgument,
	ErrCodeRequestTooLarge:        commonapis.StatusCode_InvalidArgument,
	ErrCodeInvalidParameter:       commonapis.StatusCode_InvalidArgument,
	ErrCodeMissingParameter:       commonapis.StatusCode_InvalidArgument,
	ErrCodeTooManyRequests:        commonapis.StatusCode_ResourceExhausted,
	ErrCodeRateLimitExceeded:      commonapis.StatusCode_ResourceExhausted,
	ErrCodeCircuitBreakerOpen:     commonapis.StatusCode_Unavailable,
	ErrCodeServiceDegraded:        commonapis.StatusCode_Unavailable,
	ErrCodeMiddlewareError:        commonapis.StatusCode_Internal,
	ErrCodeRecoveryError:          commonapis.StatusCode_Internal,
	ErrCodeLoggingError:           commonapis.StatusCode_Internal,
	ErrCodeTracingError:           commonapis.StatusCode_Internal,
	ErrCodeMetricsError:           commonapis.StatusCode_Internal,
	ErrCodeSecurityError:          commonapis.StatusCode_PermissionDenied,
	ErrCodeSignatureInvalid:       commonapis.StatusCode_Unauthenticated,
	ErrCodeGRPCConnectionFailed:   commonapis.StatusCode_Unavailable,
	ErrCodeGRPCServiceNotFound:    commonapis.StatusCode_NotFound,
	ErrCodeGRPCMethodNotFound:     commonapis.StatusCode_Unimplemented,
	ErrCodeGRPCTimeout:            commonapis.StatusCode_DeadlineExceeded,
	ErrCodeGRPCCanceled:           commonapis.StatusCode_Canceled,
	ErrCodeHealthCheckFailed:      commonapis.StatusCode_Unavailable,
	ErrCodeHealthCheckTimeout:     commonapis.StatusCode_DeadlineExceeded,
	ErrCodeHealthCheckUnavailable: commonapis.StatusCode_Unavailable,
	ErrCodeSwaggerNotFound:        commonapis.StatusCode_NotFound,
	ErrCodeSwaggerLoadFailed:      commonapis.StatusCode_Internal,
	ErrCodeSwaggerRenderFailed:    commonapis.StatusCode_Internal,
	ErrCodeUnknown:                commonapis.StatusCode_Unknown,
	ErrCodeInternal:               commonapis.StatusCode_Internal,
	ErrCodeOperationFailed:        commonapis.StatusCode_Internal,
	ErrCodeResourceNotFound:       commonapis.StatusCode_NotFound,
	ErrCodeConflict:               commonapis.StatusCode_AlreadyExists,
}

// AppError 应用错误结构
type AppError struct {
	Code    ErrorCode // 错误代码
	Message string    // 错误消息
	Details string    // 错误详情
}

// NewError 创建新错误
func NewError(code ErrorCode, details string) *AppError {
	message := errorMessages[ErrCodeUnknown]
	if msg, ok := errorMessages[code]; ok {
		message = msg
	}

	return &AppError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// NewErrorf 使用格式化字符串创建错误
func NewErrorf(code ErrorCode, format string, args ...interface{}) *AppError {
	return NewError(code, fmt.Sprintf(format, args...))
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.Details == "" {
		return fmt.Sprintf("[%d] %s", e.Code, e.Message)
	}
	return fmt.Sprintf("[%d] %s: %s", e.Code, e.Message, e.Details)
}

// String 实现 Stringer 接口，便于直接打印
func (e *AppError) String() string {
	return e.Error()
}

// GetCode 获取错误代码
func (e *AppError) GetCode() ErrorCode {
	return e.Code
}

// GetMessage 获取错误消息
func (e *AppError) GetMessage() string {
	return e.Message
}

// GetDetails 获取错误详情
func (e *AppError) GetDetails() string {
	return e.Details
}

// GetHTTPStatus 获取对应的HTTP状态码
func (e *AppError) GetHTTPStatus() int {
	if status, ok := httpStatusMapping[e.Code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// GetStatusCode 获取对应的gRPC状态码
func (e *AppError) GetStatusCode() commonapis.StatusCode {
	if status, ok := statusCodeMapping[e.Code]; ok {
		return status
	}
	return commonapis.StatusCode_Unknown
}

// ToResult 转换为Result结构
func (e *AppError) ToResult() *commonapis.Result {
	errorMessage := e.Message
	if e.Details != "" {
		errorMessage = e.Details
	}

	return &commonapis.Result{
		Code:   int32(e.GetHTTPStatus()),
		Error:  errorMessage,
		Status: e.GetStatusCode(),
	}
}

// WithDetails 添加错误详情
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// IsErrorCode 检查错误代码是否匹配
func IsErrorCode(err error, code ErrorCode) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == code
	}
	return false
}

// GetErrorCode 从错误中提取错误代码
func GetErrorCode(err error) ErrorCode {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code
	}
	return ErrCodeUnknown
}

// ErrorCodeString 获取错误代码的字符串表示
func ErrorCodeString(code ErrorCode) string {
	if msg, ok := errorMessages[code]; ok {
		return msg
	}
	return errorMessages[ErrCodeUnknown]
}

// 预定义错误变量

// 网关核心错误
var (
	ErrGatewayNotInitialized = NewError(ErrCodeGatewayNotInitialized, "")
	ErrInvalidConfiguration  = NewError(ErrCodeInvalidConfiguration, "")
	ErrServiceUnavailable    = NewError(ErrCodeServiceUnavailable, "")
	ErrInternalServerError   = NewError(ErrCodeInternalServerError, "")
	ErrGatewayTimeout        = NewError(ErrCodeGatewayTimeout, "")
)

// 认证授权错误
var (
	ErrUnauthorized       = NewError(ErrCodeUnauthorized, "")
	ErrForbidden          = NewError(ErrCodeForbidden, "")
	ErrInvalidToken       = NewError(ErrCodeInvalidToken, "")
	ErrTokenExpired       = NewError(ErrCodeTokenExpired, "")
	ErrInvalidCredentials = NewError(ErrCodeInvalidCredentials, "")
	ErrCSRFTokenInvalid   = NewError(ErrCodeCSRFTokenInvalid, "")
)

// 请求处理错误
var (
	ErrBadRequest         = NewError(ErrCodeBadRequest, "")
	ErrNotFound           = NewError(ErrCodeNotFound, "")
	ErrMethodNotAllowed   = NewError(ErrCodeMethodNotAllowed, "")
	ErrInvalidContentType = NewError(ErrCodeInvalidContentType, "")
	ErrRequestTooLarge    = NewError(ErrCodeRequestTooLarge, "")
	ErrInvalidParameter   = NewError(ErrCodeInvalidParameter, "")
	ErrMissingParameter   = NewError(ErrCodeMissingParameter, "")
)

// 限流和熔断错误
var (
	ErrTooManyRequests    = NewError(ErrCodeTooManyRequests, "")
	ErrRateLimitExceeded  = NewError(ErrCodeRateLimitExceeded, "")
	ErrCircuitBreakerOpen = NewError(ErrCodeCircuitBreakerOpen, "")
	ErrServiceDegraded    = NewError(ErrCodeServiceDegraded, "")
)

// 中间件错误
var (
	ErrMiddlewareError  = NewError(ErrCodeMiddlewareError, "")
	ErrRecoveryError    = NewError(ErrCodeRecoveryError, "")
	ErrLoggingError     = NewError(ErrCodeLoggingError, "")
	ErrTracingError     = NewError(ErrCodeTracingError, "")
	ErrMetricsError     = NewError(ErrCodeMetricsError, "")
	ErrSecurityError    = NewError(ErrCodeSecurityError, "")
	ErrSignatureInvalid = NewError(ErrCodeSignatureInvalid, "")
)

// gRPC相关错误
var (
	ErrGRPCConnectionFailed = NewError(ErrCodeGRPCConnectionFailed, "")
	ErrGRPCServiceNotFound  = NewError(ErrCodeGRPCServiceNotFound, "")
	ErrGRPCMethodNotFound   = NewError(ErrCodeGRPCMethodNotFound, "")
	ErrGRPCTimeout          = NewError(ErrCodeGRPCTimeout, "")
	ErrGRPCCanceled         = NewError(ErrCodeGRPCCanceled, "")
)

// 健康检查错误
var (
	ErrHealthCheckFailed      = NewError(ErrCodeHealthCheckFailed, "")
	ErrHealthCheckTimeout     = NewError(ErrCodeHealthCheckTimeout, "")
	ErrHealthCheckUnavailable = NewError(ErrCodeHealthCheckUnavailable, "")
)

// Swagger文档错误
var (
	ErrSwaggerNotFound     = NewError(ErrCodeSwaggerNotFound, "")
	ErrSwaggerLoadFailed   = NewError(ErrCodeSwaggerLoadFailed, "")
	ErrSwaggerRenderFailed = NewError(ErrCodeSwaggerRenderFailed, "")
)
