/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 16:30:00
 * @FilePath: \go-rpc-gateway\middleware\i18n.go
 * @Description: 国际化i18n中间件
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kamalyes/go-rpc-gateway/constants"
)

// contextKey 自定义context key类型，避免与其他包冲突
type contextKey string

const (
	// I18nContextKey i18n上下文键
	I18nContextKey contextKey = "i18n"
)

// I18nConfig 国际化中间件配置
type I18nConfig struct {
	// 默认语言
	DefaultLanguage string
	// 支持的语言列表
	SupportedLanguages []string
	// 语言映射关系，支持地区变体映射到基础语言
	// 例如: {"zh-cn": "zh", "zh-tw": "zh-tw", "en-us": "en", "fr-fr": "fr"}
	LanguageMapping map[string]string
	// 语言检测顺序：header, query, cookie, default
	DetectionOrder []string
	// 语言参数名称（用于query和cookie）
	LanguageParam string
	// 语言头名称
	LanguageHeader string
	// 消息文件路径
	MessagesPath string
	// 自定义消息文件路径映射，允许为特定语言指定不同的文件路径
	// 例如: {"zh-tw": "./locales/traditional", "en": "./locales/english"}
	CustomMessagePaths map[string]string
	// 是否启用回退到默认语言
	EnableFallback bool
	// 自定义消息加载器
	MessageLoader MessageLoader
}

// MessageLoader 消息加载器接口
type MessageLoader interface {
	LoadMessages(language string) (map[string]string, error)
}

// FileMessageLoader 文件消息加载器
type FileMessageLoader struct {
	basePath           string
	customMessagePaths map[string]string
	languageMapping    map[string]string
}

// NewFileMessageLoader 创建文件消息加载器
func NewFileMessageLoader(basePath string, customPaths map[string]string, langMapping map[string]string) *FileMessageLoader {
	if customPaths == nil {
		customPaths = make(map[string]string)
	}
	if langMapping == nil {
		langMapping = make(map[string]string)
	}
	return &FileMessageLoader{
		basePath:           basePath,
		customMessagePaths: customPaths,
		languageMapping:    langMapping,
	}
}

// LoadMessages 从文件加载消息
func (f *FileMessageLoader) LoadMessages(language string) (map[string]string, error) {
	// 解析语言映射
	targetLanguage := f.resolveLanguage(language)

	// 获取文件路径
	filePath := f.getMessageFilePath(targetLanguage)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// 如果文件不存在，返回默认消息作为备用
		return f.getDefaultMessages(targetLanguage), nil
	}

	// 读取文件内容
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read i18n file %s: %v", filePath, err)
	}

	// 解析JSON
	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("failed to parse i18n file %s: %v", filePath, err)
	}

	return messages, nil
} // resolveLanguage 解析语言映射
func (f *FileMessageLoader) resolveLanguage(language string) string {
	// 先检查完整匹配
	if mapped, exists := f.languageMapping[language]; exists {
		return mapped
	}

	// 尝试基础语言匹配（如 zh-cn -> zh）
	if idx := strings.Index(language, "-"); idx > 0 {
		baseLang := language[:idx]
		if mapped, exists := f.languageMapping[baseLang]; exists {
			return mapped
		}
	}

	return language
}

// getMessageFilePath 获取消息文件路径
func (f *FileMessageLoader) getMessageFilePath(language string) string {
	// 检查是否有自定义路径
	if customPath, exists := f.customMessagePaths[language]; exists {
		return filepath.Join(customPath, fmt.Sprintf("%s.json", language))
	}

	// 使用默认路径
	return filepath.Join(f.basePath, fmt.Sprintf("%s.json", language))
}

