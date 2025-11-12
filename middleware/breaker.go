/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 09:41:45
 * @FilePath: \engine-im-push-service\go-rpc-gateway\middleware\breaker.go
 * @Description: CircuitBreaker 中间件适配器 - 在 middleware 模块下统一管理中间件
 * 通过此适配器将 breaker 模块的功能集成到标准的中间件框架中
 * 配置使用 go-config/pkg/breaker 中定义的结构
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	goconfigbreaker "github.com/kamalyes/go-config/pkg/breaker"
	"github.com/kamalyes/go-rpc-gateway/breaker"
	"github.com/kamalyes/go-rpc-gateway/global"
)

// BreakerMiddlewareAdapter 熔断中间件适配器
// 提供统一的中间件工厂函数和配置管理
// 配置来自 go-config/pkg/breaker，核心逻辑使用 breaker 模块
type BreakerMiddlewareAdapter struct {
	manager   *breaker.Manager
	config    *goconfigbreaker.CircuitBreaker
	mu        sync.RWMutex
	enabled   bool
	statsLock sync.RWMutex
	stats     *BreakerStats
}

// BreakerStats 熔断器统计信息
type BreakerStats struct {
	TotalRequests    int64                  `json:"totalRequests"`
	FailedRequests   int64                  `json:"failedRequests"`
	SuccessRequests  int64                  `json:"successRequests"`
	BlockedRequests  int64                  `json:"blockedRequests"`
	OpenBreakers     int                    `json:"openBreakers"`
	HalfOpenBreakers int                    `json:"halfOpenBreakers"`
	ClosedBreakers   int                    `json:"closedBreakers"`
	AverageLatency   float64                `json:"averageLatency"`
	LastUpdatedAt    time.Time              `json:"lastUpdatedAt"`
	BreakerStats     map[string]interface{} `json:"breakerStats,omitempty"`
}

// NewBreakerMiddlewareAdapter 创建熔断中间件适配器
// 使用 go-config 中的 CircuitBreaker 配置
func NewBreakerMiddlewareAdapter(config *goconfigbreaker.CircuitBreaker) *BreakerMiddlewareAdapter {
	if config == nil {
		config = goconfigbreaker.Default()
	}

	manager := breaker.NewManager(
		config.FailureThreshold,
		config.SuccessThreshold,
		config.VolumeThreshold,
		config.Timeout,
		config.PreventionPaths,
		config.ExcludePaths,
	)

	adapter := &BreakerMiddlewareAdapter{
		manager: manager,
		config:  config,
		enabled: config.Enabled,
		stats: &BreakerStats{
			LastUpdatedAt: time.Now(),
		},
	}

	// 启动定期收集指标
	if config.Enabled {
		go adapter.collectMetricsRoutine()
	}

	return adapter
}

// NewBreakerMiddlewareAdapterWithDefaults 使用默认配置创建适配器
func NewBreakerMiddlewareAdapterWithDefaults() *BreakerMiddlewareAdapter {
	return NewBreakerMiddlewareAdapter(goconfigbreaker.Default())
}

// Middleware 返回中间件函数
func (a *BreakerMiddlewareAdapter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 如果禁用，直接通过
			if !a.enabled || !a.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// 检查路径是否需要保护
			if !a.manager.IsPathProtected(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// 获取对应路径的断路器
			b := a.manager.GetBreaker(r.URL.Path)

			// 检查断路器状态
			if !b.Allow() {
				a.incrementBlockedRequests()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)

				resp := map[string]interface{}{
					"code":    503,
					"message": "Service temporarily unavailable (circuit breaker open)",
					"success": false,
					"path":    r.URL.Path,
				}

				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			// 包装响应写入器以捕获状态码
			wrappedWriter := newBreakerResponseWriter(w)

			// 记录开始时间
			startTime := time.Now()

			// 调用下一个处理器
			next.ServeHTTP(wrappedWriter, r)

			// 计算延迟
			latency := time.Since(startTime)

			// 根据响应状态码记录成功或失败
			if wrappedWriter.statusCode >= 500 {
				b.RecordFailure()
				a.incrementFailedRequests()
			} else {
				b.RecordSuccess()
				a.incrementSuccessRequests()
			}

			a.incrementTotalRequests()
			a.updateAverageLatency(latency)
		})
	}
}

