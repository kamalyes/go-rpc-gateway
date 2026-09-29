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
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	validator "github.com/kamalyes/go-argus"
	"github.com/kamalyes/go-config/pkg/signature"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/response"
	"github.com/redis/go-redis/v9"
)

// NonceMiddleware Nonce 防重放中间件
//
// 功能：
// - 使用 Redis INCR 原子操作记录 Nonce 使用次数
// - 检测重放攻击（同一 Nonce 被多次使用）
// - 旧客户端未携带 Nonce 时基于请求指纹自动派生，无需改造即获得防重放能力
// - 便于安全审计（可以统计 Nonce 使用频率）
func NonceMiddleware(config *signature.Signature) HTTPMiddleware {
	if !config.Enabled {
		return noopMiddleware
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

			// 从 context 提取 Nonce，旧客户端未携带时基于请求指纹自动派生
			requestCommonMeta := GetRequestCommonMeta(r.Context())
			nonceValue := requestCommonMeta.Nonce
			if nonceValue == "" {
				// 无 Timestamp 时无法构造稳定指纹（时间窗口校验也不生效），直接放行
				if requestCommonMeta.Timestamp == "" {
					global.LOGGER.DebugContext(r.Context(), "Nonce and timestamp both missing, skip replay check")
					next.ServeHTTP(w, r)
					return
				}

				var deriveErr error
				nonceValue, deriveErr = deriveNonce(r, requestCommonMeta, config)
				if deriveErr != nil {
					global.LOGGER.WarnContext(r.Context(), "Failed to derive nonce: %v", deriveErr)
					response.WriteErrorResponseWithCode(w, http.StatusBadRequest, constants.SignatureErrorCodeInvalid, "Nonce validation failed")
					return
				}
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

// deriveNonce 基于请求指纹派生 Nonce（向后兼容未携带 X-Nonce 的旧客户端）
//
// 指纹 = Authorization + AccessKey + Timestamp + Query + Body 的 MD5
// 同一签名请求被原样重放时派生值相同，将被防重放检测拦截
// 正常重试会携带新 Timestamp 重新签名，派生值不同，不受影响
func deriveNonce(r *http.Request, requestCommonMeta *RequestCommonMeta, config *signature.Signature) (string, error) {
	body, err := readRequestBody(r, config.SkipBody)
	if err != nil {
		return "", fmt.Errorf("failed to read request body: %w", err)
	}

	fingerprint := md5.New()
	fingerprint.Write([]byte(requestCommonMeta.Authorization))
	fingerprint.Write([]byte(requestCommonMeta.AccessKey))
	fingerprint.Write([]byte(requestCommonMeta.Timestamp))
	fingerprint.Write([]byte(getQueryString(r, config.SkipQuery)))
	fingerprint.Write(body)

	return hex.EncodeToString(fingerprint.Sum(nil)), nil
}

// nonceIncrScript 原子递增 Nonce 计数并在首次使用时设置过期时间
// 使用 redis.NewScript 自动走 EVALSHA 复用脚本缓存；Lua 原子执行将
// 原本的 INCR + EXPIRE 两次网络往返合并为一次
var nonceIncrScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
	redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return count
`)

// checkAndIncrNonce 检查并递增 Nonce 计数（原子操作）
// 返回：使用次数，错误
//
// 使用 Lua 脚本的优势：
// 1. INCR 与 PEXPIRE 原子执行，单次网络往返
// 2. 记录使用次数，便于安全审计
// 3. 可以检测重放攻击的频率
func checkAndIncrNonce(ctx context.Context, redisClient redis.UniversalClient, keyPrefix, nonce string, ttl time.Duration) (int64, error) {
	nonceKey := keyPrefix + nonce

	result, err := nonceIncrScript.Run(ctx, redisClient, []string{nonceKey}, ttl.Milliseconds()).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment nonce counter: %w", err)
	}

	count, ok := result.(int64)
	if !ok {
		return 0, fmt.Errorf("unexpected nonce counter result type: %T", result)
	}

	return count, nil
}
