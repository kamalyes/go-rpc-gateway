/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 10:11:54
 * @FilePath: \go-rpc-gateway\response\success.go
 * @Description: HTTP成功响应函数
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package response

import (
	"net/http"

	commonapis "github.com/kamalyes/go-rpc-gateway/proto"
)

// WriteSuccessResult 写入成功响应
func WriteSuccessResult(w http.ResponseWriter, message string) {
	result := &commonapis.Result{
		Code:   http.StatusOK,
		Error:  message,
		Status: commonapis.StatusCode_OK,
	}
	WriteResult(w, http.StatusOK, result)
}

// WriteVersionResponse 写入版本信息响应
func WriteVersionResponse(w http.ResponseWriter, version, gitBranch, gitHash, buildTime string) {
	versionInfo := &VersionInfo{
		Version:   version,
		GitBranch: gitBranch,
		GitHash:   gitHash,
		BuildTime: buildTime,
	}
	WriteJSONResponse(w, http.StatusOK, versionInfo)
}

// WriteCSRFTokenResponse 写入CSRF token响应
func WriteCSRFTokenResponse(w http.ResponseWriter, token string) {
	tokenResponse := &CSRFTokenResponse{
		CSRFToken: token,
	}
	WriteJSONResponse(w, http.StatusOK, tokenResponse)
}
