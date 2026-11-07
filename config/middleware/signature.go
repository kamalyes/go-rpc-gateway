/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 15:05:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 15:05:00
 * @FilePath: \go-rpc-gateway\config\middleware\signature.go
 * @Description: 签名验证中间件配置模块
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"time"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/sign"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// SignatureType 签名类型
type SignatureType string

const (
	SignatureTypeHMAC SignatureType = "hmac" // HMAC 签名（使用 SecretKey）
	SignatureTypeRSA  SignatureType = "rsa"  // RSA 签名（使用公钥验证）
)

// Signature 签名验证中间件配置
type Signature struct {
	ModuleName       string              `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`                   // 模块名称
	Enabled          bool                `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                              // 是否启用签名验证
	Type             SignatureType       `mapstructure:"type" yaml:"type" json:"type"`                                       // 签名类型 (hmac/rsa)
	SecretKey        string              `mapstructure:"secret-key" yaml:"secret-key" json:"secretKey"`                      // HMAC 签名密钥
	PublicKeyPEM     string              `mapstructure:"public-key-pem" yaml:"public-key-pem" json:"publicKeyPem"`           // RSA 公钥 PEM（用于验证签名）
	NonceKeyPrefix   string              `mapstructure:"nonce-key-prefix" yaml:"nonce-key-prefix" json:"nonceKeyPrefix"`     // Nonce Redis key 前缀
	NonceTTL         time.Duration       `mapstructure:"nonce-ttl" yaml:"nonce-ttl" json:"nonceTtl"`                         // Nonce 过期时间
	RequireTimestamp bool                `mapstructure:"require-timestamp" yaml:"require-timestamp" json:"requireTimestamp"` // 是否强制要求 Timestamp（向后兼容：false 允许旧客户端不传）
	RequireNonce     bool                `mapstructure:"require-nonce" yaml:"require-nonce" json:"requireNonce"`             // 是否强制要求 Nonce（向后兼容：false 允许旧客户端不传）
	Algorithm        sign.HashCryptoFunc `mapstructure:"algorithm" yaml:"algorithm" json:"algorithm"`                        // 签名算法 (MD5, SHA1, SHA224, SHA256, SHA384, SHA512)
	TimeoutWindow    time.Duration       `mapstructure:"timeout-window" yaml:"timeout-window" json:"timeoutWindow"`          // 请求时间窗口
	IgnorePaths      []string            `mapstructure:"ignore-paths" yaml:"ignore-paths" json:"ignorePaths"`                // 忽略签名验证的路径
	SkipQuery        bool                `mapstructure:"skip-query" yaml:"skip-query" json:"skipQuery"`                      // 是否跳过查询参数
	SkipHeaders      []string            `mapstructure:"skip-headers" yaml:"skip-headers" json:"skipHeaders"`                // 不参与签名计算的 headers
	RequiredHeaders  []string            `mapstructure:"required-headers" yaml:"required-headers" json:"requiredHeaders"`    // 签名校验必须包含的 headers
	TimestampFormat  string              `mapstructure:"timestamp-format" yaml:"timestamp-format" json:"timestampFormat"`    // 时间戳格式
	SkipBody         bool                `mapstructure:"skip-body" yaml:"skip-body" json:"skipBody"`                         // 是否跳过请求体
}

// DefaultSignature 创建默认签名验证配置
func DefaultSignature() *Signature {
	return &Signature{
		ModuleName:       "signature",
		Enabled:          false,
		Type:             SignatureTypeHMAC, // 默认使用 HMAC
		SecretKey:        "default-secret-key-change-in-production",
		PublicKeyPEM:     "", // RSA 公钥（使用时需要配置）
		NonceKeyPrefix:   "nonce:",
		NonceTTL:         10 * time.Minute,
		RequireTimestamp: false, // 默认不强制要求（向后兼容旧客户端）
		RequireNonce:     false, // 默认不强制要求（向后兼容旧客户端）
		Algorithm:        sign.AlgorithmSHA256,
		TimeoutWindow:    time.Minute * 5,
		IgnorePaths: []string{
			"/health",
			"/metrics",
			"/ping",
		},
		SkipHeaders: []string{
			"Authorization",
			"User-Agent",
			"Accept",
			"Accept-Encoding",
			"Accept-Language",
			"Connection",
			"Cache-Control",
			"X-Signature",
		},
		RequiredHeaders: []string{
			"X-Timestamp",
			"Content-Type",
		},
		TimestampFormat: "unix",
		SkipQuery:       false,
		SkipBody:        false,
	}
}

// Get 返回配置接口
func (s *Signature) Get() interface{} {
	return s
}

// Set 设置配置数据
func (s *Signature) Set(data interface{}) {
	if cfg, ok := data.(*Signature); ok {
		*s = *cfg
	}
}

// Clone 返回配置的副本
func (s *Signature) Clone() internal.Configurable {
	var cloned Signature
	if err := syncx.DeepCopy(&cloned, s); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Signature{}
	}
	return &cloned
}

// Validate 验证配置
func (s *Signature) Validate() error {
	return internal.ValidateStruct(s)
}

// WithSecretKey 设置签名密钥
func (s *Signature) WithSecretKey(secretKey string) *Signature {
	s.SecretKey = secretKey
	return s
}

// WithAlgorithm 设置签名算法
func (s *Signature) WithAlgorithm(algorithm sign.HashCryptoFunc) *Signature {
	s.Algorithm = algorithm
	return s
}

// WithTimeoutWindow 设置时间窗口
func (s *Signature) WithTimeoutWindow(timeout time.Duration) *Signature {
	s.TimeoutWindow = timeout
	return s
}

// AddIgnorePath 添加忽略路径
func (s *Signature) AddIgnorePath(path string) *Signature {
	s.IgnorePaths = append(s.IgnorePaths, path)
	return s
}

// WithSkipQuery 设置是否跳过查询参数
func (s *Signature) WithSkipQuery(skip bool) *Signature {
	s.SkipQuery = skip
	return s
}

// WithSkipHeaders 设置不参与签名计算的 headers
func (s *Signature) WithSkipHeaders(headers []string) *Signature {
	s.SkipHeaders = headers
	return s
}

// WithRequiredHeaders 设置签名校验必须包含的 headers
func (s *Signature) WithRequiredHeaders(headers []string) *Signature {
	s.RequiredHeaders = headers
	return s
}

// WithTimestampFormat 设置时间戳格式
func (s *Signature) WithTimestampFormat(format string) *Signature {
	s.TimestampFormat = format
	return s
}

// WithSkipBody 设置是否跳过请求体
func (s *Signature) WithSkipBody(skip bool) *Signature {
	s.SkipBody = skip
	return s
}

// Enable 启用签名验证中间件
func (s *Signature) Enable() *Signature {
	s.Enabled = true
	return s
}

// Disable 禁用签名验证中间件
func (s *Signature) Disable() *Signature {
	s.Enabled = false
	return s
}

// IsEnabled 检查是否启用
func (s *Signature) IsEnabled() bool {
	return s.Enabled
}
