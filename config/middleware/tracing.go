/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-11 18:00:00
 * @FilePath: \go-rpc-gateway\config\middleware\tracing.go
 * @Description: 追踪中间件配置模块
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Tracing 追踪中间件配置
type Tracing struct {
	ModuleName          string            `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`                              // 模块名称
	Enabled             bool              `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                                         // 是否启用追踪
	ServiceName         string            `mapstructure:"service-name" yaml:"service-name" json:"serviceName"`                           // 服务名称
	ServiceVersion      string            `mapstructure:"service-version" yaml:"service-version" json:"serviceVersion"`                  // 服务版本
	Environment         string            `mapstructure:"environment" yaml:"environment" json:"environment"`                             // 环境
	Endpoint            string            `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint"`                                      // 追踪端点
	ExporterType        string            `mapstructure:"exporter-type" yaml:"exporter-type" json:"exporterType"`                        // 导出器类型 (zipkin, otlp, console, noop)
	ExporterEndpoint    string            `mapstructure:"exporter-endpoint" yaml:"exporter-endpoint" json:"exporterEndpoint"`            // 导出器端点
	SampleRate          float64           `mapstructure:"sample-rate" yaml:"sample-rate" json:"sampleRate"`                              // 采样率
	SamplerType         string            `mapstructure:"sampler-type" yaml:"sampler-type" json:"samplerType"`                           // 采样器类型 (always, never, probability, parentBased)
	SamplerProbability  float64           `mapstructure:"sampler-probability" yaml:"sampler-probability" json:"samplerProbability"`      // 采样概率
	SamplerRate         float64           `mapstructure:"sampler-rate" yaml:"sampler-rate" json:"samplerRate"`                           // 采样速率
	Headers             []string          `mapstructure:"headers" yaml:"headers" json:"headers"`                                         // 需要记录的头部
	Attributes          map[string]string `mapstructure:"attributes" yaml:"attributes" json:"attributes"`                                // 自定义属性
	ExporterHeaders     map[string]string `mapstructure:"exporter-headers" yaml:"exporter-headers" json:"exporterHeaders"`               // 导出器自定义请求头 (如 Authorization、stream-name)
	ExporterTLSInsecure bool              `mapstructure:"exporter-tls-insecure" yaml:"exporter-tls-insecure" json:"exporterTLSInsecure"` // 导出器是否跳过 TLS 验证
	TelemetryLogLevel   string            `mapstructure:"telemetry-log-level" yaml:"telemetry-log-level" json:"telemetryLogLevel"`       // 遥测日志级别 (debug/info/warn/error)
}

// DefaultTracing 创建默认追踪配置
func DefaultTracing() *Tracing {
	return &Tracing{
		ModuleName:          "tracing",
		Enabled:             false,
		ServiceName:         "go-rpc-gateway",
		ServiceVersion:      "1.0.0",
		Environment:         "development",
		Endpoint:            "http://localhost:9411/api/v2/spans",
		ExporterType:        "zipkin",
		ExporterEndpoint:    "http://localhost:9411/api/v2/spans",
		SampleRate:          0.1,
		SamplerType:         "probability",
		SamplerProbability:  0.1,
		SamplerRate:         0.1,
		Headers:             []string{"Authorization", "User-Agent"},
		Attributes:          make(map[string]string),
		ExporterHeaders:     make(map[string]string),
		ExporterTLSInsecure: true,
		TelemetryLogLevel:   "info",
	}
}

// Get 返回配置接口
func (t *Tracing) Get() interface{} {
	return t
}

// Set 设置配置数据
func (t *Tracing) Set(data interface{}) {
	if cfg, ok := data.(*Tracing); ok {
		*t = *cfg
	}
}

// Clone 返回配置的副本
func (t *Tracing) Clone() internal.Configurable {
	var cloned Tracing
	if err := syncx.DeepCopy(&cloned, t); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Tracing{}
	}
	return &cloned
}

// Validate 验证配置
func (t *Tracing) Validate() error {
	return internal.ValidateStruct(t)
}

// WithServiceName 设置服务名称
func (t *Tracing) WithServiceName(serviceName string) *Tracing {
	t.ServiceName = serviceName
	return t
}

// WithEndpoint 设置追踪端点
func (t *Tracing) WithEndpoint(endpoint string) *Tracing {
	t.Endpoint = endpoint
	return t
}

// WithSampleRate 设置采样率
func (t *Tracing) WithSampleRate(sampleRate float64) *Tracing {
	t.SampleRate = sampleRate
	return t
}

// WithHeaders 设置需要记录的头部
func (t *Tracing) WithHeaders(headers []string) *Tracing {
	t.Headers = headers
	return t
}

// WithExporterHeaders 设置导出器自定义请求头
func (t *Tracing) WithExporterHeaders(headers map[string]string) *Tracing {
	t.ExporterHeaders = headers
	return t
}

// WithExporterTLSInsecure 设置导出器是否跳过 TLS 验证
func (t *Tracing) WithExporterTLSInsecure(insecure bool) *Tracing {
	t.ExporterTLSInsecure = insecure
	return t
}

// WithTelemetryLogLevel 设置遥测日志级别
func (t *Tracing) WithTelemetryLogLevel(level string) *Tracing {
	t.TelemetryLogLevel = level
	return t
}

// Enable 启用追踪中间件
func (t *Tracing) Enable() *Tracing {
	t.Enabled = true
	return t
}

// Disable 禁用追踪中间件
func (t *Tracing) Disable() *Tracing {
	t.Enabled = false
	return t
}

// IsEnabled 检查是否启用
func (t *Tracing) IsEnabled() bool {
	return t.Enabled
}
