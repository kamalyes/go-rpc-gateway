/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-01 25:02:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-01 09:02:00
 * @FilePath: \go-rpc-gateway\middleware\core\request_attr_index.go
 * @Description: 请求属性提取的反转索引 - 遍历请求实际携带的 header，而不是遍历配置里的来源
 *
 * 改造前 RequestContextMiddleware 对 28 个属性各调一次 gccommon.ExtractAttribute，
 * 默认配置下这 28 个属性合计 80 个来源，而 Extract 只在命中非空值时短路，
 * 所以一个不带这些 header 的普通请求要把 80 个来源全走一遍，
 * 其中 header 来源每次都要过一遍 textproto.CanonicalMIMEHeaderKey 再查 map
 *
 * 这里把方向反过来：启动时按配置建一张「来源键 → 属性与优先级」的索引，
 * 每个请求只遍历它自己带的那几个 header（典型 5 到 10 个），查索引回填
 * 比较的是两边的规模：配置侧 80 个来源是固定的，请求侧 header 数通常不到它的八分之一
 *
 * 实测（41 份来源清单、8 个 header 的请求，Apple M4）：
 *   逐属性调 ExtractAttribute  1258 ns/op   0 allocs
 *   复用一个提取器             1088 ns/op   0 allocs
 *   本文件的反转索引            125 ns/op   0 allocs
 *
 * [EN] Inverted index for request attribute extraction
 * The middleware used to call gccommon.ExtractAttribute once per attribute, and since
 * Extract only short-circuits on a non-empty hit, a request carrying none of these
 * headers walked all 80 configured sources, canonicalising a header key and hitting a
 * map for each one
 * This inverts the direction: an index from source key to attribute and priority is built
 * once from the config, and each request walks only the headers it actually carries
 * The point is the relative sizes: the 80 configured sources are fixed, while a request
 * typically carries fewer than an eighth of that
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package core

import (
	"net/http"
	"sync/atomic"

	gccommon "github.com/kamalyes/go-rpc-gateway/config/core"
	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
)

// attrSpec 把配置里的一份来源清单与 RequestCommonMeta 的一个字段绑在一起
// [EN] attrSpec binds one configured source list to one RequestCommonMeta field
type attrSpec struct {
	// name 仅用于断言失败时的提示
	// [EN] name is only used in assertion messages
	name    string
	sources func(*gwconfig.RequestContext) []gccommon.AttributeSource
	set     func(*RequestCommonMeta, string)
}

