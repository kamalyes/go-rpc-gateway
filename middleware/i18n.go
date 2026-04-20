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
	gci18n "github.com/kamalyes/go-config/pkg/i18n"
	goi18n "github.com/kamalyes/go-i18n"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"net/http"
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
	return r.URL.Query().Get(config.LanguageParam)
}

func detectFromCookie(r *http.Request, config *gci18n.I18N) string {
	cookie, err := r.Cookie(config.LanguageParam)
	if err != nil {
		return ""
	}
	return cookie.Value
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
