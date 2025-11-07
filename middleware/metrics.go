/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 15:03:22
 * @FilePath: \go-rpc-gateway\middleware\metrics.go
 * @Description: 监控和链路追踪管理
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	prometheusexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/trace"
)

// MetricsConfig 监控配置
type MetricsConfig struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	Path      string `json:"path" yaml:"path"`
	Namespace string `json:"namespace" yaml:"namespace"`
	Subsystem string `json:"subsystem" yaml:"subsystem"`
}

// TracingConfig 链路追踪配置
type TracingConfig struct {
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	ServiceName string `json:"serviceName" yaml:"serviceName"`
}

// MetricsManager 监控管理器
type MetricsManager struct {
	config *MetricsConfig

	// Prometheus 指标
	httpRequestsTotal   prometheus.Counter
	httpRequestDuration prometheus.Histogram
	grpcRequestsTotal   prometheus.Counter
	grpcRequestDuration prometheus.Histogram
}

// NewMetricsManager 创建监控管理器
func NewMetricsManager(config *MetricsConfig) (*MetricsManager, error) {
	if config == nil || !config.Enabled {
		return nil, nil
	}

	manager := &MetricsManager{
		config: config,
	}

	if err := manager.initMetrics(); err != nil {
		return nil, fmt.Errorf("failed to init metrics: %w", err)
	}

	return manager, nil
}

// initMetrics 初始化监控指标
func (m *MetricsManager) initMetrics() error {
	// 创建 Prometheus 指标
	m.httpRequestsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: m.config.Namespace,
		Subsystem: m.config.Subsystem,
		Name:      "http_requests_total",
		Help:      "Total number of HTTP requests",
	})

	m.httpRequestDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: m.config.Namespace,
		Subsystem: m.config.Subsystem,
		Name:      "http_request_duration_seconds",
		Help:      "Duration of HTTP requests in seconds",
		Buckets:   prometheus.DefBuckets,
	})

	m.grpcRequestsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: m.config.Namespace,
		Subsystem: m.config.Subsystem,
		Name:      "grpc_requests_total",
		Help:      "Total number of gRPC requests",
	})

	m.grpcRequestDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: m.config.Namespace,
		Subsystem: m.config.Subsystem,
		Name:      "grpc_request_duration_seconds",
		Help:      "Duration of gRPC requests in seconds",
		Buckets:   prometheus.DefBuckets,
	})

	// 注册指标
	prometheus.MustRegister(
		m.httpRequestsTotal,
		m.httpRequestDuration,
		m.grpcRequestsTotal,
		m.grpcRequestDuration,
	)

	return nil
}

// RecordHTTPRequest 记录 HTTP 请求
func (m *MetricsManager) RecordHTTPRequest(duration time.Duration) {
	if m == nil {
		return
	}
	m.httpRequestsTotal.Inc()
	m.httpRequestDuration.Observe(duration.Seconds())
}

// RecordGRPCRequest 记录 gRPC 请求
func (m *MetricsManager) RecordGRPCRequest(duration time.Duration) {
	if m == nil {
		return
	}
	m.grpcRequestsTotal.Inc()
	m.grpcRequestDuration.Observe(duration.Seconds())
}

// TracingManager 链路追踪管理器
type TracingManager struct {
	config *TracingConfig
	tracer trace.Tracer
	meter  metric.Meter
}

// NewTracingManager 创建链路追踪管理器
func NewTracingManager(config *TracingConfig) (*TracingManager, error) {
	if config == nil || !config.Enabled {
		return nil, nil
	}

	manager := &TracingManager{
		config: config,
	}

	if err := manager.initTracing(); err != nil {
		return nil, fmt.Errorf("failed to init tracing: %w", err)
	}

	return manager, nil
}

// initTracing 初始化链路追踪
func (m *TracingManager) initTracing() error {
	// 创建 Prometheus exporter
	exporter, err := prometheusexporter.New()
	if err != nil {
		return fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	// 创建 MeterProvider
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)
	otel.SetMeterProvider(provider)

	// 创建 tracer 和 meter
	m.tracer = otel.Tracer(m.config.ServiceName)
	m.meter = otel.Meter(m.config.ServiceName)

	return nil
}

// GetTracer 获取 tracer
func (m *TracingManager) GetTracer() trace.Tracer {
	if m == nil {
		return nil
	}
	return m.tracer
}

// GetMeter 获取 meter
func (m *TracingManager) GetMeter() metric.Meter {
	if m == nil {
		return nil
	}
	return m.meter
}
