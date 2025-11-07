/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 18:06:52
 * @FilePath: \go-rpc-gateway\middleware\requestid.go
 * @Description: 请求ID中间件
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"net/http"
)

// RequestID 请求ID中间件
func RequestID() MiddlewareFunc {
	return RequestIDWithConfig(DefaultRequestIDConfig())
}

// RequestIDWithConfig 带配置的请求ID中间件
func RequestIDWithConfig(config RequestIDConfig) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get(config.Header)
			if requestID == "" {
				requestID = config.Generator()
			}

			// 设置响应头
			w.Header().Set(config.Header, requestID)

			// 添加到context中
			ctx := context.WithValue(r.Context(), "request_id", requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
