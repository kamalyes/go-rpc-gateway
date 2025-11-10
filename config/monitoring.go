/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:33:20
 * @FilePath: \go-rpc-gateway\config\monitoring.go
 * @Description: 监控配置模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package config

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Metrics MetricsConfig `mapstructure:"metrics" yaml:"metrics" json:"metrics"`
	Tracing TracingConfig `mapstructure:"tracing" yaml:"tracing" json:"tracing"`
}

// MetricsConfig 监控指标配置 - 集成 go-core 的 Prometheus 支持
type MetricsConfig struct {
	Enabled        bool                 `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Path           string               `mapstructure:"path" yaml:"path" json:"path"`
	Port           int                  `mapstructure:"port" yaml:"port" json:"port"`
	Namespace      string               `mapstructure:"namespace" yaml:"namespace" json:"namespace"`
	Subsystem      string               `mapstructure:"subsystem" yaml:"subsystem" json:"subsystem"`
	Labels         []string             `mapstructure:"labels" yaml:"labels" json:"labels"`
	PathMapping    map[string]string    `mapstructure:"path_mapping" yaml:"path_mapping" json:"path_mapping"`
	BuiltinMetrics BuiltinMetricsConfig `mapstructure:"builtin_metrics" yaml:"builtin_metrics" json:"builtin_metrics"`
}

// TracingConfig 链路追踪配置 - 集成 OpenTelemetry
type TracingConfig struct {
	Enabled  bool                  `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Exporter TracingExporterConfig `mapstructure:"exporter" yaml:"exporter" json:"exporter"`
	Sampler  TracingSamplerConfig  `mapstructure:"sampler" yaml:"sampler" json:"sampler"`
	Resource TracingResourceConfig `mapstructure:"resource" yaml:"resource" json:"resource"`
}
