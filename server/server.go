/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 21:28:30
 * @FilePath: \engine-im-agent-service\go-rpc-gateway\server\server.go
 * @Description: Gateway服务器核心结构定义
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"context"
	"net/http"
	"sync"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	goconfig "github.com/kamalyes/go-config"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-rpc-gateway/cpool"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"google.golang.org/grpc"
)

// Server Gateway服务器
type Server struct {
	config     *gwconfig.Gateway
	configSafe *goconfig.ConfigSafe // 添加安全配置访问器

	// 服务器组件
	grpcServer *grpc.Server
	httpServer *http.Server
	gwMux      *runtime.ServeMux
	httpMux    *http.ServeMux // 添加HTTP路由管理器

	// 中间件管理器
	middlewareManager *middleware.Manager

	// grpc-gateway 中间件（runtime.Middleware）
	grpcGatewayMiddlewares         []runtime.Middleware
	grpcGatewayMiddlewareProviders []func() []runtime.Middleware // 中间件提供器

	// 健康检查管理器
	healthManager *middleware.HealthManager

	// Banner管理器
	bannerManager *BannerManager

	// 功能管理器
	featureManager *FeatureManager

	// 连接池管理器
	poolManager cpool.PoolManager

	// WebSocket 服务
	webSocketService *WebSocketService

	// 状态管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 运行状态
	running bool
	mu      sync.RWMutex
}

// GetGatewayMux 获取 Gateway Mux（用于高级路由注册）
func (s *Server) GetGatewayMux() *runtime.ServeMux {
	return s.gwMux
}

// NewServer 创建新的Gateway服务器 - 使用全局 GATEWAY 配置
func NewServer() (*Server, error) {
	cfg := global.GATEWAY
	if cfg == nil {
		return nil, errors.NewError(errors.ErrCodeInvalidConfiguration, "global GATEWAY config is not initialized")
	}

	// 记录环境配置应用情况
	configSafe := goconfig.SafeConfig(cfg)
	monitoringEnabled := configSafe.IsMonitoringEnabled()

	global.LOGGER.InfoKV("服务器启动配置",
		"environment", cfg.Environment,
		"debug", cfg.Debug,
		"metrics_enabled", monitoringEnabled)

	ctx, cancel := context.WithCancel(context.Background())

	server := &Server{
		config:        cfg,
		configSafe:    configSafe, // 初始化安全配置访问器
		ctx:           ctx,
		cancel:        cancel,
		bannerManager: NewBannerManager(cfg),
	}

	// 初始化功能管理器
	server.featureManager = NewFeatureManager(server)

	// 初始化全局配置和核心组件
	if err := server.initCore(); err != nil {
		cancel()
		return nil, errors.NewErrorf(errors.ErrCodeInternalServerError, "failed to init core: %v", err)
	}

	// 初始化中间件管理器
	if err := server.initMiddleware(); err != nil {
		cancel()
		return nil, errors.NewErrorf(errors.ErrCodeInternalServerError, "failed to init middleware: %v", err)
	}

	// 初始化服务器组件
	if err := server.initServers(); err != nil {
		cancel()
		return nil, errors.NewErrorf(errors.ErrCodeInternalServerError, "failed to init servers: %v", err)
	}

	return server, nil
}

// GetConfig 获取配置
func (s *Server) GetConfig() *gwconfig.Gateway {
	return s.config
}

// GetMiddlewareManager 获取中间件管理器
func (s *Server) GetMiddlewareManager() *middleware.Manager {
	return s.middlewareManager
}

// GetBannerManager 获取Banner管理器
func (s *Server) GetBannerManager() *BannerManager {
	return s.bannerManager
}

// GetFeatureManager 获取功能管理器
func (s *Server) GetFeatureManager() *FeatureManager {
	return s.featureManager
}

// GetPoolManager 获取连接池管理器
func (s *Server) GetPoolManager() cpool.PoolManager {
	return s.poolManager
}

// GetWebSocketService 获取 WebSocket 服务
func (s *Server) GetWebSocketService() *WebSocketService {
	return s.webSocketService
}

// EnableFeature 启用指定功能（使用配置中的默认设置）
func (s *Server) EnableFeature(feature FeatureType) error {
	return s.featureManager.Enable(feature)
}

// EnableFeatureWithConfig 使用自定义配置启用功能
func (s *Server) EnableFeatureWithConfig(feature FeatureType, config interface{}) error {
	return s.featureManager.EnableWithConfig(feature, config)
}

// IsFeatureEnabled 检查功能是否已启用
func (s *Server) IsFeatureEnabled(feature FeatureType) bool {
	return s.featureManager.IsEnabled(feature)
}

// RegisterGRPCService 注册gRPC服务
func (s *Server) RegisterGRPCService(registerFunc func(*grpc.Server)) {
	if s.grpcServer != nil {
		registerFunc(s.grpcServer)
	}
}

// AddGrpcGatewayMiddleware 添加 gRPC-Gateway 中间件
// 注意：必须在 initHTTPGateway 之前调用
func (s *Server) AddGrpcGatewayMiddleware(mw runtime.Middleware) {
	s.grpcGatewayMiddlewares = append(s.grpcGatewayMiddlewares, mw)
}

// AddGrpcGatewayMiddlewareProvider 添加 gRPC-Gateway 中间件提供器
// 提供器会在 initHTTPGateway 时被调用，适用于需要在 Build 后才能创建的中间件
func (s *Server) AddGrpcGatewayMiddlewareProvider(provider func() []runtime.Middleware) {
	s.grpcGatewayMiddlewareProviders = append(s.grpcGatewayMiddlewareProviders, provider)
}
