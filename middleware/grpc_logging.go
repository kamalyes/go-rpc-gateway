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
	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/global"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"time"
)

// UnaryServerLoggingInterceptor gRPC 一元调用日志拦截器
// 注意：context 注入请使用 UnaryServerContextInterceptor，此拦截器只负责日志记录
func UnaryServerLoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		// 调用处理器
		resp, err := handler(ctx, req)

		// 记录日志（从 context 中提取 trace 信息）
		duration := time.Since(start)

		if global.LOGGER != nil {
			if err != nil {
				// 错误情况：记录详细错误信息
				st, _ := status.FromError(err)
				fields := []interface{}{
					"method", info.FullMethod,
					"duration_ms", duration.Milliseconds(),
					"status", st.Code().String(),
					"error", st.Message(),
				}

				// 添加用户信息（如果存在）
				if userID := logger.GetUserID(ctx); userID != "" {
					fields = append(fields, "user_id", userID)
				}
				if tenantID := logger.GetTenantID(ctx); tenantID != "" {
					fields = append(fields, "tenant_id", tenantID)
				}

				global.LOGGER.ErrorContextKV(ctx, "gRPC Request Error", fields...)
			} else {
				// 成功情况：记录基本信息
				fields := []interface{}{
					"method", info.FullMethod,
					"duration_ms", duration.Milliseconds(),
					"status", "OK",
				}

				// 性能告警：如果请求耗时过长
				if duration.Milliseconds() > 1000 {
					fields = append(fields, "slow_request", true)
				}

				global.LOGGER.InfoContextKV(ctx, "gRPC Request", fields...)
			}
		}

		return resp, err
	}
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
