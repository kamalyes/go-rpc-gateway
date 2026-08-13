/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-23 13:05:00
 * @FilePath: \go-rpc-gateway\server\banner.go
 * @Description: Gateway 启动横幅与展示渲染
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"context"
	"fmt"

	"github.com/kamalyes/go-config/pkg/banner"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-toolbox/pkg/netx"
)

// BannerManager 横幅管理器
// 使用 ConsoleGroup 输出纯文本展示内容，不走 JSON 格式化、不带 caller 信息
type BannerManager struct {
	ctx      context.Context
	config   *gwconfig.Gateway
	features []string
	cg       *logger.ConsoleGroup
}

// NewBannerManager 创建横幅管理器
func NewBannerManager(config *gwconfig.Gateway) *BannerManager {
	return &BannerManager{
		ctx:      context.Background(),
		config:   config,
		features: []string{},
		cg:       logger.NewConsoleGroup(),
	}
}

func (b *BannerManager) WithContext(ctx context.Context) *BannerManager {
	b.ctx = ctx
	return b
}

// AddFeature 添加功能特性
func (b *BannerManager) AddFeature(feature string) {
	b.features = append(b.features, feature)
}

func (b *BannerManager) printStartupBanner(report startupReport) {
	if !report.bannerEnabled {
		return
	}

	if report.bannerTemplate != "" {
		b.cg.Debug("%s", report.bannerTemplate)
	} else {
		b.cg.Debug("%s", banner.Default().Template)
	}
	b.cg.Info("🚀 %s - Enterprise Edition", report.title)

	b.printFieldSection("📋 基础信息", []startupField{
		{label: "🏷️  名称", value: report.title},
		{label: "📦 版本", value: report.version},
		{label: "🌍 环境", value: report.environment},
		{label: "🐞 调试模式", value: fmt.Sprintf("%v", report.debug)},
		{label: "🏗️  框架", value: report.framework},
	})
	b.printFieldSection("🔨 构建信息", []startupField{
		{label: "🕒 构建时间", value: report.buildTime},
		{label: "👤 构建用户", value: report.buildUser},
		{label: "🐹 Go版本", value: report.buildGoVersion},
	})
	b.printFieldSection("🔖 Git信息", []startupField{
		{label: "📝 Commit", value: report.gitCommit},
		{label: "🌿 Branch", value: report.gitBranch},
		{label: "🏷️  Tag", value: report.gitTag},
	})
	b.printFieldSection("⚙️  服务器配置", b.serverFields(report))
	b.printChecklist("🔧 企业级功能", b.featureLabels(report))
	b.printFieldSection("📡 核心端点", b.endpointFields(report))
	b.printFieldSection("💻 系统信息", []startupField{
		{label: "🐹 Go版本", value: report.runtime.goVersion},
		{label: "🔧 CPU核心", value: fmt.Sprintf("%d", report.runtime.cpu)},
		{label: "🧵 Goroutines", value: fmt.Sprintf("%d", report.runtime.goroutines)},
		{label: "💾 系统", value: report.runtime.osArch},
		{label: "⏰ 启动时间", value: report.runtime.startedAt},
	})

	b.cg.Info("🎉 ================================================")
}

// PrintShutdownBanner 打印关闭横幅
func (b *BannerManager) PrintShutdownBanner() {
	b.cg.Info("🛑 ================================================")
	b.cg.Info("⏹️  Gateway正在优雅关闭...")
	b.cg.Info("🛑 ================================================")
}

// PrintShutdownComplete 打印关闭完成
func (b *BannerManager) PrintShutdownComplete() {
	b.cg.Info("✅ Gateway已安全关闭")
	b.cg.Info("👋 感谢使用 Go RPC Gateway！")
}

func (b *BannerManager) printMiddlewareStatus(report startupReport) {
	b.cg.Info("🔌 中间件状态:")
	for _, item := range report.middleware {
		status := "❌ 禁用"
		if item.enabled {
			status = "✅ 启用"
		}
		b.cg.Info("   %s - %s (%s)", status, item.displayLabel(), item.name)
	}
}

