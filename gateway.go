/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 20:08:44
 * @FilePath: \go-rpc-gateway\gateway.go
 * @Description: Gateway主入口，基于go-config和go-core重构
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

// Package gateway 提供一个轻量级的gRPC-Gateway框架
// 集成了数据库、Redis和对象存储等组件
// 基于go-config和go-core架构
package gateway

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/kamalyes/go-config/pkg/register"
	"github.com/kamalyes/go-core/pkg/global"
	"github.com/kamalyes/go-rpc-gateway/config"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/kamalyes/go-rpc-gateway/server"
	"google.golang.org/grpc"
)

// Gateway 是主要的网关服务器
type Gateway struct {
	*server.Server
	pprofEnabled       bool
	pprofConfig        *register.PProf
	pprofAdapter       *middleware.PProfConfigAdapter
	pprofGatewayConfig *middleware.PProfGatewayConfig
}

// PProfOptions pprof配置选项
type PProfOptions struct {
	Enabled     bool     `json:"enabled"`       // 是否启用pprof
	AuthToken   string   `json:"auth_token"`    // 认证令牌
	AllowedIPs  []string `json:"allowed_ips"`   // 允许的IP列表
	PathPrefix  string   `json:"path_prefix"`   // 路径前缀
	DevModeOnly bool     `json:"dev_mode_only"` // 是否只在开发模式启用
}

// ServiceRegisterFunc gRPC服务注册函数类型
type ServiceRegisterFunc func(*grpc.Server)

// HandlerRegisterFunc HTTP处理器注册函数类型
type HandlerRegisterFunc func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error

// Config 网关配置类型别名
type Config = config.GatewayConfig

// getEnvOrDefault 获取环境变量或返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// New 创建新的网关实例
func New(cfg ...*Config) (*Gateway, error) {
	var gatewayConfig *Config
	if len(cfg) > 0 && cfg[0] != nil {
		gatewayConfig = cfg[0]
	} else {
		gatewayConfig = config.DefaultGatewayConfig()
	}

	srv, err := server.NewServer(gatewayConfig)
	if err != nil {
		return nil, err
	}

	defaultPProfConfig := middleware.DefaultPProfConfig()
	return &Gateway{
		Server:             srv,
		pprofEnabled:       false,
		pprofConfig:        defaultPProfConfig,
		pprofAdapter:       middleware.NewPProfConfigAdapter(defaultPProfConfig),
		pprofGatewayConfig: middleware.NewPProfGatewayConfig(),
	}, nil
}

// NewWithConfigFile 使用配置文件创建Gateway实例
func NewWithConfigFile(configPath string) (*Gateway, error) {
	// 创建配置管理器
	configManager, err := config.NewConfigManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	// 使用配置管理器创建服务器
	srv, err := server.NewServerWithConfigManager(configManager)
	if err != nil {
		return nil, err
	}

	defaultPProfConfig := middleware.DefaultPProfConfig()
	return &Gateway{
		Server:             srv,
		pprofEnabled:       false,
		pprofConfig:        defaultPProfConfig,
		pprofAdapter:       middleware.NewPProfConfigAdapter(defaultPProfConfig),
		pprofGatewayConfig: middleware.NewPProfGatewayConfig(),
	}, nil
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

// EnableSwagger 启用Swagger文档服务
// [EN] Enable Swagger documentation service
func (g *Gateway) EnableSwagger(jsonPath string) *Gateway {
	return g.EnableSwaggerWithOptions(config.SwaggerConfig{
		Enabled:     true,
		JSONPath:    jsonPath,
		UIPath:      "/swagger",
		Title:       "API Documentation",
		Description: "API Documentation powered by Swagger UI",
	})
}

// EnableSwaggerWithOptions 使用自定义选项启用Swagger
// [EN] Enable Swagger with custom options
func (g *Gateway) EnableSwaggerWithOptions(options config.SwaggerConfig) *Gateway {
	// 更新配置
	// [EN] Update configuration
	g.Server.GetConfig().Middleware.Swagger = options

	// 转换为中间件配置
	// [EN] Convert to middleware configuration
	middlewareConfig := &middleware.SwaggerConfig{
		Enabled:     options.Enabled,
		JSONPath:    options.JSONPath,
		UIPath:      options.UIPath,
		Title:       options.Title,
		Description: options.Description,
	}

	// 创建Swagger中间件
	// [EN] Create Swagger middleware
	swaggerMiddleware := middleware.NewSwaggerMiddleware(middlewareConfig)

	// 直接创建处理函数
	// [EN] Create handler functions directly
	swaggerHandler := func(w http.ResponseWriter, r *http.Request) {
		// 创建一个虚拟的下一个处理器，用于满足中间件接口
		// [EN] Create a dummy next handler to satisfy middleware interface
		nextHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			// 这个处理器不会被调用，因为Swagger中间件会直接处理请求
			// [EN] This handler won't be called as Swagger middleware handles requests directly
		})
		handler := swaggerMiddleware.Handler()(nextHandler)
		handler.ServeHTTP(w, r)
	}

	// 注册Swagger路由
	// [EN] Register Swagger routes
	g.RegisterHTTPRoute(options.UIPath+"/", swaggerHandler)
	g.RegisterHTTPRoute(options.UIPath+"/index.html", swaggerHandler)
	g.RegisterHTTPRoute(options.UIPath+"/swagger.json", swaggerHandler)

	return g
}

