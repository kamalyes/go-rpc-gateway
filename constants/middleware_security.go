/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:40:10
 * @FilePath: \go-rpc-gateway\constants\middleware_security.go
 * @Description: 安全中间件相关常量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

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
