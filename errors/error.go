/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 06:55:40
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	ErrCodeServiceOverloaded:      "Service overloaded",
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
	// JWT | JWT
	ErrCodeTokenMalformed:   "Malformed token",
	ErrCodeTokenNotValidYet: "Token not valid yet",
	// 初始化 | initialization
	ErrCodeInitializationError: "Initialization failed",
	// 中间件与国际化 | middleware and i18n
	ErrCodeLanguageLoadFailed: "Failed to load language",
	ErrCodeLanguageNotFound:   "Language not found",
	ErrCodeJSONParseFailed:    "Failed to parse JSON",
	// 配置与特性 | configuration and features
	ErrCodeInvalidConfigType:     "Invalid configuration type",
	ErrCodeFeatureNotRegistered:  "Feature not registered",
	ErrCodeFeatureEnableFailed:   "Failed to enable feature",
	ErrCodeMiddlewareInitFailed:  "Failed to initialize middleware",
	ErrCodeHealthManagerFailed:   "Failed to create health manager",
	ErrCodeGRPCServerInitFailed:  "Failed to initialize gRPC server",
	ErrCodeHTTPGatewayInitFailed: "Failed to initialize HTTP gateway",
	// 服务器与基础设施 | server and infrastructure
	ErrCodeServerCreationFailed: "Failed to create server",
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
	ErrCodeServiceOverloaded:      http.StatusServiceUnavailable,
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
	// JWT | JWT
	ErrCodeTokenMalformed:   http.StatusUnauthorized,
	ErrCodeTokenNotValidYet: http.StatusUnauthorized,
	// 初始化 | initialization
	ErrCodeInitializationError: http.StatusInternalServerError,
	// 中间件与国际化 | middleware and i18n
	ErrCodeLanguageLoadFailed: http.StatusInternalServerError,
	ErrCodeLanguageNotFound:   http.StatusNotFound,
	ErrCodeJSONParseFailed:    http.StatusInternalServerError,
	// 配置与特性 | configuration and features
	ErrCodeInvalidConfigType:     http.StatusInternalServerError,
	ErrCodeFeatureNotRegistered:  http.StatusInternalServerError,
	ErrCodeFeatureEnableFailed:   http.StatusInternalServerError,
	ErrCodeMiddlewareInitFailed:  http.StatusInternalServerError,
	ErrCodeHealthManagerFailed:   http.StatusInternalServerError,
	ErrCodeGRPCServerInitFailed:  http.StatusInternalServerError,
	ErrCodeHTTPGatewayInitFailed: http.StatusInternalServerError,
	// 服务器与基础设施 | server and infrastructure
	ErrCodeServerCreationFailed: http.StatusInternalServerError,
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
	ErrCodeServiceOverloaded:      commonapis.StatusCode_Unavailable,
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
	// JWT | JWT
	ErrCodeTokenMalformed:   commonapis.StatusCode_Unauthenticated,
	ErrCodeTokenNotValidYet: commonapis.StatusCode_Unauthenticated,
	// 初始化 | initialization
	ErrCodeInitializationError: commonapis.StatusCode_Internal,
	// 中间件与国际化 | middleware and i18n
	ErrCodeLanguageLoadFailed: commonapis.StatusCode_Internal,
	ErrCodeLanguageNotFound:   commonapis.StatusCode_NotFound,
	ErrCodeJSONParseFailed:    commonapis.StatusCode_Internal,
	// 配置与特性 | configuration and features
	ErrCodeInvalidConfigType:     commonapis.StatusCode_Internal,
	ErrCodeFeatureNotRegistered:  commonapis.StatusCode_Internal,
	ErrCodeFeatureEnableFailed:   commonapis.StatusCode_Internal,
	ErrCodeMiddlewareInitFailed:  commonapis.StatusCode_Internal,
	ErrCodeHealthManagerFailed:   commonapis.StatusCode_Internal,
	ErrCodeGRPCServerInitFailed:  commonapis.StatusCode_Internal,
	ErrCodeHTTPGatewayInitFailed: commonapis.StatusCode_Internal,
	// 服务器与基础设施 | server and infrastructure
	ErrCodeServerCreationFailed: commonapis.StatusCode_Internal,
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

// ErrorOption 构造 AppError 时的可选项
//
// 为什么从「必填 details 形参」改成选项：原签名是 NewError(code, details string)，
// 而本包里 72 处预定义错误变量全都写成 NewError(ErrCodeXxx) —— 那个空串不表达
// 任何信息，只是形参要求你填点什么。有详情的调用不到二十处，让多数派去伺候少数派
// 是反的。
//
// 选项名与 AppError 的链式方法同名（WithDetails / WithDetailsf）：方法在类型的命名
// 空间里，包级函数在包的命名空间里，Go 允许重名，而两处同名同义比起名成
// WithDetail（差一个字母）要好记得多：
//
//	errors.NewError(code, errors.WithDetails("..."))  // 构造时给
//	appErr.WithDetails("...")                          // 拿到之后给
//
// [EN] Options for constructing an AppError.
//
// The old signature was NewError(code, details string), and all 72 predefined error variables
// in this package wrote NewError(ErrCodeXxx) — an empty string carrying no information,
// present only because the parameter demanded something. Fewer than twenty call sites have
// real details, so making the majority serve the minority had it backwards.
//
// The options share names with AppError's chaining methods: methods live in the type's
// namespace and package functions in the package's, so Go permits it, and one name meaning one
// thing in both places is easier to remember than a WithDetail that differs by one letter
type ErrorOption func(*AppError)

