/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 16:25:52
 * @FilePath: \go-rpc-gateway\middleware\ratelimit.go
 * @Description: 高性能限流中间件，支持多种策略和多级别限流（使用atomic保证原子性）
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kamalyes/go-config/pkg/ratelimit"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/response"
	"github.com/kamalyes/go-toolbox/pkg/netx"
	"github.com/kamalyes/go-toolbox/pkg/safe"
	"github.com/redis/go-redis/v9"
)

// RateLimiter 限流器接口
type RateLimiter interface {
	Allow(ctx context.Context, key string, rule *ratelimit.LimitRule) (bool, error)
	Reset(ctx context.Context, key string) error
}

// TokenBucketLimiter 令牌桶限流器（使用atomic保证高性能）
type TokenBucketLimiter struct {
	limiters   sync.Map // key: string, value: *atomicTokenBucket
	globalRule *ratelimit.LimitRule
}

// atomicTokenBucket 原子令牌桶（无锁实现）
type atomicTokenBucket struct {
	tokensInt64    int64 // tokens * 1e9 (使用整数存储浮点数，避免atomic.Value)
	maxTokens      int64
	refillRate     int64 // 每秒添加的令牌数 * 1e9
	lastRefillNano int64 // 上次补充时间（纳秒时间戳）
}

// NewTokenBucketLimiter 创建令牌桶限流器
func NewTokenBucketLimiter(cfg *ratelimit.RateLimit) *TokenBucketLimiter {
	var globalRule *ratelimit.LimitRule
	if cfg != nil && cfg.GlobalLimit != nil {
		globalRule = cfg.GlobalLimit
	}
	return &TokenBucketLimiter{
		globalRule: globalRule,
	}
}

// Allow 检查是否允许请求（无锁原子操作）
func (t *TokenBucketLimiter) Allow(ctx context.Context, key string, rule *ratelimit.LimitRule) (bool, error) {
	// 如果没有提供规则，使用全局配置
	if rule == nil {
		rule = t.globalRule
	}

	// 使用safe包裹配置读取
	safeRule := safe.Safe(rule)
	rps := safeRule.Field("RequestsPerSecond").Int(100)
	burstSize := safeRule.Field("BurstSize").Int(200)

	bucketInterface, _ := t.limiters.LoadOrStore(key, &atomicTokenBucket{
		tokensInt64:    int64(burstSize) * 1e9,
		maxTokens:      int64(burstSize),
		refillRate:     int64(rps) * 1e9,
		lastRefillNano: time.Now().UnixNano(),
	})

	bucket := bucketInterface.(*atomicTokenBucket)

	const billion = 1e9
	now := time.Now().UnixNano()

	for {
		// 原子读取当前状态
		oldTokens := atomic.LoadInt64(&bucket.tokensInt64)
		oldLastRefill := atomic.LoadInt64(&bucket.lastRefillNano)

		// 计算应该补充的令牌
		elapsed := now - oldLastRefill
		if elapsed < 0 {
			elapsed = 0 // 防止时钟回拨
		}

		// 计算新令牌数（整数运算）
		addTokens := (elapsed * bucket.refillRate) / billion
		newTokens := oldTokens + addTokens
		if newTokens > bucket.maxTokens*billion {
			newTokens = bucket.maxTokens * billion
		}

		// 检查是否有足够令牌
		if newTokens < billion {
			return false, nil // 令牌不足
		}

		// CAS更新令牌数和时间戳
		if atomic.CompareAndSwapInt64(&bucket.tokensInt64, oldTokens, newTokens-billion) {
			atomic.StoreInt64(&bucket.lastRefillNano, now)
			return true, nil
		}
		// CAS失败，重试
	}
}

// Reset 重置限流器
func (t *TokenBucketLimiter) Reset(ctx context.Context, key string) error {
	t.limiters.Delete(key)
	return nil
}

// SlidingWindowLimiter 滑动窗口限流器（Redis实现）
type SlidingWindowLimiter struct {
	config *ratelimit.RateLimit
}

// NewSlidingWindowLimiter 创建滑动窗口限流器
func NewSlidingWindowLimiter(config *ratelimit.RateLimit) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		config: config,
	}
}

