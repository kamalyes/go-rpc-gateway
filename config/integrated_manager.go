/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 16:20:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 16:20:00
 * @FilePath: \go-rpc-gateway\config\integrated_manager.go
 * @Description: 重构后的集成配置管理器，专注于核心管理功能
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/kamalyes/go-toolbox/pkg/logger"
	"github.com/spf13/viper"
)

// IntegratedConfigManager 集成配置管理器
// 统一管理配置文件、环境变量、热重载和上下文的核心组件
type IntegratedConfigManager struct {
	mu              sync.RWMutex     // 读写锁，保护并发访问
	environment     *Environment     // 环境管理器
	hotReloader     HotReloader      // 热重载管理器
	contextManager  *ContextManager  // 上下文管理器
	viper           *viper.Viper     // Viper配置解析器
	config          interface{}      // 当前配置对象
	configPath      string           // 配置文件路径
	hotReloadConfig *HotReloadConfig // 热重载配置
	running         bool             // 运行状态标识
}

// IntegratedConfigOptions 集成配置管理器选项
type IntegratedConfigOptions struct {
	ConfigPath      string             // 配置文件路径
	Environment     EnvironmentType    // 初始环境
	HotReloadConfig *HotReloadConfig   // 热更新配置
	ContextOptions  *ContextKeyOptions // 上下文选项
}

// DefaultIntegratedConfigOptions 默认集成配置管理器选项
func DefaultIntegratedConfigOptions() *IntegratedConfigOptions {
	return &IntegratedConfigOptions{
		ConfigPath:      "",
		Environment:     DefaultEnv,
		HotReloadConfig: DefaultHotReloadConfig(),
		ContextOptions:  &ContextKeyOptions{Value: DefaultEnv},
	}
}

// NewIntegratedConfigManager 创建集成配置管理器
func NewIntegratedConfigManager(config interface{}, options *IntegratedConfigOptions) (*IntegratedConfigManager, error) {
	if options == nil {
		options = DefaultIntegratedConfigOptions()
	}

	// 统一补全热重载配置默认值，避免下游字段为 nil 导致解引用 panic
	if options.HotReloadConfig == nil {
		options.HotReloadConfig = DefaultHotReloadConfig()
	}

	// 创建环境管理器
	env := NewEnvironment()
	if options.Environment != "" {
		env.SetEnvironment(options.Environment)
	}

	// 设置上下文键
	if options.ContextOptions != nil {
		SetContextKey(options.ContextOptions)
	}

	// 创建Viper实例
	v := viper.New()

	// 配置Viper
	if options.ConfigPath != "" {
		if info, err := os.Stat(options.ConfigPath); err == nil && !info.IsDir() {
			// 是文件，设置配置文件路径
			v.SetConfigFile(options.ConfigPath)
			ext := filepath.Ext(options.ConfigPath)
			if len(ext) > 1 {
				v.SetConfigType(ext[1:]) // 去掉点号
			}
		} else {
			// 是目录或不存在，让Viper在该目录中查找配置文件
			v.AddConfigPath(options.ConfigPath)
			v.SetConfigName("config") // 默认配置文件名
		}
	}

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, ErrReadConfigFile(err)
	}

	// 解析配置到结构体
	if err := v.Unmarshal(config); err != nil {
		return nil, ErrUnmarshalConfig(err)
	}

	// 处理配置指针：如果传入的是指向指针的指针，需要解引用
	actualConfig := config
	if reflect.TypeOf(config).Kind() == reflect.Ptr &&
		reflect.TypeOf(config).Elem().Kind() == reflect.Ptr {
		// 传入的是 **T，解引用得到 *T
		actualConfig = reflect.ValueOf(config).Elem().Interface()
	}

	// 创建热更新器
	hotReloader, err := NewHotReloader(config, v, options.ConfigPath, options.HotReloadConfig)
	if err != nil {
		return nil, ErrCreateHotReloader(err)
	}

	// 创建上下文管理器
	contextManager := NewContextManager(env, hotReloader)

	// 初始化全局上下文管理器
	InitializeContextManager(env, hotReloader)

	manager := &IntegratedConfigManager{
		environment:     env,
		hotReloader:     hotReloader,
		contextManager:  contextManager,
		viper:           v,
		config:          actualConfig,
		configPath:      options.ConfigPath,
		hotReloadConfig: options.HotReloadConfig,
		running:         false,
	}

	// 注册内部回调
	manager.registerInternalCallbacks()

	logger.GetGlobalLogger().Info("✅ 集成配置管理器创建完成，配置文件: %s", options.ConfigPath)
	return manager, nil
}

