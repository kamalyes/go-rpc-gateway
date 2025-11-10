/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:35:36
 * @FilePath: \go-rpc-gateway\config\i18n.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package config

// I18nConfig 国际化配置 - 集成 go-toolbox 的 i18n 功能
type I18nConfig struct {
	Enabled            bool                   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	DefaultLanguage    string                 `mapstructure:"default_language" yaml:"default_language" json:"default_language"`
	Detection          I18nDetectionConfig    `mapstructure:"detection" yaml:"detection" json:"detection"`
	Translations       I18nTranslationsConfig `mapstructure:"translations" yaml:"translations" json:"translations"`
	SupportedLanguages []string               `mapstructure:"supported_languages" yaml:"supported_languages" json:"supported_languages"`
}

// I18nDetectionConfig 语言检测配置
type I18nDetectionConfig struct {
	Sources    []string `mapstructure:"sources" yaml:"sources" json:"sources"` // header, query, cookie
	HeaderName string   `mapstructure:"header_name" yaml:"header_name" json:"header_name"`
	QueryParam string   `mapstructure:"query_param" yaml:"query_param" json:"query_param"`
	CookieName string   `mapstructure:"cookie_name" yaml:"cookie_name" json:"cookie_name"`
}

// I18nTranslationsConfig 翻译文件配置
type I18nTranslationsConfig struct {
	Path     string `mapstructure:"path" yaml:"path" json:"path"`
	Format   string `mapstructure:"format" yaml:"format" json:"format"` // json, yaml
	Fallback bool   `mapstructure:"fallback" yaml:"fallback" json:"fallback"`
}
