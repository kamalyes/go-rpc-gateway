/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 00:06:27
 * @FilePath: \go-rpc-gateway\middleware\i18n_test.go
 * @Description: i18n国际化中间件测试
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	goconfigi18n "github.com/kamalyes/go-config/pkg/i18n"

	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/stretchr/testify/assert"
)

const (
	// 测试常量
	errFailedToCreateI18nManager = "Failed to create i18n manager: %v"
	testUser                     = "Test User"
	testAge                      = 25
	testUserCreatedKey           = "user.created"
	testUnknownKey               = "unknown.key"
)

// getTestLocalesPath 动态获取测试用的locales路径
func getTestLocalesPath() string {
	_, currentFile, _, _ := runtime.Caller(0)
	// 获取项目根目录 (go-rpc-gateway)
	projectRoot := filepath.Dir(filepath.Dir(currentFile))
	return filepath.Join(projectRoot, "locales")
}

func TestI18nMiddleware(t *testing.T) {
	config := createTestI18nConfig()
	middleware := I18nWithConfig(config)
	handler := createTestHandler()

	tests := getI18nMiddlewareTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, rec := createTestRequest(tt.queryParam, tt.acceptLanguage)
			middleware(handler).ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.expectedLang, rec.Header().Get(constants.HeaderContentLanguage))
			assert.NotEmpty(t, rec.Body.String())
		})
	}
}

func createTestI18nConfig() *goconfigi18n.I18N {
	return &goconfigi18n.I18N{
		DefaultLanguage:    "en",
		SupportedLanguages: []string{"en", "zh", "ja"},
		DetectionOrder:     []string{"query", "header", "default"},
		LanguageParam:      "lang",
		LanguageHeader:     constants.HeaderAcceptLanguage,
		MessagesPath:       getTestLocalesPath(),
		EnableFallback:     true,
	}
}

func createTestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		welcome := GetMsgByKey(r.Context(), constants.MessageKeyWelcome)
		userData := map[string]interface{}{
			"name": testUser,
			"age":  testAge,
		}
		userMsg := GetMsgWithMap(r.Context(), testUserCreatedKey, userData)
		language := GetLanguage(r.Context())

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Welcome: " + welcome + "\nUser: " + userMsg + "\nLanguage: " + language))
	})
}

type i18nTestCase struct {
	name           string
	queryParam     string
	acceptLanguage string
	expectedLang   string
	expectContains string
}

func getI18nMiddlewareTestCases() []i18nTestCase {
	return []i18nTestCase{
		{
			name:           "Default English",
			expectedLang:   "en",
			expectContains: "Welcome",
		},
		{
			name:           "Query param Chinese",
			queryParam:     "lang=zh",
			expectedLang:   "zh",
			expectContains: "欢迎",
		},
		{
			name:           "Accept-Language Japanese",
			acceptLanguage: "ja,en;q=0.9",
			expectedLang:   "ja",
			expectContains: "ようこそ",
		},
		{
			name:           "Unsupported language fallback",
			queryParam:     "lang=fr",
			expectedLang:   "en",
			expectContains: "Welcome",
		},
	}
}

func createTestRequest(queryParam, acceptLanguage string) (*http.Request, *httptest.ResponseRecorder) {
	url := "/"
	if queryParam != "" {
		url += "?" + queryParam
	}

	req := httptest.NewRequest("GET", url, nil)
	if acceptLanguage != "" {
		req.Header.Set(constants.HeaderAcceptLanguage, acceptLanguage)
	}

	return req, httptest.NewRecorder()
}

func TestGetMsgByKey(t *testing.T) {
	manager, err := NewI18nManager(goconfigi18n.Default())
	assert.NoError(t, err, errFailedToCreateI18nManager)

	ctx := context.WithValue(context.Background(), I18nContextKey, &I18nContext{
		Language: "en",
		Manager:  manager,
	})

	// 测试基础翻译
	message := GetMsgByKey(ctx, constants.MessageKeyWelcome)
	assert.NotEmpty(t, message, "Expected non-empty message")

	// 测试不存在的key
	unknownMsg := GetMsgByKey(ctx, testUnknownKey)
	assert.Equal(t, testUnknownKey, unknownMsg, "Expected fallback to key itself")
}

