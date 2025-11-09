/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 00:16:00
 * @FilePath: \go-rpc-gateway\examples\01-quickstart\main.go
 * @Description: 快速入门示例 - 集成go-config、go-core、go-logger的Gateway使用方式
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kamalyes/go-core/pkg/global"
	gateway "github.com/kamalyes/go-rpc-gateway"
)

func main() {
	// 1. 创建Gateway实例（使用默认配置）
	gw, err := gateway.New()
	if err != nil {
		global.LOGGER.Error("创建Gateway失败: %v", err)
		panic(err)
	}

	// 2. 注册一个简单的HTTP路由
	gw.RegisterHTTPRoute("/api/hello", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"message":   "Hello from Go RPC Gateway!",
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
			"framework": "go-rpc-gateway with go-config, go-core, go-logger",
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	// 3. 注册健康检查路由
	gw.RegisterHTTPRoute("/api/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"go-rpc-gateway"}`))
	}))

	// 4. 设置优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 5. 启动服务器（会自动显示启动信息）
	go func() {
		if err := gw.Start(); err != nil {
			global.LOGGER.Error("服务器启动失败: %v", err)
			cancel()
		}
	}()

	// 6. 等待关闭信号
	select {
	case sig := <-sigChan:
		global.LOGGER.Info("接收到信号: %v", sig)
	case <-ctx.Done():
		global.LOGGER.Info("上下文已取消")
	}

	global.LOGGER.Info("🛑 正在优雅关闭服务器...")
	if err := gw.Stop(); err != nil {
		global.LOGGER.Error("关闭服务器时出错: %v", err)
	}
	global.LOGGER.Info("✅ 服务器已成功关闭")
}