/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 12:39:24
 * @FilePath: \go-rpc-gateway\server\http.go
 * @Description: HTTP服务器和网关初始化模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/kamalyes/go-core/pkg/global"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

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
		if !s.config.Gateway.HTTPServer.EnableGzipCompress {
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
	// 创建gRPC-Gateway多路复用器
	s.gwMux = runtime.NewServeMux()

	// 创建HTTP多路复用器
	s.httpMux = http.NewServeMux()

	// 注册网关路由（默认路由到gwMux）
	s.httpMux.Handle("/", s.gwMux)

	// 注册健康检查
	if s.config.Gateway.Health.Enabled {
		s.httpMux.HandleFunc(s.config.Gateway.Health.Path, s.healthCheckHandler)
		global.LOGGER.InfoKV("❤️  健康检查已启用",
			"url", s.config.Gateway.HTTPServer.GetEndpoint()+s.config.Gateway.Health.Path)

		// 注册组件级健康检查端点
		s.registerComponentHealthChecks()
	}
	
	// 注册监控指标端点
	if s.config.Monitoring.Metrics.Enabled {
		s.httpMux.Handle(s.config.Monitoring.Prometheus.Path, promhttp.Handler())
		global.LOGGER.InfoKV("📊 监控指标服务可用",
			"url", s.config.Gateway.HTTPServer.GetEndpoint()+s.config.Monitoring.Prometheus.Path)
	}

	// 应用中间件
	var handler http.Handler = s.httpMux

	// 首先应用Gzip压缩中间件（如果启用）
	if s.config.Gateway.HTTPServer.EnableGzipCompress {
		handler = s.gzipMiddleware(handler)
		global.LOGGER.InfoMsg("✅ HTTP Gzip压缩已启用")
	}

	if s.middlewareManager != nil {
		var middlewares []middleware.MiddlewareFunc
		if s.config.Gateway.Debug {
			middlewares = s.middlewareManager.GetDevelopmentMiddlewares()
		} else {
			middlewares = s.middlewareManager.GetDefaultMiddlewares()
		}
		handler = middleware.ApplyMiddlewares(handler, middlewares...)
	}

	// 创建HTTP服务器
	s.httpServer = &http.Server{
		Addr:           fmt.Sprintf("%s:%d", s.config.Gateway.HTTPServer.Host, s.config.Gateway.HTTPServer.Port),
		Handler:        handler,
		ReadTimeout:    time.Duration(s.config.Gateway.HTTPServer.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(s.config.Gateway.HTTPServer.WriteTimeout) * time.Second,
		IdleTimeout:    time.Duration(s.config.Gateway.HTTPServer.IdleTimeout) * time.Second,
		MaxHeaderBytes: s.config.Gateway.HTTPServer.MaxHeaderBytes,
	}

	return nil
}

// registerComponentHealthChecks 注册组件级健康检查端点
func (s *Server) registerComponentHealthChecks() {
	baseURL := s.config.Gateway.HTTPServer.GetEndpoint()

	// 注册Redis健康检查
	if s.config.Gateway.Health.Redis.Enabled {
		s.httpMux.HandleFunc(s.config.Gateway.Health.Redis.Path, s.redisHealthCheckHandler)
		global.LOGGER.InfoKV("🔴 Redis健康检查已启用",
			"url", baseURL+s.config.Gateway.Health.Redis.Path)
	}

	// 注册MySQL健康检查
	if s.config.Gateway.Health.MySQL.Enabled {
		s.httpMux.HandleFunc(s.config.Gateway.Health.MySQL.Path, s.mysqlHealthCheckHandler)
		global.LOGGER.InfoKV("🗃️  MySQL健康检查已启用",
			"url", baseURL+s.config.Gateway.Health.MySQL.Path)
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
		w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"go-rpc-gateway"}`))
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
		w.WriteHeader(http.StatusServiceUnavailable)
		response := map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("%s health checker not configured", component),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// 使用健康检查管理器进行组件检查
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result := s.healthManager.Check(ctx, true)

	// 返回指定组件的检查结果
	if status, ok := result.Checks[component]; ok {
		response := map[string]interface{}{
			"status":     status.Status,
			"message":    status.Message,
			"latency_ms": status.Latency.Milliseconds(),
			"checked_at": status.CheckedAt,
			"details":    status.Details,
		}

		if status.Status == "error" {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		response := map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("%s health checker not registered", component),
		}
		json.NewEncoder(w).Encode(response)
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
