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
	"net/http"
	"time"

	"github.com/kamalyes/go-core/pkg/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

// initMiddleware 初始化中间件管理器
func (s *Server) initMiddleware() error {
	// 使用统一的配置系统创建中间件管理器
	manager, err := middleware.NewManager(&s.config.Middleware)
	if err != nil {
		return fmt.Errorf("failed to create middleware manager: %w", err)
	}

	s.middlewareManager = manager

	// 初始化健康检查管理器
	if err := s.initHealthManager(); err != nil {
		return fmt.Errorf("failed to create health manager: %w", err)
	}

	// 自动初始化Swagger文档服务
	// [EN] Auto initialize Swagger documentation service
	if err := s.initSwaggerService(); err != nil {
		return fmt.Errorf("failed to init swagger service: %w", err)
	}

	return nil
}

// initHealthManager 初始化健康检查管理器
func (s *Server) initHealthManager() error {
	serviceName := s.config.Gateway.Name
	if serviceName == "" {
		serviceName = "go-rpc-gateway"
	}

	serviceVersion := s.config.Gateway.Version
	if serviceVersion == "" {
		serviceVersion = "1.0.0"
	}

	// 创建健康检查管理器
	healthManager := middleware.NewHealthManager(serviceName, serviceVersion)

	// 添加Redis健康检查
	if s.config.Gateway.HealthCheck.Redis.Enabled {
		redisChecker := middleware.NewRedisChecker(
			time.Duration(s.config.Gateway.HealthCheck.Redis.Timeout) * time.Second,
		)
		healthManager.RegisterChecker(redisChecker)
	}

	// 添加MySQL健康检查
	if s.config.Gateway.HealthCheck.MySQL.Enabled {
		mysqlChecker := middleware.NewMySQLChecker(
			time.Duration(s.config.Gateway.HealthCheck.MySQL.Timeout) * time.Second,
		)
		healthManager.RegisterChecker(mysqlChecker)
	}

	s.healthManager = healthManager
	return nil
}

// initSwaggerService 自动初始化Swagger文档服务
// [EN] Auto initialize Swagger documentation service
func (s *Server) initSwaggerService() error {
	// 检查是否启用了Swagger
	// [EN] Check if Swagger is enabled
	if !s.config.Middleware.Swagger.Enabled {
		return nil
	}

	// 创建Swagger中间件
	// [EN] Create Swagger middleware
	swaggerMiddleware := middleware.NewSwaggerMiddleware(&middleware.SwaggerConfig{
		Enabled:     s.config.Middleware.Swagger.Enabled,
		JSONPath:    s.config.Middleware.Swagger.JSONPath,
		UIPath:      s.config.Middleware.Swagger.UIPath,
		Title:       s.config.Middleware.Swagger.Title,
		Description: s.config.Middleware.Swagger.Description,
	})

	// 直接创建处理函数
	// [EN] Create handler functions directly
	swaggerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 创建一个虚拟的下一个处理器，用于满足中间件接口
		// [EN] Create a dummy next handler to satisfy middleware interface
		nextHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			// 这个处理器不会被调用，因为Swagger中间件会直接处理请求
			// [EN] This handler won't be called as Swagger middleware handles requests directly
		})
		handler := swaggerMiddleware.Handler()(nextHandler)
		handler.ServeHTTP(w, r)
	})

	// 注册Swagger路由
	// [EN] Register Swagger routes
	s.RegisterHTTPRoute(s.config.Middleware.Swagger.UIPath+"/", swaggerHandler)
	s.RegisterHTTPRoute(s.config.Middleware.Swagger.UIPath+"/index.html", swaggerHandler)
	s.RegisterHTTPRoute(s.config.Middleware.Swagger.UIPath+"/swagger.json", swaggerHandler)

	global.LOGGER.InfoKV("✅ Swagger文档服务已启用",
		"ui_path", s.config.Middleware.Swagger.UIPath,
		"json_path", s.config.Middleware.Swagger.JSONPath,
		"title", s.config.Middleware.Swagger.Title)

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
