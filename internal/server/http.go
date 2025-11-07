/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 20:32:21
 * @FilePath: \go-rpc-gateway\internal\server\http.go
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
	"go.uber.org/zap"
)

// initHTTPGateway 初始化HTTP网关
func (s *Server) initHTTPGateway() error {
	// 创建gRPC-Gateway多路复用器
	s.gwMux = runtime.NewServeMux()

	// 创建HTTP多路复用器
	mux := http.NewServeMux()

	// 注册网关路由
	mux.Handle("/", s.gwMux)

	// 注册健康检查
	if s.config.Gateway.HealthCheck.Enabled {
		mux.HandleFunc(s.config.Gateway.HealthCheck.Path, s.healthCheckHandler)
		global.LOG.Info("❤️  健康检查已启用",
			zap.String("url", fmt.Sprintf("http://%s:%d%s",
				s.config.Gateway.HTTP.Host,
				s.config.Gateway.HTTP.Port,
				s.config.Gateway.HealthCheck.Path)))
	}

	// 注册指标路由
	if s.config.Monitoring.Metrics.Enabled {
		mux.Handle(s.config.Monitoring.Metrics.Path, promhttp.Handler())
		global.LOG.Info("📊 监控指标服务可用",
			zap.String("url", fmt.Sprintf("http://%s:%d%s",
				s.config.Gateway.HTTP.Host,
				s.config.Gateway.HTTP.Port,
				s.config.Monitoring.Metrics.Path)))
	}

	// 应用中间件
	var handler http.Handler = mux
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
	global.LOG.Info("Starting HTTP server", zap.String("address", s.httpServer.Addr))
	return s.httpServer.ListenAndServe()
}

// stopHTTPServer 停止HTTP服务器
func (s *Server) stopHTTPServer() error {
	if s.httpServer == nil {
		return nil
	}

	global.LOG.Info("Stopping HTTP server...")

	// 创建30秒超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		global.LOG.Error("Failed to shutdown HTTP server", zap.Error(err))
		return err
	}

	global.LOG.Info("HTTP server stopped")
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
	// 这里需要添加到HTTP服务器的路由中
	// 由于当前使用的是grpc-gateway的ServeMux，我们需要扩展这个功能
	// 暂时先记录，实际实现需要根据具体的HTTP服务器来定制
	global.LOG.Info("注册HTTP路由",
		zap.String("pattern", pattern),
		zap.String("handler_type", fmt.Sprintf("%T", handler)),
	)
}
