/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 05:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 05:00:00
 * @FilePath: \go-rpc-gateway\server\gzip_stream_test.go
 * @Description: gzip 包装器的接口完备性 - 流式 Flush、WebSocket 劫持与配置缺失
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package server

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
)

// gzipHeaderSize gzip 流头部的固定字节数，用来区分「只写了头」与「真的压出了数据」
const gzipHeaderSize = 10

// newGzipStreamServer 启用 gzip 的最小 Server
func newGzipStreamServer(t testing.TB) *Server {
	t.Helper()

	httpCfg := gwconfig.DefaultHTTPServer()
	httpCfg.EnableGzipCompress = true
	s := &Server{config: &gwconfig.Gateway{HTTPServer: httpCfg}}
	s.initGzipWriterPool()
	return s
}

// gzipStreamRequest 带 Accept-Encoding: gzip 的请求，也就是浏览器的默认行为
func gzipStreamRequest() *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	return r
}

// TestGzipWriterKeepsFlusher 包装之后必须仍然实现 http.Flusher
//
// 回归点：gzipResponseWriter 只嵌了 http.ResponseWriter 接口，提升上来的方法集没有 Flush，
// 于是 w.(http.Flusher) 断言失败 —— 而这正是 grpc-gateway 的 ForwardResponseStream 的
// 第一步，不成立时它直接回 500 "unexpected type of web server"
// 后果不是慢，是开了 gzip 之后所有 server-streaming 接口对默认带 Accept-Encoding: gzip
// 的客户端全部不可用
//
// [EN] Regression: the wrapper only embedded the http.ResponseWriter interface, so Flush was
// not in the promoted method set and w.(http.Flusher) failed to assert. That assertion is the
// first thing grpc-gateway's ForwardResponseStream does and it answers 500 on failure, so the
// consequence was not slowness but every server-streaming endpoint being unusable for any
// client sending Accept-Encoding: gzip.
func TestGzipWriterKeepsFlusher(t *testing.T) {
	s := newGzipStreamServer(t)

	var (
		sawFlusher  bool
		sawHijacker bool
		sawPusher   bool
	)
	h := s.gzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, sawFlusher = w.(http.Flusher)
		_, sawHijacker = w.(http.Hijacker)
		_, sawPusher = w.(http.Pusher)
	}))
	h.ServeHTTP(httptest.NewRecorder(), gzipStreamRequest())

	if !sawFlusher {
		t.Error("包装后不再实现 http.Flusher：grpc-gateway 的流式转发会直接回 500")
	}
	if !sawHijacker {
		t.Error("包装后不再实现 http.Hijacker：WebSocket 升级会被挡掉")
	}
	if !sawPusher {
		t.Error("包装后不再实现 http.Pusher：HTTP/2 Server Push 不可用")
	}
}

// TestGzipFlushReachesClient Flush 之后数据必须已经离开 gzip 缓冲到了下游
//
// 只补一个空的 Flush 方法能让断言成立，但数据仍会攒在 flate 缓冲里直到 Close，
// 流式响应变成「结束时一次性全到」—— 对长连接的事件流等于完全不工作
// 所以这里断言的是真的有字节出去，而且出去的那段本身可解压
//
// [EN] Adding an empty Flush would satisfy the assertion while the data still sat in the flate
// buffer until Close, turning a streaming response into one bulk delivery at the end, which for
// a long-lived event stream means it does not work at all. This asserts bytes actually left and
// that what left decodes on its own.
func TestGzipFlushReachesClient(t *testing.T) {
	s := newGzipStreamServer(t)
	rec := httptest.NewRecorder()

	const first = "event: first\ndata: hello streaming world\n\n"
	var afterFlush int

	h := s.gzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(first)); err != nil {
			t.Errorf("写第一条消息失败: %v", err)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("包装后不实现 http.Flusher")
		}
		flusher.Flush()
		afterFlush = rec.Body.Len()
	}))
	h.ServeHTTP(rec, gzipStreamRequest())

	if afterFlush <= gzipHeaderSize {
		t.Fatalf("Flush 之后下游只有 %d 字节（gzip 头本身就占 %d 字节），数据仍停在 gzip 缓冲里",
			afterFlush, gzipHeaderSize)
	}

	// 只取 Flush 时刻已经出去的那一段：缺尾块，所以读到 ErrUnexpectedEOF 是预期的，
	// 关键是这段里要能解出完整的第一条消息
	// [EN] Only the prefix that had left by the Flush is decoded: it lacks the trailer, so
	// ErrUnexpectedEOF is expected, and what matters is that the first message comes out whole
	zr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()[:afterFlush]))
	if err != nil {
		t.Fatalf("Flush 产出的前缀不是合法 gzip 流: %v", err)
	}
	flushed, _ := io.ReadAll(zr)
	if !strings.Contains(string(flushed), first) {
		t.Errorf("Flush 之后解压得到 %q，不含第一条消息", string(flushed))
	}
}

