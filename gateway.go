/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-20 19:32:31
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
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/kamalyes/go-rpc-gateway/config"
	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/cpool"
	grpcpool "github.com/kamalyes/go-rpc-gateway/cpool/grpc"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/kamalyes/go-rpc-gateway/server"
	"github.com/kamalyes/go-toolbox/pkg/safe"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"gorm.io/gorm"
)

// Gateway 是主要的网关服务器
type Gateway struct {
	*server.Server
	configManager *config.IntegratedConfigManager
	gatewayConfig *gwconfig.Gateway
	ctx           context.Context // Gateway 上下文，用于日志和其他操作

	// API 注册信息收集
	registeredGRPCServices    []string
	registeredGatewayHandlers []string
	registeredHTTPRoutes      []string
	grpcServiceRegistrars     []ServiceRegisterFunc
	gatewayHandlerRegistrars  []ServerHandlerRegisterFunc
	proxyHandlerRegistrations []proxyHandlerRegistration
	httpRouteRegistrations    []httpRouteRegistration

	// 应用级关闭回调，在网关 Stop() 之前执行（如 flush 异步写入器）
	shutdownCallbacks []func()

	// 下游服务发版监视
	serviceWatcher       *grpcpool.ServiceWatcher
	watchInterval        time.Duration
	watchIntervalSetOnce bool
}

// GatewayBuilder Gateway构建器 - 支持链式调用
type GatewayBuilder struct {
	configPath             string
	searchPath             string
	environment            config.EnvironmentType
	configPrefix           string
	pattern                string
	hotReloadConfig        *config.HotReloadConfig
	contextOptions         *config.ContextKeyOptions
	autoDiscovery          bool
	usePattern             bool
	useCustomPrefix        bool
	silent                 bool // 是否静默启动
	grpcGatewayMiddlewares []runtime.Middleware
	ctx                    context.Context // 用户提供的上下文
}

// ServiceRegisterFunc gRPC服务注册函数类型
type ServiceRegisterFunc func(*grpc.Server)

// HandlerRegisterFunc HTTP处理器注册函数类型（基于 endpoint + dialOpts）
type HandlerRegisterFunc func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error

// ConnHandlerRegisterFunc HTTP处理器注册函数类型（基于 *grpc.ClientConn）
type ConnHandlerRegisterFunc func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error

// ServerHandlerRegisterFunc 本地Server Handler注册函数类型 (不需要gRPC连接)
type ServerHandlerRegisterFunc func(context.Context, *runtime.ServeMux) error

type httpRouteRegistration struct {
	pattern string
	handler http.Handler
}

// proxyHandlerRegistration 代理处理器注册信息（远程调用方式）
type proxyHandlerRegistration struct {
	// 基于 endpoint + dialOpts 的注册方式
	registerFunc HandlerRegisterFunc
	endpoint     string
	dialOpts     []grpc.DialOption

	// 基于 conn 的注册方式（共享连接）
	connRegisterFunc ConnHandlerRegisterFunc
	conn             *grpc.ClientConn
	serviceName      string
	clientCfg        *gwconfig.GRPCClient
}

// NewGateway 创建新的Gateway构建器 - 链式调用API入口
// 使用示例:
//
//	gateway, err := NewGateway().
//	  WithContext(ctx).
//	  WithConfigPath("./config.yaml").
//	  WithEnvironment(config.EnvProduction).
//	  BuildAndStart()
func NewGateway() *GatewayBuilder {
	return &GatewayBuilder{
		environment: config.GetEnvironment(),
		ctx:         context.Background(),
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
func (b *GatewayBuilder) WithEnvironment(env config.EnvironmentType) *GatewayBuilder {
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
func (b *GatewayBuilder) WithHotReload(reloadCfg *config.HotReloadConfig) *GatewayBuilder {
	if reloadCfg == nil {
		reloadCfg = config.DefaultHotReloadConfig()
	}
	b.hotReloadConfig = reloadCfg
	return b
}

// WithContext 设置上下文
func (b *GatewayBuilder) WithContext(ctx context.Context) *GatewayBuilder {
	if ctx != nil {
		b.ctx = ctx
	}
	return b
}

// WithContextOptions 设置上下文选项
func (b *GatewayBuilder) WithContextOptions(options *config.ContextKeyOptions) *GatewayBuilder {
	b.contextOptions = options
	return b
}

// Silent 设置静默模式 (不显示启动banner)
func (b *GatewayBuilder) Silent() *GatewayBuilder {
	b.silent = true
	return b
}

// WithGrpcGatewayMiddleware 添加 gRPC-Gateway 中间件 (可多次调用)
func (b *GatewayBuilder) WithGrpcGatewayMiddleware(mw runtime.Middleware) *GatewayBuilder {
	b.grpcGatewayMiddlewares = append(b.grpcGatewayMiddlewares, mw)
	return b
}

// Build 构建Gateway (不启动)
func (b *GatewayBuilder) Build() (*Gateway, error) {
	// 首先初始化一个临时 logger，用于记录配置加载过程
	if err := global.EnsureLoggerInitialized(); err != nil {
		return nil, errors.NewError(errors.ErrCodeInitializationError, errors.FormatInitError("日志器", err))
	}

	// 创建配置实例：先放入默认值，再让配置文件覆盖
	// 这样嵌套的数据库配置不会在后续初始化时退回到框架默认库名
	gwCfg := gwconfig.Default()

	// 使用go-config创建并启动配置管理器
	var manager *config.IntegratedConfigManager
	var err error

	switch {
	case b.usePattern:
		// 使用模式匹配
		manager, err = config.NewManager(gwCfg).
			WithSearchPath(b.searchPath).
			WithPattern(b.pattern).
			WithEnvironment(b.environment).
			WithHotReload(b.hotReloadConfig).
			WithContext(b.contextOptions).
			BuildAndStart()

	case b.useCustomPrefix:
		// 使用自定义前缀发现
		manager, err = config.NewManager(gwCfg).
			WithSearchPath(b.searchPath).
			WithPrefix(b.configPrefix).
			WithEnvironment(b.environment).
			WithHotReload(b.hotReloadConfig).
			WithContext(b.contextOptions).
			BuildAndStart()

	case b.autoDiscovery:
		// 自动发现
		manager, err = config.NewManager(gwCfg).
			WithSearchPath(b.searchPath).
			WithEnvironment(b.environment).
			WithHotReload(b.hotReloadConfig).
			WithContext(b.contextOptions).
			BuildAndStart()

	case b.configPath != "":
		// 直接使用指定路径
		manager, err = config.NewManager(gwCfg).
			WithConfigPath(b.configPath).
			WithEnvironment(b.environment).
			WithHotReload(b.hotReloadConfig).
			WithContext(b.contextOptions).
			BuildAndStart()

	default:
		return nil, errors.ErrInvalidConfiguration
	}

	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInvalidConfiguration)
	}

	// go-config 初始加载使用 viper 默认反序列化；这里再走一次带弱类型和
	// kebab-case 兼容的反序列化，确保 db-name、max-open-conns 等字段完整覆盖默认值
	if err := config.UnmarshalWithFlexibleNaming(manager.GetViper(), gwCfg); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInvalidConfiguration)
	}

	if err := b.initializeGlobalState(manager, &gwCfg); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInitializationError)
	}

	srv, err := server.NewServer()
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeServerCreationFailed)
	}

	// 添加构建器中配置的 gRPC-Gateway 中间件
	for _, mw := range b.grpcGatewayMiddlewares {
		srv.AddGrpcGatewayMiddleware(mw)
	}

	gateway := &Gateway{
		Server:        srv,
		configManager: manager,
		gatewayConfig: gwCfg,
		ctx:           b.ctx,
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
		return nil, errors.NewError(errors.ErrCodeOperationFailed, errors.FormatStartupError("Gateway", err))
	}

	return gateway, nil
}

