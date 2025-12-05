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
	"time"

	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
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

	global.LOGGER.InfoContext(r.ctx, "🔄 ===== 服务启动状态检查 =====")

	// 打印基础信息
	r.printBasicStatus()

	// 打印功能模块状态
	r.printFeatureStatus()

	// 打印中间件状态
	r.printMiddlewareStatus()

	// 打印监控和分析功能状态
	r.printMonitoringStatus()

	global.LOGGER.InfoContext(r.ctx, "✅ ===== 启动状态检查完成 =====")
}

// printBasicStatus 打印基础状态
func (r *StartupReporter) printBasicStatus() {
	global.LOGGER.InfoContext(r.ctx, "📋 基础服务状态:")

	// HTTP 服务器
	global.LOGGER.InfoContext(r.ctx, "   🌐 HTTP服务: %s:%d",
		r.config.HTTPServer.Host,
		r.config.HTTPServer.Port)

	// gRPC 服务器
	global.LOGGER.InfoContext(r.ctx, "   📡 gRPC服务: %s:%d",
		r.config.GRPC.Server.Host,
		r.config.GRPC.Server.Port)

	// 环境模式
	global.LOGGER.InfoContext(r.ctx, "   🌍 运行环境: %s (调试模式: %v)",
		r.config.Environment,
		r.config.Debug)
}

// printFeatureStatus 打印功能状态
func (r *StartupReporter) printFeatureStatus() {
	global.LOGGER.InfoContext(r.ctx, "🔧 功能模块状态:")

	// 健康检查
	if r.config.Health.Enabled {
		global.LOGGER.InfoContext(r.ctx, "   ✅ 健康检查: 已启用 (%s)", r.config.Health.Path)
	} else {
		global.LOGGER.InfoContext(r.ctx, "   ❌ 健康检查: 已禁用")
	}

	// Swagger 文档
	if r.config.Swagger.Enabled {
		global.LOGGER.InfoContext(r.ctx, "   ✅ Swagger文档: 已启用 (%s)", r.config.Swagger.UIPath)
	} else {
		global.LOGGER.InfoContext(r.ctx, "   ❌ Swagger文档: 已禁用")
	}

	// WebSocket 支持
	if r.config.WSC.Enabled {
		global.LOGGER.InfoContext(r.ctx, "   ✅ WebSocket: 已启用 (%s)", r.config.WSC.Path)
	} else {
		global.LOGGER.InfoContext(r.ctx, "   ❌ WebSocket: 已禁用")
	}
}

// printMiddlewareStatus 打印中间件状态
func (r *StartupReporter) printMiddlewareStatus() {
	global.LOGGER.InfoContext(r.ctx, "🔌 中间件状态:")

	// CORS 跨域
	corsEnabled := r.config.CORS.AllowedAllOrigins || len(r.config.CORS.AllowedOrigins) > 0
	r.printMiddlewareItem("CORS跨域", corsEnabled)

	// 限流控制
	r.printMiddlewareItem("限流控制", r.config.RateLimit.Enabled)

	// 请求ID生成
	r.printMiddlewareItem("请求ID生成", r.config.Middleware.RequestID.Enabled)

	// 异常恢复
	r.printMiddlewareItem("异常恢复", r.config.Middleware.Recovery.Enabled)

	// 访问日志
	r.printMiddlewareItem("访问日志", r.config.Middleware.Logging.Enabled)

	// 身份认证
	authEnabled := r.config.Security.JWT.Secret != ""
	r.printMiddlewareItem("身份认证(JWT)", authEnabled)

	// 安全头设置
	r.printMiddlewareItem("安全头设置", r.config.Security.Enabled)
}

// printMonitoringStatus 打印监控和分析功能状态
func (r *StartupReporter) printMonitoringStatus() {
	global.LOGGER.InfoContext(r.ctx, "📊 监控与分析状态:")

	// Prometheus Metrics
	if r.config.Monitoring.Prometheus.Enabled {
		global.LOGGER.InfoContext(r.ctx, "   ✅ Prometheus指标: 已启用 (http://localhost:%d%s)",
			r.config.Monitoring.Prometheus.Port,
			r.config.Monitoring.Prometheus.Path)
	} else {
		global.LOGGER.InfoContext(r.ctx, "   ❌ Prometheus指标: 已禁用")
	}

	// PProf 性能分析
	if r.config.Middleware.PProf.Enabled {
		global.LOGGER.InfoContext(r.ctx, "   ✅ PProf性能分析: 已启用 (http://localhost:%d%s/)",
			r.config.Middleware.PProf.Port,
			r.config.Middleware.PProf.PathPrefix)

		// 检查认证状态
		authStatus := "已禁用 ⚠️"
		if r.config.Middleware.PProf.Authentication.Enabled {
			authStatus = "已启用 🔐"
		}
		global.LOGGER.InfoContext(r.ctx, "     🔐 PProf认证: %s", authStatus)
	} else {
		global.LOGGER.InfoContext(r.ctx, "   ❌ PProf性能分析: 已禁用")
	}

	// Jaeger 链路追踪
	if r.config.Monitoring.Jaeger.Enabled {
		global.LOGGER.InfoContext(r.ctx, "   ✅ 链路追踪: 已启用 (%s)",
			r.config.Monitoring.Jaeger.ServiceName)
	} else {
		global.LOGGER.InfoContext(r.ctx, "   ❌ 链路追踪: 已禁用")
	}
}

// printMiddlewareItem 打印中间件项状态
func (r *StartupReporter) printMiddlewareItem(name string, enabled bool) {
	status := "❌ 已禁用"
	if enabled {
		status = "✅ 已启用"
	}
	global.LOGGER.InfoContext(r.ctx, "   %s %s", status, name)
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
