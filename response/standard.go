/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-17 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 21:46:14
 * @FilePath: \go-rpc-gateway\response\standard.go
 * @Description: 标准化响应体系统 - 支持多种大厂规范的响应格式
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package response

import (
	"encoding/json"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/osx"
	"net/http"
)

// ResponseFormat 响应格式类型
type ResponseFormat string

const (
	// 常见的响应格式
	FormatStandard  ResponseFormat = "standard"  // {code, msg, data}
	FormatAliCloud  ResponseFormat = "alicloud"  // {Code, Message, Data, RequestId}
	FormatTencent   ResponseFormat = "tencent"   // {Response: {Error: {Code, Message}, Data, RequestId}}
	FormatBaidu     ResponseFormat = "baidu"     // {error_code, error_msg, result}
	FormatByteDance ResponseFormat = "bytedance" // {code, message, data}
	FormatMicrosoft ResponseFormat = "microsoft" // {value, error}
	FormatGoogle    ResponseFormat = "google"    // {data, error}
	FormatCustom    ResponseFormat = "custom"    // 自定义格式
)

// StandardResponse 标准响应结构 - 最常见的格式
type StandardResponse struct {
	Code int         `json:"code"`           // 状态码
	Msg  string      `json:"msg"`            // 消息
	Data interface{} `json:"data,omitempty"` // 数据
}

// AliCloudResponse 阿里云风格响应
type AliCloudResponse struct {
	Code      string      `json:"Code"`                // 错误码
	Message   string      `json:"Message"`             // 错误信息
	Data      interface{} `json:"Data,omitempty"`      // 数据
	RequestId string      `json:"RequestId,omitempty"` // 请求ID
}

// TencentResponse 腾讯云风格响应
type TencentResponse struct {
	Response TencentResponseBody `json:"Response"`
}

type TencentError struct {
	Code    string `json:"Code"`    // 错误码
	Message string `json:"Message"` // 错误描述
}

type TencentResponseBody struct {
	Error     *TencentError `json:"Error,omitempty"`     // 错误信息
	Data      interface{}   `json:"Data,omitempty"`      // 数据
	RequestId string        `json:"RequestId,omitempty"` // 请求ID
}

// BaiduResponse 百度风格响应
type BaiduResponse struct {
	ErrorCode int         `json:"error_code"`       // 错误码，0表示成功
	ErrorMsg  string      `json:"error_msg"`        // 错误描述
	Result    interface{} `json:"result,omitempty"` // 结果数据
	LogId     string      `json:"log_id,omitempty"` // 日志ID
}

// ByteDanceResponse 字节跳动风格响应
type ByteDanceResponse struct {
	Code    int         `json:"code"`               // 状态码
	Message string      `json:"message"`            // 消息
	Data    interface{} `json:"data,omitempty"`     // 数据
	TraceId string      `json:"trace_id,omitempty"` // 追踪ID
}

// MicrosoftResponse 微软风格响应
type MicrosoftResponse struct {
	Value interface{}     `json:"value,omitempty"` // 成功时的数据
	Error *MicrosoftError `json:"error,omitempty"` // 错误信息
}

type MicrosoftError struct {
	Code       string                 `json:"code"`                 // 错误码
	Message    string                 `json:"message"`              // 错误消息
	Target     string                 `json:"target,omitempty"`     // 错误目标
	Details    []interface{}          `json:"details,omitempty"`    // 错误详情
	InnerError map[string]interface{} `json:"innerError,omitempty"` // 内部错误
}

// GoogleResponse Google风格响应
type GoogleResponse struct {
	Data  interface{}  `json:"data,omitempty"`  // 数据
	Error *GoogleError `json:"error,omitempty"` // 错误信息
}

type GoogleError struct {
	Code    int    `json:"code"`             // 错误码
	Message string `json:"message"`          // 错误消息
	Status  string `json:"status,omitempty"` // 错误状态
}

// ListResponse 通用列表响应结构
type ListResponse struct {
	List       interface{} `json:"list"`                  // 列表数据
	Total      int64       `json:"total"`                 // 总数
	Page       int         `json:"page,omitempty"`        // 当前页
	PageSize   int         `json:"page_size,omitempty"`   // 页大小
	TotalPages int         `json:"total_pages,omitempty"` // 总页数
	HasMore    bool        `json:"has_more,omitempty"`    // 是否有更多数据
}