func (b *BannerManager) printUsageGuide(report startupReport) {
	fields := []startupField{
		{label: "📖 访问主页查看完整信息", value: report.baseURL + "/"},
	}

	for _, module := range report.modules {
		if !module.enabled {
			continue
		}
		switch module.name {
		case "health":
			fields = append(fields, startupField{
				label: "🏥 健康检查",
				value: "curl " + report.baseURL + module.path,
			})
		}
	}

	for _, item := range report.monitoring {
		if !item.enabled {
			continue
		}
		switch item.name {
		case "prometheus":
			fields = append(fields, startupField{
				label: "📊 监控指标",
				value: "curl " + report.baseURL + item.path,
			})
		}
	}

	fields = append(fields, startupField{label: "⏹️  优雅关闭", value: "按 Ctrl+C"})
	b.printFieldSection("💡 使用指南", fields)
}

func (b *BannerManager) printPProfInfo(report startupReport) {
	for _, item := range report.monitoring {
		if item.name != "pprof" || !item.enabled {
			continue
		}

		b.printFieldSection("🔬 性能分析 (PProf)", []startupField{
			{label: "🎯 状态", value: "已启用"},
			{label: "🏠 仪表板", value: report.baseURL + "/"},
			{label: "🔍 PProf索引", value: item.detail},
		})
		return
	}
}

func (b *BannerManager) serverFields(report startupReport) []startupField {
	fields := make([]startupField, 0, len(report.services)+1)
	for _, service := range report.services {
		value := netx.JoinHostPort(service.host, service.port)
		if service.name == "HTTP" {
			value = report.baseURL
		}
		if !service.enabled {
			value += " (已禁用)"
		}
		fields = append(fields, startupField{
			label: service.displayLabel() + "服务器",
			value: value,
		})
	}

	for _, module := range report.modules {
		if module.name == "health" && module.enabled {
			fields = append(fields, startupField{
				label: "❤️  健康检查",
				value: module.path,
			})
			break
		}
	}

	return fields
}

func (b *BannerManager) featureLabels(report startupReport) []string {
	labels := make([]string, 0, len(report.features))
	for _, item := range report.features {
		if !item.enabled {
			continue
		}
		label := item.label
		if item.icon != "" {
			label = item.displayLabel()
		}
		if item.detail != "" {
			label += " (" + item.detail + ")"
		}
		if item.note != "" {
			if item.detail != "" {
				label += " " + item.note
			} else {
				label += " (" + item.note + ")"
			}
		}
		labels = append(labels, label)
	}
	return labels
}

func (b *BannerManager) endpointFields(report startupReport) []startupField {
	fields := []startupField{}

	for _, module := range report.modules {
		if !module.enabled {
			continue
		}
		switch module.name {
		case "health":
			fields = append(fields, startupField{label: "🏥 健康检查", value: report.baseURL + module.path})
		case "swagger":
			fields = append(fields, startupField{label: "📚 API文档", value: report.baseURL + module.path})
		}
	}

	for _, item := range report.monitoring {
		if !item.enabled {
			continue
		}
		switch item.name {
		case "prometheus":
			fields = append(fields, startupField{label: "📊 监控指标", value: item.detail})
		case "pprof":
			fields = append(fields, startupField{label: "🔬 性能分析", value: item.detail})
		}
	}

	return fields
}

func (b *BannerManager) printFieldSection(title string, fields []startupField) {
	if len(fields) == 0 {
		return
	}

	b.cg.Info("%s:", title)
	for _, field := range fields {
		b.cg.Info("   %s: %s", field.label, field.value)
	}
	b.cg.Info("")
}

func (b *BannerManager) printChecklist(title string, items []string) {
	if len(items) == 0 {
		return
	}

	b.cg.Info("%s:", title)
	for _, item := range items {
		b.cg.Info("   ✅ %s", item)
	}
	b.cg.Info("")
}
