/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 15:13:29
 * @FilePath: \go-rpc-gateway\middleware\i18n.go
 * @Description: 国际化i18n中间件
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"fmt"
	"net/http"

	gci18n "github.com/kamalyes/go-config/pkg/i18n"
	goi18n "github.com/kamalyes/go-i18n"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type I18nManager = goi18n.Manager

func NewI18nManager(config *gci18n.I18N) (*goi18n.Manager, error) {
	return goi18n.NewManager(config)
}

func NewJSONMessageLoader(messagesJSON string) (*goi18n.JSONLoader, error) {
	return goi18n.NewJSONLoader(messagesJSON)
}

func NewFileMessageLoader(localesPath string) *goi18n.FileLoader {
	return goi18n.NewFileLoader(localesPath)
}

func I18nFromContext(ctx context.Context) *goi18n.Context {
	return goi18n.FromContext(ctx)
}

func T(ctx context.Context, key string, args ...any) string {
	return goi18n.T(ctx, key, args...)
}

func TWithMap(ctx context.Context, key string, templateData map[string]any) string {
	return goi18n.TWithMap(ctx, key, templateData)
}

func GetMsgByKey(ctx context.Context, key string) string {
	return goi18n.GetMsgByKey(ctx, key)
}

func GetMsgWithMap(ctx context.Context, key string, maps map[string]any) string {
	return goi18n.GetMsgWithMap(ctx, key, maps)
}

func GetLanguage(ctx context.Context) string {
	return goi18n.GetLanguage(ctx)
}

func SetLanguage(ctx context.Context, language string) context.Context {
	return goi18n.SetLanguage(ctx, language)
}

func I18n() MiddlewareFunc {
	manager, err := goi18n.NewManager(gci18n.Default())
	if err != nil {
		panic(fmt.Sprintf("failed to create i18n manager: %v", err))
	}

	return I18nWithManager(manager)
}

func I18nWithManager(manager *goi18n.Manager) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			language := detectLanguage(r, manager.GetConfig())

			if !manager.IsLanguageSupported(language) {
				language = manager.GetDefaultLanguage()
			}

			w.Header().Set(constants.HeaderContentLanguage, language)

			ctx := goi18n.NewContext(r.Context(), language, manager)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func detectLanguage(r *http.Request, config *gci18n.I18N) string {
	for _, method := range config.DetectionOrder {
		switch method {
		case gci18n.DetectionHeader:
			if lang := detectFromHeader(r, config); lang != "" {
				return lang
			}
		case gci18n.DetectionQuery:
			if lang := detectFromQuery(r, config); lang != "" {
				return lang
			}
		case gci18n.DetectionCookie:
			if lang := detectFromCookie(r, config); lang != "" {
				return lang
			}
		case gci18n.DetectionDefault:
			return config.DefaultLanguage
		}
	}
	return config.DefaultLanguage
}

func detectFromHeader(r *http.Request, config *gci18n.I18N) string {
	acceptLanguage := r.Header.Get(config.LanguageHeader)
	if acceptLanguage == "" {
		return ""
	}

	return config.ParseAcceptLanguage(acceptLanguage)
}

func detectFromQuery(r *http.Request, config *gci18n.I18N) string {
	if lang := r.URL.Query().Get(config.LanguageParam); lang != "" {
		return config.ResolveLanguage(lang)
	}
	return ""
}

func detectFromCookie(r *http.Request, config *gci18n.I18N) string {
	cookieName := mathx.IfEmpty(config.CookieName, config.LanguageParam)

	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return config.ResolveLanguage(cookie.Value)
}

type LocalizedError struct {
	Key     string
	Args    []any
	Context context.Context
}

func (e *LocalizedError) Error() string {
	return goi18n.T(e.Context, e.Key, e.Args...)
}

func NewLocalizedError(ctx context.Context, key string, args ...any) *LocalizedError {
	return &LocalizedError{
		Key:     key,
		Args:    args,
		Context: ctx,
	}
}

func NewLocalizedAppError(ctx context.Context, code errors.ErrorCode, key string, args ...any) *errors.AppError {
	appErr := errors.NewError(code, "")
	if ctx == nil || key == "" {
		return appErr
	}

	localizedMessage := goi18n.T(ctx, key, args...)
	if localizedMessage != "" && localizedMessage != key {
		appErr.WithDetails(localizedMessage)
	}

	return appErr
}

func NewLocalizedAppErrorWithMap(ctx context.Context, code errors.ErrorCode, key string, templateData map[string]any) *errors.AppError {
	appErr := errors.NewError(code, "")
	if ctx == nil || key == "" {
		return appErr
	}

	var localizedMessage string
	if templateData == nil {
		localizedMessage = goi18n.GetMsgByKey(ctx, key)
	} else {
		localizedMessage = goi18n.GetMsgWithMap(ctx, key, templateData)
	}

	if localizedMessage != "" && localizedMessage != key {
		appErr.WithDetails(localizedMessage)
	}

	return appErr
}

// ============================================================================
// gRPC i18n 拦截器 - 从 gRPC metadata 提取语言并注入 i18n context
// ============================================================================

// UnaryServerI18nInterceptor 创建 gRPC 服务端一元调用 i18n 拦截器
// 从 incoming metadata 中提取 x-language，创建 i18n context
// 如果 metadata 中没有语言信息，则使用 i18n 管理器的默认语言
func UnaryServerI18nInterceptor(manager *goi18n.Manager) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx = enrichI18nContextFromMetadata(ctx, manager)
		return handler(ctx, req)
	}
}

// StreamServerI18nInterceptor 创建 gRPC 服务端流式调用 i18n 拦截器
func StreamServerI18nInterceptor(manager *goi18n.Manager) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := enrichI18nContextFromMetadata(ss.Context(), manager)
		return handler(srv, &i18nWrappedStream{ServerStream: ss, ctx: ctx})
	}
}

// enrichI18nContextFromMetadata 从 gRPC metadata 提取语言信息并创建 i18n context
func enrichI18nContextFromMetadata(ctx context.Context, manager *goi18n.Manager) context.Context {
	// 如果已有 i18n context，直接返回
	if goi18n.FromContext(ctx) != nil {
		return ctx
	}

	language := ""

	// 从 incoming metadata 提取 accept-language
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(constants.MetadataAcceptLanguage); len(values) > 0 {
			acceptLanguage := values[0]
			// 使用 i18n 配置解析 Accept-Language header（与 HTTP 中间件保持一致）
			// 支持 "zh-CN,zh;q=0.9" 等复杂格式，以及语言映射（zh-cn → zh）
			config := manager.GetConfig()
			if config != nil {
				language = config.ParseAcceptLanguage(acceptLanguage)
			} else {
				language = acceptLanguage
			}
		}
	}

	// 验证语言是否受支持，不支持则使用默认语言
	if language == "" || !manager.IsLanguageSupported(language) {
		language = manager.GetDefaultLanguage()
	}

	// 创建 i18n context
	return goi18n.NewContext(ctx, language, manager)
}

// i18nWrappedStream 包装 grpc.ServerStream 以覆盖 Context
type i18nWrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *i18nWrappedStream) Context() context.Context {
	return w.ctx
}
