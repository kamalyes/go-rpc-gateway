/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-27 22:52:49
 * @FilePath: \go-rpc-gateway\server\banner.go
 * @Description: Gateway启动横幅和信息展示
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"context"
	"fmt"
	goconfig "github.com/kamalyes/go-config"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"runtime"
	"time"
)

// BannerManager 横幅管理器
type BannerManager struct {
	ctx      context.Context
	config   *gwconfig.Gateway
	features []string
}

// NewBannerManager 创建横幅管理器
func NewBannerManager(config *gwconfig.Gateway) *BannerManager {
	return &BannerManager{
		ctx:      context.Background(),
		config:   config,
		features: []string{},
	}
}

func (b *BannerManager) WithContext(ctx context.Context) *BannerManager {
	b.ctx = ctx
	return b
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
	// 检查 logger 是否初始化
	if global.LOGGER == nil {
		fmt.Println("⚠️  警告: LOGGER 未初始化，无法打印启动横幅")
		return
	}

	// 检查 banner 是否启用
	configSafe := goconfig.SafeConfig(b.config)
	if !configSafe.Field("Banner").Field("Enabled").Bool(true) {
		return // Banner 被禁用，不打印
	}

	configSafe = goconfig.SafeConfig(b.config)
	// 使用go-config中的Banner模板
	template := configSafe.Field("Banner").Field("Template").String("")
	if template != "" {
		global.LOGGER.InfoContext(b.ctx, template)
	} else {
		// 如果模板为空，打印默认的艺术字
		b.printDefaultAsciiArt()
	}
	title := configSafe.Field("Banner").Field("Title").String("Gateway")
	global.LOGGER.InfoContext(b.ctx, "🚀 "+title+" - Enterprise Edition")
	global.LOGGER.InfoContext(b.ctx, "")

	// 基础信息
	b.printBasicInfo()
	global.LOGGER.InfoContext(b.ctx, "")

	// 服务器配置
	b.printServerConfig()
	global.LOGGER.InfoContext(b.ctx, "")

	// 功能特性
	b.printFeatures()
	global.LOGGER.InfoContext(b.ctx, "")

	// 端点信息
	b.printEndpoints()
	global.LOGGER.InfoContext(b.ctx, "")

	// 系统信息
	b.printSystemInfo()
	global.LOGGER.InfoContext(b.ctx, "")

	global.LOGGER.InfoContext(b.ctx, "🎉 ================================================")
	global.LOGGER.InfoContext(b.ctx, "")
}

// PrintShutdownBanner 打印关闭横幅
func (b *BannerManager) PrintShutdownBanner() {
	global.LOGGER.InfoContext(b.ctx, "🛑 ================================================")
	global.LOGGER.InfoContext(b.ctx, "⏹️  Gateway正在优雅关闭...")
	global.LOGGER.InfoContext(b.ctx, "🛑 ================================================")
}

// PrintShutdownComplete 打印关闭完成
func (b *BannerManager) PrintShutdownComplete() {
	global.LOGGER.InfoContext(b.ctx, "✅ Gateway已安全关闭")
	global.LOGGER.InfoContext(b.ctx, "👋 感谢使用 Go RPC Gateway！")
}

// printBasicInfo 打印基础信息
func (b *BannerManager) printBasicInfo() {
	configSafe := goconfig.SafeConfig(b.config)
	global.LOGGER.InfoContext(b.ctx, "📋 基础信息:")
	title := configSafe.Field("Banner").Field("Title").String("Gateway")
	global.LOGGER.InfoContext(b.ctx, "   🏷️  名称: "+title)
	global.LOGGER.InfoContext(b.ctx, "   📦 版本: v1.0.0")
	environment := configSafe.Field("Environment").String("development")
	global.LOGGER.InfoContext(b.ctx, "   🌍 环境: "+environment)
	debug := configSafe.Field("Debug").Bool(false)
	global.LOGGER.InfoContext(b.ctx, "   🔧 调试模式: "+fmt.Sprintf("%v", debug))
	global.LOGGER.InfoContext(b.ctx, "   🏗️  框架: go-rpc-gateway (基于 go-config & go-logger & go-sqlbuilder & go-toolbox)")
}