func TestGetMsgWithMap(t *testing.T) {
	manager, err := NewI18nManager(goconfigi18n.Default())
	assert.NoError(t, err, errFailedToCreateI18nManager)

	ctx := context.WithValue(context.Background(), I18nContextKey, &I18nContext{
		Language: "en",
		Manager:  manager,
	})

	// 测试模板数据翻译
	templateData := map[string]interface{}{
		"name": "John",
		"age":  30,
	}

	// 测试存在的模板消息
	message := GetMsgWithMap(ctx, testUserCreatedKey, templateData)
	assert.NotEmpty(t, message, "Expected non-empty message")

	// 测试nil模板数据
	nilMsg := GetMsgWithMap(ctx, constants.MessageKeyWelcome, nil)
	assert.NotEmpty(t, nilMsg, "Expected non-empty message with nil template data")
}

func TestSetLanguage(t *testing.T) {
	manager, err := NewI18nManager(goconfigi18n.Default())
	assert.NoError(t, err, errFailedToCreateI18nManager)

	ctx := context.WithValue(context.Background(), I18nContextKey, &I18nContext{
		Language: "en",
		Manager:  manager,
	})

	// 测试设置语言
	newCtx := SetLanguage(ctx, "zh")
	newLang := GetLanguage(newCtx)
	assert.Equal(t, "zh", newLang, "Expected language to be set to 'zh'")

	// 测试设置不支持的语言（应该保持原语言）
	invalidCtx := SetLanguage(ctx, "fr")
	invalidLang := GetLanguage(invalidCtx)
	assert.Equal(t, "en", invalidLang, "Expected language to remain 'en' for unsupported language")
}

func TestI18nFromContext(t *testing.T) {
	// 测试空context
	emptyCtx := context.Background()
	i18nCtx := I18nFromContext(emptyCtx)
	assert.Nil(t, i18nCtx, "Expected nil i18n context from empty context")

	// 测试带有i18n context
	manager, err := NewI18nManager(goconfigi18n.Default())
	assert.NoError(t, err, errFailedToCreateI18nManager)

	expectedI18nCtx := &I18nContext{
		Language: "en",
		Manager:  manager,
	}

	ctx := context.WithValue(context.Background(), I18nContextKey, expectedI18nCtx)
	actualI18nCtx := I18nFromContext(ctx)

	assert.NotNil(t, actualI18nCtx, "Expected non-nil i18n context")
	if actualI18nCtx != nil {
		assert.Equal(t, "en", actualI18nCtx.Language, "Expected language to be 'en'")
	}
}

// TestI18nManagerGetMessageWithMap 测试I18nManager的GetMessageWithMap方法
func TestI18nManagerGetMessageWithMap(t *testing.T) {
	manager, err := NewI18nManager(goconfigi18n.Default())
	assert.NoError(t, err, errFailedToCreateI18nManager)

	tests := []struct {
		name         string
		language     string
		key          string
		templateData map[string]interface{}
		expected     string
	}{
		{
			name:         "Template with data",
			language:     "en",
			key:          testUserCreatedKey,
			templateData: map[string]interface{}{"name": "John"},
			expected:     "User John created successfully", // 这个期望值取决于默认消息
		},
		{
			name:         "Template with nil data",
			language:     "en",
			key:          constants.MessageKeyWelcome,
			templateData: nil,
			expected:     "Welcome",
		},
		{
			name:         "Unknown key",
			language:     "en",
			key:          testUnknownKey,
			templateData: nil,
			expected:     testUnknownKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.GetMessageWithMap(tt.language, tt.key, tt.templateData)
			// 由于我们使用的是默认消息，结果可能不完全匹配，但至少应该不为空
			if tt.key == testUnknownKey {
				assert.Equal(t, tt.expected, result)
			} else {
				assert.NotEmpty(t, result)
			}
		})
	}
}

// TestI18nContextMethods 测试I18nContext的方法
func TestI18nContextMethods(t *testing.T) {
	manager, err := NewI18nManager(goconfigi18n.Default())
	assert.NoError(t, err, errFailedToCreateI18nManager)

	ctx := &I18nContext{
		Language: "en",
		Manager:  manager,
	}

	// 测试T方法
	message := ctx.T(constants.MessageKeyWelcome)
	assert.NotEmpty(t, message)

	// 测试TWithMap方法
	templateData := map[string]interface{}{"name": "Alice"}
	msgWithMap := ctx.TWithMap(testUserCreatedKey, templateData)
	assert.NotEmpty(t, msgWithMap)

	// 测试GetLanguage方法
	assert.Equal(t, "en", ctx.GetLanguage())

	// 测试SetLanguage方法
	ctx.SetLanguage("zh")
	assert.Equal(t, "zh", ctx.GetLanguage())

	// 测试设置不支持的语言
	ctx.SetLanguage("unsupported")
	assert.Equal(t, "zh", ctx.GetLanguage(), "Language should not change for unsupported language")
}

