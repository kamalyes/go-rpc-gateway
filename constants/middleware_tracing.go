/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:40:10
 * @FilePath: \go-rpc-gateway\constants\middleware_tracing.go
 * @Description: 链路追踪中间件相关常量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

// 链路追踪默认配置
const (
	// 默认是否启用
	TracingDefaultEnabled = false

	// 默认服务名
	TracingDefaultServiceName = "go-rpc-gateway"

	// 默认服务版本
	TracingDefaultServiceVersion = "1.0.0"

	// 默认环境
	TracingDefaultEnvironment = "development"
)

// 导出器类型常量
const (
	TracingExporterJaeger  = "jaeger"
	TracingExporterZipkin  = "zipkin"
	TracingExporterOTLP    = "otlp"
	TracingExporterConsole = "console"
	TracingExporterNoop    = "noop"
)

// 采样器类型常量
const (
	TracingSamplerAlways       = "always"
	TracingSamplerNever        = "never"
	TracingSamplerProbability  = "probability"
	TracingSamplerRateLimiting = "rate_limiting"
	TracingSamplerParentBased  = "parent_based"
)

// 默认采样配置
const (
	// 默认采样类型
	TracingDefaultSamplerType = TracingSamplerProbability

	// 默认采样概率
	TracingDefaultSamplerProbability = 0.1

	// 默认采样率限制
	TracingDefaultSamplerRate = 100
)

// 默认导出器配置
const (
	// 默认导出器类型
	TracingDefaultExporterType = TracingExporterConsole

	// 默认 Jaeger 端点
	TracingDefaultJaegerEndpoint = "http://localhost:14268/api/traces"

	// 默认 Zipkin 端点
	TracingDefaultZipkinEndpoint = "http://localhost:9411/api/v2/spans"

	// 默认 OTLP 端点
	TracingDefaultOTLPEndpoint = "localhost:4317"

	// 默认 OTLP 是否不安全
	TracingDefaultOTLPInsecure = true
)

// Span 属性常量
const (
	// HTTP 相关属性
	TracingAttrHTTPMethod     = "http.method"
	TracingAttrHTTPURL        = "http.url"
	TracingAttrHTTPPath       = "http.target"
	TracingAttrHTTPStatusCode = "http.status_code"
	TracingAttrHTTPUserAgent  = "http.user_agent"
	TracingAttrHTTPScheme     = "http.scheme"
	TracingAttrHTTPHost       = "http.host"

	// gRPC 相关属性
	TracingAttrRPCSystem  = "rpc.system"
	TracingAttrRPCService = "rpc.service"
	TracingAttrRPCMethod  = "rpc.method"

	// 网络相关属性
	TracingAttrNetPeerIP   = "net.peer.ip"
	TracingAttrNetPeerPort = "net.peer.port"

	// 用户相关属性
	TracingAttrUserID    = "user.id"
	TracingAttrUserAgent = "user.agent"

	// 自定义属性
	TracingAttrRequestID = "request.id"
	TracingAttrSessionID = "session.id"
	TracingAttrTenantID  = "tenant.id"
)

// Span 事件常量
const (
	TracingEventRequestStart = "request.start"
	TracingEventRequestEnd   = "request.end"
	TracingEventError        = "error"
	TracingEventException    = "exception"
)

// 错误类型常量
const (
	TracingErrorTypeHTTP     = "http_error"
	TracingErrorTypeGRPC     = "grpc_error"
	TracingErrorTypeDatabase = "database_error"
	TracingErrorTypeTimeout  = "timeout_error"
	TracingErrorTypeAuth     = "auth_error"
)

// 默认跳过追踪的路径
var TracingDefaultSkipPaths = []string{
	DefaultHealthPath,
	DefaultMetricsPath,
	DefaultDebugPath,
	PProfBasePath,
}

// 默认资源属性
var TracingDefaultResourceAttributes = map[string]string{
	"service.name":           TracingDefaultServiceName,
	"service.version":        TracingDefaultServiceVersion,
	"deployment.environment": TracingDefaultEnvironment,
}
