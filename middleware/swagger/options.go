/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\options.go
 * @Description: Swagger 中间件选项模式，支持自定义日志和错误响应
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"net/http"

	logger "github.com/kamalyes/go-toolbox/pkg/logger"
)

// ErrorResponseFunc 自定义错误响应函数
// 用于替换默认的 JSON 错误响应格式，适配不同框架的错误协议
type ErrorResponseFunc func(w http.ResponseWriter, httpStatus int, message string)

// Option 中间件选项函数
type Option func(*Options)

// Options Swagger 中间件配置选项
type Options struct {
	// Logger 自定义日志器，为 nil 时使用默认日志器
	Logger logger.ILogger
	// ErrorResponseFn 自定义错误响应函数，为 nil 时使用默认 JSON 响应
	ErrorResponseFn ErrorResponseFunc
}

// ApplyOptions 应用选项列表，返回填充后的 Options 实例
func ApplyOptions(opts ...Option) *Options {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

// WithLogger 设置自定义日志器
func WithLogger(l logger.ILogger) Option {
	return func(o *Options) {
		o.Logger = l
	}
}

// WithErrorResponseFn 设置自定义错误响应函数
func WithErrorResponseFn(fn ErrorResponseFunc) Option {
	return func(o *Options) {
		o.ErrorResponseFn = fn
	}
}
