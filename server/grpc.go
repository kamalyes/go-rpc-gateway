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
	"time"

	"github.com/kamalyes/go-rpc-gateway/global"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// initGRPCServer 初始化gRPC服务器
// go-config 的 Default() 已经设置了所有默认值，无需再次设置
func (s *Server) initGRPCServer() error {
	grpcCfg := s.config.GRPC.Server

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(grpcCfg.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(grpcCfg.MaxSendMsgSize),
	}

	// 添加Keepalive配置
	if grpcCfg.KeepaliveTime > 0 {
		keepalivePolicy := keepalive.ServerParameters{
			Time:    time.Duration(grpcCfg.KeepaliveTime) * time.Second,
			Timeout: time.Duration(grpcCfg.KeepaliveTimeout) * time.Second,
		}
		opts = append(opts, grpc.KeepaliveParams(keepalivePolicy))

		global.LOGGER.InfoKV("gRPC Keepalive配置已启用",
			"keepalive_time", grpcCfg.KeepaliveTime,
			"keepalive_timeout", grpcCfg.KeepaliveTimeout)
	}

	// 添加连接超时配置
	if grpcCfg.ConnectionTimeout > 0 {
		keepaliveEnforcement := keepalive.EnforcementPolicy{
			MinTime:             time.Duration(grpcCfg.ConnectionTimeout) * time.Second,
			PermitWithoutStream: true,
		}
		opts = append(opts, grpc.KeepaliveEnforcementPolicy(keepaliveEnforcement))

		global.LOGGER.InfoKV("gRPC连接超时配置已启用",
			"connection_timeout", grpcCfg.ConnectionTimeout)
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
	if grpcCfg.EnableReflection {
		reflection.Register(s.grpcServer)
		global.LOGGER.InfoMsg("gRPC反射服务已启用")
	}

	global.LOGGER.InfoKV("gRPC服务器初始化完成",
		"max_recv_size", grpcCfg.MaxRecvMsgSize,
		"max_send_size", grpcCfg.MaxSendMsgSize,
		"reflection_enabled", grpcCfg.EnableReflection)

	return nil
}

// startGRPCServer 启动gRPC服务器
func (s *Server) startGRPCServer() error {
	grpcCfg := s.config.GRPC.Server
	address := grpcCfg.GetEndpoint()
	listener, err := net.Listen(grpcCfg.Network, address)
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
