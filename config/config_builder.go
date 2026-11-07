/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 16:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 16:00:00
 * @FilePath: \go-rpc-gateway\config\config_builder.go
 * @Description: 配置管理器构建器，提供链式API和灵活的配置发现机制
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package config

import (
	"context"
	"fmt"
	"sync"

	"github.com/kamalyes/go-toolbox/pkg/logger"
)

// ConfigBuilder 配置构建器接口
type ConfigBuilder[T any] interface {
	// WithConfigPath 设置配置文件路径
	WithConfigPath(path string) ConfigBuilder[T]

	// WithSearchPath 设置配置文件搜索路径
	WithSearchPath(path string) ConfigBuilder[T]

	// WithEnvironment 设置运行环境
	WithEnvironment(env EnvironmentType) ConfigBuilder[T]

	// WithPrefix 设置配置文件名前缀
	WithPrefix(prefix string) ConfigBuilder[T]

	// WithPattern 设置文件匹配模式
	WithPattern(pattern string) ConfigBuilder[T]

	// WithHotReload 启用配置热重载功能
	WithHotReload(config *HotReloadConfig) ConfigBuilder[T]

	// WithContext 设置上下文配置选项
	WithContext(options *ContextKeyOptions) ConfigBuilder[T]

	// Build 构建配置管理器
	Build() (*IntegratedConfigManager, error)

	// BuildAndStart 构建并启动配置管理器
	BuildAndStart(ctx ...context.Context) (*IntegratedConfigManager, error)

	// MustBuildAndStart 构建并启动配置管理器（失败时panic）
	MustBuildAndStart(ctx ...context.Context) *IntegratedConfigManager
}

// ManagerBuilder 配置管理器构建器实现
type ManagerBuilder[T any] struct {
	config          *T                 // 配置结构体指针
	configPath      string             // 直接指定的配置文件路径
	searchPath      string             // 配置文件搜索路径
	environment     EnvironmentType    // 运行环境类型
	configPrefix    string             // 配置文件名前缀
	pattern         string             // 文件匹配模式
	hotReloadConfig *HotReloadConfig   // 热重载配置
	contextOptions  *ContextKeyOptions // 上下文选项
	autoDiscovery   bool               // 是否启用自动发现
	usePattern      bool               // 是否使用模式匹配
	useCustomPrefix bool               // 是否使用自定义前缀
}

// NewConfigBuilder 创建新的配置构建器
func NewConfigBuilder[T any](config *T) ConfigBuilder[T] {
	return &ManagerBuilder[T]{
		config:      config,
		environment: GetEnvironment(),
	}
}

// WithConfigPath 设置配置文件路径
func (b *ManagerBuilder[T]) WithConfigPath(path string) ConfigBuilder[T] {
	b.configPath = path
	logger.GetGlobalLogger().Debug("🔧 设置配置文件路径: %s", path)
	return b
}

// WithSearchPath 设置配置文件搜索路径
func (b *ManagerBuilder[T]) WithSearchPath(path string) ConfigBuilder[T] {
	b.searchPath = path
	b.autoDiscovery = true
	logger.GetGlobalLogger().Debug("🔍 启用自动发现模式，搜索路径: %s", path)
	return b
}

// WithEnvironment 设置运行环境
func (b *ManagerBuilder[T]) WithEnvironment(env EnvironmentType) ConfigBuilder[T] {
	b.environment = env
	logger.GetGlobalLogger().Debug("🌍 设置运行环境: %s", env)
	return b
}

// WithPrefix 设置配置文件名前缀
func (b *ManagerBuilder[T]) WithPrefix(prefix string) ConfigBuilder[T] {
	b.configPrefix = prefix
	b.useCustomPrefix = true
	logger.GetGlobalLogger().Debug("📛 设置配置文件前缀: %s", prefix)
	return b
}

// WithPattern 设置文件匹配模式
func (b *ManagerBuilder[T]) WithPattern(pattern string) ConfigBuilder[T] {
	b.pattern = pattern
	b.usePattern = true
	logger.GetGlobalLogger().Debug("🎯 设置文件匹配模式: %s", pattern)
	return b
}

// WithHotReload 启用配置热重载功能
func (b *ManagerBuilder[T]) WithHotReload(config *HotReloadConfig) ConfigBuilder[T] {
	if config == nil {
		config = DefaultHotReloadConfig()
	}
	b.hotReloadConfig = config
	logger.GetGlobalLogger().Debug("🔥 启用热重载功能，启用状态: %v", config.Enabled)
	return b
}

// WithContext 设置上下文配置选项
func (b *ManagerBuilder[T]) WithContext(options *ContextKeyOptions) ConfigBuilder[T] {
	b.contextOptions = options
	logger.GetGlobalLogger().Debug("📋 设置上下文选项")
	return b
}

