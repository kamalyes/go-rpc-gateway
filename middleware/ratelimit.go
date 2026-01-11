/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-01-11 13:55:32
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
	"sync"
	"sync/atomic"
	"time"

	"github.com/kamalyes/go-config/pkg/ratelimit"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/response"
	"github.com/kamalyes/go-toolbox/pkg/httpx"
	"github.com/kamalyes/go-toolbox/pkg/matcher"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/netx"
	"github.com/kamalyes/go-toolbox/pkg/validator"
)

// 限流相关常量
const (
	// 默认key前缀
	defaultKeyPrefix = "ratelimit"

	// 默认清理间隔
	defaultCleanInterval = 5 * time.Minute

	// Key格式模板
	keyFormatTokenBucket   = "%s:rps_%d:burst_%d"  // 令牌桶key格式
	keyFormatSlidingWindow = "%s:%s:win_%v:rps_%d" // 滑动窗口key格式
	keyFormatFixedWindow   = "%s:win_%v:rps_%d"    // 固定窗口key格式
	keyFormatResetPattern  = "%s:%s:*"             // 重置key模式
	keyFormatBlacklist     = "blacklist:%s"        // 黑名单key格式
	keyFormatRouteUser     = "route:%s:user:%s"    // 路由+用户key格式
	keyFormatRouteIP       = "route:%s:ip:%s"      // 路由+IPkey格式
	keyFormatRoute         = "route:%s"            // 路由key格式
	keyFormatIP            = "ip:%s"               // IPkey格式
	keyFormatUser          = "user:%s"             // 用户key格式
	keyFormatRouteMethod   = "route:%s:%s"         // 路由+方法key格式

	// 特殊key值
	keyGlobal     = "global"    // 全局限流key
	keyWildcard   = "*"         // 通配符
	typeWhitelist = "whitelist" // 白名单类型

	// 精度常量
	billion = 1e9 // 十亿(纳秒精度)
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

	// 如果仍然没有规则，直接放行
	if rule == nil {
		return true, nil
	}

	// 生成包含规则参数的唯一key，确保不同规则使用不同的桶
	bucketKey := fmt.Sprintf(keyFormatTokenBucket, key, rule.RequestsPerSecond, rule.BurstSize)

	bucketInterface, loaded := t.limiters.LoadOrStore(bucketKey, &atomicTokenBucket{
		tokensInt64:    int64(rule.BurstSize) * billion,
		maxTokens:      int64(rule.BurstSize),
		refillRate:     int64(rule.RequestsPerSecond) * billion,
		lastRefillNano: time.Now().UnixNano(),
	})

	if !loaded {
		global.LOGGER.DebugContext(ctx, "[TokenBucket] 创建新桶: key=%s, BurstSize=%d, RPS=%d", bucketKey, rule.BurstSize, rule.RequestsPerSecond)
	}

	bucket := bucketInterface.(*atomicTokenBucket)

	now := time.Now().UnixNano()

	for {
		// 原子读取当前状态
		oldTokens := atomic.LoadInt64(&bucket.tokensInt64)
		oldLastRefill := atomic.LoadInt64(&bucket.lastRefillNano)

		// 计算应该补充的令牌(防止时钟回拨) - AtMost实际是max函数
		elapsed := mathx.AtMost(0, now-oldLastRefill)

		// 计算新令牌数（整数运算,先除后乘避免溢出）
		elapsedSeconds := elapsed / billion
		remainderNanos := elapsed % billion
		addTokens := elapsedSeconds*bucket.refillRate + (remainderNanos*bucket.refillRate)/billion

		// 计算新令牌数: min(maxTokens*billion, oldTokens+addTokens), 然后 max(0, result)
		// 注意: mathx.AtLeast实际是min, mathx.AtMost实际是max
		tokensAfterRefill := oldTokens + addTokens
		maxTokensInt64 := bucket.maxTokens * billion
		// 先用 AtLeast(min) 限制上限，再用 AtMost(max) 限制下限
		newTokens := mathx.AtMost(0, mathx.AtLeast(maxTokensInt64, tokensAfterRefill))

		// 检查是否有足够令牌
		if newTokens < billion {
			// 令牌不足，但需要更新lastRefillNano确保时间同步
			atomic.StoreInt64(&bucket.tokensInt64, newTokens)
			atomic.StoreInt64(&bucket.lastRefillNano, now)
			global.LOGGER.DebugContext(ctx, "[TokenBucket] 令牌不足: key=%s, newTokens=%d (需要 %d)", bucketKey, newTokens/billion, 1)
			return false, nil // 令牌不足
		}

		// CAS更新令牌数和时间戳
		if atomic.CompareAndSwapInt64(&bucket.tokensInt64, oldTokens, newTokens-billion) {
			atomic.StoreInt64(&bucket.lastRefillNano, now)
			global.LOGGER.DebugContext(ctx, "[TokenBucket] 允许请求: key=%s, 剩余令牌=%d", bucketKey, (newTokens-billion)/billion)
			return true, nil
		}
		// CAS失败，重试
	}
}

