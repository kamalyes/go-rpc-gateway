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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/kamalyes/go-config/pkg/logging"
	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
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
			processHTTPRequest(w, r, config, next)
		})
	}
}

// processHTTPRequest 处理 HTTP 请求的主要逻辑
func processHTTPRequest(w http.ResponseWriter, r *http.Request, config *logging.Logging, next http.Handler) {
	start := time.Now()
	ctx := r.Context()

	// 路径过滤和请求准备
	skipDetail := isSkipPath(r.URL.Path, config)
	reqBody := captureRequestBody(r, config, skipDetail)

	// 设置响应包装器
	wrapped := setupResponseWriter(w, skipDetail, config)
	defer wrapped.Release()

	// 执行请求
	next.ServeHTTP(wrapped, r)

	// 记录日志
	logRequestIfNeeded(ctx, r, wrapped, time.Since(start), config, reqBody, skipDetail)
}

// setupResponseWriter 设置响应写入器
func setupResponseWriter(w http.ResponseWriter, skipDetail bool, config *logging.Logging) *ResponseWriter {
	wrapped := NewResponseWriter(w)
	if !skipDetail && config.EnableResponse {
		wrapped.EnableBodyCapture()
	}
	return wrapped
}

// logRequestIfNeeded 根据需要记录请求日志
func logRequestIfNeeded(ctx context.Context, r *http.Request, wrapped *ResponseWriter, duration time.Duration, config *logging.Logging, reqBody []byte, skipDetail bool) {
	if skipDetail {
		logSkipPath(ctx, r, wrapped, duration)
		return
	}

	if shouldLogRequest(wrapped.StatusCode(), config) {
		if config.Format == "json" {
			logRequestJSON(ctx, r, wrapped, duration, config, reqBody)
		} else {
			logRequestText(ctx, r, wrapped, duration, config)
		}
	}
}

// captureRequestBody 捕获请求体
func captureRequestBody(r *http.Request, config *logging.Logging, skipDetail bool) []byte {
	if !config.EnableRequest || r.Body == nil || skipDetail {
		return nil
	}

	maxSize := getMaxBodySize(config)
	limitedBody := io.LimitReader(r.Body, int64(maxSize+1))
	reqBody, _ := io.ReadAll(limitedBody)
	r.Body = io.NopCloser(bytes.NewBuffer(reqBody))

	if len(reqBody) > maxSize {
		reqBody = reqBody[:maxSize]
	}
	return reqBody
}

// isSkipPath 检查是否为跳过路径
func isSkipPath(path string, config *logging.Logging) bool {
	for _, skip := range config.SkipPaths {
		if path == skip {
			return true
		}
	}
	return false
}

// getMaxBodySize 获取最大 body 大小
func getMaxBodySize(config *logging.Logging) int {
	if config.MaxBodySize > 0 {
		return config.MaxBodySize
	}
	return 2048 // 默认值
}

