/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-18 12:02:51
 * @FilePath: \go-rpc-gateway\server\http.go
 * @Description: HTTP服务器和网关初始化模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/kamalyes/go-rpc-gateway/response"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/protobuf/encoding/protojson"
)

// buildServeMuxOptions 构建ServeMux选项，支持从配置文件读取JSON序列化配置
func (s *Server) buildServeMuxOptions() []runtime.ServeMuxOption {
	jsonSafe := s.configSafe.Field("JSON")

	// 读取配置，使用默认值
	useProtoNames := jsonSafe.Field("UseProtoNames").Bool(true)
	emitUnpopulated := jsonSafe.Field("EmitUnpopulated").Bool(true)
	discardUnknown := jsonSafe.Field("DiscardUnknown").Bool(true)

	return []runtime.ServeMuxOption{
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				UseProtoNames:   useProtoNames,   // 使用 proto 字段名（snake_case）
				EmitUnpopulated: emitUnpopulated, // 输出所有字段，包括零值
			},
			UnmarshalOptions: protojson.UnmarshalOptions{
				DiscardUnknown: discardUnknown, // 忽略未知字段
			},
		}),
	}
}

// gzipResponseWriter 包装ResponseWriter以支持gzip压缩
type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// gzipMiddleware HTTP Gzip压缩中间件
func (s *Server) gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查是否启用压缩 - 使用安全访问
		if !s.configSafe.Field("HTTPServer").Field("EnableGzipCompress").Bool(false) {
			next.ServeHTTP(w, r)
			return
		}

		// 检查客户端是否支持gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// 设置响应头
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")

		// 创建gzip writer
		gzipWriter := gzip.NewWriter(w)
		defer gzipWriter.Close()

		// 包装ResponseWriter
		grw := gzipResponseWriter{Writer: gzipWriter, ResponseWriter: w}
		next.ServeHTTP(grw, r)
	})
}

// initHTTPGateway 初始化HTTP网关
func (s *Server) initHTTPGateway() error {
	// 创建gRPC-Gateway多路复用器，配置JSON序列化选项
	opts := s.buildServeMuxOptions()
	s.gwMux = runtime.NewServeMux(opts...)

	// 创建HTTP多路复用器
	s.httpMux = http.NewServeMux()

	// 注册网关路由（默认路由到gwMux）
	s.httpMux.Handle("/", s.gwMux)

	// 注册健康检查 - 使用安全访问
	if s.configSafe.IsHealthEnabled() {
		healthPath := s.configSafe.GetHealthPath("/health")
		s.httpMux.HandleFunc(healthPath, s.healthCheckHandler)

		httpEndpoint := s.configSafe.Field("HTTPServer").Field("Host").String("0.0.0.0") + ":" +
			string(rune(s.configSafe.Field("HTTPServer").Field("Port").Int(8080)))
		global.LOGGER.InfoKV("❤️  健康检查已启用",
			"url", "http://"+httpEndpoint+healthPath)

		// 注册组件级健康检查端点
		s.registerComponentHealthChecks()
	}

	// 注册监控指标端点 - 使用安全访问
	if s.configSafe.IsMetricsEnabled() {
		prometheusPath := s.configSafe.GetMetricsEndpoint("/metrics")
		s.httpMux.Handle(prometheusPath, promhttp.Handler())

		httpEndpoint := s.configSafe.Field("HTTPServer").Field("Host").String("0.0.0.0") + ":" +
			string(rune(s.configSafe.Field("HTTPServer").Field("Port").Int(8080)))
		global.LOGGER.InfoKV("📊 监控指标服务可用",
			"url", "http://"+httpEndpoint+prometheusPath)
	}

	// 应用中间件
	var handler http.Handler = s.httpMux

	// 首先应用Gzip压缩中间件（如果启用）
	if s.configSafe.Field("HTTPServer").Field("EnableGzipCompress").Bool(false) {
		handler = s.gzipMiddleware(handler)
		global.LOGGER.InfoMsg("✅ HTTP Gzip压缩已启用")
	}

	if s.middlewareManager != nil {
		var middlewares []middleware.MiddlewareFunc
		if s.configSafe.Field("Debug").Bool(false) {
			middlewares = s.middlewareManager.GetDevelopmentMiddlewares()
		} else {
			middlewares = s.middlewareManager.GetDefaultMiddlewares()
		}
		handler = middleware.ApplyMiddlewares(handler, middlewares...)
	}

	// 创建HTTP服务器 - 使用安全访问
	httpSafe := s.configSafe.Field("HTTPServer")
	s.httpServer = &http.Server{
		Addr:           fmt.Sprintf("%s:%d", httpSafe.Field("Host").String("0.0.0.0"), httpSafe.Field("Port").Int(8080)),
		Handler:        handler,
		ReadTimeout:    time.Duration(httpSafe.Field("ReadTimeout").Int(30)) * time.Second,
		WriteTimeout:   time.Duration(httpSafe.Field("WriteTimeout").Int(30)) * time.Second,
		IdleTimeout:    time.Duration(httpSafe.Field("IdleTimeout").Int(60)) * time.Second,
		MaxHeaderBytes: httpSafe.Field("MaxHeaderBytes").Int(1048576), // 1MB
	}

	return nil
}

