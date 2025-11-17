/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 18:22:31
 * @FilePath: \engine-im-agent-service\go-rpc-gateway\gateway.go
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
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	goconfig "github.com/kamalyes/go-config"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-rpc-gateway/cpool"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/server"
	wsc "github.com/kamalyes/go-wsc"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

// Gateway 是主要的网关服务器
type Gateway struct {
	*server.Server
	configManager  *goconfig.IntegratedConfigManager
	gatewayConfig  *gwconfig.Gateway
	enhancedServer *server.EnhancedServer // 新增增强服务器

	// API 注册信息收集
	registeredGRPCServices    []string
	registeredGatewayHandlers []string
	registeredHTTPRoutes      []string
}

// GatewayBuilder Gateway构建器 - 支持链式调用
type GatewayBuilder struct {
	configPath      string
	searchPath      string
	environment     goconfig.EnvironmentType
	configPrefix    string
	pattern         string
	hotReloadConfig *goconfig.HotReloadConfig
	contextOptions  *goconfig.ContextKeyOptions
	autoDiscovery   bool
	usePattern      bool
	useCustomPrefix bool
	silent          bool // 是否静默启动
}

// ServiceRegisterFunc gRPC服务注册函数类型
type ServiceRegisterFunc func(*grpc.Server)

// HandlerRegisterFunc HTTP处理器注册函数类型
type HandlerRegisterFunc func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error

// ServerHandlerRegisterFunc 本地Server Handler注册函数类型 (不需要gRPC连接)
type ServerHandlerRegisterFunc func(context.Context, *runtime.ServeMux) error

// NewGateway 创建新的Gateway构建器 - 链式调用API入口
// 使用示例:
//
//	gateway, err := NewGateway().
//	  WithConfigPath("./config.yaml").
//	  WithEnvironment(goconfig.EnvProduction).
//	  BuildAndStart()
func NewGateway() *GatewayBuilder {
	return &GatewayBuilder{
		environment: goconfig.GetEnvironment(),
	}
}

// WithConfigPath 设置配置文件路径 (直接指定文件)
func (b *GatewayBuilder) WithConfigPath(path string) *GatewayBuilder {
	b.configPath = path
	return b
}

// WithSearchPath 设置搜索路径 (用于自动发现)
func (b *GatewayBuilder) WithSearchPath(path string) *GatewayBuilder {
	b.searchPath = path
	b.autoDiscovery = true
	return b
}

// WithEnvironment 设置环境
func (b *GatewayBuilder) WithEnvironment(env goconfig.EnvironmentType) *GatewayBuilder {
	b.environment = env
	return b
}

// WithPrefix 设置配置文件前缀 (例如: "gateway", "app")
func (b *GatewayBuilder) WithPrefix(prefix string) *GatewayBuilder {
	b.configPrefix = prefix
	b.useCustomPrefix = true
	return b
}

// WithPattern 设置文件匹配模式 (例如: "gateway-*.yaml")
func (b *GatewayBuilder) WithPattern(pattern string) *GatewayBuilder {
	b.pattern = pattern
	b.usePattern = true
	return b
}

// WithHotReload 启用热更新 (传nil使用默认配置)
func (b *GatewayBuilder) WithHotReload(config *goconfig.HotReloadConfig) *GatewayBuilder {
	if config == nil {
		config = goconfig.DefaultHotReloadConfig()
	}
	b.hotReloadConfig = config
	return b
}

// WithContext 设置上下文选项
func (b *GatewayBuilder) WithContext(options *goconfig.ContextKeyOptions) *GatewayBuilder {
	b.contextOptions = options
	return b
}

// Silent 设置静默模式 (不显示启动banner)
func (b *GatewayBuilder) Silent() *GatewayBuilder {
	b.silent = true
	return b
}

