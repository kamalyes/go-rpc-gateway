/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-08 00:30:00
 * @FilePath: \go-rpc-gateway\examples\04-pprof\main.go
 * @Description: PProf性能分析完整演示
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
	// 1. 创建Gateway实例
	gw, err := gateway.New()
	if err != nil {
		log.Fatalf("创建Gateway失败: %v", err)
	}

	// 2. 启用PProf功能
	gw.EnablePProfWithOptions(middleware.PProfOptions{
		Enabled:       true,
		AuthToken:     "pprof-demo-2024",
		PathPrefix:    "/debug/pprof",
		DevModeOnly:   false,
		AllowedIPs:    []string{}, // 允许所有IP（仅用于演示）
		EnableLogging: true,
		Timeout:       30,
	})

	// 3. 注册性能测试API
	registerPerformanceTestAPI(gw)

	// 4. 打印启动信息
	printPProfInfo(gw)

	// 5. 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 6. 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 7. 启动后台负载生成器
	go generateBackgroundLoad(ctx)

	// 8. 启动服务器
	go func() {
		if err := gw.Start(); err != nil {
			log.Printf("服务器启动失败: %v", err)
			cancel()
		}
	}()

	// 9. 等待关闭信号
	select {
	case sig := <-sigChan:
		log.Printf("接收到信号: %v", sig)
	case <-ctx.Done():
		log.Println("上下文已取消")
	}

	log.Println("🛑 正在优雅关闭服务器...")
	if err := gw.Stop(); err != nil {
		log.Printf("关闭服务器时出错: %v", err)
	}
	log.Println("✅ 服务器已成功关闭")
}

