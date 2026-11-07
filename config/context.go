/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 11:15:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 11:31:57
 * @FilePath: \go-rpc-gateway\config\context.go
 * @Description: 上下文集成模块，将环境变量和配置信息集成到上下文中
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package config

import (
	"context"
	"sync"
	"time"

	"github.com/kamalyes/go-toolbox/pkg/contextx"
	"github.com/kamalyes/go-toolbox/pkg/logger"
)

// 上下文键定义
const (
	ContextKeyEnvironment ContextKey = "go_config_environment"
	ContextKeyConfig      ContextKey = "go_config_configuration"
	ContextKeyHotReloader ContextKey = "go_config_hot_reloader"
	ContextKeyMetadata    ContextKey = "go_config_metadata"
)

// ConfigContext 配置上下文
type ConfigContext struct {
	Environment EnvironmentType `json:"environment"` // 当前环境
	Config      any             `json:"config"`      // 配置对象
	Metadata    map[string]any  `json:"metadata"`    // 元数据
	CreatedAt   time.Time       `json:"created_at"`  // 创建时间
	UpdatedAt   time.Time       `json:"updated_at"`  // 更新时间
}

// ContextManager 上下文管理器
type ContextManager struct {
	mu          sync.RWMutex
	environment *Environment
	hotReloader HotReloader
	configCtx   *ConfigContext
	contextPool sync.Pool
}

// NewContextManager 创建新的上下文管理器
func NewContextManager(env *Environment, reloader HotReloader) *ContextManager {
	manager := &ContextManager{
		environment: env,
		hotReloader: reloader,
		configCtx: &ConfigContext{
			Environment: env.Value,
			Config:      nil,
			Metadata:    make(map[string]any),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		contextPool: sync.Pool{
			New: func() any {
				return context.Background()
			},
		},
	}

	// 注册环境变更回调
	env.RegisterCallback("context_manager", manager.onEnvironmentChanged, 0, false)

	// 注册配置变更回调
	if reloader != nil {
		reloader.RegisterCallback(manager.onConfigChanged, CallbackOptions{
			ID:       "context_manager",
			Types:    []CallbackType{CallbackTypeConfigChanged, CallbackTypeReloaded},
			Priority: 0,
			Async:    false,
		})
	}

	return manager
}

// onEnvironmentChanged 环境变更回调
func (cm *ContextManager) onEnvironmentChanged(oldEnv, newEnv EnvironmentType) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	logger.GetGlobalLogger().DebugKV("上下文管理器接收到环境变更", "oldEnv", oldEnv, "newEnv", newEnv)

	cm.configCtx.Environment = newEnv
	cm.configCtx.UpdatedAt = time.Now()
	cm.configCtx.Metadata["last_env_change"] = map[string]any{
		"old":       oldEnv,
		"new":       newEnv,
		"timestamp": time.Now(),
	}

	return nil
}

// onConfigChanged 配置变更回调
func (cm *ContextManager) onConfigChanged(ctx context.Context, event CallbackEvent) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	logger.GetGlobalLogger().DebugKV("上下文管理器接收到配置变更", "type", event.Type)

	cm.configCtx.Config = event.NewValue
	cm.configCtx.UpdatedAt = time.Now()
	cm.configCtx.Metadata["last_config_change"] = map[string]any{
		"type":      event.Type,
		"source":    event.Source,
		"timestamp": event.Timestamp,
	}

	return nil
}

// WithConfig 将配置信息添加到上下文中
func (cm *ContextManager) WithConfig(ctx context.Context) context.Context {
	cm.mu.RLock()
	configCtx := *cm.configCtx // 复制当前配置上下文
	cm.mu.RUnlock()

	// 添加配置信息到上下文
	ctx = context.WithValue(ctx, ContextKeyEnvironment, configCtx.Environment)
	ctx = context.WithValue(ctx, ContextKeyConfig, configCtx.Config)
	ctx = context.WithValue(ctx, ContextKeyMetadata, configCtx.Metadata)

	if cm.hotReloader != nil {
		ctx = context.WithValue(ctx, ContextKeyHotReloader, cm.hotReloader)
	}

	return ctx
}

// GetEnvironmentFromContext 从上下文中获取环境信息
func GetEnvironmentFromContext(ctx context.Context) (EnvironmentType, bool) {
	env, ok := ctx.Value(ContextKeyEnvironment).(EnvironmentType)
	return env, ok
}

// GetConfigFromContext 从上下文中获取配置信息
func GetConfigFromContext(ctx context.Context) (any, bool) {
	config := ctx.Value(ContextKeyConfig)
	if config == nil {
		return nil, false
	}
	return config, true
}

// GetHotReloaderFromContext 从上下文中获取热更新器
func GetHotReloaderFromContext(ctx context.Context) (HotReloader, bool) {
	reloader, ok := ctx.Value(ContextKeyHotReloader).(HotReloader)
	return reloader, ok
}

