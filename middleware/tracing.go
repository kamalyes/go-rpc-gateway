/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-05 22:15:32
 * @FilePath: \go-rpc-gateway\middleware\tracing.go
 * @Description: 链路追踪中间件 - 集成OpenTelemetry
 *
 * 配置说明（go-config 的 middleware.tracing 节点）：
 *
 *   enabled                : 是否启用追踪
 *   service-name           : 服务名称（上报到后端的 service.name）
 *   exporter-type          : 导出器类型，支持 zipkin / otlphttp / otlpgrpc / otlp(等同 otlphttp) / console / noop
 *   exporter-endpoint      : 导出器端点
 *                            - otlphttp: 支持完整 URL，如 http://127.0.0.1:30017/api/default
 *                                       会自动拆分 host 与 path，并追加 /v1/traces
 *                                       (http:// 协议自动启用 insecure)
 *                            - otlpgrpc: 仅需 host:port，如 127.0.0.1:5081
 *   exporter-headers       : 导出器自定义请求头 (map)，如 Authorization、stream-name、organization
 *   exporter-tls-insecure  : 是否跳过 TLS 验证 (默认 true)
 *   telemetry-log-level    : OTel SDK 自身日志级别 (debug/info/warn/error，默认 info)
 *                            对应 OTel Collector 的 service.telemetry.logs.level
 *                            warn/error: 仅输出错误；info: 增加诊断日志；debug: 增加详细日志
 *   sampler-type           : 采样器类型 (always/never/probability/parent_based)
 *   sampler-probability    : 采样概率
 *
 * 示例 - OTLP HTTP (OpenObserve):
 *   middleware:
 *     tracing:
 *       enabled: true
 *       service-name: test-service
 *       exporter-type: otlphttp
 *       exporter-endpoint: http://127.0.0.1:30017/api/default
 *       exporter-tls-insecure: true
 *       exporter-headers:
 *         Authorization: "Basic ZS5jbzZlbWRTVAtaW5206Q2pdAZXhhbXBsXhhbXB=="
 *         stream-name: default
 *       telemetry-log-level: warn
 *       sampler-type: always
 *       sampler-probability: 1.0
 *
 * 示例 - OTLP gRPC (OpenObserve):
 *   middleware:
 *     tracing:
 *       enabled: true
 *       service-name: test-service
 *       exporter-type: otlpgrpc
 *       exporter-endpoint: 127.0.0.1:5081
 *       exporter-tls-insecure: true
 *       exporter-headers:
 *         Authorization: "Basic ZS5jbzZlbWRTVAtaW5206Q2pdAZXhhbXBsXhhbXB=="
 *         organization: default
 *         stream-name: default
 *       telemetry-log-level: warn
 *       sampler-type: always
 *       sampler-probability: 1.0
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kamalyes/go-config/pkg/tracing"
	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/netx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
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

	// 应用遥测日志级别（控制 OpenTelemetry SDK 自身的日志输出）
	applyTelemetryLogLevel(cfg.TelemetryLogLevel)

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

	// 复用 resource.Default() 的 SchemaURL，避免与 SDK 默认资源的 Schema 版本冲突
	// (SDK 升级后 resource.Default() 的 schema 版本可能与 semconv import 版本不一致)
	defaultRes := resource.Default()
	return resource.Merge(
		defaultRes,
		resource.NewWithAttributes(
			defaultRes.SchemaURL(),
			attrs...,
		),
	)
}

// createExporter 创建导出器
func createExporter(cfg *tracing.Tracing) (sdktrace.SpanExporter, error) {
	switch cfg.ExporterType {
	case constants.TracingExporterZipkin:
		return zipkin.New(cfg.ExporterEndpoint)
	case constants.TracingExporterOTLPHTTP, constants.TracingExporterOTLP:
		return newOTLPHTTPExporter(cfg)
	case constants.TracingExporterOTLPGRPC:
		return newOTLPGRPCExporter(cfg)
	case constants.TracingExporterConsole, constants.TracingExporterNoop:
		fallthrough
	default:
		return nil, nil
	}
}

