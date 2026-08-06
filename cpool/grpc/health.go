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
	"google.golang.org/grpc/connectivity"
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
	clients   map[string]*ClientHealth
	onRecover func(serviceName string) // 服务从不可用恢复为可用时的回调
	mu        sync.RWMutex
}

var connReadyTimeout = time.Second * 2

// NewHealthChecker 创建健康检查管理器
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		clients: make(map[string]*ClientHealth),
	}
}

// SetOnRecover 设置服务恢复回调
// 当服务在定期检查中从不可用变为可用时，回调会被调用
// 用于服务恢复后重新发现服务和注册 HTTP 路由
func (hc *HealthChecker) SetOnRecover(fn func(serviceName string)) {
	hc.mu.Lock()
	hc.onRecover = fn
	hc.mu.Unlock()
}

// Register 注册客户端到健康检查
// endpoint 仅用于 conn 为 nil 时的 TCP 探测降级（测试场景），
// 生产环境 conn 非 nil 时通过 ClientConn 连接状态判断健康，支持多 Pod 负载均衡
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

// checkConnReady 检查 ClientConn 是否在超时内进入 Ready 状态
// Ready 表示至少一个后端 Pod 可用
// TransientFailure 时不立即返回，等待状态变化以支持服务恢复后连接重建
// 调用方应先 ResetConnectBackoff 触发立即重连，避免退避等待
func checkConnReady(conn *grpc.ClientConn, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return true
		}
		if !conn.WaitForStateChange(ctx, state) {
			return false // 超时
		}
	}
}

// checkEndpointHealth 检查服务健康状态
// conn 非 nil 时通过 ClientConn 连接状态判断（支持多 Pod 负载均衡），
// conn 为 nil 时降级为 TCP 端口探测（仅测试场景）
func (hc *HealthChecker) checkEndpointHealth(serviceName, endpoint string) {
	hc.mu.RLock()
	health, exists := hc.clients[serviceName]
	hc.mu.RUnlock()

	if !exists {
		return
	}

	var healthy bool
	if health.conn != nil {
		// 通过 ClientConn 连接状态判断健康（支持多 Pod 负载均衡）
		// ResetConnectBackoff 重置退避，触发立即重连，避免因历史失败状态导致健康检查误判
		health.conn.ResetConnectBackoff()
		health.conn.Connect()
		healthy = checkConnReady(health.conn, connReadyTimeout)
		if healthy {
			gwglobal.LOGGER.Info("✅ %s 服务健康 (ClientConn Ready)", serviceName)
		} else {
			gwglobal.LOGGER.Error("⚠️  %s 服务不健康 (ClientConn state: %s)", serviceName, health.conn.GetState())
		}
	} else {
		// 无 ClientConn 时降级为 TCP 端口探测（仅用于测试场景）
		conn, err := net.DialTimeout("tcp", endpoint, connReadyTimeout)
		if err == nil {
			healthy = true
			conn.Close()
			gwglobal.LOGGER.Info("✅ %s 服务端口可达 -> %s", serviceName, endpoint)
		} else {
			gwglobal.LOGGER.Error("⚠️  %s 服务端口不可达 -> %s (%v)", serviceName, endpoint, err)
		}
	}

	health.mu.Lock()
	health.healthy = healthy
	health.lastCheck = time.Now()
	health.mu.Unlock()
}

// StartPeriodicCheck 启动定期健康检查
// interval: 检查间隔时间
// endpoints: 服务名到端点的映射（仅用于 conn 为 nil 时的 TCP 探测降级）
//
// conn 非 nil 时通过 ClientConn 连接状态判断健康：
//   - Ready → 健康（至少一个 Pod 可用）
//   - TransientFailure → 不健康（所有 Pod 不可用）
//   - Connecting/Idle → 保持上一次状态，避免瞬态重连导致误判
//
// 服务从不可用恢复为可用时（状态变为 Ready），直接触发 onRecover 回调，
// 无需额外等待连接就绪，因为 ClientConn Ready 已保证连接可用
func (hc *HealthChecker) StartPeriodicCheck(interval time.Duration, endpoints map[string]string) {
	if len(hc.clients) == 0 {
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
				endpoint := endpoints[serviceName]
				go func(name, ep string, h *ClientHealth) {
					var healthy bool
				if h.conn != nil {
					// 通过 ClientConn 连接状态判断（支持多 Pod 负载均衡）
					h.conn.Connect()
					state := h.conn.GetState()
					switch state {
					case connectivity.Ready:
						healthy = true
					case connectivity.TransientFailure:
						// 重置退避定时器，使下次检查周期触发立即重连，避免 Pod 恢复后因退避延迟探测
						h.conn.ResetConnectBackoff()
						healthy = false
					default:
						// Connecting/Idle: 保持上一次状态，避免瞬态重连导致误判
						h.mu.RLock()
						healthy = h.healthy
						h.mu.RUnlock()
					}
				} else if ep != "" {
						// 无 ClientConn 时降级为 TCP 端口探测（仅测试场景）
						conn, err := net.DialTimeout("tcp", ep, connReadyTimeout)
						if err == nil {
							healthy = true
							conn.Close()
						}
					}

					h.mu.Lock()
					wasHealthy := h.healthy
					h.healthy = healthy
					h.lastCheck = time.Now()
					h.mu.Unlock()

					// 服务从不可用恢复为可用，触发回调（重新发现服务和注册路由）
					// ClientConn Ready 即代表连接就绪，无需额外等待
					if healthy && !wasHealthy {
						hc.mu.RLock()
						callback := hc.onRecover
						hc.mu.RUnlock()
						if callback != nil {
							callback(name)
						}
					}

					if !healthy && h.conn != nil {
						gwglobal.LOGGER.Error("⚠️  %s 服务不健康 (ClientConn state: %s)", name, h.conn.GetState())
					}
				}(serviceName, endpoint, health)
			}
		}
	}()
	gwglobal.LOGGER.Info("🏥 gRPC 健康检查循环已启动 (间隔: %v)", interval)
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
