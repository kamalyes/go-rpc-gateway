/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-29 10:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-19 18:18:50
 * @FilePath: \go-rpc-gateway\middleware\request_context.go
 * @Description: 统一的请求上下文中间件 - 实现 HTTP → gRPC → Service → Repository 全链路上下文传递
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	gccommon "github.com/kamalyes/go-config/pkg/common"
	goi18n "github.com/kamalyes/go-i18n"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/contextx"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/netx"
	"github.com/kamalyes/go-toolbox/pkg/osx"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// RequestCommonMeta 请求公共元信息
type RequestCommonMeta struct {
	ID             string `json:"id" header:"X-ID"`                        // 请求ID
	TraceID        string `json:"traceID" header:"X-Trace-ID"`             // 跟踪ID
	RequestID      string `json:"requestID" header:"X-Request-ID"`         // 请求ID
	UserID         string `json:"userID" header:"X-User-ID"`               // 用户ID
	UserName       string `json:"userName" header:"X-User-Name"`           // 用户名称
	Domain         string `json:"domain" header:"X-Domain"`                // 域
	RoleCode       string `json:"roleCode" header:"X-Role-Code"`           // 角色Code
	TenantID       string `json:"tenantID" header:"X-Tenant-ID"`           // 租户ID
	TenantCode     string `json:"tenantCode" header:"X-Tenant-Code"`       // 租户编码
	SessionID      string `json:"sessionID" header:"X-Session-ID"`         // 会话ID
	Timezone       string `json:"timezone" header:"X-Timezone"`            // 时区
	Timestamp      string `json:"timestamp" header:"X-Timestamp"`          // 时间戳
	Signature      string `json:"signature" header:"X-Signature"`          // 签名
	Authorization  string `json:"authorization" header:"Authorization"`    // 授权
	AccessKey      string `json:"accessKey" header:"X-Access-Key"`         // 访问密钥
	AppID          string `json:"appID" header:"X-App-ID"`                 // 应用ID
	DeviceID       string `json:"deviceID" header:"X-Device-ID"`           // 设备ID
	AppVersion     string `json:"appVersion" header:"X-App-Version"`       // 应用版本
	IPAddress      string `json:"ipAddress" header:"X-Forwarded-For"`      // IP地址
	PlatformID     string `json:"platformID" header:"X-Platform-ID"`       // 平台ID
	PlatformCode   string `json:"platformCode" header:"X-Platform-Code"`   // 平台编码
	RegionID       string `json:"regionID" header:"X-Region-ID"`           // 区域ID
	RegionCode     string `json:"regionCode" header:"X-Region-Code"`       // 区域编码
	AgentLineID    string `json:"agentLineID" header:"X-Agent-Line-ID"`    // 代理线ID
	Nonce          string `json:"nonce" header:"X-Nonce"`                  // 随机数
	Jti            string `json:"jti" header:"X-Jti"`                      // JWT ID (Token唯一标识)
	FamilyId       string `json:"familyId" header:"X-Family-ID"`           // Token家族ID
	XNsID          string `json:"xNsID" header:"X-Ns-ID"`                  // 命名空间ID
	UserAgent      string `json:"userAgent" header:"User-Agent"`           // 用户代理
	PushToken      string `json:"pushToken" header:"X-Push-Token"`         // 推送Token
	Token          string `json:"token" header:"X-Token"`                  // Token
	AcceptLanguage string `json:"acceptLanguage" header:"Accept-Language"` // 语言环境
	ForwardedHost  string `json:"forwardedHost" header:"X-Forwarded-Host"` // 转发域名
}

type requestCommonMetaKey struct{}

