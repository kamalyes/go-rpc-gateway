/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-27 00:23:21
 * @FilePath: \go-rpc-gateway\config\middleware\recovery.go
 * @Description: 恢复中间件配置模块
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"net/http"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Recovery 恢复中间件配置
type Recovery struct {
	ModuleName      string                                                `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`       // 模块名称
	Enabled         bool                                                  `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                  // 是否启用恢复
	EnableStack     bool                                                  `mapstructure:"enable-stack" yaml:"enable-stack" json:"enableStack"`    // 是否启用堆栈跟踪
	StackSize       int                                                   `mapstructure:"stack-size" yaml:"stack-size" json:"stackSize"`          // 堆栈大小
	EnableDebug     bool                                                  `mapstructure:"enable-debug" yaml:"enable-debug" json:"enableDebug"`    // 是否启用调试模式
	ErrorMessage    string                                                `mapstructure:"error-message" yaml:"error-message" json:"errorMessage"` // 默认错误消息
	LogLevel        string                                                `mapstructure:"log-level" yaml:"log-level" json:"logLevel"`             // 日志级别
	EnableNotify    bool                                                  `mapstructure:"enable-notify" yaml:"enable-notify" json:"enableNotify"` // 是否启用通知
	RecoveryHandler func(http.ResponseWriter, *http.Request, interface{}) `mapstructure:"-" yaml:"-" json:"-"`                                    // 自定义恢复处理器
	PrintStack      bool                                                  `mapstructure:"print-stack" yaml:"print-stack" json:"printStack"`       // 是否打印堆栈(兼容旧版)
}

// DefaultRecovery 创建默认恢复配置
func DefaultRecovery() *Recovery {
	return &Recovery{
		ModuleName:      "recovery",
		Enabled:         true,
		EnableStack:     true,
		StackSize:       4096,
		EnableDebug:     false,
		ErrorMessage:    "服务器内部错误",
		LogLevel:        "error",
		EnableNotify:    false,
		RecoveryHandler: nil,
		PrintStack:      true, // 兼容旧版
	}
}

// Get 返回配置接口
func (r *Recovery) Get() interface{} {
	return r
}

// Set 设置配置数据
func (r *Recovery) Set(data interface{}) {
	if cfg, ok := data.(*Recovery); ok {
		*r = *cfg
	}
}

// Clone 返回配置的副本
func (r *Recovery) Clone() internal.Configurable {
	var cloned Recovery
	if err := syncx.DeepCopy(&cloned, r); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Recovery{}
	}
	return &cloned
}

// Validate 验证配置
func (r *Recovery) Validate() error {
	return internal.ValidateStruct(r)
}

// WithPrintStack 设置是否打印堆栈
func (r *Recovery) WithPrintStack(printStack bool) *Recovery {
	r.PrintStack = printStack
	r.EnableStack = printStack // 同步设置新字段
	return r
}

// WithEnableStack 设置是否启用堆栈跟踪
func (r *Recovery) WithEnableStack(enableStack bool) *Recovery {
	r.EnableStack = enableStack
	return r
}

// WithStackSize 设置堆栈大小
func (r *Recovery) WithStackSize(stackSize int) *Recovery {
	r.StackSize = stackSize
	return r
}

// WithEnableDebug 设置是否启用调试模式
func (r *Recovery) WithEnableDebug(enableDebug bool) *Recovery {
	r.EnableDebug = enableDebug
	return r
}

// WithErrorMessage 设置默认错误消息
func (r *Recovery) WithErrorMessage(errorMessage string) *Recovery {
	r.ErrorMessage = errorMessage
	return r
}

// WithRecoveryHandler 设置自定义恢复处理器
func (r *Recovery) WithRecoveryHandler(handler func(http.ResponseWriter, *http.Request, interface{})) *Recovery {
	r.RecoveryHandler = handler
	return r
}

// WithLogLevel 设置日志级别
func (r *Recovery) WithLogLevel(logLevel string) *Recovery {
	r.LogLevel = logLevel
	return r
}

// WithNotify 设置是否启用通知
func (r *Recovery) WithNotify(notify bool) *Recovery {
	r.EnableNotify = notify
	return r
}

// Enable 启用恢复中间件
func (r *Recovery) Enable() *Recovery {
	r.Enabled = true
	return r
}

// Disable 禁用恢复中间件
func (r *Recovery) Disable() *Recovery {
	r.Enabled = false
	return r
}

// IsEnabled 检查是否启用
func (r *Recovery) IsEnabled() bool {
	return r.Enabled
}
