/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-07 21:31:11
 * @FilePath: \go-rpc-gateway\middleware\observability.go
 * @Description: 可观测性中间件 - 完整的监控、追踪、指标管理
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"context"
	"net/http"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/kamalyes/go-config/pkg/monitoring"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// BreakerStat 熔断器统计信息
type BreakerStat struct {
	Path           string
	State          string
	TotalRequests  int64
	FailedRequests int64
	FailureCount   int32
	SuccessCount   int32
}

// WSCStats WebSocket 统计信息
type WSCStats struct {
	TotalClients     int64
	WebSocketClients int64
	SSEClients       int64
	AgentConnections int64
	OnlineUsers      int
	MessagesSent     int64
	MessagesReceived int64
	BroadcastsSent   int64
	QueuedMessages   int
	Uptime           int64
}

// MetricsManager 可观测性管理器 - 统一管理 Prometheus 指标（HTTP + gRPC）
type MetricsManager struct {
	registry      *prometheus.Registry
	serverMetrics *grpc_prometheus.ServerMetrics
	clientMetrics *grpc_prometheus.ClientMetrics
	httpMetrics   *HTTPMetrics
	panicCounter  prometheus.Counter
	config        *monitoring.Monitoring

	// 扩展业务指标（通过函数注入采集逻辑，避免包循环依赖）
	rateLimitRejected *prometheus.CounterVec // 限流拒绝计数器

	// 依赖采集函数（由 server 包在初始化后注入，按需在 scrape 时调用）
	poolHealthFn   func() map[string]bool // 连接池健康检查采集函数
	breakerStatsFn func() []BreakerStat   // 熔断器统计采集函数
	wscStatsFn     func() *WSCStats       // WebSocket 统计采集函数
}

// HTTPMetrics HTTP 请求指标
type HTTPMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestSize     *prometheus.SummaryVec
	responseSize    *prometheus.SummaryVec
	activeRequests  prometheus.Gauge
	pathNormalizer  PathNormalizer // 路径规范化器，减少标签基数
}

// NewMetricsManager 创建可观测性管理器（支持 gRPC + HTTP 完整指标）
// go-config 的 Default() 已经设置了所有默认值（包括 Buckets），无需再次设置
func NewMetricsManager(cfg *monitoring.Monitoring) *MetricsManager {
	if !cfg.Metrics.Enabled {
		return nil
	}

	registry := prometheus.NewRegistry()

	// 注册 Go runtime 和 process 默认采集器，保留 runtime/process 指标
	registry.MustRegister(prometheus.NewGoCollector())
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{
		Namespace:    "",
		ReportErrors: true,
	}))

	// 安全获取直方图桶（如果无法安全访问则使用默认值）
	var buckets []float64
	if len(cfg.Metrics.Buckets) > 0 {
		buckets = cfg.Metrics.Buckets
	} else {
		// 提供默认的桶配置
		buckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	}

	// 创建 gRPC 服务器指标
	serverMetrics := grpc_prometheus.NewServerMetrics(
		grpc_prometheus.WithServerHandlingTimeHistogram(
			grpc_prometheus.WithHistogramBuckets(buckets),
		),
	)
	registry.MustRegister(serverMetrics)

	// 创建 gRPC 客户端指标
	clientMetrics := grpc_prometheus.NewClientMetrics(
		grpc_prometheus.WithClientHandlingTimeHistogram(
			grpc_prometheus.WithHistogramBuckets(buckets),
		),
	)
	registry.MustRegister(clientMetrics)

	// 创建 panic 恢复计数器
	panicCounter := promauto.With(registry).NewCounter(prometheus.CounterOpts{
		Name: "grpc_req_panics_recovered_total",
		Help: "Total number of gRPC requests recovered from internal panic.",
	})

	// 创建限流拒绝计数器（按策略和原因打标）
	rateLimitRejected := promauto.With(registry).NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_ratelimit_rejected_total",
			Help: "被限流拒绝的请求总数",
		},
		[]string{"strategy"},
	)

	// 创建 HTTP 指标
	httpMetrics := newHTTPMetrics(registry, buckets, cfg.Metrics.StaticPaths)

	mm := &MetricsManager{
		registry:          registry,
		serverMetrics:     serverMetrics,
		clientMetrics:     clientMetrics,
		httpMetrics:       httpMetrics,
		panicCounter:      panicCounter,
		rateLimitRejected: rateLimitRejected,
		config:            cfg,
	}

	// 注册扩展业务指标采集器（采集函数由 server 包在初始化后注入，未注入时输出空指标）
	registry.MustRegister(newPoolCollector(mm))
	registry.MustRegister(newBreakerCollector(mm))
	registry.MustRegister(newWSCCollector(mm))

	if global.LOGGER != nil {
		global.LOGGER.InfoMsg("Prometheus可观测性管理器已初始化（含 runtime/process 采集器）")
	}
	return mm
}

