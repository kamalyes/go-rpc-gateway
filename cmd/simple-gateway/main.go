/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-08 00:30:00
 * @FilePath: \go-rpc-gateway\cmd\simple-gateway\main.go
 * @Description: 最简单的Gateway + PProf 示例
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package main

import (
	"log"
	"net/http"

	"github.com/kamalyes/go-rpc-gateway"
)

func main() {
	log.Println("🚀 启动最简单的Gateway with PProf示例...")

	// 1. 创建Gateway实例
	gw, err := gateway.New()
	if err != nil {
		log.Fatal("❌ 创建Gateway失败:", err)
	}

	// 2. 一键启用pprof! 🎉
	gw.EnablePProf()

	// 3. 添加一个简单的业务路由
	gw.RegisterHTTPRoute("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"message": "Hello from Gateway!", 
			"pprof_enabled": true,
			"tip": "访问 / 查看pprof界面"
		}`))
	})

	// 4. 添加健康检查
	gw.RegisterHTTPRoute("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok", "service": "simple-gateway"}`))
	})

	// 输出访问信息
	log.Println("✅ Gateway配置完成!")
	log.Println("")
	log.Println("📊 访问地址:")
	log.Println("   🌐 PProf界面: http://localhost:8080/")
	log.Println("   🧪 业务接口: http://localhost:8080/hello")
	log.Println("   💗 健康检查: http://localhost:8080/health")
	log.Println("   📈 PProf API: http://localhost:8080/debug/pprof/")
	log.Println("")
	log.Println("🔐 默认认证token: gateway-pprof-2024")
	log.Println("   (可设置环境变量 PPROF_TOKEN 自定义)")
	log.Println("")

	// 5. 启动服务 (会自动处理pprof路由)
	log.Println("🚀 启动服务中...")
	if err := gw.Start(); err != nil {
		log.Fatal("❌ 启动服务失败:", err)
	}
}