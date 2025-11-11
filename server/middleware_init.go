/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 20:05:34
 * @FilePath: \go-rpc-gateway\server\middleware_init.go
 * @Description: 中间件管理器初始化模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"fmt"
	"time"

	"github.com/kamalyes/go-rpc-gateway/middleware"
)

// initMiddleware 初始化中间件管理器
func (s *Server) initMiddleware() error {
	// 使用统一的配置系统创建中间件管理器
	manager, err := middleware.NewManager(s.config)
	if err != nil {
		return fmt.Errorf("failed to create middleware manager: %w", err)
	}
	s.middlewareManager = manager

	// 初始化健康检查管理器
	if err := s.initHealthManager(); err != nil {
		return fmt.Errorf("failed to create health manager: %w", err)
	}

	return nil
}

// initHealthManager 初始化健康检查管理器
func (s *Server) initHealthManager() error {
	// 直接使用配置中的值，默认值已在 go-config 的 Default() 中设置
	healthManager := middleware.NewHealthManager(
		s.config.Gateway.Name,
		s.config.Gateway.Version,
	)

	// 添加Redis健康检查
	if s.config.Gateway.Health.Redis.Enabled {
		redisChecker := middleware.NewRedisChecker(
			time.Duration(s.config.Gateway.Health.Redis.Timeout) * time.Second,
		)
		healthManager.RegisterChecker(redisChecker)
	}

	// 添加MySQL健康检查
	if s.config.Gateway.Health.MySQL.Enabled {
		mysqlChecker := middleware.NewMySQLChecker(
			time.Duration(s.config.Gateway.Health.MySQL.Timeout) * time.Second,
		)
		healthManager.RegisterChecker(mysqlChecker)
	}

	s.healthManager = healthManager
	return nil
}

// initServers 初始化服务器组件
func (s *Server) initServers() error {
	// 初始化gRPC服务器
	if err := s.initGRPCServer(); err != nil {
		return fmt.Errorf("failed to init gRPC server: %w", err)
	}

	// 初始化HTTP网关
	if err := s.initHTTPGateway(); err != nil {
		return fmt.Errorf("failed to init HTTP gateway: %w", err)
	}

	return nil
}
