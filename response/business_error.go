/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-15 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 00:00:00
 * @FilePath: \go-rpc-gateway\response\business_error.go
 * @Description: 业务错误码处理 - 统一处理业务服务返回的错误码
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package response

import (
	"encoding/json"
	"net/http"

	"github.com/kamalyes/go-rpc-gateway/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BusinessError 业务错误接口
// 业务服务应该返回实现此接口的错误
type BusinessError interface {
	error
	GetCode() int32        // 业务错误码
	GetMessage() string    // 错误消息
	GetDetails() string    // 详细信息（可选）
}

// StandardBusinessError 标准业务错误实现
type StandardBusinessError struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e *StandardBusinessError) Error() string {
	return e.Message
}

func (e *StandardBusinessError) GetCode() int32 {
	return e.Code
}

func (e *StandardBusinessError) GetMessage() string {
	return e.Message
}

func (e *StandardBusinessError) GetDetails() string {
	return e.Details
}

// NewBusinessError 创建业务错误
func NewBusinessError(code int32, message string) *StandardBusinessError {
	return &StandardBusinessError{
		Code:    code,
		Message: message,
	}
}

// NewBusinessErrorWithDetails 创建带详情的业务错误
func NewBusinessErrorWithDetails(code int32, message, details string) *StandardBusinessError {
	return &StandardBusinessError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// ExtractBusinessError 从gRPC status中提取业务错误
// 业务服务应该使用 status.Error() 返回错误，并在details中包含业务错误码
func ExtractBusinessError(err error) BusinessError {
	if err == nil {
		return nil
	}

	// 1. 检查是否已经是BusinessError
	if bizErr, ok := err.(BusinessError); ok {
		return bizErr
	}

	// 2. 从gRPC status提取
	st, ok := status.FromError(err)
	if !ok {
		return nil
	}

	// 3. 检查status details中是否包含业务错误码
	for _, detail := range st.Details() {
		// 如果业务服务使用protobuf定义错误，可以在这里解析
		// 例如：if bizErr, ok := detail.(*pb.BusinessError); ok { ... }
		if bizErr, ok := detail.(BusinessError); ok {
			return bizErr
		}
	}

	// 4. 尝试从message中解析JSON格式的错误码
	var bizErr StandardBusinessError
	if err := json.Unmarshal([]byte(st.Message()), &bizErr); err == nil {
		if bizErr.Code != 0 {
			return &bizErr
		}
	}

	return nil
}

// WriteBusinessErrorResponse 写入业务错误响应
// 优先处理业务错误，然后才是Gateway错误
func WriteBusinessErrorResponse(w http.ResponseWriter, err error) {
	// 1. 尝试提取业务错误
	if bizErr := ExtractBusinessError(err); bizErr != nil {
		WriteUnifiedJSONResponse(w, Response{
			Code:    bizErr.GetCode(),
			Message: bizErr.GetMessage(),
			Data:    nil,
			Details: bizErr.GetDetails(),
		}, MapBusinessCodeToHTTP(bizErr.GetCode()))
		return
	}

	// 2. 检查是否是Gateway错误
	if gwErr, ok := err.(*errors.AppError); ok {
		WriteErrorResponse(w, gwErr)
		return
	}

	// 3. 从gRPC status映射
	if st, ok := status.FromError(err); ok {
		httpCode := MapGRPCCodeToHTTP(st.Code())
		WriteUnifiedJSONResponse(w, Response{
			Code:    int32(st.Code()),
			Message: st.Message(),
			Data:    nil,
		}, httpCode)
		return
	}

	// 4. 未知错误
	WriteErrorResponse(w, errors.NewErrorf(errors.ErrCodeInternalServerError, err.Error()))
}

// MapBusinessCodeToHTTP 业务错误码映射到HTTP状态码
// 业务可以根据自己的错误码范围自定义映射规则
func MapBusinessCodeToHTTP(code int32) int {
	switch {
	case code == 0:
		return http.StatusOK
	case code >= 1000 && code < 2000:
		// 参数错误 1000-1999
		return http.StatusBadRequest
	case code >= 2000 && code < 3000:
		// 认证错误 2000-2999
		return http.StatusUnauthorized
	case code >= 3000 && code < 4000:
		// 权限错误 3000-3999
		return http.StatusForbidden
	case code >= 4000 && code < 5000:
		// 资源不存在 4000-4999
		return http.StatusNotFound
	case code >= 5000 && code < 6000:
		// 业务逻辑错误 5000-5999
		return http.StatusUnprocessableEntity
	case code >= 6000 && code < 7000:
		// 限流/熔断 6000-6999
		return http.StatusTooManyRequests
	default:
		// 服务器错误 7000+
		return http.StatusInternalServerError
	}
}

// MapGRPCCodeToHTTP gRPC状态码映射到HTTP状态码
func MapGRPCCodeToHTTP(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return http.StatusRequestTimeout
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.FailedPrecondition:
		return http.StatusBadRequest
	case codes.Aborted:
		return http.StatusConflict
	case codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Internal:
		return http.StatusInternalServerError
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DataLoss:
		return http.StatusInternalServerError
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// Response 统一响应格式
type Response struct {
	Code    int32       `json:"code"`              // 业务错误码或Gateway错误码
	Message string      `json:"message"`           // 错误消息
	Data    interface{} `json:"data,omitempty"`    // 响应数据
	Details string      `json:"details,omitempty"` // 详细信息
}

// WriteUnifiedJSONResponse 写入JSON响应（统一版本）
func WriteUnifiedJSONResponse(w http.ResponseWriter, resp Response, httpCode int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(resp)
}
