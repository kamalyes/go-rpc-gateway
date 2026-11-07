/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-11 10:44:00
 * @FilePath: \go-rpc-gateway\config\core\banner.go
 * @Description: Banner配置模块
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package core

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Banner Banner配置
type Banner struct {
	ModuleName  string `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`  // 模块名称
	Enabled     bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`             // 是否启用Banner
	Template    string `mapstructure:"template" yaml:"template" json:"template"`          // 自定义模板
	Title       string `mapstructure:"title" yaml:"title" json:"title"`                   // 标题
	Description string `mapstructure:"description" yaml:"description" json:"description"` // 描述
	Author      string `mapstructure:"author" yaml:"author" json:"author"`                // 作者
	Email       string `mapstructure:"email" yaml:"email" json:"email"`                   // 邮箱
	Website     string `mapstructure:"website" yaml:"website" json:"website"`             // 网站
}

// DefaultBanner 创建默认Banner配置
func DefaultBanner() *Banner {
	defaultTemplate := `
 ██████╗  ██████╗       ██████╗ ██████╗  ██████╗     ██████╗  █████╗ ████████╗███████╗██╗    ██╗ █████╗ ██╗   ██╗
██╔════╝ ██╔═══██╗      ██╔══██╗██╔══██╗██╔════╝    ██╔════╝ ██╔══██╗╚══██╔══╝██╔════╝██║    ██║██╔══██╗╚██╗ ██╔╝
██║  ███╗██║   ██║█████╗██████╔╝██████╔╝██║         ██║  ███╗███████║   ██║   █████╗  ██║ █╗ ██║███████║ ╚████╔╝ 
██║   ██║██║   ██║╚════╝██╔══██╗██╔═══╝ ██║         ██║   ██║██╔══██║   ██║   ██╔══╝  ██║███╗██║██╔══██║  ╚██╔╝  
╚██████╔╝╚██████╔╝      ██║  ██║██║     ╚██████╗    ╚██████╔╝██║  ██║   ██║   ███████╗╚███╔███╔╝██║  ██║   ██║   
 ╚═════╝  ╚═════╝       ╚═╝  ╚═╝╚═╝      ╚═════╝     ╚═════╝ ╚═╝  ╚═╝   ╚═╝   ╚══════╝ ╚══╝╚══╝ ╚═╝  ╚═╝   ╚═╝   `

	return &Banner{
		ModuleName:  "banner",
		Enabled:     true,
		Template:    defaultTemplate,
		Title:       "Go RPC Gateway",
		Description: "A high-performance RPC gateway service",
		Author:      "kamalyes",
		Email:       "501893067@qq.com",
		Website:     "https://github.com/kamalyes/go-rpc-gateway",
	}
}

// Get 返回配置接口
func (b *Banner) Get() interface{} {
	return b
}

// Set 设置配置数据
func (b *Banner) Set(data interface{}) {
	if cfg, ok := data.(*Banner); ok {
		*b = *cfg
	}
}

// Clone 返回配置的副本
func (b *Banner) Clone() internal.Configurable {
	var cloned Banner
	if err := syncx.DeepCopy(&cloned, b); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Banner{}
	}
	return &cloned
}

// Validate 验证配置
func (b *Banner) Validate() error {
	return internal.ValidateStruct(b)
}

// WithTitle 设置标题
func (b *Banner) WithTitle(title string) *Banner {
	b.Title = title
	return b
}

// WithTemplate 设置自定义模板
func (b *Banner) WithTemplate(template string) *Banner {
	b.Template = template
	return b
}

// WithDescription 设置描述
func (b *Banner) WithDescription(description string) *Banner {
	b.Description = description
	return b
}

// WithAuthor 设置作者
func (b *Banner) WithAuthor(author string) *Banner {
	b.Author = author
	return b
}

// WithEmail 设置邮箱
func (b *Banner) WithEmail(email string) *Banner {
	b.Email = email
	return b
}

// WithWebsite 设置网站
func (b *Banner) WithWebsite(website string) *Banner {
	b.Website = website
	return b
}

// Enable 启用Banner
func (b *Banner) Enable() *Banner {
	b.Enabled = true
	return b
}

// Disable 禁用Banner
func (b *Banner) Disable() *Banner {
	b.Enabled = false
	return b
}

// IsEnabled 检查是否启用
func (b *Banner) IsEnabled() bool {
	return b.Enabled
}