// MustBuildAndStart 构建并启动Gateway (失败时panic)
func (b *GatewayBuilder) MustBuildAndStart(ctx ...context.Context) *Gateway {
	gateway, err := b.BuildAndStart(ctx...)
	if err != nil {
		panic(errors.FormatPanicError("构建并启动Gateway", err))
	}
	return gateway
}

// mergeGatewayConfigWithDefaults 合并默认配置并刷新派生字段
func mergeGatewayConfigWithDefaults(config *gwconfig.Gateway) *gwconfig.Gateway {
	merged := safe.MergeWithDefaults(config, gwconfig.Default())
	refreshGatewayDerivedFields(merged)
	return merged
}

// refreshGatewayDerivedFields 刷新派生字段
func refreshGatewayDerivedFields(config *gwconfig.Gateway) {
	if config == nil {
		return
	}
	if config.HTTPServer != nil {
		_ = config.HTTPServer.AfterLoad()
	}
	if config.GRPC != nil && config.GRPC.Server != nil {
		_ = config.GRPC.Server.AfterLoad()
	}
	for _, l := range config.Listeners {
		if l != nil {
			_ = l.AfterLoad()
		}
	}
}

// initializeGlobalState 初始化全局状态
func (b *GatewayBuilder) initializeGlobalState(manager *config.IntegratedConfigManager, config **gwconfig.Gateway) error {
	// 使用 safe.MergeWithDefaults 合并默认配置
	*config = mergeGatewayConfigWithDefaults(*config)

	// 设置全局变量
	global.CONFIG_MANAGER = manager
	global.SetGatewayConfig(*config)

	// 初始化全局上下文
	global.CTX, global.CANCEL = context.WithCancel(context.Background())

	// 注册全局配置变更回调
	if err := b.registerGlobalConfigCallbacks(manager); err != nil {
		return errors.NewError(errors.ErrCodeInvalidConfiguration, errors.FormatConfigError("注册全局配置回调", err))
	}

	// 初始化其他组件
	if err := b.initializeComponents(); err != nil {
		return errors.NewError(errors.ErrCodeInitializationError, errors.FormatInitError("组件", err))
	}

	return nil
}

// registerGlobalConfigCallbacks 注册全局配置变更回调
func (b *GatewayBuilder) registerGlobalConfigCallbacks(manager *config.IntegratedConfigManager) error {
	// 注册配置变更回调
	err := manager.RegisterConfigCallback(func(ctx context.Context, event config.CallbackEvent) error {
		if newConfig, ok := event.NewValue.(*gwconfig.Gateway); ok {
			// 合并默认配置
			newConfig = mergeGatewayConfigWithDefaults(newConfig)
			global.LOGGER.InfoContext(b.Context(), "📋 配置已更新: %s", newConfig.Name)
			global.SetGatewayConfig(newConfig)

			// 重新初始化日志器（如果日志配置发生变化）
			loggerInit := &global.LoggerInitializer{}
			if err := loggerInit.Initialize(b.Context(), newConfig); err != nil {
				global.LOGGER.ErrorContext(b.Context(), "❌ 重新初始化日志器失败: %v", err)
			}

			global.LOGGER.InfoContext(b.Context(), "🔄 配置热更新完成")
		}
		return nil
	}, config.CallbackOptions{
		ID:       "gateway_global_config_handler",
		Types:    []config.CallbackType{config.CallbackTypeConfigChanged},
		Priority: -100, // 高优先级（负数表示优先）
		Async:    false,
		Timeout:  5 * time.Second,
	})

	if err != nil {
		return errors.NewError(errors.ErrCodeInvalidConfiguration, errors.FormatConfigError("注册配置变更回调", err))
	}

	// 注册环境变更回调
	err = manager.RegisterEnvironmentCallback("gateway_global_env_handler",
		func(oldEnv, newEnv config.EnvironmentType) error {
			global.LOGGER.InfoContext(b.Context(), "🌍 环境变更: %s -> %s", oldEnv, newEnv)
			return nil
		}, -100, false) // 高优先级

	if err != nil {
		return errors.NewError(errors.ErrCodeInvalidConfiguration, errors.FormatConfigError("注册环境变更回调", err))
	}

	return nil
}