// Reset 重置限流器（删除指定key的所有限流桶）
func (t *TokenBucketLimiter) Reset(ctx context.Context, key string) error {
	// 遍历删除所有匹配key前缀的桶
	t.limiters.Range(func(k, v interface{}) bool {
		bucketKey := k.(string)
		// 如果桶的key以指定key开头，则删除
		if len(bucketKey) >= len(key) && bucketKey[:len(key)] == key {
			t.limiters.Delete(k)
		}
		return true
	})
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

// Allow 检查是否允许请求（使用Lua脚本保证原子性）
func (s *SlidingWindowLimiter) Allow(ctx context.Context, key string, rule *ratelimit.LimitRule) (bool, error) {
	if global.REDIS == nil {
		return false, fmt.Errorf("redis not available for sliding window limiter")
	}
	// 使用mathx.IfNotEmpty设置key前缀默认值
	keyPrefix := mathx.IfNotEmpty(s.config.Storage.KeyPrefix, defaultKeyPrefix)
	// 生成包含规则参数的唯一key
	fullKey := fmt.Sprintf(keyFormatSlidingWindow, keyPrefix, key, rule.WindowSize, rule.RequestsPerSecond)
	now := time.Now()
	windowStart := now.Add(-rule.WindowSize)

	// 使用分布式锁 + Lua脚本保证100%准确性：
	// 关键：用分布式锁串行化所有并发请求，确保检查和添加之间不会有其他请求插入
	script := `
		local key = KEYS[1]
		local counter_key = KEYS[2]
		local lock_key = KEYS[3]
		local now = tonumber(ARGV[1])
		local window_start = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		local window_size = tonumber(ARGV[4])
		local lock_value = ARGV[5]
		
		-- 1. 尝试获取分布式锁（NX表示不存在才设置，PX表示毫秒过期时间）
		local lock_result = redis.call('SET', lock_key, lock_value, 'NX', 'PX', 1000)
		if not lock_result then
			-- 获取锁失败，返回-1表示需要重试
			return -1
		end
		
		-- 2. 清理过期数据（窗口之前的数据）
		redis.call('ZREMRANGEBYSCORE', key, '-inf', tostring(window_start))
		
		-- 3. 统计窗口内的有效请求数
		local count = redis.call('ZCOUNT', key, tostring(window_start), '+inf')
		
		-- 4. 如果已达到限制，释放锁并拒绝
		if count >= limit then
			redis.call('DEL', lock_key)
			return 0
		end
		
		-- 5. 生成唯一member并添加
		local unique_id = redis.call('INCR', counter_key)
		local member = string.format('%d:%d', now, unique_id)
		redis.call('ZADD', key, now, member)
		
		-- 6. 设置过期时间
		redis.call('EXPIRE', key, window_size * 2)
		redis.call('EXPIRE', counter_key, window_size * 2)
		
		-- 7. 释放锁
		redis.call('DEL', lock_key)
		
		return 1
	`

	// 生成锁的唯一值
	lockKey := fullKey + ":lock"
	lockValue := fmt.Sprintf("%d", time.Now().UnixNano())
	counterKey := fullKey + ":counter"

	// 重试机制：如果获取锁失败，短暂等待后重试（最多3次）
	maxRetries := 3
	for retry := 0; retry < maxRetries; retry++ {
		result, err := global.REDIS.Eval(ctx, script, []string{fullKey, counterKey, lockKey},
			now.UnixNano(),
			windowStart.UnixNano(),
			rule.RequestsPerSecond,
			int64(rule.WindowSize.Seconds()),
			lockValue,
		).Result()

		if err != nil {
			return false, fmt.Errorf("failed to execute lua script: %w", err)
		}

		resultInt, ok := result.(int64)
		if !ok {
			return false, fmt.Errorf("unexpected result type: %T", result)
		}

		// -1 表示获取锁失败，需要重试
		if resultInt == -1 {
			if retry < maxRetries-1 {
				time.Sleep(time.Millisecond * time.Duration(10*(retry+1))) // 指数退避
				continue
			}
			// 重试失败，拒绝请求
			return false, nil
		}

		// 0=拒绝, 1=允许
		return resultInt == 1, nil
	}

	return false, nil
}

// Reset 重置限流器（使用Lua脚本分批删除，避免阻塞）
func (s *SlidingWindowLimiter) Reset(ctx context.Context, key string) error {
	if global.REDIS == nil {
		return nil
	}
	// 使用mathx.IfNotEmpty设置key前缀默认值
	keyPrefix := mathx.IfNotEmpty(s.config.Storage.KeyPrefix, defaultKeyPrefix)
	pattern := fmt.Sprintf(keyFormatResetPattern, keyPrefix, key)

	// 使用Lua脚本:SCAN+DEL，避免KEYS阻塞，每批最多100个
	script := `
		local cursor = "0"
		local deleted = 0
		repeat
			local result = redis.call('SCAN', cursor, 'MATCH', ARGV[1], 'COUNT', 100)
			cursor = result[1]
			local keys = result[2]
			if #keys > 0 then
				for i=1,#keys,100 do
					local batch = {}
					for j=i,math.min(i+99, #keys) do
						table.insert(batch, keys[j])
					end
					redis.call('DEL', unpack(batch))
					deleted = deleted + #batch
				end
			end
		until cursor == "0"
		return deleted
	`
	return global.REDIS.Eval(ctx, script, []string{}, pattern).Err()
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
	// 生成包含规则参数的唯一key
	counterKey := fmt.Sprintf(keyFormatFixedWindow, key, rule.WindowSize, rule.RequestsPerSecond)

	now := time.Now()
	resetTime := now.Add(rule.WindowSize)

	counterInterface, _ := f.counters.LoadOrStore(counterKey, &atomicCounter{
		count:         0,
		resetTimeNano: resetTime.UnixNano(),
	})

	counter := counterInterface.(*atomicCounter)

	// 原子读取重置时间
	resetTimeNano := atomic.LoadInt64(&counter.resetTimeNano)

	// 检查是否需要重置
	if now.UnixNano() > resetTimeNano {
		// 尝试重置（CAS保证只有一个goroutine重置）
		newResetTime := now.Add(rule.WindowSize).UnixNano()
		if atomic.CompareAndSwapInt64(&counter.resetTimeNano, resetTimeNano, newResetTime) {
			// 重置计数器为 1（包含当前请求）
			atomic.StoreInt64(&counter.count, 1)
			return true, nil // 重置后第一个请求必然通过
		}
		// CAS 失败说明其他 goroutine 已经重置，重新读取后继续
	}

	// 原子递增计数
	newCount := atomic.AddInt64(&counter.count, 1)

	return newCount <= int64(rule.RequestsPerSecond), nil
}

// Reset 重置限流计数器
func (f *FixedWindowLimiter) Reset(ctx context.Context, key string) error {
	// 遍历删除所有匹配key前缀的计数器
	f.counters.Range(func(k, v interface{}) bool {
		counterKey := k.(string)
		// 如果计数器的key以指定key开头，则删除
		if len(counterKey) >= len(key) && counterKey[:len(key)] == key {
			f.counters.Delete(k)
		}
		return true
	})
	return nil
}

// cleanup 清理过期的计数器
func (f *FixedWindowLimiter) cleanup() {
	// 使用mathx.IfNotZero设置清理间隔默认值
	cleanInterval := mathx.IfNotZero(f.config.Storage.CleanInterval, defaultCleanInterval)

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
}

// NewEnhancedRateLimitMiddleware 创建增强限流中间件
func NewEnhancedRateLimitMiddleware(config *ratelimit.RateLimit) *EnhancedRateLimitMiddleware {
	config = mathx.IF(config == nil, ratelimit.Default(), config)

	var limiter RateLimiter

	// 根据策略选择限流器
	switch config.Strategy {
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

			// 获取限流规则和key(内部已处理白名单)
			rule, key := e.getRuleAndKey(r)
			if rule == nil {
				// nil表示白名单或无需限流
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

// getRuleAndKey 获取限流规则和key(统一处理白名单/黑名单/限流规则)
// 优先级: 白名单 > 黑名单 > 限流规则
func (e *EnhancedRateLimitMiddleware) getRuleAndKey(r *http.Request) (*ratelimit.LimitRule, string) {
	clientIP := netx.GetClientIP(r)
	path := r.URL.Path
	method := r.Method

	global.LOGGER.InfoContext(r.Context(), "[DEBUG] getRuleAndKey: IP=%s, Path=%s, Method=%s", clientIP, path, method)

	// 第一轮: 优先检查白名单和黑名单(最高优先级)
	for i, routeLimit := range e.config.Routes {
		global.LOGGER.InfoContext(r.Context(), "[DEBUG] 检查路由[%d]: Path=%s, Methods=%v", i, routeLimit.Path, routeLimit.Methods)

		// 路径和方法匹配
		pathMatch := matcher.MatchPathWithMethod(path, method, routeLimit.Path, routeLimit.Methods)
		global.LOGGER.InfoContext(r.Context(), "[DEBUG] MatchPathWithMethod结果: %v", pathMatch)

		if !pathMatch {
			global.LOGGER.InfoContext(r.Context(), "[DEBUG] 路由[%d]不匹配,continue", i)
			continue
		}

		global.LOGGER.InfoContext(r.Context(), "[DEBUG] 路由[%d]匹配成功!", i)

		// 1. 白名单 - 最高优先级,直接放行(仅当白名单非空时检查)
		if len(routeLimit.Whitelist) > 0 && validator.IsIPAllowed(clientIP, routeLimit.Whitelist) {
			global.LOGGER.InfoContext(r.Context(), "[DEBUG] IP在白名单,返回nil放行")
			return nil, ""
		}

		// 2. 黑名单 - 第二优先级,严格限流(仅当黑名单非空时检查)
		if len(routeLimit.Blacklist) > 0 && validator.IsIPBlocked(clientIP, routeLimit.Blacklist) {
			global.LOGGER.InfoContext(r.Context(), "[DEBUG] IP在黑名单,返回严格限流规则")
			return &ratelimit.LimitRule{
				RequestsPerSecond: 1,
				BurstSize:         1,
				WindowSize:        time.Minute,
				BlockDuration:     time.Hour,
			}, fmt.Sprintf(keyFormatBlacklist, clientIP)
		}

		// 3. 应用路由限流规则
		if routeLimit.Limit != nil {
			if routeLimit.PerUser {
				userID := httpx.GetUserID(r, constants.ContextKeyUserID, constants.HeaderXUserID)
				return routeLimit.Limit, fmt.Sprintf(keyFormatRouteUser, routeLimit.Path, userID)
			}
			if routeLimit.PerIP {
				return routeLimit.Limit, fmt.Sprintf(keyFormatRouteIP, routeLimit.Path, clientIP)
			}
			return routeLimit.Limit, fmt.Sprintf(keyFormatRoute, routeLimit.Path)
		}

		// 路由匹配但无限流规则,放行
		return nil, ""
	}

	// 第二轮: 检查IP级别规则
	for _, ipRule := range e.config.IPRules {
		if !validator.MatchIPPattern(clientIP, ipRule.IP) {
			continue
		}

		// IP白名单 - 直接放行
		if ipRule.Type == typeWhitelist {
			return nil, ""
		}

		// 应用IP限流规则
		if ipRule.Limit != nil {
			return ipRule.Limit, fmt.Sprintf(keyFormatIP, clientIP)
		}
	}

	// 第三轮: 检查用户级别规则
	userID := httpx.GetUserID(r, constants.ContextKeyUserID, constants.HeaderXUserID)
	if userID != "" {
		for _, userRule := range e.config.UserRules {
			if e.matchUser(userRule, userID) {
				return userRule.Limit, fmt.Sprintf(keyFormatUser, userID)
			}
		}
	}

	// 第四轮: 使用全局限流规则
	if e.config.GlobalLimit != nil {
		key := e.generateKey(r, e.config.DefaultScope)
		return e.config.GlobalLimit, key
	}

	// 无任何限流规则,放行
	return nil, ""
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

// generateKey 生成限流key
func (e *EnhancedRateLimitMiddleware) generateKey(r *http.Request, scope ratelimit.Scope) string {
	switch scope {
	case ratelimit.ScopeGlobal:
		return keyGlobal
	case ratelimit.ScopePerIP:
		return fmt.Sprintf(keyFormatIP, netx.GetClientIP(r))
	case ratelimit.ScopePerUser:
		return fmt.Sprintf(keyFormatUser, httpx.GetUserID(r, constants.ContextKeyUserID, constants.HeaderXUserID))
	case ratelimit.ScopePerRoute:
		return fmt.Sprintf(keyFormatRouteMethod, r.Method, r.URL.Path)
	default:
		return keyGlobal
	}
}

// matchUser 匹配用户（使用通配符匹配）
func (e *EnhancedRateLimitMiddleware) matchUser(rule ratelimit.UserRule, userID string) bool {
	// 空或通配符，匹配所有
	if rule.UserID == "" || rule.UserID == keyWildcard {
		return true
	}
	// 使用 filepath.Match 进行通配符匹配
	matched, _ := filepath.Match(rule.UserID, userID)
	return matched
}

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(config *ratelimit.RateLimit) HTTPMiddleware {
	return NewEnhancedRateLimitMiddleware(config).Middleware()
}
