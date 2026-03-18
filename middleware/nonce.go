/**
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-18 13:25:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-18 17:32:18
 * @FilePath: \go-rpc-gateway\middleware\nonce.go
 * @Description: Nonce 防重放中间件
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	gccommon "github.com/kamalyes/go-config/pkg/common"
	"github.com/kamalyes/go-config/pkg/signature"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/response"
	"github.com/kamalyes/go-toolbox/pkg/validator"
	"github.com/redis/go-redis/v9"
)

// NonceMiddleware Nonce 防重放中间件
//
// 功能：
// - 使用 Redis INCR 原子操作记录 Nonce 使用次数
// - 检测重放攻击（同一 Nonce 被多次使用）
// - 便于安全审计（可以统计 Nonce 使用频率）
func NonceMiddleware(config *signature.Signature) HTTPMiddleware {
	if !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 检查是否配置要求必须有 nonce
			if !config.RequireNonce {
				// 未开启,跳过 nonce 验证（向后兼容旧客户端）
				next.ServeHTTP(w, r)
				return
			}

			// Redis 不可用，记录警告并放行
			if global.REDIS == nil {
				global.LOGGER.WarnContext(r.Context(), "Nonce middleware enabled but Redis is not available")
				next.ServeHTTP(w, r)
				return
			}

			// 检查是否在忽略路径中
			if validator.MatchPathInList(r.URL.Path, config.IgnorePaths) {
				next.ServeHTTP(w, r)
				return
			}

			// 提取 Nonce
			nonceValue := gccommon.ExtractAttribute(r, config.NonceSources)
			if nonceValue == "" {
				response.WriteErrorResponseWithCode(w, http.StatusBadRequest, constants.SignatureErrorCodeInvalid, "Missing nonce header")
				return
			}

			// 检查并递增 Nonce 计数（原子操作）
			count, err := checkAndIncrNonce(r.Context(), global.REDIS, config.NonceKeyPrefix, nonceValue, config.NonceTTL)
			if err != nil {
				global.LOGGER.WarnContext(r.Context(), "Failed to check/store nonce: %v", err)
				response.WriteErrorResponseWithCode(w, http.StatusInternalServerError, constants.SignatureErrorCodeInvalid, "Nonce validation failed")
				return
			}

			// 如果计数 > 1，说明 Nonce 已被使用（重放攻击）
			if count > 1 {
				global.LOGGER.WarnContext(r.Context(), "Nonce replay attack detected: nonce=%s, count=%d", nonceValue, count)
				response.WriteErrorResponseWithCode(w, http.StatusUnauthorized, constants.SignatureErrorCodeInvalid, "Nonce has been used (replay attack detected)")
				return
			}

			// Nonce 验证通过，继续处理
			next.ServeHTTP(w, r)
		})
	}
}

// checkAndIncrNonce 检查并递增 Nonce 计数（原子操作）
// 返回：使用次数，错误
//
// 使用 INCR 的优势：
// 1. 原子操作，线程安全
// 2. 记录使用次数，便于安全审计
// 3. 可以检测重放攻击的频率
func checkAndIncrNonce(ctx context.Context, redisClient redis.UniversalClient, keyPrefix, nonce string, ttl time.Duration) (int64, error) {
	nonceKey := keyPrefix + nonce

	// 使用 INCR 原子递增计数
	count, err := redisClient.Incr(ctx, nonceKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment nonce counter: %w", err)
	}

	// 如果是第一次使用（count == 1），设置过期时间
	if count == 1 {
		if err := redisClient.Expire(ctx, nonceKey, ttl).Err(); err != nil {
			return count, fmt.Errorf("failed to set nonce expiration: %w", err)
		}
	}

	return count, nil
}
