/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-03 19:02:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-03 19:25:00
 * @FilePath: \go-rpc-gateway\middleware\chain_bench_test.go
 * @Description: 默认中间件链的基准 - 逐个边际成本、累积成本与整链成本
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/logger"
)

// benchTarget 链末端的 handler，写一个与压测台相同量级的响应
func benchTarget() http.Handler {
	body := strings.Repeat("x", 279)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})
}

func benchSetup(t testing.TB) *Manager {
	cfg := gwconfig.Default()
	global.GATEWAY = cfg
	if err := global.EnsureLoggerInitialized(); err != nil {
		t.Fatalf("logger: %v", err)
	}
	// 日志写到 io.Discard：这样量到的是中间件自己的工作（字段组装、脱敏、体捕获），
	// 不含 sink 的成本 默认 sink 是异步批量写，在基准的满速循环里它跟不上就会反压
	// 到请求路径，测出来的是「写日志」而不是「中间件」
	// sink 自身的成本在 BenchmarkLogging_Sinks 里单独量
	//
	// [EN] Logs go to io.Discard so the measurement is the middleware's own work and not
	// the sink: the default sink batches asynchronously and back-pressures the request
	// path once it cannot keep up with a tight loop, which measures logging, not the
	// middleware; the sink itself is measured separately in BenchmarkLogging_Sinks
	if lg, ok := global.LOGGER.(*logger.Logger); ok {
		global.LOGGER = lg.WithOutput(io.Discard)
	}
	m, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("manager: %v", err)
	}
	return m
}

func drive(b *testing.B, h http.Handler) {
	req := httptest.NewRequest(http.MethodPost, "/bench.BenchService/Echo", strings.NewReader(`{"id":"x","seq":1,"text":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

// BenchmarkChain_Bare 链末端 handler 本身，作为参照
func BenchmarkChain_Bare(b *testing.B) { drive(b, benchTarget()) }

// BenchmarkChain_Each 逐个中间件单独包一层，看各自的边际成本
func BenchmarkChain_Each(b *testing.B) {
	m := benchSetup(b)
	cases := []struct {
		name string
		mw   MiddlewareFunc
	}{
		{"Recovery", m.RecoveryMiddleware()},
		{"Logging", m.LoggingMiddleware()},
		{"RequestContext", m.RequestContextMiddlewareFunc()},
		{"Breaker", m.BreakerMiddleware()},
		{"CORS", m.CORSMiddleware()},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			drive(b, c.mw(benchTarget()))
		})
	}
}

// BenchmarkChain_Full 默认配置下的完整链
func BenchmarkChain_Full(b *testing.B) {
	m := benchSetup(b)
	mws := m.GetMiddlewares()
	b.Logf("默认配置下启用 %d 个中间件", len(mws))
	drive(b, ApplyMiddlewares(benchTarget(), mws...))
}

// BenchmarkChain_Cumulative 按链上顺序逐层叠加，定位是哪一层开始变贵
func BenchmarkChain_Cumulative(b *testing.B) {
	m := benchSetup(b)
	mws := m.GetMiddlewares()
	for i := 1; i <= len(mws); i++ {
		b.Run(fmt.Sprintf("前%d层", i), func(b *testing.B) {
			drive(b, ApplyMiddlewares(benchTarget(), mws[:i]...))
		})
	}
}
