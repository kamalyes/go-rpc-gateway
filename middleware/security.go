/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 18:02:50
 * @FilePath: \go-rpc-gateway\middleware\security.go
 * @Description: 安全中间件 - 包含CORS, CSP, CSRF, IP白名单等
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kamalyes/go-config/pkg/cors"
	"github.com/kamalyes/go-config/pkg/security"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/response"
	"github.com/kamalyes/go-toolbox/pkg/netx"
	"github.com/kamalyes/go-toolbox/pkg/validator"
)

// CORSMiddleware CORS 中间件（使用默认配置）
func CORSMiddleware() HTTPMiddleware {
	config := cors.Default()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setCORSHeaders(w, r, config)

			// 处理预检请求
			if r.Method == constants.HTTPMethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// setCORSHeaders 设置CORS相关头部
func setCORSHeaders(w http.ResponseWriter, r *http.Request, config *cors.Cors) {
	setAllowOrigin(w, r.Header.Get("Origin"), config.AllowedOrigins)
	setAllowMethods(w, config.AllowedMethods)
	setAllowHeaders(w, config.AllowedHeaders)
	setAllowCredentials(w, config.AllowCredentials)
	setMaxAge(w, config.MaxAge)
}

// setAllowOrigin 设置允许的源
func setAllowOrigin(w http.ResponseWriter, origin string, allowOrigins []string) {
	if len(allowOrigins) == 0 {
		return
	}

	for _, allowedOrigin := range allowOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			w.Header().Set(constants.HeaderAccessControlAllowOrigin, origin)
			return
		}
	}
}

// setAllowMethods 设置允许的方法
func setAllowMethods(w http.ResponseWriter, methods []string) {
	if len(methods) == 0 {
		return
	}

	value := joinStrings(methods, ", ")
	w.Header().Set(constants.HeaderAccessControlAllowMethods, value)
}

// setAllowHeaders 设置允许的头部
func setAllowHeaders(w http.ResponseWriter, headers []string) {
	if len(headers) == 0 {
		return
	}

	value := joinStrings(headers, ", ")
	w.Header().Set(constants.HeaderAccessControlAllowHeaders, value)
}

// setAllowCredentials 设置是否允许凭证
func setAllowCredentials(w http.ResponseWriter, allowCredentials bool) {
	if allowCredentials {
		w.Header().Set(constants.HeaderAccessControlAllowCredentials, constants.CORSCredentialsTrue)
	}
}

// setMaxAge 设置预检请求缓存时间
func setMaxAge(w http.ResponseWriter, maxAge string) {
	if maxAge != "" {
		w.Header().Set(constants.HeaderAccessControlMaxAge, maxAge)
	}
}

// joinStrings 连接字符串数组
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}

	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// SecurityMiddleware 安全中间件 - 从配置读取 CSP 策略
