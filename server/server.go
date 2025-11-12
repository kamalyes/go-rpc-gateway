/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 13:58:19
 * @FilePath: \go-rpc-gateway\server\server.go
 * @Description: Gateway服务器核心结构定义
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	logger "github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Server Gateway服务器
type Server struct {
	config *gwconfig.Gateway

	// 服务器组件
	grpcServer *grpc.Server
	httpServer *http.Server
	gwMux      *runtime.ServeMux
	httpMux    *http.ServeMux // 添加HTTP路由管理器

	// 中间件管理器
	middlewareManager *middleware.Manager

	// 健康检查管理器
	healthManager *middleware.HealthManager

	// Banner管理器
	bannerManager *BannerManager

	// 功能管理器
	featureManager *FeatureManager

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
		return nil, fmt.Errorf("global GATEWAY config is not initialized")
	}

	// 确保全局日志器被初始化
	if err := ensureLoggerInitialized(cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// 记录环境配置应用情况
	global.LOGGER.InfoKV("服务器启动配置",
		"environment", cfg.Environment,
		"debug", cfg.Debug,
		"metrics_enabled", cfg.Monitoring.Enabled)

	ctx, cancel := context.WithCancel(context.Background())

	server := &Server{
		config:        cfg,
		ctx:           ctx,
		cancel:        cancel,
		bannerManager: NewBannerManager(cfg),
	}

	// 初始化功能管理器
	server.featureManager = NewFeatureManager(server)

	// 初始化全局配置和核心组件
	if err := server.initCore(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to init core: %w", err)
	}

	// 初始化中间件管理器
	if err := server.initMiddleware(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to init middleware: %w", err)
	}

	// 初始化服务器组件
	if err := server.initServers(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to init servers: %w", err)
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

// RegisterHTTPHandler 注册HTTP处理器到网关
func (s *Server) RegisterHTTPHandler(ctx context.Context, registerFunc func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error) error {
	grpcAddress := s.config.GRPC.Server.GetEndpoint()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	return registerFunc(ctx, s.gwMux, grpcAddress, opts)
}

// ensureLoggerInitialized 确保全局日志器被正确初始化
func ensureLoggerInitialized(cfg *gwconfig.Gateway) error {
	// 如果全局日志器已经初始化，直接返回
	if global.LOGGER != nil {
		return nil
	}

	// 使用 go-logger 创建一个新的日志器实例
	// 使用默认日志级别，具体配置交给 go-config 管理
	level := logger.INFO
	if cfg.Debug {
		level = logger.DEBUG
	}

	// 创建一个简单的 logger 实例
	newLogger := logger.CreateSimpleLogger(level)
	if newLogger == nil {
		return fmt.Errorf("failed to create logger instance")
	}

	// 将新创建的 logger 赋值给全局变量
	global.LOGGER = newLogger

	fmt.Println("[INFO] Logger initialized successfully with go-logger")
	return nil
}