// initializeComponents 初始化其他组件 - 使用统一的InitializerChain
func (b *GatewayBuilder) initializeComponents() error {
	// 创建并使用默认初始化链
	chain := global.GetDefaultInitializerChain()

	ctx, cancel := context.WithTimeout(global.CTX, 30*time.Second)
	defer cancel()

	return chain.InitializeAll(ctx, global.GetGatewayConfig())
}

// RegisterService 注册gRPC服务
func (g *Gateway) RegisterService(registerFunc ServiceRegisterFunc) {
	grpcAddr := g.gatewayConfig.GRPC.Server.GetEndpoint()
	global.LOGGER.InfoContext(g.Context(), "开始注册gRPC服务")
	g.Server.RegisterGRPCService(registerFunc)
	g.grpcServiceRegistrars = append(g.grpcServiceRegistrars, registerFunc)
	g.registeredGRPCServices = append(g.registeredGRPCServices, grpcAddr)
	global.LOGGER.InfoContext(g.Context(), "✅ gRPC服务注册完成")
}

// RegisterGatewayHandler 注册gRPC-Gateway处理器 (本地调用方式)
// 使用示例:
//
//	g.RegisterGatewayHandler(func(ctx context.Context, mux *runtime.ServeMux) error {
//	    return apis.RegisterAgentSettingsServiceHandlerServer(ctx, mux, svc)
//	})
func (g *Gateway) RegisterGatewayHandler(registerFunc ServerHandlerRegisterFunc) error {
	httpAddr := g.gatewayConfig.HTTPServer.GetEndpoint()
	global.LOGGER.InfoContext(g.Context(), "开始注册gRPC-Gateway HTTP处理器")
	gwMux := g.GetGatewayMux()
	if err := registerFunc(g.Context(), gwMux); err != nil {
		global.LOGGER.ErrorContext(g.Context(), "❌ 注册gRPC-Gateway HTTP处理器失败: error=%v", err)
		return err
	}
	g.gatewayHandlerRegistrars = append(g.gatewayHandlerRegistrars, registerFunc)
	g.registeredGatewayHandlers = append(g.registeredGatewayHandlers, "gRPC-Gateway@"+httpAddr)
	global.LOGGER.InfoContext(g.Context(), "✅ gRPC-Gateway HTTP处理器注册成功")
	return nil
}

// RegisterProxyHandler 注册gRPC-Gateway代理处理器 (远程调用方式)
// 与 RegisterGatewayHandler 不同，此方法通过 gRPC 客户端连接远程服务
// 使用示例:
//
//	g.RegisterProxyHandler(func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error {
//	    return authpb.RegisterAuthServiceHandlerFromEndpoint(ctx, mux, endpoint, opts)
//	}, "localhost:50051")
func (g *Gateway) RegisterProxyHandler(registerFunc HandlerRegisterFunc, endpoint string, dialOpts ...grpc.DialOption) error {
	global.LOGGER.InfoContext(g.Context(), "开始注册gRPC-Gateway代理处理器: endpoint=%s", endpoint)

	opts := dialOpts
	if len(opts) == 0 {
		opts = g.Server.GetDialOptions()
	}

	gwMux := g.GetGatewayMux()
	if err := registerFunc(g.Context(), gwMux, endpoint, opts); err != nil {
		global.LOGGER.ErrorContext(g.Context(), "❌ 注册gRPC-Gateway代理处理器失败: endpoint=%s, error=%v", endpoint, err)
		return err
	}

	g.proxyHandlerRegistrations = append(g.proxyHandlerRegistrations, proxyHandlerRegistration{
		registerFunc: registerFunc,
		endpoint:     endpoint,
		dialOpts:     opts,
	})
	g.registeredGatewayHandlers = append(g.registeredGatewayHandlers, "gRPC-Gateway-Proxy@"+endpoint)

	global.LOGGER.InfoContext(g.Context(), "✅ gRPC-Gateway代理处理器注册成功: endpoint=%s", endpoint)
	return nil
}

// RegisterProxyHandlerByServiceName 通过服务名注册gRPC-Gateway代理处理器
// 从网关配置的 grpc.clients 中查找服务端点并构建完整的 dial options，无需手动指定
// 同一服务名共享一个 gRPC 连接，多个 handler 复用同一连接
// 使用示例:
//
//	g.RegisterProxyHandlerByServiceName("apex-access-control-service", func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
//	    return authpb.RegisterAuthServiceHandler(ctx, mux, conn)
//	})
func (g *Gateway) RegisterProxyHandlerByServiceName(serviceName string, registerFunc ConnHandlerRegisterFunc) error {
	endpoint, clientCfg, ok := g.getGRPCEndpointWithConfig(serviceName)
	if !ok {
		return errors.NewErrorf(errors.ErrCodeInvalidConfiguration, "gRPC client endpoint not found for service: %s", serviceName)
	}

	// 优先从连接池获取已有连接（由 InitClient 创建）
	if conn, ok := grpcpool.GetConn(serviceName); ok {
		global.LOGGER.DebugContext(g.Context(), "♻️  复用连接池中的已有连接: service=%s", serviceName)
		// 使用共享连接注册 handler
		gwMux := g.GetGatewayMux()
		if err := registerFunc(g.Context(), gwMux, conn); err != nil {
			global.LOGGER.ErrorContext(g.Context(), "❌ 注册gRPC-Gateway代理处理器失败: service=%s, error=%v", serviceName, err)
			return err
		}

		g.proxyHandlerRegistrations = append(g.proxyHandlerRegistrations, proxyHandlerRegistration{
			connRegisterFunc: registerFunc,
			conn:             conn,
			serviceName:      serviceName,
			clientCfg:        clientCfg,
		})
		g.registeredGatewayHandlers = append(g.registeredGatewayHandlers, "gRPC-Gateway-Proxy@"+serviceName)

		global.LOGGER.DebugContext(g.Context(), "✅ gRPC-Gateway代理处理器注册成功(复用连接): service=%s, endpoint=%s", serviceName, endpoint)
		return nil
	}

	// 连接池中没有，创建新连接
	var dialOpts []grpc.DialOption
	if clientCfg != nil {
		dialOpts = grpcpool.BuildDialOptions(clientCfg, serviceName, nil)
	}

	var err error
	conn, err := grpc.NewClient(endpoint, dialOpts...)
	if err != nil {
		return errors.NewErrorf(errors.ErrCodeInvalidConfiguration, "failed to create gRPC client for service %s: %v", serviceName, err)
	}

	// 存入连接池供后续复用
	grpcpool.PutConn(serviceName, conn)

	global.LOGGER.DebugContext(g.Context(), "创建共享 gRPC 连接: service=%s, endpoint=%s", serviceName, endpoint)

	// 使用共享连接注册 handler
	gwMux := g.GetGatewayMux()
	if err := registerFunc(g.Context(), gwMux, conn); err != nil {
		global.LOGGER.ErrorContext(g.Context(), "❌ 注册gRPC-Gateway代理处理器失败: service=%s, error=%v", serviceName, err)
		return err
	}

	g.proxyHandlerRegistrations = append(g.proxyHandlerRegistrations, proxyHandlerRegistration{
		connRegisterFunc: registerFunc,
		conn:             conn,
		serviceName:      serviceName,
		clientCfg:        clientCfg,
	})
	g.registeredGatewayHandlers = append(g.registeredGatewayHandlers, "gRPC-Gateway-Proxy@"+serviceName)

	global.LOGGER.DebugContext(g.Context(), "✅ gRPC-Gateway代理处理器注册成功: service=%s, endpoint=%s", serviceName, endpoint)
	return nil
}

