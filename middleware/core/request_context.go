/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-29 10:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-19 18:18:50
 * @FilePath: \go-rpc-gateway\middleware\core\request_context.go
 * @Description: 统一的请求上下文中间件 - 实现 HTTP → gRPC → Service → Repository 全链路上下文传递
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package core

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/contextx"
	goi18n "github.com/kamalyes/go-toolbox/pkg/i18n"
	"github.com/kamalyes/go-toolbox/pkg/logger"
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

// requestMetaContext 一层 context 同时应答两个键：RequestCommonMeta 本身，
// 以及 go-logger 用作兜底的 trace_id 键
//
// 改造前是两层：WithRequestCommonMeta 存 meta，再 contextx.WithValue 存一次 trace_id
// 两层意味着每个请求两次分配，而且此后每一次 ctx.Value 查找都要多走一层链
// 合成一层后 trace_id 直接从 meta 读，值与原来逐字相同
//
// 用类型断言而不是 switch key：context 的键可以是不可比较类型，
// 对不可比较的动态类型做 == 会 panic，而断言不会
//
// [EN] One context node answering both keys: the meta itself and the trace id key
// go-logger falls back to
// This used to be two layers, which meant two allocations per request and one extra
// link for every later ctx.Value lookup; merged, the trace id is read straight from
// the meta and is byte-for-byte what the second layer stored
// Type assertions rather than a switch on key: context keys may be uncomparable, and
// == on an uncomparable dynamic type panics while an assertion does not
type requestMetaContext struct {
	context.Context
	meta *RequestCommonMeta
	// traceID 建节点时就把 trace_id 装箱好
	//
	// 直接在 Value 里 return c.meta.TraceID 会让每次查找都把 string 装箱进 any，
	// 也就是每次查找一次分配 合并成一层之前 context.WithValue 是在设值时装箱一次、
	// 查找免费，所以不预装箱的话这一层合并在 trace_id 被查一次以上时就是回归
	// alloc profile 里这一项曾占链路前三层分配的 3.12%
	//
	// 取舍说清楚：预装箱在建节点时固定付一次分配，查零次时比不装箱多一次，
	// 查一次打平，查两次及以上才划算 默认链上访问日志中间件会查它，
	// 实测链路前三层 33 到 30 次分配，而只有前两层（不含访问日志）是 17 到 18
	// 选预装箱是因为默认配置下访问日志是开的
	//
	// 不做「首次查找时装箱并缓存」：context 会跨 goroutine 共享，
	// 惰性写字段就是数据竞争，加锁或原子的成本高于省下的那一次分配
	//
	// [EN] The trace id is boxed once when the node is built
	// Returning c.meta.TraceID straight from Value boxes the string into an any on every
	// lookup; before the merge context.WithValue boxed once at set time and lookups were
	// free, so without pre-boxing the merge is a regression past one lookup
	// The trade-off stated plainly: pre-boxing always costs one allocation at construction,
	// so it loses by one at zero lookups, breaks even at one and only wins from two up
	// The default chain's access-log middleware does read it, measured as 33 to 30
	// allocations for the first three layers against 17 to 18 for the first two without it,
	// and pre-boxing is chosen because the access log is on by default
	// Boxing lazily on first lookup is rejected: a context is shared across goroutines, so
	// writing the field lazily is a data race, and a lock or atomic costs more than the
	// single allocation it would save
	traceID any
}

func (c *requestMetaContext) Value(key any) any {
	if _, ok := key.(requestCommonMetaKey); ok {
		return c.meta
	}
	if s, ok := key.(string); ok && s == logger.ContextKeyTraceID {
		return c.traceID
	}
	return c.Context.Value(key)
}

