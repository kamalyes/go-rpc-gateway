/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 13:40:31
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
	"github.com/kamalyes/go-core/pkg/global"
	logger "github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/config"
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
	httpMux    *http.ServeMux // 添加HTTP路由管理器

	// 中间件管理器
	middlewareManager *middleware.Manager

	// 健康检查管理器
	healthManager *middleware.HealthManager

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

	// 确保全局日志器被初始化
	if err := ensureLoggerInitialized(cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// 记录环境配置应用情况
	global.LOGGER.InfoKV("服务器启动配置",
		"environment", cfg.Gateway.Environment,
		"debug", cfg.Gateway.Debug,
		"tls_enabled", cfg.Security.TLS.Enabled,
		"metrics_enabled", cfg.Monitoring.Metrics.Enabled)

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

// ensureLoggerInitialized 确保全局日志器被正确初始化
func ensureLoggerInitialized(cfg *config.GatewayConfig) error {
	// 如果全局日志器已经初始化，直接返回
	if global.LOGGER != nil {
		return nil
	}

	// 使用 go-logger 创建一个新的日志器实例
	// 根据配置设置日志级别
	level := logger.INFO
	if cfg.SingleConfig != nil && cfg.SingleConfig.Zap.Level != "" {
		switch cfg.SingleConfig.Zap.Level {
		case "debug":
			level = logger.DEBUG
		case "info":
			level = logger.INFO
		case "warn":
			level = logger.WARN
		case "error":
			level = logger.ERROR
		}
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