// GetMetadataFromContext 从上下文中获取元数据
func GetMetadataFromContext(ctx context.Context) (map[string]any, bool) {
	metadata, ok := ctx.Value(ContextKeyMetadata).(map[string]any)
	return metadata, ok
}

// UpdateConfig 更新配置
func (cm *ContextManager) UpdateConfig(config any) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.configCtx.Config = config
	cm.configCtx.UpdatedAt = time.Now()
}

// GetCurrentConfig 获取当前配置
func (cm *ContextManager) GetCurrentConfig() any {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.configCtx.Config
}

// GetCurrentEnvironment 获取当前环境
func (cm *ContextManager) GetCurrentEnvironment() EnvironmentType {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.configCtx.Environment
}

// SetMetadata 设置元数据
func (cm *ContextManager) SetMetadata(key string, value any) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.configCtx.Metadata[key] = value
	cm.configCtx.UpdatedAt = time.Now()
}

// GetMetadata 获取元数据
func (cm *ContextManager) GetMetadata(key string) (any, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	value, exists := cm.configCtx.Metadata[key]
	return value, exists
}

// GetConfigContext 获取完整的配置上下文
func (cm *ContextManager) GetConfigContext() *ConfigContext {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 返回副本避免外部修改
	configCtx := *cm.configCtx
	// 深度复制元数据映射
	configCtx.Metadata = make(map[string]any, len(cm.configCtx.Metadata))
	for k, v := range cm.configCtx.Metadata {
		configCtx.Metadata[k] = v
	}

	return &configCtx
}

// 全局上下文管理器实例
var (
	globalContextManager   *ContextManager
	globalContextManagerMu sync.Mutex
)

// InitializeContextManager 初始化全局上下文管理器
func InitializeContextManager(env *Environment, reloader HotReloader) *ContextManager {
	globalContextManagerMu.Lock()
	defer globalContextManagerMu.Unlock()
	if globalContextManager == nil {
		globalContextManager = NewContextManager(env, reloader)
		logger.GetGlobalLogger().Info("全局上下文管理器初始化完成")
	}
	return globalContextManager
}

// GetGlobalContextManager 获取全局上下文管理器
func GetGlobalContextManager() *ContextManager {
	globalContextManagerMu.Lock()
	defer globalContextManagerMu.Unlock()
	return globalContextManager
}

// ClearGlobalContextManager 清理全局上下文管理器（主要用于测试）
func ClearGlobalContextManager() {
	globalContextManagerMu.Lock()
	defer globalContextManagerMu.Unlock()
	globalContextManager = nil
	logger.GetGlobalLogger().Info("全局上下文管理器已清理")
}

// WithGlobalConfig 使用全局上下文管理器将配置添加到上下文中
func WithGlobalConfig(ctx context.Context) context.Context {
	manager := GetGlobalContextManager()
	if manager == nil {
		logger.GetGlobalLogger().Warn("全局上下文管理器未初始化，返回原始上下文")
		return ctx
	}

	return manager.WithConfig(ctx)
}

// ContextKeyHelper 上下文键辅助工具
type ContextKeyHelper struct{}

// NewContextWithTimeout 创建带超时的配置上下文
func (ContextKeyHelper) NewContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return contextx.WithTimeoutDecorators(timeout, WithGlobalConfig)
}

// NewContextWithDeadline 创建带截止时间的配置上下文
func (ContextKeyHelper) NewContextWithDeadline(deadline time.Time) (context.Context, context.CancelFunc) {
	return contextx.WithDeadlineDecorators(deadline, WithGlobalConfig)
}

// NewConfigContext 创建新的配置上下文
func (ContextKeyHelper) NewConfigContext() context.Context {
	return WithGlobalConfig(context.Background())
}

// IsEnvironment 检查上下文中的环境是否匹配
func (ContextKeyHelper) IsEnvironment(ctx context.Context, env EnvironmentType) bool {
	if contextEnv, ok := GetEnvironmentFromContext(ctx); ok {
		return contextEnv == env
	}
	return false
}

// MustGetConfig 从上下文中获取配置，如果不存在则panic
func (ContextKeyHelper) MustGetConfig(ctx context.Context) any {
	config, ok := GetConfigFromContext(ctx)
	if !ok {
		panic("配置信息不存在于上下文中")
	}
	return config
}

// MustGetEnvironment 从上下文中获取环境，如果不存在则panic
func (ContextKeyHelper) MustGetEnvironment(ctx context.Context) EnvironmentType {
	env, ok := GetEnvironmentFromContext(ctx)
	if !ok {
		panic("环境信息不存在于上下文中")
	}
	return env
}

// 创建全局上下文辅助工具实例
var ContextHelper = ContextKeyHelper{}
