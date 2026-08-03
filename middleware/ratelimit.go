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
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	validator "github.com/kamalyes/go-argus"
	"github.com/kamalyes/go-config/pkg/ratelimit"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/response"
	"github.com/kamalyes/go-toolbox/pkg/matcher"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/netx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 限流相关常量
const (
	// 默认key前缀
	defaultKeyPrefix = "ratelimit"

	// 默认清理间隔
	defaultCleanInterval = 5 * time.Minute

	// Key格式模板
	keyFormatSlidingWindow = "%s:%s:win_%v:rps_%d" // 滑动窗口key格式
	keyFormatFixedWindow   = "%s:win_%v:rps_%d"    // 固定窗口key格式
	keyFormatResetPattern  = "%s:%s:*"             // 重置key模式
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
	bucketKey := key + ":rps_" + strconv.FormatInt(int64(rule.RequestsPerSecond), 10) + ":burst_" + strconv.FormatInt(int64(rule.BurstSize), 10)

	bucketInterface, _ := t.limiters.LoadOrStore(bucketKey, &atomicTokenBucket{
		tokensInt64:    int64(rule.BurstSize) * billion,
		maxTokens:      int64(rule.BurstSize),
		refillRate:     int64(rule.RequestsPerSecond) * billion,
		lastRefillNano: time.Now().UnixNano(),
	})

	bucket := bucketInterface.(*atomicTokenBucket)

	now := time.Now().UnixNano()

	for {
		// 原子读取当前状态
		oldTokens := atomic.LoadInt64(&bucket.tokensInt64)
		oldLastRefill := atomic.LoadInt64(&bucket.lastRefillNano)

		// 计算应该补充的令牌(防止时钟回拨) - AtMost实际是max函数
		elapsed := mathx.AtMost(0, now-oldLastRefill)

		// 限制 elapsed 不超过填满桶所需时间（避免长时间空闲后大数溢出，且超过部分本就会被 maxTokens 截断）
		// 填满时间(纳秒) = maxTokens * billion / rps，其中 rps = refillRate / billion（精确整除）
		if rps := bucket.refillRate / billion; rps > 0 {
			if maxElapsed := bucket.maxTokens * billion / rps; elapsed > maxElapsed {
				elapsed = maxElapsed
			}
		}

		// 计算新令牌数（整数运算）
		// 注意: sub-second 项必须用 remainderNanos*(refillRate/billion) 而非 (remainderNanos*refillRate)/billion
		// 后者的中间值 remainderNanos*refillRate 在 rps>=50、sub-second 间隔时会溢出 int64 产生负数
		// 由于 refillRate = rps*billion，refillRate/billion == rps 精确整除，重排后 remainderNanos*rps 不会溢出
		elapsedSeconds := elapsed / billion
		remainderNanos := elapsed % billion
		addTokens := elapsedSeconds*bucket.refillRate + remainderNanos*(bucket.refillRate/billion)

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

type rateLimiterSet struct {
	config   *ratelimit.RateLimit
	mu       sync.RWMutex
	limiters map[ratelimit.Strategy]RateLimiter
}

func newRateLimiterSet(config *ratelimit.RateLimit, defaultLimiter RateLimiter) *rateLimiterSet {
	config = mathx.IF(config == nil, ratelimit.Default(), config)

	set := &rateLimiterSet{
		config:   config,
		limiters: make(map[ratelimit.Strategy]RateLimiter, 3),
	}

	if defaultLimiter != nil {
		set.limiters[resolveRateLimiterStrategy(config.Strategy)] = defaultLimiter
	}

	return set
}

func (s *rateLimiterSet) get(strategy ratelimit.Strategy) RateLimiter {
	resolved := resolveRateLimiterStrategy(strategy)

	s.mu.RLock()
	limiter := s.limiters[resolved]
	s.mu.RUnlock()
	if limiter != nil {
		return limiter
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if limiter = s.limiters[resolved]; limiter != nil {
		return limiter
	}

	limiter = newRateLimiter(s.config, resolved)
	if limiter != nil {
		s.limiters[resolved] = limiter
	}

	return limiter
}

func resolveRateLimiterStrategy(strategy ratelimit.Strategy) ratelimit.Strategy {
	switch strategy {
	case ratelimit.StrategySlidingWindow:
		if global.REDIS == nil {
			global.LOGGER.Warn("Redis不可用,限流器降级为令牌桶模式")
			return ratelimit.StrategyTokenBucket
		}
		return ratelimit.StrategySlidingWindow
	case ratelimit.StrategyFixedWindow:
		return ratelimit.StrategyFixedWindow
	case ratelimit.StrategyTokenBucket:
		fallthrough
	default:
		return ratelimit.StrategyTokenBucket
	}
}

func newRateLimiter(config *ratelimit.RateLimit, strategy ratelimit.Strategy) RateLimiter {
	config = mathx.IF(config == nil, ratelimit.Default(), config)

	switch resolveRateLimiterStrategy(strategy) {
	case ratelimit.StrategySlidingWindow:
		return NewSlidingWindowLimiter(config)
	case ratelimit.StrategyFixedWindow:
		return NewFixedWindowLimiter(config)
	case ratelimit.StrategyTokenBucket:
		fallthrough
	default:
		return NewTokenBucketLimiter(config)
	}
}

type rateLimitMiddleware struct {
	config          *ratelimit.RateLimit
	limiter         RateLimiter
	limiters        *rateLimiterSet
	dynamicProvider DynamicRateLimitProvider
	metricsManager  *MetricsManager // 监控管理器（用于记录限流拒绝指标）
}

func newRateLimitMiddleware(config *ratelimit.RateLimit, defaultLimiter RateLimiter, provider DynamicRateLimitProvider, mm *MetricsManager) *rateLimitMiddleware {
	config = mathx.IF(config == nil, ratelimit.Default(), config)

	limiters := newRateLimiterSet(config, defaultLimiter)
	limiter := defaultLimiter
	if limiter == nil {
		limiter = limiters.get(config.Strategy)
	}

	return &rateLimitMiddleware{
		config:          config,
		limiter:         limiter,
		limiters:        limiters,
		dynamicProvider: provider,
		metricsManager:  mm,
	}
}

// Middleware 返回 HTTP 中间件
func (e *rateLimitMiddleware) Middleware() HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !e.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			decisions, appErr := e.getDecisions(r)
			if appErr != nil {
				response.WriteAppError(w, appErr)
				return
			}
			if len(decisions) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			if !e.allowRequests(w, r, decisions) {
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// allowRequests 检查是否允许请求
func (e *rateLimitMiddleware) allowRequests(w http.ResponseWriter, r *http.Request, decisions []RateLimitDecision) bool {
	for _, decision := range decisions {
		limiter := e.getLimiter(decision.Strategy)
		if limiter == nil {
			response.WriteAppError(w, errors.NewError(errors.ErrCodeInternalServerError, fmt.Sprintf("unsupported rate limit strategy: %s", decision.Strategy)))
			return false
		}

		allowed, err := limiter.Allow(r.Context(), decision.Key, decision.Rule)
		if err != nil {
			response.WriteAppError(w, errors.NewError(errors.ErrCodeInternalServerError, err.Error()))
			return false
		}

		if !allowed {
			// 记录限流拒绝指标（按策略打标）
			if e.metricsManager != nil {
				e.metricsManager.RecordRateLimitRejected(string(decision.Strategy))
			}
			response.WriteErrorResponse(w, errors.ErrRateLimitExceeded)
			return false
		}
	}

	return true
}

func (e *rateLimitMiddleware) getDecisions(r *http.Request) ([]RateLimitDecision, *errors.AppError) {
	if e.dynamicProvider != nil {
		result, appErr := e.dynamicProvider.ResolveRateLimit(r)
		if appErr != nil {
			return nil, appErr
		}
		if result != nil {
			if result.Skip {
				return nil, nil
			}
			if len(result.Decisions) > 0 {
				return e.normalizeDecisions(r, result.Decisions), nil
			}
		}
	}

	rule, key := e.getRuleAndKey(r)
	if rule == nil {
		return nil, nil
	}

	return []RateLimitDecision{{
		Rule:     rule,
		Key:      key,
		Strategy: e.config.Strategy,
	}}, nil
}

func (e *rateLimitMiddleware) normalizeDecisions(r *http.Request, decisions []RateLimitDecision) []RateLimitDecision {
	normalized := make([]RateLimitDecision, 0, len(decisions))
	for _, decision := range decisions {
		if decision.Rule == nil {
			continue
		}
		if decision.Key == "" {
			decision.Key = e.generateKey(r, e.config.DefaultScope)
		}
		if decision.Strategy == "" {
			decision.Strategy = e.config.Strategy
		}
		normalized = append(normalized, decision)
	}
	return normalized
}

// getRuleAndKey 获取限流规则和key（HTTP 入口）
// 优先级: 白名单 > 黑名单 > 限流规则
func (e *rateLimitMiddleware) getRuleAndKey(r *http.Request) (*ratelimit.LimitRule, string) {
	return e.resolveRuleAndKey(r.Context(), r.URL.Path, r.Method, netx.GetClientIP(r), GetRequestCommonMeta(r.Context()).UserID)
}

// resolveRuleAndKey 限流规则解析核心（HTTP 与 gRPC 共用）
// 优先级: 路由白名单 > 路由黑名单 > 路由限流 > IP规则 > 用户规则 > 全局规则
func (e *rateLimitMiddleware) resolveRuleAndKey(ctx context.Context, path, method, clientIP, userID string) (*ratelimit.LimitRule, string) {
	// 第一轮: 优先检查白名单和黑名单(最高优先级)
	for _, routeLimit := range e.config.Routes {
		// 路径和方法匹配
		if !matcher.MatchPathWithMethod(path, method, routeLimit.Path, routeLimit.Methods) {
			continue
		}

		// 1. 白名单 - 最高优先级,直接放行(仅当白名单非空时检查)
		if len(routeLimit.Whitelist) > 0 && validator.IsIPAllowed(clientIP, routeLimit.Whitelist) {
			return nil, ""
		}

		// 2. 黑名单 - 第二优先级,严格限流(仅当黑名单非空时检查)
		if len(routeLimit.Blacklist) > 0 && validator.IsIPBlocked(clientIP, routeLimit.Blacklist) {
			return &ratelimit.LimitRule{
				RequestsPerSecond: 1,
				BurstSize:         1,
				WindowSize:        time.Minute,
				BlockDuration:     time.Hour,
			}, "blacklist:" + clientIP
		}

		// 3. 应用路由限流规则
		if routeLimit.Limit != nil {
			if routeLimit.PerUser {
				return routeLimit.Limit, "route:" + routeLimit.Path + ":user:" + userID
			}
			if routeLimit.PerIP {
				return routeLimit.Limit, "route:" + routeLimit.Path + ":ip:" + clientIP
			}
			return routeLimit.Limit, "route:" + routeLimit.Path
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
			return ipRule.Limit, "ip:" + clientIP
		}
	}

	// 第三轮: 检查用户级别规则
	if userID != "" {
		for _, userRule := range e.config.UserRules {
			if e.matchUser(userRule, userID) {
				return userRule.Limit, "user:" + userID
			}
		}
	}

	// 第四轮: 使用全局限流规则
	if e.config.GlobalLimit != nil {
		return e.config.GlobalLimit, e.generateKeyForScope(e.config.DefaultScope, clientIP, userID, method, path)
	}

	// 无任何限流规则,放行
	return nil, ""
}

func (e *rateLimitMiddleware) getLimiter(strategy ratelimit.Strategy) RateLimiter {
	if limiter := e.limiters.get(strategy); limiter != nil {
		return limiter
	}
	return e.limiter
}

// generateKey 生成限流key（HTTP 入口）
func (e *rateLimitMiddleware) generateKey(r *http.Request, scope ratelimit.Scope) string {
	return e.generateKeyForScope(scope, netx.GetClientIP(r), GetRequestCommonMeta(r.Context()).UserID, r.Method, r.URL.Path)
}

// generateKeyForScope 按作用域生成限流key（HTTP 与 gRPC 共用）
func (e *rateLimitMiddleware) generateKeyForScope(scope ratelimit.Scope, clientIP, userID, method, path string) string {
	switch scope {
	case ratelimit.ScopeGlobal:
		return keyGlobal
	case ratelimit.ScopePerIP:
		return "ip:" + clientIP
	case ratelimit.ScopePerUser:
		return "user:" + userID
	case ratelimit.ScopePerRoute:
		return "route:" + method + ":" + path
	default:
		return keyGlobal
	}
}

// matchUser 匹配用户（使用通配符匹配）
func (e *rateLimitMiddleware) matchUser(rule ratelimit.UserRule, userID string) bool {
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
	return newRateLimitMiddleware(config, nil, nil, nil).Middleware()
}

// RateLimitMiddlewareWithProvider 限流中间件（支持动态规则）
func RateLimitMiddlewareWithProvider(config *ratelimit.RateLimit, provider DynamicRateLimitProvider) HTTPMiddleware {
	return newRateLimitMiddleware(config, nil, provider, nil).Middleware()
}

// checkGRPCRateLimit gRPC 限流共享逻辑（Unary 和 Stream 复用）
// 返回 error 非 nil 表示拒绝（已包含 grpc status），调用方直接返回即可
func (e *rateLimitMiddleware) checkGRPCRateLimit(ctx context.Context, fullMethod string) error {
	if !e.config.Enabled {
		return nil
	}
	meta := GetRequestCommonMeta(ctx)
	// gRPC 协议 method 固定为 POST
	rule, key := e.resolveRuleAndKey(ctx, fullMethod, http.MethodPost, meta.IPAddress, meta.UserID)
	if rule == nil {
		return nil
	}
	rl := e.getLimiter(e.config.Strategy)
	if rl == nil {
		return nil
	}
	allowed, err := rl.Allow(ctx, key, rule)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	if !allowed {
		global.LOGGER.WarnContext(ctx, "[RateLimit] gRPC 限流触发: method=%s key=%s", fullMethod, key)
		return status.Error(codes.ResourceExhausted, "rate limit exceeded")
	}
	return nil
}

// GRPCRateLimitUnaryInterceptor gRPC 一元限流拦截器
// 复用与 HTTP 相同的限流配置与限流器实例，使 gRPC 方法路径（如 /pkg.Svc/Method）也能命中 routes 规则
func GRPCRateLimitUnaryInterceptor(config *ratelimit.RateLimit, limiter RateLimiter) grpc.UnaryServerInterceptor {
	mw := newRateLimitMiddleware(config, limiter, nil, nil)
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if err := mw.checkGRPCRateLimit(ctx, info.FullMethod); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// GRPCRateLimitStreamInterceptor gRPC 流式限流拦截器（在流建立时限流一次）
func GRPCRateLimitStreamInterceptor(config *ratelimit.RateLimit, limiter RateLimiter) grpc.StreamServerInterceptor {
	mw := newRateLimitMiddleware(config, limiter, nil, nil)
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := mw.checkGRPCRateLimit(ss.Context(), info.FullMethod); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}
