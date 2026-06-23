/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-21 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-04 13:51:50
 * @FilePath: \go-rpc-gateway\cpool\grpc\client.go
 * @Description: gRPC 客户端初始化辅助函数
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	gwglobal "github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// connPool 全局连接池，按服务名缓存 *grpc.ClientConn
var (
	connPool   = make(map[string]*grpc.ClientConn)
	connPoolMu sync.RWMutex
)

// GetConn 从连接池获取指定服务名的 gRPC 连接
func GetConn(serviceName string) (*grpc.ClientConn, bool) {
	connPoolMu.RLock()
	defer connPoolMu.RUnlock()
	conn, ok := connPool[serviceName]
	return conn, ok
}

// PutConn 将 gRPC 连接存入连接池
func PutConn(serviceName string, conn *grpc.ClientConn) {
	connPoolMu.Lock()
	defer connPoolMu.Unlock()
	connPool[serviceName] = conn
}

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

	// 优先从连接池获取已有连接
	connPoolMu.RLock()
	conn, connExists := connPool[serviceName]
	connPoolMu.RUnlock()
	if connExists {
		gwglobal.LOGGER.Debug("♻️  %s 复用连接池中的已有连接", serviceName)
		return factory(conn), true
	}

	endpoint := clientCfg.Endpoints[0]

	// 构建 TLS 配置
	creds := buildTLSConfig(clientCfg, serviceName)

	// 构建拨号选项
	dialOpts := buildDialOptions(clientCfg, serviceName, creds, healthChecker)

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

	// 存入连接池
	connPoolMu.Lock()
	connPool[serviceName] = conn
	connPoolMu.Unlock()

	gwglobal.LOGGER.Debug("✅ %s 客户端已创建 -> %s (健康检查中...)", serviceName, endpoint)
	return factory(conn), true
}

// InitClientTo 初始化 gRPC 客户端并赋值到目标指针，同时记录日志
// 是 InitClient 的便捷封装，用于减少调用方的样板代码（自动注入风格）
// 成功时将客户端赋值到 *target 并记录 Info 日志，失败时记录 Warn 日志
//
// 使用示例:
//
//	grpcpool.InitClientTo(g.healthChecker, clients, "user-service", "UserService",
//	    userpb.NewUserServiceClient, &g.userClient)
func InitClientTo[T any](
	healthChecker *HealthChecker,
	clients map[string]*gwconfig.GRPCClient,
	serviceName, label string,
	factory func(grpc.ClientConnInterface) T,
	target *T,
) bool {
	client, ok := InitClient(healthChecker, clients, serviceName, factory)
	if ok {
		*target = client
		gwglobal.LOGGER.Info("%s client initialized", label)
	} else {
		gwglobal.LOGGER.Warn("%s client initialization failed, check config", label)
	}
	return ok
}

// BuildDialOptions 构建 gRPC 客户端拨号选项（公开方法）
// 根据客户端配置构建完整的 dial options，包括 TLS、keepalive、消息大小等
func BuildDialOptions(clientCfg *gwconfig.GRPCClient, serviceName string, healthChecker *HealthChecker) []grpc.DialOption {
	creds := buildTLSConfig(clientCfg, serviceName)
	return buildDialOptions(clientCfg, serviceName, creds, healthChecker)
}

