/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-08 03:33:56
 * @FilePath: \go-rpc-gateway\middleware\manager.go
 * @Description: 中间件管理器
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/kamalyes/go-config/pkg/register"
	"github.com/kamalyes/go-rpc-gateway/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

// PProfConfig 为了向后兼容，保持这个类型别名
type PProfConfig = register.PProf

// Manager 中间件管理器 - 统一使用config包配置
type Manager struct {
	// 监控管理器
	metricsManager *MetricsManager
	tracingManager *TracingManager

	// 统一配置 - 全部使用config包
	middlewareConfig *config.MiddlewareConfig

	// 功能组件
	rateLimiter         RateLimiter
	accessRecordHandler AccessRecordHandler
	signatureValidator  SignatureValidator
	pprofScenarios      *PProfScenarios
	pprofAdapter        *PProfConfigAdapter
	i18nManager         *I18nManager
}

// NewManager 创建中间件管理器 - 使用统一配置
func NewManager(middlewareConfig *config.MiddlewareConfig) (*Manager, error) {
	var err error

	// 如果没有提供配置，使用默认配置
	if middlewareConfig == nil {
		middlewareConfig = &config.MiddlewareConfig{
			// 这里可以设置一些默认值，或者从defaults.go获取
		}
	}

	manager := &Manager{
		middlewareConfig:    middlewareConfig,
		accessRecordHandler: &LogAccessRecordHandler{},
		signatureValidator:  &HMACValidator{},
		pprofScenarios:      NewPProfScenarios(),
	}

	// 创建pprof适配器
	if middlewareConfig.PProf.Enabled {
		pprofConfig := &register.PProf{
			Enabled: middlewareConfig.PProf.Enabled,
		}
		manager.pprofAdapter = NewPProfConfigAdapter(pprofConfig)
	}

	// 初始化监控管理器
	if middlewareConfig.Metrics.Enabled {
		metricsConfig := &MetricsConfig{
			Enabled:   middlewareConfig.Metrics.Enabled,
			Path:      middlewareConfig.Metrics.Path,
			Namespace: middlewareConfig.Metrics.Namespace,
			Subsystem: middlewareConfig.Metrics.Subsystem,
		}
		manager.metricsManager, err = NewMetricsManager(metricsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to init metrics manager: %w", err)
		}
	}

	// 初始化链路追踪管理器
	if middlewareConfig.Tracing.Enabled {
		manager.tracingManager, err = NewTracingManager(&middlewareConfig.Tracing)
		if err != nil {
			return nil, fmt.Errorf("failed to init tracing manager: %w", err)
		}
	}

	// 初始化i18n管理器
	if middlewareConfig.I18n.Enabled {
		i18nConfig := &I18nConfig{
			DefaultLanguage:    middlewareConfig.I18n.DefaultLanguage,
			SupportedLanguages: middlewareConfig.I18n.SupportedLanguages,
			DetectionOrder:     middlewareConfig.I18n.Detection.Sources,
			LanguageParam:      middlewareConfig.I18n.Detection.QueryParam,
			LanguageHeader:     middlewareConfig.I18n.Detection.HeaderName,
			MessagesPath:       middlewareConfig.I18n.Translations.Path,
			EnableFallback:     middlewareConfig.I18n.Translations.Fallback,
		}
		manager.i18nManager, err = NewI18nManager(i18nConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to init i18n manager: %w", err)
		}
	}

	return manager, nil
}

// SetMiddlewareConfig 设置中间件配置
func (m *Manager) SetMiddlewareConfig(config *config.MiddlewareConfig) *Manager {
	if config != nil {
		m.middlewareConfig = config
	}
	return m
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

// HTTPMetricsMiddleware HTTP 监控中间件
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

// LoggingMiddleware 日志中间件
func (m *Manager) LoggingMiddleware() MiddlewareFunc {
	if m.middlewareConfig != nil && m.middlewareConfig.Logging.Enabled {
		return MiddlewareFunc(ConfigurableLoggingMiddleware(&m.middlewareConfig.Logging))
	}
	return MiddlewareFunc(LoggingMiddleware(nil)) // 回退到默认实现
}

// CORSMiddleware CORS 中间件
func (m *Manager) CORSMiddleware() MiddlewareFunc {
	// CORS配置通常在gateway配置中，这里暂时返回空实现
	return func(next http.Handler) http.Handler { return next }
}

// RecoveryMiddleware 恢复中间件
func (m *Manager) RecoveryMiddleware() MiddlewareFunc {
	if m.middlewareConfig != nil && m.middlewareConfig.Recovery.Enabled {
		return MiddlewareFunc(RecoveryMiddleware())
	}
	return MiddlewareFunc(RecoveryMiddleware()) // 恢复中间件通常总是启用
}

// RequestIDMiddleware 请求 ID 中间件
func (m *Manager) RequestIDMiddleware() MiddlewareFunc {
	if m.middlewareConfig != nil && m.middlewareConfig.RequestID.Enabled {
		return MiddlewareFunc(ConfigurableRequestIDMiddleware(&m.middlewareConfig.RequestID))
	}
	return RequestID() // 回退到默认实现
}

// SecurityMiddleware 安全中间件
func (m *Manager) SecurityMiddleware() MiddlewareFunc {
	if m.middlewareConfig != nil && m.middlewareConfig.Security.Enabled {
		return MiddlewareFunc(ConfigurableSecurityMiddleware(&m.middlewareConfig.Security))
	}
	return MiddlewareFunc(SecurityMiddleware()) // 回退到默认安全中间件
}

// RateLimitMiddleware 限流中间件
func (m *Manager) RateLimitMiddleware() MiddlewareFunc {
	if m.middlewareConfig != nil && m.middlewareConfig.RateLimit.Enabled {
		return MiddlewareFunc(ConfigurableRateLimitMiddleware(&m.middlewareConfig.RateLimit))
	}
	// 使用自定义限流器
	if m.rateLimiter != nil {
		return MiddlewareFunc(RateLimitMiddleware(m.rateLimiter))
	}
	return func(next http.Handler) http.Handler { return next } // 禁用时返回空中间件
}

// AccessRecordMiddleware 访问记录中间件
func (m *Manager) AccessRecordMiddleware() MiddlewareFunc {
	if m.middlewareConfig != nil && m.middlewareConfig.AccessLog.Enabled {
		// 这里需要实现AccessLog配置的中间件，暂时返回空实现
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return next }
}

// SignatureMiddleware 签名验证中间件
func (m *Manager) SignatureMiddleware() MiddlewareFunc {
	if m.middlewareConfig != nil && m.middlewareConfig.Signature.Enabled {
		// 这里需要实现Signature配置的中间件，暂时返回空实现
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return next }
}

// TimestampMiddleware 时间戳验证中间件
func (m *Manager) TimestampMiddleware() MiddlewareFunc {
	if m.middlewareConfig != nil && m.middlewareConfig.Signature.Enabled {
		// 简化的时间戳验证中间件实现
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// 时间戳验证逻辑可以在这里实现
				// 目前暂时直接通过
				next.ServeHTTP(w, r)
			})
		}
	}
	return func(next http.Handler) http.Handler { return next }
}

