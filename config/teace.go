/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:40:10
 * @FilePath: \go-rpc-gateway\config\teace.go
 * @Description:
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package config

// TracingMiddlewareConfig 中间件级别的链路追踪配置 (扩展已有的TracingConfig)
type TracingMiddlewareConfig struct {
	Enabled  bool                  `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Exporter TracingExporterConfig `mapstructure:"exporter" yaml:"exporter" json:"exporter"`
	Sampler  TracingSamplerConfig  `mapstructure:"sampler" yaml:"sampler" json:"sampler"`
	Resource TracingResourceConfig `mapstructure:"resource" yaml:"resource" json:"resource"`
}

// TracingExporterConfig 追踪导出器配置
type TracingExporterConfig struct {
	Type         string `mapstructure:"type" yaml:"type" json:"type"` // jaeger, zipkin, otlp
	Endpoint     string `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint"`
	OTLPEndpoint string `mapstructure:"otlp_endpoint" yaml:"otlp_endpoint" json:"otlp_endpoint"`
	OTLPInsecure bool   `mapstructure:"otlp_insecure" yaml:"otlp_insecure" json:"otlp_insecure"`
}

// TracingSamplerConfig 采样配置
type TracingSamplerConfig struct {
	Type        string  `mapstructure:"type" yaml:"type" json:"type"` // always, never, probability, rate_limiting
	Probability float64 `mapstructure:"probability" yaml:"probability" json:"probability"`
	Rate        int     `mapstructure:"rate" yaml:"rate" json:"rate"`
}

// TracingResourceConfig 资源配置
type TracingResourceConfig struct {
	ServiceName    string            `mapstructure:"service_name" yaml:"service_name" json:"service_name"`
	ServiceVersion string            `mapstructure:"service_version" yaml:"service_version" json:"service_version"`
	Environment    string            `mapstructure:"environment" yaml:"environment" json:"environment"`
	Attributes     map[string]string `mapstructure:"attributes" yaml:"attributes" json:"attributes"`
}