// WithRequestCommonMeta 为上下文添加请求公共元信息
// 同时让 go-logger 的 trace_id 兜底键可读，调用方不必再单独存一次
//
// [EN] WithRequestCommonMeta attaches the request meta and also serves go-logger's
// trace id fallback key, so callers need not store it separately
func WithRequestCommonMeta(ctx context.Context, requestCommonMeta *RequestCommonMeta) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if requestCommonMeta == nil {
		return ctx
	}
	return &requestMetaContext{
		Context: ctx,
		meta:    requestCommonMeta,
		traceID: requestCommonMeta.TraceID,
	}
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

			// 走 GetGatewayConfig 而不是直接读 global.GATEWAY：后者是普通包级变量，
			// 与热重载的写入构成数据竞态，race detector 实测就指在这一行
			// 顺带补上 nil 保护 —— 原先 global.GATEWAY 为 nil 时这里直接 panic，
			// 而 buildAttrIndex 本来就能接受 nil 配置（返回一张空索引）
			//
			// [EN] GetGatewayConfig rather than a direct read of global.GATEWAY: that plain
			// package-level variable races with the hot-reload write, and the race detector
			// pointed at this very line. The nil guard comes along for free: this used to panic
			// outright when global.GATEWAY was nil, while buildAttrIndex already accepts a nil
			// config and answers with an empty index.
			var requestContext *gwconfig.RequestContext
			if cfg := global.GetGatewayConfig(); cfg != nil {
				requestContext = cfg.RequestContext
			}

			// 按来源提取的 28 个属性走反转索引：只遍历请求实际带的 header，
			// 而不是把配置里 80 个来源逐个试一遍
			// 索引按配置建一次，热重载换了配置会自动重建，见 request_attr_index.go
			//
			// [EN] The 28 source-driven attributes go through the inverted index, walking
			// only the headers the request carries instead of trying all 80 configured
			// sources; the index is built once per config, see request_attr_index.go
			requestCommonMeta := &RequestCommonMeta{}
			fillFromRequest(requestAttrIndex(requestContext), r, requestCommonMeta)

			// 以下几项不是「按来源清单取第一个非空」的形态，单独处理
			// TraceID 与 RequestID 取不到时要兜底生成；IPAddress 走 netx 自己的多 header
			// 优先级逻辑；UserAgent 直接取标准库的解析结果
			//
			// [EN] These are not "first non-empty from a source list" and are handled apart:
			// TraceID and RequestID fall back to generation, IPAddress follows netx's own
			// multi-header precedence, and UserAgent comes from the standard library
			requestCommonMeta.TraceID = extractOrGenerateTraceID(ctx, requestCommonMeta.TraceID)
			requestCommonMeta.RequestID = extractOrGenerateRequestID(requestCommonMeta.RequestID)
			requestCommonMeta.IPAddress = netx.GetClientIP(r)
			requestCommonMeta.UserAgent = r.UserAgent()

			// 单层 context 同时带上 meta 与 trace_id，见 requestMetaContext
			// [EN] A single context node carries both the meta and the trace id
			ctx = WithRequestCommonMeta(ctx, requestCommonMeta)

			// 5. 设置响应头（便于客户端追踪）
			w.Header().Set(constants.HeaderXTraceID, requestCommonMeta.TraceID)
			w.Header().Set(constants.HeaderXRequestID, requestCommonMeta.RequestID)
			// 注入服务节点标识，便于在 K8s 等环境中定位处理请求的具体 Pod
			if serverNode := global.GetServerNode(); serverNode != "" {
				w.Header().Set(constants.HeaderXServerNode, serverNode)
			}

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
		TraceID:        contextx.GetValue[string](ctx, logger.ContextKeyTraceID),
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
		wrappedStream := &ContextWrappedServerStream{
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

	userName, _ := url.QueryUnescape(firstMetadataValue(constants.MetadataUserName))

	// 提取所有字段，构建 RequestCommonMeta
	meta := &RequestCommonMeta{
		ID:             firstMetadataValue(constants.MetadataID),
		TraceID:        extractOrGenerateTraceID(ctx, firstMetadataValue(constants.MetadataTraceID)),
		RequestID:      extractOrGenerateRequestID(firstMetadataValue(constants.MetadataRequestID)),
		Authorization:  firstMetadataValue(constants.MetadataAuthorization),
		UserID:         firstMetadataValue(constants.MetadataUserID),
		UserName:       userName,
		Domain:         firstMetadataValue(constants.MetadataDomain),
		RoleCode:       firstMetadataValue(constants.MetadataRoleCode),
		TenantID:       firstMetadataValue(constants.MetadataTenantID),
		TenantCode:     firstMetadataValue(constants.MetadataTenantCode),
		SessionID:      firstMetadataValue(constants.MetadataSessionID),
		Timezone:       firstMetadataValue(constants.MetadataTimezone),
		IPAddress:      firstMetadataValue(constants.MetadataIPAddress),
		AppID:          firstMetadataValue(constants.MetadataAppID),
		DeviceID:       firstMetadataValue(constants.MetadataDeviceID),
		AppVersion:     firstMetadataValue(constants.MetadataAppVersion),
		PlatformID:     firstMetadataValue(constants.MetadataPlatformID),
		PlatformCode:   firstMetadataValue(constants.MetadataPlatformCode),
		RegionID:       firstMetadataValue(constants.MetadataRegionID),
		RegionCode:     firstMetadataValue(constants.MetadataRegionCode),
		AgentLineID:    firstMetadataValue(constants.MetadataAgentLineID),
		Nonce:          firstMetadataValue(constants.MetadataNonce),
		Jti:            firstMetadataValue(constants.MetadataJti),
		FamilyId:       firstMetadataValue(constants.MetadataFamilyId),
		XNsID:          firstMetadataValue(constants.MetadataXNsID),
		UserAgent:      firstMetadataValue(constants.MetadataUserAgent),
		PushToken:      firstMetadataValue(constants.MetadataPushToken),
		Token:          firstMetadataValue(constants.MetadataToken),
		Timestamp:      firstMetadataValue(constants.MetadataTimestamp),
		Signature:      firstMetadataValue(constants.MetadataSignature),
		AccessKey:      firstMetadataValue(constants.MetadataAccessKey),
		AcceptLanguage: firstMetadataValue(constants.MetadataAcceptLanguage),
		ForwardedHost:  firstMetadataValue(constants.MetadataForwardedHost),
	}

	// 单层 context 同时带上 meta 与 trace_id，见 requestMetaContext
	// [EN] A single context node carries both the meta and the trace id
	return WithRequestCommonMeta(ctx, meta)
}

