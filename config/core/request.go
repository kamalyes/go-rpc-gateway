/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-21 00:16:15
 * @FilePath: \go-rpc-gateway\config\core\request.go
 * @Description: Gateway 统一请求字段提取配置
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package core

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// RequestContext Gateway 统一请求上下文提取配置
// 用于定义 RequestCommon 所需字段从哪些来源提取，供所有 middleware 共享
type RequestContext struct {
	// 标准请求头
	AuthorizationSources []AttributeSource `mapstructure:"authorization-sources" yaml:"authorization-sources" json:"authorizationSources"` // Authorization 提取来源
	AuthPayloadSources   []AttributeSource `mapstructure:"auth-payload-sources" yaml:"auth-payload-sources" json:"authPayloadSources"`     // Auth-Payload 提取来源
	JtiSources           []AttributeSource `mapstructure:"jti-sources" yaml:"jti-sources" json:"jtiSources"`                               // Jti 提取来源
	FamilyIdSources      []AttributeSource `mapstructure:"family-id-sources" yaml:"family-id-sources" json:"familyIdSources"`              // FamilyId 提取来源
	UserAgentSources     []AttributeSource `mapstructure:"user-agent-sources" yaml:"user-agent-sources" json:"userAgentSources"`           // User-Agent 提取来源
	AcceptSources        []AttributeSource `mapstructure:"accept-sources" yaml:"accept-sources" json:"acceptSources"`                      // Accept 提取来源
	CacheControlSources  []AttributeSource `mapstructure:"cache-control-sources" yaml:"cache-control-sources" json:"cacheControlSources"`  // Cache-Control 提取来源
	ConnectionSources    []AttributeSource `mapstructure:"connection-sources" yaml:"connection-sources" json:"connectionSources"`          // Connection 提取来源

	// 自定义请求头
	RequestIDSources      []AttributeSource `mapstructure:"request-id-sources" yaml:"request-id-sources" json:"requestIdSources"`                // X-Request-Id 提取来源
	TraceIDSources        []AttributeSource `mapstructure:"trace-id-sources" yaml:"trace-id-sources" json:"traceIdSources"`                      // X-Trace-Id 提取来源
	RealIPSources         []AttributeSource `mapstructure:"real-ip-sources" yaml:"real-ip-sources" json:"realIpSources"`                         // X-Real-IP 提取来源
	ForwardedForSources   []AttributeSource `mapstructure:"forwarded-for-sources" yaml:"forwarded-for-sources" json:"forwardedForSources"`       // X-Forwarded-For 提取来源
	ForwardedProtoSources []AttributeSource `mapstructure:"forwarded-proto-sources" yaml:"forwarded-proto-sources" json:"forwardedProtoSources"` // X-Forwarded-Proto 提取来源
	ForwardedHostSources  []AttributeSource `mapstructure:"forwarded-host-sources" yaml:"forwarded-host-sources" json:"forwardedHostSources"`    // X-Forwarded-Host 提取来源

	// 用户上下文相关头部
	ClientIDSources   []AttributeSource `mapstructure:"client-id-sources" yaml:"client-id-sources" json:"clientIdSources"`       // X-Client-ID 提取来源
	UserIDSources     []AttributeSource `mapstructure:"user-id-sources" yaml:"user-id-sources" json:"userIdSources"`             // X-User-ID 提取来源
	UserTypeSources   []AttributeSource `mapstructure:"user-type-sources" yaml:"user-type-sources" json:"userTypeSources"`       // X-User-Type 提取来源
	TenantIDSources   []AttributeSource `mapstructure:"tenant-id-sources" yaml:"tenant-id-sources" json:"tenantIdSources"`       // X-Tenant-ID 提取来源
	TenantCodeSources []AttributeSource `mapstructure:"tenant-code-sources" yaml:"tenant-code-sources" json:"tenantCodeSources"` // X-Tenant-Code 提取来源
	SessionIDSources  []AttributeSource `mapstructure:"session-id-sources" yaml:"session-id-sources" json:"sessionIdSources"`    // X-Session-ID 提取来源
	TimezoneSources   []AttributeSource `mapstructure:"timezone-sources" yaml:"timezone-sources" json:"timezoneSources"`         // X-Timezone 提取来源
	IDSources         []AttributeSource `mapstructure:"id-sources" yaml:"id-sources" json:"idSources"`                           // X-ID 提取来源
	DomainSources     []AttributeSource `mapstructure:"domain-sources" yaml:"domain-sources" json:"domainSources"`               // X-Domain 提取来源
	RoleCodeSources   []AttributeSource `mapstructure:"role-code-sources" yaml:"role-code-sources" json:"roleCodeSources"`       // X-Role-Code 提取来源
	PushTokenSources  []AttributeSource `mapstructure:"push-token-sources" yaml:"push-token-sources" json:"pushTokenSources"`    // X-Push-Token 提取来源
	TokenSources      []AttributeSource `mapstructure:"token-sources" yaml:"token-sources" json:"tokenSources"`                  // X-Token 提取来源

	// 设备和应用相关头部
	DeviceIDSources     []AttributeSource `mapstructure:"device-id-sources" yaml:"device-id-sources" json:"deviceIdSources"`             // X-Device-Id / X-Device-ID 提取来源
	AppIDSources        []AttributeSource `mapstructure:"app-id-sources" yaml:"app-id-sources" json:"appIdSources"`                      // X-App-Id 提取来源
	AppVersionSources   []AttributeSource `mapstructure:"app-version-sources" yaml:"app-version-sources" json:"appVersionSources"`       // X-App-Version 提取来源
	PlatformSources     []AttributeSource `mapstructure:"platform-sources" yaml:"platform-sources" json:"platformSources"`               // X-Platform 提取来源
	PlatformIDSources   []AttributeSource `mapstructure:"platform-id-sources" yaml:"platform-id-sources" json:"platformIDSources"`       // X-Platform-ID 提取来源
	PlatformCodeSources []AttributeSource `mapstructure:"platform-code-sources" yaml:"platform-code-sources" json:"platformCodeSources"` // X-Platform-Code 提取来源
	RegionIDSources     []AttributeSource `mapstructure:"region-id-sources" yaml:"region-id-sources" json:"regionIDSources"`             // X-Region-ID 提取来源
	RegionCodeSources   []AttributeSource `mapstructure:"region-code-sources" yaml:"region-code-sources" json:"regionCodeSources"`       // X-Region-Code 提取来源
	AgentLineIDSources  []AttributeSource `mapstructure:"agent-line-id-sources" yaml:"agent-line-id-sources" json:"agentLineIDSources"`  // X-Agent-Line 提取来源
	TimestampSources    []AttributeSource `mapstructure:"timestamp-sources" yaml:"timestamp-sources" json:"timestampSources"`            // X-Timestamp 提取来源
	SignatureSources    []AttributeSource `mapstructure:"signature-sources" yaml:"signature-sources" json:"signatureSources"`            // X-Signature 提取来源
	NonceSources        []AttributeSource `mapstructure:"nonce-sources" yaml:"nonce-sources" json:"nonceSources"`                        // X-Nonce 提取来源
	AccessKeySources    []AttributeSource `mapstructure:"access-key-sources" yaml:"access-key-sources" json:"accessKeySources"`          // X-Access-Key 提取来源

	// 其他请求头
	OriginSources    []AttributeSource `mapstructure:"origin-sources" yaml:"origin-sources" json:"originSources"`            // Origin 提取来源
	CSRFTokenSources []AttributeSource `mapstructure:"csrf-token-sources" yaml:"csrf-token-sources" json:"csrfTokenSources"` // X-CSRF-Token 提取来源
}