// WithRequestCommonMeta 为上下文添加请求公共元信息
func WithRequestCommonMeta(ctx context.Context, requestCommonMeta *RequestCommonMeta) context.Context {
	return contextx.WithValue(ctx, requestCommonMetaKey{}, requestCommonMeta)
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
				Domain:        gccommon.ExtractAttribute(r, requestContext.DomainSources),
				RoleCode:      gccommon.ExtractAttribute(r, requestContext.RoleCodeSources),
				TenantID:      gccommon.ExtractAttribute(r, requestContext.TenantIDSources),
				TenantCode:    gccommon.ExtractAttribute(r, requestContext.TenantCodeSources),
				SessionID:     gccommon.ExtractAttribute(r, requestContext.SessionIDSources),
				Timezone:      gccommon.ExtractAttribute(r, requestContext.TimezoneSources),
				Timestamp:     gccommon.ExtractAttribute(r, requestContext.TimestampSources),
				Signature:     gccommon.ExtractAttribute(r, requestContext.SignatureSources),
				Authorization: gccommon.ExtractAttribute(r, requestContext.AuthorizationSources),
				Jti:           gccommon.ExtractAttribute(r, requestContext.JtiSources),
				FamilyId:      gccommon.ExtractAttribute(r, requestContext.FamilyIdSources),
				AccessKey:     gccommon.ExtractAttribute(r, requestContext.AccessKeySources),
				AppID:         gccommon.ExtractAttribute(r, requestContext.AppIDSources),
				DeviceID:      gccommon.ExtractAttribute(r, requestContext.DeviceIDSources),
				AppVersion:    gccommon.ExtractAttribute(r, requestContext.AppVersionSources),
				PlatformID:    gccommon.ExtractAttribute(r, requestContext.PlatformIDSources),
				PlatformCode:  gccommon.ExtractAttribute(r, requestContext.PlatformCodeSources),
				RegionID:      gccommon.ExtractAttribute(r, requestContext.RegionIDSources),
				RegionCode:    gccommon.ExtractAttribute(r, requestContext.RegionCodeSources),
				AgentLineID:   gccommon.ExtractAttribute(r, requestContext.AgentLineIDSources),
				IPAddress:     netx.GetClientIP(r),
				Nonce:         gccommon.ExtractAttribute(r, requestContext.NonceSources),
				UserAgent:     r.UserAgent(),
				PushToken:     gccommon.ExtractAttribute(r, requestContext.PushTokenSources),
				Token:         gccommon.ExtractAttribute(r, requestContext.TokenSources),
				ForwardedHost: gccommon.ExtractAttribute(r, requestContext.ForwardedHostSources),
			}

			// 将核心链路字段注入 context，便于日志和下游组件统一获取
			ctx = NewContextBuilder(ctx).
				WithID(requestCommonMeta.ID).
				WithTraceID(requestCommonMeta.TraceID).
				WithRequestID(requestCommonMeta.RequestID).
				WithUserID(requestCommonMeta.UserID).
				WithDomain(requestCommonMeta.Domain).
				WithRoleCode(requestCommonMeta.RoleCode).
				WithTenantID(requestCommonMeta.TenantID).
				WithTenantCode(requestCommonMeta.TenantCode).
				WithAuthorization(requestCommonMeta.Authorization).
				WithSessionID(requestCommonMeta.SessionID).
				WithTimezone(requestCommonMeta.Timezone).
				WithIPAddress(requestCommonMeta.IPAddress).
				WithAppID(requestCommonMeta.AppID).
				WithDeviceID(requestCommonMeta.DeviceID).
				WithAppVersion(requestCommonMeta.AppVersion).
				WithPlatformID(requestCommonMeta.PlatformID).
				WithPlatformCode(requestCommonMeta.PlatformCode).
				WithRegionID(requestCommonMeta.RegionID).
				WithRegionCode(requestCommonMeta.RegionCode).
				WithAgentLineID(requestCommonMeta.AgentLineID).
				WithNonce(requestCommonMeta.Nonce).
				WithJti(requestCommonMeta.Jti).
				WithFamilyId(requestCommonMeta.FamilyId).
				WithUserAgent(requestCommonMeta.UserAgent).
				WithPushToken(requestCommonMeta.PushToken).
				WithToken(requestCommonMeta.Token).
				WithTimestamp(requestCommonMeta.Timestamp).
				WithSignature(requestCommonMeta.Signature).
				WithAccessKey(requestCommonMeta.AccessKey).
				WithForwardedHost(requestCommonMeta.ForwardedHost).
				Build()
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
		ID:             contextx.GetValue[string](ctx, constants.MetadataID),
		TraceID:        contextx.GetValue[string](ctx, constants.MetadataTraceID),
		RequestID:      contextx.GetValue[string](ctx, constants.MetadataRequestID),
		Authorization:  contextx.GetValue[string](ctx, constants.MetadataAuthorization),
		UserID:         contextx.GetValue[string](ctx, constants.MetadataUserID),
		UserName:       contextx.GetValue[string](ctx, constants.MetadataUserName),
		Domain:         contextx.GetValue[string](ctx, constants.MetadataDomain),
		RoleCode:       contextx.GetValue[string](ctx, constants.MetadataRoleCode),
		TenantID:       contextx.GetValue[string](ctx, constants.MetadataTenantID),
		TenantCode:     contextx.GetValue[string](ctx, constants.MetadataTenantCode),
		SessionID:      contextx.GetValue[string](ctx, constants.MetadataSessionID),
		Timezone:       contextx.GetValue[string](ctx, constants.MetadataTimezone),
		IPAddress:      contextx.GetValue[string](ctx, constants.MetadataIPAddress),
		AppID:          contextx.GetValue[string](ctx, constants.MetadataAppID),
		DeviceID:       contextx.GetValue[string](ctx, constants.MetadataDeviceID),
		AppVersion:     contextx.GetValue[string](ctx, constants.MetadataAppVersion),
		PlatformID:     contextx.GetValue[string](ctx, constants.MetadataPlatformID),
		PlatformCode:   contextx.GetValue[string](ctx, constants.MetadataPlatformCode),
		RegionID:       contextx.GetValue[string](ctx, constants.MetadataRegionID),
		RegionCode:     contextx.GetValue[string](ctx, constants.MetadataRegionCode),
		AgentLineID:    contextx.GetValue[string](ctx, constants.MetadataAgentLineID),
		Nonce:          contextx.GetValue[string](ctx, constants.MetadataNonce),
		Jti:            contextx.GetValue[string](ctx, constants.MetadataJti),
		FamilyId:       contextx.GetValue[string](ctx, constants.MetadataFamilyId),
		XNsID:          contextx.GetValue[string](ctx, constants.MetadataXNsID),
		UserAgent:      contextx.GetValue[string](ctx, constants.MetadataUserAgent),
		PushToken:      contextx.GetValue[string](ctx, constants.MetadataPushToken),
		Token:          contextx.GetValue[string](ctx, constants.MetadataToken),
		Timestamp:      contextx.GetValue[string](ctx, constants.MetadataTimestamp),
		Signature:      contextx.GetValue[string](ctx, constants.MetadataSignature),
		AccessKey:      contextx.GetValue[string](ctx, constants.MetadataAccessKey),
		AcceptLanguage: contextx.GetValue[string](ctx, constants.MetadataAcceptLanguage),
		ForwardedHost:  contextx.GetValue[string](ctx, constants.MetadataForwardedHost),
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
	userName, _ := url.QueryUnescape(firstMetadataValue(constants.MetadataUserName))
	domain := firstMetadataValue(constants.MetadataDomain)
	roleCode := firstMetadataValue(constants.MetadataRoleCode)
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
	agentLineID := firstMetadataValue(constants.MetadataAgentLineID)
	nonce := firstMetadataValue(constants.MetadataNonce)
	jti := firstMetadataValue(constants.MetadataJti)
	familyId := firstMetadataValue(constants.MetadataFamilyId)
	xNsID := firstMetadataValue(constants.MetadataXNsID)
	userAgent := firstMetadataValue(constants.MetadataUserAgent)
	pushToken := firstMetadataValue(constants.MetadataPushToken)
	token := firstMetadataValue(constants.MetadataToken)
	timestamp := firstMetadataValue(constants.MetadataTimestamp)
	signature := firstMetadataValue(constants.MetadataSignature)
	accessKey := firstMetadataValue(constants.MetadataAccessKey)
	acceptLanguage := firstMetadataValue(constants.MetadataAcceptLanguage)

	ctx = NewContextBuilder(ctx).
		WithID(id).
		WithRequestID(requestID).
		WithTraceID(traceID).
		WithAuthorization(authorization).
		WithUserID(userID).
		WithUserName(userName).
		WithDomain(domain).
		WithRoleCode(roleCode).
		WithTenantID(tenantID).
		WithTenantCode(tenantCode).
		WithSessionID(sessionID).
		WithIPAddress(ipAddress).
		WithTimezone(timezone).
		WithAppID(appID).
		WithDeviceID(deviceID).
		WithAppVersion(appVersion).
		WithPlatformID(platformID).
		WithPlatformCode(platformCode).
		WithRegionID(regionID).
		WithRegionCode(regionCode).
		WithAgentLineID(agentLineID).
		WithNonce(nonce).
		WithJti(jti).
		WithFamilyId(familyId).
		WithXNsID(xNsID).
		WithUserAgent(userAgent).
		WithPushToken(pushToken).
		WithToken(token).
		WithTimestamp(timestamp).
		WithSignature(signature).
		WithAccessKey(accessKey).
		WithAcceptLanguage(acceptLanguage).
		WithForwardedHost(firstMetadataValue(constants.MetadataForwardedHost)).
		Build()

	return context.WithValue(ctx, requestCommonMetaKey{}, &RequestCommonMeta{
		ID:             id,
		TraceID:        traceID,
		RequestID:      requestID,
		Authorization:  authorization,
		UserID:         userID,
		UserName:       userName,
		Domain:         domain,
		RoleCode:       roleCode,
		TenantID:       tenantID,
		TenantCode:     tenantCode,
		SessionID:      sessionID,
		Timezone:       timezone,
		IPAddress:      ipAddress,
		AppID:          appID,
		DeviceID:       deviceID,
		AppVersion:     appVersion,
		PlatformID:     platformID,
		PlatformCode:   platformCode,
		RegionID:       regionID,
		RegionCode:     regionCode,
		AgentLineID:    agentLineID,
		Nonce:          nonce,
		Jti:            jti,
		FamilyId:       familyId,
		XNsID:          xNsID,
		UserAgent:      userAgent,
		PushToken:      pushToken,
		Token:          token,
		Timestamp:      timestamp,
		Signature:      signature,
		AccessKey:      accessKey,
		AcceptLanguage: acceptLanguage,
		ForwardedHost:  firstMetadataValue(constants.MetadataForwardedHost),
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
		constants.MetadataUserName, url.QueryEscape(requestCommonMeta.UserName),
		constants.MetadataDomain, requestCommonMeta.Domain,
		constants.MetadataRoleCode, requestCommonMeta.RoleCode,
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
		constants.MetadataAgentLineID, requestCommonMeta.AgentLineID,
		constants.MetadataNonce, requestCommonMeta.Nonce,
		constants.MetadataJti, requestCommonMeta.Jti,
		constants.MetadataFamilyId, requestCommonMeta.FamilyId,
		constants.MetadataXNsID, requestCommonMeta.XNsID,
		constants.MetadataUserAgent, requestCommonMeta.UserAgent,
		constants.MetadataPushToken, requestCommonMeta.PushToken,
		constants.MetadataToken, requestCommonMeta.Token,
		constants.MetadataTimestamp, requestCommonMeta.Timestamp,
		constants.MetadataSignature, requestCommonMeta.Signature,
		constants.MetadataAccessKey, requestCommonMeta.AccessKey,
		constants.MetadataAcceptLanguage, requestCommonMeta.AcceptLanguage,
		constants.MetadataForwardedHost, requestCommonMeta.ForwardedHost,
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

	// 优先使用 RequestCommonMeta 中的 Accept Language，若为空则从 i18n context 获取
	acceptLanguage := mathx.IfEmpty(requestCommonMeta.AcceptLanguage, goi18n.GetLanguage(ctx))

	// 直接注入所有字段，空值也可以传递
	md := metadata.Pairs(
		constants.MetadataID, requestCommonMeta.ID,
		constants.MetadataTraceID, requestCommonMeta.TraceID,
		constants.MetadataRequestID, requestCommonMeta.RequestID,
		constants.MetadataAuthorization, requestCommonMeta.Authorization,
		constants.MetadataUserID, requestCommonMeta.UserID,
		constants.MetadataUserName, url.QueryEscape(requestCommonMeta.UserName),
		constants.MetadataDomain, requestCommonMeta.Domain,
		constants.MetadataRoleCode, requestCommonMeta.RoleCode,
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
		constants.MetadataAgentLineID, requestCommonMeta.AgentLineID,
		constants.MetadataNonce, requestCommonMeta.Nonce,
		constants.MetadataJti, requestCommonMeta.Jti,
		constants.MetadataFamilyId, requestCommonMeta.FamilyId,
		constants.MetadataXNsID, requestCommonMeta.XNsID,
		constants.MetadataUserAgent, requestCommonMeta.UserAgent,
		constants.MetadataPushToken, requestCommonMeta.PushToken,
		constants.MetadataToken, requestCommonMeta.Token,
		constants.MetadataTimestamp, requestCommonMeta.Timestamp,
		constants.MetadataSignature, requestCommonMeta.Signature,
		constants.MetadataAccessKey, requestCommonMeta.AccessKey,
		constants.MetadataAcceptLanguage, acceptLanguage,
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

// ForwardOutgoingContext 将 HTTP 请求的 Header 转发为 gRPC outgoing metadata
func ForwardOutgoingContext(r *http.Request) context.Context {
	md := metadata.New(nil)
	for key, values := range r.Header {
		if strings.EqualFold(key, "connection") {
			continue
		}
		for _, value := range values {
			md.Append(key, value)
		}
	}
	return metadata.NewOutgoingContext(r.Context(), md)
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

// GetUserName 从 context 获取 UserName
func GetUserName(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.UserName
}

// GetDomain 从 context 获取 Domain
func GetDomain(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.Domain
}

// GetRoleCode 从 context 获取 RoleCode
func GetRoleCode(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.RoleCode
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

// GetAuthorization 从 context 获取 Authorization
func GetAuthorization(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.Authorization
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

// GetAgentLineID 从 context 获取 AgentLineID
func GetAgentLineID(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.AgentLineID
}

// GetXNsID 从 context 获取 XNsID
func GetXNsID(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.XNsID
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

// GetJti 从 context 获取 Jti (JWT ID)
func GetJti(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.Jti
}

// GetFamilyId 从 context 获取 FamilyId (Token家族ID)
func GetFamilyId(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.FamilyId
}

// GetUserAgent 从 context 获取 UserAgent
func GetUserAgent(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.UserAgent
}

// GetTimestamp 从 context 获取 Timestamp
func GetTimestamp(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.Timestamp
}

// GetSignature 从 context 获取 Signature
func GetSignature(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.Signature
}

// GetAccessKey 从 context 获取 AccessKey
func GetAccessKey(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.AccessKey
}

// GetPushToken 从 context 获取 PushToken
func GetPushToken(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.PushToken
}

// GetToken 从 context 获取 Token
func GetToken(ctx context.Context) string {
	requestCommonMeta := GetRequestCommonMeta(ctx)
	return requestCommonMeta.Token
}

// updateRequestCommonMetaField 同步更新 RequestCommonMeta 中对应字段
func updateRequestCommonMetaField(ctx context.Context, update func(*RequestCommonMeta)) {
	if meta, ok := ctx.Value(requestCommonMetaKey{}).(*RequestCommonMeta); ok && meta != nil {
		update(meta)
	}
}

// WithTraceID 将 TraceID 设置到 context 并同步更新 RequestCommonMeta
func WithTraceID(ctx context.Context, traceID string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataTraceID, traceID)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.TraceID = traceID })
	return ctx
}

// WithRequestID 将 RequestID 设置到 context 并同步更新 RequestCommonMeta
func WithRequestID(ctx context.Context, requestID string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataRequestID, requestID)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.RequestID = requestID })
	return ctx
}

// WithAuthorization 将 Authorization 设置到 context 并同步更新 RequestCommonMeta
func WithAuthorization(ctx context.Context, authorization string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataAuthorization, authorization)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.Authorization = authorization })
	return ctx
}

