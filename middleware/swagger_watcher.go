/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-18 16:28:15
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-18 16:28:15
 * @FilePath: \go-rpc-gateway\middleware\swagger_watcher.go
 * @Description: Swagger 文件监听器，支持文件变动自动重载
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/kamalyes/go-rpc-gateway/global"
)

// SwaggerWatcher Swagger 文件监听器
type SwaggerWatcher struct {
	middleware *SwaggerMiddleware
	watcher    *fsnotify.Watcher
	watchPaths []string
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
	debounce   time.Duration
	lastReload time.Time
}

// NewSwaggerWatcher 创建 Swagger 文件监听器
func NewSwaggerWatcher(middleware *SwaggerMiddleware) *SwaggerWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &SwaggerWatcher{
		middleware: middleware,
		ctx:        ctx,
		cancel:     cancel,
		debounce:   2 * time.Second, // 防抖时间，避免频繁重载
	}
}

// Start 启动文件监听
func (w *SwaggerWatcher) Start() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("创建文件监听器失败: %w", err)
	}
	w.watcher = watcher

	// 收集需要监听的文件路径
	if err := w.collectWatchPaths(); err != nil {
		w.watcher.Close()
		return fmt.Errorf("收集监听路径失败: %w", err)
	}

	// 添加监听路径
	for _, path := range w.watchPaths {
		if err := w.watcher.Add(path); err != nil {
			global.LOGGER.Warn("添加监听路径失败: %s, 错误: %v", path, err)
			continue
		}
		global.LOGGER.Info("✅ 开始监听 Swagger 文件: %s", path)
	}

	// 启动监听协程
	go w.watchLoop()

	global.LOGGER.Info("✅ Swagger 文件监听器已启动，监听 %d 个文件", len(w.watchPaths))
	return nil
}

// Stop 停止文件监听
func (w *SwaggerWatcher) Stop() error {
	w.cancel()
	if w.watcher != nil {
		return w.watcher.Close()
	}
	return nil
}

// collectWatchPaths 收集需要监听的文件路径
func (w *SwaggerWatcher) collectWatchPaths() error {
	w.watchPaths = make([]string, 0)

	config := w.middleware.config

	// 单服务模式：监听配置的 Swagger 文件
	if !config.IsAggregateEnabled() {
		w.addPathIfExists(config.SpecPath)
		w.addPathIfExists(config.YamlPath)
		w.addPathIfExists(config.JSONPath)
		return nil
	}

	// 聚合模式：监听所有服务的 Swagger 文件
	if config.Aggregate != nil {
		for _, service := range config.Aggregate.Services {
			if service.Enabled && service.SpecPath != "" {
				w.addPathIfExists(service.SpecPath)
			}
		}
	}

	if len(w.watchPaths) == 0 {
		return fmt.Errorf("没有找到需要监听的 Swagger 文件")
	}

	return nil
}

// addPathIfExists 添加路径（如果存在）
func (w *SwaggerWatcher) addPathIfExists(path string) {
	if path == "" {
		return
	}

	// 转换为绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		global.LOGGER.Warn("无法解析路径: %s, 错误: %v", path, err)
		return
	}

	w.watchPaths = append(w.watchPaths, absPath)
}

// watchLoop 监听循环
func (w *SwaggerWatcher) watchLoop() {
	for {
		select {
		case <-w.ctx.Done():
			global.LOGGER.Info("Swagger 文件监听器已停止")
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleFileEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			global.LOGGER.Error("文件监听错误: %v", err)
		}
	}
}

// handleFileEvent 处理文件事件
func (w *SwaggerWatcher) handleFileEvent(event fsnotify.Event) {
	// 只处理写入和创建事件
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
		return
	}

	// 检查是否是我们监听的文件
	if !w.isWatchedFile(event.Name) {
		return
	}

	global.LOGGER.Info("🔄 检测到 Swagger 文件变动: %s", event.Name)

	// 防抖：避免短时间内多次重载
	w.mu.Lock()
	now := time.Now()
	if now.Sub(w.lastReload) < w.debounce {
		w.mu.Unlock()
		global.LOGGER.Debug("防抖跳过重载（距上次重载 %v）", now.Sub(w.lastReload))
		return
	}
	w.lastReload = now
	w.mu.Unlock()

	// 延迟一小段时间，确保文件写入完成
	time.Sleep(100 * time.Millisecond)

	// 重新加载 Swagger 规范
	if err := w.reloadSwagger(); err != nil {
		global.LOGGER.Error("❌ 重新加载 Swagger 失败: %v", err)
	} else {
		global.LOGGER.Info("✅ Swagger 文件已重新加载")
	}
}

// isWatchedFile 检查是否是监听的文件
func (w *SwaggerWatcher) isWatchedFile(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, watchPath := range w.watchPaths {
		if absPath == watchPath {
			return true
		}
	}
	return false
}

// reloadSwagger 重新加载 Swagger 规范
func (w *SwaggerWatcher) reloadSwagger() error {
	config := w.middleware.config

	// 单服务模式
	if !config.IsAggregateEnabled() {
		return w.middleware.ReloadSwaggerJSON()
	}

	// 聚合模式：重新加载所有服务规范
	return w.middleware.RefreshSpecs()
}
