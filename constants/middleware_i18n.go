/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 13:12:59
 * @FilePath: \go-rpc-gateway\constants\middleware_i18n.go
 * @Description: 国际化中间件相关常量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

// contextKey 自定义context key类型，避免与其他包冲突
type I18nContextKey string

const (
	// I18nContextKeyValue i18n上下文键值
	I18nContextKeyValue I18nContextKey = "i18n"
)

// 国际化默认配置常量
const (
	// 默认语言
	I18nDefaultLanguage = "en"

	// 默认语言参数名
	I18nDefaultLanguageParam = "lang"

	// 默认语言头
	I18nDefaultLanguageHeader = "Accept-Language"

	// 默认Cookie名称
	I18nDefaultCookieName = "language"

	// 默认消息文件路径
	I18nDefaultMessagesPath = "./locales"
)

// 语言代码常量
const (
	LangEn   = "en"    // English
	LangZh   = "zh"    // Chinese (Simplified)
	LangZhTW = "zh-tw" // Chinese (Traditional)
	LangJa   = "ja"    // Japanese
	LangKo   = "ko"    // Korean
	LangFr   = "fr"    // French
	LangDe   = "de"    // German
	LangEs   = "es"    // Spanish
	LangPt   = "pt"    // Portuguese
	LangIt   = "it"    // Italian
	LangRu   = "ru"    // Russian
	LangAr   = "ar"    // Arabic
	LangHi   = "hi"    // Hindi
	LangTh   = "th"    // Thai
	LangTr   = "tr"    // Turkish
	LangNl   = "nl"    // Dutch
	LangSv   = "sv"    // Swedish
)

// 国际化检测方法常量
const (
	I18nDetectionHeader = "header"
	I18nDetectionQuery  = "query"
	I18nDetectionCookie = "cookie"
)

// 消息键常量
const (
	MessageKeyWelcome         = "welcome"
	MessageKeyErrorNotFound   = "error.not_found"
	MessageKeyErrorBadRequest = "error.bad_request"
	MessageKeyErrorInternal   = "error.internal"
	MessageKeySuccess         = "success"
)

// 支持的语言列表
var I18nDefaultSupportedLanguages = []string{
	LangEn, LangZh, LangZhTW, LangJa, LangKo,
	LangFr, LangDe, LangEs, LangIt, LangPt, LangRu, LangAr,
}

// 默认检测顺序
var I18nDefaultDetectionOrder = []string{
	I18nDetectionHeader,
	I18nDetectionQuery,
	I18nDetectionCookie,
}

// 默认语言映射
var I18nDefaultLanguageMapping = map[string]string{
	"zh-cn": LangZh,
	LangZhTW: LangZhTW,
	"en-us": LangEn,
	"en-gb": LangEn,
	"fr-fr": LangFr,
	"pt-br": LangPt,
}