// WithUserID 将 UserID 设置到 context 并同步更新 RequestCommonMeta
func WithUserID(ctx context.Context, userID string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataUserID, userID)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.UserID = userID })
	return ctx
}

// WithUserName 将 UserName 设置到 context 并同步更新 RequestCommonMeta
func WithUserName(ctx context.Context, userName string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataUserName, userName)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.UserName = userName })
	return ctx
}

// WithDomain 将 Domain 设置到 context 并同步更新 RequestCommonMeta
func WithDomain(ctx context.Context, domain string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataDomain, domain)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.Domain = domain })
	return ctx
}

// WithRoleCode 将 RoleCode 设置到 context 并同步更新 RequestCommonMeta
func WithRoleCode(ctx context.Context, roleCode string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataRoleCode, roleCode)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.RoleCode = roleCode })
	return ctx
}

// WithTenantID 将 TenantID 设置到 context 并同步更新 RequestCommonMeta
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataTenantID, tenantID)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.TenantID = tenantID })
	return ctx
}

// WithSessionID 将 SessionID 设置到 context 并同步更新 RequestCommonMeta
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataSessionID, sessionID)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.SessionID = sessionID })
	return ctx
}

// WithTimezone 将 Timezone 设置到 context 并同步更新 RequestCommonMeta
func WithTimezone(ctx context.Context, timezone string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataTimezone, timezone)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.Timezone = timezone })
	return ctx
}

