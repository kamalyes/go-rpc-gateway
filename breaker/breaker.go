/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 09:25:30
 * @FilePath: \engine-im-push-service\go-rpc-gateway\breaker\breaker.go
 * @Description: CircuitBreaker 核心模块 - 断路器业务逻辑独立维护
 * 将配置管理下沉到 go-config，核心业务逻辑由此模块维护
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package breaker

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/kamalyes/go-rpc-gateway/global"
)

// State 熔断器状态
type State string

const (
	Closed   State = "closed"    // 关闭，正常工作
	Open     State = "open"      // 打开，拒绝请求
	HalfOpen State = "half_open" // 半开，尝试恢复
)

// Breaker 断路器 - 核心业务逻辑
type Breaker struct {
	mu                sync.RWMutex
	state             State
	failureThreshold  int
	successThreshold  int
	timeout           time.Duration
	volumeThreshold   int
	failureCount      int32
	successCount      int32
	totalRequests     int64
	failedRequests    int64
	lastFailureTime   time.Time
	lastSuccessTime   time.Time
	lastStateChangeAt time.Time
}

// New 创建断路器
func New(failureThreshold, successThreshold, volumeThreshold int, timeout time.Duration) *Breaker {
	breaker := &Breaker{
		state:             Closed,
		failureThreshold:  failureThreshold,
		successThreshold:  successThreshold,
		volumeThreshold:   volumeThreshold,
		timeout:           timeout,
		lastStateChangeAt: time.Now(),
	}

	return breaker
}

// Allow 检查是否允许请求
func (b *Breaker) Allow() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.state == Closed {
		return true
	}

	if b.state == Open {
		// 检查是否可以尝试恢复
		if time.Since(b.lastFailureTime) > b.timeout {
			b.mu.RUnlock()
			b.transitionTo(HalfOpen)
			b.mu.RLock()
			return true
		}
		return false
	}

	// HalfOpen 状态，允许部分请求
	return true
}

// RecordSuccess 记录成功
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	atomic.AddInt64(&b.totalRequests, 1)
	atomic.StoreInt32(&b.failureCount, 0)

	switch b.state {
	case Closed:
		b.lastSuccessTime = time.Now()

	case HalfOpen:
		atomic.AddInt32(&b.successCount, 1)
		if atomic.LoadInt32(&b.successCount) >= int32(b.successThreshold) {
			b.mu.Unlock()
			b.transitionTo(Closed)
			b.mu.Lock()
		}
		b.lastSuccessTime = time.Now()
	}
}

// RecordFailure 记录失败
func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	atomic.AddInt64(&b.totalRequests, 1)
	atomic.AddInt64(&b.failedRequests, 1)
	atomic.AddInt32(&b.failureCount, 1)
	atomic.StoreInt32(&b.successCount, 0)

	b.lastFailureTime = time.Now()

	switch b.state {
	case Closed:
		if atomic.LoadInt32(&b.failureCount) >= int32(b.failureThreshold) {
			if atomic.LoadInt64(&b.totalRequests) >= int64(b.volumeThreshold) {
				b.mu.Unlock()
				b.transitionTo(Open)
				b.mu.Lock()

				if global.LOGGER != nil {
					global.LOGGER.WithFields(map[string]interface{}{
						"failure_count":  b.failureCount,
						"total_requests": b.totalRequests,
					}).WarnMsg("Circuit breaker opened due to high failure rate")
				}
			}
		}

	case HalfOpen:
		b.mu.Unlock()
		b.transitionTo(Open)
		b.mu.Lock()
	}
}

// transitionTo 转换到新状态
func (b *Breaker) transitionTo(newState State) {
	b.mu.Lock()
	defer b.mu.Unlock()

	oldState := b.state
	b.state = newState
	b.lastStateChangeAt = time.Now()

	// 重置计数器
	if newState == Closed {
		atomic.StoreInt32(&b.failureCount, 0)
		atomic.StoreInt32(&b.successCount, 0)
	} else if newState == HalfOpen {
		atomic.StoreInt32(&b.failureCount, 0)
		atomic.StoreInt32(&b.successCount, 0)
	}

	if global.LOGGER != nil {
		global.LOGGER.WithFields(map[string]interface{}{
			"old_state": oldState,
			"new_state": newState,
			"timestamp": b.lastStateChangeAt,
		}).InfoMsg("Circuit breaker state changed")
	}
}

// GetState 获取当前状态
func (b *Breaker) GetState() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// GetStats 获取统计信息
func (b *Breaker) GetStats() map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()

	failureRate := 0.0
	if b.totalRequests > 0 {
		failureRate = float64(b.failedRequests) / float64(b.totalRequests) * 100
	}

	return map[string]interface{}{
		"state":             b.state,
		"total_requests":    atomic.LoadInt64(&b.totalRequests),
		"failed_requests":   atomic.LoadInt64(&b.failedRequests),
		"failure_rate":      failureRate,
		"failure_count":     atomic.LoadInt32(&b.failureCount),
		"success_count":     atomic.LoadInt32(&b.successCount),
		"last_failure_time": b.lastFailureTime,
		"last_success_time": b.lastSuccessTime,
		"last_state_change": b.lastStateChangeAt,
		"uptime":            time.Since(b.lastStateChangeAt).String(),
	}
}

// Reset 重置断路器
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = Closed
	atomic.StoreInt32(&b.failureCount, 0)
	atomic.StoreInt32(&b.successCount, 0)
	atomic.StoreInt64(&b.totalRequests, 0)
	atomic.StoreInt64(&b.failedRequests, 0)
	b.lastStateChangeAt = time.Now()

	if global.LOGGER != nil {
		global.LOGGER.InfoMsg("Circuit breaker reset")
	}
}