// Build 构建Gateway (不启动)
func (b *GatewayBuilder) Build() (*Gateway, error) {
	// 确保全局日志器被初始化
	if err := global.EnsureLoggerInitialized(); err != nil {
		return nil, fmt.Errorf("初始化日志器失败: %w", err)
	}

	// 创建配置实例
	config := &gwconfig.Gateway{}

	// 使用go-config创建并启动配置管理器
	var manager *goconfig.IntegratedConfigManager
	var err error

	switch {
	case b.usePattern:
		// 使用模式匹配
		manager, err = goconfig.NewManager(config).
			WithSearchPath(b.searchPath).
			WithPattern(b.pattern).
			WithEnvironment(b.environment).
			WithHotReload(b.hotReloadConfig).
			WithContext(b.contextOptions).
			BuildAndStart()

	case b.useCustomPrefix:
		// 使用自定义前缀发现
		manager, err = goconfig.NewManager(config).
			WithSearchPath(b.searchPath).
			WithPrefix(b.configPrefix).
			WithEnvironment(b.environment).
			WithHotReload(b.hotReloadConfig).
			WithContext(b.contextOptions).
			BuildAndStart()

	case b.autoDiscovery:
		// 自动发现
		manager, err = goconfig.NewManager(config).
			WithSearchPath(b.searchPath).
			WithEnvironment(b.environment).
			WithHotReload(b.hotReloadConfig).
			WithContext(b.contextOptions).
			BuildAndStart()

	case b.configPath != "":
		// 直接使用指定路径
		manager, err = goconfig.NewManager(config).
			WithConfigPath(b.configPath).
			WithEnvironment(b.environment).
			WithHotReload(b.hotReloadConfig).
			WithContext(b.contextOptions).
			BuildAndStart()

	default:
		return nil, errors.ErrInvalidConfiguration
	}

	if err != nil {
		return nil, errors.WrapWithContext(err, errors.ErrCodeInvalidConfiguration)
	}

	// 初始化全局状态
	if err := b.initializeGlobalState(manager, config); err != nil {
		return nil, errors.WrapWithContext(err, errors.ErrCodeInitializationError)
	}

	// 创建Gateway实例
	srv, err := server.NewServer()
	if err != nil {
		return nil, errors.WrapWithContext(err, errors.ErrCodeServerCreationFailed)
	}

	gateway := &Gateway{
		Server:        srv,
		configManager: manager,
		gatewayConfig: config,
	}

	// 注册配置变更回调
	gateway.RegisterConfigCallbacks()

	return gateway, nil
}

// BuildAndStart 构建并启动Gateway (推荐使用)
func (b *GatewayBuilder) BuildAndStart(ctx ...context.Context) (*Gateway, error) {
	gateway, err := b.Build()
	if err != nil {
		return nil, err
	}

	// 启动Gateway
	if b.silent {
		err = gateway.StartSilent()
	} else {
		err = gateway.Start()
	}

	if err != nil {
		return nil, fmt.Errorf("启动Gateway失败: %w", err)
	}

	return gateway, nil
}

// MustBuildAndStart 构建并启动Gateway (失败时panic)
func (b *GatewayBuilder) MustBuildAndStart(ctx ...context.Context) *Gateway {
	gateway, err := b.BuildAndStart(ctx...)
	if err != nil {
		panic(fmt.Sprintf("构建并启动Gateway失败: %v", err))
	}
	return gateway
}

// initializeGlobalState 初始化全局状态
func (b *GatewayBuilder) initializeGlobalState(manager *goconfig.IntegratedConfigManager, config *gwconfig.Gateway) error {
	// 设置全局变量
	global.CONFIG_MANAGER = manager
	global.GATEWAY = config

	// 初始化全局上下文
	global.CTX, global.CANCEL = context.WithCancel(context.Background())

	// 注册全局配置变更回调
	if err := b.registerGlobalConfigCallbacks(manager); err != nil {
		return fmt.Errorf("注册全局配置回调失败: %w", err)
	}

	// 初始化其他组件
	if err := b.initializeComponents(); err != nil {
		return fmt.Errorf("初始化组件失败: %w", err)
	}

	return nil
}