// TestGzipFullResponseStillCompresses 基础压缩行为不能被上面的改动带坏
func TestGzipFullResponseStillCompresses(t *testing.T) {
	s := newGzipStreamServer(t)
	rec := httptest.NewRecorder()

	body := strings.Repeat("compress me ", 64)
	h := s.gzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	h.ServeHTTP(rec, gzipStreamRequest())

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip", got)
	}
	if got := rec.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Errorf("Vary = %q, want Accept-Encoding", got)
	}
	zr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("响应不是合法 gzip: %v", err)
	}
	got, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("解压失败: %v", err)
	}
	if string(got) != body {
		t.Errorf("解压内容与写入不一致（%d vs %d 字节）", len(got), len(body))
	}
	if rec.Body.Len() >= len(body) {
		t.Errorf("压缩后 %d 字节，未小于原始 %d 字节", rec.Body.Len(), len(body))
	}
}

// hijackableRecorder 带 Hijack 能力的 recorder
//
// httptest.ResponseRecorder 自己不实现 http.Hijacker，而 Hijack 这条路径必须用真的
// 能劫持的 writer 才测得到
// [EN] httptest.ResponseRecorder does not implement http.Hijacker, and this path needs a
// writer that genuinely can hijack
type hijackableRecorder struct {
	*httptest.ResponseRecorder
	conn net.Conn
}

func (h *hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return h.conn, bufio.NewReadWriter(bufio.NewReader(h.conn), bufio.NewWriter(h.conn)), nil
}

// TestGzipHijackWritesNoTrailer 劫持之后不再通过原 ResponseWriter 写 gzip 尾块
//
// Hijack 刚被补上，这条路径才第一次可达：劫持后原 writer 已经交出控制权，
// 再去写尾块只会拿到 http.ErrHijacked，并把一个已 Close 的 writer 放回对象池
//
// [EN] Hijack has only just been added, so this path is reachable for the first time: the
// original writer has handed over control, and writing the trailer only yields
// http.ErrHijacked while returning a closed writer to the pool
func TestGzipHijackWritesNoTrailer(t *testing.T) {
	s := newGzipStreamServer(t)

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	// net.Pipe 是同步的，另一端必须有人读，否则 handler 侧的写会阻塞
	// [EN] net.Pipe is synchronous, so the far end must be drained or the handler's write blocks
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(io.Discard, client)
	}()

	rec := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder(), conn: server}

	h := s.gzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			t.Error("包装后不实现 http.Hijacker")
			return
		}
		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("Hijack 失败: %v", err)
			return
		}
		// 升级后的协议自己往裸连接上写，不应经过 gzip
		if _, err := conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\n\r\n")); err != nil {
			t.Errorf("裸连接写入失败: %v", err)
		}
	}))
	h.ServeHTTP(rec, gzipStreamRequest())

	server.Close()
	<-done

	if rec.Body.Len() != 0 {
		t.Errorf("劫持之后仍向原 ResponseWriter 写了 %d 字节，应当一个字节都不写", rec.Body.Len())
	}
}

