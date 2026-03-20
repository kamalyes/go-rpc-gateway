/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-13 18:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-23 13:20:10
 * @FilePath: \go-rpc-gateway\server\startup.go
 * @Description: 启动展示统一模型与渲染入口
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"fmt"
	"runtime"
	"time"

	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

type startupField struct {
	label string
	value string
}

type startupService struct {
	icon    string
	name    string
	host    string
	port    int
	enabled bool
}

type startupToggle struct {
	name    string
	icon    string
	label   string
	enabled bool
	path    string
	detail  string
	note    string
}

type startupRuntime struct {
	goVersion  string
	cpu        int
	goroutines int
	osArch     string
	startedAt  string
}

type startupSummary struct {
	enabledCount int
	totalCount   int
	startedAt    string
}

func (s startupService) displayLabel() string {
	if s.icon == "" {
		return s.name
	}
	return s.icon + " " + s.name
}

func (t startupToggle) displayLabel() string {
	if t.icon == "" {
		return t.label
	}
	return t.icon + " " + t.label
}

func (s startupSummary) rate() string {
	if s.totalCount == 0 {
		return "0.0%"
	}
	return fmt.Sprintf("%.1f%%", float64(s.enabledCount)/float64(s.totalCount)*100)
}

type startupReport struct {
	title          string
	bannerEnabled  bool
	bannerTemplate string
	baseURL        string
	version        string
	environment    string
	debug          bool
	framework      string
	buildTime      string
	buildUser      string
	buildGoVersion string
	gitCommit      string
	gitBranch      string
	gitTag         string
	startedAt      string
	services       []startupService
	modules        []startupToggle
	features       []startupToggle
	middleware     []startupToggle
	monitoring     []startupToggle
	runtime        startupRuntime
	summary        startupSummary
}

// PrintStartupChecks 打印启动前检查
func (b *BannerManager) PrintStartupChecks() {
	if b == nil || b.config == nil {
		return
	}

	report := b.buildStartupReport()
	b.printStartupTimestamp(report)
	b.printStartupStatus(report)
}

// PrintStartupReport 打印启动成功后的完整报告
func (b *BannerManager) PrintStartupReport() {
	if b == nil || b.config == nil {
		return
	}

	report := b.buildStartupReport()
	b.printStartupBanner(report)
	b.printMiddlewareStatus(report)
	b.printUsageGuide(report)
	b.printPProfInfo(report)
	b.printStartupSummary(report)
}

