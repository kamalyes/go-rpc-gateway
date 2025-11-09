/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-08 00:30:00
 * @FilePath: \go-rpc-gateway\examples\pprof_example.go
 * @Description: pprof功能使用示例
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package main

import (
	"context"
	"log"
	"time"

	gateway "github.com/kamalyes/go-rpc-gateway"
)

func main() {
	// 创建Gateway实例
	gw, err := gateway.New()
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	// 示例1: 使用默认配置启用pprof
	gw.EnablePProf()

	// 示例2: 使用自定义token启用pprof
	// gw.EnablePProfWithToken("my-secret-token")

	// 示例3: 启用开发环境pprof配置
	// gw.EnablePProfForDevelopment()

	// 示例4: 使用完整配置启用pprof
	/*
	gw.EnablePProfWithOptions(middleware.PProfOptions{
		Enabled:     true,
		AuthToken:   "custom-token-2024",
		PathPrefix:  "/debug/pprof",
		DevModeOnly: false,
		AllowedIPs:  []string{"127.0.0.1", "::1", "192.168.1.0/24"},
		EnableLogging: true,
		Timeout:     60, // 60秒超时
	})
	*/

	// 检查pprof状态
	log.Printf("PProf enabled: %t", gw.IsPProfEnabled())
	
	if gw.IsPProfEnabled() {
		config := gw.GetPProfConfig()
		log.Printf("PProf path prefix: %s", config.PathPrefix)
		log.Printf("PProf auth required: %t", config.RequireAuth)
		log.Printf("PProf auth token: %s", config.AuthToken)
		
		// 获取所有可用端点
		endpoints := gw.GetPProfEndpoints()
		log.Printf("Available pprof endpoints: %d", len(endpoints))
		for _, endpoint := range endpoints {
			log.Printf("  %s %s - %s", endpoint.Method, endpoint.Path, endpoint.Description)
		}
	}

	// 启动服务器
	log.Println("Starting gateway server...")
	log.Println("PProf dashboard available at: http://localhost:8080/")
	log.Println("PProf status API available at: http://localhost:8080/api/pprof/status")
	log.Println("Standard pprof endpoints available at: http://localhost:8080/debug/pprof/")
	
	// 创建一个上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动一个goroutine来运行服务器
	go func() {
		if err := gw.Start(); err != nil {
			log.Printf("Server error: %v", err)
			cancel()
		}
	}()

	// 等待5分钟后自动关闭（用于演示）
	select {
	case <-time.After(5 * time.Minute):
		log.Println("Demo timeout, shutting down...")
	case <-ctx.Done():
		log.Println("Context cancelled, shutting down...")
	}

	// 优雅关闭
	if err := gw.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}
	log.Println("Server stopped")
}