// GetGRPCEndpoint 获取gRPC客户端端点地址
func (g *Gateway) GetGRPCEndpoint(serviceName string) (string, bool) {
	if g.gatewayConfig == nil || g.gatewayConfig.GRPC == nil || g.gatewayConfig.GRPC.Clients == nil {
		return "", false
	}
	clientCfg, exists := g.gatewayConfig.GRPC.Clients[serviceName]
	if !exists || clientCfg == nil || len(clientCfg.Endpoints) == 0 {
		return "", false
	}
	return clientCfg.Endpoints[0], true
}

// getGRPCEndpointWithConfig 获取gRPC客户端端点地址及配置
func (g *Gateway) getGRPCEndpointWithConfig(serviceName string) (string, *gwconfig.GRPCClient, bool) {
	if g.gatewayConfig == nil || g.gatewayConfig.GRPC == nil || g.gatewayConfig.GRPC.Clients == nil {
		return "", nil, false
	}
	clientCfg, exists := g.gatewayConfig.GRPC.Clients[serviceName]
	if !exists || clientCfg == nil || len(clientCfg.Endpoints) == 0 {
		return "", nil, false
	}
	return clientCfg.Endpoints[0], clientCfg, true
}

// RegisterHandler 注册HTTP处理器
func (g *Gateway) RegisterHandler(pattern string, handler http.Handler) {
	global.LOGGER.DebugContext(g.Context(), "注册HTTP处理器: pattern=%s", pattern)
	g.Server.RegisterHTTPRoute(pattern, handler)
	g.httpRouteRegistrations = append(g.httpRouteRegistrations, httpRouteRegistration{pattern: pattern, handler: handler})
	g.registeredHTTPRoutes = append(g.registeredHTTPRoutes, pattern)
	global.LOGGER.DebugContext(g.Context(), "✅ HTTP处理器注册成功: pattern=%s", pattern)
}

// RegisterHTTPRoute 注册HTTP路由 (便捷方法)
func (g *Gateway) RegisterHTTPRoute(pattern string, handlerFunc http.HandlerFunc) {
	global.LOGGER.DebugContext(g.Context(), "注册HTTP路由: pattern=%s", pattern)
	g.Server.RegisterHTTPRoute(pattern, handlerFunc)
	g.httpRouteRegistrations = append(g.httpRouteRegistrations, httpRouteRegistration{pattern: pattern, handler: handlerFunc})
	g.registeredHTTPRoutes = append(g.registeredHTTPRoutes, pattern)
	global.LOGGER.DebugContext(g.Context(), "✅ HTTP路由注册成功: pattern=%s", pattern)
}

// RegisterHTTPRoutes 批量注册HTTP路由
func (g *Gateway) RegisterHTTPRoutes(routes map[string]http.HandlerFunc) {
	for pattern, handler := range routes {
		g.RegisterHTTPRoute(pattern, handler)
	}
}

// AutoRegister 自动注册所有 gRPC 客户端和 HTTP Gateway Handler
// 基于 gRPC Server Reflection 自动发现服务，业务层无需写任何注册代码
// 前提: gRPC server 需要启用 reflection (reflection.Register(server))
//
// 使用示例:
//
//	gw, _ := gateway.NewGateway().WithSearchPath("config").Build()
//	result := gw.AutoRegister()  // 一行搞定所有注册
//	fmt.Println(result.Summary())
func (g *Gateway) AutoRegister() *grpcpool.AutoRegisterResult {
	return g.AutoRegisterWithHealthCheck(nil)
}