// registerPerformanceTestAPI 注册性能测试API
func registerPerformanceTestAPI(gw *gateway.Gateway) {
	// 内存分配测试
	gw.RegisterHTTPRoute("/api/perf/memory", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// 分配不同大小的内存块
		var allocations [][]byte
		sizes := []int{1024, 4096, 65536, 1048576} // 1KB, 4KB, 64KB, 1MB
		
		for _, size := range sizes {
			for i := 0; i < 100; i++ {
				data := make([]byte, size)
				rand.Read(data[:10]) // 只填充前10字节
				allocations = append(allocations, data)
			}
		}

		duration := time.Since(start)
		
		// 获取内存统计
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		response := map[string]interface{}{
			"test":         "memory allocation",
			"allocations":  len(allocations),
			"total_size":   calculateTotalSize(allocations),
			"duration":     duration.String(),
			"memory_stats": map[string]interface{}{
				"alloc":      bToMb(m.Alloc),
				"sys":        bToMb(m.Sys),
				"heap_alloc": bToMb(m.HeapAlloc),
				"heap_sys":   bToMb(m.HeapSys),
				"gc_runs":    m.NumGC,
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	// CPU密集型测试
	gw.RegisterHTTPRoute("/api/perf/cpu", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// 执行CPU密集型计算
		result := fibonacci(35)
		
		duration := time.Since(start)
		
		response := map[string]interface{}{
			"test":        "cpu intensive",
			"function":    "fibonacci(35)",
			"result":      result,
			"duration":    duration.String(),
			"cpu_cores":   runtime.NumCPU(),
			"goroutines":  runtime.NumGoroutine(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	// Goroutine创建测试
	gw.RegisterHTTPRoute("/api/perf/goroutines", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		goroutinesBefore := runtime.NumGoroutine()
		
		// 创建短生命周期的goroutines
		numGoroutines := 1000
		done := make(chan bool, numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				time.Sleep(time.Millisecond * 50)
				done <- true
			}(i)
		}
		
		// 等待所有goroutine完成
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
		
		duration := time.Since(start)
		goroutinesAfter := runtime.NumGoroutine()
		
		response := map[string]interface{}{
			"test":               "goroutine creation",
			"goroutines_created": numGoroutines,
			"goroutines_before":  goroutinesBefore,
			"goroutines_after":   goroutinesAfter,
			"duration":           duration.String(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	// GC强制触发
	gw.RegisterHTTPRoute("/api/perf/gc", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m1, m2 runtime.MemStats
		runtime.ReadMemStats(&m1)
		
		start := time.Now()
		runtime.GC()
		duration := time.Since(start)
		
		runtime.ReadMemStats(&m2)
		
		response := map[string]interface{}{
			"test":     "garbage collection",
			"duration": duration.String(),
			"before": map[string]interface{}{
				"alloc":    bToMb(m1.Alloc),
				"heap_alloc": bToMb(m1.HeapAlloc),
				"gc_runs":  m1.NumGC,
			},
			"after": map[string]interface{}{
				"alloc":    bToMb(m2.Alloc),
				"heap_alloc": bToMb(m2.HeapAlloc),
				"gc_runs":  m2.NumGC,
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	// 系统信息API
	gw.RegisterHTTPRoute("/api/perf/status", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		response := map[string]interface{}{
			"pprof": map[string]interface{}{
				"enabled":   gw.IsPProfEnabled(),
				"endpoints": len(gw.GetPProfEndpoints()),
			},
			"system": map[string]interface{}{
				"go_version":  runtime.Version(),
				"cpu_cores":   runtime.NumCPU(),
				"goroutines":  runtime.NumGoroutine(),
				"os":          runtime.GOOS,
				"arch":        runtime.GOARCH,
			},
			"memory": map[string]interface{}{
				"alloc":         bToMb(m.Alloc),
				"total_alloc":   bToMb(m.TotalAlloc),
				"sys":           bToMb(m.Sys),
				"heap_alloc":    bToMb(m.HeapAlloc),
				"heap_sys":      bToMb(m.HeapSys),
				"heap_objects":  m.HeapObjects,
				"gc_runs":       m.NumGC,
				"last_gc":       time.Unix(0, int64(m.LastGC)).Format(time.RFC3339),
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

// generateBackgroundLoad 生成后台负载
func generateBackgroundLoad(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	log.Println("🔄 后台负载生成器已启动")
	
	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 后台负载生成器已停止")
			return
		case <-ticker.C:
			// 定期分配一些内存
			data := make([]byte, 1024*1024) // 1MB
			rand.Read(data[:100])
			
			// 执行一些计算
			go func() {
				for i := 0; i < 100000; i++ {
					_ = i * i
				}
			}()
			
			// 定期触发GC
			if rand.Intn(5) == 0 {
				runtime.GC()
			}
		}
	}
}

// fibonacci 计算斐波那契数列（递归版本，CPU密集）
func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	return fibonacci(n-1) + fibonacci(n-2)
}

// calculateTotalSize 计算分配的总内存大小
func calculateTotalSize(allocations [][]byte) string {
	total := 0
	for _, data := range allocations {
		total += len(data)
	}
	return bToMb(uint64(total))
}

// bToMb 将字节转换为MB字符串
func bToMb(b uint64) string {
	return fmt.Sprintf("%.2f MB", float64(b)/(1024*1024))
}

// printPProfInfo 打印PProf信息
func printPProfInfo(gw *gateway.Gateway) {
	fmt.Println("🎉 ================================================")
	fmt.Println("🔬 Go RPC Gateway PProf 性能分析演示")
	fmt.Println("🎉 ================================================")
	fmt.Println()
	
	if !gw.IsPProfEnabled() {
		fmt.Println("❌ PProf未启用")
		return
	}
	
	config := gw.GetPProfConfig()
	token := config.AuthToken
	
	fmt.Printf("✅ PProf状态: 已启用\n")
	fmt.Printf("🔑 认证Token: %s\n", token)
	fmt.Printf("🌐 路径前缀: %s\n", config.PathPrefix)
	fmt.Println()
	
	fmt.Println("📊 主要访问地址:")
	fmt.Println("   🏠 PProf仪表板: http://localhost:8080/")
	fmt.Printf("   🔍 PProf索引: http://localhost:8080%s/?token=%s\n", config.PathPrefix, token)
	fmt.Printf("   📈 CPU分析: http://localhost:8080%s/profile?seconds=30&token=%s\n", config.PathPrefix, token)
	fmt.Printf("   💾 内存分析: http://localhost:8080%s/heap?token=%s\n", config.PathPrefix, token)
	fmt.Printf("   🧵 协程分析: http://localhost:8080%s/goroutine?token=%s\n", config.PathPrefix, token)
	fmt.Println()
	
	fmt.Println("🧪 性能测试API:")
	fmt.Println("   💾 内存测试: http://localhost:8080/api/perf/memory")
	fmt.Println("   🔋 CPU测试: http://localhost:8080/api/perf/cpu")
	fmt.Println("   🧵 协程测试: http://localhost:8080/api/perf/goroutines")
	fmt.Println("   🗑️ GC测试: http://localhost:8080/api/perf/gc")
	fmt.Println("   ℹ️ 系统状态: http://localhost:8080/api/perf/status")
	fmt.Println()
	
	fmt.Println("🛠️ 使用指南:")
	fmt.Println("   1. 访问API端点生成负载")
	fmt.Println("   2. 使用PProf端点收集性能数据")
	fmt.Println("   3. 使用go tool pprof分析数据")
	fmt.Println()
	
	fmt.Println("💡 命令行分析示例:")
	fmt.Printf("   curl -H \"Authorization: Bearer %s\" \"http://localhost:8080%s/profile?seconds=30\" -o cpu.prof\n", token, config.PathPrefix)
	fmt.Println("   go tool pprof cpu.prof")
	fmt.Println("   (pprof) top10")
	fmt.Println("   (pprof) web")
	fmt.Println()
	fmt.Println("🔄 后台负载生成器将自动运行，提供持续的性能数据")
	fmt.Println("🎉 ================================================")
	fmt.Println()
}