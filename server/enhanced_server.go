/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-13 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 05:31:09
 * @FilePath: \go-rpc-gateway\server\enhanced_server.go
 * @Description: 增强的服务器实现，集成业务服务注入能力
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"runtime/debug"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/validator"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/cpool"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

const TraceID = "trace_id"

// EnhancedServer 增强的服务器 - 继承基础Server并添加业务服务注入能力
type EnhancedServer struct {
	*Server                               // 继承基础Server
	
	// 业务服务注入管理器
	businessManager *BusinessInjectionManager
	
	// Prometheus监控
	promReg       *prometheus.Registry    // Prometheus注册器
	metricsServer *http.Server           // Metrics服务器
	
	// gRPC增强功能
	serverMetrics *grpc_prometheus.ServerMetrics  // 服务端metrics
	clientMetrics *grpc_prometheus.ClientMetrics  // 客户端metrics
	panicsTotal   prometheus.Counter               // panic计数器
	
	// 中间件链
	httpMiddlewares []middleware.MiddlewareFunc    // HTTP中间件链
	
	// 服务注册
	grpcServices []func(*grpc.Server)              // gRPC服务注册函数
	httpHandlers map[string]http.Handler          // HTTP处理器
	
	// 增强配置
	dialOptions  []grpc.DialOption                // gRPC拨号选项
}

// NewEnhancedServer 创建增强的服务器实例
func NewEnhancedServer() (*EnhancedServer, error) {
	// 创建基础服务器
	baseServer, err := NewServer()
	if err != nil {
		return nil, fmt.Errorf("创建基础服务器失败: %w", err)
	}
	
	config := global.GetConfig()
	if config == nil {
		return nil, fmt.Errorf("获取配置失败，请确保已初始化")
	}
	
	// 创建增强服务器
	enhanced := &EnhancedServer{
		Server:          baseServer,
		businessManager: NewBusinessInjectionManager(), // 业务服务注入管理器
		promReg:         prometheus.NewRegistry(),
		httpHandlers:    make(map[string]http.Handler),
	}
	
	// 初始化OpenTelemetry追踪
	if err := enhanced.initOpenTelemetry(); err != nil {
		return nil, fmt.Errorf("初始化OpenTelemetry失败: %w", err)
	}
	
	// 初始化Prometheus监控
	if err := enhanced.initPrometheusMetrics(); err != nil {
		return nil, fmt.Errorf("初始化Prometheus监控失败: %w", err)
	}
	
	// 初始化中间件链
	enhanced.initMiddlewares()
	
	// 初始化gRPC拨号选项
	enhanced.initDialOptions()
	
	global.GetLogger().Info("增强服务器创建成功")
	return enhanced, nil
}

// RegisterBusinessService 注册业务服务 - 新增的业务服务注入功能
func (es *EnhancedServer) RegisterBusinessService(name string, service BusinessServiceProvider) error {
	return es.businessManager.RegisterBusinessService(name, service)
}

// UnregisterBusinessService 注销业务服务
func (es *EnhancedServer) UnregisterBusinessService(name string) error {
	return es.businessManager.UnregisterBusinessService(name)
}

// GetBusinessService 获取业务服务
func (es *EnhancedServer) GetBusinessService(name string) (BusinessServiceProvider, error) {
	return es.businessManager.GetBusinessService(name)
}

// ListBusinessServices 列出所有业务服务
func (es *EnhancedServer) ListBusinessServices() map[string]BusinessServiceProvider {
	return es.businessManager.ListBusinessServices()
}

// GetBusinessServiceStatus 获取业务服务状态
func (es *EnhancedServer) GetBusinessServiceStatus() map[string]bool {
	return es.businessManager.GetServiceStatus()
}

// GetBusinessServiceCount 获取业务服务数量
func (es *EnhancedServer) GetBusinessServiceCount() int {
	return es.businessManager.GetServiceCount()
}

// initOpenTelemetry 初始化OpenTelemetry追踪
func (es *EnhancedServer) initOpenTelemetry() error {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, 
		propagation.Baggage{},
	))
	
	global.GetLogger().Info("OpenTelemetry初始化完成")
	return nil
}