// PaginationMeta 分页元数据
type PaginationMeta struct {
	Page       int   `json:"page"`        // 当前页
	PageSize   int   `json:"page_size"`   // 页大小
	Total      int64 `json:"total"`       // 总数
	TotalPages int   `json:"total_pages"` // 总页数
	HasMore    bool  `json:"has_more"`    // 是否有更多数据
}

// ResponseConfig 响应配置
type ResponseConfig struct {
	Format           ResponseFormat         `json:"format"`                  // 响应格式
	IncludeTimestamp bool                   `json:"include_timestamp"`       // 是否包含时间戳
	IncludeRequestId bool                   `json:"include_request_id"`      // 是否包含请求ID
	IncludeTraceId   bool                   `json:"include_trace_id"`        // 是否包含追踪ID
	CustomFields     map[string]interface{} `json:"custom_fields,omitempty"` // 自定义字段
}

// ResponseBuilder 响应构建器
type ResponseBuilder struct {
	config *ResponseConfig
	format ResponseFormat
}

// NewResponseBuilder 创建响应构建器
func NewResponseBuilder(format ResponseFormat) *ResponseBuilder {
	return &ResponseBuilder{
		format: format,
		config: &ResponseConfig{
			Format:           format,
			IncludeTimestamp: true,
			IncludeRequestId: true,
			IncludeTraceId:   true,
		},
	}
}

// WithConfig 设置配置
func (rb *ResponseBuilder) WithConfig(config *ResponseConfig) *ResponseBuilder {
	rb.config = config
	return rb
}

// BuildSuccessResponse 构建成功响应
func (rb *ResponseBuilder) BuildSuccessResponse(data interface{}, message ...string) interface{} {
	msg := "success"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}

	switch rb.format {
	case FormatStandard:
		return &StandardResponse{
			Code: 0,
			Msg:  msg,
			Data: data,
		}
	case FormatAliCloud:
		return &AliCloudResponse{
			Code:      "Success",
			Message:   msg,
			Data:      data,
			RequestId: osx.HashUnixMicroCipherText(), // 生成请求ID
		}
	case FormatTencent:
		return &TencentResponse{
			Response: TencentResponseBody{
				Data:      data,
				RequestId: osx.HashUnixMicroCipherText(), // 生成请求ID
			},
		}
	case FormatBaidu:
		return &BaiduResponse{
			ErrorCode: 0,
			ErrorMsg:  msg,
			Result:    data,
			LogId:     osx.HashUnixMicroCipherText(), // 生成请求ID
		}
	case FormatByteDance:
		return &ByteDanceResponse{
			Code:    0,
			Message: msg,
			Data:    data,
			TraceId: osx.HashUnixMicroCipherText(), // 生成追踪ID
		}
	case FormatMicrosoft:
		return &MicrosoftResponse{
			Value: data,
		}
	case FormatGoogle:
		return &GoogleResponse{
			Data: data,
		}
	default:
		return &StandardResponse{
			Code: 0,
			Msg:  msg,
			Data: data,
		}
	}
}

// BuildErrorResponse 构建错误响应
func (rb *ResponseBuilder) BuildErrorResponse(appErr *errors.AppError) interface{} {
	code := int(appErr.GetCode())
	message := appErr.GetMessage()
	if appErr.GetDetails() != "" {
		message = appErr.GetDetails()
	}

	switch rb.format {
	case FormatStandard:
		return &StandardResponse{
			Code: code,
			Msg:  message,
			Data: nil,
		}
	case FormatAliCloud:
		return &AliCloudResponse{
			Code:      appErr.GetMessage(),
			Message:   message,
			RequestId: osx.HashUnixMicroCipherText(), // 生成请求ID
		}
	case FormatTencent:
		return &TencentResponse{
			Response: TencentResponseBody{
				Error: &TencentError{
					Code:    appErr.GetMessage(),
					Message: message,
				},
				RequestId: osx.HashUnixMicroCipherText(), // 生成请求ID
			},
		}
	case FormatBaidu:
		return &BaiduResponse{
			ErrorCode: code,
			ErrorMsg:  message,
			LogId:     osx.HashUnixMicroCipherText(), // 生成请求ID
		}
	case FormatByteDance:
		return &ByteDanceResponse{
			Code:    code,
			Message: message,
			Data:    nil,
			TraceId: osx.HashUnixMicroCipherText(), // 生成追踪ID
		}
	case FormatMicrosoft:
		return &MicrosoftResponse{
			Error: &MicrosoftError{
				Code:    appErr.GetMessage(),
				Message: message,
			},
		}
	case FormatGoogle:
		return &GoogleResponse{
			Error: &GoogleError{
				Code:    code,
				Message: message,
				Status:  appErr.GetMessage(),
			},
		}
	default:
		return &StandardResponse{
			Code: code,
			Msg:  message,
			Data: nil,
		}
	}
}

