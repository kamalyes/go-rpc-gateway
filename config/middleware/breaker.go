/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12
 * @FilePath: \go-rpc-gateway\config\middleware\breaker.go
 * @Description: CircuitBreaker 断路器配置模块
 * 提供统一的断路器配置管理，支持全局和路径级别的配置
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"time"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// CircuitBreaker 断路器配置
type CircuitBreaker struct {
	ModuleName          string   `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`                              // 模块名称
	Enabled             bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                                         // 是否启用断路器
	FailureThreshold    int      `mapstructure:"failure-threshold" yaml:"failure-threshold" json:"failureThreshold"`            // 失败阈值
	SuccessThreshold    int      `mapstructure:"success-threshold" yaml:"success-threshold" json:"successThreshold"`            // 成功阈值
	Timeout             int64    `mapstructure:"timeout" yaml:"timeout" json:"timeout"`                                         // 熔断后恢复时间
	VolumeThreshold     int      `mapstructure:"volume-threshold" yaml:"volume-threshold" json:"volumeThreshold"`               // 最小请求量阈值
	SlidingWindowSize   int      `mapstructure:"sliding-window-size" yaml:"sliding-window-size" json:"slidingWindowSize"`       // 滑动窗口大小
	SlidingWindowBucket int64    `mapstructure:"sliding-window-bucket" yaml:"sliding-window-bucket" json:"slidingWindowBucket"` // 滑动窗口桶大小
	PreventionPaths     []string `mapstructure:"prevention-paths" yaml:"prevention-paths" json:"preventionPaths"`               // 需要保护的路径
	ExcludePaths        []string `mapstructure:"exclude-paths" yaml:"exclude-paths" json:"excludePaths"`                        // 排除的路径
}

// WebSocketBreaker WebSocket 专用断路器配置
type WebSocketBreaker struct {
	ModuleName          string  `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`                              // 模块名称
	Enabled             bool    `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                                         // 是否启用
	FailureThreshold    int     `mapstructure:"failure-threshold" yaml:"failure-threshold" json:"failureThreshold"`            // 失败阈值
	SuccessThreshold    int     `mapstructure:"success-threshold" yaml:"success-threshold" json:"successThreshold"`            // 成功阈值
	Timeout             int64   `mapstructure:"timeout" yaml:"timeout" json:"timeout"`                                         // 熔断恢复时间
	MaxRetries          int     `mapstructure:"max-retries" yaml:"max-retries" json:"maxRetries"`                              // 最大重试次数
	RetryBackoffFactor  float64 `mapstructure:"retry-backoff-factor" yaml:"retry-backoff-factor" json:"retryBackoffFactor"`    // 重试退避因子
	HealthCheckInterval int64   `mapstructure:"health-check-interval" yaml:"health-check-interval" json:"healthCheckInterval"` // 健康检查间隔
	MessageQueueSize    int     `mapstructure:"message-queue-size" yaml:"message-queue-size" json:"messageQueueSize"`          // 消息队列大小
}

// DefaultCircuitBreaker 创建默认断路器配置
func DefaultCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		ModuleName:          "circuit_breaker",
		Enabled:             true,
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             int64(30 * time.Second),
		VolumeThreshold:     10,
		SlidingWindowSize:   100,
		SlidingWindowBucket: int64(1 * time.Second),
		PreventionPaths:     []string{"/api/"},
		ExcludePaths:        []string{"/health", "/metrics", "/debug"},
	}
}

// DefaultWebSocketBreaker 创建默认 WebSocket 断路器配置
func DefaultWebSocketBreaker() *WebSocketBreaker {
	return &WebSocketBreaker{
		ModuleName:          "websocket_breaker",
		Enabled:             true,
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:             int64(30 * time.Second),
		MaxRetries:          3,
		RetryBackoffFactor:  2.0,
		HealthCheckInterval: int64(10 * time.Second),
		MessageQueueSize:    1000,
	}
}

// GetLabel 获取配置标签
func (cb *CircuitBreaker) GetLabel() string {
	return cb.ModuleName
}

// Init 初始化配置
func (cb *CircuitBreaker) Init() error {
	return internal.ValidateStruct(cb)
}

// Validate 验证配置
func (cb *CircuitBreaker) Validate() error {
	return internal.ValidateStruct(cb)
}

// GetLabel 获取配置标签
func (wb *WebSocketBreaker) GetLabel() string {
	return wb.ModuleName
}

// Init 初始化配置
func (wb *WebSocketBreaker) Init() error {
	return internal.ValidateStruct(wb)
}

// Validate 验证配置
func (wb *WebSocketBreaker) Validate() error {
	return internal.ValidateStruct(wb)
}

// Clone 返回配置的副本
func (cb *CircuitBreaker) Clone() internal.Configurable {
	var cloned CircuitBreaker
	if err := syncx.DeepCopy(&cloned, cb); err != nil {
		// 如果深拷贝失败，返回空配置
		return &CircuitBreaker{}
	}
	return &cloned
}

// Enable 启用断路器
func (cb *CircuitBreaker) Enable() *CircuitBreaker {
	cb.Enabled = true
	return cb
}

// Disable 禁用断路器
func (cb *CircuitBreaker) Disable() *CircuitBreaker {
	cb.Enabled = false
	return cb
}

// IsEnabled 检查是否启用
func (cb *CircuitBreaker) IsEnabled() bool {
	return cb.Enabled
}

// Get 返回配置接口
func (cb *CircuitBreaker) Get() interface{} {
	return cb
}

// Set 设置配置数据
func (cb *CircuitBreaker) Set(data interface{}) {
	if cfg, ok := data.(*CircuitBreaker); ok {
		*cb = *cfg
	}
}

// Clone 返回 WebSocket 断路器配置的副本
func (wb *WebSocketBreaker) Clone() internal.Configurable {
	var cloned WebSocketBreaker
	if err := syncx.DeepCopy(&cloned, wb); err != nil {
		// 如果深拷贝失败，返回空配置
		return &WebSocketBreaker{}
	}
	return &cloned
}

// Enable 启用 WebSocket 断路器
func (wb *WebSocketBreaker) Enable() *WebSocketBreaker {
	wb.Enabled = true
	return wb
}

// Disable 禁用 WebSocket 断路器
func (wb *WebSocketBreaker) Disable() *WebSocketBreaker {
	wb.Enabled = false
	return wb
}

// IsEnabled 检查是否启用
func (wb *WebSocketBreaker) IsEnabled() bool {
	return wb.Enabled
}

// Get 返回配置接口
func (wb *WebSocketBreaker) Get() interface{} {
	return wb
}

// Set 设置配置数据
func (wb *WebSocketBreaker) Set(data interface{}) {
	if cfg, ok := data.(*WebSocketBreaker); ok {
		*wb = *cfg
	}
}
