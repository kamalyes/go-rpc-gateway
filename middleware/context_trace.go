/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-29 10:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 15:08:15
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
			if userID := r.Header.Get(constants.HeaderXUserID); userID != "" {
				ctx = logger.WithUserID(ctx, userID)
			}
			if tenantID := r.Header.Get(constants.HeaderXTenantID); tenantID != "" {
				ctx = logger.WithTenantID(ctx, tenantID)
			}
			if sessionID := r.Header.Get(constants.HeaderXSessionID); sessionID != "" {
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
// 3. 设置响应 metadata 返回 trace_id 和 request_id（与 HTTP 保持一致）
// 4. 确保后续 Service 和 Repository 层能够获取到这些值
func UnaryServerContextInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 增强 context
		ctx = enrichContextFromMetadata(ctx)

		// 设置响应 metadata（必须在 handler 调用前）
		setResponseMetadata(ctx)

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

		// 设置响应 metadata（必须在 handler 调用前）
		setResponseMetadata(ctx)

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
	traceID := getFirstMetadataValue(md, constants.MetadataTraceID)
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
	requestID := getFirstMetadataValue(md, constants.MetadataRequestID)
	if requestID == "" {
		requestID = logger.GenerateRequestID()
	}
	ctx = logger.WithRequestID(ctx, requestID)

	// 提取其他可选字段
	if userID := getFirstMetadataValue(md, constants.MetadataUserID); userID != "" {
		ctx = logger.WithUserID(ctx, userID)
	}
	if tenantID := getFirstMetadataValue(md, constants.MetadataTenantID); tenantID != "" {
		ctx = logger.WithTenantID(ctx, tenantID)
	}
	if sessionID := getFirstMetadataValue(md, constants.MetadataSessionID); sessionID != "" {
		ctx = logger.WithSessionID(ctx, sessionID)
	}

	return ctx
}

// setResponseMetadata 设置 gRPC 响应 metadata（与 HTTP 的 w.Header().Set 对应）
func setResponseMetadata(ctx context.Context) {
	md := metadata.Pairs()

	// 添加 trace_id
	if traceID := logger.GetTraceID(ctx); traceID != "" {
		md.Set(constants.MetadataTraceID, traceID)
	}

	// 添加 request_id
	if requestID := logger.GetRequestID(ctx); requestID != "" {
		md.Set(constants.MetadataRequestID, requestID)
	}

	// 发送 metadata（忽略错误，因为可能已经发送过）
	if len(md) > 0 {
		grpc.SetHeader(ctx, md)
	}
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
		pairs = append(pairs, constants.MetadataTraceID, traceID)
	}
	if requestID := logger.GetRequestID(ctx); requestID != "" {
		pairs = append(pairs, constants.MetadataRequestID, requestID)
	}
	if userID := logger.GetUserID(ctx); userID != "" {
		pairs = append(pairs, constants.MetadataUserID, userID)
	}
	if tenantID := logger.GetTenantID(ctx); tenantID != "" {
		pairs = append(pairs, constants.MetadataTenantID, tenantID)
	}
	if sessionID := logger.GetSessionID(ctx); sessionID != "" {
		pairs = append(pairs, constants.MetadataSessionID, sessionID)
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
		fields[constants.LogFieldTraceID] = v
	}
	if v := logger.GetRequestID(ctx); v != "" {
		fields[constants.LogFieldRequestID] = v
	}
	if v := logger.GetUserID(ctx); v != "" {
		fields[constants.LogFieldUserID] = v
	}
	if v := logger.GetTenantID(ctx); v != "" {
		fields[constants.LogFieldTenantID] = v
	}
	if v := logger.GetSessionID(ctx); v != "" {
		fields[constants.LogFieldSessionID] = v
	}

	return fields
}

// ============================================================================
// 通用工具方法，供其他组件使用
// ============================================================================

// GetTraceID 从 context 获取 TraceID
func GetTraceID(ctx context.Context) string {
	return logger.GetTraceID(ctx)
}

// GetRequestID 从 context 获取 RequestID
func GetRequestID(ctx context.Context) string {
	return logger.GetRequestID(ctx)
}

// GetUserID 从 context 获取 UserID
func GetUserID(ctx context.Context) string {
	return logger.GetUserID(ctx)
}

// GetTenantID 从 context 获取 TenantID
func GetTenantID(ctx context.Context) string {
	return logger.GetTenantID(ctx)
}

// GetSessionID 从 context 获取 SessionID
func GetSessionID(ctx context.Context) string {
	return logger.GetSessionID(ctx)
}

// WithTraceID 将 TraceID 设置到 context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return logger.WithTraceID(ctx, traceID)
}

// WithRequestID 将 RequestID 设置到 context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return logger.WithRequestID(ctx, requestID)
}

// WithUserID 将 UserID 设置到 context
func WithUserID(ctx context.Context, userID string) context.Context {
	return logger.WithUserID(ctx, userID)
}

// WithTenantID 将 TenantID 设置到 context
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return logger.WithTenantID(ctx, tenantID)
}

// WithSessionID 将 SessionID 设置到 context
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return logger.WithSessionID(ctx, sessionID)
}

// GenerateTraceID 生成新的 TraceID
func GenerateTraceID() string {
	return logger.GenerateTraceID()
}

// GenerateRequestID 生成新的 RequestID
func GenerateRequestID() string {
	return logger.GenerateRequestID()
}

// InjectTraceToHTTPHeader 将 context 中的 trace 信息注入到 HTTP Header
func InjectTraceToHTTPHeader(ctx context.Context, header http.Header) {
	if traceID := GetTraceID(ctx); traceID != "" {
		header.Set(constants.HeaderXTraceID, traceID)
	}
	if requestID := GetRequestID(ctx); requestID != "" {
		header.Set(constants.HeaderXRequestID, requestID)
	}
	if userID := GetUserID(ctx); userID != "" {
		header.Set(constants.HeaderXUserID, userID)
	}
	if tenantID := GetTenantID(ctx); tenantID != "" {
		header.Set(constants.HeaderXTenantID, tenantID)
	}
	if sessionID := GetSessionID(ctx); sessionID != "" {
		header.Set(constants.HeaderXSessionID, sessionID)
	}
}

// ExtractTraceFromHTTPHeader 从 HTTP Header 提取 trace 信息到 context
func ExtractTraceFromHTTPHeader(ctx context.Context, header http.Header) context.Context {
	if traceID := header.Get(constants.HeaderXTraceID); traceID != "" {
		ctx = WithTraceID(ctx, traceID)
	}
	if requestID := header.Get(constants.HeaderXRequestID); requestID != "" {
		ctx = WithRequestID(ctx, requestID)
	}
	if userID := header.Get(constants.HeaderXUserID); userID != "" {
		ctx = WithUserID(ctx, userID)
	}
	if tenantID := header.Get(constants.HeaderXTenantID); tenantID != "" {
		ctx = WithTenantID(ctx, tenantID)
	}
	if sessionID := header.Get(constants.HeaderXSessionID); sessionID != "" {
		ctx = WithSessionID(ctx, sessionID)
	}
	return ctx
}
