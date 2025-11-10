/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 07:40:26
 * @FilePath: \go-rpc-gateway\config\manager.go
 * @Description: 配置管理器
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package config

import (
	"fmt"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/kamalyes/go-config/pkg/env"
	"github.com/kamalyes/go-core/pkg/global"
	"github.com/kamalyes/go-logger"
	"github.com/spf13/viper"
)

// ConfigManager Gateway配置管理器，集成go-config功能
type ConfigManager struct {
	config      *GatewayConfig
	viper       *viper.Viper
	configPath  string
	environment env.EnvironmentType
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configPath string) (*ConfigManager, error) {
	manager := &ConfigManager{
		config:     DefaultGatewayConfig(),
		configPath: configPath,
		viper:      viper.New(),
	}

	// 设置环境
	envVar := os.Getenv("GO_ENV")
	if envVar != "" {
		manager.environment = env.EnvironmentType(envVar)
	} else {
		manager.environment = env.Dev
	}

	// 加载配置
	if err := manager.LoadConfig(); err != nil {
		return nil, err
	}

	return manager, nil
}

// LoadConfig 加载配置文件并初始化全局组件
func (cm *ConfigManager) LoadConfig() error {
	if cm.configPath != "" && fileExists(cm.configPath) {
		cm.viper.SetConfigFile(cm.configPath)

		if err := cm.viper.ReadInConfig(); err != nil {
			return fmt.Errorf("读取配置文件失败: %w", err)
		}

		// 将viper配置解析到结构体
		if err := cm.viper.Unmarshal(cm.config); err != nil {
			return fmt.Errorf("解析配置失败: %w", err)
		}

		// 初始化全局组件
		if err := cm.InitGlobalComponents(); err != nil {
			return fmt.Errorf("初始化全局组件失败: %w", err)
		}

		// 使用初始化后的logger记录日志
		global.LOGGER.InfoKV("配置文件加载成功",
			"path", cm.configPath,
			"environment", string(cm.environment))
	} else {
		// 即使没有配置文件，也要初始化全局组件
		if err := cm.InitGlobalComponents(); err != nil {
			return fmt.Errorf("初始化全局组件失败: %w", err)
		}
	}

	return nil
}

// InitGlobalComponents 初始化全局组件，集成go-core和go-logger
func (cm *ConfigManager) InitGlobalComponents() error {
	// 1. 设置全局配置到go-core
	global.VP = cm.viper
	global.CONFIG = cm.config.SingleConfig

	// 2. 初始化go-logger实例
	loggerConfig := logger.DefaultConfig()
	if cm.config.SingleConfig.Zap.Prefix != "" {
		loggerConfig.Prefix = cm.config.SingleConfig.Zap.Prefix
	}
	if cm.config.SingleConfig.Zap.Level != "" {
		if level, err := logger.ParseLevel(cm.config.SingleConfig.Zap.Level); err == nil {
			loggerConfig.Level = level
		}
	}

	// 创建并设置全局logger
	global.LOGGER = logger.NewLogger(loggerConfig)

	// 3. 应用环境特定配置
	cm.applyEnvironmentDefaults()

	return nil
}

// applyEnvironmentDefaults 应用环境特定的默认配置
func (cm *ConfigManager) applyEnvironmentDefaults() {
	environment := cm.config.Gateway.Environment
	if environment == "" {
		// 从环境变量或manager环境设置
		environment = string(cm.environment)
		cm.config.Gateway.Environment = environment
	}

	switch environment {
	case "development", "dev":
		cm.applyDevelopmentDefaults()
	case "test", "testing":
		cm.applyTestDefaults()
	case "production", "prod":
		cm.applyProductionDefaults()
	case "staging":
		cm.applyStagingDefaults()
	default:
		global.LOGGER.WarnKV("未识别的环境类型，使用默认配置", "environment", environment)
		cm.applyDevelopmentDefaults() // 默认使用开发环境配置
	}

	global.LOGGER.InfoKV("已应用环境特定配置",
		"environment", environment,
		"debug", cm.config.Gateway.Debug)
}

