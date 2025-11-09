/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 01:15:03
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

// MetricsConfig 指标配置
type MetricsConfig struct {
	Enabled   bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Path      string `mapstructure:"path" yaml:"path" json:"path"`
	Port      int    `mapstructure:"port" yaml:"port" json:"port"`
	Namespace string `mapstructure:"namespace" yaml:"namespace" json:"namespace"`
	Subsystem string `mapstructure:"subsystem" yaml:"subsystem" json:"subsystem"`
}

// TracingConfig 链路追踪配置
type TracingConfig struct {
	Enabled     bool    `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	ServiceName string  `mapstructure:"service_name" yaml:"service_name" json:"service_name"`
	Endpoint    string  `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint"`
	SampleRate  float64 `mapstructure:"sample_rate" yaml:"sample_rate" json:"sample_rate"`
}