// DefaultRequestContext 创建默认请求上下文提取配置
// DefaultRequestContext 的 header 来源 Key 一律写成 HTTP 规范形式（X-User-Id 而非 X-User-ID）
//
// 这不是风格偏好，是分配开销：Header.Get 内部会对 key 调 CanonicalMIMEHeaderKey，
// key 本来就是规范形式时直接返回、不分配；不是的话每次查找都要现算并分配一个新字符串
// 两种写法的查找结果完全一样（都归一到 X-User-Id），所以这是纯粹的削减
//
// 实测：提取器修好重复解析之后，canonicalMIMEHeaderKey 占了剩余分配的 100%，
// 把这些 key 写成规范形式后 header 路径降到零分配
//
// 新增 header 来源时请照规范形式写：连字符分段、每段仅首字母大写
//
// [EN] Header source keys are written in canonical HTTP form (X-User-Id, not X-User-ID). This is
// not a style preference but an allocation: Header.Get canonicalizes the key on every lookup and
// returns it untouched only when it is already canonical. Both spellings resolve to the same
// header, so this is a pure saving -- after the repeated-parsing fix, canonicalMIMEHeaderKey
// accounted for all remaining allocations on the header path
func DefaultRequestContext() *RequestContext {
	return &RequestContext{
		AuthorizationSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "Authorization"},
			{Type: SourceTypeQuery, Key: "authorization"},
			{Type: SourceTypeCookie, Key: "authorization"},
		},
		AuthPayloadSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Auth-Payload"},
		},
		JtiSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Jti"},
		},
		FamilyIdSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Family-Id"},
		},
		UserAgentSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "User-Agent"},
		},
		AcceptSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "Accept"},
		},
		CacheControlSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "Cache-Control"},
		},
		ConnectionSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "Connection"},
		},

		RequestIDSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Request-Id"},
			{Type: SourceTypeQuery, Key: "request_id"},
			{Type: SourceTypeQuery, Key: "requestId"},
			{Type: SourceTypeCookie, Key: "request_id"},
		},
		TraceIDSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Trace-Id"},
			{Type: SourceTypeQuery, Key: "trace_id"},
			{Type: SourceTypeQuery, Key: "traceId"},
			{Type: SourceTypeCookie, Key: "trace_id"},
		},
		RealIPSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Real-Ip"},
		},
		ForwardedForSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Forwarded-For"},
		},
		ForwardedProtoSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Forwarded-Proto"},
		},
		ForwardedHostSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Forwarded-Host"},
		},

		ClientIDSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Client-Id"},
			{Type: SourceTypeQuery, Key: "client_id"},
			{Type: SourceTypeQuery, Key: "clientId"},
			{Type: SourceTypeCookie, Key: "client_id"},
		},
		UserIDSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-User-Id"},
			{Type: SourceTypeQuery, Key: "user_id"},
			{Type: SourceTypeQuery, Key: "userId"},
			{Type: SourceTypeCookie, Key: "user_id"},
		},
		UserTypeSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-User-Type"},
			{Type: SourceTypeQuery, Key: "user_type"},
			{Type: SourceTypeQuery, Key: "userType"},
			{Type: SourceTypeCookie, Key: "user_type"},
		},
		TenantIDSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Tenant-Id"},
			{Type: SourceTypeQuery, Key: "tenant_id"},
			{Type: SourceTypeQuery, Key: "tenantId"},
			{Type: SourceTypeCookie, Key: "tenant_id"},
		},
		TenantCodeSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Tenant-Code"},
			{Type: SourceTypeQuery, Key: "tenant_code"},
			{Type: SourceTypeQuery, Key: "tenantCode"},
			{Type: SourceTypeCookie, Key: "tenant_code"},
		},
		SessionIDSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Session-Id"},
			{Type: SourceTypeQuery, Key: "session_id"},
			{Type: SourceTypeQuery, Key: "sessionId"},
			{Type: SourceTypeCookie, Key: "session_id"},
		},
		TimezoneSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Timezone"},
			{Type: SourceTypeQuery, Key: "timezone"},
			{Type: SourceTypeCookie, Key: "timezone"},
		},
		IDSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Id"},
			{Type: SourceTypeQuery, Key: "id"},
			{Type: SourceTypeCookie, Key: "id"},
		},
		DomainSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Domain"},
			{Type: SourceTypeQuery, Key: "domain"},
			{Type: SourceTypeCookie, Key: "domain"},
		},
		RoleCodeSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Role-Code"},
			{Type: SourceTypeQuery, Key: "role_code"},
			{Type: SourceTypeQuery, Key: "roleCode"},
			{Type: SourceTypeCookie, Key: "role_code"},
		},
		AgentLineIDSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Agent-Line-Id"},
			{Type: SourceTypeQuery, Key: "agent_line_id"},
			{Type: SourceTypeQuery, Key: "agentLineId"},
			{Type: SourceTypeCookie, Key: "agent_line_id"},
		},
		PushTokenSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Push-Token"},
			{Type: SourceTypeQuery, Key: "push_token"},
			{Type: SourceTypeQuery, Key: "pushToken"},
			{Type: SourceTypeCookie, Key: "push_token"},
		},
		TokenSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Token"},
			{Type: SourceTypeQuery, Key: "token"},
			{Type: SourceTypeQuery, Key: "token"},
			{Type: SourceTypeCookie, Key: "token"},
		},

		DeviceIDSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Device-Id"},
			{Type: SourceTypeHeader, Key: "X-Device-Id"},
			{Type: SourceTypeQuery, Key: "device_id"},
			{Type: SourceTypeQuery, Key: "deviceId"},
			{Type: SourceTypeCookie, Key: "device_id"},
		},
		AppIDSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-App-Id"},
			{Type: SourceTypeQuery, Key: "app_id"},
			{Type: SourceTypeQuery, Key: "appId"},
			{Type: SourceTypeCookie, Key: "app_id"},
		},
		AppVersionSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-App-Version"},
			{Type: SourceTypeQuery, Key: "app_version"},
			{Type: SourceTypeQuery, Key: "appVersion"},
			{Type: SourceTypeCookie, Key: "app_version"},
		},
		PlatformSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Platform"},
			{Type: SourceTypeQuery, Key: "platform"},
			{Type: SourceTypeCookie, Key: "platform"},
		},
		PlatformIDSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Platform-Id"},
			{Type: SourceTypeQuery, Key: "platform_id"},
			{Type: SourceTypeQuery, Key: "platformId"},
			{Type: SourceTypeCookie, Key: "platform_id"},
		},
		PlatformCodeSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Platform-Code"},
			{Type: SourceTypeQuery, Key: "platform_code"},
			{Type: SourceTypeQuery, Key: "platformCode"},
			{Type: SourceTypeCookie, Key: "platform_code"},
		},
		RegionIDSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Region-Id"},
			{Type: SourceTypeQuery, Key: "region_id"},
			{Type: SourceTypeQuery, Key: "regionId"},
			{Type: SourceTypeCookie, Key: "region_id"},
		},
		RegionCodeSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Region-Code"},
			{Type: SourceTypeQuery, Key: "region_code"},
			{Type: SourceTypeQuery, Key: "regionCode"},
			{Type: SourceTypeCookie, Key: "region_code"},
		},
		TimestampSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Timestamp"},
			{Type: SourceTypeQuery, Key: "timestamp"},
			{Type: SourceTypeCookie, Key: "timestamp"},
		},
		SignatureSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Signature"},
			{Type: SourceTypeQuery, Key: "signature"},
			{Type: SourceTypeCookie, Key: "signature"},
		},
		NonceSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Nonce"},
			{Type: SourceTypeQuery, Key: "nonce"},
			{Type: SourceTypeCookie, Key: "nonce"},
		},
		AccessKeySources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Access-Key"},
			{Type: SourceTypeQuery, Key: "access_key"},
			{Type: SourceTypeQuery, Key: "accessKey"},
			{Type: SourceTypeCookie, Key: "access_key"},
		},
		OriginSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "Origin"},
		},
		CSRFTokenSources: []AttributeSource{
			{Type: SourceTypeHeader, Key: "X-Csrf-Token"},
		},
	}
}

