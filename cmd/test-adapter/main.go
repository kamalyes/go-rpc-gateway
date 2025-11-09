/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:45:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-08 00:45:00
 * @FilePath: \go-rpc-gateway\cmd\test-adapter\main.go
 * @Description: 测试适配器模式的pprof集成
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package main

import (
	"log"
	"net/http"
	"os"

	"github.com/kamalyes/go-config/pkg/register"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

func main() {
	log.Println("🔧 测试适配器模式的PProf集成...")

	// 1. 创建go-config的pprof配置
	pprofConfig := &register.PProf{
		Enabled:       true,
		PathPrefix:    "/debug/pprof",
		RequireAuth:   true,
		AuthToken:     getEnvOrDefault("PPROF_TOKEN", "test-adapter-token"),
		AllowedIPs:    []string{}, // 允许所有IP
		EnableLogging: true,
		Timeout:       30,
	}

	// 2. 创建适配器
	adapter := middleware.NewPProfConfigAdapter(pprofConfig)
	
	// 3. 注册性能测试场景
	adapter.RegisterScenarios()

	// 4. 创建pprof中间件
	pprofMiddleware := middleware.PProfMiddleware(adapter)

	// 5. 创建HTTP服务器
	mux := http.NewServeMux()

	// 添加业务路由
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := `<!DOCTYPE html>
<html>
<head><title>PProf Adapter Test</title></head>
<body>
    <h1>🔧 PProf 适配器模式测试</h1>
    <p>✅ 适配器模式正常工作！</p>
    <p>🔐 认证token: ` + pprofConfig.AuthToken + `</p>
    <h2>测试链接：</h2>
    <ul>
        <li><a href="/debug/pprof/?token=` + pprofConfig.AuthToken + `">PProf 索引</a></li>
        <li><a href="/debug/pprof/gc/small-objects?token=` + pprofConfig.AuthToken + `">小对象GC测试</a></li>
        <li><a href="/debug/pprof/heap?token=` + pprofConfig.AuthToken + `">内存堆分析</a></li>
    </ul>
    <p>🚀 服务器正在使用go-config的register.PProf配置</p>
</body>
</html>`
		w.Write([]byte(html))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"status": "ok", 
			"pprof_enabled": true,
			"adapter_mode": true,
			"config_type": "register.PProf"
		}`))
	})

	// 6. 应用中间件
	handler := pprofMiddleware(mux)

	// 7. 启动服务器
	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	log.Println("✅ 适配器创建成功!")
	log.Println("")
	log.Println("📊 访问地址:")
	log.Println("   🌐 主页面: http://localhost:8080/")
	log.Println("   💗 健康检查: http://localhost:8080/health")
	log.Println("   📈 PProf: http://localhost:8080/debug/pprof/")
	log.Println("")
	log.Printf("🔐 认证token: %s", pprofConfig.AuthToken)
	log.Println("   (Header: Authorization: Bearer <token>)")
	log.Println("   (Query: ?token=<token>)")
	log.Println("")
	log.Println("🚀 启动服务器...")

	if err := server.ListenAndServe(); err != nil {
		log.Fatal("❌ 启动失败:", err)
	}
}

// getEnvOrDefault 获取环境变量或返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}