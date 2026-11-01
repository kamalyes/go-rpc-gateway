/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-01 21:05:03
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-01 21:05:03
 * @FilePath: \go-rpc-gateway\server\gzip_skip_test.go
 * @Description: gzip 跳过规则的等价性与性能测试 | Equivalence and performance of the gzip skip rules
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
)

// legacyShouldSkipGzip 是改造前那版实现的语义，逐字保留，只作为等价性比对的基准
//
// 保留它的理由：这次改的是「怎么查」而不是「查什么」，所以必须有一个独立的参照物
// 能证明两者对同一批路径给出同样的答案，否则快了也没有意义
//
// [EN] The semantics of the previous implementation, kept verbatim purely as a reference for
// the equivalence check. This change alters how the lookup runs rather than what it matches,
// so an independent reference is what makes the speedup meaningful
func legacyShouldSkipGzip(prefixes, extensions []string, reqPath string) bool {
	exact := make(map[string]bool, len(prefixes))
	for _, p := range prefixes {
		exact[p] = true
	}
	if exact[reqPath] {
		return true
	}
	for skipPath := range exact {
		if strings.HasPrefix(reqPath, skipPath) {
			return true
		}
	}
	for _, ext := range extensions {
		if strings.HasSuffix(reqPath, ext) {
			return true
		}
	}
	return false
}

func newGzipTestServer(t testing.TB, prefixes, extensions []string) *Server {
	t.Helper()

	httpCfg := gwconfig.DefaultHTTPServer()
	httpCfg.GzipSkipPaths = prefixes
	httpCfg.GzipSkipExtensions = extensions

	s := &Server{config: &gwconfig.Gateway{HTTPServer: httpCfg}}
	s.initGzipWriterPool()
	return s
}

func gzipTestRequest(reqPath string) *http.Request {
	return &http.Request{URL: &url.URL{Path: reqPath}}
}

// 覆盖的形状：普通 API、前缀命中、精确等于前缀、各类扩展名、目录名带点、
// 隐藏文件、无扩展名、空路径、以及扩展名大小写（旧实现是大小写敏感的）
var gzipSkipPaths = []string{
	"/api/v1/users",
	"/api/v1/users/12345",
	"/ws",
	"/ws/stream",
	"/metrics",
	"/metricsx",
	"/health",
	"/debug/pprof/heap",
	"/static/app.js",
	"/static/app.min.js",
	"/static/logo.png",
	"/static/logo.PNG",
	"/download/archive.tar.gz",
	"/a.b/c",
	"/a.b/c.png",
	"/.gitignore",
	"/noext",
	"/",
	"",
	"/trailing.",
	"/x.jpeg",
	"/x.pdf",
}

func TestShouldSkipGzip_MatchesLegacySemantics(t *testing.T) {
	cfg := gwconfig.DefaultHTTPServer()
	s := newGzipTestServer(t, cfg.GzipSkipPaths, cfg.GzipSkipExtensions)

	if s.gzipSkipExtMap == nil {
		t.Fatal("默认扩展名全是单点形状，应该建出扩展名表才对")
	}

	for _, p := range gzipSkipPaths {
		want := legacyShouldSkipGzip(cfg.GzipSkipPaths, cfg.GzipSkipExtensions, p)
		if got := s.shouldSkipGzip(gzipTestRequest(p)); got != want {
			t.Errorf("路径 %q：新实现 %v，旧语义 %v", p, got, want)
		}
	}
}

// 带两个点的扩展名必须走后缀循环：取扩展名只能得到 .js，与 HasSuffix 不等价
func TestShouldSkipGzip_MultiDotFallsBackToSuffixLoop(t *testing.T) {
	prefixes := []string{"/ws"}
	extensions := []string{".min.js", ".gz"}
	s := newGzipTestServer(t, prefixes, extensions)

	if s.gzipSkipExtMap != nil {
		t.Fatal("出现多点扩展名时不该建扩展名表，否则 .min.js 会被当成 .js")
	}

	for _, p := range append([]string{"/static/app.min.js", "/static/app.js"}, gzipSkipPaths...) {
		want := legacyShouldSkipGzip(prefixes, extensions, p)
		if got := s.shouldSkipGzip(gzipTestRequest(p)); got != want {
			t.Errorf("路径 %q：新实现 %v，旧语义 %v", p, got, want)
		}
	}
}

// 不带点的扩展名配置同样要退回后缀循环
func TestShouldSkipGzip_DotlessExtensionFallsBack(t *testing.T) {
	s := newGzipTestServer(t, nil, []string{"js"})
	if s.gzipSkipExtMap != nil {
		t.Fatal("不带点的配置不该建扩展名表")
	}
	if !s.shouldSkipGzip(gzipTestRequest("/static/barjs")) {
		t.Error("旧语义下 HasSuffix(\"/static/barjs\", \"js\") 为真，新实现必须保持一致")
	}
}

