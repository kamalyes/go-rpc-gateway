/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 10:11:54
 * @FilePath: \go-rpc-gateway\server\response.go
 * @Description: HTTP响应标准化工具模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package response

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	commonapis "github.com/kamalyes/go-rpc-gateway/proto"
)

// jsonEncoderPool JSON 编码器对象池
var jsonEncoderPool = sync.Pool{
	New: func() any {
		return json.NewEncoder(io.Discard)
	},
}

// HTTPStatus 定义HTTP状态码对应的Result Code
const (
	// 成功状态
	HTTPStatusOK = 200

	// 客户端错误状态
	HTTPStatusBadRequest       = 400
	HTTPStatusUnauthorized     = 401
	HTTPStatusForbidden        = 403
	HTTPStatusNotFound         = 404
	HTTPStatusMethodNotAllowed = 405
	HTTPStatusConflict         = 409
	HTTPStatusTooManyRequests  = 429

	// 服务器错误状态
	HTTPStatusInternalServerError = 500
	HTTPStatusBadGateway          = 502
	HTTPStatusServiceUnavailable  = 503
	HTTPStatusGatewayTimeout      = 504
)

// WriteResult 写入标准化Result响应
func WriteResult(w http.ResponseWriter, httpStatus int, result *commonapis.Result) {
	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
	w.WriteHeader(httpStatus)

	encoder := jsonEncoderPool.Get().(*json.Encoder)
	defer jsonEncoderPool.Put(encoder)

	// 创建新的 encoder 指向当前 writer
	*encoder = *json.NewEncoder(w)

	if err := encoder.Encode(result); err != nil && global.LOGGER != nil {
		global.LOGGER.WithError(err).ErrorMsg("Failed to encode Result response")
	}
}

// WriteSuccessResult 写入成功响应
func WriteSuccessResult(w http.ResponseWriter, message string) {
	result := &commonapis.Result{
		Code:   HTTPStatusOK,
		Error:  message,
		Status: commonapis.StatusCode_OK,
	}
	WriteResult(w, http.StatusOK, result)
}

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
func WriteAppErrorf(w http.ResponseWriter, code errors.ErrorCode, format string, args ...interface{}) {
	appErr := errors.NewErrorf(code, format, args...)
	WriteAppError(w, appErr)
}

// WriteJSONResponse 写入自定义JSON响应
func WriteJSONResponse(w http.ResponseWriter, httpStatus int, data interface{}) {
	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
	w.WriteHeader(httpStatus)

	encoder := jsonEncoderPool.Get().(*json.Encoder)
	defer jsonEncoderPool.Put(encoder)

	// 创建新的 encoder 指向当前 writer
	*encoder = *json.NewEncoder(w)

	if err := encoder.Encode(data); err != nil && global.LOGGER != nil {
		global.LOGGER.WithError(err).ErrorMsg("Failed to encode JSON response")
	}
}

// VersionInfo 版本信息结构
type VersionInfo struct {
	Version   string `json:"version"`
	GitBranch string `json:"git_branch"`
	GitHash   string `json:"git_hash"`
	BuildTime string `json:"build_time"`
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

// CSRFTokenResponse CSRF token响应结构
type CSRFTokenResponse struct {
	CSRFToken string `json:"csrf_token"`
}

// WriteCSRFTokenResponse 写入CSRF token响应
func WriteCSRFTokenResponse(w http.ResponseWriter, token string) {
	tokenResponse := &CSRFTokenResponse{
		CSRFToken: token,
	}
	WriteJSONResponse(w, http.StatusOK, tokenResponse)
}

// PProfStatusResponse PProf状态响应结构
type PProfStatusResponse struct {
	PProfEnabled   bool   `json:"pprof_enabled"`
	PProfPath      string `json:"pprof_path"`
	AuthRequired   bool   `json:"auth_required"`
	EndpointsCount int    `json:"endpoints_count"`
}

// WritePProfStatusResponse 写入PProf状态响应
func WritePProfStatusResponse(w http.ResponseWriter, enabled bool, path string, authRequired bool, endpointsCount int) {
	statusResponse := &PProfStatusResponse{
		PProfEnabled:   enabled,
		PProfPath:      path,
		AuthRequired:   authRequired,
		EndpointsCount: endpointsCount,
	}
	WriteJSONResponse(w, http.StatusOK, statusResponse)
}

// WriteHealthCheckResult 写入健康检查结果
func WriteHealthCheckResult(w http.ResponseWriter, isHealthy bool, component string, message string, details map[string]interface{}) {
	if isHealthy {
		result := &commonapis.Result{
			Code:   HTTPStatusOK,
			Error:  message,
			Status: commonapis.StatusCode_OK,
		}
		WriteResult(w, http.StatusOK, result)
	} else {
		result := &commonapis.Result{
			Code:   HTTPStatusServiceUnavailable,
			Error:  message,
			Status: commonapis.StatusCode_Unavailable,
		}
		WriteResult(w, http.StatusServiceUnavailable, result)
	}
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
