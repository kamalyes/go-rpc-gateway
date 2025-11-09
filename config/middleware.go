/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 01:14:03
 * @FilePath: \go-rpc-gateway\config\middleware.go
 * @Description: 中间件配置模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package config

// MiddlewareConfig 中间件配置
type MiddlewareConfig struct {
	// 限流配置
	RateLimit RateLimitConfig `mapstructure:"rate_limit" yaml:"rate_limit" json:"rate_limit"`

	// 访问记录
	AccessLog AccessLogConfig `mapstructure:"access_log" yaml:"access_log" json:"access_log"`

	// 请求签名
	Signature SignatureConfig `mapstructure:"signature" yaml:"signature" json:"signature"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled    bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Algorithm  string `mapstructure:"algorithm" yaml:"algorithm" json:"algorithm"` // token_bucket, sliding_window
	Rate       int    `mapstructure:"rate" yaml:"rate" json:"rate"`
	Burst      int    `mapstructure:"burst" yaml:"burst" json:"burst"`
	WindowSize int    `mapstructure:"window_size" yaml:"window_size" json:"window_size"`
}

// AccessLogConfig 访问日志配置
type AccessLogConfig struct {
	Enabled        bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Format         string `mapstructure:"format" yaml:"format" json:"format"` // json, text
	IncludeBody    bool   `mapstructure:"include_body" yaml:"include_body" json:"include_body"`
	IncludeHeaders bool   `mapstructure:"include_headers" yaml:"include_headers" json:"include_headers"`
}

// SignatureConfig 请求签名配置
type SignatureConfig struct {
	Enabled   bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Algorithm string   `mapstructure:"algorithm" yaml:"algorithm" json:"algorithm"` // hmac-sha256
	SecretKey string   `mapstructure:"secret_key" yaml:"secret_key" json:"secret_key"`
	SkipPaths []string `mapstructure:"skip_paths" yaml:"skip_paths" json:"skip_paths"`
	TTL       int      `mapstructure:"ttl" yaml:"ttl" json:"ttl"` // 签名有效期（秒）
}