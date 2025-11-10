/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:40:10
 * @FilePath: \go-rpc-gateway\constants\middleware_signature.go
 * @Description: 签名验证中间件相关常量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

import "time"

// 签名算法常量
const (
	SignatureAlgorithmHMAC256 = "HMAC-SHA256"
	SignatureAlgorithmHMAC512 = "HMAC-SHA512"
	SignatureAlgorithmMD5     = "MD5"
	SignatureAlgorithmSHA1    = "SHA1"
	SignatureAlgorithmSHA256  = "SHA256"
)

// 签名默认配置常量
const (
	// 默认算法
	SignatureDefaultAlgorithm = SignatureAlgorithmHMAC256

	// 默认时间窗口（秒）
	SignatureDefaultTimeWindow = 300 // 5分钟

	// 默认签名头
	SignatureDefaultHeader = HeaderXSignature

	// 默认时间戳头
	SignatureDefaultTimestampHeader = HeaderXTimestamp

	// 默认是否启用
	SignatureDefaultEnabled = false

	// 默认密钥长度
	SignatureDefaultSecretKeyLength = 32
)

// 签名验证模式
const (
	SignatureModeStrict = "strict" // 严格模式，必须验证所有字段
	SignatureModeLoose  = "loose"  // 宽松模式，部分验证
	SignatureModeBasic  = "basic"  // 基础模式，仅验证签名
)

// 签名参数包含方式
const (
	SignatureIncludeHeaders = "headers"
	SignatureIncludeBody    = "body"
	SignatureIncludeQuery   = "query"
	SignatureIncludeAll     = "all"
)

// 时间戳验证配置
const (
	// 最大时间偏差（秒）
	SignatureMaxTimeSkew = 300 // 5分钟

	// 时间戳格式
	SignatureTimestampFormat = "2006-01-02T15:04:05Z07:00"

	// Unix 时间戳格式
	SignatureTimestampFormatUnix = "unix"
)

// 默认时间窗口
const SignatureDefaultTimeWindowDuration = 5 * time.Minute

// 默认跳过的头部（不参与签名计算）
var SignatureDefaultSkipHeaders = []string{
	"Authorization",
	"User-Agent",
	"Accept",
	"Accept-Encoding",
	"Accept-Language",
	"Connection",
	"Cache-Control",
	HeaderXSignature, // 签名本身不参与计算
}

// 必须包含的头部（参与签名计算）
var SignatureDefaultRequiredHeaders = []string{
	HeaderXTimestamp,
	HeaderContentType,
}

// 错误信息常量
const (
	SignatureErrorInvalid          = "Invalid signature"
	SignatureErrorMissing          = "Signature missing"
	SignatureErrorTimestampMissing = "Timestamp missing"
	SignatureErrorTimestampExpired = "Timestamp expired"
	SignatureErrorTimestampInvalid = "Invalid timestamp format"
)