// SetSwaggerJSON 设置Swagger JSON数据
// [EN] Set Swagger JSON data
func (g *Gateway) SetSwaggerJSON(jsonData []byte) error {
	// 查找现有的Swagger中间件
	// [EN] Find existing Swagger middleware
	if middlewareManager := g.Server.GetMiddlewareManager(); middlewareManager != nil {
		// 这里需要实现中间件管理器中的查找和更新功能
		// [EN] Need to implement find and update functionality in middleware manager
		// 暂时返回nil，后续可以扩展
		// [EN] Return nil for now, can be extended later
	}
	return nil
}

// GetConfig 获取网关配置
func (g *Gateway) GetConfig() *config.GatewayConfig {
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
	return g.Server.Stop()
}

// EnablePProf 启用pprof性能分析功能
// 这是一个简化的API，使用默认配置启用pprof
func (g *Gateway) EnablePProf() *Gateway {
	return g.EnablePProfWithOptions(middleware.PProfOptions{
		Enabled:     true,
		AuthToken:   getEnvOrDefault("PPROF_TOKEN", "gateway-pprof-2024"),
		PathPrefix:  "/debug/pprof",
		DevModeOnly: false,
		AllowedIPs:  []string{}, // 默认允许所有IP
	})
}

// EnablePProfWithOptions 使用自定义选项启用pprof
func (g *Gateway) EnablePProfWithOptions(options middleware.PProfOptions) *Gateway {
	// 使用pprofGatewayConfig管理配置
	g.pprofGatewayConfig.EnablePProfWithOptions(options)

	// 同步到原有字段，保持向后兼容性
	g.pprofConfig = g.pprofGatewayConfig.GetPProfConfig()
	g.pprofAdapter = g.pprofGatewayConfig.GetPProfAdapter()
	g.pprofEnabled = g.pprofGatewayConfig.IsPProfEnabled()

	// 自动注册pprof相关的Web界面路由
	if g.pprofEnabled {
		g.registerPProfWebInterface()
	}

	return g
}

// EnablePProfWithToken 使用指定token启用pprof (便捷方法)
func (g *Gateway) EnablePProfWithToken(token string) *Gateway {
	return g.EnablePProfWithOptions(middleware.PProfOptions{
		Enabled:    true,
		AuthToken:  token,
		PathPrefix: "/debug/pprof",
		AllowedIPs: []string{},
	})
}

// EnablePProfForDevelopment 启用开发环境pprof (便捷方法)
func (g *Gateway) EnablePProfForDevelopment() *Gateway {
	return g.EnablePProfWithOptions(middleware.PProfOptions{
		Enabled:     true,
		AuthToken:   "dev-debug-token",
		PathPrefix:  "/debug/pprof",
		DevModeOnly: true,
		AllowedIPs:  []string{"127.0.0.1", "::1"},
	})
}

// GetPProfConfig 获取pprof配置
func (g *Gateway) GetPProfConfig() *register.PProf {
	return g.pprofGatewayConfig.GetPProfConfig()
}

// IsPProfEnabled 检查pprof是否启用
func (g *Gateway) IsPProfEnabled() bool {
	return g.pprofGatewayConfig.IsPProfEnabled()
}

// GetPProfEndpoints 获取所有可用的pprof端点信息
func (g *Gateway) GetPProfEndpoints() []middleware.PProfInfo {
	return g.pprofGatewayConfig.GetPProfEndpoints()
}

// registerPProfWebInterface 注册pprof Web界面
func (g *Gateway) registerPProfWebInterface() {
	if !g.IsPProfEnabled() {
		return
	}

	// 注册主页，显示pprof信息
	g.RegisterHTTPRoute("/", g.pprofGatewayConfig.CreatePProfWebInterface())

	// 注册pprof状态API
	g.RegisterHTTPRoute("/api/pprof/status", g.pprofGatewayConfig.CreatePProfStatusAPIHandler())
}