// FieldSourceKeys 存储单个字段的所有来源 key（header / query / cookie）
type FieldSourceKeys struct {
	Header string
	Query  string
	Cookie string
}

// SourceKeys 存储每个字段对应的各来源类型的首选 Key
// 用于外部包动态获取 header / query / cookie key，而非依赖硬编码常量
type SourceKeys struct {
	ID             FieldSourceKeys
	TraceID        FieldSourceKeys
	RequestID      FieldSourceKeys
	Authorization  FieldSourceKeys
	AuthPayload    FieldSourceKeys
	Jti            FieldSourceKeys
	FamilyId       FieldSourceKeys
	UserAgent      FieldSourceKeys
	ClientID       FieldSourceKeys
	UserID         FieldSourceKeys
	UserType       FieldSourceKeys
	Domain         FieldSourceKeys
	RoleCode       FieldSourceKeys
	TenantID       FieldSourceKeys
	TenantCode     FieldSourceKeys
	SessionID      FieldSourceKeys
	Timezone       FieldSourceKeys
	RealIP         FieldSourceKeys
	ForwardedFor   FieldSourceKeys
	ForwardedProto FieldSourceKeys
	ForwardedHost  FieldSourceKeys
	DeviceID       FieldSourceKeys
	AppID          FieldSourceKeys
	AppVersion     FieldSourceKeys
	PlatformID     FieldSourceKeys
	PlatformCode   FieldSourceKeys
	RegionID       FieldSourceKeys
	RegionCode     FieldSourceKeys
	AgentLineID    FieldSourceKeys
	Timestamp      FieldSourceKeys
	Signature      FieldSourceKeys
	Nonce          FieldSourceKeys
	AccessKey      FieldSourceKeys
	PushToken      FieldSourceKeys
	Token          FieldSourceKeys
	Origin         FieldSourceKeys
	CSRFToken      FieldSourceKeys
}

