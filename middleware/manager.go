/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 17:55:10
 * @FilePath: \go-rpc-gateway\middleware\manager.go
 * @Description: 中间件管理器
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-config/pkg/ratelimit"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	swaggerMiddleware "github.com/kamalyes/go-swagger"
	"google.golang.org/grpc"
)

// Manager 中间件管理器 - 使用 go-config 的 middleware 配置
type Manager struct {
	cfg                     *gwconfig.Gateway
	metricsManager          *MetricsManager
	tracingManager          *TracingManager
	rateLimiter             RateLimiter
	dynamicRateLimit        DynamicRateLimitProvider
	dynamicSignature        DynamicSignatureProvider
	i18nManager             *I18nManager
	swaggerMiddleware       *swaggerMiddleware.Middleware
	breakerManager          *BreakerManager  // 熔断器管理器（用于指标采集）
	shedderManager          *ShedderManager  // 自适应降载管理器（用于指标采集）
	preRateLimitMiddlewares []HTTPMiddleware // 在限流之前执行的自定义中间件（如认证）
}

// NewManager 创建中间件管理器 - 使用全局 GATEWAY 配置
func NewManager(cfg *gwconfig.Gateway) (*Manager, error) {
	var err error
	manager := &Manager{
		cfg: cfg,
	}

	// 初始化监控管理器（使用 monitoring 配置）
	if cfg.Monitoring.Metrics.Enabled {
		manager.metricsManager = NewMetricsManager(cfg.Monitoring)
	}

	// 初始化链路追踪管理器
	if cfg.Middleware.Tracing.Enabled {
		manager.tracingManager, err = NewTracingManager(cfg.Middleware.Tracing)
		if err != nil {
			return nil, errors.NewErrorf(errors.ErrCodeTracingError, "failed to init tracing manager: %v", err)
		}
	}

	// 初始化i18n管理器
	if cfg.Middleware.I18N.Enabled {
		manager.i18nManager, err = NewI18nManager(cfg.Middleware.I18N)
		if err != nil {
			return nil, errors.NewErrorf(errors.ErrCodeMiddlewareError, "failed to init i18n manager: %v", err)
		}
	}

	// 初始化 Swagger 中间件
	if cfg.Swagger.Enabled {
		manager.swaggerMiddleware = swaggerMiddleware.NewMiddleware(cfg.Swagger,
			swaggerMiddleware.WithLogger(global.LOGGER),
		)
		if err != nil {
			return nil, errors.NewErrorf(errors.ErrCodeMiddlewareError, "failed to init swagger middleware: %v", err)
		}
		global.LOGGER.Info("Swagger文档中间件已初始化 [ui_path=%s, enabled=%v]",
			cfg.Swagger.UIPath, true)
	}

	// 初始化限流器（如果启用）
	if cfg.RateLimit.Enabled {
		// 根据策略选择限流器实现
		switch cfg.RateLimit.Strategy {
		case ratelimit.StrategyTokenBucket:
			manager.rateLimiter = NewTokenBucketLimiter(cfg.RateLimit)
		case ratelimit.StrategySlidingWindow:
			if global.REDIS != nil {
				manager.rateLimiter = NewSlidingWindowLimiter(cfg.RateLimit)
			} else {
				manager.rateLimiter = NewTokenBucketLimiter(cfg.RateLimit) // 降级到令牌桶
				global.LOGGER.Warn("Redis不可用，限流器降级为令牌桶模式")
			}
		case ratelimit.StrategyFixedWindow:
			manager.rateLimiter = NewFixedWindowLimiter(cfg.RateLimit)
		default:
			manager.rateLimiter = NewTokenBucketLimiter(cfg.RateLimit)
		}

		var rps, burst int
		if cfg.RateLimit.GlobalLimit != nil {
			rps = cfg.RateLimit.GlobalLimit.RequestsPerSecond
			burst = cfg.RateLimit.GlobalLimit.BurstSize
		}
		global.LOGGER.Info("限流器已初始化 [strategy=%s, rps=%d, burst=%d, enabled=%v]",
			cfg.RateLimit.Strategy, rps, burst, true)
	}

	// 初始化熔断器管理器（直接传入 go-config 的 CircuitBreaker 配置）
	if cfg.Middleware.CircuitBreaker.Enabled {
		manager.breakerManager = NewBreakerManager(cfg.Middleware.CircuitBreaker)
		global.LOGGER.Info("熔断器管理器已初始化 [failure_threshold=%d, timeout=%d, enabled=%v]",
			cfg.Middleware.CircuitBreaker.FailureThreshold, cfg.Middleware.CircuitBreaker.Timeout, true)
	}

	// 初始化自适应降载管理器（直接传入 go-config 的 Shedder 配置，与熔断同一套写法）
	// 配置缺省（没有 middleware.shedder 子树且使用方自建了 Middleware 配置）时视为未启用
	if shedderCfg := cfg.Middleware.Shedder; shedderCfg != nil && shedderCfg.Enabled {
		manager.shedderManager = NewShedderManager(shedderCfg)
		effective := manager.shedderManager.Config()
		global.LOGGER.Info("自适应降载管理器已初始化 [cpu_threshold=%d, max_concurrency=%d, window_ms=%d, buckets=%d, cooldown_ms=%d, cpu_source=%s, enabled=%v]",
			effective.CPUThreshold, effective.MaxConcurrency, effective.WindowMS,
			effective.WindowBuckets, effective.CooldownMS, CPUUsageSource(), true)
	}

	return manager, nil
}