// getDefaultMessages 获取默认消息
func (f *FileMessageLoader) getDefaultMessages(language string) map[string]string {
	switch language {
	case constants.LangEn: // English
		return map[string]string{
			constants.MessageKeyWelcome:         "Welcome",
			constants.MessageKeyErrorNotFound:   "Resource not found",
			constants.MessageKeyErrorBadRequest: "Bad request",
			constants.MessageKeyErrorInternal:   "Internal server error",
			constants.MessageKeySuccess:         "Success",
		}
	case constants.LangZh: // Chinese (Simplified)
		return map[string]string{
			constants.MessageKeyWelcome:         "欢迎",
			constants.MessageKeyErrorNotFound:   "资源未找到",
			constants.MessageKeyErrorBadRequest: "请求无效",
			constants.MessageKeyErrorInternal:   "服务器内部错误",
			constants.MessageKeySuccess:         "成功",
		}
	case constants.LangJa: // Japanese
		return map[string]string{
			constants.MessageKeyWelcome:         "ようこそ",
			constants.MessageKeyErrorNotFound:   "リソースが見つかりません",
			constants.MessageKeyErrorBadRequest: "無効なリクエスト",
			constants.MessageKeyErrorInternal:   "サーバー内部エラー",
			constants.MessageKeySuccess:         "成功",
		}
	case constants.LangKo: // Korean
		return map[string]string{
			constants.MessageKeyWelcome:         "환영합니다",
			constants.MessageKeyErrorNotFound:   "리소스를 찾을 수 없습니다",
			constants.MessageKeyErrorBadRequest: "잘못된 요청",
			constants.MessageKeyErrorInternal:   "서버 내부 오류",
			constants.MessageKeySuccess:         "성공",
		}
	case constants.LangFr: // French
		return map[string]string{
			constants.MessageKeyWelcome:         "Bienvenue",
			constants.MessageKeyErrorNotFound:   "Ressource non trouvée",
			constants.MessageKeyErrorBadRequest: "Requête invalide",
			constants.MessageKeyErrorInternal:   "Erreur interne du serveur",
			constants.MessageKeySuccess:         "Succès",
		}
	case constants.LangDe: // German
		return map[string]string{
			constants.MessageKeyWelcome:         "Willkommen",
			constants.MessageKeyErrorNotFound:   "Ressource nicht gefunden",
			constants.MessageKeyErrorBadRequest: "Ungültige Anfrage",
			constants.MessageKeyErrorInternal:   "Interner Serverfehler",
			constants.MessageKeySuccess:         "Erfolg",
		}
	case constants.LangEs: // Spanish
		return map[string]string{
			constants.MessageKeyWelcome:         "Bienvenido",
			constants.MessageKeyErrorNotFound:   "Recurso no encontrado",
			constants.MessageKeyErrorBadRequest: "Solicitud incorrecta",
			constants.MessageKeyErrorInternal:   "Error interno del servidor",
			constants.MessageKeySuccess:         "Éxito",
		}
	case constants.LangPt: // Portuguese
		return map[string]string{
			constants.MessageKeyWelcome:         "Bem-vindo",
			constants.MessageKeyErrorNotFound:   "Recurso não encontrado",
			constants.MessageKeyErrorBadRequest: "Solicitação inválida",
			constants.MessageKeyErrorInternal:   "Erro interno do servidor",
			constants.MessageKeySuccess:         "Sucesso",
		}
	case constants.LangIt: // Italian
		return map[string]string{
			constants.MessageKeyWelcome:         "Benvenuto",
			constants.MessageKeyErrorNotFound:   "Risorsa non trovata",
			constants.MessageKeyErrorBadRequest: "Richiesta non valida",
			constants.MessageKeyErrorInternal:   "Errore interno del server",
			constants.MessageKeySuccess:         "Successo",
		}
	case constants.LangRu: // Russian
		return map[string]string{
			constants.MessageKeyWelcome:         "Добро пожаловать",
			constants.MessageKeyErrorNotFound:   "Ресурс не найден",
			constants.MessageKeyErrorBadRequest: "Неверный запрос",
			constants.MessageKeyErrorInternal:   "Внутренняя ошибка сервера",
			constants.MessageKeySuccess:         "Успех",
		}
	case constants.LangAr: // Arabic
		return map[string]string{
			constants.MessageKeyWelcome:         "مرحباً",
			constants.MessageKeyErrorNotFound:   "المورد غير موجود",
			constants.MessageKeyErrorBadRequest: "طلب غير صحيح",
			constants.MessageKeyErrorInternal:   "خطأ داخلي في الخادم",
			constants.MessageKeySuccess:         "نجح",
		}
	case constants.LangHi: // Hindi
		return map[string]string{
			constants.MessageKeyWelcome:         "स्वागत है",
			constants.MessageKeyErrorNotFound:   "संसाधन नहीं मिला",
			constants.MessageKeyErrorBadRequest: "गलत अनुरोध",
			constants.MessageKeyErrorInternal:   "सर्वर आंतरिक त्रुटि",
			constants.MessageKeySuccess:         "सफलता",
		}
	case constants.LangTh: // Thai
		return map[string]string{
			constants.MessageKeyWelcome:         "ยินดีต้อนรับ",
			constants.MessageKeyErrorNotFound:   "ไม่พบทรัพยากร",
			constants.MessageKeyErrorBadRequest: "คำขอไม่ถูกต้อง",
			constants.MessageKeyErrorInternal:   "ข้อผิดพลาดภายในเซิร์ฟเวอร์",
			constants.MessageKeySuccess:         "สำเร็จ",
		}
	case constants.LangTr: // Turkish
		return map[string]string{
			constants.MessageKeyWelcome:         "Hoş geldiniz",
			constants.MessageKeyErrorNotFound:   "Kaynak bulunamadı",
			constants.MessageKeyErrorBadRequest: "Geçersiz istek",
			constants.MessageKeyErrorInternal:   "Sunucu iç hatası",
			constants.MessageKeySuccess:         "Başarı",
		}
	case constants.LangNl: // Dutch
		return map[string]string{
			constants.MessageKeyWelcome:         "Welkom",
			constants.MessageKeyErrorNotFound:   "Bron niet gevonden",
			constants.MessageKeyErrorBadRequest: "Ongeldig verzoek",
			constants.MessageKeyErrorInternal:   "Interne serverfout",
			constants.MessageKeySuccess:         "Succes",
		}
	case constants.LangSv: // Swedish
		return map[string]string{
			constants.MessageKeyWelcome:         "Välkommen",
			constants.MessageKeyErrorNotFound:   "Resursen hittades inte",
			constants.MessageKeyErrorBadRequest: "Ogiltigt förfrågan",
			constants.MessageKeyErrorInternal:   "Intern serverfel",
			constants.MessageKeySuccess:         "Framgång",
		}
	default:
		return nil
	}
} // I18nManager 国际化管理器
type I18nManager struct {
	config   *I18nConfig
	messages map[string]map[string]string
	mutex    sync.RWMutex
}