// Allow 检查是否允许请求
func (s *SlidingWindowLimiter) Allow(ctx context.Context, key string, rule *ratelimit.LimitRule) (bool, error) {
	if global.REDIS == nil {
		return true, fmt.Errorf("redis not available for sliding window limiter")
	}

	// 使用safe包裹配置读取
	safeConfig := safe.Safe(s.config)
	keyPrefix := safeConfig.Field("Storage").Field("KeyPrefix").String("rate_limit:")

	safeRule := safe.Safe(rule)
	windowSize := safeRule.Field("WindowSize").Duration(time.Minute)
	rps := safeRule.Field("RequestsPerSecond").Int(100)

	fullKey := fmt.Sprintf("%s:%s", keyPrefix, key)
	now := time.Now()
	windowStart := now.Add(-windowSize)

	// 使用 Pipeline 批量操作减少网络往返
	pipe := global.REDIS.Pipeline()

	// 删除窗口外的请求
	pipe.ZRemRangeByScore(ctx, fullKey, "0", fmt.Sprintf("%d", windowStart.UnixNano()))

	// 添加当前请求
	pipe.ZAdd(ctx, fullKey, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: now.UnixNano(),
	})

	// 统计窗口内的请求数
	pipe.ZCard(ctx, fullKey)

	// 设置过期时间
	pipe.Expire(ctx, fullKey, windowSize)

	// 执行管道
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to execute pipeline: %w", err)
	}

	// 获取计数结果（第3个命令是ZCard）
	count := cmds[2].(*redis.IntCmd).Val()
	return count <= int64(rps), nil
}

// Reset 重置限流器
func (s *SlidingWindowLimiter) Reset(ctx context.Context, key string) error {
	if global.REDIS == nil {
		return nil
	}
	safeConfig := safe.Safe(s.config)
	keyPrefix := safeConfig.Field("Storage").Field("KeyPrefix").String("rate_limit:")
	fullKey := fmt.Sprintf("%s:%s", keyPrefix, key)
	return global.REDIS.Del(ctx, fullKey).Err()
}

// FixedWindowLimiter 固定窗口限流器（使用atomic保证高性能）
type FixedWindowLimiter struct {
	config   *ratelimit.RateLimit
	counters sync.Map // key: string, value: *atomicCounter
	stopChan chan struct{}
	once     sync.Once
}

// atomicCounter 原子计数器
type atomicCounter struct {
	count         int64 // 原子计数
	resetTimeNano int64 // 重置时间（纳秒时间戳）
}

// NewFixedWindowLimiter 创建固定窗口限流器
func NewFixedWindowLimiter(config *ratelimit.RateLimit) *FixedWindowLimiter {
	limiter := &FixedWindowLimiter{
		config:   config,
		stopChan: make(chan struct{}),
	}

	// 启动清理协程
	go limiter.cleanup()

	return limiter
}

// Allow 检查是否允许请求（使用atomic）
func (f *FixedWindowLimiter) Allow(ctx context.Context, key string, rule *ratelimit.LimitRule) (bool, error) {
	// 使用safe包裹配置读取
	safeRule := safe.Safe(rule)
	windowSize := safeRule.Field("WindowSize").Duration(time.Minute)
	rps := safeRule.Field("RequestsPerSecond").Int(100)

	now := time.Now()
	resetTime := now.Add(windowSize)

	counterInterface, _ := f.counters.LoadOrStore(key, &atomicCounter{
		count:         0,
		resetTimeNano: resetTime.UnixNano(),
	})

	counter := counterInterface.(*atomicCounter)

	// 原子读取重置时间
	resetTimeNano := atomic.LoadInt64(&counter.resetTimeNano)

	// 检查是否需要重置
	if now.UnixNano() > resetTimeNano {
		// 尝试重置（CAS保证只有一个goroutine重置）
		newResetTime := now.Add(windowSize).UnixNano()
		if atomic.CompareAndSwapInt64(&counter.resetTimeNano, resetTimeNano, newResetTime) {
			atomic.StoreInt64(&counter.count, 0)
		}
	}

	// 原子递增计数
	newCount := atomic.AddInt64(&counter.count, 1)

	return newCount <= int64(rps), nil
}

// Reset 重置限流计数器
func (f *FixedWindowLimiter) Reset(ctx context.Context, key string) error {
	f.counters.Delete(key)
	return nil
}

// cleanup 清理过期的计数器
func (f *FixedWindowLimiter) cleanup() {
	safeConfig := safe.Safe(f.config)
	cleanInterval := safeConfig.Field("Storage").Field("CleanInterval").Duration(5 * time.Minute)

	ticker := time.NewTicker(cleanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now().UnixNano()
			f.counters.Range(func(key, value interface{}) bool {
				counter := value.(*atomicCounter)
				resetTimeNano := atomic.LoadInt64(&counter.resetTimeNano)
				if now > resetTimeNano+int64(cleanInterval) {
					f.counters.Delete(key)
				}
				return true
			})
		case <-f.stopChan:
			return
		}
	}
}

