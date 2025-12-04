/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-28
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-29 12:00:00
 * @FilePath: \go-rpc-gateway\middleware\grpc_logging.go
 * @Description: gRPC 日志拦截器（纯日志功能，context 注入请使用 context_trace.go）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"context"
	"encoding/json"
	"time"

	"github.com/kamalyes/go-config/pkg/logging"
	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/global"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryServerLoggingInterceptor gRPC 一元调用日志拦截器
// 注意：context 注入请使用 UnaryServerContextInterceptor，此拦截器只负责日志记录
func UnaryServerLoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		// 调用处理器
		resp, err := handler(ctx, req)

		// 记录日志
		duration := time.Since(start)
		if global.LOGGER != nil {
			logGRPCRequest(ctx, info, req, resp, err, duration)
		}

		return resp, err
	}
}

// logGRPCRequest 记录 gRPC 请求日志（提取函数降低复杂度）
func logGRPCRequest(ctx context.Context, info *grpc.UnaryServerInfo, req, resp interface{}, err error, duration time.Duration) {
	// 获取日志配置
	enableRequest, enableResponse := getGRPCLoggingConfig()

	// 构建基础字段
	fields := buildGRPCBaseFields(ctx, info.FullMethod, duration)

	if err != nil {
		// 错误情况
		logGRPCError(ctx, fields, req, err, enableRequest)
	} else {
		// 成功情况
		logGRPCSuccess(ctx, fields, req, resp, duration, enableRequest, enableResponse)
	}
}

// getGRPCLoggingConfig 获取日志配置
func getGRPCLoggingConfig() (enableRequest, enableResponse bool) {
	if global.GATEWAY != nil && global.GATEWAY.Middleware != nil && global.GATEWAY.Middleware.Logging != nil {
		enableRequest = global.GATEWAY.Middleware.Logging.EnableRequest
		enableResponse = global.GATEWAY.Middleware.Logging.EnableResponse
	}
	return
}

// buildGRPCBaseFields 构建基础日志字段
func buildGRPCBaseFields(ctx context.Context, method string, duration time.Duration) []interface{} {
	fields := []interface{}{
		"method", method,
		"duration_ms", duration.Milliseconds(),
	}

	// 添加用户信息（如果存在）
	if userID := logger.GetUserID(ctx); userID != "" {
		fields = append(fields, "user_id", userID)
	}
	if tenantID := logger.GetTenantID(ctx); tenantID != "" {
		fields = append(fields, "tenant_id", tenantID)
	}

	return fields
}

// logGRPCError 记录 gRPC 错误日志
func logGRPCError(ctx context.Context, fields []interface{}, req interface{}, err error, enableRequest bool) {
	st, _ := status.FromError(err)
	fields = append(fields,
		"status", st.Code().String(),
		"error", st.Message(),
	)

	// 添加请求体（带脱敏）
	if enableRequest && req != nil {
		if safeReq := marshalAndMaskGRPC(req); safeReq != "" {
			fields = append(fields, "request", safeReq)
		}
	}

	global.LOGGER.ErrorContextKV(ctx, "gRPC Request Error", fields...)
}

// logGRPCSuccess 记录 gRPC 成功日志
func logGRPCSuccess(ctx context.Context, fields []interface{}, req, resp interface{}, duration time.Duration, enableRequest, enableResponse bool) {
	fields = append(fields, "status", "OK")
	if duration.Milliseconds() > 1000 {
		fields = append(fields, "slow_request", true)
	}
	if enableRequest && req != nil {
		if safeReq := marshalAndMaskGRPC(req); safeReq != "" {
			fields = append(fields, "request", safeReq)
		}
	}
	if enableResponse && resp != nil {
		if safeResp := marshalAndMaskGRPC(resp); safeResp != "" {
			fields = append(fields, "response", safeResp)
		}
	}
	global.LOGGER.InfoContextKV(ctx, "gRPC Request", fields...)
}

// marshalAndMaskGRPC 序列化并脱敏 gRPC 消息
func marshalAndMaskGRPC(data interface{}) string {
	if data == nil {
		return ""
	}

	config := getLoggingConfig()
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return ""
	}

	maxSize := getMaxBodySize(config)
	if len(jsonBytes) > maxSize {
		jsonBytes = jsonBytes[:maxSize]
	}

	// 复用 HTTP 的脱敏函数
	return maskSensitiveData(jsonBytes, config)
}

// getLoggingConfig 获取日志配置
func getLoggingConfig() *logging.Logging {
	if global.GATEWAY != nil && global.GATEWAY.Middleware != nil && global.GATEWAY.Middleware.Logging != nil {
		return global.GATEWAY.Middleware.Logging
	}
	return logging.Default()
}

// StreamServerLoggingInterceptor gRPC 流式调用日志拦截器
// 注意：context 注入请使用 StreamServerContextInterceptor，此拦截器只负责日志记录
func StreamServerLoggingInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		ctx := ss.Context()

		// 调用处理器
		err := handler(srv, ss)

		// 记录日志（从 context 中提取 trace 信息）
		duration := time.Since(start)

		if global.LOGGER != nil {
			fields := []interface{}{
				"method", info.FullMethod,
				"duration_ms", duration.Milliseconds(),
				"is_client_stream", info.IsClientStream,
				"is_server_stream", info.IsServerStream,
			}

			if err != nil {
				st, _ := status.FromError(err)
				fields = append(fields,
					"status", st.Code().String(),
					"error", st.Message(),
				)
				global.LOGGER.ErrorContextKV(ctx, "gRPC Stream Request Error", fields...)
			} else {
				fields = append(fields, "status", "OK")
				// 性能告警
				if duration.Milliseconds() > 5000 {
					fields = append(fields, "slow_stream", true)
				}
				global.LOGGER.InfoContextKV(ctx, "gRPC Stream Request", fields...)
			}
		}

		return err
	}
}
