/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 15:06:55
 * @FilePath: \go-rpc-gateway\server\middleware_init.go
 * @Description: 中间件管理器初始化模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"time"

	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

// initMiddleware 初始化中间件管理器
func (s *Server) initMiddleware() error {
	// 使用统一的配置系统创建中间件管理器
	manager, err := middleware.NewManager()
	if err != nil {
		return errors.WrapWithContext(err, errors.ErrCodeMiddlewareInitFailed)
	}
	s.middlewareManager = manager

	// 初始化健康检查管理器
	if err := s.initHealthManager(); err != nil {
		return errors.WrapWithContext(err, errors.ErrCodeHealthManagerFailed)
	}

	return nil
}

// initHealthManager 初始化健康检查管理器
func (s *Server) initHealthManager() error {
	// 直接使用配置中的值，默认值已在 go-config 的 Default() 中设置
	healthManager := middleware.NewHealthManager(
		s.configSafe.Field("Name").String("Gateway"),
		s.configSafe.Field("Version").String("v1.0.0"),
	)

	// 添加Redis健康检查 - 简洁的链式调用
	if s.configSafe.IsRedisHealthEnabled() {
		timeout := s.configSafe.GetRedisHealthTimeout(30 * time.Second)
		redisChecker := middleware.NewRedisChecker(timeout)
		healthManager.RegisterChecker(redisChecker)
	}

	// 添加MySQL健康检查 - 简洁的链式调用
	if s.configSafe.IsMySQLHealthEnabled() {
		timeout := s.configSafe.GetMySQLHealthTimeout(30 * time.Second)
		mysqlChecker := middleware.NewMySQLChecker(timeout)
		healthManager.RegisterChecker(mysqlChecker)
	}

	s.healthManager = healthManager
	return nil
}

// initServers 初始化服务器组件
func (s *Server) initServers() error {
	// 初始化gRPC服务器
	if err := s.initGRPCServer(); err != nil {
		return errors.WrapWithContext(err, errors.ErrCodeGRPCServerInitFailed)
	}

	// 初始化HTTP网关
	if err := s.initHTTPGateway(); err != nil {
		return errors.WrapWithContext(err, errors.ErrCodeHTTPGatewayInitFailed)
	}

	return nil
}
