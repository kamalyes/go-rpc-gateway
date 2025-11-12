/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 14:38:32
 * @FilePath: \go-rpc-gateway\middleware\security.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kamalyes/go-config/pkg/cors"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	commonapis "github.com/kamalyes/go-rpc-gateway/proto"
	"github.com/kamalyes/go-rpc-gateway/response"
)

// CORSMiddleware CORS 中间件
func CORSMiddleware() HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

			if r.Method == "OPTIONS" {
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSConfig CORS 配置
type CORSConfig struct {
	AllowOrigins     []string `json:"allowOrigins" yaml:"allowOrigins"`
	AllowMethods     []string `json:"allowMethods" yaml:"allowMethods"`
	AllowHeaders     []string `json:"allowHeaders" yaml:"allowHeaders"`
	AllowCredentials bool     `json:"allowCredentials" yaml:"allowCredentials"`
	MaxAge           int      `json:"maxAge" yaml:"maxAge"`
}

// DefaultCORSConfig 默认 CORS 配置
func DefaultCORSConfig() *cors.Cors {
	return &cors.Cors{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders: []string{
			"Accept",
			"Content-Type",
			"Content-Length",
			"Accept-Encoding",
			"X-CSRF-Token",
			"Authorization",
			"X-Request-Id",
			"X-Trace-Id",
		},
		AllowCredentials: false,
		MaxAge:           "86400", // 24小时
	}
}

// CORSMiddlewareWithConfig 带配置的 CORS 中间件
func CORSMiddlewareWithConfig(config *cors.Cors) HTTPMiddleware {
	if config == nil {
		config = DefaultCORSConfig()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setCORSHeaders(w, r, config)

			// 处理预检请求
			if r.Method == "OPTIONS" {
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
			w.Header().Set("Access-Control-Allow-Origin", origin)
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
	w.Header().Set("Access-Control-Allow-Methods", value)
}

// setAllowHeaders 设置允许的头部
func setAllowHeaders(w http.ResponseWriter, headers []string) {
	if len(headers) == 0 {
		return
	}

	value := joinStrings(headers, ", ")
	w.Header().Set("Access-Control-Allow-Headers", value)
}

// setAllowCredentials 设置是否允许凭证
func setAllowCredentials(w http.ResponseWriter, allowCredentials bool) {
	if allowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
}

// setMaxAge 设置预检请求缓存时间
func setMaxAge(w http.ResponseWriter, maxAge string) {
	if maxAge != "" {
		w.Header().Set("Access-Control-Max-Age", maxAge)
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

// SecurityMiddleware 安全中间件 - 使用默认配置
func SecurityMiddleware() HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 设置基础安全头部
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			next.ServeHTTP(w, r)
		})
	}
}

// CSRFProtectionMiddleware CSRF防护中间件
func CSRFProtectionMiddleware(enabled bool) HTTPMiddleware {
	tokens := make(map[string]time.Time)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				next.ServeHTTP(w, r)
				return
			}

			// 只对状态改变的请求进行CSRF检查
			if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
				next.ServeHTTP(w, r)
				return
			}

			// 检查CSRF token
			token := r.Header.Get("X-CSRF-Token")
			if token == "" {
				token = r.FormValue("_csrf_token")
			}

			if !validateCSRFToken(token, tokens) {
				if global.LOGGER != nil {
					global.LOGGER.WarnKV("CSRF token验证失败",
						"method", r.Method,
						"path", r.URL.Path,
						"remote_addr", getClientIP(r))
				}

				response.WriteAppError(w, errors.ErrCSRFTokenInvalid)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// IPWhitelistMiddleware IP白名单中间件
func IPWhitelistMiddleware(allowedIPs []string) HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(allowedIPs) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := getClientIP(r)

			if !isIPAllowed(clientIP, allowedIPs) {
				if global.LOGGER != nil {
					global.LOGGER.WarnKV("IP访问被拒绝",
						"client_ip", clientIP,
						"path", r.URL.Path,
						"user_agent", r.Header.Get(constants.HeaderUserAgent))
				}

				response.WriteAppError(w, errors.ErrForbidden.WithDetails("IP access denied"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isIPAllowed 检查IP是否在允许列表中
func isIPAllowed(clientIP string, allowedIPs []string) bool {
	for _, allowedIP := range allowedIPs {
		if clientIP == allowedIP || allowedIP == "*" {
			return true
		}
		// TODO: 支持CIDR格式的IP范围
	}
	return false
}

// generateCSRFToken 生成CSRF token
func generateCSRFToken() string {
	timestamp := time.Now().Unix()
	data := fmt.Sprintf("%d:%s", timestamp, "csrf-secret")

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

// writeSecurityError 写入安全相关错误响应
func writeSecurityError(w http.ResponseWriter, httpStatus int, statusCode commonapis.StatusCode, message string) {
	result := &commonapis.Result{
		Code:   int32(httpStatus),
		Error:  message,
		Status: statusCode,
	}

	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
	w.WriteHeader(httpStatus)

	if err := json.NewEncoder(w).Encode(result); err != nil && global.LOGGER != nil {
		global.LOGGER.WithError(err).ErrorMsg("Failed to encode security error response")
	}
}

// CSRFTokenHandler 提供CSRF token的端点
func CSRFTokenHandler(w http.ResponseWriter, r *http.Request) {
	token := generateCSRFToken()

	w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
	w.WriteHeader(http.StatusOK)

	response := fmt.Sprintf(`{"csrf_token": "%s"}`, token)
	w.Write([]byte(response))
}
