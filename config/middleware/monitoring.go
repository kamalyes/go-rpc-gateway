/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-13 11:32:50
 * @FilePath: \go-rpc-gateway\config\middleware\monitoring.go
 * @Description: 监控配置模块 - 统一管理所有监控相关功能
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Monitoring 监控配置 - 去掉Config后缀
type Monitoring struct {
	ModuleName string             `mapstructure:"module-name" yaml:"module-name" json:"moduleName"` // 模块名称
	Enabled    bool               `mapstructure:"enabled" yaml:"enabled" json:"enabled"`            // 是否启用监控
	Prometheus *Prometheus        `mapstructure:"prometheus" yaml:"prometheus" json:"prometheus"`   // Prometheus配置
	Jaeger     *Jaeger            `mapstructure:"jaeger" yaml:"jaeger" json:"jaeger"`               // Jaeger配置
	Metrics    *MonitoringMetrics `mapstructure:"metrics" yaml:"metrics" json:"metrics"`            // 指标配置
}

// MonitoringMetrics 指标配置
type MonitoringMetrics struct {
	Enabled           bool           `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                                   // 是否启用指标
	RequestCount      bool           `mapstructure:"request-count" yaml:"request-count" json:"requestCount"`                  // 是否记录请求数
	Duration          bool           `mapstructure:"duration" yaml:"duration" json:"duration"`                                // 是否记录请求时长
	ResponseSize      bool           `mapstructure:"response-size" yaml:"response-size" json:"responseSize"`                  // 是否记录响应大小
	RequestSize       bool           `mapstructure:"request-size" yaml:"request-size" json:"requestSize"`                     // 是否记录请求大小
	Buckets           []float64      `mapstructure:"buckets" yaml:"buckets" json:"buckets"`                                   // 直方图桶配置
	EnableOpenMetrics bool           `mapstructure:"enable-open-metrics" yaml:"enable-open-metrics" json:"enableOpenMetrics"` // 是否启用 OpenMetrics 格式
	CustomMetrics     []CustomMetric `mapstructure:"custom-metrics" yaml:"custom-metrics" json:"customMetrics"`               // 自定义指标
	StaticPaths       []string       `mapstructure:"static-paths" yaml:"static-paths" json:"staticPaths"`                     // 静态路径列表（不进行路径规范化）
	Endpoint          string         `mapstructure:"-" yaml:"-" json:"-"`                                                     // 指标端点（自动计算）
}

// CustomMetric 自定义指标配置
type CustomMetric struct {
	Name       string    `mapstructure:"name" yaml:"name" json:"name"`                   // 指标名称
	Type       string    `mapstructure:"type" yaml:"type" json:"type"`                   // 指标类型 (counter, gauge, histogram, summary)
	Help       string    `mapstructure:"help" yaml:"help" json:"help"`                   // 帮助信息
	Labels     []string  `mapstructure:"labels" yaml:"labels" json:"labels"`             // 标签列表
	Buckets    []float64 `mapstructure:"buckets" yaml:"buckets" json:"buckets"`          // 直方图桶 (仅histogram类型)
	Objectives string    `mapstructure:"objectives" yaml:"objectives" json:"objectives"` // 分位数目标 (仅summary类型)
}

// DefaultStaticPaths 返回默认的静态路径列表
func DefaultStaticPaths() []string {
	return []string{
		// 根路径
		"/",
		// 健康检查
		"/health", "/healthz", "/health/live", "/health/ready",
		"/livez", "/readyz", "/ping", "/status",
		// 监控指标
		"/metrics", "/prometheus", "/actuator/prometheus",
		// 管理端点
		"/ready", "/alive", "/startup",
		// 调试端点
		"/debug/pprof", "/debug/vars",
		// API 文档
		"/swagger", "/swagger-ui", "/api-docs", "/docs",
		"/openapi.json", "/swagger.json",
		// 其他常见静态端点
		"/favicon.ico", "/robots.txt",
	}
}

// DefaultMonitoring 创建默认监控配置
func DefaultMonitoring() *Monitoring {
	m := &Monitoring{
		ModuleName: "monitoring",
		Enabled:    false,
		Prometheus: DefaultPrometheus(),
		Jaeger:     DefaultJaeger(),
		Metrics: &MonitoringMetrics{
			Enabled:           false,
			RequestCount:      true,
			Duration:          true,
			ResponseSize:      true,
			RequestSize:       true,
			Buckets:           []float64{0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120},
			EnableOpenMetrics: false,
			CustomMetrics:     []CustomMetric{},
			StaticPaths:       DefaultStaticPaths(),
		},
	}
	internal.CallAfterLoad(m) // 自动计算 Endpoint
	return m
}

// BeforeLoad 配置加载前的钩子
func (m *Monitoring) BeforeLoad() error {
	return nil
}

// AfterLoad 配置加载后的钩子 - 计算 Metrics Endpoint
func (m *Monitoring) AfterLoad() error {
	// 如果 Prometheus 配置存在且有 Path，使用它来计算 Metrics Endpoint
	if m.Prometheus != nil && m.Prometheus.Path != "" {
		m.Metrics.Endpoint = m.Prometheus.Path
	} else if m.Metrics != nil {
		// 否则使用默认的 /metrics
		m.Metrics.Endpoint = "/metrics"
	}
	return nil
}

// Get 返回配置接口
func (m *Monitoring) Get() interface{} {
	return m
}

// Set 设置配置数据
func (m *Monitoring) Set(data interface{}) {
	if cfg, ok := data.(*Monitoring); ok {
		*m = *cfg
	}
}

// Clone 返回配置的副本
func (m *Monitoring) Clone() internal.Configurable {
	var cloned Monitoring
	if err := syncx.DeepCopy(&cloned, m); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Monitoring{}
	}
	internal.CallAfterLoad(cloned) // 重新计算 Endpoint
	return &cloned
}

// Validate 验证配置
func (m *Monitoring) Validate() error {
	if err := internal.ValidateStruct(m); err != nil {
		return err
	}

	// 验证子模块
	if m.Prometheus != nil {
		if err := m.Prometheus.Validate(); err != nil {
			return err
		}
	}
	if m.Jaeger != nil {
		if err := m.Jaeger.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// WithModuleName 设置模块名称
func (m *Monitoring) WithModuleName(moduleName string) *Monitoring {
	m.ModuleName = moduleName
	return m
}

// WithEnabled 设置是否启用监控
func (m *Monitoring) WithEnabled(enabled bool) *Monitoring {
	m.Enabled = enabled
	return m
}

// WithPrometheus 设置 Prometheus 配置
func (m *Monitoring) WithPrometheus(p *Prometheus) *Monitoring {
	m.Prometheus = p
	internal.CallAfterLoad(m) // 重新计算 Endpoint
	return m
}

// WithJaeger 设置Jaeger配置
func (m *Monitoring) WithJaeger(cfg *Jaeger) *Monitoring {
	m.Jaeger = cfg
	return m
}

// WithMetrics 设置指标配置
func (m *Monitoring) WithMetrics(enabled, requestCount, duration, responseSize, requestSize bool) *Monitoring {
	if m.Metrics == nil {
		m.Metrics = &MonitoringMetrics{}
	}
	m.Metrics.Enabled = enabled
	m.Metrics.RequestCount = requestCount
	m.Metrics.Duration = duration
	m.Metrics.ResponseSize = responseSize
	m.Metrics.RequestSize = requestSize
	return m
}

// AddCustomMetric 添加自定义指标
func (m *Monitoring) AddCustomMetric(name, metricType, help string, labels []string) *Monitoring {
	if m.Metrics == nil {
		m.Metrics = &MonitoringMetrics{}
	}
	m.Metrics.CustomMetrics = append(m.Metrics.CustomMetrics, CustomMetric{
		Name:   name,
		Type:   metricType,
		Help:   help,
		Labels: labels,
	})
	return m
}

// EnablePrometheus 启用Prometheus
func (m *Monitoring) EnablePrometheus() *Monitoring {
	if m.Prometheus != nil {
		m.Prometheus.Enable()
	}
	return m
}

// EnableJaeger 启用Jaeger
func (m *Monitoring) EnableJaeger() *Monitoring {
	if m.Jaeger != nil {
		m.Jaeger.Enable()
	}
	return m
}

// EnableMetrics 启用指标
func (m *Monitoring) EnableMetrics() *Monitoring {
	if m.Metrics == nil {
		m.Metrics = &MonitoringMetrics{}
	}
	m.Metrics.Enabled = true
	return m
}

// Enable 启用监控
func (m *Monitoring) Enable() *Monitoring {
	m.Enabled = true
	return m
}

// Disable 禁用监控
func (m *Monitoring) Disable() *Monitoring {
	m.Enabled = false
	return m
}

// IsEnabled 检查是否启用
func (m *Monitoring) IsEnabled() bool {
	return m.Enabled
}

// GetEndpoint 获取 Metrics 的 Endpoint（确保已计算）
func (m *Monitoring) GetEndpoint() string {
	if m.Metrics != nil && m.Metrics.Endpoint == "" {
		internal.CallAfterLoad(m)
	}
	if m.Metrics != nil {
		return m.Metrics.Endpoint
	}
	return "/metrics" // 默认值
}
