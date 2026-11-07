/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 15:28:28
 * @FilePath: \go-rpc-gateway\config\cache\cache.go
 * @Description: 缓存总配置定义
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package cache

import (
	"time"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// CacheType 缓存类型
type CacheType string

const (
	CacheTypeRedis CacheType = "redis" // Redis缓存
)

// Cache 总缓存配置
type Cache struct {
	ModuleName string    `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`
	Type       CacheType `mapstructure:"type" yaml:"type" json:"type"`
	Enabled    bool      `mapstructure:"enabled" yaml:"enabled" json:"enabled"`

	// 基础配置
	DefaultTTL time.Duration `mapstructure:"default-ttl" yaml:"default-ttl" json:"defaultTtl"`
	KeyPrefix  string        `mapstructure:"key-prefix" yaml:"key-prefix" json:"keyPrefix"`
	Serializer string        `mapstructure:"serializer" yaml:"serializer" json:"serializer"` // json, gob, msgpack

	// 缓存实现配置
	Redis Redis `mapstructure:"redis" yaml:"redis" json:"redis"`
}

// NewCache 创建一个新的 Cache 实例
func NewCache(opt *Cache) *Cache {
	var cacheInstance *Cache

	internal.LockFunc(func() {
		cacheInstance = opt
	})
	return cacheInstance
}

// Clone 返回 Cache 配置的副本
func (c *Cache) Clone() internal.Configurable {
	var cloned Cache
	if err := syncx.DeepCopy(&cloned, c); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Cache{}
	}
	return &cloned
}

// Get 返回 Cache 配置的所有字段
func (c *Cache) Get() interface{} {
	return c
}

// Set 更新 Cache 配置的字段
func (c *Cache) Set(data interface{}) {
	if configData, ok := data.(*Cache); ok {
		c.ModuleName = configData.ModuleName
		c.Type = configData.Type
		c.Enabled = configData.Enabled
		c.Redis = configData.Redis
		c.DefaultTTL = configData.DefaultTTL
		c.KeyPrefix = configData.KeyPrefix
		c.Serializer = configData.Serializer
	}
}

// GetModuleName 返回模块名称
func (c *Cache) GetModuleName() string {
	return c.ModuleName
}

// IsEnabled 检查缓存是否启用
func (c *Cache) IsEnabled() bool {
	return c.Enabled
}

// GetType 获取缓存类型
func (c *Cache) GetType() CacheType {
	return c.Type
}

// GetKeyPrefix 获取键前缀
func (c *Cache) GetKeyPrefix() string {
	if c.KeyPrefix == "" {
		return "cache:"
	}
	return c.KeyPrefix
}

// GetSerializer 获取序列化器类型
func (c *Cache) GetSerializer() string {
	if c.Serializer == "" {
		return "json"
	}
	return c.Serializer
}

// GetDefaultTTL 获取默认TTL
func (c *Cache) GetDefaultTTL() time.Duration {
	if c.DefaultTTL == 0 {
		return 30 * time.Minute
	}
	return c.DefaultTTL
}

// Validate 验证配置是否有效
func (c *Cache) Validate() error {
	if !c.Enabled {
		return nil // 未启用时不需要验证
	}

	// 验证具体的缓存配置
	switch c.Type {
	case CacheTypeRedis:
		return c.Redis.Validate()
	}

	return internal.ValidateStruct(c)
}

// DefaultCache 返回默认缓存配置
func DefaultCache() Cache {
	return Cache{
		ModuleName: "default",
		Type:       CacheTypeRedis,
		Enabled:    false,
		DefaultTTL: 30 * time.Minute,
		KeyPrefix:  "cache:",
		Serializer: "json",
		Redis:      DefaultRedisConfig(),
	}
}

// Default 返回默认缓存配置的指针，支持链式调用
func Default() *Cache {
	config := DefaultCache()
	return &config
}

// WithModuleName 设置模块名称
func (c *Cache) WithModuleName(moduleName string) *Cache {
	c.ModuleName = moduleName
	return c
}

// WithType 设置缓存类型
func (c *Cache) WithType(cacheType CacheType) *Cache {
	c.Type = cacheType
	return c
}

// WithEnabled 设置是否启用缓存
func (c *Cache) WithEnabled(enabled bool) *Cache {
	c.Enabled = enabled
	return c
}

// WithDefaultTTL 设置默认TTL
func (c *Cache) WithDefaultTTL(ttl time.Duration) *Cache {
	c.DefaultTTL = ttl
	return c
}

// WithKeyPrefix 设置键前缀
func (c *Cache) WithKeyPrefix(prefix string) *Cache {
	c.KeyPrefix = prefix
	return c
}

// WithSerializer 设置序列化器
func (c *Cache) WithSerializer(serializer string) *Cache {
	c.Serializer = serializer
	return c
}

// WithRedis 设置Redis缓存配置
func (c *Cache) WithRedis(redis Redis) *Cache {
	c.Redis = redis
	return c
}
