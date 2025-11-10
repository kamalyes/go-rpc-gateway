/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 20:12:15
 * @FilePath: \go-rpc-gateway\config\gateway.go
 * @Description: Gateway主配置文件，集成go-config、go-core和go-logger
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package config

import (
	goconfig "github.com/kamalyes/go-config"
)

// GatewayConfig Gateway配置结构，基于go-config简化配置管理
type GatewayConfig struct {
	// 基础配置完全使用go-config
	*goconfig.SingleConfig `mapstructure:",squash" yaml:",inline" json:",inline"`

	// Gateway特有的扩展配置
	Gateway    GatewaySettings  `mapstructure:"gateway" yaml:"gateway" json:"gateway"`
	Middleware MiddlewareConfig `mapstructure:"middleware" yaml:"middleware" json:"middleware"`
	Monitoring MonitoringConfig `mapstructure:"monitoring" yaml:"monitoring" json:"monitoring"`
	Security   SecurityConfig   `mapstructure:"security" yaml:"security" json:"security"`
}

// GatewaySettings Gateway基础设置
type GatewaySettings struct {
	Name        string `mapstructure:"name" yaml:"name" json:"name"`
	Version     string `mapstructure:"version" yaml:"version" json:"version"`
	Environment string `mapstructure:"environment" yaml:"environment" json:"environment"`
	Debug       bool   `mapstructure:"debug" yaml:"debug" json:"debug"`

	// 服务器配置
	HTTP HTTPConfig `mapstructure:"http" yaml:"http" json:"http"`
	GRPC GRPCConfig `mapstructure:"grpc" yaml:"grpc" json:"grpc"`

	// 健康检查
	HealthCheck HealthCheckConfig `mapstructure:"health_check" yaml:"health_check" json:"health_check"`
}

// HTTPConfig HTTP服务器配置
type HTTPConfig struct {
	Host               string `mapstructure:"host" yaml:"host" json:"host"`
	Port               int    `mapstructure:"port" yaml:"port" json:"port"`
	ReadTimeout        int    `mapstructure:"read_timeout" yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout       int    `mapstructure:"write_timeout" yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout        int    `mapstructure:"idle_timeout" yaml:"idle_timeout" json:"idle_timeout"`
	MaxHeaderBytes     int    `mapstructure:"max_header_bytes" yaml:"max_header_bytes" json:"max_header_bytes"`
	EnableGzipCompress bool   `mapstructure:"enable_gzip_compress" yaml:"enable_gzip_compress" json:"enable_gzip_compress"`
}

// GRPCConfig GRPC服务器配置
type GRPCConfig struct {
	Host              string `mapstructure:"host" yaml:"host" json:"host"`
	Port              int    `mapstructure:"port" yaml:"port" json:"port"`
	Network           string `mapstructure:"network" yaml:"network" json:"network"`
	MaxRecvMsgSize    int    `mapstructure:"max_recv_msg_size" yaml:"max_recv_msg_size" json:"max_recv_msg_size"`
	MaxSendMsgSize    int    `mapstructure:"max_send_msg_size" yaml:"max_send_msg_size" json:"max_send_msg_size"`
	ConnectionTimeout int    `mapstructure:"connection_timeout" yaml:"connection_timeout" json:"connection_timeout"`
	KeepaliveTime     int    `mapstructure:"keepalive_time" yaml:"keepalive_time" json:"keepalive_time"`
	KeepaliveTimeout  int    `mapstructure:"keepalive_timeout" yaml:"keepalive_timeout" json:"keepalive_timeout"`
	EnableReflection  bool   `mapstructure:"enable_reflection" yaml:"enable_reflection" json:"enable_reflection"`
}
