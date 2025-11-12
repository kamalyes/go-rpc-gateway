/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 00:00:00
 * @FilePath: \go-rpc-gateway\server\tracing.go
 * @Description: OpenTelemetry 链路追踪管理器
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"context"
	"fmt"

	gojaeger "github.com/kamalyes/go-config/pkg/jaeger"
	"github.com/kamalyes/go-rpc-gateway/global"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TracingManager OpenTelemetry 链路追踪管理器
type TracingManager struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	config   *gojaeger.Jaeger
	enabled  bool
}

// NewTracingManager 创建追踪管理器
func NewTracingManager(cfg *gojaeger.Jaeger) (*TracingManager, error) {
	// 如果追踪未启用，返回禁用的管理器
	if cfg == nil || !cfg.Enabled {
		return &TracingManager{
			enabled: false,
		}, nil
	}

	tm := &TracingManager{
		config:  cfg,
		enabled: true,
	}

	if err := tm.initTracing(); err != nil {
		return nil, fmt.Errorf("failed to initialize tracing: %w", err)
	}

	global.LOGGER.InfoKV("OpenTelemetry追踪已初始化",
		"service", cfg.ServiceName,
		"endpoint", cfg.Endpoint)

	return tm, nil
}

// initTracing 初始化 OpenTelemetry
func (tm *TracingManager) initTracing() error {
	// 创建 Jaeger exporter
	exporter, err := jaeger.New(
		jaeger.WithCollectorEndpoint(
			jaeger.WithEndpoint(tm.config.Endpoint),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create jaeger exporter: %w", err)
	}

	// 创建资源
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(tm.config.ServiceName),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// 配置采样器
	sampler := tm.getSampler()

	// 创建 TracerProvider
	tm.provider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// 设置全局 TracerProvider
	otel.SetTracerProvider(tm.provider)

	// 设置全局传播器
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	// 创建 Tracer
	tm.tracer = tm.provider.Tracer(tm.config.ServiceName)

	return nil
}

// getSampler 获取采样器
func (tm *TracingManager) getSampler() sdktrace.Sampler {
	// 使用配置中的采样类型，默认值已在 go-config 的 Default() 中设置
	samplingType := tm.config.Sampling.Type
	samplingParam := tm.config.Sampling.Param

	switch samplingType {
	case "const":
		if samplingParam >= 1 {
			return sdktrace.AlwaysSample()
		}
		return sdktrace.NeverSample()

	case "probabilistic":
		return sdktrace.TraceIDRatioBased(samplingParam)

	case "ratelimiting":
		// Note: SDK doesn't have built-in rate limiting sampler
		// You might need a custom implementation or use ParentBased
		return sdktrace.ParentBased(sdktrace.AlwaysSample())

	default:
		return sdktrace.AlwaysSample()
	}
}

// GetTracer 获取 Tracer
func (tm *TracingManager) GetTracer() trace.Tracer {
	if tm == nil || !tm.enabled {
		return noop.NewTracerProvider().Tracer("")
	}
	return tm.tracer
}

// GetProvider 获取 TracerProvider
func (tm *TracingManager) GetProvider() trace.TracerProvider {
	if tm == nil || !tm.enabled {
		return noop.NewTracerProvider()
	}
	return tm.provider
}

// IsEnabled 检查追踪是否启用
func (tm *TracingManager) IsEnabled() bool {
	return tm != nil && tm.enabled
}

// Shutdown 关闭追踪管理器
func (tm *TracingManager) Shutdown(ctx context.Context) error {
	if tm == nil || !tm.enabled || tm.provider == nil {
		return nil
	}

	global.LOGGER.InfoMsg("正在关闭OpenTelemetry追踪...")

	if err := tm.provider.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown tracer provider: %w", err)
	}

	global.LOGGER.InfoMsg("OpenTelemetry追踪已关闭")
	return nil
}

// StartSpan 开始一个新的 span
func (tm *TracingManager) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if tm == nil || !tm.enabled {
		return ctx, trace.SpanFromContext(ctx)
	}
	return tm.tracer.Start(ctx, name, opts...)
}

// EnableTracing 启用链路追踪功能（使用配置文件）
func (s *Server) EnableTracing() error {
	if s.config.Monitoring.Jaeger.Enabled {
		return s.EnableTracingWithConfig()
	}
	return nil
}

// EnableTracingWithConfig 使用自定义配置启用链路追踪
func (s *Server) EnableTracingWithConfig() error {
	if !s.config.Monitoring.Jaeger.Enabled {
		return nil
	}

	// 创建 TracingManager
	tracingManager, err := NewTracingManager(s.config.Monitoring.Jaeger)
	if err != nil {
		return fmt.Errorf("failed to create tracing manager: %w", err)
	}

	// 保存到 Server（可选，如果需要在其他地方访问）
	// s.tracingManager = tracingManager

	global.LOGGER.InfoKV("链路追踪已启用",
		"service", s.config.Monitoring.Jaeger.ServiceName,
		"endpoint", s.config.Monitoring.Jaeger.Endpoint,
		"sampling_type", s.config.Monitoring.Jaeger.Sampling.Type)

	// 注册关闭钩子
	go func() {
		<-s.ctx.Done()
		if err := tracingManager.Shutdown(context.Background()); err != nil {
			global.LOGGER.ErrorKV("关闭链路追踪失败", "error", err)
		}
	}()

	return nil
}
