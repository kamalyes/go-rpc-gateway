package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/kamalyes/go-core/pkg/global"
	gateway "github.com/kamalyes/go-rpc-gateway"
)

// 演示四核心库集成的简化示例
func main() {
	// 🎯 Step 1: 创建网关 (集成四大核心库)
	gw, err := gateway.New()
	if err != nil {
		panic("创建网关失败: " + err.Error())
	}

	// 🔧 Step 2: 演示 go-core 企业级组件使用
	demonstrateGoCoreComponents()

	// 🛡️ Step 3: 添加健康检查路由
	setupHealthCheckRoute()

	// � 启动网关
	if err := gw.Start(); err != nil {
		panic("网关启动失败: " + err.Error())
	}

	// 优雅关闭
	gw.Shutdown()
}

// demonstrateGoCoreComponents 演示 go-core 企业级组件的使用
func demonstrateGoCoreComponents() {
	println("🔧 演示 go-core 企业级组件")

	// 1. 检查数据库连接
	if db := global.DB; db != nil {
		println("✅ 数据库连接已建立")
	} else {
		println("⚠️  数据库未配置")
	}

	// 2. 检查Redis连接
	if redis := global.REDIS; redis != nil {
		println("✅ Redis连接已建立")

		// 测试Redis操作
		ctx := context.Background()
		err := redis.Set(ctx, "gateway:test", "ok", time.Minute).Err()
		if err == nil {
			println("✅ Redis写入成功")
		}
	} else {
		println("⚠️  Redis未配置")
	}

	// 3. 检查MinIO连接
	if minio := global.MinIO; minio != nil {
		println("✅ MinIO客户端已初始化")
	} else {
		println("⚠️  MinIO未配置")
	}

	println("🔧 go-core组件检查完成")
}

// setupHealthCheckRoute 设置健康检查路由
func setupHealthCheckRoute() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		health := map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "v1.0.0",
			"components": map[string]bool{
				"database": global.DB != nil,
				"redis":    global.REDIS != nil,
				"storage":  global.MinIO != nil,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	})

	println("✅ 健康检查路由设置完成: /health")
}