// registerGlobalConfigCallbacks 注册全局配置变更回调
func (b *GatewayBuilder) registerGlobalConfigCallbacks(manager *goconfig.IntegratedConfigManager) error {
	// 注册配置变更回调
	err := manager.RegisterConfigCallback(func(ctx context.Context, event goconfig.CallbackEvent) error {
		if newConfig, ok := event.NewValue.(*gwconfig.Gateway); ok {
			global.LOGGER.Info("📋 配置已更新: %s\n", newConfig.Name)
			global.GATEWAY = newConfig

			// 重新初始化日志器（如果日志配置发生变化）
			loggerInit := &global.LoggerInitializer{}
			if err := loggerInit.Initialize(ctx, newConfig); err != nil {
				global.LOGGER.Error("❌ 重新初始化日志器失败: %v\n", err)
			}

			global.LOGGER.Info("🔄 配置热更新完成\n")
		}
		return nil
	}, goconfig.CallbackOptions{
		ID:       "gateway_config_handler",
		Types:    []goconfig.CallbackType{goconfig.CallbackTypeConfigChanged},
		Priority: goconfig.CallbackPriorityHigh,
		Async:    false,
		Timeout:  5 * time.Second,
	})

	if err != nil {
		return fmt.Errorf("注册配置变更回调失败: %w", err)
	}

	// 注册环境变更回调
	err = manager.RegisterEnvironmentCallback("gateway_env_handler",
		func(oldEnv, newEnv goconfig.EnvironmentType) error {
			global.LOGGER.Info("🌍 环境变更: %s -> %s\n", oldEnv, newEnv)
			return nil
		}, goconfig.CallbackPriorityHigh, false)

	if err != nil {
		return fmt.Errorf("注册环境变更回调失败: %w", err)
	}

	return nil
}

// initializeComponents 初始化其他组件 - 使用统一的InitializerChain
func (b *GatewayBuilder) initializeComponents() error {
	// 创建并使用默认初始化链
	chain := global.GetDefaultInitializerChain()

	ctx, cancel := context.WithTimeout(global.CTX, 30*time.Second)
	defer cancel()

	return chain.InitializeAll(ctx, global.GATEWAY)
}

// 注意：initializeLogger, initializeSnowflakeNode, initializePoolManager 和 bindPoolResourcesToGlobal
// 已被统一的 InitializerChain 替代，具体实现请参见 global/initializer.go

// RegisterService 注册gRPC服务
func (g *Gateway) RegisterService(registerFunc ServiceRegisterFunc) {
	grpcAddr := "unknown"
	if g.gatewayConfig != nil && g.gatewayConfig.GRPC != nil && g.gatewayConfig.GRPC.Server != nil {
		grpcAddr = g.gatewayConfig.GRPC.Server.GetEndpoint()
	}
	fmt.Printf("🔷 注册 gRPC 服务: %s\n", grpcAddr)
	g.Server.RegisterGRPCService(registerFunc)
	g.registeredGRPCServices = append(g.registeredGRPCServices, grpcAddr)
}

// RegisterGatewayHandler 注册gRPC-Gateway处理器 (本地调用方式)
// 使用示例:
//
//	g.RegisterGatewayHandler(func(ctx context.Context, mux *runtime.ServeMux) error {
//	    return agentsettingsApis.RegisterAgentSettingsServiceHandlerServer(ctx, mux, svc)
//	})
func (g *Gateway) RegisterGatewayHandler(registerFunc ServerHandlerRegisterFunc) error {
	httpAddr := "unknown"
	if g.gatewayConfig != nil && g.gatewayConfig.HTTPServer != nil {
		httpAddr = g.gatewayConfig.HTTPServer.GetEndpoint()
	}
	fmt.Printf("🌐 注册 gRPC-Gateway 处理器: %s (本地模式)\n", httpAddr)
	gwMux := g.GetGatewayMux()
	if err := registerFunc(global.CTX, gwMux); err != nil {
		fmt.Printf("❌ 注册失败: %v\n", err)
		global.LOGGER.ErrorKV("注册gRPC-Gateway HTTP处理器失败", "error", err)
		return err
	}
	g.registeredGatewayHandlers = append(g.registeredGatewayHandlers, "gRPC-Gateway@"+httpAddr)
	return nil
}

