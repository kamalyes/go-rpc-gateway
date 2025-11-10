/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 10:33:16
 * @FilePath: \go-rpc-gateway\server\banner.go
 * @Description: Gateway启动横幅和信息展示
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"fmt"
	"runtime"
	"time"

	"github.com/kamalyes/go-core/pkg/global"
	"github.com/kamalyes/go-rpc-gateway/config"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

// BannerManager 横幅管理器
type BannerManager struct {
	config   *config.GatewayConfig
	features []string
}

// NewBannerManager 创建横幅管理器
func NewBannerManager(config *config.GatewayConfig) *BannerManager {
	return &BannerManager{
		config:   config,
		features: []string{},
	}
}

// AddFeature 添加功能特性
func (b *BannerManager) AddFeature(feature string) {
	b.features = append(b.features, feature)
}

// PrintStartupBanner 打印启动横幅
func (b *BannerManager) PrintStartupBanner() {
	global.LOGGER.Info("🎉 ================================================")
	global.LOGGER.Info("🚀 Go RPC Gateway - Enterprise Edition")
	global.LOGGER.Info("🎉 ================================================")
	global.LOGGER.Info("")

	// 基础信息
	b.printBasicInfo()
	global.LOGGER.Info("")

	// 服务器配置
	b.printServerConfig()
	global.LOGGER.Info("")

	// 功能特性
	b.printFeatures()
	global.LOGGER.Info("")

	// 端点信息
	b.printEndpoints()
	global.LOGGER.Info("")

	// 系统信息
	b.printSystemInfo()
	global.LOGGER.Info("")

	global.LOGGER.Info("🎉 ================================================")
	global.LOGGER.Info("")
}

// PrintShutdownBanner 打印关闭横幅
func (b *BannerManager) PrintShutdownBanner() {
	global.LOGGER.Info("🛑 ================================================")
	global.LOGGER.Info("⏹️  Gateway正在优雅关闭...")
	global.LOGGER.Info("🛑 ================================================")
}

// PrintShutdownComplete 打印关闭完成
func (b *BannerManager) PrintShutdownComplete() {
	global.LOGGER.Info("✅ Gateway已安全关闭")
	global.LOGGER.Info("👋 感谢使用 Go RPC Gateway！")
}

// printBasicInfo 打印基础信息
func (b *BannerManager) printBasicInfo() {
	global.LOGGER.Info("📋 基础信息:")
	global.LOGGER.Info("   🏷️  名称: " + b.config.Gateway.Name)
	global.LOGGER.Info("   📦 版本: " + b.config.Gateway.Version)
	global.LOGGER.Info("   🌍 环境: " + b.config.Gateway.Environment)
	global.LOGGER.Info("   🔧 调试模式: " + fmt.Sprintf("%t", b.config.Gateway.Debug))
	global.LOGGER.Info("   🏗️  框架: go-rpc-gateway (基于 go-config & go-core)")
}

// printServerConfig 打印服务器配置
func (b *BannerManager) printServerConfig() {
	global.LOGGER.Info("⚙️  服务器配置:")
	global.LOGGER.Info("   🌐 HTTP服务器: " + fmt.Sprintf("%s:%d", b.config.Gateway.HTTP.Host, b.config.Gateway.HTTP.Port))
	global.LOGGER.Info("   📡 gRPC服务器: " + fmt.Sprintf("%s:%d", b.config.Gateway.GRPC.Host, b.config.Gateway.GRPC.Port))

	if b.config.Gateway.HealthCheck.Enabled {
		global.LOGGER.Info("   ❤️  健康检查: " + b.config.Gateway.HealthCheck.Path)
	}
}

// printFeatures 打印功能特性
func (b *BannerManager) printFeatures() {
	global.LOGGER.Info("🔧 企业级功能:")

	// 基础功能
	baseFeatures := []string{
		"gRPC-Gateway集成",
		"中间件生态系统",
		"配置热重载",
		"优雅关闭",
		"I18n国际化支持",
	}

	for _, feature := range baseFeatures {
		global.LOGGER.Info("   ✅ " + feature)
	}

	// 中间件功能
	b.printMiddlewareFeatures()

	// 监控功能
	b.printMonitoringFeatures()

	// 自定义功能
	for _, feature := range b.features {
		global.LOGGER.Info("   ✅ " + feature)
	}
}

// printMiddlewareFeatures 打印中间件功能
func (b *BannerManager) printMiddlewareFeatures() {
	// 使用go-config的CORS配置
	if b.config.SingleConfig.Cors.AllowedAllOrigins || len(b.config.SingleConfig.Cors.AllowedOrigins) > 0 {
		global.LOGGER.Info("   ✅ CORS跨域支持")
	}

	if b.config.Middleware.RateLimit.Enabled {
		global.LOGGER.Info("   ✅ 限流控制 (" + b.config.Middleware.RateLimit.Algorithm + "算法)")
	}

	if b.config.Middleware.AccessLog.Enabled {
		global.LOGGER.Info("   ✅ 访问日志记录")
	}

	// 使用go-config的JWT配置来判断认证功能
	if b.config.SingleConfig.JWT.SigningKey != "" {
		global.LOGGER.Info("   ✅ 身份认证 (JWT)")
	}

	if b.config.Middleware.Signature.Enabled {
		global.LOGGER.Info("   ✅ 请求签名验证")
	}
}

// printMonitoringFeatures 打印监控功能
func (b *BannerManager) printMonitoringFeatures() {
	if b.config.Monitoring.Metrics.Enabled {
		global.LOGGER.Info("   ✅ Prometheus指标 (" + b.config.Monitoring.Metrics.Path + ")")
	}

	if b.config.Monitoring.Tracing.Enabled {
		global.LOGGER.Info("   ✅ 链路追踪 (" + b.config.Monitoring.Tracing.Resource.ServiceName + ")")
	}
}

