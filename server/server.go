/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-02 09:29:55
 * @FilePath: \go-rpc-gateway\server\server.go
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
	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/cpool"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/kamalyes/go-toolbox/pkg/desensitize"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Server Gateway服务器
type Server struct {
	config *gwconfig.Gateway

	// 服务器组件
	grpcServer  *grpc.Server
	httpServer  *http.Server
	gwMux       *runtime.ServeMux
	pprofServer *middleware.PProfServer
	httpMux     *http.ServeMux // 添加HTTP路由管理器

	// 命名监听器（多端口支持，如 Ops/Tenant 分离）
	namedListeners map[string]*namedListener

	// 中间件管理器
	middlewareManager *middleware.Manager

	// grpc-gateway 中间件（runtime.Middleware）
	grpcGatewayMiddlewares         []runtime.Middleware
	grpcGatewayMiddlewareProviders []func() []runtime.Middleware // 中间件提供器

	// 健康检查管理器
	healthManager *middleware.HealthManager

	// Banner管理器
	bannerManager *BannerManager

	// 连接池管理器
	poolManager cpool.PoolManager

	// 端点信息收集器
	endpointCollector *EndpointCollector

	// Gzip writer 对象池（用于 HTTP 压缩优化）
	gzipWriterPool *sync.Pool

	// Gzip 跳过规则（预处理，避免每次请求读配置切片）
	//
	// 前缀用切片而不是 map：前缀匹配必须逐个 HasPrefix，map 给不了 O(1)，
	// 反而每次 range 都要付 map 迭代的随机化起点与指针跳转
	// 扩展名在「每项都是单个点开头」时另建一张 map，配合取一次扩展名做单次查找；
	// 否则退回逐个 HasSuffix，保证 .min.js 这类多点配置的语义不变
	//
	// [EN] Prefixes live in a slice rather than a map: prefix matching has to run HasPrefix
	// item by item, so a map buys no O(1) lookup and instead charges map-iteration overhead
	// on every request. Extensions additionally get a map when every entry is a single
	// dot-prefixed suffix, which makes one extraction plus one lookup exactly equivalent;
	// otherwise the suffix loop stays, so a multi-dot entry keeps its meaning
	gzipSkipPrefixes  []string
	gzipSkipSuffixes  []string
	gzipSkipExtMap    map[string]struct{}
	httpRoutePatterns map[string]struct{}

	// 数据脱敏器（用于日志敏感数据脱敏）
	dataMasker *desensitize.DataMasker

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
	cfg := global.GetGatewayConfig()
	if cfg == nil {
		return nil, errors.NewError(errors.ErrCodeInvalidConfiguration, "global GATEWAY config is not initialized")
	}

	// 记录环境配置应用情况
	monitoringEnabled := cfg.Monitoring != nil && cfg.Monitoring.Enabled

	global.LOGGER.InfoKV("服务器启动配置",
		"environment", cfg.Environment,
		"debug", cfg.Debug,
		"metrics_enabled", monitoringEnabled)

	ctx, cancel := context.WithCancel(context.Background())

	server := &Server{
		config:        cfg,
		ctx:           ctx,
		cancel:        cancel,
		bannerManager: NewBannerManager(cfg).WithContext(ctx),
	}

	// 初始化 Gzip writer 对象池（从配置读取压缩级别）
	server.initGzipWriterPool()

	// 初始化数据脱敏器（从配置读取敏感字段）
	server.initDataMasker()

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

// GetPoolManager 获取连接池管理器
func (s *Server) GetPoolManager() cpool.PoolManager {
	return s.poolManager
}

// GetEndpointCollector 获取端点收集器
func (s *Server) GetEndpointCollector() *EndpointCollector {
	return s.endpointCollector
}

// GetDataMasker 获取数据脱敏器
func (s *Server) GetDataMasker() *desensitize.DataMasker {
	return s.dataMasker
}

// GetGRPCServer 获取gRPC服务器实例
func (s *Server) GetGRPCServer() *grpc.Server {
	return s.grpcServer
}

// GetEndpoint 获取gRPC服务端点地址
func (s *Server) GetEndpoint() string {
	if s.config == nil || s.config.GRPC == nil || s.config.GRPC.Server == nil {
		return ""
	}
	return s.config.GRPC.Server.GetEndpoint()
}

// GetContext 获取服务器上下文
func (s *Server) GetContext() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

// GetDialOptions 获取gRPC客户端拨号选项
func (s *Server) GetDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// 注册 otelgrpc StatsHandler，确保 fallback 拨号路径也能传播 OTel trace context
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}
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
