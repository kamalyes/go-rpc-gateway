/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-13 11:33:50
 * @FilePath: \go-rpc-gateway\config\smtp\smtp.go
 * @Description: 邮箱配置模块
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package smtp

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Smtp 邮箱配置
type Smtp struct {
	ModuleName  string            `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`    // 模块名称
	Enabled     bool              `mapstructure:"enabled" yaml:"enabled" json:"enabled"`               // 是否启用
	SMTPHost    string            `mapstructure:"smtp-host" yaml:"smtp-host" json:"smtpHost"`          // SMTP主机
	SMTPPort    int               `mapstructure:"smtp-port" yaml:"smtp-port" json:"smtpPort"`          // SMTP端口
	Username    string            `mapstructure:"username" yaml:"username" json:"username"`            // 用户名
	Password    string            `mapstructure:"password" yaml:"password" json:"password"`            // 密码
	FromAddress string            `mapstructure:"from-address" yaml:"from-address" json:"fromAddress"` // 发件人地址
	ToAddresses []string          `mapstructure:"to-addresses" yaml:"to-addresses" json:"toAddresses"` // 收件人地址列表
	EnableTLS   bool              `mapstructure:"enable-tls" yaml:"enable-tls" json:"enableTls"`       // 是否启用TLS
	Headers     map[string]string `mapstructure:"headers" yaml:"headers" json:"headers"`               // 自定义头部
	PoolSize    int               `mapstructure:"pool-size" yaml:"pool-size" json:"poolSize"`          // 连接池大小
}

// Default 创建默认邮箱配置
func Default() *Smtp {
	return &Smtp{
		ModuleName:  "smtp",
		Enabled:     false,
		SMTPHost:    "127.0.0.1",
		SMTPPort:    587,
		Username:    "smtp_user",
		Password:    "smtp_password",
		FromAddress: "noreply@example.com",
		ToAddresses: []string{"admin@example.com"},
		EnableTLS:   false,
		Headers:     make(map[string]string),
		PoolSize:    5,
	}
}

// Get 返回配置接口
func (e *Smtp) Get() interface{} {
	return e
}

// Set 设置配置数据
func (e *Smtp) Set(data interface{}) {
	if cfg, ok := data.(*Smtp); ok {
		*e = *cfg
	}
}

// Clone 返回配置的副本
func (e *Smtp) Clone() internal.Configurable {
	var cloned Smtp
	if err := syncx.DeepCopy(&cloned, e); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Smtp{}
	}
	return &cloned
}

// Validate 验证配置
func (e *Smtp) Validate() error {
	return internal.ValidateStruct(e)
}

// WithModuleName 设置模块名称
func (e *Smtp) WithModuleName(name string) *Smtp {
	e.ModuleName = name
	return e
}

// WithEnabled 设置是否启用
func (e *Smtp) WithEnabled(enabled bool) *Smtp {
	e.Enabled = enabled
	return e
}

// WithSMTPHost 设置SMTP主机
func (e *Smtp) WithSMTPHost(host string) *Smtp {
	e.SMTPHost = host
	return e
}

// WithSMTPPort 设置SMTP端口
func (e *Smtp) WithSMTPPort(port int) *Smtp {
	e.SMTPPort = port
	return e
}

// WithUsername 设置用户名
func (e *Smtp) WithUsername(username string) *Smtp {
	e.Username = username
	return e
}

// WithPassword 设置密码
func (e *Smtp) WithPassword(password string) *Smtp {
	e.Password = password
	return e
}

// WithFromAddress 设置发件人地址
func (e *Smtp) WithFromAddress(address string) *Smtp {
	e.FromAddress = address
	return e
}

// WithPoolSize 设置连接池大小
func (e *Smtp) WithPoolSize(size int) *Smtp {
	e.PoolSize = size
	return e
}

// AddToAddress 添加收件人地址
func (e *Smtp) AddToAddress(address string) *Smtp {
	e.ToAddresses = append(e.ToAddresses, address)
	return e
}

// EnableTLSService 启用TLS
func (e *Smtp) EnableTLSService() *Smtp {
	e.EnableTLS = true
	return e
}

// DisableTLS 禁用TLS
func (e *Smtp) DisableTLS() *Smtp {
	e.EnableTLS = false
	return e
}

// AddHeader 添加自定义头部
func (e *Smtp) AddHeader(key, value string) *Smtp {
	if e.Headers == nil {
		e.Headers = make(map[string]string)
	}
	e.Headers[key] = value
	return e
}
