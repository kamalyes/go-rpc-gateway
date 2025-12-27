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
	"context"
	"fmt"
	"time"

	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/global"
)

// StartupReporter 启动状态报告器
type StartupReporter struct {
	ctx    context.Context
	config *gwconfig.Gateway
}

// NewStartupReporter 创建启动状态报告器
func NewStartupReporter(config *gwconfig.Gateway) *StartupReporter {
	return &StartupReporter{
		ctx:    context.Background(),
		config: config,
	}
}

// WithContext 设置上下文
func (r *StartupReporter) WithContext(ctx context.Context) *StartupReporter {
	if ctx != nil {
		r.ctx = ctx
	}
	return r
}

// PrintStartupStatus 打印启动状态
func (r *StartupReporter) PrintStartupStatus() {
	if r.config == nil {
		global.LOGGER.WarnContext(r.ctx, "⚠️  配置未初始化，无法打印启动状态")
		return
	}

	// 使用 Console 分组展示启动状态
	cg := global.LOGGER.NewConsoleGroup()
	cg.Group("🚀 Gateway 服务启动状态检查")

	// 打印基础信息
	r.printBasicStatus(cg)

	// 打印功能模块状态
	r.printFeatureStatus(cg)

	// 打印中间件状态
	r.printMiddlewareStatus(cg)

	// 打印监控和分析功能状态
	r.printMonitoringStatus(cg)

	// 打印启动摘要
	r.printStartupSummaryInternal(cg)

	cg.Info("✅ 启动状态检查完成")
	cg.GroupEnd()
}

// printBasicStatus 打印基础状态
func (r *StartupReporter) printBasicStatus(cg *logger.ConsoleGroup) {
	cg.Group("📋 基础服务状态")

	basicInfo := [][]string{
		{"服务类型", "地址", "端口", "状态"},
		{"HTTP", r.config.HTTPServer.Host, fmt.Sprintf("%d", r.config.HTTPServer.Port), "✅ 运行中"},
		{"gRPC", r.config.GRPC.Server.Host, fmt.Sprintf("%d", r.config.GRPC.Server.Port), "✅ 运行中"},
	}
	cg.Table(basicInfo)

	envInfo := map[string]interface{}{
		"运行环境": r.config.Environment,
		"调试模式": r.config.Debug,
	}
	cg.Table(envInfo)

	cg.GroupEnd()
}

// printFeatureStatus 打印功能状态
func (r *StartupReporter) printFeatureStatus(cg *logger.ConsoleGroup) {
	cg.Group("🔧 功能模块状态")

	features := []map[string]interface{}{
		{
			"功能名称": "健康检查",
			"状态":   r.getStatusIcon(r.config.Health.Enabled),
			"路径":   r.config.Health.Path,
		},
		{
			"功能名称": "Swagger文档",
			"状态":   r.getStatusIcon(r.config.Swagger.Enabled),
			"路径":   r.config.Swagger.UIPath,
		},
		{
			"功能名称": "WebSocket",
			"状态":   r.getStatusIcon(r.config.WSC.Enabled),
			"路径":   r.config.WSC.Path,
		},
	}
	cg.Table(features)

	cg.GroupEnd()
}

// printMiddlewareStatus 打印中间件状态
func (r *StartupReporter) printMiddlewareStatus(cg *logger.ConsoleGroup) {
	cg.Group("🔌 中间件状态")

	corsEnabled := r.config.CORS.AllowedAllOrigins || len(r.config.CORS.AllowedOrigins) > 0
	authEnabled := r.config.Security.JWT.Secret != ""

	middlewares := []map[string]interface{}{
		{"中间件": "CORS跨域", "状态": r.getStatusIcon(corsEnabled)},
		{"中间件": "限流控制", "状态": r.getStatusIcon(r.config.RateLimit.Enabled)},
		{"中间件": "请求ID生成", "状态": r.getStatusIcon(r.config.Middleware.RequestID.Enabled)},
		{"中间件": "异常恢复", "状态": r.getStatusIcon(r.config.Middleware.Recovery.Enabled)},
		{"中间件": "访问日志", "状态": r.getStatusIcon(r.config.Middleware.Logging.Enabled)},
		{"中间件": "身份认证(JWT)", "状态": r.getStatusIcon(authEnabled)},
		{"中间件": "CSP内容安全策略", "状态": r.getStatusIcon(r.config.Security.CSP.Enabled)},
		{"中间件": "指标收集", "状态": r.getStatusIcon(r.config.Middleware.Metrics.Enabled)},
		{"中间件": "链路追踪", "状态": r.getStatusIcon(r.config.Middleware.Tracing.Enabled)},
		{"中间件": "熔断器", "状态": r.getStatusIcon(r.config.Middleware.CircuitBreaker.Enabled)},
		{"中间件": "签名验证", "状态": r.getStatusIcon(r.config.Middleware.Signature.Enabled)},
		{"中间件": "国际化", "状态": r.getStatusIcon(r.config.Middleware.I18N.Enabled)},
	}
	cg.Table(middlewares)

	cg.GroupEnd()
}

