/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-11 18:00:00
 * @FilePath: \go-rpc-gateway\config\middleware\metrics.go
 * @Description: 指标中间件配置模块
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Metrics 指标中间件配置
type Metrics struct {
	ModuleName   string    `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`       // 模块名称
	Enabled      bool      `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                  // 是否启用指标
	Path         string    `mapstructure:"path" yaml:"path" json:"path"`                           // 指标路径
	Namespace    string    `mapstructure:"namespace" yaml:"namespace" json:"namespace"`            // 命名空间
	Subsystem    string    `mapstructure:"subsystem" yaml:"subsystem" json:"subsystem"`            // 子系统名称
	SkipPaths    []string  `mapstructure:"skip-paths" yaml:"skip-paths" json:"skipPaths"`          // 跳过的路径
	Buckets      []float64 `mapstructure:"buckets" yaml:"buckets" json:"buckets"`                  // 直方图桶
	RequestCount bool      `mapstructure:"request-count" yaml:"request-count" json:"requestCount"` // 请求计数
	Duration     bool      `mapstructure:"duration" yaml:"duration" json:"duration"`               // 请求时长
	RequestSize  bool      `mapstructure:"request-size" yaml:"request-size" json:"requestSize"`    // 请求大小
	ResponseSize bool      `mapstructure:"response-size" yaml:"response-size" json:"responseSize"` // 响应大小
}

// DefaultMetrics 创建默认指标配置
func DefaultMetrics() *Metrics {
	return &Metrics{
		ModuleName:   "metrics",
		Enabled:      false,
		Path:         "/metrics",
		Namespace:    "gateway",
		Subsystem:    "http",
		SkipPaths:    []string{"/health"},
		Buckets:      []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		RequestCount: true,
		Duration:     true,
		RequestSize:  true,
		ResponseSize: true,
	}
}

// Get 返回配置接口
func (m *Metrics) Get() interface{} {
	return m
}

// Set 设置配置数据
func (m *Metrics) Set(data interface{}) {
	if cfg, ok := data.(*Metrics); ok {
		*m = *cfg
	}
}

// Clone 返回配置的副本
func (m *Metrics) Clone() internal.Configurable {
	var cloned Metrics
	if err := syncx.DeepCopy(&cloned, m); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Metrics{}
	}
	return &cloned
}

// Validate 验证配置
func (m *Metrics) Validate() error {
	return internal.ValidateStruct(m)
}

// WithPath 设置指标路径
func (m *Metrics) WithPath(path string) *Metrics {
	m.Path = path
	return m
}

// WithNamespace 设置命名空间
func (m *Metrics) WithNamespace(namespace string) *Metrics {
	m.Namespace = namespace
	return m
}

// WithSubsystem 设置子系统名称
func (m *Metrics) WithSubsystem(subsystem string) *Metrics {
	m.Subsystem = subsystem
	return m
}

// WithSkipPaths 设置跳过的路径
func (m *Metrics) WithSkipPaths(skipPaths []string) *Metrics {
	m.SkipPaths = skipPaths
	return m
}

// WithBuckets 设置直方图桶
func (m *Metrics) WithBuckets(buckets []float64) *Metrics {
	m.Buckets = buckets
	return m
}

// Enable 启用指标中间件
func (m *Metrics) Enable() *Metrics {
	m.Enabled = true
	return m
}

// Disable 禁用指标中间件
func (m *Metrics) Disable() *Metrics {
	m.Enabled = false
	return m
}

// IsEnabled 检查是否启用
func (m *Metrics) IsEnabled() bool {
	return m.Enabled
}
