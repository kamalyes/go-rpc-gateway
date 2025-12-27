/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 16:09:15
 * @FilePath: \go-rpc-gateway\server\banner.go
 * @Description: Gateway启动横幅和信息展示
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/kamalyes/go-config/pkg/banner"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-rpc-gateway/global"
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

// getBaseURL 获取基础 URL
func (b *BannerManager) getBaseURL() string {
	return fmt.Sprintf("http://%s:%d", b.config.HTTPServer.Host, b.config.HTTPServer.Port)
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
	if !b.config.Banner.Enabled {
		return
	}

	// 使用go-config中的Banner模板
	if b.config.Banner.Template != "" {
		global.LOGGER.InfoContext(b.ctx, b.config.Banner.Template)
	} else {
		global.LOGGER.InfoContext(b.ctx, banner.Default().Template)
	}
	title := b.config.Banner.Title
	if title == "" {
		title = "Gateway"
	}
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
	global.LOGGER.InfoContext(b.ctx, "📋 基础信息:")
	global.LOGGER.InfoContext(b.ctx, "   🏷️  名称: "+b.config.Banner.Title)
	global.LOGGER.InfoContext(b.ctx, "   📦 版本: "+b.config.Version)
	global.LOGGER.InfoContext(b.ctx, "   🌍 环境: "+b.config.Environment)
	global.LOGGER.InfoContext(b.ctx, "   � 调试模式: "+fmt.Sprintf("%v", b.config.Debug))
	global.LOGGER.InfoContext(b.ctx, "")

	// 构建信息
	global.LOGGER.InfoContext(b.ctx, "🔨 构建信息:")
	global.LOGGER.InfoContext(b.ctx, "   🕒 构建时间: "+b.config.BuildTime)
	global.LOGGER.InfoContext(b.ctx, "   👤 构建用户: "+b.config.BuildUser)
	global.LOGGER.InfoContext(b.ctx, "   🐹 Go版本: "+b.config.GoVersion)
	global.LOGGER.InfoContext(b.ctx, "")

	// Git信息
	global.LOGGER.InfoContext(b.ctx, "🔖 Git信息:")
	global.LOGGER.InfoContext(b.ctx, "   📝 Commit: "+b.config.GitCommit)
	global.LOGGER.InfoContext(b.ctx, "   🌿 Branch: "+b.config.GitBranch)
	global.LOGGER.InfoContext(b.ctx, "   🏷️  Tag: "+b.config.GitTag)
	global.LOGGER.InfoContext(b.ctx, "")

	global.LOGGER.InfoContext(b.ctx, "   🏗️  框架: go-rpc-gateway (基于 go-config & go-logger & go-sqlbuilder & go-toolbox)")
}