// printMonitoringStatus 打印监控和分析功能状态
func (r *StartupReporter) printMonitoringStatus(cg *logger.ConsoleGroup) {
	cg.Group("📊 监控与分析状态")

	monitoring := []map[string]interface{}{}

	if r.config.Monitoring.Prometheus.Enabled {
		monitoring = append(monitoring, map[string]interface{}{
			"类型": "Prometheus指标",
			"状态": "✅ 已启用",
			"访问": fmt.Sprintf("http://localhost:%d%s", r.config.Monitoring.Prometheus.Port, r.config.Monitoring.Prometheus.Path),
		})
	}

	if r.config.Middleware.PProf.Enabled {
		authStatus := "⚠️  未启用认证"
		if r.config.Middleware.PProf.Authentication.Enabled {
			authStatus = "🔐 已启用认证"
		}
		monitoring = append(monitoring, map[string]interface{}{
			"类型": "PProf性能分析",
			"状态": "✅ 已启用",
			"访问": fmt.Sprintf("http://localhost:%d%s/", r.config.Middleware.PProf.Port, r.config.Middleware.PProf.PathPrefix),
			"认证": authStatus,
		})
	}

	if r.config.Monitoring.Jaeger.Enabled {
		monitoring = append(monitoring, map[string]interface{}{
			"类型":   "Jaeger链路追踪",
			"状态":   "✅ 已启用",
			"服务名称": r.config.Monitoring.Jaeger.ServiceName,
		})
	}

	if len(monitoring) > 0 {
		cg.Table(monitoring)
	} else {
		cg.Info("所有监控功能均未启用")
	}

	cg.GroupEnd()
}

// printStartupSummaryInternal 打印启动摘要（内部方法，用于 Console 分组）
func (r *StartupReporter) printStartupSummaryInternal(cg *logger.ConsoleGroup) {
	if r.config == nil {
		return
	}

	enabledCount := 0
	totalCount := 0

	// 统计功能状态
	features := []bool{
		r.config.Health.Enabled,
		r.config.Swagger.Enabled,
		r.config.Monitoring.Prometheus.Enabled,
		r.config.Middleware.PProf.Enabled,
		r.config.Monitoring.Jaeger.Enabled,
		r.config.WSC.Enabled,
		r.config.CORS.AllowedAllOrigins || len(r.config.CORS.AllowedOrigins) > 0,
		r.config.RateLimit.Enabled,
	}

	for _, enabled := range features {
		totalCount++
		if enabled {
			enabledCount++
		}
	}

	summary := map[string]interface{}{
		"已启用功能": enabledCount,
		"总功能数":  totalCount,
		"启用率":   fmt.Sprintf("%.1f%%", float64(enabledCount)/float64(totalCount)*100),
		"启动时间":  time.Now().Format("2006-01-02 15:04:05"),
	}
	cg.Table(summary)
}

// getStatusIcon 获取状态图标
func (r *StartupReporter) getStatusIcon(enabled bool) string {
	if enabled {
		return "✅ 已启用"
	}
	return "❌ 已禁用"
}

// PrintStartupTimestamp 打印启动时间戳
func (r *StartupReporter) PrintStartupTimestamp() {
	global.LOGGER.InfoContext(r.ctx, "🕐 服务启动时间: %s",
		time.Now().Format("2006-01-02 15:04:05"))
}

// PrintStartupSummary 打印启动摘要
func (r *StartupReporter) PrintStartupSummary() {
	if r.config == nil {
		return
	}

	enabledCount := 0
	totalCount := 0

	// 统计功能状态
	features := []bool{
		r.config.Health.Enabled,
		r.config.Swagger.Enabled,
		r.config.Monitoring.Prometheus.Enabled,
		r.config.Middleware.PProf.Enabled,
		r.config.Monitoring.Jaeger.Enabled,
		r.config.WSC.Enabled,
		r.config.CORS.AllowedAllOrigins || len(r.config.CORS.AllowedOrigins) > 0,
		r.config.RateLimit.Enabled,
	}

	for _, enabled := range features {
		totalCount++
		if enabled {
			enabledCount++
		}
	}

	global.LOGGER.InfoContext(r.ctx, "📋 功能启用摘要: %d/%d 个功能已启用 (%.1f%%)",
		enabledCount, totalCount, float64(enabledCount)/float64(totalCount)*100)
}
