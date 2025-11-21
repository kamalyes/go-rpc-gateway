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
	"time"

	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	gwglobal "github.com/kamalyes/go-rpc-gateway/global"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

	// 创建连接（不等待就绪）
	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
const DefaultHealthCheckInterval = 30 * time.Second