func (b *BannerManager) buildStartupReport() startupReport {
	startedAt := time.Now().Format(time.RFC3339)
	title := mathx.IfEmpty(b.config.Banner.Title, "Gateway")

	baseURL := fmt.Sprintf("http://%s:%d", b.config.HTTPServer.Host, b.config.HTTPServer.Port)
	metricsURL := fmt.Sprintf("http://%s:%d%s", b.config.HTTPServer.Host, b.config.Monitoring.Prometheus.Port, b.config.Monitoring.Prometheus.Path)
	pprofURL := fmt.Sprintf("http://%s:%d%s/", b.config.HTTPServer.Host, b.config.Middleware.PProf.Port, b.config.Middleware.PProf.PathPrefix)
	pprofAuthStatus := "已禁用 (开发模式)"
	if b.config.Middleware.PProf.Authentication.Enabled {
		pprofAuthStatus = "已启用"
	}

	services := []startupService{
		{icon: "🌐", name: "HTTP", host: b.config.HTTPServer.Host, port: b.config.HTTPServer.Port, enabled: true},
		{icon: "📡", name: "gRPC", host: b.config.GRPC.Server.Host, port: b.config.GRPC.Server.Port, enabled: b.config.GRPC.Server.Enable},
	}

	modules := []startupToggle{
		{name: "health", icon: "🏥", label: "健康检查", enabled: b.config.Health.Enabled, path: b.config.Health.Path, detail: b.config.Health.Path},
		{name: "swagger", icon: "📚", label: "Swagger文档", enabled: b.config.Swagger.Enabled, path: b.config.Swagger.UIPath, detail: b.config.Swagger.UIPath},
		{name: "websocket", icon: "🔌", label: "WebSocket", enabled: b.config.WSC.Enabled, path: b.config.WSC.Path, detail: b.config.WSC.Path},
	}

	features := []startupToggle{
		{name: "grpc_gateway", icon: "🚪", label: "gRPC-Gateway集成", enabled: true},
		{name: "middleware_ecosystem", icon: "🧩", label: "中间件生态系统", enabled: true},
		{name: "config_hot_reload", icon: "♻️", label: "配置热重载", enabled: true},
		{name: "graceful_shutdown", icon: "🛑", label: "优雅关闭", enabled: true},
		{name: "i18n", icon: "🌍", label: "I18n国际化支持", enabled: true},
		{name: "request_id", icon: "🆔", label: "请求ID生成", enabled: true},
		{name: "recovery", icon: "🛡️", label: "异常恢复", enabled: true},
		{name: "security_headers", icon: "🔐", label: "安全头设置", enabled: true},
		{name: "logging", icon: "📝", label: "日志记录与管理", enabled: true},
		{name: "swagger_support", icon: "📘", label: "Swagger文档支持", enabled: true},
		{name: "cors", icon: "🌐", label: "CORS跨域支持", enabled: b.config.CORS.AllowedAllOrigins || len(b.config.CORS.AllowedOrigins) > 0},
		{name: "rate_limit", icon: "🚦", label: "限流控制", enabled: b.config.RateLimit.Enabled},
		{name: "access_logging", icon: "📋", label: "访问日志记录", enabled: b.config.Middleware.Logging.Enabled},
		{name: "jwt", icon: "🔑", label: "身份认证 (JWT)", enabled: b.config.Security.JWT.Secret != ""},
		{name: "prometheus", icon: "📊", label: "Prometheus指标", enabled: b.config.Monitoring.Prometheus.Enabled, detail: metricsURL},
		{name: "pprof", icon: "🔬", label: "PProf性能分析", enabled: b.config.Middleware.PProf.Enabled, detail: pprofURL, note: "认证: " + pprofAuthStatus},
		{name: "jaeger", icon: "🕸️", label: "链路追踪", enabled: b.config.Monitoring.Jaeger.Enabled, detail: b.config.Monitoring.Jaeger.ServiceName},
	}

	for _, feature := range b.features {
		features = append(features, startupToggle{
			name:    "custom_feature",
			icon:    "✨",
			label:   feature,
			enabled: true,
		})
	}

	middlewareItems := []startupToggle{
		{name: "recovery", icon: "🛡️", label: "异常恢复", enabled: b.config.Middleware.Recovery.Enabled},
		{name: "request_id", icon: "🆔", label: "请求ID生成", enabled: true},
		{name: "i18n", icon: "🌍", label: "国际化支持", enabled: b.config.Middleware.I18N.Enabled},
		{name: "request_context", icon: "🧭", label: "请求上下文", enabled: true},
		{name: "cors", icon: "🌐", label: "跨域处理", enabled: b.config.CORS.AllowedAllOrigins || len(b.config.CORS.AllowedOrigins) > 0},
		{name: "csp", icon: "🔐", label: "内容安全策略", enabled: b.config.Security.CSP.Enabled},
		{name: "jwt", icon: "🔑", label: "身份认证", enabled: b.config.Security.JWT.Secret != ""},
		{name: "signature", icon: "✍️", label: "签名验证", enabled: b.config.Middleware.Signature.Enabled},
		{name: "rate_limit", icon: "🚦", label: "限流控制", enabled: b.config.RateLimit.Enabled},
		{name: "circuit_breaker", icon: "⚡", label: "熔断保护", enabled: b.config.Middleware.CircuitBreaker.Enabled},
		{name: "logging", icon: "📝", label: "访问日志", enabled: b.config.Middleware.Logging.Enabled},
		{name: "metrics", icon: "📈", label: "性能指标", enabled: b.config.Middleware.Metrics.Enabled},
		{name: "tracing", icon: "🕸️", label: "链路追踪", enabled: b.config.Middleware.Tracing.Enabled},
		{name: "swagger", icon: "📘", label: "API文档", enabled: b.config.Swagger.Enabled},
		{name: "pprof", icon: "🔬", label: "性能分析", enabled: b.config.Middleware.PProf.Enabled},
	}

	monitoring := []startupToggle{
		{name: "prometheus", icon: "📊", label: "Prometheus指标", enabled: b.config.Monitoring.Prometheus.Enabled, path: b.config.Monitoring.Prometheus.Path, detail: metricsURL},
		{name: "pprof", icon: "🔬", label: "PProf性能分析", enabled: b.config.Middleware.PProf.Enabled, path: b.config.Middleware.PProf.PathPrefix, detail: pprofURL, note: "认证: " + pprofAuthStatus},
		{name: "jaeger", icon: "🕸️", label: "Jaeger链路追踪", enabled: b.config.Monitoring.Jaeger.Enabled, detail: b.config.Monitoring.Jaeger.ServiceName},
	}

	summaryFlags := []bool{
		b.config.Health.Enabled,
		b.config.Swagger.Enabled,
		b.config.Monitoring.Prometheus.Enabled,
		b.config.Middleware.PProf.Enabled,
		b.config.Monitoring.Jaeger.Enabled,
		b.config.WSC.Enabled,
		b.config.CORS.AllowedAllOrigins || len(b.config.CORS.AllowedOrigins) > 0,
		b.config.RateLimit.Enabled,
	}

	enabledCount := 0
	for _, enabled := range summaryFlags {
		if enabled {
			enabledCount++
		}
	}

	return startupReport{
		title:          title,
		bannerEnabled:  b.config.Banner.Enabled,
		bannerTemplate: b.config.Banner.Template,
		baseURL:        baseURL,
		version:        b.config.Version,
		environment:    b.config.Environment,
		debug:          b.config.Debug,
		framework:      "go-rpc-gateway (基于 go-config & go-logger & go-sqlbuilder & go-toolbox)",
		buildTime:      b.config.BuildTime,
		buildUser:      b.config.BuildUser,
		buildGoVersion: b.config.GoVersion,
		gitCommit:      b.config.GitCommit,
		gitBranch:      b.config.GitBranch,
		gitTag:         b.config.GitTag,
		startedAt:      startedAt,
		services:       services,
		modules:        modules,
		features:       features,
		middleware:     middlewareItems,
		monitoring:     monitoring,
		runtime: startupRuntime{
			goVersion:  runtime.Version(),
			cpu:        runtime.NumCPU(),
			goroutines: runtime.NumGoroutine(),
			osArch:     runtime.GOOS + "/" + runtime.GOARCH,
			startedAt:  startedAt,
		},
		summary: startupSummary{
			enabledCount: enabledCount,
			totalCount:   len(summaryFlags),
			startedAt:    startedAt,
		},
	}
}

