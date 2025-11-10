/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 07:53:14
 * @FilePath: \go-rpc-gateway\constants\gateway.go
 * @Description: HTTP相关常量定义
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package constants

// 常量定义
const (
	DefaultServiceName = "go-rpc-gateway"
	DefaultHealthPath  = "/health"
	DefaultMetricsPath = "/metrics"
	DefaultDebugPath   = "/debug"
)

// 上下文键常量
const (
	ContextKeyI18n = "i18n"
)

// JSON 响应模板
const (
	JSONSuccessTemplate = `{"success": true, "message": "%s", "data": %s}`
	JSONErrorTemplate   = `{"success": false, "error": "%s", "message": "%s"}`
	JSONSimpleError     = `{"error": "%s"}`
	JSONSimpleSuccess   = `{"success": true}`
	JSONRateLimitError  = `{"error": "Too Many Requests", "message": "Rate limit exceeded"}`
	JSONInternalError   = `{"error": "Internal server error"}`
	JSONUnauthorized    = `{"error": "Unauthorized", "message": "Authentication required"}`
	JSONForbidden       = `{"error": "Forbidden", "message": "Permission denied"}`
	JSONNotFound        = `{"error": "Not Found", "message": "Resource not found"}`
	JSONBadRequest      = `{"error": "Bad Request", "message": "Invalid request"}`
)
