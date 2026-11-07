/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-11 18:00:00
 * @FilePath: \go-rpc-gateway\config\middleware\i18n.go
 * @Description: 国际化中间件配置模块
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"maps"
	"slices"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/stringx"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// MessageLoader 消息加载器接口
type MessageLoader interface {
	LoadMessages(language string) (map[string]string, error)
}

// MappingType 映射类型
type MappingType string

const (
	// LegacyMapping 遗留映射
	LegacyMapping MappingType = "legacy"
	// StandardMapping 标准映射
	StandardMapping MappingType = "standard"
)

// String 返回映射类型的字符串表示
func (m MappingType) String() string {
	return string(m)
}

// DetectionType 语言检测类型
type DetectionType string

const (
	// DetectionHeader 从 HTTP 头检测
	DetectionHeader DetectionType = "header"
	// DetectionQuery 从查询参数检测
	DetectionQuery DetectionType = "query"
	// DetectionCookie 从 Cookie 检测
	DetectionCookie DetectionType = "cookie"
	// DetectionDefault 使用默认语言
	DetectionDefault DetectionType = "default"
)

// String 返回检测类型的字符串表示
func (d DetectionType) String() string {
	return string(d)
}

// I18N 国际化中间件配置
type I18N struct {
	ModuleName            string            `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`                                    // 模块名称
	Enabled               bool              `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                                               // 是否启用国际化
	DefaultLanguage       string            `mapstructure:"default-language" yaml:"default-language" json:"defaultLanguage"`                     // 默认语言
	SupportedLanguages    []string          `mapstructure:"supported-languages" yaml:"supported-languages" json:"supportedLanguages"`            // 支持的语言
	LanguageMapping       map[string]string `mapstructure:"language-mapping" yaml:"language-mapping" json:"languageMapping"`                     // 语言映射关系（标准映射，如 zh-cn -> zh）
	LegacyLanguageMapping map[string]string `mapstructure:"legacy-language-mapping" yaml:"legacy-language-mapping" json:"legacyLanguageMapping"` // 遗留语言映射（将之前使用的错误代码映射到正确代码）
	ResolutionOrder       []MappingType     `mapstructure:"resolution-order" yaml:"resolution-order" json:"resolutionOrder"`                     // 语言解析顺序（legacy, standard），可自定义顺序和组合
	DetectionOrder        []DetectionType   `mapstructure:"detection-order" yaml:"detection-order" json:"detectionOrder"`                        // 语言检测顺序
	LanguageParam         string            `mapstructure:"language-param" yaml:"language-param" json:"languageParam"`                           // 语言参数名称（用于query和cookie）
	LanguageHeader        string            `mapstructure:"language-header" yaml:"language-header" json:"languageHeader"`                        // 语言头名称
	MessagesPath          string            `mapstructure:"messages-path" yaml:"messages-path" json:"messagesPath"`                              // 消息文件路径
	CustomMessagePaths    map[string]string `mapstructure:"custom-message-paths" yaml:"custom-message-paths" json:"customMessagePaths"`          // 自定义消息文件路径映射
	EnableFallback        bool              `mapstructure:"enable-fallback" yaml:"enable-fallback" json:"enableFallback"`                        // 是否启用回退到默认语言
	MessageLoader         MessageLoader     `mapstructure:"-" yaml:"-" json:""`                                                                  // 自定义消息加载器
	CookieName            string            `mapstructure:"cookie-name" yaml:"cookie-name" json:"cookieName"`                                    // Cookie名称
}

// DefaultI18N 创建默认国际化配置
func DefaultI18N() *I18N {
	return &I18N{
		ModuleName:         "i18n",
		Enabled:            false,
		DefaultLanguage:    "en",
		SupportedLanguages: []string{"en", "zh"},
		LanguageMapping: map[string]string{
			"zh-cn": "zh",
			"en-us": "en",
		},
		LegacyLanguageMapping: make(map[string]string),
		ResolutionOrder:       []MappingType{LegacyMapping, StandardMapping},                                       // 默认顺序：优先遗留映射（向后兼容）
		DetectionOrder:        []DetectionType{DetectionHeader, DetectionQuery, DetectionCookie, DetectionDefault}, // 默认检测顺序
		LanguageParam:         "lang",
		LanguageHeader:        "Accept-Language",
		MessagesPath:          "./locales",
		CustomMessagePaths:    make(map[string]string),
		EnableFallback:        true,
		CookieName:            "language",
	}
}

// Get 返回配置接口
func (i *I18N) Get() any {
	return i
}

// Set 设置配置数据
func (i *I18N) Set(data any) {
	if cfg, ok := data.(*I18N); ok {
		*i = *cfg
	}
}

// Clone 返回配置的副本
func (i *I18N) Clone() internal.Configurable {
	var cloned I18N
	if err := syncx.DeepCopy(&cloned, i); err != nil {
		// 如果深拷贝失败，返回空配置
		return &I18N{}
	}
	return &cloned
}

// Validate 验证配置
func (i *I18N) Validate() error {
	return internal.ValidateStruct(i)
}

// WithDefaultLanguage 设置默认语言
func (i *I18N) WithDefaultLanguage(defaultLanguage string) *I18N {
	i.DefaultLanguage = defaultLanguage
	return i
}

// WithSupportedLanguages 设置支持的语言
func (i *I18N) WithSupportedLanguages(supportedLanguages []string) *I18N {
	i.SupportedLanguages = supportedLanguages
	return i
}

// WithMessagesPath 设置消息文件路径
func (i *I18N) WithMessagesPath(messagesPath string) *I18N {
	i.MessagesPath = messagesPath
	return i
}

// WithLanguageHeader 设置语言头名称
func (i *I18N) WithLanguageHeader(languageHeader string) *I18N {
	i.LanguageHeader = languageHeader
	return i
}

// WithLanguageParam 设置语言参数名称
func (i *I18N) WithLanguageParam(languageParam string) *I18N {
	i.LanguageParam = languageParam
	return i
}

// WithCookieName 设置Cookie名称
func (i *I18N) WithCookieName(cookieName string) *I18N {
	i.CookieName = cookieName
	return i
}

// WithResolutionOrder 设置语言解析顺序
func (i *I18N) WithResolutionOrder(order []MappingType) *I18N {
	i.ResolutionOrder = order
	return i
}

// Enable 启用国际化中间件
func (i *I18N) Enable() *I18N {
	i.Enabled = true
	return i
}

// Disable 禁用国际化中间件
func (i *I18N) Disable() *I18N {
	i.Enabled = false
	return i
}

// IsEnabled 检查是否启用
func (i *I18N) IsEnabled() bool {
	return i.Enabled
}

// AddLanguageMapping 添加语言映射关系（非标准语言代码映射到标准代码）
// 例如：AddLanguageMapping("zh-tw", "zh") 将繁体中文映射到简体中文
func (i *I18N) AddLanguageMapping(from, to string) *I18N {
	i.LanguageMapping[from] = to
	return i
}

// AddLanguageMappings 批量添加语言映射关系
func (i *I18N) AddLanguageMappings(mappings map[string]string) *I18N {
	maps.Copy(i.LanguageMapping, mappings)
	return i
}

// GetMappedLanguage 获取映射后的语言代码，如果没有映射则返回原值
func (i *I18N) GetMappedLanguage(lang string) string {
	if mapped, ok := i.LanguageMapping[lang]; ok {
		return mapped
	}
	return lang
}

// AddLegacyLanguageMapping 添加遗留语言映射关系（将之前使用的错误代码映射到正确代码）
// 例如：AddLegacyLanguageMapping("cn", "zh") 将错误的 "cn" 映射到正确的 "zh"
func (i *I18N) AddLegacyLanguageMapping(from, to string) *I18N {
	i.LegacyLanguageMapping[from] = to
	return i
}

// AddLegacyLanguageMappings 批量添加遗留语言映射关系
func (i *I18N) AddLegacyLanguageMappings(mappings map[string]string) *I18N {
	maps.Copy(i.LegacyLanguageMapping, mappings)
	return i
}

// ResolveLanguage 解析语言代码（根据配置的解析顺序进行映射，最后验证是否支持）
//
// 处理流程：
// 1. 按照 ResolutionOrder 配置的顺序依次应用映射（legacy -> standard）
// 2. 验证映射后的语言是否在支持列表中
// 3. 如果不支持则返回默认语言
//
// 参数：
//   - lang: 原始语言代码（如 "cn", "zh-cn", "en-us"）
//
// 返回：
//   - 最终的标准语言代码（如 "zh", "en"）
func (i *I18N) ResolveLanguage(lang string) string {
	// 按照配置的顺序依次应用映射
	for _, resolverType := range i.ResolutionOrder {
		lang = i.applyMapping(lang, resolverType)
	}

	// 验证是否在支持的语言列表中
	if i.IsSupportedLanguage(lang) {
		return lang
	}

	// 不支持则返回默认语言
	return i.DefaultLanguage
}

// applyMapping 根据映射类型应用对应的语言映射规则
// 支持 LegacyMapping（遗留映射）和 StandardMapping（标准映射）两种类型
func (i *I18N) applyMapping(lang string, mappingType MappingType) string {
	switch mappingType {
	case LegacyMapping:
		if mapped, ok := i.LegacyLanguageMapping[lang]; ok {
			return mapped
		}
	case StandardMapping:
		if mapped, ok := i.LanguageMapping[lang]; ok {
			return mapped
		}
	}
	return lang
}

// ParseAcceptLanguage 解析 Accept-Language 头（支持多语言和权重）
//
// 处理流程：
// 1. 解析语言列表，提取语言代码（忽略权重参数）
// 2. 按优先级顺序依次尝试解析每个语言
// 3. 返回第一个支持的语言，如果都不支持则返回默认语言
//
// 示例：
//   - "zh-CN,zh;q=0.9,en;q=0.8" -> 返回 "zh"（如果支持）
//   - "fr,de;q=0.8" -> 返回默认语言（如果都不支持）
func (i *I18N) ParseAcceptLanguage(acceptLang string) string {
	if acceptLang == "" {
		return i.DefaultLanguage
	}

	// 按逗号分割多个语言
	languages := i.parseLanguageList(acceptLang)

	// 依次尝试解析每个语言
	for _, lang := range languages {
		resolved := i.ResolveLanguage(lang)
		// 只有当 resolved 是 lang 的有效映射结果时才返回
		// 即 lang 本身受支持，或 lang 通过映射解析到了受支持的语言
		// 排除 ResolveLanguage 回退到默认语言的情况（lang 不支持且无映射）
		if resolved != i.DefaultLanguage {
			// 解析到了非默认的支持语言，直接返回
			return resolved
		}
		// resolved == DefaultLanguage，需要区分是 lang 本身就是默认语言，还是回退的
		if i.IsSupportedLanguage(lang) {
			// lang 本身受支持（如 lang == "en"），返回
			return resolved
		}
		// lang 不受支持且无映射，ResolveLanguage 回退到了默认语言，继续尝试下一个
	}

	return i.DefaultLanguage
}

// parseLanguageList 解析语言列表，提取语言代码并忽略权重参数
// 将 "zh-CN,zh;q=0.9,en;q=0.8" 解析为 ["zh-cn", "zh", "en"]（转为小写）
func (i *I18N) parseLanguageList(acceptLang string) []string {
	// 按逗号分割并去除空格
	parts := stringx.SplitTrim(acceptLang, ",")

	languages := make([]string, 0, len(parts))
	for _, part := range parts {
		// 移除权重参数（;q=0.9）
		lang := part
		if idx := stringx.IndexOf(lang, ";"); idx != -1 {
			lang = lang[:idx]
		}

		// 转换为小写并去除空格
		lang = stringx.ToLower(stringx.Trim(lang))
		if lang != "" {
			languages = append(languages, lang)
		}
	}

	return languages
}

// IsSupportedLanguage 检查指定语言是否在支持的语言列表中
func (i *I18N) IsSupportedLanguage(lang string) bool {
	return slices.Contains(i.SupportedLanguages, lang)
}
