/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 13:43:33
 * @FilePath: \go-rpc-gateway\middleware\recovery.go
 * @Description: Recovery恢复中间件，使用go-logger
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"

	"github.com/kamalyes/go-rpc-gateway/global"
	commonapis "github.com/kamalyes/go-rpc-gateway/proto"
)

// Recovery 恢复中间件，捕获panic并返回友好的错误响应
func Recovery() MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// 获取堆栈信息
					buf := make([]byte, 2048)
					n := runtime.Stack(buf, false)
					stackTrace := string(buf[:n])

					// 使用全局logger记录错误
					global.LOGGER.ErrorKV("请求恐慌恢复",
						"error", err,
						"path", r.URL.Path,
						"method", r.Method,
						"remote_addr", r.RemoteAddr,
						"stack_trace", string(stackTrace))

					// 设置响应头
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)

					// 创建标准化错误响应
					result := &commonapis.Result{
						Code:   int32(http.StatusInternalServerError),
						Error:  "服务器内部错误",
						Status: commonapis.StatusCode_Internal,
					}

					if err := json.NewEncoder(w).Encode(result); err != nil && global.LOGGER != nil {
						global.LOGGER.WithError(err).ErrorMsg("写入panic响应失败")
					}
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// RecoveryWithConfig Recovery中间件配置版本
func RecoveryWithConfig(config RecoveryConfig) MiddlewareFunc {
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
					logger := global.LOGGER.WithField("error", err).
						WithField("path", r.URL.Path).
						WithField("method", r.Method).
						WithField("remote_addr", r.RemoteAddr)
					if config.EnableStack {
						logger = logger.WithField("stack_trace", stackTrace)
					}
					logger.ErrorMsg("请求恐慌恢复")

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

					result := &commonapis.Result{
						Code:   int32(http.StatusInternalServerError),
						Error:  message,
						Status: commonapis.StatusCode_Internal,
					}

					if config.EnableDebug {
						// 对于调试信息，我们可以将其添加到错误消息中
						debugInfo := fmt.Sprintf("%v", err)
						if config.EnableStack {
							debugInfo += fmt.Sprintf(" | Stack: %s", stackTrace)
						}
						result.Error = fmt.Sprintf("%s | Debug: %s", message, debugInfo)
					}

					json.NewEncoder(w).Encode(result)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// RecoveryConfig Recovery中间件配置
type RecoveryConfig struct {
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
