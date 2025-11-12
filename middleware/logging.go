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
	"fmt"
	"net/http"
	"time"

	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/osx"
)

// LoggingConfig 日志配置
type LoggingConfig struct {
	Enabled        bool     `json:"enabled" yaml:"enabled"`
	Format         string   `json:"format" yaml:"format"` // "json" 或 "text"
	IncludeBody    bool     `json:"includeBody" yaml:"includeBody"`
	IncludeQuery   bool     `json:"includeQuery" yaml:"includeQuery"`
	IncludeHeaders []string `json:"includeHeaders" yaml:"includeHeaders"`
}

// DefaultLoggingConfig 默认日志配置
func DefaultLoggingConfig() *LoggingConfig {
	return &LoggingConfig{
		Enabled:        true,
		Format:         "text",
		IncludeBody:    false,
		IncludeQuery:   true,
		IncludeHeaders: []string{constants.HeaderUserAgent, constants.HeaderXRequestID, constants.HeaderXTraceID},
	}
}

// LoggingMiddleware 日志中间件
func LoggingMiddleware(config *LoggingConfig) HTTPMiddleware {
	if config == nil {
		config = DefaultLoggingConfig()
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
func logRequestText(r *http.Request, rw *loggingResponseWriter, duration time.Duration, config *LoggingConfig) {
	logLine := fmt.Sprintf("[%s] %s %s %d %d %v %s",
		time.Now().Format(time.RFC3339),
		r.Method,
		r.URL.Path,
		rw.statusCode,
		rw.bytesWritten,
		duration,
		r.RemoteAddr,
	)

	if config.IncludeQuery && r.URL.RawQuery != "" {
		logLine += fmt.Sprintf(" query=%s", r.URL.RawQuery)
	}

	// 包含指定的头部
	for _, header := range config.IncludeHeaders {
		if value := r.Header.Get(header); value != "" {
			logLine += fmt.Sprintf(" %s=%s", header, value)
		}
	}

	global.LOGGER.Info(logLine)
}

// logRequestJSON JSON 格式日志
func logRequestJSON(r *http.Request, rw *loggingResponseWriter, duration time.Duration, config *LoggingConfig) {
	logData := map[string]interface{}{
		"timestamp":     time.Now().Format(time.RFC3339),
		"method":        r.Method,
		"path":          r.URL.Path,
		"status_code":   rw.statusCode,
		"bytes_written": rw.bytesWritten,
		"duration_ms":   duration.Milliseconds(),
		"remote_addr":   r.RemoteAddr,
		"user_agent":    r.UserAgent(),
	}

	if config.IncludeQuery && r.URL.RawQuery != "" {
		logData["query"] = r.URL.RawQuery
	}

	// 包含指定的头部
	headers := make(map[string]string)
	for _, header := range config.IncludeHeaders {
		if value := r.Header.Get(header); value != "" {
			headers[header] = value
		}
	}
	if len(headers) > 0 {
		logData["headers"] = headers
	}

	// 简单的 JSON 输出（生产环境建议使用专业的日志库）
	global.LOGGER.WithField("request", logData).InfoMsg("REQUEST")
}

// RecoveryMiddleware 恢复中间件
func RecoveryMiddleware() HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					global.LOGGER.ErrorKV("PANIC",
						"error", err,
						"request", r.Method+" "+r.URL.String(),
						"remote", r.RemoteAddr,
						"user_agent", r.UserAgent())

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