// WithDetails 填充错误详情
// [EN] WithDetails fills in the error's details
func WithDetails(details string) ErrorOption {
	return func(e *AppError) { e.Details = details }
}

// WithDetailsf 格式化填充错误详情
// [EN] WithDetailsf fills in the details from a format string
func WithDetailsf(format string, args ...interface{}) ErrorOption {
	return func(e *AppError) { e.Details = fmt.Sprintf(format, args...) }
}

// WithMessage 覆盖码表给出的消息
//
// 码表是单一来源，所以这个选项是给「同一个码在某一处需要更贴切的说法」用的例外口子，
// 不是绕开码表的常规写法 —— 消息写死在调用点，就回到了硬编码文案那条路上
//
// [EN] Overrides the message the code table supplies.
//
// The table is the single source, so this is an escape hatch for the one place where a code
// needs more precise wording, not the ordinary way to phrase an error: a message written at
// the call site is hardcoded copy again
func WithMessage(message string) ErrorOption {
	return func(e *AppError) { e.Message = message }
}

// NewError 按错误码创建错误；消息取自码表，详情等由选项给出
// [EN] NewError creates an error from a code, taking the message from the table and anything
// else from options
func NewError(code ErrorCode, opts ...ErrorOption) *AppError {
	message := errorMessages[ErrCodeUnknown]
	if msg, ok := errorMessages[code]; ok {
		message = msg
	}

	e := &AppError{
		Code:    code,
		Message: message,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}
	return e
}

// NewErrorf 使用格式化字符串创建错误
func NewErrorf(code ErrorCode, format string, args ...interface{}) *AppError {
	return NewError(code, WithDetailsf(format, args...))
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
	return NewError(code, WithDetails(err.Error()))
}

// Wrapf 包装标准错误并添加额外的详细信息
// 使用错误码对应的消息，额外信息会添加到Details字段
func Wrapf(err error, code ErrorCode, format string, args ...interface{}) *AppError {
	if err == nil {
		return nil
	}
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	// 使用错误码的标准消息，将原始错误和格式化信息放入Details
	fullDetails := fmt.Sprintf(format, args...)
	if err.Error() != "" {
		fullDetails = fmt.Sprintf("%s: %s", fullDetails, err.Error())
	}
	return NewError(code, WithDetails(fullDetails))
}

// ToGRPCError 将 AppError 转换为 gRPC status.Error
// 使用 commonapis.StatusCode 转换为标准 gRPC codes
func (e *AppError) ToGRPCError() error {
	if e == nil {
		return nil
	}

	// 获取 gRPC 状态码
	statusCode := e.GetStatusCode()

	// 转换为 gRPC codes
	var code codes.Code
	switch statusCode {
	case commonapis.StatusCode_OK:
		code = codes.OK
	case commonapis.StatusCode_Canceled:
		code = codes.Canceled
	case commonapis.StatusCode_Unknown:
		code = codes.Unknown
	case commonapis.StatusCode_InvalidArgument:
		code = codes.InvalidArgument
	case commonapis.StatusCode_DeadlineExceeded:
		code = codes.DeadlineExceeded
	case commonapis.StatusCode_NotFound:
		code = codes.NotFound
	case commonapis.StatusCode_AlreadyExists:
		code = codes.AlreadyExists
	case commonapis.StatusCode_PermissionDenied:
		code = codes.PermissionDenied
	case commonapis.StatusCode_ResourceExhausted:
		code = codes.ResourceExhausted
	case commonapis.StatusCode_FailedPrecondition:
		code = codes.FailedPrecondition
	case commonapis.StatusCode_Aborted:
		code = codes.Aborted
	case commonapis.StatusCode_OutOfRange:
		code = codes.OutOfRange
	case commonapis.StatusCode_Unimplemented:
		code = codes.Unimplemented
	case commonapis.StatusCode_Internal:
		code = codes.Internal
	case commonapis.StatusCode_Unavailable:
		code = codes.Unavailable
	case commonapis.StatusCode_DataLoss:
		code = codes.DataLoss
	case commonapis.StatusCode_Unauthenticated:
		code = codes.Unauthenticated
	default:
		code = codes.Unknown
	}

	// 构建错误消息
	message := e.Message
	if e.Details != "" {
		message = fmt.Sprintf("%s: %s", e.Message, e.Details)
	}

	return status.Error(code, message)
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
	ErrGatewayNotInitialized = NewError(ErrCodeGatewayNotInitialized)
	ErrInvalidConfiguration  = NewError(ErrCodeInvalidConfiguration)
	ErrServiceUnavailable    = NewError(ErrCodeServiceUnavailable)
	ErrInternalServerError   = NewError(ErrCodeInternalServerError)
	ErrGatewayTimeout        = NewError(ErrCodeGatewayTimeout)
)