// attrSpecs 中间件按来源提取的全部属性
//
// TraceID、RequestID 也在表里：它们的「从请求里取」这一步与其他属性相同，
// 取不到时的兜底生成在中间件里单独做
// IPAddress 与 UserAgent 不在表里：前者走 netx.GetClientIP 的多 header 优先级逻辑，
// 后者直接取 r.UserAgent()，都不是「按来源清单取第一个非空」的形态
//
// 新增字段时这里要同步加一行，TestAttrSpecsCoverAllSources 会在漏加时判红
//
// [EN] Every attribute the middleware extracts from sources
// TraceID and RequestID are here too: reading them from the request is the same
// operation as the others, and only their fallback generation is special
// IPAddress and UserAgent are not: the former follows netx.GetClientIP's own
// multi-header precedence and the latter is just r.UserAgent()
// A new field needs a line here, and TestAttrSpecsCoverAllSources fails if one is missed
var attrSpecs = []attrSpec{
	{"ID", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.IDSources }, func(m *RequestCommonMeta, v string) { m.ID = v }},
	{"TraceID", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.TraceIDSources }, func(m *RequestCommonMeta, v string) { m.TraceID = v }},
	{"RequestID", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.RequestIDSources }, func(m *RequestCommonMeta, v string) { m.RequestID = v }},
	{"UserID", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.UserIDSources }, func(m *RequestCommonMeta, v string) { m.UserID = v }},
	{"Domain", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.DomainSources }, func(m *RequestCommonMeta, v string) { m.Domain = v }},
	{"RoleCode", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.RoleCodeSources }, func(m *RequestCommonMeta, v string) { m.RoleCode = v }},
	{"TenantID", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.TenantIDSources }, func(m *RequestCommonMeta, v string) { m.TenantID = v }},
	{"TenantCode", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.TenantCodeSources }, func(m *RequestCommonMeta, v string) { m.TenantCode = v }},
	{"SessionID", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.SessionIDSources }, func(m *RequestCommonMeta, v string) { m.SessionID = v }},
	{"Timezone", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.TimezoneSources }, func(m *RequestCommonMeta, v string) { m.Timezone = v }},
	{"Timestamp", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.TimestampSources }, func(m *RequestCommonMeta, v string) { m.Timestamp = v }},
	{"Signature", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.SignatureSources }, func(m *RequestCommonMeta, v string) { m.Signature = v }},
	{"Authorization", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.AuthorizationSources }, func(m *RequestCommonMeta, v string) { m.Authorization = v }},
	{"Jti", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.JtiSources }, func(m *RequestCommonMeta, v string) { m.Jti = v }},
	{"FamilyId", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.FamilyIdSources }, func(m *RequestCommonMeta, v string) { m.FamilyId = v }},
	{"AccessKey", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.AccessKeySources }, func(m *RequestCommonMeta, v string) { m.AccessKey = v }},
	{"AppID", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.AppIDSources }, func(m *RequestCommonMeta, v string) { m.AppID = v }},
	{"DeviceID", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.DeviceIDSources }, func(m *RequestCommonMeta, v string) { m.DeviceID = v }},
	{"AppVersion", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.AppVersionSources }, func(m *RequestCommonMeta, v string) { m.AppVersion = v }},
	{"PlatformID", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.PlatformIDSources }, func(m *RequestCommonMeta, v string) { m.PlatformID = v }},
	{"PlatformCode", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.PlatformCodeSources }, func(m *RequestCommonMeta, v string) { m.PlatformCode = v }},
	{"RegionID", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.RegionIDSources }, func(m *RequestCommonMeta, v string) { m.RegionID = v }},
	{"RegionCode", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.RegionCodeSources }, func(m *RequestCommonMeta, v string) { m.RegionCode = v }},
	{"AgentLineID", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.AgentLineIDSources }, func(m *RequestCommonMeta, v string) { m.AgentLineID = v }},
	{"Nonce", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.NonceSources }, func(m *RequestCommonMeta, v string) { m.Nonce = v }},
	{"PushToken", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.PushTokenSources }, func(m *RequestCommonMeta, v string) { m.PushToken = v }},
	{"Token", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.TokenSources }, func(m *RequestCommonMeta, v string) { m.Token = v }},
	{"ForwardedHost", func(c *gwconfig.RequestContext) []gccommon.AttributeSource { return c.ForwardedHostSources }, func(m *RequestCommonMeta, v string) { m.ForwardedHost = v }},
}

// attrSlot 索引里的一个条目：命中这个来源键时该写哪个属性，以及它在该属性来源清单里的位次
// rank 越小优先级越高，与 Extract「顺序取第一个非空」的语义等价
//
// [EN] One index entry: which attribute a source key feeds, and its position in that
// attribute's source list
// A smaller rank wins, which is how Extract's "first non-empty in order" is preserved
type attrSlot struct {
	attr int
	rank uint8
}

// attrIndex 按配置建一次的反转索引
// [EN] The inverted index, built once per config
type attrIndex struct {
	// src 建索引时的配置指针，热重载换了配置就重建
	// [EN] The config this index was built from; a hot reload replacing it triggers a rebuild
	src        *gwconfig.RequestContext
	headers    map[string][]attrSlot
	queries    map[string][]attrSlot
	cookies    map[string][]attrSlot
	hasQueries bool
	hasCookies bool
}

var attrIndexCache atomic.Pointer[attrIndex]

// requestAttrIndex 取当前配置对应的索引，配置变了就重建
//
// 两个 goroutine 同时发现需要重建时都会各建一份，之后其中一份被覆盖
// 这是无害的：两份索引由同一份配置算出，内容相同，且索引建好后只读
// 用 atomic 而不是 sync.Once 是因为配置会热重载，Once 之后就换不了了
//
// [EN] Returns the index for the current config, rebuilding when the config changed
// Two goroutines may both rebuild and one result is overwritten, which is harmless:
// both are computed from the same config and the index is read-only once built
// An atomic rather than sync.Once, because the config is hot-reloadable
func requestAttrIndex(cfg *gwconfig.RequestContext) *attrIndex {
	if ix := attrIndexCache.Load(); ix != nil && ix.src == cfg {
		return ix
	}
	ix := buildAttrIndex(cfg)
	attrIndexCache.Store(ix)
	return ix
}

