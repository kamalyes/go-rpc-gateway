/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-07 22:05:32
 * @FilePath: \go-rpc-gateway\middleware\recovery.go
 * @Description: HTTP Recovery 中间件 - 处理 panic 恢复（增强版）
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"

	"github.com/kamalyes/go-config/pkg/recovery"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	commonapis "github.com/kamalyes/go-rpc-gateway/proto"
	"github.com/kamalyes/go-toolbox/pkg/netx"
)

// RecoveryMiddleware 恢复中间件 - 处理 panic 恢复
func RecoveryMiddleware(cfg *recovery.Recovery) HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					handlePanicRecovery(w, r, err, cfg)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// handlePanicRecovery 处理 panic 恢复（增强版）
func handlePanicRecovery(w http.ResponseWriter, r *http.Request, err interface{}, config *recovery.Recovery) {
	ctx := r.Context()

	// 获取堆栈信息
	var stackTrace string
	if config.EnableStack {
		buf := make([]byte, config.StackSize)
		n := runtime.Stack(buf, false)
		stackTrace = string(buf[:n])
	}

	// 记录 panic 信息（使用新的 context-aware API）
	logPanicError(ctx, r, err, stackTrace, config)

	// 自定义恢复处理
	if config.RecoveryHandler != nil {
		config.RecoveryHandler(w, r, err)
		return
	}

	// 默认处理：设置响应
	setPanicErrorResponse(w, ctx, err, stackTrace, config)
}

// logPanicError 记录 panic 错误日志
func logPanicError(ctx context.Context, r *http.Request, err any, stackTrace string, config *recovery.Recovery) {
	fields := []any{
		constants.LogFieldError, err,
		constants.LogFieldMethod, r.Method,
		constants.LogFieldPath, r.URL.String(),
		constants.LogFieldRemoteAddr, netx.GetClientIP(r),
		constants.LogFieldUserAgent, r.UserAgent(),
	}

	// 添加堆栈信息
	if config.EnableStack && stackTrace != "" {
		fields = append(fields, constants.LogFieldStackTrace, stackTrace)
	}

	// 添加用户上下文信息
	traceInfo := GetCachedTraceInfo(ctx)
	if traceInfo.UserID != "" {
		fields = append(fields, constants.LogFieldUserID, traceInfo.UserID)
	}
	if traceInfo.TenantID != "" {
		fields = append(fields, constants.LogFieldTenantID, traceInfo.TenantID)
	}
	if traceInfo.TraceID != "" {
		fields = append(fields, constants.LogFieldTraceID, traceInfo.TraceID)
	}
	if traceInfo.RequestID != "" {
		fields = append(fields, constants.LogFieldRequestID, traceInfo.RequestID)
	}

	global.LOGGER.ErrorContextKV(ctx, constants.LogMsgPanicRecovered, fields...)
}

// setPanicErrorResponse 设置 panic 错误响应
func setPanicErrorResponse(w http.ResponseWriter, ctx context.Context, err interface{}, stackTrace string, config *recovery.Recovery) {
	// 设置响应头
	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
	setTraceHeaders(w, ctx)
	w.WriteHeader(http.StatusInternalServerError)

	// 构建错误消息
	message := config.ErrorMessage
	if message == "" {
		message = constants.MsgInternalError
	}

	// 构建响应对象
	result := &commonapis.Result{
		Code:   int32(http.StatusInternalServerError),
		Error:  message,
		Status: commonapis.StatusCode_Internal,
	}

	// 调试模式：添加详细错误信息
	if config.EnableDebug {
		debugInfo := fmt.Sprintf("%v", err)
		if config.EnableStack && stackTrace != "" {
			debugInfo += fmt.Sprintf(" | Stack: %s", stackTrace)
		}
		result.Error = fmt.Sprintf("%s | Debug: %s", message, debugInfo)
	}

	// 写入响应
	if err := json.NewEncoder(w).Encode(result); err != nil && global.LOGGER != nil {
		global.LOGGER.ErrorContext(ctx, constants.LogMsgWriteResponseError, constants.LogFieldError, err)
	}
}

// setTraceHeaders 设置追踪头信息
func setTraceHeaders(w http.ResponseWriter, ctx context.Context) {
	traceInfo := GetCachedTraceInfo(ctx)
	if traceInfo.TraceID != "" {
		w.Header().Set(constants.HeaderXTraceID, traceInfo.TraceID)
	}
	if traceInfo.RequestID != "" {
		w.Header().Set(constants.HeaderXRequestID, traceInfo.RequestID)
	}
}