// firstSourceKey 返回 sources 中第一个指定类型来源的 Key
func firstSourceKey(sources []AttributeSource, sourceType AttributeSourceType) string {
	for _, s := range sources {
		if s.Type == sourceType {
			return s.Key
		}
	}
	return ""
}

// extractFieldSourceKeys 从 sources 中提取所有来源类型的首选 Key
func extractFieldSourceKeys(sources []AttributeSource) FieldSourceKeys {
	return FieldSourceKeys{
		Header: firstSourceKey(sources, SourceTypeHeader),
		Query:  firstSourceKey(sources, SourceTypeQuery),
		Cookie: firstSourceKey(sources, SourceTypeCookie),
	}
}

// GetSourceKeys 从当前 RequestContext 配置中提取所有字段的各来源首选 key
// 当配置变更时，调用此方法可获取最新的 key
func (c *RequestContext) GetSourceKeys() *SourceKeys {
	return &SourceKeys{
		ID:             extractFieldSourceKeys(c.IDSources),
		TraceID:        extractFieldSourceKeys(c.TraceIDSources),
		RequestID:      extractFieldSourceKeys(c.RequestIDSources),
		Authorization:  extractFieldSourceKeys(c.AuthorizationSources),
		AuthPayload:    extractFieldSourceKeys(c.AuthPayloadSources),
		Jti:            extractFieldSourceKeys(c.JtiSources),
		FamilyId:       extractFieldSourceKeys(c.FamilyIdSources),
		UserAgent:      extractFieldSourceKeys(c.UserAgentSources),
		ClientID:       extractFieldSourceKeys(c.ClientIDSources),
		UserID:         extractFieldSourceKeys(c.UserIDSources),
		UserType:       extractFieldSourceKeys(c.UserTypeSources),
		Domain:         extractFieldSourceKeys(c.DomainSources),
		RoleCode:       extractFieldSourceKeys(c.RoleCodeSources),
		TenantID:       extractFieldSourceKeys(c.TenantIDSources),
		TenantCode:     extractFieldSourceKeys(c.TenantCodeSources),
		SessionID:      extractFieldSourceKeys(c.SessionIDSources),
		Timezone:       extractFieldSourceKeys(c.TimezoneSources),
		RealIP:         extractFieldSourceKeys(c.RealIPSources),
		ForwardedFor:   extractFieldSourceKeys(c.ForwardedForSources),
		ForwardedProto: extractFieldSourceKeys(c.ForwardedProtoSources),
		ForwardedHost:  extractFieldSourceKeys(c.ForwardedHostSources),
		DeviceID:       extractFieldSourceKeys(c.DeviceIDSources),
		AppID:          extractFieldSourceKeys(c.AppIDSources),
		AppVersion:     extractFieldSourceKeys(c.AppVersionSources),
		PlatformID:     extractFieldSourceKeys(c.PlatformIDSources),
		PlatformCode:   extractFieldSourceKeys(c.PlatformCodeSources),
		RegionID:       extractFieldSourceKeys(c.RegionIDSources),
		RegionCode:     extractFieldSourceKeys(c.RegionCodeSources),
		AgentLineID:    extractFieldSourceKeys(c.AgentLineIDSources),
		Timestamp:      extractFieldSourceKeys(c.TimestampSources),
		Signature:      extractFieldSourceKeys(c.SignatureSources),
		Nonce:          extractFieldSourceKeys(c.NonceSources),
		AccessKey:      extractFieldSourceKeys(c.AccessKeySources),
		PushToken:      extractFieldSourceKeys(c.PushTokenSources),
		Token:          extractFieldSourceKeys(c.TokenSources),
		Origin:         extractFieldSourceKeys(c.OriginSources),
		CSRFToken:      extractFieldSourceKeys(c.CSRFTokenSources),
	}
}