// initPrometheusMetrics 初始化Prometheus监控指标
func (es *EnhancedServer) initPrometheusMetrics() error {
	// 服务端metrics
	es.serverMetrics = grpc_prometheus.NewServerMetrics(
		grpc_prometheus.WithServerHandlingTimeHistogram(
			grpc_prometheus.WithHistogramBuckets([]float64{
				0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120,
			}),
		),
	)
	es.promReg.MustRegister(es.serverMetrics)
	
	// 客户端metrics
	es.clientMetrics = grpc_prometheus.NewClientMetrics(
		grpc_prometheus.WithClientHandlingTimeHistogram(
			grpc_prometheus.WithHistogramBuckets([]float64{
				0.001, 0.01, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20, 30, 60, 90, 120,
			}),
		),
	)
	es.promReg.MustRegister(es.clientMetrics)
	
	// panic恢复计数器
	es.panicsTotal = promauto.With(es.promReg).NewCounter(prometheus.CounterOpts{
		Name: "grpc_req_panics_recovered_total",
		Help: "Total number of gRPC requests recovered from internal panic.",
	})
	
	global.GetLogger().Info("Prometheus监控指标初始化完成")
	return nil
}

// initMiddlewares 初始化中间件链
func (es *EnhancedServer) initMiddlewares() {
	// 使用基础Server的中间件管理器
	mgr := es.Server.GetMiddlewareManager()
	if mgr != nil {
		// 根据环境选择中间件链
		if es.isProductionMode() {
			es.httpMiddlewares = mgr.GetProductionMiddlewares()
		} else {
			es.httpMiddlewares = mgr.GetDevelopmentMiddlewares()
		}
	}
	
	global.GetLogger().Info("中间件链初始化完成", "middleware_count", len(es.httpMiddlewares))
}

// initDialOptions 初始化gRPC拨号选项
func (es *EnhancedServer) initDialOptions() {
	config := global.GetConfig()
	
	// 基础拨号选项
	es.dialOptions = []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(config.GRPC.Server.MaxRecvMsgSize*1024*1024),
			grpc.MaxCallSendMsgSize(config.GRPC.Server.MaxSendMsgSize*1024*1024),
		),
	}
	
	// 添加客户端拦截器
	exemplarFromContext := func(ctx context.Context) prometheus.Labels {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return prometheus.Labels{TraceID: span.TraceID().String()}
		}
		return nil
	}
	
	logTraceID := func(ctx context.Context) grpc_logging.Fields {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return grpc_logging.Fields{TraceID, span.TraceID().String()}
		}
		return nil
	}
	
	interceptorLogger := func(l logger.ILogger) grpc_logging.Logger {
		return grpc_logging.LoggerFunc(func(ctx context.Context, lvl grpc_logging.Level, msg string, fields ...any) {
			switch lvl {
			case grpc_logging.LevelDebug:
				l.DebugKV(msg, fields...)
			case grpc_logging.LevelInfo:
				l.InfoKV(msg, fields...)
			case grpc_logging.LevelWarn:
				l.WarnKV(msg, fields...)
			case grpc_logging.LevelError:
				l.ErrorKV(msg, fields...)
			}
		})
	}
	
	// 添加客户端拦截器链
	es.dialOptions = append(es.dialOptions,
		grpc.WithChainStreamInterceptor(
			es.clientMetrics.StreamClientInterceptor(
				grpc_prometheus.WithExemplarFromContext(exemplarFromContext),
			),
			grpc_logging.StreamClientInterceptor(
				interceptorLogger(global.GetLogger()),
				grpc_logging.WithFieldsFromContext(logTraceID),
			),
		),
		grpc.WithChainUnaryInterceptor(
			es.clientMetrics.UnaryClientInterceptor(
				grpc_prometheus.WithExemplarFromContext(exemplarFromContext),
			),
			grpc_validator.UnaryClientInterceptor(),
			grpc_logging.UnaryClientInterceptor(
				interceptorLogger(global.GetLogger()),
				grpc_logging.WithFieldsFromContext(logTraceID),
			),
		),
	)
}

