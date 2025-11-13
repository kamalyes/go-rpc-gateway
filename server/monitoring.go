/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 14:08:45
 * @FilePath: \go-rpc-gateway\server\monitoring.go
 * @Description: Monitoring 功能实现
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	goconfig "github.com/kamalyes/go-config"
)

// EnableMonitoring 启用监控功能（使用配置文件）
func (s *Server) EnableMonitoring() error {
	configSafe := goconfig.SafeConfig(s.config)
	if configSafe.IsMonitoringEnabled() {
		return s.EnableMonitoringWithConfig()
	}
	return nil
}

// EnableMonitoringWithConfig 使用自定义配置启用监控
func (s *Server) EnableMonitoringWithConfig() error {
	configSafe := goconfig.SafeConfig(s.config)

	if !configSafe.IsMonitoringEnabled() {
		return nil
	}

	// 创建 MetricsManager（已有实现）
	// 注意：这里需要传递原始config对象，因为NewMetricsManager可能需要完整结构
	metricsManager := NewMetricsManager(s.config.Monitoring)

	// 注册 Prometheus metrics 端点
	if configSafe.IsMetricsEnabled() {
		endpoint := configSafe.GetMetricsEndpoint("/metrics")
		s.RegisterHTTPRoute(endpoint, metricsManager.Handler())
	}

	return nil
}
