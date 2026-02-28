/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 10:11:54
 * @FilePath: \go-rpc-gateway\response\error.go
 * @Description: HTTP错误响应函数
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package response

import (
	"fmt"
	"net/http"

	"github.com/kamalyes/go-rpc-gateway/errors"
	commonapis "github.com/kamalyes/go-rpc-gateway/proto"
)

// WriteErrorResult 写入错误响应
func WriteErrorResult(w http.ResponseWriter, httpStatus int, errorMsg string, statusCode commonapis.StatusCode) {
	result := &commonapis.Result{
		Code:   int32(httpStatus),
		Error:  errorMsg,
		Status: statusCode,
	}
	WriteResult(w, httpStatus, result)
}

// WriteInternalServerErrorResult 写入500内部服务器错误
func WriteInternalServerErrorResult(w http.ResponseWriter, errorMsg string) {
	WriteErrorResult(w, http.StatusInternalServerError, errorMsg, commonapis.StatusCode_Internal)
}

// WriteServiceUnavailableResult 写入503服务不可用错误
func WriteServiceUnavailableResult(w http.ResponseWriter, errorMsg string) {
	WriteErrorResult(w, http.StatusServiceUnavailable, errorMsg, commonapis.StatusCode_Unavailable)
}

// WriteBadRequestResult 写入400请求错误
func WriteBadRequestResult(w http.ResponseWriter, errorMsg string) {
	WriteErrorResult(w, http.StatusBadRequest, errorMsg, commonapis.StatusCode_InvalidArgument)
}

// WriteNotFoundResult 写入404未找到错误
func WriteNotFoundResult(w http.ResponseWriter, errorMsg string) {
	WriteErrorResult(w, http.StatusNotFound, errorMsg, commonapis.StatusCode_NotFound)
}

// WriteUnauthorizedResult 写入401未授权错误
func WriteUnauthorizedResult(w http.ResponseWriter, errorMsg string) {
	WriteErrorResult(w, http.StatusUnauthorized, errorMsg, commonapis.StatusCode_Unauthenticated)
}

// WriteForbiddenResult 写入403禁止访问错误
func WriteForbiddenResult(w http.ResponseWriter, errorMsg string) {
	WriteErrorResult(w, http.StatusForbidden, errorMsg, commonapis.StatusCode_PermissionDenied)
}

// WriteTooManyRequestsResult 写入429请求过多错误
func WriteTooManyRequestsResult(w http.ResponseWriter, errorMsg string) {
	WriteErrorResult(w, http.StatusTooManyRequests, errorMsg, commonapis.StatusCode_ResourceExhausted)
}

// WriteAppError 写入AppError响应
func WriteAppError(w http.ResponseWriter, appErr *errors.AppError) {
	result := appErr.ToResult()
	WriteResult(w, appErr.GetHTTPStatus(), result)
}

// WriteAppErrorf 写入格式化的AppError响应
func WriteAppErrorf(w http.ResponseWriter, code errors.ErrorCode, format string, args ...any) {
	appErr := errors.NewErrorf(code, format, args...)
	WriteAppError(w, appErr)
}

// WriteErrorResponseWithCode 写入带错误码的错误响应
// 这个方法提供了更细粒度的错误响应控制，支持自定义错误码和消息
func WriteErrorResponseWithCode(w http.ResponseWriter, statusCode int, errorCode, message string) {
	// 根据HTTP状态码选择合适的StatusCode
	var status commonapis.StatusCode
	switch statusCode {
	case http.StatusBadRequest:
		status = commonapis.StatusCode_InvalidArgument
	case http.StatusUnauthorized:
		status = commonapis.StatusCode_Unauthenticated
	case http.StatusForbidden:
		status = commonapis.StatusCode_PermissionDenied
	case http.StatusNotFound:
		status = commonapis.StatusCode_NotFound
	default:
		status = commonapis.StatusCode_Internal
	}

	// 使用标准的错误响应方法
	WriteErrorResult(w, statusCode, fmt.Sprintf("%s: %s", errorCode, message), status)
}
