/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 02:25:08
 * @FilePath: \go-rpc-gateway\server\swagger.go
 * @Description: Swagger 文档服务管理
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"net/http"

	goswagger "github.com/kamalyes/go-config/pkg/swagger"
	"github.com/kamalyes/go-core/pkg/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

// EnableSwagger 启用 Swagger 文档服务
func (s *Server) EnableSwagger() error {
	// 使用 go-config 的 Swagger 配置
	swaggerConfig := goswagger.Default().
		WithEnabled(s.config.Swagger.Enabled).
		WithJSONPath(s.config.Swagger.JSONPath).
		WithUIPath(s.config.Swagger.UIPath).
		WithTitle(s.config.Swagger.Title).
		WithDescription(s.config.Swagger.Description)
	return s.EnableSwaggerWithConfig(swaggerConfig)
}

// EnableSwaggerWithConfig 使用 go-config 的 Swagger 配置启用服务
func (s *Server) EnableSwaggerWithConfig(config *goswagger.Swagger) error {
	if !config.Enabled {
		return nil
	}

	// 转换为中间件配置
	middlewareConfig := &middleware.SwaggerConfig{
		Enabled:     config.Enabled,
		JSONPath:    config.JSONPath,
		UIPath:      config.UIPath,
		Title:       config.Title,
		Description: config.Description,
	}

	// 创建 Swagger 中间件
	swaggerMiddleware := middleware.NewSwaggerMiddleware(middlewareConfig)

	// 创建处理函数
	swaggerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Swagger 中间件会直接处理请求，不需要传递给下一个处理器
		nextHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			// Empty handler - Swagger middleware handles the request directly
		})
		handler := swaggerMiddleware.Handler()(nextHandler)
		handler.ServeHTTP(w, r)
	})

	// 注册 Swagger 路由
	s.RegisterHTTPRoute(config.UIPath+"/", swaggerHandler)
	s.RegisterHTTPRoute(config.UIPath+"/index.html", swaggerHandler)
	s.RegisterHTTPRoute(config.UIPath+"/swagger.json", swaggerHandler)

	global.LOGGER.InfoKV("✅ Swagger 文档服务已启用",
		"ui_path", config.UIPath,
		"json_path", config.JSONPath,
		"title", config.Title)

	return nil
}