// WithIPAddress 将 IPAddress 设置到 context 并同步更新 RequestCommonMeta
func WithIPAddress(ctx context.Context, ipAddress string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataIPAddress, ipAddress)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.IPAddress = ipAddress })
	return ctx
}

// WithID 将 ID 设置到 context 并同步更新 RequestCommonMeta
func WithID(ctx context.Context, id string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataID, id)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.ID = id })
	return ctx
}

// WithTenantCode 将 TenantCode 设置到 context 并同步更新 RequestCommonMeta
func WithTenantCode(ctx context.Context, tenantCode string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataTenantCode, tenantCode)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.TenantCode = tenantCode })
	return ctx
}

// WithPlatformID 将 PlatformID 设置到 context 并同步更新 RequestCommonMeta
func WithPlatformID(ctx context.Context, platformID string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataPlatformID, platformID)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.PlatformID = platformID })
	return ctx
}

// WithPlatformCode 将 PlatformCode 设置到 context 并同步更新 RequestCommonMeta
func WithPlatformCode(ctx context.Context, platformCode string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataPlatformCode, platformCode)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.PlatformCode = platformCode })
	return ctx
}

// WithRegionID 将 RegionID 设置到 context 并同步更新 RequestCommonMeta
func WithRegionID(ctx context.Context, regionID string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataRegionID, regionID)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.RegionID = regionID })
	return ctx
}

