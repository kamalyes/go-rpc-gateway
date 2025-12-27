/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 17:56:53
 * @FilePath: \go-rpc-gateway\constants\middleware_security.go
 * @Description: 安全中间件相关常量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

// ============================================================================
// 安全中间件默认配置
// ============================================================================

// 安全头部默认值常量
const (
	// X-Frame-Options 默认值
	SecurityDefaultFrameOptions = "DENY"

	// X-Content-Type-Options 默认值
	SecurityDefaultContentTypeOptions = "nosniff"

	// X-XSS-Protection 默认值
	SecurityDefaultXSSProtection = "1; mode=block"

	// Referrer-Policy 默认值
	SecurityDefaultReferrerPolicy = "strict-origin-when-cross-origin"

	// HSTS 默认最大年龄（秒）
	SecurityDefaultHSTSMaxAge = 31536000 // 1年

	// 默认内容安全策略
	SecurityDefaultCSP = "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'"
)

// XSS 防护模式常量
const (
	XSSProtectionModeBlock  = "1; mode=block"
	XSSProtectionModeReport = "1; report=uri"
	XSSProtectionDisabled   = "0"
)

// Frame Options 常量
const (
	FrameOptionsDeny       = "DENY"
	FrameOptionsSameOrigin = "SAMEORIGIN"
	FrameOptionsAllowFrom  = "ALLOW-FROM"
)

// Referrer Policy 常量
const (
	ReferrerPolicyNoReferrer                  = "no-referrer"
	ReferrerPolicyNoReferrerWhenDowngrade     = "no-referrer-when-downgrade"
	ReferrerPolicyOrigin                      = "origin"
	ReferrerPolicyOriginWhenCrossOrigin       = "origin-when-cross-origin"
	ReferrerPolicyStrictOrigin                = "strict-origin"
	ReferrerPolicyStrictOriginWhenCrossOrigin = "strict-origin-when-cross-origin"
	ReferrerPolicySameOrigin                  = "same-origin"
	ReferrerPolicyUnsafeURL                   = "unsafe-url"
)

// CORS 默认配置
const (
	CORSDefaultAllowOrigins     = "*"
	CORSDefaultAllowMethods     = "GET,POST,PUT,DELETE,OPTIONS,HEAD"
	CORSDefaultAllowHeaders     = "Origin,Content-Type,Accept,Authorization,X-Request-ID"
	CORSDefaultExposeHeaders    = "X-Request-ID"
	CORSDefaultMaxAge           = 86400 // 24小时
	CORSDefaultAllowCredentials = true
)

// 安全检测模式
const (
	SecurityModeStrict   = "strict"
	SecurityModeModerate = "moderate"
	SecurityModeBasic    = "basic"
)

// 常见的危险文件扩展名
var SecurityDangerousExtensions = []string{
	".exe", ".bat", ".cmd", ".scr", ".pif", ".com",
	".js", ".vbs", ".ps1", ".sh", ".php", ".asp",
}

// XSS 攻击模式
var SecurityXSSPatterns = []string{
	"<script",
	"javascript:",
	"onload=",
	"onerror=",
	"onclick=",
	"onmouseover=",
}

// 路径保护相关常量
const (
	// 认证类型
	AuthTypeBasic  = "basic"
	AuthTypeBearer = "bearer"
	AuthTypeAPIKey = "apikey"
	AuthTypeCustom = "custom"

	// 认证方案
	AuthSchemeBasic  = "Basic"
	AuthSchemeBearer = "Bearer"

	// 认证Realm
	AuthRealmProtected = "Protected Area"
	AuthRealmPProf     = "PProf"
	AuthRealmSwagger   = "Swagger"
	AuthRealmMetrics   = "Metrics"
)

// HTTP 方法常量
const (
	HTTPMethodOptions = "OPTIONS"
	HTTPMethodGet     = "GET"
	HTTPMethodHead    = "HEAD"
	HTTPMethodPost    = "POST"
	HTTPMethodPut     = "PUT"
	HTTPMethodPatch   = "PATCH"
	HTTPMethodTrace   = "TRACE"
	HTTPMethodDelete  = "DELETE"
)

// HTTPMethods 有效的 HTTP 方法集合
var HTTPMethods = map[string]bool{
	HTTPMethodOptions: true,
	HTTPMethodGet:     true,
	HTTPMethodHead:    true,
	HTTPMethodPost:    true,
	HTTPMethodPut:     true,
	HTTPMethodPatch:   true,
	HTTPMethodTrace:   true,
	HTTPMethodDelete:  true,
}

// CSRFExemptMethods CSRF 豁免的 HTTP 方法（GET、HEAD、OPTIONS、TRACE）
var CSRFExemptMethods = map[string]bool{
	HTTPMethodGet:     true,
	HTTPMethodHead:    true,
	HTTPMethodOptions: true,
	HTTPMethodTrace:   true,
}

// CSRF Token 相关常量
const (
	CSRFTokenFormField  = "_csrf_token"
	CSRFTokenCookieName = "csrf_token"
	CSRFTokenLength     = 32
	CSRFSecret          = "csrf-secret"
)

// 安全头部值常量
const (
	SecurityHeaderNosniff         = "nosniff"
	SecurityHeaderDeny            = "DENY"
	SecurityHeaderXSSBlock        = "1; mode=block"
	SecurityHeaderHSTS            = "max-age=31536000; includeSubDomains"
	SecurityHeaderReferrerDefault = "strict-origin-when-cross-origin"
)

// CORS 相关常量
const (
	CORSCredentialsTrue = "true"
)

// 安全中间件错误消息常量
const (
	ErrMsgIPNotAllowed   = "Forbidden: IP not allowed"
	ErrMsgUnauthorized   = "Unauthorized"
	ErrMsgHTTPSRequired  = "HTTPS Required"
	ErrMsgAccessDenied   = "Access Denied"
	ErrMsgInvalidToken   = "Invalid Token"
	ErrMsgInvalidAuth    = "Invalid Authentication"
	ErrMsgIPAccessDenied = "IP access denied"
)

// 安全中间件日志消息常量
const (
	LogMsgIPNotInWhitelist     = "路径保护: IP不在白名单"
	LogMsgAuthFailed           = "路径保护: 认证失败"
	LogMsgHTTPSRequired        = "路径保护: 要求HTTPS"
	LogMsgAccessGranted        = "路径保护: 访问通过"
	LogMsgCSRFValidationFailed = "CSRF token验证失败"
	LogMsgIPAccessDenied       = "IP访问被拒绝"
)
