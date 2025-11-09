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