// newOTLPHTTPExporter 创建 OTLP HTTP 导出器
// endpoint 支持完整 URL（如 http://host:port/api/default），会自动拆分为 host 与 URL path，
// 并追加 /v1/traces 作为 traces 路径（与 OpenTelemetry Collector otlphttp 行为一致）
func newOTLPHTTPExporter(cfg *tracing.Tracing) (sdktrace.SpanExporter, error) {
	host, urlPath, schemeIsHTTP := parseOTLPHTTPAddress(cfg.ExporterEndpoint)

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(host),
		otlptracehttp.WithURLPath(urlPath),
	}
	// http 协议或显式配置 insecure 时跳过 TLS
	if schemeIsHTTP || cfg.ExporterTLSInsecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	if len(cfg.ExporterHeaders) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.ExporterHeaders))
	}

	if global.LOGGER != nil {
		global.LOGGER.DebugMsg(fmt.Sprintf("[tracing] OTLP HTTP exporter: endpoint=%s path=%s insecure=%v", host, urlPath, schemeIsHTTP || cfg.ExporterTLSInsecure))
	}

	return otlptracehttp.New(context.Background(), opts...)
}

// newOTLPGRPCExporter 创建 OTLP gRPC 导出器
// endpoint 仅需 host:port（如 127.0.0.1:5081），TLS 由 exporter-tls-insecure 控制
func newOTLPGRPCExporter(cfg *tracing.Tracing) (sdktrace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.ExporterEndpoint),
	}
	if cfg.ExporterTLSInsecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	if len(cfg.ExporterHeaders) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(cfg.ExporterHeaders))
	}

	if global.LOGGER != nil {
		global.LOGGER.DebugMsg(fmt.Sprintf("[tracing] OTLP gRPC exporter: endpoint=%s insecure=%v", cfg.ExporterEndpoint, cfg.ExporterTLSInsecure))
	}

	return otlptracegrpc.New(context.Background(), opts...)
}