// NewI18nManager 创建国际化管理器
func NewI18nManager(config *I18nConfig) (*I18nManager, error) {
	if config.MessageLoader == nil {
		config.MessageLoader = NewFileMessageLoader(
			config.MessagesPath,
			config.CustomMessagePaths,
			config.LanguageMapping,
		)
	}

	manager := &I18nManager{
		config:   config,
		messages: make(map[string]map[string]string),
	}

	// 预加载所有支持的语言消息
	for _, lang := range config.SupportedLanguages {
		if err := manager.loadLanguage(lang); err != nil {
			return nil, fmt.Errorf("failed to load language %s: %v", lang, err)
		}
	}

	// 预加载语言映射中的目标语言
	if config.LanguageMapping != nil {
		for _, targetLang := range config.LanguageMapping {
			// 如果目标语言还没有加载过，则加载它
			if _, exists := manager.messages[targetLang]; !exists {
				if err := manager.loadLanguage(targetLang); err != nil {
					// 注意：这里不返回错误，因为目标语言文件可能不存在
					continue
				}
			}
		}
	}

	return manager, nil
}

// loadLanguage 加载指定语言的消息
func (i *I18nManager) loadLanguage(language string) error {
	messages, err := i.config.MessageLoader.LoadMessages(language)
	if err != nil {
		return err
	}

	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.messages[language] = messages

	return nil
} // GetMessage 获取翻译消息
func (i *I18nManager) GetMessage(language, key string, args ...interface{}) string {
	return i.getMessageInternal(language, key, args, nil)
}

// GetMessageWithMap 使用map模板数据获取翻译消息
func (i *I18nManager) GetMessageWithMap(language, key string, templateData map[string]interface{}) string {
	return i.getMessageInternal(language, key, nil, templateData)
}

