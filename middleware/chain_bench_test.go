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
	"bytes"
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

// resetReadCloser 可重复读取的请求体，Close 为空操作
//
// 不用 io.NopCloser(bytes.NewReader(body)) 每轮重建：那会在每次迭代里多两次分配，
// 把基准自身的开销计进中间件成本
// [EN] A rewindable request body whose Close is a no-op. Rebuilding it every iteration with
// io.NopCloser(bytes.NewReader(body)) would add two allocations per iteration and charge them
// to the middleware under measurement
type resetReadCloser struct{ *bytes.Reader }

func (resetReadCloser) Close() error { return nil }

func drive(b *testing.B, h http.Handler) {
	body := []byte(`{"id":"x","seq":1,"text":"hi"}`)
	reader := bytes.NewReader(body)
	req := httptest.NewRequest(http.MethodPost, "/bench.BenchService/Echo", reader)
	req.Header.Set("Content-Type", "application/json")
	reqBody := resetReadCloser{Reader: reader}

	b.ReportAllocs()
	for b.Loop() {
		// 每轮把请求体复位，而不是复用上一轮被中间件包过的 r.Body：
		// logging 会把 r.Body 换成 prefixedBody{prefix, rest: original}，
		// 复用同一个 request 就是每轮在旧包装外再套一层。包装链随 b.N 线性增长，
		// 量到的就成了「链表深度 + 堆增长/GC」而不是中间件成本
		// （实测同一基准 2k 次 20µs/op、20k 次 175µs/op，典型的伪值）
		// [EN] Rewind the body each iteration instead of reusing the r.Body wrapped by the
		// previous iteration: logging replaces r.Body with prefixedBody{prefix, rest: original},
		// so reusing one request stacks another wrapper every round. The chain grows linearly
		// with b.N and the benchmark measures chain depth plus heap growth/GC, not middleware
		// cost (measured: 20µs/op at 2k iterations vs 175µs/op at 20k — a textbook pseudo-value)
		if _, err := reader.Seek(0, io.SeekStart); err != nil {
			b.Fatalf("reset request body: %v", err)
		}
		req.Body = reqBody

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
