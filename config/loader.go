/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 00:00:00
 * @FilePath: \go-rpc-gateway\config\loader.go
 * @Description: 配置加载器 - 使用 go-config 的配置管理器
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package config

import (
	"context"
	"fmt"
	"os"

	goconfig "github.com/kamalyes/go-config"
	"github.com/kamalyes/go-core/pkg/global"
)

// Loader 配置加载器
type Loader struct {
	manager *goconfig.SingleConfigManager
	config  *GatewayConfig
}

// NewLoader 创建配置加载器
func NewLoader() *Loader {
	return &Loader{
		config: NewGatewayConfig(),
	}
}

// LoadFromFile 从文件加载配置
func (l *Loader) LoadFromFile(configPath string) (*GatewayConfig, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path is empty")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	// 使用 go-config 的 SingleConfigManager 加载配置
	// 从文件路径中提取配置路径和文件名（这里需要根据实际情况调整）
	options := goconfig.GetDefaultConfigOptions()
	options.ConfigPath = configPath
	
	manager, err := goconfig.NewSingleConfigManager(context.Background(), options)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	l.manager = manager
	l.config = (*GatewayConfig)(manager.SingleConfig)

	// 应用环境特定的默认值
	l.applyEnvironmentDefaults()

	// 初始化全局变量
	l.initGlobals()

	return l.config, nil
}

// LoadDefault 加载默认配置
func (l *Loader) LoadDefault() *GatewayConfig {
	l.config = DefaultGatewayConfig()
	l.applyEnvironmentDefaults()
	l.initGlobals()
	return l.config
}

// applyEnvironmentDefaults 根据环境应用默认配置（使用 go-config 的 With 方法）
func (l *Loader) applyEnvironmentDefaults() {
	// 从环境变量或配置中获取环境类型
	envType := os.Getenv("GO_ENV")
	if envType == "" {
		envType = "development"
	}
	
	switch envType {
	case "production", "prod":
		l.applyProductionDefaults()
	case "staging":
		l.applyStagingDefaults()
	case "test", "testing":
		l.applyTestDefaults()
	default:
		l.applyDevelopmentDefaults()
	}
}

// applyDevelopmentDefaults 应用开发环境默认配置（使用 With 方法）
func (l *Loader) applyDevelopmentDefaults() {
	// 使用 go-config 的 With 方法设置日志配置
	l.config.Zap.
		WithLevel("debug").
		WithDevelopment(true).
		WithShowLine(true).
		WithPrefix("[GATEWAY-DEV]")
	
	// 启用 PProf
	l.config.Pprof.
		WithEnabled(true).
		WithPathPrefix("/debug/pprof")
	
	// 启用 CORS（允许所有来源）
	l.config.Cors.
		WithAllowedAllOrigins(true).
		WithAllowCredentials(true)
}

// applyProductionDefaults 应用生产环境默认配置
func (l *Loader) applyProductionDefaults() {
	// 使用 go-config 的 With 方法设置日志配置
	l.config.Zap.
		WithLevel("info").
		WithDevelopment(false).
		WithShowLine(false).
		WithPrefix("[GATEWAY]").
		WithFormat("json")
	
	// 禁用 PProf
	l.config.Pprof.WithEnabled(false)
	
	// 严格的 CORS 配置
	l.config.Cors.
		WithAllowedAllOrigins(false).
		WithAllowedOrigins([]string{}).
		WithAllowCredentials(true)
}

// applyStagingDefaults 应用预发布环境默认配置
func (l *Loader) applyStagingDefaults() {
	// 使用 go-config 的 With 方法设置日志配置
	l.config.Zap.
		WithLevel("debug").
		WithDevelopment(false).
		WithPrefix("[GATEWAY-STAGING]")
	
	// 启用 PProf（预发布环境保留调试功能）
	l.config.Pprof.
		WithEnabled(true).
		WithPathPrefix("/debug/pprof")
}

// applyTestDefaults 应用测试环境默认配置
func (l *Loader) applyTestDefaults() {
	// 使用 go-config 的 With 方法设置日志配置
	l.config.Zap.
		WithLevel("info").
		WithDevelopment(false).
		WithPrefix("[GATEWAY-TEST]")
	
	// 禁用 PProf
	l.config.Pprof.WithEnabled(false)
}

// initGlobals 初始化全局变量
func (l *Loader) initGlobals() {
	if l.config.Viper != nil {
		global.VP = l.config.Viper
	}
	global.CONFIG = l.config
}

// GetConfig 获取配置
func (l *Loader) GetConfig() *GatewayConfig {
	return l.config
}
