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
	// JWT和认证扩展
	ErrCodeTokenMalformed:        "Token格式错误",
	ErrCodeTokenNotValidYet:      "Token尚未激活",
	ErrCodeAccountLoginElsewhere: "账号已在其他地方登录，您已被迫下线",
	ErrCodeRedisParseError:       "解析Redis中的用户token时出错",
	ErrCodeDBQueryError:          "从数据库获取用户token异常",
	ErrCodeClaimsParseFailed:     "获取用户claims失败",
	// 数据转换和验证
	ErrCodePBMessageNil:         "PB message不能为空",
	ErrCodeModelMessageNil:      "Model message不能为空",
	ErrCodeFieldConversionError: "字段转换失败",
	ErrCodeTypeConversionError:  "类型转换失败",
	ErrCodeInitializationError:  "初始化失败",
	ErrCodeInvalidFieldMapping:  "无效的字段映射",
	ErrCodeUserIDMissing:        "缺少用户ID",
	ErrCodeMustBePointer:        "必须是指针类型",
	ErrCodeMustBeSlice:          "必须是切片类型",
	ErrCodeMustBeStruct:         "必须是结构体类型",
	ErrCodeElementConversion:    "元素转换失败",
	ErrCodeItemNil:              "项目为空",
	// 中间件和国际化
	ErrCodeLanguageLoadFailed: "语言加载失败",
	ErrCodeLanguageNotFound:   "语言未找到",
	ErrCodeJSONParseFailed:    "JSON解析失败",
	// 配置和特性
	ErrCodeInvalidConfigType:     "无效的配置类型",
	ErrCodeFeatureNotRegistered:  "特性未注册",
	ErrCodeFeatureEnableFailed:   "特性启用失败",
	ErrCodeMiddlewareInitFailed:  "中间件初始化失败",
	ErrCodeHealthManagerFailed:   "健康检查管理器创建失败",
	ErrCodeGRPCServerInitFailed:  "gRPC服务器初始化失败",
	ErrCodeHTTPGatewayInitFailed: "HTTP网关初始化失败",
	ErrCodeWSCNotEnabled:         "WSC功能未在配置中启用",
	ErrCodeWSCRouteFailed:        "注册WSC路由失败",
	ErrCodeUserAuthNotFound:      "未找到用户认证信息",
	// 服务器和基础设施
	ErrCodeServerCreationFailed: "创建服务器失败",
	ErrCodeScanTypeMismatch:     "扫描类型不匹配",
	ErrCodeUnknown:              "Unknown error",
	ErrCodeInternal:             "Internal error",
	ErrCodeOperationFailed:      "Operation failed",
	ErrCodeResourceNotFound:     "Resource not found",
	ErrCodeConflict:             "Conflict",
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
	// JWT和认证扩展
	ErrCodeTokenMalformed:        http.StatusUnauthorized,
	ErrCodeTokenNotValidYet:      http.StatusUnauthorized,
	ErrCodeAccountLoginElsewhere: http.StatusUnauthorized,
	ErrCodeRedisParseError:       http.StatusInternalServerError,
	ErrCodeDBQueryError:          http.StatusInternalServerError,
	ErrCodeClaimsParseFailed:     http.StatusUnauthorized,
	// 数据转换和验证
	ErrCodePBMessageNil:         http.StatusBadRequest,
	ErrCodeModelMessageNil:      http.StatusBadRequest,
	ErrCodeFieldConversionError: http.StatusInternalServerError,
	ErrCodeTypeConversionError:  http.StatusBadRequest,
	ErrCodeInitializationError:  http.StatusInternalServerError,
	ErrCodeInvalidFieldMapping:  http.StatusInternalServerError,
	ErrCodeUserIDMissing:        http.StatusBadRequest,
	ErrCodeMustBePointer:        http.StatusBadRequest,
	ErrCodeMustBeSlice:          http.StatusBadRequest,
	ErrCodeMustBeStruct:         http.StatusBadRequest,
	ErrCodeElementConversion:    http.StatusInternalServerError,
	ErrCodeItemNil:              http.StatusBadRequest,
	// 中间件和国际化
	ErrCodeLanguageLoadFailed: http.StatusInternalServerError,
	ErrCodeLanguageNotFound:   http.StatusNotFound,
	ErrCodeJSONParseFailed:    http.StatusInternalServerError,
	// 配置和特性
	ErrCodeInvalidConfigType:     http.StatusInternalServerError,
	ErrCodeFeatureNotRegistered:  http.StatusInternalServerError,
	ErrCodeFeatureEnableFailed:   http.StatusInternalServerError,
	ErrCodeMiddlewareInitFailed:  http.StatusInternalServerError,
	ErrCodeHealthManagerFailed:   http.StatusInternalServerError,
	ErrCodeGRPCServerInitFailed:  http.StatusInternalServerError,
	ErrCodeHTTPGatewayInitFailed: http.StatusInternalServerError,
	ErrCodeWSCNotEnabled:         http.StatusServiceUnavailable,
	ErrCodeWSCRouteFailed:        http.StatusInternalServerError,
	ErrCodeUserAuthNotFound:      http.StatusUnauthorized,
	// 服务器和基础设施
	ErrCodeServerCreationFailed: http.StatusInternalServerError,
	ErrCodeScanTypeMismatch:     http.StatusInternalServerError,
	ErrCodeUnknown:              http.StatusInternalServerError,
	ErrCodeInternal:             http.StatusInternalServerError,
	ErrCodeOperationFailed:      http.StatusInternalServerError,
	ErrCodeResourceNotFound:     http.StatusNotFound,
	ErrCodeConflict:             http.StatusConflict,
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
	// JWT和认证扩展
	ErrCodeTokenMalformed:        commonapis.StatusCode_Unauthenticated,
	ErrCodeTokenNotValidYet:      commonapis.StatusCode_Unauthenticated,
	ErrCodeAccountLoginElsewhere: commonapis.StatusCode_Unauthenticated,
	ErrCodeRedisParseError:       commonapis.StatusCode_Internal,
	ErrCodeDBQueryError:          commonapis.StatusCode_Internal,
	ErrCodeClaimsParseFailed:     commonapis.StatusCode_Unauthenticated,
	// 数据转换和验证
	ErrCodePBMessageNil:         commonapis.StatusCode_InvalidArgument,
	ErrCodeModelMessageNil:      commonapis.StatusCode_InvalidArgument,
	ErrCodeFieldConversionError: commonapis.StatusCode_Internal,
	ErrCodeTypeConversionError:  commonapis.StatusCode_InvalidArgument,
	ErrCodeInitializationError:  commonapis.StatusCode_Internal,
	ErrCodeInvalidFieldMapping:  commonapis.StatusCode_Internal,
	ErrCodeUserIDMissing:        commonapis.StatusCode_InvalidArgument,
	ErrCodeMustBePointer:        commonapis.StatusCode_InvalidArgument,
	ErrCodeMustBeSlice:          commonapis.StatusCode_InvalidArgument,
	ErrCodeMustBeStruct:         commonapis.StatusCode_InvalidArgument,
	ErrCodeElementConversion:    commonapis.StatusCode_Internal,
	ErrCodeItemNil:              commonapis.StatusCode_InvalidArgument,
	// 中间件和国际化
	ErrCodeLanguageLoadFailed: commonapis.StatusCode_Internal,
	ErrCodeLanguageNotFound:   commonapis.StatusCode_NotFound,
	ErrCodeJSONParseFailed:    commonapis.StatusCode_Internal,
	// 配置和特性
	ErrCodeInvalidConfigType:     commonapis.StatusCode_Internal,
	ErrCodeFeatureNotRegistered:  commonapis.StatusCode_Internal,
	ErrCodeFeatureEnableFailed:   commonapis.StatusCode_Internal,
	ErrCodeMiddlewareInitFailed:  commonapis.StatusCode_Internal,
	ErrCodeHealthManagerFailed:   commonapis.StatusCode_Internal,
	ErrCodeGRPCServerInitFailed:  commonapis.StatusCode_Internal,
	ErrCodeHTTPGatewayInitFailed: commonapis.StatusCode_Internal,
	ErrCodeWSCNotEnabled:         commonapis.StatusCode_Unavailable,
	ErrCodeWSCRouteFailed:        commonapis.StatusCode_Internal,
	ErrCodeUserAuthNotFound:      commonapis.StatusCode_Unauthenticated,
	// 服务器和基础设施
	ErrCodeServerCreationFailed: commonapis.StatusCode_Internal,
	ErrCodeScanTypeMismatch:     commonapis.StatusCode_Internal,
	ErrCodeUnknown:              commonapis.StatusCode_Unknown,
	ErrCodeInternal:             commonapis.StatusCode_Internal,
	ErrCodeOperationFailed:      commonapis.StatusCode_Internal,
	ErrCodeResourceNotFound:     commonapis.StatusCode_NotFound,
	ErrCodeConflict:             commonapis.StatusCode_AlreadyExists,
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