// printServerConfig 打印服务器配置
func (b *BannerManager) printServerConfig() {
	configSafe := goconfig.SafeConfig(b.config)
	global.LOGGER.InfoContext(b.ctx, "⚙️  服务器配置:")
	endpoint := configSafe.Field("HTTPServer").Field("Endpoint").String("http://localhost:8080")
	global.LOGGER.InfoContext(b.ctx, "   🌐 HTTP服务器: "+endpoint)
	host := configSafe.Field("HTTPServer").Field("Host").String("localhost")
	grpcPort := configSafe.Field("HTTPServer").Field("GrpcPort").Int(9090)
	global.LOGGER.InfoContext(b.ctx, "   📡 gRPC服务器: "+fmt.Sprintf("%s:%d", host, grpcPort))

	if configSafe.IsHealthEnabled() {
		healthPath := configSafe.GetHealthPath("/health")
		global.LOGGER.InfoContext(b.ctx, "   ❤️  健康检查: "+healthPath)
	}
}

// printFeatures 打印功能特性
func (b *BannerManager) printFeatures() {
	global.LOGGER.InfoContext(b.ctx, "🔧 企业级功能:")

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
		global.LOGGER.InfoContext(b.ctx, "   ✅ "+feature)
	}

	// 中间件功能
	b.printMiddlewareFeatures()

	// 监控功能
	b.printMonitoringFeatures()

	// 自定义功能
	for _, feature := range b.features {
		global.LOGGER.InfoContext(b.ctx, "   ✅ "+feature)
	}
}

// printMiddlewareFeatures 打印中间件功能
func (b *BannerManager) printMiddlewareFeatures() {
	configSafe := goconfig.SafeConfig(b.config)
	// 使用go-config的CORS配置
	allowedAllOrigins := configSafe.Field("CORS").Field("AllowedAllOrigins").Bool(false)
	allowedOrigins := configSafe.Field("CORS").Field("AllowedOrigins").String("")
	if allowedAllOrigins || allowedOrigins != "" {
		global.LOGGER.InfoContext(b.ctx, "   ✅ CORS跨域支持")
	}

	if configSafe.Field("RateLimit").Field("Enabled").Bool(false) {
		global.LOGGER.InfoContext(b.ctx, "   ✅ 限流控制")
	}

	if configSafe.Field("Middleware").Field("Logging").Field("Enabled").Bool(false) {
		global.LOGGER.InfoContext(b.ctx, "   ✅ 访问日志记录")
	}

	// 使用go-config的JWT配置来判断认证功能
	signingKey := configSafe.Field("JWT").Field("SigningKey").String("")
	if signingKey != "" {
		global.LOGGER.InfoContext(b.ctx, "   ✅ 身份认证 (JWT)")
	}
}