// Stop 停止清理协程
func (f *FixedWindowLimiter) Stop() {
	f.once.Do(func() {
		close(f.stopChan)
	})
}

// EnhancedRateLimitMiddleware 增强的限流中间件
type EnhancedRateLimitMiddleware struct {
	config  *ratelimit.RateLimit
	limiter RateLimiter
	mu      sync.RWMutex
}

// NewEnhancedRateLimitMiddleware 创建增强限流中间件
func NewEnhancedRateLimitMiddleware(config *ratelimit.RateLimit) *EnhancedRateLimitMiddleware {
	if config == nil {
		config = ratelimit.Default()
	}

	var limiter RateLimiter

	// 根据策略选择限流器
	switch config.Strategy {
	case ratelimit.StrategyTokenBucket:
		limiter = NewTokenBucketLimiter(config)
	case ratelimit.StrategySlidingWindow:
		if global.REDIS != nil {
			limiter = NewSlidingWindowLimiter(config)
		} else {
			limiter = NewTokenBucketLimiter(config) // 降级到令牌桶
		}
	case ratelimit.StrategyFixedWindow:
		limiter = NewFixedWindowLimiter(config)
	default:
		limiter = NewTokenBucketLimiter(config)
	}

	return &EnhancedRateLimitMiddleware{
		config:  config,
		limiter: limiter,
	}
}

