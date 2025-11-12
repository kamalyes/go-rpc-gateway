/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 09:25:30
 * @FilePath: \engine-im-push-service\go-rpc-gateway\breaker\middleware.go
 * @Description: HTTP 中间件 - 为 HTTP 处理器提供断路器保护
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package breaker

import (
	"encoding/json"
	"net/http"
)

// HTTPMiddleware 创建 HTTP 中间件
func HTTPMiddleware(manager *Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 检查路径是否需要保护
			if !manager.IsPathProtected(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// 获取对应路径的断路器
			breaker := manager.GetBreaker(r.URL.Path)

			// 检查断路器状态
			if !breaker.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)

				resp := map[string]interface{}{
					"code":    503,
					"message": "Service temporarily unavailable (circuit breaker open)",
					"success": false,
				}

				_ = json.NewEncoder(w).Encode(resp)
				return
			}

			// 包装响应写入器以捕获状态码
			wrappedWriter := newResponseWriter(w)

			// 调用下一个处理器
			next.ServeHTTP(wrappedWriter, r)

			// 根据响应状态码记录成功或失败
			if wrappedWriter.statusCode >= 500 {
				breaker.RecordFailure()
			} else {
				breaker.RecordSuccess()
			}
		})
	}
}

// responseWriter 响应写入器包装器（来自 tracing.go）
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// newResponseWriter 创建响应写入器包装器
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

// WriteHeader 重写 WriteHeader 方法
func (rw *responseWriter) WriteHeader(statusCode int) {
	if !rw.written {
		rw.statusCode = statusCode
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Write 重写 Write 方法
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.statusCode = http.StatusOK
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}