// parseOTLPHTTPAddress 将 OTLP HTTP endpoint 解析为 host、urlPath 及是否为 http(明文)协议
// 支持两种输入：
//   - 完整 URL：http://127.0.0.1:30017/api/default → host=127.0.0.1:30017, path=/api/default/v1/traces
//   - host:port：127.0.0.1:30017 → host=127.0.0.1:30017, path=/v1/traces
func parseOTLPHTTPAddress(rawEndpoint string) (host, urlPath string, schemeIsHTTP bool) {
	rawEndpoint = strings.TrimSpace(rawEndpoint)
	if strings.HasPrefix(rawEndpoint, "http://") || strings.HasPrefix(rawEndpoint, "https://") {
		schemeIsHTTP = strings.HasPrefix(rawEndpoint, "http://")
		if u, err := url.Parse(rawEndpoint); err == nil {
			host = u.Host
			urlPath = strings.TrimRight(u.Path, "/")
		}
	}
	if host == "" {
		host = rawEndpoint
	}
	// 确保包含 traces 路径后缀，避免重复拼接
	if urlPath == "" {
		urlPath = "/v1/traces"
	} else if !strings.HasSuffix(urlPath, "/v1/traces") {
		urlPath = urlPath + "/v1/traces"
	}
	return
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
	// 如果未启用或manager为空，返回直通中间件
	if manager == nil || manager.config == nil || !manager.config.Enabled {
		return MiddlewareFunc(noopMiddleware)
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

			// 设置span属性（请求阶段）
			setSpanAttributes(span, r)

			// 注入trace信息到响应头
			otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(w.Header()))

			// 同步自定义 trace_id 与 OTel trace_id，确保 HTTP 日志、gRPC metadata、
			// OTel 后端（Jaeger/Zipkin）使用同一个 trace_id，实现全链路打通
			// 仅当客户端未显式提供 X-Trace-Id 头时才覆盖（尊重客户端传入的 trace_id）
			if r.Header.Get(constants.HeaderXTraceID) == "" {
				if spanCtx := span.SpanContext(); spanCtx.IsValid() {
					otelTraceID := spanCtx.TraceID().String()
					ctx = WithTraceID(ctx, otelTraceID)
					w.Header().Set(constants.HeaderXTraceID, otelTraceID)
				}
			}

			// 使用统一的 ResponseWriter 包装器
			rw := NewResponseWriter(w)
			defer rw.Release()

			// 将上下文传递给下一个处理器
			next.ServeHTTP(rw, r.WithContext(ctx))

			// 设置响应阶段属性
			setResponseAttributes(span, rw)
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

// setSpanAttributes 设置span属性（请求阶段）
func setSpanAttributes(span oteltrace.Span, r *http.Request) {
	// ---- HTTP 基础属性 ----
	span.SetAttributes(
		attribute.String(constants.TracingAttrHTTPMethod, r.Method),
		attribute.String(constants.TracingAttrHTTPURL, r.URL.String()),
		attribute.String(constants.TracingAttrHTTPPath, r.URL.Path),
		attribute.String(constants.TracingAttrHTTPScheme, r.URL.Scheme),
		attribute.String(constants.TracingAttrHTTPHost, r.Host),
		attribute.String(constants.TracingAttrHTTPUserAgent, r.Header.Get(constants.HeaderUserAgent)),
		attribute.String(constants.TracingAttrHTTPFlavor, r.Proto),
		attribute.String(constants.TracingAttrHTTPRequestContentType, r.Header.Get(constants.HeaderContentType)),
		attribute.String(constants.TracingAttrHTTPClientIP, netx.GetClientIP(r)),
	)

	// 请求体大小
	if r.ContentLength > 0 {
		span.SetAttributes(attribute.Int64(constants.TracingAttrHTTPRequestContentLength, r.ContentLength))
	}

	// 查询参数
	if r.URL.RawQuery != "" {
		span.SetAttributes(attribute.String(constants.TracingAttrHTTPQuery, r.URL.RawQuery))
	}

	// Referer
	if referer := r.Referer(); referer != "" {
		span.SetAttributes(attribute.String(constants.TracingAttrHTTPReferer, referer))
	}

	// X-Forwarded-For（记录原始转发链路）
	if xff := r.Header.Get(constants.HeaderXForwardedFor); xff != "" {
		span.SetAttributes(attribute.String(constants.TracingAttrForwardedFor, xff))
	}

	// ---- 网络属性 ----
	if remoteAddr := r.RemoteAddr; remoteAddr != "" {
		host, port, _ := net.SplitHostPort(remoteAddr)
		if host != "" {
			span.SetAttributes(attribute.String(constants.TracingAttrNetPeerIP, host))
		}
		if port != "" {
			span.SetAttributes(attribute.String(constants.TracingAttrNetPeerPort, port))
		}
	}

}

// setResponseAttributes 设置响应阶段span属性
func setResponseAttributes(span oteltrace.Span, rw *ResponseWriter) {
	span.SetAttributes(attribute.Int(constants.TracingAttrHTTPStatusCode, rw.StatusCode()))
	if bytesWritten := rw.BytesWritten(); bytesWritten > 0 {
		span.SetAttributes(attribute.Int64(constants.TracingAttrHTTPResponseContentLength, bytesWritten))
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
		span.AddEvent(constants.TracingEventLog, oteltrace.WithAttributes(attrs...))
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
		span.AddEvent(constants.TracingEventError, oteltrace.WithAttributes(attrs...))
		span.RecordError(err)
	}
}

// GetProvider 获取 TracerProvider
func (tm *TracingManager) GetProvider() oteltrace.TracerProvider {
	if tm == nil || tm.provider == nil {
		return noop.NewTracerProvider()
	}
	return tm.provider
}

// IsEnabled 检查追踪是否启用
func (tm *TracingManager) IsEnabled() bool {
	return tm != nil && tm.config != nil && tm.config.Enabled
}

// applyTelemetryLogLevel 根据 telemetry-log-level 配置 OpenTelemetry SDK 自身的日志输出
// 对应 OTel Collector 中 service.telemetry.logs.level 的语义：
//   - error/warn：仅输出错误（SDK Info 级诊断日志被抑制）
//   - info：输出 Info 级诊断日志与错误
//   - debug：输出全部诊断日志（含 V(1) 详细日志）
//
// 级别通过 logr.LogSink 适配器桥接到全局 logger.ILogger，未配置时使用 info 级别
func applyTelemetryLogLevel(level string) {
	lvl := parseTelemetryLogLevel(level)

	sink := &otelLogSink{logger: global.LOGGER, level: lvl}
	otel.SetLogger(logr.New(sink))
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		// 错误始终通过 error 级别输出（error >= warn，符合 error/warn 级别可见语义）
		if global.LOGGER != nil && logger.ERROR >= lvl {
			global.LOGGER.WithError(err).ErrorMsg("[otel] SDK error")
		}
	}))
}