// WithRegionCode 将 RegionCode 设置到 context 并同步更新 RequestCommonMeta
func WithRegionCode(ctx context.Context, regionCode string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataRegionCode, regionCode)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.RegionCode = regionCode })
	return ctx
}

// WithAgentLineID 将 AgentLineID 设置到 context 并同步更新 RequestCommonMeta
func WithAgentLineID(ctx context.Context, agentLineID string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataAgentLineID, agentLineID)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.AgentLineID = agentLineID })
	return ctx
}

// WithXNsID 将 XNsID 设置到 context 并同步更新 RequestCommonMeta
func WithXNsID(ctx context.Context, xNsID string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataXNsID, xNsID)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.XNsID = xNsID })
	return ctx
}

// WithAppID 将 AppID 设置到 context 并同步更新 RequestCommonMeta
func WithAppID(ctx context.Context, appID string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataAppID, appID)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.AppID = appID })
	return ctx
}

// WithDeviceID 将 DeviceID 设置到 context 并同步更新 RequestCommonMeta
func WithDeviceID(ctx context.Context, deviceID string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataDeviceID, deviceID)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.DeviceID = deviceID })
	return ctx
}

// WithAppVersion 将 AppVersion 设置到 context 并同步更新 RequestCommonMeta
func WithAppVersion(ctx context.Context, appVersion string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataAppVersion, appVersion)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.AppVersion = appVersion })
	return ctx
}

