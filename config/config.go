/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 00:00:00
 * @FilePath: \go-rpc-gateway\config\config.go
 * @Description: Gateway 配置 - 完全基于 go-config
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package config

import (
	goconfig "github.com/kamalyes/go-config"
)

// GatewayConfig Gateway 配置类型别名，完全使用 go-config.SingleConfig
// 所有配置项（Server, Zap, Cors, JWT, Redis, Jaeger, Pprof等）都由 go-config 提供
type GatewayConfig = goconfig.SingleConfig

// NewGatewayConfig 创建新的 Gateway 配置
func NewGatewayConfig() *GatewayConfig {
	config := &GatewayConfig{}
	config.InitializeExternalVipers()
	return config
}

// DefaultGatewayConfig 返回默认 Gateway 配置（使用 go-config 的所有默认值）
func DefaultGatewayConfig() *GatewayConfig {
	config := NewGatewayConfig()
	// 所有默认值都由 go-config 的 Default 函数提供，无需在此设置
	return config
}
