/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-11 18:00:00
 * @FilePath: \go-rpc-gateway\config\middleware\prometheus.go
 * @Description: Prometheus配置模块
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Prometheus Prometheus配置
type Prometheus struct {
	ModuleName  string       `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`    // 模块名称
	Enabled     bool         `mapstructure:"enabled" yaml:"enabled" json:"enabled"`               // 是否启用Prometheus
	Path        string       `mapstructure:"path" yaml:"path" json:"path"`                        // Prometheus路径
	Port        int          `mapstructure:"port" yaml:"port" json:"port"`                        // Prometheus端口
	Endpoint    string       `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint"`            // Prometheus端点
	PushGateway *PushGateway `mapstructure:"push-gateway" yaml:"push-gateway" json:"pushGateway"` // PushGateway配置
	Scraping    *Scraping    `mapstructure:"scraping" yaml:"scraping" json:"scraping"`            // 抓取配置
}

// PushGateway PushGateway配置
type PushGateway struct {
	Enabled  bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`    // 是否启用PushGateway
	Endpoint string `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint"` // PushGateway端点
	JobName  string `mapstructure:"job-name" yaml:"job-name" json:"jobName"`  // Job名称
}

// Scraping 抓取配置
type Scraping struct {
	Interval    string `mapstructure:"interval" yaml:"interval" json:"interval"`            // 抓取间隔
	Timeout     string `mapstructure:"timeout" yaml:"timeout" json:"timeout"`               // 超时时间
	MetricsPath string `mapstructure:"metrics-path" yaml:"metrics-path" json:"metricsPath"` // 指标路径
}

// DefaultPrometheus 创建默认Prometheus配置
func DefaultPrometheus() *Prometheus {
	return &Prometheus{
		ModuleName: "prometheus",
		Enabled:    false,
		Path:       "/metrics",
		Port:       9090,
		Endpoint:   "http://localhost:9090",
		PushGateway: &PushGateway{
			Enabled:  false,
			Endpoint: "http://localhost:9091",
			JobName:  "go-rpc-gateway",
		},
		Scraping: &Scraping{
			Interval:    "15s",
			Timeout:     "10s",
			MetricsPath: "/metrics",
		},
	}
}

// Get 返回配置接口
func (p *Prometheus) Get() interface{} {
	return p
}

// Set 设置配置数据
func (p *Prometheus) Set(data interface{}) {
	if cfg, ok := data.(*Prometheus); ok {
		*p = *cfg
	}
}

// Clone 返回配置的副本
func (p *Prometheus) Clone() internal.Configurable {
	var cloned Prometheus
	if err := syncx.DeepCopy(&cloned, p); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Prometheus{}
	}
	return &cloned
}

// Validate 验证配置
func (p *Prometheus) Validate() error {
	return internal.ValidateStruct(p)
}

// WithModuleName 设置模块名称
func (p *Prometheus) WithModuleName(moduleName string) *Prometheus {
	p.ModuleName = moduleName
	return p
}

// WithEnabled 设置是否启用
func (p *Prometheus) WithEnabled(enabled bool) *Prometheus {
	p.Enabled = enabled
	return p
}

// WithPath 设置路径
func (p *Prometheus) WithPath(path string) *Prometheus {
	p.Path = path
	return p
}

// WithPort 设置端口
func (p *Prometheus) WithPort(port int) *Prometheus {
	p.Port = port
	return p
}

// WithEndpoint 设置端点
func (p *Prometheus) WithEndpoint(endpoint string) *Prometheus {
	p.Endpoint = endpoint
	return p
}

// WithPushGateway 设置PushGateway配置
func (p *Prometheus) WithPushGateway(enabled bool, endpoint, jobName string) *Prometheus {
	if p.PushGateway == nil {
		p.PushGateway = &PushGateway{}
	}
	p.PushGateway.Enabled = enabled
	p.PushGateway.Endpoint = endpoint
	p.PushGateway.JobName = jobName
	return p
}

// WithScraping 设置抓取配置
func (p *Prometheus) WithScraping(interval, timeout, metricsPath string) *Prometheus {
	if p.Scraping == nil {
		p.Scraping = &Scraping{}
	}
	p.Scraping.Interval = interval
	p.Scraping.Timeout = timeout
	p.Scraping.MetricsPath = metricsPath
	return p
}

// EnablePushGateway 启用PushGateway
func (p *Prometheus) EnablePushGateway() *Prometheus {
	if p.PushGateway == nil {
		p.PushGateway = &PushGateway{}
	}
	p.PushGateway.Enabled = true
	return p
}

// Enable 启用Prometheus
func (p *Prometheus) Enable() *Prometheus {
	p.Enabled = true
	return p
}

// Disable 禁用Prometheus
func (p *Prometheus) Disable() *Prometheus {
	p.Enabled = false
	return p
}

// IsEnabled 检查是否启用
func (p *Prometheus) IsEnabled() bool {
	return p.Enabled
}
