/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 15:15:03
 * @FilePath: \go-rpc-gateway\server\health.go
 * @Description: Health 功能实现
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import goconfig "github.com/kamalyes/go-config"

// EnableHealth 启用健康检查功能（使用配置文件）
func (s *Server) EnableHealth() error {
	configSafe := goconfig.SafeConfig(s.config)
	if configSafe.IsHealthEnabled() {
		return s.EnableHealthWithConfig()
	}
	return nil
}

// EnableHealthWithConfig 使用自定义配置启用健康检查
func (s *Server) EnableHealthWithConfig() error {
	configSafe := goconfig.SafeConfig(s.config)
	if !configSafe.IsHealthEnabled() {
		return nil
	}

	// healthManager 在 initHealthManager 中已经初始化
	// 这里只需要注册路由
	if s.healthManager != nil {
		// 注册主健康检查端点
		path := configSafe.GetHealthPath("/health")
		if path == "" {
			path = "/health"
		}
		s.RegisterHTTPRoute(path, s.healthManager.HTTPHandler())

		// 注册 Redis 健康检查端点
		if configSafe.IsRedisHealthEnabled() {
			redisPath := configSafe.GetRedisHealthPath("/health/redis")
			if redisPath == "" {
				redisPath = "/health/redis"
			}
			s.RegisterHTTPRoute(redisPath, s.healthManager.HTTPHandler())
		}

		// 注册 MySQL 健康检查端点
		if configSafe.IsMySQLHealthEnabled() {
			mysqlPath := configSafe.GetMySQLHealthPath("/health/mysql")
			if mysqlPath == "" {
				mysqlPath = "/health/mysql"
			}
			s.RegisterHTTPRoute(mysqlPath, s.healthManager.HTTPHandler())
		}
	}

	return nil
}
