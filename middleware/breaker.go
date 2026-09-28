/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 15:15:05
 * @FilePath: \go-rpc-gateway\middleware\breaker.go
 * @Description: 熔断器核心 - 断路器状态机与统计
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	breakerconfig "github.com/kamalyes/go-config/pkg/breaker"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

// breakerOpenResp 预分配的熔断 503 JSON 响应体（避免每次拒绝请求时 map 分配 + json 反射编码）
var breakerOpenResp = []byte(`{"code":503,"message":"Service temporarily unavailable (circuit breaker open)","success":false}` + "\n")

// BreakerState 熔断器状态
type BreakerState string

const (
	BreakerClosed   BreakerState = "closed"    // 关闭，正常工作
	BreakerOpen     BreakerState = "open"      // 打开，拒绝请求
	BreakerHalfOpen BreakerState = "half_open" // 半开，尝试恢复
)

// Breaker 断路器 - 核心业务逻辑
// 状态用 atomic.Value 存储，计数字段用原子操作，热路径（Allow/RecordSuccess/RecordFailure）完全无锁
// mu 仅保护状态转换复合操作（lastFailureTime/lastStateChangeAt 及转换原子性）
type Breaker struct {
	mu                sync.Mutex
	state             atomic.Value // BreakerState
	failureThreshold  int
	successThreshold  int
	timeout           time.Duration
	volumeThreshold   int
	failureCount      atomic.Int32 // 连续失败计数
	successCount      atomic.Int32 // 半开状态连续成功计数
	totalRequests     atomic.Int64
	failedRequests    atomic.Int64
	lastFailureTime   time.Time // mu 保护，仅 Open 超时判断读取
	lastStateChangeAt time.Time // mu 保护
}

// NewBreaker 创建断路器
func NewBreaker(failureThreshold, successThreshold, volumeThreshold int, timeout time.Duration) *Breaker {
	b := &Breaker{
		failureThreshold:  failureThreshold,
		successThreshold:  successThreshold,
		volumeThreshold:   volumeThreshold,
		timeout:           timeout,
		lastStateChangeAt: time.Now(),
	}
	b.state.Store(BreakerClosed)
	return b
}

// currentState 无锁读取当前状态（atomic.Value 保证读原子性）
func (b *Breaker) currentState() BreakerState {
	if v := b.state.Load(); v != nil {
		return v.(BreakerState)
	}
	return BreakerClosed
}

// Allow 检查是否允许请求
// Closed/HalfOpen 常态无锁放行，仅 Open 状态升级写锁检查超时恢复
func (b *Breaker) Allow() bool {
	// 快速路径：非 Open 状态直接放行（无锁）
	if b.currentState() != BreakerOpen {
		return true
	}

	// Open 状态：需要写锁检查超时并可能转换状态
	b.mu.Lock()
	defer b.mu.Unlock()

	// 二次检查（可能在升级写锁前被其他 goroutine 转换）
	if b.currentState() != BreakerOpen {
		return true
	}

	// 超时后尝试恢复，转为半开
	if time.Since(b.lastFailureTime) > b.timeout {
		b.transitionToLocked(BreakerHalfOpen)
		return true
	}
	return false
}

// RecordSuccess 记录成功
// Closed 常态仅原子清零失败计数，完全无锁
func (b *Breaker) RecordSuccess() {
	b.totalRequests.Add(1)

	if b.currentState() == BreakerClosed {
		b.failureCount.Store(0)
		return
	}

	// 慢路径：HalfOpen 需维护连续成功计数并可能恢复闭合
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failureCount.Store(0)
	if b.currentState() == BreakerHalfOpen {
		if b.successCount.Add(1) >= int32(b.successThreshold) {
			b.transitionToLocked(BreakerClosed)
		}
	}
}

// RecordFailure 记录失败
// Closed 常态仅原子更新计数，达到阈值时才升级写锁完成状态转换
func (b *Breaker) RecordFailure() {
	b.totalRequests.Add(1)
	b.failedRequests.Add(1)
	b.successCount.Store(0)

	if b.currentState() == BreakerClosed {
		// 快速路径：未达阈值直接返回（绝大多数失败请求走此分支）
		if b.failureCount.Add(1) < int32(b.failureThreshold) ||
			b.totalRequests.Load() < int64(b.volumeThreshold) {
			return
		}
		// 达到阈值：升级写锁完成状态转换（double-check）
		b.mu.Lock()
		b.lastFailureTime = time.Now()
		if b.currentState() == BreakerClosed &&
			b.failureCount.Load() >= int32(b.failureThreshold) &&
			b.totalRequests.Load() >= int64(b.volumeThreshold) {
			b.transitionToLocked(BreakerOpen)
		}
		b.mu.Unlock()
		return
	}

	// 慢路径：HalfOpen 失败立即重新熔断
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failureCount.Add(1)
	b.lastFailureTime = time.Now()

	switch b.currentState() {
	case BreakerClosed: // 竞态兜底：快速路径读取状态后可能已被转换
		if b.failureCount.Load() >= int32(b.failureThreshold) &&
			b.totalRequests.Load() >= int64(b.volumeThreshold) {
			b.transitionToLocked(BreakerOpen)
		}
	case BreakerHalfOpen:
		b.transitionToLocked(BreakerOpen)
	}
}