// AutoRegisterWithHealthCheck 自动注册（带健康检查器）
// 与 AutoRegister 相同，但支持传入自定义健康检查器
func (g *Gateway) AutoRegisterWithHealthCheck(healthChecker *grpcpool.HealthChecker) *grpcpool.AutoRegisterResult {
	global.LOGGER.InfoContext(g.Context(), "正在自动注册 gRPC 客户端和 HTTP Handler...")
	config := g.GetConfig()
	if config == nil || config.GRPC == nil || config.GRPC.Clients == nil {
		global.LOGGER.WarnContext(g.Context(), "gRPC client config is empty, skip auto register")
		return &grpcpool.AutoRegisterResult{}
	}

	// 先初始化连接和发现服务（不注册 handler）
	clientNames := grpcpool.InitConnectionsAndDiscover(g.Context(), healthChecker, config.GRPC.Clients)

	// 将动态 handler 注册函数加入 gatewayHandlerRegistrars，确保 RebuildHTTPGateway 时能重放
	gatewayMux := g.GetGatewayMux()
	dynamicRegisterFunc := func(ctx context.Context, mux *runtime.ServeMux) error {
		handlers, err := grpcpool.RegisterDynamicHandlers(ctx, mux, config.GRPC.Clients)
		if err != nil {
			return err
		}
		global.LOGGER.InfoContext(ctx, "✅ 动态注册 %d 个 HTTP handler", len(handlers))
		return nil
	}
	// 立即注册一次（到当前 mux）
	if err := dynamicRegisterFunc(g.Context(), gatewayMux); err != nil {
		global.LOGGER.WarnContext(g.Context(), "动态注册 HTTP handler 失败: %v", err)
	}
	// 加入 registrars 以便重建时重放
	g.gatewayHandlerRegistrars = append(g.gatewayHandlerRegistrars, dynamicRegisterFunc)

	result := &grpcpool.AutoRegisterResult{
		Clients:       clientNames,
		TotalClients:  len(config.GRPC.Clients),
		SkippedManual: 0,
	}

	global.LOGGER.InfoContext(g.Context(), "📋 %s", result.Summary())

	for _, name := range result.Clients {
		g.registeredGRPCServices = append(g.registeredGRPCServices, "auto-client:"+name)
	}

	// 启动下游服务发版监视（连接事件 + 契约指纹兜底）
	g.startServiceWatch(healthChecker, config.GRPC.Clients)

	// 设置服务恢复回调：服务从不可用恢复为可用时，重新发现服务和注册 HTTP 路由
	if healthChecker != nil {
		if watcher := g.serviceWatcher; watcher != nil {
			// 监视器已经在跟踪连接状态，"恢复"这个信号交给它合并即可。
			// 各自独立触发的话，一次服务恢复会打两次 reflection，而且两条路径会
			// 并发往同一个 mux 注册 —— 后者由 muxRegisterMu 兜住，但没必要先制造出来
			// [EN] The watcher already tracks connection state; folding the recovery signal
			// into it avoids a duplicate reflection round trip and a needless concurrent
			// registration against the same mux
			healthChecker.SetOnRecover(watcher.Trigger)
		} else {
			healthChecker.SetOnRecover(func(serviceName string) {
				mux := g.GetGatewayMux()
				if err := grpcpool.RediscoverAndRegisterService(g.Context(), mux, serviceName); err != nil {
					global.LOGGER.WarnContext(g.Context(), "服务 %s 恢复后重新注册失败: %v", serviceName, err)
				}
			})
		}
	}

	if healthChecker != nil {
		endpoints := grpcpool.BuildEndpointMap(config.GRPC.Clients)
		healthChecker.StartPeriodicCheck(grpcpool.DefaultHealthCheckInterval, endpoints)
	}

	return result
}

// WithServiceWatchInterval 设置下游服务发版的指纹兜底轮询间隔
//
// 传 0 表示关闭轮询，只保留连接事件驱动的感知（开销最低，但覆盖不到
// "多副本滚动发布期间连接全程 Ready" 这个盲区，详见 grpcpool.ServiceWatcher）
// 必须在 AutoRegister 之前调用
// [EN] Zero disables the fallback poll, leaving only connection-event detection — cheapest,
// but blind to a multi-replica rolling deploy that never leaves the Ready state.
// Must be called before AutoRegister
func (g *Gateway) WithServiceWatchInterval(interval time.Duration) *Gateway {
	g.watchInterval = interval
	g.watchIntervalSetOnce = true
	return g
}

// GetServiceWatcher 获取发版监视器，未启动时返回 nil
// 发布侧想做到秒级感知，可以在部署完成后调用它的 Trigger
// [EN] A publish-side hook can call Trigger on it for second-level convergence
func (g *Gateway) GetServiceWatcher() *grpcpool.ServiceWatcher {
	return g.serviceWatcher
}

// startServiceWatch 启动下游服务发版监视
func (g *Gateway) startServiceWatch(healthChecker *grpcpool.HealthChecker, clients map[string]*gwconfig.GRPCClient) {
	if g.serviceWatcher != nil {
		return
	}

	interval := g.watchInterval
	if !g.watchIntervalSetOnce {
		interval = grpcpool.DefaultWatchInterval
	}

	// 只有"已注册且判定为不健康"才算不可用：HealthChecker.IsHealthy 对未注册的服务
	// 同样返回 false，直接拿它当过滤条件会让启动时连接失败、从未进入健康检查的服务
	// 被永久跳过轮询。这与健康拦截器的判定口径（exists && !healthy）保持一致
	// [EN] IsHealthy also returns false for services it never registered, which would skip
	// polling forever for a service whose connection failed at startup. Matches the health
	// interceptor's own exists && !healthy test
	var isHealthy func(string) bool
	if healthChecker != nil {
		isHealthy = func(serviceName string) bool {
			healthy, exists := healthChecker.GetServiceHealth(serviceName)
			return !exists || healthy
		}
	}

	watcher := grpcpool.NewServiceWatcher(grpcpool.ServiceWatchOptions{
		// 传函数而不是当前 mux：RebuildHTTPGateway 会换掉整个 mux
		MuxFn:     g.GetGatewayMux,
		Services:  grpcpool.WatchedServiceNames(clients),
		Interval:  interval,
		IsHealthy: isHealthy,
		OnChange: func(serviceName string, routes []string) {
			global.LOGGER.InfoContext(g.Context(), "🔄 服务 %s 发版已被感知，%d 个 HTTP 路由已更新", serviceName, len(routes))
		},
	})
	if watcher == nil {
		return
	}

	g.serviceWatcher = watcher
	watcher.Start(g.Context())
}

// AddGrpcGatewayMiddleware 添加 gRPC-Gateway 中间件
// 注意：必须在网关启动之前调用
func (g *Gateway) AddGrpcGatewayMiddleware(mw runtime.Middleware) {
	g.Server.AddGrpcGatewayMiddleware(mw)
	global.LOGGER.InfoContext(g.Context(), "✅ 已添加 gRPC-Gateway 中间件")
}