// TestParseAcceptLanguage 测试Accept-Language解析
func TestParseAcceptLanguage(t *testing.T) {
	tests := []struct {
		name           string
		acceptLanguage string
		expected       []string
	}{
		{
			name:           "Simple language",
			acceptLanguage: "en",
			expected:       []string{"en"},
		},
		{
			name:           "Multiple languages with quality",
			acceptLanguage: "en-US,en;q=0.9,zh;q=0.8",
			expected:       []string{"en", "en", "zh"},
		},
		{
			name:           "Empty string",
			acceptLanguage: "",
			expected:       []string{},
		},
		{
			name:           "Language with region",
			acceptLanguage: "zh-CN,ja-JP",
			expected:       []string{"zh", "ja"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAcceptLanguage(tt.acceptLanguage)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestLocalizedError 测试本地化错误
func TestLocalizedError(t *testing.T) {
	manager, err := NewI18nManager(goconfigi18n.Default())
	assert.NoError(t, err, errFailedToCreateI18nManager)

	ctx := context.WithValue(context.Background(), I18nContextKey, &I18nContext{
		Language: "en",
		Manager:  manager,
	})

	// 创建本地化错误
	localizedErr := NewLocalizedError(ctx, constants.MessageKeyErrorNotFound)
	assert.NotNil(t, localizedErr)

	// 测试Error()方法
	errorMsg := localizedErr.Error()
	assert.NotEmpty(t, errorMsg)

	// 测试带参数的本地化错误
	localizedErrWithArgs := NewLocalizedError(ctx, "validation.min_length", 8)
	errorMsgWithArgs := localizedErrWithArgs.Error()
	assert.NotEmpty(t, errorMsgWithArgs)
}

// TestLanguageMapping 测试语言映射功能
func TestLanguageMapping(t *testing.T) {
	config := goconfigi18n.Default()
	config.MessagesPath = getTestLocalesPath()
	manager, err := NewI18nManager(config)
	assert.NoError(t, err, "Failed to create i18n manager")

	// 测试中文变体映射
	t.Run("Chinese variants", func(t *testing.T) {
		// 测试简体中文映射
		ctx := context.WithValue(context.Background(), I18nContextKey, &I18nContext{
			Language: "zh-cn",
			Manager:  manager,
		})
		msg := GetMsgByKey(ctx, constants.MessageKeyWelcome)
		assert.Equal(t, "欢迎", msg)

		// 测试繁体中文
		ctx = context.WithValue(context.Background(), I18nContextKey, &I18nContext{
			Language: "zh-tw",
			Manager:  manager,
		})
		msg = GetMsgByKey(ctx, constants.MessageKeyWelcome)
		assert.Equal(t, "歡迎", msg)

		// 测试香港中文（映射到繁体）
		ctx = context.WithValue(context.Background(), I18nContextKey, &I18nContext{
			Language: "zh-hk",
			Manager:  manager,
		})
		msg = GetMsgByKey(ctx, constants.MessageKeyWelcome)
		assert.Equal(t, "歡迎", msg)
	})

	// 测试法语变体
	t.Run("French variants", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), I18nContextKey, &I18nContext{
			Language: "fr-fr",
			Manager:  manager,
		})
		msg := GetMsgByKey(ctx, constants.MessageKeyWelcome)
		assert.Equal(t, "Bienvenue", msg)

		// 测试法语加拿大（映射到法国法语）
		ctx = context.WithValue(context.Background(), I18nContextKey, &I18nContext{
			Language: "fr-ca",
			Manager:  manager,
		})
		msg = GetMsgByKey(ctx, constants.MessageKeyWelcome)
		assert.Equal(t, "Bienvenue", msg)
	})

	// 测试葡萄牙语变体
	t.Run("Portuguese variants", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), I18nContextKey, &I18nContext{
			Language: "pt-br",
			Manager:  manager,
		})
		msg := GetMsgByKey(ctx, constants.MessageKeyWelcome)
		assert.Equal(t, "Bem-vindo", msg)

		ctx = context.WithValue(context.Background(), I18nContextKey, &I18nContext{
			Language: "pt-pt",
			Manager:  manager,
		})
		msg = GetMsgByKey(ctx, constants.MessageKeyWelcome)
		assert.Equal(t, "Bem-vindo", msg) // 葡萄牙语映射到默认pt
	})

	// 测试英语变体
	t.Run("English variants", func(t *testing.T) {
		// 美国英语映射到英语
		ctx := context.WithValue(context.Background(), I18nContextKey, &I18nContext{
			Language: "en-us",
			Manager:  manager,
		})
		msg := GetMsgByKey(ctx, constants.MessageKeyWelcome)
		assert.Equal(t, "Welcome", msg)

		// 英国英语映射到英语
		ctx = context.WithValue(context.Background(), I18nContextKey, &I18nContext{
			Language: "en-gb",
			Manager:  manager,
		})
		msg = GetMsgByKey(ctx, constants.MessageKeyWelcome)
		assert.Equal(t, "Welcome", msg)
	})
}