// logSkipPath 记录跳过的路径（仅记录错误）
func logSkipPath(ctx context.Context, r *http.Request, rw *ResponseWriter, duration time.Duration) {
	if rw.StatusCode() >= 400 {
		global.LOGGER.WarnContextKV(ctx, "[Gateway] HTTP Request (Skip Path)",
			"path", r.URL.Path,
			"status", rw.StatusCode(),
			"duration_ms", duration.Milliseconds(),
		)
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
	global.LOGGER.InfoContextKV(ctx, "[Gateway] HTTP Request",
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
func logRequestJSON(ctx context.Context, r *http.Request, rw *ResponseWriter, duration time.Duration, config *logging.Logging, reqBody []byte) {
	fields := buildBaseLogFields(r, rw, duration, config)
	fields = appendRequestBodyFields(fields, reqBody, r, config)
	fields = appendResponseBodyFields(fields, rw, config)
	fields = appendUserContextFields(fields, ctx)

	global.LOGGER.InfoContextKV(ctx, "[Gateway] HTTP Request", fields...)
}

// buildBaseLogFields 构建基础日志字段
func buildBaseLogFields(r *http.Request, rw *ResponseWriter, duration time.Duration, config *logging.Logging) []interface{} {
	query := ""
	if config.EnableRequest && r.URL.RawQuery != "" {
		query = r.URL.RawQuery
	}

	return []interface{}{
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
}

// appendRequestBodyFields 添加请求体字段
func appendRequestBodyFields(fields []interface{}, reqBody []byte, r *http.Request, config *logging.Logging) []interface{} {
	if config.EnableRequest && len(reqBody) > 0 {
		return appendBodyFields(fields, "request", reqBody, r.Header.Get("Content-Type"), config)
	}
	return fields
}

// appendResponseBodyFields 添加响应体字段
func appendResponseBodyFields(fields []interface{}, rw *ResponseWriter, config *logging.Logging) []interface{} {
	if respBody := rw.GetBody(); len(respBody) > 0 {
		return appendBodyFields(fields, "response", respBody, rw.Header().Get("Content-Type"), config)
	} else if config.EnableResponse {
		return append(fields, "response_empty", true)
	}
	return fields
}

// appendUserContextFields 添加用户上下文字段
func appendUserContextFields(fields []interface{}, ctx context.Context) []interface{} {
	if userID := logger.GetUserID(ctx); userID != "" {
		fields = append(fields, "user_id", userID)
	}
	if tenantID := logger.GetTenantID(ctx); tenantID != "" {
		fields = append(fields, "tenant_id", tenantID)
	}
	return fields
}

// RecoveryMiddleware 恢复中间件
func RecoveryMiddleware() HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					handlePanicRecovery(w, r, err)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// handlePanicRecovery 处理 panic 恢复
func handlePanicRecovery(w http.ResponseWriter, r *http.Request, err interface{}) {
	ctx := r.Context()

	// 记录 panic 信息
	logPanicError(ctx, r, err)

	// 设置错误响应
	setPanicErrorResponse(w, ctx)
}

// logPanicError 记录 panic 错误日志
func logPanicError(ctx context.Context, r *http.Request, err interface{}) {
	if global.LOGGER == nil {
		return
	}

	fields := buildPanicLogFields(r, err)
	fields = appendUserContextFields(fields, ctx)
	global.LOGGER.ErrorContextKV(ctx, "PANIC Recovered", fields...)
}

// buildPanicLogFields 构建 panic 日志字段
func buildPanicLogFields(r *http.Request, err interface{}) []interface{} {
	return []interface{}{
		"error", err,
		"method", r.Method,
		"path", r.URL.String(),
		"remote_addr", getClientIP(r),
		"user_agent", r.UserAgent(),
	}
}

// setPanicErrorResponse 设置 panic 错误响应
func setPanicErrorResponse(w http.ResponseWriter, ctx context.Context) {
	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
	setTraceHeaders(w, ctx)
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(constants.JSONInternalError))
}

// setTraceHeaders 设置追踪头信息
func setTraceHeaders(w http.ResponseWriter, ctx context.Context) {
	if traceID := logger.GetTraceID(ctx); traceID != "" {
		w.Header().Set(constants.HeaderXTraceID, traceID)
	}
	if requestID := logger.GetRequestID(ctx); requestID != "" {
		w.Header().Set(constants.HeaderXRequestID, requestID)
	}
}

// appendBodyFields 添加 body 字段
func appendBodyFields(fields []interface{}, name string, body []byte, contentType string, config *logging.Logging) []interface{} {
	if isLoggableContentType(contentType, config) {
		fields = append(fields, name, maskSensitiveData(body, config))
		if len(body) > getMaxBodySize(config) {
			fields = append(fields, name+"_truncated", true)
		}
	} else {
		fields = append(fields, name, "<binary>")
	}
	return fields
}

// isLoggableContentType 检查 Content-Type 是否可记录
func isLoggableContentType(contentType string, config *logging.Logging) bool {
	if contentType == "" {
		return true
	}
	contentType = strings.ToLower(contentType)
	for _, prefix := range config.LoggableContentTypes {
		if strings.HasPrefix(contentType, prefix) {
			return true
		}
	}
	return false
}

// maskSensitiveData 脱敏敏感数据
func maskSensitiveData(data []byte, config *logging.Logging) string {
	if len(data) == 0 {
		return ""
	}

	body := truncateDataIfNeeded(data, config)

	// 尝试 JSON 脱敏
	if jsonResult := tryMaskJSONData(body, config); jsonResult != "" {
		return jsonResult
	}

	// 非 JSON: 正则脱敏
	return maskNonJSONData(body, config)
}

// truncateDataIfNeeded 如果数据超过最大尺寸则截断
func truncateDataIfNeeded(data []byte, config *logging.Logging) []byte {
	maxSize := getMaxBodySize(config)
	if len(data) > maxSize {
		return data[:maxSize]
	}
	return data
}

// tryMaskJSONData 尝试按 JSON 格式脱敏，如果成功返回脱敏后的字符串，否则返回空字符串
func tryMaskJSONData(body []byte, config *logging.Logging) string {
	var jsonData map[string]interface{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		return ""
	}

	maskSensitiveFields(jsonData, config)
	masked, err := json.Marshal(jsonData)
	if err != nil {
		return ""
	}
	return string(masked)
}

// maskNonJSONData 对非 JSON 数据进行正则脱敏
func maskNonJSONData(body []byte, config *logging.Logging) string {
	result := string(body)
	mask := getSensitiveMask(config)
	for _, key := range config.SensitiveKeys {
		re := regexp.MustCompile(`(?i)"?` + key + `"?\s*[:=]\s*"?[^"&,}\s]+`)
		result = re.ReplaceAllString(result, key+"="+mask)
	}
	return result
}

// getSensitiveMask 获取敏感数据掩码
func getSensitiveMask(config *logging.Logging) string {
	if config.SensitiveMask != "" {
		return config.SensitiveMask
	}
	return "***REDACTED***"
}

// maskSensitiveFields 递归脱敏 JSON 对象
func maskSensitiveFields(data interface{}, config *logging.Logging) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			if isSensitiveKey(key, config) {
				v[key] = getSensitiveMask(config)
			} else {
				maskSensitiveFields(value, config)
			}
		}
	case []interface{}:
		for _, item := range v {
			maskSensitiveFields(item, config)
		}
	}
}

// isSensitiveKey 检查是否为敏感字段
func isSensitiveKey(key string, config *logging.Logging) bool {
	lowerKey := strings.ToLower(key)
	for _, sensitive := range config.SensitiveKeys {
		if strings.Contains(lowerKey, sensitive) {
			return true
		}
	}
	return false
}