// registerComponentHealthChecks 注册组件级健康检查端点
func (s *Server) registerComponentHealthChecks() {
	httpSafe := s.configSafe.Field("HTTPServer")
	baseURL := fmt.Sprintf("http://%s:%d",
		httpSafe.Field("Host").String("0.0.0.0"),
		httpSafe.Field("Port").Int(8080))

	// 注册Redis健康检查
	healthSafe := s.configSafe.Field("Health")
	if healthSafe.Field("Redis").Field("Enabled").Bool(false) {
		redisPath := healthSafe.Field("Redis").Field("Path").String("/health/redis")
		s.httpMux.HandleFunc(redisPath, s.redisHealthCheckHandler)
		global.LOGGER.InfoKV("🔴 Redis健康检查已启用",
			"url", baseURL+redisPath)
	}

	// 注册MySQL健康检查
	if healthSafe.Field("MySQL").Field("Enabled").Bool(false) {
		mysqlPath := healthSafe.Field("MySQL").Field("Path").String("/health/mysql")
		s.httpMux.HandleFunc(mysqlPath, s.mysqlHealthCheckHandler)
		global.LOGGER.InfoKV("🗃️  MySQL健康检查已启用",
			"url", baseURL+mysqlPath)
	}

	// 后续可以在这里继续添加其他组件的健康检查
	// 如: Elasticsearch, MongoDB, Kafka 等
}

// startHTTPServer 启动HTTP服务器
func (s *Server) startHTTPServer() error {
	address := s.httpServer.Addr

	// TLS 支持待实现（需要在 go-config/pkg/security 中添加 TLS 配置）
	// if s.config.Security.TLS.Enabled {
	// 	return s.httpServer.ListenAndServeTLS(certFile, keyFile)
	// }

	global.LOGGER.InfoKV("Starting HTTP server", "address", address)
	return s.httpServer.ListenAndServe()
}

// stopHTTPServer 停止HTTP服务器
func (s *Server) stopHTTPServer() error {
	if s.httpServer == nil {
		return nil
	}

	global.LOGGER.InfoMsg("Stopping HTTP server...")

	// 创建30秒超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		global.LOGGER.WithError(err).ErrorMsg("Failed to shutdown HTTP server")
		return err
	}

	global.LOGGER.InfoMsg("HTTP server stopped")
	return nil
}

// healthCheckHandler 健康检查处理器
func (s *Server) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if s.healthManager != nil {
		// 使用健康检查管理器处理请求
		handler := s.healthManager.HTTPHandler()
		handler(w, r)
	} else {
		// 降级为基础健康检查
		response.WriteSuccessResult(w, "go-rpc-gateway service is healthy")
	}
}

// redisHealthCheckHandler Redis健康检查处理器
func (s *Server) redisHealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	s.componentHealthCheck(w, r, "redis")
}

// mysqlHealthCheckHandler MySQL健康检查处理器
func (s *Server) mysqlHealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	s.componentHealthCheck(w, r, "mysql")
}

// componentHealthCheck 组件健康检查通用处理器
func (s *Server) componentHealthCheck(w http.ResponseWriter, r *http.Request, component string) {
	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)

	if s.healthManager == nil {
		response.WriteServiceUnavailableResult(w, fmt.Sprintf("%s health checker not configured", component))
		return
	}

	// 使用健康检查管理器进行组件检查
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result := s.healthManager.Check(ctx, true)

	// 返回指定组件的检查结果
	if status, ok := result.Checks[component]; ok {
		isHealthy := status.Status != "error"
		message := fmt.Sprintf("%s: %s (latency: %dms, checked at: %v)",
			status.Status, status.Message, status.Latency.Milliseconds(), status.CheckedAt)

		// 安全地处理 details 类型转换
		var details map[string]interface{}
		if status.Details != nil {
			if d, ok := status.Details.(map[string]interface{}); ok {
				details = d
			}
		}

		response.WriteHealthCheckResult(w, isHealthy, component, message, details)
	} else {
		response.WriteServiceUnavailableResult(w, fmt.Sprintf("%s health checker not registered", component))
	}
}

// RegisterHTTPRoute 注册HTTP路由
func (s *Server) RegisterHTTPRoute(pattern string, handler http.Handler) {
	if s.httpMux == nil {
		global.LOGGER.ErrorMsg("HTTP multiplexer not initialized")
		return
	}

	s.httpMux.Handle(pattern, handler)
	global.LOGGER.InfoKV("✅ 注册HTTP路由成功",
		"pattern", pattern,
		"handler_type", fmt.Sprintf("%T", handler))
}

// RegisterHTTPHandlerFunc 注册HTTP处理函数
func (s *Server) RegisterHTTPHandlerFunc(pattern string, handlerFunc http.HandlerFunc) {
	if s.httpMux == nil {
		global.LOGGER.ErrorMsg("HTTP multiplexer not initialized")
		return
	}

	s.httpMux.HandleFunc(pattern, handlerFunc)
	global.LOGGER.InfoKV("✅ 注册HTTP处理函数成功", "pattern", pattern)
}
