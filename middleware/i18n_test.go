/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-20 20:06:43
 * @FilePath: \go-rpc-gateway\middleware\i18n_test.go
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

	gci18n "github.com/kamalyes/go-config/pkg/i18n"
	goi18n "github.com/kamalyes/go-i18n"
	"github.com/kamalyes/go-rpc-gateway/constants"
	gwerrors "github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestI18nResolveLanguage(t *testing.T) {
	config := gci18n.Default()
	config.SupportedLanguages = []string{"en", "zh", "my"}
	config.AddLanguageMapping("zh-cn", "zh").AddLanguageMapping("en-us", "en")
	config.AddLegacyLanguageMapping("cn", "zh").AddLegacyLanguageMapping("bm", "my")
	config.ResolutionOrder = []gci18n.MappingType{gci18n.LegacyMapping, gci18n.StandardMapping}
	config.MessageLoader, _ = goi18n.NewJSONLoader(`{"en":{"t":"test"},"zh":{"t":"测试"},"my":{"t":"ujian"}}`)

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
	config := gci18n.Default()
	config.SupportedLanguages = []string{"en", "zh"}
	config.AddLanguageMapping("zh-cn", "zh").AddLegacyLanguageMapping("cn", "zh")
	config.ResolutionOrder = []gci18n.MappingType{gci18n.LegacyMapping, gci18n.StandardMapping}
	config.DetectionOrder = []gci18n.DetectionType{gci18n.DetectionHeader, gci18n.DetectionQuery, gci18n.DetectionCookie, gci18n.DetectionDefault}
	config.MessageLoader, _ = goi18n.NewJSONLoader(`{"en":{"t":"test"},"zh":{"t":"测试"}}`)

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
	config := gci18n.Default()
	config.SupportedLanguages = []string{"en", "zh"}
	config.MessageLoader, _ = goi18n.NewJSONLoader(`{"en":{"hello":"Hello","welcome":"Welcome %s"},"zh":{"hello":"你好","welcome":"欢迎 %s"}}`)

	manager, _ := NewI18nManager(config)

	assert.Equal(t, "Hello", manager.GetMessage("en", "hello"))
	assert.Equal(t, "你好", manager.GetMessage("zh", "hello"))
	assert.Equal(t, "Welcome John", manager.GetMessage("en", "welcome", "John"))
	assert.Equal(t, "欢迎 张三", manager.GetMessage("zh", "welcome", "张三"))
	assert.Equal(t, "notfound", manager.GetMessage("en", "notfound"))
}

func TestI18nGlobalFunctions(t *testing.T) {
	config := gci18n.Default()
	config.SupportedLanguages = []string{"en", "zh"}
	config.MessageLoader, _ = goi18n.NewJSONLoader(`{"en":{"greeting":"Hello"},"zh":{"greeting":"你好"}}`)

	manager, _ := NewI18nManager(config)
	ctx := goi18n.NewContext(context.Background(), "zh", manager)

	assert.Equal(t, "你好", T(ctx, "greeting"))
	assert.Equal(t, "zh", GetLanguage(ctx))
	assert.Equal(t, "greeting", T(context.Background(), "greeting"))
}

func TestI18nMessageWithMap(t *testing.T) {
	config := gci18n.Default()
	config.SupportedLanguages = []string{"en"}
	config.MessageLoader, _ = goi18n.NewJSONLoader(`{"en":{"legacy":"User: {{Name}}, Age: {{Age}}","mixed":"User: {{.Name}}, Age: {{Age}}"}}`)

	manager, _ := NewI18nManager(config)

	legacyResult := manager.GetMessageWithMap("en", "legacy", map[string]any{"Name": "John", "Age": 30})
	assert.Equal(t, "User: John, Age: 30", legacyResult)

	mixedResult := manager.GetMessageWithMap("en", "mixed", map[string]any{"Name": "John", "Age": 30})
	assert.Equal(t, "User: John, Age: 30", mixedResult)
}