// UpdateConfig 更新中间件管理器配置
// 保留动态提供程序（如限流、签名等）
func (m *Manager) UpdateConfig(cfg *gwconfig.Gateway) error {
	if m == nil {
		return nil
	}

	dynamicRateLimit := m.dynamicRateLimit
	dynamicSignature := m.dynamicSignature

	if m.swaggerMiddleware != nil && cfg != nil && cfg.Swagger != nil {
		if err := m.swaggerMiddleware.UpdateConfig(cfg.Swagger); err != nil {
			return err
		}
	}

	next, err := NewManager(cfg)
	if err != nil {
		return err
	}

	next.dynamicRateLimit = dynamicRateLimit
	next.dynamicSignature = dynamicSignature
	*m = *next
	return nil
}

// HTTPMetricsMiddleware HTTP 监控中间件
func (m *Manager) HTTPMetricsMiddleware() MiddlewareFunc {
	return HTTPMetricsMiddleware(m.metricsManager)
}

// HTTPTracingMiddleware HTTP 链路追踪中间件
func (m *Manager) HTTPTracingMiddleware() MiddlewareFunc {
	return Tracing(m.tracingManager)
}

// GRPCMetricsInterceptor gRPC 监控拦截器
func (m *Manager) GRPCMetricsInterceptor() GRPCInterceptor {
	return GRPCMetricsInterceptor(m.metricsManager)
}

// GRPCTracingInterceptor gRPC 链路追踪拦截器
func (m *Manager) GRPCTracingInterceptor() GRPCInterceptor {
	return GRPCTracingInterceptor(m.tracingManager)
}

// GRPCTracingStreamInterceptor gRPC 流式链路追踪拦截器
func (m *Manager) GRPCTracingStreamInterceptor() grpc.StreamServerInterceptor {
	return GRPCTracingStreamInterceptor(m.tracingManager)
}

// GetTracingManager 获取链路追踪管理器（供 WSC 等非中间件链组件使用）
func (m *Manager) GetTracingManager() *TracingManager {
	return m.tracingManager
}

// GRPCStructTagValidatorInterceptor gRPC struct tag 参数校验拦截器
// 配合 protoc-go-inject-tag 在 pb 字段上注入的 `validate:"..."` 标签生效。
func (m *Manager) GRPCStructTagValidatorInterceptor() GRPCInterceptor {
	return StructTagValidatorUnaryInterceptor()
}