// registerInternalCallbacks 注册内部回调函数
func (icm *IntegratedConfigManager) registerInternalCallbacks() {
	// 注册配置变更回调
	icm.hotReloader.RegisterCallback(icm.onConfigReloaded, CallbackOptions{
		ID:       "integrated_manager_config",
		Types:    []CallbackType{CallbackTypeConfigChanged, CallbackTypeReloaded},
		Priority: -1000, // 最高优先级
		Async:    false,
		Timeout:  5 * time.Second,
	})

	// 注册环境变更回调
	icm.environment.RegisterCallback("integrated_manager_env", icm.onEnvironmentChanged, -1000, false)

	// 注册错误回调
	icm.hotReloader.RegisterCallback(icm.onError, CallbackOptions{
		ID:       "integrated_manager_error",
		Types:    []CallbackType{CallbackTypeError},
		Priority: -1000, // 最高优先级
		Async:    true,
		Timeout:  10 * time.Second,
	})
}

// onConfigReloaded 处理配置重新加载事件
func (icm *IntegratedConfigManager) onConfigReloaded(ctx context.Context, event CallbackEvent) error {
	icm.mu.Lock()
	defer icm.mu.Unlock()

	logger.GetGlobalLogger().Info("🔄 集成管理器: 配置已重新加载: %s", event.Source)

	// 更新本地配置引用
	icm.config = event.NewValue

	// 更新上下文管理器中的配置
	icm.contextManager.UpdateConfig(event.NewValue)

	return nil
}

// onEnvironmentChanged 处理环境变更事件
func (icm *IntegratedConfigManager) onEnvironmentChanged(oldEnv, newEnv EnvironmentType) error {
	logger.GetGlobalLogger().Info("🌍 集成管理器: 环境已变更: %s -> %s", oldEnv, newEnv)
	return nil
}

// onError 处理错误事件
func (icm *IntegratedConfigManager) onError(ctx context.Context, event CallbackEvent) error {
	logger.GetGlobalLogger().Error("❌ 集成管理器: 发生错误: %s, 来源: %s", event.Error, event.Source)
	return nil
}

// Start 启动配置管理器
func (icm *IntegratedConfigManager) Start(ctx context.Context) error {
	icm.mu.Lock()
	defer icm.mu.Unlock()

	if icm.running {
		return ErrManagerRunning
	}

	// 启动热更新器
	if err := icm.hotReloader.Start(ctx); err != nil {
		return ErrStartHotReloader(err)
	}

	// 更新上下文管理器配置
	icm.contextManager.UpdateConfig(icm.config)

	icm.running = true
	logger.GetGlobalLogger().Info("🚀 集成配置管理器启动成功")
	return nil
}

// Stop 停止配置管理器
func (icm *IntegratedConfigManager) Stop() error {
	icm.mu.Lock()
	defer icm.mu.Unlock()

	// 幂等：未启动时直接返回 nil，使 defer Stop() 安全
	if !icm.running {
		return nil
	}

	// 停止热更新器
	if err := icm.hotReloader.Stop(); err != nil {
		return ErrStopHotReloader(err)
	}

	// 停止环境监控
	icm.environment.StopWatch()

	icm.running = false
	logger.GetGlobalLogger().Warn("⏹️ 集成配置管理器已停止")
	return nil
}

// IsRunning 检查管理器是否正在运行
func (icm *IntegratedConfigManager) IsRunning() bool {
	icm.mu.RLock()
	defer icm.mu.RUnlock()
	return icm.running
}

// GetConfig 获取当前配置
func (icm *IntegratedConfigManager) GetConfig() interface{} {
	icm.mu.RLock()
	defer icm.mu.RUnlock()
	return icm.config
}

// GetConfigAs 获取指定类型的配置
func GetConfigAs[T any](icm *IntegratedConfigManager) (*T, error) {
	config := icm.GetConfig()
	if typedConfig, ok := config.(*T); ok {
		return typedConfig, nil
	}
	return nil, ErrConfigTypeMismatch(new(T), config)
}

// GetEnvironment 获取当前环境
func (icm *IntegratedConfigManager) GetEnvironment() EnvironmentType {
	return icm.environment.Value
}

// GetConfigPath 获取配置文件路径
func (icm *IntegratedConfigManager) GetConfigPath() string {
	icm.mu.RLock()
	defer icm.mu.RUnlock()
	return icm.configPath
}

// GetViper 获取Viper实例
func (icm *IntegratedConfigManager) GetViper() *viper.Viper {
	return icm.viper
}

// GetHotReloader 获取热重载器
func (icm *IntegratedConfigManager) GetHotReloader() HotReloader {
	return icm.hotReloader
}

// GetContextManager 获取上下文管理器
func (icm *IntegratedConfigManager) GetContextManager() *ContextManager {
	return icm.contextManager
}

// GetEnvironmentManager 获取环境管理器
func (icm *IntegratedConfigManager) GetEnvironmentManager() *Environment {
	return icm.environment
}

// WithContext 将配置信息注入到上下文中
func (icm *IntegratedConfigManager) WithContext(ctx context.Context) context.Context {
	return icm.contextManager.WithConfig(ctx)
}

