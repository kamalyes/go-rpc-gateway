/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-08 00:30:00
 * @FilePath: \go-rpc-gateway\examples\06-enterprise\main.go
 * @Description: 企业级完整示例 - 展示Gateway在生产环境中的完整使用
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gateway "github.com/kamalyes/go-rpc-gateway"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

func main() {
	// 1. 使用配置文件创建Gateway实例
	gw, err := createGatewayWithConfig()
	if err != nil {
		log.Fatalf("创建Gateway失败: %v", err)
	}

	// 2. 配置企业级功能
	setupEnterpriseFeatures(gw)

	// 3. 注册业务API
	registerBusinessAPI(gw)

	// 4. 注册监控和管理API
	registerManagementAPI(gw)

	// 5. 打印企业级功能信息
	printEnterpriseInfo(gw)

	// 6. 设置优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 7. 启动服务器
	go func() {
		if err := gw.Start(); err != nil {
			log.Printf("服务器启动失败: %v", err)
			cancel()
		}
	}()

	// 8. 启动后台任务
	go startBackgroundTasks(ctx)

	// 9. 等待关闭信号
	select {
	case sig := <-sigChan:
		log.Printf("收到关闭信号: %v", sig)
	case <-ctx.Done():
		log.Println("上下文已取消")
	}

	// 10. 优雅关闭
	log.Println("🛑 开始优雅关闭...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := gracefulShutdown(shutdownCtx, gw); err != nil {
		log.Printf("优雅关闭失败: %v", err)
	} else {
		log.Println("✅ 优雅关闭完成")
	}
}

// createGatewayWithConfig 创建带配置的Gateway
func createGatewayWithConfig() (*gateway.Gateway, error) {
	// 尝试使用配置文件
	configFile := "examples/06-enterprise/config.yaml"
	if _, err := os.Stat(configFile); err == nil {
		return gateway.NewWithConfigFile(configFile)
	}

	// 配置文件不存在，使用代码配置
	gw, err := gateway.New()
	if err != nil {
		return nil, err
	}

	log.Println("⚠️ 配置文件不存在，使用默认配置")
	return gw, nil
}

// setupEnterpriseFeatures 设置企业级功能
func setupEnterpriseFeatures(gw *gateway.Gateway) {
	// 1. 启用PProf性能分析
	gw.EnablePProfWithOptions(middleware.PProfOptions{
		Enabled:       true,
		AuthToken:     getEnvOrDefault("PPROF_TOKEN", "enterprise-pprof-2024"),
		PathPrefix:    "/debug/pprof",
		DevModeOnly:   false,
		AllowedIPs:    []string{"127.0.0.1", "::1"}, // 限制访问IP
		EnableLogging: true,
		Timeout:       60,
	})

	log.Println("🔧 企业级功能配置完成:")
	log.Println("   ✅ 性能分析 (PProf)")
	log.Println("   ✅ 中间件链")
	log.Println("   ✅ 安全控制") 
	log.Println("   ✅ 监控指标")
	log.Println("   ✅ 链路追踪")
}

// registerBusinessAPI 注册业务API
func registerBusinessAPI(gw *gateway.Gateway) {
	// API版本 v1
	registerV1API(gw)

	// API版本 v2  
	registerV2API(gw)

	log.Println("📡 业务API注册完成")
}

// registerV1API 注册v1版本API
func registerV1API(gw *gateway.Gateway) {
	// 用户管理
	gw.RegisterHTTPRoute("/api/v1/users", http.HandlerFunc(userHandler))
	gw.RegisterHTTPRoute("/api/v1/users/", http.HandlerFunc(userDetailHandler))

	// 订单管理
	gw.RegisterHTTPRoute("/api/v1/orders", http.HandlerFunc(orderHandler))
	gw.RegisterHTTPRoute("/api/v1/orders/", http.HandlerFunc(orderDetailHandler))

	// 产品管理
	gw.RegisterHTTPRoute("/api/v1/products", http.HandlerFunc(productHandler))
}

// registerV2API 注册v2版本API
func registerV2API(gw *gateway.Gateway) {
	// v2版本的增强API
	gw.RegisterHTTPRoute("/api/v2/users", http.HandlerFunc(userV2Handler))
	gw.RegisterHTTPRoute("/api/v2/analytics", http.HandlerFunc(analyticsHandler))
}

// registerManagementAPI 注册管理API
func registerManagementAPI(gw *gateway.Gateway) {
	// 系统健康检查
	gw.RegisterHTTPRoute("/admin/health/detailed", http.HandlerFunc(detailedHealthHandler))

	// 配置管理
	gw.RegisterHTTPRoute("/admin/config", http.HandlerFunc(configHandler))

	// 指标查看
	gw.RegisterHTTPRoute("/admin/metrics/summary", http.HandlerFunc(metricsSummaryHandler))

	// 服务信息
	gw.RegisterHTTPRoute("/admin/info", http.HandlerFunc(serviceInfoHandler))

	// 性能报告
	gw.RegisterHTTPRoute("/admin/performance", http.HandlerFunc(performanceHandler))

	log.Println("🛠️ 管理API注册完成")
}