// RegisterHandler 注册HTTP处理器
func (g *Gateway) RegisterHandler(pattern string, handler http.Handler) {
	fmt.Printf("🔗 注册 HTTP 处理器: %s\n", pattern)
	g.Server.RegisterHTTPRoute(pattern, handler)
	g.registeredHTTPRoutes = append(g.registeredHTTPRoutes, pattern)
}

// RegisterHTTPRoute 注册HTTP路由 (便捷方法)
func (g *Gateway) RegisterHTTPRoute(pattern string, handlerFunc http.HandlerFunc) {
	fmt.Printf("🔗 注册 HTTP 路由: %s\n", pattern)
	g.Server.RegisterHTTPRoute(pattern, handlerFunc)
	g.registeredHTTPRoutes = append(g.registeredHTTPRoutes, pattern)
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
	global.LOGGER.InfoKV("启用功能", "feature", feature)
	if err := g.Server.EnableFeature(feature); err != nil {
		global.LOGGER.ErrorKV("❌ 启用功能失败", "feature", feature, "error", err)
		return err
	}
	global.LOGGER.InfoKV("✅ 功能启用成功", "feature", feature)
	return nil
}

// EnableFeatureWithConfig 使用自定义配置启用功能（通用接口）
func (g *Gateway) EnableFeatureWithConfig(feature server.FeatureType, config interface{}) error {
	global.LOGGER.InfoKV("使用自定义配置启用功能", "feature", feature)
	if err := g.Server.EnableFeatureWithConfig(feature, config); err != nil {
		global.LOGGER.ErrorKV("❌ 使用自定义配置启用功能失败", "feature", feature, "error", err)
		return err
	}
	global.LOGGER.InfoKV("✅ 功能启用成功(自定义配置)", "feature", feature)
	return nil
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
	// 创建并使用启动状态报告器
	startupReporter := server.NewStartupReporter(g.configManager.GetConfig())

	// 打印启动时间戳
	startupReporter.PrintStartupTimestamp()

	// 打印详细的启动状态检查
	startupReporter.PrintStartupStatus()

	// 默认启用Swagger文档服务
	if g.gatewayConfig != nil && g.gatewayConfig.Swagger != nil && g.gatewayConfig.Swagger.Enabled {
		// 直接传递Swagger配置指针
		if err := g.EnableSwaggerWithConfig(g.gatewayConfig.Swagger); err != nil {
			global.LOGGER.Warn("⚠️  启用Swagger失败: %v", err)
		} else {
			global.LOGGER.Info("✅ Swagger已成功启用: %s", g.gatewayConfig.Swagger.UIPath)
		}
	} else {
		// 如果配置中没有Swagger配置，使用默认配置
		if err := g.EnableSwagger(); err != nil {
			global.LOGGER.Warn("⚠️  使用默认配置启用Swagger失败: %v", err)
		} else {
			global.LOGGER.Info("✅ 使用默认配置启用Swagger成功")
		}
	}

	// 启动服务
	if err := g.Server.Start(); err != nil {
		fmt.Printf("启动网关失败: %v\n", err)
		return err
	}

	// 显示启动banner和启动摘要
	g.PrintStartupInfo()
	startupReporter.PrintStartupSummary()

	return nil
}

