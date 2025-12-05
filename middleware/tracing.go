/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 13:25:55
 * @FilePath: \go-rpc-gateway\middleware\tracing.go
 * @Description: 链路追踪中间件 - 集成OpenTelemetry
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"context"
	"github.com/kamalyes/go-config/pkg/tracing"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"net/http"
)

// TracingManager 链路追踪管理器
type TracingManager struct {
	config   *tracing.Tracing
	tracer   oteltrace.Tracer
	provider *sdktrace.TracerProvider
}

// NewTracingManager 创建链路追踪管理器
func NewTracingManager(cfg *tracing.Tracing) (*TracingManager, error) {
	if !cfg.Enabled {
		return &TracingManager{config: cfg}, nil
	}

	// go-config 的 Default() 已经设置了所有默认值，无需再次设置

	// 创建资源
	res, err := createResource(cfg)
	if err != nil {
		return nil, err
	}

	// 创建导出器
	exporter, err := createExporter(cfg)
	if err != nil {
		return nil, err
	}

	sampler := createSampler(cfg)

	// 创建TracerProvider
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	}
	if exporter != nil {
		opts = append(opts, sdktrace.WithBatcher(exporter))
	}
	tp := sdktrace.NewTracerProvider(opts...)

	// 设置全局TracerProvider
	otel.SetTracerProvider(tp)

	// 设置全局传播器
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// 创建tracer
	tracer := tp.Tracer(cfg.ServiceName)

	return &TracingManager{
		config:   cfg,
		tracer:   tracer,
		provider: tp,
	}, nil
}

// GetTracer 获取 tracer
func (m *TracingManager) GetTracer() oteltrace.Tracer {
	if m == nil {
		return nil
	}
	return m.tracer
}

// createResource 创建OpenTelemetry资源
func createResource(cfg *tracing.Tracing) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(cfg.ServiceName),
		semconv.ServiceVersionKey.String(cfg.ServiceVersion),
		semconv.DeploymentEnvironmentKey.String(cfg.Environment),
	}

	// 添加自定义属性
	for key, value := range cfg.Attributes {
		attrs = append(attrs, attribute.String(key, value))
	}

	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			attrs...,
		),
	)
}

// createExporter 创建导出器
func createExporter(cfg *tracing.Tracing) (sdktrace.SpanExporter, error) {
	switch cfg.ExporterType {
	case constants.TracingExporterZipkin:
		return zipkin.New(cfg.ExporterEndpoint)
	case constants.TracingExporterOTLP:
		return otlptracehttp.New(
			context.Background(),
			otlptracehttp.WithEndpoint(cfg.ExporterEndpoint),
			otlptracehttp.WithInsecure(),
		)
	case constants.TracingExporterConsole, constants.TracingExporterNoop:
		fallthrough
	default:
		return nil, nil
	}
}

// createSampler 创建采样器
func createSampler(cfg *tracing.Tracing) sdktrace.Sampler {
	switch cfg.SamplerType {
	case constants.TracingSamplerAlways:
		return sdktrace.AlwaysSample()
	case constants.TracingSamplerNever:
		return sdktrace.NeverSample()
	case constants.TracingSamplerProbability:
		return sdktrace.TraceIDRatioBased(cfg.SamplerProbability)
	case constants.TracingSamplerParentBased:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplerProbability))
	default:
		return sdktrace.TraceIDRatioBased(constants.TracingDefaultSamplerProbability)
	}
}

// Tracing 链路追踪中间件
func Tracing(manager *TracingManager) MiddlewareFunc {
	return TracingWithConfig(manager)
}

// TracingWithConfig 带配置的链路追踪中间件
func TracingWithConfig(manager *TracingManager) MiddlewareFunc {
	// 如果未启用或manager为空，返回透明中间件
	if manager == nil || manager.config == nil || !manager.config.Enabled {
		return func(next http.Handler) http.Handler {
			return next // 直接返回下一个处理器
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 检查是否应该跳过追踪
			if shouldSkipTracing(r) {
				next.ServeHTTP(w, r)
				return
			}

			// 从请求头中提取传播的上下文
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// 创建span
			ctx, span := manager.tracer.Start(ctx, r.Method+" "+r.URL.Path)
			defer span.End()

			// 设置span属性
			setSpanAttributes(span, r)

			// 注入trace信息到响应头
			otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(w.Header()))

			// 使用统一的 ResponseWriter 包装器
			rw := NewResponseWriter(w)
			defer rw.Release()

			// 将上下文传递给下一个处理器
			next.ServeHTTP(rw, r.WithContext(ctx))

			// 设置响应状态相关属性
			span.SetAttributes(attribute.Int(constants.TracingAttrHTTPStatusCode, rw.StatusCode()))
			if rw.IsError() {
				span.RecordError(nil) // 记录错误状态
			}
		})
	}
}

// shouldSkipTracing 检查是否应该跳过追踪
func shouldSkipTracing(r *http.Request) bool {
	// 检查路径是否在跳过列表中
	for _, path := range constants.TracingDefaultSkipPaths {
		if r.URL.Path == path {
			return true
		}
	}

	// 检查用户代理是否在跳过列表中
	userAgent := r.Header.Get(constants.HeaderUserAgent)
	for _, ua := range constants.MiddlewareDefaultSkipUserAgents {
		if userAgent == ua {
			return true
		}
	}

	return false
}

// setSpanAttributes 设置span属性
func setSpanAttributes(span oteltrace.Span, r *http.Request) {
	span.SetAttributes(
		attribute.String(constants.TracingAttrHTTPMethod, r.Method),
		attribute.String(constants.TracingAttrHTTPURL, r.URL.String()),
		attribute.String(constants.TracingAttrHTTPPath, r.URL.Path),
		attribute.String(constants.TracingAttrHTTPScheme, r.URL.Scheme),
		attribute.String(constants.TracingAttrHTTPHost, r.Host),
		attribute.String(constants.TracingAttrHTTPUserAgent, r.Header.Get(constants.HeaderUserAgent)),
	)

	// 添加网络相关属性
	if remoteAddr := r.RemoteAddr; remoteAddr != "" {
		span.SetAttributes(attribute.String(constants.TracingAttrNetPeerIP, remoteAddr))
	}

}

// Shutdown 关闭链路追踪
func (tm *TracingManager) Shutdown(ctx context.Context) error {
	if tm.provider != nil {
		return tm.provider.Shutdown(ctx)
	}
	return nil
}

// StartSpan 开始一个新的span
func (tm *TracingManager) StartSpan(ctx context.Context, operationName string) (context.Context, oteltrace.Span) {
	if tm.tracer != nil {
		return tm.tracer.Start(ctx, operationName)
	}
	return ctx, oteltrace.SpanFromContext(ctx)
}

// LogInfo 记录信息到span
func LogInfo(ctx context.Context, message string, fields ...attribute.KeyValue) {
	span := oteltrace.SpanFromContext(ctx)
	if span.IsRecording() {
		attrs := append([]attribute.KeyValue{
			attribute.String("level", "info"),
			attribute.String("message", message),
		}, fields...)
		span.AddEvent("log", oteltrace.WithAttributes(attrs...))
	}
}

// LogError 记录错误到span
func LogError(ctx context.Context, err error, message string, fields ...attribute.KeyValue) {
	span := oteltrace.SpanFromContext(ctx)
	if span.IsRecording() {
		attrs := append([]attribute.KeyValue{
			attribute.String("level", "error"),
			attribute.String("message", message),
			attribute.String("error", err.Error()),
		}, fields...)
		span.AddEvent("error", oteltrace.WithAttributes(attrs...))
		span.RecordError(err)
	}
}
