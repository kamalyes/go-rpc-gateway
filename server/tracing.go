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

	gojaeger "github.com/kamalyes/go-config/pkg/jaeger"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TracingManager OpenTelemetry 链路追踪管理器（使用OTLP协议，兼容Jaeger）
type TracingManager struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	config   *gojaeger.Jaeger // 保持兼容性，配置结构不变
	enabled  bool
}

// NewTracingManager 创建追踪管理器（使用OTLP协议）
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
		return nil, errors.NewErrorf(errors.ErrCodeTracingError, "failed to initialize tracing: %v", err)
	}

	global.LOGGER.InfoKV("OpenTelemetry追踪已初始化",
		"service", cfg.ServiceName,
		"endpoint", cfg.Endpoint)

	return tm, nil
}

// initTracing 初始化 OpenTelemetry
func (tm *TracingManager) initTracing() error {
	// 创建 OTLP HTTP exporter (推荐使用，兼容 Jaeger)
	exporter, err := otlptracehttp.New(
		context.Background(),
		otlptracehttp.WithEndpoint(tm.config.Endpoint),
		otlptracehttp.WithInsecure(), // 如果使用 HTTP 而非 HTTPS
	)
	if err != nil {
		return errors.NewErrorf(errors.ErrCodeTracingError, "failed to create OTLP exporter: %v", err)
	}

	// 创建资源
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(tm.config.ServiceName),
		),
	)
	if err != nil {
		return errors.NewErrorf(errors.ErrCodeTracingError, "failed to create resource: %v", err)
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
		return errors.NewErrorf(errors.ErrCodeTracingError, "failed to shutdown tracer provider: %v", err)
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

// EnableTracing 启用链路追踪功能（使用配置文件，通过OTLP协议兼容Jaeger）
func (s *Server) EnableTracing() error {
	return mathx.IF(s.configSafe.IsJaegerEnabled(),
		s.EnableTracingWithConfig(),
		nil)
}

// EnableTracingWithConfig 使用自定义配置启用链路追踪（通过OTLP协议）
func (s *Server) EnableTracingWithConfig() error {
	if !s.configSafe.IsJaegerEnabled() {
		return nil // 如果未启用链路追踪，直接返回
	}

	// 创建 TracingManager
	tracingManager, err := NewTracingManager(s.config.Monitoring.Jaeger)
	if err != nil {
		return errors.NewErrorf(errors.ErrCodeTracingError, "failed to create tracing manager: %v", err)
	}

	// 保存到 Server（可选，如果需要在其他地方访问）
	// s.tracingManager = tracingManager

	global.LOGGER.InfoKV("链路追踪已启用（OTLP协议）",
		"service", s.configSafe.GetJaegerServiceName(""),
		"endpoint", s.configSafe.GetJaegerEndpoint(""),
		"sampling_type", s.configSafe.GetJaegerSamplingType(""))
	go func() {
		<-s.ctx.Done()
		if err := tracingManager.Shutdown(context.Background()); err != nil {
			global.LOGGER.ErrorKV("关闭链路追踪失败", "error", err)
		}
	}()

	return nil
}