// setResponseMetadata 设置 gRPC 响应 metadata（与 HTTP 的 w.Header().Set 对应）
//
// 与出站注入一样跳过空值，理由同 injectTraceToOutgoingContext：空值在读取侧与键缺失
// 不可区分，却要实打实占一条响应头
// 顺带去掉了原先永真的 if len(md) > 0 —— metadata.Pairs 对空值也建条目，
// 那个判断在改造前从来没有为假过
//
// 凭据类字段不再回显 原因是这些值并不会「留在 gRPC 层」：
// server/http.go 的 buildServeMuxOptions 只设了 WithIncomingHeaderMatcher，
// 出站方向走 grpc-gateway 的 defaultOutgoingHeaderMatcher，而它对**每一个**键都返回
// (Grpc-Metadata-<键>, true)，于是每条响应 metadata 都会变成 HTTP 响应头发给客户端
// 也就是说改造前每个响应都带着 Grpc-Metadata-Authorization: Bearer … 回去
// 这些值本来就是客户端自己送上来的，回显一遍不增加任何信息，却让凭据进入响应头 ——
// 响应头比请求头更容易被反向代理、CDN、浏览器 devtools 和共享缓存记录下来
//
// 只摘掉持有即可用的凭据（Authorization / Token / AccessKey / Signature / PushToken）
// Jti、FamilyId、Nonce 保留：它们是标识符而非凭据，且对排查链路有用
//
// [EN] Empty values are skipped for the same reason as in injectTraceToOutgoingContext: they
// are indistinguishable from an absent key on the reading side yet still occupy a response
// header. The previously always-true `if len(md) > 0` is gone with them, since metadata.Pairs
// creates an entry for an empty value too and that test was never false.
//
// Credential fields are no longer echoed. These values do not stay at the gRPC layer:
// buildServeMuxOptions in server/http.go sets only WithIncomingHeaderMatcher, so the outgoing
// direction uses grpc-gateway's defaultOutgoingHeaderMatcher, which returns
// (Grpc-Metadata-<key>, true) for *every* key — every response metadata entry becomes an HTTP
// response header sent to the client. Before this change every response carried
// Grpc-Metadata-Authorization: Bearer … back out. The client supplied those values itself, so
// echoing them adds no information while putting credentials into response headers, which
// reverse proxies, CDNs, browser devtools and shared caches log far more readily than request
// headers.
//
// Only bearer-style credentials are withheld (Authorization, Token, AccessKey, Signature,
// PushToken). Jti, FamilyId and Nonce stay: they are identifiers rather than credentials and
// are useful when tracing a request.
func setResponseMetadata(ctx context.Context) {
	md := buildResponseMetadata(ctx)
	if md == nil {
		return
	}
	// 发送 metadata（忽略错误，因为可能已经发送过）
	grpc.SetHeader(ctx, md)
}