// I18nMiddleware 国际化中间件
func (m *Manager) I18nMiddleware() MiddlewareFunc {
	if m.middlewareConfig != nil && m.middlewareConfig.I18n.Enabled {
		return MiddlewareFunc(ConfigurableI18nMiddleware(&m.middlewareConfig.I18n))
	}
	// 使用内部 i18n 管理器
	if m.i18nManager != nil {
		return I18nWithManager(m.i18nManager)
	}
	return I18n() // 回退到默认配置
}

// PProfMiddleware pprof性能分析中间件
func (m *Manager) PProfMiddleware() MiddlewareFunc {
	if m.middlewareConfig != nil && m.middlewareConfig.PProf.Enabled {
		return MiddlewareFunc(PProfMiddleware(m.pprofAdapter))
	}
	return func(next http.Handler) http.Handler { return next }
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
	if m.middlewareConfig == nil || !m.middlewareConfig.PProf.Enabled {
		return http.NotFoundHandler()
	}
	return CreatePProfHandler(m.pprofAdapter)
}

// GetDefaultMiddlewares 获取默认中间件链
func (m *Manager) GetDefaultMiddlewares() []MiddlewareFunc {
	middlewares := []MiddlewareFunc{
		m.RecoveryMiddleware(),
		m.RequestIDMiddleware(),
		m.I18nMiddleware(),
	}

	// 添加限流中间件（如果启用）
	if m.middlewareConfig != nil && m.middlewareConfig.RateLimit.Enabled {
		middlewares = append(middlewares, m.RateLimitMiddleware())
	}

	// 添加签名验证中间件（如果启用）
	if m.middlewareConfig != nil && m.middlewareConfig.Signature.Enabled {
		middlewares = append(middlewares, m.SignatureMiddleware())
	}

	middlewares = append(middlewares,
		m.LoggingMiddleware(),
		m.CORSMiddleware(),
		m.SecurityMiddleware(),
	)

	// 添加访问记录中间件（如果启用）
	if m.middlewareConfig != nil && m.middlewareConfig.AccessLog.Enabled {
		middlewares = append(middlewares, m.AccessRecordMiddleware())
	}

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

// GetProductionMiddlewares 获取生产环境中间件链
func (m *Manager) GetProductionMiddlewares() []MiddlewareFunc {
	middlewares := []MiddlewareFunc{
		m.RecoveryMiddleware(),
		m.RequestIDMiddleware(),
		m.RateLimitMiddleware(),
		m.SignatureMiddleware(),
		m.SecurityMiddleware(),
		m.CORSMiddleware(),
		m.AccessRecordMiddleware(),
	}

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

// GetDevelopmentMiddlewares 获取开发环境中间件链
func (m *Manager) GetDevelopmentMiddlewares() []MiddlewareFunc {
	middlewares := []MiddlewareFunc{
		m.RecoveryMiddleware(),
		m.RequestIDMiddleware(),
		m.LoggingMiddleware(),
		m.CORSMiddleware(),
	}

	// 在开发环境中启用pprof中间件
	if m.middlewareConfig != nil && m.middlewareConfig.PProf.Enabled {
		middlewares = append(middlewares, m.PProfMiddleware())
	}

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
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 这里可以添加通用的gRPC拦截逻辑
		return handler(ctx, req)
	}
}

// StreamServerInterceptor 返回gRPC流拦截器
func (m *Manager) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// 这里可以添加通用的gRPC流拦截逻辑
		return handler(srv, ss)
	}
}

// isProductionMode 检查是否为生产模式
func (m *Manager) isProductionMode() bool {
	// 这里可以根据环境变量或配置判断
	return false // 默认开发模式
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
