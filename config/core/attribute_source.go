/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 15:05:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 15:05:00
 * @FilePath: \go-rpc-gateway\config\core\attribute_source.go
 * @Description:
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package core

import (
	"net/http"
	"net/textproto"
	"net/url"

	validator "github.com/kamalyes/go-toolbox/pkg/argus"
)

// AttributeSourceType 属性提取来源类型
type AttributeSourceType string

const (
	// SourceTypeQuery 从 URL 查询参数提取
	SourceTypeQuery AttributeSourceType = "query"
	// SourceTypeHeader 从 HTTP Header 提取
	SourceTypeHeader AttributeSourceType = "header"
	// SourceTypeCookie 从 Cookie 提取
	SourceTypeCookie AttributeSourceType = "cookie"
)

// IsValid 验证来源类型是否有效
func (t AttributeSourceType) IsValid() bool {
	switch t {
	case SourceTypeQuery, SourceTypeHeader, SourceTypeCookie:
		return true
	default:
		return false
	}
}

// String 返回字符串表示
func (t AttributeSourceType) String() string {
	return string(t)
}

// AttributeSource 属性提取来源配置
type AttributeSource struct {
	Type AttributeSourceType `mapstructure:"type" yaml:"type" json:"type" validate:"required,oneof=query header cookie"` // 来源类型: query, header, cookie
	Key  string              `mapstructure:"key" yaml:"key" json:"key" validate:"required"`                              // 提取的字段名
}

// Validate 验证属性来源配置
func (a *AttributeSource) Validate() error {
	if !a.Type.IsValid() {
		return validator.New().Var(a.Type, "oneof=query header cookie")
	}
	if a.Key == "" {
		return validator.New().Var(a.Key, "required")
	}
	return nil
}

// AttributeExtractor 请求级的属性提取器：把一次请求里重复的解析各收敛成一次
//
// 为什么需要它：query 与 cookie 的提取在标准库里都是「每次调用重新解析」——
//
//	r.URL.Query() 每次都跑一遍 url.ParseQuery(RawQuery) 并新建一个 url.Values
//	r.Cookie(name) 每次都重解一遍整个 Cookie 头
//
// 单看一次调用没问题，但真实调用方是「一个属性一次 ExtractAttribute」：
// 网关要取 28 个属性，默认来源清单里有 43 个 query 来源与 25 个 cookie 来源，
// 于是同一个查询串每请求被解析几十遍
//
// 实测（go-config/pkg/gateway 的 BenchmarkExtractAttribute_DefaultSources，
// 按真实默认配置取齐 28 个属性）：带查询串时 14.2 微秒、327 次分配，
// 其中 url.ParseQuery 占调用方那条中间件全部分配对象的 57%
//
// 用法：一次请求建一个提取器，28 个属性共用它
//
//	e := core.NewAttributeExtractor(r)
//	traceID := e.Extract(cfg.TraceIDSources)
//	userID  := e.Extract(cfg.UserIDSources)
//
// 语义与 ExtractAttribute 完全一致：按顺序取第一个非空值
//
// 并发：提取器是请求级的，不做加锁，不要跨 goroutine 共享
//
// [EN] A request-scoped extractor that collapses the repeated parsing into one pass each. Both
// r.URL.Query() and r.Cookie(name) re-parse from scratch on every call, which is fine for one
// call but not for the real usage pattern of one ExtractAttribute call per attribute: a gateway
// reads 28 attributes while the default source lists hold 43 query and 25 cookie sources, so the
// same query string is parsed dozens of times per request. Semantics are unchanged -- the first
// non-empty source wins. The extractor is request-scoped and unsynchronized
type AttributeExtractor struct {
	r *http.Request

	query     url.Values
	queryDone bool

	// batchCookies 决定 cookie 怎么查：建整表还是按名字单查
	//
	// 这个区分是量出来的，不是设计出来的：r.Cookie(name) 带名字过滤，只为匹配项分配；
	// 而建整表要把每个 cookie 都解析成 *http.Cookie 再塞进 map
	// 一个来源清单里最多只有 1 个 cookie 来源，所以 ExtractAttribute 那条路上建整表是纯亏 ——
	// 实测慢 2.3 倍、分配从 78 涨到 182
	// 整表只在提取器跨多个属性复用时才划算，那时一次解析答 25 次查找
	//
	// [EN] Measured rather than designed: r.Cookie(name) filters by name and allocates only for
	// a match, while building the whole table parses every cookie into a *http.Cookie first. A
	// single source list holds at most one cookie source, so the table is a pure loss on the
	// per-call path -- 2.3x slower, 78 allocations becoming 182. It only pays off when the
	// extractor is reused across attributes, where one parse answers 25 lookups
	batchCookies bool
	cookies      map[string]string
	cookiesDone  bool
}

// NewAttributeExtractor 为一次请求创建提取器，跨多个属性复用
func NewAttributeExtractor(r *http.Request) *AttributeExtractor {
	return &AttributeExtractor{r: r, batchCookies: true}
}

// Extract 按优先级提取属性：顺序取第一个非空值
func (e *AttributeExtractor) Extract(sources []AttributeSource) string {
	for i := range sources {
		if value := e.FromSource(sources[i]); value != "" {
			return value
		}
	}
	return ""
}

