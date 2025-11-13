/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-13 18:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 18:30:00
 * @FilePath: \go-rpc-gateway\server\startup.go
 * @Description: 启动状态打印和检测功能
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"fmt"
	"time"

	goconfig "github.com/kamalyes/go-config"
	"github.com/kamalyes/go-rpc-gateway/global"
)

// StartupReporter 启动状态报告器
type StartupReporter struct {
	config interface{}
}

// NewStartupReporter 创建启动状态报告器
func NewStartupReporter(config interface{}) *StartupReporter {
	return &StartupReporter{
		config: config,
	}
}

// PrintStartupStatus 打印启动状态
func (r *StartupReporter) PrintStartupStatus() {
	if r.config == nil {
		global.LOGGER.Warn("⚠️  配置未初始化，无法打印启动状态")
		return
	}

	configSafe := goconfig.SafeConfig(r.config)

	global.LOGGER.Info("🔄 ===== 服务启动状态检查 =====")

	// 打印基础信息
	r.printBasicStatus(configSafe)

	// 打印功能模块状态
	r.printFeatureStatus(configSafe)

	// 打印中间件状态
	r.printMiddlewareStatus(configSafe)

	// 打印监控和分析功能状态
	r.printMonitoringStatus(configSafe)

	global.LOGGER.Info("✅ ===== 启动状态检查完成 =====")
}

// printBasicStatus 打印基础状态
func (r *StartupReporter) printBasicStatus(configSafe *goconfig.ConfigSafe) {
	global.LOGGER.Info("📋 基础服务状态:")

	// HTTP 服务器
	httpHost := configSafe.Field("HTTPServer").Field("Host").String("localhost")
	httpPort := configSafe.Field("HTTPServer").Field("Port").Int(8080)
	global.LOGGER.Info(fmt.Sprintf("   🌐 HTTP服务: %s:%d", httpHost, httpPort))

	// gRPC 服务器
	grpcHost := configSafe.Field("GRPCServer").Field("Host").String("localhost")
	grpcPort := configSafe.Field("GRPCServer").Field("Port").Int(9090)
	global.LOGGER.Info(fmt.Sprintf("   📡 gRPC服务: %s:%d", grpcHost, grpcPort))

	// 环境模式
	environment := configSafe.Field("Environment").String("development")
	debug := configSafe.Field("Debug").Bool(false)
	global.LOGGER.Info(fmt.Sprintf("   🌍 运行环境: %s (调试模式: %v)", environment, debug))
}

// printFeatureStatus 打印功能状态
func (r *StartupReporter) printFeatureStatus(configSafe *goconfig.ConfigSafe) {
	global.LOGGER.Info("🔧 功能模块状态:")

	// 健康检查
	if configSafe.IsHealthEnabled() {
		healthPath := configSafe.GetHealthPath("/health")
		global.LOGGER.Info(fmt.Sprintf("   ✅ 健康检查: 已启用 (%s)", healthPath))
	} else {
		global.LOGGER.Info("   ❌ 健康检查: 已禁用")
	}

	// Swagger 文档
	swaggerEnabled := configSafe.Field("Swagger").Field("Enabled").Bool(false)
	if swaggerEnabled {
		swaggerPath := configSafe.Field("Swagger").Field("UIPath").String("/swagger")
		global.LOGGER.Info(fmt.Sprintf("   ✅ Swagger文档: 已启用 (%s)", swaggerPath))
	} else {
		global.LOGGER.Info("   ❌ Swagger文档: 已禁用")
	}

	// WebSocket 支持
	wscEnabled := configSafe.Field("WSC").Field("Enabled").Bool(false)
	if wscEnabled {
		wscPath := configSafe.Field("WSC").Field("Path").String("/ws")
		global.LOGGER.Info(fmt.Sprintf("   ✅ WebSocket: 已启用 (%s)", wscPath))
	} else {
		global.LOGGER.Info("   ❌ WebSocket: 已禁用")
	}
}

// printMiddlewareStatus 打印中间件状态
func (r *StartupReporter) printMiddlewareStatus(configSafe *goconfig.ConfigSafe) {
	global.LOGGER.Info("🔌 中间件状态:")

	// CORS 跨域
	corsEnabled := configSafe.Field("CORS").Field("AllowedAllOrigins").Bool(false) ||
		configSafe.Field("CORS").Field("AllowedOrigins").String("") != ""
	r.printMiddlewareItem("CORS跨域", corsEnabled)

	// 限流控制
	rateLimitEnabled := configSafe.Field("RateLimit").Field("Enabled").Bool(false)
	r.printMiddlewareItem("限流控制", rateLimitEnabled)

	// 请求ID生成
	requestIDEnabled := configSafe.Field("Middleware").Field("RequestID").Field("Enabled").Bool(false)
	r.printMiddlewareItem("请求ID生成", requestIDEnabled)

	// 异常恢复
	recoveryEnabled := configSafe.Field("Middleware").Field("Recovery").Field("Enabled").Bool(false)
	r.printMiddlewareItem("异常恢复", recoveryEnabled)

	// 访问日志
	accessLogEnabled := configSafe.Field("Middleware").Field("Logging").Field("Enabled").Bool(false)
	r.printMiddlewareItem("访问日志", accessLogEnabled)

	// 身份认证
	authEnabled := configSafe.Field("JWT").Field("SigningKey").String("") != ""
	r.printMiddlewareItem("身份认证(JWT)", authEnabled)

	// 安全头设置
	securityEnabled := configSafe.Field("Security").Field("Enabled").Bool(false)
	r.printMiddlewareItem("安全头设置", securityEnabled)
}

