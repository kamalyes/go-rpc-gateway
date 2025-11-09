/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 20:32:21
 * @FilePath: \go-rpc-gateway\server\http.go
 * @Description: HTTP服务器和网关初始化模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/kamalyes/go-core/pkg/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	}

	// 注册监控指标端点
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","service":"go-rpc-gateway"}`))
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