// newHTTPMetrics 创建 HTTP 指标（智能路径规范化）
func newHTTPMetrics(registry *prometheus.Registry, buckets []float64, staticPaths []string) *HTTPMetrics {
	return &HTTPMetrics{
		requestsTotal: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		requestDuration: promauto.With(registry).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request latencies in seconds",
				Buckets: buckets,
			},
			[]string{"method", "path"},
		),
		requestSize: promauto.With(registry).NewSummaryVec(
			prometheus.SummaryOpts{
				Name: "http_request_size_bytes",
				Help: "HTTP request sizes in bytes",
			},
			[]string{"method", "path"},
		),
		responseSize: promauto.With(registry).NewSummaryVec(
			prometheus.SummaryOpts{
				Name: "http_response_size_bytes",
				Help: "HTTP response sizes in bytes",
			},
			[]string{"method", "path"},
		),
		activeRequests: promauto.With(registry).NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_in_flight",
				Help: "Current number of HTTP requests being served",
			},
		),
		pathNormalizer: newSmartPathNormalizer(staticPaths),
	}
}

// RecordHTTPRequest 记录 HTTP 请求（使用详细指标）
func (mm *MetricsManager) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration, requestSize, responseSize int64) {
	if mm == nil || mm.httpMetrics == nil {
		return
	}

	// 规范化路径，减少标签基数
	normalizedPath := mm.httpMetrics.pathNormalizer.Normalize(path)

	// 记录请求总数
	mm.httpMetrics.requestsTotal.WithLabelValues(method, normalizedPath, http.StatusText(statusCode)).Inc()

	// 记录请求持续时间
	mm.httpMetrics.requestDuration.WithLabelValues(method, normalizedPath).Observe(duration.Seconds())

	// 记录请求大小
	if requestSize > 0 {
		mm.httpMetrics.requestSize.WithLabelValues(method, normalizedPath).Observe(float64(requestSize))
	}

	// 记录响应大小
	if responseSize > 0 {
		mm.httpMetrics.responseSize.WithLabelValues(method, normalizedPath).Observe(float64(responseSize))
	}
}

// RecordGRPCRequest 记录 gRPC 请求（gRPC 指标由 serverMetrics 自动处理）
func (mm *MetricsManager) RecordGRPCRequest(duration time.Duration) {
	// gRPC 指标由 grpc_prometheus.ServerMetrics 自动记录
	// 此方法保留用于兼容性或自定义逻辑
}

// HTTPMetricsMiddleware HTTP 指标中间件（简化版，用于快速集成）
func HTTPMetricsMiddleware(m *MetricsManager) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m == nil || m.httpMetrics == nil {
				next.ServeHTTP(w, r)
				return
			}

			// 增加活跃请求计数
			m.httpMetrics.activeRequests.Inc()
			defer m.httpMetrics.activeRequests.Dec()

			// 包装 ResponseWriter
			wrapped := &metricsResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			start := time.Now()
			next.ServeHTTP(wrapped, r)
			duration := time.Since(start)

			// 记录指标
			m.RecordHTTPRequest(
				r.Method,
				r.URL.Path,
				wrapped.statusCode,
				duration,
				r.ContentLength,
				int64(wrapped.bytesWritten),
			)
		})
	}
}

// GetRegistry 获取 Prometheus 注册表
func (mm *MetricsManager) GetRegistry() *prometheus.Registry {
	return mm.registry
}

// GetServerMetrics 获取 gRPC 服务器指标
func (mm *MetricsManager) GetServerMetrics() *grpc_prometheus.ServerMetrics {
	return mm.serverMetrics
}

// GetClientMetrics 获取 gRPC 客户端指标
func (mm *MetricsManager) GetClientMetrics() *grpc_prometheus.ClientMetrics {
	return mm.clientMetrics
}