// transitionToLocked 转换状态（调用者必须持有 mu 写锁）
func (b *Breaker) transitionToLocked(newState BreakerState) {
	oldState := b.currentState()
	b.state.Store(newState)
	b.lastStateChangeAt = time.Now()

	// Closed 和 HalfOpen 重置计数器
	if newState == BreakerClosed || newState == BreakerHalfOpen {
		b.failureCount.Store(0)
		b.successCount.Store(0)
	}

	// 状态转换非热路径，日志在锁内可接受
	if global.LOGGER != nil {
		fields := map[string]interface{}{
			"old_state": oldState,
			"new_state": newState,
		}
		if newState == BreakerOpen {
			fields["failure_count"] = b.failureCount.Load()
			fields["total_requests"] = b.totalRequests.Load()
			global.LOGGER.WithFields(fields).WarnMsg("Circuit breaker opened due to high failure rate")
		} else {
			global.LOGGER.WithFields(fields).InfoMsg("Circuit breaker state changed")
		}
	}
}

// GetState 获取当前状态
func (b *Breaker) GetState() BreakerState {
	return b.currentState()
}

// GetStats 获取统计信息（基于 Snapshot，避免 map 分配）
func (b *Breaker) GetStats() BreakerSnapshot {
	return b.Snapshot()
}

// BreakerSnapshot 熔断器快照（强类型，便于指标采集）
type BreakerSnapshot struct {
	State          BreakerState
	TotalRequests  int64
	FailedRequests int64
	FailureCount   int32
	SuccessCount   int32
}

// Snapshot 获取强类型快照（全原子读取，无锁）
func (b *Breaker) Snapshot() BreakerSnapshot {
	return BreakerSnapshot{
		State:          b.currentState(),
		TotalRequests:  b.totalRequests.Load(),
		FailedRequests: b.failedRequests.Load(),
		FailureCount:   b.failureCount.Load(),
		SuccessCount:   b.successCount.Load(),
	}
}

// Reset 重置断路器
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state.Store(BreakerClosed)
	b.failureCount.Store(0)
	b.successCount.Store(0)
	b.totalRequests.Store(0)
	b.failedRequests.Store(0)
	b.lastStateChangeAt = time.Now()

	if global.LOGGER != nil {
		global.LOGGER.InfoMsg("Circuit breaker reset")
	}
}

// ============================================================================
// BreakerManager 断路器管理器 - 管理多个路径的断路器实例
// ============================================================================

// BreakerManager 断路器管理器 - 直接持有 go-config 的 breaker.CircuitBreaker 配置
type BreakerManager struct {
	mu             sync.RWMutex
	breakers       map[string]*Breaker
	config         *breakerconfig.CircuitBreaker
	excludePathSet map[string]struct{} // 排除路径集合，O(1) 查找替代线性扫描
}

// NewBreakerManager 创建断路器管理器（直接接收 go-config 配置对象）
func NewBreakerManager(cfg *breakerconfig.CircuitBreaker) *BreakerManager {
	cfg = mathx.IF(cfg == nil, breakerconfig.Default(), cfg)
	// 预构建排除路径集合，热路径 O(1) 查找
	excludeSet := make(map[string]struct{}, len(cfg.ExcludePaths))
	for _, p := range cfg.ExcludePaths {
		excludeSet[p] = struct{}{}
	}
	return &BreakerManager{
		breakers:       make(map[string]*Breaker),
		config:         cfg,
		excludePathSet: excludeSet,
	}
}

// GetBreaker 获取或创建断路器
func (m *BreakerManager) GetBreaker(path string) *Breaker {
	m.mu.RLock()
	if breaker, exists := m.breakers[path]; exists {
		m.mu.RUnlock()
		return breaker
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// 再次检查（double-check lock）
	if breaker, exists := m.breakers[path]; exists {
		return breaker
	}

	// 从配置创建断路器（timeout 为纳秒时间戳，转换为 time.Duration）
	var duration time.Duration
	if m.config.Timeout > 0 {
		duration = time.Duration(m.config.Timeout)
	}

	breaker := NewBreaker(m.config.FailureThreshold, m.config.SuccessThreshold, m.config.VolumeThreshold, duration)
	m.breakers[path] = breaker
	return breaker
}

// GetAllBreakers 获取所有断路器
func (m *BreakerManager) GetAllBreakers() map[string]*Breaker {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Breaker)
	for path, breaker := range m.breakers {
		result[path] = breaker
	}
	return result
}

