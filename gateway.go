/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 15:11:15
 * @FilePath: \go-rpc-gateway\gateway.go
 * @Description: Gateway主入口，基于go-config
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

// Package gateway 提供一个轻量级的gRPC-Gateway框架
// 集成了数据库、Redis和对象存储等组件
// 基于go-config
package gateway

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	goconfig "github.com/kamalyes/go-config"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-rpc-gateway/server"
	"google.golang.org/grpc"
)

// Gateway 是主要的网关服务器
type Gateway struct {
	*server.Server
	configManager *goconfig.IntegratedConfigManager
	gatewayConfig *gwconfig.Gateway
}

// ServiceRegisterFunc gRPC服务注册函数类型
type ServiceRegisterFunc func(*grpc.Server)

// HandlerRegisterFunc HTTP处理器注册函数类型
type HandlerRegisterFunc func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error

// New 创建新的网关实例 - 使用全局配置
func New() (*Gateway, error) {
	srv, err := server.NewServer()
	if err != nil {
		return nil, err
	}

	return &Gateway{
		Server: srv,
	}, nil
}

// RegisterService 注册gRPC服务
func (g *Gateway) RegisterService(registerFunc ServiceRegisterFunc) {
	g.Server.RegisterGRPCService(registerFunc)
}

// RegisterHandler 注册HTTP处理器
func (g *Gateway) RegisterHandler(pattern string, handler http.Handler) {
	g.Server.RegisterHTTPRoute(pattern, handler)
}

// RegisterHTTPRoute 注册HTTP路由 (便捷方法)
func (g *Gateway) RegisterHTTPRoute(pattern string, handlerFunc http.HandlerFunc) {
	g.Server.RegisterHTTPRoute(pattern, handlerFunc)
}

// RegisterHTTPRoutes 批量注册HTTP路由
func (g *Gateway) RegisterHTTPRoutes(routes map[string]http.HandlerFunc) {
	for pattern, handler := range routes {
		g.RegisterHTTPRoute(pattern, handler)
	}
}

// EnableSwagger 启用 Swagger 文档服务 (委托给 Server 层)
func (g *Gateway) EnableSwagger() error {
	return g.Server.EnableFeature(server.FeatureSwagger)
}

// EnableSwaggerWithConfig 使用自定义配置启用 Swagger
func (g *Gateway) EnableSwaggerWithConfig(config interface{}) error {
	return g.Server.EnableFeatureWithConfig(server.FeatureSwagger, config)
}

// IsSwaggerEnabled 检查 Swagger 是否已启用
func (g *Gateway) IsSwaggerEnabled() bool {
	return g.Server.IsFeatureEnabled(server.FeatureSwagger)
}

// EnableMonitoring 启用监控功能
func (g *Gateway) EnableMonitoring() error {
	return g.Server.EnableFeature(server.FeatureMonitoring)
}

// EnableMonitoringWithConfig 使用自定义配置启用监控
func (g *Gateway) EnableMonitoringWithConfig(config interface{}) error {
	return g.Server.EnableFeatureWithConfig(server.FeatureMonitoring, config)
}

// IsMonitoringEnabled 检查监控是否已启用
func (g *Gateway) IsMonitoringEnabled() bool {
	return g.Server.IsFeatureEnabled(server.FeatureMonitoring)
}

// EnableHealth 启用健康检查功能
func (g *Gateway) EnableHealth() error {
	return g.Server.EnableFeature(server.FeatureHealth)
}

// EnableHealthWithConfig 使用自定义配置启用健康检查
func (g *Gateway) EnableHealthWithConfig(config interface{}) error {
	return g.Server.EnableFeatureWithConfig(server.FeatureHealth, config)
}

// IsHealthEnabled 检查健康检查是否已启用
func (g *Gateway) IsHealthEnabled() bool {
	return g.Server.IsFeatureEnabled(server.FeatureHealth)
}

// EnablePProf 启用性能分析功能
func (g *Gateway) EnablePProf() error {
	return g.Server.EnableFeature(server.FeaturePProf)
}

// EnablePProfWithConfig 使用自定义配置启用性能分析
func (g *Gateway) EnablePProfWithConfig(config interface{}) error {
	return g.Server.EnableFeatureWithConfig(server.FeaturePProf, config)
}

// IsPProfEnabled 检查性能分析是否已启用
func (g *Gateway) IsPProfEnabled() bool {
	return g.Server.IsFeatureEnabled(server.FeaturePProf)
}

// EnableTracing 启用链路追踪功能
func (g *Gateway) EnableTracing() error {
	return g.Server.EnableFeature(server.FeatureTracing)
}

// EnableTracingWithConfig 使用自定义配置启用链路追踪
func (g *Gateway) EnableTracingWithConfig(config interface{}) error {
	return g.Server.EnableFeatureWithConfig(server.FeatureTracing, config)
}

