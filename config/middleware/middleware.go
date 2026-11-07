/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-13 11:53:15
 * @FilePath: \go-rpc-gateway\config\middleware\middleware.go
 * @Description: 中间件配置模块
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Middleware 中间件配置
type Middleware struct {
	ModuleName     string          `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`             // 模块名称
	Enabled        bool            `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                        // 是否启用中间件
	Logging        *Logging        `mapstructure:"logging" yaml:"logging" json:"logging"`                        // 日志中间件
	Recovery       *Recovery       `mapstructure:"recovery" yaml:"recovery" json:"recovery"`                     // 恢复中间件
	Tracing        *Tracing        `mapstructure:"tracing" yaml:"tracing" json:"tracing"`                        // 追踪中间件
	Metrics        *Metrics        `mapstructure:"metrics" yaml:"metrics" json:"metrics"`                        // 指标中间件
	I18N           *I18N           `mapstructure:"i18n" yaml:"i18n" json:"i18n"`                                 // 国际化中间件
	PProf          *PProf          `mapstructure:"pprof" yaml:"pprof" json:"pprof"`                              // PProf中间件
	CircuitBreaker *CircuitBreaker `mapstructure:"circuit-breaker" yaml:"circuit-breaker" json:"circuitBreaker"` // 断路器配置
	Shedder        *Shedder        `mapstructure:"shedder" yaml:"shedder" json:"shedder"`                        // 自适应降载配置
	Signature      *Signature      `mapstructure:"signature" yaml:"signature" json:"signature"`                  // 签名验证中间件
}

// Default 创建默认中间件配置
func Default() *Middleware {
	return &Middleware{
		ModuleName:     "middleware",
		Enabled:        false,
		Logging:        DefaultLogging(),
		Recovery:       DefaultRecovery(),
		Tracing:        DefaultTracing(),
		Metrics:        DefaultMetrics(),
		I18N:           DefaultI18N(),
		PProf:          DefaultPProf(),
		CircuitBreaker: DefaultCircuitBreaker(),
		Shedder:        DefaultShedder(),
		Signature:      DefaultSignature(),
	}
}

// Get 返回配置接口
func (m *Middleware) Get() interface{} {
	return m
}

// Set 设置配置数据
func (m *Middleware) Set(data interface{}) {
	if cfg, ok := data.(*Middleware); ok {
		*m = *cfg
	}
}

// Clone 返回配置的副本
func (m *Middleware) Clone() internal.Configurable {
	var cloned Middleware
	if err := syncx.DeepCopy(&cloned, m); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Middleware{}
	}
	return &cloned
}

// Validate 验证配置
func (m *Middleware) Validate() error {
	return internal.ValidateStruct(m)
}

// WithModuleName 设置模块名称
func (m *Middleware) WithModuleName(moduleName string) *Middleware {
	m.ModuleName = moduleName
	return m
}

// WithEnabled 设置是否启用中间件
func (m *Middleware) WithEnabled(enabled bool) *Middleware {
	m.Enabled = enabled
	return m
}

// WithLogging 设置日志中间件配置
func (m *Middleware) WithLogging(logging *Logging) *Middleware {
	m.Logging = logging
	return m
}

// WithRecovery 设置恢复中间件配置
func (m *Middleware) WithRecovery(recovery *Recovery) *Middleware {
	m.Recovery = recovery
	return m
}

// WithTracing 设置追踪中间件配置
func (m *Middleware) WithTracing(tracing *Tracing) *Middleware {
	m.Tracing = tracing
	return m
}

// WithMetrics 设置指标中间件配置
func (m *Middleware) WithMetrics(metrics *Metrics) *Middleware {
	m.Metrics = metrics
	return m
}

// WithI18N 设置国际化中间件配置
func (m *Middleware) WithI18N(i18n *I18N) *Middleware {
	m.I18N = i18n
	return m
}

// WithPProf 设置PProf中间件配置
func (m *Middleware) WithPProf(pprof *PProf) *Middleware {
	m.PProf = pprof
	return m
}

// EnableLogging 启用日志中间件
func (m *Middleware) EnableLogging() *Middleware {
	if m.Logging != nil {
		m.Enable()
	}
	return m
}

// EnableRecovery 启用恢复中间件
func (m *Middleware) EnableRecovery() *Middleware {
	if m.Recovery != nil {
		m.Enable()
	}
	return m
}

// EnableTracing 启用追踪中间件
func (m *Middleware) EnableTracing() *Middleware {
	if m.Tracing != nil {
		m.Enable()
	}
	return m
}

// EnableMetrics 启用指标中间件
func (m *Middleware) EnableMetrics() *Middleware {
	if m.Metrics != nil {
		m.Enable()
	}
	return m
}

// EnableI18N 启用国际化中间件
func (m *Middleware) EnableI18N() *Middleware {
	if m.I18N != nil {
		m.Enable()
	}
	return m
}

// EnablePProf 启用PProf中间件
func (m *Middleware) EnablePProf() *Middleware {
	if m.PProf != nil {
		m.Enable()
	}
	return m
}

// Enable 启用中间件
func (m *Middleware) Enable() *Middleware {
	m.Enabled = true
	return m
}

// Disable 禁用中间件
func (m *Middleware) Disable() *Middleware {
	m.Enabled = false
	return m
}

// IsEnabled 检查是否启用
func (m *Middleware) IsEnabled() bool {
	return m.Enabled
}

// WithCircuitBreaker 设置断路器配置
func (m *Middleware) WithCircuitBreaker(circuitBreaker *CircuitBreaker) *Middleware {
	m.CircuitBreaker = circuitBreaker
	return m
}

// WithSignature 设置签名验证中间件配置
func (m *Middleware) WithSignature(signature *Signature) *Middleware {
	m.Signature = signature
	return m
}

// EnableCircuitBreaker 启用断路器
func (m *Middleware) EnableCircuitBreaker() *Middleware {
	if m.CircuitBreaker != nil {
		m.CircuitBreaker.Enable()
	}
	return m
}

// WithShedder 设置自适应降载配置
func (m *Middleware) WithShedder(shedder *Shedder) *Middleware {
	m.Shedder = shedder
	return m
}

// EnableShedder 启用自适应降载
func (m *Middleware) EnableShedder() *Middleware {
	if m.Shedder != nil {
		m.Enable()
	}
	return m
}

// EnableSignature 启用签名验证
func (m *Middleware) EnableSignature() *Middleware {
	if m.Signature != nil {
		m.Enable()
	}
	return m
}