// Build 构建配置管理器
func (b *ManagerBuilder[T]) Build() (*IntegratedConfigManager, error) {
	// 解析配置路径
	configPath, err := b.resolveConfigPath()
	if err != nil {
		return nil, ErrResolveConfigPath(err)
	}

	// 创建集成配置管理器
	options := &IntegratedConfigOptions{
		ConfigPath:      configPath,
		Environment:     b.environment,
		HotReloadConfig: b.hotReloadConfig,
		ContextOptions:  b.contextOptions,
	}

	manager, err := NewIntegratedConfigManager(b.config, options)
	if err != nil {
		return nil, ErrCreateManager(err)
	}

	logger.GetGlobalLogger().Info("🎯 配置管理器构建完成")
	return manager, nil
}

// BuildAndStart 构建并启动配置管理器
func (b *ManagerBuilder[T]) BuildAndStart(ctx ...context.Context) (*IntegratedConfigManager, error) {
	manager, err := b.Build()
	if err != nil {
		return nil, err
	}

	// 使用提供的上下文或创建默认上下文
	var startCtx context.Context
	if len(ctx) > 0 && ctx[0] != nil {
		startCtx = ctx[0]
	} else {
		// 使用 Background context，不设置 timeout
		// 因为热更新需要长期运行
		startCtx = context.Background()
	}

	if err := manager.Start(startCtx); err != nil {
		return nil, ErrStartManager(err)
	}

	logger.GetGlobalLogger().Info("🚀 配置管理器构建并启动成功")
	return manager, nil
}

// MustBuildAndStart 构建并启动配置管理器
func (b *ManagerBuilder[T]) MustBuildAndStart(ctx ...context.Context) *IntegratedConfigManager {
	manager, err := b.BuildAndStart(ctx...)
	if err != nil {
		panic(fmt.Sprintf("构建并启动配置管理器失败: %v", err))
	}
	return manager
}

// resolveConfigPath 解析配置文件路径
func (b *ManagerBuilder[T]) resolveConfigPath() (string, error) {
	discovery := GetGlobalConfigDiscovery()

	switch {
	case b.usePattern:
		return b.resolveByPattern(discovery)
	case b.useCustomPrefix:
		return b.resolveByPrefix()
	case b.autoDiscovery:
		return b.resolveByAutoDiscovery(discovery)
	case b.configPath != "":
		return b.resolveByDirectPath()
	default:
		return "", ErrNoConfigPath
	}
}

// resolveByPattern 使用模式匹配解析
func (b *ManagerBuilder[T]) resolveByPattern(discovery *ConfigDiscovery) (string, error) {
	configFiles, err := discovery.FindConfigFileByPattern(b.searchPath, b.pattern, b.environment)
	if err != nil {
		return "", ErrFindConfigByPattern(err)
	}
	if len(configFiles) == 0 {
		return "", ErrNoMatchingConfig(b.pattern)
	}

	selectedFile := configFiles[0]
	logger.GetGlobalLogger().Info("🎯 模式匹配找到配置文件: %s (优先级: %d)", selectedFile.Path, selectedFile.Priority)
	return selectedFile.Path, nil
}

// resolveByPrefix 使用自定义前缀解析
func (b *ManagerBuilder[T]) resolveByPrefix() (string, error) {
	// 创建专用的配置发现器（使用全局定义的环境前缀）
	prefixDiscovery := &ConfigDiscovery{
		SupportedExtensions: DefaultSupportedExtensions,
		DefaultNames:        []string{b.configPrefix},
		EnvPrefixes:         DefaultEnvPrefixes,
	}

	configFiles, err := prefixDiscovery.DiscoverConfigFiles(b.searchPath, b.environment)
	if err != nil {
		return "", ErrDiscoverConfigFiles(err)
	}

	for _, file := range configFiles {
		if file.Exists {
			logger.GetGlobalLogger().Info("📛 前缀匹配找到配置文件: %s (前缀: %s, 优先级: %d)",
				file.Path, b.configPrefix, file.Priority)
			return file.Path, nil
		}
	}

	// 找不到配置文件时，打印支持的环境前缀帮助用户排查
	return "", b.buildConfigNotFoundError(b.configPrefix)
}

// resolveByAutoDiscovery 使用自动发现解析
func (b *ManagerBuilder[T]) resolveByAutoDiscovery(discovery *ConfigDiscovery) (string, error) {
	configFiles, err := discovery.DiscoverConfigFiles(b.searchPath, b.environment)
	if err != nil {
		return "", ErrAutoDiscoverConfigFiles(err)
	}

	for _, file := range configFiles {
		if file.Exists {
			logger.GetGlobalLogger().Info("🔍 自动发现配置文件: %s (优先级: %d)", file.Path, file.Priority)
			return file.Path, nil
		}
	}

	// 找不到配置文件时，打印支持的环境前缀帮助用户排查
	return "", b.buildConfigNotFoundError("")
}

