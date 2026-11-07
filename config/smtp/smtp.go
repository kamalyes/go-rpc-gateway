/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-13 11:33:50
 * @FilePath: \go-rpc-gateway\config\smtp\smtp.go
 * @Description: 邮箱配置（单 DSN 方案）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package smtp

import (
	"fmt"
	"net/url"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Smtp 邮箱配置（单 DSN）
//
// 服务器地址、端口、认证、传输加密方式全部写在 DSN 一个字段里；
// 保留的字段只有发件人、收件人、自定义头部——那是邮件内容的事，不属于连接参数
//
// ─── DSN 格式 ───────────────────────────────────────────────────────────────
//
//	smtp://[用户名:密码@]主机:端口[?参数=值&...]    明文投递，服务端支持时自动升级 STARTTLS
//	smtps://[用户名:密码@]主机:端口[?参数=值&...]   连上即 TLS（隐式 TLS，端口通常 465）
//
// 用户名多数情况下就是邮箱地址，其中的 @ 必须转义成 %40，否则会被当成 userinfo
// 与 host 的分隔符，解析出来的主机就错了
//
// ─── DSN 示例 ───────────────────────────────────────────────────────────────
//
//	标准 587 端口，提交邮件并自动 STARTTLS 升级：
//	  smtp://noreply%40example.com:app_password@smtp.example.com:587
//
//	465 端口隐式 TLS：
//	  smtps://noreply%40example.com:app_password@smtp.example.com:465
//
//	强制走 STARTTLS，且跳过自签证书校验（内网自建邮件服务器）：
//	  smtp://noreply%40corp.lan:pwd@mail.corp.lan:587?starttls=true&skip_verify=true
//
//	本地 25 端口中继，无需认证：
//	  smtp://127.0.0.1:25
//
//	Gmail 应用专用密码：
//	  smtp://someone%40gmail.com:abcd_efgh_ijkl_mnop@smtp.gmail.com:587
//
// ─── 查询参数 ───────────────────────────────────────────────────────────────
//
//	starttls     强制使用 STARTTLS 并由网关提供 TLS 配置（仅 smtp:// 有意义）。
//	             不加这项时 smtp:// 走 net/smtp 的默认行为：服务端声明支持 STARTTLS
//	             就自动升级，不支持就明文投递——但那条路径不接受自定义 TLS 配置，
//	             也就用不上 skip_verify
//	skip_verify  跳过 TLS 证书校验，需与 smtps:// 或 starttls=true 同时使用
//
// [EN] SMTP config, collapsed to a single DSN: host, port, credentials and transport
// encryption all live in DSN, and the only remaining fields describe the message rather
// than the connection. A username is usually an email address, whose @ must be written as
// %40 or it is read as the userinfo separator and the parsed host comes out wrong. Note
// that plain smtp:// without starttls=true uses net/smtp's own opportunistic upgrade, which
// accepts no custom TLS config and therefore ignores skip_verify
type Smtp struct {
	ModuleName  string            `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`    // 模块名称
	Enabled     bool              `mapstructure:"enabled" yaml:"enabled" json:"enabled"`               // 是否启用
	DSN         string            `mapstructure:"dsn" yaml:"dsn" json:"dsn"`                           // 连接 DSN，格式与示例见类型注释
	FromAddress string            `mapstructure:"from-address" yaml:"from-address" json:"fromAddress"` // 发件人地址，留空时取 DSN 里的用户名
	ToAddresses []string          `mapstructure:"to-addresses" yaml:"to-addresses" json:"toAddresses"` // 默认收件人地址列表
	Headers     map[string]string `mapstructure:"headers" yaml:"headers" json:"headers"`               // 自定义头部，附加到每封邮件上
}

// Default 创建默认邮箱配置
func Default() *Smtp {
	return &Smtp{
		ModuleName:  "smtp",
		Enabled:     false,
		DSN:         "smtp://smtp_user:smtp_password@127.0.0.1:587",
		FromAddress: "noreply@example.com",
		ToAddresses: []string{"admin@example.com"},
		Headers:     make(map[string]string),
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
//
// 未启用时不校验 DSN：Enabled=false 的模块允许留空配置
//
// [EN] Validate skips the DSN when the module is disabled, since a disabled module is
// allowed to carry no configuration at all
func (e *Smtp) Validate() error {
	if !e.Enabled {
		return nil
	}

	if e.DSN == "" {
		return fmt.Errorf("smtp dsn is empty")
	}

	u, err := url.Parse(e.DSN)
	if err != nil {
		return fmt.Errorf("smtp dsn is invalid: %w", err)
	}
	if u.Scheme != "smtp" && u.Scheme != "smtps" {
		return fmt.Errorf("smtp dsn scheme must be smtp or smtps, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("smtp dsn has no host: %s", e.DSN)
	}

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

// WithDSN 设置连接 DSN，格式与示例见 Smtp 类型注释
func (e *Smtp) WithDSN(dsn string) *Smtp {
	e.DSN = dsn
	return e
}

// WithFromAddress 设置发件人地址
func (e *Smtp) WithFromAddress(address string) *Smtp {
	e.FromAddress = address
	return e
}

// AddToAddress 添加收件人地址
func (e *Smtp) AddToAddress(address string) *Smtp {
	e.ToAddresses = append(e.ToAddresses, address)
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