// printMonitoringFeatures 打印监控功能
func (b *BannerManager) printMonitoringFeatures() {
	configSafe := goconfig.SafeConfig(b.config)

	// Prometheus Metrics 功能
	if configSafe.IsMetricsEnabled() {
		metricsHost := configSafe.Field("metrics").Field("host").String("0.0.0.0")
		metricsPort := configSafe.Field("metrics").Field("port").Int(9090)
		prometheusPath := configSafe.Field("Monitoring").Field("Prometheus").Field("Path").String("/metrics")

		displayHost := metricsHost
		if metricsHost == "0.0.0.0" {
			displayHost = "localhost"
		}
		global.LOGGER.InfoContext(b.ctx, fmt.Sprintf("   ✅ Prometheus指标 (http://%s:%d%s)",
			displayHost, metricsPort, prometheusPath))

		// 显示自定义指标配置状态
		httpMetrics := configSafe.Field("metrics").Field("custom_metrics").Field("http_requests_total").Field("enabled").Bool(false)
		grpcMetrics := configSafe.Field("metrics").Field("custom_metrics").Field("grpc_requests_total").Field("enabled").Bool(false)
		redisMetrics := configSafe.Field("metrics").Field("custom_metrics").Field("redis_operations_total").Field("enabled").Bool(false)
		if httpMetrics || grpcMetrics || redisMetrics {
			global.LOGGER.InfoContext(b.ctx, fmt.Sprintf("     📈 自定义指标: HTTP:%v, gRPC:%v, Redis:%v", httpMetrics, grpcMetrics, redisMetrics))
		}
	}

	// PProf 性能分析功能
	if configSafe.IsPProfEnabled() {
		pprofHost := configSafe.Field("pprof").Field("host").String("0.0.0.0")
		pprofPort := configSafe.Field("pprof").Field("port").Int(6060)
		pprofPath := configSafe.GetPProfPathPrefix("/debug/pprof")

		displayHost := pprofHost
		if pprofHost == "0.0.0.0" {
			displayHost = "localhost"
		}
		global.LOGGER.InfoContext(b.ctx, fmt.Sprintf("   ✅ PProf性能分析 (http://%s:%d%s/)",
			displayHost, pprofPort, pprofPath))

		// 显示认证状态
		pprofAuth := configSafe.Field("pprof").Field("auth").Field("enabled").Bool(false)
		authStatus := "已禁用 (开发模式)"
		if pprofAuth {
			authStatus = "已启用"
		}
		global.LOGGER.InfoContext(b.ctx, "     🔐 认证状态: "+authStatus)
	}

	if configSafe.IsJaegerEnabled() {
		serviceName := configSafe.GetJaegerServiceName("gateway-service")
		global.LOGGER.InfoContext(b.ctx, "   ✅ 链路追踪 ("+serviceName+")")
	}
}

// printEndpoints 打印端点信息
func (b *BannerManager) printEndpoints() {
	baseURL := b.getBaseURL()
	configSafe := goconfig.SafeConfig(b.config)

	global.LOGGER.InfoContext(b.ctx, "📡 核心端点:")

	// 健康检查端点
	if configSafe.IsHealthEnabled() {
		healthPath := configSafe.GetHealthPath("/health")
		global.LOGGER.InfoContext(b.ctx, "   🏥 健康检查: "+baseURL+healthPath)
	}

	// Swagger 文档端点
	if configSafe.Field("Swagger").Field("Enabled").Bool(false) {
		swaggerPath := configSafe.Field("Swagger").Field("UIPath").String("/swagger")
		global.LOGGER.InfoContext(b.ctx, "   📚 API文档: "+baseURL+swaggerPath)
	}

	// Prometheus 指标端点
	if configSafe.IsMetricsEnabled() {
		metricsHost := configSafe.Field("metrics").Field("host").String("0.0.0.0")
		metricsPort := configSafe.Field("metrics").Field("port").Int(9090)
		prometheusPath := configSafe.Field("metrics").Field("path").String("/metrics")

		displayHost := metricsHost
		if metricsHost == "0.0.0.0" {
			displayHost = "localhost"
		}
		metricsURL := fmt.Sprintf("http://%s:%d%s", displayHost, metricsPort, prometheusPath)
		global.LOGGER.InfoContext(b.ctx, "   📊 监控指标: "+metricsURL)
	}

	// PProf 性能分析端点
	if configSafe.IsPProfEnabled() {
		pprofHost := configSafe.Field("pprof").Field("host").String("0.0.0.0")
		pprofPort := configSafe.Field("pprof").Field("port").Int(6060)
		pprofPath := configSafe.GetPProfPathPrefix("/debug/pprof")

		displayHost := pprofHost
		if pprofHost == "0.0.0.0" {
			displayHost = "localhost"
		}
		pprofURL := fmt.Sprintf("http://%s:%d%s/", displayHost, pprofPort, pprofPath)
		global.LOGGER.InfoContext(b.ctx, "   🔬 性能分析: "+pprofURL)
	}
}

