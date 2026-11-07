/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-14 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-13 11:33:06
 * @FilePath: \go-rpc-gateway\config\middleware\ratelimit.go
 * @Description: 增强的限流配置（支持路由级别、用户级别、IP级别等）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"time"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Strategy 限流策略类型
type Strategy string

const (
	StrategyTokenBucket   Strategy = "token-bucket"   // 令牌桶算法
	StrategyLeakyBucket   Strategy = "leaky-bucket"   // 漏桶算法
	StrategySlidingWindow Strategy = "sliding-window" // 滑动窗口
	StrategyFixedWindow   Strategy = "fixed-window"   // 固定窗口
)

// Scope 限流作用域
type Scope string

const (
	ScopeGlobal   Scope = "global"    // 全局限流
	ScopePerIP    Scope = "per-ip"    // 按IP限流
	ScopePerUser  Scope = "per-user"  // 按用户限流
	ScopePerRoute Scope = "per-route" // 按路由限流
)

// RateLimit 增强限流配置
type RateLimit struct {
	ModuleName        string            `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`                        // 模块名称
	Enabled           bool              `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                                   // 是否启用
	Strategy          Strategy          `mapstructure:"strategy" yaml:"strategy" json:"strategy"`                                // 限流策略
	DefaultScope      Scope             `mapstructure:"default-scope" yaml:"default-scope" json:"defaultScope"`                  // 默认作用域
	GlobalLimit       *LimitRule        `mapstructure:"global-limit" yaml:"global-limit" json:"globalLimit"`                     // 全局限流规则
	Routes            []RouteLimit      `mapstructure:"routes" yaml:"routes" json:"routes"`                                      // 路由级别限流
	IPRules           []IPRule          `mapstructure:"ip-rules" yaml:"ip-rules" json:"ipRules"`                                 // IP规则
	UserRules         []UserRule        `mapstructure:"user-rules" yaml:"user-rules" json:"userRules"`                           // 用户规则
	Storage           StorageConfig     `mapstructure:"storage" yaml:"storage" json:"storage"`                                   // 存储配置
	CustomRuleLoader  string            `mapstructure:"custom-rule-loader" yaml:"custom-rule-loader" json:"customRuleLoader"`    // 自定义规则加载器名称
	EnableDynamicRule bool              `mapstructure:"enable-dynamic-rule" yaml:"enable-dynamic-rule" json:"enableDynamicRule"` // 是否启用动态规则
	EmailAlertConfig  *EmailAlertConfig `mapstructure:"email-alert-config" yaml:"email-alert-config" json:"emailAlertConfig"`    // 邮件告警配置名称
}

// LimitRule 限流规则
type LimitRule struct {
	RequestsPerSecond int           `mapstructure:"requests-per-second" yaml:"requests-per-second" json:"requestsPerSecond"` // 每秒请求数
	BurstSize         int           `mapstructure:"burst-size" yaml:"burst-size" json:"burstSize"`                           // 突发大小
	WindowSize        time.Duration `mapstructure:"window-size" yaml:"window-size" json:"windowSize"`                        // 时间窗口
	BlockDuration     time.Duration `mapstructure:"block-duration" yaml:"block-duration" json:"blockDuration"`               // 阻塞时长
}

// RouteLimit 路由级别限流
type RouteLimit struct {
	Path      string     `mapstructure:"path" yaml:"path" json:"path"`                // 路由路径（支持通配符）
	Methods   []string   `mapstructure:"methods" yaml:"methods" json:"methods"`       // HTTP方法
	Limit     *LimitRule `mapstructure:"limit" yaml:"limit" json:"limit"`             // 限流规则
	PerUser   bool       `mapstructure:"per-user" yaml:"per-user" json:"perUser"`     // 是否按用户限流
	PerIP     bool       `mapstructure:"per-ip" yaml:"per-ip" json:"perIp"`           // 是否按IP限流
	Whitelist []string   `mapstructure:"whitelist" yaml:"whitelist" json:"whitelist"` // 白名单
	Blacklist []string   `mapstructure:"blacklist" yaml:"blacklist" json:"blacklist"` // 黑名单
}

// IPRule IP限流规则
type IPRule struct {
	IP       string     `mapstructure:"ip" yaml:"ip" json:"ip"`                   // IP地址（支持CIDR）
	Limit    *LimitRule `mapstructure:"limit" yaml:"limit" json:"limit"`          // 限流规则
	Type     string     `mapstructure:"type" yaml:"type" json:"type"`             // 类型: whitelist, blacklist, custom
	Priority int        `mapstructure:"priority" yaml:"priority" json:"priority"` // 优先级
}

// UserRule 用户限流规则
type UserRule struct {
	UserID   string     `mapstructure:"user-id" yaml:"user-id" json:"userId"`       // 用户ID（支持通配符）
	UserType string     `mapstructure:"user-type" yaml:"user-type" json:"userType"` // 用户类型
	Role     string     `mapstructure:"role" yaml:"role" json:"role"`               // 用户角色
	Limit    *LimitRule `mapstructure:"limit" yaml:"limit" json:"limit"`            // 限流规则
	Priority int        `mapstructure:"priority" yaml:"priority" json:"priority"`   // 优先级
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type          string        `mapstructure:"type" yaml:"type" json:"type"`                              // 存储类型: memory, redis
	KeyPrefix     string        `mapstructure:"key-prefix" yaml:"key-prefix" json:"keyPrefix"`             // Key前缀
	CleanInterval time.Duration `mapstructure:"clean-interval" yaml:"clean-interval" json:"cleanInterval"` // 清理间隔
	RedisConfig   *RedisStorage `mapstructure:"redis" yaml:"redis" json:"redis"`                           // Redis配置（可选）
}

// RedisStorage Redis存储配置（复用cache.Redis字段）
type RedisStorage struct {
	Addresses    []string      `mapstructure:"addresses" yaml:"addresses" json:"addresses"`              // Redis地址
	Password     string        `mapstructure:"password" yaml:"password" json:"password"`                 // 密码
	DB           int           `mapstructure:"db" yaml:"db" json:"db"`                                   // 数据库
	PoolSize     int           `mapstructure:"pool-size" yaml:"pool-size" json:"poolSize"`               // 连接池
	MinIdleConns int           `mapstructure:"min-idle-conns" yaml:"min-idle-conns" json:"minIdleConns"` // 最小空闲连接
	MaxRetries   int           `mapstructure:"max-retries" yaml:"max-retries" json:"maxRetries"`         // 最大重试
	ReadTimeout  time.Duration `mapstructure:"read-timeout" yaml:"read-timeout" json:"readTimeout"`      // 读超时
	WriteTimeout time.Duration `mapstructure:"write-timeout" yaml:"write-timeout" json:"writeTimeout"`   // 写超时
	ClusterMode  bool          `mapstructure:"cluster-mode" yaml:"cluster-mode" json:"clusterMode"`      // 集群模式
}

// DefaultRateLimit 默认限流配置
func DefaultRateLimit() *RateLimit {
	return &RateLimit{
		ModuleName:   "ratelimit",
		Enabled:      false,
		Strategy:     StrategyTokenBucket,
		DefaultScope: ScopeGlobal,
		GlobalLimit: &LimitRule{
			RequestsPerSecond: 100,
			BurstSize:         200,
			WindowSize:        time.Minute,
			BlockDuration:     time.Minute,
		},
		Storage: StorageConfig{
			Type:          "memory",
			KeyPrefix:     "rate_limit:",
			CleanInterval: 5 * time.Minute,
		},
		Routes:            []RouteLimit{},
		IPRules:           []IPRule{},
		UserRules:         []UserRule{},
		EnableDynamicRule: false,
		EmailAlertConfig:  DefaultEmailAlertConfig(),
	}
}

// Clone 返回配置副本
func (r *RateLimit) Clone() internal.Configurable {
	var cloned RateLimit
	if err := syncx.DeepCopy(&cloned, r); err != nil {
		// 如果深拷贝失败，返回空配置
		return &RateLimit{}
	}
	return &cloned
}

// Get 返回配置
func (r *RateLimit) Get() interface{} {
	return r
}

// Set 设置配置
func (r *RateLimit) Set(data interface{}) {
	if cfg, ok := data.(*RateLimit); ok {
		*r = *cfg
	}
}

// Validate 验证配置
func (r *RateLimit) Validate() error {
	return internal.ValidateStruct(r)
}

// WithGlobalLimit 设置全局限流
func (r *RateLimit) WithGlobalLimit(limit *LimitRule) *RateLimit {
	r.GlobalLimit = limit
	return r
}

// AddRouteLimit 添加路由限流
func (r *RateLimit) AddRouteLimit(route RouteLimit) *RateLimit {
	r.Routes = append(r.Routes, route)
	return r
}

// AddIPRule 添加IP规则
func (r *RateLimit) AddIPRule(rule IPRule) *RateLimit {
	r.IPRules = append(r.IPRules, rule)
	return r
}

// AddUserRule 添加用户规则
func (r *RateLimit) AddUserRule(rule UserRule) *RateLimit {
	r.UserRules = append(r.UserRules, rule)
	return r
}

// WithStrategy 设置限流策略
func (r *RateLimit) WithStrategy(strategy Strategy) *RateLimit {
	r.Strategy = strategy
	return r
}

// WithStorage 设置存储配置
func (r *RateLimit) WithStorage(storage StorageConfig) *RateLimit {
	r.Storage = storage
	return r
}

// EnableDynamic 启用动态规则
func (r *RateLimit) EnableDynamic() *RateLimit {
	r.EnableDynamicRule = true
	return r
}

// WithCustomLoader 设置自定义加载器
func (r *RateLimit) WithCustomLoader(loaderName string) *RateLimit {
	r.CustomRuleLoader = loaderName
	return r
}

// Enable 启用限流中间件
func (r *RateLimit) Enable() *RateLimit {
	r.Enabled = true
	return r
}

// Disable 禁用限流中间件
func (r *RateLimit) Disable() *RateLimit {
	r.Enabled = false
	return r
}

// IsEnabled 检查是否启用
func (r *RateLimit) IsEnabled() bool {
	return r.Enabled
}
