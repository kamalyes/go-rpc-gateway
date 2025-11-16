/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 15:00:50
 * @FilePath: \go-rpc-gateway\server\monitoring.go
 * @Description: Monitoring 功能实现
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

// EnableMonitoring 启用监控功能（使用配置文件）
func (s *Server) EnableMonitoring() error {
	return mathx.IF(s.configSafe.IsMonitoringEnabled(),
		s.EnableMonitoringWithConfig(),
		nil)
}

// EnableMonitoringWithConfig 使用自定义配置启用监控
func (s *Server) EnableMonitoringWithConfig() error {
	if !s.configSafe.IsMonitoringEnabled() {
		return nil
	}

	// 创建 MetricsManager（已有实现）
	metricsManager := NewMetricsManager(s.config.Monitoring)

	// 注册 Prometheus metrics 端点
	if s.configSafe.IsMetricsEnabled() {
		endpoint := s.configSafe.GetMetricsEndpoint("/metrics")
		s.RegisterHTTPRoute(endpoint, metricsManager.Handler())
	}

	return nil
}