// PrintPProfInfo 打印PProf信息
// go-config 的 Default() 已经设置了所有默认值，无需再次设置
func (b *BannerManager) PrintPProfInfo(ctx context.Context, pprofConfig *middleware.PProfGatewayConfig) {
	configSafe := goconfig.SafeConfig(b.config)
	if !configSafe.IsPProfEnabled() {
		return
	}

	baseURL := b.getBaseURL()

	global.LOGGER.InfoContext(b.ctx, "🔬 性能分析 (PProf):")
	global.LOGGER.InfoContext(b.ctx, "   🎯 状态: 已启用")
	global.LOGGER.InfoContext(b.ctx, "   🏠 仪表板: "+baseURL+"/")
	pprofPrefix := configSafe.GetPProfPathPrefix("/debug/pprof")
	global.LOGGER.InfoContext(b.ctx, "   🔍 PProf索引: "+baseURL+pprofPrefix+"/")

	global.LOGGER.InfoContext(b.ctx, "   🧪 性能测试场景:")
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
		global.LOGGER.InfoContext(b.ctx, "     • "+scenario.desc+": "+baseURL+pprofPrefix+scenario.path)
	}
}

// printSystemInfo 打印系统信息
func (b *BannerManager) printSystemInfo() {
	global.LOGGER.InfoContext(b.ctx, "💻 系统信息:")
	global.LOGGER.InfoContext(b.ctx, "   🐹 Go版本: "+runtime.Version())
	global.LOGGER.InfoContext(b.ctx, "   🔧 CPU核心: "+fmt.Sprintf("%d", runtime.NumCPU()))
	global.LOGGER.InfoContext(b.ctx, "   🧵 Goroutines: "+fmt.Sprintf("%d", runtime.NumGoroutine()))
	global.LOGGER.InfoContext(b.ctx, "   💾 系统: "+runtime.GOOS+"/"+runtime.GOARCH)
	global.LOGGER.InfoContext(b.ctx, "   ⏰ 启动时间: "+time.Now().Format("2006-01-02 15:04:05"))
}

// PrintMiddlewareStatus 打印中间件状态
func (b *BannerManager) PrintMiddlewareStatus() {
	configSafe := goconfig.SafeConfig(b.config)
	global.LOGGER.InfoContext(b.ctx, "🔌 中间件状态:")

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
		global.LOGGER.InfoContext(b.ctx, "   "+status+" - "+mw.desc+" ("+mw.name+")")
	}
}

// PrintUsageGuide 打印使用指南
func (b *BannerManager) PrintUsageGuide() {
	baseURL := b.getBaseURL()
	configSafe := goconfig.SafeConfig(b.config)

	global.LOGGER.InfoContext(b.ctx, "💡 使用指南:")
	global.LOGGER.InfoContext(b.ctx, "   📖 访问主页查看完整信息: "+baseURL+"/")

	if configSafe.IsHealthEnabled() {
		healthPath := configSafe.GetHealthPath("/health")
		global.LOGGER.InfoContext(b.ctx, "   🏥 健康检查: curl "+baseURL+healthPath)
	}

	if configSafe.Field("Monitoring").Field("Prometheus").Field("Enabled").Bool(false) {
		prometheusPath := configSafe.Field("Monitoring").Field("Prometheus").Field("Path").String("/metrics")
		global.LOGGER.InfoContext(b.ctx, "   📊 监控指标: curl "+baseURL+prometheusPath)
	}

	global.LOGGER.InfoContext(b.ctx, "   ⏹️  优雅关闭: 按 Ctrl+C")
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
	global.LOGGER.InfoContext(b.ctx, art)
}
