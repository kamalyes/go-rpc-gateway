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

// TraceInfo 追踪信息缓存结构
type TraceInfo struct {
	TraceID   string
	RequestID string
	UserID    string
	TenantID  string
	SessionID string
	Timezone  string
}

type traceInfoKey struct{}

// ContextTraceMiddleware HTTP 层统一的 context 追踪中间件
// 职责：
// 1. 从 HTTP Header 提取或生成 trace_id 和 request_id
// 2. 将这些值存入 context（使用 go-logger 的标准 ContextKey）
// 3. 设置响应头返回 trace_id 和 request_id
func ContextTraceMiddleware() HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// 提取或生成 TraceID 和 RequestID
			traceID := extractOrGenerateTraceID(ctx, r.Header.Get(constants.HeaderXTraceID))
			requestID := extractOrGenerateRequestID(r.Header.Get(constants.HeaderXRequestID))

			// 将 TraceID 和 RequestID 存入 context（使用 go-logger 的标准方式）
			ctx = WithTraceID(ctx, traceID)
			ctx = WithRequestID(ctx, requestID)

			// 提取其他上下文信息
			userID := r.Header.Get(constants.HeaderXUserID)
			tenantID := r.Header.Get(constants.HeaderXTenantID)
			sessionID := r.Header.Get(constants.HeaderXSessionID)
			timezone := r.Header.Get(constants.HeaderXTimezone)

			ctx = WithUserID(ctx, userID)
			ctx = WithTenantID(ctx, tenantID)
			ctx = WithSessionID(ctx, sessionID)
			ctx = WithTimezone(ctx, timezone)

			// 缓存 TraceInfo（避免后续中间件重复查找）
			ctx = context.WithValue(ctx, traceInfoKey{}, &TraceInfo{
				TraceID:   traceID,
				RequestID: requestID,
				UserID:    userID,
				TenantID:  tenantID,
				SessionID: sessionID,
				Timezone:  timezone,
			})

			// 5. 设置响应头（便于客户端追踪）
			w.Header().Set(constants.HeaderXTraceID, traceID)
			w.Header().Set(constants.HeaderXRequestID, requestID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetCachedTraceInfo 获取缓存的追踪信息
// 优先从 context 中获取已缓存的 TraceInfo，如果不存在则从 logger 中提取并构建
func GetCachedTraceInfo(ctx context.Context) *TraceInfo {
	if info, ok := ctx.Value(traceInfoKey{}).(*TraceInfo); ok {
		return info
	}

	// 回退：从 logger 提取
	return &TraceInfo{
		TraceID:   logger.GetTraceID(ctx),
		RequestID: logger.GetRequestID(ctx),
		UserID:    logger.GetUserID(ctx),
		TenantID:  logger.GetTenantID(ctx),
		SessionID: logger.GetSessionID(ctx),
		Timezone:  logger.GetTimezone(ctx),
	}
}

// ============================================================================
// gRPC Server 拦截器
// ============================================================================

// UnaryServerContextInterceptor gRPC Server 一元调用拦截器
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

// StreamServerContextInterceptor gRPC Server 流式调用拦截器
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
		// 没有 metadata 时，生成新的 TraceID 和 RequestID
		md = metadata.MD{}
	}

	// 提取 TraceID & RequestID
	traceID := extractOrGenerateTraceID(ctx, getFirstMetadataValue(md, constants.MetadataTraceID))
	requestID := extractOrGenerateRequestID(getFirstMetadataValue(md, constants.MetadataRequestID))

	// 提取其他可选字段
	userID := getFirstMetadataValue(md, constants.MetadataUserID)
	sessionID := getFirstMetadataValue(md, constants.MetadataSessionID)
	tenantID := getFirstMetadataValue(md, constants.MetadataTenantID)
	timezone := getFirstMetadataValue(md, constants.MetadataTimezone)

	ctx = WithRequestID(ctx, requestID)
	ctx = WithTraceID(ctx, traceID)
	ctx = WithUserID(ctx, userID)
	ctx = WithTenantID(ctx, tenantID)
	ctx = WithSessionID(ctx, sessionID)
	ctx = WithTimezone(ctx, timezone)

	return ctx
}

