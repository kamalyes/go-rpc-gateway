/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-26 12:05:06
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
	"net"
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
	// 配置已通过 safe.MergeWithDefaults 合并默认值，直接使用
	useProtoNames := s.config.JSON.UseProtoNames
	emitUnpopulated := s.config.JSON.EmitUnpopulated
	discardUnknown := s.config.JSON.DiscardUnknown

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
		// 🔑 将所有 HTTP Header 传递到 gRPC metadata (支持认证等功能)
		runtime.WithIncomingHeaderMatcher(func(key string) (string, bool) {
			return key, true // 传递所有 header
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
		// 检查是否启用压缩
		if !s.config.HTTPServer.EnableGzipCompress {
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

	// 收集所有中间件（静态 + 动态提供）并去重
	const middlewareWarnThreshold = 100
	middlewareSet := make(map[string]bool)
	var allMiddlewares []runtime.Middleware

	// 添加静态中间件
	for i, mw := range s.grpcGatewayMiddlewares {
		key := fmt.Sprintf("static_%d", i)
		if middlewareSet[key] {
			continue
		}
		allMiddlewares = append(allMiddlewares, mw)
		middlewareSet[key] = true
	}

	// 添加动态中间件
	for providerIdx, provider := range s.grpcGatewayMiddlewareProviders {
		mws := provider()
		if len(mws) == 0 {
			continue
		}

		for mwIdx, mw := range mws {
			key := fmt.Sprintf("provider_%d_%d", providerIdx, mwIdx)
			if middlewareSet[key] {
				continue
			}
			allMiddlewares = append(allMiddlewares, mw)
			middlewareSet[key] = true
		}
	}

	// 中间件数量超过阈值时警告（warn-only 模式，不硬限制）
	if len(allMiddlewares) > middlewareWarnThreshold {
		global.LOGGER.WarnContext(s.ctx, "⚠️  中间件数量超过建议值",
			"count", len(allMiddlewares),
			"threshold", middlewareWarnThreshold)
	}

	// 添加所有中间件
	if len(allMiddlewares) > 0 {
		opts = append(opts, runtime.WithMiddlewares(allMiddlewares...))
		global.LOGGER.InfoContext(s.ctx, "✅ 已注册 %d 个 gRPC-Gateway 中间件", len(allMiddlewares))
	}

	s.gwMux = runtime.NewServeMux(opts...)

	// 创建HTTP多路复用器
	s.httpMux = http.NewServeMux()

	// 注册网关路由（默认路由到gwMux）
	s.httpMux.Handle("/", s.gwMux)

	// 注册健康检查
	if s.config.Health.Enabled {
		healthPath := s.config.Health.Path
		s.httpMux.HandleFunc(healthPath, s.healthCheckHandler)

		httpEndpoint := fmt.Sprintf("%s:%d", s.config.HTTPServer.Host, s.config.HTTPServer.Port)
		global.LOGGER.InfoKV("❤️  健康检查已启用",
			"url", "http://"+httpEndpoint+healthPath)

		// 注册组件级健康检查端点
		s.registerComponentHealthChecks()
	}

	// 注册监控指标端点
	if s.config.Monitoring.Metrics.Enabled {
		prometheusPath := s.config.Monitoring.Metrics.Endpoint
		s.httpMux.Handle(prometheusPath, promhttp.Handler())

		httpEndpoint := fmt.Sprintf("%s:%d", s.config.HTTPServer.Host, s.config.HTTPServer.Port)
		global.LOGGER.InfoKV("📊 监控指标服务可用",
			"url", "http://"+httpEndpoint+prometheusPath)
	}

	// 应用中间件
	var handler http.Handler = s.httpMux

	if s.middlewareManager != nil {
		var middlewares []middleware.MiddlewareFunc
		middlewares = s.middlewareManager.GetMiddlewares()
		handler = middleware.ApplyMiddlewares(handler, middlewares...)
	}

	// 最后应用Gzip压缩中间件（如果启用）
	// 注意：Gzip 应该在日志中间件之后执行，否则日志记录的是压缩后的乱码
	if s.config.HTTPServer.EnableGzipCompress {
		handler = s.gzipMiddleware(handler)
		global.LOGGER.InfoMsg("✅ HTTP Gzip压缩已启用")
	}

	// 创建 HTTP 服务器（配置已通过 safe.MergeWithDefaults 合并默认值）
	s.httpServer = &http.Server{
		Addr:           fmt.Sprintf("%s:%d", s.config.HTTPServer.Host, s.config.HTTPServer.Port),
		Handler:        handler,
		ReadTimeout:    time.Duration(s.config.HTTPServer.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(s.config.HTTPServer.WriteTimeout) * time.Second,
		IdleTimeout:    time.Duration(s.config.HTTPServer.IdleTimeout) * time.Second,
		MaxHeaderBytes: s.config.HTTPServer.MaxHeaderBytes,
	}

	return nil
}

// RebuildHTTPGateway 重建 HTTP网关（用于在添加中间件后重新初始化）
func (s *Server) RebuildHTTPGateway() error {
	global.LOGGER.InfoContext(s.ctx, "🔄 重建 HTTP Gateway...")
	return s.initHTTPGateway()
}

// registerComponentHealthChecks 注册组件级健康检查端点
func (s *Server) registerComponentHealthChecks() {
	baseURL := fmt.Sprintf("http://%s:%d", s.config.HTTPServer.Host, s.config.HTTPServer.Port)

	// 注册Redis健康检查
	if s.config.Health.Redis.Enabled {
		s.httpMux.HandleFunc(s.config.Health.Redis.Path, s.redisHealthCheckHandler)
		global.LOGGER.InfoKV("🔴 Redis健康检查已启用",
			"url", baseURL+s.config.Health.Redis.Path)
	}

	// 注册MySQL健康检查
	if s.config.Health.MySQL.Enabled {
		s.httpMux.HandleFunc(s.config.Health.MySQL.Path, s.mysqlHealthCheckHandler)
		global.LOGGER.InfoKV("🗃️  MySQL健康检查已启用",
			"url", baseURL+s.config.Health.MySQL.Path)
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

	// 从配置中获取网络类型
	listener, err := net.Listen(s.config.HTTPServer.Network, address)
	if err != nil {
		return fmt.Errorf("failed to create %s listener: %w", s.config.HTTPServer.Network, err)
	}
	defer listener.Close() // Fix 确保 listener 关闭，防止连接泄漏

	return s.httpServer.Serve(listener)
}

// stopHTTPServer 停止HTTP服务器
func (s *Server) stopHTTPServer() error {
	if s.httpServer == nil {
		return nil
	}

	// 创建30秒超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	global.LOGGER.InfoContext(ctx, "Stopping HTTP server...")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		global.LOGGER.WithError(err).ErrorContext(ctx, "Failed to shutdown HTTP server")
		return err
	}

	global.LOGGER.InfoContext(ctx, "HTTP server stopped")
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