// buildResponseMetadata 构造响应 metadata，与发送动作分开以便单测
// 没有任何可发送字段时返回 nil
//
// [EN] Builds the response metadata separately from sending it, so it can be unit-tested;
// returns nil when there is nothing to send
func buildResponseMetadata(ctx context.Context) metadata.MD {
	requestCommonMeta := GetRequestCommonMeta(ctx)

	var buf [34 * 2]string
	kv := buf[:0]
	add := func(key, value string) {
		if value != "" {
			kv = append(kv, key, value)
		}
	}

	add(constants.MetadataID, requestCommonMeta.ID)
	add(constants.MetadataTraceID, requestCommonMeta.TraceID)
	add(constants.MetadataRequestID, requestCommonMeta.RequestID)
	add(constants.MetadataUserID, requestCommonMeta.UserID)
	add(constants.MetadataUserName, url.QueryEscape(requestCommonMeta.UserName))
	add(constants.MetadataDomain, requestCommonMeta.Domain)
	add(constants.MetadataRoleCode, requestCommonMeta.RoleCode)
	add(constants.MetadataTenantID, requestCommonMeta.TenantID)
	add(constants.MetadataTenantCode, requestCommonMeta.TenantCode)
	add(constants.MetadataSessionID, requestCommonMeta.SessionID)
	add(constants.MetadataTimezone, requestCommonMeta.Timezone)
	add(constants.MetadataIPAddress, requestCommonMeta.IPAddress)
	add(constants.MetadataAppID, requestCommonMeta.AppID)
	add(constants.MetadataDeviceID, requestCommonMeta.DeviceID)
	add(constants.MetadataAppVersion, requestCommonMeta.AppVersion)
	add(constants.MetadataPlatformID, requestCommonMeta.PlatformID)
	add(constants.MetadataPlatformCode, requestCommonMeta.PlatformCode)
	add(constants.MetadataRegionID, requestCommonMeta.RegionID)
	add(constants.MetadataRegionCode, requestCommonMeta.RegionCode)
	add(constants.MetadataAgentLineID, requestCommonMeta.AgentLineID)
	add(constants.MetadataNonce, requestCommonMeta.Nonce)
	add(constants.MetadataJti, requestCommonMeta.Jti)
	add(constants.MetadataFamilyId, requestCommonMeta.FamilyId)
	add(constants.MetadataXNsID, requestCommonMeta.XNsID)
	add(constants.MetadataUserAgent, requestCommonMeta.UserAgent)
	add(constants.MetadataTimestamp, requestCommonMeta.Timestamp)
	add(constants.MetadataAcceptLanguage, requestCommonMeta.AcceptLanguage)
	add(constants.MetadataForwardedHost, requestCommonMeta.ForwardedHost)

	// 注入服务节点标识（K8s 环境下为 Pod 名称），便于下游服务定位处理请求的具体节点
	add(constants.MetadataServerNode, global.GetServerNode())

	if len(kv) == 0 {
		return nil
	}

	md := make(metadata.MD, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		md[kv[i]] = append(md[kv[i]], kv[i+1])
	}
	return md
}

