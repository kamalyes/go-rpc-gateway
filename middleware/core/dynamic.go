/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 19:06:50
 * @FilePath: \go-rpc-gateway\middleware\core\dynamic.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package core

import (
	"net/http"

	gcmiddleware "github.com/kamalyes/go-rpc-gateway/config/middleware"
	gwerrors "github.com/kamalyes/go-rpc-gateway/errors"
)

// SignatureValidator 签名验证器接口
// [EN] SignatureValidator validates the signature carried by a request
// 接口定义在 core 是因为 ResolvedSignature 需要引用它，具体实现（HMAC/RSA）在 security 子包
// [EN] The interface lives in core because ResolvedSignature refers to it; the HMAC/RSA implementations live in the security subpackage
type SignatureValidator interface {
	Validate(r *http.Request, config *gcmiddleware.Signature) error
}

// ResolvedSignature 表示按请求动态解析后的签名配置
type ResolvedSignature struct {
	Config    *gcmiddleware.Signature
	Validator SignatureValidator
	Skip      bool
}

// DynamicSignatureProvider 动态签名提供器
type DynamicSignatureProvider interface {
	ResolveSignature(r *http.Request) (*ResolvedSignature, *gwerrors.AppError)
}

// RateLimitDecision 表示单次请求需要执行的一条限流决策
type RateLimitDecision struct {
	Rule     *gcmiddleware.LimitRule
	Key      string
	Strategy gcmiddleware.Strategy
}

// DynamicRateLimitResult 表示按请求动态解析后的限流结果
type DynamicRateLimitResult struct {
	Decisions []RateLimitDecision
	Skip      bool
}

// DynamicRateLimitProvider 动态限流提供器
type DynamicRateLimitProvider interface {
	ResolveRateLimit(r *http.Request) (*DynamicRateLimitResult, *gwerrors.AppError)
}
