/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-08 03:36:25
 * @FilePath: \go-rpc-gateway\middleware\security.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"net/http"

	"github.com/kamalyes/go-config/pkg/cors"
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

// SecurityMiddleware 安全中间件
func SecurityMiddleware() HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 设置安全头部
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
