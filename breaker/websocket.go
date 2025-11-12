/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 07:18:12
 * @FilePath: \go-rpc-gateway\breaker\websocket.go
 * @Description: WebSocket 适配器 - 为 WebSocket 连接提供断路器保护
 * 集成 go-wsc 库，实现连接级别的断路器
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package breaker

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kamalyes/go-rpc-gateway/global"
	wsc "github.com/kamalyes/go-wsc"
)

// WSConnection WebSocket 受保护的连接
type WSConnection struct {
	mu              sync.RWMutex
	conn            *wsc.Wsc
	breaker         *Breaker
	messageQueue    chan interface{}
	failureCount    int32
	totalRequests   int64
	failedRequests  int64
	lastFailureTime time.Time
	isHealthy       bool
	closeOnce       sync.Once
	done            chan struct{}
	maxRetries      int
	retryBackoff    float64
	healthCheckFreq time.Duration
}

// NewWSConnection 创建受保护的 WebSocket 连接
func NewWSConnection(conn *wsc.Wsc, breaker *Breaker, maxRetries int, retryBackoff float64, healthCheckFreq time.Duration) *WSConnection {
	wsconn := &WSConnection{
		conn:            conn,
		breaker:         breaker,
		messageQueue:    make(chan interface{}, 1000),
		isHealthy:       true,
		done:            make(chan struct{}),
		maxRetries:      maxRetries,
		retryBackoff:    retryBackoff,
		healthCheckFreq: healthCheckFreq,
	}

	// 启动消息处理和健康检查协程
	go wsconn.processMessageQueue()
	go wsconn.healthCheck()

	return wsconn
}

// SendMessage 发送文本消息（带重试和断路器保护）
func (wsc *WSConnection) SendMessage(message string) error {
	if !wsc.breaker.Allow() {
		atomic.AddInt64(&wsc.failedRequests, 1)
		return fmt.Errorf("circuit breaker is open")
	}

	var lastErr error
	retryDelay := time.Duration(0)

	for attempt := 0; attempt <= wsc.maxRetries; attempt++ {
		if attempt > 0 {
			retryDelay = time.Duration(float64(time.Millisecond*100) * wsc.exponentialBackoff(attempt))
			time.Sleep(retryDelay)
		}

		err := wsc.conn.SendTextMessage(message)
		if err == nil {
			atomic.AddInt64(&wsc.totalRequests, 1)
			wsc.breaker.RecordSuccess()
			atomic.StoreInt32(&wsc.failureCount, 0)
			return nil
		}

		lastErr = err
		atomic.AddInt32(&wsc.failureCount, 1)
	}

	atomic.AddInt64(&wsc.totalRequests, 1)
	atomic.AddInt64(&wsc.failedRequests, 1)
	wsc.breaker.RecordFailure()
	wsc.lastFailureTime = time.Now()

	if global.LOGGER != nil {
		global.LOGGER.WithFields(map[string]interface{}{
			"attempt":       wsc.maxRetries + 1,
			"error":         lastErr,
			"failure_count": atomic.LoadInt32(&wsc.failureCount),
		}).ErrorMsg("Failed to send WebSocket message after retries")
	}

	return lastErr
}

// SendBinaryMessage 发送二进制消息（带重试和断路器保护）
func (wsc *WSConnection) SendBinaryMessage(data []byte) error {
	if !wsc.breaker.Allow() {
		atomic.AddInt64(&wsc.failedRequests, 1)
		return fmt.Errorf("circuit breaker is open")
	}

	var lastErr error
	retryDelay := time.Duration(0)

	for attempt := 0; attempt <= wsc.maxRetries; attempt++ {
		if attempt > 0 {
			retryDelay = time.Duration(float64(time.Millisecond*100) * wsc.exponentialBackoff(attempt))
			time.Sleep(retryDelay)
		}

		err := wsc.conn.SendBinaryMessage(data)
		if err == nil {
			atomic.AddInt64(&wsc.totalRequests, 1)
			wsc.breaker.RecordSuccess()
			atomic.StoreInt32(&wsc.failureCount, 0)
			return nil
		}

		lastErr = err
		atomic.AddInt32(&wsc.failureCount, 1)
	}

	atomic.AddInt64(&wsc.totalRequests, 1)
	atomic.AddInt64(&wsc.failedRequests, 1)
	wsc.breaker.RecordFailure()
	wsc.lastFailureTime = time.Now()

	if global.LOGGER != nil {
		global.LOGGER.WithFields(map[string]interface{}{
			"attempt":       wsc.maxRetries + 1,
			"error":         lastErr,
			"failure_count": atomic.LoadInt32(&wsc.failureCount),
		}).ErrorMsg("Failed to send binary WebSocket message after retries")
	}

	return lastErr
}

// QueueMessage 将消息加入队列（异步处理）
func (wsc *WSConnection) QueueMessage(message interface{}) error {
	select {
	case wsc.messageQueue <- message:
		return nil
	case <-wsc.done:
		return fmt.Errorf("connection is closed")
	default:
		return fmt.Errorf("message queue is full")
	}
}

