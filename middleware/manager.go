/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 15:02:43
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

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

// Manager 中间件管理器
type Manager struct {
	metricsManager      *MetricsManager
	tracingManager      *TracingManager
	loggingConfig       *LoggingConfig
	corsConfig          *CORSConfig
	rateLimitConfig     *RateLimitConfig
	accessRecordConfig  *AccessRecordConfig
	signatureConfig     *SignatureConfig
	rateLimiter         RateLimiter
	accessRecordHandler AccessRecordHandler
	signatureValidator  SignatureValidator
}

// NewManager 创建中间件管理器
func NewManager(metricsConfig *MetricsConfig, tracingConfig *TracingConfig) (*Manager, error) {
	var err error

	manager := &Manager{
		loggingConfig:       DefaultLoggingConfig(),
		corsConfig:          DefaultCORSConfig(),
		rateLimitConfig:     DefaultRateLimitConfig(),
		accessRecordConfig:  DefaultAccessRecordConfig(),
		signatureConfig:     DefaultSignatureConfig(),
		accessRecordHandler: &LogAccessRecordHandler{},
		signatureValidator:  &HMACValidator{},
	}

	// 初始化监控管理器
	if metricsConfig != nil && metricsConfig.Enabled {
		manager.metricsManager, err = NewMetricsManager(metricsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to init metrics manager: %w", err)
		}
	}

	// 初始化链路追踪管理器
	if tracingConfig != nil && tracingConfig.Enabled {
		manager.tracingManager, err = NewTracingManager(tracingConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to init tracing manager: %w", err)
		}
	}

	return manager, nil
}

// WithLoggingConfig 设置日志配置
func (m *Manager) WithLoggingConfig(config *LoggingConfig) *Manager {
	if config != nil {
		m.loggingConfig = config
	}
	return m
}

// WithCORSConfig 设置 CORS 配置
func (m *Manager) WithCORSConfig(config *CORSConfig) *Manager {
	if config != nil {
		m.corsConfig = config
	}
	return m
}

// WithRateLimitConfig 设置限流配置
func (m *Manager) WithRateLimitConfig(config *RateLimitConfig) *Manager {
	if config != nil {
		m.rateLimitConfig = config
	}
	return m
}

// WithAccessRecordConfig 设置访问记录配置
func (m *Manager) WithAccessRecordConfig(config *AccessRecordConfig) *Manager {
	if config != nil {
		m.accessRecordConfig = config
	}
	return m
}

// WithSignatureConfig 设置签名配置
func (m *Manager) WithSignatureConfig(config *SignatureConfig) *Manager {
	if config != nil {
		m.signatureConfig = config
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
func (m *Manager) HTTPMetricsMiddleware() HTTPMiddleware {
	return HTTPMetricsMiddleware(m.metricsManager)
}

// HTTPTracingMiddleware HTTP 链路追踪中间件
func (m *Manager) HTTPTracingMiddleware() HTTPMiddleware {
	return HTTPTracingMiddleware(m.tracingManager)
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
func (m *Manager) LoggingMiddleware() HTTPMiddleware {
	return LoggingMiddleware(m.loggingConfig)
}

// CORSMiddleware CORS 中间件
func (m *Manager) CORSMiddleware() HTTPMiddleware {
	return CORSMiddlewareWithConfig(m.corsConfig)
}

// RecoveryMiddleware 恢复中间件
func (m *Manager) RecoveryMiddleware() HTTPMiddleware {
	return RecoveryMiddleware()
}

// RequestIDMiddleware 请求 ID 中间件
func (m *Manager) RequestIDMiddleware() HTTPMiddleware {
	return RequestIDMiddleware()
}

// SecurityMiddleware 安全中间件
func (m *Manager) SecurityMiddleware() HTTPMiddleware {
	return SecurityMiddleware()
}

// RateLimitMiddleware 限流中间件
func (m *Manager) RateLimitMiddleware() HTTPMiddleware {
	if m.rateLimiter != nil {
		return RateLimitMiddleware(m.rateLimiter)
	}
	return RateLimitMiddlewareWithConfig(m.rateLimitConfig)
}

// AccessRecordMiddleware 访问记录中间件
func (m *Manager) AccessRecordMiddleware() HTTPMiddleware {
	return AccessRecordMiddleware(m.accessRecordConfig, m.accessRecordHandler)
}

// SignatureMiddleware 签名验证中间件
func (m *Manager) SignatureMiddleware() HTTPMiddleware {
	return SignatureMiddleware(m.signatureConfig, m.signatureValidator)
}

// TimestampMiddleware 时间戳验证中间件
func (m *Manager) TimestampMiddleware() HTTPMiddleware {
	return TimestampMiddleware(m.signatureConfig)
}

// MetricsHandler 返回监控指标处理器
func (m *Manager) MetricsHandler() http.Handler {
	if m.metricsManager == nil {
		return http.NotFoundHandler()
	}
	return promhttp.Handler()
}

// GetDefaultMiddlewares 获取默认中间件链
func (m *Manager) GetDefaultMiddlewares() []HTTPMiddleware {
	middlewares := []HTTPMiddleware{
		m.RecoveryMiddleware(),
		m.RequestIDMiddleware(),
	}

	// 添加限流中间件（如果启用）
	if m.rateLimitConfig != nil && m.rateLimitConfig.Enabled {
		middlewares = append(middlewares, m.RateLimitMiddleware())
	}

	// 添加签名验证中间件（如果启用）
	if m.signatureConfig != nil && m.signatureConfig.Enabled {
		middlewares = append(middlewares, m.SignatureMiddleware())
	}

	middlewares = append(middlewares,
		m.LoggingMiddleware(),
		m.CORSMiddleware(),
		m.SecurityMiddleware(),
	)

	// 添加访问记录中间件（如果启用）
	if m.accessRecordConfig != nil && m.accessRecordConfig.Enabled {
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
func (m *Manager) GetProductionMiddlewares() []HTTPMiddleware {
	middlewares := []HTTPMiddleware{
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
func (m *Manager) GetDevelopmentMiddlewares() []HTTPMiddleware {
	middlewares := []HTTPMiddleware{
		m.RecoveryMiddleware(),
		m.RequestIDMiddleware(),
		m.LoggingMiddleware(),
		m.CORSMiddleware(),
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
	var middlewares []HTTPMiddleware

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
func ApplyMiddlewares(handler http.Handler, middlewares ...HTTPMiddleware) http.Handler {
	// 倒序应用中间件
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}
