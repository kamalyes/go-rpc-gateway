/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 17:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-13 19:55:55
 * @FilePath: \go-rpc-gateway\config\swagger\swagger.go
 * @Description: Swagger配置模块 - 基于go-config设计模式
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"fmt"
	"strings"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// CDN 资源路径常量
const (
	// CDN 资源文件路径
	CDNPathCSS       = "/swagger-ui.css"
	CDNPathBundleJS  = "/swagger-ui-bundle.js"
	CDNPathPresetJS  = "/swagger-ui-standalone-preset.js"
	CDNPathFavicon32 = "/favicon-32x32.png"
	CDNPathFavicon16 = "/favicon-16x16.png"

	// CDN 基础 URL 模板
	CDNBaseURLUnpkg    = "https://unpkg.com/swagger-ui-dist@"
	CDNBaseURLJsDelivr = "https://cdn.jsdelivr.net/npm/swagger-ui-dist@"
)

// AuthType Swagger认证类型
type AuthType string

const (
	// AuthNone 无认证
	AuthNone AuthType = "NONE"
	// AuthBasic 基本认证
	AuthBasic AuthType = "BASIC"
	// AuthBearer Bearer Token认证
	AuthBearer AuthType = "BEARER"
	// AuthCustom 自定义认证
	AuthCustom AuthType = "CUSTOM"
)

// Contact Swagger联系信息
type Contact struct {
	Name  string `mapstructure:"name" yaml:"name" json:"name"`    // 联系人姓名
	URL   string `mapstructure:"url" yaml:"url" json:"url"`       // 联系URL
	Email string `mapstructure:"email" yaml:"email" json:"email"` // 联系邮箱
}

// License Swagger许可证信息
type License struct {
	Name string `mapstructure:"name" yaml:"name" json:"name"` // 许可证名称
	URL  string `mapstructure:"url" yaml:"url" json:"url"`    // 许可证URL
}

// AuthConfig Swagger认证配置
type AuthConfig struct {
	Enabled     bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`               // 是否启用认证
	Type        AuthType `mapstructure:"type" yaml:"type" json:"type"`                        // 认证类型
	Username    string   `mapstructure:"username" yaml:"username" json:"username"`            // 用户名（基本认证）
	Password    string   `mapstructure:"password" yaml:"password" json:"password"`            // 密码（基本认证）
	Token       string   `mapstructure:"token" yaml:"token" json:"token"`                     // Token（Bearer认证）
	HeaderName  string   `mapstructure:"header-name" yaml:"header-name" json:"headerName"`    // 自定义header名称
	HeaderValue string   `mapstructure:"header-value" yaml:"header-value" json:"headerValue"` // 自定义header值
}

// ServiceSpec 单个微服务Swagger规范配置
type ServiceSpec struct {
	Name        string   `mapstructure:"name" yaml:"name" json:"name"`                      // 服务名称
	Description string   `mapstructure:"description" yaml:"description" json:"description"` // 服务描述
	SpecPath    string   `mapstructure:"spec-path" yaml:"spec-path" json:"specPath"`        // Swagger规范文件路径
	URL         string   `mapstructure:"url" yaml:"url" json:"url"`                         // 远程Swagger文档URL
	Version     string   `mapstructure:"version" yaml:"version" json:"version"`             // 服务版本
	Enabled     bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`             // 是否启用
	BasePath    string   `mapstructure:"base-path" yaml:"base-path" json:"basePath"`        // API基础路径前缀
	Tags        []string `mapstructure:"tags" yaml:"tags" json:"tags"`                      // 服务标签
}

// DocumentPathSelector 独立文档中的接口选择器
type DocumentPathSelector struct {
	Path    string   `mapstructure:"path" yaml:"path" json:"path"`          // Swagger路径，如 /v1/open-platform/apps
	Methods []string `mapstructure:"methods" yaml:"methods" json:"methods"` // 可选的HTTP方法列表，留空表示该路径下全部方法
}

// DocumentSource 独立文档的数据源
type DocumentSource struct {
	Service string                  `mapstructure:"service" yaml:"service" json:"service"` // 来源服务名，对应 aggregate.services[].name
	Paths   []*DocumentPathSelector `mapstructure:"paths" yaml:"paths" json:"paths"`       // include 的兼容简写，留空表示该服务全部路径
	Include []*DocumentPathSelector `mapstructure:"include" yaml:"include" json:"include"` // 显式包含的路径列表，留空表示该服务全部路径
	Exclude []*DocumentPathSelector `mapstructure:"exclude" yaml:"exclude" json:"exclude"` // 需要排除的路径列表
}

