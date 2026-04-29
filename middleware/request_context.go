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
	"github.com/kamalyes/go-toolbox/pkg/netx"
	"github.com/kamalyes/go-toolbox/pkg/osx"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// RequestCommonMeta 请求公共元信息
type RequestCommonMeta struct {
	ID                string `json:"id" header:"X-ID"`                                 // 请求ID
	TraceID           string `json:"traceID" header:"X-Trace-ID"`                      // 跟踪ID
	RequestID         string `json:"requestID" header:"X-Request-ID"`                  // 请求ID
	UserID            string `json:"userID" header:"X-User-ID"`                        // 用户ID
	TenantID          string `json:"tenantID" header:"X-Tenant-ID"`                    // 租户ID
	TenantCode        string `json:"tenantCode" header:"X-Tenant-Code"`                // 租户编码
	SessionID         string `json:"sessionID" header:"X-Session-ID"`                  // 会话ID
	Timezone          string `json:"timezone" header:"X-Timezone"`                     // 时区
	Timestamp         string `json:"timestamp" header:"X-Timestamp"`                   // 时间戳
	Signature         string `json:"signature" header:"X-Signature"`                   // 签名
	Authorization     string `json:"authorization" header:"Authorization"`             // 授权
	AccessKey         string `json:"accessKey" header:"X-Access-Key"`                  // 访问密钥
	AppID             string `json:"appID" header:"X-App-ID"`                          // 应用ID
	DeviceID          string `json:"deviceID" header:"X-Device-ID"`                    // 设备ID
	AppVersion        string `json:"appVersion" header:"X-App-Version"`                // 应用版本
	IPAddress         string `json:"ipAddress" header:"X-Forwarded-For"`               // IP地址
	PlatformID        string `json:"platformID" header:"X-Platform-ID"`                // 平台ID
	PlatformCode      string `json:"platformCode" header:"X-Platform-Code"`            // 平台编码
	RegionID          string `json:"regionID" header:"X-Region-ID"`                    // 区域ID
	RegionCode        string `json:"regionCode" header:"X-Region-Code"`                // 区域编码
	Nonce             string `json:"nonce" header:"X-Nonce"`                           // 随机数
	XNsID             string `json:"xNsID" header:"X-Ns-ID"`                           // 命名空间ID
	GrpcMetadataXNsID string `json:"grpcMetadataXNsID" header:"Grpc-Metadata-X-Ns-ID"` // gRPC元数据命名空间ID
}

type requestCommonMetaKey struct{}

