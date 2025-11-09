/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 13:40:31
 * @FilePath: \go-rpc-gateway\server\middleware.go
 * @Description: 中间件管理器初始化模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"fmt"

	"github.com/kamalyes/go-rpc-gateway/middleware"
)

// initMiddleware 初始化中间件管理器
func (s *Server) initMiddleware() error {
	var metricsConfig *middleware.MetricsConfig
	var tracingConfig *middleware.TracingConfig

	// 配置监控中间件
	if s.config.Monitoring.Metrics.Enabled {
		metricsConfig = &middleware.MetricsConfig{
			Enabled:   true,
			Namespace: s.config.Monitoring.Metrics.Namespace,
			Subsystem: s.config.Monitoring.Metrics.Subsystem,
		}
	}

	// 配置链路追踪中间件
	if s.config.Monitoring.Tracing.Enabled {
		tracingConfig = &middleware.TracingConfig{
			Enabled:     true,
			ServiceName: s.config.Monitoring.Tracing.ServiceName,
		}
	}

	manager, err := middleware.NewManager(metricsConfig, tracingConfig)
	if err != nil {
		return fmt.Errorf("failed to create middleware manager: %w", err)
	}

	s.middlewareManager = manager
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