// FromSource 从指定来源提取值
func (e *AttributeExtractor) FromSource(source AttributeSource) string {
	switch source.Type {
	case SourceTypeQuery:
		return e.queryValues().Get(source.Key)
	case SourceTypeHeader:
		return e.r.Header.Get(source.Key)
	case SourceTypeCookie:
		return e.cookieValue(source.Key)
	default:
		return ""
	}
}

// queryValues 惰性解析查询串，整个请求只解析一次
func (e *AttributeExtractor) queryValues() url.Values {
	if !e.queryDone {
		e.queryDone = true
		if raw := e.r.URL.RawQuery; raw != "" {
			// 与 r.URL.Query() 保持一致：ParseQuery 的错误被吞掉，返回已解析出的部分
			// [EN] Matches r.URL.Query(), which discards the parse error and keeps what parsed
			e.query, _ = url.ParseQuery(raw)
		}
		// 查询串为空时留 nil：url.Values(nil).Get 安全返回空串，省掉一个空 map
		// 这一条是常态 —— 大多数请求根本没有查询串，而 ParseQuery("") 仍会 make 一个 map
		// [EN] Left nil when there is no query string: reading a nil map is safe and this avoids
		// the empty map that ParseQuery("") allocates anyway, which is the common case
	}
	return e.query
}

// cookieValue 取 cookie 值：复用模式走整表，单次模式走标准库的按名字查找
func (e *AttributeExtractor) cookieValue(name string) string {
	if !e.batchCookies {
		return singleCookie(e.r, name)
	}
	if !e.cookiesDone {
		e.cookiesDone = true
		e.cookies = cookieTable(e.r)
	}
	return e.cookies[name]
}

// singleCookie 按名字单查：标准库会按名字过滤，只为匹配项分配
// [EN] A single lookup: the standard library filters by name and allocates only for a match
func singleCookie(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// cookieTable 把 Cookie 头解析成一张表，供一次请求里的多次查找共用
//
// 没有 Cookie 头或解析不出条目时返回 nil：读 nil map 安全返回零值，
// 于是既省掉一次解析也省掉一个 map
// [EN] Returns nil when there is no Cookie header or nothing parses: reading a nil map yields
// the zero value, so this skips both the parse and the map allocation
func cookieTable(r *http.Request) map[string]string {
	if r.Header.Get("Cookie") == "" {
		return nil
	}
	cookies := r.Cookies()
	if len(cookies) == 0 {
		return nil
	}
	table := make(map[string]string, len(cookies))
	for _, c := range cookies {
		// 与 r.Cookie(name) 保持一致：同名取第一个
		// [EN] Matches r.Cookie(name), which returns the first cookie of that name
		if _, dup := table[c.Name]; !dup {
			table[c.Name] = c.Value
		}
	}
	return table
}

// NormalizeKey 把 header 来源的 Key 归一成 HTTP 规范形式（x-user-id -> X-User-Id）
//
// 为什么要在加载后做一次：Header.Get 每次查找都会对 key 调 CanonicalMIMEHeaderKey，
// key 本来就是规范形式时它原样返回、不分配；不是的话每次查找都要现算并分配一个新字符串
// 配置是加载一次、查找千万次，所以这笔归一化该在加载时付一次，而不是让每次查找各付一遍
//
// 两种拼法的查找结果完全一样（都归到同一个 header），所以这是纯粹的削减
//
// 实测：提取器修掉重复解析之后，canonicalMIMEHeaderKey 占了 header 路径剩余分配的 100%；
// key 归一之后该路径降到零分配
//
// 非 header 来源不动：query 与 cookie 的键名是大小写敏感的，规范化会改变语义
//
// [EN] Canonicalizes a header source's key once after load. Header.Get canonicalizes on every
// lookup and returns the key untouched only when it is already canonical, so this cost belongs
// at load time rather than on every one of millions of lookups. Both spellings resolve to the
// same header, making this a pure saving. Query and cookie keys are left alone because they are
// case-sensitive and normalizing them would change meaning
func (a *AttributeSource) NormalizeKey() {
	if a.Type != SourceTypeHeader {
		return
	}
	a.Key = textproto.CanonicalMIMEHeaderKey(a.Key)
}

// ExtractAttribute 从请求中按优先级提取属性
//
// 取单个属性时用这个就够了。要取多个属性请改用 NewAttributeExtractor：
// 这里每次调用都会重建提取器，于是查询串与 Cookie 头在每次调用里各解析一次，
// 调 28 次就解析 28 次
// [EN] Enough for a single attribute. For several attributes use NewAttributeExtractor instead:
// this rebuilds the extractor per call, so the query string is parsed once per call
func ExtractAttribute(r *http.Request, sources []AttributeSource) string {
	// 取值而不是 New：不逃逸时编译器可以把它留在栈上，包装层本身不产生分配
	// [EN] A value rather than New so escape analysis can keep it on the stack
	e := AttributeExtractor{r: r}
	return e.Extract(sources)
}

// ExtractFromSource 从指定来源提取值
func ExtractFromSource(r *http.Request, source AttributeSource) string {
	e := AttributeExtractor{r: r}
	return e.FromSource(source)
}
