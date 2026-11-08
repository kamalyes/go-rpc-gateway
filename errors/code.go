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
	// ErrCodeServiceOverloaded 自适应降载拒绝：服务端 CPU/并发已过载，主动丢弃请求
	// 与 ErrCodeServiceDegraded(4004) 区分开：后者表示「功能降级后仍在服务」，
	// 前者表示「这一次请求根本没有被处理」，调用方应带退避重试而不是降级消费结果
	// [EN] Rejected by adaptive load shedding. Kept distinct from ErrCodeServiceDegraded,
	// which means "still serving, with reduced functionality", whereas this means the
	// request was not processed at all and the caller should retry with backoff
	ErrCodeServiceOverloaded ErrorCode = 4005

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

	// JWT 错误 (2100-2199)
	//
	// 只保留 JWT 本身的状态。原先这里还有「账号已在其他地方登录」「解析 Redis 中的用户
	// token 出错」「从数据库获取用户 token 异常」这类码 —— 那是具体业务的事，
	// 连文案都是产品文案，不该由一个网关框架定义
	// [EN] Only JWT's own states. This range also held codes for "account logged in
	// elsewhere" and for parsing a user token out of Redis or a database — those are one
	// application's concerns, down to the product copy, not a gateway framework's
	ErrCodeTokenMalformed   ErrorCode = 2101
	ErrCodeTokenNotValidYet ErrorCode = 2102

	// 初始化错误 (3100-3199)
	//
	// 这一段原先叫「数据转换和验证错误」，放着 PB/Model 互转、反射类型断言失败这类码。
	// 那些是转换库的职责（pbmo 自己就有 NewConversionError / NewNilValueError /
	// NewTypeMismatchError），网关一个都没用过，下游也没有，所以删掉，只留初始化这一个
	// [EN] This range used to be "data conversion and validation", holding PB/Model
	// conversion and reflection type-assertion codes. Those belong to the conversion
	// library — pbmo has NewConversionError, NewNilValueError and NewTypeMismatchError of
	// its own — and the gateway never used a single one, nor did anything downstream
	ErrCodeInitializationError ErrorCode = 3105

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

	// 服务器和基础设施错误 (1200-1299)
	ErrCodeServerCreationFailed ErrorCode = 1201

	// 通用错误 (9000-9999)
	ErrCodeUnknown          ErrorCode = 9000
	ErrCodeInternal         ErrorCode = 9001
	ErrCodeOperationFailed  ErrorCode = 9002
	ErrCodeResourceNotFound ErrorCode = 9003
	ErrCodeConflict         ErrorCode = 9004
)
