/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-09 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\core\hopbyhop_test.go
 * @Description: 逐跳头不得转发到下游 gRPC
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package core

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/metadata"
)

// TestIsHopByHopHeader 逐跳头判定，大小写不敏感
func TestIsHopByHopHeader(t *testing.T) {
	for _, k := range []string{
		"Connection", "connection", "CONNECTION",
		"Keep-Alive", "keep-alive",
		"Proxy-Connection", "proxy-connection",
		"Transfer-Encoding", "transfer-encoding",
		"Upgrade", "upgrade",
		"TE", "te", "Te",
	} {
		if !IsHopByHopHeader(k) {
			t.Errorf("%q 是逐跳头，应当被剔除", k)
		}
	}

	// 不能过度剔除：这些是必须透传的业务与链路头
	// [EN] Must not over-filter: these are business and tracing headers that have to pass
	for _, k := range []string{
		"Authorization", "Cookie", "Content-Type", "Content-Length",
		"User-Agent", "Accept", "Accept-Language", "Accept-Encoding",
		"X-Trace-Id", "X-Request-Id", "X-User-Id", "X-Tenant-Id",
		"X-Forwarded-For", "X-Forwarded-Host", "X-Real-Ip",
		"", "X", "Connection-Id", "Upgrade-Insecure-Requests",
	} {
		if IsHopByHopHeader(k) {
			t.Errorf("%q 不是逐跳头，不该被剔除", k)
		}
	}
}

// TestForwardOutgoingContextDropsHopByHop 转发时逐跳头一律不进 metadata
//
// 回归点：原先只剔了 Connection。把 te 或 transfer-encoding 转进 gRPC metadata
// 会让 gRPC 服务端直接回 RST_STREAM PROTOCOL_ERROR —— 不是多传点东西，
// 是这一路请求整个失败，而且报错出现在下游、很难回溯到网关这一行
//
// [EN] Regression: only Connection used to be dropped. Forwarding te or transfer-encoding
// into gRPC metadata makes the gRPC server answer RST_STREAM PROTOCOL_ERROR — not extra bytes
// on the wire but the whole request failing, with the error surfacing downstream and hard to
// trace back to this line
func TestForwardOutgoingContextDropsHopByHop(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/echo", nil)

	// 逐跳头
	r.Header.Set("Connection", "keep-alive")
	r.Header.Set("Keep-Alive", "timeout=5")
	r.Header.Set("Proxy-Connection", "keep-alive")
	r.Header.Set("Transfer-Encoding", "chunked")
	r.Header.Set("Upgrade", "websocket")
	r.Header.Set("TE", "trailers")

	// 必须透传的头
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("X-Trace-Id", "4bf92f3577b34da6a3ce929d0e0e4736")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Add("X-Multi", "a")
	r.Header.Add("X-Multi", "b")

	md, ok := metadata.FromOutgoingContext(ForwardOutgoingContext(r))
	if !ok {
		t.Fatal("转发后取不到 outgoing metadata")
	}

	for _, k := range []string{
		"connection", "keep-alive", "proxy-connection",
		"transfer-encoding", "upgrade", "te",
	} {
		if got := md.Get(k); len(got) != 0 {
			t.Errorf("逐跳头 %s 被转发了：%v（gRPC 服务端会回 RST_STREAM PROTOCOL_ERROR）", k, got)
		}
	}

	for k, want := range map[string]string{
		"authorization": "Bearer tok",
		"x-trace-id":    "4bf92f3577b34da6a3ce929d0e0e4736",
		"content-type":  "application/json",
	} {
		if got := md.Get(k); len(got) != 1 || got[0] != want {
			t.Errorf("头 %s 期望 [%q]，实际 %v", k, want, got)
		}
	}

	// 多值头要保序保全
	if got := md.Get("x-multi"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("多值头 x-multi 期望 [a b]，实际 %v", got)
	}
}

// TestForwardOutgoingContextEmptyHeader 无头请求不 panic，返回空 metadata
func TestForwardOutgoingContextEmptyHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/v1/ping", nil)
	r.Header = http.Header{}

	md, ok := metadata.FromOutgoingContext(ForwardOutgoingContext(r))
	if !ok {
		t.Fatal("转发后取不到 outgoing metadata")
	}
	if md.Len() != 0 {
		t.Errorf("空请求头应得到空 metadata，实际 %d 条: %v", md.Len(), md)
	}
}
