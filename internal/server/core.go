/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 18:36:15
 * @FilePath: \go-rpc-gateway\internal\server\core.go
 * @Description: 核心组件初始化模块，包括数据库、Redis、日志等
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"fmt"

	"github.com/kamalyes/go-core/pkg/global"
	gocoreZap "github.com/kamalyes/go-core/pkg/zap"
	"github.com/kamalyes/go-rpc-gateway/internal/config"
	"go.uber.org/zap"
)

// initCore 初始化核心组件，集成go-core
func (s *Server) initCore() error {
	// 设置全局配置
	global.CONFIG = s.config.SingleConfig

	// 初始化日志
	if err := s.initLogger(); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	// 初始化数据库
	if err := s.initDatabase(); err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}

	// 初始化Redis
	if err := s.initRedis(); err != nil {
		return fmt.Errorf("failed to init redis: %w", err)
	}

	// 初始化其他go-core组件
	if err := s.initOtherComponents(); err != nil {
		return fmt.Errorf("failed to init other components: %w", err)
	}

	return nil
}

// initOtherComponents 初始化其他go-core组件
func (s *Server) initOtherComponents() error {
	// 初始化雪花ID生成器
	if err := s.initSnowflake(); err != nil {
		return fmt.Errorf("failed to init snowflake: %w", err)
	}

	// 初始化MinIO客户端
	if err := s.initMinIO(); err != nil {
		return fmt.Errorf("failed to init minio: %w", err)
	}

	// 初始化MQTT客户端（如果需要）
	if err := s.initMQTT(); err != nil {
		return fmt.Errorf("failed to init mqtt: %w", err)
	}

	// 初始化Casbin权限管理（如果需要）
	if err := s.initCasbin(); err != nil {
		return fmt.Errorf("failed to init casbin: %w", err)
	}

	return nil
}

// initLogger 初始化日志系统，使用go-core的zap模块
func (s *Server) initLogger() error {
	global.LOG = gocoreZap.Zap()
	return nil
}

// initDatabase 初始化数据库
func (s *Server) initDatabase() error {
	// 根据配置初始化数据库连接
	// 这里需要结合go-core的数据库初始化逻辑
	return nil
}

// initRedis 初始化Redis
func (s *Server) initRedis() error {
	// 根据配置初始化Redis连接
	// 这里需要结合go-core的Redis初始化逻辑
	return nil
}

// initSnowflake 初始化雪花ID生成器
func (s *Server) initSnowflake() error {
	// 这里可以根据配置初始化雪花ID
	// 暂时跳过，等待具体实现
	return nil
}

// initMinIO 初始化MinIO客户端
func (s *Server) initMinIO() error {
	// 根据配置初始化MinIO客户端
	// 这里需要结合go-core的MinIO初始化逻辑
	return nil
}

// initMQTT 初始化MQTT客户端
func (s *Server) initMQTT() error {
	// 根据配置初始化MQTT客户端
	return nil
}

// initCasbin 初始化Casbin权限管理
func (s *Server) initCasbin() error {
	// 根据配置初始化Casbin
	return nil
}

// onConfigChanged 配置变更回调
func (s *Server) onConfigChanged(newConfig *config.GatewayConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if global.LOG != nil {
		global.LOG.Info("配置发生变化，准备热重载")
	}

	// 更新配置
	oldConfig := s.config
	s.config = newConfig

	// 更新全局配置
	global.CONFIG = newConfig.SingleConfig

	// 这里可以添加其他需要热重载的组件
	// 比如重新初始化中间件、更新限流配置等

	if global.LOG != nil {
		global.LOG.Info("配置热重载完成",
			zap.String("old_version", oldConfig.Gateway.Version),
			zap.String("new_version", newConfig.Gateway.Version))
	}
}
