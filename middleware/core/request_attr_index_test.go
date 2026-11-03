/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-01 04:12:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-01 04:12:00
 * @FilePath: \go-rpc-gateway\middleware\core\request_attr_index_test.go
 * @Description: 反转索引与逐属性提取的等价性测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package core

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	gccommon "github.com/kamalyes/go-config/pkg/common"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// legacyFill 改造前的做法：对每个属性各调一次包级 ExtractAttribute
// 留作等价性比对的参照实现，别删
//
// [EN] The pre-change behaviour: one package-level ExtractAttribute call per attribute
// Kept as the reference implementation for the equivalence test
func legacyFill(cfg *gwconfig.RequestContext, r *http.Request, meta *RequestCommonMeta) {
	for _, spec := range attrSpecs {
		if v := gccommon.ExtractAttribute(r, spec.sources(cfg)); v != "" {
			spec.set(meta, v)
		}
	}
}

// TestAttrSpecsFitFixedArray attrSpecs 不能超过栈上定长数组的容量
// 超了会越界，所以用测试挡住而不是靠人记得
//
// [EN] attrSpecs must fit the fixed stack arrays; a test guards this rather than memory
func TestAttrSpecsFitFixedArray(t *testing.T) {
	assert.LessOrEqual(t, len(attrSpecs), maxAttrs,
		"attrSpecs 有 %d 项，超过 maxAttrs=%d，请调大 maxAttrs", len(attrSpecs), maxAttrs)
	assert.Less(t, len(attrSpecs), int(rankUnset),
		"属性个数必须小于 rankUnset，否则 rank 与「未设置」无法区分")
}

// TestAttrSpecsCoverAllSources 配置里每一份 []AttributeSource 都该在 attrSpecs 里有对应项，
// 或者被显式列为「中间件不从来源提取」
// 新增配置字段却忘了加 spec 时，这个测试会指名道姓地红
//
// [EN] Every []AttributeSource field in the config must map to an attrSpec entry, or be
// explicitly listed as not source-extracted by the middleware
// Adding a config field without a spec fails this test by name
func TestAttrSpecsCoverAllSources(t *testing.T) {
	// 中间件不按来源清单提取的字段，分两类：
	//
	// 一类是提取方式不同：IPAddress 走 netx.GetClientIP 自己的多 header 优先级，
	// UserAgent 直接取标准库解析结果，另外几项只在 gRPC metadata 路径用到
	//
	// 另一类是 RequestCommonMeta 里压根没有对应字段，配置里这些键目前无处落地 ——
	// 改造前逐属性提取的那一版也没有读它们，不是本次改动引入的
	// 要么给 RequestCommonMeta 补字段，要么从 go-config 的 RequestContext 里摘掉这些键，
	// 现状是配置可以写、写了没有任何效果
	//
	// [EN] Fields the middleware does not extract from a source list, in two groups
	// One is extracted differently: IPAddress follows netx.GetClientIP's own multi-header
	// precedence, UserAgent comes from the standard library, and a few are only used on
	// the gRPC metadata path
	// The other has no field in RequestCommonMeta at all, so these config keys have
	// nowhere to land; the pre-change per-attribute version did not read them either, so
	// this is not introduced here
	// Either RequestCommonMeta gains the fields or go-config drops the keys; as it stands
	// they are configurable with no effect
	notSourceExtracted := map[string]bool{
		// 提取方式不同
		"IPAddressSources":      true,
		"UserAgentSources":      true,
		"UserNameSources":       true,
		"AuthPayloadSources":    true,
		"LanguageSources":       true,
		"AcceptLanguageSources": true,

		// RequestCommonMeta 无对应字段
		"AcceptSources":         true,
		"CacheControlSources":   true,
		"ConnectionSources":     true,
		"RealIPSources":         true,
		"ForwardedForSources":   true,
		"ForwardedProtoSources": true,
		"ClientIDSources":       true,
		"UserTypeSources":       true,
		"PlatformSources":       true,
		"OriginSources":         true,
		"CSRFTokenSources":      true,
	}

	covered := make(map[string]bool, len(attrSpecs))
	cfg := gwconfig.Default().RequestContext
	cv := reflect.ValueOf(cfg).Elem()
	ct := cv.Type()
	srcType := reflect.TypeOf([]gccommon.AttributeSource{})

	// 用「spec 取出的切片与配置字段是同一个底层数组」来判定对应关系，
	// 比按名字拼字符串可靠
	// [EN] Correspondence is decided by identity of the underlying slice rather than by
	// reassembling names from strings
	for _, spec := range attrSpecs {
		got := spec.sources(cfg)
		require.NotNil(t, got, "spec %s 取到的来源清单为 nil", spec.name)
		for i := 0; i < ct.NumField(); i++ {
			if ct.Field(i).Type != srcType {
				continue
			}
			field := cv.Field(i).Interface().([]gccommon.AttributeSource)
			if len(field) > 0 && len(got) > 0 && &field[0] == &got[0] {
				covered[ct.Field(i).Name] = true
			}
		}
	}

	var missing []string
	for i := 0; i < ct.NumField(); i++ {
		name := ct.Field(i).Name
		if ct.Field(i).Type != srcType || notSourceExtracted[name] {
			continue
		}
		if !covered[name] {
			missing = append(missing, name)
		}
	}
	assert.Empty(t, missing,
		"配置里这些来源清单没有对应的 attrSpec，中间件会静默不提取它们: %v", missing)
}

