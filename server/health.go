/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 15:10:05
 * @FilePath: \go-rpc-gateway\server\health.go
 * @Description: Health 功能实现
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

// EnableHealth 启用健康检查功能（使用配置文件）
func (s *Server) EnableHealth() error {
	return mathx.IF(s.configSafe.IsHealthEnabled(),
		s.EnableHealthWithConfig(),
		nil)
}

// EnableHealthWithConfig 使用自定义配置启用健康检查
func (s *Server) EnableHealthWithConfig() error {
	if !s.configSafe.IsHealthEnabled() {
		return nil
	}

	// healthManager 在 initHealthManager 中已经初始化
	// 这里只需要注册路由
	if s.healthManager != nil {
		// 注册主健康检查端点
		path := s.configSafe.GetHealthPath("/health")
		if path == "" {
			path = "/health"
		}
		s.RegisterHTTPRoute(path, s.healthManager.HTTPHandler())

		// 注册 Redis 健康检查端点
		if s.configSafe.IsRedisHealthEnabled() {
			redisPath := s.configSafe.GetRedisHealthPath("/health/redis")
			if redisPath == "" {
				redisPath = "/health/redis"
			}
			s.RegisterHTTPRoute(redisPath, s.healthManager.HTTPHandler())
		}

		// 注册 MySQL 健康检查端点
		if s.configSafe.IsMySQLHealthEnabled() {
			mysqlPath := s.configSafe.GetMySQLHealthPath("/health/mysql")
			if mysqlPath == "" {
				mysqlPath = "/health/mysql"
			}
			s.RegisterHTTPRoute(mysqlPath, s.healthManager.HTTPHandler())
		}
	}

	return nil
}
