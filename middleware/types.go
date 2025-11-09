/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 07:49:58
 * @FilePath: \go-rpc-gateway\middleware\types.go
 * @Description: 中间件类型定义
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"net/http"
	"time"

	logger "github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-toolbox/pkg/osx"
)

// MiddlewareFunc 中间件函数类型
type MiddlewareFunc func(http.Handler) http.Handler

// ChainFunc 创建中间件链函数
func ChainFunc(middlewares ...MiddlewareFunc) MiddlewareFunc {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

// RequestIDConfig 请求ID中间件配置
type RequestIDConfig struct {
	Header    string
	Generator func() string
}

// DefaultRequestIDConfig 默认请求ID配置
func DefaultRequestIDConfig() RequestIDConfig {
	return RequestIDConfig{
		Header: constants.HeaderXRequestID,
		Generator: func() string {
			return osx.HashUnixMicroCipherText()
		},
	}
}

// TimeoutConfig 超时中间件配置
type TimeoutConfig struct {
	Timeout time.Duration
	Message string
}

// DefaultTimeoutConfig 默认超时配置
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Timeout: 30 * time.Second,
		Message: "请求超时",
	}
}

// LoggerConfig 日志中间件配置
type LoggerConfig struct {
	Logger         *logger.Logger
	EnableRequest  bool
	EnableResponse bool
	SkipPaths      []string
}

// DefaultLoggerConfig 默认日志配置
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		EnableRequest:  true,
		EnableResponse: true,
		SkipPaths:      []string{constants.DefaultHealthPath, constants.DefaultMetricsPath, constants.DefaultDebugPath},
	}
}
