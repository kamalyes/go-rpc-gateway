/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 00:01:16
 * @FilePath: \go-rpc-gateway\middleware\manager.go
 * @Description: 中间件管理器
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"net/http"

	goconfig "github.com/kamalyes/go-config"
	breakercof "github.com/kamalyes/go-config/pkg/breaker"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-config/pkg/middleware"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

// Manager 中间件管理器 - 使用 go-config 的 middleware 配置
type Manager struct {
	// 监控管理器
	metricsManager *MetricsManager
	tracingManager *TracingManager
	// 统一配置 - 使用 go-config 的 Gateway
	cfg *gwconfig.Gateway
	// 功能组件
	rateLimiter            RateLimiter
	accessRecordHandler    AccessRecordHandler
	signatureValidator     SignatureValidator
	pprofScenarios         *PProfScenarios
	pprofAdapter           *PProfConfigAdapter
	i18nManager            *I18nManager
	breakerAdapter         *BreakerMiddlewareAdapter
	pbValidationMiddleware *PBValidationMiddleware
}

// NewManager 创建中间件管理器 - 使用全局 GATEWAY 配置
func NewManager() (*Manager, error) {
	var err error

	// 使用全局配置
	cfg := global.GATEWAY
	if cfg == nil {
		return nil, errors.NewError(errors.ErrCodeInvalidConfiguration, "global GATEWAY config is not initialized")
	}

	// 确保 Middleware 配置存在
	if cfg.Middleware == nil {
		cfg.Middleware = middleware.Default()
	}

	manager := &Manager{
		cfg:                 cfg,
		accessRecordHandler: &LogAccessRecordHandler{},
		signatureValidator:  &HMACValidator{},
		pprofScenarios:      NewPProfScenarios(),
	}

	// 创建pprof适配器
	if cfg.Middleware.PProf != nil && cfg.Middleware.PProf.Enabled {
		manager.pprofAdapter = NewPProfConfigAdapter(cfg.Middleware.PProf)
	}

	// 初始化监控管理器
	if cfg.Middleware.Metrics != nil && cfg.Middleware.Metrics.Enabled {
		manager.metricsManager, err = NewMetricsManager(cfg.Middleware.Metrics)
		if err != nil {
			return nil, errors.NewErrorf(errors.ErrCodeMetricsError, "failed to init metrics manager: %v", err)
		}
	}

	// 初始化链路追踪管理器
	if cfg.Middleware.Tracing != nil && cfg.Middleware.Tracing.Enabled {
		manager.tracingManager, err = NewTracingManager(cfg.Middleware.Tracing)
		if err != nil {
			return nil, errors.NewErrorf(errors.ErrCodeTracingError, "failed to init tracing manager: %v", err)
		}
	}

	// 初始化i18n管理器
	if cfg.Middleware.I18N != nil && cfg.Middleware.I18N.Enabled {
		manager.i18nManager, err = NewI18nManager(cfg.Middleware.I18N)
		if err != nil {
			return nil, errors.NewErrorf(errors.ErrCodeMiddlewareError, "failed to init i18n manager: %v", err)
		}
	}

	// 初始化熔断中间件适配器（使用 go-config 的 CircuitBreaker 配置）
	manager.breakerAdapter = NewBreakerMiddlewareAdapter(breakercof.Default())

	// 初始化PB验证中间件
	manager.pbValidationMiddleware = NewPBValidationMiddleware()

	return manager, nil
}

// WithRateLimiter 设置限流器
func (m *Manager) WithRateLimiter(limiter RateLimiter) *Manager {
	m.rateLimiter = limiter
	return m
}

// WithAccessRecordHandler 设置访问记录处理器
func (m *Manager) WithAccessRecordHandler(handler AccessRecordHandler) *Manager {
	if handler != nil {
		m.accessRecordHandler = handler
	}
	return m
}

// WithSignatureValidator 设置签名验证器
func (m *Manager) WithSignatureValidator(validator SignatureValidator) *Manager {
	if validator != nil {
		m.signatureValidator = validator
	}
	return m
}