// Stop 停止网关服务
func (g *Gateway) Stop() error {
	global.LOGGER.Info("🛑 开始停止网关服务...")

	// 先停止服务器
	if err := g.Server.Stop(); err != nil {
		global.LOGGER.ErrorKV("❌ 停止服务器失败", "error", err)
		return err
	}
	global.LOGGER.Info("✅ 服务器已停止")

	// 再停止配置管理器
	if g.configManager != nil {
		global.LOGGER.Info("停止配置管理器...")
		g.configManager.Stop()
		global.LOGGER.Info("✅ 配置管理器已停止")
	}

	global.LOGGER.Info("✅ 网关服务已完全停止")
	return nil
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

// PrintAPIRegistrationSummary 打印API注册汇总信息
func (g *Gateway) PrintAPIRegistrationSummary() {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📋 API 注册汇总 (API Registration Summary)")
	fmt.Println(strings.Repeat("=", 80))

	// gRPC 服务统计
	fmt.Printf("\n🔷 gRPC Services: %d\n", len(g.registeredGRPCServices))
	if len(g.registeredGRPCServices) > 0 {
		for i, svc := range g.registeredGRPCServices {
			fmt.Printf("  %d. %s\n", i+1, svc)
		}
	} else {
		fmt.Println("  (无注册服务)")
	}

	// gRPC-Gateway 处理器统计
	fmt.Printf("\n🌐 gRPC-Gateway Handlers: %d\n", len(g.registeredGatewayHandlers))
	if len(g.registeredGatewayHandlers) > 0 {
		for i, handler := range g.registeredGatewayHandlers {
			fmt.Printf("  %d. %s\n", i+1, handler)
		}
	} else {
		fmt.Println("  (无注册处理器)")
	}

	// HTTP 路由统计
	fmt.Printf("\n🔗 HTTP Routes: %d\n", len(g.registeredHTTPRoutes))
	if len(g.registeredHTTPRoutes) > 0 {
		for i, route := range g.registeredHTTPRoutes {
			fmt.Printf("  %d. %s\n", i+1, route)
		}
	} else {
		fmt.Println("  (无注册路由)")
	}

	// 总计
	totalAPIs := len(g.registeredGRPCServices) + len(g.registeredGatewayHandlers) + len(g.registeredHTTPRoutes)
	fmt.Printf("\n✅ 总计注册 API 数量: %d\n", totalAPIs)
	fmt.Println(strings.Repeat("=", 80) + "\n")
}

// GetGatewayConfig 获取网关配置
func (g *Gateway) GetGatewayConfig() *gwconfig.Gateway {
	return g.gatewayConfig
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

// ================ 连接池管理方法 ================

// GetPoolManager 获取连接池管理器
func (g *Gateway) GetPoolManager() cpool.PoolManager {
	return g.Server.GetPoolManager()
}

// GetDB 获取数据库连接
func (g *Gateway) GetDB() *gorm.DB {
	if poolManager := g.GetPoolManager(); poolManager != nil {
		return poolManager.GetDB()
	}
	return nil
}

// GetRedis 获取Redis客户端
func (g *Gateway) GetRedis() *redis.Client {
	if poolManager := g.GetPoolManager(); poolManager != nil {
		return poolManager.GetRedis()
	}
	return nil
}

// GetMinIO 获取MinIO客户端
func (g *Gateway) GetMinIO() *minio.Client {
	if poolManager := g.GetPoolManager(); poolManager != nil {
		return poolManager.GetMinIO()
	}
	return nil
}

// GetSnowflake 获取雪花ID生成器
func (g *Gateway) GetSnowflake() *snowflake.Node {
	if poolManager := g.GetPoolManager(); poolManager != nil {
		return poolManager.GetSnowflake()
	}
	return nil
}

// HealthCheck 获取所有连接的健康状态
func (g *Gateway) HealthCheck() map[string]bool {
	if poolManager := g.GetPoolManager(); poolManager != nil {
		return poolManager.HealthCheck()
	}
	return make(map[string]bool)
}

// ===============================================================================
// 便捷启动方法 - 一键启动Gateway
// ===============================================================================

// QuickStart 快速启动Gateway - 使用默认配置路径和自动发现
func QuickStart(configPath ...string) error {
	path := "./config"
	if len(configPath) > 0 {
		path = configPath[0]
	}

	gw, err := NewGateway().
		WithSearchPath(path).
		WithEnvironment(goconfig.GetEnvironment()).
		WithHotReload(nil).
		BuildAndStart()

	if err != nil {
		return fmt.Errorf("快速启动失败: %w", err)
	}

	// 等待关闭信号
	return gw.WaitForShutdown()
}

// QuickStartWithConfigFile 使用指定配置文件快速启动
func QuickStartWithConfigFile(configFilePath string) error {
	gw, err := NewGateway().
		WithConfigPath(configFilePath).
		WithEnvironment(goconfig.GetEnvironment()).
		WithHotReload(nil).
		BuildAndStart()

	if err != nil {
		return fmt.Errorf("启动失败: %w", err)
	}

	return gw.WaitForShutdown()
}

// QuickStartWithConfigFilePerfix 使用指定配置文件快速启动
func QuickStartWithConfigFilePerfix(configFilePath string, perfix string) error {
	gw, err := NewGateway().
		WithConfigPath(configFilePath).
		WithPrefix(perfix).
		WithEnvironment(goconfig.GetEnvironment()).
		WithHotReload(nil).
		BuildAndStart()

	if err != nil {
		return fmt.Errorf("启动失败: %w", err)
	}

	return gw.WaitForShutdown()
}

// WaitForShutdown 等待关闭信号并优雅关闭
func (g *Gateway) WaitForShutdown() error {
	// 设置信号处理
	g.setupGracefulShutdown()

	// 阻塞等待
	select {}
}

// setupGracefulShutdown 设置优雅关闭信号处理
func (g *Gateway) setupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-c
		fmt.Printf("\n🛑 收到信号 %v，开始优雅关闭...\n", sig)

		// 显示关闭信息
		g.PrintShutdownInfo()

		// 停止服务
		if err := g.Stop(); err != nil {
			fmt.Printf("❌ 停止服务时发生错误: %v\n", err)
		}

		// 显示关闭完成信息
		g.PrintShutdownComplete()

		os.Exit(0)
	}()
}

