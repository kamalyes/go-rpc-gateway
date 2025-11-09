/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 02:00:25
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
)

var (
	configFile = flag.String("resources", "dev_gateway.yaml", "配置文件路径")
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
		global.LOGGER.WarnKV("使用配置文件创建Gateway失败，尝试使用默认配置", "error", err, "config_file", *configFile)
		if gw, err = gateway.New(); err != nil {
			global.LOGGER.WithError(err).FatalMsg("创建Gateway失败")
		}
	} else {
		global.LOGGER.InfoKV("使用配置文件创建Gateway成功", "config_file", *configFile)
	}

	global.LOGGER.InfoMsg("🚀 Starting Go RPC Gateway")
	global.LOGGER.InfoMsg("Built with go-config and go-core")

	// 启动Gateway（默认显示Banner）
	if err := gw.Start(); err != nil {
		global.LOGGER.WithError(err).FatalMsg("启动Gateway失败")
	}

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 打印关闭横幅
	gw.PrintShutdownInfo()

	// 优雅关闭
	if err := gw.Stop(); err != nil {
		global.LOGGER.WithError(err).ErrorMsg("Gateway关闭过程中出现错误")
	} else {
		// 打印关闭完成信息
		gw.PrintShutdownComplete()
	}
}
