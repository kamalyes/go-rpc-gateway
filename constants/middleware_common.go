/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:40:10
 * @FilePath: \go-rpc-gateway\constants\middleware_common.go
 * @Description: 中间件通用常量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

import "time"

// 中间件通用配置
const (
	// 默认中间件启用状态
	MiddlewareDefaultEnabled = true

	// 默认中间件禁用状态
	MiddlewareDefaultDisabled = false

	// 默认超时时间
	MiddlewareDefaultTimeout = 30 * time.Second

	// 默认重试次数
	MiddlewareDefaultRetryCount = 3

	// 默认重试间隔
	MiddlewareDefaultRetryInterval = time.Second
)

// 中间件类型常量
const (
	MiddlewareTypeLogging   = "logging"
	MiddlewareTypeSecurity  = "security"
	MiddlewareTypeRateLimit = "rate_limit"
	MiddlewareTypeI18n      = "i18n"
	MiddlewareTypeRequestID = "request_id"
	MiddlewareTypeSignature = "signature"
	MiddlewareTypeRecovery  = "recovery"
	MiddlewareTypePProf     = "pprof"
	MiddlewareTypeMetrics   = "metrics"
	MiddlewareTypeTracing   = "tracing"
	MiddlewareTypeHealth    = "health"
	MiddlewareTypeCORS      = "cors"
	MiddlewareTypeAuth      = "auth"
)

// 中间件执行顺序常量
const (
	// 基础中间件 (优先级 1-100)
	MiddlewareOrderLogging   = 10
	MiddlewareOrderRecovery  = 20
	MiddlewareOrderRequestID = 30
	MiddlewareOrderCORS      = 40

	// 安全中间件 (优先级 101-200)
	MiddlewareOrderSecurity  = 110
	MiddlewareOrderAuth      = 120
	MiddlewareOrderSignature = 130

	// 限流中间件 (优先级 201-300)
	MiddlewareOrderRateLimit = 210

	// 功能中间件 (优先级 301-400)
	MiddlewareOrderI18n    = 310
	MiddlewareOrderMetrics = 320
	MiddlewareOrderTracing = 330
	MiddlewareOrderPProf   = 340
	MiddlewareOrderHealth  = 350
)

// 请求上下文键常量
const (
	ContextKeyRequestID  = "request_id"
	ContextKeyUserID     = "user_id"
	ContextKeyTenantID   = "tenant_id"
	ContextKeySessionID  = "session_id"
	ContextKeyLang       = "lang"
	ContextKeyTraceID    = "trace_id"
	ContextKeySpanID     = "span_id"
	ContextKeyStartTime  = "start_time"
	ContextKeyRemoteAddr = "remote_addr"
	ContextKeyUserAgent  = "user_agent"
)

// 错误代码常量
const (
	ErrCodeMiddlewareInit     = "MIDDLEWARE_INIT_ERROR"
	ErrCodeMiddlewareConfig   = "MIDDLEWARE_CONFIG_ERROR"
	ErrCodeMiddlewareExec     = "MIDDLEWARE_EXEC_ERROR"
	ErrCodeMiddlewareTimeout  = "MIDDLEWARE_TIMEOUT_ERROR"
	ErrCodeMiddlewareDisabled = "MIDDLEWARE_DISABLED_ERROR"
)

// 错误消息常量
const (
	ErrMsgMiddlewareInit     = "Failed to initialize middleware"
	ErrMsgMiddlewareConfig   = "Invalid middleware configuration"
	ErrMsgMiddlewareExec     = "Failed to execute middleware"
	ErrMsgMiddlewareTimeout  = "Middleware execution timeout"
	ErrMsgMiddlewareDisabled = "Middleware is disabled"
)

// 默认跳过的路径
var MiddlewareDefaultSkipPaths = []string{
	"/favicon.ico",
	"/robots.txt",
	"/sitemap.xml",
	"/_internal",
	"/__internal",
}

// 默认跳过的用户代理
var MiddlewareDefaultSkipUserAgents = []string{
	"healthcheck",
	"probe",
	"monitor",
	"nagios",
	"zabbix",
}

// 默认跳过的方法
var MiddlewareDefaultSkipMethods = []string{
	"OPTIONS",
}

// 中间件配置验证规则
var MiddlewareValidationRules = map[string]interface{}{
	"timeout":     "required,min=1s,max=300s",
	"retry_count": "required,min=0,max=10",
	"enabled":     "required,boolean",
	"skip_paths":  "slice",
	"order":       "required,min=1,max=1000",
}
