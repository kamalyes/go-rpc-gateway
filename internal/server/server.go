/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 13:40:31
 * @FilePath: \go-rpc-gateway\internal\server\server.go
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
	"github.com/kamalyes/go-rpc-gateway/internal/config"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Server Gateway服务器
type Server struct {
	config        *config.GatewayConfig
	configManager *config.ConfigManager

	// 服务器组件
	grpcServer *grpc.Server
	httpServer *http.Server
	gwMux      *runtime.ServeMux
	httpMux    *http.ServeMux  // 添加HTTP路由管理器

	// 中间件管理器
	middlewareManager *middleware.Manager
	
	// Banner管理器
	bannerManager *BannerManager

	// 状态管理
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 运行状态
	running bool
	mu      sync.RWMutex
}

// NewServer 创建新的Gateway服务器
func NewServer(cfg *config.GatewayConfig) (*Server, error) {
	if cfg == nil {
		cfg = config.DefaultGatewayConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	server := &Server{
		config:        cfg,
		ctx:           ctx,
		cancel:        cancel,
		bannerManager: NewBannerManager(cfg),
	}

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

// NewServerWithConfigManager 使用配置管理器创建服务器
func NewServerWithConfigManager(configManager *config.ConfigManager) (*Server, error) {
	server, err := NewServer(configManager.GetConfig())
	if err != nil {
		return nil, err
	}

	server.configManager = configManager

	// 启用配置热重载
	configManager.WatchConfig(server.onConfigChanged)

	return server, nil
}

// GetConfig 获取配置
func (s *Server) GetConfig() *config.GatewayConfig {
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

// RegisterGRPCService 注册gRPC服务
func (s *Server) RegisterGRPCService(registerFunc func(*grpc.Server)) {
	if s.grpcServer != nil {
		registerFunc(s.grpcServer)
	}
}

// RegisterHTTPHandler 注册HTTP处理器到网关
func (s *Server) RegisterHTTPHandler(ctx context.Context, registerFunc func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error) error {
	grpcAddress := fmt.Sprintf("%s:%d", s.config.Gateway.GRPC.Host, s.config.Gateway.GRPC.Port)
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	return registerFunc(ctx, s.gwMux, grpcAddress, opts)
}