// GetAllBreakerSnapshots 获取所有断路器的强类型快照（用于指标采集）
func (m *BreakerManager) GetAllBreakerSnapshots() map[string]BreakerSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]BreakerSnapshot, len(m.breakers))
	for path, breaker := range m.breakers {
		result[path] = breaker.Snapshot()
	}
	return result
}

// ResetBreaker 重置特定路径的断路器
func (m *BreakerManager) ResetBreaker(path string) bool {
	m.mu.RLock()
	breaker, exists := m.breakers[path]
	m.mu.RUnlock()

	if !exists {
		return false
	}

	breaker.Reset()
	return true
}

// ResetAllBreakers 重置所有断路器
func (m *BreakerManager) ResetAllBreakers() {
	m.mu.RLock()
	breakers := make([]*Breaker, 0, len(m.breakers))
	for _, breaker := range m.breakers {
		breakers = append(breakers, breaker)
	}
	m.mu.RUnlock()

	for _, breaker := range breakers {
		breaker.Reset()
	}
}

// GetStats 获取所有断路器的统计信息
// GetStats 获取所有断路器统计（等价于 GetAllBreakerSnapshots，保留以兼容旧 API）
func (m *BreakerManager) GetStats() map[string]BreakerSnapshot {
	return m.GetAllBreakerSnapshots()
}

// IsPathProtected 检查路径是否需要保护（排除路径 O(1) map 查找，保护路径前缀匹配）
func (m *BreakerManager) IsPathProtected(path string) bool {
	// 排除列表：O(1) map 查找
	if _, excluded := m.excludePathSet[path]; excluded {
		return false
	}

	// 保护列表：前缀匹配
	for _, preventionPath := range m.config.PreventionPaths {
		if len(path) >= len(preventionPath) && path[:len(preventionPath)] == preventionPath {
			return true
		}
	}

	return false
}

// BreakerHealthStatus 熔断器健康状态（强类型，替代 map[string]interface{}）
type BreakerHealthStatus struct {
	Total    int  // 断路器总数
	Open     int  // 打开数量
	HalfOpen int  // 半开数量
	Closed   int  // 关闭数量
	Healthy  bool // 是否健康（无打开的断路器）
}

// CountByState 单次遍历统计各状态断路器数量（替代三次独立遍历）
func (m *BreakerManager) CountByState() BreakerHealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := BreakerHealthStatus{Total: len(m.breakers)}
	for _, breaker := range m.breakers {
		switch breaker.GetState() {
		case BreakerOpen:
			status.Open++
		case BreakerHalfOpen:
			status.HalfOpen++
		case BreakerClosed:
			status.Closed++
		}
	}
	status.Healthy = status.Open == 0
	return status
}

// CountOpenBreakers 统计打开的断路器数量
func (m *BreakerManager) CountOpenBreakers() int {
	return m.CountByState().Open
}

// CountHalfOpenBreakers 统计半开的断路器数量
func (m *BreakerManager) CountHalfOpenBreakers() int {
	return m.CountByState().HalfOpen
}

// CountClosedBreakers 统计关闭的断路器数量
func (m *BreakerManager) CountClosedBreakers() int {
	return m.CountByState().Closed
}

// GetHealthStatus 获取健康状态（返回强类型结构体，单次遍历）
func (m *BreakerManager) GetHealthStatus() BreakerHealthStatus {
	return m.CountByState()
}

// ============================================================================
// HTTP 中间件适配
// ============================================================================

// BreakerHTTPMiddleware 创建熔断 HTTP 中间件（使用共享的 BreakerManager）
// 熔断器打开时返回 503 JSON 错误响应（使用预分配的响应体，零分配）
func BreakerHTTPMiddleware(manager *BreakerManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 检查路径是否需要保护
			if !manager.IsPathProtected(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// 获取对应路径的断路器
			breaker := manager.GetBreaker(r.URL.Path)

			// 检查断路器状态
			if !breaker.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write(breakerOpenResp)
				return
			}

			// 使用统一的 ResponseWriter 捕获状态码
			rw := NewResponseWriter(w)
			defer rw.Release()

			// 调用下一个处理器
			next.ServeHTTP(rw, r)

			// 根据响应状态码记录成功或失败
			if rw.IsServerError() {
				breaker.RecordFailure()
			} else {
				breaker.RecordSuccess()
			}
		})
	}
}
