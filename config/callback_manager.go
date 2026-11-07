/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 15:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 15:00:00
 * @FilePath: \go-rpc-gateway\config\callback_manager.go
 * @Description: 通用回调管理组件，提供统一的事件回调机制
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package config

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/kamalyes/go-toolbox/pkg/logger"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// CallbackType 回调类型枚举
type CallbackType string

const (
	CallbackTypeConfigChanged CallbackType = "config_changed" // 配置变更
	CallbackTypeReloaded      CallbackType = "reloaded"       // 重新加载完成
	CallbackTypeError         CallbackType = "error"          // 错误事件
	CallbackTypeStarted       CallbackType = "started"        // 启动事件
	CallbackTypeStopped       CallbackType = "stopped"        // 停止事件
	CallbackTypeEnvironment   CallbackType = "environment"    // 环境变更
	CallbackTypeValidation    CallbackType = "validation"     // 配置验证
	CallbackTypeFileChanged   CallbackType = "file_changed"   // 文件变更
	CallbackTypeEnvVarChanged CallbackType = "envvar_changed" // 环境变量变更
)

// CallbackEvent 回调事件结构
type CallbackEvent struct {
	Type        CallbackType           `json:"type"`        // 事件类型
	Timestamp   time.Time              `json:"timestamp"`   // 事件时间
	Source      string                 `json:"source"`      // 事件源（文件路径、环境变量名等）
	OldValue    interface{}            `json:"old_value"`   // 旧值
	NewValue    interface{}            `json:"new_value"`   // 新值
	Environment EnvironmentType        `json:"environment"` // 当前环境
	Error       error                  `json:"error"`       // 错误信息（仅错误回调）
	Metadata    map[string]interface{} `json:"metadata"`    // 附加元数据
	ConfigPath  string                 `json:"config_path"` // 配置文件路径
	Duration    time.Duration          `json:"duration"`    // 事件处理耗时
}

// CallbackFunc 回调函数类型
type CallbackFunc func(ctx context.Context, event CallbackEvent) error

// CallbackOptions 回调选项配置
type CallbackOptions struct {
	ID       string                   // 回调的唯一标识符
	Types    []CallbackType           // 监听的事件类型
	Priority int                      // 优先级（越小越优先）
	Async    bool                     // 是否异步执行
	Timeout  time.Duration            // 超时时间
	Retry    int                      // 重试次数
	Filter   func(CallbackEvent) bool // 事件过滤器
}

// DefaultCallbackOptions 默认回调选项
func DefaultCallbackOptions() CallbackOptions {
	return CallbackOptions{
		Priority: 0,
		Async:    true,
		Timeout:  30 * time.Second,
		Retry:    3,
		Types:    []CallbackType{CallbackTypeConfigChanged},
	}
}

// callbackInfo 内部回调信息
type callbackInfo struct {
	fn      CallbackFunc
	options CallbackOptions
}

// CallbackManager 回调管理器接口
// 提供统一的回调注册、注销和执行机制
type CallbackManager interface {
	// RegisterCallback 注册回调函数
	RegisterCallback(callback CallbackFunc, options CallbackOptions) error

	// UnregisterCallback 注销回调函数
	UnregisterCallback(id string) error

	// TriggerCallbacks 触发回调函数
	TriggerCallbacks(ctx context.Context, event CallbackEvent) error

	// ListCallbacks 列出所有已注册的回调
	ListCallbacks() []string

	// ClearCallbacks 清空所有回调
	ClearCallbacks()

	// HasCallback 检查是否存在指定ID的回调
	HasCallback(id string) bool
}

// CommonCallbackManager 通用回调管理器实现
type CommonCallbackManager struct {
	mu        sync.RWMutex
	callbacks map[string]*callbackInfo
}

// NewCallbackManager 创建新的回调管理器
func NewCallbackManager() CallbackManager {
	return &CommonCallbackManager{
		callbacks: make(map[string]*callbackInfo),
	}
}

