/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 01:05:02
 * @FilePath: \go-rpc-gateway\examples\02-with-config\main.go
 * @Description: 使用配置文件的Gateway示例
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gateway "github.com/kamalyes/go-rpc-gateway"
)

func main() {
	// 1. 使用配置文件创建Gateway实例
	gw, err := gateway.NewWithConfigFile("examples/02-with-config/config.yaml")
	if err != nil {
		log.Fatalf("使用配置文件创建Gateway失败: %v", err)
	}

	// 2. 注册示例API路由
	registerAPIRoutes(gw)

	log.Println("🚀 配置文件示例启动中...")
	log.Println("📋 配置文件: examples/02-with-config/config.yaml")
	log.Println("📡 API端点:")
	log.Println("   - http://localhost:8080/api/config")
	log.Println("   - http://localhost:8080/api/database/status") 
	log.Println("   - http://localhost:8080/api/redis/status")
	log.Println("   - http://localhost:8080/api/storage/status")
	log.Println("   - http://localhost:8080/health")
	log.Println("   - http://localhost:8080/metrics")

	// 3. 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 4. 启动服务器
	go func() {
		if err := gw.Start(); err != nil {
			log.Printf("服务器启动失败: %v", err)
		}
	}()

	// 5. 等待关闭信号
	<-sigChan
	log.Println("🛑 正在优雅关闭服务器...")
	
	if err := gw.Stop(); err != nil {
		log.Printf("关闭服务器时出错: %v", err)
	}
	log.Println("✅ 服务器已成功关闭")
}

// registerAPIRoutes 注册API路由
func registerAPIRoutes(gw *gateway.Gateway) {
	// 配置信息API
	gw.RegisterHTTPRoute("/api/config", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config := gw.GetConfig()
		response := map[string]interface{}{
			"gateway": map[string]interface{}{
				"name":        config.Gateway.Name,
				"version":     config.Gateway.Version,
				"environment": config.Gateway.Environment,
				"debug":       config.Gateway.Debug,
			},
			"http": map[string]interface{}{
				"host": config.Gateway.HTTP.Host,
				"port": config.Gateway.HTTP.Port,
			},
			"grpc": map[string]interface{}{
				"host": config.Gateway.GRPC.Host,
				"port": config.Gateway.GRPC.Port,
			},
			"middleware": map[string]interface{}{
				"cors_enabled":        config.SingleConfig.Cors.AllowedAllOrigins || len(config.SingleConfig.Cors.AllowedOrigins) > 0,
				"rate_limit_enabled":  config.Middleware.RateLimit.Enabled,
				"access_log_enabled":  config.Middleware.AccessLog.Enabled,
			},
			"monitoring": map[string]interface{}{
				"metrics_enabled": config.Monitoring.Metrics.Enabled,
				"tracing_enabled": config.Monitoring.Tracing.Enabled,
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	// 数据库状态API
	gw.RegisterHTTPRoute("/api/database/status", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config := gw.GetConfig()
		response := map[string]interface{}{
			"type": "mysql",
			"host": config.MySQL.Host,
			"port": config.MySQL.Port,
			"database": config.MySQL.Dbname,
			"max_idle_conns": config.MySQL.MaxIdleConns,
			"max_open_conns": config.MySQL.MaxOpenConns,
			"status": "configured", // 实际项目中可以检查连接状态
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	// Redis状态API  
	gw.RegisterHTTPRoute("/api/redis/status", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config := gw.GetConfig()
		response := map[string]interface{}{
			"addr": config.Redis.Addr,
			"db":   config.Redis.DB,
			"status": "configured", // 实际项目中可以检查连接状态
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	// 对象存储状态API
	gw.RegisterHTTPRoute("/api/storage/status", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config := gw.GetConfig()
		response := map[string]interface{}{
			"type": "minio",
			"endpoint": config.Minio.Endpoint,
			"access_key": config.Minio.AccessKey,
			"status": "configured", // 实际项目中可以检查连接状态
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	// 系统信息API
	gw.RegisterHTTPRoute("/api/system/info", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"uptime": "running",
			"go_version": "1.21+",
			"framework": "go-rpc-gateway",
			"features": []string{
				"gRPC-Gateway",
				"Middleware支持",
				"配置热重载", 
				"企业级监控",
				"数据库集成",
				"缓存支持",
				"对象存储",
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}