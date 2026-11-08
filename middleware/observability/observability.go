/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-07 21:31:11
 * @FilePath: \go-rpc-gateway\middleware\observability\observability.go
 * @Description: 可观测性中间件 - 完整的监控、追踪、指标管理
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package observability

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	gcmiddleware "github.com/kamalyes/go-rpc-gateway/config/middleware"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware/core"
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

// MetricsManager 可观测性管理器 - 统一管理 Prometheus 指标（HTTP + gRPC）
type MetricsManager struct {
	registry      *prometheus.Registry
	serverMetrics *grpc_prometheus.ServerMetrics
	clientMetrics *grpc_prometheus.ClientMetrics
	httpMetrics   *HTTPMetrics
	panicCounter  prometheus.Counter
	config        *gcmiddleware.Monitoring

	// 扩展业务指标（通过函数注入采集逻辑，避免包循环依赖）
	rateLimitRejected *prometheus.CounterVec // 限流拒绝计数器

	// 依赖采集函数（由 server 包在初始化后注入，按需在 scrape 时调用）
	poolHealthFn   func() map[string]bool // 连接池健康检查采集函数
	breakerStatsFn func() []BreakerStat   // 熔断器统计采集函数
}

// HTTPMetrics HTTP 请求指标
type HTTPMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestSize     *prometheus.SummaryVec
	responseSize    *prometheus.SummaryVec
	activeRequests  prometheus.Gauge
	pathNormalizer  core.PathNormalizer // 路径规范化器，减少标签基数
}

// registerProcessCollector 注册 process 采集器，并在该平台不支持时撤回
//
// process 采集器读 /proc，只在 Linux 上可用 配 ReportErrors: true 时，
// 不支持的平台会让它发出一条 invalid metric，于是整个 registry 的 Gather 失败，
// promhttp 把 /metrics 答成 500 —— runtime 指标和业务指标也一起拿不到
// macOS 上开发时这个端点是坏的，而 CI 只跑 ubuntu，所以一直没暴露
//
// 这里的做法是注册后试探一次 Gather：成功就保留（支持的平台上 ReportErrors 继续生效，
// 权限之类的真问题照样报出来），失败就撤回这个采集器并记一条警告，
// 让 /metrics 退化为「没有 process 指标」而不是「整个端点 500」
// 不按 GOOS 硬编码，是因为同一个系统上也可能因权限或容器限制读不到
//
// [EN] registerProcessCollector registers the process collector and withdraws it where
// the platform does not support it
// The collector reads /proc and only works on Linux; with ReportErrors: true an
// unsupported platform makes it emit an invalid metric, which fails Gather for the whole
// registry and turns /metrics into a 500, taking runtime and business metrics down with it
// This was invisible because CI runs only on ubuntu while development happens on macOS
// A trial Gather decides: on success the collector stays, so genuine problems such as
// permissions are still reported; on failure it is withdrawn with a warning, degrading
// /metrics to "no process metrics" rather than a dead endpoint
// GOOS is deliberately not hardcoded, since permissions or container limits can block
// /proc on a supported system too
func registerProcessCollector(registry *prometheus.Registry) {
	collector := prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{
		Namespace:    "",
		ReportErrors: true,
	})
	registry.MustRegister(collector)

	if _, err := registry.Gather(); err != nil {
		registry.Unregister(collector)
		if global.LOGGER != nil {
			global.LOGGER.WarnKV("process 指标在本平台不可用，已跳过该采集器",
				"error", err.Error())
		}
	}
}

