/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-23 23:59:26
 * @FilePath: go-rpc-gateway\server\core.go
 * @Description: 核心组件初始化模块，集成企业级组件和go-logger
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	wscconfig "github.com/kamalyes/go-config/pkg/wsc"
	"github.com/kamalyes/go-rpc-gateway/cpool"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
)

// initCore 初始化核心组件，集成企业级组件
func (s *Server) initCore() error {
	// 注意：全局配置和日志已经由ConfigManager初始化，这里不再重复初始化

	// 创建并初始化连接池管理器（注入logger）
	poolManager := cpool.NewManager(global.LOGGER)
	if err := poolManager.Initialize(s.ctx, s.config); err != nil {
		return errors.NewErrorf(errors.ErrCodeInternalServerError, "failed to initialize connection pool manager: %v", err)
	}

	// 将连接池管理器保存到服务器中
	s.poolManager = poolManager

	// 初始化端点收集器
	s.endpointCollector = NewEndpointCollector()
	global.LOGGER.InfoMsg("✅ 端点收集器已初始化")

	// 初始化 WebSocket 服务（如果启用）
	if err := s.initWebSocket(); err != nil {
		global.LOGGER.WithError(err).WarnMsg("WebSocket 服务初始化失败，将跳过启动")
		// 注意：不返回错误，允许系统在没有 WebSocket 的情况下继续运行
	}

	return nil
}

// initWebSocket 初始化 WebSocket 服务
func (s *Server) initWebSocket() error {
	// 使用安全访问方式获取 WSC 配置（与其他配置访问保持一致）
	wscSafe := s.configSafe.Field("WSC")

	// 检查 WebSocket 是否启用
	isEnabled := wscSafe.Field("Enabled").Bool(false)
	if !isEnabled {
		global.LOGGER.DebugMsg("WebSocket 服务未启用，跳过初始化")
		return nil
	}

	// 获取完整的 WSC 配置
	if s.config.WSC == nil {
		global.LOGGER.DebugMsg("WebSocket 配置为空，使用默认配置")
		s.config.WSC = wscconfig.Default()
	}

	// 创建 WebSocket 服务
	wsSvc, err := NewWebSocketService(s.config.WSC)
	if err != nil {
		return errors.NewErrorf(errors.ErrCodeInternalServerError, "failed to create WebSocket service: %v", err)
	}

	s.webSocketService = wsSvc
	return nil
}

// 注意：配置变更回调已移除，配置管理现在由 go-config 负责
