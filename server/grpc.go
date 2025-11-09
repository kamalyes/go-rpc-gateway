/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 01:39:29
 * @FilePath: \go-rpc-gateway\server\grpc.go
 * @Description: gRPC服务器初始化和启动模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"fmt"
	"net"

	"github.com/kamalyes/go-core/pkg/global"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// initGRPCServer 初始化gRPC服务器
func (s *Server) initGRPCServer() error {
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.config.Gateway.GRPC.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(s.config.Gateway.GRPC.MaxSendMsgSize),
	}

	// 添加中间件拦截器
	if s.middlewareManager != nil {
		// 添加gRPC监控拦截器
		if metricsInterceptor := s.middlewareManager.GRPCMetricsInterceptor(); metricsInterceptor != nil {
			opts = append(opts, grpc.UnaryInterceptor(metricsInterceptor))
		}

		// 添加gRPC链路追踪拦截器
		if tracingInterceptor := s.middlewareManager.GRPCTracingInterceptor(); tracingInterceptor != nil {
			opts = append(opts, grpc.ChainUnaryInterceptor(tracingInterceptor))
		}
	}

	s.grpcServer = grpc.NewServer(opts...)

	// 启用反射
	if s.config.Gateway.GRPC.EnableReflection {
		reflection.Register(s.grpcServer)
	}

	return nil
}

// startGRPCServer 启动gRPC服务器
func (s *Server) startGRPCServer() error {
	address := fmt.Sprintf("%s:%d", s.config.Gateway.GRPC.Host, s.config.Gateway.GRPC.Port)
	listener, err := net.Listen(s.config.Gateway.GRPC.Network, address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	global.LOGGER.InfoKV("Starting gRPC server", "address", address)
	return s.grpcServer.Serve(listener)
}

// stopGRPCServer 停止gRPC服务器
func (s *Server) stopGRPCServer() {
	if s.grpcServer != nil {
		global.LOGGER.Info("Stopping gRPC server...")
		s.grpcServer.GracefulStop()
		global.LOGGER.Info("gRPC server stopped")
	}
}
