/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 11:40:03
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

	goconfig "github.com/kamalyes/go-config"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

// BannerManager 横幅管理器
type BannerManager struct {
	config   *gwconfig.Gateway
	features []string
}

// NewBannerManager 创建横幅管理器
func NewBannerManager(config *gwconfig.Gateway) *BannerManager {
	return &BannerManager{
		config:   config,
		features: []string{},
	}
}

// getBaseURL 获取基础 URL，处理 0.0.0.0 的情况
func (b *BannerManager) getBaseURL() string {
	configSafe := goconfig.SafeConfig(b.config)
	host := configSafe.Field("HTTPServer").Field("Host").String("localhost")
	if host == "0.0.0.0" || host == "" {
		host = "localhost"
	}
	port := configSafe.Field("HTTPServer").Field("Port").Int(8080)
	return fmt.Sprintf("http://%s:%d", host, port)
}

// AddFeature 添加功能特性
func (b *BannerManager) AddFeature(feature string) {
	b.features = append(b.features, feature)
}

// PrintStartupBanner 打印启动横幅
func (b *BannerManager) PrintStartupBanner() {
	configSafe := goconfig.SafeConfig(b.config)
	// 使用go-config中的Banner模板
	template := configSafe.Field("Banner").Field("Template").String("")
	if template != "" {
		global.LOGGER.Info(template)
	} else {
		// 如果模板为空，打印默认的艺术字
		b.printDefaultAsciiArt()
	}
	title := configSafe.Field("Banner").Field("Title").String("Gateway")
	global.LOGGER.Info("🚀 " + title + " - Enterprise Edition")
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
	configSafe := goconfig.SafeConfig(b.config)
	global.LOGGER.Info("📋 基础信息:")
	title := configSafe.Field("Banner").Field("Title").String("Gateway")
	global.LOGGER.Info("   🏷️  名称: " + title)
	global.LOGGER.Info("   📦 版本: v1.0.0")
	environment := configSafe.Field("Environment").String("development")
	global.LOGGER.Info("   🌍 环境: " + environment)
	debug := configSafe.Field("Debug").Bool(false)
	global.LOGGER.Info("   🔧 调试模式: " + fmt.Sprintf("%v", debug))
	global.LOGGER.Info("   🏗️  框架: go-rpc-gateway (基于 go-config & go-logger & go-sqlbuilder & go-toolbox)")
}

