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

	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-config/pkg/ratelimit"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

// Manager 中间件管理器 - 使用 go-config 的 middleware 配置
type Manager struct {
	cfg                    *gwconfig.Gateway
	metricsManager         *MetricsManager
	tracingManager         *TracingManager
	rateLimiter            RateLimiter
	signatureValidator     SignatureValidator
	i18nManager            *I18nManager
	pbValidationMiddleware *PBValidationMiddleware
	swaggerMiddleware      *SwaggerMiddleware
}

// NewManager 创建中间件管理器 - 使用全局 GATEWAY 配置
func NewManager(cfg *gwconfig.Gateway) (*Manager, error) {
	var err error
	manager := &Manager{
		cfg:                cfg,
		signatureValidator: &HMACValidator{},
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

	// 初始化PB验证中间件
	manager.pbValidationMiddleware = NewPBValidationMiddleware()

	// 初始化 Swagger 中间件
	if cfg.Swagger.Enabled {
		manager.swaggerMiddleware = NewSwaggerMiddleware(cfg.Swagger)
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

	return manager, nil
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

// CORSMiddleware CORS 中间件
func (m *Manager) CORSMiddleware() MiddlewareFunc {
	return MiddlewareFunc(CORSMiddleware(m.cfg.CORS))
}

// RecoveryMiddleware 恢复中间件
func (m *Manager) RecoveryMiddleware() MiddlewareFunc {
	return MiddlewareFunc(RecoveryMiddleware(m.cfg.Middleware.Recovery))
}

// ContextTraceMiddlewareFunc 统一的 Context 追踪中间件
// 负责 trace_id、request_id 等的注入，使用 go-logger 的统一管理
func (m *Manager) ContextTraceMiddlewareFunc() MiddlewareFunc {
	return MiddlewareFunc(ContextTraceMiddleware())
}

// SCPMiddleware 安全中间件 - 从配置读取 CSP 策略
func (m *Manager) SCPMiddleware() MiddlewareFunc {
	return MiddlewareFunc(SCPMiddleware(m.cfg.Security.CSP))
}

// RateLimitMiddleware 限流中间件
func (m *Manager) RateLimitMiddleware() MiddlewareFunc {
	return MiddlewareFunc(RateLimitMiddleware(m.cfg.RateLimit))
}

// LoggingMiddleware HTTP日志中间件
func (m *Manager) LoggingMiddleware() MiddlewareFunc {
	return MiddlewareFunc(LoggingMiddleware(m.cfg.Middleware.Logging))
}

// SignatureMiddleware 签名验证中间件
func (m *Manager) SignatureMiddleware() MiddlewareFunc {
	return MiddlewareFunc(SignatureMiddleware(m.cfg.Middleware.Signature, m.signatureValidator))
}

// TimestampMiddleware 时间戳验证中间件
func (m *Manager) TimestampMiddleware() MiddlewareFunc {
	return MiddlewareFunc(TimestampMiddleware(m.cfg.Middleware.Signature))
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

// BreakerMiddleware 熔断中间件
func (m *Manager) BreakerMiddleware() MiddlewareFunc {
	return MiddlewareFunc(BreakerMiddleware(m.cfg.Middleware.CircuitBreaker))
}

// MetricsHandler 返回监控指标处理器
func (m *Manager) MetricsHandler() http.Handler {
	if m.metricsManager == nil {
		return http.NotFoundHandler()
	}
	return promhttp.Handler()
}

// SwaggerHandler 返回 Swagger 文档处理器
func (m *Manager) SwaggerHandler() http.Handler {
	if m.swaggerMiddleware == nil {
		return http.NotFoundHandler()
	}

	// 创建处理函数
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Swagger 中间件会直接处理请求，不需要传递给下一个处理器
		nextHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			// Empty handler - Swagger middleware handles the request directly
		})
		handler := m.swaggerMiddleware.Handler()(nextHandler)
		handler.ServeHTTP(w, r)
	})
}

// GetSwaggerPaths 获取 Swagger 路由路径
func (m *Manager) GetSwaggerPaths() []string {
	if m.swaggerMiddleware == nil || !m.cfg.Swagger.Enabled {
		return nil
	}

	return []string{
		m.cfg.Swagger.UIPath + "/",
		m.cfg.Swagger.UIPath + "/index.html",
		m.cfg.Swagger.UIPath + "/swagger.json",
	}
}

// GetMiddlewares 获取中间件链（完全基于配置驱动）
func (m *Manager) GetMiddlewares() []MiddlewareFunc {
	var middlewares []MiddlewareFunc

	// 1. Recovery 中间件（始终启用，最先执行）
	middlewares = append(middlewares, m.RecoveryMiddleware())

	// 2. Context 追踪中间件（始终启用）
	middlewares = append(middlewares, m.ContextTraceMiddlewareFunc())

	// 3. 日志中间件（根据配置）
	if m.cfg.Middleware.Logging.Enabled {
		middlewares = append(middlewares, m.LoggingMiddleware())
	}

	// 4. 国际化中间件（根据配置）
	if m.cfg.Middleware.I18N.Enabled {
		middlewares = append(middlewares, m.I18nMiddleware())
	}

	// 5. 监控中间件（根据配置）
	if m.cfg.Monitoring.Metrics.Enabled && m.metricsManager != nil {
		middlewares = append(middlewares, m.HTTPMetricsMiddleware())
	}

	// 6. 链路追踪中间件（根据配置）
	if m.cfg.Middleware.Tracing.Enabled && m.tracingManager != nil {
		middlewares = append(middlewares, m.HTTPTracingMiddleware())
	}

	// 7. 限流中间件（根据配置）
	if m.cfg.RateLimit.Enabled && m.rateLimiter != nil {
		middlewares = append(middlewares, m.RateLimitMiddleware())
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
