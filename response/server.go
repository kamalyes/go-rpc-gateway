/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 15:00:00
 * @FilePath: \go-rpc-gateway\response\response.go
 * @Description: HTTP响应工具函数，避免循环导入
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package response

import (
	"encoding/json"
	"net/http"

	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	commonapis "github.com/kamalyes/go-rpc-gateway/proto"
)

// WriteErrorResponse 写入标准化的错误响应
func WriteErrorResponse(w http.ResponseWriter, appErr *errors.AppError) {
	result := appErr.ToResult()
	WriteResultResponse(w, appErr.GetHTTPStatus(), result)
}

// WriteResultResponse 写入Result响应
func WriteResultResponse(w http.ResponseWriter, httpStatus int, result *commonapis.Result) {
	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
	w.WriteHeader(httpStatus)

	if err := json.NewEncoder(w).Encode(result); err != nil && global.LOGGER != nil {
		global.LOGGER.WithError(err).ErrorMsg("Failed to encode Result response")
	}
}

// WriteSimpleError 写入简单的错误响应（用于避免循环导入的场景）
func WriteSimpleError(w http.ResponseWriter, httpStatus int, statusCode commonapis.StatusCode, message string) {
	result := &commonapis.Result{
		Code:   int32(httpStatus),
		Error:  message,
		Status: statusCode,
	}
	WriteResultResponse(w, httpStatus, result)
}