// ===============================================================================
// WebSocket 相关便捷方法
// ===============================================================================

// GetWebSocketService 获取 WebSocket 服务实例
func (g *Gateway) GetWebSocketService() *server.WebSocketService {
	if g.Server == nil {
		return nil
	}
	return g.Server.GetWebSocketService()
}

// IsWebSocketEnabled 检查 WebSocket 是否启用
func (g *Gateway) IsWebSocketEnabled() bool {
	wsSvc := g.GetWebSocketService()
	return wsSvc != nil && wsSvc.IsRunning()
}

// OnWebSocketClientConnect 添加客户端连接回调（支持链式调用）
// 示例:
//
//	gateway.
//	  OnWebSocketClientConnect(func(ctx context.Context, client *wsc.Client) error {
//	    fmt.Printf("客户端已连接: %s\n", client.ID)
//	    return nil
//	  }).
//	  OnWebSocketClientDisconnect(func(ctx context.Context, client *wsc.Client, reason string) error {
//	    fmt.Printf("客户端已断开: %s (原因: %s)\n", client.ID, reason)
//	    return nil
//	  })
func (g *Gateway) OnWebSocketClientConnect(cb server.ClientConnectCallback) *Gateway {
	wsSvc := g.GetWebSocketService()
	if wsSvc != nil {
		wsSvc.OnClientConnect(cb)
	}
	return g
}

// OnWebSocketClientDisconnect 添加客户端断开连接回调
func (g *Gateway) OnWebSocketClientDisconnect(cb server.ClientDisconnectCallback) *Gateway {
	wsSvc := g.GetWebSocketService()
	if wsSvc != nil {
		wsSvc.OnClientDisconnect(cb)
	}
	return g
}

// OnWebSocketMessageReceived 添加消息接收回调
func (g *Gateway) OnWebSocketMessageReceived(cb server.MessageReceivedCallback) *Gateway {
	wsSvc := g.GetWebSocketService()
	if wsSvc != nil {
		wsSvc.OnMessageReceived(cb)
	}
	return g
}

// OnWebSocketError 添加错误处理回调
func (g *Gateway) OnWebSocketError(cb server.ErrorCallback) *Gateway {
	wsSvc := g.GetWebSocketService()
	if wsSvc != nil {
		wsSvc.OnError(cb)
	}
	return g
}

// ============================================================================
// WebSocket 消息推送 API - 直接暴露 Hub 能力
// ============================================================================

