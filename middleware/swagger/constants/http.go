/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\constants\http.go
 * @Description: HTTP 常量 - 请求头、CORS、MIME 类型、HTTP 方法
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package constants

import (
	"net/http"
	"strings"
)

// ==================== HTTP 头与 CORS ====================

const (
	HeaderContentType               = "Content-Type"
	HeaderAccessControlAllowOrigin  = "Access-Control-Allow-Origin"
	HeaderAccessControlAllowMethods = "Access-Control-Allow-Methods"
	HeaderAccessControlAllowHeaders = "Access-Control-Allow-Headers"
	CORSAllowAll                    = "*"
	CORSDefaultMethods              = "GET, POST, PUT, DELETE, OPTIONS"
	CORSDefaultHeaders              = "Content-Type, Authorization"
	HTTPMethodOptions               = "OPTIONS"
)

// ==================== 认证相关 ====================

const (
	HeaderAuthorization   = "Authorization"
	HeaderWWWAuthenticate = "WWW-Authenticate"
	HeaderCacheControl    = "Cache-Control"
	HeaderAccept          = "Accept"
	HeaderXForwardedProto = "X-Forwarded-Proto"

	// SchemeHTTPS X-Forwarded-Proto 判定 HTTPS 的取值
	// [EN] SchemeHTTPS is the X-Forwarded-Proto value that indicates HTTPS
	SchemeHTTPS = "https"

	// AuthSchemeBasic/AuthSchemeBearer Authorization 头的认证方案前缀
	// [EN] AuthSchemeBasic and AuthSchemeBearer are the Authorization header scheme prefixes
	AuthSchemeBasic  = "Basic"
	AuthSchemeBearer = "Bearer"

	// AuthRealm 401 挑战中的 realm，浏览器原生登录弹窗会展示该文本
	// [EN] AuthRealm is the realm sent in the 401 challenge and shown in the browser's native
	// login dialog
	AuthRealm = "Swagger Documentation"

	// AuthCookieName Bearer/Custom 凭据的会话 Cookie 名
	// 浏览器地址栏直接访问页面时无法携带自定义请求头，用 Cookie 承接凭据，
	// 使页面内 Swagger UI 发起的 doc.json 等 XHR 也能通过鉴权
	//
	// [EN] AuthCookieName is the session cookie holding a Bearer or Custom credential. A page
	// opened straight from the address bar cannot carry a custom header, so the cookie holds the
	// credential and keeps the XHRs that Swagger UI issues for doc.json authenticated too
	AuthCookieName = "swagger_auth"

	// AuthQueryToken 允许通过查询参数首次传入凭据，命中后写入 Cookie
	// [EN] AuthQueryToken lets a credential arrive once via query parameter, after which it is
	// written to the cookie
	AuthQueryToken = "token"

	// NoStoreCacheControl 鉴权失败响应禁止缓存，避免 401 被中间层缓存
	// [EN] NoStoreCacheControl keeps failed-auth responses out of caches so a 401 is never cached
	// by an intermediary
	NoStoreCacheControl = "no-store"
)

// ==================== MIME 类型 ====================

const (
	MimeApplicationJSON        = "application/json"
	MimeTextHTML               = "text/html"
	MimeApplicationJSONCharset = "application/json; charset=utf-8"
	MimeTextHTMLCharset        = "text/html; charset=utf-8"
	MimeYAML                   = "yaml"
	MimeYML                    = "yml"
)

// ==================== 文件扩展名 ====================

const (
	FileExtYAML = ".yaml"
	FileExtYML  = ".yml"
	FileExtJSON = ".json"
)

// ==================== HTTP 方法标准化 ====================

// ValidHTTPMethods Swagger 规范中合法的 HTTP 操作方法（大写）
var ValidHTTPMethods = []string{
	http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete,
	http.MethodOptions, http.MethodHead, http.MethodPatch,
	http.MethodTrace, http.MethodConnect,
}

// validHTTPMethodSet 内部快速查找集合
var validHTTPMethodSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(ValidHTTPMethods))
	for _, v := range ValidHTTPMethods {
		m[v] = struct{}{}
	}
	return m
}()

// NormalizeHTTPMethod 将 HTTP 方法名标准化为大写，不合法时返回空字符串
func NormalizeHTTPMethod(method string) string {
	upper := strings.ToUpper(method)
	if _, ok := validHTTPMethodSet[upper]; ok {
		return upper
	}
	return ""
}

// IsValidHTTPMethod 判断是否是合法的 Swagger 操作方法
func IsValidHTTPMethod(method string) bool {
	_, ok := validHTTPMethodSet[strings.ToUpper(method)]
	return ok
}
