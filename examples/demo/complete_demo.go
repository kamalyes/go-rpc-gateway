/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-08 00:30:00
 * @FilePath: \go-rpc-gateway\examples\demo\main.go
 * @Description: 完整的Gateway + PProf演示程序
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	gateway "github.com/kamalyes/go-rpc-gateway"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

func main() {
	// 创建Gateway实例
	gw, err := gateway.New()
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	// 启用pprof功能 - 使用自定义配置
	gw.EnablePProfWithOptions(middleware.PProfOptions{
		Enabled:       true,
		AuthToken:     "demo-token-2024",
		PathPrefix:    "/debug/pprof",
		DevModeOnly:   false,
		AllowedIPs:    []string{}, // 允许所有IP访问（仅用于演示）
		EnableLogging: true,
		Timeout:       30,
	})

	// 注册一些演示路由
	registerDemoRoutes(gw)

	// 显示启动信息
	printStartupInfo(gw)

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动服务器
	go func() {
		log.Println("🚀 Starting Gateway server...")
		if err := gw.Start(); err != nil {
			log.Printf("❌ Server error: %v", err)
			cancel()
		}
	}()

	// 启动一个goroutine来生成一些负载
	go generateLoad(ctx)

	// 等待关闭信号
	select {
	case sig := <-sigChan:
		log.Printf("📡 Received signal: %v, shutting down...", sig)
	case <-ctx.Done():
		log.Println("⏹️ Context cancelled, shutting down...")
	}

	// 优雅关闭
	log.Println("⏳ Gracefully shutting down...")
	if err := gw.Stop(); err != nil {
		log.Printf("❌ Error stopping server: %v", err)
	}
	log.Println("✅ Server stopped successfully")
}

// registerDemoRoutes 注册演示路由
func registerDemoRoutes(gw *gateway.Gateway) {
	// 注册简单的API路由
	gw.RegisterHTTPRoute("/api/hello", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"message":   "Hello from Gateway!",
			"timestamp": time.Now().Format(time.RFC3339),
			"method":    r.Method,
			"path":      r.URL.Path,
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	// 注册一个内存分配测试端点
	gw.RegisterHTTPRoute("/api/allocate", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allocateMemory(w, r)
	}))

	// 注册一个CPU密集型测试端点
	gw.RegisterHTTPRoute("/api/cpu-test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cpuIntensiveTask(w, r)
	}))

	// 注册一个协程创建测试端点
	gw.RegisterHTTPRoute("/api/goroutines", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		createGoroutines(w, r)
	}))

	// 注册系统信息端点
	gw.RegisterHTTPRoute("/api/info", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		getSystemInfo(w, r)
	}))
}