// RegisterCallback 注册回调函数
func (cm *CommonCallbackManager) RegisterCallback(callback CallbackFunc, options CallbackOptions) error {
	if callback == nil {
		return ErrCallbackFuncNil
	}

	if options.ID == "" {
		return ErrCallbackIDEmpty
	}

	if len(options.Types) == 0 {
		options.Types = []CallbackType{CallbackTypeConfigChanged}
	}

	if options.Timeout == 0 {
		options.Timeout = 30 * time.Second
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.callbacks[options.ID]; exists {
		return fmt.Errorf("ID为 %s 的回调已存在", options.ID)
	}

	cm.callbacks[options.ID] = &callbackInfo{
		fn:      callback,
		options: options,
	}

	logger.GetGlobalLogger().Debug("✅ 回调已注册: %s (类型: %v, 优先级: %d)",
		options.ID, options.Types, options.Priority)

	return nil
}

// UnregisterCallback 注销回调函数
func (cm *CommonCallbackManager) UnregisterCallback(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.callbacks[id]; !exists {
		return fmt.Errorf("ID为 %s 的回调不存在", id)
	}

	delete(cm.callbacks, id)
	logger.GetGlobalLogger().Debug("🗑️ 回调已注销: %s", id)

	return nil
}

// TriggerCallbacks 触发回调函数
func (cm *CommonCallbackManager) TriggerCallbacks(ctx context.Context, event CallbackEvent) error {
	cm.mu.RLock()

	// 筛选匹配的回调
	var matchedCallbacks []*callbackInfo
	for _, info := range cm.callbacks {
		if cm.shouldTrigger(info, event) {
			matchedCallbacks = append(matchedCallbacks, info)
		}
	}
	cm.mu.RUnlock()

	if len(matchedCallbacks) == 0 {
		return nil
	}

	// 按优先级排序
	sort.Slice(matchedCallbacks, func(i, j int) bool {
		return matchedCallbacks[i].options.Priority < matchedCallbacks[j].options.Priority
	})

	// 执行回调
	var wg sync.WaitGroup
	errChan := make(chan error, len(matchedCallbacks))

	for _, info := range matchedCallbacks {
		if info.options.Async {
			wg.Add(1)
			currentInfo := info // 捕获循环变量
			syncx.Go(ctx).
				OnPanic(func(r any) {
					logger.GetGlobalLogger().ErrorContext(ctx, "回调执行时发生panic: %v", r)
				}).
				Exec(func() {
					defer wg.Done()
					if err := cm.executeCallback(ctx, currentInfo, event); err != nil {
						errChan <- err
					}
				})
		} else {
			if err := cm.executeCallback(ctx, info, event); err != nil {
				errChan <- err
			}
		}
	}

	// 等待异步回调完成
	if len(matchedCallbacks) > 0 {
		syncx.Go().Exec(func() {
			wg.Wait()
			close(errChan)
		})
	} else {
		close(errChan)
	}

	// 收集错误
	var errors []error
	for err := range errChan {
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("执行回调时发生 %d 个错误: %v", len(errors), errors)
	}

	return nil
}

// shouldTrigger 判断是否应该触发回调
func (cm *CommonCallbackManager) shouldTrigger(info *callbackInfo, event CallbackEvent) bool {
	// 检查事件类型匹配
	typeMatched := false
	for _, eventType := range info.options.Types {
		if eventType == event.Type {
			typeMatched = true
			break
		}
	}

	if !typeMatched {
		return false
	}

	// 应用过滤器
	if info.options.Filter != nil && !info.options.Filter(event) {
		return false
	}

	return true
}

// executeCallback 执行回调函数，包含重试机制
func (cm *CommonCallbackManager) executeCallback(ctx context.Context, info *callbackInfo, event CallbackEvent) error {
	var lastErr error

	for attempt := 0; attempt <= info.options.Retry; attempt++ {
		// 创建超时上下文
		callbackCtx, cancel := context.WithTimeout(ctx, info.options.Timeout)

		// 执行回调
		done := make(chan error, 1)
		syncx.Go(callbackCtx).
			OnPanic(func(r any) {
				done <- fmt.Errorf("回调函数panic: %v", r)
			}).
			Exec(func() {
				done <- info.fn(callbackCtx, event)
			})

		select {
		case err := <-done:
			cancel()
			if err == nil {
				if attempt > 0 {
					logger.GetGlobalLogger().InfoContext(ctx, "✅ 回调 %s 在第 %d 次重试后成功", info.options.ID, attempt)
				}
				return nil
			}
			lastErr = err
			if attempt < info.options.Retry {
				logger.GetGlobalLogger().WarnContext(ctx, "⚠️ 回调 %s 执行失败，准备重试 (尝试 %d/%d): %v",
					info.options.ID, attempt+1, info.options.Retry+1, err)
				time.Sleep(time.Duration(attempt+1) * time.Second) // 递增延迟
			}
		case <-callbackCtx.Done():
			cancel()
			lastErr = fmt.Errorf("回调超时")
			if attempt < info.options.Retry {
				logger.GetGlobalLogger().WarnContext(ctx, "⏰ 回调 %s 执行超时，准备重试 (尝试 %d/%d)",
					info.options.ID, attempt+1, info.options.Retry+1)
			}
		}
	}

	logger.GetGlobalLogger().ErrorContext(ctx, "❌ 回调 %s 执行失败，已用尽所有重试: %v", info.options.ID, lastErr)
	return fmt.Errorf("回调 %s 执行失败: %w", info.options.ID, lastErr)
}

// ListCallbacks 列出所有已注册的回调
func (cm *CommonCallbackManager) ListCallbacks() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var ids []string
	for id := range cm.callbacks {
		ids = append(ids, id)
	}

	return ids
}