// GRPCStructTagValidatorStreamInterceptor gRPC 流式 struct tag 参数校验拦截器
func (m *Manager) GRPCStructTagValidatorStreamInterceptor() grpc.StreamServerInterceptor {
	return StructTagValidatorStreamInterceptor()
}

// GRPCGatewayStructTagValidatorMiddleware grpc-gateway HTTP 层 struct tag 参数校验中间件
// 用于本地 Handler 模式（RegisterXxxHandlerServer），HTTP 请求绕过 gRPC 拦截器链，
// 需要通过此中间件在 gateway 层拦截校验
func (m *Manager) GRPCGatewayStructTagValidatorMiddleware() runtime.Middleware {
	return StructTagValidatorGatewayMiddleware()
}

// CORSMiddleware CORS 中间件
func (m *Manager) CORSMiddleware() MiddlewareFunc {
	return MiddlewareFunc(CORSMiddleware(m.cfg.CORS))
}

// RecoveryMiddleware 恢复中间件
func (m *Manager) RecoveryMiddleware() MiddlewareFunc {
	return MiddlewareFunc(RecoveryMiddleware(m.cfg.Middleware.Recovery))
}

// RequestContextMiddlewareFunc 统一的请求上下文中间件
// 负责 trace_id、request_id 等的注入，使用 go-logger 的统一管理
func (m *Manager) RequestContextMiddlewareFunc() MiddlewareFunc {
	return MiddlewareFunc(RequestContextMiddleware())
}

// SCPMiddleware 安全中间件 - 从配置读取 CSP 策略
func (m *Manager) SCPMiddleware() MiddlewareFunc {
	return MiddlewareFunc(SCPMiddleware(m.cfg.Security.CSP))
}

// RateLimitMiddleware 限流中间件
func (m *Manager) RateLimitMiddleware() MiddlewareFunc {
	return MiddlewareFunc(newRateLimitMiddleware(m.cfg.RateLimit, m.rateLimiter, m.dynamicRateLimit, m.metricsManager).Middleware())
}

// AddPreRateLimitMiddleware 添加在限流之前执行的自定义中间件（如认证）。
// 认证中间件在此处执行后，UserID 会注入 context，限流中间件即可正确读取。
func (m *Manager) AddPreRateLimitMiddleware(mw ...HTTPMiddleware) {
	m.preRateLimitMiddlewares = append(m.preRateLimitMiddlewares, mw...)
}

// GRPCRateLimitUnaryInterceptor gRPC 一元限流拦截器（未启用限流时返回 nil）
func (m *Manager) GRPCRateLimitUnaryInterceptor() grpc.UnaryServerInterceptor {
	if !m.cfg.RateLimit.Enabled {
		return nil
	}
	return GRPCRateLimitUnaryInterceptor(m.cfg.RateLimit, m.rateLimiter)
}

// GRPCRateLimitStreamInterceptor gRPC 流式限流拦截器（未启用限流时返回 nil）
func (m *Manager) GRPCRateLimitStreamInterceptor() grpc.StreamServerInterceptor {
	if !m.cfg.RateLimit.Enabled {
		return nil
	}
	return GRPCRateLimitStreamInterceptor(m.cfg.RateLimit, m.rateLimiter)
}

// LoggingMiddleware HTTP日志中间件
func (m *Manager) LoggingMiddleware() MiddlewareFunc {
	return MiddlewareFunc(LoggingMiddleware())
}

// SignatureMiddleware 签名验证中间件
func (m *Manager) SignatureMiddleware() MiddlewareFunc {
	return MiddlewareFunc(SignatureMiddlewareWithProvider(m.cfg.Middleware.Signature, m.dynamicSignature))
}

// SetDynamicSignatureProvider 设置动态签名提供器
func (m *Manager) SetDynamicSignatureProvider(provider DynamicSignatureProvider) {
	m.dynamicSignature = provider
}