// RegisterConfigCallback 注册配置变更回调
func (icm *IntegratedConfigManager) RegisterConfigCallback(callback CallbackFunc, options CallbackOptions) error {
	return icm.hotReloader.RegisterCallback(callback, options)
}

// RegisterEnvironmentCallback 注册环境变更回调
func (icm *IntegratedConfigManager) RegisterEnvironmentCallback(id string, callback EnvironmentCallback, priority int, async bool) error {
	return icm.environment.RegisterCallback(id, callback, priority, async)
}

// UnregisterConfigCallback 取消配置变更回调
func (icm *IntegratedConfigManager) UnregisterConfigCallback(id string) error {
	return icm.hotReloader.UnregisterCallback(id)
}

// UnregisterEnvironmentCallback 取消环境变更回调
func (icm *IntegratedConfigManager) UnregisterEnvironmentCallback(id string) error {
	return icm.environment.UnregisterCallback(id)
}

// SetEnvironment 设置应用环境
func (icm *IntegratedConfigManager) SetEnvironment(env EnvironmentType) error {
	icm.environment.SetEnvironment(env)
	return nil
}

// ValidateConfig 验证配置有效性
func (icm *IntegratedConfigManager) ValidateConfig() error {
	if icm.config == nil {
		return ErrConfigEmpty
	}

	logger.GetGlobalLogger().Debug("✅ 配置验证通过")
	return nil
}

// GetConfigMetadata 获取配置元数据
func (icm *IntegratedConfigManager) GetConfigMetadata() map[string]interface{} {
	metadata := make(map[string]interface{})

	metadata["config_path"] = icm.configPath
	metadata["environment"] = icm.GetEnvironment()
	metadata["running"] = icm.IsRunning()
	metadata["hot_reload_enabled"] = icm.hotReloadConfig.Enabled
	metadata["created_at"] = icm.contextManager.GetConfigContext().CreatedAt
	metadata["updated_at"] = icm.contextManager.GetConfigContext().UpdatedAt

	return metadata
}

// MustStart 必须成功启动配置管理器
func (icm *IntegratedConfigManager) MustStart(ctx context.Context) {
	if err := icm.Start(ctx); err != nil {
		panic(fmt.Sprintf("启动集成配置管理器失败: %v", err))
	}
}

// MustGetConfigAs 必须成功获取指定类型的配置
func MustGetConfigAs[T any](icm *IntegratedConfigManager) *T {
	config, err := GetConfigAs[T](icm)
	if err != nil {
		panic(fmt.Sprintf("获取配置失败: %v", err))
	}
	return config
}

// CreateIntegratedManager 创建集成配置管理器的便捷函数
func CreateIntegratedManager(config interface{}, configPath string, env EnvironmentType) (*IntegratedConfigManager, error) {
	options := &IntegratedConfigOptions{
		ConfigPath:      configPath,
		Environment:     env,
		HotReloadConfig: DefaultHotReloadConfig(),
		ContextOptions: &ContextKeyOptions{
			Value: env,
		},
	}

	return NewIntegratedConfigManager(config, options)
}

// ScanAndDisplayConfigs 扫描并显示可用的配置文件
func ScanAndDisplayConfigs(searchPath string, env EnvironmentType) ([]*ConfigFileInfo, error) {
	discovery := GetGlobalConfigDiscovery()

	// 发现所有配置文件
	allConfigs, err := discovery.DiscoverConfigFiles(searchPath, env)
	if err != nil {
		return nil, err
	}

	// 扫描目录中实际存在的配置文件
	existingConfigs, err := discovery.ScanDirectory(searchPath)
	if err != nil {
		logger.GetGlobalLogger().Error("扫描目录失败: %v", err)
	}

	logger.GetGlobalLogger().InfoLines(
		"📋 配置文件发现报告:",
		fmt.Sprintf("🔍 搜索路径: %s", searchPath),
		fmt.Sprintf("🌍 目标环境: %s", env),
	)

	if len(existingConfigs) > 0 {
		logger.GetGlobalLogger().Info("\n✅ 发现的现有配置文件:")
		for i, info := range existingConfigs {
			if i < 5 { // 只显示前5个
				logger.GetGlobalLogger().Debug("   %d. %s (环境: %s, 优先级: %d)",
					i+1, info.Name, info.Environment, info.Priority)
			}
		}
		if len(existingConfigs) > 5 {
			logger.GetGlobalLogger().Debug("   ... 还有 %d 个文件", len(existingConfigs)-5)
		}
	}

	// 显示推荐的配置文件
	logger.GetGlobalLogger().Debug("\n💡 推荐的配置文件候选:")
	shown := 0
	for _, info := range allConfigs {
		if shown >= 3 {
			break
		}
		status := "❌ 不存在"
		if info.Exists {
			status = "✅ 存在"
		}
		logger.GetGlobalLogger().Debug("   %d. %s (%s, 优先级: %d)",
			shown+1, info.Name, status, info.Priority)
		shown++
	}

	return allConfigs, nil
}