// RegisterGRPCService 注册gRPC服务 (增强版本)
func (es *EnhancedServer) RegisterGRPCService(serviceFunc func(*grpc.Server)) {
	if serviceFunc != nil {
		es.grpcServices = append(es.grpcServices, serviceFunc)
		global.GetLogger().Info("gRPC服务已注册")
	}
}

// RegisterHTTPHandler 注册HTTP处理器 (增强版本)
func (es *EnhancedServer) RegisterHTTPHandler(pattern string, handler http.Handler) {
	if handler != nil {
		es.httpHandlers[pattern] = handler
		global.GetLogger().Info("HTTP处理器已注册", "pattern", pattern)
	}
}

// RegisterGatewayHandler 注册Gateway处理器
func (es *EnhancedServer) RegisterGatewayHandler(ctx context.Context, registerFunc func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error) error {
	config := global.GetConfig()
	grpcEndpoint := config.GRPC.Server.GetEndpoint()
	
	if err := registerFunc(ctx, es.gwMux, grpcEndpoint, es.dialOptions); err != nil {
		return fmt.Errorf("注册Gateway处理器失败: %w", err)
	}
	
	global.GetLogger().Info("Gateway处理器注册成功", "grpc_endpoint", grpcEndpoint)
	return nil
}

// StartEnhanced 启动增强的服务器
func (es *EnhancedServer) StartEnhanced() error {
	ctx := global.GetContext()
	
	// 启动业务服务管理器
	if err := es.businessManager.StartAllBusinessServices(); err != nil {
		return fmt.Errorf("启动业务服务管理器失败: %w", err)
	}
	
	// 启动gRPC服务器
	if err := es.startGRPCServer(ctx); err != nil {
		return fmt.Errorf("启动gRPC服务器失败: %w", err)
	}
	
	// 启动HTTP服务器  
	if err := es.startHTTPServer(ctx); err != nil {
		return fmt.Errorf("启动HTTP服务器失败: %w", err)
	}
	
	// 启动Metrics服务器(如果配置了)
	config := global.GetConfig()
	if config.Monitoring != nil && config.Monitoring.Enabled {
		if err := es.startMetricsServer(ctx); err != nil {
			return fmt.Errorf("启动Metrics服务器失败: %w", err)
		}
	}
	
	return nil
}

