/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-11
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 15:15:17
 * @FilePath: \go-rpc-gateway\middleware\logging.go
 * @Description: 统一日志中间件 - 支持 HTTP 和 gRPC
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kamalyes/go-config/pkg/logging"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/netx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// RequestLogger 统一的请求日志记录器
type RequestLogger struct {
	config *logging.Logging
	ctx    context.Context
}

// NewRequestLogger 创建请求日志记录器
func NewRequestLogger(ctx context.Context) *RequestLogger {
	config := getLoggingConfig()
	return &RequestLogger{
		config: config,
		ctx:    ctx,
	}
}

// LogFields 日志字段构建器
type LogFields struct {
	fields []any
}

// NewLogFields 创建日志字段构建器
func NewLogFields() *LogFields {
	return &LogFields{fields: make([]any, 0, 20)}
}

// Add 添加字段
func (lf *LogFields) Add(key string, value any) *LogFields {
	lf.fields = append(lf.fields, key, value)
	return lf
}

// AddIf 条件添加字段
func (lf *LogFields) AddIf(condition bool, key string, value any) *LogFields {
	if condition {
		lf.fields = append(lf.fields, key, value)
	}
	return lf
}

// AddUserContext 添加用户上下文信息
func (lf *LogFields) AddUserContext(ctx context.Context) *LogFields {
	traceInfo := GetCachedTraceInfo(ctx)
	if traceInfo.UserID != "" {
		lf.fields = append(lf.fields, "user_id", traceInfo.UserID)
	}
	if traceInfo.TenantID != "" {
		lf.fields = append(lf.fields, "tenant_id", traceInfo.TenantID)
	}
	return lf
}

// AddSlow 添加慢请求标记 🐌
func (lf *LogFields) AddSlow(duration, threshold time.Duration) *LogFields {
	if duration > threshold {
		lf.fields = append(lf.fields, "slow_request", true)
	}
	return lf
}

// Build 构建字段列表
func (lf *LogFields) Build() []any {
	return lf.fields
}

// Log 记录日志
func (rl *RequestLogger) Log(level string, message string, fields *LogFields) {
	if global.LOGGER == nil {
		return
	}

	fieldList := fields.Build()

	switch level {
	case "info":
		global.LOGGER.InfoContextKV(rl.ctx, message, fieldList...)
	case "warn":
		global.LOGGER.WarnContextKV(rl.ctx, message, fieldList...)
	case "error":
		global.LOGGER.ErrorContextKV(rl.ctx, message, fieldList...)
	}
}

// getLoggingConfig 获取日志配置
func getLoggingConfig() *logging.Logging {
	if global.GATEWAY != nil &&
		global.GATEWAY.Middleware != nil &&
		global.GATEWAY.Middleware.Logging != nil {
		return global.GATEWAY.Middleware.Logging
	}
	return logging.Default()
}

// shouldCaptureRequest 是否应该捕获请求体
func shouldCaptureRequest() bool {
	config := getLoggingConfig()
	return config.EnableRequest
}

// shouldCaptureResponse 是否应该捕获响应体
func shouldCaptureResponse() bool {
	config := getLoggingConfig()
	return config.EnableResponse
}

// isLoggableContentType 检查 Content-Type 是否可记录
func isLoggableContentType(contentType string) bool {
	if contentType == "" {
		return true
	}

	config := getLoggingConfig()
	contentType = strings.ToLower(contentType)

	for _, prefix := range config.LoggableContentTypes {
		if strings.HasPrefix(contentType, prefix) {
			return true
		}
	}
	return false
}

// isSkipPath 检查是否为跳过路径
func isSkipPath(path string) bool {
	config := getLoggingConfig()
	for _, skip := range config.SkipPaths {
		if path == skip {
			return true
		}
	}
	return false
}

// ============================================================================
// HTTP 日志中间件
// ============================================================================

// LoggingMiddleware HTTP 日志中间件
func LoggingMiddleware() HTTPMiddleware {
	config := getLoggingConfig()
	if !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := r.Context()

			// 跳过路径检查
			if isSkipPath(r.URL.Path) {
				wrapped := NewResponseWriter(w)
				next.ServeHTTP(wrapped, r)
				if wrapped.StatusCode() >= 400 {
					logHTTPError(ctx, r, wrapped, time.Since(start))
				}
				wrapped.Release()
				return
			}

			// 捕获请求体
			var reqBody []byte
			if shouldCaptureRequest() && r.Body != nil {
				var err error
				reqBody, err = io.ReadAll(r.Body)
				if err != nil && global.LOGGER != nil {
					global.LOGGER.ErrorContextKV(ctx, "❌ Failed to read request body",
						"path", r.URL.Path,
						"method", r.Method,
						"error", err)
				}
				r.Body = io.NopCloser(bytes.NewBuffer(reqBody))
			}

			// 包装响应
			wrapped := NewResponseWriter(w)
			if shouldCaptureResponse() {
				wrapped.EnableBodyCapture()
			}
			defer wrapped.Release()

			// 执行请求
			next.ServeHTTP(wrapped, r)

			// 记录日志
			logHTTPRequest(ctx, r, wrapped, time.Since(start), config, reqBody)
		})
	}
}

