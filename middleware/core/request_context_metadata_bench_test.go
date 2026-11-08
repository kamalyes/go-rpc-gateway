/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-06 21:00:19
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-06 22:00:02
 * @FilePath: \go-rpc-gateway\middleware\core\request_context_metadata_bench_test.go
 * @Description: 出站 metadata 注入基准 - 网关代理每一次下游调用都要走这里
 *
 * 为什么按「填充密度」分形状量：injectTraceToOutgoingContext 把 33 个字段逐个写进
 * metadata.MD，而真实请求里绝大多数字段是空的 —— 一个只带 trace/request id 的请求
 * 与一个带满租户、平台、设备、签名的请求，付的代价完全不同
 * 只量「填满」会高估常态开销，只量「全空」会看不见 Join 的复制成本
 *
 * [EN] Shape-split by fill density: the injector writes 33 fields into a metadata.MD while
 * most fields are empty on a real request, so a request carrying only trace and request ids
 * costs very differently from one carrying tenant, platform, device and signature. Measuring
 * only the full shape overstates the common case; measuring only the empty one hides the
 * copy cost of Join
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package core

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

// benchMetaShapes 按字段填充密度覆盖三种形态
func benchMetaShapes() []struct {
	name string
	meta *RequestCommonMeta
} {
	return []struct {
		name string
		meta *RequestCommonMeta
	}{
		{
			// 网关常态：只有链路标识，其余 31 个字段为空
			// [EN] The common gateway case: trace identifiers only, the other 31 empty
			name: "sparse",
			meta: &RequestCommonMeta{
				TraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
				RequestID: "req-0123456789abcdef",
			},
		},
		{
			// 带身份与租户，典型已登录业务请求
			// [EN] Identity plus tenancy, a typical authenticated request
			name: "typical",
			meta: &RequestCommonMeta{
				TraceID:       "4bf92f3577b34da6a3ce929d0e0e4736",
				RequestID:     "req-0123456789abcdef",
				UserID:        "u-100286",
				UserName:      "张三",
				TenantID:      "t-001",
				TenantCode:    "acme",
				Authorization: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig",
				IPAddress:     "203.0.113.10",
				UserAgent:     "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
			},
		},
		{
			// 33 个字段全部非空，上界
			// [EN] All 33 fields non-empty, the upper bound
			name: "full",
			meta: &RequestCommonMeta{
				ID: "id", TraceID: "trace", RequestID: "req", Authorization: "auth",
				UserID: "uid", UserName: "name", Domain: "dom", RoleCode: "role",
				TenantID: "tid", TenantCode: "tcode", SessionID: "sid", Timezone: "tz",
				IPAddress: "ip", AppID: "app", DeviceID: "dev", AppVersion: "ver",
				PlatformID: "pid", PlatformCode: "pcode", RegionID: "rid", RegionCode: "rcode",
				AgentLineID: "aid", Nonce: "nonce", Jti: "jti", FamilyId: "fam",
				XNsID: "ns", UserAgent: "ua", PushToken: "push", Token: "token",
				Timestamp: "ts", Signature: "sig", AccessKey: "ak",
				AcceptLanguage: "zh-CN", ForwardedHost: "example.com",
			},
		},
	}
}

// BenchmarkOutgoingInjectByShape 出站注入，无已有 outgoing metadata
func BenchmarkOutgoingInjectByShape(b *testing.B) {
	for _, shape := range benchMetaShapes() {
		b.Run(shape.name, func(b *testing.B) {
			ctx := WithRequestCommonMeta(context.Background(), shape.meta)
			b.ReportAllocs()
			for b.Loop() {
				_ = injectTraceToOutgoingContext(ctx)
			}
		})
	}
}

// BenchmarkOutgoingInjectByShapeWithExisting 已有 outgoing metadata，会多走一次 Join 复制
//
// 这是 grpc-gateway 代理路径的真实形状：mux 先把入站 header 转成 outgoing metadata，
// 客户端拦截器再往上叠，所以 Join 分支才是常态而不是例外
//
// [EN] This is the real shape on the grpc-gateway proxy path: the mux turns inbound headers
// into outgoing metadata first and the client interceptor adds to it, so the Join branch is
// the common case rather than the exception
func BenchmarkOutgoingInjectByShapeWithExisting(b *testing.B) {
	for _, shape := range benchMetaShapes() {
		b.Run(shape.name, func(b *testing.B) {
			ctx := WithRequestCommonMeta(context.Background(), shape.meta)
			ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
				"x-forwarded-for", "203.0.113.10",
				"content-type", "application/grpc",
			))
			b.ReportAllocs()
			for b.Loop() {
				_ = injectTraceToOutgoingContext(ctx)
			}
		})
	}
}

// BenchmarkOutgoingMetadataWireSize 不量时间，量注入后真正上线的 metadata 条数
//
// 空值照发不是「免费」：每条都要经 HTTP/2 HPACK 编码送到下游，
// 而下游 enrichContextFromMetadata 用 md.Get 读，键缺失与键存在但值为空返回完全一样
// 所以这个数字直接反映有多少条是纯浪费
//
// [EN] Not a timing benchmark but a count of the metadata entries that actually go on the
// wire. Empty values are not free: each is HPACK-encoded to the downstream, while the
// downstream reads with md.Get, for which an absent key and a present-but-empty key are
// indistinguishable -- so this number is how many entries are pure waste
func BenchmarkOutgoingMetadataWireSize(b *testing.B) {
	b.Skip("信息性度量，由 TestOutgoingMetadataWireSize 断言")
}

// TestOutgoingMetadataWireSize 记录各形状下实际注入的 metadata 条数
func TestOutgoingMetadataWireSize(t *testing.T) {
	for _, shape := range benchMetaShapes() {
		ctx := WithRequestCommonMeta(context.Background(), shape.meta)
		out := injectTraceToOutgoingContext(ctx)
		md, _ := metadata.FromOutgoingContext(out)

		empty := 0
		for _, vals := range md {
			for _, v := range vals {
				if v == "" {
					empty++
				}
			}
		}
		t.Logf("形状 %-8s 共 %2d 条 metadata，其中空值 %2d 条", shape.name, md.Len(), empty)
	}
}