// breakerResponseWriter 响应写入器包装器 - 用于捕获状态码
type breakerResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// newBreakerResponseWriter 创建响应写入器包装器
func newBreakerResponseWriter(w http.ResponseWriter) *breakerResponseWriter {
	return &breakerResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

// WriteHeader 重写 WriteHeader 方法
func (rw *breakerResponseWriter) WriteHeader(statusCode int) {
	if !rw.written {
		rw.statusCode = statusCode
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write 重写 Write 方法
func (rw *breakerResponseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.statusCode = http.StatusOK
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

// Enable 启用熔断器
func (a *BreakerMiddlewareAdapter) Enable() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = true
}

// Disable 禁用熔断器
func (a *BreakerMiddlewareAdapter) Disable() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = false
}

// IsEnabled 检查是否启用
func (a *BreakerMiddlewareAdapter) IsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.enabled
}

// GetManager 获取管理器
func (a *BreakerMiddlewareAdapter) GetManager() *breaker.Manager {
	return a.manager
}

// GetConfig 获取配置
func (a *BreakerMiddlewareAdapter) GetConfig() *goconfigbreaker.CircuitBreaker {
	return a.config
}

// GetStats 获取统计信息
func (a *BreakerMiddlewareAdapter) GetStats() *BreakerStats {
	a.statsLock.RLock()
	defer a.statsLock.RUnlock()

	stats := &BreakerStats{
		TotalRequests:   a.stats.TotalRequests,
		FailedRequests:  a.stats.FailedRequests,
		SuccessRequests: a.stats.SuccessRequests,
		BlockedRequests: a.stats.BlockedRequests,
		AverageLatency:  a.stats.AverageLatency,
		LastUpdatedAt:   a.stats.LastUpdatedAt,
	}

	// 获取管理器统计信息
	managerStats := a.manager.GetStats()
	if len(managerStats) > 0 {
		// 转换嵌套 map: map[string]map[string]interface{} -> map[string]interface{}
		for pathKey, pathStats := range managerStats {
			stats.BreakerStats[pathKey] = pathStats
		}
	}

	return stats
}

// Reset 重置统计信息
func (a *BreakerMiddlewareAdapter) Reset() {
	a.statsLock.Lock()
	defer a.statsLock.Unlock()

	a.stats = &BreakerStats{
		LastUpdatedAt: time.Now(),
	}

	// 重置所有断路器
	for _, b := range a.manager.GetAllBreakers() {
		b.Reset()
	}
}

// collectMetricsRoutine 定期收集指标
func (a *BreakerMiddlewareAdapter) collectMetricsRoutine() {
	// 如果配置中指定了间隔，使用该间隔（转换为 nanoseconds），否则使用默认 10 秒
	interval := 10 * time.Second
	if a.config.SlidingWindowBucket > 0 {
		interval = time.Duration(a.config.SlidingWindowBucket) // SlidingWindowBucket 以纳秒为单位
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if !a.IsEnabled() {
			continue
		}

		a.statsLock.Lock()
		a.stats.OpenBreakers = a.manager.CountOpenBreakers()
		a.stats.HalfOpenBreakers = a.manager.CountHalfOpenBreakers()
		a.stats.ClosedBreakers = a.manager.CountClosedBreakers()
		a.stats.LastUpdatedAt = time.Now()
		a.statsLock.Unlock()

		// 可选：记录到日志或上报到监控系统
		if global.LOG != nil {
			global.LOG.Sugar().Debugf(
				"CircuitBreaker Stats - Total: %d, Failed: %d, Success: %d, Blocked: %d, Open: %d, HalfOpen: %d",
				a.stats.TotalRequests,
				a.stats.FailedRequests,
				a.stats.SuccessRequests,
				a.stats.BlockedRequests,
				a.stats.OpenBreakers,
				a.stats.HalfOpenBreakers,
			)
		}
	}
}

// 辅助方法

func (a *BreakerMiddlewareAdapter) incrementTotalRequests() {
	a.statsLock.Lock()
	defer a.statsLock.Unlock()
	a.stats.TotalRequests++
}

func (a *BreakerMiddlewareAdapter) incrementFailedRequests() {
	a.statsLock.Lock()
	defer a.statsLock.Unlock()
	a.stats.FailedRequests++
}

func (a *BreakerMiddlewareAdapter) incrementSuccessRequests() {
	a.statsLock.Lock()
	defer a.statsLock.Unlock()
	a.stats.SuccessRequests++
}

func (a *BreakerMiddlewareAdapter) incrementBlockedRequests() {
	a.statsLock.Lock()
	defer a.statsLock.Unlock()
	a.stats.BlockedRequests++
}

func (a *BreakerMiddlewareAdapter) updateAverageLatency(latency time.Duration) {
	a.statsLock.Lock()
	defer a.statsLock.Unlock()

	if a.stats.TotalRequests == 0 {
		a.stats.AverageLatency = float64(latency.Milliseconds())
	} else {
		// 计算加权平均延迟
		a.stats.AverageLatency = (a.stats.AverageLatency*float64(a.stats.TotalRequests-1) + float64(latency.Milliseconds())) / float64(a.stats.TotalRequests)
	}
}

// GetBreakerStats 获取所有断路器详细统计
// 返回管理器聚合后的统计信息
func (a *BreakerMiddlewareAdapter) GetBreakerStats() map[string]interface{} {
	allStats := make(map[string]interface{})
	managerStats := a.manager.GetStats()
	// managerStats 是 map[string]map[string]interface{}
	for pathKey, pathData := range managerStats {
		// pathData 已经是 map[string]interface{}，直接赋值
		allStats[pathKey] = pathData
	}
	return allStats
}

// GetHealthStatus 获取整体健康状态
func (a *BreakerMiddlewareAdapter) GetHealthStatus() string {
	statusMap := a.manager.GetHealthStatus()
	if statusMap != nil {
		if status, ok := statusMap["status"].(string); ok {
			return status
		}
		// 尝试转换其他可能的格式
		return fmt.Sprintf("%v", statusMap)
	}
	return "unknown"
}
