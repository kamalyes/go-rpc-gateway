/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-13 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 00:00:00
 * @FilePath: \go-rpc-gateway\examples\main.go
 * @Description: 基于go-config重构后的Gateway示例启动程序
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	goconfig "github.com/kamalyes/go-config"
	gateway "github.com/kamalyes/go-rpc-gateway"
	"github.com/kamalyes/go-rpc-gateway/global"
)

var (
	configPath   = flag.String("config", "./config", "配置文件路径或目录")
	autoDiscover = flag.Bool("auto", false, "启用自动配置发现模式")
	environment  = flag.String("env", "", "指定环境 (dev, sit, fat, uat, prod)")
	showVersion  = flag.Bool("version", false, "显示版本信息")
	showHelp     = flag.Bool("help", false, "显示帮助信息")
)

func main() {
	flag.Parse()
	
	if *showHelp {
		showUsage()
		return
	}
	
	if *showVersion {
		showVersionInfo()
		return
	}
	
	// 设置环境变量（如果指定）
	if *environment != "" {
		env := goconfig.EnvironmentType(*environment)
		if err := global.SetEnvironment(env); err != nil {
			fmt.Printf("❌ 设置环境失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("🌍 环境已设置为: %s\n", env)
	}
	
	// 初始化配置
	if err := initializeConfig(); err != nil {
		fmt.Printf("❌ 初始化配置失败: %v\n", err)
		os.Exit(1)
	}
	
	// 创建网关实例
	gw, err := gateway.New()
	if err != nil {
		fmt.Printf("❌ 创建网关失败: %v\n", err)
		os.Exit(1)
	}
	
	// 注册信号处理
	setupSignalHandling(gw)
	
	// 启动服务
	fmt.Printf("🚀 正在启动 Gateway 服务...\n")
	if err := gw.StartWithBanner(); err != nil {
		fmt.Printf("❌ 启动服务失败: %v\n", err)
		os.Exit(1)
	}
	
	// 等待信号
	waitForShutdown(gw)
}

// initializeConfig 初始化配置
func initializeConfig() error {
	if *autoDiscover {
		fmt.Printf("🔍 使用自动发现模式初始化配置...\n")
		return global.InitializeGatewayWithAutoDiscovery(*configPath)
	} else {
		fmt.Printf("📁 使用指定路径初始化配置...\n")
		return global.InitializeGatewayWithConfigPath(*configPath)
	}
}

// setupSignalHandling 设置信号处理
func setupSignalHandling(gw *gateway.Gateway) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		sig := <-c
		fmt.Printf("\n🛑 收到信号 %v，开始优雅关闭...\n", sig)
		
		// 显示关闭信息
		gw.PrintShutdownInfo()
		
		// 停止服务
		if err := gw.Stop(); err != nil {
			fmt.Printf("❌ 停止服务时发生错误: %v\n", err)
		}
		
		// 清理全局资源
		global.CleanupGlobal()
		
		// 显示关闭完成信息
		gw.PrintShutdownComplete()
		
		os.Exit(0)
	}()
}

// waitForShutdown 等待关闭信号
func waitForShutdown(gw *gateway.Gateway) {
	// 设置优雅关闭
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("❌ Gateway 发生panic: %v\n", r)
			gw.PrintShutdownInfo()
			global.CleanupGlobal()
		}
	}()
	
	// 阻塞直到收到信号
	select {}
}

// showUsage 显示使用说明
func showUsage() {
	fmt.Printf(`
🌟 go-rpc-gateway 企业级微服务网关框架

用法:
  go run main.go [选项]

选项:
  -config string    配置文件路径或目录 (默认: "./config")
  -auto            启用自动配置发现模式
  -env string      指定环境 (dev, sit, fat, uat, prod)
  -version         显示版本信息
  -help            显示此帮助信息

示例:
  # 使用指定配置文件
  go run main.go -config ./config/gateway-dev.yaml

  # 使用自动发现模式
  go run main.go -config ./config -auto

  # 指定环境
  go run main.go -config ./config -auto -env dev

环境变量:
  APP_ENV          应用环境 (dev, sit, fat, uat, prod)

配置文件支持格式:
  - YAML (.yaml, .yml)
  - JSON (.json) 
  - TOML (.toml)

更多信息请访问: https://github.com/kamalyes/go-rpc-gateway
`)
}

// showVersionInfo 显示版本信息
func showVersionInfo() {
	fmt.Printf(`
🌟 go-rpc-gateway v1.0.0

构建信息:
  - 基于 go-config 配置管理
  - 支持配置热更新
  - 企业级微服务网关
  - 高性能 gRPC-Gateway

作者: kamalyes
许可证: MIT
仓库: https://github.com/kamalyes/go-rpc-gateway
`)
}