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

	goconfig "github.com/kamalyes/go-config"
	goswagger "github.com/kamalyes/go-config/pkg/swagger"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

// getString 安全获取map中的字符串值
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return ""
}

// getBool 安全获取map中的布尔值
func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// getStringSlice 安全获取map中的字符串切片
func getStringSlice(m map[string]interface{}, key string) []string {
	if v, ok := m[key]; ok {
		if slice, ok := v.([]interface{}); ok {
			result := make([]string, 0, len(slice))
			for _, item := range slice {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return nil
}

// EnableSwagger 启用 Swagger 文档服务
func (s *Server) EnableSwagger() error {
	// 使用安全访问模式获取 Swagger 配置
	configSafe := goconfig.SafeConfig(s.config)
	swaggerSafe := configSafe.Field("Swagger")

	swaggerConfig := goswagger.Default().
		WithEnabled(swaggerSafe.Field("Enabled").Bool(false)).
		WithJSONPath(swaggerSafe.Field("JSONPath").String("")).
		WithUIPath(swaggerSafe.Field("UIPath").String("/swagger")).
		WithTitle(swaggerSafe.Field("Title").String("API Documentation")).
		WithDescription(swaggerSafe.Field("Description").String("")).
		WithVersion(swaggerSafe.Field("Version").String("1.0.0"))

	// 处理 Aggregate 配置
	if aggregateSafe := swaggerSafe.Field("Aggregate"); aggregateSafe.IsValid() {
		aggregate := &goswagger.AggregateConfig{
			Enabled: aggregateSafe.Field("Enabled").Bool(false),
			Mode:    aggregateSafe.Field("Mode").String("merge"),
		}

		// 处理服务列表
		if servicesSafe := aggregateSafe.Field("Services"); servicesSafe.IsValid() {
			if servicesValue := servicesSafe.Value(); servicesValue != nil {
				if servicesSlice, ok := servicesValue.([]interface{}); ok {
					for _, serviceInterface := range servicesSlice {
						if serviceMap, ok := serviceInterface.(map[string]interface{}); ok {
							service := &goswagger.ServiceSpec{
								Name:        getString(serviceMap, "name"),
								Description: getString(serviceMap, "description"),
								SpecPath:    getString(serviceMap, "spec_path"),
								URL:         getString(serviceMap, "url"),
								Version:     getString(serviceMap, "version"),
								Enabled:     getBool(serviceMap, "enabled"),
								BasePath:    getString(serviceMap, "base_path"),
								Tags:        getStringSlice(serviceMap, "tags"),
							}
							aggregate.Services = append(aggregate.Services, service)
						}
					}
				}
			}
		}

		swaggerConfig = swaggerConfig.WithAggregate(aggregate)

		global.LOGGER.InfoKV("🔧 解析聚合配置",
			"enabled", aggregate.Enabled,
			"mode", aggregate.Mode,
			"services_count", len(aggregate.Services))
	}

	// Contact 和 License 如果不为空则设置
	if contact := swaggerSafe.Field("Contact").Value(); contact != nil {
		if contactPtr, ok := contact.(*goswagger.Contact); ok {
			swaggerConfig = swaggerConfig.WithContact(contactPtr)
		}
	}
	if license := swaggerSafe.Field("License").Value(); license != nil {
		if licensePtr, ok := license.(*goswagger.License); ok {
			swaggerConfig = swaggerConfig.WithLicense(licensePtr)
		}
	}

	return s.EnableSwaggerWithConfig(swaggerConfig)
} // EnableSwaggerWithConfig 使用 go-config 的 Swagger 配置启用服务
func (s *Server) EnableSwaggerWithConfig(config *goswagger.Swagger) error {
	if !config.Enabled {
		return nil
	}

	// 直接使用 go-config 的配置创建中间件
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

	global.LOGGER.InfoKV("✅ Swagger 文档服务已启用",
		"ui_path", config.UIPath,
		"json_path", config.JSONPath,
		"title", config.Title)

	return nil
}