// printServerConfig 打印服务器配置
func (b *BannerManager) printServerConfig() {
	configSafe := goconfig.SafeConfig(b.config)
	global.LOGGER.Info("⚙️  服务器配置:")
	endpoint := configSafe.Field("HTTPServer").Field("Endpoint").String("http://localhost:8080")
	global.LOGGER.Info("   🌐 HTTP服务器: " + endpoint)
	host := configSafe.Field("HTTPServer").Field("Host").String("localhost")
	grpcPort := configSafe.Field("HTTPServer").Field("GrpcPort").Int(9090)
	global.LOGGER.Info("   📡 gRPC服务器: " + fmt.Sprintf("%s:%d", host, grpcPort))

	if configSafe.IsHealthEnabled() {
		healthPath := configSafe.GetHealthPath("/health")
		global.LOGGER.Info("   ❤️  健康检查: " + healthPath)
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
		"请求ID生成",
		"异常恢复",
		"安全头设置",
		"日志记录与管理",
		"Swagger文档支持",
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
	configSafe := goconfig.SafeConfig(b.config)
	// 使用go-config的CORS配置
	allowedAllOrigins := configSafe.Field("CORS").Field("AllowedAllOrigins").Bool(false)
	allowedOrigins := configSafe.Field("CORS").Field("AllowedOrigins").String("")
	if allowedAllOrigins || allowedOrigins != "" {
		global.LOGGER.Info("   ✅ CORS跨域支持")
	}

	if configSafe.Field("RateLimit").Field("Enabled").Bool(false) {
		global.LOGGER.Info("   ✅ 限流控制")
	}

	if configSafe.Field("Middleware").Field("Logging").Field("Enabled").Bool(false) {
		global.LOGGER.Info("   ✅ 访问日志记录")
	}

	// 使用go-config的JWT配置来判断认证功能
	signingKey := configSafe.Field("JWT").Field("SigningKey").String("")
	if signingKey != "" {
		global.LOGGER.Info("   ✅ 身份认证 (JWT)")
	}
}

// printMonitoringFeatures 打印监控功能
func (b *BannerManager) printMonitoringFeatures() {
	configSafe := goconfig.SafeConfig(b.config)
	if configSafe.IsMetricsEnabled() {
		prometheusPath := configSafe.Field("Monitoring").Field("Prometheus").Field("Path").String("/metrics")
		global.LOGGER.Info("   ✅ Prometheus指标 (" + prometheusPath + ")")
	}

	if configSafe.IsJaegerEnabled() {
		serviceName := configSafe.GetJaegerServiceName("gateway-service")
		global.LOGGER.Info("   ✅ 链路追踪 (" + serviceName + ")")
	}
}

// printEndpoints 打印端点信息
func (b *BannerManager) printEndpoints() {
	baseURL := b.getBaseURL()
	configSafe := goconfig.SafeConfig(b.config)

	global.LOGGER.Info("📡 核心端点:")

	if configSafe.IsHealthEnabled() {
		healthPath := configSafe.GetHealthPath("/health")
		global.LOGGER.Info("   🏥 健康检查: " + baseURL + healthPath)
	}

	if configSafe.Field("Monitoring").Field("Prometheus").Field("Enabled").Bool(false) {
		prometheusPath := configSafe.Field("Monitoring").Field("Prometheus").Field("Path").String("/metrics")
		global.LOGGER.Info("   📊 监控指标: " + baseURL + prometheusPath)
	}
}

// PrintPProfInfo 打印PProf信息
// go-config 的 Default() 已经设置了所有默认值，无需再次设置
func (b *BannerManager) PrintPProfInfo(pprofConfig *middleware.PProfGatewayConfig) {
	configSafe := goconfig.SafeConfig(b.config)
	if !configSafe.IsPProfEnabled() {
		return
	}

	baseURL := b.getBaseURL()

	global.LOGGER.Info("🔬 性能分析 (PProf):")
	global.LOGGER.Info("   🎯 状态: 已启用")
	global.LOGGER.Info("   🏠 仪表板: " + baseURL + "/")
	pprofPrefix := configSafe.GetPProfPathPrefix("/debug/pprof")
	global.LOGGER.Info("   🔍 PProf索引: " + baseURL + pprofPrefix + "/")

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
		pprofPrefix := configSafe.GetPProfPathPrefix("/debug/pprof")
		global.LOGGER.Info("     • " + scenario.desc + ": " + baseURL + pprofPrefix + scenario.path)
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
	configSafe := goconfig.SafeConfig(b.config)
	global.LOGGER.Info("🔌 中间件状态:")

	middlewares := []struct {
		name    string
		enabled bool
		desc    string
	}{
		{"Swagger", configSafe.Field("Swagger").Field("Enabled").Bool(false), "Swagger文档"},
		{"Recovery", configSafe.Field("Middleware").Field("Recovery").Field("Enabled").Bool(false), "异常恢复"},
		{"RequestID", configSafe.Field("Middleware").Field("RequestID").Field("Enabled").Bool(false), "请求ID生成"},
		{"I18n", configSafe.Field("Middleware").Field("I18N").Field("Enabled").Bool(false), "国际化支持"},
		{"CORS", configSafe.Field("CORS").Field("AllowedAllOrigins").Bool(false) || configSafe.Field("CORS").Field("AllowedOrigins").String("") != "", "跨域处理"},
		{"RateLimit", configSafe.Field("RateLimit").Field("Enabled").Bool(false), "限流控制"},
		{"AccessLog", configSafe.Field("Middleware").Field("Logging").Field("Enabled").Bool(false), "访问日志"},
		{"Auth", configSafe.Field("JWT").Field("SigningKey").String("") != "", "身份认证"},
		{"Security", configSafe.Field("Security").Field("Enabled").Bool(false), "安全头设置"},
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
	baseURL := b.getBaseURL()
	configSafe := goconfig.SafeConfig(b.config)

	global.LOGGER.Info("💡 使用指南:")
	global.LOGGER.Info("   📖 访问主页查看完整信息: " + baseURL + "/")

	if configSafe.IsHealthEnabled() {
		healthPath := configSafe.GetHealthPath("/health")
		global.LOGGER.Info("   🏥 健康检查: curl " + baseURL + healthPath)
	}

	if configSafe.Field("Monitoring").Field("Prometheus").Field("Enabled").Bool(false) {
		prometheusPath := configSafe.Field("Monitoring").Field("Prometheus").Field("Path").String("/metrics")
		global.LOGGER.Info("   📊 监控指标: curl " + baseURL + prometheusPath)
	}

	global.LOGGER.Info("   ⏹️  优雅关闭: 按 Ctrl+C")
}

// printDefaultAsciiArt 打印默认的艺术字横幅
func (b *BannerManager) printDefaultAsciiArt() {
	art := `
 ██████╗  ██████╗       ██████╗ ██████╗  ██████╗       ██████╗  █████╗ ████████╗███████╗██╗    ██╗ █████╗ ██╗   ██╗
██╔════╝ ██╔═══██╗      ██╔══██╗██╔══██╗██╔════╝      ██╔════╝ ██╔══██╗╚══██╔══╝██╔════╝██║    ██║██╔══██╗╚██╗ ██╔╝
██║  ███╗██║   ██║█████╗██████╔╝██████╔╝██║     █████╗██║  ███╗███████║   ██║   █████╗  ██║ █╗ ██║███████║ ╚████╔╝ 
██║   ██║██║   ██║╚════╝██╔══██╗██╔═══╝ ██║     ╚════╝██║   ██║██╔══██║   ██║   ██╔══╝  ██║███╗██║██╔══██║  ╚██╔╝  
╚██████╔╝╚██████╔╝      ██║  ██║██║     ╚██████╗      ╚██████╔╝██║  ██║   ██║   ███████╗╚███╔███╔╝██║  ██║   ██║   
 ╚═════╝  ╚═════╝       ╚═╝  ╚═╝╚═╝      ╚═════╝       ╚═════╝ ╚═╝  ╚═╝   ╚═╝   ╚══════╝ ╚══╝╚══╝ ╚═╝  ╚═╝   ╚═╝   
                                                                                                                       
  ██████████████████████████████████████████████████████████████████████████████████████████████████████████████    
                                                                                                                       
                        🚀 高性能微服务网关 | Enterprise Edition v2.0                                                  
                        ⚡ 基于 gRPC-Gateway + OpenTelemetry + Prometheus                                             
                        🛡️  生产就绪 | 云原生架构 | 企业级功能                                                          
`
	global.LOGGER.Info(art)
}
