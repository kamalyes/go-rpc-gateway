/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-13 11:52:56
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

	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// initGRPCServer 初始化gRPC服务器
// go-config 的 Default() 已经设置了所有默认值，无需再次设置
func (s *Server) initGRPCServer() error {
	// 配置已通过 safe.MergeWithDefaults 合并默认值
	grpcServer := s.config.GRPC.Server

	// 检查是否启用 gRPC 服务
	if !grpcServer.Enable {
		global.LOGGER.InfoMsg("gRPC服务未启用,跳过初始化")
		return nil
	}

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(grpcServer.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(grpcServer.MaxSendMsgSize),
	}

	// 添加Keepalive配置
	if grpcServer.KeepaliveTime > 0 {
		keepalivePolicy := keepalive.ServerParameters{
			Time:    time.Duration(grpcServer.KeepaliveTime) * time.Second,
			Timeout: time.Duration(grpcServer.KeepaliveTimeout) * time.Second,
		}
		opts = append(opts, grpc.KeepaliveParams(keepalivePolicy))

		global.LOGGER.InfoKV("gRPC Keepalive配置已启用",
			"keepalive_time", grpcServer.KeepaliveTime,
			"keepalive_timeout", grpcServer.KeepaliveTimeout)
	}

	// 添加连接超时配置
	if grpcServer.ConnectionTimeout > 0 {
		keepaliveEnforcement := keepalive.EnforcementPolicy{
			MinTime:             time.Duration(grpcServer.ConnectionTimeout) * time.Second,
			PermitWithoutStream: true,
		}
		opts = append(opts, grpc.KeepaliveEnforcementPolicy(keepaliveEnforcement))

		global.LOGGER.InfoKV("gRPC连接超时配置已启用",
			"connection_timeout", grpcServer.ConnectionTimeout)
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
	if grpcServer.EnableReflection {
		reflection.Register(s.grpcServer)
		global.LOGGER.InfoMsg("gRPC反射服务已启用")
	}

	global.LOGGER.InfoKV("gRPC服务器初始化完成",
		"max_recv_size", grpcServer.MaxRecvMsgSize,
		"max_send_size", grpcServer.MaxSendMsgSize,
		"reflection_enabled", grpcServer.EnableReflection)

	return nil
}

// startGRPCServer 启动gRPC服务器
func (s *Server) startGRPCServer() error {
	grpcServer := s.config.GRPC.Server

	// 检查是否启用 gRPC 服务
	if !grpcServer.Enable {
		global.LOGGER.InfoMsg("gRPC服务未启用,跳过启动")
		return nil
	}

	// 获取网络和地址配置
	address := fmt.Sprintf("%s:%d", grpcServer.Host, grpcServer.Port)

	listener, err := net.Listen(grpcServer.Network, address)
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
