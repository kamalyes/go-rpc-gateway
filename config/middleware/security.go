/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-13 12:05:20
 * @FilePath: \go-rpc-gateway\config\middleware\security.go
 * @Description: 统一安全配置模块 - 管理所有安全相关功能
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Security 统一安全配置 - 直接使用Security而不是SecurityConfig
type Security struct {
	ModuleName string      `mapstructure:"module-name" yaml:"module-name" json:"moduleName"` // 模块名称
	JWT        *JWT        `mapstructure:"jwt" yaml:"jwt" json:"jwt"`                        // JWT配置
	Auth       *Auth       `mapstructure:"auth" yaml:"auth" json:"auth"`                     // 通用认证配置
	Protection *Protection `mapstructure:"protection" yaml:"protection" json:"protection"`   // 服务保护配置
	CSP        *CSP        `mapstructure:"csp" yaml:"csp" json:"csp"`                        // CSP内容安全策略配置
	CSRF       *CSRF       `mapstructure:"csrf" yaml:"csrf" json:"csrf"`                     // CSRF 防护配置
	Validation *Validation `mapstructure:"validation" yaml:"validation" json:"validation"`   // 安全校验规则
}

// JWT JWT配置
type JWT struct {
	Enabled   bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`       // 是否启用JWT
	Secret    string `mapstructure:"secret" yaml:"secret" json:"secret"`          // JWT密钥
	Expiry    int    `mapstructure:"expiry" yaml:"expiry" json:"expiry"`          // 过期时间(小时)
	Issuer    string `mapstructure:"issuer" yaml:"issuer" json:"issuer"`          // 签发者
	Algorithm string `mapstructure:"algorithm" yaml:"algorithm" json:"algorithm"` // 加密算法
}

// Auth 通用认证配置 - 支持多种认证方式
type Auth struct {
	Enabled     bool        `mapstructure:"enabled" yaml:"enabled" json:"enabled"`               // 是否启用认证
	Type        string      `mapstructure:"type" yaml:"type" json:"type"`                        // 认证类型 (basic, bearer, custom, apikey)
	HeaderName  string      `mapstructure:"header-name" yaml:"header-name" json:"headerName"`    // 认证头名称
	TokenPrefix string      `mapstructure:"token-prefix" yaml:"token-prefix" json:"tokenPrefix"` // 令牌前缀
	Basic       *BasicAuth  `mapstructure:"basic" yaml:"basic" json:"basic"`                     // Basic认证
	Bearer      *BearerAuth `mapstructure:"bearer" yaml:"bearer" json:"bearer"`                  // Bearer认证
	APIKey      *APIKeyAuth `mapstructure:"apikey" yaml:"apikey" json:"apikey"`                  // APIKey认证
	Custom      *CustomAuth `mapstructure:"custom" yaml:"custom" json:"custom"`                  // 自定义认证
}

// BasicAuth Basic认证配置
type BasicAuth struct {
	Users []User `mapstructure:"users" yaml:"users" json:"users"` // 用户列表
}

// BearerAuth Bearer认证配置
type BearerAuth struct {
	Tokens []string `mapstructure:"tokens" yaml:"tokens" json:"tokens"` // 有效令牌列表
}

// APIKeyAuth API Key认证配置
type APIKeyAuth struct {
	Keys       []string `mapstructure:"keys" yaml:"keys" json:"keys"`                     // 有效API Key列表
	HeaderName string   `mapstructure:"header-name" yaml:"header-name" json:"headerName"` // API Key头名称
	QueryParam string   `mapstructure:"query-param" yaml:"query-param" json:"queryParam"` // API Key查询参数名
}

// CustomAuth 自定义认证配置
type CustomAuth struct {
	HeaderName    string            `mapstructure:"header-name" yaml:"header-name" json:"headerName"`          // 自定义头名称
	ExpectedValue string            `mapstructure:"expected-value" yaml:"expected-value" json:"expectedValue"` // 期望值
	Headers       map[string]string `mapstructure:"headers" yaml:"headers" json:"headers"`                     // 自定义头部验证
}

// User 用户信息
type User struct {
	Username    string   `mapstructure:"username" yaml:"username" json:"username"`          // 用户名
	Password    string   `mapstructure:"password" yaml:"password" json:"password"`          // 密码(建议加密存储)
	Role        string   `mapstructure:"role" yaml:"role" json:"role"`                      // 角色
	Permissions []string `mapstructure:"permissions" yaml:"permissions" json:"permissions"` // 权限列表
}

// Protection 服务保护配置 - 针对不同服务的安全设置
type Protection struct {
	Swagger *ServiceProtection `mapstructure:"swagger" yaml:"swagger" json:"swagger"` // Swagger保护
	PProf   *ServiceProtection `mapstructure:"pprof" yaml:"pprof" json:"pprof"`       // PProf保护
	Metrics *ServiceProtection `mapstructure:"metrics" yaml:"metrics" json:"metrics"` // Metrics保护
	Health  *ServiceProtection `mapstructure:"health" yaml:"health" json:"health"`    // Health保护
	API     *ServiceProtection `mapstructure:"api" yaml:"api" json:"api"`             // API保护
}

// Pprof 安全配置
type Pprof struct {
	Enabled      bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                  // 是否启用安全认证
	Username     string   `mapstructure:"username" yaml:"username" json:"username"`               // 用户名
	Password     string   `mapstructure:"password" yaml:"password" json:"password"`               // 密码
	AllowedIPs   []string `mapstructure:"allowed-ips" yaml:"allowed-ips" json:"allowedIps"`       // 允许的IP地址
	RequireHTTPS bool     `mapstructure:"require-https" yaml:"require-https" json:"requireHttps"` // 是否要求HTTPS
}

// ServiceProtection 服务保护配置 - 统一的服务安全配置
type ServiceProtection struct {
	Enabled      bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                  // 是否启用保护
	AuthRequired bool     `mapstructure:"auth-required" yaml:"auth-required" json:"authRequired"` // 是否需要认证
	AuthType     string   `mapstructure:"auth-type" yaml:"auth-type" json:"authType"`             // 认证类型
	IPWhitelist  []string `mapstructure:"ip-whitelist" yaml:"ip-whitelist" json:"ipWhitelist"`    // IP白名单
	RequireHTTPS bool     `mapstructure:"require-https" yaml:"require-https" json:"requireHttps"` // 是否要求HTTPS
	Username     string   `mapstructure:"username" yaml:"username" json:"username"`               // 用户名
	Password     string   `mapstructure:"password" yaml:"password" json:"password"`               // 密码
}

// CSRF 防护配置
type CSRF struct {
	Enabled         bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                             // 是否启用 CSRF 防护
	ExemptMethods   []string `mapstructure:"exempt-methods" yaml:"exempt-methods" json:"exemptMethods"`         // 豁免校验的 HTTP 方法
	TokenFormField  string   `mapstructure:"token-form-field" yaml:"token-form-field" json:"tokenFormField"`    // 表单字段名
	TokenCookieName string   `mapstructure:"token-cookie-name" yaml:"token-cookie-name" json:"tokenCookieName"` // Cookie 名称
	TokenLength     int      `mapstructure:"token-length" yaml:"token-length" json:"tokenLength"`               // Token 长度
	Secret          string   `mapstructure:"secret" yaml:"secret" json:"secret"`                                // Token 生成密钥
}

// Validation 安全校验规则配置
type Validation struct {
	HTTPMethods         []string `mapstructure:"http-methods" yaml:"http-methods" json:"httpMethods"`                         // 有效 HTTP 方法集合
	DangerousExtensions []string `mapstructure:"dangerous-extensions" yaml:"dangerous-extensions" json:"dangerousExtensions"` // 危险文件扩展名
	XSSPatterns         []string `mapstructure:"xss-patterns" yaml:"xss-patterns" json:"xssPatterns"`                         // XSS 检测模式
}

// CSP 内容安全策略配置
type CSP struct {
	Enabled bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"` // 是否启用CSP
	Mode    string `mapstructure:"mode" yaml:"mode" json:"mode"`          // CSP模式: strict, development, balanced, relaxed, api, custom
	Custom  string `mapstructure:"custom" yaml:"custom" json:"custom"`    // 自定义CSP策略（当mode=custom时使用）
}