// equivalenceCases 覆盖 header、查询串、cookie、优先级与空请求
func equivalenceCases() []struct {
	name    string
	prepare func(*http.Request)
} {
	return []struct {
		name    string
		prepare func(*http.Request)
	}{
		{"空请求", func(r *http.Request) {}},
		{"仅 Content-Type", func(r *http.Request) {
			r.Header.Set("Content-Type", "application/json")
		}},
		{"典型 8 个 header", func(r *http.Request) {
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("User-Agent", "bench/1.0")
			r.Header.Set("X-Request-Id", "req-1")
			r.Header.Set("X-Trace-Id", "trace-1")
			r.Header.Set("Authorization", "Bearer tok")
			r.Header.Set("X-Tenant-Id", "t-1")
			r.Header.Set("X-Forwarded-For", "10.0.0.1")
			r.Header.Set("Accept-Encoding", "gzip")
		}},
		{"header 大小写不规范", func(r *http.Request) {
			r.Header["x-request-id"] = []string{"lower-case-key"}
			r.Header.Set("X-Trace-Id", "trace-1")
		}},
		{"仅查询串", func(r *http.Request) {
			r.URL.RawQuery = "request_id=q-req&trace_id=q-trace&tenant_id=q-tenant"
		}},
		{"仅 cookie", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: "authorization", Value: "cookie-tok"})
			r.AddCookie(&http.Cookie{Name: "session_id", Value: "cookie-sess"})
		}},
		{"header 与查询串同时命中，header 优先", func(r *http.Request) {
			r.Header.Set("X-Request-Id", "from-header")
			r.URL.RawQuery = "request_id=from-query"
		}},
		{"查询串与 cookie 同时命中", func(r *http.Request) {
			r.URL.RawQuery = "authorization=from-query"
			r.AddCookie(&http.Cookie{Name: "authorization", Value: "from-cookie"})
		}},
		{"三种来源全命中", func(r *http.Request) {
			r.Header.Set("Authorization", "from-header")
			r.URL.RawQuery = "authorization=from-query"
			r.AddCookie(&http.Cookie{Name: "authorization", Value: "from-cookie"})
		}},
		{"空值 header 不应覆盖", func(r *http.Request) {
			r.Header["X-Request-Id"] = []string{""}
			r.URL.RawQuery = "request_id=from-query"
		}},
		{"同名 header 多值取第一个", func(r *http.Request) {
			r.Header["X-Request-Id"] = []string{"first", "second"}
		}},
	}
}