// WithRequestCommonMeta 为上下文添加请求公共元信息
func WithRequestCommonMeta(ctx context.Context, requestCommonMeta *RequestCommonMeta) context.Context {
	return contextx.WithValue(ctx, &requestCommonMetaKey{}, requestCommonMeta)
}

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
				ID:            gccommon.ExtractAttribute(r, requestContext.IDSources),
				TraceID:       extractOrGenerateTraceID(ctx, gccommon.ExtractAttribute(r, requestContext.TraceIDSources)),
				RequestID:     extractOrGenerateRequestID(gccommon.ExtractAttribute(r, requestContext.RequestIDSources)),
				UserID:        gccommon.ExtractAttribute(r, requestContext.UserIDSources),
				TenantID:      gccommon.ExtractAttribute(r, requestContext.TenantIDSources),
				TenantCode:    gccommon.ExtractAttribute(r, requestContext.TenantCodeSources),
				SessionID:     gccommon.ExtractAttribute(r, requestContext.SessionIDSources),
				Timezone:      gccommon.ExtractAttribute(r, requestContext.TimezoneSources),
				Timestamp:     gccommon.ExtractAttribute(r, requestContext.TimestampSources),
				Signature:     gccommon.ExtractAttribute(r, requestContext.SignatureSources),
				Authorization: gccommon.ExtractAttribute(r, requestContext.AuthorizationSources),
				AccessKey:     gccommon.ExtractAttribute(r, requestContext.AccessKeySources),
				AppID:         gccommon.ExtractAttribute(r, requestContext.AppIDSources),
				DeviceID:      gccommon.ExtractAttribute(r, requestContext.DeviceIDSources),
				AppVersion:    gccommon.ExtractAttribute(r, requestContext.AppVersionSources),
				PlatformID:    gccommon.ExtractAttribute(r, requestContext.PlatformIDSources),
				PlatformCode:  gccommon.ExtractAttribute(r, requestContext.PlatformCodeSources),
				RegionID:      gccommon.ExtractAttribute(r, requestContext.RegionIDSources),
				RegionCode:    gccommon.ExtractAttribute(r, requestContext.RegionCodeSources),
				IPAddress:     netx.GetClientIP(r),
				Nonce:         gccommon.ExtractAttribute(r, requestContext.NonceSources),
			}

			// 将核心链路字段注入 context，便于日志和下游组件统一获取
			ctx = WithID(ctx, requestCommonMeta.ID)
			ctx = WithTraceID(ctx, requestCommonMeta.TraceID)
			ctx = WithRequestID(ctx, requestCommonMeta.RequestID)
			ctx = WithUserID(ctx, requestCommonMeta.UserID)
			ctx = WithTenantID(ctx, requestCommonMeta.TenantID)
			ctx = WithTenantCode(ctx, requestCommonMeta.TenantCode)
			ctx = WithSessionID(ctx, requestCommonMeta.SessionID)
			ctx = WithTimezone(ctx, requestCommonMeta.Timezone)
			ctx = WithIPAddress(ctx, requestCommonMeta.IPAddress)
			ctx = WithAppID(ctx, requestCommonMeta.AppID)
			ctx = WithDeviceID(ctx, requestCommonMeta.DeviceID)
			ctx = WithAppVersion(ctx, requestCommonMeta.AppVersion)
			ctx = WithPlatformID(ctx, requestCommonMeta.PlatformID)
			ctx = WithPlatformCode(ctx, requestCommonMeta.PlatformCode)
			ctx = WithRegionID(ctx, requestCommonMeta.RegionID)
			ctx = WithRegionCode(ctx, requestCommonMeta.RegionCode)
			ctx = WithNonce(ctx, requestCommonMeta.Nonce)
			ctx = WithRequestCommonMeta(ctx, requestCommonMeta)

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
		ID:                contextx.GetValue[string](ctx, constants.MetadataID),
		TraceID:           contextx.GetValue[string](ctx, constants.MetadataTraceID),
		RequestID:         contextx.GetValue[string](ctx, constants.MetadataRequestID),
		Authorization:     contextx.GetValue[string](ctx, constants.MetadataAuthorization),
		UserID:            contextx.GetValue[string](ctx, constants.MetadataUserID),
		TenantID:          contextx.GetValue[string](ctx, constants.MetadataTenantID),
		TenantCode:        contextx.GetValue[string](ctx, constants.MetadataTenantCode),
		SessionID:         contextx.GetValue[string](ctx, constants.MetadataSessionID),
		Timezone:          contextx.GetValue[string](ctx, constants.MetadataTimezone),
		IPAddress:         contextx.GetValue[string](ctx, constants.MetadataIPAddress),
		AppID:             contextx.GetValue[string](ctx, constants.MetadataAppID),
		DeviceID:          contextx.GetValue[string](ctx, constants.MetadataDeviceID),
		AppVersion:        contextx.GetValue[string](ctx, constants.MetadataAppVersion),
		PlatformID:        contextx.GetValue[string](ctx, constants.MetadataPlatformID),
		PlatformCode:      contextx.GetValue[string](ctx, constants.MetadataPlatformCode),
		RegionID:          contextx.GetValue[string](ctx, constants.MetadataRegionID),
		RegionCode:        contextx.GetValue[string](ctx, constants.MetadataRegionCode),
		Nonce:             contextx.GetValue[string](ctx, constants.MetadataNonce),
		XNsID:             contextx.GetValue[string](ctx, constants.MetadataXNsID),
		GrpcMetadataXNsID: contextx.GetValue[string](ctx, constants.MetadataGrpcMetadataXNsID),
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
	id := firstMetadataValue(constants.MetadataID)
	authorization := firstMetadataValue(constants.MetadataAuthorization)
	userID := firstMetadataValue(constants.MetadataUserID)
	sessionID := firstMetadataValue(constants.MetadataSessionID)
	tenantID := firstMetadataValue(constants.MetadataTenantID)
	tenantCode := firstMetadataValue(constants.MetadataTenantCode)
	timezone := firstMetadataValue(constants.MetadataTimezone)
	ipAddress := firstMetadataValue(constants.MetadataIPAddress)
	appID := firstMetadataValue(constants.MetadataAppID)
	deviceID := firstMetadataValue(constants.MetadataDeviceID)
	appVersion := firstMetadataValue(constants.MetadataAppVersion)
	platformID := firstMetadataValue(constants.MetadataPlatformID)
	platformCode := firstMetadataValue(constants.MetadataPlatformCode)
	regionID := firstMetadataValue(constants.MetadataRegionID)
	regionCode := firstMetadataValue(constants.MetadataRegionCode)
	nonce := firstMetadataValue(constants.MetadataNonce)
	xNsID := firstMetadataValue(constants.MetadataXNsID)
	grpcMetadataXNsID := firstMetadataValue(constants.MetadataGrpcMetadataXNsID)

	ctx = WithID(ctx, id)
	ctx = WithRequestID(ctx, requestID)
	ctx = WithTraceID(ctx, traceID)
	ctx = WithAuthorization(ctx, authorization)
	ctx = WithUserID(ctx, userID)
	ctx = WithTenantID(ctx, tenantID)
	ctx = WithTenantCode(ctx, tenantCode)
	ctx = WithSessionID(ctx, sessionID)
	ctx = WithIPAddress(ctx, ipAddress)
	ctx = WithTimezone(ctx, timezone)
	ctx = WithAppID(ctx, appID)
	ctx = WithDeviceID(ctx, deviceID)
	ctx = WithAppVersion(ctx, appVersion)
	ctx = WithPlatformID(ctx, platformID)
	ctx = WithPlatformCode(ctx, platformCode)
	ctx = WithRegionID(ctx, regionID)
	ctx = WithRegionCode(ctx, regionCode)
	ctx = WithNonce(ctx, nonce)
	ctx = WithXNsID(ctx, xNsID)
	ctx = WithGrpcMetadataXNsID(ctx, grpcMetadataXNsID)

	return context.WithValue(ctx, requestCommonMetaKey{}, &RequestCommonMeta{
		ID:                id,
		TraceID:           traceID,
		RequestID:         requestID,
		Authorization:     authorization,
		UserID:            userID,
		TenantID:          tenantID,
		TenantCode:        tenantCode,
		SessionID:         sessionID,
		Timezone:          timezone,
		IPAddress:         ipAddress,
		AppID:             appID,
		DeviceID:          deviceID,
		AppVersion:        appVersion,
		PlatformID:        platformID,
		PlatformCode:      platformCode,
		RegionID:          regionID,
		RegionCode:        regionCode,
		Nonce:             nonce,
		XNsID:             xNsID,
		GrpcMetadataXNsID: grpcMetadataXNsID,
	})
}

