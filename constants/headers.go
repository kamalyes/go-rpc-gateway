/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 09:02:19
 * @FilePath: \go-rpc-gateway\constants\headers.go
 * @Description: HTTP相关常量定义
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package constants

// HTTP 头部常量
const (
	// 标准请求头
	HeaderContentType     = "Content-Type"
	HeaderContentLanguage = "Content-Language"
	HeaderContentLength   = "Content-Length"
	HeaderAuthorization   = "Authorization"
	HeaderUserAgent       = "User-Agent"
	HeaderAccept          = "Accept"
	HeaderAcceptEncoding  = "Accept-Encoding"
	HeaderAcceptLanguage  = "Accept-Language"
	HeaderCacheControl    = "Cache-Control"
	HeaderConnection      = "Connection"

	// 自定义请求头
	HeaderXRequestID      = "X-Request-Id"
	HeaderXTraceID        = "X-Trace-Id"
	HeaderXRealIP         = "X-Real-IP"
	HeaderXForwardedFor   = "X-Forwarded-For"
	HeaderXForwardedProto = "X-Forwarded-Proto"
	HeaderWWWAuthenticate = "WWW-Authenticate"

	// 用户上下文相关头部
	HeaderXUserID    = "X-User-ID"
	HeaderXTenantID  = "X-Tenant-ID"
	HeaderXSessionID = "X-Session-ID"

	// 设备和应用相关头部
	HeaderXDeviceID       = "X-Device-Id"
	HeaderXAppVersion     = "X-App-Version"
	HeaderXPlatform       = "X-Platform"
	HeaderXTimestamp      = "X-Timestamp"
	HeaderXSignature      = "X-Signature"
	HeaderXResponseFormat = "X-Response-Format"

	// 安全相关头部
	HeaderXFrameOptions           = "X-Frame-Options"
	HeaderXContentTypeOptions     = "X-Content-Type-Options"
	HeaderXXSSProtection          = "X-XSS-Protection"
	HeaderStrictTransportSecurity = "Strict-Transport-Security"
	HeaderContentSecurityPolicy   = "Content-Security-Policy"
	HeaderReferrerPolicy          = "Referrer-Policy"
	HeaderPermissionsPolicy       = "Permissions-Policy"

	// CORS 相关头部
	HeaderAccessControlAllowOrigin      = "Access-Control-Allow-Origin"
	HeaderAccessControlAllowMethods     = "Access-Control-Allow-Methods"
	HeaderAccessControlAllowHeaders     = "Access-Control-Allow-Headers"
	HeaderAccessControlAllowCredentials = "Access-Control-Allow-Credentials"
	HeaderAccessControlMaxAge           = "Access-Control-Max-Age"
	HeaderOrigin                        = "Origin"

	// CSRF 相关头部
	HeaderXCSRFToken = "X-CSRF-Token"
)

// MIME 类型常量
const (
	MimeApplicationJSON        = "application/json"
	MimeApplicationXML         = "application/xml"
	MimeApplicationForm        = "application/x-www-form-urlencoded"
	MimeApplicationOctetStream = "application/octet-stream"
	MimeMultipartFormData      = "multipart/form-data"
	MimeTextPlain              = "text/plain"
	MimeTextHTML               = "text/html"
	MimeTextXML                = "text/xml"
	MimeTextCSV                = "text/csv"
	MimeImageJPEG              = "image/jpeg"
	MimeImagePNG               = "image/png"
	MimeImageGIF               = "image/gif"
	MimeImageWebP              = "image/webp"
)
