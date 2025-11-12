/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 01:07:24
 * @FilePath: \go-rpc-gateway\middleware\requestid.go
 * @Description: 请求ID中间件
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/kamalyes/go-config/pkg/requestid"
	"github.com/kamalyes/go-rpc-gateway/global"
)

// RequestID 请求ID中间件
func RequestID() MiddlewareFunc {
	return RequestIDWithConfig(DefaultRequestIDConfig())
}

// RequestIDWithConfig 带配置的请求ID中间件
func RequestIDWithConfig(config RequestIDConfig) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get(config.Header)
			if requestID == "" {
				requestID = config.Generator()
			}

			// 设置响应头
			w.Header().Set(config.Header, requestID)

			// 添加到context中
			ctx := context.WithValue(r.Context(), requestIDKey("request_id"), requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ConfigurableRequestIDMiddleware 可配置的请求ID中间件
func ConfigurableRequestIDMiddleware(config *requestid.RequestID) HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// 获取请求ID
			headerName := config.HeaderName
			if headerName == "" {
				headerName = "X-Request-ID" // 默认头名称
			}

			requestID := r.Header.Get(headerName)
			if requestID == "" {
				requestID = generateRequestID(config.Generator)
			}

			// 设置响应头
			w.Header().Set(headerName, requestID)

			// 添加到context中
			ctx := context.WithValue(r.Context(), requestIDKey("request_id"), requestID)

			// 记录日志（如果启用）
			if global.LOGGER != nil {
				global.LOGGER.DebugKV("请求ID已生成",
					"request_id", requestID,
					"method", r.Method,
					"path", r.URL.Path,
					"generator", config.Generator)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// generateRequestID 根据配置生成请求ID
func generateRequestID(generator string) string {
	switch generator {
	case "uuid":
		return generateUUID()
	case "snowflake":
		return generateSnowflakeID()
	case "timestamp":
		return generateTimestampID()
	default:
		return generateUUID() // 默认使用UUID
	}
}

// generateUUID 生成UUID格式的请求ID
func generateUUID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// 如果随机数生成失败，使用时间戳
		return generateTimestampID()
	}

	// 设置版本 (4) 和变体位
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

// generateSnowflakeID 生成雪花ID格式的请求ID
func generateSnowflakeID() string {
	// 简化的雪花ID实现
	timestamp := time.Now().UnixMilli()
	randomPart := make([]byte, 4)
	rand.Read(randomPart)

	return fmt.Sprintf("%d%s", timestamp, hex.EncodeToString(randomPart))
}

// generateTimestampID 生成时间戳格式的请求ID
func generateTimestampID() string {
	return fmt.Sprintf("req_%d_%x",
		time.Now().UnixNano(),
		time.Now().UnixNano()%1000000)
}

// requestIDKey 是一个类型为 string 的键，用于 context.WithValue
type requestIDKey string
