/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:40:10
 * @FilePath: \go-rpc-gateway\constants\middleware_common.go
 * @Description: 中间件通用常量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

// ============================================================================
// 中间件默认跳过规则
// ============================================================================

// 默认跳过的用户代理
var MiddlewareDefaultSkipUserAgents = []string{
	"healthcheck",
	"probe",
	"monitor",
	"nagios",
	"zabbix",
}

// 默认跳过的方法
var MiddlewareDefaultSkipMethods = []string{
	"OPTIONS",
}