// logHTTPRequest 记录 HTTP 请求
func logHTTPRequest(ctx context.Context, r *http.Request, rw *ResponseWriter, duration time.Duration, config *logging.Logging, reqBody []byte) {
	logger := NewRequestLogger(ctx)
	masker := global.DATAMASKER

	fields := NewLogFields().
		Add(constants.LogFieldMethod, r.Method).
		Add(constants.LogFieldPath, r.URL.Path).
		Add(constants.LogFieldStatus, rw.StatusCode()).
		Add(constants.LogFieldBytes, rw.BytesWritten()).
		Add(constants.LogFieldDuration, duration.Milliseconds()).
		Add(constants.LogFieldIP, netx.GetClientIP(r)).
		Add(constants.LogFieldUserAgent, r.Header.Get(constants.HeaderUserAgent)).
		AddSlow(duration, time.Duration(config.SlowHTTPThreshold)*time.Millisecond).
		AddUserContext(ctx)

	// 请求参数
	if config.EnableRequest && r.URL.RawQuery != "" {
		fields.Add(constants.LogFieldQuery, r.URL.RawQuery)
	}

	// 请求体
	if len(reqBody) > 0 && isLoggableContentType(r.Header.Get(constants.HeaderContentType)) {
		fields.Add(constants.LogFieldRequest, masker.Mask(reqBody))
	}

	// 响应体
	if respBody := rw.GetBody(); len(respBody) > 0 && isLoggableContentType(rw.Header().Get(constants.HeaderContentType)) {
		fields.Add(constants.LogFieldResponse, masker.Mask(respBody))
	}

	level := constants.LogLevelInfo
	message := "🚀 " + constants.LogMsgHTTPRequest
	if rw.StatusCode() >= 500 {
		level = constants.LogLevelError
		message = "❌ " + constants.LogMsgHTTPRequest
	} else if rw.StatusCode() >= 400 {
		level = constants.LogLevelWarn
		message = "⚠️ " + constants.LogMsgHTTPRequest
	} else {
		message = "✅ " + constants.LogMsgHTTPRequest
	}

	logger.Log(level, message, fields)
}

// logHTTPError 记录跳过路径的错误 🚫
func logHTTPError(ctx context.Context, r *http.Request, rw *ResponseWriter, duration time.Duration) {
	logger := NewRequestLogger(ctx)
	fields := NewLogFields().
		Add(constants.LogFieldPath, r.URL.Path).
		Add(constants.LogFieldStatus, rw.StatusCode()).
		Add(constants.LogFieldDuration, duration.Milliseconds())

	logger.Log(constants.LogLevelWarn, "⚠️ "+constants.LogMsgHTTPRequestSkip, fields)
}

// ============================================================================
// gRPC 日志拦截器
// ============================================================================

// UnaryServerLoggingInterceptor gRPC 一元调用日志拦截器
func UnaryServerLoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		logGRPCUnary(ctx, info.FullMethod, req, resp, err, time.Since(start))
		return resp, err
	}
}

// StreamServerLoggingInterceptor gRPC 流式调用日志拦截器
func StreamServerLoggingInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		logGRPCStream(ss.Context(), info, err, time.Since(start))
		return err
	}
}

// logGRPCUnary 记录 gRPC 一元调用
func logGRPCUnary(ctx context.Context, method string, req, resp any, err error, duration time.Duration) {
	if global.LOGGER == nil {
		return
	}

	config := getLoggingConfig()
	logger := NewRequestLogger(ctx)
	masker := global.DATAMASKER

	fields := NewLogFields().
		Add(constants.LogFieldMethod, method).
		Add(constants.LogFieldDuration, duration.Milliseconds()).
		AddSlow(duration, time.Duration(config.SlowGRPCThreshold)*time.Millisecond).
		AddUserContext(ctx)

	if err != nil {
		st, _ := status.FromError(err)
		fields.Add(constants.LogFieldStatus, st.Code().String()).Add(constants.LogFieldError, st.Message())
		if shouldCaptureRequest() && req != nil {
			fields.Add(constants.LogFieldRequest, masker.Mask(marshalProto(req)))
		}
		logger.Log(constants.LogLevelError, "❌ "+constants.LogMsgGRPCRequestError, fields)
	} else {
		fields.Add(constants.LogFieldStatus, "OK")
		if shouldCaptureRequest() && req != nil {
			fields.Add(constants.LogFieldRequest, masker.Mask(marshalProto(req)))
		}
		if shouldCaptureResponse() && resp != nil {
			fields.Add(constants.LogFieldResponse, masker.Mask(marshalProto(resp)))
		}
		logger.Log(constants.LogLevelInfo, "✅ "+constants.LogMsgGRPCRequest, fields)
	}
}

// logGRPCStream 记录 gRPC 流式调用
func logGRPCStream(ctx context.Context, info *grpc.StreamServerInfo, err error, duration time.Duration) {
	if global.LOGGER == nil {
		return
	}

	config := getLoggingConfig()
	logger := NewRequestLogger(ctx)
	fields := NewLogFields().
		Add(constants.LogFieldMethod, info.FullMethod).
		Add(constants.LogFieldDuration, duration.Milliseconds()).
		Add(constants.LogFieldClientStream, info.IsClientStream).
		Add(constants.LogFieldServerStream, info.IsServerStream).
		AddSlow(duration, time.Duration(config.SlowStreamThreshold)*time.Millisecond).
		AddUserContext(ctx)

	if err != nil {
		st, _ := status.FromError(err)
		fields.Add(constants.LogFieldStatus, st.Code().String()).Add(constants.LogFieldError, st.Message())
		logger.Log(constants.LogLevelError, "❌ "+constants.LogMsgGRPCStreamError, fields)
	} else {
		fields.Add(constants.LogFieldStatus, "OK")
		logger.Log(constants.LogLevelInfo, "📊 "+constants.LogMsgGRPCStream, fields)
	}
}

// marshalProto 序列化 protobuf 消息
func marshalProto(data any) []byte {
	if data == nil {
		return nil
	}
	jsonBytes, _ := json.Marshal(data)
	return jsonBytes
}