// GetPanicCounter 获取 panic 计数器
func (mm *MetricsManager) GetPanicCounter() prometheus.Counter {
	return mm.panicCounter
}

// GetRateLimitRejected 获取限流拒绝计数器（供限流中间件记录）
func (mm *MetricsManager) GetRateLimitRejected() *prometheus.CounterVec {
	return mm.rateLimitRejected
}

// RecordRateLimitRejected 记录限流拒绝（strategy 为限流策略名称）
func (mm *MetricsManager) RecordRateLimitRejected(strategy string) {
	if mm == nil || mm.rateLimitRejected == nil {
		return
	}
	mm.rateLimitRejected.WithLabelValues(strategy).Inc()
}

// SetPoolHealthFn 注入连接池健康检查采集函数
func (mm *MetricsManager) SetPoolHealthFn(fn func() map[string]bool) {
	if mm == nil {
		return
	}
	mm.poolHealthFn = fn
}

// SetBreakerStatsFn 注入熔断器统计采集函数
func (mm *MetricsManager) SetBreakerStatsFn(fn func() []BreakerStat) {
	if mm == nil {
		return
	}
	mm.breakerStatsFn = fn
}

// SetWSCStatsFn 注入 WebSocket 统计采集函数
func (mm *MetricsManager) SetWSCStatsFn(fn func() *WSCStats) {
	if mm == nil {
		return
	}
	mm.wscStatsFn = fn
}

// Handler 创建 Prometheus HTTP 处理器
func (mm *MetricsManager) Handler() http.Handler {
	if mm == nil || mm.registry == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Metrics not available", http.StatusServiceUnavailable)
		})
	}

	opts := promhttp.HandlerOpts{}

	if mm.config.Metrics.EnableOpenMetrics {
		opts.EnableOpenMetrics = true
	}

	return promhttp.HandlerFor(mm.registry, opts)
}

// ExemplarFromContext 从上下文中提取 Exemplar（用于关联 trace）
// 传入 context.Context，从中提取 OTel span 的 trace_id 作为 Prometheus Exemplar
func ExemplarFromContext(ctx context.Context) prometheus.Labels {
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() && spanCtx.IsSampled() {
		return prometheus.Labels{"trace_id": spanCtx.TraceID().String()}
	}
	return nil
}

// HTTPMiddleware 返回 HTTP 指标中间件
func (mm *MetricsManager) HTTPMiddleware() func(http.Handler) http.Handler {
	if mm == nil || mm.httpMetrics == nil {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 增加活跃请求计数
			mm.httpMetrics.activeRequests.Inc()
			defer mm.httpMetrics.activeRequests.Dec()

			// 规范化路径，减少标签基数
			normalizedPath := mm.httpMetrics.pathNormalizer.Normalize(r.URL.Path)

			// 记录请求大小
			if r.ContentLength > 0 {
				mm.httpMetrics.requestSize.WithLabelValues(r.Method, normalizedPath).Observe(float64(r.ContentLength))
			}

			// 包装 ResponseWriter 以捕获状态码和响应大小
			wrapped := &metricsResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// 记录请求开始时间
			start := time.Now()

			// 执行下一个处理器
			next.ServeHTTP(wrapped, r)

			// 记录持续时间
			duration := time.Since(start).Seconds()
			mm.httpMetrics.requestDuration.WithLabelValues(r.Method, normalizedPath).Observe(duration)

			// 记录请求总数
			mm.httpMetrics.requestsTotal.WithLabelValues(
				r.Method,
				normalizedPath,
				http.StatusText(wrapped.statusCode),
			).Inc()

			// 记录响应大小
			if wrapped.bytesWritten > 0 {
				mm.httpMetrics.responseSize.WithLabelValues(r.Method, normalizedPath).Observe(float64(wrapped.bytesWritten))
			}
		})
	}
}

// metricsResponseWriter 包装 http.ResponseWriter 以捕获状态码和写入字节数
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *metricsResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *metricsResponseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// ============================================================================
// 可观测性中间件 - HTTP & gRPC 拦截器
// ============================================================================

// HTTPMiddleware HTTP 中间件接口
type HTTPMiddleware func(http.Handler) http.Handler

// GRPCInterceptor gRPC 拦截器类型
type GRPCInterceptor = grpc.UnaryServerInterceptor

