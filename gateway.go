/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 11:38:38
 * @FilePath: \go-rpc-gateway\gateway.go
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
	"syscall"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	goconfig "github.com/kamalyes/go-config"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/cpool"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/server"
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
		return nil, fmt.Errorf("未指定配置路径或搜索选项")
	}

	if err != nil {
		return nil, fmt.Errorf("创建配置管理器失败: %w", err)
	}

	// 初始化全局状态
	if err := b.initializeGlobalState(manager, config); err != nil {
		return nil, fmt.Errorf("初始化全局状态失败: %w", err)
	}

	// 创建Gateway实例
	srv, err := server.NewServer()
	if err != nil {
		return nil, fmt.Errorf("创建服务器失败: %w", err)
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
			if err := initializeLogger(); err != nil {
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

// initializeComponents 初始化其他组件
func (b *GatewayBuilder) initializeComponents() error {
	// 初始化日志器
	if err := initializeLogger(); err != nil {
		return fmt.Errorf("初始化日志器失败: %w", err)
	}

	// 初始化连接池管理器
	if err := initializePoolManager(); err != nil {
		return fmt.Errorf("初始化连接池管理器失败: %w", err)
	}

	// 初始化Snowflake节点（用于分布式ID生成）
	if err := initializeSnowflakeNode(); err != nil {
		return fmt.Errorf("初始化Snowflake节点失败: %w", err)
	}

	// 从pool manager中绑定全局资源
	if err := bindPoolResourcesToGlobal(); err != nil {
		return fmt.Errorf("绑定池资源到全局失败: %w", err)
	}

	return nil
}

// initializeLogger 初始化日志器
func initializeLogger() error {
	if global.GATEWAY == nil {
		return fmt.Errorf("GATEWAY 配置为空")
	}

	// 根据配置设置日志级别
	level := logger.INFO
	if global.GATEWAY.Debug {
		level = logger.DEBUG
	}

	// 如果已存在日志器，更新级别；否则创建新的
	if global.LOGGER != nil {
		global.LOGGER.Info("🔄 更新日志器配置: level=%s, debug=%t\n", level.String(), global.GATEWAY.Debug)
	} else {
		// 创建新的日志器
		global.LOGGER = logger.CreateSimpleLogger(level)
		if global.LOGGER == nil {
			return fmt.Errorf("创建日志器失败")
		}
		global.LOG = global.LOGGER // 兼容别名
		global.LOGGER.Info("📝 日志器初始化完成: level=%s, debug=%t\n", level.String(), global.GATEWAY.Debug)
	}

	return nil
}

// initializeSnowflakeNode 初始化Snowflake节点用于分布式ID生成
func initializeSnowflakeNode() error {
	// 使用节点ID 1（可以从配置中读取）
	var err error
	global.Node, err = snowflake.NewNode(1)
	if err != nil {
		return fmt.Errorf("创建Snowflake节点失败: %w", err)
	}
	global.LOGGER.Info("❄️  Snowflake节点初始化完成\n")
	return nil
}

// initializePoolManager 初始化连接池管理器及其所有资源
func initializePoolManager() error {
	if global.GATEWAY == nil {
		return fmt.Errorf("GATEWAY 配置为空")
	}

	if global.LOGGER == nil {
		return fmt.Errorf("LOGGER 未初始化")
	}

	// 创建连接池管理器（注入 logger）
	manager := cpool.NewManager(global.LOGGER)

	// 初始化 Manager（这会初始化所有连接池）
	if err := manager.Initialize(global.CTX, global.GATEWAY); err != nil {
		return fmt.Errorf("初始化 Pool Manager 失败: %w", err)
	}

	// 将 Manager 的资源绑定到全局变量
	if db := manager.GetDB(); db != nil {
		global.DB = db
	}
	if rdb := manager.GetRedis(); rdb != nil {
		global.REDIS = rdb
	}
	if minio := manager.GetMinIO(); minio != nil {
		global.MinIO = minio
	}
	if node := manager.GetSnowflake(); node != nil {
		global.Node = node
	}

	global.POOL_MANAGER = manager
	global.LOGGER.Info("✅ 连接池管理器初始化完成\n")

	return nil
}

// bindPoolResourcesToGlobal 从连接池管理器绑定资源到全局变量
func bindPoolResourcesToGlobal() error {
	if global.POOL_MANAGER == nil {
		return fmt.Errorf("连接池管理器未初始化")
	}

	// 资源已在 initializePoolManager 中直接绑定到全局变量
	// 这里只需确保它们是否已绑定
	if global.DB == nil {
		global.DB = global.POOL_MANAGER.GetDB()
	}
	if global.REDIS == nil {
		global.REDIS = global.POOL_MANAGER.GetRedis()
	}
	if global.MinIO == nil {
		global.MinIO = global.POOL_MANAGER.GetMinIO()
	}

	return nil
}

// RegisterService 注册gRPC服务
func (g *Gateway) RegisterService(registerFunc ServiceRegisterFunc) {
	g.Server.RegisterGRPCService(registerFunc)
}

// RegisterHandler 注册HTTP处理器
func (g *Gateway) RegisterHandler(pattern string, handler http.Handler) {
	g.Server.RegisterHTTPRoute(pattern, handler)
}

// RegisterHTTPRoute 注册HTTP路由 (便捷方法)
func (g *Gateway) RegisterHTTPRoute(pattern string, handlerFunc http.HandlerFunc) {
	g.Server.RegisterHTTPRoute(pattern, handlerFunc)
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
	return g.Server.EnableFeature(feature)
}

// EnableFeatureWithConfig 使用自定义配置启用功能（通用接口）
func (g *Gateway) EnableFeatureWithConfig(feature server.FeatureType, config interface{}) error {
	return g.Server.EnableFeatureWithConfig(feature, config)
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
		return err
	}

	// 显示启动banner
	g.PrintStartupInfo()
	return nil
}

// Stop 停止网关服务
func (g *Gateway) Stop() error {
	// 先停止服务器
	err := g.Server.Stop()

	// 再停止配置管理器
	if g.configManager != nil {
		g.configManager.Stop()
	}

	return err
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