// getMessageInternal 内部获取消息的实现
func (i *I18nManager) getMessageInternal(language, key string, args []interface{}, templateData map[string]interface{}) string {
	i.mutex.RLock()
	defer i.mutex.RUnlock()

	// 尝试获取指定语言的消息
	if message := i.getMessageFromLanguage(language, key); message != "" {
		return i.formatMessage(message, args, templateData)
	}

	// 如果启用回退，尝试默认语言
	if i.config.EnableFallback && language != i.config.DefaultLanguage {
		if message := i.getMessageFromLanguage(i.config.DefaultLanguage, key); message != "" {
			return i.formatMessage(message, args, templateData)
		}
	}

	// 如果都没有找到，返回key本身
	return key
}

// getMessageFromLanguage 从指定语言获取消息
func (i *I18nManager) getMessageFromLanguage(language, key string) string {
	// 先尝试直接查找原始语言
	if message := i.findMessageInLanguage(language, key); message != "" {
		return message
	}

	// 如果没有找到，尝试语言映射
	if i.config.LanguageMapping != nil {
		if message := i.findMessageWithMapping(language, key); message != "" {
			return message
		}
	}

	return ""
}

// findMessageInLanguage 在指定语言中查找消息
func (i *I18nManager) findMessageInLanguage(language, key string) string {
	if langMessages, exists := i.messages[language]; exists {
		if message, exists := langMessages[key]; exists {
			return message
		}
	}
	return ""
}

// findMessageWithMapping 使用语言映射查找消息
func (i *I18nManager) findMessageWithMapping(language, key string) string {
	// 尝试直接映射
	if targetLang, exists := i.config.LanguageMapping[language]; exists && targetLang != language {
		if message := i.findMessageInLanguage(targetLang, key); message != "" {
			return message
		}
	}

	// 尝试基础语言匹配（如 zh-cn -> zh）
	if idx := strings.Index(language, "-"); idx > 0 {
		baseLang := language[:idx]
		if targetLang, exists := i.config.LanguageMapping[baseLang]; exists {
			if message := i.findMessageInLanguage(targetLang, key); message != "" {
				return message
			}
		}
	}

	return ""
}

// formatMessage 格式化消息
func (i *I18nManager) formatMessage(message string, args []interface{}, templateData map[string]interface{}) string {
	// 如果有模板数据，使用模板数据格式化
	if templateData != nil {
		return i.formatWithTemplateData(message, templateData)
	}

	// 如果有参数，使用printf风格格式化
	if len(args) > 0 {
		return fmt.Sprintf(message, args...)
	}

	return message
}

// formatWithTemplateData 使用模板数据格式化消息
func (i *I18nManager) formatWithTemplateData(message string, templateData map[string]interface{}) string {
	result := message
	for key, value := range templateData {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		valueStr := fmt.Sprintf("%v", value)
		result = strings.ReplaceAll(result, placeholder, valueStr)
	}
	// 清理未替换的占位符
	result = strings.ReplaceAll(result, ": <no value>", "")
	return result
}

// IsLanguageSupported 检查语言是否被支持
func (i *I18nManager) IsLanguageSupported(language string) bool {
	for _, supported := range i.config.SupportedLanguages {
		if supported == language {
			return true
		}
	}
	return false
}

// DefaultI18nConfig 默认国际化配置
func DefaultI18nConfig() *I18nConfig {
	return &I18nConfig{
		DefaultLanguage:    constants.I18nDefaultLanguage,
		SupportedLanguages: constants.I18nDefaultSupportedLanguages,
		DetectionOrder:     constants.I18nDefaultDetectionOrder,
		LanguageParam:      constants.I18nDefaultLanguageParam,
		LanguageHeader:     constants.HeaderAcceptLanguage,
		MessagesPath:       constants.I18nDefaultMessagesPath,
		EnableFallback:     true,
		// 默认语言映射，支持常见的地区变体
		LanguageMapping: constants.I18nDefaultLanguageMapping,
		// 自定义消息路径，可以为特定语言指定不同目录
		CustomMessagePaths: map[string]string{
			// 示例：特殊语言可以放在不同目录
			// "zh-tw": "locales/traditional",
			// "fr-fr": "locales/france",
		},
	}
}

// I18n 国际化中间件
func I18n() MiddlewareFunc {
	return I18nWithConfig(DefaultI18nConfig())
}

// I18nWithConfig 带配置的国际化中间件
func I18nWithConfig(config *I18nConfig) MiddlewareFunc {
	manager, err := NewI18nManager(config)
	if err != nil {
		panic(fmt.Sprintf("failed to create i18n manager: %v", err))
	}

	return I18nWithManager(manager)
}

