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
	"fmt"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/kamalyes/go-config/pkg/monitoring"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"net/http"
	"time"
)

// MetricsManager 可观测性管理器 - 统一管理 Prometheus 指标（HTTP + gRPC）
type MetricsManager struct {
	registry      *prometheus.Registry
	serverMetrics *grpc_prometheus.ServerMetrics
	clientMetrics *grpc_prometheus.ClientMetrics
	httpMetrics   *HTTPMetrics
	panicCounter  prometheus.Counter
	config        *monitoring.Monitoring
}

// HTTPMetrics HTTP 请求指标
type HTTPMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestSize     *prometheus.SummaryVec
	responseSize    *prometheus.SummaryVec
	activeRequests  prometheus.Gauge
}

// NewMetricsManager 创建可观测性管理器（支持 gRPC + HTTP 完整指标）
// go-config 的 Default() 已经设置了所有默认值（包括 Buckets），无需再次设置
func NewMetricsManager(cfg *monitoring.Monitoring) *MetricsManager {
	if !cfg.Metrics.Enabled {
		return nil
	}

	registry := prometheus.NewRegistry()

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

	// 创建 HTTP 指标
	httpMetrics := newHTTPMetrics(registry, buckets)

	mm := &MetricsManager{
		registry:      registry,
		serverMetrics: serverMetrics,
		clientMetrics: clientMetrics,
		httpMetrics:   httpMetrics,
		panicCounter:  panicCounter,
		config:        cfg,
	}

	if global.LOGGER != nil {
		global.LOGGER.InfoMsg("Prometheus可观测性管理器已初始化")
	}
	return mm
}

// newHTTPMetrics 创建 HTTP 指标
func newHTTPMetrics(registry *prometheus.Registry, buckets []float64) *HTTPMetrics {
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
	}
}

// RecordHTTPRequest 记录 HTTP 请求（使用详细指标）
func (mm *MetricsManager) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration, requestSize, responseSize int64) {
	if mm == nil || mm.httpMetrics == nil {
		return
	}

	// 记录请求总数
	mm.httpMetrics.requestsTotal.WithLabelValues(method, path, http.StatusText(statusCode)).Inc()

	// 记录请求持续时间
	mm.httpMetrics.requestDuration.WithLabelValues(method, path).Observe(duration.Seconds())

	// 记录请求大小
	if requestSize > 0 {
		mm.httpMetrics.requestSize.WithLabelValues(method, path).Observe(float64(requestSize))
	}

	// 记录响应大小
	if responseSize > 0 {
		mm.httpMetrics.responseSize.WithLabelValues(method, path).Observe(float64(responseSize))
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
func ExemplarFromContext(ctx interface{}) prometheus.Labels {
	if spanCtx, ok := ctx.(trace.SpanContext); ok && spanCtx.IsSampled() {
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

			// 记录请求大小
			if r.ContentLength > 0 {
				mm.httpMetrics.requestSize.WithLabelValues(r.Method, r.URL.Path).Observe(float64(r.ContentLength))
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
			mm.httpMetrics.requestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)

			// 记录请求总数
			mm.httpMetrics.requestsTotal.WithLabelValues(
				r.Method,
				r.URL.Path,
				http.StatusText(wrapped.statusCode),
			).Inc()

			// 记录响应大小
			if wrapped.bytesWritten > 0 {
				mm.httpMetrics.responseSize.WithLabelValues(r.Method, r.URL.Path).Observe(float64(wrapped.bytesWritten))
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
func HTTPTracingMiddleware(tracingManager *TracingManager) HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		if tracingManager == nil || tracingManager.GetTracer() == nil {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := tracingManager.GetTracer().Start(r.Context(), fmt.Sprintf("%s %s", r.Method, r.URL.Path))
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

		ctx, span := tracingManager.GetTracer().Start(ctx, info.FullMethod)
		defer span.End()

		// 添加属性
		span.SetAttributes(
			attribute.String(constants.TracingAttrRPCSystem, "grpc"),
			attribute.String(constants.TracingAttrRPCMethod, info.FullMethod),
		)

		// 从 metadata 中获取额外信息
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if userAgent := md.Get("user-agent"); len(userAgent) > 0 {
				span.SetAttributes(attribute.String("user_agent", userAgent[0]))
			}
		}

		// 处理请求
		return handler(ctx, req)
	}
}