func newLocalizedAppErrorTestContext(t *testing.T, language string) context.Context {
	t.Helper()

	loader, err := goi18n.NewJSONLoader(`{
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

	manager, err := NewI18nManager(&gci18n.I18N{
		DefaultLanguage:    "en",
		SupportedLanguages: []string{"en", "zh"},
		LanguageHeader:     "Accept-Language",
		LanguageParam:      "lang",
		DetectionOrder:     []gci18n.DetectionType{gci18n.DetectionHeader, gci18n.DetectionQuery, gci18n.DetectionDefault},
		EnableFallback:     true,
		MessageLoader:      loader,
	})
	require.NoError(t, err)

	return goi18n.NewContext(context.Background(), language, manager)
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

// ============================================================================
// gRPC i18n 拦截器测试
// ============================================================================

func newTestI18nManager(t *testing.T) *goi18n.Manager {
	t.Helper()
	loader, err := goi18n.NewJSONLoader(`{"en":{"hello":"Hello","error.xxx":"Error"},"zh":{"hello":"你好","error.xxx":"错误"}}`)
	require.NoError(t, err)
	manager, err := NewI18nManager(&gci18n.I18N{
		DefaultLanguage:    "en",
		SupportedLanguages: []string{"en", "zh"},
		MessageLoader:      loader,
	})
	require.NoError(t, err)
	return manager
}

func TestUnaryServerI18nInterceptor(t *testing.T) {
	manager := newTestI18nManager(t)
	interceptor := UnaryServerI18nInterceptor(manager)

	t.Run("从 metadata 提取语言并创建 i18n context", func(t *testing.T) {
		md := metadata.Pairs(constants.MetadataAcceptLanguage, "zh")
		ctx := metadata.NewIncomingContext(context.Background(), md)

		var gotLang string
		_, err := interceptor(ctx, nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
			gotLang = GetLanguage(ctx)
			return nil, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "zh", gotLang)
	})

	t.Run("metadata 无语言时使用默认语言", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs())

		var gotLang string
		_, err := interceptor(ctx, nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
			gotLang = GetLanguage(ctx)
			return nil, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "en", gotLang)
	})

	t.Run("metadata 语言不受支持时使用默认语言", func(t *testing.T) {
		md := metadata.Pairs(constants.MetadataAcceptLanguage, "fr")
		ctx := metadata.NewIncomingContext(context.Background(), md)

		var gotLang string
		_, err := interceptor(ctx, nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
			gotLang = GetLanguage(ctx)
			return nil, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "en", gotLang)
	})

	t.Run("已有 i18n context 时保留不覆盖", func(t *testing.T) {
		md := metadata.Pairs(constants.MetadataAcceptLanguage, "en")
		ctx := metadata.NewIncomingContext(context.Background(), md)
		// 先注入一个 zh 的 i18n context
		ctx = goi18n.NewContext(ctx, "zh", manager)

		var gotLang string
		_, err := interceptor(ctx, nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
			gotLang = GetLanguage(ctx)
			return nil, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "zh", gotLang) // 不应被 metadata 的 en 覆盖
	})

	t.Run("无 metadata 时使用默认语言", func(t *testing.T) {
		var gotLang string
		_, err := interceptor(context.Background(), nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
			gotLang = GetLanguage(ctx)
			return nil, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "en", gotLang)
	})

	t.Run("i18n context 可正常翻译", func(t *testing.T) {
		md := metadata.Pairs(constants.MetadataAcceptLanguage, "zh")
		ctx := metadata.NewIncomingContext(context.Background(), md)

		var translated string
		_, err := interceptor(ctx, nil, nil, func(ctx context.Context, req interface{}) (interface{}, error) {
			translated = T(ctx, "hello")
			return nil, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "你好", translated)
	})
}

func TestStreamServerI18nInterceptor(t *testing.T) {
	manager := newTestI18nManager(t)
	interceptor := StreamServerI18nInterceptor(manager)

	t.Run("Stream 拦截器从 metadata 提取语言", func(t *testing.T) {
		md := metadata.Pairs(constants.MetadataAcceptLanguage, "zh")
		ctx := metadata.NewIncomingContext(context.Background(), md)

		mockStream := &i18nMockServerStream{ctx: ctx}

		var gotLang string
		err := interceptor(nil, mockStream, nil, func(srv interface{}, ss grpc.ServerStream) error {
			gotLang = GetLanguage(ss.Context())
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, "zh", gotLang)
	})
}

// i18nMockServerStream 用于测试的 mock ServerStream
type i18nMockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *i18nMockServerStream) Context() context.Context {
	return m.ctx
}

func TestEnrichI18nContextFromMetadata(t *testing.T) {
	manager := newTestI18nManager(t)

	t.Run("从 metadata 提取语言", func(t *testing.T) {
		md := metadata.Pairs(constants.MetadataAcceptLanguage, "zh")
		ctx := metadata.NewIncomingContext(context.Background(), md)

		ctx = enrichI18nContextFromMetadata(ctx, manager)
		assert.Equal(t, "zh", GetLanguage(ctx))
	})

	t.Run("无 metadata 时默认语言", func(t *testing.T) {
		ctx := enrichI18nContextFromMetadata(context.Background(), manager)
		assert.Equal(t, "en", GetLanguage(ctx))
	})

	t.Run("已有 i18n context 不覆盖", func(t *testing.T) {
		md := metadata.Pairs(constants.MetadataAcceptLanguage, "en")
		ctx := metadata.NewIncomingContext(context.Background(), md)
		ctx = goi18n.NewContext(ctx, "zh", manager)

		ctx = enrichI18nContextFromMetadata(ctx, manager)
		assert.Equal(t, "zh", GetLanguage(ctx))
	})

	t.Run("从复杂 Accept-Language header 解析语言", func(t *testing.T) {
		md := metadata.Pairs(constants.MetadataAcceptLanguage, "zh-CN,zh;q=0.9,en;q=0.8")
		ctx := metadata.NewIncomingContext(context.Background(), md)

		ctx = enrichI18nContextFromMetadata(ctx, manager)
		assert.Equal(t, "zh", GetLanguage(ctx))
	})

	t.Run("从带语言映射的 Accept-Language header 解析", func(t *testing.T) {
		// 创建带语言映射的 manager
		config := gci18n.Default()
		config.SupportedLanguages = []string{"en", "zh"}
		config.AddLanguageMapping("zh-cn", "zh").AddLanguageMapping("en-us", "en")
		config.ResolutionOrder = []gci18n.MappingType{gci18n.StandardMapping}
		config.MessageLoader, _ = goi18n.NewJSONLoader(`{"en":{"t":"test"},"zh":{"t":"测试"}}`)
		mappedManager, err := NewI18nManager(config)
		require.NoError(t, err)

		md := metadata.Pairs(constants.MetadataAcceptLanguage, "zh-CN")
		ctx := metadata.NewIncomingContext(context.Background(), md)

		ctx = enrichI18nContextFromMetadata(ctx, mappedManager)
		assert.Equal(t, "zh", GetLanguage(ctx))
	})
}