func (m *Manager) HTTPMetricsMiddleware() MiddlewareFunc {
	return HTTPMetrics(m.metricsManager)
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

// CORSMiddleware CORS 中间件
func (m *Manager) CORSMiddleware() MiddlewareFunc {
	// CORS配置通常在gateway配置中，这里暂时返回空实现
	return func(next http.Handler) http.Handler { return next }
}

// RecoveryMiddleware 恢复中间件
func (m *Manager) RecoveryMiddleware() MiddlewareFunc {
	// if m.cfg.Middleware.Recovery != nil && m.cfg.Middleware.Recovery.Enabled {
	// 	return MiddlewareFunc(RecoveryMiddleware())
	// }
	return MiddlewareFunc(RecoveryMiddleware()) // 恢复中间件通常总是启用
}

// ContextTraceMiddlewareFunc 统一的 Context 追踪中间件
// 负责 trace_id、request_id 等的注入，使用 go-logger 的统一管理
func (m *Manager) ContextTraceMiddlewareFunc() MiddlewareFunc {
	return MiddlewareFunc(ContextTraceMiddleware())
}

// SecurityMiddleware 安全中间件
// TODO: 重构为使用 go-config 的 security.Security 配置
func (m *Manager) SecurityMiddleware() MiddlewareFunc {
	// if m.cfg.Middleware.Security != nil && m.cfg.Middleware.Security.Enabled {
	// 	return MiddlewareFunc(ConfigurableSecurityMiddleware(m.cfg.Middleware.Security))
	// }
	return MiddlewareFunc(SecurityMiddleware()) // 回退到默认安全中间件
}

// RateLimitMiddleware 限流中间件
func (m *Manager) RateLimitMiddleware() MiddlewareFunc {
	// 优先使用配置
	if m.cfg != nil && m.cfg.RateLimit != nil && m.cfg.RateLimit.Enabled {
		return MiddlewareFunc(RateLimitMiddleware(m.cfg.RateLimit))
	}
	// 回退到空中间件
	return func(next http.Handler) http.Handler { return next }
}

// AccessRecordMiddleware 访问记录中间件
// TODO: 实现访问日志配置
func (m *Manager) AccessRecordMiddleware() MiddlewareFunc {
	return func(next http.Handler) http.Handler { return next }
}

// LoggingMiddleware HTTP日志中间件
func (m *Manager) LoggingMiddleware() MiddlewareFunc {
	// 使用配置的日志中间件
	if m.cfg.Middleware != nil && m.cfg.Middleware.Logging != nil {
		return MiddlewareFunc(LoggingMiddleware(m.cfg.Middleware.Logging))
	}
	// 回退到默认实现
	return MiddlewareFunc(LoggingMiddleware(nil))
}

// SignatureMiddleware 签名验证中间件
// TODO: 实现签名配置
func (m *Manager) SignatureMiddleware() MiddlewareFunc {
	return func(next http.Handler) http.Handler { return next }
}

// TimestampMiddleware 时间戳验证中间件
// TODO: 实现时间戳验证配置
func (m *Manager) TimestampMiddleware() MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 时间戳验证逻辑可以在这里实现
			// 目前暂时直接通过
			next.ServeHTTP(w, r)
		})
	}
}

// I18nMiddleware 国际化中间件
// TODO: 重构为使用 go-config 的 i18n.I18N 配置
func (m *Manager) I18nMiddleware() MiddlewareFunc {
	// if m.cfg.Middleware.I18N != nil && m.cfg.Middleware.I18N.Enabled {
	// 	return MiddlewareFunc(ConfigurableI18nMiddleware(m.cfg.Middleware.I18N))
	// }
	// 使用内部 i18n 管理器
	if m.i18nManager != nil {
		return I18nWithManager(m.i18nManager)
	}
	return I18n() // 回退到默认配置
}

// PProfMiddleware pprof性能分析中间件
func (m *Manager) PProfMiddleware() MiddlewareFunc {
	if m.cfg != nil && m.cfg.Middleware.PProf != nil && m.cfg.Middleware.PProf.Enabled {
		return MiddlewareFunc(PProfMiddleware(m.pprofAdapter))
	}
	return func(next http.Handler) http.Handler { return next }
}

// BreakerMiddleware 熔断中间件
func (m *Manager) BreakerMiddleware() MiddlewareFunc {
	if m.breakerAdapter == nil || !m.breakerAdapter.IsEnabled() {
		return func(next http.Handler) http.Handler { return next }
	}
	return m.breakerAdapter.Middleware()
}

// MetricsHandler 返回监控指标处理器
func (m *Manager) MetricsHandler() http.Handler {
	if m.metricsManager == nil {
		return http.NotFoundHandler()
	}
	return promhttp.Handler()
}

