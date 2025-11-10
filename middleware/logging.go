/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 18:15:00
 * @FilePath: \go-rpc-gateway\middleware\logging.go
 * @Description: 日志中间件
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kamalyes/go-core/pkg/global"
	"github.com/kamalyes/go-rpc-gateway/config"
	"github.com/kamalyes/go-rpc-gateway/constants"
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

// ConfigurableLoggingMiddleware 可配置的日志中间件
func ConfigurableLoggingMiddleware(loggingConfig *config.LoggingConfig) HTTPMiddleware {
	if loggingConfig == nil || !loggingConfig.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// 解析最小延迟
	var minLatency time.Duration
	if loggingConfig.Filters.MinLatency != "" {
		if duration, err := time.ParseDuration(loggingConfig.Filters.MinLatency); err == nil {
			minLatency = duration
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 检查是否应该忽略此路径
			if shouldIgnorePath(r.URL.Path, loggingConfig.Filters.IgnorePaths) {
				next.ServeHTTP(w, r)
				return
			}

			// 检查是否应该忽略此User-Agent
			if shouldIgnoreUserAgent(r.UserAgent(), loggingConfig.Filters.IgnoreUserAgents) {
				next.ServeHTTP(w, r)
				return
			}

			// 包装 ResponseWriter 以获取状态码和响应大小
			wrapped := &configLoggingResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				requestSize:    getRequestSize(r),
			}

			next.ServeHTTP(wrapped, r)

			// 计算延迟
			duration := time.Since(start)

			// 检查最小延迟过滤
			if minLatency > 0 && duration < minLatency {
				return
			}

			// 检查是否应该忽略此状态码
			if shouldIgnoreStatusCode(wrapped.statusCode, loggingConfig.Filters.IgnoreStatusCodes) {
				return
			}

			// 记录日志
			if loggingConfig.Format == "json" {
				logConfigurableRequestJSON(r, wrapped, duration, loggingConfig)
			} else {
				logConfigurableRequestText(r, wrapped, duration, loggingConfig)
			}
		})
	}
}

// configLoggingResponseWriter 增强的响应写入器
type configLoggingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
	requestSize  int64
}

func (rw *configLoggingResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *configLoggingResponseWriter) Write(data []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(data)
	rw.bytesWritten += int64(n)
	return n, err
}

// shouldIgnorePath 检查是否应该忽略此路径
func shouldIgnorePath(path string, ignorePaths []string) bool {
	for _, ignorePath := range ignorePaths {
		if strings.HasPrefix(path, ignorePath) {
			return true
		}
	}
	return false
}

// shouldIgnoreUserAgent 检查是否应该忽略此User-Agent
func shouldIgnoreUserAgent(userAgent string, ignoreUserAgents []string) bool {
	for _, ignoreUA := range ignoreUserAgents {
		if strings.Contains(userAgent, ignoreUA) {
			return true
		}
	}
	return false
}

// shouldIgnoreStatusCode 检查是否应该忽略此状态码
func shouldIgnoreStatusCode(statusCode int, ignoreStatusCodes []int) bool {
	for _, ignoreCode := range ignoreStatusCodes {
		if statusCode == ignoreCode {
			return true
		}
	}
	return false
}

// getRequestSize 获取请求大小
func getRequestSize(r *http.Request) int64 {
	if r.ContentLength > 0 {
		return r.ContentLength
	}
	return 0
}

// logConfigurableRequestText 使用配置的文本格式日志
func logConfigurableRequestText(r *http.Request, rw *configLoggingResponseWriter, duration time.Duration, config *config.LoggingConfig) {
	var parts []string

	if config.Fields.Timestamp {
		parts = append(parts, fmt.Sprintf("[%s]", time.Now().Format(time.RFC3339)))
	}

	if config.Fields.Method {
		parts = append(parts, r.Method)
	}

	if config.Fields.URI {
		parts = append(parts, r.URL.String())
	}

	if config.Fields.Protocol {
		parts = append(parts, r.Proto)
	}

	if config.Fields.StatusCode {
		parts = append(parts, strconv.Itoa(rw.statusCode))
	}

	if config.Fields.ResponseSize {
		parts = append(parts, fmt.Sprintf("%dB", rw.bytesWritten))
	}

	if config.Fields.RequestSize {
		parts = append(parts, fmt.Sprintf("req:%dB", rw.requestSize))
	}

	if config.Fields.Latency {
		parts = append(parts, fmt.Sprintf("lat:%v", duration))
	}

	if config.Fields.RemoteAddr {
		parts = append(parts, fmt.Sprintf("ip:%s", r.RemoteAddr))
	}

	if config.Fields.UserAgent {
		parts = append(parts, fmt.Sprintf("ua:%s", r.UserAgent()))
	}

	if config.Fields.Referer {
		if referer := r.Referer(); referer != "" {
			parts = append(parts, fmt.Sprintf("ref:%s", referer))
		}
	}

	// 添加自定义字段
	for key, value := range config.CustomFields {
		parts = append(parts, fmt.Sprintf("%s:%s", key, value))
	}

	logLine := strings.Join(parts, " ")
	global.LOGGER.Info(logLine)
}

// logConfigurableRequestJSON 使用配置的JSON格式日志
func logConfigurableRequestJSON(r *http.Request, rw *configLoggingResponseWriter, duration time.Duration, config *config.LoggingConfig) {
	logData := make(map[string]interface{})

	if config.Fields.Timestamp {
		logData["timestamp"] = time.Now().Format(time.RFC3339)
	}

	if config.Fields.Method {
		logData["method"] = r.Method
	}

	if config.Fields.URI {
		logData["uri"] = r.URL.String()
	}

	if config.Fields.Protocol {
		logData["protocol"] = r.Proto
	}

	if config.Fields.StatusCode {
		logData["status_code"] = rw.statusCode
	}

	if config.Fields.ResponseSize {
		logData["response_size"] = rw.bytesWritten
	}

	if config.Fields.RequestSize {
		logData["request_size"] = rw.requestSize
	}

	if config.Fields.Latency {
		logData["latency_ms"] = duration.Milliseconds()
	}

	if config.Fields.RemoteAddr {
		logData["remote_addr"] = r.RemoteAddr
	}

	if config.Fields.UserAgent {
		logData["user_agent"] = r.UserAgent()
	}

	if config.Fields.Referer {
		if referer := r.Referer(); referer != "" {
			logData["referer"] = referer
		}
	}

	if config.Fields.RequestID {
		if requestID := r.Header.Get(constants.HeaderXRequestID); requestID != "" {
			logData["request_id"] = requestID
		}
	}

	if config.Fields.IncludeHeaders {
		headers := make(map[string]string)
		for key, values := range r.Header {
			headers[key] = strings.Join(values, ",")
		}
		logData["headers"] = headers
	}

	// 添加自定义字段
	for key, value := range config.CustomFields {
		logData[key] = value
	}

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