// setResponseMetadata 设置 gRPC 响应 metadata（与 HTTP 的 w.Header().Set 对应）
func setResponseMetadata(ctx context.Context) {
	traceInfo := GetCachedTraceInfo(ctx)

	md := metadata.Pairs(
		constants.MetadataTraceID, traceInfo.TraceID,
		constants.MetadataRequestID, traceInfo.RequestID,
	)

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

// ============================================================================
// gRPC Client 拦截器
// ============================================================================

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
	traceInfo := GetCachedTraceInfo(ctx)

	// 直接注入所有字段，空值也可以传递
	md := metadata.Pairs(
		constants.MetadataTraceID, traceInfo.TraceID,
		constants.MetadataRequestID, traceInfo.RequestID,
		constants.MetadataUserID, traceInfo.UserID,
		constants.MetadataTenantID, traceInfo.TenantID,
		constants.MetadataSessionID, traceInfo.SessionID,
		constants.MetadataTimezone, traceInfo.Timezone,
	)

	// 合并已有的 outgoing metadata
	if existingMD, ok := metadata.FromOutgoingContext(ctx); ok {
		md = metadata.Join(existingMD, md)
	}
	return metadata.NewOutgoingContext(ctx, md)
}

// ============================================================================
// 工具函数
// ============================================================================

// extractOrGenerateTraceID 提取或生成 TraceID（优先级：参数 > OpenTelemetry > 生成）
func extractOrGenerateTraceID(ctx context.Context, traceID string) string {
	if traceID != "" {
		return traceID
	}

	// 尝试从 OpenTelemetry span 获取
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.IsValid() {
		return spanCtx.TraceID().String()
	}

	return logger.GenerateTraceID()
}

// extractOrGenerateRequestID 提取或生成 RequestID
func extractOrGenerateRequestID(requestID string) string {
	if requestID != "" {
		return requestID
	}
	return logger.GenerateRequestID()
}

// GetTraceInfoFromContext 从 context 获取追踪信息（用于构建响应）
func GetTraceInfoFromContext(ctx context.Context) (traceID, requestID string) {
	traceInfo := GetCachedTraceInfo(ctx)
	return traceInfo.TraceID, traceInfo.RequestID
}

// ============================================================================
// 通用工具方法，供其他组件使用
// ============================================================================

// GetTraceID 从 context 获取 TraceID
func GetTraceID(ctx context.Context) string {
	traceInfo := GetCachedTraceInfo(ctx)
	return traceInfo.TraceID
}

// GetRequestID 从 context 获取 RequestID
func GetRequestID(ctx context.Context) string {
	traceInfo := GetCachedTraceInfo(ctx)
	return traceInfo.RequestID
}

// GetUserID 从 context 获取 UserID
func GetUserID(ctx context.Context) string {
	traceInfo := GetCachedTraceInfo(ctx)
	return traceInfo.UserID
}

// GetTenantID 从 context 获取 TenantID
func GetTenantID(ctx context.Context) string {
	traceInfo := GetCachedTraceInfo(ctx)
	return traceInfo.TenantID
}

// GetSessionID 从 context 获取 SessionID
func GetSessionID(ctx context.Context) string {
	traceInfo := GetCachedTraceInfo(ctx)
	return traceInfo.SessionID
}

// GetTimezone 从 context 获取 Timezone
func GetTimezone(ctx context.Context) string {
	traceInfo := GetCachedTraceInfo(ctx)
	return traceInfo.Timezone
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

// WithTimezone 将 Timezone 设置到 context
func WithTimezone(ctx context.Context, timezone string) context.Context {
	return logger.WithTimezone(ctx, timezone)
}
