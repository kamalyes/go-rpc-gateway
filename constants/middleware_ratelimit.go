/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:40:10
 * @FilePath: \go-rpc-gateway\constants\middleware_ratelimit.go
 * @Description: 限流中间件相关常量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

import "time"

// ============================================================================
// 限流中间件配置常量
// ============================================================================

// 限流中间件默认配置
const (
	RateLimitDefaultRPS       = 100 // 默认每秒请求数
	RateLimitDefaultBurstSize = 200 // 默认突发容量
)

// 限流策略常量（别名）
const (
	RateLimitStrategyTokenBucket    = "token_bucket"
	RateLimitStrategyLeakyBucket    = "leaky_bucket"
	RateLimitStrategyFixedWindow    = "fixed_window"
	RateLimitStrategySlidingLog     = "sliding_log"
	RateLimitAlgorithmTokenBucket   = "token_bucket"
	RateLimitAlgorithmSlidingWindow = "sliding_window"
	RateLimitAlgorithmFixedWindow   = "fixed_window"
	RateLimitAlgorithmLeakyBucket   = "leaky_bucket"
)

// 限流级别常量
const (
	RateLimitLevelGlobal = "global"
	RateLimitLevelIP     = "ip"
	RateLimitLevelUser   = "user"
	RateLimitLevelPath   = "path"
)

// 限流计数器精度
const (
	RateLimitBillion = 1e9 // 用于整数运算的精度因子
)

// 限流键生成函数类型
const (
	RateLimitKeyFuncIP     = "ip"
	RateLimitKeyFuncUser   = "user"
	RateLimitKeyFuncHeader = "header"
	RateLimitKeyFuncPath   = "path"
)

// 限流默认配置常量
const (
	// 默认每秒请求数
	RateLimitDefaultRate = 1000

	// 默认突发请求数
	RateLimitDefaultBurst = 100

	// 默认时间窗口大小（秒）
	RateLimitDefaultWindowSize = 60

	// 默认清理间隔
	RateLimitDefaultCleanupInterval = 5 * time.Minute

	// 默认键生成函数
	RateLimitDefaultKeyFunc = RateLimitKeyFuncIP

	// 默认算法
	RateLimitDefaultAlgorithm = RateLimitAlgorithmTokenBucket
)

// 限流响应头常量
const (
	// 标准限流头部
	RateLimitHeaderLimit      = "X-RateLimit-Limit"
	RateLimitHeaderRemaining  = "X-RateLimit-Remaining"
	RateLimitHeaderReset      = "X-RateLimit-Reset"
	RateLimitHeaderRetryAfter = "Retry-After"

	// GitHub 风格限流头部
	RateLimitHeaderGitHubLimit     = "X-RateLimit-Limit"
	RateLimitHeaderGitHubRemaining = "X-RateLimit-Remaining"
	RateLimitHeaderGitHubReset     = "X-RateLimit-Reset"

	// Twitter 风格限流头部
	RateLimitHeaderTwitterLimit     = "x-rate-limit-limit"
	RateLimitHeaderTwitterRemaining = "x-rate-limit-remaining"
	RateLimitHeaderTwitterReset     = "x-rate-limit-reset"
)

// 限流错误信息
const (
	RateLimitErrorMessage     = "Too Many Requests"
	RateLimitErrorDescription = "Rate limit exceeded. Please try again later."
)

// 默认白名单IP
var RateLimitDefaultWhitelistIPs = []string{
	"127.0.0.1",
	"::1",
	"localhost",
}