// WithNonce 将 Nonce 设置到 context 并同步更新 RequestCommonMeta
func WithNonce(ctx context.Context, nonce string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataNonce, nonce)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.Nonce = nonce })
	return ctx
}

// WithJti 将 Jti 设置到 context 并同步更新 RequestCommonMeta
func WithJti(ctx context.Context, jti string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataJti, jti)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.Jti = jti })
	return ctx
}

// WithFamilyId 将 FamilyId 设置到 context 并同步更新 RequestCommonMeta
func WithFamilyId(ctx context.Context, familyId string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataFamilyId, familyId)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.FamilyId = familyId })
	return ctx
}

// WithUserAgent 将 UserAgent 设置到 context 并同步更新 RequestCommonMeta
func WithUserAgent(ctx context.Context, userAgent string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataUserAgent, userAgent)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.UserAgent = userAgent })
	return ctx
}

// WithTimestamp 将 Timestamp 设置到 context 并同步更新 RequestCommonMeta
func WithTimestamp(ctx context.Context, timestamp string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataTimestamp, timestamp)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.Timestamp = timestamp })
	return ctx
}

// WithSignature 将 Signature 设置到 context 并同步更新 RequestCommonMeta
func WithSignature(ctx context.Context, signature string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataSignature, signature)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.Signature = signature })
	return ctx
}

