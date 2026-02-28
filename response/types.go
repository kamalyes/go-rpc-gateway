/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 10:11:54
 * @FilePath: \go-rpc-gateway\response\types.go
 * @Description: HTTP响应类型定义
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package response

// VersionInfo 版本信息结构
type VersionInfo struct {
	Version   string `json:"version"`
	GitBranch string `json:"git_branch"`
	GitHash   string `json:"git_hash"`
	BuildTime string `json:"build_time"`
}

// CSRFTokenResponse CSRF token响应结构
type CSRFTokenResponse struct {
	CSRFToken string `json:"csrf_token"`
}
