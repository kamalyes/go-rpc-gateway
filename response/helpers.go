/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-17 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 22:00:00
 * @FilePath: \go-rpc-gateway\response\helpers.go
 * @Description: 响应辅助工具函数
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package response

import (
	"github.com/kamalyes/go-rpc-gateway/errors"
	"net/http"
)

// 快速响应函数集合 - 使用最常见的标准格式

// OK 返回成功响应
func OK(w http.ResponseWriter, data interface{}, message ...string) {
	WriteSuccess(w, data, message...)
}

// Created 返回创建成功响应
func Created(w http.ResponseWriter, data interface{}, message ...string) {
	msg := "created"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	WriteStandardResponse(w, FormatStandard, http.StatusCreated, data, msg)
}

// NoContent 返回无内容响应
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// BadRequest 返回400错误
func BadRequest(w http.ResponseWriter, message ...string) {
	msg := "Bad request"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	WriteErrorByCode(w, errors.ErrCodeBadRequest, msg)
}

// Unauthorized 返回401错误
func Unauthorized(w http.ResponseWriter, message ...string) {
	msg := "Unauthorized"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	WriteErrorByCode(w, errors.ErrCodeUnauthorized, msg)
}

// Forbidden 返回403错误
func Forbidden(w http.ResponseWriter, message ...string) {
	msg := "Forbidden"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	WriteErrorByCode(w, errors.ErrCodeForbidden, msg)
}

// NotFound 返回404错误
func NotFound(w http.ResponseWriter, message ...string) {
	msg := "Not found"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	WriteErrorByCode(w, errors.ErrCodeNotFound, msg)
}

// MethodNotAllowed 返回405错误
func MethodNotAllowed(w http.ResponseWriter, message ...string) {
	msg := "Method not allowed"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	WriteErrorByCode(w, errors.ErrCodeMethodNotAllowed, msg)
}

// Conflict 返回409错误
func Conflict(w http.ResponseWriter, message ...string) {
	msg := "Conflict"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	WriteErrorByCode(w, errors.ErrCodeConflict, msg)
}

// TooManyRequests 返回429错误
func TooManyRequests(w http.ResponseWriter, message ...string) {
	msg := "Too many requests"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	WriteErrorByCode(w, errors.ErrCodeTooManyRequests, msg)
}

// InternalServerError 返回500错误
func InternalServerError(w http.ResponseWriter, message ...string) {
	msg := "Internal server error"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	WriteErrorByCode(w, errors.ErrCodeInternalServerError, msg)
}

// ServiceUnavailable 返回503错误
func ServiceUnavailable(w http.ResponseWriter, message ...string) {
	msg := "Service unavailable"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	WriteErrorByCode(w, errors.ErrCodeServiceUnavailable, msg)
}

// GatewayTimeout 返回504错误
func GatewayTimeout(w http.ResponseWriter, message ...string) {
	msg := "Gateway timeout"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	WriteErrorByCode(w, errors.ErrCodeGatewayTimeout, msg)
}

// 列表响应辅助函数

// ListData 构建列表数据
func ListData(items interface{}, total int64, page, pageSize int) *ListResponse {
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	hasMore := page < totalPages

	return &ListResponse{
		List:       items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
		HasMore:    hasMore,
	}
}

// PaginationData 构建分页数据
func PaginationData(total int64, page, pageSize int) *PaginationMeta {
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	hasMore := page < totalPages

	return &PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		HasMore:    hasMore,
	}
}

// OKList 返回列表成功响应
func OKList(w http.ResponseWriter, items interface{}, total int64, page, pageSize int, message ...string) {
	pagination := PaginationData(total, page, pageSize)
	WriteList(w, items, pagination, message...)
}

// 不同格式的快速响应函数

// AliCloudOK 返回阿里云格式的成功响应
func AliCloudOK(w http.ResponseWriter, data interface{}, message ...string) {
	WriteStandardResponse(w, FormatAliCloud, http.StatusOK, data, message...)
}

// AliCloudErr 返回阿里云格式的错误响应
func AliCloudErr(w http.ResponseWriter, code errors.ErrorCode, details ...string) {
	WriteStandardErrorWithCode(w, FormatAliCloud, code, details...)
}

// TencentOK 返回腾讯云格式的成功响应
func TencentOK(w http.ResponseWriter, data interface{}, message ...string) {
	WriteStandardResponse(w, FormatTencent, http.StatusOK, data, message...)
}

// TencentErr 返回腾讯云格式的错误响应
func TencentErr(w http.ResponseWriter, code errors.ErrorCode, details ...string) {
	WriteStandardErrorWithCode(w, FormatTencent, code, details...)
}

// BaiduOK 返回百度格式的成功响应
func BaiduOK(w http.ResponseWriter, data interface{}, message ...string) {
	WriteStandardResponse(w, FormatBaidu, http.StatusOK, data, message...)
}

// BaiduErr 返回百度格式的错误响应
func BaiduErr(w http.ResponseWriter, code errors.ErrorCode, details ...string) {
	WriteStandardErrorWithCode(w, FormatBaidu, code, details...)
}

// ByteDanceOK 返回字节跳动格式的成功响应
func ByteDanceOK(w http.ResponseWriter, data interface{}, message ...string) {
	WriteStandardResponse(w, FormatByteDance, http.StatusOK, data, message...)
}

// ByteDanceErr 返回字节跳动格式的错误响应
func ByteDanceErr(w http.ResponseWriter, code errors.ErrorCode, details ...string) {
	WriteStandardErrorWithCode(w, FormatByteDance, code, details...)
}