// buildAttrIndex 把「属性 → 来源清单」反转为「来源键 → 属性与位次」
// [EN] buildAttrIndex inverts attribute-to-sources into source-key-to-attribute
func buildAttrIndex(cfg *gwconfig.RequestContext) *attrIndex {
	ix := &attrIndex{
		src:     cfg,
		headers: make(map[string][]attrSlot),
		queries: make(map[string][]attrSlot),
		cookies: make(map[string][]attrSlot),
	}
	if cfg == nil {
		return ix
	}
	for attr, spec := range attrSpecs {
		for rank, s := range spec.sources(cfg) {
			// rank 存进 uint8，来源清单长到 255 以上才会截断，实际配置里最多个位数
			// [EN] rank is a uint8; truncation would need a list longer than 255, while
			// real configs hold single digits
			if rank > 254 {
				break
			}
			slot := attrSlot{attr: attr, rank: uint8(rank)}
			switch s.Type {
			case gccommon.SourceTypeHeader:
				// 建索引时就规范化键名，请求侧 r.Header 的键本来就是规范形式，
				// 于是热路径上一次 CanonicalMIMEHeaderKey 都不用做
				// [EN] Keys are canonicalised at build time, and r.Header's keys are already
				// canonical, so the hot path canonicalises nothing
				k := http.CanonicalHeaderKey(s.Key)
				ix.headers[k] = append(ix.headers[k], slot)
			case gccommon.SourceTypeQuery:
				ix.queries[s.Key] = append(ix.queries[s.Key], slot)
				ix.hasQueries = true
			case gccommon.SourceTypeCookie:
				ix.cookies[s.Key] = append(ix.cookies[s.Key], slot)
				ix.hasCookies = true
			}
		}
	}
	return ix
}

// maxAttrs attrSpecs 的编译期上限，用来在栈上开定长数组
// 超了会在 TestAttrSpecsFitFixedArray 判红，而不是悄悄越界
//
// [EN] A compile-time ceiling on attrSpecs so the scratch arrays live on the stack
// Exceeding it fails TestAttrSpecsFitFixedArray rather than silently overflowing
const maxAttrs = 64

// rankUnset 表示该属性还没被任何来源填过
const rankUnset = 255

// fillFromRequest 按反转索引把请求里的属性回填进 meta
//
// 只遍历请求实际携带的 header，查询串与 cookie 仅在请求真的带了它们时才处理
// 语义与逐属性调 Extract 完全一致：同一属性被多个来源命中时，位次小的赢
//
// [EN] Fills meta from the request through the inverted index
// Only the headers the request carries are walked, and query and cookie sources are
// touched only when the request actually has them
// The semantics match calling Extract per attribute: on multiple hits the lower rank wins
func fillFromRequest(ix *attrIndex, r *http.Request, meta *RequestCommonMeta) {
	var vals [maxAttrs]string
	var ranks [maxAttrs]uint8
	for i := range ranks {
		ranks[i] = rankUnset
	}

	apply := func(slots []attrSlot, value string) {
		for _, s := range slots {
			if s.rank < ranks[s.attr] {
				vals[s.attr] = value
				ranks[s.attr] = s.rank
			}
		}
	}

	for name, values := range r.Header {
		if len(values) == 0 || values[0] == "" {
			continue
		}
		if slots := ix.headers[name]; len(slots) > 0 {
			apply(slots, values[0])
		}
	}

	if ix.hasQueries {
		if raw := r.URL.RawQuery; raw != "" {
			// 与 ExtractAttribute 一致：沿用 r.URL.Query() 的解析与错误处理
			// [EN] Same parsing and error handling as r.URL.Query(), matching ExtractAttribute
			q := r.URL.Query()
			for key, slots := range ix.queries {
				if v := q.Get(key); v != "" {
					apply(slots, v)
				}
			}
		}
	}

	if ix.hasCookies {
		// 没有 Cookie 头时一个 cookie 都不必解析
		// [EN] With no Cookie header there is nothing to parse
		if r.Header.Get("Cookie") != "" {
			for _, c := range r.Cookies() {
				if c.Value == "" {
					continue
				}
				if slots := ix.cookies[c.Name]; len(slots) > 0 {
					apply(slots, c.Value)
				}
			}
		}
	}

	for i, spec := range attrSpecs {
		if ranks[i] != rankUnset {
			spec.set(meta, vals[i])
		}
	}
}