// PrintFeatureStatus 打印Gateway功能状态 (框架内置方法)
func (g *Gateway) PrintFeatureStatus() {
	config := g.GetConfig()

	global.LOGGER.Info("🔧 Gateway功能状态:")

	// PProf状态
	if g.IsPProfEnabled() {
		pprofConfig := g.GetPProfConfig()
		global.LOGGER.InfoKV("   ✅ 性能分析 (PProf)", "path_prefix", pprofConfig.PathPrefix)
	} else {
		global.LOGGER.InfoMsg("   ❌ 性能分析 (PProf) - 未启用")
	}

	// 中间件状态
	if manager := g.GetMiddlewareManager(); manager != nil {
		global.LOGGER.InfoMsg("   ✅ 中间件链 - 已配置")

		// CORS状态 - 使用go-config的配置
		if config.SingleConfig.Cors.AllowedAllOrigins || len(config.SingleConfig.Cors.AllowedOrigins) > 0 {
			global.LOGGER.InfoKV("     • CORS - 已启用", "allow_origins", config.SingleConfig.Cors.AllowedOrigins)
		} else {
			global.LOGGER.InfoMsg("     • CORS - 未启用")
		}

		// 限流状态
		if config.Middleware.RateLimit.Enabled {
			global.LOGGER.InfoKV("     • 限流控制 - 已启用", "rate", config.Middleware.RateLimit.Rate, "unit", "req/s")
		} else {
			global.LOGGER.InfoMsg("     • 限流控制 - 未启用")
		}

		// 访问日志状态
		if config.Middleware.AccessLog.Enabled {
			global.LOGGER.InfoMsg("     • 访问日志 - 已启用")
		} else {
			global.LOGGER.InfoMsg("     • 访问日志 - 未启用")
		}

		// 认证状态 - 使用go-config的JWT配置
		if config.SingleConfig.JWT.SigningKey != "" {
			global.LOGGER.InfoKV("     • 认证控制 - 已启用", "type", "JWT")
		} else {
			global.LOGGER.InfoMsg("     • 认证控制 - 未启用")
		}

		// 签名验证状态
		if config.Middleware.Signature.Enabled {
			global.LOGGER.InfoMsg("     • 签名验证 - 已启用")
		} else {
			global.LOGGER.InfoMsg("     • 签名验证 - 未启用")
		}
	} else {
		global.LOGGER.InfoMsg("   ❌ 中间件链 - 未初始化")
	}

	// 安全控制状态
	if config.Security.TLS.Enabled {
		global.LOGGER.InfoMsg("   ✅ 安全控制 - HTTPS已启用")
	} else {
		global.LOGGER.InfoMsg("   ⚠️  安全控制 - 仅HTTP (建议启用HTTPS)")
	}

	// 监控功能状态
	if config.Monitoring.Metrics.Enabled {
		global.LOGGER.InfoKV("   ✅ 监控指标 - 已启用", "path", config.Monitoring.Metrics.Path)
	} else {
		global.LOGGER.InfoMsg("   ❌ 监控指标 - 未启用")
	}

	// 链路追踪状态 - 使用go-config的Jaeger配置
	if config.SingleConfig.Jaeger.Service != "" {
		global.LOGGER.InfoKV("   ✅ 链路追踪 - 已启用", "service_name", config.SingleConfig.Jaeger.Service)
	} else {
		global.LOGGER.InfoMsg("   ❌ 链路追踪 - 未启用")
	} // 健康检查状态
	if config.Gateway.HealthCheck.Enabled {
		global.LOGGER.InfoKV("   ✅ 健康检查 - 已启用", "path", config.Gateway.HealthCheck.Path)
	} else {
		global.LOGGER.InfoMsg("   ❌ 健康检查 - 未启用")
	}
}

// PrintStartupInfo 打印启动信息 (框架内置方法)
func (g *Gateway) PrintStartupInfo() {
	// 使用专门的BannerManager来打印启动信息
	if bannerManager := g.Server.GetBannerManager(); bannerManager != nil {
		bannerManager.PrintStartupBanner()

		// 如果启用了pprof，打印pprof信息
		if g.IsPProfEnabled() {
			bannerManager.PrintPProfInfo(g.pprofGatewayConfig)
		}

		// 打印中间件状态
		bannerManager.PrintMiddlewareStatus()

		// 打印使用指南
		bannerManager.PrintUsageGuide()
	}
}

// PrintShutdownInfo 打印关闭信息 (框架内置方法)
func (g *Gateway) PrintShutdownInfo() {
	// 使用专门的BannerManager来打印关闭信息
	if bannerManager := g.Server.GetBannerManager(); bannerManager != nil {
		bannerManager.PrintShutdownBanner()
	}
}

// PrintShutdownComplete 打印关闭完成信息 (框架内置方法)
func (g *Gateway) PrintShutdownComplete() {
	// 使用专门的BannerManager来打印关闭完成信息
	if bannerManager := g.Server.GetBannerManager(); bannerManager != nil {
		bannerManager.PrintShutdownComplete()
	}
}