// InitClientAny 初始化 gRPC 客户端（返回 interface{}，用于自动注册场景）
// 与 InitClient 逻辑相同，但工厂函数和返回值均为 interface{}，便于反射调用
func InitClientAny(
	healthChecker *HealthChecker,
	clients map[string]*gwconfig.GRPCClient,
	serviceName string,
	factory func(grpc.ClientConnInterface) interface{},
) (interface{}, bool) {
	clientCfg, exists := clients[serviceName]
	if !exists || clientCfg == nil || len(clientCfg.Endpoints) == 0 {
		return nil, false
	}

	// 优先从连接池获取已有连接
	connPoolMu.RLock()
	conn, connExists := connPool[serviceName]
	connPoolMu.RUnlock()
	if connExists {
		gwglobal.LOGGER.Debug("♻️  %s 复用连接池中的已有连接", serviceName)
		return factory(conn), true
	}

	endpoint := clientCfg.Endpoints[0]
	creds := buildTLSConfig(clientCfg, serviceName)
	dialOpts := buildDialOptions(clientCfg, serviceName, creds, healthChecker)

	conn, err := grpc.NewClient(endpoint, dialOpts...)
	if err != nil {
		gwglobal.LOGGER.Warn("⚠️  %s 创建连接失败: %v", serviceName, err)
		return nil, false
	}

	if healthChecker != nil {
		healthChecker.Register(serviceName, conn, endpoint)
	}

	connPoolMu.Lock()
	connPool[serviceName] = conn
	connPoolMu.Unlock()

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

// buildTLSConfig 构建 TLS 配置
func buildTLSConfig(clientCfg *gwconfig.GRPCClient, serviceName string) credentials.TransportCredentials {
	if !clientCfg.EnableTLS {
		return insecure.NewCredentials()
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	// 加载 CA 证书
	if clientCfg.TLSCAFile != "" {
		caCert, err := os.ReadFile(clientCfg.TLSCAFile)
		if err != nil {
			gwglobal.LOGGER.Error("❌ %s 读取 CA 证书失败: %v", serviceName, err)
		} else {
			caCertPool := x509.NewCertPool()
			if caCertPool.AppendCertsFromPEM(caCert) {
				tlsConfig.RootCAs = caCertPool
				tlsConfig.InsecureSkipVerify = false
				gwglobal.LOGGER.Debug("🔒 %s 已加载 CA 证书: %s", serviceName, clientCfg.TLSCAFile)
			}
		}
	}

	// 加载客户端证书（双向认证）
	if clientCfg.TLSCertFile != "" && clientCfg.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(clientCfg.TLSCertFile, clientCfg.TLSKeyFile)
		if err != nil {
			gwglobal.LOGGER.Error("❌ %s 加载客户端证书失败: %v", serviceName, err)
		} else {
			tlsConfig.Certificates = []tls.Certificate{cert}
			gwglobal.LOGGER.Debug("🔒 %s 已加载客户端证书", serviceName)
		}
	}

	gwglobal.LOGGER.Info("🔒 %s 启用 TLS 连接", serviceName)
	return credentials.NewTLS(tlsConfig)
}

// buildDialOptions 构建 Dial 选项
func buildDialOptions(clientCfg *gwconfig.GRPCClient, serviceName string, creds credentials.TransportCredentials, healthChecker *HealthChecker) []grpc.DialOption {
	// Keepalive 配置
	keepaliveTime := mathx.IF(clientCfg.KeepaliveTime > 0, time.Duration(clientCfg.KeepaliveTime)*time.Second, 10*time.Second)
	keepaliveTimeout := mathx.IF(clientCfg.KeepaliveTimeout > 0, time.Duration(clientCfg.KeepaliveTimeout)*time.Second, 3*time.Second)

	// 消息大小配置
	maxRecvMsgSize := mathx.IF(clientCfg.MaxRecvMsgSize > 0, clientCfg.MaxRecvMsgSize, 16*1024*1024)
	maxSendMsgSize := mathx.IF(clientCfg.MaxSendMsgSize > 0, clientCfg.MaxSendMsgSize, 16*1024*1024)

	// HTTP/2 窗口大小配置（从配置文件读取）
	initialWindowSize := mathx.IF(clientCfg.InitialWindowSize > 0, clientCfg.InitialWindowSize, 1<<20)
	initialConnWindowSize := mathx.IF(clientCfg.InitialConnWindowSize > 0, clientCfg.InitialConnWindowSize, 1<<21)

	// 准备拨号选项
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		// 默认调用选项
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxRecvMsgSize),
			grpc.MaxCallSendMsgSize(maxSendMsgSize),
			grpc.WaitForReady(clientCfg.WaitForReady),
		),
		// Keepalive 配置（保持连接活跃）
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                keepaliveTime,    // 发送 keepalive ping 的间隔
			Timeout:             keepaliveTimeout, // 等待 keepalive ping 响应的超时时间
			PermitWithoutStream: true,             // 允许在没有活动流时发送 keepalive ping
		}),
		// HTTP/2 窗口大小配置
		grpc.WithInitialWindowSize(initialWindowSize),         // 初始窗口
		grpc.WithInitialConnWindowSize(initialConnWindowSize), // 连接窗口
	}

	// 启用客户端压缩
	if clientCfg.EnableCompression {
		ApplyClientCompression(clientCfg)
		compressType := ResolveCompressType(clientCfg.CompressionType)
		dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(grpc.UseCompressor(compressType)))
		gwglobal.LOGGER.Info("📦 %s 启用压缩: %s", serviceName, compressType)
	}

	// 负载均衡配置
	if clientCfg.EnableLoadBalance {
		policy := mathx.IF(clientCfg.LoadBalancePolicy != "", clientCfg.LoadBalancePolicy, "round_robin")
		// 使用 Service Config 配置负载均衡策略
		dialOpts = append(dialOpts, grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, policy)))
		gwglobal.LOGGER.Info("⚖️ %s 启用负载均衡: %s", serviceName, policy)
	}

	// 添加 Context 传播拦截器（确保 trace_id 在服务调用链中传递）
	dialOpts = append(dialOpts,
		grpc.WithChainUnaryInterceptor(
			middleware.UnaryClientRequestContextInterceptor(), // RequestContext 传播
			UnaryClientHealthInterceptor(serviceName, healthChecker),
		),
		grpc.WithChainStreamInterceptor(
			middleware.StreamClientRequestContextInterceptor(), // Stream RequestContext 传播
			StreamClientHealthInterceptor(serviceName, healthChecker),
		),
	)

	// 如果配置了 Network，添加到拨号选项
	if clientCfg.Network != "" {
		// 从配置读取连接超时
		dialTimeout := mathx.IF(clientCfg.ConnectionTimeout > 0, time.Duration(clientCfg.ConnectionTimeout)*time.Second, 30*time.Second)

		dialOpts = append(dialOpts, grpc.WithContextDialer(
			func(ctx context.Context, addr string) (net.Conn, error) {
				// 优化 TCP 连接参数（从配置读取）
				dialer := &net.Dialer{
					Timeout:   dialTimeout,
					KeepAlive: keepaliveTime,
				}
				return dialer.DialContext(ctx, clientCfg.Network, addr)
			},
		))
		gwglobal.LOGGER.Debug("🌐 %s 使用网络类型: %s (连接超时: %v)", serviceName, clientCfg.Network, dialTimeout)
	}

	return dialOpts
}