// 认证授权错误
var (
	ErrUnauthorized       = NewError(ErrCodeUnauthorized)
	ErrForbidden          = NewError(ErrCodeForbidden)
	ErrInvalidToken       = NewError(ErrCodeInvalidToken)
	ErrTokenExpired       = NewError(ErrCodeTokenExpired)
	ErrInvalidCredentials = NewError(ErrCodeInvalidCredentials)
	ErrCSRFTokenInvalid   = NewError(ErrCodeCSRFTokenInvalid)
)

// 请求处理错误
var (
	ErrBadRequest         = NewError(ErrCodeBadRequest)
	ErrNotFound           = NewError(ErrCodeNotFound)
	ErrMethodNotAllowed   = NewError(ErrCodeMethodNotAllowed)
	ErrInvalidContentType = NewError(ErrCodeInvalidContentType)
	ErrRequestTooLarge    = NewError(ErrCodeRequestTooLarge)
	ErrInvalidParameter   = NewError(ErrCodeInvalidParameter)
	ErrMissingParameter   = NewError(ErrCodeMissingParameter)
)

// 限流和熔断错误
var (
	ErrTooManyRequests    = NewError(ErrCodeTooManyRequests)
	ErrRateLimitExceeded  = NewError(ErrCodeRateLimitExceeded)
	ErrCircuitBreakerOpen = NewError(ErrCodeCircuitBreakerOpen)
	ErrServiceDegraded    = NewError(ErrCodeServiceDegraded)
	ErrServiceOverloaded  = NewError(ErrCodeServiceOverloaded)
)

// 中间件错误
var (
	ErrMiddlewareError  = NewError(ErrCodeMiddlewareError)
	ErrRecoveryError    = NewError(ErrCodeRecoveryError)
	ErrLoggingError     = NewError(ErrCodeLoggingError)
	ErrTracingError     = NewError(ErrCodeTracingError)
	ErrMetricsError     = NewError(ErrCodeMetricsError)
	ErrSecurityError    = NewError(ErrCodeSecurityError)
	ErrSignatureInvalid = NewError(ErrCodeSignatureInvalid)
)

// gRPC相关错误
var (
	ErrGRPCConnectionFailed = NewError(ErrCodeGRPCConnectionFailed)
	ErrGRPCServiceNotFound  = NewError(ErrCodeGRPCServiceNotFound)
	ErrGRPCMethodNotFound   = NewError(ErrCodeGRPCMethodNotFound)
	ErrGRPCTimeout          = NewError(ErrCodeGRPCTimeout)
	ErrGRPCCanceled         = NewError(ErrCodeGRPCCanceled)
)

// 健康检查错误
var (
	ErrHealthCheckFailed      = NewError(ErrCodeHealthCheckFailed)
	ErrHealthCheckTimeout     = NewError(ErrCodeHealthCheckTimeout)
	ErrHealthCheckUnavailable = NewError(ErrCodeHealthCheckUnavailable)
)

// Swagger文档错误
var (
	ErrSwaggerNotFound     = NewError(ErrCodeSwaggerNotFound)
	ErrSwaggerLoadFailed   = NewError(ErrCodeSwaggerLoadFailed)
	ErrSwaggerRenderFailed = NewError(ErrCodeSwaggerRenderFailed)
)

// JWT和认证扩展错误
var (
	ErrTokenMalformed   = NewError(ErrCodeTokenMalformed)
	ErrTokenNotValidYet = NewError(ErrCodeTokenNotValidYet)
)

// 数据转换和验证错误
var (
	ErrInitializationError = NewError(ErrCodeInitializationError)
)

// 中间件和国际化错误
var (
	ErrLanguageLoadFailed = NewError(ErrCodeLanguageLoadFailed)
	ErrLanguageNotFound   = NewError(ErrCodeLanguageNotFound)
	ErrJSONParseFailed    = NewError(ErrCodeJSONParseFailed)
)

// 配置和特性错误
var (
	ErrInvalidConfigType     = NewError(ErrCodeInvalidConfigType)
	ErrFeatureNotRegistered  = NewError(ErrCodeFeatureNotRegistered)
	ErrFeatureEnableFailed   = NewError(ErrCodeFeatureEnableFailed)
	ErrMiddlewareInitFailed  = NewError(ErrCodeMiddlewareInitFailed)
	ErrHealthManagerFailed   = NewError(ErrCodeHealthManagerFailed)
	ErrGRPCServerInitFailed  = NewError(ErrCodeGRPCServerInitFailed)
	ErrHTTPGatewayInitFailed = NewError(ErrCodeHTTPGatewayInitFailed)
)

// 服务器和基础设施错误
var (
	ErrServerCreationFailed = NewError(ErrCodeServerCreationFailed)
)
