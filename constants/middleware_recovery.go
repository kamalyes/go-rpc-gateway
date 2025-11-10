/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:40:10
 * @FilePath: \go-rpc-gateway\constants\middleware_recovery.go
 * @Description: 恢复中间件相关常量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

import "net/http"

// 恢复中间件默认配置
const (
	// 默认是否启用堆栈跟踪
	RecoveryDefaultPrintStack = true

	// 默认是否启用
	RecoveryDefaultEnabled = true

	// 默认错误状态码
	RecoveryDefaultStatusCode = http.StatusInternalServerError

	// 默认错误消息
	RecoveryDefaultErrorMessage = "Internal Server Error"

	// 默认内容类型
	RecoveryDefaultContentType = "application/json"
)

// 恢复模式常量
const (
	RecoveryModeProduction  = "production"  // 生产模式：隐藏错误详情
	RecoveryModeDevelopment = "development" // 开发模式：显示详细错误信息
	RecoveryModeDebug       = "debug"       // 调试模式：显示堆栈信息
)

// 错误级别常量
const (
	RecoveryLevelPanic = "panic"
	RecoveryLevelError = "error"
	RecoveryLevelWarn  = "warn"
	RecoveryLevelInfo  = "info"
)

// 默认错误响应格式
const (
	RecoveryErrorResponseJSON = `{"error": "%s", "message": "An unexpected error occurred"}`
	RecoveryErrorResponseText = "Internal Server Error: %s"
	RecoveryErrorResponseHTML = `<html><body><h1>Internal Server Error</h1><p>%s</p></body></html>`
)

// 敏感信息过滤模式
const (
	RecoveryFilterModeStrict   = "strict"   // 严格模式：过滤所有敏感信息
	RecoveryFilterModeModerate = "moderate" // 适中模式：过滤关键敏感信息
	RecoveryFilterModeLoose    = "loose"    // 宽松模式：最小过滤
)

// 需要过滤的敏感关键词
var RecoverySensitiveKeywords = []string{
	"password", "secret", "token", "key", "auth",
	"credential", "private", "confidential",
}

// 堆栈跟踪过滤
var RecoveryStackTraceSkipPackages = []string{
	"runtime/",
	"net/http/",
	"github.com/gin-gonic/gin",
}