// I18nWithManager 带管理器的国际化中间件
func I18nWithManager(manager *I18nManager) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			language := detectLanguage(r, manager.config)

			// 验证语言是否被支持
			if !manager.IsLanguageSupported(language) {
				language = manager.config.DefaultLanguage
			}

			// 设置响应头
			w.Header().Set(constants.HeaderContentLanguage, language)

			// 创建带有i18n上下文的新请求
			ctx := context.WithValue(r.Context(), I18nContextKey, &I18nContext{
				Language: language,
				Manager:  manager,
			})

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// detectLanguage 检测用户语言偏好
func detectLanguage(r *http.Request, config *I18nConfig) string {
	for _, method := range config.DetectionOrder {
		switch method {
		case "header":
			if lang := detectFromHeader(r, config); lang != "" {
				return lang
			}
		case "query":
			if lang := detectFromQuery(r, config); lang != "" {
				return lang
			}
		case "cookie":
			if lang := detectFromCookie(r, config); lang != "" {
				return lang
			}
		case "default":
			return config.DefaultLanguage
		}
	}
	return config.DefaultLanguage
}

// detectFromHeader 从HTTP头检测语言
func detectFromHeader(r *http.Request, config *I18nConfig) string {
	acceptLanguage := r.Header.Get(config.LanguageHeader)
	if acceptLanguage == "" {
		return ""
	}

	// 解析Accept-Language头
	languages := parseAcceptLanguage(acceptLanguage)

	// 查找第一个支持的语言
	for _, lang := range languages {
		for _, supported := range config.SupportedLanguages {
			if strings.HasPrefix(lang, supported) || strings.HasPrefix(supported, lang) {
				return supported
			}
		}
	}

	return ""
}

// detectFromQuery 从查询参数检测语言
func detectFromQuery(r *http.Request, config *I18nConfig) string {
	return r.URL.Query().Get(config.LanguageParam)
}

