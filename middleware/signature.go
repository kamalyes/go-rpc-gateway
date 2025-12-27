/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 15:58:58
 * @FilePath: \go-rpc-gateway\middleware\signature.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kamalyes/go-config/pkg/request"
	"github.com/kamalyes/go-config/pkg/signature"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/response"
	"github.com/kamalyes/go-toolbox/pkg/sign"
)

// RequestCommon 通用请求结构 - 继承自go-config的BaseRequest
type RequestCommon struct {
	request.BaseRequest
	// 可以在这里添加Gateway特有的字段
}

// SignatureValidator 签名验证器接口
type SignatureValidator interface {
	Validate(r *http.Request, config *signature.Signature) error
	GenerateSignature(reqCommon *RequestCommon, secretKey string, body []byte, query url.Values, algorithm sign.HashCryptoFunc) (string, error)
}

// HMACValidator HMAC 签名验证器
type HMACValidator struct{}

// Validate 验证签名
func (v *HMACValidator) Validate(r *http.Request, config *signature.Signature) error {
	if !config.Enabled {
		return nil
	}

	// 检查是否在忽略路径中
	for _, ignorePath := range config.IgnorePaths {
		if r.URL.Path == ignorePath || strings.HasPrefix(r.URL.Path, ignorePath) {
			return nil
		}
	}

	// 提取请求公共信息
	reqCommon := extractRequestCommon(r, config)

	// 验证时间戳
	if err := v.validateTimestamp(reqCommon.Timestamp, config.TimeoutWindow); err != nil {
		return err
	}

	// 读取请求体
	var body []byte
	if !config.SkipBody && r.Body != nil {
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("failed to read request body: %w", err)
		}
		// 重新设置请求体
		r.Body = io.NopCloser(bytes.NewBuffer(body))
	}

	// 获取查询参数
	var query url.Values
	if !config.SkipQuery {
		query = r.URL.Query()
	}

	// 生成期望的签名
	expectedSign, err := v.GenerateSignature(reqCommon, config.SecretKey, body, query, config.Algorithm)
	if err != nil {
		return fmt.Errorf("failed to generate signature: %w", err)
	}

	// 验证签名
	if expectedSign != reqCommon.Signature {
		return fmt.Errorf(constants.SignatureErrorMismatch)
	}

	return nil
}

// GenerateSignature 生成签名
func (v *HMACValidator) GenerateSignature(reqCommon *RequestCommon, secretKey string, body []byte, query url.Values, algorithm sign.HashCryptoFunc) (string, error) {
	// 构建签名数据
	var dataToSign string

	// 添加时间戳
	dataToSign += reqCommon.Timestamp

	// 添加查询参数
	if query != nil {
		dataToSign += query.Encode()
	}

	// 添加请求体
	if body != nil {
		dataToSign += string(body)
	}

	// 使用 go-toolbox 的 HMAC 签名器
	signer, err := sign.NewHMACSigner(algorithm)
	if err != nil {
		return "", fmt.Errorf("failed to create HMAC signer: %w", err)
	}

	// 生成签名
	signatureBytes, err := signer.Sign([]byte(dataToSign), []byte(secretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign data: %w", err)
	}

	// 返回 Base64 编码的签名
	return base64.StdEncoding.EncodeToString(signatureBytes), nil
}

// validateTimestamp 验证时间戳
func (v *HMACValidator) validateTimestamp(timestampStr string, expireDuration time.Duration) error {
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return fmt.Errorf("%s: %w", constants.SignatureErrorTimestampInvalid, err)
	}

	now := time.Now().Unix()
	if now-timestamp > int64(expireDuration.Seconds()) {
		return fmt.Errorf(constants.SignatureErrorTimestampExpired)
	}

	return nil
}

// extractRequestCommon 提取请求公共信息
func extractRequestCommon(r *http.Request, config *signature.Signature) *RequestCommon {
	return &RequestCommon{
		BaseRequest: request.BaseRequest{
			Timestamp:     getValueFromRequest(r, config.TimestampHeader),
			Signature:     getValueFromRequest(r, config.SignatureHeader),
			TraceID:       r.Header.Get(constants.HeaderXTraceID),
			RequestID:     r.Header.Get(constants.HeaderXRequestID),
			Authorization: r.Header.Get(constants.HeaderAuthorization),
			DeviceID:      r.Header.Get(constants.HeaderXDeviceID),
			AppVersion:    r.Header.Get(constants.HeaderXAppVersion),
			Platform:      r.Header.Get(constants.HeaderXPlatform),
		},
	}
}

// getValueFromRequest 从请求中获取值（优先从 Header，然后从 Query）
func getValueFromRequest(r *http.Request, fieldName string) string {
	// 尝试从 Header 获取
	if value := r.Header.Get(fieldName); value != "" {
		return value
	}

	// 尝试从 Query 参数获取
	if value := r.URL.Query().Get(fieldName); value != "" {
		return value
	}

	return ""
}

// SignatureMiddleware 签名验证中间件
func SignatureMiddleware(config *signature.Signature, validator SignatureValidator) HTTPMiddleware {
	if !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	if validator == nil {
		validator = &HMACValidator{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// 验证签名
			if err := validator.Validate(r, config); err != nil {
				response.WriteErrorResponseWithCode(w, http.StatusForbidden, constants.SignatureErrorCodeInvalid, err.Error())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// TimestampMiddleware 时间戳验证中间件（独立使用）
func TimestampMiddleware(config *signature.Signature) HTTPMiddleware {
	if !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			timestampStr := getValueFromRequest(r, config.TimestampHeader)
			if timestampStr == "" {
				response.WriteErrorResponseWithCode(w, http.StatusBadRequest, constants.SignatureErrorCodeTimestampMissing, constants.SignatureErrorTimestampMissing)
				return
			}
			timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
			if err != nil {
				response.WriteErrorResponseWithCode(w, http.StatusBadRequest, constants.SignatureErrorCodeTimestampInvalid, constants.SignatureErrorTimestampInvalid)
				return
			}
			now := time.Now().Unix()
			if now-timestamp > int64(config.TimeoutWindow.Seconds()) {
				response.WriteErrorResponseWithCode(w, http.StatusForbidden, constants.SignatureErrorCodeTimestampExpired, constants.SignatureErrorTimestampExpired)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