// HTTPTracingMiddleware HTTP 链路追踪中间件
// Deprecated: 此实现缺少 W3C traceparent 传播提取/注入及 trace_id 同步，
// 请使用 tracing.go 中的 Tracing() 中间件（Manager.HTTPTracingMiddleware 已正确委托）。
func HTTPTracingMiddleware(tracingManager *TracingManager) HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		if tracingManager == nil || tracingManager.GetTracer() == nil {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := tracingManager.GetTracer().Start(r.Context(), r.Method+" "+r.URL.Path)
			defer span.End()

			// 添加属性
			span.SetAttributes(
				attribute.String(constants.TracingAttrHTTPMethod, r.Method),
				attribute.String(constants.TracingAttrHTTPURL, r.URL.String()),
				attribute.String(constants.TracingAttrHTTPUserAgent, r.UserAgent()),
			)

			// 处理请求
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GRPCMetricsInterceptor gRPC 监控拦截器
func GRPCMetricsInterceptor(metricsManager *MetricsManager) GRPCInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if metricsManager == nil {
			return handler(ctx, req)
		}

		start := time.Now()

		// 处理请求
		resp, err := handler(ctx, req)

		// 记录指标
		duration := time.Since(start)
		metricsManager.RecordGRPCRequest(duration)

		return resp, err
	}
}

// GRPCTracingInterceptor gRPC 链路追踪拦截器
func GRPCTracingInterceptor(tracingManager *TracingManager) GRPCInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if tracingManager == nil || tracingManager.GetTracer() == nil {
			return handler(ctx, req)
		}

		// 从 gRPC metadata 提取 W3C traceparent 传播上下文
		ctx = extractPropagationFromMetadata(ctx)

		// 创建 span
		ctx, span := tracingManager.GetTracer().Start(ctx, info.FullMethod)
		defer span.End()

		// 设置 span 属性
		span.SetAttributes(
			attribute.String(constants.TracingAttrRPCSystem, "grpc"),
			attribute.String(constants.TracingAttrRPCMethod, info.FullMethod),
		)

		// 从 metadata 中获取额外信息
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if userAgent := md.Get("user-agent"); len(userAgent) > 0 {
				span.SetAttributes(attribute.String(constants.TracingAttrHTTPUserAgent, userAgent[0]))
			}
		}

		// 同步 OTel trace_id 与自定义 trace_id，确保日志、gRPC metadata、OTel 后端一致
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if len(md.Get(constants.MetadataTraceID)) == 0 {
				if spanCtx := span.SpanContext(); spanCtx.IsValid() {
					otelTraceID := spanCtx.TraceID().String()
					ctx = WithTraceID(ctx, otelTraceID)
				}
			}
		}

		// 处理请求
		return handler(ctx, req)
	}
}

// GRPCTracingStreamInterceptor gRPC 流式链路追踪拦截器
func GRPCTracingStreamInterceptor(tracingManager *TracingManager) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if tracingManager == nil || tracingManager.GetTracer() == nil {
			return handler(srv, ss)
		}

		ctx := ss.Context()

		// 从 gRPC metadata 提取 W3C traceparent 传播上下文
		ctx = extractPropagationFromMetadata(ctx)

		// 创建 span
		ctx, span := tracingManager.GetTracer().Start(ctx, info.FullMethod)
		defer span.End()

		// 设置 span 属性
		span.SetAttributes(
			attribute.String(constants.TracingAttrRPCSystem, "grpc"),
			attribute.String(constants.TracingAttrRPCMethod, info.FullMethod),
		)

		// 从 metadata 中获取额外信息
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if userAgent := md.Get("user-agent"); len(userAgent) > 0 {
				span.SetAttributes(attribute.String(constants.TracingAttrHTTPUserAgent, userAgent[0]))
			}
		}

		// 同步 OTel trace_id
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if len(md.Get(constants.MetadataTraceID)) == 0 {
				if spanCtx := span.SpanContext(); spanCtx.IsValid() {
					ctx = WithTraceID(ctx, spanCtx.TraceID().String())
				}
			}
		}

		// 包装 ServerStream 以使用增强后的 context
		wrappedStream := &contextWrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		return handler(srv, wrappedStream)
	}
}