// IsTracingEnabled 检查链路追踪是否已启用
func (g *Gateway) IsTracingEnabled() bool {
	return g.Server.IsFeatureEnabled(server.FeatureTracing)
}

// EnableFeature 启用指定功能（通用接口）
func (g *Gateway) EnableFeature(feature server.FeatureType) error {
	return g.Server.EnableFeature(feature)
}

// EnableFeatureWithConfig 使用自定义配置启用功能（通用接口）
func (g *Gateway) EnableFeatureWithConfig(feature server.FeatureType, config interface{}) error {
	return g.Server.EnableFeatureWithConfig(feature, config)
}

// IsFeatureEnabled 检查功能是否已启用（通用接口）
func (g *Gateway) IsFeatureEnabled(feature server.FeatureType) bool {
	return g.Server.IsFeatureEnabled(feature)
}

// GetConfig 获取网关配置
func (g *Gateway) GetConfig() *gwconfig.Gateway {
	return g.Server.GetConfig()
}

// Start 启动网关服务并显示banner（默认行为）
func (g *Gateway) Start() error {
	return g.StartWithBanner()
}

// StartSilent 静默启动网关服务（不显示banner）
func (g *Gateway) StartSilent() error {
	return g.Server.Start()
}

// StartWithBanner 启动网关服务并显示banner
func (g *Gateway) StartWithBanner() error {
	// 启动服务
	if err := g.Server.Start(); err != nil {
		return err
	}

	// 显示启动banner
	g.PrintStartupInfo()
	return nil
}

// Stop 停止网关服务
func (g *Gateway) Stop() error {
	// 先停止服务器
	err := g.Server.Stop()

	// 再停止配置管理器
	if g.configManager != nil {
		g.configManager.Stop()
	}

	return err
}

// PrintStartupInfo 打印启动信息
func (g *Gateway) PrintStartupInfo() {
	if bannerManager := g.Server.GetBannerManager(); bannerManager != nil {
		bannerManager.PrintStartupBanner()
		bannerManager.PrintMiddlewareStatus()
		bannerManager.PrintUsageGuide()
	}
}

// PrintShutdownInfo 打印关闭信息
func (g *Gateway) PrintShutdownInfo() {
	if bannerManager := g.Server.GetBannerManager(); bannerManager != nil {
		bannerManager.PrintShutdownBanner()
	}
}

// PrintShutdownComplete 打印关闭完成信息
func (g *Gateway) PrintShutdownComplete() {
	if bannerManager := g.Server.GetBannerManager(); bannerManager != nil {
		bannerManager.PrintShutdownComplete()
	}
}

// GetGatewayConfig 获取网关配置
func (g *Gateway) GetGatewayConfig() *gwconfig.Gateway {
	return g.gatewayConfig
}

// CreateConfigManager 创建配置管理器
func (g *Gateway) CreateConfigManager(config *gwconfig.Gateway, configPath string) (*goconfig.IntegratedConfigManager, error) {
	// 检查configPath是文件还是目录
	if stat, err := os.Stat(configPath); err == nil && stat.IsDir() {
		fmt.Printf("🔍 使用自动发现模式，搜索路径: %s\n", configPath)

		// 使用自动发现创建管理器
		return goconfig.CreateAndStartIntegratedManagerWithAutoDiscovery(
			config,
			configPath,
			goconfig.GetEnvironment(),
			"gateway",
		)
	} else {
		fmt.Printf("📄 使用指定配置文件: %s\n", configPath)

		// 使用传统方式
		return goconfig.CreateAndStartIntegratedManager(
			config,
			configPath,
			goconfig.GetEnvironment(),
		)
	}
}

// RegisterConfigCallbacks 注册配置变更回调
func (g *Gateway) RegisterConfigCallbacks() {
	if g.configManager == nil {
		return
	}

	// 注册配置变更回调
	g.configManager.RegisterConfigCallback(func(ctx context.Context, event goconfig.CallbackEvent) error {
		if newConfig, ok := event.NewValue.(*gwconfig.Gateway); ok {
			fmt.Printf("📋 配置已更新: %s\n", newConfig.Name)
			g.gatewayConfig = newConfig
			if newConfig.HTTPServer != nil {
				fmt.Printf("🌐 HTTP端点: %s\n", newConfig.HTTPServer.GetEndpoint())
			}
		}
		return nil
	}, goconfig.CallbackOptions{
		ID:       "gateway_config_handler",
		Types:    []goconfig.CallbackType{goconfig.CallbackTypeConfigChanged},
		Priority: goconfig.CallbackPriorityHigh,
		Async:    false,
		Timeout:  5 * time.Second,
	})

	// 注册环境变更回调
	g.configManager.RegisterEnvironmentCallback("gateway_env_handler", func(oldEnv, newEnv goconfig.EnvironmentType) error {
		fmt.Printf("🌍 环境变更: %s -> %s\n", oldEnv, newEnv)
		return nil
	}, goconfig.CallbackPriorityHigh, false)
}
