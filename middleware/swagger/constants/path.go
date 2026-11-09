/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\constants\path.go
 * @Description: 路径常量 - Swagger 路由路径、分隔符、前缀
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package constants

// ==================== 路径常量 ====================

const (
	JSONPath      = "/swagger.json"
	ServicesPath  = "/services"
	DocumentsPath = "/documents"
	AggregatePath = "/aggregate.json"
	DebugPath     = "/debug/services"
	IndexHTML     = "/index.html"
	JSONExt       = ".json"

	// LogoutPath 退出登录路径：清理凭据 Cookie 并重新发起 401 挑战
	// [EN] LogoutPath clears the credential cookie and re-issues the 401 challenge
	LogoutPath = "/logout"
)

// ==================== 路径分隔与匹配 ====================

const (
	PathSeparator      = "/"
	PathServicePrefix  = "/services/"
	PathDocumentPrefix = "/documents/"
)