// PProfHandler 返回pprof处理器
func (m *Manager) PProfHandler() http.Handler {
	if m.cfg == nil || m.cfg.Middleware.PProf == nil || !m.cfg.Middleware.PProf.Enabled {
		return http.NotFoundHandler()
	}
	return CreatePProfHandler(m.pprofAdapter)
}

// GetBreakerAdapter 获取熔断中间件适配器
func (m *Manager) GetBreakerAdapter() *BreakerMiddlewareAdapter {
	return m.breakerAdapter
}

// getBaseMiddlewares 获取基础中间件链（所有环境共用）
func (m *Manager) getBaseMiddlewares() []MiddlewareFunc {
	return []MiddlewareFunc{
		m.RecoveryMiddleware(),         // Panic 恢复
		m.ContextTraceMiddlewareFunc(), // Context 追踪（trace_id、request_id）
		m.I18nMiddleware(),             // 国际化
	}
}

// appendObservabilityMiddlewares 添加可观测性中间件（监控、追踪）
func (m *Manager) appendObservabilityMiddlewares(middlewares []MiddlewareFunc) []MiddlewareFunc {
	// 添加监控中间件（如果启用）
	if m.metricsManager != nil {
		middlewares = append(middlewares, m.HTTPMetricsMiddleware())
	}

	// 添加链路追踪中间件（如果启用）
	if m.tracingManager != nil {
		middlewares = append(middlewares, m.HTTPTracingMiddleware())
	}

	return middlewares
}

// GetDefaultMiddlewares 获取默认中间件链
func (m *Manager) GetDefaultMiddlewares() []MiddlewareFunc {
	// 基础中间件
	middlewares := m.getBaseMiddlewares()

	// 业务中间件
	middlewares = append(middlewares, m.LoggingMiddleware())

	// 添加限流中间件（如果启用）
	if m.rateLimiter != nil {
		middlewares = append(middlewares, m.RateLimitMiddleware())
	}

	// 添加熔断中间件
	middlewares = append(middlewares, m.BreakerMiddleware())

	// 安全和功能中间件
	middlewares = append(middlewares,
		m.CORSMiddleware(),
		m.SecurityMiddleware(),
		m.AccessRecordMiddleware(),
	)

	// 可观测性中间件
	return m.appendObservabilityMiddlewares(middlewares)
}

// GetProductionMiddlewares 获取生产环境中间件链
func (m *Manager) GetProductionMiddlewares() []MiddlewareFunc {
	// 基础中间件
	middlewares := m.getBaseMiddlewares()

	// 生产环境核心中间件
	middlewares = append(middlewares,
		m.RateLimitMiddleware(),    // 限流
		m.BreakerMiddleware(),      // 熔断
		m.SignatureMiddleware(),    // 签名验证
		m.SecurityMiddleware(),     // 安全防护
		m.CORSMiddleware(),         // 跨域
		m.AccessRecordMiddleware(), // 访问记录
	)

	// 可观测性中间件
	return m.appendObservabilityMiddlewares(middlewares)
}

// GetDevelopmentMiddlewares 获取开发环境中间件链
func (m *Manager) GetDevelopmentMiddlewares() []MiddlewareFunc {
	// 基础中间件
	middlewares := m.getBaseMiddlewares()

	// 开发环境中间件
	middlewares = append(middlewares,
		m.LoggingMiddleware(), // 详细日志
		m.CORSMiddleware(),    // 跨域（开发常需要）
	)

	// PProf 性能分析（仅开发环境）
	if m.cfg != nil && m.cfg.Middleware.PProf != nil && m.cfg.Middleware.PProf.Enabled {
		middlewares = append(middlewares, m.PProfMiddleware())
	}

	// 可观测性中间件
	return m.appendObservabilityMiddlewares(middlewares)
}

// HTTPMiddleware 应用HTTP中间件链
func (m *Manager) HTTPMiddleware(handler http.Handler) http.Handler {
	var middlewares []MiddlewareFunc

	// 根据配置选择中间件链
	if m.isProductionMode() {
		middlewares = m.GetProductionMiddlewares()
	} else {
		middlewares = m.GetDevelopmentMiddlewares()
	}

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

// isProductionMode 检查是否为生产模式
func (m *Manager) isProductionMode() bool {
	return goconfig.IsProduction()
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
