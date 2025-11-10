/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:05:01
 * @FilePath: \go-rpc-gateway\server\http.go
 * @Description: HTTP服务器和网关初始化模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/kamalyes/go-core/pkg/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTP响应常量
const (
	ContentTypeJSON   = "application/json"
	HeaderContentType = "Content-Type"
)

// initHTTPGateway 初始化HTTP网关
func (s *Server) initHTTPGateway() error {
	// 创建gRPC-Gateway多路复用器
	s.gwMux = runtime.NewServeMux()

	// 创建HTTP多路复用器
	s.httpMux = http.NewServeMux()

	// 注册网关路由（默认路由到gwMux）
	s.httpMux.Handle("/", s.gwMux)

	// 注册健康检查
	if s.config.Gateway.HealthCheck.Enabled {
		s.httpMux.HandleFunc(s.config.Gateway.HealthCheck.Path, s.healthCheckHandler)
		global.LOGGER.InfoKV("❤️  健康检查已启用",
			"url", fmt.Sprintf("http://%s:%d%s",
				s.config.Gateway.HTTP.Host,
				s.config.Gateway.HTTP.Port,
				s.config.Gateway.HealthCheck.Path))

		// 注册组件级健康检查端点
		s.registerComponentHealthChecks()
	} // 注册监控指标端点
	if s.config.Monitoring.Metrics.Enabled {
		s.httpMux.Handle(s.config.Monitoring.Metrics.Path, promhttp.Handler())
		global.LOGGER.InfoKV("📊 监控指标服务可用",
			"url", fmt.Sprintf("http://%s:%d%s",
				s.config.Gateway.HTTP.Host,
				s.config.Gateway.HTTP.Port,
				s.config.Monitoring.Metrics.Path))
	}

	// 应用中间件
	var handler http.Handler = s.httpMux
	if s.middlewareManager != nil {
		var middlewares []middleware.HTTPMiddleware
		if s.config.Gateway.Debug {
			middlewares = s.middlewareManager.GetDevelopmentMiddlewares()
		} else {
			middlewares = s.middlewareManager.GetDefaultMiddlewares()
		}
		handler = middleware.ApplyMiddlewares(handler, middlewares...)
	}

	// 创建HTTP服务器
	s.httpServer = &http.Server{
		Addr:           fmt.Sprintf("%s:%d", s.config.Gateway.HTTP.Host, s.config.Gateway.HTTP.Port),
		Handler:        handler,
		ReadTimeout:    time.Duration(s.config.Gateway.HTTP.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(s.config.Gateway.HTTP.WriteTimeout) * time.Second,
		IdleTimeout:    time.Duration(s.config.Gateway.HTTP.IdleTimeout) * time.Second,
		MaxHeaderBytes: s.config.Gateway.HTTP.MaxHeaderBytes,
	}

	return nil
}

// registerComponentHealthChecks 注册组件级健康检查端点
func (s *Server) registerComponentHealthChecks() {
	baseURL := fmt.Sprintf("http://%s:%d", s.config.Gateway.HTTP.Host, s.config.Gateway.HTTP.Port)

	// 注册Redis健康检查
	if s.config.Gateway.HealthCheck.Redis.Enabled {
		s.httpMux.HandleFunc("/health/redis", s.redisHealthCheckHandler)
		global.LOGGER.InfoKV("🔴 Redis健康检查已启用",
			"url", baseURL+"/health/redis",
			"redis_host", fmt.Sprintf("%s:%d",
				s.config.Gateway.HealthCheck.Redis.Host,
				s.config.Gateway.HealthCheck.Redis.Port))
	}

	// 注册MySQL健康检查
	if s.config.Gateway.HealthCheck.MySQL.Enabled {
		s.httpMux.HandleFunc("/health/mysql", s.mysqlHealthCheckHandler)
		global.LOGGER.InfoKV("🗃️  MySQL健康检查已启用",
			"url", baseURL+"/health/mysql",
			"mysql_host", fmt.Sprintf("%s:%d/%s",
				s.config.Gateway.HealthCheck.MySQL.Host,
				s.config.Gateway.HealthCheck.MySQL.Port,
				s.config.Gateway.HealthCheck.MySQL.Database))
	}

	// 后续可以在这里继续添加其他组件的健康检查
	// 如: Elasticsearch, MongoDB, Kafka 等
}

// startHTTPServer 启动HTTP服务器
func (s *Server) startHTTPServer() error {
	global.LOGGER.InfoKV("Starting HTTP server", "address", s.httpServer.Addr)
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
		w.Header().Set(HeaderContentType, ContentTypeJSON)
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
	w.Header().Set(HeaderContentType, ContentTypeJSON)

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
