/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-21 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-21 10:30:00
 * @FilePath: \go-rpc-gateway\cpool\grpc\client.go
 * @Description: gRPC 客户端初始化辅助函数
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package grpc

import (
	"context"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	gwglobal "github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"net"
	"time"
)

// InitClient 初始化 gRPC 客户端的泛型辅助函数
// T: 客户端类型
// healthChecker: 健康检查管理器（可选）
// clients: gRPC 客户端配置
// serviceName: 服务名称
// factory: 客户端工厂函数
func InitClient[T any](
	healthChecker *HealthChecker,
	clients map[string]*gwconfig.GRPCClient,
	serviceName string,
	factory func(grpc.ClientConnInterface) T,
) (T, bool) {
	var zero T

	clientCfg, exists := clients[serviceName]
	if !exists || clientCfg == nil || len(clientCfg.Endpoints) == 0 {
		return zero, false
	}

	endpoint := clientCfg.Endpoints[0]

	// 准备拨号选项
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// 默认调用超时时间
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(16*1024*1024), // 16MB 最大接收消息
			grpc.MaxCallSendMsgSize(16*1024*1024), // 16MB 最大发送消息
		),
		// Keepalive 配置（保持连接活跃）
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second, // 每 10 秒发送一次 keepalive ping
			Timeout:             3 * time.Second,  // 等待 keepalive ping 响应的超时时间
			PermitWithoutStream: true,             // 允许在没有活动流时发送 keepalive ping
		}),
	}

	// 添加 Context 传播拦截器（确保 trace_id 在服务调用链中传递）
	dialOpts = append(dialOpts,
		grpc.WithChainUnaryInterceptor(
			middleware.UnaryClientContextInterceptor(), // Context 传播
		),
		grpc.WithChainStreamInterceptor(
			middleware.StreamClientContextInterceptor(), // Stream Context 传播
		),
	)

	// 如果配置了 Network，添加到拨号选项
	if clientCfg.Network != "" {
		dialOpts = append(dialOpts, grpc.WithContextDialer(
			func(ctx context.Context, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, clientCfg.Network, addr)
			},
		))
		gwglobal.LOGGER.Debug("🌐 %s 使用网络类型: %s", serviceName, clientCfg.Network)
	}

	// 创建连接（不等待就绪）
	conn, err := grpc.NewClient(endpoint, dialOpts...)
	if err != nil {
		gwglobal.LOGGER.Warn("⚠️  %s 创建连接失败: %v", serviceName, err)
		return zero, false
	}

	// 如果提供了健康检查器，注册到健康检查
	if healthChecker != nil {
		healthChecker.Register(serviceName, conn, endpoint)
	}

	gwglobal.LOGGER.Debug("✅ %s 客户端已创建 -> %s (健康检查中...)", serviceName, endpoint)
	return factory(conn), true
}

// BuildEndpointMap 从配置构建服务名到端点的映射
func BuildEndpointMap(clients map[string]*gwconfig.GRPCClient) map[string]string {
	endpoints := make(map[string]string)
	for name, client := range clients {
		if client != nil && len(client.Endpoints) > 0 {
			endpoints[name] = client.Endpoints[0]
		}
	}
	return endpoints
}

// DefaultHealthCheckInterval 默认健康检查间隔
const DefaultHealthCheckInterval = 3 * time.Second
