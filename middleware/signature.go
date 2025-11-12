/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 18:04:41
 * @FilePath: \go-rpc-gateway\middleware\signature.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/kamalyes/go-rpc-gateway/constants"
)

// SignatureConfig 签名验证配置
type SignatureConfig struct {
	Enabled        bool          `json:"enabled" yaml:"enabled"`               // 是否启用签名验证
	SecretKey      string        `json:"secretKey" yaml:"secretKey"`           // 签名密钥
	ExpireDuration time.Duration `json:"expireDuration" yaml:"expireDuration"` // 签名过期时间
	TimestampField string        `json:"timestampField" yaml:"timestampField"` // 时间戳字段名
	SignatureField string        `json:"signatureField" yaml:"signatureField"` // 签名字段名
	Algorithm      string        `json:"algorithm" yaml:"algorithm"`           // 签名算法
	IncludeQuery   bool          `json:"includeQuery" yaml:"includeQuery"`     // 是否包含查询参数
	IncludeBody    bool          `json:"includeBody" yaml:"includeBody"`       // 是否包含请求体
}

// DefaultSignatureConfig 默认签名配置
func DefaultSignatureConfig() *SignatureConfig {
	return &SignatureConfig{
		Enabled:        true,
		ExpireDuration: 10 * time.Minute,
		TimestampField: "timestamp",
		SignatureField: "signature",
		Algorithm:      "HMAC-SHA256",
		IncludeQuery:   true,
		IncludeBody:    true,
	}
}

// RequestCommon 通用请求结构
type RequestCommon struct {
	Timestamp     string `json:"timestamp" header:"X-Timestamp"`       // 时间戳 (constants.HeaderXTimestamp)
	Signature     string `json:"signature" header:"X-Signature"`       // 签名 (constants.HeaderXSignature)
	TraceID       string `json:"traceId" header:"X-Trace-Id"`          // 链路追踪ID (constants.HeaderXTraceID)
	RequestID     string `json:"requestId" header:"X-Request-Id"`      // 请求ID (constants.HeaderXRequestID)
	Authorization string `json:"authorization" header:"Authorization"` // 授权信息 (constants.HeaderAuthorization)
	DeviceID      string `json:"deviceId" header:"X-Device-Id"`        // 设备ID (constants.HeaderXDeviceID)
	AppVersion    string `json:"appVersion" header:"X-App-Version"`    // 应用版本 (constants.HeaderXAppVersion)
	Platform      string `json:"platform" header:"X-Platform"`         // 平台 (constants.HeaderXPlatform)
}

// SignatureValidator 签名验证器接口
type SignatureValidator interface {
	Validate(r *http.Request, config *SignatureConfig) error
	GenerateSignature(reqCommon *RequestCommon, secretKey string, body []byte, query url.Values) (string, error)
}

// HMACValidator HMAC 签名验证器
type HMACValidator struct{}

// Validate 验证签名
func (v *HMACValidator) Validate(r *http.Request, config *SignatureConfig) error {
	if !config.Enabled {
		return nil
	}

	// 提取请求公共信息
	reqCommon := extractRequestCommon(r, config)

	// 验证时间戳
	if err := v.validateTimestamp(reqCommon.Timestamp, config.ExpireDuration); err != nil {
		return err
	}

	// 读取请求体
	var body []byte
	if config.IncludeBody && r.Body != nil {
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
	if config.IncludeQuery {
		query = r.URL.Query()
	}

	// 生成期望的签名
	expectedSign, err := v.GenerateSignature(reqCommon, config.SecretKey, body, query)
	if err != nil {
		return fmt.Errorf("failed to generate signature: %w", err)
	}

	// 验证签名
	if expectedSign != reqCommon.Signature {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// GenerateSignature 生成签名
func (v *HMACValidator) GenerateSignature(reqCommon *RequestCommon, secretKey string, body []byte, query url.Values) (string, error) {
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

	// 生成 HMAC-SHA256 签名
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(dataToSign))
	signature := h.Sum(nil)

	// 返回 Base64 编码的签名
	return base64.StdEncoding.EncodeToString(signature), nil
}

// validateTimestamp 验证时间戳
func (v *HMACValidator) validateTimestamp(timestampStr string, expireDuration time.Duration) error {
	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp format: %w", err)
	}

	now := time.Now().Unix()
	if now-timestamp > int64(expireDuration.Seconds()) {
		return fmt.Errorf("timestamp expired")
	}

	return nil
}

// extractRequestCommon 提取请求公共信息
func extractRequestCommon(r *http.Request, config *SignatureConfig) *RequestCommon {
	return &RequestCommon{
		Timestamp:     getValueFromRequest(r, config.TimestampField),
		Signature:     getValueFromRequest(r, config.SignatureField),
		TraceID:       r.Header.Get(constants.HeaderXTraceID),
		RequestID:     r.Header.Get(constants.HeaderXRequestID),
		Authorization: r.Header.Get(constants.HeaderAuthorization),
		DeviceID:      r.Header.Get(constants.HeaderXDeviceID),
		AppVersion:    r.Header.Get(constants.HeaderXAppVersion),
		Platform:      r.Header.Get(constants.HeaderXPlatform),
	}
}

// getValueFromRequest 从请求中获取值（优先从 Header，然后从 Query）
func getValueFromRequest(r *http.Request, fieldName string) string {
	// 尝试从 Header 获取
	if value := r.Header.Get("X-" + fieldName); value != "" {
		return value
	}

	// 尝试从 Query 参数获取
	if value := r.URL.Query().Get(fieldName); value != "" {
		return value
	}

	return ""
}

// SignatureMiddleware 签名验证中间件
func SignatureMiddleware(config *SignatureConfig, validator SignatureValidator) HTTPMiddleware {
	if config == nil {
		config = DefaultSignatureConfig()
	}

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
				writeErrorResponse(w, http.StatusUnauthorized, "SIGNATURE_INVALID", err.Error())
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TimestampMiddleware 时间戳验证中间件（独立使用）
func TimestampMiddleware(config *SignatureConfig) HTTPMiddleware {
	if config == nil {
		config = DefaultSignatureConfig()
	}

	if !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			timestampStr := getValueFromRequest(r, config.TimestampField)
			if timestampStr == "" {
				writeErrorResponse(w, http.StatusBadRequest, "TIMESTAMP_MISSING", "Timestamp is required")
				return
			}

			timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
			if err != nil {
				writeErrorResponse(w, http.StatusBadRequest, "TIMESTAMP_INVALID", "Invalid timestamp format")
				return
			}

			now := time.Now().Unix()
			if now-timestamp > int64(config.ExpireDuration.Seconds()) {
				writeErrorResponse(w, http.StatusUnauthorized, "TIMESTAMP_EXPIRED", "Timestamp expired")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// writeErrorResponse 写入错误响应
func writeErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
	w.WriteHeader(statusCode)

	response := fmt.Sprintf(`{
		"error": "%s",
		"message": "%s",
		"timestamp": %d
	}`, errorCode, message, time.Now().Unix())

	w.Write([]byte(response))
} // SignatureMiddlewareWithConfig 带配置的签名中间件
func SignatureMiddlewareWithConfig(secretKey string, expireDuration time.Duration) HTTPMiddleware {
	config := &SignatureConfig{
		Enabled:        true,
		SecretKey:      secretKey,
		ExpireDuration: expireDuration,
		TimestampField: "timestamp",
		SignatureField: "signature",
		Algorithm:      "HMAC-SHA256",
		IncludeQuery:   true,
		IncludeBody:    true,
	}

	return SignatureMiddleware(config, &HMACValidator{})
}
