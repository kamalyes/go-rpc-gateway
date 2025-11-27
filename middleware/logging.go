/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 01:11:21
 * @FilePath: \go-rpc-gateway\middleware\logging.go
 * @Description: 日志中间件
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	gologging "github.com/kamalyes/go-config/pkg/logging"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/osx"
	"net/http"
	"time"
)

// LoggingMiddleware 日志中间件
func LoggingMiddleware(config *gologging.Logging) HTTPMiddleware {
	if config == nil {
		config = gologging.Default()
	}

	if !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 包装 ResponseWriter 以获取状态码和响应大小
			wrapped := &loggingResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(wrapped, r)

			// 记录请求日志
			duration := time.Since(start)

			if config.Format == "json" {
				logRequestJSON(r, wrapped, duration, config)
			} else {
				logRequestText(r, wrapped, duration, config)
			}
		})
	}
}

// loggingResponseWriter 包装器用于获取状态码和响应大小
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (rw *loggingResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *loggingResponseWriter) Write(data []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(data)
	rw.bytesWritten += int64(n)
	return n, err
}

// logRequestText 文本格式日志
func logRequestText(r *http.Request, rw *loggingResponseWriter, duration time.Duration, config *gologging.Logging) {
	ctx := r.Context()

	// 提取关键信息
	requestID := r.Header.Get(constants.HeaderXRequestID)
	traceID := r.Header.Get(constants.HeaderXTraceID)

	// 使用 ContextKV 记录日志
	global.LOGGER.InfoContextKV(ctx, "HTTP Request",
		"method", r.Method,
		"path", r.URL.Path,
		"status", rw.statusCode,
		"bytes", rw.bytesWritten,
		"duration_ms", duration.Milliseconds(),
		"remote_addr", r.RemoteAddr,
		"request_id", requestID,
		"trace_id", traceID,
		"user_agent", r.Header.Get(constants.HeaderUserAgent),
	)
}

// logRequestJSON JSON 格式日志
func logRequestJSON(r *http.Request, rw *loggingResponseWriter, duration time.Duration, config *gologging.Logging) {
	ctx := r.Context()

	// 提取关键信息
	requestID := r.Header.Get(constants.HeaderXRequestID)
	traceID := r.Header.Get(constants.HeaderXTraceID)
	query := ""
	if config.EnableRequest && r.URL.RawQuery != "" {
		query = r.URL.RawQuery
	}

	// 使用 ContextKV 记录详细日志
	global.LOGGER.InfoContextKV(ctx, "HTTP Request",
		"timestamp", time.Now().Format(time.RFC3339),
		"method", r.Method,
		"path", r.URL.Path,
		"query", query,
		"status_code", rw.statusCode,
		"bytes_written", rw.bytesWritten,
		"duration_ms", duration.Milliseconds(),
		"remote_addr", r.RemoteAddr,
		"user_agent", r.UserAgent(),
		"request_id", requestID,
		"trace_id", traceID,
	)
}

// RecoveryMiddleware 恢复中间件
func RecoveryMiddleware() HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					ctx := r.Context()
					requestID := r.Header.Get(constants.HeaderXRequestID)
					traceID := r.Header.Get(constants.HeaderXTraceID)

					global.LOGGER.ErrorContextKV(ctx, "PANIC Recovered",
						"error", err,
						"method", r.Method,
						"path", r.URL.String(),
						"remote_addr", r.RemoteAddr,
						"user_agent", r.UserAgent(),
						"request_id", requestID,
						"trace_id", traceID,
					)

					// 返回 500 错误
					w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(constants.JSONInternalError))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDMiddleware 请求 ID 中间件
func RequestIDMiddleware() HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 尝试从头部获取请求 ID
			requestID := r.Header.Get(constants.HeaderXRequestID)
			if requestID == "" {
				// 如果没有，生成一个新的
				requestID = osx.HashUnixMicroCipherText()
			}

			// 设置到响应头中
			w.Header().Set(constants.HeaderXRequestID, requestID)

			// 添加到请求上下文中（如果需要的话）
			// ctx := context.WithValue(r.Context(), "request_id", requestID)
			// r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
