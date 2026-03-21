/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 15:15:29
 * @FilePath: \go-rpc-gateway\middleware\i18n.go
 * @Description: 国际化i18n中间件
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	goi18n "github.com/kamalyes/go-config/pkg/i18n"
	gwerrors "github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestI18nResolveLanguage(t *testing.T) {
	config := goi18n.Default()
	config.SupportedLanguages = []string{"en", "zh", "my"}
	config.AddLanguageMapping("zh-cn", "zh").AddLanguageMapping("en-us", "en")
	config.AddLegacyLanguageMapping("cn", "zh").AddLegacyLanguageMapping("bm", "my")
	config.ResolutionOrder = []goi18n.MappingType{goi18n.LegacyMapping, goi18n.StandardMapping}
	config.MessageLoader, _ = NewJSONMessageLoader(`{"en":{"t":"test"},"zh":{"t":"测试"},"my":{"t":"ujian"}}`)

	manager, err := NewI18nManager(config)
	assert.NoError(t, err)

	assert.Equal(t, "zh", config.ResolveLanguage("cn"))
	assert.Equal(t, "my", config.ResolveLanguage("bm"))
	assert.Equal(t, "zh", config.ResolveLanguage("zh-cn"))
	assert.Equal(t, "en", config.ResolveLanguage("en-us"))
	assert.Equal(t, "en", config.ResolveLanguage("en"))
	assert.Equal(t, "en", config.ResolveLanguage("fr"))
	assert.True(t, manager.IsLanguageSupported("zh"))
	assert.False(t, manager.IsLanguageSupported("fr"))
}

func TestI18nMiddlewareDetect(t *testing.T) {
	config := goi18n.Default()
	config.SupportedLanguages = []string{"en", "zh"}
	config.AddLanguageMapping("zh-cn", "zh").AddLegacyLanguageMapping("cn", "zh")
	config.ResolutionOrder = []goi18n.MappingType{goi18n.LegacyMapping, goi18n.StandardMapping}
	config.DetectionOrder = []goi18n.DetectionType{goi18n.DetectionHeader, goi18n.DetectionQuery, goi18n.DetectionCookie, goi18n.DetectionDefault}
	config.MessageLoader, _ = NewJSONMessageLoader(`{"en":{"t":"test"},"zh":{"t":"测试"}}`)

	manager, _ := NewI18nManager(config)
	middleware := I18nWithManager(manager)

	tests := []struct {
		name   string
		header string
		query  string
		want   string
	}{
		{"Accept-Language", "zh-CN,zh;q=0.9", "", "zh"},
		{"Query param", "", "zh", "zh"},
		{"Legacy mapping", "cn", "", "zh"},
		{"Default", "", "", "en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test?lang="+tt.query, nil)
			if tt.header != "" {
				req.Header.Set("Accept-Language", tt.header)
			}

			var lang string
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if ctx := I18nFromContext(r.Context()); ctx != nil {
					lang = ctx.GetLanguage()
				}
			}))

			handler.ServeHTTP(httptest.NewRecorder(), req)
			assert.Equal(t, tt.want, lang)
		})
	}
}

func TestI18nTranslation(t *testing.T) {
	config := goi18n.Default()
	config.SupportedLanguages = []string{"en", "zh"}
	config.MessageLoader, _ = NewJSONMessageLoader(`{"en":{"hello":"Hello","welcome":"Welcome %s"},"zh":{"hello":"你好","welcome":"欢迎 %s"}}`)

	manager, _ := NewI18nManager(config)

	assert.Equal(t, "Hello", manager.GetMessage("en", "hello"))
	assert.Equal(t, "你好", manager.GetMessage("zh", "hello"))
	assert.Equal(t, "Welcome John", manager.GetMessage("en", "welcome", "John"))
	assert.Equal(t, "欢迎 张三", manager.GetMessage("zh", "welcome", "张三"))
	assert.Equal(t, "notfound", manager.GetMessage("en", "notfound"))
}