// BuildListResponse 构建列表响应
func (rb *ResponseBuilder) BuildListResponse(list interface{}, pagination *PaginationMeta, message ...string) interface{} {
	msg := "success"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}

	listResp := &ListResponse{
		List:  list,
		Total: pagination.Total,
	}

	if pagination != nil {
		listResp.Page = pagination.Page
		listResp.PageSize = pagination.PageSize
		listResp.TotalPages = pagination.TotalPages
		listResp.HasMore = pagination.HasMore
	}

	return rb.BuildSuccessResponse(listResp, msg)
}

// WriteStandardResponse 写入标准响应
func WriteStandardResponse(w http.ResponseWriter, format ResponseFormat, httpStatus int, data interface{}, message ...string) {
	builder := NewResponseBuilder(format)
	response := builder.BuildSuccessResponse(data, message...)
	writeJSONResponse(w, httpStatus, response)
}

// WriteStandardError 写入标准错误响应
func WriteStandardError(w http.ResponseWriter, format ResponseFormat, appErr *errors.AppError) {
	builder := NewResponseBuilder(format)
	response := builder.BuildErrorResponse(appErr)
	writeJSONResponse(w, appErr.GetHTTPStatus(), response)
}

// WriteStandardErrorWithCode 根据错误码写入标准错误响应
func WriteStandardErrorWithCode(w http.ResponseWriter, format ResponseFormat, code errors.ErrorCode, details ...string) {
	detail := ""
	if len(details) > 0 {
		detail = details[0]
	}
	appErr := errors.NewError(code, detail)
	WriteStandardError(w, format, appErr)
}

// WriteStandardList 写入标准列表响应
func WriteStandardList(w http.ResponseWriter, format ResponseFormat, list interface{}, pagination *PaginationMeta, message ...string) {
	builder := NewResponseBuilder(format)
	response := builder.BuildListResponse(list, pagination, message...)
	writeJSONResponse(w, http.StatusOK, response)
}

// writeJSONResponse 写入JSON响应
func writeJSONResponse(w http.ResponseWriter, httpStatus int, data interface{}) {
	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
	w.WriteHeader(httpStatus)

	if err := json.NewEncoder(w).Encode(data); err != nil && global.LOGGER != nil {
		global.LOGGER.Error("Failed to encode JSON response: %v", err)
	}
}

// 便捷方法 - 使用默认标准格式

// WriteSuccess 写入成功响应（使用标准格式）
func WriteSuccess(w http.ResponseWriter, data interface{}, message ...string) {
	WriteStandardResponse(w, FormatStandard, http.StatusOK, data, message...)
}

// WriteError 写入错误响应（使用标准格式）
func WriteError(w http.ResponseWriter, appErr *errors.AppError) {
	WriteStandardError(w, FormatStandard, appErr)
}

// WriteErrorByCode 根据错误码写入错误响应（使用标准格式）
func WriteErrorByCode(w http.ResponseWriter, code errors.ErrorCode, details ...string) {
	WriteStandardErrorWithCode(w, FormatStandard, code, details...)
}

// WriteList 写入列表响应（使用标准格式）
func WriteList(w http.ResponseWriter, list interface{}, pagination *PaginationMeta, message ...string) {
	WriteStandardList(w, FormatStandard, list, pagination, message...)
}
