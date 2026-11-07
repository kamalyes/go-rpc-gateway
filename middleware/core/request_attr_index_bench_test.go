/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-03 16:02:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-03 16:18:00
 * @FilePath: \go-rpc-gateway\middleware\core\request_attr_index_bench_test.go
 * @Description: 请求属性提取三种做法的 A/B 基准 - 逐属性提取、复用提取器、反转索引
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package core

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	gccommon "github.com/kamalyes/go-rpc-gateway/config/core"
	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
)

// abSourceLists 取默认配置里的 41 份来源清单
func abSourceLists() [][]gccommon.AttributeSource {
	rc := gwconfig.Default().RequestContext
	v := reflect.ValueOf(rc).Elem()
	var out [][]gccommon.AttributeSource
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Type() == reflect.TypeOf([]gccommon.AttributeSource{}) {
			out = append(out, f.Interface().([]gccommon.AttributeSource))
		}
	}
	return out
}

// abRequest 贴近真实的请求：8 个 header，无查询串，无 cookie
func abRequest() *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/bench.BenchService/Echo", nil)
	for k, v := range map[string]string{
		"Content-Type":    "application/json",
		"Accept-Encoding": "gzip",
		"User-Agent":      "bench/1.0",
		"X-Request-Id":    "req-123",
		"X-Trace-Id":      "trace-abc",
		"Authorization":   "Bearer token",
		"X-Tenant-Id":     "t-1",
		"X-Forwarded-For": "10.0.0.1",
	} {
		r.Header.Set(k, v)
	}
	return r
}

// BenchmarkAB_Current 现状：41 次包级 ExtractAttribute
func BenchmarkAB_Current(b *testing.B) {
	lists := abSourceLists()
	r := abRequest()
	out := make([]string, len(lists))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j, srcs := range lists {
			out[j] = gccommon.ExtractAttribute(r, srcs)
		}
	}
}

// BenchmarkAB_Reused 复用一个提取器
func BenchmarkAB_Reused(b *testing.B) {
	lists := abSourceLists()
	r := abRequest()
	out := make([]string, len(lists))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ex := gccommon.NewAttributeExtractor(r)
		for j, srcs := range lists {
			out[j] = ex.Extract(srcs)
		}
	}
}

type abSlot struct {
	attr int
	rank uint8
}

type abIndex struct {
	headers map[string][]abSlot
	queries map[string][]abSlot
	cookies map[string][]abSlot
	n       int
}

func abBuildIndex(lists [][]gccommon.AttributeSource) *abIndex {
	ix := &abIndex{
		headers: map[string][]abSlot{},
		queries: map[string][]abSlot{},
		cookies: map[string][]abSlot{},
		n:       len(lists),
	}
	for attr, srcs := range lists {
		for rank, s := range srcs {
			sl := abSlot{attr: attr, rank: uint8(rank)}
			switch s.Type {
			case gccommon.SourceTypeHeader:
				k := http.CanonicalHeaderKey(s.Key)
				ix.headers[k] = append(ix.headers[k], sl)
			case gccommon.SourceTypeQuery:
				ix.queries[s.Key] = append(ix.queries[s.Key], sl)
			case gccommon.SourceTypeCookie:
				ix.cookies[s.Key] = append(ix.cookies[s.Key], sl)
			}
		}
	}
	return ix
}

// BenchmarkAB_Inverted 反转索引：只遍历请求实际携带的 header
func BenchmarkAB_Inverted(b *testing.B) {
	ix := abBuildIndex(abSourceLists())
	r := abRequest()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var vals [64]string
		var ranks [64]uint8
		for j := range ranks {
			ranks[j] = 255
		}
		for name, vs := range r.Header {
			if len(vs) == 0 || vs[0] == "" {
				continue
			}
			for _, sl := range ix.headers[name] {
				if sl.rank < ranks[sl.attr] {
					vals[sl.attr] = vs[0]
					ranks[sl.attr] = sl.rank
				}
			}
		}
		if raw := r.URL.RawQuery; raw != "" {
			q := r.URL.Query()
			for k, sls := range ix.queries {
				if v := q.Get(k); v != "" {
					for _, sl := range sls {
						if sl.rank < ranks[sl.attr] {
							vals[sl.attr] = v
							ranks[sl.attr] = sl.rank
						}
					}
				}
			}
		}
		if r.Header.Get("Cookie") != "" {
			for _, c := range r.Cookies() {
				if c.Value == "" {
					continue
				}
				for _, sl := range ix.cookies[c.Name] {
					if sl.rank < ranks[sl.attr] {
						vals[sl.attr] = c.Value
						ranks[sl.attr] = sl.rank
					}
				}
			}
		}
		_ = vals
	}
}