// WithAccessKey 将 AccessKey 设置到 context 并同步更新 RequestCommonMeta
func WithAccessKey(ctx context.Context, accessKey string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataAccessKey, accessKey)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.AccessKey = accessKey })
	return ctx
}

// WithPushToken 将 PushToken 设置到 context 并同步更新 RequestCommonMeta
func WithPushToken(ctx context.Context, pushToken string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataPushToken, pushToken)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.PushToken = pushToken })
	return ctx
}

// WithToken 将 Token 设置到 context 并同步更新 RequestCommonMeta
func WithToken(ctx context.Context, token string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataToken, token)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.Token = token })
	return ctx
}

// WithAcceptLanguage 将 Accept Language 设置到 context 并同步更新 RequestCommonMeta
func WithAcceptLanguage(ctx context.Context, language string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataAcceptLanguage, language)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.AcceptLanguage = language })
	return ctx
}

// GetAcceptLanguageFromMeta 从 RequestCommonMeta 获取 Accept Language
func GetAcceptLanguageFromMeta(ctx context.Context) string {
	if meta := GetRequestCommonMeta(ctx); meta != nil {
		return meta.AcceptLanguage
	}
	return ""
}

// WithForwardedHost 为上下文添加转发域名
func WithForwardedHost(ctx context.Context, host string) context.Context {
	ctx = contextx.WithValue(ctx, constants.MetadataForwardedHost, host)
	updateRequestCommonMetaField(ctx, func(m *RequestCommonMeta) { m.ForwardedHost = host })
	return ctx
}

// GetForwardedHost 从上下文获取转发域名
func GetForwardedHost(ctx context.Context) string {
	if meta := GetRequestCommonMeta(ctx); meta != nil {
		return meta.ForwardedHost
	}
	return contextx.GetValue[string](ctx, constants.MetadataForwardedHost)
}

// ContextBuilder 上下文字段链式构建器
type ContextBuilder struct {
	ctx context.Context
}

// NewContextBuilder 创建上下文构建器
//
// 用法：
//
//	ctx = NewContextBuilder(ctx).
//	    WithTraceID("trace-123").
//	    WithUserID("user-456").
//	    WithTenantID("tenant-789").
//	    Build()
func NewContextBuilder(ctx context.Context) *ContextBuilder {
	return &ContextBuilder{ctx: ctx}
}

// Build 构建并返回最终的 context
func (b *ContextBuilder) Build() context.Context {
	return b.ctx
}

// ---- 链式 With* 方法 ----

func (b *ContextBuilder) WithTraceID(traceID string) *ContextBuilder {
	b.ctx = WithTraceID(b.ctx, traceID)
	return b
}

func (b *ContextBuilder) WithRequestID(requestID string) *ContextBuilder {
	b.ctx = WithRequestID(b.ctx, requestID)
	return b
}

func (b *ContextBuilder) WithID(id string) *ContextBuilder {
	b.ctx = WithID(b.ctx, id)
	return b
}

func (b *ContextBuilder) WithUserID(userID string) *ContextBuilder {
	b.ctx = WithUserID(b.ctx, userID)
	return b
}

func (b *ContextBuilder) WithUserName(userName string) *ContextBuilder {
	b.ctx = WithUserName(b.ctx, userName)
	return b
}

func (b *ContextBuilder) WithDomain(domain string) *ContextBuilder {
	b.ctx = WithDomain(b.ctx, domain)
	return b
}

func (b *ContextBuilder) WithRoleCode(roleCode string) *ContextBuilder {
	b.ctx = WithRoleCode(b.ctx, roleCode)
	return b
}

func (b *ContextBuilder) WithTenantID(tenantID string) *ContextBuilder {
	b.ctx = WithTenantID(b.ctx, tenantID)
	return b
}

