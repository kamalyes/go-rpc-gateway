/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-08 19:56:12 +0800
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-08 19:56:12 +0800
 * @FilePath: \go-rpc-gateway\middleware\i18n\struct_tag_validator_bench_test.go
 * @Description: 路由查找的性能基准
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package i18n

import (
	"fmt"
	"net/http"
	"testing"

	gwproto "github.com/kamalyes/go-rpc-gateway/proto"
)

// 这组基准守的是「每请求的热路径不随路由数增长」
//
// 带路径参数的路由，进来的具体路径永远不会精确命中模板键，所以每个这样的请求都要走回退
// 匹配回退原先是 O(表长) 且每条都重新切分字符串，实测 500 条路由时一个带参请求
// 60665 ns / 1003 次分配改成「按方法+段数分桶 + 模板注册时切好」之后：
// 10 条 134 ns / 1 次，100 条 589 ns / 1 次，500 条 2449 ns / 2 次
//
// 自动发现会把表变大（协议里所有带注解的路由都进来），所以这条路径的复杂度必须守住：
// 哪天 Param_500 又回到几万纳秒，说明回退退化成了线性扫描
//
// [EN] These guard that the per-request path does not grow with the number of routes. A
// concrete path never matches a placeholder template exactly, so every parameterised request
// falls through to the template search. That search used to be linear in the table and re-split
// strings per entry: 60665 ns and 1003 allocations at 500 routes; now 134 ns at ten, 589 at a hundred and 2449 at
// five hundred. Auto
// discovery enlarges the table with every annotated route in the contract, so this complexity
// has to stay bounded — if Param_500 returns to tens of microseconds, the fallback has gone
// linear again.

func seedRoutes(n int) {
	gatewayMessageTypeRegistry.mu.Lock()
	gatewayMessageTypeRegistry.data = make(map[string]func() any, n)
	gatewayMessageTypeRegistry.templates = nil
	gatewayMessageTypeRegistry.mu.Unlock()

	for i := 0; i < n; i++ {
		RegisterGatewayMessageType(http.MethodPost,
			fmt.Sprintf("/v1/mod%d/things/{thing_id}/sub%d", i, i),
			func() any { return &gwproto.Paging{} })
	}
	RegisterGatewayMessageType(http.MethodPost, "/v1/exact/route",
		func() any { return &gwproto.Paging{} })
}

func benchmarkLookup(b *testing.B, n int, path string) {
	b.Helper()
	seedRoutes(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := lookupGatewayMessageType(http.MethodPost, path); !ok {
			b.Fatal("未命中 | no match")
		}
	}
}

func BenchmarkLookupExact500(b *testing.B) { benchmarkLookup(b, 500, "/v1/exact/route") }
func BenchmarkLookupParam10(b *testing.B)  { benchmarkLookup(b, 10, "/v1/mod5/things/abc/sub5") }
func BenchmarkLookupParam100(b *testing.B) { benchmarkLookup(b, 100, "/v1/mod50/things/abc/sub50") }
func BenchmarkLookupParam500(b *testing.B) {
	benchmarkLookup(b, 500, "/v1/mod250/things/abc/sub250")
}

// 自动发现只在第一次查找时跑一次，这里单独量它，确认它不是启动期的负担
// [EN] Discovery runs once, on the first lookup; measured separately to confirm it is not a
// startup burden
func BenchmarkAutoDiscover(b *testing.B) {
	for i := 0; i < b.N; i++ {
		autoDiscoverGatewayMessageTypes()
	}
}
