/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 18:38:27
 * @FilePath: \go-rpc-gateway\cmd\gateway\main.go
 * @Description: Gateway主程序入口
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/kamalyes/go-core/pkg/global"
	gateway "github.com/kamalyes/go-rpc-gateway"
	"go.uber.org/zap"
)

var (
	configFile = flag.String("config", "config.yaml", "配置文件路径")
)

// loadConfigAndCreateGateway 加载配置并创建网关实例
func loadConfigAndCreateGateway(configFile string) (*gateway.Gateway, error) {
	// 如果提供了配置文件路径，使用该路径创建网关
	if configFile != "" {
		return gateway.NewWithConfigFile(configFile)
	}

	// 否则使用默认配置
	return gateway.New()
}

func main() {
	flag.Parse()

	// 加载配置
	gw, err := loadConfigAndCreateGateway(*configFile)
	if err != nil {
		global.LOG.Warn("使用配置文件创建Gateway失败，尝试使用默认配置", zap.Error(err), zap.String("config_file", *configFile))
		if gw, err = gateway.New(); err != nil {
			global.LOG.Fatal("创建Gateway失败", zap.Error(err))
		}
	} else {
		global.LOG.Info("使用配置文件创建Gateway成功", zap.String("config_file", *configFile))
	}

	global.LOG.Info("🚀 Starting Go RPC Gateway")
	global.LOG.Info("Built with go-config and go-core")

	// 启动Gateway
	if err := gw.Start(); err != nil {
		global.LOG.Fatal("启动Gateway失败", zap.Error(err))
	}

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	global.LOG.Info("🛑 接收到关闭信号，正在优雅关闭Gateway...")

	// 优雅关闭
	if err := gw.Stop(); err != nil {
		global.LOG.Error("Gateway关闭过程中出现错误", zap.Error(err))
	} else {
		global.LOG.Info("✅ Gateway已安全关闭")
	}

	// 同步日志
	global.LOG.Sync()
}