// printEndpoints 打印端点信息
func (b *BannerManager) printEndpoints() {
	baseURL := fmt.Sprintf("http://%s:%d", b.config.Gateway.HTTP.Host, b.config.Gateway.HTTP.Port)
	if b.config.Gateway.HTTP.Host == "0.0.0.0" {
		baseURL = fmt.Sprintf("http://localhost:%d", b.config.Gateway.HTTP.Port)
	}

	global.LOGGER.Info("📡 核心端点:")

	if b.config.Gateway.HealthCheck.Enabled {
		global.LOGGER.Info("   🏥 健康检查: " + baseURL + b.config.Gateway.HealthCheck.Path)
	}

	if b.config.Monitoring.Metrics.Enabled {
		global.LOGGER.Info("   📊 监控指标: " + baseURL + b.config.Monitoring.Metrics.Path)
	}
}

// PrintPProfInfo 打印PProf信息
func (b *BannerManager) PrintPProfInfo(pprofConfig *middleware.PProfGatewayConfig) {
	if !pprofConfig.IsPProfEnabled() {
		return
	}

	config := pprofConfig.GetPProfConfig()
	baseURL := fmt.Sprintf("http://localhost:%d", b.config.Gateway.HTTP.Port)

	global.LOGGER.Info("🔬 性能分析 (PProf):")
	global.LOGGER.Info("   🎯 状态: 已启用")
	global.LOGGER.Info("   🔑 认证: " + fmt.Sprintf("%t", config.RequireAuth))

	if config.RequireAuth {
		global.LOGGER.Info("   🎟️  Token: " + config.AuthToken)
	}

	global.LOGGER.Info("   🏠 仪表板: " + baseURL + "/")
	global.LOGGER.Info("   🔍 PProf索引: " + baseURL + config.PathPrefix + "/")

	if config.RequireAuth {
		global.LOGGER.Info("   💡 认证URL: " + baseURL + config.PathPrefix + "/?token=" + config.AuthToken)
	}

	global.LOGGER.Info("   🧪 性能测试场景:")
	scenarios := []struct {
		path string
		desc string
	}{
		{"/gc/small-objects", "小对象GC测试"},
		{"/gc/large-objects", "大对象GC测试"},
		{"/memory/allocate", "内存分配测试"},
		{"/cpu/intensive", "CPU密集测试"},
		{"/goroutine/spawn", "协程创建测试"},
	}

	for _, scenario := range scenarios {
		tokenParam := ""
		if config.RequireAuth {
			tokenParam = "?token=" + config.AuthToken
		}
		global.LOGGER.Info("     • " + scenario.desc + ": " + baseURL + config.PathPrefix + scenario.path + tokenParam)
	}
}

// printSystemInfo 打印系统信息
func (b *BannerManager) printSystemInfo() {
	global.LOGGER.Info("💻 系统信息:")
	global.LOGGER.Info("   🐹 Go版本: " + runtime.Version())
	global.LOGGER.Info("   🔧 CPU核心: " + fmt.Sprintf("%d", runtime.NumCPU()))
	global.LOGGER.Info("   🧵 Goroutines: " + fmt.Sprintf("%d", runtime.NumGoroutine()))
	global.LOGGER.Info("   💾 系统: " + runtime.GOOS + "/" + runtime.GOARCH)
	global.LOGGER.Info("   ⏰ 启动时间: " + time.Now().Format("2006-01-02 15:04:05"))
}

// PrintMiddlewareStatus 打印中间件状态
func (b *BannerManager) PrintMiddlewareStatus() {
	global.LOGGER.Info("🔌 中间件状态:")

	middlewares := []struct {
		name    string
		enabled bool
		desc    string
	}{
		{"Recovery", true, "异常恢复"},
		{"RequestID", true, "请求ID生成"},
		{"I18n", true, "国际化支持"},
		{"CORS", b.config.SingleConfig.Cors.AllowedAllOrigins || len(b.config.SingleConfig.Cors.AllowedOrigins) > 0, "跨域处理"},
		{"RateLimit", b.config.Middleware.RateLimit.Enabled, "限流控制"},
		{"AccessLog", b.config.Middleware.AccessLog.Enabled, "访问日志"},
		{"Auth", b.config.SingleConfig.JWT.SigningKey != "", "身份认证"},
		{"Signature", b.config.Middleware.Signature.Enabled, "签名验证"},
		{"Security", true, "安全头设置"},
	}

	for _, mw := range middlewares {
		status := "❌ 禁用"
		if mw.enabled {
			status = "✅ 启用"
		}
		global.LOGGER.Info("   " + status + " - " + mw.desc + " (" + mw.name + ")")
	}
}

// PrintUsageGuide 打印使用指南
func (b *BannerManager) PrintUsageGuide() {
	baseURL := fmt.Sprintf("http://localhost:%d", b.config.Gateway.HTTP.Port)

	global.LOGGER.Info("💡 使用指南:")
	global.LOGGER.Info("   📖 访问主页查看完整信息: " + baseURL + "/")

	if b.config.Gateway.HealthCheck.Enabled {
		global.LOGGER.Info("   🏥 健康检查: curl " + baseURL + b.config.Gateway.HealthCheck.Path)
	}

	if b.config.Monitoring.Metrics.Enabled {
		global.LOGGER.Info("   📊 监控指标: curl " + baseURL + b.config.Monitoring.Metrics.Path)
	}

	global.LOGGER.Info("   ⏹️  优雅关闭: 按 Ctrl+C")
}