// Middleware 返回 HTTP 中间件
func (e *EnhancedRateLimitMiddleware) Middleware() HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !e.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			rule, key := e.getRuleAndKey(r, e.config.GlobalLimit, e.config.DefaultScope)
			if rule == nil {
				next.ServeHTTP(w, r)
				return
			}

			if !e.allowRequest(w, r, key, rule) {
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getRuleAndKey 获取限流规则和key
func (e *EnhancedRateLimitMiddleware) getRuleAndKey(r *http.Request, globalLimit *ratelimit.LimitRule, defaultScope ratelimit.Scope) (*ratelimit.LimitRule, string) {
	// 先尝试获取特定规则（路由/IP/用户规则）
	rule, key := e.getApplicableRule(r)
	if rule != nil {
		return rule, key
	}

	// 没有特定规则，使用全局限流规则
	if globalLimit == nil {
		return nil, ""
	}

	// 根据默认作用域生成限流key
	key = e.generateKey(r, defaultScope)
	return globalLimit, key
}

// allowRequest 检查是否允许请求
func (e *EnhancedRateLimitMiddleware) allowRequest(w http.ResponseWriter, r *http.Request, key string, rule *ratelimit.LimitRule) bool {
	allowed, err := e.limiter.Allow(r.Context(), key, rule)
	if err != nil {
		response.WriteErrorResponse(w, errors.ErrInternalServerError.WithDetails(err.Error()))
		return false
	}

	if !allowed {
		response.WriteErrorResponse(w, errors.ErrRateLimitExceeded)
		return false
	}

	return true
}

// getApplicableRule 获取适用的限流规则（使用safe读取配置）
func (e *EnhancedRateLimitMiddleware) getApplicableRule(r *http.Request) (*ratelimit.LimitRule, string) {
	// 1. 检查路由级别规则
	if rule, key := e.checkRouteRules(r); rule != nil || key != "" {
		return rule, key
	}

	// 2. 检查IP规则
	if rule, key := e.checkIPRules(r); rule != nil || key != "" {
		return rule, key
	}

	// 3. 检查用户规则
	if rule, key := e.checkUserRules(r); rule != nil || key != "" {
		return rule, key
	}

	return nil, ""
}

// checkRouteRules 检查路由级别规则
func (e *EnhancedRateLimitMiddleware) checkRouteRules(r *http.Request) (*ratelimit.LimitRule, string) {
	if len(e.config.Routes) == 0 {
		return nil, ""
	}

	clientIP := netx.GetClientIP(r)

	for _, routeLimit := range e.config.Routes {
		if !e.matchRoute(routeLimit.Path, r.URL.Path) || !e.matchMethod(routeLimit.Methods, r.Method) {
			continue
		}

		// 检查白名单
		if e.inList(clientIP, routeLimit.Whitelist) {
			return nil, "" // 白名单放行
		}

		// 检查黑名单
		if e.inList(clientIP, routeLimit.Blacklist) {
			return &ratelimit.LimitRule{
				RequestsPerSecond: 1,
				BurstSize:         1,
				WindowSize:        time.Minute,
				BlockDuration:     time.Hour,
			}, fmt.Sprintf("blacklist:%s", clientIP)
		}

		// 生成路由key
		if routeLimit.PerUser {
			userID := e.getUserID(r)
			return routeLimit.Limit, fmt.Sprintf("route:%s:user:%s", routeLimit.Path, userID)
		}
		if routeLimit.PerIP {
			return routeLimit.Limit, fmt.Sprintf("route:%s:ip:%s", routeLimit.Path, clientIP)
		}
		return routeLimit.Limit, fmt.Sprintf("route:%s", routeLimit.Path)
	}

	return nil, ""
}

// checkIPRules 检查IP规则
func (e *EnhancedRateLimitMiddleware) checkIPRules(r *http.Request) (*ratelimit.LimitRule, string) {
	if len(e.config.IPRules) == 0 {
		return nil, ""
	}

	clientIP := netx.GetClientIP(r)
	for _, ipRule := range e.config.IPRules {
		if !e.matchIP(ipRule.IP, clientIP) {
			continue
		}

		if ipRule.Type == "whitelist" {
			return nil, "" // 白名单放行
		}
		return ipRule.Limit, fmt.Sprintf("ip:%s", clientIP)
	}

	return nil, ""
}

// checkUserRules 检查用户规则
func (e *EnhancedRateLimitMiddleware) checkUserRules(r *http.Request) (*ratelimit.LimitRule, string) {
	userID := e.getUserID(r)
	if userID == "" || len(e.config.UserRules) == 0 {
		return nil, ""
	}

	for _, userRule := range e.config.UserRules {
		if e.matchUser(userRule, userID, r) {
			return userRule.Limit, fmt.Sprintf("user:%s", userID)
		}
	}

	return nil, ""
}

// generateKey 生成限流key
func (e *EnhancedRateLimitMiddleware) generateKey(r *http.Request, scope ratelimit.Scope) string {
	switch e.config.DefaultScope {
	case ratelimit.ScopeGlobal:
		return "global"
	case ratelimit.ScopePerIP:
		return fmt.Sprintf("ip:%s", netx.GetClientIP(r))
	case ratelimit.ScopePerUser:
		return fmt.Sprintf("user:%s", e.getUserID(r))
	case ratelimit.ScopePerRoute:
		return fmt.Sprintf("route:%s:%s", r.Method, r.URL.Path)
	default:
		return "global"
	}
}

// matchRoute 匹配路由
func (e *EnhancedRateLimitMiddleware) matchRoute(pattern, path string) bool {
	matched, _ := filepath.Match(pattern, path)
	return matched || pattern == path
}

// matchMethod 匹配HTTP方法
func (e *EnhancedRateLimitMiddleware) matchMethod(methods []string, method string) bool {
	if len(methods) == 0 {
		return true // 没有指定方法，匹配所有
	}
	for _, m := range methods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

// matchIP 匹配IP（简单实现，支持CIDR需要额外库）
func (e *EnhancedRateLimitMiddleware) matchIP(pattern, ip string) bool {
	// 简单匹配，完整实现需要支持CIDR
	return pattern == ip || pattern == "*"
}

// matchUser 匹配用户
func (e *EnhancedRateLimitMiddleware) matchUser(rule ratelimit.UserRule, userID string, r *http.Request) bool {
	// 匹配用户ID
	if rule.UserID != "" && rule.UserID != "*" {
		matched, _ := filepath.Match(rule.UserID, userID)
		if !matched {
			return false
		}
	}
	return true
}

// inList 检查是否在列表中
func (e *EnhancedRateLimitMiddleware) inList(value string, list []string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// getUserID 获取用户ID（从上下文或请求头）
func (e *EnhancedRateLimitMiddleware) getUserID(r *http.Request) string {
	// 优先从上下文获取
	if userID := r.Context().Value(constants.ContextKeyUserID); userID != nil {
		if uid, ok := userID.(string); ok {
			return uid
		}
	}

	// 从请求头获取
	if userID := r.Header.Get(constants.HeaderXUserID); userID != "" {
		return userID
	}

	return ""
}

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(config *ratelimit.RateLimit) HTTPMiddleware {
	return NewEnhancedRateLimitMiddleware(config).Middleware()
}