// 参数 cspConfig: 从 go-config/pkg/security 读取的 CSP 配置
func SecurityMiddleware(cspConfig *security.CSP) HTTPMiddleware {
	// 获取 CSP 策略字符串
	var cspPolicy = cspConfig.GetPolicy()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 设置基础安全头部
			w.Header().Set(constants.HeaderXContentTypeOptions, constants.SecurityHeaderNosniff)
			w.Header().Set(constants.HeaderXFrameOptions, constants.SecurityHeaderDeny)
			w.Header().Set(constants.HeaderXXSSProtection, constants.SecurityHeaderXSSBlock)
			w.Header().Set(constants.HeaderStrictTransportSecurity, constants.SecurityHeaderHSTS)
			w.Header().Set(constants.HeaderReferrerPolicy, constants.SecurityHeaderReferrerDefault)

			// 设置 CSP 策略（从配置读取）
			if cspPolicy != "" {
				w.Header().Set(constants.HeaderContentSecurityPolicy, cspPolicy)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CSRFProtectionMiddleware CSRF防护中间件
func CSRFProtectionMiddleware(enabled bool) HTTPMiddleware {
	tokens := make(map[string]time.Time)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled || isCSRFExemptMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			token := getCSRFToken(r)
			if !validateCSRFToken(token, tokens) {
				logCSRFValidationFailure(r)
				response.WriteAppError(w, errors.ErrCSRFTokenInvalid)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isCSRFExemptMethod 检查是否为 CSRF 豁免的 HTTP 方法
func isCSRFExemptMethod(method string) bool {
	return method == constants.HTTPMethodGet ||
		method == constants.HTTPMethodHead ||
		method == constants.HTTPMethodOptions
}

// getCSRFToken 从请求中获取 CSRF token
func getCSRFToken(r *http.Request) string {
	token := r.Header.Get(constants.HeaderXCSRFToken)
	if token == "" {
		token = r.FormValue(constants.CSRFTokenFormField)
	}
	return token
}

// logCSRFValidationFailure 记录 CSRF 验证失败日志
func logCSRFValidationFailure(r *http.Request) {
	global.LOGGER.WarnKV(constants.LogMsgCSRFValidationFailed,
		constants.LogFieldMethod, r.Method,
		constants.LogFieldPath, r.URL.Path,
		constants.LogFieldRemoteAddr, netx.GetClientIP(r))
}

// IPWhitelistMiddleware IP白名单中间件
func IPWhitelistMiddleware(allowedIPs []string) HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allowedIPs) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := netx.GetClientIP(r)

			if !isIPAllowed(clientIP, allowedIPs) {
				if global.LOGGER != nil {
					global.LOGGER.WarnKV(constants.LogMsgIPAccessDenied,
						constants.LogFieldClientIP, clientIP,
						constants.LogFieldPath, r.URL.Path,
						constants.LogFieldUserAgent, r.Header.Get(constants.HeaderUserAgent))
				}

				response.WriteAppError(w, errors.ErrForbidden.WithDetails(constants.ErrMsgIPAccessDenied))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isIPAllowed 检查IP是否在允许列表中（支持CIDR格式、通配符、IPv6）
func isIPAllowed(clientIP string, allowedIPs []string) bool {
	return validator.IsIPAllowed(clientIP, allowedIPs)
}

// generateCSRFToken 生成CSRF token
func generateCSRFToken() string {
	timestamp := time.Now().Unix()
	data := fmt.Sprintf("%d:%s", timestamp, constants.CSRFSecret)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// validateCSRFToken 验证CSRF token
func validateCSRFToken(token string, tokens map[string]time.Time) bool {
	if token == "" {
		return false
	}

	// 检查token是否存在且未过期
	expiry, exists := tokens[token]
	if !exists {
		return false
	}

	if time.Now().After(expiry) {
		// 清理过期token
		delete(tokens, token)
		return false
	}

	return true
}

// CSRFTokenHandler 提供CSRF token的端点
func CSRFTokenHandler(w http.ResponseWriter, r *http.Request) {
	token := generateCSRFToken()
	response.WriteCSRFTokenResponse(w, token)
}

// PathProtectionMiddleware 路径级保护中间件 - 基于路径前缀的安全控制
// 用于 pprof, swagger, metrics 等敏感端点的统一保护
func PathProtectionMiddleware(pathPrefix string, cfg *security.ServiceProtection) HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !shouldProtectPath(r.URL.Path, pathPrefix, cfg) {
				next.ServeHTTP(w, r)
				return
			}

			if err := validatePathAccess(w, r, pathPrefix, cfg); err != nil {
				return
			}

			logAccessGranted(r, pathPrefix)
			next.ServeHTTP(w, r)
		})
	}
}

// shouldProtectPath 检查路径是否需要保护
func shouldProtectPath(path, pathPrefix string, cfg *security.ServiceProtection) bool {
	if !strings.HasPrefix(path, pathPrefix) {
		return false
	}
	return cfg != nil && cfg.Enabled
}

// validatePathAccess 验证路径访问权限（IP、认证、HTTPS）
func validatePathAccess(w http.ResponseWriter, r *http.Request, pathPrefix string, cfg *security.ServiceProtection) error {
	if err := checkIPWhitelist(r, pathPrefix, cfg.IPWhitelist); err != nil {
		http.Error(w, constants.ErrMsgIPNotAllowed, http.StatusForbidden)
		return err
	}

	if err := checkAuthentication(w, r, pathPrefix, cfg); err != nil {
		w.Header().Set(constants.HeaderWWWAuthenticate, fmt.Sprintf(`%s realm="%s"`, constants.AuthSchemeBasic, constants.AuthRealmProtected))
		http.Error(w, constants.ErrMsgUnauthorized, http.StatusUnauthorized)
		return err
	}

	if err := checkHTTPS(r, pathPrefix, cfg.RequireHTTPS); err != nil {
		http.Error(w, constants.ErrMsgHTTPSRequired, http.StatusUpgradeRequired)
		return err
	}

	return nil
}

// checkIPWhitelist 检查 IP 白名单
func checkIPWhitelist(r *http.Request, pathPrefix string, whitelist []string) error {
	if len(whitelist) == 0 {
		return nil
	}

	clientIP := netx.GetClientIP(r)
	if !isIPInWhitelist(clientIP, whitelist) {
		if global.LOGGER != nil {
			global.LOGGER.Warn(constants.LogMsgIPNotInWhitelist,
				constants.LogFieldPath, r.URL.Path,
				constants.LogFieldClientIP, clientIP,
				constants.LogFieldProtectionPath, pathPrefix)
		}
		return fmt.Errorf("IP not in whitelist")
	}
	return nil
}

// checkAuthentication 检查认证
func checkAuthentication(w http.ResponseWriter, r *http.Request, pathPrefix string, cfg *security.ServiceProtection) error {
	if !cfg.AuthRequired {
		return nil
	}

	if !checkPathAuthentication(r, cfg) {
		if global.LOGGER != nil {
			global.LOGGER.Warn(constants.LogMsgAuthFailed,
				constants.LogFieldPath, r.URL.Path,
				constants.LogFieldClientIP, netx.GetClientIP(r),
				constants.LogFieldProtectionPath, pathPrefix)
		}
		return fmt.Errorf("authentication failed")
	}
	return nil
}

// checkHTTPS 检查 HTTPS
func checkHTTPS(r *http.Request, pathPrefix string, requireHTTPS bool) error {
	if !requireHTTPS || r.TLS != nil {
		return nil
	}

	if global.LOGGER != nil {
		global.LOGGER.Warn(constants.LogMsgHTTPSRequired,
			constants.LogFieldPath, r.URL.Path,
			constants.LogFieldProtectionPath, pathPrefix)
	}
	return fmt.Errorf("HTTPS required")
}

// logAccessGranted 记录访问授权日志
func logAccessGranted(r *http.Request, pathPrefix string) {
	if global.LOGGER != nil {
		global.LOGGER.Debug(constants.LogMsgAccessGranted,
			constants.LogFieldPath, r.URL.Path,
			constants.LogFieldClientIP, netx.GetClientIP(r),
			constants.LogFieldProtectionPath, pathPrefix)
	}
}

// isIPInWhitelist 检查IP是否在白名单中(支持CIDR、通配符、IPv6)
func isIPInWhitelist(clientIP string, whitelist []string) bool {
	return validator.IsIPAllowed(clientIP, whitelist)
}

// checkPathAuthentication 检查路径认证
func checkPathAuthentication(r *http.Request, cfg *security.ServiceProtection) bool {
	authType := strings.ToLower(cfg.AuthType)

	switch authType {
	case constants.AuthTypeBearer:
		return checkBearerAuthentication(r, cfg.Username, cfg.Password)
	default:
		// 默认使用Basic认证
		return checkBasicAuthentication(r, cfg.Username, cfg.Password)
	}
}

// checkBasicAuthentication 检查Basic认证
func checkBasicAuthentication(r *http.Request, username, password string) bool {
	authUsername, authPassword, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return authUsername == username && authPassword == password
}

// checkBearerAuthentication 检查Bearer Token认证
func checkBearerAuthentication(r *http.Request, username, password string) bool {
	authHeader := r.Header.Get(constants.HeaderAuthorization)
	if authHeader == "" {
		return false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], constants.AuthSchemeBearer) {
		return false
	}

	// 简化版本: username:password的base64编码作为token
	expectedToken := base64.StdEncoding.EncodeToString(
		[]byte(fmt.Sprintf("%s:%s", username, password)),
	)

	return parts[1] == expectedToken
}