// startGRPCServer 启动gRPC服务器 - 集成业务服务注入
func (es *EnhancedServer) startGRPCServer(ctx context.Context) error {
	config := global.GetConfig()
	
	// panic恢复处理器
	panicRecoveryHandler := func(p any) (err error) {
		es.panicsTotal.Inc()
		global.GetLogger().Error("从panic中恢复", 
			"panic", p, 
			"stack", string(debug.Stack()))
		return status.Errorf(codes.Internal, "%s", p)
	}
	
	// 日志和追踪辅助函数
	logTraceID := func(ctx context.Context) grpc_logging.Fields {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return grpc_logging.Fields{TraceID, span.TraceID().String()}
		}
		return nil
	}
	
	interceptorLogger := func(l logger.ILogger) grpc_logging.Logger {
		return grpc_logging.LoggerFunc(func(ctx context.Context, lvl grpc_logging.Level, msg string, fields ...any) {
			switch lvl {
			case grpc_logging.LevelDebug:
				l.DebugKV(msg, fields...)
			case grpc_logging.LevelInfo:
				l.InfoKV(msg, fields...)
			case grpc_logging.LevelWarn:
				l.WarnKV(msg, fields...)
			case grpc_logging.LevelError:
				l.ErrorKV(msg, fields...)
			default:
				l.InfoKV(msg, fields...)
			}
		})
	}
	
	exemplarFromContext := func(ctx context.Context) prometheus.Labels {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return prometheus.Labels{TraceID: span.TraceID().String()}
		}
		return nil
	}
	
	// gRPC服务器选项
	var serverOptions = []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainStreamInterceptor(
			es.serverMetrics.StreamServerInterceptor(
				grpc_prometheus.WithExemplarFromContext(exemplarFromContext),
			),
			grpc_validator.StreamServerInterceptor(),
			grpc_logging.StreamServerInterceptor(
				interceptorLogger(global.GetLogger()), 
				grpc_logging.WithFieldsFromContext(logTraceID),
			),
			grpc_recovery.StreamServerInterceptor(
				grpc_recovery.WithRecoveryHandler(panicRecoveryHandler),
			),
		),
		grpc.ChainUnaryInterceptor(
			es.serverMetrics.UnaryServerInterceptor(
				grpc_prometheus.WithExemplarFromContext(exemplarFromContext),
			),
			grpc_validator.UnaryServerInterceptor(),
			grpc_logging.UnaryServerInterceptor(
				interceptorLogger(global.GetLogger()), 
				grpc_logging.WithFieldsFromContext(logTraceID),
			),
			grpc_recovery.UnaryServerInterceptor(
				grpc_recovery.WithRecoveryHandler(panicRecoveryHandler),
			),
		),
		grpc.MaxRecvMsgSize(config.GRPC.Server.MaxRecvMsgSize * 1024 * 1024),
		grpc.MaxSendMsgSize(config.GRPC.Server.MaxSendMsgSize * 1024 * 1024),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     time.Hour,
			MaxConnectionAge:      time.Hour,
			MaxConnectionAgeGrace: time.Minute * 10,
			Time:                  time.Minute * 10,
			Timeout:               time.Second * 30,
		}),
	}
	
	// 创建gRPC服务器
	grpcServer := grpc.NewServer(serverOptions...)
	es.serverMetrics.InitializeMetrics(grpcServer)
	
	// 注册传统的gRPC服务
	for _, serviceFunc := range es.grpcServices {
		serviceFunc(grpcServer)
	}
	
	// 注册所有业务服务的gRPC接口 - 业务服务注入核心功能
	es.businessManager.RegisterAllGRPCServices(grpcServer)
	
	// 更新基础Server的gRPC服务器
	es.Server.grpcServer = grpcServer
	
	// 启动监听器
	endpoint := config.GRPC.Server.GetEndpoint()
	lis, err := net.Listen("tcp", endpoint)
	if err != nil {
		return fmt.Errorf("监听失败: %w", err)
	}
	
	global.GetLogger().Info("gRPC服务器启动", 
		"endpoint", endpoint,
		"business_services", es.businessManager.GetServiceCount())
	
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			global.GetLogger().Error("gRPC服务器关闭", "error", err)
		}
	}()
	
	return nil
}

// startHTTPServer 启动HTTP服务器
func (es *EnhancedServer) startHTTPServer(ctx context.Context) error {
	config := global.GetConfig()
	
	// 创建HTTP多路复用器
	mux := http.NewServeMux()
	
	// 注册pprof处理器
	if config.Debug {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		global.GetLogger().Info("pprof调试端点已启用")
	}
	
	// 注册业务服务状态端点
	mux.HandleFunc("/business/services/status", es.handleBusinessServiceStatus)
	mux.HandleFunc("/business/services/list", es.handleBusinessServiceList)
	
	// 注册自定义HTTP处理器
	for pattern, handler := range es.httpHandlers {
		mux.Handle(pattern, handler)
	}
	
	// 注册gateway mux作为默认处理器
	mux.Handle("/", es.Server.gwMux)
	
	// 应用中间件链
	var finalHandler http.Handler = mux
	if len(es.httpMiddlewares) > 0 {
		finalHandler = middleware.ApplyMiddlewares(mux, es.httpMiddlewares...)
	}
	
	// 创建HTTP服务器
	endpoint := config.HTTPServer.GetEndpoint()
	es.Server.httpServer = &http.Server{
		Addr:         endpoint,
		Handler:      finalHandler,
		ReadTimeout:  time.Duration(config.HTTPServer.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.HTTPServer.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(config.HTTPServer.IdleTimeout) * time.Second,
	}
	
	global.GetLogger().Info("HTTP服务器启动", "endpoint", endpoint)
	
	go func() {
		if err := es.Server.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			global.GetLogger().Error("HTTP服务器关闭", "error", err)
		}
	}()
	
	return nil
}

