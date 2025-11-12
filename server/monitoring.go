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

// EnableMonitoring 启用监控功能（使用配置文件）
func (s *Server) EnableMonitoring() error {
	if s.config.Monitoring.Enabled {
		return s.EnableMonitoringWithConfig()
	}
	return nil
}

// EnableMonitoringWithConfig 使用自定义配置启用监控
func (s *Server) EnableMonitoringWithConfig() error {
	if !s.config.Monitoring.Enabled {
		return nil
	}

	// 创建 MetricsManager（已有实现）
	metricsManager := NewMetricsManager(s.config.Monitoring)

	// 注册 Prometheus metrics 端点
	if s.config.Monitoring.Metrics != nil && s.config.Monitoring.Metrics.Enabled {
		endpoint := s.config.Monitoring.Metrics.Endpoint
		if endpoint == "" {
			endpoint = "/metrics"
		}
		s.RegisterHTTPRoute(endpoint, metricsManager.Handler())
	}

	return nil
}