// responseMetadataWithheldKeys 不回显到响应 metadata 的凭据类键
//
// 由 TestResponseMetadataWithholdsCredentials 用来断言这些键确实不出现在响应里
// [EN] Credential keys withheld from response metadata, asserted by
// TestResponseMetadataWithholdsCredentials
var responseMetadataWithheldKeys = []string{
	constants.MetadataAuthorization,
	constants.MetadataToken,
	constants.MetadataAccessKey,
	constants.MetadataSignature,
	constants.MetadataPushToken,
}

// ContextWrappedServerStream 包装 grpc.ServerStream 以支持自定义 context
// [EN] ContextWrappedServerStream wraps a grpc.ServerStream so that a customized context is served
type ContextWrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// NewContextWrappedServerStream 用指定 context 包装 ServerStream，供其它子包复用
// [EN] NewContextWrappedServerStream wraps the stream with the given context, for reuse by sibling subpackages
func NewContextWrappedServerStream(ss grpc.ServerStream, ctx context.Context) *ContextWrappedServerStream {
	return &ContextWrappedServerStream{ServerStream: ss, ctx: ctx}
}

// Context 返回增强后的 context
// [EN] Context returns the enriched context
func (w *ContextWrappedServerStream) Context() context.Context {
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
//
// 这条路径是网关代理的最热点：每一次下游 RPC 调用都要走一遍
//
// 空值不再注入 这不是「少传点东西」的取舍，而是空值本来就传不出信息：
// 下游读取走 enrichContextFromMetadata 的 firstMetadataValue，它用 md.Get(key) 取
// 第一个值，「键不存在」与「键存在但值为空」返回的都是 ""，两者对读取方完全等价
// 而每一条空值都要实打实经 HTTP/2 HPACK 编码送到下游 实测常态请求
// （只带 trace/request id）33 条里有 30 条是空的，也就是 90% 的注入是纯浪费
//
// 不再走 Pairs + Join + NewOutgoingContext，改用 AppendToOutgoingContext：
// Join 的 out 是 MD{} 建的，没有容量提示，35 个键要经历多次 rehash；
// 而 FromOutgoingContext 为了拿到 existingMD 会把整张基础 map 连同每个值切片
// 全量复制一遍（copyOf），复制完只是为了再被 Join 复制第二遍
// AppendToOutgoingContext 压根不碰基础 map，只往 rawMD.added 挂一条键值序列
//
// 顺序语义逐字保持：FromOutgoingContext 先铺 raw.md 再按序合并 raw.added，
// 于是「已有值在前、本层注入在后」与原先 Join(existingMD, md) 完全一致，
// 下游 firstMetadataValue 取 values[0] 拿到的仍是已有值
//
// 键全部直接用常量不再过 ToLower：constants 里 35 个 Metadata* 常量已全是小写
// （TestMetadataKeysAreLowercase 锁住这个前提），AppendToOutgoingContext 内部
// 那次 ToLower 对它们是空操作
//
// [EN] The hottest path in the proxy: every downstream RPC goes through it.
//
// Empty values are no longer injected. This is not a trade-off of sending less: an empty
// value carries no information in the first place, because the downstream reads through
// firstMetadataValue, which takes md.Get(key)[0], and "key absent" and "key present but
// empty" both yield "" there -- indistinguishable to every reader. Meanwhile each empty
// value really is HPACK-encoded onto the wire: measured, a common request carrying only
// trace and request ids sent 33 entries of which 30 were empty, so 90% was pure waste.
//
// Pairs + Join + NewOutgoingContext is replaced by AppendToOutgoingContext. Join builds its
// output from MD{} with no capacity hint, so 35 keys cost several rehashes, and getting
// existingMD at all makes FromOutgoingContext deep-copy the whole base map plus every value
// slice, only for Join to copy it a second time. AppendToOutgoingContext never touches the
// base map and appends one key/value sequence to rawMD.added.
//
// The ordering semantics are preserved exactly: FromOutgoingContext lays down raw.md first
// and then merges raw.added in order, so "existing values first, this layer appended after"
// matches the previous Join(existingMD, md), and the downstream firstMetadataValue still
// reads the existing value at values[0].
//
// Keys go in straight from the constants without ToLower: all 35 Metadata* constants are
// already lowercase (locked by TestMetadataKeysAreLowercase), so the ToLower inside
// AppendToOutgoingContext is a no-op for them.
func injectTraceToOutgoingContext(ctx context.Context) context.Context {
	requestCommonMeta := GetRequestCommonMeta(ctx)

	// 优先使用 RequestCommonMeta 中的 Accept Language，若为空则从 i18n context 获取
	acceptLanguage := mathx.IfEmpty(requestCommonMeta.AcceptLanguage, goi18n.GetLanguage(ctx))

	// UserName 仍按原先的方式转义后再判空：QueryEscape 不会把非空串变成空串，
	// 所以「转义后为空」等价于「原值为空」，判空位置前后移动不改变结果
	// [EN] UserName is still escaped before the emptiness test: QueryEscape never turns a
	// non-empty string into an empty one, so testing before or after is equivalent
	userName := url.QueryEscape(requestCommonMeta.UserName)

	// 固定长度的栈上数组，避免为键值序列再分配一次
	// 34 对 = 33 个字段加服务节点标识
	// [EN] A fixed-size stack array avoids a further allocation for the key/value sequence;
	// 34 pairs is the 33 fields plus the server node identifier
	var buf [34 * 2]string
	kv := buf[:0]
	add := func(key, value string) {
		if value != "" {
			kv = append(kv, key, value)
		}
	}

	add(constants.MetadataID, requestCommonMeta.ID)
	add(constants.MetadataTraceID, requestCommonMeta.TraceID)
	add(constants.MetadataRequestID, requestCommonMeta.RequestID)
	add(constants.MetadataAuthorization, requestCommonMeta.Authorization)
	add(constants.MetadataUserID, requestCommonMeta.UserID)
	add(constants.MetadataUserName, userName)
	add(constants.MetadataDomain, requestCommonMeta.Domain)
	add(constants.MetadataRoleCode, requestCommonMeta.RoleCode)
	add(constants.MetadataTenantID, requestCommonMeta.TenantID)
	add(constants.MetadataTenantCode, requestCommonMeta.TenantCode)
	add(constants.MetadataSessionID, requestCommonMeta.SessionID)
	add(constants.MetadataTimezone, requestCommonMeta.Timezone)
	add(constants.MetadataIPAddress, requestCommonMeta.IPAddress)
	add(constants.MetadataAppID, requestCommonMeta.AppID)
	add(constants.MetadataDeviceID, requestCommonMeta.DeviceID)
	add(constants.MetadataAppVersion, requestCommonMeta.AppVersion)
	add(constants.MetadataPlatformID, requestCommonMeta.PlatformID)
	add(constants.MetadataPlatformCode, requestCommonMeta.PlatformCode)
	add(constants.MetadataRegionID, requestCommonMeta.RegionID)
	add(constants.MetadataRegionCode, requestCommonMeta.RegionCode)
	add(constants.MetadataAgentLineID, requestCommonMeta.AgentLineID)
	add(constants.MetadataNonce, requestCommonMeta.Nonce)
	add(constants.MetadataJti, requestCommonMeta.Jti)
	add(constants.MetadataFamilyId, requestCommonMeta.FamilyId)
	add(constants.MetadataXNsID, requestCommonMeta.XNsID)
	add(constants.MetadataUserAgent, requestCommonMeta.UserAgent)
	add(constants.MetadataPushToken, requestCommonMeta.PushToken)
	add(constants.MetadataToken, requestCommonMeta.Token)
	add(constants.MetadataTimestamp, requestCommonMeta.Timestamp)
	add(constants.MetadataSignature, requestCommonMeta.Signature)
	add(constants.MetadataAccessKey, requestCommonMeta.AccessKey)
	add(constants.MetadataAcceptLanguage, acceptLanguage)
	add(constants.MetadataForwardedHost, requestCommonMeta.ForwardedHost)

	// 透传服务节点标识到下游服务，便于全链路定位处理请求的具体节点
	add(constants.MetadataServerNode, global.GetServerNode())

	if len(kv) == 0 {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, kv...)
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
	// 预置容量：原先是 metadata.New(nil)，也就是从空 map 起步，
	// 一个浏览器请求十几个头要经历几次 rehash
	// [EN] Pre-sized: this used to start from an empty map via metadata.New(nil), so a
	// browser's dozen-plus headers cost several rehashes
	md := make(metadata.MD, len(r.Header))
	for key, values := range r.Header {
		// 逐跳头必须剔掉 原先只剔了 Connection，而把 te 或 transfer-encoding
		// 转进 gRPC metadata 会让 gRPC 服务端直接回 RST_STREAM PROTOCOL_ERROR ——
		// 不是「多传了点东西」，是这一路请求整个失败
		//
		// 清单与 server/http.go 的 WithIncomingHeaderMatcher 共用同一个判定（见
		// IsHopByHopHeader）：同一条规则散在两处，就必然出现一处全、另一处只剩一项的情况，
		// 这个 bug 本身就是那样来的
		//
		// [EN] Hop-by-hop headers have to go. Only Connection used to be dropped, while
		// forwarding te or transfer-encoding into gRPC metadata makes the gRPC server answer
		// RST_STREAM PROTOCOL_ERROR — not "a little extra on the wire" but the whole request
		// failing. The list is shared with WithIncomingHeaderMatcher in server/http.go through
		// IsHopByHopHeader: one rule spelled out in two places is how one copy ends up complete
		// and the other down to a single entry, which is exactly how this bug arose
		if IsHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			md.Append(key, value)
		}
	}
	return metadata.NewOutgoingContext(r.Context(), md)
}

