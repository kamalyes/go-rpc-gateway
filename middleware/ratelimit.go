/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 12:54:59
 * @FilePath: \go-rpc-gateway\middleware\ratelimit.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kamalyes/go-core/pkg/global"
	"github.com/kamalyes/go-rpc-gateway/config"
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

// ConfigurableRateLimitMiddleware 可配置的限流中间件
func ConfigurableRateLimitMiddleware(rateLimitConfig *config.RateLimitConfig) HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rateLimitConfig.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// 检查IP白名单
			clientIP := getClientIPFromRequest(r)
			if isIPInWhitelist(clientIP, rateLimitConfig.Whitelist.IPs) {
				next.ServeHTTP(w, r)
				return
			}

			// 检查头部白名单
			if isHeaderInWhitelist(r, rateLimitConfig.Whitelist.Headers) {
				next.ServeHTTP(w, r)
				return
			}

			// 生成限流键
			key := generateRateLimitKey(r, rateLimitConfig)

			// 检查特定路径的限流规则
			rule := findMatchingRule(r.URL.Path, rateLimitConfig.Rules)
			rate := rateLimitConfig.Rate
			burst := rateLimitConfig.Burst

			if rule != nil {
				rate = rule.Rate
				burst = rule.Burst
			}

			// 执行限流检查
			allowed, remaining, resetTime := checkRateLimit(key, rate, burst, rateLimitConfig)

			// 设置限流响应头
			setRateLimitHeaders(w, rateLimitConfig.Headers, rate, remaining, resetTime)

			if !allowed {
				if global.LOGGER != nil {
					global.LOGGER.WarnKV("请求被限流",
						"client_ip", clientIP,
						"path", r.URL.Path,
						"key", key,
						"rate", rate,
						"burst", burst)
				}

				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIPFromRequest 从请求中获取客户端IP
func getClientIPFromRequest(r *http.Request) string {
	// X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// RemoteAddr
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}

	return r.RemoteAddr
}

// isIPInWhitelist 检查IP是否在白名单中
func isIPInWhitelist(clientIP string, whitelist []string) bool {
	for _, ip := range whitelist {
		if ip == "*" || ip == clientIP {
			return true
		}
		// TODO: 支持CIDR格式
	}
	return false
}

// isHeaderInWhitelist 检查请求头是否在白名单中
func isHeaderInWhitelist(r *http.Request, whitelist map[string]string) bool {
	for header, value := range whitelist {
		if r.Header.Get(header) == value {
			return true
		}
	}
	return false
}

// generateRateLimitKey 生成限流键
func generateRateLimitKey(r *http.Request, config *config.RateLimitConfig) string {
	switch config.KeyFunc {
	case "ip":
		return "rate_limit:ip:" + getClientIPFromRequest(r)
	case "user":
		// 从Authorization头或其他地方获取用户信息
		if auth := r.Header.Get("Authorization"); auth != "" {
			return "rate_limit:user:" + auth
		}
		return "rate_limit:ip:" + getClientIPFromRequest(r)
	case "header":
		if config.CustomKeyHeader != "" {
			if value := r.Header.Get(config.CustomKeyHeader); value != "" {
				return "rate_limit:header:" + value
			}
		}
		return "rate_limit:ip:" + getClientIPFromRequest(r)
	default:
		return "rate_limit:ip:" + getClientIPFromRequest(r)
	}
}

// findMatchingRule 查找匹配的限流规则
func findMatchingRule(path string, rules []config.RateLimitRuleConfig) *config.RateLimitRuleConfig {
	for _, rule := range rules {
		if strings.HasPrefix(path, rule.Path) {
			return &rule
		}
	}
	return nil
}

// checkRateLimit 检查限流状态
func checkRateLimit(key string, rate, burst int, config *config.RateLimitConfig) (allowed bool, remaining int, resetTime int64) {
	// 这里是一个简化的内存限流实现
	// 在生产环境中，建议使用Redis等分布式存储

	now := time.Now()
	// windowStart 暂时注释掉，等实际实现时使用
	// windowStart := now.Add(-time.Duration(config.WindowSize) * time.Second)

	// TODO: 实现基于Redis的分布式限流
	// 或使用现有的令牌桶/滑动窗口算法

	// 简化实现：总是允许（需要实际的限流逻辑）
	return true, burst - 1, now.Add(time.Duration(config.WindowSize) * time.Second).Unix()
}

// setRateLimitHeaders 设置限流相关的响应头
func setRateLimitHeaders(w http.ResponseWriter, headers config.RateLimitHeadersConfig, limit, remaining int, resetTime int64) {
	if headers.Limit != "" {
		w.Header().Set(headers.Limit, strconv.Itoa(limit))
	} else {
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
	}

	if headers.Remaining != "" {
		w.Header().Set(headers.Remaining, strconv.Itoa(remaining))
	} else {
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
	}

	if headers.Reset != "" {
		w.Header().Set(headers.Reset, strconv.FormatInt(resetTime, 10))
	} else {
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))
	}
}