// GetPolicy 获取CSP策略字符串
func (c *CSP) GetPolicy() string {
	if !c.Enabled {
		return ""
	}

	// 如果是自定义模式，直接返回自定义策略
	if c.Mode == "custom" && c.Custom != "" {
		return c.Custom
	}

	// 根据模式返回预定义策略
	switch c.Mode {
	case "strict":
		return "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self' ws: wss:; frame-src 'none'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"
	case "development":
		// 开发模式：允许所有HTTPS资源，方便开发调试
		return "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' https:; style-src 'self' 'unsafe-inline' https:; img-src 'self' data: blob: https:; font-src 'self' data: https:; connect-src 'self' ws: wss: http: https:; media-src 'self' https:; object-src 'none'; frame-src 'self'; base-uri 'self'; form-action 'self'"
	case "relaxed":
		return "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline' https:; img-src 'self' data: blob: https:; font-src 'self' data: https:; connect-src 'self' ws: wss: http: https:; media-src 'self' https:; object-src 'none'; frame-src 'self' https:; base-uri 'self'; form-action 'self'"
	case "api":
		return "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"
	default:
		// 默认使用平衡模式 - 生产环境推荐，允许HTTPS CDN但禁用unsafe-eval
		return "default-src 'self'; script-src 'self' https:; style-src 'self' 'unsafe-inline' https:; img-src 'self' data: https:; font-src 'self' data: https:; connect-src 'self' ws: wss:; media-src 'self' https:; object-src 'none'; frame-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'"
	}
}

