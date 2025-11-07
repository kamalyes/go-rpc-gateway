/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 16:30:00
 * @FilePath: \go-rpc-gateway\middleware\recovery.go
 * @Description: Recovery恢复中间件
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"

	"github.com/kamalyes/go-core/pkg/response"
	"go.uber.org/zap"
)

// Recovery 恢复中间件，捕获panic并返回友好的错误响应
func Recovery(logger *zap.Logger) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// 获取堆栈信息
					buf := make([]byte, 2048)
					n := runtime.Stack(buf, false)
					stackTrace := string(buf[:n])

					logger.Error("请求恐慌恢复",
						zap.Any("panic", err),
						zap.String("path", r.URL.Path),
						zap.String("method", r.Method),
						zap.String("remote_addr", r.RemoteAddr),
						zap.String("stack_trace", stackTrace),
					)

					// 设置响应头
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)

					// 创建错误响应
					resp := map[string]interface{}{
						"code":    response.ServerError,
						"message": "服务器内部错误",
						"success": false,
					}

					if err := json.NewEncoder(w).Encode(resp); err != nil {
						logger.Error("写入panic响应失败", zap.Error(err))
					}
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// RecoveryWithConfig Recovery中间件配置版本
func RecoveryWithConfig(config RecoveryConfig) MiddlewareFunc {
	if config.Logger == nil {
		panic("Recovery中间件需要Logger配置")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// 获取堆栈信息
					var stackTrace string
					if config.EnableStack {
						buf := make([]byte, config.StackSize)
						n := runtime.Stack(buf, false)
						stackTrace = string(buf[:n])
					}

					// 记录日志
					fields := []zap.Field{
						zap.Any("panic", err),
						zap.String("path", r.URL.Path),
						zap.String("method", r.Method),
						zap.String("remote_addr", r.RemoteAddr),
					}

					if config.EnableStack {
						fields = append(fields, zap.String("stack_trace", stackTrace))
					}

					config.Logger.Error("请求恐慌恢复", fields...)

					// 自定义恢复处理
					if config.RecoveryHandler != nil {
						config.RecoveryHandler(w, r, err)
						return
					}

					// 默认处理
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)

					message := config.ErrorMessage
					if message == "" {
						message = "服务器内部错误"
					}

					resp := map[string]interface{}{
						"code":    response.ServerError,
						"message": message,
						"success": false,
					}

					if config.EnableDebug {
						resp["debug"] = fmt.Sprintf("%v", err)
						if config.EnableStack {
							resp["stack"] = stackTrace
						}
					}

					json.NewEncoder(w).Encode(resp)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// RecoveryConfig Recovery中间件配置
type RecoveryConfig struct {
	Logger          *zap.Logger
	EnableStack     bool
	StackSize       int
	EnableDebug     bool
	ErrorMessage    string
	RecoveryHandler func(http.ResponseWriter, *http.Request, interface{})
}

// DefaultRecoveryConfig 默认Recovery配置
func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		EnableStack:  true,
		StackSize:    4096,
		EnableDebug:  false,
		ErrorMessage: "服务器内部错误",
	}
}