func (b *BannerManager) printStartupTimestamp(report startupReport) {
	global.LOGGER.InfoContext(b.ctx, "🕐 服务启动时间: %s", report.startedAt)
}

func (b *BannerManager) printStartupStatus(report startupReport) {
	cg := global.LOGGER.NewConsoleGroup()
	cg.Group("🚀 Gateway 服务启动状态检查")

	cg.Group("📋 基础服务状态")
	serviceRows := [][]string{{"服务类型", "地址", "端口", "状态"}}
	for _, service := range report.services {
		serviceRows = append(serviceRows, []string{
			service.displayLabel(),
			service.host,
			fmt.Sprintf("%d", service.port),
			b.getStatusIcon(service.enabled),
		})
	}
	cg.Table(serviceRows)
	cg.Table(map[string]interface{}{
		"运行环境": report.environment,
		"调试模式": report.debug,
	})
	cg.GroupEnd()

	cg.Group("🔧 功能模块状态")
	moduleRows := make([]map[string]interface{}, 0, len(report.modules))
	for _, module := range report.modules {
		moduleRows = append(moduleRows, map[string]interface{}{
			"功能名称": module.displayLabel(),
			"状态":   b.getStatusIcon(module.enabled),
			"路径":   module.path,
		})
	}
	cg.Table(moduleRows)
	cg.GroupEnd()

	cg.Group("🔌 中间件状态")
	middlewareRows := make([]map[string]interface{}, 0, len(report.middleware))
	for _, item := range report.middleware {
		middlewareRows = append(middlewareRows, map[string]interface{}{
			"中间件": item.displayLabel(),
			"状态":  b.getStatusIcon(item.enabled),
		})
	}
	cg.Table(middlewareRows)
	cg.GroupEnd()

	cg.Group("📊 监控与分析状态")
	monitoringRows := make([]map[string]interface{}, 0, len(report.monitoring))
	for _, item := range report.monitoring {
		detail := item.detail
		if item.note != "" {
			if detail != "" {
				detail += " | " + item.note
			} else {
				detail = item.note
			}
		}
		if detail == "" {
			detail = "-"
		}
		monitoringRows = append(monitoringRows, map[string]interface{}{
			"类型": item.displayLabel(),
			"状态": b.getStatusIcon(item.enabled),
			"说明": detail,
		})
	}
	cg.Table(monitoringRows)
	cg.GroupEnd()

	cg.Table(map[string]interface{}{
		"已启用功能": report.summary.enabledCount,
		"总功能数":  report.summary.totalCount,
		"启用率":   report.summary.rate(),
		"启动时间":  report.summary.startedAt,
	})

	cg.Info("✅ 启动状态检查完成")
	cg.GroupEnd()
}

func (b *BannerManager) printStartupSummary(report startupReport) {
	global.LOGGER.InfoContext(b.ctx, "📋 功能启用摘要: %d/%d 个功能已启用 (%s)",
		report.summary.enabledCount, report.summary.totalCount, report.summary.rate())
}

func (b *BannerManager) getStatusIcon(enabled bool) string {
	if enabled {
		return "✅ 已启用"
	}
	return "❌ 已禁用"
}
