/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-29 12:00:00
 * @FilePath: \go-rpc-gateway\middleware\logging.go
 * @Description: HTTP 日志中间件（纯日志功能，context 注入请使用 context_trace.go）
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"context"
	"github.com/kamalyes/go-config/pkg/logging"
	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"net/http"
	"time"
)

// LoggingMiddleware HTTP 日志中间件
// 注意：context 注入请使用 ContextTraceMiddleware，此中间件只负责日志记录
func LoggingMiddleware(config *logging.Logging) HTTPMiddleware {
	if config == nil {
		config = logging.Default()
	}

	if !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := r.Context()

			// 使用统一的 ResponseWriter 包装器
			wrapped := NewResponseWriter(w)
			defer wrapped.Release() // 确保归还到对象池

			next.ServeHTTP(wrapped, r)

			// 记录请求日志（从 context 中提取 trace 信息）
			duration := time.Since(start)

			// 日志采样:仅记录部分成功请求,但总是记录错误
			if shouldLogRequest(wrapped.StatusCode(), config) {
				if config.Format == "json" {
					logRequestJSON(ctx, r, wrapped, duration, config)
				} else {
					logRequestText(ctx, r, wrapped, duration, config)
				}
			}
		})
	}
}

// shouldLogRequest 判断是否应该记录该请求日志
func shouldLogRequest(statusCode int, config *logging.Logging) bool {
	// 始终记录错误（4xx, 5xx）
	if statusCode >= 400 {
		return true
	}

	// 如果配置了采样率，仅记录部分成功请求
	// 注意：这需要在 logging.Logging 配置中添加 SampleRate 字段
	// 这里假设所有请求都记录，后续可根据需要调整
	return true
}

// logRequestText 记录文本格式日志
func logRequestText(ctx context.Context, r *http.Request, rw *ResponseWriter, duration time.Duration, config *logging.Logging) {
	// 使用 ContextKV 记录日志，trace 信息从 context 中自动提取
	global.LOGGER.InfoContextKV(ctx, "HTTP Request",
		"method", r.Method,
		"path", r.URL.Path,
		"status", rw.StatusCode(),
		"bytes", rw.BytesWritten(),
		"duration_ms", duration.Milliseconds(),
		"remote_addr", getClientIP(r),
		"user_agent", r.Header.Get(constants.HeaderUserAgent),
	)
}

// logRequestJSON JSON 格式日志
func logRequestJSON(ctx context.Context, r *http.Request, rw *ResponseWriter, duration time.Duration, config *logging.Logging) {
	query := ""
	if config.EnableRequest && r.URL.RawQuery != "" {
		query = r.URL.RawQuery
	}

	// 使用 ContextKV 记录详细日志，trace 信息从 context 中自动提取
	fields := []interface{}{
		"timestamp", time.Now().Format(time.RFC3339),
		"method", r.Method,
		"path", r.URL.Path,
		"query", query,
		"status_code", rw.StatusCode(),
		"bytes_written", rw.BytesWritten(),
		"duration_ms", duration.Milliseconds(),
		"remote_addr", getClientIP(r),
		"user_agent", r.UserAgent(),
	}

	// 添加可选的用户和租户信息
	if userID := logger.GetUserID(ctx); userID != "" {
		fields = append(fields, "user_id", userID)
	}
	if tenantID := logger.GetTenantID(ctx); tenantID != "" {
		fields = append(fields, "tenant_id", tenantID)
	}

	global.LOGGER.InfoContextKV(ctx, "HTTP Request", fields...)
}

// RecoveryMiddleware 恢复中间件
func RecoveryMiddleware() HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					ctx := r.Context()

					// 记录详细的 panic 信息
					if global.LOGGER != nil {
						fields := []interface{}{
							"error", err,
							"method", r.Method,
							"path", r.URL.String(),
							"remote_addr", getClientIP(r),
							"user_agent", r.UserAgent(),
						}

						// 添加用户和租户信息（如果存在）
						if userID := logger.GetUserID(ctx); userID != "" {
							fields = append(fields, "user_id", userID)
						}
						if tenantID := logger.GetTenantID(ctx); tenantID != "" {
							fields = append(fields, "tenant_id", tenantID)
						}

						global.LOGGER.ErrorContextKV(ctx, "PANIC Recovered", fields...)
					}

					// 返回 500 错误（包含 trace_id 便于追踪）
					w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
					// 确保 trace_id 在响应头中（可能在 panic 前未设置）
					if traceID := logger.GetTraceID(ctx); traceID != "" {
						w.Header().Set(constants.HeaderXTraceID, traceID)
					}
					if requestID := logger.GetRequestID(ctx); requestID != "" {
						w.Header().Set(constants.HeaderXRequestID, requestID)
					}
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(constants.JSONInternalError))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