// SetDynamicRateLimitProvider 设置动态限流提供器
func (m *Manager) SetDynamicRateLimitProvider(provider DynamicRateLimitProvider) {
	m.dynamicRateLimit = provider
}

// TimestampMiddleware 时间戳验证中间件
func (m *Manager) TimestampMiddleware() MiddlewareFunc {
	return MiddlewareFunc(TimestampMiddleware(m.cfg.Middleware.Signature))
}

// NonceMiddleware 防重放验证中间件
func (m *Manager) NonceMiddleware() MiddlewareFunc {
	return MiddlewareFunc(NonceMiddleware(m.cfg.Middleware.Signature))
}

// I18nMiddleware 国际化中间件
// 由配置驱动：启用时使用 NewManager 中创建的 i18nManager，未启用时直通
func (m *Manager) I18nMiddleware() MiddlewareFunc {
	if !m.cfg.Middleware.I18N.Enabled {
		return MiddlewareFunc(noopMiddleware)
	}
	return I18nWithManager(m.i18nManager)
}

// GRPCUnaryI18nInterceptor 返回 gRPC 服务端 i18n 一元调用拦截器（未启用时返回 nil）
func (m *Manager) GRPCUnaryI18nInterceptor() grpc.UnaryServerInterceptor {
	if !m.cfg.Middleware.I18N.Enabled {
		return nil
	}
	return UnaryServerI18nInterceptor(m.i18nManager)
}

// GRPCStreamI18nInterceptor 返回 gRPC 服务端 i18n 流式调用拦截器（未启用时返回 nil）
func (m *Manager) GRPCStreamI18nInterceptor() grpc.StreamServerInterceptor {
	if !m.cfg.Middleware.I18N.Enabled {
		return nil
	}
	return StreamServerI18nInterceptor(m.i18nManager)
}

// BreakerMiddleware 熔断中间件
// 由配置驱动：启用时使用 NewManager 中创建的共享 breakerManager（便于指标采集），未启用时直通
func (m *Manager) BreakerMiddleware() MiddlewareFunc {
	if !m.cfg.Middleware.CircuitBreaker.Enabled {
		return MiddlewareFunc(noopMiddleware)
	}
	return MiddlewareFunc(BreakerHTTPMiddleware(m.breakerManager))
}

// GRPCBreakerUnaryInterceptor gRPC 一元熔断拦截器（未启用熔断时返回 nil）
// 与 HTTP 熔断中间件共用 NewManager 中创建的同一个 breakerManager，
// 因此同一方法在 HTTP 与 gRPC 两侧共享状态与统计，指标采集也只有一份数据源
//
// [EN] Unary breaker interceptor, nil when the breaker is disabled. It shares the single
// breakerManager created in NewManager with the HTTP middleware, so state, statistics and
// metrics collection all come from one source
func (m *Manager) GRPCBreakerUnaryInterceptor() grpc.UnaryServerInterceptor {
	if !m.cfg.Middleware.CircuitBreaker.Enabled || m.breakerManager == nil {
		return nil
	}
	return UnaryServerBreakerInterceptor(m.breakerManager)
}

// GRPCBreakerStreamInterceptor gRPC 流式熔断拦截器（未启用熔断时返回 nil）
//
// [EN] Stream breaker interceptor, nil when the breaker is disabled
func (m *Manager) GRPCBreakerStreamInterceptor() grpc.StreamServerInterceptor {
	if !m.cfg.Middleware.CircuitBreaker.Enabled || m.breakerManager == nil {
		return nil
	}
	return StreamServerBreakerInterceptor(m.breakerManager)
}

// GetBreakerManager 获取熔断器管理器（供 server 包注入指标采集函数）
func (m *Manager) GetBreakerManager() *BreakerManager {
	return m.breakerManager
}