// processMessageQueue 处理消息队列
func (wsc *WSConnection) processMessageQueue() {
	for {
		select {
		case msg := <-wsc.messageQueue:
			if msg == nil {
				return
			}

			// 类型断言处理不同的消息类型
			switch v := msg.(type) {
			case string:
				_ = wsc.SendMessage(v)
			case []byte:
				_ = wsc.SendBinaryMessage(v)
			default:
				if global.LOGGER != nil {
					global.LOGGER.WithField("type", fmt.Sprintf("%T", msg)).WarnMsg("Unknown message type in queue")
				}
			}

		case <-wsc.done:
			return
		}
	}
}

// healthCheck 健康检查协程
func (wsc *WSConnection) healthCheck() {
	ticker := time.NewTicker(wsc.healthCheckFreq)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			wsc.mu.Lock()
			if wsc.conn != nil {
				wsc.isHealthy = true
			} else {
				wsc.isHealthy = false
			}
			wsc.mu.Unlock()

		case <-wsc.done:
			return
		}
	}
}

// exponentialBackoff 指数退避计算
func (wsc *WSConnection) exponentialBackoff(attempt int) float64 {
	return wsc.retryBackoff * float64(attempt)
}

// Close 关闭连接
func (wsc *WSConnection) Close() error {
	wsc.closeOnce.Do(func() {
		close(wsc.done)
		close(wsc.messageQueue)

		wsc.mu.Lock()
		if wsc.conn != nil {
			wsc.conn.Close()
		}
		wsc.mu.Unlock()

		if global.LOGGER != nil {
			global.LOGGER.WithFields(map[string]interface{}{
				"total_requests":  atomic.LoadInt64(&wsc.totalRequests),
				"failed_requests": atomic.LoadInt64(&wsc.failedRequests),
			}).InfoMsg("Protected WebSocket connection closed")
		}
	})
	return nil
}

// GetStats 获取统计信息
func (wsc *WSConnection) GetStats() map[string]interface{} {
	wsc.mu.RLock()
	defer wsc.mu.RUnlock()

	total := atomic.LoadInt64(&wsc.totalRequests)
	failed := atomic.LoadInt64(&wsc.failedRequests)
	failureRate := 0.0

	if total > 0 {
		failureRate = float64(failed) / float64(total) * 100
	}

	return map[string]interface{}{
		"total_requests":    total,
		"failed_requests":   failed,
		"failure_rate":      failureRate,
		"is_healthy":        wsc.isHealthy,
		"last_failure_time": wsc.lastFailureTime,
		"queue_size":        len(wsc.messageQueue),
		"failure_count":     atomic.LoadInt32(&wsc.failureCount),
	}
}

// IsHealthy 检查连接是否健康
func (wsc *WSConnection) IsHealthy() bool {
	wsc.mu.RLock()
	defer wsc.mu.RUnlock()
	return wsc.isHealthy && wsc.breaker.Allow()
}

// WSPool WebSocket 连接池（带断路器保护）
type WSPool struct {
	mu          sync.RWMutex
	connections map[string]*WSConnection
	manager     *Manager
}

// NewWSPool 创建 WebSocket 连接池
func NewWSPool(manager *Manager) *WSPool {
	return &WSPool{
		connections: make(map[string]*WSConnection),
		manager:     manager,
	}
}

// Register 注册受保护的连接
func (wsp *WSPool) Register(connID string, conn *wsc.Wsc, maxRetries int, retryBackoff float64, healthCheckFreq time.Duration) (*WSConnection, error) {
	wsp.mu.Lock()
	defer wsp.mu.Unlock()

	if _, exists := wsp.connections[connID]; exists {
		return nil, fmt.Errorf("connection already registered: %s", connID)
	}

	breaker := wsp.manager.GetBreaker(connID)
	wsconn := NewWSConnection(conn, breaker, maxRetries, retryBackoff, healthCheckFreq)
	wsp.connections[connID] = wsconn

	return wsconn, nil
}

// Unregister 注销连接
func (wsp *WSPool) Unregister(connID string) error {
	wsp.mu.Lock()
	conn, exists := wsp.connections[connID]
	delete(wsp.connections, connID)
	wsp.mu.Unlock()

	if !exists {
		return fmt.Errorf("connection not found: %s", connID)
	}

	return conn.Close()
}

// GetConnection 获取连接
func (wsp *WSPool) GetConnection(connID string) *WSConnection {
	wsp.mu.RLock()
	defer wsp.mu.RUnlock()
	return wsp.connections[connID]
}

// GetAllStats 获取所有连接的统计信息
func (wsp *WSPool) GetAllStats() map[string]interface{} {
	wsp.mu.RLock()
	defer wsp.mu.RUnlock()

	stats := make(map[string]interface{})
	totalConns := 0
	totalHealthy := 0

	for connID, conn := range wsp.connections {
		connStats := conn.GetStats()
		stats[connID] = connStats

		totalConns++
		if conn.IsHealthy() {
			totalHealthy++
		}
	}

	return map[string]interface{}{
		"total_connections":   totalConns,
		"healthy_connections": totalHealthy,
		"connections":         stats,
	}
}

// Close 关闭所有连接
func (wsp *WSPool) Close() error {
	wsp.mu.Lock()
	defer wsp.mu.Unlock()

	for _, conn := range wsp.connections {
		_ = conn.Close()
	}

	wsp.connections = make(map[string]*WSConnection)
	return nil
}