// NewMetricsManager 创建可观测性管理器（支持 gRPC + HTTP 完整指标）
// go-config 的 Default() 已经设置了所有默认值（包括 Buckets），无需再次设置
func NewMetricsManager(cfg *gcmiddleware.Monitoring) *MetricsManager {
	if !cfg.Metrics.Enabled {
		return nil
	}

	registry := prometheus.NewRegistry()

	// 注册 Go runtime 和 process 默认采集器，保留 runtime/process 指标
	registry.MustRegister(prometheus.NewGoCollector())
	registerProcessCollector(registry)

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
		pathNormalizer: core.NewSmartPathNormalizer(staticPaths),
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
func HTTPMetricsMiddleware(m *MetricsManager) core.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m == nil || m.httpMetrics == nil {
				next.ServeHTTP(w, r)
				return
			}

			// 增加活跃请求计数
			m.httpMetrics.activeRequests.Inc()
			defer m.httpMetrics.activeRequests.Dec()

			// 包装 ResponseWriter（对象池复用）
			wrapped := acquireMetricsResponseWriter(w)
			defer releaseMetricsResponseWriter(wrapped)

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

			// 包装 ResponseWriter 以捕获状态码和响应大小（对象池复用）
			wrapped := acquireMetricsResponseWriter(w)
			defer releaseMetricsResponseWriter(wrapped)

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
//
// Flusher / Hijacker / Pusher 必须逐个显式实现，嵌入 http.ResponseWriter 接口
// 只会提升 Header / Write / WriteHeader 三个方法，包装之后 w.(http.Flusher)
// 断言即不再成立 —— 与 server/http.go 里 gzipResponseWriter 同一个坑
//
// 这一层在中间件链上处于 Logging 与 RateLimit 之间（见 middleware/manager.go
// GetMiddlewares 第 6 步），而链上其它包装器（core.ResponseWriter）的 Flush 是
// 逐层往内委托的：中间夹一层不实现 http.Flusher 的包装器，整条委托链就在这里断掉
//  1. 流式响应（SSE、grpc-gateway 的 server-streaming）静默不再刷新，数据一路攒到 handler 返回才整段到达客户端
//  2. grpc-gateway 的 ForwardResponseStream 第一步就是 w.(http.Flusher) 断言，它是最内层时断言失败直接回 500 "unexpected type of web server"
//  3. WebSocket 升级要裸连接，Hijack 断言失败即 http.ErrNotSupported，升级彻底失败
//
// 三者都只在「指标开启」时出现，而生产模式默认开启指标
//
// [EN] Flusher, Hijacker and Pusher must each be implemented explicitly: embedding the
// http.ResponseWriter interface promotes only Header, Write and WriteHeader, so
// w.(http.Flusher) stops asserting once wrapped — the same trap documented on
// gzipResponseWriter in server/http.go.
// This layer sits between Logging and RateLimit in the chain, and the Flush of the other
// wrappers (core.ResponseWriter) delegates inward layer by layer, so a layer that is not
// an http.Flusher breaks that delegation chain here: streaming responses silently stop
// flushing, grpc-gateway's ForwardResponseStream answers 500 when this is the innermost
// writer, and WebSocket upgrades fail outright with http.ErrNotSupported. All three appear
// only when metrics are enabled, which production mode does by default.
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

// metricsResponseWriterPool 响应包装器对象池（热路径每请求一次分配 → 复用）
var metricsResponseWriterPool = sync.Pool{
	New: func() any {
		return &metricsResponseWriter{}
	},
}

// acquireMetricsResponseWriter 从池中获取包装器并绑定底层 writer
func acquireMetricsResponseWriter(w http.ResponseWriter) *metricsResponseWriter {
	rw := metricsResponseWriterPool.Get().(*metricsResponseWriter)
	rw.ResponseWriter = w
	rw.statusCode = http.StatusOK
	rw.bytesWritten = 0
	return rw
}

// releaseMetricsResponseWriter 归还包装器到池（解除底层 writer 引用，防止泄漏）
func releaseMetricsResponseWriter(rw *metricsResponseWriter) {
	rw.ResponseWriter = nil
	metricsResponseWriterPool.Put(rw)
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

// Flush 实现 http.Flusher 接口（流式响应、SSE、server-streaming）
func (rw *metricsResponseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack 实现 http.Hijacker 接口（WebSocket 升级需要裸连接）
func (rw *metricsResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Push 实现 http.Pusher 接口（HTTP/2 Server Push）
func (rw *metricsResponseWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := rw.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

// Unwrap 返回底层的 http.ResponseWriter（供 http.ResponseController 使用）
func (rw *metricsResponseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// ============================================================================
// 可观测性中间件 - HTTP & gRPC 拦截器
// ============================================================================

// HTTPTracingMiddleware HTTP 链路追踪中间件
// Deprecated: 此实现缺少 W3C traceparent 传播提取/注入及 trace_id 同步，
// 请使用 tracing.go 中的 Tracing() 中间件（Manager.HTTPTracingMiddleware 已正确委托）。
func HTTPTracingMiddleware(tracingManager *TracingManager) core.HTTPMiddleware {
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
func GRPCMetricsInterceptor(metricsManager *MetricsManager) core.GRPCInterceptor {
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
func GRPCTracingInterceptor(tracingManager *TracingManager) core.GRPCInterceptor {
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
					ctx = core.WithTraceID(ctx, otelTraceID)
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
					ctx = core.WithTraceID(ctx, spanCtx.TraceID().String())
				}
			}
		}

		// 包装 ServerStream 以使用增强后的 context
		wrappedStream := core.NewContextWrappedServerStream(ss, ctx)

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