// buildConfigNotFoundError 构建配置文件未找到的详细错误信息
func (b *ManagerBuilder[T]) buildConfigNotFoundError(prefix string) error {
	var msg string
	if prefix != "" {
		msg = fmt.Sprintf("未找到前缀为 '%s' 的配置文件", prefix)
	} else {
		msg = fmt.Sprintf("在路径 '%s' 中未找到有效配置文件", b.searchPath)
	}

	// 获取当前环境支持的后缀
	var supportedSuffixes []string
	if suffixes, ok := DefaultEnvPrefixes[b.environment]; ok {
		supportedSuffixes = suffixes
	}

	// 构建详细错误信息
	lines := []string{
		fmt.Sprintf("❌ %s", msg),
		fmt.Sprintf("📍 搜索路径: %s", b.searchPath),
		fmt.Sprintf("🌍 当前环境: %s", b.environment),
	}

	if len(supportedSuffixes) > 0 {
		lines = append(lines, fmt.Sprintf("📋 当前环境支持的配置文件后缀: %v", supportedSuffixes))
		if prefix != "" {
			lines = append(lines, "💡 建议创建以下配置文件之一:")
			for _, suffix := range supportedSuffixes {
				for _, ext := range DefaultSupportedExtensions[:2] { // 只显示 .yaml 和 .yml
					lines = append(lines, fmt.Sprintf("   - %s-%s%s", prefix, suffix, ext))
				}
			}
		}
	} else {
		lines = append(lines,
			fmt.Sprintf("⚠️ 当前环境 '%s' 未在 DefaultEnvPrefixes 中注册", b.environment),
			"📋 已注册的环境及其后缀:",
		)
		for env, suffixes := range DefaultEnvPrefixes {
			lines = append(lines, fmt.Sprintf("   - %s: %v", env, suffixes))
		}
		lines = append(lines,
			"💡 如需注册自定义环境，请在程序启动前注册:",
			"   示例代码:",
			"   func init() {",
			fmt.Sprintf("       boot.RegisterEnvPrefixes(\"%s\", \"%s\", \"custom-alias\")", b.environment, b.environment),
			"   }",
		)
	}
	logger.GetGlobalLogger().ErrorLines(lines...)

	return fmt.Errorf("%s (环境: %s, 搜索路径: %s)", msg, b.environment, b.searchPath)
}

// resolveByDirectPath 使用直接路径解析
func (b *ManagerBuilder[T]) resolveByDirectPath() (string, error) {
	logger.GetGlobalLogger().Info("📁 使用指定配置文件: %s", b.configPath)
	return b.configPath, nil
}

// ConfigBuilderOptions 配置构建器选项
type ConfigBuilderOptions struct {
	ConfigPath      string             `json:"config_path"`       // 配置文件路径
	SearchPath      string             `json:"search_path"`       // 搜索路径
	Environment     EnvironmentType    `json:"environment"`       // 环境类型
	ConfigPrefix    string             `json:"config_prefix"`     // 配置前缀
	Pattern         string             `json:"pattern"`           // 匹配模式
	HotReloadConfig *HotReloadConfig   `json:"hot_reload_config"` // 热重载配置
	ContextOptions  *ContextKeyOptions `json:"context_options"`   // 上下文选项
}

// BuilderFactory 构建器工厂
type BuilderFactory struct {
	defaultOptions ConfigBuilderOptions
}

// NewBuilderFactory 创建新的构建器工厂
func NewBuilderFactory() *BuilderFactory {
	return &BuilderFactory{
		defaultOptions: ConfigBuilderOptions{
			Environment:     GetEnvironment(),
			HotReloadConfig: DefaultHotReloadConfig(),
		},
	}
}

// SetDefaults 设置默认选项
func (f *BuilderFactory) SetDefaults(options ConfigBuilderOptions) *BuilderFactory {
	f.defaultOptions = options
	logger.GetGlobalLogger().Debug("🔧 构建器工厂设置默认选项")
	return f
}

// 全局构建器工厂
var globalBuilderFactory *BuilderFactory
var globalBuilderFactoryOnce sync.Once

// GetGlobalBuilderFactory 获取全局构建器工厂
func GetGlobalBuilderFactory() *BuilderFactory {
	globalBuilderFactoryOnce.Do(func() {
		globalBuilderFactory = NewBuilderFactory()
	})
	return globalBuilderFactory
}

// SetGlobalBuilderFactory 设置全局构建器工厂
func SetGlobalBuilderFactory(factory *BuilderFactory) {
	globalBuilderFactory = factory
	logger.GetGlobalLogger().Debug("🌍 设置全局构建器工厂")
}

// NewManager 创建新的配置管理器构建器（简化API）
func NewManager[T any](config *T) ConfigBuilder[T] {
	return NewConfigBuilder(config)
}

// QuickBuild 快速构建配置管理器的便捷函数
func QuickBuild[T any](config *T, configPath string, env EnvironmentType) (*IntegratedConfigManager, error) {
	return NewConfigBuilder(config).
		WithConfigPath(configPath).
		WithEnvironment(env).
		WithHotReload(DefaultHotReloadConfig()).
		Build()
}

// QuickStart 快速启动配置管理器的便捷函数
func QuickStart[T any](config *T, configPath string, env EnvironmentType) (*IntegratedConfigManager, error) {
	return NewConfigBuilder(config).
		WithConfigPath(configPath).
		WithEnvironment(env).
		WithHotReload(DefaultHotReloadConfig()).
		BuildAndStart()
}