// ShedderMiddleware 自适应降载中间件
// 由配置驱动：启用时使用 NewManager 中创建的共享 shedderManager（便于指标采集），未启用时直通
//
// [EN] Adaptive load-shedding middleware, sharing the shedderManager created in NewManager
// so that metrics come from one source; a pass-through when shedding is disabled
func (m *Manager) ShedderMiddleware() MiddlewareFunc {
	if m.shedderManager == nil {
		return MiddlewareFunc(noopMiddleware)
	}
	return MiddlewareFunc(ShedderHTTPMiddleware(m.shedderManager))
}

// GRPCShedderUnaryInterceptor gRPC 一元降载拦截器（未启用降载时返回 nil）
// 与 HTTP 降载中间件共用 NewManager 中创建的同一个 shedderManager，
// 因此 HTTP 与 gRPC 两侧的飞行请求数、响应时间滑窗合并统计——降载判定的是整个进程的负载
//
// [EN] Unary shedding interceptor, nil when shedding is disabled. It shares the single
// shedderManager created in NewManager with the HTTP middleware, so in-flight counts and the
// response-time window are pooled across both transports: shedding judges whole-process load
func (m *Manager) GRPCShedderUnaryInterceptor() grpc.UnaryServerInterceptor {
	if m.shedderManager == nil {
		return nil
	}
	return UnaryServerShedderInterceptor(m.shedderManager)
}

// GRPCShedderStreamInterceptor gRPC 流式降载拦截器（未启用降载时返回 nil）
//
// [EN] Stream shedding interceptor, nil when shedding is disabled
func (m *Manager) GRPCShedderStreamInterceptor() grpc.StreamServerInterceptor {
	if m.shedderManager == nil {
		return nil
	}
	return StreamServerShedderInterceptor(m.shedderManager)
}

// GetShedderManager 获取降载管理器（供 server 包注册指标采集器）
//
// [EN] Returns the shedding manager, for the server package to register its collector
func (m *Manager) GetShedderManager() *ShedderManager {
	return m.shedderManager
}

// MetricsHandler 返回监控指标处理器（使用 MetricsManager 的独立 registry）
func (m *Manager) MetricsHandler() http.Handler {
	if m.metricsManager == nil {
		return http.NotFoundHandler()
	}
	return m.metricsManager.Handler()
}

// GetMetricsManager 获取监控管理器（供 server 包注入扩展指标采集函数）
func (m *Manager) GetMetricsManager() *MetricsManager {
	return m.metricsManager
}

// SwaggerHandler 返回 Swagger 文档处理器
func (m *Manager) SwaggerHandler() http.Handler {
	if m.swaggerMiddleware == nil {
		return http.NotFoundHandler()
	}
	return m.swaggerMiddleware
}

// GetSwaggerPaths 获取 Swagger 路由路径
func (m *Manager) GetSwaggerPaths() []string {
	if m.swaggerMiddleware == nil || !m.cfg.Swagger.Enabled {
		return nil
	}
	return m.swaggerMiddleware.GetSwaggerPaths()
}