// ClearCallbacks 清空所有回调
func (cm *CommonCallbackManager) ClearCallbacks() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	count := len(cm.callbacks)
	cm.callbacks = make(map[string]*callbackInfo)

	logger.GetGlobalLogger().Info("🧹 已清空所有回调 (共 %d 个)", count)
}

// HasCallback 检查是否存在指定ID的回调
func (cm *CommonCallbackManager) HasCallback(id string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	_, exists := cm.callbacks[id]
	return exists
}

// GetCallbackInfo 获取回调信息（用于调试）
func (cm *CommonCallbackManager) GetCallbackInfo(id string) (CallbackOptions, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if info, exists := cm.callbacks[id]; exists {
		return info.options, true
	}

	return CallbackOptions{}, false
}

// GetCallbackCount 获取回调总数
func (cm *CommonCallbackManager) GetCallbackCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return len(cm.callbacks)
}

// CreateEvent 创建回调事件的辅助函数
func CreateEvent(eventType CallbackType, source string, oldValue, newValue interface{}) CallbackEvent {
	return CallbackEvent{
		Type:        eventType,
		Timestamp:   time.Now(),
		Source:      source,
		OldValue:    oldValue,
		NewValue:    newValue,
		Environment: GetEnvironment(), // 自动获取当前环境
		Metadata:    make(map[string]interface{}),
		ConfigPath:  "", // 可以后续通过WithMetadata设置
		Duration:    0,  // 可以后续通过WithMetadata设置
	}
}

// CreateErrorEvent 创建错误事件的辅助函数
func CreateErrorEvent(source string, err error) CallbackEvent {
	return CallbackEvent{
		Type:        CallbackTypeError,
		Timestamp:   time.Now(),
		Source:      source,
		Error:       err,
		Environment: GetEnvironment(),
		Metadata:    make(map[string]interface{}),
	}
}

// WithMetadata 为事件添加元数据
func (e *CallbackEvent) WithMetadata(key string, value interface{}) *CallbackEvent {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value

	// 特殊处理一些常用字段
	switch key {
	case "duration":
		if d, ok := value.(time.Duration); ok {
			e.Duration = d
		}
	case "config_path":
		if path, ok := value.(string); ok {
			e.ConfigPath = path
		}
	case "environment":
		if env, ok := value.(EnvironmentType); ok {
			e.Environment = env
		}
	}

	return e
}

// GetMetadata 获取事件元数据
func (e *CallbackEvent) GetMetadata(key string) (interface{}, bool) {
	if e.Metadata == nil {
		return nil, false
	}
	value, exists := e.Metadata[key]
	return value, exists
}
