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

// ============================================================================
// Recovery 中间件配置常量
// ============================================================================

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

// 堆栈跟踪过滤
var RecoveryStackTraceSkipPackages = []string{
	"runtime/",
	"net/http/",
}