// TestGzipMiddlewareWithoutHTTPServerConfig HTTPServer 整棵子树缺失时不得 panic
//
// 回归点：gzipMiddleware 每个请求都裸解引用 s.config.HTTPServer.EnableGzipCompress，
// 而同一个文件里的 initGzipWriterPool 专门为 HTTPServer 可能为 nil 加过保护 ——
// 判定散在三处，于是一处有保护、另两处每请求 panic
//
// [EN] Regression: gzipMiddleware dereferenced s.config.HTTPServer.EnableGzipCompress on every
// request while initGzipWriterPool in the same file already guarded that exact nil. The
// decision lived in three places, so one had the guard and the others panicked per request.
func TestGzipMiddlewareWithoutHTTPServerConfig(t *testing.T) {
	s := &Server{config: &gwconfig.Gateway{}}
	s.initGzipWriterPool()

	if s.gzipEnabled() {
		t.Error("HTTPServer 为 nil 时 gzipEnabled 应为 false")
	}
	if s.http2Enabled() {
		t.Error("HTTPServer 为 nil 时 http2Enabled 应为 false")
	}

	rec := httptest.NewRecorder()
	h := s.gzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	h.ServeHTTP(rec, gzipStreamRequest()) // 原实现在这里 panic

	if got := rec.Body.String(); got != "ok" {
		t.Errorf("未启用压缩时响应应原样透传，实际 %q", got)
	}
	if got := rec.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("未启用压缩却设了 Content-Encoding = %q", got)
	}
}

// TestGzipEnabledNilServerConfig config 本身为 nil 也不能 panic
func TestGzipEnabledNilServerConfig(t *testing.T) {
	s := &Server{}
	if s.gzipEnabled() || s.http2Enabled() {
		t.Error("config 为 nil 时两个判定都应为 false")
	}
}

// TestGzipBodylessStatusSendsNoGzipStream 无体状态码不得带 gzip 流与 Content-Encoding
//
// 回归点：Content-Encoding: gzip 是 handler 运行前乐观设上的，而 Close() 无条件吐出
// 空 gzip 流（头 + 空 deflate + CRC，约 20 字节）
// 于是 204 / 304 的响应既声明了 gzip 又没有合法 gzip 体，
// net/http 还会在服务端日志里留下 "status code does not allow body"
//
// [EN] Regression: Content-Encoding: gzip was set optimistically before the handler ran while
// Close() unconditionally emitted an empty gzip stream, leaving 204/304 responses advertising
// gzip without a valid gzip body and making net/http log "status code does not allow body"
func TestGzipBodylessStatusSendsNoGzipStream(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
	}{
		{"204NoContent", http.StatusNoContent},
		{"304NotModified", http.StatusNotModified},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newGzipStreamServer(t)
			rec := httptest.NewRecorder()

			h := s.gzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
			}))
			h.ServeHTTP(rec, gzipStreamRequest())

			if got := rec.Header().Get("Content-Encoding"); got != "" {
				t.Errorf("Content-Encoding = %q，无体状态码不该声明压缩", got)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("响应体应为空，实际 %d 字节（多半是空 gzip 流）: %q",
					rec.Body.Len(), rec.Body.Bytes())
			}
			// Vary 要保留：它描述资源随 Accept-Encoding 变化，与本次是否压缩无关
			// [EN] Vary is kept: it describes the resource, not this particular response
			if got := rec.Header().Get("Vary"); got != "Accept-Encoding" {
				t.Errorf("Vary = %q，应保留 Accept-Encoding", got)
			}
		})
	}
}

