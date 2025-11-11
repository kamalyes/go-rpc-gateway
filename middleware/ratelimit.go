/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 01:11:55
 * @FilePath: \go-rpc-gateway\middleware\ratelimit.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kamalyes/go-core/pkg/global"
	"github.com/kamalyes/go-rpc-gateway/constants"
)

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	RequestsPerSecond int           `json:"requestsPerSecond" yaml:"requestsPerSecond"` // 每秒允许的请求数
	BurstSize         int           `json:"burstSize" yaml:"burstSize"`                 // 突发请求数
	CleanupInterval   time.Duration `json:"cleanupInterval" yaml:"cleanupInterval"`     // 清理间隔
	WindowSize        time.Duration `json:"windowSize" yaml:"windowSize"`               // 时间窗口大小
	KeyPrefix         string        `json:"keyPrefix" yaml:"keyPrefix"`                 // Redis key 前缀
	Enabled           bool          `json:"enabled" yaml:"enabled"`                     // 是否启用
}

// DefaultRateLimitConfig 默认限流配置
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		RequestsPerSecond: 100,
		BurstSize:         200,
		CleanupInterval:   time.Minute,
		WindowSize:        time.Minute,
		KeyPrefix:         "rate_limit",
		Enabled:           true,
	}
}

// RateLimiter 限流器接口
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
	Reset(ctx context.Context, key string) error
}

// RedisRateLimiter Redis 限流器
type RedisRateLimiter struct {
	config *RateLimitConfig
	mu     sync.RWMutex
}

// NewRedisRateLimiter 创建 Redis 限流器
func NewRedisRateLimiter(config *RateLimitConfig) *RedisRateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	return &RedisRateLimiter{
		config: config,
	}
}

// Allow 检查是否允许请求
func (r *RedisRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	if !r.config.Enabled {
		return true, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	fullKey := fmt.Sprintf("%s:%s", r.config.KeyPrefix, key)

	// 获取当前计数
	count, err := global.REDIS.Incr(ctx, fullKey).Result()
	if err != nil {
		return false, fmt.Errorf("failed to increment counter: %w", err)
	}

	// 首次访问，设置过期时间
	if count == 1 {
		err = global.REDIS.Expire(ctx, fullKey, r.config.WindowSize).Err()
		if err != nil {
			return false, fmt.Errorf("failed to set expiration: %w", err)
		}
	}

	// 检查是否超过限制
	return count <= int64(r.config.RequestsPerSecond), nil
}

// Reset 重置限流计数器
func (r *RedisRateLimiter) Reset(ctx context.Context, key string) error {
	if global.REDIS == nil {
		return nil
	}

	fullKey := fmt.Sprintf("%s:%s", r.config.KeyPrefix, key)
	return global.REDIS.Del(ctx, fullKey).Err()
}

// MemoryRateLimiter 内存限流器
type MemoryRateLimiter struct {
	config   *RateLimitConfig
	counters map[string]*rateLimitCounter
	mu       sync.RWMutex
}

type rateLimitCounter struct {
	count     int64
	resetTime time.Time
	mu        sync.Mutex
}

// NewMemoryRateLimiter 创建内存限流器
func NewMemoryRateLimiter(config *RateLimitConfig) *MemoryRateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	limiter := &MemoryRateLimiter{
		config:   config,
		counters: make(map[string]*rateLimitCounter),
	}

	// 启动清理协程
	go limiter.cleanup()

	return limiter
}

// Allow 检查是否允许请求
func (m *MemoryRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	if !m.config.Enabled {
		return true, nil
	}

	m.mu.Lock()
	counter, exists := m.counters[key]
	if !exists {
		counter = &rateLimitCounter{
			count:     0,
			resetTime: time.Now().Add(m.config.WindowSize),
		}
		m.counters[key] = counter
	}
	m.mu.Unlock()

	counter.mu.Lock()
	defer counter.mu.Unlock()

	now := time.Now()
	if now.After(counter.resetTime) {
		counter.count = 0
		counter.resetTime = now.Add(m.config.WindowSize)
	}

	counter.count++
	return counter.count <= int64(m.config.RequestsPerSecond), nil
}

// Reset 重置限流计数器
func (m *MemoryRateLimiter) Reset(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.counters, key)
	return nil
}

// cleanup 清理过期的计数器
func (m *MemoryRateLimiter) cleanup() {
	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		m.mu.Lock()
		for key, counter := range m.counters {
			counter.mu.Lock()
			if now.After(counter.resetTime) {
				delete(m.counters, key)
			}
			counter.mu.Unlock()
		}
		m.mu.Unlock()
	}
}

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(limiter RateLimiter) HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 生成限流 key（基于 IP + Method + Path）
			key := fmt.Sprintf("%s:%s:%s", getClientIP(r), r.Method, r.URL.Path)

			// 检查是否允许请求
			allowed, err := limiter.Allow(r.Context(), key)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			if !allowed {
				w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error": "Too Many Requests", "message": "Rate limit exceeded"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitMiddlewareWithConfig 带配置的限流中间件
func RateLimitMiddlewareWithConfig(config *RateLimitConfig) HTTPMiddleware {
	var limiter RateLimiter

	// 如果全局Redis可用，使用Redis限流器，否则使用内存限流器
	if global.REDIS != nil {
		limiter = NewRedisRateLimiter(config)
	} else {
		limiter = NewMemoryRateLimiter(config)
	}

	return RateLimitMiddleware(limiter)
}

// getClientIP 获取客户端真实IP
func getClientIP(r *http.Request) string {
	// 检查X-Forwarded-For头
	if xff := r.Header.Get(constants.HeaderXForwardedFor); xff != "" {
		// 取第一个IP
		if ips := strings.Split(xff, ","); len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// 检查X-Real-IP头
	if xri := r.Header.Get(constants.HeaderXRealIP); xri != "" {
		return xri
	}

	// 使用RemoteAddr
	if ip := strings.Split(r.RemoteAddr, ":"); len(ip) > 0 {
		return ip[0]
	}

	return r.RemoteAddr
}