// Get 返回配置接口
func (c *RequestContext) Get() any {
	return c
}

// Set 设置配置
func (c *RequestContext) Set(data any) {
	if cfg, ok := data.(*RequestContext); ok {
		*c = *cfg
	}
}

// Clone 返回副本
func (c *RequestContext) Clone() internal.Configurable {
	var cloned RequestContext
	if err := syncx.DeepCopy(&cloned, c); err != nil {
		return &RequestContext{}
	}
	return &cloned
}

// Validate 验证配置
func (c *RequestContext) Validate() error {
	if err := internal.ValidateStruct(c); err != nil {
		return err
	}

	sourceGroups := [][]AttributeSource{
		c.AuthorizationSources,
		c.AuthPayloadSources,
		c.JtiSources,
		c.FamilyIdSources,
		c.UserAgentSources,
		c.AcceptSources,
		c.CacheControlSources,
		c.ConnectionSources,
		c.RequestIDSources,
		c.TraceIDSources,
		c.RealIPSources,
		c.ForwardedForSources,
		c.ForwardedProtoSources,
		c.ForwardedHostSources,
		c.ClientIDSources,
		c.UserIDSources,
		c.UserTypeSources,
		c.TenantIDSources,
		c.TenantCodeSources,
		c.SessionIDSources,
		c.TimezoneSources,
		c.IDSources,
		c.DomainSources,
		c.RoleCodeSources,
		c.DeviceIDSources,
		c.AppIDSources,
		c.AppVersionSources,
		c.PlatformIDSources,
		c.PlatformCodeSources,
		c.RegionIDSources,
		c.RegionCodeSources,
		c.AgentLineIDSources,
		c.TimestampSources,
		c.SignatureSources,
		c.NonceSources,
		c.AccessKeySources,
		c.PushTokenSources,
		c.TokenSources,
		c.OriginSources,
		c.CSRFTokenSources,
	}

	// 这里按下标遍历，而 group 与 c.XxxSources 共享底层数组，所以 NormalizeKey 的改动
	// 会落到真正的配置上。Validate 是 Gateway.Validate 必经之路，等于加载后钩子
	// [EN] Iterating by index over slices that share their backing array with the config fields,
	// so NormalizeKey's change lands on the real configuration. Validate is on Gateway.Validate's
	// path and therefore acts as the after-load hook
	for _, group := range sourceGroups {
		for i := range group {
			if err := group[i].Validate(); err != nil {
				return err
			}
			group[i].NormalizeKey()
		}
	}

	return nil
}