func TestI18nGlobalFunctions(t *testing.T) {
	config := goi18n.Default()
	config.SupportedLanguages = []string{"en", "zh"}
	config.MessageLoader, _ = NewJSONMessageLoader(`{"en":{"greeting":"Hello"},"zh":{"greeting":"你好"}}`)

	manager, _ := NewI18nManager(config)
	ctx := context.WithValue(context.Background(), I18nContextKey, &I18nContext{Language: "zh", Manager: manager})

	assert.Equal(t, "你好", T(ctx, "greeting"))
	assert.Equal(t, "zh", GetLanguage(ctx))
	assert.Equal(t, "greeting", T(context.Background(), "greeting"))
}

func TestI18nMessageWithMap(t *testing.T) {
	config := goi18n.Default()
	config.SupportedLanguages = []string{"en"}
	config.MessageLoader, _ = NewJSONMessageLoader(`{"en":{"user":"User: {{.Name}}, Age: {{.Age}}"}}`)

	manager, _ := NewI18nManager(config)
	result := manager.GetMessageWithMap("en", "user", map[string]any{"Name": "John", "Age": 30})
	assert.Equal(t, "User: John, Age: 30", result)
}

func newLocalizedAppErrorTestContext(t *testing.T, language string) context.Context {
	t.Helper()

	loader, err := NewJSONMessageLoader(`{
		"en": {
			"error.credential_expired": "Credential expired",
			"error.rate_limit": "Rate limit for {{.Name}}"
		},
		"zh": {
			"error.credential_expired": "凭证已过期",
			"error.rate_limit": "{{.Name}} 的限流已触发"
		}
	}`)
	require.NoError(t, err)

	manager, err := NewI18nManager(&goi18n.I18N{
		DefaultLanguage:    "en",
		SupportedLanguages: []string{"en", "zh"},
		LanguageHeader:     "Accept-Language",
		LanguageParam:      "lang",
		DetectionOrder:     []goi18n.DetectionType{goi18n.DetectionHeader, goi18n.DetectionQuery, goi18n.DetectionDefault},
		EnableFallback:     true,
		MessageLoader:      loader,
	})
	require.NoError(t, err)

	return context.WithValue(context.Background(), I18nContextKey, &I18nContext{
		Language: language,
		Manager:  manager,
	})
}

func TestNewLocalizedAppError(t *testing.T) {
	t.Run("uses localized details when translation exists", func(t *testing.T) {
		ctx := newLocalizedAppErrorTestContext(t, "zh")

		appErr := NewLocalizedAppError(ctx, gwerrors.ErrCodeInvalidCredentials, "error.credential_expired")

		require.NotNil(t, appErr)
		assert.Equal(t, gwerrors.ErrCodeInvalidCredentials, appErr.GetCode())
		assert.Equal(t, "凭证已过期", appErr.GetDetails())
	})

	t.Run("falls back to gateway default message without i18n context", func(t *testing.T) {
		appErr := NewLocalizedAppError(context.Background(), gwerrors.ErrCodeInvalidCredentials, "error.credential_expired")

		require.NotNil(t, appErr)
		assert.Equal(t, gwerrors.ErrCodeInvalidCredentials, appErr.GetCode())
		assert.Empty(t, appErr.GetDetails())
		assert.Equal(t, "Invalid credentials", appErr.GetMessage())
	})

	t.Run("falls back to gateway default message when key is missing", func(t *testing.T) {
		ctx := newLocalizedAppErrorTestContext(t, "zh")

		appErr := NewLocalizedAppError(ctx, gwerrors.ErrCodeInvalidCredentials, "error.missing")

		require.NotNil(t, appErr)
		assert.Equal(t, gwerrors.ErrCodeInvalidCredentials, appErr.GetCode())
		assert.Empty(t, appErr.GetDetails())
		assert.Equal(t, "Invalid credentials", appErr.GetMessage())
	})
}

func TestNewLocalizedAppErrorWithMap(t *testing.T) {
	ctx := newLocalizedAppErrorTestContext(t, "zh")

	appErr := NewLocalizedAppErrorWithMap(ctx, gwerrors.ErrCodeTooManyRequests, "error.rate_limit", map[string]any{
		"Name": "open-app",
	})

	require.NotNil(t, appErr)
	assert.Equal(t, gwerrors.ErrCodeTooManyRequests, appErr.GetCode())
	assert.Equal(t, "open-app 的限流已触发", appErr.GetDetails())
}
