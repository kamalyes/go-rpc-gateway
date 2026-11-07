/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 18:02:50
 * @FilePath: \go-rpc-gateway\middleware\security\security.go
 * @Description: 安全中间件 - 包含CORS, CSP, CSRF, IP白名单等
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package security

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	gcmiddleware "github.com/kamalyes/go-rpc-gateway/config/middleware"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware/core"
	"github.com/kamalyes/go-rpc-gateway/response"
	validator "github.com/kamalyes/go-toolbox/pkg/argus"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/netx"
)

// CORSMiddleware CORS 中间件
// methods/headers 的逗号拼接与 origin 集合在构建期预计算，热路径零拼接零分配
func CORSMiddleware(corsConfig *gcmiddleware.Cors) core.HTTPMiddleware {
	mergedHeaders := mathx.SliceUnion(gcmiddleware.DefaultCors().AllowedHeaders, corsConfig.AllowedHeaders)

	// 预计算允许方法/头部的逗号分隔字符串（避免每请求 strings.Join）
	allowMethodsValue := strings.Join(corsConfig.AllowedMethods, ", ")
	allowHeadersValue := strings.Join(mergedHeaders, ", ")

	// 预构建 origin 集合：通配符单独标记，其余 O(1) map 查找
	allowAllOrigins := false
	originSet := make(map[string]struct{}, len(corsConfig.AllowedOrigins))
	for _, origin := range corsConfig.AllowedOrigins {
		if origin == "*" {
			allowAllOrigins = true
			continue
		}
		originSet[origin] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			isPreflight := r.Method == constants.HTTPMethodOptions

			// 不带 Origin 的请求不是 CORS 请求，一个响应头都不必写
			//
			// 改造前是无条件写 3 到 4 个头，而浏览器对非 CORS 响应里的这些头全部忽略
			// 服务间调用与同源请求都不带 Origin，在多数部署里这是绝大多数流量
			// 每个 header.Set 要过一遍 CanonicalMIMEHeaderKey、插一次 map、再分配一个
			// []string，实测这一段占 CORS 中间件 263 ns 开销与 4 次分配的全部
			//
			// [EN] A request without Origin is not a CORS request and needs no response header
			// This used to write three or four unconditionally, all of which a browser ignores
			// on a non-CORS response, while service-to-service and same-origin traffic carries
			// no Origin and is the bulk of requests in most deployments
			// Each header.Set canonicalises the key, inserts into the map and allocates a
			// []string, which measured as the whole of this middleware's 263 ns and 4 allocations
			origin := r.Header.Get(constants.HeaderOrigin)
			if origin == "" {
				if isPreflight {
					// 保持原有行为：任何 OPTIONS 都答 204，哪怕不是 CORS 预检
					// [EN] Unchanged behaviour: any OPTIONS answers 204, CORS preflight or not
					w.WriteHeader(http.StatusNoContent)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			header := w.Header()

			// Vary: Origin 是回显 Origin 的必要条件
			//
			// Allow-Origin 的取值依赖请求的 Origin，不声明 Vary 的话共享缓存（CDN、
			// 反向代理）会把带 A 源的响应缓存下来发给 B 源，B 源于是读到为 A 源签发的
			// Allow-Origin allowAllOrigins 模式下回显的是任意 origin，这条尤其要紧
			// 改造前全仓库没有任何地方设 Vary
			//
			// [EN] Vary: Origin is required whenever Origin is echoed back
			// Allow-Origin depends on the request's Origin, and without Vary a shared cache
			// can serve a response minted for one origin to another, which matters most in
			// the allow-all mode where an arbitrary origin is echoed
			// Nothing in this repository set Vary before
			header.Add(constants.HeaderVary, constants.HeaderOrigin)

			allowed := setAllowOrigin(header, origin, allowAllOrigins, originSet)

			if corsConfig.AllowCredentials && allowed {
				header.Set(constants.HeaderAccessControlAllowCredentials, constants.CORSCredentialsTrue)
			}

			// Allow-Methods、Allow-Headers 与 Max-Age 只在预检响应里被读取
			// 真实请求的响应里带着它们对浏览器没有任何作用
			//
			// [EN] Allow-Methods, Allow-Headers and Max-Age are only read on a preflight
			// response and do nothing on an actual request's response
			if isPreflight {
				if allowMethodsValue != "" {
					header.Set(constants.HeaderAccessControlAllowMethods, allowMethodsValue)
				}
				if allowHeadersValue != "" {
					header.Set(constants.HeaderAccessControlAllowHeaders, allowHeadersValue)
				}
				if corsConfig.MaxAge != "" {
					header.Set(constants.HeaderAccessControlMaxAge, corsConfig.MaxAge)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// setAllowOrigin 设置允许的源（构建期已解析 origin 集合，热路径仅做 O(1) 查找）
// 返回该 origin 是否被放行，调用方据此决定要不要发 Allow-Credentials ——
// 没放行却发 Allow-Credentials 是自相矛盾的响应
//
// [EN] setAllowOrigin reports whether the origin was allowed, so the caller can decide
// whether to send Allow-Credentials: sending it for a disallowed origin is contradictory
func setAllowOrigin(header http.Header, origin string, allowAllOrigins bool, originSet map[string]struct{}) bool {
	if origin == "" {
		return false
	}
	if allowAllOrigins {
		header.Set(constants.HeaderAccessControlAllowOrigin, origin)
		return true
	}
	if _, ok := originSet[origin]; ok {
		header.Set(constants.HeaderAccessControlAllowOrigin, origin)
		return true
	}
	return false
}

// SCPMiddleware 安全中间件 - 从配置读取 CSP 策略
// 参数 cspConfig: 从 go-config/pkg/security 读取的 CSP 配置
func SCPMiddleware(cspConfig *gcmiddleware.CSP) core.HTTPMiddleware {
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
func CSRFProtectionMiddleware(enabled bool) core.HTTPMiddleware {
	tokens := make(map[string]time.Time)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled || constants.CSRFExemptMethods[r.Method] {
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
func IPWhitelistMiddleware(allowedIPs []string) core.HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allowedIPs) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := netx.GetClientIP(r)

			if !isIPAllowed(clientIP, allowedIPs) {
				global.LOGGER.WarnKV(constants.LogMsgIPAccessDenied,
					constants.LogFieldClientIP, clientIP,
					constants.LogFieldPath, r.URL.Path,
					constants.LogFieldUserAgent, r.Header.Get(constants.HeaderUserAgent))

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
func PathProtectionMiddleware(pathPrefix string, cfg *gcmiddleware.ServiceProtection) core.HTTPMiddleware {
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
func shouldProtectPath(path, pathPrefix string, cfg *gcmiddleware.ServiceProtection) bool {
	if !strings.HasPrefix(path, pathPrefix) {
		return false
	}
	return cfg != nil && cfg.Enabled
}

// validatePathAccess 验证路径访问权限（IP、认证、HTTPS）
func validatePathAccess(w http.ResponseWriter, r *http.Request, pathPrefix string, cfg *gcmiddleware.ServiceProtection) error {
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
		global.LOGGER.Warn(constants.LogMsgIPNotInWhitelist,
			constants.LogFieldPath, r.URL.Path,
			constants.LogFieldClientIP, clientIP,
			constants.LogFieldProtectionPath, pathPrefix)
		return fmt.Errorf("IP not in whitelist")
	}
	return nil
}

// checkAuthentication 检查认证
func checkAuthentication(w http.ResponseWriter, r *http.Request, pathPrefix string, cfg *gcmiddleware.ServiceProtection) error {
	if !cfg.AuthRequired {
		return nil
	}

	if !checkPathAuthentication(r, cfg) {
		global.LOGGER.Warn(constants.LogMsgAuthFailed,
			constants.LogFieldPath, r.URL.Path,
			constants.LogFieldClientIP, netx.GetClientIP(r),
			constants.LogFieldProtectionPath, pathPrefix)
		return fmt.Errorf("authentication failed")
	}
	return nil
}

// checkHTTPS 检查 HTTPS
func checkHTTPS(r *http.Request, pathPrefix string, requireHTTPS bool) error {
	if !requireHTTPS || r.TLS != nil {
		return nil
	}

	global.LOGGER.Warn(constants.LogMsgHTTPSRequired,
		constants.LogFieldPath, r.URL.Path,
		constants.LogFieldProtectionPath, pathPrefix)
	return fmt.Errorf("HTTPS required")
}

// logAccessGranted 记录访问授权日志
func logAccessGranted(r *http.Request, pathPrefix string) {
	global.LOGGER.Debug(constants.LogMsgAccessGranted,
		constants.LogFieldPath, r.URL.Path,
		constants.LogFieldClientIP, netx.GetClientIP(r),
		constants.LogFieldProtectionPath, pathPrefix)
}

// isIPInWhitelist 检查IP是否在白名单中(支持CIDR、通配符、IPv6)
func isIPInWhitelist(clientIP string, whitelist []string) bool {
	return validator.IsIPAllowed(clientIP, whitelist)
}

// checkPathAuthentication 检查路径认证
func checkPathAuthentication(r *http.Request, cfg *gcmiddleware.ServiceProtection) bool {
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