// applyDevelopmentDefaults 应用开发环境默认配置
func (cm *ConfigManager) applyDevelopmentDefaults() {
	// 开发环境：启用调试、详细日志、开发工具
	cm.config.Gateway.Debug = true

	// 日志配置
	if cm.config.SingleConfig.Zap.Level == "" {
		cm.config.SingleConfig.Zap.Level = "debug"
	}
	cm.config.SingleConfig.Zap.Development = true
	cm.config.SingleConfig.Zap.ShowLine = true

	// 启用开发工具
	cm.config.Gateway.GRPC.EnableReflection = true
	cm.config.Middleware.PProf.Enabled = true
	cm.config.Middleware.Banner.Enabled = true
	cm.config.Middleware.Banner.ShowSystemInfo = true
	cm.config.Middleware.Banner.ShowMiddleware = true

	// 监控配置
	cm.config.Monitoring.Metrics.Enabled = true
	cm.config.Monitoring.Tracing.Enabled = false // 开发环境可选

	// 宽松的超时配置
	if cm.config.Gateway.HTTP.ReadTimeout == 0 {
		cm.config.Gateway.HTTP.ReadTimeout = 30
	}
	if cm.config.Gateway.HTTP.WriteTimeout == 0 {
		cm.config.Gateway.HTTP.WriteTimeout = 30
	}
}

// applyTestDefaults 应用测试环境默认配置
func (cm *ConfigManager) applyTestDefaults() {
	// 测试环境：快速启动、最小依赖、稳定配置
	cm.config.Gateway.Debug = false

	// 日志配置
	if cm.config.SingleConfig.Zap.Level == "" {
		cm.config.SingleConfig.Zap.Level = "info"
	}
	cm.config.SingleConfig.Zap.Development = false

	// 禁用非必要功能
	cm.config.Middleware.PProf.Enabled = false
	cm.config.Middleware.Banner.Enabled = false
	cm.config.Gateway.GRPC.EnableReflection = false

	// 启用基础监控
	cm.config.Monitoring.Metrics.Enabled = true
	cm.config.Monitoring.Tracing.Enabled = true

	// 较短的超时时间
	if cm.config.Gateway.HTTP.ReadTimeout == 0 {
		cm.config.Gateway.HTTP.ReadTimeout = 10
	}
	if cm.config.Gateway.HTTP.WriteTimeout == 0 {
		cm.config.Gateway.HTTP.WriteTimeout = 10
	}
}

// applyProductionDefaults 应用生产环境默认配置
func (cm *ConfigManager) applyProductionDefaults() {
	// 生产环境：安全第一、性能优化、完整监控
	cm.config.Gateway.Debug = false

	// 日志配置
	if cm.config.SingleConfig.Zap.Level == "" {
		cm.config.SingleConfig.Zap.Level = "info"
	}
	cm.config.SingleConfig.Zap.Development = false
	cm.config.SingleConfig.Zap.ShowLine = false // 生产环境不显示行号提升性能

	// 安全配置
	cm.config.Security.TLS.Enabled = true
	cm.config.Security.Security.Enabled = true
	cm.config.Security.Security.XSSProtection = true
	cm.config.Security.Security.ContentTypeNoSniff = true
	cm.config.Security.Security.FrameOptions = "DENY"
	cm.config.Security.Security.HSTSMaxAge = 31536000 // 1年

	// 禁用调试工具
	cm.config.Middleware.PProf.Enabled = false
	cm.config.Gateway.GRPC.EnableReflection = false
	cm.config.Middleware.Banner.Enabled = true // 生产环境显示启动信息
	cm.config.Middleware.Banner.ShowSystemInfo = false

	// 启用完整监控
	cm.config.Monitoring.Metrics.Enabled = true
	cm.config.Monitoring.Tracing.Enabled = true

	// 优化的超时配置
	if cm.config.Gateway.HTTP.ReadTimeout == 0 {
		cm.config.Gateway.HTTP.ReadTimeout = 15
	}
	if cm.config.Gateway.HTTP.WriteTimeout == 0 {
		cm.config.Gateway.HTTP.WriteTimeout = 15
	}
	if cm.config.Gateway.HTTP.IdleTimeout == 0 {
		cm.config.Gateway.HTTP.IdleTimeout = 60
	}

	// 启用HTTP压缩
	cm.config.Gateway.HTTP.EnableGzipCompress = true

	// 限制头部大小
	if cm.config.Gateway.HTTP.MaxHeaderBytes == 0 {
		cm.config.Gateway.HTTP.MaxHeaderBytes = 1 << 20 // 1MB
	}
}

