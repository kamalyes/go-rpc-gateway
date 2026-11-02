/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 15:13:29
 * @FilePath: \go-rpc-gateway\middleware\i18n\i18n.go
 * @Description: 国际化i18n中间件
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package i18n

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	gci18n "github.com/kamalyes/go-config/pkg/i18n"
	goi18n "github.com/kamalyes/go-i18n"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/middleware/core"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type I18nManager = goi18n.Manager

// toI18nConfig 把 go-config 的 i18n 配置转为 go-i18n 自有的 Config
//
// go-i18n 自 v0.3.0 起不再依赖 go-config（零依赖重构），并且刻意只保留翻译引擎
// 真正会读的字段：detection-order / language-param / language-header / cookie-name
// 这几项是「从请求的哪里取语言」，属于 HTTP 中间件的职责，也就是本包
// 因此这次转换只在边界上做一次：翻译相关字段交给 go-i18n，检测相关字段留在本包
//
// [EN] go-i18n dropped its go-config dependency in v0.3.0 and deliberately keeps
// only the fields its translation engine reads; the detection fields describe
// where a request's language comes from, which is this middleware's job, so the
// conversion happens once at the boundary
func toI18nConfig(c *gci18n.I18N) *goi18n.Config {
	if c == nil {
		return nil
	}

	order := make([]goi18n.MappingType, 0, len(c.ResolutionOrder))
	for _, m := range c.ResolutionOrder {
		order = append(order, goi18n.MappingType(m))
	}

	return &goi18n.Config{
		Enabled:               c.Enabled,
		DefaultLanguage:       c.DefaultLanguage,
		SupportedLanguages:    c.SupportedLanguages,
		LanguageMapping:       c.LanguageMapping,
		LegacyLanguageMapping: c.LegacyLanguageMapping,
		ResolutionOrder:       order,
		MessagesPath:          c.MessagesPath,
		EnableFallback:        c.EnableFallback,
		MessageLoader:         c.MessageLoader,
	}
}

func NewI18nManager(config *gci18n.I18N) (*goi18n.Manager, error) {
	return goi18n.NewManager(toI18nConfig(config))
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

func I18n() core.MiddlewareFunc {
	config := gci18n.Default()
	manager, err := goi18n.NewManager(toI18nConfig(config))
	if err != nil {
		panic(fmt.Sprintf("failed to create i18n manager: %v", err))
	}

	return I18nWithManager(manager, config)
}

// I18nWithManager 返回国际化中间件
//
// detect 为可变参数，用于指定「从请求的哪里取语言」的检测配置（Header / Query /
// Cookie 顺序与键名）；不传则用 gci18n.Default()
// 之所以不再从 manager.GetConfig() 取：go-i18n v0.3.0 的 Config 已不含检测字段，
// 那些字段属于本中间件而非翻译引擎
// [EN] detect carries the request-language detection settings; it is no longer
// read from manager.GetConfig() because go-i18n's Config no longer holds them
func I18nWithManager(manager *goi18n.Manager, detect ...*gci18n.I18N) core.MiddlewareFunc {
	detectConfig := gci18n.Default()
	if len(detect) > 0 && detect[0] != nil {
		detectConfig = detect[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			language := detectLanguage(r, detectConfig, manager.GetConfig())

			if !manager.IsLanguageSupported(language) {
				language = manager.GetDefaultLanguage()
			}

			w.Header().Set(constants.HeaderContentLanguage, language)

			ctx := goi18n.NewContext(r.Context(), language, manager)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// detectLanguage 按配置顺序从请求中探测语言
//
// 两个配置参数按字段归属拆开：
//   - detect 提供「从请求的哪里取」（DetectionOrder / LanguageHeader /
//     LanguageParam / CookieName），这些字段只存在于 go-config 的 i18n 配置里
//   - resolve 提供「取到之后怎么映射」（标准映射、遗留映射、解析顺序、默认语言），
//     来自 manager.GetConfig()，也就是调用方实际传入的那份配置
//
// 之所以不能两者都用 detect：go-i18n v0.3.0 的 Config 不再含检测字段，而映射配置
// 必须用调用方的那份，否则自定义的遗留映射会被默认配置悄悄吞掉
// [EN] The two configs are split by ownership: detect says where to read the
// language from, resolve says how to map it. Using detect for both would silently
// drop a caller's custom legacy mappings
func detectLanguage(r *http.Request, detect *gci18n.I18N, resolve *goi18n.Config) string {
	for _, method := range detect.DetectionOrder {
		switch method {
		case gci18n.DetectionHeader:
			if lang := detectFromHeader(r, detect, resolve); lang != "" {
				return lang
			}
		case gci18n.DetectionQuery:
			if lang := detectFromQuery(r, detect, resolve); lang != "" {
				return lang
			}
		case gci18n.DetectionCookie:
			if lang := detectFromCookie(r, detect, resolve); lang != "" {
				return lang
			}
		case gci18n.DetectionDefault:
			return resolve.DefaultLanguage
		}
	}
	return resolve.DefaultLanguage
}

func detectFromHeader(r *http.Request, detect *gci18n.I18N, resolve *goi18n.Config) string {
	acceptLanguage := r.Header.Get(detect.LanguageHeader)
	if acceptLanguage == "" {
		return ""
	}

	return resolve.ParseAcceptLanguage(acceptLanguage)
}

// queryValue 轻量提取单个查询参数值（避免 r.URL.Query() 解析全部参数并分配 map/slice）
func queryValue(rawQuery, key string) string {
	for len(rawQuery) > 0 {
		pair := rawQuery
		if idx := strings.IndexByte(rawQuery, '&'); idx >= 0 {
			pair, rawQuery = rawQuery[:idx], rawQuery[idx+1:]
		} else {
			rawQuery = ""
		}
		if strings.HasPrefix(pair, key) && len(pair) > len(key) && pair[len(key)] == '=' {
			if value, err := url.QueryUnescape(pair[len(key)+1:]); err == nil {
				return value
			}
			return ""
		}
	}
	return ""
}

func detectFromQuery(r *http.Request, detect *gci18n.I18N, resolve *goi18n.Config) string {
	if lang := queryValue(r.URL.RawQuery, detect.LanguageParam); lang != "" {
		return resolve.ResolveLanguage(lang)
	}
	return ""
}

func detectFromCookie(r *http.Request, detect *gci18n.I18N, resolve *goi18n.Config) string {
	cookieName := mathx.IfEmpty(detect.CookieName, detect.LanguageParam)

	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return ""
	}
	return resolve.ResolveLanguage(cookie.Value)
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
