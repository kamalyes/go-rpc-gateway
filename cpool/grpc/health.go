/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-21 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-21 10:30:00
 * @FilePath: \go-rpc-gateway\cpool\grpc\health.go
 * @Description: gRPC 客户端健康检查管理
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package grpc

import (
	"context"
	"net"
	"sync"
	"time"

	gwglobal "github.com/kamalyes/go-rpc-gateway/global"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ClientHealth gRPC 客户端健康状态
type ClientHealth struct {
	conn      *grpc.ClientConn
	healthy   bool
	lastCheck time.Time
	mu        sync.RWMutex
}

// HealthChecker gRPC 健康检查管理器
type HealthChecker struct {
	clients map[string]*ClientHealth
	mu      sync.RWMutex
}

// NewHealthChecker 创建健康检查管理器
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		clients: make(map[string]*ClientHealth),
	}
}

// Register 注册客户端到健康检查
func (hc *HealthChecker) Register(serviceName string, conn *grpc.ClientConn, endpoint string) {
	health := &ClientHealth{
		conn:      conn,
		healthy:   false,
		lastCheck: time.Now(),
	}

	hc.mu.Lock()
	hc.clients[serviceName] = health
	hc.mu.Unlock()

	// 同步执行首次健康检查，避免客户端调用早于首轮检查导致误判不可用
	hc.checkEndpointHealth(serviceName, endpoint)
}

// IsHealthy 检查服务是否健康
func (hc *HealthChecker) IsHealthy(serviceName string) bool {
	healthy, _ := hc.GetServiceHealth(serviceName)
	return healthy
}

// GetServiceHealth 获取服务健康状态及是否已注册
func (hc *HealthChecker) GetServiceHealth(serviceName string) (healthy bool, exists bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	health, exists := hc.clients[serviceName]
	if !exists {
		return false, false
	}

	health.mu.RLock()
	defer health.mu.RUnlock()
	return health.healthy, true
}

// checkEndpointHealth 通过 TCP 连接检查服务端口可达性（类似 telnet）
func (hc *HealthChecker) checkEndpointHealth(serviceName, endpoint string) {
	hc.mu.RLock()
	health, exists := hc.clients[serviceName]
	hc.mu.RUnlock()

	if !exists {
		return
	}

	// 尝试 TCP 连接，超时 3 秒
	conn, err := net.DialTimeout("tcp", endpoint, 3*time.Second)

	health.mu.Lock()
	if err == nil {
		health.healthy = true
		conn.Close() // 立即关闭测试连接
		gwglobal.LOGGER.Info("✅ %s 服务端口可达 -> %s", serviceName, endpoint)
	} else {
		health.healthy = false
		gwglobal.LOGGER.Warn("⚠️  %s 服务端口不可达 -> %s (%v)", serviceName, endpoint, err)
	}
	health.lastCheck = time.Now()
	health.mu.Unlock()
}

// StartPeriodicCheck 启动定期健康检查
// interval: 检查间隔时间
// endpoints: 服务名到端点的映射
func (hc *HealthChecker) StartPeriodicCheck(interval time.Duration, endpoints map[string]string) {
	if len(endpoints) == 0 {
		return
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			hc.mu.RLock()
			services := make(map[string]*ClientHealth, len(hc.clients))
			for k, v := range hc.clients {
				services[k] = v
			}
			hc.mu.RUnlock()

			for serviceName, health := range services {
				if endpoint, exists := endpoints[serviceName]; exists {
					go func(name, ep string, h *ClientHealth) {
						// TCP 端口连接检查
						conn, err := net.DialTimeout("tcp", ep, 3*time.Second)

						h.mu.Lock()
						if err == nil {
							h.healthy = true
							conn.Close()
						} else {
							h.healthy = false
							gwglobal.LOGGER.Warn("⚠️  %s 服务端口不可达 -> %s (%v)", name, ep, err)
						}
						h.lastCheck = time.Now()
						h.mu.Unlock()
					}(serviceName, endpoint, health)
				}
			}
		}
	}()
	gwglobal.LOGGER.Info("🏥 gRPC 健康检查循环已启动 (TCP 端口探测，间隔: %v)", interval)
}

// GetHealthStatus 获取所有服务的健康状态
func (hc *HealthChecker) GetHealthStatus() map[string]bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	status := make(map[string]bool, len(hc.clients))
	for name, health := range hc.clients {
		health.mu.RLock()
		status[name] = health.healthy
		health.mu.RUnlock()
	}
	return status
}

// Close 关闭所有客户端连接
func (hc *HealthChecker) Close() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	for name, health := range hc.clients {
		if health.conn != nil {
			if err := health.conn.Close(); err != nil {
				gwglobal.LOGGER.Warn("⚠️  关闭 %s 连接失败: %v", name, err)
			}
		}
	}
	hc.clients = make(map[string]*ClientHealth)
	return nil
}

// ServiceGuard 服务可用性链式校验器
type ServiceGuard struct {
	serviceName string
	client      any
	isHealthy   func(string) bool
}

// NewServiceGuard 创建服务校验器
func NewServiceGuard(serviceName string) ServiceGuard {
	return ServiceGuard{
		serviceName: serviceName,
	}
}

// WithServiceName 设置服务名称
func (g ServiceGuard) WithServiceName(serviceName string) ServiceGuard {
	g.serviceName = serviceName
	return g
}

// WithClient 设置客户端
func (g ServiceGuard) WithClient(client any) ServiceGuard {
	g.client = client
	return g
}

// WithHealthChecker 设置健康检查函数
func (g ServiceGuard) WithHealthChecker(isHealthy func(string) bool) ServiceGuard {
	g.isHealthy = isHealthy
	return g
}

// Ensure 执行校验
func (g ServiceGuard) Ensure() error {
	return EnsureServiceReady(g.client, g.isHealthy, g.serviceName)
}

// EnsureServiceReady 校验服务依赖是否可用
func EnsureServiceReady(client any, isHealthy func(string) bool, serviceName string) error {
	if client == nil {
		return status.Errorf(codes.FailedPrecondition, "%s client is not initialized", serviceName)
	}

	if isHealthy != nil && !isHealthy(serviceName) {
		return status.Errorf(codes.Unavailable, "%s is unavailable", serviceName)
	}

	return nil
}

// UnaryClientHealthInterceptor gRPC Unary 客户端健康检查拦截器
func UnaryClientHealthInterceptor(serviceName string, checker *HealthChecker) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if checker != nil {
			if healthy, exists := checker.GetServiceHealth(serviceName); exists && !healthy {
				return status.Errorf(codes.Unavailable, "%s is unavailable", serviceName)
			}
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// StreamClientHealthInterceptor gRPC Stream 客户端健康检查拦截器
func StreamClientHealthInterceptor(serviceName string, checker *HealthChecker) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if checker != nil {
			if healthy, exists := checker.GetServiceHealth(serviceName); exists && !healthy {
				return nil, status.Errorf(codes.Unavailable, "%s is unavailable", serviceName)
			}
		}
		return streamer(ctx, desc, cc, method, opts...)
	}
}