// printMonitoringStatus 打印监控和分析功能状态
func (r *StartupReporter) printMonitoringStatus(configSafe *goconfig.ConfigSafe) {
	global.LOGGER.Info("📊 监控与分析状态:")

	// Prometheus Metrics
	metricsEnabled := configSafe.IsMetricsEnabled()
	if metricsEnabled {
		metricsHost := configSafe.Field("metrics").Field("host").String("0.0.0.0")
		metricsPort := configSafe.Field("metrics").Field("port").Int(9090)
		metricsPath := configSafe.Field("metrics").Field("path").String("/metrics")
		displayHost := metricsHost
		if metricsHost == "0.0.0.0" {
			displayHost = "localhost"
		}
		global.LOGGER.Info(fmt.Sprintf("   ✅ Prometheus指标: 已启用 (http://%s:%d%s)",
			displayHost, metricsPort, metricsPath))

		// 检查自定义指标状态
		httpMetrics := configSafe.Field("metrics").Field("custom_metrics").Field("http_requests_total").Field("enabled").Bool(false)
		grpcMetrics := configSafe.Field("metrics").Field("custom_metrics").Field("grpc_requests_total").Field("enabled").Bool(false)
		redisMetrics := configSafe.Field("metrics").Field("custom_metrics").Field("redis_operations_total").Field("enabled").Bool(false)

		if httpMetrics || grpcMetrics || redisMetrics {
			global.LOGGER.Info(fmt.Sprintf("     📈 自定义指标: HTTP(%v) gRPC(%v) Redis(%v)",
				httpMetrics, grpcMetrics, redisMetrics))
		}

		// 检查中间件指标状态
		middlewareMetricsEnabled := configSafe.Field("middleware").Field("metrics").Field("enabled").Bool(false)
		if middlewareMetricsEnabled {
			excludePaths := configSafe.Field("middleware").Field("metrics").Field("exclude_paths").String("")
			global.LOGGER.Info(fmt.Sprintf("     🔗 中间件指标: 已启用 (排除路径: %s)", excludePaths))
		}
	} else {
		global.LOGGER.Info("   ❌ Prometheus指标: 已禁用")
	}

	// PProf 性能分析
	pprofEnabled := configSafe.IsPProfEnabled()
	if pprofEnabled {
		pprofHost := configSafe.Field("pprof").Field("host").String("0.0.0.0")
		pprofPort := configSafe.Field("pprof").Field("port").Int(6060)
		pprofPath := configSafe.GetPProfPathPrefix("/debug/pprof")
		displayHost := pprofHost
		if pprofHost == "0.0.0.0" {
			displayHost = "localhost"
		}
		global.LOGGER.Info(fmt.Sprintf("   ✅ PProf性能分析: 已启用 (http://%s:%d%s/)",
			displayHost, pprofPort, pprofPath))

		// 检查认证状态
		pprofAuth := configSafe.Field("pprof").Field("auth").Field("enabled").Bool(false)
		authStatus := "已禁用 ⚠️"
		if pprofAuth {
			authStatus = "已启用 🔐"
		}
		global.LOGGER.Info("     🔐 PProf认证: " + authStatus)

		// 检查中间件状态
		pprofMiddlewareEnabled := configSafe.Field("middleware").Field("pprof").Field("enabled").Bool(false)
		if pprofMiddlewareEnabled {
			global.LOGGER.Info("     🔗 PProf中间件: 已启用")
		}
	} else {
		global.LOGGER.Info("   ❌ PProf性能分析: 已禁用")
	}

	// Jaeger 链路追踪
	jaegerEnabled := configSafe.IsJaegerEnabled()
	if jaegerEnabled {
		serviceName := configSafe.GetJaegerServiceName("gateway-service")
		global.LOGGER.Info(fmt.Sprintf("   ✅ 链路追踪: 已启用 (%s)", serviceName))
	} else {
		global.LOGGER.Info("   ❌ 链路追踪: 已禁用")
	}
}

// printMiddlewareItem 打印中间件项状态
func (r *StartupReporter) printMiddlewareItem(name string, enabled bool) {
	status := "❌ 已禁用"
	if enabled {
		status = "✅ 已启用"
	}
	global.LOGGER.Info(fmt.Sprintf("   %s %s", status, name))
}

// PrintStartupTimestamp 打印启动时间戳
func (r *StartupReporter) PrintStartupTimestamp() {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	global.LOGGER.Info(fmt.Sprintf("🕐 服务启动时间: %s", timestamp))
}

// PrintStartupSummary 打印启动摘要
func (r *StartupReporter) PrintStartupSummary() {
	if r.config == nil {
		return
	}

	configSafe := goconfig.SafeConfig(r.config)

	enabledCount := 0
	totalCount := 0

	// 统计功能状态
	features := []bool{
		configSafe.IsHealthEnabled(),
		configSafe.Field("Swagger").Field("Enabled").Bool(false),
		configSafe.IsMetricsEnabled(),
		configSafe.IsPProfEnabled(),
		configSafe.IsJaegerEnabled(),
		configSafe.Field("WSC").Field("Enabled").Bool(false),
		configSafe.Field("CORS").Field("AllowedAllOrigins").Bool(false) || configSafe.Field("CORS").Field("AllowedOrigins").String("") != "",
		configSafe.Field("RateLimit").Field("Enabled").Bool(false),
	}

	for _, enabled := range features {
		totalCount++
		if enabled {
			enabledCount++
		}
	}

	global.LOGGER.Info(fmt.Sprintf("📋 功能启用摘要: %d/%d 个功能已启用 (%.1f%%)",
		enabledCount, totalCount, float64(enabledCount)/float64(totalCount)*100))
}
