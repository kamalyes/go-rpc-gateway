/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 18:05:15
 * @FilePath: \go-rpc-gateway\internal\constants\http.go
 * @Description: HTTP相关常量定义
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package constants

// HTTP 头部常量
const (
	// 标准请求头
	HeaderContentType    = "Content-Type"
	HeaderAuthorization  = "Authorization"
	HeaderUserAgent      = "User-Agent"
	HeaderAccept         = "Accept"
	HeaderAcceptEncoding = "Accept-Encoding"
	HeaderAcceptLanguage = "Accept-Language"
	HeaderCacheControl   = "Cache-Control"
	HeaderConnection     = "Connection"

	// 自定义请求头
	HeaderXRequestID      = "X-Request-Id"
	HeaderXTraceID        = "X-Trace-Id"
	HeaderXRealIP         = "X-Real-IP"
	HeaderXForwardedFor   = "X-Forwarded-For"
	HeaderXForwardedProto = "X-Forwarded-Proto"
	
	// 设备和应用相关头部
	HeaderXDeviceID       = "X-Device-Id"
	HeaderXAppVersion     = "X-App-Version"
	HeaderXPlatform       = "X-Platform"
	HeaderXTimestamp      = "X-Timestamp"
	HeaderXSignature      = "X-Signature"

	// 安全相关头部
	HeaderXFrameOptions           = "X-Frame-Options"
	HeaderXContentTypeOptions     = "X-Content-Type-Options"
	HeaderXXSSProtection          = "X-XSS-Protection"
	HeaderStrictTransportSecurity = "Strict-Transport-Security"
	HeaderContentSecurityPolicy   = "Content-Security-Policy"
	HeaderReferrerPolicy          = "Referrer-Policy"
	HeaderPermissionsPolicy       = "Permissions-Policy"
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