// allocateMemory 内存分配测试
func allocateMemory(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	
	// 分配一些内存
	data := make([][]byte, 1000)
	for i := range data {
		data[i] = make([]byte, 1024) // 1KB per allocation
		rand.Read(data[i]) // 填充随机数据
	}
	
	duration := time.Since(start)
	
	response := map[string]interface{}{
		"message":        "Memory allocated successfully",
		"allocations":    len(data),
		"size_per_alloc": "1KB",
		"total_size":     fmt.Sprintf("%dKB", len(data)),
		"duration":       duration.String(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// cpuIntensiveTask CPU密集型任务
func cpuIntensiveTask(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	
	// 执行CPU密集型计算
	result := 0
	for i := 0; i < 10000000; i++ {
		result += i * i
	}
	
	duration := time.Since(start)
	
	response := map[string]interface{}{
		"message":     "CPU intensive task completed",
		"result":      result,
		"iterations":  10000000,
		"duration":    duration.String(),
		"cpu_cores":   runtime.NumCPU(),
		"goroutines":  runtime.NumGoroutine(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// createGoroutines 创建协程测试
func createGoroutines(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	goroutinesBefore := runtime.NumGoroutine()
	
	// 创建一些短生命周期的协程
	numGoroutines := 100
	done := make(chan bool, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			time.Sleep(time.Millisecond * 100) // 短暂工作
			done <- true
		}(i)
	}
	
	// 等待所有协程完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
	
	duration := time.Since(start)
	goroutinesAfter := runtime.NumGoroutine()
	
	response := map[string]interface{}{
		"message":            "Goroutines test completed",
		"goroutines_created": numGoroutines,
		"goroutines_before":  goroutinesBefore,
		"goroutines_after":   goroutinesAfter,
		"duration":           duration.String(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// getSystemInfo 获取系统信息
func getSystemInfo(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	response := map[string]interface{}{
		"system": map[string]interface{}{
			"go_version":     runtime.Version(),
			"cpu_cores":      runtime.NumCPU(),
			"goroutines":     runtime.NumGoroutine(),
			"os":             runtime.GOOS,
			"arch":           runtime.GOARCH,
		},
		"memory": map[string]interface{}{
			"alloc":         bToMb(m.Alloc),
			"total_alloc":   bToMb(m.TotalAlloc),
			"sys":           bToMb(m.Sys),
			"heap_alloc":    bToMb(m.HeapAlloc),
			"heap_sys":      bToMb(m.HeapSys),
			"heap_objects":  m.HeapObjects,
			"stack_inuse":   bToMb(m.StackInuse),
			"stack_sys":     bToMb(m.StackSys),
			"num_gc":        m.NumGC,
			"last_gc":       time.Unix(0, int64(m.LastGC)).Format(time.RFC3339),
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// bToMb 将字节转换为MB
func bToMb(b uint64) string {
	return fmt.Sprintf("%.2f MB", float64(b)/(1024*1024))
}

// generateLoad 生成一些负载
func generateLoad(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 定期分配一些内存
			data := make([]byte, 1024*1024) // 1MB
			rand.Read(data[:100]) // 只填充前100字节
			
			// 强制GC
			runtime.GC()
		}
	}
}

// printStartupInfo 打印启动信息
func printStartupInfo(gw *gateway.Gateway) {
	fmt.Println("🎉 ================================================")
	fmt.Println("🚀 Go RPC Gateway with PProf Demo")
	fmt.Println("🎉 ================================================")
	fmt.Println()
	
	if gw.IsPProfEnabled() {
		config := gw.GetPProfConfig()
		fmt.Printf("✅ PProf Status: ENABLED\n")
		fmt.Printf("🔧 Path Prefix: %s\n", config.PathPrefix)
		fmt.Printf("🔐 Auth Required: %t\n", config.RequireAuth)
		if config.RequireAuth {
			fmt.Printf("🎟️  Auth Token: %s\n", config.AuthToken)
		}
		fmt.Println()
		
		fmt.Println("📊 Available URLs:")
		fmt.Println("   🏠 Main Dashboard: http://localhost:8080/")
		fmt.Println("   📈 PProf Status API: http://localhost:8080/api/pprof/status")
		fmt.Printf("   🔍 PProf Index: http://localhost:8080%s/\n", config.PathPrefix)
		
		if config.RequireAuth {
			fmt.Printf("   🔍 PProf Index (with token): http://localhost:8080%s/?token=%s\n", config.PathPrefix, config.AuthToken)
		}
		
		fmt.Println()
		fmt.Println("🧪 Test Endpoints:")
		fmt.Println("   📡 Hello API: http://localhost:8080/api/hello")
		fmt.Println("   💾 Memory Test: http://localhost:8080/api/allocate")
		fmt.Println("   🔋 CPU Test: http://localhost:8080/api/cpu-test")
		fmt.Println("   🧵 Goroutines Test: http://localhost:8080/api/goroutines")
		fmt.Println("   ℹ️  System Info: http://localhost:8080/api/info")
		fmt.Println()
		
		fmt.Println("🔧 PProf Performance Test Scenarios:")
		fmt.Printf("   📦 Small Objects GC: http://localhost:8080%s/gc/small-objects", config.PathPrefix)
		if config.RequireAuth {
			fmt.Printf("?token=%s", config.AuthToken)
		}
		fmt.Println()
		
		fmt.Printf("   📦 Large Objects GC: http://localhost:8080%s/gc/large-objects", config.PathPrefix)
		if config.RequireAuth {
			fmt.Printf("?token=%s", config.AuthToken)
		}
		fmt.Println()
		
		fmt.Printf("   ⚡ High CPU Test: http://localhost:8080%s/gc/high-cpu", config.PathPrefix)
		if config.RequireAuth {
			fmt.Printf("?token=%s", config.AuthToken)
		}
		fmt.Println()
		
		fmt.Printf("   💾 Memory Allocation: http://localhost:8080%s/memory/allocate", config.PathPrefix)
		if config.RequireAuth {
			fmt.Printf("?token=%s", config.AuthToken)
		}
		fmt.Println()
		
		fmt.Printf("   🔋 CPU Intensive: http://localhost:8080%s/cpu/intensive", config.PathPrefix)
		if config.RequireAuth {
			fmt.Printf("?token=%s", config.AuthToken)
		}
		fmt.Println()
		
		fmt.Println()
		fmt.Println("📖 Usage Tips:")
		fmt.Println("   1. 访问主页查看完整的PProf仪表板")
		fmt.Println("   2. 使用测试端点生成负载")
		fmt.Println("   3. 使用PProf端点分析性能")
		fmt.Println("   4. 按 Ctrl+C 优雅关闭服务器")
		
		if config.RequireAuth {
			fmt.Println()
			fmt.Printf("🔑 认证方式:\n")
			fmt.Printf("   Header: Authorization: Bearer %s\n", config.AuthToken)
			fmt.Printf("   Query:  ?token=%s\n", config.AuthToken)
		}
		
	} else {
		fmt.Println("❌ PProf Status: DISABLED")
	}
	
	fmt.Println()
	fmt.Println("🎉 ================================================")
	fmt.Println()
}