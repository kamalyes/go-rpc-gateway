/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-02 11:08:27
 * @FilePath: \go-rpc-gateway\constants\middleware_signature.go
 * @Description: 签名验证中间件相关常量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

// 错误信息常量
const (
	SignatureErrorInvalid          = "Invalid signature"
	SignatureErrorMissing          = "Signature missing"
	SignatureErrorTimestampMissing = "Timestamp missing"
	SignatureErrorTimestampExpired = "Timestamp expired"
	SignatureErrorTimestampInvalid = "Invalid timestamp format"
	SignatureErrorSecretKeyMissing = "Secret key missing"
	SignatureErrorBodyReadFailed   = "Failed to read request body"
	SignatureErrorGenerateFailed   = "Failed to generate signature"
	SignatureErrorMismatch         = "Signature mismatch"
)

// 签名验证错误代码
const (
	SignatureErrorCodeInvalid          = "SIGNATURE_INVALID"
	SignatureErrorCodeMissing          = "SIGNATURE_MISSING"
	SignatureErrorCodeTimestampMissing = "TIMESTAMP_MISSING"
	SignatureErrorCodeTimestampInvalid = "TIMESTAMP_INVALID"
	SignatureErrorCodeTimestampExpired = "TIMESTAMP_EXPIRED"
	SignatureErrorCodeSecretKeyMissing = "SECRET_KEY_MISSING"
	SignatureErrorCodeBodyReadFailed   = "BODY_READ_FAILED"
	SignatureErrorCodeGenerateFailed   = "SIGNATURE_GENERATE_FAILED"
)
