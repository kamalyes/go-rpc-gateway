/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 15:08:33
 * @FilePath: \go-rpc-gateway\errors\code.go
 * @Description: 统一的错误代码定义和管理
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package errors

// ErrorCode 定义错误代码类型
type ErrorCode int

// 定义错误代码常量
const (
	ErrCodeOK ErrorCode = iota

	// 网关核心错误 (1000-1999)
	ErrCodeGatewayNotInitialized ErrorCode = 1001
	ErrCodeInvalidConfiguration  ErrorCode = 1002
	ErrCodeServiceUnavailable    ErrorCode = 1003
	ErrCodeInternalServerError   ErrorCode = 1004
	ErrCodeGatewayTimeout        ErrorCode = 1005

	// 认证授权错误 (2000-2999)
	ErrCodeUnauthorized       ErrorCode = 2001
	ErrCodeForbidden          ErrorCode = 2002
	ErrCodeInvalidToken       ErrorCode = 2003
	ErrCodeTokenExpired       ErrorCode = 2004
	ErrCodeInvalidCredentials ErrorCode = 2005
	ErrCodeCSRFTokenInvalid   ErrorCode = 2006

	// 请求处理错误 (3000-3999)
	ErrCodeBadRequest         ErrorCode = 3001
	ErrCodeNotFound           ErrorCode = 3002
	ErrCodeMethodNotAllowed   ErrorCode = 3003
	ErrCodeInvalidContentType ErrorCode = 3004
	ErrCodeRequestTooLarge    ErrorCode = 3005
	ErrCodeInvalidParameter   ErrorCode = 3006
	ErrCodeMissingParameter   ErrorCode = 3007

	// 限流和熔断错误 (4000-4999)
	ErrCodeTooManyRequests    ErrorCode = 4001
	ErrCodeRateLimitExceeded  ErrorCode = 4002
	ErrCodeCircuitBreakerOpen ErrorCode = 4003
	ErrCodeServiceDegraded    ErrorCode = 4004

	// 中间件错误 (5000-5999)
	ErrCodeMiddlewareError  ErrorCode = 5001
	ErrCodeRecoveryError    ErrorCode = 5002
	ErrCodeLoggingError     ErrorCode = 5003
	ErrCodeTracingError     ErrorCode = 5004
	ErrCodeMetricsError     ErrorCode = 5005
	ErrCodeSecurityError    ErrorCode = 5006
	ErrCodeSignatureInvalid ErrorCode = 5007

	// gRPC相关错误 (6000-6999)
	ErrCodeGRPCConnectionFailed ErrorCode = 6001
	ErrCodeGRPCServiceNotFound  ErrorCode = 6002
	ErrCodeGRPCMethodNotFound   ErrorCode = 6003
	ErrCodeGRPCTimeout          ErrorCode = 6004
	ErrCodeGRPCCanceled         ErrorCode = 6005

	// 健康检查错误 (7000-7999)
	ErrCodeHealthCheckFailed      ErrorCode = 7001
	ErrCodeHealthCheckTimeout     ErrorCode = 7002
	ErrCodeHealthCheckUnavailable ErrorCode = 7003

	// Swagger文档错误 (8000-8999)
	ErrCodeSwaggerNotFound     ErrorCode = 8001
	ErrCodeSwaggerLoadFailed   ErrorCode = 8002
	ErrCodeSwaggerRenderFailed ErrorCode = 8003

	// JWT和认证扩展错误 (2100-2199)
	ErrCodeTokenMalformed        ErrorCode = 2101
	ErrCodeTokenNotValidYet      ErrorCode = 2102
	ErrCodeAccountLoginElsewhere ErrorCode = 2103
	ErrCodeRedisParseError       ErrorCode = 2104
	ErrCodeDBQueryError          ErrorCode = 2105
	ErrCodeClaimsParseFailed     ErrorCode = 2106

	// 数据转换和验证错误 (3100-3199)
	ErrCodePBMessageNil         ErrorCode = 3101
	ErrCodeModelMessageNil      ErrorCode = 3102
	ErrCodeFieldConversionError ErrorCode = 3103
	ErrCodeTypeConversionError  ErrorCode = 3104
	ErrCodeInitializationError  ErrorCode = 3105
	ErrCodeInvalidFieldMapping  ErrorCode = 3106
	ErrCodeUserIDMissing        ErrorCode = 3107
	ErrCodeMustBePointer        ErrorCode = 3108
	ErrCodeMustBeSlice          ErrorCode = 3109
	ErrCodeMustBeStruct         ErrorCode = 3110
	ErrCodeElementConversion    ErrorCode = 3111
	ErrCodeItemNil              ErrorCode = 3112

	// 中间件和国际化错误 (5100-5199)
	ErrCodeLanguageLoadFailed ErrorCode = 5101
	ErrCodeLanguageNotFound   ErrorCode = 5102
	ErrCodeJSONParseFailed    ErrorCode = 5103

	// 配置和特性错误 (1100-1199)
	ErrCodeInvalidConfigType     ErrorCode = 1101
	ErrCodeFeatureNotRegistered  ErrorCode = 1102
	ErrCodeFeatureEnableFailed   ErrorCode = 1103
	ErrCodeMiddlewareInitFailed  ErrorCode = 1104
	ErrCodeHealthManagerFailed   ErrorCode = 1105
	ErrCodeGRPCServerInitFailed  ErrorCode = 1106
	ErrCodeHTTPGatewayInitFailed ErrorCode = 1107
	ErrCodeWSCNotEnabled         ErrorCode = 1108
	ErrCodeWSCRouteFailed        ErrorCode = 1109
	ErrCodeUserAuthNotFound      ErrorCode = 1110

	// 服务器和基础设施错误 (1200-1299)
	ErrCodeServerCreationFailed ErrorCode = 1201
	ErrCodeScanTypeMismatch     ErrorCode = 1202

	// 通用错误 (9000-9999)
	ErrCodeUnknown          ErrorCode = 9000
	ErrCodeInternal         ErrorCode = 9001
	ErrCodeOperationFailed  ErrorCode = 9002
	ErrCodeResourceNotFound ErrorCode = 9003
	ErrCodeConflict         ErrorCode = 9004
)