// extractPropagationFromMetadata 从 gRPC metadata 提取 W3C traceparent 传播上下文
// gRPC 场景下传播信息存放在 metadata（等价于 HTTP/2 头部）中，
// 需要手动提取后注入 context，后续 tracer.Start 才能正确关联父 span
func extractPropagationFromMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	// 将 gRPC metadata 转为 HTTP HeaderCarrier 以复用 OTel TextMapPropagator
	carrier := propagation.HeaderCarrier(md)
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// ============================================================================
// 扩展业务指标自定义 Collector（按需采集，scrape 时才调用采集函数）
// ============================================================================

// poolCollector 连接池健康指标采集器
type poolCollector struct {
	mm   *MetricsManager
	desc *prometheus.Desc
}

func newPoolCollector(mm *MetricsManager) *poolCollector {
	return &poolCollector{
		mm: mm,
		desc: prometheus.NewDesc(
			"gateway_pool_health",
			"连接池组件健康状态（1=健康, 0=不健康）",
			[]string{"component"},
			nil,
		),
	}
}

func (c *poolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *poolCollector) Collect(ch chan<- prometheus.Metric) {
	if c.mm == nil || c.mm.poolHealthFn == nil {
		return
	}
	health := c.mm.poolHealthFn()
	for component, ok := range health {
		val := 0.0
		if ok {
			val = 1.0
		}
		if m, err := prometheus.NewConstMetric(c.desc, prometheus.GaugeValue, val, component); err == nil {
			ch <- m
		}
	}
}

// breakerCollector 熔断器指标采集器
type breakerCollector struct {
	mm             *MetricsManager
	stateDesc      *prometheus.Desc
	totalDesc      *prometheus.Desc
	failedDesc     *prometheus.Desc
	failureCntDesc *prometheus.Desc
	successCntDesc *prometheus.Desc
}

func newBreakerCollector(mm *MetricsManager) *breakerCollector {
	return &breakerCollector{
		mm: mm,
		stateDesc: prometheus.NewDesc(
			"gateway_breaker_state",
			"熔断器状态（0=closed, 1=open, 2=half_open）",
			[]string{"path"},
			nil,
		),
		totalDesc: prometheus.NewDesc(
			"gateway_breaker_requests_total",
			"熔断器总请求数",
			[]string{"path"},
			nil,
		),
		failedDesc: prometheus.NewDesc(
			"gateway_breaker_failed_requests_total",
			"熔断器失败请求数",
			[]string{"path"},
			nil,
		),
		failureCntDesc: prometheus.NewDesc(
			"gateway_breaker_failure_count",
			"熔断器当前连续失败计数",
			[]string{"path"},
			nil,
		),
		successCntDesc: prometheus.NewDesc(
			"gateway_breaker_success_count",
			"熔断器当前连续成功计数",
			[]string{"path"},
			nil,
		),
	}
}

func (c *breakerCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.stateDesc
	ch <- c.totalDesc
	ch <- c.failedDesc
	ch <- c.failureCntDesc
	ch <- c.successCntDesc
}

func (c *breakerCollector) Collect(ch chan<- prometheus.Metric) {
	if c.mm == nil || c.mm.breakerStatsFn == nil {
		return
	}
	stats := c.mm.breakerStatsFn()
	for _, s := range stats {
		// 状态映射：closed=0, open=1, half_open=2
		var stateVal float64
		switch s.State {
		case "closed":
			stateVal = 0
		case "open":
			stateVal = 1
		case "half_open":
			stateVal = 2
		}
		if m, err := prometheus.NewConstMetric(c.stateDesc, prometheus.GaugeValue, stateVal, s.Path); err == nil {
			ch <- m
		}
		if m, err := prometheus.NewConstMetric(c.totalDesc, prometheus.CounterValue, float64(s.TotalRequests), s.Path); err == nil {
			ch <- m
		}
		if m, err := prometheus.NewConstMetric(c.failedDesc, prometheus.CounterValue, float64(s.FailedRequests), s.Path); err == nil {
			ch <- m
		}
		if m, err := prometheus.NewConstMetric(c.failureCntDesc, prometheus.GaugeValue, float64(s.FailureCount), s.Path); err == nil {
			ch <- m
		}
		if m, err := prometheus.NewConstMetric(c.successCntDesc, prometheus.GaugeValue, float64(s.SuccessCount), s.Path); err == nil {
			ch <- m
		}
	}
}

