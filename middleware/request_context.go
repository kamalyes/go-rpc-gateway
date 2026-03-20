/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-29 10:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-23 11:53:54
 * @FilePath: \go-rpc-gateway\middleware\request_context.go
 * @Description: 统一的请求上下文中间件 - 实现 HTTP → gRPC → Service → Repository 全链路上下文传递
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"net/http"

	gccommon "github.com/kamalyes/go-config/pkg/common"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/contextx"
	"github.com/kamalyes/go-toolbox/pkg/osx"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// RequestCommonMeta 请求公共元信息
type RequestCommonMeta struct {
	TraceID       string `json:"traceId" header:"X-Trace-Id"`
	RequestID     string `json:"requestId" header:"X-Request-Id"`
	UserID        string `json:"userId" header:"X-User-ID"`
	TenantID      string `json:"tenantId" header:"X-Tenant-ID"`
	SessionID     string `json:"sessionId" header:"X-Session-ID"`
	Timezone      string `json:"timezone" header:"X-Timezone"`
	Timestamp     string `json:"timestamp" header:"X-Timestamp"`
	Signature     string `json:"signature" header:"X-Signature"`
	Authorization string `json:"authorization" header:"Authorization"`
	AccessKey     string `json:"accessKey" header:"X-Access-Key"`
	AppID         string `json:"appId" header:"X-App-Id"`
	DeviceID      string `json:"deviceId" header:"X-Device-Id"`
	AppVersion    string `json:"appVersion" header:"X-App-Version"`
	Platform      string `json:"platform" header:"X-Platform"`
	Nonce         string `json:"nonce" header:"X-Nonce"`
}

type requestCommonMetaKey struct{}

// RequestContextMiddleware HTTP 层统一的请求上下文中间件
// 职责：
// 1. 从 HTTP Header 提取或生成 trace_id 和 request_id
// 2. 将这些值存入 context（使用 go-logger 的标准 ContextKey）
// 3. 设置响应头返回 trace_id 和 request_id
func RequestContextMiddleware() HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			requestContext := global.GATEWAY.RequestContext
			requestCommonMeta := &RequestCommonMeta{
				TraceID:       extractOrGenerateTraceID(ctx, gccommon.ExtractAttribute(r, requestContext.TraceIDSources)),
				RequestID:     extractOrGenerateRequestID(gccommon.ExtractAttribute(r, requestContext.RequestIDSources)),
				UserID:        gccommon.ExtractAttribute(r, requestContext.UserIDSources),
				TenantID:      gccommon.ExtractAttribute(r, requestContext.TenantIDSources),
				SessionID:     gccommon.ExtractAttribute(r, requestContext.SessionIDSources),
				Timezone:      gccommon.ExtractAttribute(r, requestContext.TimezoneSources),
				Timestamp:     gccommon.ExtractAttribute(r, requestContext.TimestampSources),
				Signature:     gccommon.ExtractAttribute(r, requestContext.SignatureSources),
				Authorization: gccommon.ExtractAttribute(r, requestContext.AuthorizationSources),
				AccessKey:     gccommon.ExtractAttribute(r, requestContext.AccessKeySources),
				AppID:         gccommon.ExtractAttribute(r, requestContext.AppIDSources),
				DeviceID:      gccommon.ExtractAttribute(r, requestContext.DeviceIDSources),
				AppVersion:    gccommon.ExtractAttribute(r, requestContext.AppVersionSources),
				Platform:      gccommon.ExtractAttribute(r, requestContext.PlatformSources),
				Nonce:         gccommon.ExtractAttribute(r, requestContext.NonceSources),
			}

			// 将核心链路字段注入 context，便于日志和下游组件统一获取
			ctx = WithTraceID(ctx, requestCommonMeta.TraceID)
			ctx = WithRequestID(ctx, requestCommonMeta.RequestID)
			ctx = WithUserID(ctx, requestCommonMeta.UserID)
			ctx = WithTenantID(ctx, requestCommonMeta.TenantID)
			ctx = WithSessionID(ctx, requestCommonMeta.SessionID)
			ctx = WithTimezone(ctx, requestCommonMeta.Timezone)
			ctx = context.WithValue(ctx, requestCommonMetaKey{}, requestCommonMeta)

			// 5. 设置响应头（便于客户端追踪）
			w.Header().Set(constants.HeaderXTraceID, requestCommonMeta.TraceID)
			w.Header().Set(constants.HeaderXRequestID, requestCommonMeta.RequestID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetRequestCommonMeta 获取缓存的请求公共元信息。
func GetRequestCommonMeta(ctx context.Context) *RequestCommonMeta {
	if ctx == nil {
		return &RequestCommonMeta{}
	}
	if requestCommonMeta, ok := ctx.Value(requestCommonMetaKey{}).(*RequestCommonMeta); ok && requestCommonMeta != nil {
		return requestCommonMeta
	}

	// 回退：直接从 context 中提取链路字段，避免递归调用。
	return &RequestCommonMeta{
		TraceID:   contextx.GetValue[string](ctx, constants.MetadataTraceID),
		RequestID: contextx.GetValue[string](ctx, constants.MetadataRequestID),
		UserID:    contextx.GetValue[string](ctx, constants.MetadataUserID),
		TenantID:  contextx.GetValue[string](ctx, constants.MetadataTenantID),
		SessionID: contextx.GetValue[string](ctx, constants.MetadataSessionID),
		Timezone:  contextx.GetValue[string](ctx, constants.MetadataTimezone),
	}
}

// ============================================================================
// gRPC Server 拦截器
// ============================================================================

// UnaryServerRequestContextInterceptor gRPC Server 一元调用拦截器
func UnaryServerRequestContextInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 增强 context
		ctx = enrichContextFromMetadata(ctx)

		// 设置响应 metadata（必须在 handler 调用前）
		setResponseMetadata(ctx)

		// 调用处理器
		return handler(ctx, req)
	}
}

