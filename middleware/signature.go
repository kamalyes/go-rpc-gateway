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

	"github.com/kamalyes/go-config/pkg/signature"
	"github.com/kamalyes/go-rpc-gateway/constants"
	gwerrors "github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/response"
	"github.com/kamalyes/go-toolbox/pkg/httpx"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/sign"
	"github.com/kamalyes/go-toolbox/pkg/validator"
)

// SignatureValidator 签名验证器接口
type SignatureValidator interface {
	Validate(r *http.Request, config *signature.Signature) error
}

// ===============================================================================
// HMAC 签名验证器
// ===============================================================================

// HMACValidator HMAC 签名验证器
type HMACValidator struct{}

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
	expectedSign, err := v.generateSignature(dataToSign, config.SecretKey, config.Algorithm)
	if err != nil {
		return fmt.Errorf("failed to generate signature: %w", err)
	}

	// 验证签名
	if expectedSign != requestCommonMeta.Signature {
		global.LOGGER.DebugContext(r.Context(), "🔐 HMAC 签名验证失败:")
		global.LOGGER.DebugContext(r.Context(), "  - 期望签名: %s", expectedSign)
		global.LOGGER.DebugContext(r.Context(), "  - 实际签名: %s", requestCommonMeta.Signature)
		return fmt.Errorf(constants.SignatureErrorMismatch)
	}

	return nil
}

// generateSignature 生成 HMAC 签名
func (v *HMACValidator) generateSignature(dataToSign, secretKey string, algorithm sign.HashCryptoFunc) (string, error) {
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
		global.LOGGER.DebugContext(r.Context(), "🔐 RSA 签名验证失败: %v", err)
		return fmt.Errorf(constants.SignatureErrorMismatch)
	}

	return nil
}

// verifySignature 验证 RSA-SHA256 签名
func (v *RSAValidator) verifySignature(dataToSign, signatureBase64 string) error {
	// Base64 解码签名
	signatureBytes, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// 计算签名数据的 SHA256 哈希
	hashed := sha256.Sum256([]byte(dataToSign))

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
func buildSigningData(req *RequestCommonMeta, queryString string, body []byte) string {
	var dataToSign string

	// 添加时间戳
	dataToSign += req.Timestamp

	// 添加查询字符串（直接使用原始字符串，保持参数顺序）
	if queryString != "" {
		dataToSign += queryString
	}

	// 添加请求体
	if body != nil {
		bodyStr := string(body)
		// 调试：打印签名参
		// 调试：打印签名参数
		global.LOGGER.Debug("🔐 后端签名参数:")
		global.LOGGER.Debug("  - Timestamp: %s", req.Timestamp)
		global.LOGGER.Debug("  - QueryString: %s", queryString)
		global.LOGGER.Debug("  - Body length: %d bytes, %d chars", len(body), len(bodyStr))
		global.LOGGER.Debug("  - Body 完整内容: %s", body)
		global.LOGGER.Debug("  - 客户端签名: %s", req.Signature)
		dataToSign += bodyStr
	}

	return dataToSign
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
		return &HMACValidator{}, nil
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
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	validator, appErr := buildSignatureValidator(config)
	if appErr != nil {
		global.LOGGER.Error("❌ 创建签名验证器失败: %v", appErr)
		return func(next http.Handler) http.Handler {
			return next
		}
	}
	if validator == nil {
		return func(next http.Handler) http.Handler {
			return next
		}
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

// SignatureMiddlewareWithProvider 签名验证中间件（按请求动态解析配置/验证器）
func SignatureMiddlewareWithProvider(config *signature.Signature, provider DynamicSignatureProvider) HTTPMiddleware {
	if !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resolvedConfig := config
			var validator SignatureValidator

			if provider != nil {
				resolved, appErr := provider.ResolveSignature(r)
				if appErr != nil {
					response.WriteAppError(w, appErr)
					return
				}
				if resolved != nil {
					if resolved.Skip {
						next.ServeHTTP(w, r)
						return
					}
					if resolved.Config != nil {
						resolvedConfig = resolved.Config
					}
					validator = resolved.Validator
				}
			}

			if resolvedConfig == nil || !resolvedConfig.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			if validator == nil {
				var appErr *gwerrors.AppError
				validator, appErr = buildSignatureValidator(resolvedConfig)
				if appErr != nil {
					response.WriteAppError(w, appErr)
					return
				}
				if validator == nil {
					next.ServeHTTP(w, r)
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
