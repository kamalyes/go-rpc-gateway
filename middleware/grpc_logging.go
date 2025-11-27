/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-28
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-28 01:31:02
 * @FilePath: \engine-im-service\go-rpc-gateway\middleware\grpc_logging.go
 * @Description: gRPC 日志拦截器
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"context"
	"time"

	"github.com/kamalyes/go-rpc-gateway/global"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryServerLoggingInterceptor gRPC 一元调用日志拦截器
func UnaryServerLoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		// 提取 metadata 中的 TraceID 和 RequestID
		md, _ := metadata.FromIncomingContext(ctx)
		traceID := getMetadataValue(md, "x-trace-id")
		requestID := getMetadataValue(md, "x-request-id")

		// 调用处理器
		resp, err := handler(ctx, req)

		// 记录日志
		duration := time.Since(start)
		statusCode := "OK"
		if err != nil {
			st, _ := status.FromError(err)
			statusCode = st.Code().String()
		}

		if global.LOGGER != nil {
			global.LOGGER.InfoContextKV(ctx, "gRPC Request",
				"method", info.FullMethod,
				"duration_ms", duration.Milliseconds(),
				"status", statusCode,
				"trace_id", traceID,
				"request_id", requestID,
			)
		}

		return resp, err
	}
}

// StreamServerLoggingInterceptor gRPC 流式调用日志拦截器
func StreamServerLoggingInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		ctx := ss.Context()

		// 提取 metadata 中的 TraceID 和 RequestID
		md, _ := metadata.FromIncomingContext(ctx)
		traceID := getMetadataValue(md, "x-trace-id")
		requestID := getMetadataValue(md, "x-request-id")

		// 调用处理器
		err := handler(srv, ss)

		// 记录日志
		duration := time.Since(start)
		statusCode := "OK"
		if err != nil {
			st, _ := status.FromError(err)
			statusCode = st.Code().String()
		}

		if global.LOGGER != nil {
			global.LOGGER.InfoContextKV(ctx, "gRPC Stream Request",
				"method", info.FullMethod,
				"duration_ms", duration.Milliseconds(),
				"status", statusCode,
				"is_client_stream", info.IsClientStream,
				"is_server_stream", info.IsServerStream,
				"trace_id", traceID,
				"request_id", requestID,
			)
		}

		return err
	}
}

// getMetadataValue 从 metadata 中获取值
func getMetadataValue(md metadata.MD, key string) string {
	if values := md.Get(key); len(values) > 0 {
		return values[0]
	}
	return ""
}
