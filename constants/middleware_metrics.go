/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:40:10
 * @FilePath: \go-rpc-gateway\constants\middleware_metrics.go
 * @Description: 监控中间件相关常量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

// Metrics 默认配置
const (
	// 默认是否启用
	MetricsDefaultEnabled = true

	// 默认命名空间
	MetricsDefaultNamespace = "go_rpc_gateway"

	// 默认子系统
	MetricsDefaultSubsystem = "http"

	// 默认路径
	MetricsDefaultPath = "/metrics"
)

// 内置指标名称
const (
	// HTTP 请求总数
	MetricsHTTPRequestsTotal = "http_requests_total"

	// HTTP 请求持续时间
	MetricsHTTPRequestDuration = "http_request_duration_seconds"

	// HTTP 请求大小
	MetricsHTTPRequestSize = "http_request_size_bytes"

	// HTTP 响应大小
	MetricsHTTPResponseSize = "http_response_size_bytes"

	// 当前处理中的请求数
	MetricsHTTPRequestsInFlight = "http_requests_in_flight"

	// gRPC 请求总数
	MetricsGRPCRequestsTotal = "grpc_requests_total"

	// gRPC 请求持续时间
	MetricsGRPCRequestDuration = "grpc_request_duration_seconds"
)

// 标签常量
const (
	// HTTP方法标签
	MetricsLabelMethod = "method"

	// 路径标签
	MetricsLabelPath = "path"

	// 状态码标签
	MetricsLabelStatusCode = "status_code"

	// 服务标签
	MetricsLabelService = "service"

	// 版本标签
	MetricsLabelVersion = "version"

	// 环境标签
	MetricsLabelEnvironment = "environment"

	// 实例标签
	MetricsLabelInstance = "instance"
)

// 监控类型
const (
	MetricsTypeCounter   = "counter"
	MetricsTypeGauge     = "gauge"
	MetricsTypeHistogram = "histogram"
	MetricsTypeSummary   = "summary"
)

// 默认直方图桶
var MetricsDefaultHistogramBuckets = []float64{
	0.001, 0.01, 0.1, 0.3, 1.2, 5, 10, 30, 60, 300, 600, 1800, 3600,
}

// 默认摘要分位数
var MetricsDefaultSummaryQuantiles = map[float64]float64{
	0.5:  0.05,
	0.9:  0.01,
	0.99: 0.001,
}

// 指标收集间隔
const (
	MetricsCollectionIntervalFast   = "1s"  // 快速收集
	MetricsCollectionIntervalNormal = "10s" // 正常收集
	MetricsCollectionIntervalSlow   = "60s" // 慢速收集
)

// 默认跳过监控的路径
var MetricsDefaultSkipPaths = []string{
	DefaultHealthPath,
	DefaultMetricsPath,
	DefaultDebugPath,
	PProfBasePath,
}