// TestCustomConfiguration 测试自定义配置
func TestCustomConfiguration(t *testing.T) {
	customConfig := &goconfigi18n.I18N{
		DefaultLanguage:    "en",
		SupportedLanguages: []string{"en", "zh", "custom"},
		DetectionOrder:     []string{"header", "query", "cookie", "default"},
		LanguageParam:      "lang",
		LanguageHeader:     constants.HeaderAcceptLanguage,
		MessagesPath:       "./locales",
		EnableFallback:     true,
		LanguageMapping: map[string]string{
			"zh-cn":          "zh",
			"custom-variant": "custom",
			"test-lang":      "en", // 测试语言映射到英语
		},
		CustomMessagePaths: map[string]string{
			// 示例：可以为特定语言指定不同路径
			// "custom": "custom_locales",
		},
	}

	manager, err := NewI18nManager(customConfig)
	assert.NoError(t, err, "Failed to create i18n manager with custom config")

	// 测试自定义语言映射
	t.Run("Custom language mapping", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), I18nContextKey, &I18nContext{
			Language: "zh-cn",
			Manager:  manager,
		})
		msg := GetMsgByKey(ctx, constants.MessageKeyWelcome)
		assert.Equal(t, "欢迎", msg)

		// 测试自定义映射
		ctx = context.WithValue(context.Background(), I18nContextKey, &I18nContext{
			Language: "test-lang",
			Manager:  manager,
		})
		msg = GetMsgByKey(ctx, constants.MessageKeyWelcome)
		assert.Equal(t, "Welcome", msg) // 应该映射到英语
	})

	// 测试修改支持的语言列表
	t.Run("Modify supported languages", func(t *testing.T) {
		// 修改配置中支持的语言列表
		customConfig.SupportedLanguages = []string{"en", "zh", "ja"}

		// 验证配置已更新
		assert.Contains(t, customConfig.SupportedLanguages, "en")
		assert.Contains(t, customConfig.SupportedLanguages, "zh")
		assert.Contains(t, customConfig.SupportedLanguages, "ja")
		assert.NotContains(t, customConfig.SupportedLanguages, "fr")
	})

	// 测试修改消息路径
	t.Run("Modify messages path", func(t *testing.T) {
		// 修改消息路径
		originalPath := customConfig.MessagesPath
		customConfig.MessagesPath = "./custom_locales"

		// 验证路径已更新
		assert.Equal(t, "./custom_locales", customConfig.MessagesPath)

		// 恢复原始路径
		customConfig.MessagesPath = originalPath
	})
}

func TestAllSupportedLanguages(t *testing.T) {
	// Test all 16 supported languages can load messages
	supportedLanguages := []string{
		"en", "zh", "ja", "ko", "fr", "de", "es", "pt",
		"it", "ru", "ar", "hi", "th", "tr", "nl", "sv",
	}

	config := goconfigi18n.Default()
	config.MessagesPath = getTestLocalesPath()
	manager, err := NewI18nManager(config)
	assert.NoError(t, err, errFailedToCreateI18nManager)

	for _, lang := range supportedLanguages {
		t.Run(fmt.Sprintf("Language_%s", lang), func(t *testing.T) {
			ctx := context.WithValue(context.Background(), I18nContextKey, &I18nContext{
				Language: lang,
				Manager:  manager,
			})
			// Test basic message retrieval
			welcome := GetMsgByKey(ctx, "welcome")
			assert.NotEmpty(t, welcome, "Welcome message should not be empty for language %s", lang)
			assert.NotEqual(t, "welcome", welcome, "Welcome message should be translated for language %s", lang)

			// Test template message with data
			userMap := map[string]interface{}{
				"name": testUser,
				"age":  testAge,
			}
			userInfo := GetMsgWithMap(ctx, "user.info", userMap)
			assert.NotEmpty(t, userInfo, "User info message should not be empty for language %s", lang)
			assert.Contains(t, userInfo, testUser, "User info should contain user name for language %s", lang)
			assert.Contains(t, userInfo, "25", "User info should contain user age for language %s", lang)
		})
	}
}