// setResponseMetadata 设置 gRPC 响应 metadata（与 HTTP 的 w.Header().Set 对应）
func setResponseMetadata(ctx context.Context) {
	requestCommonMeta := GetRequestCommonMeta(ctx)

	md := metadata.Pairs(
		constants.MetadataID, requestCommonMeta.ID,
		constants.MetadataTraceID, requestCommonMeta.TraceID,
		constants.MetadataRequestID, requestCommonMeta.RequestID,
		constants.MetadataAuthorization, requestCommonMeta.Authorization,
		constants.MetadataUserID, requestCommonMeta.UserID,
		constants.MetadataTenantID, requestCommonMeta.TenantID,
		constants.MetadataTenantCode, requestCommonMeta.TenantCode,
		constants.MetadataSessionID, requestCommonMeta.SessionID,
		constants.MetadataTimezone, requestCommonMeta.Timezone,
		constants.MetadataIPAddress, requestCommonMeta.IPAddress,
		constants.MetadataAppID, requestCommonMeta.AppID,
		constants.MetadataDeviceID, requestCommonMeta.DeviceID,
		constants.MetadataAppVersion, requestCommonMeta.AppVersion,
		constants.MetadataPlatformID, requestCommonMeta.PlatformID,
		constants.MetadataPlatformCode, requestCommonMeta.PlatformCode,
		constants.MetadataRegionID, requestCommonMeta.RegionID,
		constants.MetadataRegionCode, requestCommonMeta.RegionCode,
		constants.MetadataNonce, requestCommonMeta.Nonce,
		constants.MetadataXNsID, requestCommonMeta.XNsID,
		constants.MetadataGrpcMetadataXNsID, requestCommonMeta.GrpcMetadataXNsID,
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
		constants.MetadataID, requestCommonMeta.ID,
		constants.MetadataTraceID, requestCommonMeta.TraceID,
		constants.MetadataRequestID, requestCommonMeta.RequestID,
		constants.MetadataAuthorization, requestCommonMeta.Authorization,
		constants.MetadataUserID, requestCommonMeta.UserID,
		constants.MetadataTenantID, requestCommonMeta.TenantID,
		constants.MetadataTenantCode, requestCommonMeta.TenantCode,
		constants.MetadataSessionID, requestCommonMeta.SessionID,
		constants.MetadataTimezone, requestCommonMeta.Timezone,
		constants.MetadataIPAddress, requestCommonMeta.IPAddress,
		constants.MetadataAppID, requestCommonMeta.AppID,
		constants.MetadataDeviceID, requestCommonMeta.DeviceID,
		constants.MetadataAppVersion, requestCommonMeta.AppVersion,
		constants.MetadataPlatformID, requestCommonMeta.PlatformID,
		constants.MetadataPlatformCode, requestCommonMeta.PlatformCode,
		constants.MetadataRegionID, requestCommonMeta.RegionID,
		constants.MetadataRegionCode, requestCommonMeta.RegionCode,
		constants.MetadataNonce, requestCommonMeta.Nonce,
		constants.MetadataXNsID, requestCommonMeta.XNsID,
		constants.MetadataGrpcMetadataXNsID, requestCommonMeta.GrpcMetadataXNsID,
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

// GetIPAddress 从 context 获取 IPAddress
func GetIPAddress(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.IPAddress
}

// GetID 从 context 获取 ID
func GetID(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.ID
}

// GetTenantCode 从 context 获取 TenantCode
func GetTenantCode(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.TenantCode
}

// GetPlatformID 从 context 获取 PlatformID
func GetPlatformID(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.PlatformID
}

// GetPlatformCode 从 context 获取 PlatformCode
func GetPlatformCode(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.PlatformCode
}

// GetRegionID 从 context 获取 RegionID
func GetRegionID(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.RegionID
}

// GetRegionCode 从 context 获取 RegionCode
func GetRegionCode(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.RegionCode
}

// GetXNsID 从 context 获取 XNsID
func GetXNsID(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.XNsID
}

// GetGrpcMetadataXNsID 从 context 获取 GrpcMetadataXNsID
func GetGrpcMetadataXNsID(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.GrpcMetadataXNsID
}

// GetAppID 从 context 获取 AppID
func GetAppID(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.AppID
}

// GetDeviceID 从 context 获取 DeviceID
func GetDeviceID(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.DeviceID
}

// GetAppVersion 从 context 获取 AppVersion
func GetAppVersion(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.AppVersion
}

// GetNonce 从 context 获取 Nonce
func GetNonce(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.Nonce
}

// WithTraceID 将 TraceID 设置到 context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataTraceID, traceID)
}

// WithRequestID 将 RequestID 设置到 context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataRequestID, requestID)
}

