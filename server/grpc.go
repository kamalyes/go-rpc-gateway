/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 15:09:37
 * @FilePath: \go-rpc-gateway\server\grpc.go
 * @Description: gRPC服务器初始化和启动模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"fmt"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"net"
	"time"
)

// initGRPCServer 初始化gRPC服务器
// go-config 的 Default() 已经设置了所有默认值，无需再次设置
func (s *Server) initGRPCServer() error {
	// 使用安全访问模式
	grpcSafe := s.configSafe.Field("GRPC").Field("Server")

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(grpcSafe.Field("MaxRecvMsgSize").Int(4194304)),
		grpc.MaxSendMsgSize(grpcSafe.Field("MaxSendMsgSize").Int(4194304)),
	}

	// 添加Keepalive配置
	keepaliveTime := grpcSafe.Field("KeepaliveTime").Int(0)
	if keepaliveTime > 0 {
		keepaliveTimeout := grpcSafe.Field("KeepaliveTimeout").Int(10)
		keepalivePolicy := keepalive.ServerParameters{
			Time:    time.Duration(keepaliveTime) * time.Second,
			Timeout: time.Duration(keepaliveTimeout) * time.Second,
		}
		opts = append(opts, grpc.KeepaliveParams(keepalivePolicy))

		global.LOGGER.InfoKV("gRPC Keepalive配置已启用",
			"keepalive_time", keepaliveTime,
			"keepalive_timeout", keepaliveTimeout)
	}

	// 添加连接超时配置
	connectionTimeout := grpcSafe.Field("ConnectionTimeout").Int(0)
	if connectionTimeout > 0 {
		keepaliveEnforcement := keepalive.EnforcementPolicy{
			MinTime:             time.Duration(connectionTimeout) * time.Second,
			PermitWithoutStream: true,
		}
		opts = append(opts, grpc.KeepaliveEnforcementPolicy(keepaliveEnforcement))

		global.LOGGER.InfoKV("gRPC连接超时配置已启用",
			"connection_timeout", connectionTimeout)
	}

	// 添加中间件拦截器链（按执行顺序）
	if s.middlewareManager != nil {
		// 构建 Unary 拦截器链
		unaryInterceptors := []grpc.UnaryServerInterceptor{
			middleware.UnaryServerContextInterceptor(), // 1. Context 注入（最先执行，注入 trace_id/request_id）
			middleware.UnaryServerLoggingInterceptor(), // 2. 日志记录
		}

		// 添加监控拦截器（如果启用）
		if metricsInterceptor := s.middlewareManager.GRPCMetricsInterceptor(); metricsInterceptor != nil {
			unaryInterceptors = append(unaryInterceptors, metricsInterceptor)
		}

		// 添加链路追踪拦截器（如果启用）
		if tracingInterceptor := s.middlewareManager.GRPCTracingInterceptor(); tracingInterceptor != nil {
			unaryInterceptors = append(unaryInterceptors, tracingInterceptor)
		}

		opts = append(opts, grpc.ChainUnaryInterceptor(unaryInterceptors...))

		// 构建 Stream 拦截器链
		streamInterceptors := []grpc.StreamServerInterceptor{
			middleware.StreamServerContextInterceptor(), // 1. Context 注入
			middleware.StreamServerLoggingInterceptor(), // 2. 日志记录
		}
		opts = append(opts, grpc.ChainStreamInterceptor(streamInterceptors...))
	}

	s.grpcServer = grpc.NewServer(opts...)

	// 启用反射
	enableReflection := grpcSafe.Field("EnableReflection").Bool(false)
	if enableReflection {
		reflection.Register(s.grpcServer)
		global.LOGGER.InfoMsg("gRPC反射服务已启用")
	}

	maxRecvSize := grpcSafe.Field("MaxRecvMsgSize").Int(4194304)
	maxSendSize := grpcSafe.Field("MaxSendMsgSize").Int(4194304)
	global.LOGGER.InfoKV("gRPC服务器初始化完成",
		"max_recv_size", maxRecvSize,
		"max_send_size", maxSendSize,
		"reflection_enabled", enableReflection)

	return nil
}

// startGRPCServer 启动gRPC服务器
func (s *Server) startGRPCServer() error {
	grpcSafe := s.configSafe.Field("GRPC").Field("Server")

	// 安全获取网络和地址配置，默认使用 tcp4 强制 IPv4
	network := grpcSafe.Field("Network").String("tcp4")
	host := grpcSafe.Field("Host").String("0.0.0.0")
	port := grpcSafe.Field("Port").Int(9090)
	address := fmt.Sprintf("%s:%d", host, port)

	listener, err := net.Listen(network, address)
	if err != nil {
		return errors.NewErrorf(errors.ErrCodeGRPCConnectionFailed, "failed to listen on %s: %v", address, err)
	}

	global.LOGGER.InfoKV("Starting gRPC server", "address", address)
	return s.grpcServer.Serve(listener)
}

// stopGRPCServer 停止gRPC服务器
func (s *Server) stopGRPCServer() {
	if s.grpcServer != nil {
		global.LOGGER.InfoContext(s.ctx, "Stopping gRPC server...")
		s.grpcServer.GracefulStop()
		global.LOGGER.InfoContext(s.ctx, "gRPC server stopped")
	}
}
