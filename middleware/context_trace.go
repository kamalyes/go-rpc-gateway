/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-29 10:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-29 10:00:00
 * @FilePath: \go-rpc-gateway\middleware\context_trace.go
 * @Description: 统一的 Context 追踪中间件 - 实现 HTTP → gRPC → Service → Repository 全链路追踪
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"net/http"

	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ContextTraceMiddleware HTTP 层统一的 context 追踪中间件
// 职责：
// 1. 从 HTTP Header 提取或生成 trace_id 和 request_id
// 2. 将这些值存入 context（使用 go-logger 的标准 ContextKey）
// 3. 设置响应头返回 trace_id 和 request_id
func ContextTraceMiddleware() HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// 1. 提取或生成 TraceID
			traceID := r.Header.Get(constants.HeaderXTraceID)
			if traceID == "" {
				// 尝试从 OpenTelemetry span 获取
				if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
					traceID = spanCtx.TraceID().String()
				}
			}
			if traceID == "" {
				// 生成新的 TraceID
				traceID = logger.GenerateTraceID()
			}

			// 2. 提取或生成 RequestID
			requestID := r.Header.Get(constants.HeaderXRequestID)
			if requestID == "" {
				requestID = logger.GenerateRequestID()
			}

			// 3. 将 TraceID 和 RequestID 存入 context（使用 go-logger 的标准方式）
			ctx = logger.WithTraceID(ctx, traceID)
			ctx = logger.WithRequestID(ctx, requestID)

			// 4. 可选：提取其他上下文信息
			if userID := r.Header.Get("X-User-ID"); userID != "" {
				ctx = logger.WithUserID(ctx, userID)
			}
			if tenantID := r.Header.Get("X-Tenant-ID"); tenantID != "" {
				ctx = logger.WithTenantID(ctx, tenantID)
			}
			if sessionID := r.Header.Get("X-Session-ID"); sessionID != "" {
				ctx = logger.WithSessionID(ctx, sessionID)
			}

			// 5. 设置响应头（便于客户端追踪）
			w.Header().Set(constants.HeaderXTraceID, traceID)
			w.Header().Set(constants.HeaderXRequestID, requestID)

			// 6. 继续处理请求
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UnaryServerContextInterceptor gRPC Server 一元调用 context 注入拦截器
// 职责：
// 1. 从 gRPC metadata 提取 trace_id 和 request_id
// 2. 将这些值存入 context（使用 go-logger 的标准 ContextKey）
// 3. 确保后续 Service 和 Repository 层能够获取到这些值
func UnaryServerContextInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 增强 context
		ctx = enrichContextFromMetadata(ctx)

		// 调用处理器
		return handler(ctx, req)
	}
}

// StreamServerContextInterceptor gRPC Server 流式调用 context 注入拦截器
func StreamServerContextInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()

		// 增强 context
		ctx = enrichContextFromMetadata(ctx)

		// 包装 ServerStream 以使用增强后的 context
		wrappedStream := &contextWrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		return handler(srv, wrappedStream)
	}
}

// enrichContextFromMetadata 从 gRPC metadata 提取追踪信息并存入 context
func enrichContextFromMetadata(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	// 提取 TraceID
	traceID := getFirstMetadataValue(md, "x-trace-id")
	if traceID == "" {
		// 尝试从 OpenTelemetry span 获取
		if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
			traceID = spanCtx.TraceID().String()
		}
	}
	if traceID == "" {
		traceID = logger.GenerateTraceID()
	}
	ctx = logger.WithTraceID(ctx, traceID)

	// 提取 RequestID
	requestID := getFirstMetadataValue(md, "x-request-id")
	if requestID == "" {
		requestID = logger.GenerateRequestID()
	}
	ctx = logger.WithRequestID(ctx, requestID)

	// 提取其他可选字段
	if userID := getFirstMetadataValue(md, "x-user-id"); userID != "" {
		ctx = logger.WithUserID(ctx, userID)
	}
	if tenantID := getFirstMetadataValue(md, "x-tenant-id"); tenantID != "" {
		ctx = logger.WithTenantID(ctx, tenantID)
	}
	if sessionID := getFirstMetadataValue(md, "x-session-id"); sessionID != "" {
		ctx = logger.WithSessionID(ctx, sessionID)
	}

	return ctx
}

// getFirstMetadataValue 从 metadata 获取第一个值
func getFirstMetadataValue(md metadata.MD, key string) string {
	if values := md.Get(key); len(values) > 0 {
		return values[0]
	}
	return ""
}

// contextWrappedServerStream 包装 grpc.ServerStream 以支持自定义 context
type contextWrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context 返回增强后的 context
func (w *contextWrappedServerStream) Context() context.Context {
	return w.ctx
}

// UnaryClientContextInterceptor gRPC Client 一元调用拦截器
// 职责：将 context 中的 trace 信息传递到 gRPC metadata
func UnaryClientContextInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// 将 context 中的 trace 信息注入到 outgoing metadata
		ctx = injectTraceToOutgoingContext(ctx)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// StreamClientContextInterceptor gRPC Client 流式调用拦截器
func StreamClientContextInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		// 将 context 中的 trace 信息注入到 outgoing metadata
		ctx = injectTraceToOutgoingContext(ctx)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

// injectTraceToOutgoingContext 将 context 中的 trace 信息注入到 outgoing gRPC metadata
func injectTraceToOutgoingContext(ctx context.Context) context.Context {
	pairs := make([]string, 0, 10)

	// 提取 trace 信息
	if traceID := logger.GetTraceID(ctx); traceID != "" {
		pairs = append(pairs, "x-trace-id", traceID)
	}
	if requestID := logger.GetRequestID(ctx); requestID != "" {
		pairs = append(pairs, "x-request-id", requestID)
	}
	if userID := logger.GetUserID(ctx); userID != "" {
		pairs = append(pairs, "x-user-id", userID)
	}
	if tenantID := logger.GetTenantID(ctx); tenantID != "" {
		pairs = append(pairs, "x-tenant-id", tenantID)
	}
	if sessionID := logger.GetSessionID(ctx); sessionID != "" {
		pairs = append(pairs, "x-session-id", sessionID)
	}

	if len(pairs) > 0 {
		md := metadata.Pairs(pairs...)
		// 合并已有的 outgoing metadata
		if existingMD, ok := metadata.FromOutgoingContext(ctx); ok {
			md = metadata.Join(existingMD, md)
		}
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	return ctx
}

// GetTraceInfoFromContext 从 context 获取追踪信息（用于构建响应）
func GetTraceInfoFromContext(ctx context.Context) (traceID, requestID string) {
	return logger.GetTraceID(ctx), logger.GetRequestID(ctx)
}

// ExtractAllTraceFields 从 context 提取所有追踪字段（用于日志或响应）
func ExtractAllTraceFields(ctx context.Context) map[string]string {
	fields := make(map[string]string)

	if v := logger.GetTraceID(ctx); v != "" {
		fields["trace_id"] = v
	}
	if v := logger.GetRequestID(ctx); v != "" {
		fields["request_id"] = v
	}
	if v := logger.GetUserID(ctx); v != "" {
		fields["user_id"] = v
	}
	if v := logger.GetTenantID(ctx); v != "" {
		fields["tenant_id"] = v
	}
	if v := logger.GetSessionID(ctx); v != "" {
		fields["session_id"] = v
	}

	return fields
}
