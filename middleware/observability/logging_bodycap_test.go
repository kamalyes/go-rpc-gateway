/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-03 17:02:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-03 17:20:00
 * @FilePath: \\go-rpc-gateway\\middleware\\observability\\logging_bodycap_test.go
 * @Description: 请求与响应体捕获的边界测试 - 捕获有上限而 handler 仍见完整请求体
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package observability

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// closeTracker 记录 body 是否被 Close，验证 prefixedBody 把 Close 转交给了原始 body
type closeTracker struct {
	io.Reader
	closed bool
}

func (c *closeTracker) Close() error {
	c.closed = true
	return nil
}

func setupLogging(t *testing.T, maxBody int) {
	t.Helper()
	cfg := gwconfig.Default()
	cfg.Middleware.Logging.MaxBodySize = maxBody
	cfg.Middleware.Logging.EnableRequest = true
	global.GATEWAY = cfg
	require.NoError(t, global.EnsureLoggerInitialized())
}

// TestLoggingPassesFullBodyToHandler 捕获加了上限之后，handler 必须仍然读到完整请求体
//
// 这是整个改动最要紧的一条：捕获只取前 MaxBodySize 字节，
// 若 MultiReader 没把剩余拼回去，超过上限的请求体会被静默截断 ——
// 表现是大 body 的接口开始报解析错误，而日志看起来一切正常
//
// [EN] With capture bounded, the handler must still read the complete request body
// This is the critical assertion: capture keeps only the first MaxBodySize bytes, and if
// the MultiReader fails to rejoin the remainder the body is silently truncated, which
// surfaces as parse errors on large-body endpoints while the logs look fine
func TestLoggingPassesFullBodyToHandler(t *testing.T) {
	const limit = 64
	setupLogging(t, limit)

	for _, size := range []int{0, 1, limit - 1, limit, limit + 1, limit * 10, 1 << 16} {
		t.Run(strings.TrimSpace(strings.Repeat(" ", 0))+itoa(size), func(t *testing.T) {
			payload := bytes.Repeat([]byte("abcdefgh"), (size/8)+1)[:size]

			var seen []byte
			h := LoggingMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				seen, err = io.ReadAll(r.Body)
				require.NoError(t, err)
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			h.ServeHTTP(httptest.NewRecorder(), req)

			assert.Equal(t, payload, seen, "handler 读到的请求体与原始请求体不一致，size=%d", size)
		})
	}
}

// TestLoggingClosesOriginalBody prefixedBody.Close 必须转交给原始 body
// 用 io.NopCloser 包 MultiReader 会让原始 body 永不关闭，keep-alive 连接因此泄漏
//
// [EN] prefixedBody.Close must delegate to the original body
// Wrapping the MultiReader in io.NopCloser would leave it unclosed, leaking keep-alive
// connections
func TestLoggingClosesOriginalBody(t *testing.T) {
	setupLogging(t, 16)

	tracker := &closeTracker{Reader: bytes.NewReader(bytes.Repeat([]byte("x"), 100))}
	h := LoggingMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		require.NoError(t, r.Body.Close())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Body = tracker
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(httptest.NewRecorder(), req)

	assert.True(t, tracker.closed, "原始 body 没有被 Close")
}

// TestResponseWriterCaptureIsBounded 响应体捕获不得超过上限
// 改造前 Write 无条件把每一片都写进 buffer，流式大响应会被整段缓冲
//
// [EN] Response capture must not exceed its ceiling
// Write used to append every chunk, buffering a large streaming response whole
func TestResponseWriterCaptureIsBounded(t *testing.T) {
	const limit = 32
	rec := httptest.NewRecorder()
	rw := core.NewResponseWriter(rec)
	rw.EnableBodyCaptureLimit(limit)
	defer rw.Release()

	// 分多片写，总量远超上限
	chunk := bytes.Repeat([]byte("y"), 100)
	for i := 0; i < 50; i++ {
		n, err := rw.Write(chunk)
		require.NoError(t, err)
		assert.Equal(t, len(chunk), n, "Write 必须报告写出了全部字节，捕获上限不影响实际响应")
	}

	assert.Len(t, rw.GetBody(), limit, "捕获量应恰好停在上限")
	assert.Equal(t, 50*len(chunk), rec.Body.Len(), "真实响应体必须完整写给下游")
	assert.Equal(t, int64(50*len(chunk)), rw.BytesWritten())
}

// TestResponseWriterDefaultCaptureLimit 不指定上限时走默认值，而不是无界
// [EN] Without an explicit ceiling the default applies rather than no ceiling at all
func TestResponseWriterDefaultCaptureLimit(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := core.NewResponseWriter(rec)
	rw.EnableBodyCapture()
	defer rw.Release()

	_, err := rw.Write(bytes.Repeat([]byte("z"), core.DefaultBodyCaptureLimit*3))
	require.NoError(t, err)
	assert.Len(t, rw.GetBody(), core.DefaultBodyCaptureLimit)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
