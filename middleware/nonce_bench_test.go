/**
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-10-17 18:20:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-10-17 18:20:00
 * @FilePath: \go-rpc-gateway\middleware\nonce_bench_test.go
 * @Description: Nonce 派生性能基准测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kamalyes/go-config/pkg/signature"
)

// benchmarkDeriveNonceConfig 派生路径基准配置（对齐生产签名配置）
var benchmarkDeriveNonceConfig = &signature.Signature{
	Enabled:        true,
	Type:           signature.SignatureTypeHMAC,
	NonceKeyPrefix: "nonce:",
	NonceTTL:       10 * time.Minute,
	RequireNonce:   true,
}

// benchmarkDeriveNonceMeta 旧客户端公共元信息（携带 Authorization/AccessKey/Timestamp，未携带 Nonce）
var benchmarkDeriveNonceMeta = &RequestCommonMeta{
	Authorization: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMDAyMzQiLCJ0ZW5hbnQiOiJ0LWNoYXJnZSJ9.ZWFzdGFzaWhhaWppbWVyY2hhcmdl",
	AccessKey:     "ak-20261017-gateway-009",
	Timestamp:     "1760695200",
}

// buildBenchmarkBody 构造指定大小的订单批量 JSON 请求体（模拟生产请求体规模）
func buildBenchmarkBody(size int) string {
	if size == 0 {
		return ""
	}
	record := `{"userId":100234,"tenantCode":"t-charge","symbol":"BTCUSDT","amount":"123.45","side":"BUY","orderType":"LIMIT","price":"67231.80","fee":"0.10"},`
	return "[" + strings.TrimSuffix(strings.Repeat(record, size/len(record)+1), ",") + "]"
}

// BenchmarkDeriveNonce 旧客户端未携带 Nonce 时的派生开销（读 body + MD5 + hex 编码）
//
// 覆盖典型生产请求体规模：GET 无 body / 512B / 4KB / 64KB
// readRequestBody 会还原请求体，同一请求可重复派生，单次调用即等价于单请求热路径成本
// 新客户端（显式携带 X-Nonce）不进入该路径，零额外开销
func BenchmarkDeriveNonce(b *testing.B) {
	for _, size := range []int{0, 512, 4096, 65536} {
		b.Run(fmt.Sprintf("body=%dB", size), func(b *testing.B) {
			body := buildBenchmarkBody(size)
			r := httptest.NewRequest(http.MethodPost, "/api/v1/orders?symbol=BTCUSDT&limit=100&page=1", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")

			b.ReportAllocs()
			if len(body) > 0 {
				b.SetBytes(int64(len(body)))
			}
			for b.Loop() {
				nonce, err := deriveNonce(r, benchmarkDeriveNonceMeta, benchmarkDeriveNonceConfig)
				if err != nil {
					b.Fatal(err)
				}
				_ = nonce
			}
		})
	}
}
