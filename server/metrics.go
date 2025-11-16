/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 00:00:00
 * @FilePath: \go-rpc-gateway\server\metrics.go
 * @Description: Prometheus 指标管理器
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"net/http"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	goconfig "github.com/kamalyes/go-config"
	"github.com/kamalyes/go-config/pkg/monitoring"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/trace"
)

// MetricsManager Prometheus 指标管理器
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

// NewMetricsManager 创建指标管理器
// go-config 的 Default() 已经设置了所有默认值（包括 Buckets），无需再次设置
func NewMetricsManager(cfg *monitoring.Monitoring) *MetricsManager {
	// 使用SafeConfig安全访问
	configSafe := goconfig.SafeConfig(cfg)
	if cfg == nil || !configSafe.Metrics().Enabled(false) {
		return nil
	}

	registry := prometheus.NewRegistry()

	// 安全获取直方图桶（如果无法安全访问则使用默认值）
	var buckets []float64
	if cfg.Metrics != nil && len(cfg.Metrics.Buckets) > 0 {
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

	global.LOGGER.InfoMsg("Prometheus指标管理器已初始化")
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
	opts := promhttp.HandlerOpts{}

	if mm.config != nil {
		// 使用SafeConfig安全访问
		configSafe := goconfig.SafeConfig(mm.config)
		if configSafe.Metrics().Field("EnableOpenMetrics").Bool(false) {
			opts.EnableOpenMetrics = true
		}
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
			wrapped := &responseWriter{
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

// responseWriter 包装 http.ResponseWriter 以捕获状态码和写入字节数
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}
