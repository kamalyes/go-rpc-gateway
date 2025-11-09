/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-08 00:30:00
 * @FilePath: \go-rpc-gateway\examples\03-middleware\main.go
 * @Description: 中间件功能演示 - 展示限流、CORS、日志等中间件的使用
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	gateway "github.com/kamalyes/go-rpc-gateway"
)

func main() {
	// 1. 创建Gateway实例
	gw, err := gateway.New()
	if err != nil {
		log.Fatalf("创建Gateway失败: %v", err)
	}

	// 2. 配置中间件
	setupMiddleware(gw)

	// 3. 注册测试路由
	registerTestRoutes(gw)

	log.Println("🚀 中间件功能演示启动中...")
	log.Println("🔧 中间件功能:")
	log.Println("   - ✅ CORS跨域支持")
	log.Println("   - ✅ 限流控制") 
	log.Println("   - ✅ 访问日志记录")
	log.Println("   - ✅ 异常恢复")
	log.Println("   - ✅ 请求ID追踪")
	log.Println("   - ✅ 安全头设置")
	log.Println()
	log.Println("🧪 测试端点:")
	log.Println("   - http://localhost:8080/api/test/cors")
	log.Println("   - http://localhost:8080/api/test/rate-limit")
	log.Println("   - http://localhost:8080/api/test/slow")
	log.Println("   - http://localhost:8080/api/test/error")
	log.Println("   - http://localhost:8080/api/test/panic")
	log.Println("   - http://localhost:8080/api/middleware/status")
	log.Println()
	log.Println("💡 使用curl测试:")
	log.Println(`   curl -H "Origin: https://example.com" http://localhost:8080/api/test/cors`)
	log.Println(`   curl http://localhost:8080/api/test/rate-limit`)
	
	// 4. 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 5. 启动服务器
	go func() {
		if err := gw.Start(); err != nil {
			log.Printf("服务器启动失败: %v", err)
		}
	}()

	// 6. 等待关闭信号
	<-sigChan
	log.Println("🛑 正在优雅关闭服务器...")
	
	if err := gw.Stop(); err != nil {
		log.Printf("关闭服务器时出错: %v", err)
	}
	log.Println("✅ 服务器已成功关闭")
}

// setupMiddleware 配置中间件
func setupMiddleware(gw *gateway.Gateway) {
	// 获取中间件管理器
	manager := gw.GetMiddlewareManager()
	if manager == nil {
		log.Println("⚠️ 中间件管理器未初始化")
		return
	}

	log.Println("🔧 正在配置中间件...")
	log.Println("   - 恢复中间件: 捕获panic并恢复")
	log.Println("   - 请求ID中间件: 为每个请求生成唯一ID")
	log.Println("   - 日志中间件: 记录请求和响应")
	log.Println("   - CORS中间件: 处理跨域请求")
	log.Println("   - 安全中间件: 设置安全响应头")
}

// registerTestRoutes 注册测试路由
func registerTestRoutes(gw *gateway.Gateway) {
	// CORS测试
	gw.RegisterHTTPRoute("/api/test/cors", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"message": "CORS test successful",
			"origin":  r.Header.Get("Origin"),
			"method":  r.Method,
			"headers": map[string]string{
				"Access-Control-Allow-Origin":  w.Header().Get("Access-Control-Allow-Origin"),
				"Access-Control-Allow-Methods": w.Header().Get("Access-Control-Allow-Methods"),
				"Access-Control-Allow-Headers": w.Header().Get("Access-Control-Allow-Headers"),
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	// 限流测试
	gw.RegisterHTTPRoute("/api/test/rate-limit", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"message":   "Rate limit test - request processed",
			"timestamp": time.Now().Format(time.RFC3339),
			"tip":       "Send multiple requests quickly to test rate limiting",
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	// 慢请求测试（用于日志记录）
	gw.RegisterHTTPRoute("/api/test/slow", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟慢请求
		delay := rand.Intn(3) + 1 // 1-3秒随机延迟
		time.Sleep(time.Duration(delay) * time.Second)
		
		response := map[string]interface{}{
			"message": "Slow request completed",
			"delay":   fmt.Sprintf("%ds", delay),
			"note":    "Check logs for request duration",
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	// 错误测试
	gw.RegisterHTTPRoute("/api/test/error", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errorCode := r.URL.Query().Get("code")
		if errorCode == "" {
			errorCode = "500"
		}
		
		code, _ := strconv.Atoi(errorCode)
		if code < 400 || code > 599 {
			code = 500
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   true,
			"code":    code,
			"message": fmt.Sprintf("Test error with code %d", code),
			"tip":     "Use ?code=404 to test different error codes",
		})
	}))

	// Panic测试（用于恢复中间件）
	gw.RegisterHTTPRoute("/api/test/panic", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 这会触发panic，但恢复中间件会捕获它
		panic("Test panic for recovery middleware!")
	}))

	// 中间件状态
	gw.RegisterHTTPRoute("/api/middleware/status", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"middleware_status": map[string]interface{}{
				"recovery":    "enabled",
				"request_id":  "enabled", 
				"logging":     "enabled",
				"cors":        "enabled",
				"security":    "enabled",
				"rate_limit":  "enabled",
			},
			"request_info": map[string]interface{}{
				"method":     r.Method,
				"path":       r.URL.Path,
				"user_agent": r.Header.Get("User-Agent"),
				"request_id": r.Header.Get("X-Request-ID"),
			},
			"security_headers": map[string]string{
				"X-Frame-Options":        w.Header().Get("X-Frame-Options"),
				"X-Content-Type-Options": w.Header().Get("X-Content-Type-Options"),
				"X-XSS-Protection":       w.Header().Get("X-XSS-Protection"),
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	// 压力测试端点
	gw.RegisterHTTPRoute("/api/test/stress", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟一些CPU工作
		start := time.Now()
		for i := 0; i < 1000000; i++ {
			_ = i * i
		}
		duration := time.Since(start)
		
		response := map[string]interface{}{
			"message":        "Stress test completed",
			"duration":       duration.String(),
			"iterations":     1000000,
			"requests_count": "Check logs for rate limiting",
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}