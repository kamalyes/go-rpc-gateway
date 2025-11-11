/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 02:40:00
 * @FilePath: \go-rpc-gateway\server\monitoring.go
 * @Description: Monitoring 功能实现
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	gomonitoring "github.com/kamalyes/go-config/pkg/monitoring"
)

// EnableMonitoring 启用监控功能（使用配置文件）
func (s *Server) EnableMonitoring() error {
	if s.config.Monitoring.Enabled {
		return s.EnableMonitoringWithConfig(&s.config.Monitoring)
	}
	return nil
}

// EnableMonitoringWithConfig 使用自定义配置启用监控
func (s *Server) EnableMonitoringWithConfig(config *gomonitoring.Monitoring) error {
	if !config.Enabled {
		return nil
	}

	// 创建 MetricsManager（已有实现）
	metricsManager := NewMetricsManager(config)
	
	// 注册 Prometheus metrics 端点
	if config.Metrics != nil && config.Metrics.Enabled {
		endpoint := config.Metrics.Endpoint
		if endpoint == "" {
			endpoint = "/metrics"
		}
		s.RegisterHTTPRoute(endpoint, metricsManager.Handler())
	}

	return nil
}
