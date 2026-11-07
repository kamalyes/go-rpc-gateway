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

// ==================== MIME 类型 ====================

const (
	MimeApplicationJSON        = "application/json"
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
