/**
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-18 16:05:21
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-18 16:05:21
 * @FilePath: \go-rpc-gateway\middleware\timestamp.go
 * @Description: 时间戳验证中间件（独立）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/kamalyes/go-config/pkg/signature"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/response"
	"github.com/kamalyes/go-toolbox/pkg/validator"
)

// TimestampMiddleware 时间戳验证中间件
//
// 功能：
// - 验证请求时间戳是否在有效时间窗口内
// - 防止重放攻击（配合 Nonce 中间件使用效果更好）
//
// 使用场景：
// - 可以单独使用，只验证时间窗口
// - 通常与签名验证中间件配合使用
func TimestampMiddleware(config *signature.Signature) HTTPMiddleware {
	if !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 检查是否配置要求必须有 timestamp
			if !config.RequireTimestamp {
				// 未开启,跳过时间戳验证（向后兼容旧客户端）
				next.ServeHTTP(w, r)
				return
			}

			// 检查是否在忽略路径中
			if validator.MatchPathInList(r.URL.Path, config.IgnorePaths) {
				global.LOGGER.DebugContext(r.Context(), "Ignoring path %s as per config", r.URL.Path)
				next.ServeHTTP(w, r)
				return
			}

			// 从 context 提取时间戳
			timestampStr := GetRequestCommonMeta(r.Context()).Timestamp
			if timestampStr == "" {
				response.WriteErrorResponseWithCode(w, http.StatusBadRequest, constants.SignatureErrorCodeTimestampMissing, constants.SignatureErrorTimestampMissing)
				return
			}

			// 解析时间戳
			timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
			if err != nil {
				global.LOGGER.DebugContext(r.Context(), "Invalid timestamp format: %s", timestampStr)
				response.WriteErrorResponseWithCode(w, http.StatusBadRequest, constants.SignatureErrorCodeTimestampInvalid, constants.SignatureErrorTimestampInvalid)
				return
			}

			// 验证时间窗口
			now := time.Now().Unix()
			diff := now - timestamp
			if diff < 0 {
				diff = -diff
			}

			// 检查是否超出时间窗口
			if diff > int64(config.TimeoutWindow.Seconds()) {
				global.LOGGER.DebugContext(r.Context(), "Timestamp expired: %d seconds ago, timeout window is %d seconds", diff, config.TimeoutWindow.Seconds())
				response.WriteErrorResponseWithCode(w, http.StatusForbidden, constants.SignatureErrorCodeTimestampExpired, constants.SignatureErrorTimestampExpired)
				return
			}

			// 时间戳验证通过，继续处理
			next.ServeHTTP(w, r)
		})
	}
}