// StreamServerRequestContextInterceptor gRPC Server 流式调用拦截器
func StreamServerRequestContextInterceptor() grpc.StreamServerInterceptor {
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

	firstMetadataValue := func(key string) string {
		if values := md.Get(key); len(values) > 0 {
			return values[0]
		}
		return ""
	}

	// 提取 TraceID & RequestID
	traceID := extractOrGenerateTraceID(ctx, firstMetadataValue(constants.MetadataTraceID))
	requestID := extractOrGenerateRequestID(firstMetadataValue(constants.MetadataRequestID))

	// 提取其他可选字段
	userID := firstMetadataValue(constants.MetadataUserID)
	sessionID := firstMetadataValue(constants.MetadataSessionID)
	tenantID := firstMetadataValue(constants.MetadataTenantID)
	timezone := firstMetadataValue(constants.MetadataTimezone)

	ctx = WithRequestID(ctx, requestID)
	ctx = WithTraceID(ctx, traceID)
	ctx = WithUserID(ctx, userID)
	ctx = WithTenantID(ctx, tenantID)
	ctx = WithSessionID(ctx, sessionID)
	ctx = WithTimezone(ctx, timezone)

	return context.WithValue(ctx, requestCommonMetaKey{}, &RequestCommonMeta{
		TraceID:   traceID,
		RequestID: requestID,
		UserID:    userID,
		TenantID:  tenantID,
		SessionID: sessionID,
		Timezone:  timezone,
	})
}

// setResponseMetadata 设置 gRPC 响应 metadata（与 HTTP 的 w.Header().Set 对应）
func setResponseMetadata(ctx context.Context) {
	requestCommonMeta := GetRequestCommonMeta(ctx)

	md := metadata.Pairs(
		constants.MetadataTraceID, requestCommonMeta.TraceID,
		constants.MetadataRequestID, requestCommonMeta.RequestID,
	)

	// 发送 metadata（忽略错误，因为可能已经发送过）
	if len(md) > 0 {
		grpc.SetHeader(ctx, md)
	}
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

// UnaryClientRequestContextInterceptor gRPC Client 一元调用拦截器
// 职责：将 context 中的 trace 信息传递到 gRPC metadata
func UnaryClientRequestContextInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// 将 context 中的 trace 信息注入到 outgoing metadata
		ctx = injectTraceToOutgoingContext(ctx)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// StreamClientRequestContextInterceptor gRPC Client 流式调用拦截器
func StreamClientRequestContextInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		// 将 context 中的 trace 信息注入到 outgoing metadata
		ctx = injectTraceToOutgoingContext(ctx)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

// injectTraceToOutgoingContext 将 context 中的 trace 信息注入到 outgoing gRPC metadata
func injectTraceToOutgoingContext(ctx context.Context) context.Context {
	requestCommonMeta := GetRequestCommonMeta(ctx)

	// 直接注入所有字段，空值也可以传递
	md := metadata.Pairs(
		constants.MetadataTraceID, requestCommonMeta.TraceID,
		constants.MetadataRequestID, requestCommonMeta.RequestID,
		constants.MetadataUserID, requestCommonMeta.UserID,
		constants.MetadataTenantID, requestCommonMeta.TenantID,
		constants.MetadataSessionID, requestCommonMeta.SessionID,
		constants.MetadataTimezone, requestCommonMeta.Timezone,
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

	return osx.HashUnixMicroCipherText()
}

// extractOrGenerateRequestID 提取或生成 RequestID
func extractOrGenerateRequestID(requestID string) string {
	if requestID != "" {
		return requestID
	}
	return osx.HashUnixMicroCipherText()
}

// ============================================================================
// 通用工具方法，供其他组件使用
// ============================================================================

// GetTraceID 从 context 获取 TraceID
func GetTraceID(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.TraceID
}

// GetRequestID 从 context 获取 RequestID
func GetRequestID(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.RequestID
}

// GetUserID 从 context 获取 UserID
func GetUserID(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.UserID
}

// GetTenantID 从 context 获取 TenantID
func GetTenantID(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.TenantID
}

// GetSessionID 从 context 获取 SessionID
func GetSessionID(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.SessionID
}

// GetTimezone 从 context 获取 Timezone
func GetTimezone(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.Timezone
}

// WithTraceID 将 TraceID 设置到 context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataTraceID, traceID)
}

// WithRequestID 将 RequestID 设置到 context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataRequestID, requestID)
}

// WithUserID 将 UserID 设置到 context
func WithUserID(ctx context.Context, userID string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataUserID, userID)
}

// WithTenantID 将 TenantID 设置到 context
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataTenantID, tenantID)
}

// WithSessionID 将 SessionID 设置到 context
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataSessionID, sessionID)
}

// WithTimezone 将 Timezone 设置到 context
func WithTimezone(ctx context.Context, timezone string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataTimezone, timezone)
}
