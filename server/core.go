/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 22:15:00
 * @FilePath: \go-rpc-gateway\server\core.go
 * @Description: 核心组件初始化模块，集成企业级组件和go-logger
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"fmt"

	"github.com/kamalyes/go-rpc-gateway/cpool"
	"github.com/kamalyes/go-rpc-gateway/global"
)

// initCore 初始化核心组件，集成企业级组件
func (s *Server) initCore() error {
	// 注意：全局配置和日志已经由ConfigManager初始化，这里不再重复初始化

	// 创建并初始化连接池管理器（注入logger）
	poolManager := cpool.NewManager(global.LOGGER)
	if err := poolManager.Initialize(s.ctx, s.config); err != nil {
		return fmt.Errorf("failed to initialize connection pool manager: %w", err)
	}

	// 将连接池管理器保存到服务器中
	s.poolManager = poolManager

	return nil
}

// 注意：配置变更回调已移除，配置管理现在由 go-config 负责