// TestFillFromRequestMatchesLegacy 反转索引与逐属性提取必须产出逐字段相同的结果
//
// 这是整个改造的正确性闸门：性能换来的 10 倍只有在语义不变时才算数
// 比的是整个 RequestCommonMeta 结构体，不是挑几个字段看
//
// [EN] The inverted index must produce a field-for-field identical result
// This is the correctness gate for the change: a 10x speedup only counts if the
// semantics are unchanged, so the whole RequestCommonMeta is compared, not a few fields
func TestFillFromRequestMatchesLegacy(t *testing.T) {
	cfg := gwconfig.Default().RequestContext
	ix := buildAttrIndex(cfg)

	for _, tc := range equivalenceCases() {
		t.Run(tc.name, func(t *testing.T) {
			rNew := httptest.NewRequest(http.MethodPost, "/x", nil)
			tc.prepare(rNew)
			rOld := httptest.NewRequest(http.MethodPost, "/x", nil)
			tc.prepare(rOld)

			var got, want RequestCommonMeta
			fillFromRequest(ix, rNew, &got)
			legacyFill(cfg, rOld, &want)

			assert.Equal(t, want, got, "反转索引与逐属性提取结果不一致")
		})
	}
}

// TestFillFromRequestAllSingleHeaders 对每个 header 来源单独构造一个请求，
// 确认每一条来源都真的被索引覆盖
// 用例表里的组合是抽样，这里是逐条穷举
//
// [EN] One request per header source, exhaustively, confirming every single source is
// actually covered by the index, where the case table above only samples combinations
func TestFillFromRequestAllSingleHeaders(t *testing.T) {
	cfg := gwconfig.Default().RequestContext
	ix := buildAttrIndex(cfg)

	checked := 0
	for _, spec := range attrSpecs {
		for _, src := range spec.sources(cfg) {
			if src.Type != gccommon.SourceTypeHeader {
				continue
			}
			checked++
			t.Run(fmt.Sprintf("%s/%s", spec.name, src.Key), func(t *testing.T) {
				value := "v-" + src.Key
				rNew := httptest.NewRequest(http.MethodPost, "/x", nil)
				rNew.Header.Set(src.Key, value)
				rOld := httptest.NewRequest(http.MethodPost, "/x", nil)
				rOld.Header.Set(src.Key, value)

				var got, want RequestCommonMeta
				fillFromRequest(ix, rNew, &got)
				legacyFill(cfg, rOld, &want)
				assert.Equal(t, want, got)
			})
		}
	}
	assert.Positive(t, checked, "没有逐条检查到任何 header 来源，用例本身失效了")
	t.Logf("逐条检查了 %d 个 header 来源", checked)
}

// TestRequestAttrIndexRebuildsOnConfigChange 热重载换掉配置后索引必须重建
// [EN] The index must be rebuilt after a hot reload replaces the config
func TestRequestAttrIndexRebuildsOnConfigChange(t *testing.T) {
	attrIndexCache.Store(nil)

	first := gwconfig.Default().RequestContext
	ix1 := requestAttrIndex(first)
	require.Same(t, ix1, requestAttrIndex(first), "同一份配置应命中缓存")

	second := gwconfig.Default().RequestContext
	require.NotSame(t, first, second, "用例需要两份不同的配置指针")
	ix2 := requestAttrIndex(second)
	assert.NotSame(t, ix1, ix2, "配置换了却复用了旧索引")
	assert.Same(t, second, ix2.src)
}

// TestBuildAttrIndexNilConfig 配置为 nil 时不得 panic
// [EN] A nil config must not panic
func TestBuildAttrIndexNilConfig(t *testing.T) {
	assert.NotPanics(t, func() {
		ix := buildAttrIndex(nil)
		var meta RequestCommonMeta
		fillFromRequest(ix, httptest.NewRequest(http.MethodPost, "/x", nil), &meta)
		assert.Equal(t, RequestCommonMeta{}, meta)
	})
}