// DocumentSpec 独立Swagger文档配置
type DocumentSpec struct {
	Name        string            `mapstructure:"name" yaml:"name" json:"name"`                      // 文档名称，同时用于路由标识
	Title       string            `mapstructure:"title" yaml:"title" json:"title"`                   // 文档标题
	Description string            `mapstructure:"description" yaml:"description" json:"description"` // 文档描述
	Version     string            `mapstructure:"version" yaml:"version" json:"version"`             // 文档版本
	Enabled     bool              `mapstructure:"enabled" yaml:"enabled" json:"enabled"`             // 是否启用
	Sources     []*DocumentSource `mapstructure:"sources" yaml:"sources" json:"sources"`             // 文档来源
}

// AggregateConfig 聚合Swagger配置
type AggregateConfig struct {
	Enabled                  bool            `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                                                        // 是否启用聚合
	Mode                     string          `mapstructure:"mode" yaml:"mode" json:"mode"`                                                                 // 聚合模式: merge|selector
	Services                 []*ServiceSpec  `mapstructure:"services" yaml:"services" json:"services"`                                                     // 微服务列表
	Documents                []*DocumentSpec `mapstructure:"documents" yaml:"documents" json:"documents"`                                                  // 独立文档列表
	UILayout                 string          `mapstructure:"ui-layout" yaml:"ui-layout" json:"uiLayout"`                                                   // UI布局: tabs|dropdown|list
	SharedDefinitionPrefixes []string        `mapstructure:"shared-definition-prefixes" yaml:"shared-definition-prefixes" json:"sharedDefinitionPrefixes"` // 共享定义前缀列表（不添加服务名前缀）
}

// Swagger Swagger配置结构
type Swagger struct {
	ModuleName  string           `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`  // 模块名称
	Enabled     bool             `mapstructure:"enabled" yaml:"enabled" json:"enabled"`             // 是否启用Swagger
	JSONPath    string           `mapstructure:"json-path" yaml:"json-path" json:"jsonPath"`        // Swagger JSON文件路径
	UIPath      string           `mapstructure:"ui-path" yaml:"ui-path" json:"uiPath"`              // Swagger UI路由路径
	YamlPath    string           `mapstructure:"yaml-path" yaml:"yaml-path" json:"yamlPath"`        // Swagger YAML文件路径
	SpecPath    string           `mapstructure:"spec-path" yaml:"spec-path" json:"specPath"`        // Swagger规范文件路径(自动检测格式)
	Title       string           `mapstructure:"title" yaml:"title" json:"title"`                   // 文档标题
	Description string           `mapstructure:"description" yaml:"description" json:"description"` // 文档描述
	Version     string           `mapstructure:"version" yaml:"version" json:"version"`             // Swagger版本
	Contact     *Contact         `mapstructure:"contact" yaml:"contact" json:"contact"`             // 联系信息
	License     *License         `mapstructure:"license" yaml:"license" json:"license"`             // 许可证信息
	Auth        *AuthConfig      `mapstructure:"auth" yaml:"auth" json:"auth"`                      // 认证配置
	Aggregate   *AggregateConfig `mapstructure:"aggregate" yaml:"aggregate" json:"aggregate"`       // 聚合配置
	HotReload   bool             `mapstructure:"hot-reload" yaml:"hot-reload" json:"hotReload"`     // 是否启用文件热重载

	// CDN 资源配置
	CDNVersion     string `mapstructure:"cdn-version" yaml:"cdn-version" json:"cdnVersion"`               // Swagger UI CDN版本
	CDNBaseURL     string `mapstructure:"cdn-base-url" yaml:"cdn-base-url" json:"cdnBaseURL"`             // CDN基础URL
	CDNCSSURL      string `mapstructure:"cdn-css-url" yaml:"cdn-css-url" json:"cdnCSSURL"`                // CSS文件URL
	CDNBundleJS    string `mapstructure:"cdn-bundle-js" yaml:"cdn-bundle-js" json:"cdnBundleJS"`          // Bundle JS URL
	CDNPresetJS    string `mapstructure:"cdn-preset-js" yaml:"cdn-preset-js" json:"cdnPresetJS"`          // Preset JS URL
	CDNFavicon32   string `mapstructure:"cdn-favicon32" yaml:"cdn-favicon32" json:"cdnFavicon32"`         // 32x32 Favicon URL
	CDNFavicon16   string `mapstructure:"cdn-favicon16" yaml:"cdn-favicon16" json:"cdnFavicon16"`         // 16x16 Favicon URL
	UseCDN         bool   `mapstructure:"use-cdn" yaml:"use-cdn" json:"useCDN"`                           // 是否使用CDN资源
	CDNFallbackURL string `mapstructure:"cdn-fallback-url" yaml:"cdn-fallback-url" json:"cdnFallbackURL"` // CDN备用URL
}

// Default 创建默认Swagger配置
func Default() *Swagger {
	cdnVersion := "5.30.2"
	cdnBaseURL := CDNBaseURLUnpkg + cdnVersion

	return &Swagger{
		ModuleName:  "swagger",
		Enabled:     false, // 默认不启用Swagger
		JSONPath:    "/swagger/doc.json",
		UIPath:      "/swagger",
		YamlPath:    "/swagger/doc.yaml",
		SpecPath:    "./docs/swagger.yaml", // 默认规范文件路径
		Title:       "API Documentation",
		Description: "API Documentation powered by Swagger UI",
		Version:     "1.0.0",
		Contact: &Contact{
			Name:  "API Support",
			URL:   "https://example.com/support",
			Email: "support@example.com",
		},
		License: &License{
			Name: "MIT",
			URL:  "https://opensource.org/licenses/MIT",
		},
		Auth: &AuthConfig{
			Type: AuthNone,
		},
		Aggregate: &AggregateConfig{
			Enabled:                  false,
			Mode:                     "merge",
			Services:                 []*ServiceSpec{},
			Documents:                []*DocumentSpec{},
			UILayout:                 "tabs",
			SharedDefinitionPrefixes: []string{"common", "rpcStatus", "protobuf", "google", "googleapis", "protoc"},
		},
		// CDN 默认配置
		UseCDN:         true,
		CDNVersion:     cdnVersion,
		CDNBaseURL:     cdnBaseURL,
		CDNCSSURL:      cdnBaseURL + CDNPathCSS,
		CDNBundleJS:    cdnBaseURL + CDNPathBundleJS,
		CDNPresetJS:    cdnBaseURL + CDNPathPresetJS,
		CDNFavicon32:   cdnBaseURL + CDNPathFavicon32,
		CDNFavicon16:   cdnBaseURL + CDNPathFavicon16,
		CDNFallbackURL: CDNBaseURLJsDelivr + cdnVersion,
	}
}

// WithModuleName 设置模块名称
func (c *Swagger) WithModuleName(moduleName string) *Swagger {
	c.ModuleName = moduleName
	return c
}

// WithEnabled 设置是否启用Swagger
func (c *Swagger) WithEnabled(enabled bool) *Swagger {
	c.Enabled = enabled
	return c
}

// WithJSONPath 设置Swagger JSON文件路径
func (c *Swagger) WithJSONPath(path string) *Swagger {
	c.JSONPath = path
	return c
}

// WithUIPath 设置Swagger UI路由路径
func (c *Swagger) WithUIPath(path string) *Swagger {
	c.UIPath = path
	return c
}

// WithYamlPath 设置Swagger YAML文件路径
func (c *Swagger) WithYamlPath(path string) *Swagger {
	c.YamlPath = path
	return c
}

// WithSpecPath 设置Swagger规范文件路径
func (c *Swagger) WithSpecPath(path string) *Swagger {
	c.SpecPath = path
	return c
}

// WithTitle 设置文档标题
func (c *Swagger) WithTitle(title string) *Swagger {
	c.Title = title
	return c
}

// WithDescription 设置文档描述
func (c *Swagger) WithDescription(description string) *Swagger {
	c.Description = description
	return c
}

// WithAuth 设置认证配置
func (c *Swagger) WithAuth(auth *AuthConfig) *Swagger {
	c.Auth = auth
	return c
}

// WithVersion 设置Swagger版本
func (c *Swagger) WithVersion(version string) *Swagger {
	c.Version = version
	return c
}

// WithContact 设置联系信息
func (c *Swagger) WithContact(contact *Contact) *Swagger {
	c.Contact = contact
	return c
}

// WithLicense 设置许可证信息
func (c *Swagger) WithLicense(license *License) *Swagger {
	c.License = license
	return c
}

// WithBasicAuth 设置基本认证
func (c *Swagger) WithBasicAuth(username, password string) *Swagger {
	if c.Auth == nil {
		c.Auth = &AuthConfig{}
	}
	c.Auth.Type = AuthBasic
	c.Auth.Username = username
	c.Auth.Password = password
	return c
}

// WithBearerAuth 设置Bearer Token认证
func (c *Swagger) WithBearerAuth(token string) *Swagger {
	if c.Auth == nil {
		c.Auth = &AuthConfig{}
	}
	c.Auth.Type = AuthBearer
	c.Auth.Token = token
	return c
}

// WithAggregate 设置聚合配置
func (c *Swagger) WithAggregate(aggregate *AggregateConfig) *Swagger {
	c.Aggregate = aggregate
	return c
}

// WithAggregateServices 设置聚合服务列表
func (c *Swagger) WithAggregateServices(services []*ServiceSpec) *Swagger {
	if c.Aggregate == nil {
		c.Aggregate = &AggregateConfig{
			Enabled:  true,
			Mode:     "merge",
			UILayout: "tabs",
		}
	}
	c.Aggregate.Services = services
	c.Aggregate.Enabled = len(services) > 0
	return c
}

// AddAggregateService 添加单个聚合服务
func (c *Swagger) AddAggregateService(service *ServiceSpec) *Swagger {
	if c.Aggregate == nil {
		c.Aggregate = &AggregateConfig{
			Enabled:   true,
			Mode:      "merge",
			UILayout:  "tabs",
			Services:  []*ServiceSpec{},
			Documents: []*DocumentSpec{},
		}
	}
	c.Aggregate.Services = append(c.Aggregate.Services, service)
	c.Aggregate.Enabled = true
	return c
}

// WithAggregateDocuments 设置独立文档列表
func (c *Swagger) WithAggregateDocuments(documents []*DocumentSpec) *Swagger {
	if c.Aggregate == nil {
		c.Aggregate = &AggregateConfig{
			Mode:      "merge",
			UILayout:  "tabs",
			Services:  []*ServiceSpec{},
			Documents: []*DocumentSpec{},
		}
	}
	c.Aggregate.Documents = documents
	return c
}

// AddAggregateDocument 添加单个独立文档
func (c *Swagger) AddAggregateDocument(document *DocumentSpec) *Swagger {
	if c.Aggregate == nil {
		c.Aggregate = &AggregateConfig{
			Mode:      "merge",
			UILayout:  "tabs",
			Services:  []*ServiceSpec{},
			Documents: []*DocumentSpec{},
		}
	}
	c.Aggregate.Documents = append(c.Aggregate.Documents, document)
	return c
}

// WithAggregateMode 设置聚合模式
func (c *Swagger) WithAggregateMode(mode string) *Swagger {
	if c.Aggregate == nil {
		c.Aggregate = &AggregateConfig{
			Enabled:   false,
			UILayout:  "tabs",
			Services:  []*ServiceSpec{},
			Documents: []*DocumentSpec{},
		}
	}
	c.Aggregate.Mode = mode
	return c
}

// WithAggregateUILayout 设置UI布局
func (c *Swagger) WithAggregateUILayout(layout string) *Swagger {
	if c.Aggregate == nil {
		c.Aggregate = &AggregateConfig{
			Enabled:   false,
			Mode:      "merge",
			Services:  []*ServiceSpec{},
			Documents: []*DocumentSpec{},
		}
	}
	c.Aggregate.UILayout = layout
	return c
}

// EnableAggregate 启用聚合功能
func (c *Swagger) EnableAggregate() *Swagger {
	if c.Aggregate == nil {
		c.Aggregate = &AggregateConfig{
			Mode:                     "merge",
			UILayout:                 "tabs",
			Services:                 []*ServiceSpec{},
			Documents:                []*DocumentSpec{},
			SharedDefinitionPrefixes: []string{"common", "api", "protobuf", "google"},
		}
	}
	c.Aggregate.Enabled = true
	return c
}

// DisableAggregate 禁用聚合功能
func (c *Swagger) DisableAggregate() *Swagger {
	if c.Aggregate != nil {
		c.Aggregate.Enabled = false
	}
	return c
}

// WithSharedDefinitionPrefixes 设置共享定义前缀列表
func (c *Swagger) WithSharedDefinitionPrefixes(prefixes []string) *Swagger {
	c.Aggregate.SharedDefinitionPrefixes = prefixes
	return c
}

// AddSharedDefinitionPrefix 添加单个共享定义前缀
func (c *Swagger) AddSharedDefinitionPrefix(prefix string) *Swagger {
	c.Aggregate.SharedDefinitionPrefixes = append(c.Aggregate.SharedDefinitionPrefixes, prefix)
	return c
}

// GetSharedDefinitionPrefixes 获取共享定义前缀列表
func (c *Swagger) GetSharedDefinitionPrefixes() []string {
	if c.Aggregate == nil || len(c.Aggregate.SharedDefinitionPrefixes) == 0 {
		return []string{"common", "api", "protobuf", "google"} // 默认值
	}
	return c.Aggregate.SharedDefinitionPrefixes
}

// WithCDNVersion 设置CDN版本
func (c *Swagger) WithCDNVersion(version string) *Swagger {
	c.CDNVersion = version
	// 自动更新相关的CDN URLs
	c.CDNBaseURL = CDNBaseURLUnpkg + version
	c.CDNCSSURL = c.CDNBaseURL + CDNPathCSS
	c.CDNBundleJS = c.CDNBaseURL + CDNPathBundleJS
	c.CDNPresetJS = c.CDNBaseURL + CDNPathPresetJS
	c.CDNFavicon32 = c.CDNBaseURL + CDNPathFavicon32
	c.CDNFavicon16 = c.CDNBaseURL + CDNPathFavicon16
	c.CDNFallbackURL = CDNBaseURLJsDelivr + version
	return c
}

// WithCDNBaseURL 设置CDN基础URL
func (c *Swagger) WithCDNBaseURL(baseURL string) *Swagger {
	c.CDNBaseURL = baseURL
	c.CDNCSSURL = baseURL + CDNPathCSS
	c.CDNBundleJS = baseURL + CDNPathBundleJS
	c.CDNPresetJS = baseURL + CDNPathPresetJS
	c.CDNFavicon32 = baseURL + CDNPathFavicon32
	c.CDNFavicon16 = baseURL + CDNPathFavicon16
	return c
}

// WithCustomCDNURLs 自定义CDN资源URLs
func (c *Swagger) WithCustomCDNURLs(cssURL, bundleJS, presetJS, favicon32, favicon16 string) *Swagger {
	c.CDNCSSURL = cssURL
	c.CDNBundleJS = bundleJS
	c.CDNPresetJS = presetJS
	c.CDNFavicon32 = favicon32
	c.CDNFavicon16 = favicon16
	return c
}

// WithCDNFallbackURL 设置CDN备用URL
func (c *Swagger) WithCDNFallbackURL(fallbackURL string) *Swagger {
	c.CDNFallbackURL = fallbackURL
	return c
}

// EnableCDN 启用CDN资源
func (c *Swagger) EnableCDN() *Swagger {
	c.UseCDN = true
	return c
}

// DisableCDN 禁用CDN资源（使用本地资源）
func (c *Swagger) DisableCDN() *Swagger {
	c.UseCDN = false
	return c
}

// GetCDNCSSURL 获取CSS URL（支持备用）
func (c *Swagger) GetCDNCSSURL() string {
	if c.CDNCSSURL != "" {
		return c.CDNCSSURL
	}
	if c.CDNFallbackURL != "" {
		return c.CDNFallbackURL + CDNPathCSS
	}
	return c.CDNBaseURL + CDNPathCSS
}

// GetCDNBundleJS 获取Bundle JS URL（支持备用）
func (c *Swagger) GetCDNBundleJS() string {
	if c.CDNBundleJS != "" {
		return c.CDNBundleJS
	}
	if c.CDNFallbackURL != "" {
		return c.CDNFallbackURL + CDNPathBundleJS
	}
	return c.CDNBaseURL + CDNPathBundleJS
}

// GetCDNPresetJS 获取Preset JS URL（支持备用）
func (c *Swagger) GetCDNPresetJS() string {
	if c.CDNPresetJS != "" {
		return c.CDNPresetJS
	}
	if c.CDNFallbackURL != "" {
		return c.CDNFallbackURL + CDNPathPresetJS
	}
	return c.CDNBaseURL + CDNPathPresetJS
}

// GetCDNFavicon32 获取32x32 Favicon URL（支持备用）
func (c *Swagger) GetCDNFavicon32() string {
	if c.CDNFavicon32 != "" {
		return c.CDNFavicon32
	}
	if c.CDNFallbackURL != "" {
		return c.CDNFallbackURL + CDNPathFavicon32
	}
	return c.CDNBaseURL + CDNPathFavicon32
}

// GetCDNFavicon16 获取16x16 Favicon URL（支持备用）
func (c *Swagger) GetCDNFavicon16() string {
	if c.CDNFavicon16 != "" {
		return c.CDNFavicon16
	}
	if c.CDNFallbackURL != "" {
		return c.CDNFallbackURL + CDNPathFavicon16
	}
	return c.CDNBaseURL + CDNPathFavicon16
}

// IsAggregateEnabled 检查聚合功能是否启用
func (c *Swagger) IsAggregateEnabled() bool {
	return c.Aggregate != nil && c.Aggregate.Enabled
}

// WithDefaults 使用默认配置填充未设置的字段
func (c *Swagger) WithDefaults() *Swagger {
	defaults := Default()

	if c.JSONPath == "" {
		c.JSONPath = defaults.JSONPath
	}
	if c.UIPath == "" {
		c.UIPath = defaults.UIPath
	}
	if c.Title == "" {
		c.Title = defaults.Title
	}
	if c.Description == "" {
		c.Description = defaults.Description
	}
	if c.Auth == nil {
		c.Auth = defaults.Auth
	}
	if c.Version == "" {
		c.Version = defaults.Version
	}

	return c
}

// WithBasePath 设置基础路径（同时设置JSON和UI路径）
func (c *Swagger) WithBasePath(basePath string) *Swagger {
	c.JSONPath = basePath + "/doc.json"
	c.UIPath = basePath + "/*any"
	return c
}

// WithCustomPaths 自定义JSON和UI路径
func (c *Swagger) WithCustomPaths(jsonPath, uiPath string) *Swagger {
	c.JSONPath = jsonPath
	c.UIPath = uiPath
	return c
}

// WithDocumentInfo 设置文档信息（标题和描述）
func (c *Swagger) WithDocumentInfo(title, description string) *Swagger {
	c.Title = title
	c.Description = description
	return c
}

// Clone 克隆配置对象
func (c *Swagger) Clone() internal.Configurable {
	var cloned Swagger
	if err := syncx.DeepCopy(&cloned, c); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Swagger{}
	}
	return &cloned
}

// Get 返回 Swagger 配置的所有字段
func (c *Swagger) Get() interface{} {
	return c
}

// Set 更新 Swagger 配置的字段
func (c *Swagger) Set(data interface{}) {
	if configData, ok := data.(*Swagger); ok {
		c.ModuleName = configData.ModuleName
		c.Enabled = configData.Enabled
		c.JSONPath = configData.JSONPath
		c.UIPath = configData.UIPath
		c.Title = configData.Title
		c.Description = configData.Description
		c.Version = configData.Version
		c.Contact = configData.Contact
		c.License = configData.License
		c.Auth = configData.Auth
		c.Aggregate = configData.Aggregate
	}
}

// Validate 检查 Swagger 配置的有效性
func (c *Swagger) Validate() error {
	return internal.ValidateStruct(c)
}

// Reset 重置为默认配置
func (c *Swagger) Reset() *Swagger {
	defaults := Default()
	c.ModuleName = defaults.ModuleName
	c.Enabled = defaults.Enabled
	c.JSONPath = defaults.JSONPath
	c.UIPath = defaults.UIPath
	c.Title = defaults.Title
	c.Description = defaults.Description
	c.Version = defaults.Version
	c.Contact = defaults.Contact
	c.License = defaults.License
	c.Auth = defaults.Auth
	c.Aggregate = defaults.Aggregate
	return c
}

// Disable 禁用Swagger
func (c *Swagger) Disable() *Swagger {
	c.Enabled = false
	return c
}

// Enable 启用Swagger
func (c *Swagger) Enable() *Swagger {
	c.Enabled = true
	return c
}

// IsEnabled 检查Swagger是否启用
func (c *Swagger) IsEnabled() bool {
	return c.Enabled
}

// NormalizeAuthType 将认证类型归一化为标准常量
// YAML/JSON 配置里写 basic、Bearer、CUSTOM 都应当生效：键名由 FlexibleMatchName 兼容各种
// 命名风格，但取值不会被归一化，不处理的话 type: basic 会既不等于 AuthNone（于是判定为已启用）
// 又匹配不上任何分支（于是拒绝所有请求），表现为文档整个打不开且看不出原因
// 空值视为未配置，返回 AuthNone；无法识别的取值原样返回，交由上层按「失败即锁死」处理
//
// [EN] NormalizeAuthType canonicalizes an auth type constant. Writing basic, Bearer or CUSTOM in
// YAML/JSON must work: FlexibleMatchName already tolerates any key naming style, but values are
// never normalized, and without this a `type: basic` is neither equal to AuthNone (so auth counts
// as enabled) nor matches any branch (so every request is denied) — the docs simply stop opening
// with no visible cause. An empty value means unconfigured and maps to AuthNone; an unrecognizable
// value is returned as-is so the caller can apply its fail-closed policy
func NormalizeAuthType(authType AuthType) AuthType {
	switch AuthType(strings.ToUpper(strings.TrimSpace(string(authType)))) {
	case "":
		return AuthNone
	case AuthNone:
		return AuthNone
	case AuthBasic:
		return AuthBasic
	case AuthBearer:
		return AuthBearer
	case AuthCustom:
		return AuthCustom
	default:
		return authType
	}
}

// IsAuthEnabled 检查是否启用了认证
// Enabled 与 Type 两个信号里任一表达了「要加锁」就加锁，取的是安全侧而非直觉侧：
//   - enabled: false 配上 type: basic，仍然加锁。只看 Enabled 的话，
//     WithBasicAuth 这类不设 Enabled 的构造方式会静默失去锁，是从「锁住」滑向「敞开」
//   - enabled: true 却没写 type，也算加锁。此时 GetAuthType 为 AuthNone，
//     凭据校验匹配不到任何分支，由 authorize 按「失败即锁死」拒绝全部请求并告警，
//     而不是当成「没配认证」放行
//
// mapstructure 分不清「没写 enabled」和「写了 enabled: false」（除非改成 *bool，那是 API 破坏），
// 所以不能把 Enabled 当作必需的开关来读
//
// [EN] IsAuthEnabled reports whether authentication is enabled. The docs are locked when either
// Enabled or Type signals an intent to lock, which is the safe reading rather than the intuitive
// one:
//   - enabled: false together with type: basic still locks. Honoring Enabled alone would make
//     constructors like WithBasicAuth, which never set it, silently lose the lock — a slide from
//     locked towards wide open
//   - enabled: true without a type also locks. GetAuthType is then AuthNone, no credential branch
//     matches, and authorize denies every request under its fail-closed policy with a warning,
//     instead of treating it as "auth not configured" and letting requests through
//
// mapstructure cannot tell "enabled absent" from "enabled: false" (short of making it a *bool,
// which breaks the API), so Enabled must not be read as a required switch
func (c *Swagger) IsAuthEnabled() bool {
	if c.Auth == nil {
		return false
	}
	return c.Auth.Enabled || NormalizeAuthType(c.Auth.Type) != AuthNone
}

// GetAuthType 获取认证类型（已归一化）
// [EN] GetAuthType returns the normalized auth type
func (c *Swagger) GetAuthType() AuthType {
	if c.Auth == nil {
		return AuthNone
	}
	return NormalizeAuthType(c.Auth.Type)
}

// GetFullUIPath 获取完整的UI路径
func (c *Swagger) GetFullUIPath() string {
	if c.UIPath == "" {
		return "/swagger/*any"
	}
	return c.UIPath
}

// GetFullJSONPath 获取完整的JSON路径
func (c *Swagger) GetFullJSONPath() string {
	if c.JSONPath == "" {
		return "/swagger/doc.json"
	}
	return c.JSONPath
}

// ValidateAuth 验证认证配置
func (c *Swagger) ValidateAuth() error {
	if c.Auth == nil || !c.IsAuthEnabled() {
		return nil
	}

	switch c.GetAuthType() {
	case AuthBasic:
		if c.Auth.Username == "" || c.Auth.Password == "" {
			return fmt.Errorf("basic auth requires username and password")
		}
	case AuthBearer:
		if c.Auth.Token == "" {
			return fmt.Errorf("bearer auth requires token")
		}
	case AuthCustom:
		if c.Auth.HeaderName == "" || c.Auth.HeaderValue == "" {
			return fmt.Errorf("custom auth requires header name and value")
		}
	default:
		return fmt.Errorf("unsupported auth type: %s", c.Auth.Type)
	}

	return nil
}

// NewServiceSpec 创建新的服务规范配置
func NewServiceSpec(name, description, specPath string) *ServiceSpec {
	return &ServiceSpec{
		Name:        name,
		Description: description,
		SpecPath:    specPath,
		Enabled:     true,
		Version:     "1.0.0",
		Tags:        []string{},
	}
}

// NewRemoteServiceSpec 创建远程服务规范配置
func NewRemoteServiceSpec(name, description, url string) *ServiceSpec {
	return &ServiceSpec{
		Name:        name,
		Description: description,
		URL:         url,
		Enabled:     true,
		Version:     "1.0.0",
		Tags:        []string{},
	}
}

// NewDocumentSpec 创建新的独立Swagger文档配置
func NewDocumentSpec(name string) *DocumentSpec {
	return &DocumentSpec{
		Name:    name,
		Title:   name,
		Version: "1.0.0",
		Enabled: true,
		Sources: []*DocumentSource{},
	}
}

// NewDocumentSource 创建新的独立文档来源配置
func NewDocumentSource(service string) *DocumentSource {
	return &DocumentSource{
		Service: service,
		Paths:   []*DocumentPathSelector{},
		Include: []*DocumentPathSelector{},
		Exclude: []*DocumentPathSelector{},
	}
}

// WithName 设置服务名称
func (s *ServiceSpec) WithName(name string) *ServiceSpec {
	s.Name = name
	return s
}

// WithDescription 设置服务描述
func (s *ServiceSpec) WithDescription(description string) *ServiceSpec {
	s.Description = description
	return s
}

// WithVersion 设置服务版本
func (s *ServiceSpec) WithVersion(version string) *ServiceSpec {
	s.Version = version
	return s
}

// WithBasePath 设置API基础路径
func (s *ServiceSpec) WithBasePath(basePath string) *ServiceSpec {
	s.BasePath = basePath
	return s
}

// WithTags 设置服务标签
func (s *ServiceSpec) WithTags(tags []string) *ServiceSpec {
	s.Tags = tags
	return s
}

// AddTag 添加单个标签
func (s *ServiceSpec) AddTag(tag string) *ServiceSpec {
	if s.Tags == nil {
		s.Tags = []string{}
	}
	s.Tags = append(s.Tags, tag)
	return s
}

// Enable 启用服务
func (s *ServiceSpec) Enable() *ServiceSpec {
	s.Enabled = true
	return s
}

// Disable 禁用服务
func (s *ServiceSpec) Disable() *ServiceSpec {
	s.Enabled = false
	return s
}

// WithTitle 设置文档标题
func (d *DocumentSpec) WithTitle(title string) *DocumentSpec {
	d.Title = title
	return d
}

// WithDescription 设置文档描述
func (d *DocumentSpec) WithDescription(description string) *DocumentSpec {
	d.Description = description
	return d
}

// WithVersion 设置文档版本
func (d *DocumentSpec) WithVersion(version string) *DocumentSpec {
	d.Version = version
	return d
}

// AddSource 添加文档来源
func (d *DocumentSpec) AddSource(source *DocumentSource) *DocumentSpec {
	if d.Sources == nil {
		d.Sources = []*DocumentSource{}
	}
	d.Sources = append(d.Sources, source)
	return d
}

// Enable 启用独立文档
func (d *DocumentSpec) Enable() *DocumentSpec {
	d.Enabled = true
	return d
}

// Disable 禁用独立文档
func (d *DocumentSpec) Disable() *DocumentSpec {
	d.Enabled = false
	return d
}

// AddIncludePath 添加包含路径选择器
func (d *DocumentSource) AddIncludePath(path string, methods ...string) *DocumentSource {
	if d.Include == nil {
		d.Include = []*DocumentPathSelector{}
	}
	d.Include = append(d.Include, &DocumentPathSelector{
		Path:    path,
		Methods: methods,
	})
	return d
}

// AddExcludePath 添加排除路径选择器
func (d *DocumentSource) AddExcludePath(path string, methods ...string) *DocumentSource {
	if d.Exclude == nil {
		d.Exclude = []*DocumentPathSelector{}
	}
	d.Exclude = append(d.Exclude, &DocumentPathSelector{
		Path:    path,
		Methods: methods,
	})
	return d
}

// GetEffectiveInclude 获取生效的 include 选择器，优先使用 include，其次兼容 paths
func (d *DocumentSource) GetEffectiveInclude() []*DocumentPathSelector {
	if d == nil {
		return nil
	}
	if len(d.Include) > 0 {
		return d.Include
	}
	return d.Paths
}
