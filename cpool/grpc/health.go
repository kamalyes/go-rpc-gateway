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
	"net"
	"sync"
	"time"

	gwglobal "github.com/kamalyes/go-rpc-gateway/global"
	"google.golang.org/grpc"
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

	// 异步初始健康检查
	go hc.checkEndpointHealth(serviceName, endpoint)
}

// IsHealthy 检查服务是否健康
func (hc *HealthChecker) IsHealthy(serviceName string) bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if health, exists := hc.clients[serviceName]; exists {
		health.mu.RLock()
		defer health.mu.RUnlock()
		return health.healthy
	}
	return false
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
