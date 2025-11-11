/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 02:50:00
 * @FilePath: \go-rpc-gateway\examples\features\main.go
 * @Description: 功能管理机制使用示例
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package main

import (
	"log"

	"github.com/kamalyes/go-config/pkg/health"
	"github.com/kamalyes/go-config/pkg/monitoring"
	"github.com/kamalyes/go-config/pkg/swagger"
	gateway "github.com/kamalyes/go-rpc-gateway"
	"github.com/kamalyes/go-rpc-gateway/server"
)

func main() {
	// 创建 Gateway 实例
	gw, err := gateway.New()
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	// ========================================
	// 方式一：简单启用（使用配置文件设置）
	// ========================================
	
	// 启用 Swagger（自动从 config.Swagger 读取）
	if err := gw.EnableSwagger(); err != nil {
		log.Printf("Failed to enable swagger: %v", err)
	}

	// 启用监控（自动从 config.Monitoring 读取）
	if err := gw.EnableMonitoring(); err != nil {
		log.Printf("Failed to enable monitoring: %v", err)
	}

	// 启用健康检查（自动从 config.Health 读取）
	if err := gw.EnableHealth(); err != nil {
		log.Printf("Failed to enable health: %v", err)
	}

	// ========================================
	// 方式二：自定义配置启用
	// ========================================
	
	// 自定义 Swagger 配置
	customSwagger := &swagger.Swagger{
		Enabled:     true,
		JSONPath:    "/api/swagger.json",
		UIPath:      "/docs",
		Title:       "Custom API Documentation",
		Description: "自定义 API 文档",
		Version:     "2.0.0",
	}
	if err := gw.EnableSwaggerWithConfig(customSwagger); err != nil {
		log.Printf("Failed to enable custom swagger: %v", err)
	}

	// 自定义监控配置
	customMonitoring := &monitoring.Monitoring{
		Enabled: true,
		Metrics: &monitoring.Metrics{
			Enabled:  true,
			Endpoint: "/custom/metrics",
		},
	}
	if err := gw.EnableMonitoringWithConfig(customMonitoring); err != nil {
		log.Printf("Failed to enable custom monitoring: %v", err)
	}

	// 自定义健康检查配置
	customHealth := &health.Health{
		Enabled: true,
		Path:    "/custom/health",
		Redis: &health.RedisConfig{
			Enabled: true,
			Path:    "/custom/health/redis",
		},
	}
	if err := gw.EnableHealthWithConfig(customHealth); err != nil {
		log.Printf("Failed to enable custom health: %v", err)
	}

	// ========================================
	// 方式三：使用通用接口
	// ========================================
	
	// 通过 FeatureType 启用功能
	if err := gw.EnableFeature(server.FeatureSwagger); err != nil {
		log.Printf("Failed to enable feature: %v", err)
	}

	if err := gw.EnableFeature(server.FeatureMonitoring); err != nil {
		log.Printf("Failed to enable feature: %v", err)
	}

	if err := gw.EnableFeature(server.FeatureHealth); err != nil {
		log.Printf("Failed to enable feature: %v", err)
	}

	// ========================================
	// 方式四：检查功能状态
	// ========================================
	
	// 检查特定功能是否启用
	if gw.IsSwaggerEnabled() {
		log.Println("✓ Swagger is enabled")
	}

	if gw.IsMonitoringEnabled() {
		log.Println("✓ Monitoring is enabled")
	}

	if gw.IsHealthEnabled() {
		log.Println("✓ Health check is enabled")
	}

	// 使用通用接口检查
	if gw.IsFeatureEnabled(server.FeatureSwagger) {
		log.Println("✓ Swagger feature is enabled")
	}

	// ========================================
	// 方式五：批量启用所有配置中的功能
	// ========================================
	
	// 通过 Server 的 FeatureManager 批量启用
	// if err := gw.Server.GetFeatureManager().EnableAll(); err != nil {
	// 	log.Printf("Failed to enable all features: %v", err)
	// }

	// ========================================
	// 启动服务
	// ========================================
	
	log.Println("Starting gateway server...")
	if err := gw.Start(); err != nil {
		log.Fatalf("Failed to start gateway: %v", err)
	}
}