func (b *ContextBuilder) WithTenantCode(tenantCode string) *ContextBuilder {
	b.ctx = WithTenantCode(b.ctx, tenantCode)
	return b
}

func (b *ContextBuilder) WithSessionID(sessionID string) *ContextBuilder {
	b.ctx = WithSessionID(b.ctx, sessionID)
	return b
}

func (b *ContextBuilder) WithTimezone(timezone string) *ContextBuilder {
	b.ctx = WithTimezone(b.ctx, timezone)
	return b
}

func (b *ContextBuilder) WithTimestamp(timestamp string) *ContextBuilder {
	b.ctx = WithTimestamp(b.ctx, timestamp)
	return b
}

func (b *ContextBuilder) WithSignature(signature string) *ContextBuilder {
	b.ctx = WithSignature(b.ctx, signature)
	return b
}

func (b *ContextBuilder) WithAuthorization(authorization string) *ContextBuilder {
	b.ctx = WithAuthorization(b.ctx, authorization)
	return b
}

func (b *ContextBuilder) WithAccessKey(accessKey string) *ContextBuilder {
	b.ctx = WithAccessKey(b.ctx, accessKey)
	return b
}

func (b *ContextBuilder) WithAppID(appID string) *ContextBuilder {
	b.ctx = WithAppID(b.ctx, appID)
	return b
}

func (b *ContextBuilder) WithDeviceID(deviceID string) *ContextBuilder {
	b.ctx = WithDeviceID(b.ctx, deviceID)
	return b
}

func (b *ContextBuilder) WithAppVersion(appVersion string) *ContextBuilder {
	b.ctx = WithAppVersion(b.ctx, appVersion)
	return b
}

func (b *ContextBuilder) WithIPAddress(ipAddress string) *ContextBuilder {
	b.ctx = WithIPAddress(b.ctx, ipAddress)
	return b
}

func (b *ContextBuilder) WithPlatformID(platformID string) *ContextBuilder {
	b.ctx = WithPlatformID(b.ctx, platformID)
	return b
}

func (b *ContextBuilder) WithPlatformCode(platformCode string) *ContextBuilder {
	b.ctx = WithPlatformCode(b.ctx, platformCode)
	return b
}

func (b *ContextBuilder) WithRegionID(regionID string) *ContextBuilder {
	b.ctx = WithRegionID(b.ctx, regionID)
	return b
}

func (b *ContextBuilder) WithRegionCode(regionCode string) *ContextBuilder {
	b.ctx = WithRegionCode(b.ctx, regionCode)
	return b
}

func (b *ContextBuilder) WithAgentLineID(agentLineID string) *ContextBuilder {
	b.ctx = WithAgentLineID(b.ctx, agentLineID)
	return b
}

func (b *ContextBuilder) WithNonce(nonce string) *ContextBuilder {
	b.ctx = WithNonce(b.ctx, nonce)
	return b
}

func (b *ContextBuilder) WithJti(jti string) *ContextBuilder {
	b.ctx = WithJti(b.ctx, jti)
	return b
}

func (b *ContextBuilder) WithFamilyId(familyId string) *ContextBuilder {
	b.ctx = WithFamilyId(b.ctx, familyId)
	return b
}

func (b *ContextBuilder) WithXNsID(xNsID string) *ContextBuilder {
	b.ctx = WithXNsID(b.ctx, xNsID)
	return b
}

func (b *ContextBuilder) WithUserAgent(userAgent string) *ContextBuilder {
	b.ctx = WithUserAgent(b.ctx, userAgent)
	return b
}

func (b *ContextBuilder) WithPushToken(pushToken string) *ContextBuilder {
	b.ctx = WithPushToken(b.ctx, pushToken)
	return b
}

func (b *ContextBuilder) WithToken(token string) *ContextBuilder {
	b.ctx = WithToken(b.ctx, token)
	return b
}

func (b *ContextBuilder) WithAcceptLanguage(acceptLanguage string) *ContextBuilder {
	b.ctx = WithAcceptLanguage(b.ctx, acceptLanguage)
	return b
}

func (b *ContextBuilder) WithForwardedHost(forwardedHost string) *ContextBuilder {
	b.ctx = WithForwardedHost(b.ctx, forwardedHost)
	return b
}
