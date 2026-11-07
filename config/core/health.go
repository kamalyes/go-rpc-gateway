/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 09:51:05
 * @FilePath: \go-rpc-gateway\config\core\health.go
 * @Description: 健康检查配置模块
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package core

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Health 健康检查配置
type Health struct {
	ModuleName string       `mapstructure:"module-name" yaml:"module-name" json:"moduleName"` // 模块名称
	Enabled    bool         `mapstructure:"enabled" yaml:"enabled" json:"enabled"`            // 是否启用健康检查
	Path       string       `mapstructure:"path" yaml:"path" json:"path"`                     // 健康检查路径
	Port       int          `mapstructure:"port" yaml:"port" json:"port"`                     // 健康检查端口
	Timeout    int          `mapstructure:"timeout" yaml:"timeout" json:"timeout"`            // 超时时间(秒)
	Redis      *RedisConfig `mapstructure:"redis" yaml:"redis" json:"redis"`                  // Redis健康检查配置
	MySQL      *MySQLConfig `mapstructure:"mysql" yaml:"mysql" json:"mysql"`                  // MySQL健康检查配置
}

// RedisConfig Redis健康检查配置
type RedisConfig struct {
	Enabled bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"` // 是否启用Redis健康检查
	Path    string `mapstructure:"path" yaml:"path" json:"path"`          // 健康检查路径
	Timeout int    `mapstructure:"timeout" yaml:"timeout" json:"timeout"` // 超时时间(秒)
}

// MySQLConfig MySQL健康检查配置
type MySQLConfig struct {
	Enabled bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"` // 是否启用MySQL健康检查
	Path    string `mapstructure:"path" yaml:"path" json:"path"`          // 健康检查路径
	Timeout int    `mapstructure:"timeout" yaml:"timeout" json:"timeout"` // 超时时间(秒)
}

// DefaultHealth 创建默认健康检查配置
func DefaultHealth() *Health {
	return &Health{
		ModuleName: "health",
		Enabled:    true,
		Path:       "/health",
		Port:       8080,
		Timeout:    30,
		Redis: &RedisConfig{
			Enabled: false,
			Path:    "/health/redis",
			Timeout: 5,
		},
		MySQL: &MySQLConfig{
			Enabled: false,
			Path:    "/health/mysql",
			Timeout: 5,
		},
	}
}

// Get 返回配置接口
func (c *Health) Get() interface{} {
	return c
}

// Set 设置配置数据
func (c *Health) Set(data interface{}) {
	if cfg, ok := data.(*Health); ok {
		*c = *cfg
	}
}

// Clone 返回配置的副本
func (c *Health) Clone() internal.Configurable {
	var cloned Health
	if err := syncx.DeepCopy(&cloned, c); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Health{}
	}
	return &cloned
}

// Validate 验证配置
func (c *Health) Validate() error {
	return internal.ValidateStruct(c)
}

// WithModuleName 设置模块名称
func (c *Health) WithModuleName(moduleName string) *Health {
	c.ModuleName = moduleName
	return c
}

// WithEnabled 设置是否启用健康检查
func (c *Health) WithEnabled(enabled bool) *Health {
	c.Enabled = enabled
	return c
}

// WithPath 设置健康检查路径
func (c *Health) WithPath(path string) *Health {
	c.Path = path
	return c
}

// WithPort 设置健康检查端口
func (c *Health) WithPort(port int) *Health {
	c.Port = port
	return c
}

// WithTimeout 设置超时时间
func (c *Health) WithTimeout(timeout int) *Health {
	c.Timeout = timeout
	return c
}

// WithRedisCheck 设置Redis健康检查
func (c *Health) WithRedisCheck(enabled bool, path string, timeout int) *Health {
	if c.Redis == nil {
		c.Redis = &RedisConfig{}
	}
	c.Redis.Enabled = enabled
	c.Redis.Path = path
	c.Redis.Timeout = timeout
	return c
}

// WithMySQLCheck 设置MySQL健康检查
func (c *Health) WithMySQLCheck(enabled bool, path string, timeout int) *Health {
	if c.MySQL == nil {
		c.MySQL = &MySQLConfig{}
	}
	c.MySQL.Enabled = enabled
	c.MySQL.Path = path
	c.MySQL.Timeout = timeout
	return c
}

// Enable 启用健康检查
func (c *Health) Enable() *Health {
	c.Enabled = true
	return c
}

// Disable 禁用健康检查
func (c *Health) Disable() *Health {
	c.Enabled = false
	return c
}

// IsEnabled 检查是否启用
func (c *Health) IsEnabled() bool {
	return c.Enabled
}