// AddGrpcGatewayMiddlewareProvider 添加 gRPC-Gateway 中间件提供器
// 提供器会在 HTTP Gateway 初始化时被调用，适用于需要在 Build 后才能创建的中间件
func (g *Gateway) AddGrpcGatewayMiddlewareProvider(provider func() []runtime.Middleware) {
	g.Server.AddGrpcGatewayMiddlewareProvider(provider)
	global.LOGGER.InfoContext(g.Context(), "✅ 已添加 gRPC-Gateway 中间件提供器")
}

// RebuildHTTPGateway 重建 HTTP Gateway（用于在添加中间件后重新初始化）
// 注意：需要在注册 HTTP Handlers 之前调用
func (g *Gateway) RebuildHTTPGateway() error {
	if err := g.Server.RebuildHTTPGateway(); err != nil {
		return err
	}
	return g.replayHTTPRegistrations()
}

func (g *Gateway) replayHTTPRegistrations() error {
	for _, registerFunc := range g.gatewayHandlerRegistrars {
		if registerFunc == nil {
			continue
		}
		if err := registerFunc(g.Context(), g.GetGatewayMux()); err != nil {
			return err
		}
	}

	// 重放代理处理器注册
	for _, proxyReg := range g.proxyHandlerRegistrations {
		if proxyReg.connRegisterFunc != nil && proxyReg.conn != nil {
			// 基于 conn 的注册方式（共享连接）
			if err := proxyReg.connRegisterFunc(g.Context(), g.GetGatewayMux(), proxyReg.conn); err != nil {
				return err
			}
		} else if proxyReg.registerFunc != nil {
			// 基于 endpoint + dialOpts 的注册方式
			if err := proxyReg.registerFunc(g.Context(), g.GetGatewayMux(), proxyReg.endpoint, proxyReg.dialOpts); err != nil {
				return err
			}
		}
	}

	for _, route := range g.httpRouteRegistrations {
		if route.handler == nil {
			continue
		}
		g.Server.RegisterHTTPRoute(route.pattern, route.handler)
	}

	if g.gatewayConfig != nil && g.gatewayConfig.Swagger != nil && g.gatewayConfig.Swagger.Enabled {
		if err := g.Server.EnableSwagger(); err != nil {
			return err
		}
	}

	return nil
}

// GetConfig 获取网关配置
func (g *Gateway) GetConfig() *gwconfig.Gateway {
	return g.Server.GetConfig()
}

// SetDynamicSignatureProvider 设置动态签名提供器
func (g *Gateway) SetDynamicSignatureProvider(provider middleware.DynamicSignatureProvider) {
	if manager := g.Server.GetMiddlewareManager(); manager != nil {
		manager.SetDynamicSignatureProvider(provider)
		global.LOGGER.InfoContext(g.Context(), "✅ 已设置动态签名提供器")
	}
}

// SetDynamicRateLimitProvider 设置动态限流提供器
func (g *Gateway) SetDynamicRateLimitProvider(provider middleware.DynamicRateLimitProvider) {
	if manager := g.Server.GetMiddlewareManager(); manager != nil {
		manager.SetDynamicRateLimitProvider(provider)
		global.LOGGER.InfoContext(g.Context(), "✅ 已设置动态限流提供器")
	}
}

// Context 获取 Gateway 的上下文
func (g *Gateway) Context() context.Context {
	if g.ctx == nil {
		return context.Background()
	}
	return g.ctx
}

// Context 获取 GatewayBuilder 的上下文
func (b *GatewayBuilder) Context() context.Context {
	if b.ctx == nil {
		return context.Background()
	}
	return b.ctx
}

// Start 启动网关服务并显示banner（默认行为）
func (g *Gateway) Start() error {
	return g.StartWithBanner()
}

// StartSilent 静默启动网关服务（不显示banner）
func (g *Gateway) StartSilent() error {
	if g.gatewayConfig != nil && g.gatewayConfig.Swagger != nil && g.gatewayConfig.Swagger.Enabled {
		if err := g.EnableSwagger(); err != nil {
			global.LOGGER.WarnContext(g.Context(), "enable swagger failed: %v", err)
		}
	}
	return g.Server.Start()
}

// Run starts the gateway and waits for a shutdown signal.
func (g *Gateway) Run() error {
	if err := g.Start(); err != nil {
		global.LOGGER.WithError(err).ErrorMsg("Failed to start gateway")
		return err
	}
	return g.WaitForShutdown()
}

// StartWithBanner 启动网关服务并显示banner
func (g *Gateway) StartWithBanner() error {
	bannerManager := g.Server.GetBannerManager()

	if bannerManager != nil {
		bannerManager.PrintStartupChecks()
	}

	// 默认启用Swagger文档服务
	if g.gatewayConfig != nil && g.gatewayConfig.Swagger != nil && g.gatewayConfig.Swagger.Enabled {
		if err := g.EnableSwagger(); err != nil {
			global.LOGGER.WarnContext(g.Context(), "⚠️  启用Swagger失败: %v", err)
		} else {
			global.LOGGER.InfoContext(g.Context(), "✅ Swagger已成功启用: %s", g.gatewayConfig.Swagger.UIPath)
		}
	}

	cg := global.LOGGER.NewConsoleGroup()
	cg.Info("🚀 正在启动服务器...")

	// 启动服务
	if err := g.Server.Start(); err != nil {
		global.LOGGER.ErrorContext(g.Context(), "❌ 启动网关失败: %v", err)
		return err
	}

	cg.Info("✅ 服务器启动成功!")

	// 🎯 启动成功后打印完整的 Banner 和配置信息
	g.PrintStartupInfo()

	return nil
}