// WithAuthorization 将 Authorization 设置到 context
func WithAuthorization(ctx context.Context, authorization string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataAuthorization, authorization)
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

// WithIPAddress 将 IPAddress 设置到 context
func WithIPAddress(ctx context.Context, ipAddress string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataIPAddress, ipAddress)
}

// WithID 将 ID 设置到 context
func WithID(ctx context.Context, id string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataID, id)
}

// WithTenantCode 将 TenantCode 设置到 context
func WithTenantCode(ctx context.Context, tenantCode string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataTenantCode, tenantCode)
}

// WithPlatformID 将 PlatformID 设置到 context
func WithPlatformID(ctx context.Context, platformID string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataPlatformID, platformID)
}

// WithPlatformCode 将 PlatformCode 设置到 context
func WithPlatformCode(ctx context.Context, platformCode string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataPlatformCode, platformCode)
}

// WithRegionID 将 RegionID 设置到 context
func WithRegionID(ctx context.Context, regionID string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataRegionID, regionID)
}

// WithRegionCode 将 RegionCode 设置到 context
func WithRegionCode(ctx context.Context, regionCode string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataRegionCode, regionCode)
}

// WithXNsID 将 XNsID 设置到 context
func WithXNsID(ctx context.Context, xNsID string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataXNsID, xNsID)
}

// WithGrpcMetadataXNsID 将 GrpcMetadataXNsID 设置到 context
func WithGrpcMetadataXNsID(ctx context.Context, grpcMetadataXNsID string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataGrpcMetadataXNsID, grpcMetadataXNsID)
}

// WithAppID 将 AppID 设置到 context
func WithAppID(ctx context.Context, appID string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataAppID, appID)
}

// WithDeviceID 将 DeviceID 设置到 context
func WithDeviceID(ctx context.Context, deviceID string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataDeviceID, deviceID)
}

// WithAppVersion 将 AppVersion 设置到 context
func WithAppVersion(ctx context.Context, appVersion string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataAppVersion, appVersion)
}

// WithNonce 将 Nonce 设置到 context
func WithNonce(ctx context.Context, nonce string) context.Context {
	return contextx.WithValue(ctx, constants.MetadataNonce, nonce)
}
