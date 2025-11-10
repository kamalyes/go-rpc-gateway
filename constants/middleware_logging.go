/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:40:10
 * @FilePath: \go-rpc-gateway\constants\middleware_logging.go
 * @Description: 日志中间件相关常量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

// 日志格式常量
const (
	LoggingFormatText = "text"
	LoggingFormatJSON = "json"
)

// 日志默认配置常量
const (
	// 默认日志格式
	LoggingDefaultFormat = LoggingFormatText

	// 默认是否包含请求体
	LoggingDefaultIncludeBody = false

	// 默认是否包含查询参数
	LoggingDefaultIncludeQuery = true

	// 默认是否启用
	LoggingDefaultEnabled = true
)

// 默认包含的头部信息
var LoggingDefaultIncludeHeaders = []string{
	HeaderUserAgent,
	HeaderXRequestID,
	HeaderXTraceID,
}

// 默认跳过的路径
var LoggingDefaultSkipPaths = []string{
	DefaultHealthPath,
	DefaultMetricsPath,
	DefaultDebugPath,
}

// 日志输出类型常量
const (
	LoggingOutputFile   = "file"
	LoggingOutputStdout = "stdout"
	LoggingOutputSyslog = "syslog"
)

// 日志轮转默认配置
const (
	LoggingDefaultMaxSize    = 100 // MB
	LoggingDefaultMaxBackups = 30  // 个数
	LoggingDefaultMaxAge     = 7   // 天
)
