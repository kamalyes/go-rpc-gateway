/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-01 17:05:23
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-01 17:36:51
 * @FilePath: \go-rpc-gateway\middleware\core\request_context_shapes_bench_test.go
 * @Description: RequestContextMiddleware 的调用形状基准
 *
 * 为什么需要按形状分开量：这条中间件要取 28 个属性，每个属性配了若干来源，
 * 而 ExtractAttribute 是「按顺序取第一个非空」—— 所以属性**从 header 命中**时会提前返回、
 * 根本不碰 query，而属性**哪儿都没有**时会把所有来源走穿
 * 默认配置下共 49 个 query 来源与 29 个 cookie 来源，于是最坏情况是属性全不存在，
 * 那正是网关的常态：绝大多数请求不会带满 28 个自定义头
 *
 * 只量一种形状会看不见这个差别，也就判断不了优化到底动了哪条路径
 *
 * [EN] Shape-split benchmarks for the request-context middleware. It extracts 28 attributes,
 * each from an ordered source list where the first non-empty wins, so a header hit returns
 * before the query is ever touched while an attribute present nowhere walks every source. The
 * default configuration carries 49 query sources and 29 cookie sources, which makes "absent
 * everywhere" the worst case -- and that is the common case for a gateway, since most requests
 * do not carry 28 custom headers. Measuring one shape only hides that difference
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package core

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// benchRequestShapes 覆盖四种来源命中形态
func benchRequestShapes() []struct {
	name string
	req  func() *http.Request
} {
	return []struct {
		name string
		req  func() *http.Request
	}{
		{
			// 最坏情况也是最常见情况：28 个属性哪儿都找不到，所有来源被走穿
			// [EN] The worst and most common case: no attribute is present anywhere
			name: "absent",
			req: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/api/v1/order/create", nil)
			},
		},
		{
			// 属性从 header 命中，ExtractAttribute 提前返回，query 与 cookie 都不碰
			// [EN] Attributes hit on the header source, so the query is never parsed
			name: "headers",
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/api/v1/order/create", nil)
				r.Header.Set("X-Trace-Id", "trace-0123456789abcdef")
				r.Header.Set("X-Request-Id", "req-0123456789abcdef")
				r.Header.Set("X-User-ID", "u-100286")
				r.Header.Set("X-Tenant-ID", "t-37")
				r.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.e30.x")
				r.Header.Set("X-Device-ID", "dev-ios-9f8e7d")
				r.Header.Set("X-App-Version", "3.17.2")
				r.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
				return r
			},
		},
		{
			// 带查询串：query 来源真的要做解析工作，而不只是构造一个空 map
			// [EN] With a query string, the query sources do real parsing work
			name: "query",
			req: func() *http.Request {
				return httptest.NewRequest(http.MethodGet,
					"/api/v1/order/create?trace_id=trace-0123456789abcdef&request_id=req-0123456789abcdef"+
						"&tenant_id=t-37&page=3&page_size=20&sort=-created_at&keyword=%E8%AE%A2%E5%8D%95", nil)
			},
		},
		{
			// 带 Cookie 头：cookie 来源每次都要重解整个头
			// [EN] With a Cookie header, each cookie source re-parses the whole header
			name: "cookies",
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/api/v1/order/create", nil)
				r.Header.Set("Cookie",
					"trace_id=trace-0123456789abcdef; request_id=req-0123456789abcdef; "+
						"session_id=sess-abcdef0123456789; authorization=Bearer%20token; theme=dark")
				return r
			},
		},
		{
			// 混合形态，最接近真实流量
			// [EN] A mixed shape, closest to real traffic
			name: "mixed",
			req: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet,
					"/api/v1/order/create?page=3&page_size=20&sort=-created_at", nil)
				r.Header.Set("X-Trace-Id", "trace-0123456789abcdef")
				r.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.e30.x")
				r.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X)")
				r.Header.Set("Cookie", "session_id=sess-abcdef0123456789; theme=dark")
				return r
			},
		},
	}
}

// BenchmarkRequestContextMiddleware_Shapes 按来源命中形态分别量这条中间件
func BenchmarkRequestContextMiddleware_Shapes(b *testing.B) {
	mw := RequestContextMiddleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, shape := range benchRequestShapes() {
		b.Run(shape.name, func(b *testing.B) {
			req := shape.req()
			// 复用同一个 ResponseWriter：httptest.NewRecorder 每次都分配，
			// 那是基准装置的开销，不是中间件的成本，不该混进测量
			// [EN] One recorder reused across iterations: allocating a new one per iteration is
			// harness cost, not middleware cost, and must not enter the measurement
			rec := httptest.NewRecorder()
			b.ReportAllocs()
			for b.Loop() {
				handler.ServeHTTP(rec, req)
			}
		})
	}
}
