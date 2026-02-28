/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-11 10:11:15
 * @FilePath: \go-rpc-gateway\response\health.go
 * @Description: HTTP健康检查响应函数
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package response

import (
	"net/http"

	commonapis "github.com/kamalyes/go-rpc-gateway/proto"
)

// WriteHealthCheckResult 写入健康检查结果
func WriteHealthCheckResult(w http.ResponseWriter, isHealthy bool, component string, message string, details map[string]any) {
	if isHealthy {
		result := &commonapis.Result{
			Code:   http.StatusOK,
			Error:  message,
			Status: commonapis.StatusCode_OK,
		}
		WriteResult(w, http.StatusOK, result)
		return
	}

	result := &commonapis.Result{
		Code:   http.StatusServiceUnavailable,
		Error:  message,
		Status: commonapis.StatusCode_Unavailable,
	}
	WriteResult(w, http.StatusServiceUnavailable, result)
}