// startMetricsServer 启动Metrics服务器
func (es *EnhancedServer) startMetricsServer(ctx context.Context) error {
	config := global.GetConfig()
	if config.Monitoring == nil || config.Monitoring.Prometheus == nil {
		global.GetLogger().Info("Prometheus配置为空，跳过Metrics服务器启动")
		return nil
	}
	
	// 构建metrics端点
	endpoint := config.Monitoring.Prometheus.Endpoint
	if endpoint == "" {
		endpoint = "0.0.0.0"
	}
	endpoint = fmt.Sprintf("%s:%d", endpoint, config.Monitoring.Prometheus.Port)

	es.metricsServer = &http.Server{
		Addr: endpoint,
		Handler: promhttp.HandlerFor(
			es.promReg,
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
			},
		),
	}
	
	global.GetLogger().Info("Metrics服务器启动", "endpoint", endpoint)
	
	go func() {
		if err := es.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			global.GetLogger().Error("Metrics服务器关闭", "error", err)
		}
	}()
	
	return nil
}

// StopEnhanced 优雅停止增强服务器
func (es *EnhancedServer) StopEnhanced() error {
	global.GetLogger().Info("开始停止增强服务器")
	
	// 停止业务服务管理器
	if err := es.businessManager.StopAllBusinessServices(); err != nil {
		global.GetLogger().Error("停止业务服务管理器失败", "error", err)
	}
	
	// 停止gRPC服务器
	if es.Server.grpcServer != nil {
		global.GetLogger().Info("停止gRPC服务器")
		es.Server.grpcServer.GracefulStop()
	}
	
	// 停止HTTP服务器
	if es.Server.httpServer != nil {
		global.GetLogger().Info("停止HTTP服务器")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := es.Server.httpServer.Shutdown(ctx); err != nil {
			global.GetLogger().Error("停止HTTP服务器失败", "error", err)
		}
	}
	
	// 停止Metrics服务器
	if es.metricsServer != nil {
		global.GetLogger().Info("停止Metrics服务器")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := es.metricsServer.Shutdown(ctx); err != nil {
			global.GetLogger().Error("停止Metrics服务器失败", "error", err)
		}
	}
	
	global.GetLogger().Info("增强服务器停止完成")
	return nil
}

// handleBusinessServiceStatus 处理业务服务状态请求
func (es *EnhancedServer) handleBusinessServiceStatus(w http.ResponseWriter, r *http.Request) {
	status := es.businessManager.GetServiceStatus()
	
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": %t, "services": %v}`, 
		es.businessManager.IsRunning(), 
		status)
}

// handleBusinessServiceList 处理业务服务列表请求
func (es *EnhancedServer) handleBusinessServiceList(w http.ResponseWriter, r *http.Request) {
	services := es.businessManager.ListBusinessServices()
	serviceNames := make([]string, 0, len(services))
	
	for name := range services {
		serviceNames = append(serviceNames, name)
	}
	
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"count": %d, "services": %v}`, len(serviceNames), serviceNames)
}

// GetPrometheusRegistry 获取Prometheus注册器
func (es *EnhancedServer) GetPrometheusRegistry() *prometheus.Registry {
	return es.promReg
}

// GetDialOptions 获取gRPC拨号选项
func (es *EnhancedServer) GetDialOptions() []grpc.DialOption {
	return es.dialOptions
}

// GetPoolManager 获取连接池管理器
func (es *EnhancedServer) GetPoolManager() cpool.PoolManager {
	return es.Server.GetPoolManager()
}

// GetBusinessInjectionManager 获取业务服务注入管理器
func (es *EnhancedServer) GetBusinessInjectionManager() *BusinessInjectionManager {
	return es.businessManager
}

// isProductionMode 检查是否为生产模式
func (es *EnhancedServer) isProductionMode() bool {
	config := global.GetConfig()
	return config.Environment == "production" || config.Environment == "prod"
}