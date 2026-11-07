/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\watcher.go
 * @Description: 文件监听器 - 热重载、防抖、路径收集
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	serrors "github.com/kamalyes/go-rpc-gateway/middleware/swagger/errors"
	errorx "github.com/kamalyes/go-toolbox/pkg/errorx"
)

type Watcher struct {
	middleware *Middleware
	watcher    *fsnotify.Watcher
	watchPaths []string
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
	debounce   time.Duration
	lastReload time.Time
}

func NewWatcher(middleware *Middleware) *Watcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Watcher{
		middleware: middleware,
		ctx:        ctx,
		cancel:     cancel,
		debounce:   2 * time.Second,
	}
}

func (w *Watcher) Start() error {
	fswatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return errorx.NewError(serrors.ErrTypeWatcherStartFailed, err)
	}
	w.watcher = fswatcher

	if err := w.collectWatchPaths(); err != nil {
		w.watcher.Close()
		return errorx.NewError(serrors.ErrTypeWatcherStartFailed, err)
	}

	for _, path := range w.watchPaths {
		if err := w.watcher.Add(path); err != nil {
			w.middleware.logger.Warn("添加监听路径失败: %s, 错误: %v", path, err)
			continue
		}
		w.middleware.logger.Info("开始监听 Swagger 文件: %s", path)
	}

	go w.watchLoop()
	w.middleware.logger.Info("Swagger 文件监听器已启动，监听 %d 个文件", len(w.watchPaths))
	return nil
}

func (w *Watcher) Stop() error {
	w.cancel()
	if w.watcher != nil {
		return w.watcher.Close()
	}
	return nil
}

func (w *Watcher) collectWatchPaths() error {
	w.watchPaths = make([]string, 0)
	config := w.middleware.config

	if !config.IsAggregateEnabled() {
		w.addPathIfExists(config.SpecPath)
		w.addPathIfExists(config.YamlPath)
		w.addPathIfExists(config.JSONPath)
		return nil
	}

	if config.Aggregate != nil {
		for _, service := range config.Aggregate.Services {
			if service.Enabled && service.SpecPath != "" {
				w.addPathIfExists(service.SpecPath)
			}
		}
	}

	if len(w.watchPaths) == 0 {
		return errorx.NewError(serrors.ErrTypeWatcherNoFiles)
	}
	return nil
}

func (w *Watcher) addPathIfExists(path string) {
	if path == "" {
		return
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		w.middleware.logger.Warn("无法解析路径: %s, 错误: %v", path, err)
		return
	}
	w.watchPaths = append(w.watchPaths, absPath)
}

func (w *Watcher) watchLoop() {
	for {
		select {
		case <-w.ctx.Done():
			w.middleware.logger.Info("Swagger 文件监听器已停止")
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
			w.middleware.logger.Error("文件监听错误: %v", err)
		}
	}
}

func (w *Watcher) handleFileEvent(event fsnotify.Event) {
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
		return
	}
	if !w.isWatchedFile(event.Name) {
		return
	}

	w.middleware.logger.Info("检测到 Swagger 文件变动: %s", event.Name)

	w.mu.Lock()
	now := time.Now()
	if now.Sub(w.lastReload) < w.debounce {
		w.mu.Unlock()
		w.middleware.logger.Debug("防抖跳过重载（距上次重载 %v）", now.Sub(w.lastReload))
		return
	}
	w.lastReload = now
	w.mu.Unlock()

	time.Sleep(100 * time.Millisecond)
	if err := w.reloadSwagger(); err != nil {
		w.middleware.logger.Error("重新加载 Swagger 失败: %v", err)
	} else {
		w.middleware.logger.Info("Swagger 文件已重新加载")
	}
}

func (w *Watcher) isWatchedFile(path string) bool {
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

func (w *Watcher) reloadSwagger() error {
	config := w.middleware.config
	if !config.IsAggregateEnabled() {
		return w.middleware.ReloadSwaggerJSON()
	}
	return w.middleware.RefreshSpecs()
}

func (m *Middleware) EnableFileWatcher() error {
	if m.watcher != nil {
		return errorx.NewError(serrors.ErrTypeWatcherAlreadyStart)
	}
	w := NewWatcher(m)
	if err := w.Start(); err != nil {
		return errorx.NewError(serrors.ErrTypeWatcherStartFailed, err)
	}
	m.watcher = w
	m.logger.Info("Swagger 文件热重载已启用")
	return nil
}

func (m *Middleware) DisableFileWatcher() error {
	if m.watcher == nil {
		return nil
	}
	if err := m.watcher.Stop(); err != nil {
		return errorx.NewError(serrors.ErrTypeWatcherStopFailed, err)
	}
	m.watcher = nil
	m.logger.Info("Swagger 文件监听器已停止")
	return nil
}

func (m *Middleware) GetWatchPaths() []string {
	if m.watcher == nil {
		return nil
	}
	return m.watcher.watchPaths
}

func (m *Middleware) IsWatcherRunning() bool {
	return m.watcher != nil
}

func (m *Middleware) FormatWatchPaths() string {
	paths := m.GetWatchPaths()
	if len(paths) == 0 {
		return "无监听文件"
	}
	return fmt.Sprintf("监听 %d 个文件: %v", len(paths), paths)
}
