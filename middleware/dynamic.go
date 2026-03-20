/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 19:06:50
 * @FilePath: \go-rpc-gateway\middleware\dynamic.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"net/http"

	"github.com/kamalyes/go-config/pkg/ratelimit"
	"github.com/kamalyes/go-config/pkg/signature"
	gwerrors "github.com/kamalyes/go-rpc-gateway/errors"
)

// ResolvedSignature 表示按请求动态解析后的签名配置
type ResolvedSignature struct {
	Config    *signature.Signature
	Validator SignatureValidator
	Skip      bool
}

// DynamicSignatureProvider 动态签名提供器
type DynamicSignatureProvider interface {
	ResolveSignature(r *http.Request) (*ResolvedSignature, *gwerrors.AppError)
}

// RateLimitDecision 表示单次请求需要执行的一条限流决策
type RateLimitDecision struct {
	Rule     *ratelimit.LimitRule
	Key      string
	Strategy ratelimit.Strategy
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