// detectFromCookie 从Cookie检测语言
func detectFromCookie(r *http.Request, config *I18nConfig) string {
	cookie, err := r.Cookie(config.LanguageParam)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// parseAcceptLanguage 解析Accept-Language头
func parseAcceptLanguage(acceptLanguage string) []string {
	var languages []string

	if acceptLanguage == "" {
		return []string{}
	}

	parts := strings.Split(acceptLanguage, ",")
	for _, part := range parts {
		// 移除权重信息（如 en;q=0.9）
		lang := strings.TrimSpace(strings.Split(part, ";")[0])
		if lang != "" {
			// 处理语言代码（如 en-US -> en）
			if idx := strings.Index(lang, "-"); idx > 0 {
				lang = lang[:idx]
			}
			languages = append(languages, lang)
		}
	}

	if languages == nil {
		return []string{}
	}

	return languages
}

// I18nContext 国际化上下文
type I18nContext struct {
	Language string
	Manager  *I18nManager
}

// T 翻译函数（简化调用）
func (ctx *I18nContext) T(key string, args ...interface{}) string {
	return ctx.Manager.GetMessage(ctx.Language, key, args...)
}

// TWithMap 使用map模板数据翻译
func (ctx *I18nContext) TWithMap(key string, templateData map[string]interface{}) string {
	return ctx.Manager.GetMessageWithMap(ctx.Language, key, templateData)
}

// GetLanguage 获取当前语言
func (ctx *I18nContext) GetLanguage() string {
	return ctx.Language
}

// SetLanguage 设置当前语言
func (ctx *I18nContext) SetLanguage(language string) {
	if ctx.Manager.IsLanguageSupported(language) {
		ctx.Language = language
	}
}

// I18nFromContext 从context中获取i18n上下文的辅助函数
func I18nFromContext(ctx context.Context) *I18nContext {
	if i18nCtx, ok := ctx.Value(I18nContextKey).(*I18nContext); ok {
		return i18nCtx
	}
	return nil
}

// T 全局翻译函数
func T(ctx context.Context, key string, args ...interface{}) string {
	if i18nCtx := I18nFromContext(ctx); i18nCtx != nil {
		return i18nCtx.T(key, args...)
	}
	return key
}

// TWithMap 全局使用map模板数据翻译函数
func TWithMap(ctx context.Context, key string, templateData map[string]interface{}) string {
	if i18nCtx := I18nFromContext(ctx); i18nCtx != nil {
		return i18nCtx.TWithMap(key, templateData)
	}
	return key
}

// GetMsgByKey 通过键获取消息（业务层级函数）
func GetMsgByKey(ctx context.Context, key string) string {
	return T(ctx, key)
}

// GetMsgWithMap 使用map模板数据获取消息（业务层级函数）
func GetMsgWithMap(ctx context.Context, key string, maps map[string]interface{}) string {
	if maps == nil {
		return GetMsgByKey(ctx, key)
	}

	content := TWithMap(ctx, key, maps)
	// 清理未替换的占位符
	content = strings.ReplaceAll(content, ": <no value>", "")

	if content == "" {
		return key
	}
	return content
}

// GetLanguage 全局获取语言函数
func GetLanguage(ctx context.Context) string {
	if i18nCtx := I18nFromContext(ctx); i18nCtx != nil {
		return i18nCtx.GetLanguage()
	}
	return constants.I18nDefaultLanguage // 默认返回英语
}

// SetLanguage 全局设置语言函数
func SetLanguage(ctx context.Context, language string) context.Context {
	if i18nCtx := I18nFromContext(ctx); i18nCtx != nil {
		i18nCtx.SetLanguage(language)
		return context.WithValue(ctx, I18nContextKey, i18nCtx)
	}
	return ctx
}

// LocalizedError 本地化错误结构
type LocalizedError struct {
	Key     string
	Args    []interface{}
	Context context.Context
}

// Error 实现error接口
func (e *LocalizedError) Error() string {
	return T(e.Context, e.Key, e.Args...)
}

// NewLocalizedError 创建本地化错误
func NewLocalizedError(ctx context.Context, key string, args ...interface{}) *LocalizedError {
	return &LocalizedError{
		Key:     key,
		Args:    args,
		Context: ctx,
	}
}

// JSONMessageLoader JSON消息加载器，用于从JSON数据加载消息
type JSONMessageLoader struct {
	messages map[string]map[string]string
}

// NewJSONMessageLoader 创建JSON消息加载器
func NewJSONMessageLoader(messagesJSON string) (*JSONMessageLoader, error) {
	var messages map[string]map[string]string
	if err := json.Unmarshal([]byte(messagesJSON), &messages); err != nil {
		return nil, fmt.Errorf("failed to parse JSON messages: %v", err)
	}

	return &JSONMessageLoader{messages: messages}, nil
}

// LoadMessages 加载指定语言的消息
func (j *JSONMessageLoader) LoadMessages(language string) (map[string]string, error) {
	if messages, exists := j.messages[language]; exists {
		return messages, nil
	}
	return nil, fmt.Errorf("language %s not found", language)
}

// ConfigurableI18nMiddleware 可配置的国际化中间件
// TODO: 重构为使用 go-config 的 i18n.I18N 配置
/*
func ConfigurableI18nMiddleware(i18nConfig *config.I18nConfig) HTTPMiddleware {
	if i18nConfig == nil || !i18nConfig.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// 创建 I18n 管理器
	config := &I18nConfig{
		DefaultLanguage:    i18nConfig.DefaultLanguage,
		SupportedLanguages: i18nConfig.SupportedLanguages,
		DetectionOrder:     i18nConfig.Detection.Sources,
		LanguageParam:      i18nConfig.Detection.QueryParam,
		LanguageHeader:     i18nConfig.Detection.HeaderName,
		MessagesPath:       i18nConfig.Translations.Path,
		EnableFallback:     i18nConfig.Translations.Fallback,
	}

	// 设置默认值
	if config.DefaultLanguage == "" {
		config.DefaultLanguage = constants.I18nDefaultLanguage
	}
	if config.LanguageParam == "" {
		config.LanguageParam = constants.I18nDefaultLanguageParam
	}
	if config.LanguageHeader == "" {
		config.LanguageHeader = constants.I18nDefaultLanguageHeader
	}
	if len(config.DetectionOrder) == 0 {
		config.DetectionOrder = constants.I18nDefaultDetectionOrder
	}

	manager, err := NewI18nManager(config)
	if err != nil {
		// 如果初始化失败，返回一个无操作的中间件
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return HTTPMiddleware(I18nWithManager(manager))
}
*/