// IsHopByHopHeader 报告该 HTTP 头是否为逐跳头，不得转发到下游
//
// 这些头描述的是「本次 TCP 连接」而不是「这个请求」，按 RFC 9110 §7.6.1 不可转发
// 对 gRPC 来说后果更硬：HTTP/2 禁止 connection 相关头，gRPC 服务端收到就发
// RST_STREAM PROTOCOL_ERROR，整路请求失败
//
// 清单与 grpc-gateway 的入站匹配器保持一致（server/http.go 的 buildServeMuxOptions）
// RFC 还把 Proxy-Authenticate / Proxy-Authorization 也列为逐跳，但此处刻意不加：
// 两条路径的判定必须完全一致，而扩大清单是一个独立的行为决定，不该夹在 bug 修复里
//
// [EN] Reports whether an HTTP header is hop-by-hop and must not be forwarded downstream.
// These describe the TCP connection rather than the request and are non-forwardable per
// RFC 9110 §7.6.1. For gRPC the consequence is harder: HTTP/2 forbids connection-specific
// headers and a gRPC server answers RST_STREAM PROTOCOL_ERROR, failing the whole request.
//
// The list matches grpc-gateway's incoming matcher in buildServeMuxOptions. The RFC also lists
// Proxy-Authenticate and Proxy-Authorization as hop-by-hop, deliberately not added here: both
// paths must agree exactly, and widening the list is a separate behavioural decision that does
// not belong inside a bug fix
func IsHopByHopHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "keep-alive", "proxy-connection",
		"transfer-encoding", "upgrade", "te":
		return true
	}
	return false
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
	ctx = contextx.WithValue(ctx, logger.ContextKeyTraceID, traceID)
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