// parseTelemetryLogLevel 将字符串级别转换为 logger.LogLevel，非法值降级为 INFO
func parseTelemetryLogLevel(level string) logger.LogLevel {
	level = strings.ToLower(strings.TrimSpace(level))
	switch level {
	case "debug":
		return logger.DEBUG
	case "warn", "warning":
		return logger.WARN
	case "error":
		return logger.ERROR
	default:
		return logger.INFO
	}
}

// otelLogSink 实现 logr.LogSink，将 OpenTelemetry SDK 的日志桥接到 go-logger.ILogger
type otelLogSink struct {
	logger logger.ILogger
	level  logger.LogLevel // 允许输出的最低级别
}

// Init logr.LogSink 初始化钩子
func (s *otelLogSink) Init(logr.RuntimeInfo) {}

// Enabled logr 中 level=0 对应 Info，level>=1 对应更详细的 V(N) 诊断日志
func (s *otelLogSink) Enabled(level int) bool {
	if s.logger == nil {
		return false
	}
	if level >= 1 {
		// 详细诊断日志仅在 debug 级别输出
		return s.level == logger.DEBUG
	}
	// Info 级诊断日志在 info/debug 级别输出
	return s.level == logger.DEBUG || s.level == logger.INFO
}

// Info 仅在 Enabled(level) 为 true 时被调用
func (s *otelLogSink) Info(level int, msg string, keysAndValues ...interface{}) {
	if s.logger == nil {
		return
	}
	if level >= 1 {
		s.logger.DebugMsg(formatOtelMsg(msg, keysAndValues...))
	} else {
		s.logger.InfoMsg(formatOtelMsg(msg, keysAndValues...))
	}
}

// Error 错误在 error/warn/info/debug 级别均可见（除 off 外）
func (s *otelLogSink) Error(err error, msg string, keysAndValues ...interface{}) {
	if s.logger == nil || logger.ERROR < s.level {
		return
	}
	if err != nil {
		s.logger.WithError(err).ErrorMsg(formatOtelMsg(msg, keysAndValues...))
	} else {
		s.logger.ErrorMsg(formatOtelMsg(msg, keysAndValues...))
	}
}

func (s *otelLogSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return s
}

func (s *otelLogSink) WithName(name string) logr.LogSink {
	return s
}

// formatOtelMsg 将 logr 的 msg 与 keysAndValues 格式化为单行文本
func formatOtelMsg(msg string, keysAndValues ...interface{}) string {
	if len(keysAndValues) == 0 {
		return "[otel] " + msg
	}
	parts := make([]string, 0, len(keysAndValues)/2+1)
	parts = append(parts, msg)
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		parts = append(parts, fmt.Sprintf("%v=%v", keysAndValues[i], keysAndValues[i+1]))
	}
	return "[otel] " + strings.Join(parts, " ")
}
