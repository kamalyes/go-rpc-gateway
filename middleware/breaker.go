/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 15:15:05
 * @FilePath: \go-rpc-gateway\middleware\breaker.go
 * @Description: CircuitBreaker 中间件 - 直接使用 go-config 配置
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"net/http"

	gobreaker "github.com/kamalyes/go-config/pkg/breaker"
	"github.com/kamalyes/go-rpc-gateway/breaker"
)

// BreakerMiddleware 创建熔断中间件
func BreakerMiddleware(config *gobreaker.CircuitBreaker) func(http.Handler) http.Handler {
	manager := breaker.NewManager(
		config.FailureThreshold,
		config.SuccessThreshold,
		config.VolumeThreshold,
		config.Timeout,
		config.PreventionPaths,
		config.ExcludePaths,
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 如果禁用，直接通过
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// 检查路径是否需要保护
			if !manager.IsPathProtected(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// 获取断路器
			breaker := manager.GetBreaker(r.URL.Path)
			if !breaker.Allow() {
				http.Error(w, "Service Unavailable - Circuit Breaker Open", http.StatusServiceUnavailable)
				return
			}

			// 创建响应包装器以捕获状态码
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// 执行请求
			next.ServeHTTP(rw, r)

			// 根据响应状态码记录成功或失败
			if rw.statusCode >= 500 {
				breaker.RecordFailure()
			} else {
				breaker.RecordSuccess()
			}
		})
	}
}

// responseWriter 响应包装器，用于捕获状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