// DefaultSecurity 创建默认安全配置
func DefaultSecurity() *Security {
	return &Security{
		ModuleName: "security",
		JWT: &JWT{
			Enabled:   false,
			Secret:    "jwt_secret_key_please_change_in_production",
			Expiry:    24,
			Issuer:    "go-rpc-gateway",
			Algorithm: "HS256",
		},
		Auth: &Auth{
			Enabled:     false,
			Type:        "bearer",
			HeaderName:  "Authorization",
			TokenPrefix: "Bearer ",
			Basic:       &BasicAuth{Users: []User{}},
			Bearer:      &BearerAuth{Tokens: []string{}},
			APIKey: &APIKeyAuth{
				Keys:       []string{},
				HeaderName: "X-API-Key",
				QueryParam: "api_key",
			},
			Custom: &CustomAuth{
				Headers: make(map[string]string),
			},
		},
		Protection: &Protection{
			Swagger: &ServiceProtection{
				Enabled:      false,
				AuthRequired: false,
				AuthType:     "basic",
				IPWhitelist:  []string{},
				RequireHTTPS: false,
			},
			PProf: &ServiceProtection{
				Enabled:      false,
				AuthRequired: true,
				AuthType:     "basic",
				IPWhitelist:  []string{"127.0.0.1", "::1"},
				RequireHTTPS: true,
				Username:     "admin",
				Password:     "",
			},
			Metrics: &ServiceProtection{
				Enabled:      false,
				AuthRequired: false,
				IPWhitelist:  []string{},
			},
			Health: &ServiceProtection{
				Enabled:      false,
				AuthRequired: false,
				IPWhitelist:  []string{},
			},
			API: &ServiceProtection{
				Enabled:      false,
				AuthRequired: false,
				IPWhitelist:  []string{},
			},
		},
		CSP: &CSP{
			Enabled: false,
			Mode:    "balanced", // 默认使用平衡模式
			Custom:  "",
		},
		CSRF:       DefaultCSRF(),
		Validation: DefaultValidation(),
	}
}

// DefaultCSRF 创建默认 CSRF 配置
func DefaultCSRF() *CSRF {
	return &CSRF{
		Enabled:         false,
		ExemptMethods:   []string{"GET", "HEAD", "OPTIONS", "TRACE"},
		TokenFormField:  "_csrf_token",
		TokenCookieName: "csrf_token",
		TokenLength:     32,
		Secret:          "csrf-secret",
	}
}

// DefaultValidation 创建默认安全校验规则
func DefaultValidation() *Validation {
	return &Validation{
		HTTPMethods: []string{"OPTIONS", "GET", "HEAD", "POST", "PUT", "PATCH", "TRACE", "DELETE"},
		DangerousExtensions: []string{
			".exe", ".bat", ".cmd", ".scr", ".pif", ".com",
			".js", ".vbs", ".ps1", ".sh", ".php", ".asp",
		},
		XSSPatterns: []string{
			"<script",
			"javascript:",
			"onload=",
			"onerror=",
			"onclick=",
			"onmouseover=",
		},
	}
}

// Get 返回配置接口
func (s *Security) Get() interface{} {
	return s
}

// Set 设置配置数据
func (s *Security) Set(data interface{}) {
	if cfg, ok := data.(*Security); ok {
		*s = *cfg
	}
}

// Clone 返回配置的副本
func (s *Security) Clone() internal.Configurable {
	var cloned Security
	if err := syncx.DeepCopy(&cloned, s); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Security{}
	}
	return &cloned
}

// Validate 验证配置
func (s *Security) Validate() error {
	return internal.ValidateStruct(s)
}

// WithModuleName 设置模块名称
func (s *Security) WithModuleName(moduleName string) *Security {
	s.ModuleName = moduleName
	return s
}

// WithJWT 设置JWT配置
func (s *Security) WithJWT(enabled bool, secret string, expiry int, issuer, algorithm string) *Security {
	s.JWT.Enabled = enabled
	s.JWT.Secret = secret
	s.JWT.Expiry = expiry
	s.JWT.Issuer = issuer
	s.JWT.Algorithm = algorithm
	return s
}

// WithAuth 设置认证配置
func (s *Security) WithAuth(enabled bool, authType, headerName, tokenPrefix string) *Security {
	s.Auth.Enabled = enabled
	s.Auth.Type = authType
	s.Auth.HeaderName = headerName
	s.Auth.TokenPrefix = tokenPrefix
	return s
}

// WithCSP 设置CSP配置
func (s *Security) WithCSP(enabled bool, mode, custom string) *Security {
	s.CSP.Enabled = enabled
	s.CSP.Mode = mode
	s.CSP.Custom = custom
	return s
}

// EnableJWT 启用JWT
func (s *Security) EnableJWT() *Security {
	s.JWT.Enabled = true
	return s
}

// EnableAuth 启用认证
func (s *Security) EnableAuth() *Security {
	s.Auth.Enabled = true
	return s
}

// EnableCSP 启用CSP
func (s *Security) EnableCSP() *Security {
	s.CSP.Enabled = true
	return s
}

// IsJWTEnabled 检查JWT是否启用
func (s *Security) IsJWTEnabled() bool {
	return s.JWT.Enabled
}

// IsAuthEnabled 检查认证是否启用
func (s *Security) IsAuthEnabled() bool {
	return s.Auth.Enabled
}

// IsCSPEnabled 检查CSP是否启用
func (s *Security) IsCSPEnabled() bool {
	return s.CSP.Enabled
}
