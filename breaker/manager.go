/*
* @Author: kamalyes 501893067@qq.com
* @Date: 2024-11-07 00:00:00
* @LastEditors: kamalyes 501893067@qq.com
* @LastEditTime: 2025-11-12 09:25:30
* @FilePath: \go-rpc-gateway\breaker\manager.go
* @Description: CircuitBreaker 管理器 - 管理多个断路器实例
*
* Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package breaker

import (
	"sync"
	"time"
)

// Manager 断路器管理器
type Manager struct {
	mu               sync.RWMutex
	breakers         map[string]*Breaker
	failureThreshold int
	successThreshold int
	timeout          int64 // 纳秒
	volumeThreshold  int
	preventionPaths  []string
	excludePaths     []string
}

// NewManager 创建断路器管理器
func NewManager(failureThreshold, successThreshold, volumeThreshold int, timeout int64, preventionPaths, excludePaths []string) *Manager {
	return &Manager{
		breakers:         make(map[string]*Breaker),
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
		volumeThreshold:  volumeThreshold,
		preventionPaths:  preventionPaths,
		excludePaths:     excludePaths,
	}
}

// GetBreaker 获取或创建断路器
func (m *Manager) GetBreaker(path string) *Breaker {
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

	// 从时间戳创建 duration
	var duration time.Duration
	if m.timeout > 0 {
		duration = time.Duration(m.timeout)
	}

	breaker := New(m.failureThreshold, m.successThreshold, m.volumeThreshold, duration)
	m.breakers[path] = breaker
	return breaker
}

// GetAllBreakers 获取所有断路器
func (m *Manager) GetAllBreakers() map[string]*Breaker {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Breaker)
	for path, breaker := range m.breakers {
		result[path] = breaker
	}
	return result
}

// ResetBreaker 重置特定路径的断路器
func (m *Manager) ResetBreaker(path string) bool {
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
func (m *Manager) ResetAllBreakers() {
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
func (m *Manager) GetStats() map[string]map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]map[string]interface{})
	for path, breaker := range m.breakers {
		stats[path] = breaker.GetStats()
	}
	return stats
}

// IsPathProtected 检查路径是否需要保护
func (m *Manager) IsPathProtected(path string) bool {
	// 检查排除列表
	for _, excludePath := range m.excludePaths {
		if path == excludePath {
			return false
		}
	}

	// 检查保护列表
	for _, preventionPath := range m.preventionPaths {
		if len(path) >= len(preventionPath) && path[:len(preventionPath)] == preventionPath {
			return true
		}
	}

	return false
}

// CountOpenBreakers 统计打开的断路器数量
func (m *Manager) CountOpenBreakers() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, breaker := range m.breakers {
		if breaker.GetState() == Open {
			count++
		}
	}
	return count
}

// CountHalfOpenBreakers 统计半开的断路器数量
func (m *Manager) CountHalfOpenBreakers() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, breaker := range m.breakers {
		if breaker.GetState() == HalfOpen {
			count++
		}
	}
	return count
}

// CountClosedBreakers 统计关闭的断路器数量
func (m *Manager) CountClosedBreakers() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, breaker := range m.breakers {
		if breaker.GetState() == Closed {
			count++
		}
	}
	return count
}

// GetHealthStatus 获取健康状态
func (m *Manager) GetHealthStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalBreakers := len(m.breakers)
	openCount := 0
	halfOpenCount := 0
	closedCount := 0

	for _, breaker := range m.breakers {
		state := breaker.GetState()
		switch state {
		case Open:
			openCount++
		case HalfOpen:
			halfOpenCount++
		case Closed:
			closedCount++
		}
	}

	isHealthy := openCount == 0

	return map[string]interface{}{
		"is_healthy":         isHealthy,
		"total_breakers":     totalBreakers,
		"open_breakers":      openCount,
		"half_open_breakers": halfOpenCount,
		"closed_breakers":    closedCount,
	}
}