// WithDetailsf 使用格式化字符串添加错误详情
func (e *AppError) WithDetailsf(format string, args ...interface{}) *AppError {
	e.Details = fmt.Sprintf(format, args...)
	return e
}

// Wrap 包装标准错误为AppError，自动使用错误码对应的消息
func Wrap(err error, code ErrorCode) *AppError {
	if err == nil {
		return nil
	}
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	return NewError(code, err.Error())
}

// Wrapf 包装标准错误并添加额外的详细信息
// 使用错误码对应的消息，额外信息会添加到Details字段
func Wrapf(err error, code ErrorCode, details string) *AppError {
	if err == nil {
		return nil
	}
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	// 使用错误码的标准消息，将原始错误和额外信息放入Details
	fullDetails := details
	if err.Error() != "" {
		fullDetails = fmt.Sprintf("%s: %s", details, err.Error())
	}
	return NewError(code, fullDetails)
}

// WrapWithContext 包装错误并添加上下文信息（纯错误码模式）
// 只使用错误码，不传递消息
func WrapWithContext(err error, code ErrorCode) *AppError {
	if err == nil {
		return nil
	}
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	// 只使用错误码，原始错误作为Details
	return NewError(code, err.Error())
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

// JWT和认证扩展错误
var (
	ErrTokenMalformed        = NewError(ErrCodeTokenMalformed, "")
	ErrTokenNotValidYet      = NewError(ErrCodeTokenNotValidYet, "")
	ErrAccountLoginElsewhere = NewError(ErrCodeAccountLoginElsewhere, "")
	ErrRedisParseError       = NewError(ErrCodeRedisParseError, "")
	ErrDBQueryError          = NewError(ErrCodeDBQueryError, "")
	ErrClaimsParseFailed     = NewError(ErrCodeClaimsParseFailed, "")
)

// 数据转换和验证错误
var (
	ErrPBMessageNil         = NewError(ErrCodePBMessageNil, "")
	ErrModelMessageNil      = NewError(ErrCodeModelMessageNil, "")
	ErrFieldConversionError = NewError(ErrCodeFieldConversionError, "")
	ErrTypeConversionError  = NewError(ErrCodeTypeConversionError, "")
	ErrInitializationError  = NewError(ErrCodeInitializationError, "")
	ErrInvalidFieldMapping  = NewError(ErrCodeInvalidFieldMapping, "")
	ErrUserIDMissing        = NewError(ErrCodeUserIDMissing, "")
	ErrMustBePointer        = NewError(ErrCodeMustBePointer, "")
	ErrMustBeSlice          = NewError(ErrCodeMustBeSlice, "")
	ErrMustBeStruct         = NewError(ErrCodeMustBeStruct, "")
	ErrElementConversion    = NewError(ErrCodeElementConversion, "")
	ErrItemNil              = NewError(ErrCodeItemNil, "")
)

// 中间件和国际化错误
var (
	ErrLanguageLoadFailed = NewError(ErrCodeLanguageLoadFailed, "")
	ErrLanguageNotFound   = NewError(ErrCodeLanguageNotFound, "")
	ErrJSONParseFailed    = NewError(ErrCodeJSONParseFailed, "")
)

// 配置和特性错误
var (
	ErrInvalidConfigType     = NewError(ErrCodeInvalidConfigType, "")
	ErrFeatureNotRegistered  = NewError(ErrCodeFeatureNotRegistered, "")
	ErrFeatureEnableFailed   = NewError(ErrCodeFeatureEnableFailed, "")
	ErrMiddlewareInitFailed  = NewError(ErrCodeMiddlewareInitFailed, "")
	ErrHealthManagerFailed   = NewError(ErrCodeHealthManagerFailed, "")
	ErrGRPCServerInitFailed  = NewError(ErrCodeGRPCServerInitFailed, "")
	ErrHTTPGatewayInitFailed = NewError(ErrCodeHTTPGatewayInitFailed, "")
	ErrWSCNotEnabled         = NewError(ErrCodeWSCNotEnabled, "")
	ErrWSCRouteFailed        = NewError(ErrCodeWSCRouteFailed, "")
	ErrUserAuthNotFound      = NewError(ErrCodeUserAuthNotFound, "")
)

// 服务器和基础设施错误
var (
	ErrServerCreationFailed = NewError(ErrCodeServerCreationFailed, "")
	ErrScanTypeMismatch     = NewError(ErrCodeScanTypeMismatch, "")
)
