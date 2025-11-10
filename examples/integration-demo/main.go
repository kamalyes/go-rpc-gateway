package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kamalyes/go-core/pkg/global"
	gateway "github.com/kamalyes/go-rpc-gateway"
)

// 演示四大核心库集成示例
// go-config: 统一配置管理
// go-core: 企业级组件 (DB/Redis/MinIO/RabbitMQ/Consul)
// go-logger: 结构化日志系统
// go-toolbox: 常用工具函数集
func main() {
	fmt.Println("🚀 Go RPC Gateway - 四大核心库集成演示")

	// 🎯 创建网关实例 (自动集成四大核心库)
	gw, err := gateway.New()
	if err != nil {
		panic("创建网关失败: " + err.Error())
	}

	// 🔧 检查 go-core 企业级组件
	checkGoCoreComponents()

	// 🛡️ 设置健康检查API
	setupHealthAPI()

	// 📊 设置组件状态API
	setupComponentsAPI()

	// 🚀 启动网关
	if err := gw.Start(); err != nil {
		panic("网关启动失败: " + err.Error())
	}
	gw.Shutdown()
}

// checkGoCoreComponents 检查 go-core 企业级组件状态
func checkGoCoreComponents() {
	fmt.Println("\n🔧 检查 go-core 企业级组件:")

	// 检查数据库连接
	if global.DB != nil {
		fmt.Println("  ✅ 数据库: 已连接")
		// 可以执行数据库操作
		// var count int64
		// global.DB.Raw("SELECT 1").Scan(&count)
	} else {
		fmt.Println("  ⚠️  数据库: 未配置 (可在配置文件中启用)")
	}

	// 检查Redis连接
	if global.REDIS != nil {
		fmt.Println("  ✅ Redis: 已连接")
		testRedisConnection()
	} else {
		fmt.Println("  ⚠️  Redis: 未配置 (可在配置文件中启用)")
	}

	// 检查MinIO存储
	if global.MinIO != nil {
		fmt.Println("  ✅ MinIO: 已初始化")
		testMinIOConnection()
	} else {
		fmt.Println("  ⚠️  MinIO: 未配置 (可在配置文件中启用)")
	}

	// 检查其他组件
	fmt.Println("  ℹ️  RabbitMQ: 可通过配置文件启用")
	fmt.Println("  ℹ️  Consul: 可通过配置文件启用")
}

// testRedisConnection 测试Redis连接
func testRedisConnection() {
	ctx := context.Background()
	testKey := "gateway:health:test"
	testValue := fmt.Sprintf("ok-%d", time.Now().Unix())

	// 写入测试
	err := global.REDIS.Set(ctx, testKey, testValue, time.Minute).Err()
	if err != nil {
		fmt.Printf("    ❌ Redis写入测试失败: %v\n", err)
		return
	}

	// 读取测试
	val, err := global.REDIS.Get(ctx, testKey).Result()
	if err != nil {
		fmt.Printf("    ❌ Redis读取测试失败: %v\n", err)
		return
	}

	if val == testValue {
		fmt.Println("    ✅ Redis读写测试成功")
	} else {
		fmt.Println("    ❌ Redis数据不匹配")
	}

	// 清理测试数据
	global.REDIS.Del(ctx, testKey)
}

// testMinIOConnection 测试MinIO连接
func testMinIOConnection() {
	ctx := context.Background()

	// 检查MinIO连接
	if global.MinIO == nil {
		fmt.Println("    ❌ MinIO客户端未初始化")
		return
	}

	// 检查默认存储桶
	buckets, err := global.MinIO.ListBuckets(ctx)
	if err != nil {
		fmt.Printf("    ❌ MinIO列举存储桶失败: %v\n", err)
		return
	}

	fmt.Printf("    ✅ MinIO连接正常，发现 %d 个存储桶\n", len(buckets))
}

// setupHealthAPI 设置健康检查API
func setupHealthAPI() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		health := map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "v1.0.0",
			"service":   "go-rpc-gateway",
			"message":   "四大核心库集成正常",
			"uptime":    time.Since(startTime).String(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Powered-By", "go-rpc-gateway")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(health)
	})
}

// setupComponentsAPI 设置组件状态API
func setupComponentsAPI() {
	http.HandleFunc("/components", func(w http.ResponseWriter, r *http.Request) {
		components := map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"libraries": map[string]interface{}{
				"go-config": map[string]interface{}{
					"description": "统一配置管理",
					"features":    []string{"多格式支持", "热重载", "环境变量", "配置验证"},
					"status":      "active",
				},
				"go-core": map[string]interface{}{
					"description": "企业级组件",
					"features":    []string{"数据库", "缓存", "存储", "消息队列", "服务发现"},
					"status":      "active",
				},
				"go-logger": map[string]interface{}{
					"description": "结构化日志",
					"features":    []string{"高性能", "多输出", "日志轮转", "上下文"},
					"status":      "active",
				},
				"go-toolbox": map[string]interface{}{
					"description": "工具函数集",
					"features":    []string{"加密", "ID生成", "字符串", "时间", "网络"},
					"status":      "active",
				},
			},
			"components": map[string]interface{}{
				"database": map[string]interface{}{
					"available":   global.DB != nil,
					"type":        "GORM (MySQL/PostgreSQL/SQLite)",
					"description": "关系型数据库ORM",
				},
				"redis": map[string]interface{}{
					"available":   global.REDIS != nil,
					"type":        "go-redis (单机/集群/哨兵)",
					"description": "内存数据库和缓存",
				},
				"storage": map[string]interface{}{
					"available":   global.MinIO != nil,
					"type":        "MinIO (S3兼容)",
					"description": "对象存储服务",
				},
				"message_queue": map[string]interface{}{
					"available":   false, // RabbitMQ需要额外配置
					"type":        "RabbitMQ (可选)",
					"description": "消息队列服务",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Powered-By", "go-rpc-gateway")
		json.NewEncoder(w).Encode(components)
	})
}

// 启动时间 (用于计算运行时长)
var startTime = time.Now()