// 业务处理器实现
func userHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		users := []map[string]interface{}{
			{"id": 1, "name": "Alice", "email": "alice@corp.com", "role": "admin"},
			{"id": 2, "name": "Bob", "email": "bob@corp.com", "role": "user"},
			{"id": 3, "name": "Charlie", "email": "charlie@corp.com", "role": "manager"},
		}
		
		response := map[string]interface{}{
			"success":    true,
			"data":       users,
			"total":      len(users),
			"api_version": "v1",
			"timestamp":  time.Now().Format(time.RFC3339),
		}
		
		writeJSONResponse(w, http.StatusOK, response)

	case "POST":
		var user map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			writeJSONResponse(w, http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"error":   "Invalid JSON payload",
			})
			return
		}
		
		user["id"] = time.Now().Unix()
		user["created_at"] = time.Now().Format(time.RFC3339)
		
		response := map[string]interface{}{
			"success": true,
			"data":    user,
			"message": "User created successfully",
		}
		
		writeJSONResponse(w, http.StatusCreated, response)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func userDetailHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Path[len("/api/v1/users/"):]
	
	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"id":         userID,
			"name":       "User " + userID,
			"email":      fmt.Sprintf("user%s@corp.com", userID),
			"created_at": time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			"last_login": time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
		},
		"api_version": "v1",
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func orderHandler(w http.ResponseWriter, r *http.Request) {
	orders := []map[string]interface{}{
		{"id": 1001, "user_id": 1, "amount": 99.99, "status": "completed"},
		{"id": 1002, "user_id": 2, "amount": 149.99, "status": "pending"},
		{"id": 1003, "user_id": 1, "amount": 79.99, "status": "shipped"},
	}
	
	response := map[string]interface{}{
		"success": true,
		"data":    orders,
		"total":   len(orders),
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func orderDetailHandler(w http.ResponseWriter, r *http.Request) {
	orderID := r.URL.Path[len("/api/v1/orders/"):]
	
	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"id":         orderID,
			"user_id":    1,
			"amount":     99.99,
			"status":     "completed",
			"items":      []string{"Product A", "Product B"},
			"created_at": time.Now().Add(-48 * time.Hour).Format(time.RFC3339),
		},
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func productHandler(w http.ResponseWriter, r *http.Request) {
	products := []map[string]interface{}{
		{"id": 1, "name": "Enterprise License", "price": 999.99, "category": "software"},
		{"id": 2, "name": "Premium Support", "price": 299.99, "category": "service"},
		{"id": 3, "name": "Cloud Storage", "price": 49.99, "category": "storage"},
	}
	
	response := map[string]interface{}{
		"success": true,
		"data":    products,
		"total":   len(products),
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func userV2Handler(w http.ResponseWriter, r *http.Request) {
	// v2版本的增强用户API
	response := map[string]interface{}{
		"success":     true,
		"api_version": "v2",
		"data": map[string]interface{}{
			"users": []map[string]interface{}{
				{
					"id":       1,
					"name":     "Alice",
					"email":    "alice@corp.com",
					"profile":  map[string]interface{}{"department": "Engineering", "level": "Senior"},
					"activity": map[string]interface{}{"last_active": time.Now().Format(time.RFC3339)},
				},
			},
		},
		"metadata": map[string]interface{}{
			"pagination": map[string]interface{}{"page": 1, "size": 10, "total": 1},
			"filters":    []string{},
		},
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func analyticsHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"daily_stats": map[string]interface{}{
				"requests":      12450,
				"errors":        23,
				"response_time": "45ms",
			},
			"user_analytics": map[string]interface{}{
				"total_users":   1250,
				"active_users":  890,
				"new_users":     15,
			},
			"system_metrics": map[string]interface{}{
				"cpu_usage":     "12%",
				"memory_usage":  "68%",
				"disk_usage":    "45%",
			},
		},
		"generated_at": time.Now().Format(time.RFC3339),
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

// 管理处理器实现
func detailedHealthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status": "healthy",
		"checks": map[string]interface{}{
			"database":    map[string]interface{}{"status": "up", "response_time": "5ms"},
			"redis":       map[string]interface{}{"status": "up", "response_time": "2ms"},
			"storage":     map[string]interface{}{"status": "up", "response_time": "10ms"},
			"external_api": map[string]interface{}{"status": "up", "response_time": "150ms"},
		},
		"system": map[string]interface{}{
			"uptime":     "72h 15m 30s",
			"version":    "1.0.0",
			"go_version": "1.21",
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"gateway": map[string]interface{}{
			"name":        "Enterprise Gateway",
			"version":     "1.0.0",
			"environment": "production",
			"debug":       false,
		},
		"features": map[string]interface{}{
			"pprof_enabled":   true,
			"metrics_enabled": true,
			"tracing_enabled": true,
			"cors_enabled":    true,
			"rate_limit":      true,
		},
		"endpoints": map[string]interface{}{
			"total_registered": 15,
			"health_checks":    3,
			"business_apis":    8,
			"admin_apis":       4,
		},
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func metricsSummaryHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"requests": map[string]interface{}{
			"total":    125430,
			"success":  124890,
			"errors":   540,
			"success_rate": "99.57%",
		},
		"performance": map[string]interface{}{
			"avg_response_time": "45ms",
			"p95_response_time": "120ms",
			"p99_response_time": "250ms",
		},
		"traffic": map[string]interface{}{
			"rps_current":     142,
			"rps_peak_today":  890,
			"bandwidth_in":    "2.1 MB/s",
			"bandwidth_out":   "5.8 MB/s",
		},
		"collected_at": time.Now().Format(time.RFC3339),
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func serviceInfoHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"service": map[string]interface{}{
			"name":         "Go RPC Gateway",
			"version":      "1.0.0",
			"build_time":   "2024-11-08T00:00:00Z",
			"commit_hash":  "abc123def456",
		},
		"runtime": map[string]interface{}{
			"go_version":   "1.21",
			"goroutines":   45,
			"memory_mb":    128.5,
			"gc_runs":      234,
		},
		"configuration": map[string]interface{}{
			"http_port":    8080,
			"grpc_port":    9090,
			"log_level":    "info",
			"environment":  "production",
		},
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func performanceHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"cpu": map[string]interface{}{
			"usage_percent": 15.7,
			"cores":         8,
			"load_avg":      []float64{1.2, 1.5, 1.8},
		},
		"memory": map[string]interface{}{
			"used_mb":      256,
			"total_mb":     2048,
			"usage_percent": 12.5,
		},
		"gc": map[string]interface{}{
			"num_gc":       456,
			"pause_total":  "125ms",
			"pause_avg":    "0.27ms",
		},
		"goroutines": map[string]interface{}{
			"active":       45,
			"peak_today":   78,
		},
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

// 工具函数
func writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func startBackgroundTasks(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	log.Println("🔄 后台任务启动")

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 后台任务停止")
			return
		case <-ticker.C:
			// 模拟后台任务：数据收集、清理、监控等
			log.Println("🔄 执行后台任务...")
		}
	}
}

func gracefulShutdown(ctx context.Context, gw *gateway.Gateway) error {
	log.Println("📝 保存状态...")
	time.Sleep(100 * time.Millisecond)

	log.Println("🔄 清理资源...")
	time.Sleep(100 * time.Millisecond)

	log.Println("🛑 关闭Gateway...")
	return gw.Stop()
}

func printEnterpriseInfo(gw *gateway.Gateway) {
	fmt.Println("🏢 ================================================")
	fmt.Println("🚀 Go RPC Gateway - 企业级部署示例")
	fmt.Println("🏢 ================================================")
	fmt.Println()
	
	fmt.Println("🔧 企业级功能:")
	fmt.Println("   ✅ 多版本API支持 (v1, v2)")
	fmt.Println("   ✅ 完整的中间件链")
	fmt.Println("   ✅ 性能监控和分析") 
	fmt.Println("   ✅ 健康检查和监控")
	fmt.Println("   ✅ 配置管理")
	fmt.Println("   ✅ 优雅关闭")
	fmt.Println("   ✅ 后台任务管理")
	
	if gw.IsPProfEnabled() {
		config := gw.GetPProfConfig()
		fmt.Printf("   ✅ 性能分析 (%s)\n", config.PathPrefix)
	}
	
	fmt.Println()
	fmt.Println("📡 业务API端点:")
	fmt.Println("   - GET  /api/v1/users")
	fmt.Println("   - POST /api/v1/users")
	fmt.Println("   - GET  /api/v1/users/{id}")
	fmt.Println("   - GET  /api/v1/orders")
	fmt.Println("   - GET  /api/v1/products")
	fmt.Println("   - GET  /api/v2/users (增强版)")
	fmt.Println("   - GET  /api/v2/analytics")
	
	fmt.Println()
	fmt.Println("🛠️ 管理API端点:")
	fmt.Println("   - GET  /admin/health/detailed")
	fmt.Println("   - GET  /admin/config")
	fmt.Println("   - GET  /admin/metrics/summary")
	fmt.Println("   - GET  /admin/info")
	fmt.Println("   - GET  /admin/performance")
	
	fmt.Println()
	fmt.Println("📊 监控端点:")
	fmt.Println("   - GET  /health (基础健康检查)")
	fmt.Println("   - GET  /metrics (Prometheus指标)")
	
	if gw.IsPProfEnabled() {
		config := gw.GetPProfConfig()
		fmt.Printf("   - GET  %s/ (性能分析)\n", config.PathPrefix)
	}
	
	fmt.Println()
	fmt.Println("🏢 ================================================")
	fmt.Println()
}