// GetMiddlewares 获取中间件链（完全基于配置驱动）
func (m *Manager) GetMiddlewares() []MiddlewareFunc {
	var middlewares []MiddlewareFunc

	// 1. Recovery 中间件（始终启用，最先执行）
	middlewares = append(middlewares, m.RecoveryMiddleware())

	// 2. 链路追踪中间件（必须在 RequestContext 之前）
	// 原因：先创建 OTel span，后续 RequestContextMiddleware 的 extractOrGenerateTraceID
	// 才能从 span context 中提取 OTel trace_id，确保日志、gRPC metadata、OTel 后端
	// 使用同一个 trace_id，实现全链路打通；若顺序颠倒，Logging 等中间件会记录
	// RequestContext 自行生成的随机 trace_id，与 OTel trace_id 不一致
	if m.cfg.Middleware.Tracing.Enabled && m.tracingManager != nil {
		middlewares = append(middlewares, m.HTTPTracingMiddleware())
	}

	// 3. Context 追踪中间件（始终启用）
	middlewares = append(middlewares, m.RequestContextMiddlewareFunc())

	// 4. 日志中间件（根据配置）
	if m.cfg.Middleware.Logging.Enabled {
		middlewares = append(middlewares, m.LoggingMiddleware())
	}

	// 5. 国际化中间件（根据配置）
	if m.cfg.Middleware.I18N.Enabled {
		middlewares = append(middlewares, m.I18nMiddleware())
	}

	// 6. 监控中间件（根据配置）
	if m.cfg.Monitoring.Metrics.Enabled && m.metricsManager != nil {
		middlewares = append(middlewares, m.HTTPMetricsMiddleware())
	}

	// 6.5. 限流前自定义中间件（如认证，需要在限流之前注入 UserID）
	for _, mw := range m.preRateLimitMiddlewares {
		middlewares = append(middlewares, MiddlewareFunc(mw))
	}

	// 7. 限流中间件（根据配置）
	if m.cfg.RateLimit.Enabled && m.rateLimiter != nil {
		middlewares = append(middlewares, m.RateLimitMiddleware())
	}

	// 7.5. 自适应降载中间件（根据配置，夹在限流与熔断之间）
	// 位置理由（与 server/grpc.go 的 gRPC 链完全一致，详见 docs/SHEDDER.md）：
	//  1. 必须在限流之后——限流是确定性的配额判定，被限流拒绝的请求根本不会触达 handler，
	//     放在降载之前可以让它们既不占用飞行计数、也不把「拒绝得极快」的响应时间
	//     掺进滑窗，否则最小平均 RT 会被压低，承载上限被低估，反过来加剧降载
	//  2. 必须在熔断之前（即熔断在降载内层）——降载拒绝返回 503/codes.Unavailable，
	//     而 Unavailable 正是熔断计入失败的码；若降载在熔断内层，熔断器会把降载的
	//     自我保护拒绝当成下游故障，形成「过载→降载拒绝→熔断打开」的正反馈，
	//     哪怕负载已经退潮也要等熔断超时才能恢复 顺序反过来后熔断器永远看不到降载拒绝
	//  3. 两者语义本就不同：降载回答「本进程现在还扛得住吗」，熔断回答「下游还活着吗」，
	//     前者是全局自保、应当更早拒绝，后者需要贴近 handler 才能观测到真实健康度
	if m.shedderManager != nil {
		middlewares = append(middlewares, m.ShedderMiddleware())
	}

	// 8. 熔断中间件（根据配置）
	if m.cfg.Middleware.CircuitBreaker.Enabled {
		middlewares = append(middlewares, m.BreakerMiddleware())
	}

	// 9. 安全中间件（根据配置）
	if m.cfg.Security.CSP.Enabled {
		middlewares = append(middlewares, m.SCPMiddleware())
	}

	// 10. CORS 中间件（根据配置）
	if m.cfg.CORS.Enabled {
		middlewares = append(middlewares, m.CORSMiddleware())
	}

	// 11. 签名验证中间件
	if m.cfg.Middleware.Signature.Enabled {
		middlewares = append(middlewares, m.TimestampMiddleware())
		middlewares = append(middlewares, m.NonceMiddleware())
		middlewares = append(middlewares, m.SignatureMiddleware())
	}

	return middlewares
}

// HTTPMiddleware 应用HTTP中间件链
func (m *Manager) HTTPMiddleware(handler http.Handler) http.Handler {
	middlewares := m.GetMiddlewares()
	return ApplyMiddlewares(handler, middlewares...)
}

// UnaryServerInterceptor 返回gRPC一元拦截器
func (m *Manager) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return UnaryServerLoggingInterceptor()
}

// StreamServerInterceptor 返回gRPC流拦截器
func (m *Manager) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return StreamServerLoggingInterceptor()
}

// ApplyMiddlewares 应用中间件链到处理器
func ApplyMiddlewares(handler http.Handler, middlewares ...MiddlewareFunc) http.Handler {
	// 倒序应用中间件
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] == nil {
			continue
		}
		handler = middlewares[i](handler)
	}
	return handler
}
