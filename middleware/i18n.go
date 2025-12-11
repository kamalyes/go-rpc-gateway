/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-21 11:27:10
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

	goconfigi18n "github.com/kamalyes/go-config/pkg/i18n"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/errors"
)

// contextKey 自定义context key类型，避免与其他包冲突
type contextKey string

const (
	// I18nContextKey i18n上下文键
	I18nContextKey contextKey = "i18n"
)

// I18nManager 国际化管理器
type I18nManager struct {
	config   *goconfigi18n.I18N
	messages map[string]map[string]string
	mutex    sync.RWMutex
}

// NewI18nManager 创建国际化管理器
func NewI18nManager(config *goconfigi18n.I18N) (*I18nManager, error) {
	if config == nil {
		config = goconfigi18n.Default()
	}

	// 如果没有设置 MessageLoader，根据 MessagesPath 自动创建 FileMessageLoader
	if config.MessageLoader == nil && config.MessagesPath != "" {
		config.MessageLoader = NewFileMessageLoader(config.MessagesPath)
	}

	// 如果仍然没有 MessageLoader，返回错误
	if config.MessageLoader == nil {
		return nil, errors.NewErrorf(errors.ErrCodeMiddlewareError, "MessageLoader is required for i18n")
	}

	manager := &I18nManager{
		config:   config,
		messages: make(map[string]map[string]string),
	}

	// 预加载所有支持的语言消息
	for _, lang := range config.SupportedLanguages {
		if err := manager.loadLanguage(lang); err != nil {
			return nil, errors.NewErrorf(errors.ErrCodeLanguageLoadFailed, "language %s: %v", lang, err)
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

// I18n 国际化中间件，使用默认配置
func I18n() MiddlewareFunc {
	return I18nWithConfig(goconfigi18n.Default())
}

// I18nWithConfig 带配置的国际化中间件
func I18nWithConfig(config *goconfigi18n.I18N) MiddlewareFunc {
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
func detectLanguage(r *http.Request, config *goconfigi18n.I18N) string {
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
func detectFromHeader(r *http.Request, config *goconfigi18n.I18N) string {
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
func detectFromQuery(r *http.Request, config *goconfigi18n.I18N) string {
	return r.URL.Query().Get(config.LanguageParam)
}

// detectFromCookie 从Cookie检测语言
func detectFromCookie(r *http.Request, config *goconfigi18n.I18N) string {
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
		return nil, errors.WrapWithContext(err, errors.ErrCodeJSONParseFailed)
	}

	return &JSONMessageLoader{messages: messages}, nil
}

// LoadMessages 加载指定语言的消息
func (j *JSONMessageLoader) LoadMessages(language string) (map[string]string, error) {
	if messages, exists := j.messages[language]; exists {
		return messages, nil
	}
	return nil, errors.NewErrorf(errors.ErrCodeLanguageNotFound, "language: %s", language)
}

// FileMessageLoader 文件系统消息加载器，从磁盘文件加载翻译消息
type FileMessageLoader struct {
	localesPath string
}

// NewFileMessageLoader 创建文件消息加载器
// localesPath: 翻译文件所在目录路径，如 "./locales" 或 "resources/locales"
func NewFileMessageLoader(localesPath string) *FileMessageLoader {
	return &FileMessageLoader{
		localesPath: localesPath,
	}
}

// LoadMessages 加载指定语言的消息（支持嵌套JSON，自动扁平化为点号格式）
// language: 语言代码，如 "zh", "en", "ja" 等
// 会读取 {localesPath}/{language}.json 文件
func (f *FileMessageLoader) LoadMessages(language string) (map[string]string, error) {
	filePath := filepath.Join(f.localesPath, language+".json")

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, errors.NewErrorf(errors.ErrCodeLanguageNotFound, "language file not found: %s", filePath)
	}

	// 读取文件内容
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.NewErrorf(errors.ErrCodeLanguageLoadFailed, "failed to read language file %s: %v", filePath, err)
	}

	// 先尝试解析为嵌套结构
	var nested map[string]interface{}
	if err := json.Unmarshal(data, &nested); err != nil {
		return nil, errors.NewErrorf(errors.ErrCodeJSONParseFailed, "failed to parse language file %s: %v", filePath, err)
	}

	// 扁平化为点号格式
	messages := flattenJSON(nested, "")

	return messages, nil
}

// flattenJSON 递归扁平化嵌套JSON为点号格式
// 例如: {"error": {"internal": "错误"}} -> {"error.internal": "错误"}
func flattenJSON(data map[string]interface{}, prefix string) map[string]string {
	result := make(map[string]string)

	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case string:
			// 字符串值直接存储
			result[fullKey] = v
		case map[string]interface{}:
			// 递归处理嵌套对象
			for k, val := range flattenJSON(v, fullKey) {
				result[k] = val
			}
		default:
			// 其他类型转换为字符串
			result[fullKey] = fmt.Sprintf("%v", v)
		}
	}

	return result
}