// applyStagingDefaults 应用预发布环境默认配置
func (cm *ConfigManager) applyStagingDefaults() {
	// 预发布环境：生产配置 + 部分调试功能
	cm.applyProductionDefaults()

	// 但保留一些调试功能
	cm.config.Gateway.Debug = true
	cm.config.SingleConfig.Zap.Level = "debug"
	cm.config.Middleware.PProf.Enabled = true
	cm.config.Gateway.GRPC.EnableReflection = true
}

// ValidateConfig 验证配置的合理性
func (cm *ConfigManager) ValidateConfig() error {
	config := cm.config

	// 验证端口范围
	if config.Gateway.HTTP.Port < 1 || config.Gateway.HTTP.Port > 65535 {
		return fmt.Errorf("HTTP端口超出范围: %d", config.Gateway.HTTP.Port)
	}
	if config.Gateway.GRPC.Port < 1 || config.Gateway.GRPC.Port > 65535 {
		return fmt.Errorf("gRPC端口超出范围: %d", config.Gateway.GRPC.Port)
	}

	// 验证超时配置
	if config.Gateway.HTTP.ReadTimeout < 0 || config.Gateway.HTTP.ReadTimeout > 300 {
		return fmt.Errorf("HTTP读取超时时间不合理: %d秒", config.Gateway.HTTP.ReadTimeout)
	}
	if config.Gateway.HTTP.WriteTimeout < 0 || config.Gateway.HTTP.WriteTimeout > 300 {
		return fmt.Errorf("HTTP写入超时时间不合理: %d秒", config.Gateway.HTTP.WriteTimeout)
	}

	// 验证gRPC消息大小
	if config.Gateway.GRPC.MaxRecvMsgSize < 1024 || config.Gateway.GRPC.MaxRecvMsgSize > 100*1024*1024 {
		return fmt.Errorf("gRPC最大接收消息大小不合理: %d字节", config.Gateway.GRPC.MaxRecvMsgSize)
	}

	// 验证TLS配置
	if config.Security.TLS.Enabled {
		if config.Security.TLS.CertFile == "" || config.Security.TLS.KeyFile == "" {
			return fmt.Errorf("启用TLS时必须配置证书文件和私钥文件")
		}
		if !fileExists(config.Security.TLS.CertFile) {
			return fmt.Errorf("证书文件不存在: %s", config.Security.TLS.CertFile)
		}
		if !fileExists(config.Security.TLS.KeyFile) {
			return fmt.Errorf("私钥文件不存在: %s", config.Security.TLS.KeyFile)
		}
	}

	return nil
}

// GetConfig 获取配置
func (cm *ConfigManager) GetConfig() *GatewayConfig {
	return cm.config
}

// GetEnvironment 获取当前环境
func (cm *ConfigManager) GetEnvironment() env.EnvironmentType {
	return cm.environment
}

// WatchConfig 监听配置变化
func (cm *ConfigManager) WatchConfig(callback func(*GatewayConfig)) {
	if cm.configPath == "" {
		return
	}

	cm.viper.WatchConfig()
	cm.viper.OnConfigChange(func(e fsnotify.Event) {
		// 使用go-logger记录日志
		global.LOGGER.InfoKV("配置文件发生变化", "file", e.Name)

		// 重新加载配置
		if err := cm.LoadConfig(); err != nil {
			global.LOGGER.WithError(err).ErrorMsg("重新加载配置失败")
			return
		}

		// 回调通知
		if callback != nil {
			callback(cm.config)
		}
	})
}

// IsProduction 检查是否为生产环境
func (cm *ConfigManager) IsProduction() bool {
	return cm.environment == env.Prod
}

// IsDevelopment 检查是否为开发环境
func (cm *ConfigManager) IsDevelopment() bool {
	return cm.environment == env.Dev
}

// fileExists 检查文件是否存在
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}
