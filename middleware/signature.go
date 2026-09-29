/**
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 15:58:58
 * @FilePath: \go-rpc-gateway\middleware\signature.go
 * @Description: 签名验证中间件（支持 HMAC 和 RSA）
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"

	validator "github.com/kamalyes/go-argus"
	"github.com/kamalyes/go-config/pkg/signature"
	"github.com/kamalyes/go-rpc-gateway/constants"
	gwerrors "github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/response"
	"github.com/kamalyes/go-toolbox/pkg/httpx"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/sign"
)

// SignatureValidator 签名验证器接口
type SignatureValidator interface {
	Validate(r *http.Request, config *signature.Signature) error
}

// ===============================================================================
// HMAC 签名验证器
// ===============================================================================

// hmacSignerCache 按算法缓存的 HMAC 签名器（进程内构建一次全局复用）
// 签名器无状态，Sign 每次调用内部新建密钥哈希，可安全并发复用
var hmacSignerCache = buildHMACSignerCache()

// buildHMACSignerCache 为所有支持的哈希算法预构建签名器
func buildHMACSignerCache() map[sign.HashCryptoFunc]sign.Signer {
	cache := make(map[sign.HashCryptoFunc]sign.Signer, len(sign.SupportHMACCryptoFunc))
	for algorithm := range sign.SupportHMACCryptoFunc {
		if signer, err := sign.NewHMACSigner(algorithm); err == nil {
			cache[algorithm] = signer
		}
	}
	return cache
}

// HMACValidator HMAC 签名验证器
type HMACValidator struct {
	signer sign.Signer // 构建时从缓存获取，避免每请求重复创建
}

// NewHMACValidator 创建 HMAC 签名验证器（复用全局缓存的签名器实例）
func NewHMACValidator(algorithm sign.HashCryptoFunc) (*HMACValidator, error) {
	if signer, ok := hmacSignerCache[algorithm]; ok {
		return &HMACValidator{signer: signer}, nil
	}
	return nil, fmt.Errorf("unsupported HMAC algorithm: %s", algorithm)
}

// Validate 验证 HMAC 签名
func (v *HMACValidator) Validate(r *http.Request, config *signature.Signature) error {
	if !config.Enabled {
		return nil
	}

	// 检查是否在忽略路径中
	if validator.MatchPathInList(r.URL.Path, config.IgnorePaths) {
		return nil
	}

	requestCommonMeta := GetRequestCommonMeta(r.Context())

	// 读取请求体
	body, err := readRequestBody(r, config.SkipBody)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	// 获取查询字符串
	queryString := getQueryString(r, config.SkipQuery)

	// 构建签名数据
	dataToSign := buildSigningData(requestCommonMeta, queryString, body)

	// 生成期望的签名
	expectedSign, err := v.generateSignature(dataToSign, config.SecretKey)
	if err != nil {
		return fmt.Errorf("failed to generate signature: %w", err)
	}

	// 验证签名
	if expectedSign != requestCommonMeta.Signature {
		global.LOGGER.ErrorLines(
			"🔐 HMAC 签名验证失败",
			fmt.Sprintf("  - Timestamp: %s", requestCommonMeta.Timestamp),
			fmt.Sprintf("  - QueryString: %s", queryString),
			fmt.Sprintf("  - Body: %s", string(body)),
			fmt.Sprintf("  - 期望签名: %s", expectedSign),
			fmt.Sprintf("  - 实际签名: %s", requestCommonMeta.Signature),
		)
		return fmt.Errorf(constants.SignatureErrorMismatch)
	}

	return nil
}

// generateSignature 生成 HMAC 签名（复用构建期缓存的签名器，仅密钥哈希每次新建）
func (v *HMACValidator) generateSignature(dataToSign []byte, secretKey string) (string, error) {
	signatureBytes, err := v.signer.Sign(dataToSign, []byte(secretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign data: %w", err)
	}

	// 返回 Base64 编码的签名
	return base64.StdEncoding.EncodeToString(signatureBytes), nil
}

// ===============================================================================
// RSA 签名验证器
// ===============================================================================

// RSAValidator RSA 签名验证器
type RSAValidator struct {
	publicKey *rsa.PublicKey
}

// NewRSAValidator 创建 RSA 签名验证器
func NewRSAValidator(publicKeyPEM []byte) (*RSAValidator, error) {
	publicKey, err := sign.ParsePublicKey(publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA public key: %w", err)
	}
	return &RSAValidator{publicKey: publicKey}, nil
}

// Validate 验证 RSA 签名
func (v *RSAValidator) Validate(r *http.Request, config *signature.Signature) error {
	if !config.Enabled {
		return nil
	}

	// 检查是否在忽略路径中
	if validator.MatchPathInList(r.URL.Path, config.IgnorePaths) {
		return nil
	}

	requestCommonMeta := GetRequestCommonMeta(r.Context())

	// 读取请求体
	body, err := readRequestBody(r, config.SkipBody)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	// 获取查询字符串
	queryString := getQueryString(r, config.SkipQuery)

	// 构建签名数据
	dataToSign := buildSigningData(requestCommonMeta, queryString, body)

	// 验证 RSA 签名
	if err := v.verifySignature(dataToSign, requestCommonMeta.Signature); err != nil {
		global.LOGGER.ErrorLines(
			"🔐 RSA 签名验证失败",
			fmt.Sprintf("  - Timestamp: %s", requestCommonMeta.Timestamp),
			fmt.Sprintf("  - QueryString: %s", queryString),
			fmt.Sprintf("  - Body: %s", string(body)),
			fmt.Sprintf("  - 错误: %v", err),
		)
		return fmt.Errorf(constants.SignatureErrorMismatch)
	}

	return nil
}

// verifySignature 验证 RSA-SHA256 签名
func (v *RSAValidator) verifySignature(dataToSign []byte, signatureBase64 string) error {
	// Base64 解码签名
	signatureBytes, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// 计算签名数据的 SHA256 哈希
	hashed := sha256.Sum256(dataToSign)

	// 使用公钥验证签名（PKCS1v15）
	err = rsa.VerifyPKCS1v15(v.publicKey, crypto.SHA256, hashed[:], signatureBytes)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}

// ===============================================================================
// 辅助函数
// ===============================================================================

// buildSigningData 构建签名数据（HMAC 和 RSA 使用相同的逻辑）
// 签名数据格式：timestamp + queryString + body
// 返回 []byte 直接供哈希计算使用，避免 string→[]byte 的整份 body 二次拷贝
func buildSigningData(req *RequestCommonMeta, queryString string, body []byte) []byte {
	// 预估容量：timestamp + queryString + body（append 空 slice 本身是 no-op，无需分支）
	buf := make([]byte, 0, len(req.Timestamp)+len(queryString)+len(body))
	buf = append(buf, req.Timestamp...)
	buf = append(buf, queryString...)
	return append(buf, body...)
}

// readRequestBody 读取请求体
func readRequestBody(r *http.Request, skipBody bool) ([]byte, error) {
	if skipBody {
		return nil, nil
	}
	return httpx.ReadRequestBody(r)
}

// getQueryString 获取查询字符串
func getQueryString(r *http.Request, skipQuery bool) string {
	return mathx.IF(skipQuery, "", r.URL.RawQuery)
}

func buildSignatureValidator(config *signature.Signature) (SignatureValidator, *gwerrors.AppError) {
	if config == nil || !config.Enabled {
		return nil, nil
	}

	switch config.Type {
	case signature.SignatureTypeRSA:
		if config.PublicKeyPEM == "" {
			global.LOGGER.Warn("⚠️  RSA 签名已启用但未配置公钥，将跳过签名验证")
			return nil, nil
		}
		validator, err := NewRSAValidator([]byte(config.PublicKeyPEM))
		if err != nil {
			return nil, gwerrors.NewError(gwerrors.ErrCodeInternalServerError, fmt.Sprintf("failed to create RSA validator: %v", err))
		}
		return validator, nil
	case signature.SignatureTypeHMAC:
		fallthrough
	default:
		validator, err := NewHMACValidator(config.Algorithm)
		if err != nil {
			return nil, gwerrors.NewError(gwerrors.ErrCodeInternalServerError, fmt.Sprintf("failed to create HMAC validator: %v", err))
		}
		return validator, nil
	}
}

// ===============================================================================
// 签名验证中间件
// ===============================================================================

// SignatureMiddleware 签名验证中间件（自动根据配置选择验证器）
//
// 支持两种签名方式：
// 1. HMAC：使用 SecretKey，适用于服务端之间的通信
// 2. RSA：使用公钥验证，适用于开放平台 API
//
// 使用示例：
//
//	自动选择验证器（根据配置）
//	middleware.SignatureMiddleware(config)
func SignatureMiddleware(config *signature.Signature) HTTPMiddleware {
	if !config.Enabled {
		return noopMiddleware
	}
	return SignatureMiddlewareWithProvider(config, nil)
}

// SignatureMiddlewareWithProvider 签名验证中间件（按请求动态解析配置/验证器）
// 静态配置在构建期解析一次验证器并缓存，动态配置仍按请求解析
func SignatureMiddlewareWithProvider(config *signature.Signature, provider DynamicSignatureProvider) HTTPMiddleware {
	if !config.Enabled {
		return noopMiddleware
	}

	// 构建期完成静态验证器解析（HMAC 签名器缓存 / RSA 公钥解析只做一次）
	staticValidator, staticErr := buildSignatureValidator(config)

	// resolveValidator 解析请求对应的签名验证器
	// 静态配置直接复用构建期结果，动态配置按请求构建
	resolveValidator := func(resolvedConfig *signature.Signature) (SignatureValidator, *gwerrors.AppError) {
		if resolvedConfig == config {
			return staticValidator, staticErr
		}
		return buildSignatureValidator(resolvedConfig)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resolvedConfig, validator, skip, appErr := resolveDynamic(provider, config, staticValidator, r)
			if appErr != nil {
				response.WriteAppError(w, appErr)
				return
			}
			if skip {
				next.ServeHTTP(w, r)
				return
			}
			if resolvedConfig == nil || !resolvedConfig.Enabled {
				next.ServeHTTP(w, r)
				return
			}
			if validator == nil {
				if validator, appErr = resolveValidator(resolvedConfig); appErr != nil {
					response.WriteAppError(w, appErr)
					return
				}
				if validator == nil {
					next.ServeHTTP(w, r) // 无需验证（如 RSA 未配置公钥）
					return
				}
			}

			if err := validator.Validate(r, resolvedConfig); err != nil {
				response.WriteErrorResponseWithCode(w, http.StatusForbidden, constants.SignatureErrorCodeInvalid, err.Error())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// resolveDynamic 解析动态签名结果
// 无 provider 或 provider 返回空结果时，沿用构建期的静态配置与验证器
func resolveDynamic(provider DynamicSignatureProvider, config *signature.Signature, staticValidator SignatureValidator, r *http.Request) (resolvedConfig *signature.Signature, validator SignatureValidator, skip bool, appErr *gwerrors.AppError) {
	resolvedConfig, validator = config, staticValidator
	if provider == nil {
		return
	}

	resolved, appErr := provider.ResolveSignature(r)
	if appErr != nil || resolved == nil {
		return
	}

	skip = resolved.Skip
	if resolved.Config != nil {
		resolvedConfig = resolved.Config
	}
	if resolved.Validator != nil {
		validator = resolved.Validator
	}
	return
}

// SignatureMiddlewareWithValidator 签名验证中间件（使用自定义验证器）
//
// 适用于需要动态获取公钥的场景（如开放平台根据 AccessKey 查询公钥）
//
// 使用示例：
//
//	 使用自定义 RSA 验证器
//		rsaValidator, _ := middleware.NewRSAValidator([]byte(publicKeyPEM))
//		middleware.SignatureMiddlewareWithValidator(config, rsaValidator)
func SignatureMiddlewareWithValidator(config *signature.Signature, validator SignatureValidator) HTTPMiddleware {
	return SignatureMiddlewareWithProvider(config, staticSignatureProvider{validator: validator})
}

type staticSignatureProvider struct {
	validator SignatureValidator
}

// ResolveSignature 返回静态签名配置对应的解析结果。
func (p staticSignatureProvider) ResolveSignature(_ *http.Request) (*ResolvedSignature, *gwerrors.AppError) {
	if p.validator == nil {
		global.LOGGER.Warn("⚠️  签名验证已启用但未提供验证器，将跳过签名验证")
		return &ResolvedSignature{Skip: true}, nil
	}
	return &ResolvedSignature{Validator: p.validator}, nil
}
