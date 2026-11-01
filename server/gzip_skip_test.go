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
	"compress/gzip"
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
func TestInitGzipWriterPool_CompressionLevelIndependentOfTLS(t *testing.T) {
	httpCfg := gwconfig.DefaultHTTPServer()
	httpCfg.GzipCompressionLevel = 9
	httpCfg.TLS = nil

	s := &Server{config: &gwconfig.Gateway{HTTPServer: httpCfg}}
	s.initGzipWriterPool()

	// 池里的 writer 用配置级别建出来，压同一份高重复载荷时应明显小于默认级别的产物
	payload := []byte(strings.Repeat("kamalyes-go-rpc-gateway-", 512))
	got := gzipSizeFromPool(t, s, payload)

	httpCfg5 := gwconfig.DefaultHTTPServer()
	httpCfg5.GzipCompressionLevel = 1
	httpCfg5.TLS = nil
	s1 := &Server{config: &gwconfig.Gateway{HTTPServer: httpCfg5}}
	s1.initGzipWriterPool()
	fast := gzipSizeFromPool(t, s1, payload)

	if got >= fast {
		t.Errorf("级别 9 应该压得比级别 1 更小，实际 9 档 %d 字节、1 档 %d 字节；说明配置没被读进去", got, fast)
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

// gzipSizeFromPool 用池里的 writer 压一遍，返回产物字节数
func gzipSizeFromPool(t testing.TB, s *Server, payload []byte) int {
	t.Helper()

	var buf countingWriter
	w := s.gzipWriterPool.Get().(*gzip.Writer)
	w.Reset(&buf)
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("关闭失败：%v", err)
	}
	s.gzipWriterPool.Put(w)
	return buf.n
}

type countingWriter struct{ n int }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += len(p)
	return len(p), nil
}