// wscCollector WebSocket 指标采集器
type wscCollector struct {
	mm               *MetricsManager
	totalClientsDesc *prometheus.Desc
	wsClientsDesc    *prometheus.Desc
	sseClientsDesc   *prometheus.Desc
	agentConnDesc    *prometheus.Desc
	onlineUsersDesc  *prometheus.Desc
	msgSentDesc      *prometheus.Desc
	msgRecvDesc      *prometheus.Desc
	broadcastsDesc   *prometheus.Desc
	queuedMsgDesc    *prometheus.Desc
	uptimeDesc       *prometheus.Desc
}

func newWSCCollector(mm *MetricsManager) *wscCollector {
	return &wscCollector{
		mm: mm,
		totalClientsDesc: prometheus.NewDesc(
			"gateway_wsc_total_clients", "WebSocket 服务总客户端数", nil, nil,
		),
		wsClientsDesc: prometheus.NewDesc(
			"gateway_wsc_websocket_clients", "WebSocket 客户端数", nil, nil,
		),
		sseClientsDesc: prometheus.NewDesc(
			"gateway_wsc_sse_clients", "SSE 客户端数", nil, nil,
		),
		agentConnDesc: prometheus.NewDesc(
			"gateway_wsc_agent_connections", "座席连接数", nil, nil,
		),
		onlineUsersDesc: prometheus.NewDesc(
			"gateway_wsc_online_users", "在线用户数", nil, nil,
		),
		msgSentDesc: prometheus.NewDesc(
			"gateway_wsc_messages_sent_total", "已发送消息总数", nil, nil,
		),
		msgRecvDesc: prometheus.NewDesc(
			"gateway_wsc_messages_received_total", "已接收消息总数", nil, nil,
		),
		broadcastsDesc: prometheus.NewDesc(
			"gateway_wsc_broadcasts_sent_total", "已发送广播总数", nil, nil,
		),
		queuedMsgDesc: prometheus.NewDesc(
			"gateway_wsc_queued_messages", "排队消息数", nil, nil,
		),
		uptimeDesc: prometheus.NewDesc(
			"gateway_wsc_uptime_seconds", "WebSocket 服务运行时长（秒）", nil, nil,
		),
	}
}

func (c *wscCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.totalClientsDesc
	ch <- c.wsClientsDesc
	ch <- c.sseClientsDesc
	ch <- c.agentConnDesc
	ch <- c.onlineUsersDesc
	ch <- c.msgSentDesc
	ch <- c.msgRecvDesc
	ch <- c.broadcastsDesc
	ch <- c.queuedMsgDesc
	ch <- c.uptimeDesc
}

func (c *wscCollector) Collect(ch chan<- prometheus.Metric) {
	if c.mm == nil || c.mm.wscStatsFn == nil {
		return
	}
	stats := c.mm.wscStatsFn()
	if stats == nil {
		return
	}
	c.emit(ch, c.totalClientsDesc, prometheus.GaugeValue, float64(stats.TotalClients))
	c.emit(ch, c.wsClientsDesc, prometheus.GaugeValue, float64(stats.WebSocketClients))
	c.emit(ch, c.sseClientsDesc, prometheus.GaugeValue, float64(stats.SSEClients))
	c.emit(ch, c.agentConnDesc, prometheus.GaugeValue, float64(stats.AgentConnections))
	c.emit(ch, c.onlineUsersDesc, prometheus.GaugeValue, float64(stats.OnlineUsers))
	c.emit(ch, c.msgSentDesc, prometheus.CounterValue, float64(stats.MessagesSent))
	c.emit(ch, c.msgRecvDesc, prometheus.CounterValue, float64(stats.MessagesReceived))
	c.emit(ch, c.broadcastsDesc, prometheus.CounterValue, float64(stats.BroadcastsSent))
	c.emit(ch, c.queuedMsgDesc, prometheus.GaugeValue, float64(stats.QueuedMessages))
	c.emit(ch, c.uptimeDesc, prometheus.GaugeValue, float64(stats.Uptime)/1000.0)
}

func (c *wscCollector) emit(ch chan<- prometheus.Metric, desc *prometheus.Desc, valueType prometheus.ValueType, val float64) {
	if m, err := prometheus.NewConstMetric(desc, valueType, val); err == nil {
		ch <- m
	}
}
