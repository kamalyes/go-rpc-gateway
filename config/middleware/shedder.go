/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-01 16:05:12
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-01 21:28:30
 * @FilePath: \go-rpc-gateway\config\middleware\shedder.go
 * @Description: Shedder 自适应降载配置模块 - 服务端过载自保，三个信号为 CPU 使用率、飞行请求数与响应时间滑窗
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Shedder 自适应降载配置
//
// 时间字段统一带 -ms 后缀并以毫秒为单位，不沿用 breaker.CircuitBreaker.Timeout 的
// 「int64 纳秒」写法：纳秒值写进 YAML 只能是 5000000000 这种数字，既不可读也容易误配成秒
//
// [EN] Adaptive load-shedding configuration. Duration fields carry an explicit -ms suffix
// and are milliseconds rather than following breaker.CircuitBreaker.Timeout's
// int64-nanoseconds convention, because a nanosecond count is unreadable in YAML and easy to
// mistake for seconds
type Shedder struct {
	ModuleName     string   `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`             // 模块名称
	Enabled        bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                        // 是否启用降载
	CPUThreshold   int64    `mapstructure:"cpu-threshold" yaml:"cpu-threshold" json:"cpuThreshold"`       // CPU 使用率阈值，千分比 0-1000
	MaxConcurrency int64    `mapstructure:"max-concurrency" yaml:"max-concurrency" json:"maxConcurrency"` // 硬并发上限，0 表示不限
	WindowMS       int64    `mapstructure:"window-ms" yaml:"window-ms" json:"windowMs"`                   // 响应时间滑窗总长度，毫秒
	WindowBuckets  int      `mapstructure:"window-buckets" yaml:"window-buckets" json:"windowBuckets"`    // 滑窗桶数
	CooldownMS     int64    `mapstructure:"cooldown-ms" yaml:"cooldown-ms" json:"cooldownMs"`             // 降载冷却时长，毫秒
	MinRTMS        int64    `mapstructure:"min-rt-ms" yaml:"min-rt-ms" json:"minRtMs"`                    // 滑窗无样本时的兜底响应时间，毫秒
	FlyingBeta     float64  `mapstructure:"flying-beta" yaml:"flying-beta" json:"flyingBeta"`             // 飞行请求数 EWMA 平滑系数，取值 (0,1)
	ExcludePaths   []string `mapstructure:"exclude-paths" yaml:"exclude-paths" json:"excludePaths"`       // 排除的路径（前缀匹配）
}

// DefaultShedder 创建默认降载配置
//
// Enabled 默认 false，与 DefaultCircuitBreaker() 默认 true 不同：
// 降载会在服务还能响应的时候主动丢请求，阈值要结合容量压测结果调，默认开启等于
// 替使用方做了一个会丢流量的决定；熔断只在下游已经坏掉时才动手，默认开启是安全的
//
// ExcludePaths 默认排除健康检查与指标端点：前者被降载会让编排系统在服务过载时
// 直接重启或摘流，把一次本可自愈的过载变成重启风暴；后者被降载会让过载期间拿不到指标
//
// [EN] Returns the default configuration. Enabled is false, unlike DefaultCircuitBreaker():
// shedding actively drops requests while the service is still answering and its thresholds
// need capacity testing, so enabling it is a decision the operator must make, whereas a
// breaker only acts once the dependency is already broken. Health and metrics endpoints are
// excluded by default, since shedding health checks makes the orchestrator restart or evict
// the pod exactly when it is overloaded, and shedding metrics blinds the operator
func DefaultShedder() *Shedder {
	return &Shedder{
		ModuleName:     "shedder",
		Enabled:        false,
		CPUThreshold:   900, // 90%
		MaxConcurrency: 0,   // 不限
		WindowMS:       5000,
		WindowBuckets:  50, // 5000ms / 50 = 每桶 100ms
		CooldownMS:     1000,
		MinRTMS:        1000,
		FlyingBeta:     0.9,
		ExcludePaths:   []string{"/health", "/healthz", "/readyz", "/livez", "/metrics", "/debug"},
	}
}

// GetLabel 获取配置标签
func (s *Shedder) GetLabel() string {
	return s.ModuleName
}

// Init 初始化配置
func (s *Shedder) Init() error {
	return internal.ValidateStruct(s)
}

// Validate 验证配置
func (s *Shedder) Validate() error {
	return internal.ValidateStruct(s)
}

// Clone 返回配置的副本
func (s *Shedder) Clone() internal.Configurable {
	var cloned Shedder
	if err := syncx.DeepCopy(&cloned, s); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Shedder{}
	}
	return &cloned
}

// Enable 启用降载
func (s *Shedder) Enable() *Shedder {
	s.Enabled = true
	return s
}

// Disable 禁用降载
func (s *Shedder) Disable() *Shedder {
	s.Enabled = false
	return s
}

// IsEnabled 检查是否启用
func (s *Shedder) IsEnabled() bool {
	return s.Enabled
}

// Get 返回配置接口
func (s *Shedder) Get() interface{} {
	return s
}

// Set 设置配置数据
func (s *Shedder) Set(data interface{}) {
	if cfg, ok := data.(*Shedder); ok {
		*s = *cfg
	}
}

// WithModuleName 设置模块名称
func (s *Shedder) WithModuleName(moduleName string) *Shedder {
	s.ModuleName = moduleName
	return s
}

// WithCPUThreshold 设置 CPU 使用率阈值（千分比 0-1000）
func (s *Shedder) WithCPUThreshold(threshold int64) *Shedder {
	s.CPUThreshold = threshold
	return s
}

// WithMaxConcurrency 设置硬并发上限（0 表示不限）
func (s *Shedder) WithMaxConcurrency(maxConcurrency int64) *Shedder {
	s.MaxConcurrency = maxConcurrency
	return s
}

// WithWindow 设置响应时间滑窗的总长度（毫秒）与桶数
func (s *Shedder) WithWindow(windowMS int64, buckets int) *Shedder {
	s.WindowMS = windowMS
	s.WindowBuckets = buckets
	return s
}

// WithCooldownMS 设置降载冷却时长（毫秒）
func (s *Shedder) WithCooldownMS(cooldownMS int64) *Shedder {
	s.CooldownMS = cooldownMS
	return s
}

// WithMinRTMS 设置滑窗无样本时的兜底响应时间（毫秒）
func (s *Shedder) WithMinRTMS(minRTMS int64) *Shedder {
	s.MinRTMS = minRTMS
	return s
}

// WithFlyingBeta 设置飞行请求数 EWMA 平滑系数
func (s *Shedder) WithFlyingBeta(beta float64) *Shedder {
	s.FlyingBeta = beta
	return s
}

// WithExcludePaths 设置排除的路径（前缀匹配）
func (s *Shedder) WithExcludePaths(paths []string) *Shedder {
	s.ExcludePaths = paths
	return s
}