// HTTPServer 为 nil 时不能 panic：原实现判完 nil 之后一行就无条件解引用了
func TestInitGzipWriterPool_NilHTTPServerDoesNotPanic(t *testing.T) {
	s := &Server{config: &gwconfig.Gateway{}}
	s.initGzipWriterPool()

	if s.gzipWriterPool == nil {
		t.Error("即使没有 HTTPServer 配置，writer 池也该建出来")
	}
	if s.shouldSkipGzip(gzipTestRequest("/anything.png")) {
		t.Error("没有跳过规则时不该跳过任何路径")
	}
}

// 压缩级别不该再被 TLS 判断挡住
//
// 直接断言级别解析的结果，不去压数据比字节数 —— 后者跨 Go 版本与平台不稳：
// 实测同一段高重复载荷在 macOS 上 1 档与 9 档有差，在 Linux / Go 1.27 上两档都是 91 字节，
// 于是测试在 CI 上判红而被测代码其实是对的
// [EN] The resolved level is asserted directly rather than by compressing and comparing byte
// counts, which is not stable across Go versions or platforms: the same repetitive payload
// differed between levels on macOS and was identical at 91 bytes on Linux with Go 1.27,
// failing in CI while the code under test was correct
func TestResolveGzipLevel(t *testing.T) {
	defaultLevel := gwconfig.DefaultHTTPServer().GzipCompressionLevel

	cases := []struct {
		name string
		cfg  func() *gwconfig.HTTPServer
		want int
	}{
		{"配置为空时回落默认", func() *gwconfig.HTTPServer { return nil }, defaultLevel},
		{"没配 TLS 也要读到配置值", func() *gwconfig.HTTPServer {
			c := gwconfig.DefaultHTTPServer()
			c.GzipCompressionLevel = 9
			c.TLS = nil
			return c
		}, 9},
		{"配了 TLS 同样读到配置值", func() *gwconfig.HTTPServer {
			c := gwconfig.DefaultHTTPServer()
			c.GzipCompressionLevel = 1
			return c
		}, 1},
		{"级别为 0 视为没配，回落默认", func() *gwconfig.HTTPServer {
			c := gwconfig.DefaultHTTPServer()
			c.GzipCompressionLevel = 0
			return c
		}, defaultLevel},
		{"级别超出 1-9 回落默认", func() *gwconfig.HTTPServer {
			c := gwconfig.DefaultHTTPServer()
			c.GzipCompressionLevel = 10
			return c
		}, defaultLevel},
		{"级别为负回落默认", func() *gwconfig.HTTPServer {
			c := gwconfig.DefaultHTTPServer()
			c.GzipCompressionLevel = -1
			return c
		}, defaultLevel},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveGzipLevel(tc.cfg()); got != tc.want {
				t.Errorf("期望级别 %d，实际 %d", tc.want, got)
			}
		})
	}
}

// 池里取出的 writer 要能正常压缩并解压回原文，不断言压缩率
func TestGzipWriterPoolProducesValidOutput(t *testing.T) {
	httpCfg := gwconfig.DefaultHTTPServer()
	httpCfg.GzipCompressionLevel = 9
	httpCfg.TLS = nil

	s := &Server{config: &gwconfig.Gateway{HTTPServer: httpCfg}}
	s.initGzipWriterPool()

	payload := []byte(strings.Repeat("kamalyes-go-rpc-gateway-", 512))

	var buf bytes.Buffer
	w := s.gzipWriterPool.Get().(*gzip.Writer)
	w.Reset(&buf)
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("关闭失败：%v", err)
	}
	s.gzipWriterPool.Put(w)

	r, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatalf("建解压器失败：%v", err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("解压失败：%v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("解压回来的内容与原文不一致：%d 字节 vs %d 字节", len(got), len(payload))
	}
}

func BenchmarkShouldSkipGzip_NoMatch(b *testing.B) {
	cfg := gwconfig.DefaultHTTPServer()
	s := newGzipTestServer(b, cfg.GzipSkipPaths, cfg.GzipSkipExtensions)
	r := gzipTestRequest("/api/v1/users/12345/orders")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if s.shouldSkipGzip(r) {
			b.Fatal("这条路径不该命中")
		}
	}
}

func BenchmarkShouldSkipGzip_Legacy_NoMatch(b *testing.B) {
	cfg := gwconfig.DefaultHTTPServer()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if legacyShouldSkipGzip(cfg.GzipSkipPaths, cfg.GzipSkipExtensions, "/api/v1/users/12345/orders") {
			b.Fatal("这条路径不该命中")
		}
	}
}

func BenchmarkShouldSkipGzip_PrefixMatch(b *testing.B) {
	cfg := gwconfig.DefaultHTTPServer()
	s := newGzipTestServer(b, cfg.GzipSkipPaths, cfg.GzipSkipExtensions)
	r := gzipTestRequest("/metrics")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !s.shouldSkipGzip(r) {
			b.Fatal("这条路径该命中")
		}
	}
}

func BenchmarkShouldSkipGzip_ExtensionMatch(b *testing.B) {
	cfg := gwconfig.DefaultHTTPServer()
	s := newGzipTestServer(b, cfg.GzipSkipPaths, cfg.GzipSkipExtensions)
	r := gzipTestRequest("/static/assets/logo.png")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !s.shouldSkipGzip(r) {
			b.Fatal("这条路径该命中")
		}
	}
}