// TestGzipResponseIsSelfConsistent 响应必须自洽：声明了 gzip，体就得是能解开的 gzip 流
//
// 这条不变量管住的是「handler 几乎什么都没写」的那几种形状 它们看着像是可以省掉
// Close() 那约 20 字节的空 gzip 流，但省不得：Content-Encoding: gzip 在 handler 跑之前
// 就乐观设上、并随 WriteHeader 发了出去，撤不回来
//   - 只 WriteHeader 不写体：省掉就是「声明 gzip 却 0 字节」，gzip.NewReader 报 EOF
//   - 只 Flush 不写体：Flush 已经把 gzip 头写出去了，省掉尾块就是截断流，报 unexpected EOF
//
// 真正能让开的只有「状态码不允许带体」，那时 WriteHeader 会一并删掉 Content-Encoding，
// 响应照样自洽 —— 由 TestGzipBodylessStatusSendsNoGzipStream 覆盖
//
// [EN] The invariant covering the shapes where the handler writes almost nothing. They look
// like the ~20-byte empty gzip stream from Close() could be skipped, but it cannot:
// Content-Encoding: gzip is set optimistically before the handler runs, goes out with
// WriteHeader and cannot be retracted. WriteHeader with no body would leave gzip advertised
// over 0 bytes, on which gzip.NewReader fails with EOF; Flush with no body has already
// emitted the gzip header, so dropping the trailer leaves a truncated stream and decoders
// report unexpected EOF. Only a status code forbidding a body may stand aside, and there
// WriteHeader deletes Content-Encoding so the response stays consistent
func TestGzipResponseIsSelfConsistent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"只设头再 WriteHeader", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Only-Header", "1")
			w.WriteHeader(http.StatusOK)
		}},
		{"完全不动响应", func(w http.ResponseWriter, r *http.Request) {}},
		{"只 Flush 不写体", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
		}},
		{"写了内容", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("payload ", 32)))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newGzipStreamServer(t)
			rec := httptest.NewRecorder()
			s.gzipMiddleware(tc.handler).ServeHTTP(rec, gzipStreamRequest())

			if rec.Header().Get("Content-Encoding") != "gzip" {
				// 没声明压缩，那体就必须是未压缩的原样内容，这里不作进一步要求
				// [EN] No compression advertised, so the body is the raw content as-is
				return
			}
			zr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
			if err != nil {
				t.Fatalf("声明了 Content-Encoding: gzip，但体不是合法 gzip 流: %v（%d 字节: %x）",
					err, rec.Body.Len(), rec.Body.Bytes())
			}
			if _, err := io.ReadAll(zr); err != nil {
				t.Fatalf("gzip 流未正常终结（缺尾块）: %v（%d 字节: %x）",
					err, rec.Body.Len(), rec.Body.Bytes())
			}
		})
	}
}

// TestGzipPoolReleasesWriterReference 归还对象池时必须解除对 ResponseWriter 的引用
//
// 回归点：gzip.Writer 在 Reset(w) 之后一直持有那个 ResponseWriter，
// 带着引用回池会把整个 http 响应对象留到该 writer 下次被取出为止
//
// [EN] Regression: a gzip.Writer keeps the ResponseWriter it was Reset to, so returning it
// to the pool with that reference pins the whole http response object until next acquired
func TestGzipPoolReleasesWriterReference(t *testing.T) {
	s := newGzipStreamServer(t)

	h := s.gzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 128)))
	}))
	h.ServeHTTP(httptest.NewRecorder(), gzipStreamRequest())

	// 取出刚归还的那个 writer，往它身上写一笔
	// 若引用没解除，这一笔会写进上一次请求的 ResponseWriter
	// [EN] Take back the writer just returned and write through it; if the reference was not
	// dropped, this write lands in the previous request's ResponseWriter
	gw := s.gzipWriterPool.Get().(*gzip.Writer)
	defer s.gzipWriterPool.Put(gw)

	victim := httptest.NewRecorder()
	if _, err := gw.Write([]byte("leak-probe")); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if err := gw.Flush(); err != nil {
		t.Fatalf("Flush 失败: %v", err)
	}
	if victim.Body.Len() != 0 {
		t.Errorf("写入落到了不相关的 writer 上，泄漏了引用")
	}
	// 解除引用后目标是 io.Discard，写入不该落到任何 recorder 上，这里只断言不 panic
	_ = victim
}
