/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-26 12:11:05
 * @FilePath: \go-rpc-gateway\server\swagger.go
 * @Description: Swagger 文档服务管理
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"net/http"

	goswagger "github.com/kamalyes/go-config/pkg/swagger"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/kamalyes/go-toolbox/pkg/safe"
)

// EnableSwagger 启用 Swagger 文档服务
func (s *Server) EnableSwagger() error {
	// 配置已通过 safe.MergeWithDefaults 合并,直接使用
	if !s.config.Swagger.Enabled {
		return nil
	}

	// 处理 Aggregate 配置
	if s.config.Swagger.Aggregate.Enabled {
		global.LOGGER.InfoContext(s.ctx, "🔧 解析聚合配置: enabled=%v, mode=%s, services_count=%d",
			s.config.Swagger.Aggregate.Enabled,
			s.config.Swagger.Aggregate.Mode,
			len(s.config.Swagger.Aggregate.Services))
	}

	return s.EnableSwaggerWithConfig(s.config.Swagger)
}

// EnableSwaggerWithConfig 使用 go-config 的 Swagger 配置启用服务
func (s *Server) EnableSwaggerWithConfig(config *goswagger.Swagger) error {
	if !config.Enabled {
		return nil
	}

	// 验证并修正 UIPath 以避免路由冲突
	if config.UIPath == "" || config.UIPath == "/" {
		originalPath := config.UIPath
		config.UIPath = "/swagger"
		global.LOGGER.WarnContext(s.ctx, "⚠️  Swagger UIPath为空或根路径，已重置为默认值: %s -> %s",
			originalPath, "/swagger")
	}

	global.LOGGER.InfoContext(s.ctx, "🔧 启用Swagger配置: ui_path=%s, json_path=%s, enabled=%v",
		config.UIPath, config.JSONPath, config.Enabled) // 直接使用 go-config 的配置创建中间件
	swaggerMiddleware := middleware.NewSwaggerMiddleware(config)

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

	global.LOGGER.InfoContext(s.ctx, "✅ Swagger 文档服务已启用: ui_path=%s, json_path=%s, title=%s",
		config.UIPath, config.JSONPath, config.Title)

	return nil
}

// parseAggregateServices 解析聚合服务配置
func (s *Server) parseAggregateServices(servicesSafe interface{ Value() interface{} }) []*goswagger.ServiceSpec {
	var services []*goswagger.ServiceSpec

	servicesValue := servicesSafe.Value()
	if servicesValue == nil {
		return services
	}

	servicesSlice, ok := servicesValue.([]interface{})
	if !ok {
		global.LOGGER.WarnContext(s.ctx, "services 配置不是数组类型")
		return services
	}

	for i, serviceInterface := range servicesSlice {
		serviceMap, ok := serviceInterface.(map[string]interface{})
		if !ok {
			global.LOGGER.WarnContext(s.ctx, "跳过无效的服务配置: index=%d, type=not_map", i)
			continue
		}

		service := &goswagger.ServiceSpec{
			Name:        safe.SafeGetString(serviceMap, "name"),
			Description: safe.SafeGetString(serviceMap, "description"),
			SpecPath:    safe.SafeGetString(serviceMap, "spec-path"),
			URL:         safe.SafeGetString(serviceMap, "url"),
			Version:     safe.SafeGetString(serviceMap, "version"),
			Enabled:     safe.SafeGetBool(serviceMap, "enabled"),
			BasePath:    safe.SafeGetString(serviceMap, "base-path"),
			Tags:        safe.SafeGetStringSlice(serviceMap, "tags"),
		}

		// 验证必要字段
		if service.Name == "" {
			global.LOGGER.WarnContext(s.ctx, "跳过缺少名称的服务配置: index=%d", i)
			continue
		}

		services = append(services, service)
		global.LOGGER.DebugContext(s.ctx, "解析服务配置: name=%s, enabled=%v, spec_path=%s",
			service.Name, service.Enabled, service.SpecPath)
	}

	return services
}
