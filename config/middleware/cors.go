/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2024-11-07 23:55:58
 * @FilePath: \go-rpc-gateway\config\middleware\cors.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Cors 跨域资源共享配置
type Cors struct {
	Enabled             bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                                                       // 是否启用CORS
	AllowedOrigins      []string `mapstructure:"allowed-origins" yaml:"allowed-origins" json:"allowedOrigins"       validate:"dive,required"` // 允许的来源
	AllowedMethods      []string `mapstructure:"allowed-methods" yaml:"allowed-methods" json:"allowedMethods"       validate:"dive,required"` // 允许的方法
	AllowedHeaders      []string `mapstructure:"allowed-headers" yaml:"allowed-headers" json:"allowedHeaders"       validate:"dive,required"` // 允许的头部
	MaxAge              string   `mapstructure:"max-age" yaml:"max-age" json:"maxAge"               validate:"required"`                      // 最大缓存时间
	AllowedAllOrigins   bool     `mapstructure:"allowed-all-origins" yaml:"allowed-all-origins" json:"allowedAllOrigins"`                     // 是否允许所有来源
	AllowedAllMethods   bool     `mapstructure:"allowed-all-methods" yaml:"allowed-all-methods" json:"allowedAllMethods"`                     // 是否允许所有方法
	ExposedHeaders      []string `mapstructure:"exposed-headers" yaml:"exposed-headers" json:"exposedHeaders"`                                // 暴露的头部
	AllowCredentials    bool     `mapstructure:"allow-credentials" yaml:"allow-credentials" json:"allowCredentials"`                          // 允许凭证
	OptionsResponseCode int      `mapstructure:"options-response-code" yaml:"options-response-code" json:"optionsResponseCode"`               // Options响应Code
	ModuleName          string   `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`                                            // 模块名称
}

// NewCors 创建一个新的 Cors 实例
func NewCors(opt *Cors) *Cors {
	var corsInstance *Cors

	internal.LockFunc(func() {
		corsInstance = opt
	})
	return corsInstance
}

// Clone 返回 Cors 配置的副本
func (c *Cors) Clone() internal.Configurable {
	var cloned Cors
	if err := syncx.DeepCopy(&cloned, c); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Cors{}
	}
	return &cloned
}

// Get 返回 Cors 配置的所有字段
func (c *Cors) Get() interface{} {
	return c
}

// Set 更新 Cors 配置的字段
func (c *Cors) Set(data interface{}) {
	if configData, ok := data.(*Cors); ok {
		c.Enabled = configData.Enabled
		c.ModuleName = configData.ModuleName
		c.AllowedAllOrigins = configData.AllowedAllOrigins
		c.AllowedAllMethods = configData.AllowedAllMethods
		c.AllowedOrigins = configData.AllowedOrigins
		c.AllowedMethods = configData.AllowedMethods
		c.AllowedHeaders = configData.AllowedHeaders
		c.MaxAge = configData.MaxAge
		c.ExposedHeaders = configData.ExposedHeaders
		c.AllowCredentials = configData.AllowCredentials
		c.OptionsResponseCode = configData.OptionsResponseCode
	}
}

// Validate 检查 Cors 配置的有效性
func (c *Cors) Validate() error {
	return internal.ValidateStruct(c)
}

// DefaultCorsValue 返回默认Cors配置
func DefaultCorsValue() Cors {
	return Cors{
		Enabled:        true,
		ModuleName:     "cors",
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"},
		AllowedHeaders: []string{
			"Origin",
			"Content-Length",
			"Content-Type",
			"Authorization",
			"Accept",
			"Accept-Language",
			"Accept-Encoding",
			"Cache-Control",
			"X-Requested-With",
			"X-Request-ID",
			"X-Correlation-ID",
			"X-API-Key",
			"X-CSRF-Token",
			"X-Signature",
			"X-Timestamp",
			"X-Nonce",
			"User-Agent",
			"Referer",
			"X-Timezone",
		},
		MaxAge:              "86400",
		AllowedAllOrigins:   false,
		AllowedAllMethods:   false,
		ExposedHeaders:      []string{},
		AllowCredentials:    true,
		OptionsResponseCode: 200,
	}
}

// DefaultCors 返回默认Cors配置的指针，支持链式调用
func DefaultCors() *Cors {
	config := DefaultCorsValue()
	return &config
}

// WithEnabled 设置是否启用CORS
func (c *Cors) WithEnabled(enabled bool) *Cors {
	c.Enabled = enabled
	return c
}

// WithModuleName 设置模块名称
func (c *Cors) WithModuleName(moduleName string) *Cors {
	c.ModuleName = moduleName
	return c
}

// WithAllowedOrigins 设置允许的来源
func (c *Cors) WithAllowedOrigins(allowedOrigins []string) *Cors {
	c.AllowedOrigins = allowedOrigins
	return c
}

// WithAllowedMethods 设置允许的方法
func (c *Cors) WithAllowedMethods(allowedMethods []string) *Cors {
	c.AllowedMethods = allowedMethods
	return c
}

// WithAllowedHeaders 设置允许的头部
func (c *Cors) WithAllowedHeaders(allowedHeaders []string) *Cors {
	c.AllowedHeaders = allowedHeaders
	return c
}

// WithMaxAge 设置最大缓存时间
func (c *Cors) WithMaxAge(maxAge string) *Cors {
	c.MaxAge = maxAge
	return c
}

// WithAllowedAllOrigins 设置是否允许所有来源
func (c *Cors) WithAllowedAllOrigins(allowedAllOrigins bool) *Cors {
	c.AllowedAllOrigins = allowedAllOrigins
	return c
}

// WithAllowedAllMethods 设置是否允许所有方法
func (c *Cors) WithAllowedAllMethods(allowedAllMethods bool) *Cors {
	c.AllowedAllMethods = allowedAllMethods
	return c
}

// WithExposedHeaders 设置暴露的头部
func (c *Cors) WithExposedHeaders(exposedHeaders []string) *Cors {
	c.ExposedHeaders = exposedHeaders
	return c
}

// WithAllowCredentials 设置是否允许凭证
func (c *Cors) WithAllowCredentials(allowCredentials bool) *Cors {
	c.AllowCredentials = allowCredentials
	return c
}

// WithOptionsResponseCode 设置Options响应码
func (c *Cors) WithOptionsResponseCode(optionsResponseCode int) *Cors {
	c.OptionsResponseCode = optionsResponseCode
	return c
}