// Stop 停止网关服务
func (g *Gateway) Stop() error {
	cg := global.LOGGER.NewConsoleGroup()
	cg.Info("🛑 开始停止网关服务...")

	// 先停发版监视：它会往 ServeMux 注册路由，必须在服务器停止之前收干净
	// [EN] Stopped first because it registers routes into the ServeMux
	if g.serviceWatcher != nil {
		g.serviceWatcher.Stop()
		g.serviceWatcher = nil
		cg.Info("✅ 服务发版监视已停止")
	}

	// 先停止服务器
	if err := g.Server.Stop(); err != nil {
		global.LOGGER.ErrorContext(g.Context(), "❌ 停止服务器失败: error=%v", err)
		return err
	}
	cg.Info("✅ 服务器已停止")

	// 再停止配置管理器
	if g.configManager != nil {
		cg.Info("停止配置管理器...")
		g.configManager.Stop()
		cg.Info("✅ 配置管理器已停止")
	}

	cg.Info("✅ 网关服务已完全停止")
	return nil
}

// PrintStartupInfo 打印启动信息
func (g *Gateway) PrintStartupInfo() {
	if bannerManager := g.Server.GetBannerManager(); bannerManager != nil {
		bannerManager.PrintStartupReport()
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
	cg := global.LOGGER.NewConsoleGroup()
	cg.Info("📋 API 注册汇总 (API Registration Summary)")

	// gRPC 服务统计
	cg.Info("🔷 gRPC Services: %d", len(g.registeredGRPCServices))
	if len(g.registeredGRPCServices) > 0 {
		for i, svc := range g.registeredGRPCServices {
			cg.Info("  %d. %s", i+1, svc)
		}
	} else {
		cg.Info("  (无注册服务)")
	}

	// gRPC-Gateway 处理器统计
	cg.Info("🌐 gRPC-Gateway Handlers: %d", len(g.registeredGatewayHandlers))
	if len(g.registeredGatewayHandlers) > 0 {
		for i, handler := range g.registeredGatewayHandlers {
			cg.Info("  %d. %s", i+1, handler)
		}
	} else {
		cg.Info("  (无注册处理器)")
	}

	// HTTP 路由统计
	cg.Info("🔗 HTTP Routes: %d", len(g.registeredHTTPRoutes))
	if len(g.registeredHTTPRoutes) > 0 {
		for i, route := range g.registeredHTTPRoutes {
			cg.Info("  %d. %s", i+1, route)
		}
	} else {
		cg.Info("  (无注册路由)")
	}

	// 总计
	totalAPIs := len(g.registeredGRPCServices) + len(g.registeredGatewayHandlers) + len(g.registeredHTTPRoutes)
	cg.Info("✅ 总计注册 API 数量: %d", totalAPIs)
}

// GetGatewayConfig 获取网关配置
func (g *Gateway) GetGatewayConfig() *gwconfig.Gateway {
	return g.gatewayConfig
}

// GetListener 按名称获取监听器配置
func (g *Gateway) GetListener(name string) *gwconfig.Listener {
	if g.gatewayConfig == nil {
		return nil
	}
	return g.gatewayConfig.GetListener(name)
}

// GetListenerEndpoint 按名称获取监听器端点地址
func (g *Gateway) GetListenerEndpoint(name string) string {
	if l := g.GetListener(name); l != nil {
		return l.GetEndpoint()
	}
	return ""
}

// RegisterConfigCallbacks 注册配置变更回调
func (g *Gateway) RegisterConfigCallbacks() {
	if g.configManager == nil {
		return
	}

	// 注册配置变更回调
	g.configManager.RegisterConfigCallback(func(ctx context.Context, event config.CallbackEvent) error {
		if newConfig, ok := event.NewValue.(*gwconfig.Gateway); ok {
			cg := global.LOGGER.NewConsoleGroup()
			cg.Info("%s", errors.FormatConfigUpdateInfo(newConfig.Name))
			g.gatewayConfig = newConfig
			if newConfig.HTTPServer != nil {
				cg.Info("%s", errors.FormatConnectionInfo("HTTP", newConfig.HTTPServer.GetEndpoint()))
			}
		}
		return nil
	}, config.CallbackOptions{
		ID:       "gateway_config_handler",
		Types:    []config.CallbackType{config.CallbackTypeConfigChanged},
		Priority: -100, // 高优先级
		Async:    false,
		Timeout:  5 * time.Second,
	})

	// 注册环境变更回调
	if err := g.configManager.RegisterConfigCallback(func(ctx context.Context, event config.CallbackEvent) error {
		newConfig, ok := event.NewValue.(*gwconfig.Gateway)
		if !ok || newConfig == nil {
			return nil
		}

		newConfig = mergeGatewayConfigWithDefaults(newConfig)
		return g.applyReloadedConfig(ctx, newConfig)
	}, config.CallbackOptions{
		ID:       "gateway_runtime_config_handler",
		Types:    []config.CallbackType{config.CallbackTypeConfigChanged},
		Priority: -90,
		Async:    false,
		Timeout:  30 * time.Second,
	}); err != nil {
		global.LOGGER.WarnContext(g.Context(), "register gateway runtime config callback failed: %v", err)
	}

	g.configManager.RegisterEnvironmentCallback("gateway_runtime_env_handler", func(oldEnv, newEnv config.EnvironmentType) error {
		global.LOGGER.NewConsoleGroup().Info("%s", errors.FormatEnvironmentChangeInfo(string(oldEnv), string(newEnv)))
		return nil
	}, -100, false) // 高优先级
}

// applyReloadedConfig applies runtime-sensitive config changes without a full process restart.
func (g *Gateway) applyReloadedConfig(ctx context.Context, newConfig *gwconfig.Gateway) error {
	oldConfig := g.Server.GetConfig()

	cg := global.LOGGER.NewConsoleGroup()
	cg.Info("%s", errors.FormatConfigUpdateInfo(newConfig.Name))
	g.gatewayConfig = newConfig
	global.SetGatewayConfig(newConfig)

	httpChanged := httpRuntimeChanged(oldConfig, newConfig) || swaggerRuntimeChanged(oldConfig, newConfig)
	grpcChanged := grpcRuntimeChanged(oldConfig, newConfig)
	pprofChanged := pprofRuntimeChanged(oldConfig, newConfig)

	if httpChanged {
		cg.Info("HTTP/Swagger config changed, reloading HTTP gateway")
		if err := g.Server.ReloadHTTPGateway(newConfig, g.replayHTTPRegistrations); err != nil {
			return err
		}
	}

	if grpcChanged {
		cg.Info("gRPC server config changed, reloading gRPC server")
		registrars := make([]func(*grpc.Server), 0, len(g.grpcServiceRegistrars))
		for _, register := range g.grpcServiceRegistrars {
			registrars = append(registrars, register)
		}
		if err := g.Server.ReloadGRPCServer(newConfig, registrars); err != nil {
			return err
		}
	}

	if pprofChanged {
		cg.Info("PProf config changed, reloading PProf server")
		if err := g.Server.ReloadPProfServer(newConfig); err != nil {
			return err
		}
	}

	if !httpChanged && !grpcChanged && !pprofChanged {
		g.Server.ApplyConfig(newConfig)
	}

	if newConfig.HTTPServer != nil {
		cg.Info("%s", errors.FormatConnectionInfo("HTTP", newConfig.HTTPServer.GetEndpoint()))
	}
	if newConfig.GRPC != nil && newConfig.GRPC.Server != nil {
		cg.Info("%s", errors.FormatConnectionInfo("gRPC", newConfig.GRPC.Server.GetEndpoint()))
	}

	return nil
}

func httpRuntimeChanged(oldConfig, newConfig *gwconfig.Gateway) bool {
	if oldConfig == nil || newConfig == nil {
		return oldConfig != newConfig
	}
	if oldConfig.HTTPServer == nil || newConfig.HTTPServer == nil {
		return oldConfig.HTTPServer != newConfig.HTTPServer
	}

	oldHTTP := *oldConfig.HTTPServer
	newHTTP := *newConfig.HTTPServer
	oldHTTP.Endpoint = ""
	newHTTP.Endpoint = ""
	return !reflect.DeepEqual(oldHTTP, newHTTP)
}

func grpcRuntimeChanged(oldConfig, newConfig *gwconfig.Gateway) bool {
	if oldConfig == nil || newConfig == nil {
		return oldConfig != newConfig
	}
	if oldConfig.GRPC == nil || newConfig.GRPC == nil {
		return oldConfig.GRPC != newConfig.GRPC
	}
	if oldConfig.GRPC.Server == nil || newConfig.GRPC.Server == nil {
		return oldConfig.GRPC.Server != newConfig.GRPC.Server
	}

	oldServer := *oldConfig.GRPC.Server
	newServer := *newConfig.GRPC.Server
	oldServer.Endpoint = ""
	newServer.Endpoint = ""
	return !reflect.DeepEqual(oldServer, newServer)
}

func swaggerRuntimeChanged(oldConfig, newConfig *gwconfig.Gateway) bool {
	if oldConfig == nil || newConfig == nil {
		return oldConfig != newConfig
	}
	return !reflect.DeepEqual(oldConfig.Swagger, newConfig.Swagger)
}

func pprofRuntimeChanged(oldConfig, newConfig *gwconfig.Gateway) bool {
	if oldConfig == nil || newConfig == nil {
		return oldConfig != newConfig
	}
	if oldConfig.Middleware == nil || newConfig.Middleware == nil {
		return oldConfig.Middleware != newConfig.Middleware
	}
	return !reflect.DeepEqual(oldConfig.Middleware.PProf, newConfig.Middleware.PProf)
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
func (g *Gateway) GetRedis() redis.UniversalClient {
	if poolManager := g.GetPoolManager(); poolManager != nil {
		return poolManager.GetRedis()
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
		WithContext(context.Background()).
		WithSearchPath(path).
		WithEnvironment(config.GetEnvironment()).
		WithHotReload(nil).
		BuildAndStart()

	if err != nil {
		return errors.NewError(errors.ErrCodeOperationFailed, errors.FormatStartupError("快速启动", err))
	}

	// 等待关闭信号
	return gw.WaitForShutdown()
}

// QuickStartWithConfigFile 使用指定配置文件快速启动
func QuickStartWithConfigFile(configFilePath string) error {
	gw, err := NewGateway().
		WithContext(context.Background()).
		WithConfigPath(configFilePath).
		WithEnvironment(config.GetEnvironment()).
		WithHotReload(nil).
		BuildAndStart()

	if err != nil {
		return errors.NewError(errors.ErrCodeOperationFailed, errors.FormatStartupError("使用指定配置文件启动", err))
	}

	return gw.WaitForShutdown()
}

// QuickStartWithConfigFilePrefix 使用指定配置文件快速启动
func QuickStartWithConfigFilePrefix(configFilePath string, prefix string) error {
	gw, err := NewGateway().
		WithContext(context.Background()).
		WithConfigPath(configFilePath).
		WithPrefix(prefix).
		WithEnvironment(config.GetEnvironment()).
		WithHotReload(nil).
		BuildAndStart()

	if err != nil {
		return errors.NewError(errors.ErrCodeOperationFailed, errors.FormatStartupError("使用指定配置文件和前缀启动", err))
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

// OnShutdown 注册应用级关闭回调，在网关 Stop() 之前执行
// 用于 flush 异步写入器、关闭连接等资源清理，避免 os.Exit 导致数据丢失
func (g *Gateway) OnShutdown(fn func()) {
	if fn != nil {
		g.shutdownCallbacks = append(g.shutdownCallbacks, fn)
	}
}

// setupGracefulShutdown 设置优雅关闭信号处理
func (g *Gateway) setupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-c
		cg := global.LOGGER.NewConsoleGroup()

		cg.Info("%s", errors.FormatShutdownInfo(sig.String()))

		// 显示关闭信息
		g.PrintShutdownInfo()

		// 执行应用级关闭回调（在网关 Stop 之前 flush 资源）
		cg.Info("执行应用级关闭回调（%d 个）...", len(g.shutdownCallbacks))
		for _, cb := range g.shutdownCallbacks {
			cb()
		}
		cg.Info("应用级关闭回调执行完毕")
		// 停止服务
		if err := g.Stop(); err != nil {
			global.LOGGER.ErrorContext(g.Context(), errors.FormatStopError(err))
		}

		// 显示关闭完成信息
		g.PrintShutdownComplete()

		os.Exit(0)
	}()
}