// SendToWebSocketUser 发送消息给特定用户
// 使用示例:
//
//	msg := &wsc.HubMessage{
//	  From: "admin",
//	  Content: []byte("Hello"),
//	}
//	if err := gateway.SendToWebSocketUser(ctx, "user123", msg); err != nil {
//	  log.Printf("Failed to send message: %v", err)
//	}
func (g *Gateway) SendToWebSocketUser(ctx context.Context, userID string, msg *wsc.HubMessage) error {
	wsSvc := g.GetWebSocketService()
	if wsSvc == nil {
		return errors.NewError(errors.ErrCodeServiceUnavailable, "WebSocket service not available")
	}
	return wsSvc.SendToUser(ctx, userID, msg)
}

// SendToWebSocketUserWithAck 发送消息给用户（带 ACK）
// 示例:
//
//	ack, err := gateway.SendToWebSocketUserWithAck(ctx, "user123", msg, 5*time.Second, 3)
//	if err != nil {
//	  log.Printf("Failed to send with ACK: %v", err)
//	} else {
//	  log.Printf("Message delivered successfully")
//	}
func (g *Gateway) SendToWebSocketUserWithAck(ctx context.Context, userID string, msg *wsc.HubMessage, timeout time.Duration, maxRetry int) (*wsc.AckMessage, error) {
	wsSvc := g.GetWebSocketService()
	if wsSvc == nil {
		return nil, errors.NewError(errors.ErrCodeServiceUnavailable, "WebSocket service not available")
	}
	return wsSvc.SendToUserWithAck(ctx, userID, msg, timeout, maxRetry)
}

// SendToWebSocketTicket 发送消息给特定凭证 ID
func (g *Gateway) SendToWebSocketTicket(ctx context.Context, ticketID string, msg *wsc.HubMessage) error {
	wsSvc := g.GetWebSocketService()
	if wsSvc == nil {
		return errors.NewError(errors.ErrCodeServiceUnavailable, "WebSocket service not available")
	}
	return wsSvc.SendToTicket(ctx, ticketID, msg)
}

// SendToWebSocketTicketWithAck 发送消息给凭证（带 ACK）
func (g *Gateway) SendToWebSocketTicketWithAck(ctx context.Context, ticketID string, msg *wsc.HubMessage, timeout time.Duration, maxRetry int) (*wsc.AckMessage, error) {
	wsSvc := g.GetWebSocketService()
	if wsSvc == nil {
		return nil, errors.NewError(errors.ErrCodeServiceUnavailable, "WebSocket service not available")
	}
	return wsSvc.SendToTicketWithAck(ctx, ticketID, msg, timeout, maxRetry)
}

// BroadcastWebSocketMessage 广播消息给所有连接的客户端
// 使用示例:
//
//	msg := &wsc.HubMessage{
//	  From: "admin",
//	  Content: []byte("Server announcement"),
//	}
//	gateway.BroadcastWebSocketMessage(ctx, msg)
func (g *Gateway) BroadcastWebSocketMessage(ctx context.Context, msg *wsc.HubMessage) {
	wsSvc := g.GetWebSocketService()
	if wsSvc != nil && msg != nil {
		wsSvc.Broadcast(ctx, msg)
	}
}

// GetWebSocketOnlineUsers 获取所有在线用户列表
func (g *Gateway) GetWebSocketOnlineUsers() []string {
	wsSvc := g.GetWebSocketService()
	if wsSvc == nil {
		return []string{}
	}
	return wsSvc.GetOnlineUsers()
}

// GetWebSocketOnlineUserCount 获取在线用户数量
func (g *Gateway) GetWebSocketOnlineUserCount() int {
	wsSvc := g.GetWebSocketService()
	if wsSvc == nil {
		return 0
	}
	return wsSvc.GetOnlineUserCount()
}

// GetWebSocketStats 获取 WebSocket 统计信息
// 返回包含以下信息的映射:
// - online_users: 当前在线用户数
// - is_running: 服务是否运行中
// - uptime_seconds: 服务运行时间（秒）
// - total_messages_sent: 总发送消息数
// - total_messages_recv: 总接收消息数
func (g *Gateway) GetWebSocketStats() map[string]interface{} {
	wsSvc := g.GetWebSocketService()
	if wsSvc == nil {
		return map[string]interface{}{
			"online_users":   0,
			"is_running":     false,
			"uptime_seconds": 0,
		}
	}
	return wsSvc.GetStats()
}