// printServerConfig 打印服务器配置
func (b *BannerManager) printServerConfig() {
	global.LOGGER.InfoContext(b.ctx, "⚙️  服务器配置:")
	baseURL := b.getBaseURL()
	global.LOGGER.InfoContext(b.ctx, "   🌐 HTTP服务器: "+baseURL)

	host := b.config.HTTPServer.Host
	global.LOGGER.InfoContext(b.ctx, "   📡 gRPC服务器: "+fmt.Sprintf("%s:%d", host, b.config.GRPC.Server.Port))

	if b.config.Health.Enabled {
		global.LOGGER.InfoContext(b.ctx, "   ❤️  健康检查: "+b.config.Health.Path)
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
	if b.config.CORS.AllowedAllOrigins || len(b.config.CORS.AllowedOrigins) > 0 {
		global.LOGGER.InfoContext(b.ctx, "   ✅ CORS跨域支持")
	}

	if b.config.RateLimit.Enabled {
		global.LOGGER.InfoContext(b.ctx, "   ✅ 限流控制")
	}

	if b.config.Middleware.Logging.Enabled {
		global.LOGGER.InfoContext(b.ctx, "   ✅ 访问日志记录")
	}

	if b.config.Security.JWT.Secret != "" {
		global.LOGGER.InfoContext(b.ctx, "   ✅ 身份认证 (JWT)")
	}
}

// printMonitoringFeatures 打印监控功能
func (b *BannerManager) printMonitoringFeatures() {
	if b.config.Monitoring.Prometheus.Enabled {
		global.LOGGER.InfoContext(b.ctx, fmt.Sprintf("   ✅ Prometheus指标 (http://localhost:%d%s)",
			b.config.Monitoring.Prometheus.Port, b.config.Monitoring.Prometheus.Path))
	}

	if b.config.Middleware.PProf.Enabled {
		global.LOGGER.InfoContext(b.ctx, fmt.Sprintf("   ✅ PProf性能分析 (http://localhost:%d%s/)",
			b.config.Middleware.PProf.Port, b.config.Middleware.PProf.PathPrefix))

		authStatus := "已禁用 (开发模式)"
		if b.config.Middleware.PProf.Authentication.Enabled {
			authStatus = "已启用"
		}
		global.LOGGER.InfoContext(b.ctx, "     🔐 认证状态: "+authStatus)
	}

	if b.config.Monitoring.Jaeger.Enabled {
		global.LOGGER.InfoContext(b.ctx, "   ✅ 链路追踪 ("+b.config.Monitoring.Jaeger.ServiceName+")")
	}
}

// printEndpoints 打印端点信息
func (b *BannerManager) printEndpoints() {
	baseURL := b.getBaseURL()

	global.LOGGER.InfoContext(b.ctx, "📡 核心端点:")

	if b.config.Health.Enabled {
		global.LOGGER.InfoContext(b.ctx, "   🏥 健康检查: "+baseURL+b.config.Health.Path)
	}

	if b.config.Swagger.Enabled {
		global.LOGGER.InfoContext(b.ctx, "   📚 API文档: "+baseURL+b.config.Swagger.UIPath)
	}

	if b.config.Monitoring.Prometheus.Enabled {
		metricsURL := fmt.Sprintf("http://localhost:%d%s", b.config.Monitoring.Prometheus.Port, b.config.Monitoring.Prometheus.Path)
		global.LOGGER.InfoContext(b.ctx, "   📊 监控指标: "+metricsURL)
	}

	if b.config.Middleware.PProf.Enabled {
		pprofURL := fmt.Sprintf("http://localhost:%d%s/", b.config.Middleware.PProf.Port, b.config.Middleware.PProf.PathPrefix)
		global.LOGGER.InfoContext(b.ctx, "   🔬 性能分析: "+pprofURL)
	}
}

// PrintPProfInfo 打印PProf信息
// go-config 的 Default() 已经设置了所有默认值，无需再次设置
func (b *BannerManager) PrintPProfInfo(ctx context.Context) {
	if !b.config.Middleware.PProf.Enabled {
		return
	}

	baseURL := b.getBaseURL()

	global.LOGGER.InfoContext(b.ctx, "🔬 性能分析 (PProf):")
	global.LOGGER.InfoContext(b.ctx, "   🎯 状态: 已启用")
	global.LOGGER.InfoContext(b.ctx, "   🏠 仪表板: "+baseURL+"/")
	pprofPrefix := b.config.Middleware.PProf.PathPrefix
	global.LOGGER.InfoContext(b.ctx, "   🔍 PProf索引: "+baseURL+pprofPrefix+"/")
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
	global.LOGGER.InfoContext(b.ctx, "🔌 中间件状态:")

	middlewares := []struct {
		name    string
		enabled bool
		desc    string
	}{
		// 核心中间件
		{"Recovery", b.config.Middleware.Recovery.Enabled, "异常恢复"},
		{"RequestID", b.config.Middleware.RequestID.Enabled, "请求ID生成"},
		{"I18n", b.config.Middleware.I18N.Enabled, "国际化支持"},
		{"ContextTrace", b.config.Middleware.RequestID.Enabled, "上下文追踪"},

		// 安全中间件
		{"CORS", b.config.CORS.AllowedAllOrigins || len(b.config.CORS.AllowedOrigins) > 0, "跨域处理"},
		{"CSP", b.config.Security.CSP.Enabled, "内容安全策略"},
		{"JWT", b.config.Security.JWT.Secret != "", "身份认证"},
		{"Signature", b.config.Middleware.Signature.Enabled, "签名验证"},

		// 流量控制
		{"RateLimit", b.config.RateLimit.Enabled, "限流控制"},
		{"CircuitBreaker", b.config.Middleware.CircuitBreaker.Enabled, "熔断保护"},

		// 日志和监控
		{"Logging", b.config.Middleware.Logging.Enabled, "访问日志"},
		{"Metrics", b.config.Middleware.Metrics.Enabled, "性能指标"},
		{"Tracing", b.config.Middleware.Tracing.Enabled, "链路追踪"},

		// 开发工具
		{"Swagger", b.config.Swagger.Enabled, "API文档"},
		{"PProf", b.config.Middleware.PProf.Enabled, "性能分析"},
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

	global.LOGGER.InfoContext(b.ctx, "💡 使用指南:")
	global.LOGGER.InfoContext(b.ctx, "   📖 访问主页查看完整信息: "+baseURL+"/")

	if b.config.Health.Enabled {
		global.LOGGER.InfoContext(b.ctx, "   🏥 健康检查: curl "+baseURL+b.config.Health.Path)
	}

	if b.config.Monitoring.Prometheus.Enabled {
		global.LOGGER.InfoContext(b.ctx, "   📊 监控指标: curl "+baseURL+b.config.Monitoring.Prometheus.Path)
	}

	global.LOGGER.InfoContext(b.ctx, "   ⏹️  优雅关闭: 按 Ctrl+C")
}
