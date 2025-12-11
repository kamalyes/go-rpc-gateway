/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-07 21:50:00
 * @FilePath: \go-rpc-gateway\server\swagger.go
 * @Description: Swagger 文档服务管理 - 通过 middleware manager 统一管理
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"github.com/kamalyes/go-rpc-gateway/global"
)

// EnableSwagger 启用 Swagger 文档服务（通过 middleware manager）
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

	// 验证并修正 UIPath 以避免路由冲突
	if s.config.Swagger.UIPath == "" || s.config.Swagger.UIPath == "/" {
		originalPath := s.config.Swagger.UIPath
		s.config.Swagger.UIPath = "/swagger"
		global.LOGGER.WarnContext(s.ctx, "⚠️  Swagger UIPath为空或根路径，已重置为默认值: %s -> %s",
			originalPath, "/swagger")
	}

	global.LOGGER.InfoContext(s.ctx, "🔧 启用Swagger配置: ui_path=%s, json_path=%s, enabled=%v",
		s.config.Swagger.UIPath, s.config.Swagger.JSONPath, s.config.Swagger.Enabled)

	// 从 middleware manager 获取 Swagger 处理器
	swaggerHandler := s.middlewareManager.SwaggerHandler()
	
	// 注册 Swagger 路由
	for _, path := range s.middlewareManager.GetSwaggerPaths() {
		s.RegisterHTTPRoute(path, swaggerHandler)
	}

	global.LOGGER.InfoContext(s.ctx, "✅ Swagger 文档服务已启用: ui_path=%s, json_path=%s, title=%s",
		s.config.Swagger.UIPath, s.config.Swagger.JSONPath, s.config.Swagger.Title)

	return nil
}
