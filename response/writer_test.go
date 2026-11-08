/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\response\writer_test.go
 * @Description: HTTP 响应写入测试与基准
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package response

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/kamalyes/go-rpc-gateway/constants"
	commonapis "github.com/kamalyes/go-rpc-gateway/proto"
	"github.com/kamalyes/go-toolbox/pkg/httpx"
)

// TestWriteJSONResponseOutput 输出必须是「JSON + 单个换行」，与 json.Encoder 的行为一致
func TestWriteJSONResponseOutput(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSONResponse(rec, http.StatusOK, map[string]any{"a": 1, "b": "x"})

	if rec.Code != http.StatusOK {
		t.Errorf("状态码期望 200，实际 %d", rec.Code)
	}
	if ct := rec.Header().Get(constants.HeaderContentType); ct != httpx.ContentTypeApplicationJSON {
		t.Errorf("Content-Type 期望 %q，实际 %q", httpx.ContentTypeApplicationJSON, ct)
	}

	body := rec.Body.String()
	// json.Encoder.Encode 固定补一个结尾换行，改造后仍要逐字保持
	// [EN] json.Encoder.Encode always appends one trailing newline, preserved verbatim
	if !strings.HasSuffix(body, "\n") {
		t.Errorf("响应体应以换行结尾（json.Encoder 的行为），实际 %q", body)
	}
	if strings.Count(body, "\n") != 1 {
		t.Errorf("响应体应只有一个换行，实际 %q", body)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("响应体不是合法 JSON: %v (%q)", err, body)
	}
	if got["b"] != "x" {
		t.Errorf("字段 b 期望 x，实际 %v", got["b"])
	}
}

// TestWriteResultOutput Result 响应可被正常解析
func TestWriteResultOutput(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteResult(rec, http.StatusOK, &commonapis.Result{Code: 0, Error: "ok"})

	if rec.Code != http.StatusOK {
		t.Errorf("状态码期望 200，实际 %d", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("响应体不是合法 JSON: %v (%q)", err, rec.Body.String())
	}
}

// unencodableValue json.Marshal 无法编码的值，用来走错误路径
type unencodableValue struct{}

func (unencodableValue) MarshalJSON() ([]byte, error) {
	return nil, errors.New("boom")
}

// TestWriteJSONResponseEncodeErrorDoesNotPanic 编码失败且 LOGGER 未初始化时不得 panic
//
// 改造前错误路径上是裸调 global.LOGGER.WithError(...)，
// LOGGER 为 nil（程序化构造、启动早期、单测）时会把「响应编码失败」放大成一次崩溃
//
// [EN] Before this change the error path called global.LOGGER.WithError unguarded, so a nil
// LOGGER (programmatic construction, early startup, unit tests) turned a failed response
// encoding into a crash
func TestWriteJSONResponseEncodeErrorDoesNotPanic(t *testing.T) {
	rec := httptest.NewRecorder()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("编码失败不该 panic，实际 panic: %v", r)
		}
	}()
	WriteJSONResponse(rec, http.StatusOK, unencodableValue{})
}

// TestJSONWriterPoolReuseIsIsolated 池化复用不得让两次响应串写
//
// encoder 现在永久绑定在自己的 buffer 上，每次取出先 Reset；
// 漏掉 Reset 会让上一次的响应体粘在下一次前面，这是池化最容易出的错
//
// [EN] The encoder is now permanently bound to its own buffer and the buffer is reset on
// acquisition; a missing reset would prepend the previous response to the next one, the
// classic pooling mistake
func TestJSONWriterPoolReuseIsIsolated(t *testing.T) {
	for i := 0; i < 50; i++ {
		rec := httptest.NewRecorder()
		WriteJSONResponse(rec, http.StatusOK, map[string]any{"seq": i})

		var got map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("第 %d 次响应不是合法 JSON: %v (%q)", i, err, rec.Body.String())
		}
		if int(got["seq"].(float64)) != i {
			t.Fatalf("第 %d 次响应串写，实际 seq=%v", i, got["seq"])
		}
		if len(got) != 1 {
			t.Fatalf("第 %d 次响应混入了其它字段: %v", i, got)
		}
	}
}

// TestJSONWriterPoolConcurrent 并发下每个响应独立完整
func TestJSONWriterPoolConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(seq int) {
			defer wg.Done()
			rec := httptest.NewRecorder()
			WriteJSONResponse(rec, http.StatusOK, map[string]any{"seq": seq})
			var got map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Errorf("seq=%d 响应不是合法 JSON: %v (%q)", seq, err, rec.Body.String())
				return
			}
			if int(got["seq"].(float64)) != seq {
				t.Errorf("seq=%d 响应内容错位: %v", seq, got["seq"])
			}
		}(i)
	}
	wg.Wait()
}

// discardWriter 只丢弃，不让 httptest.Recorder 的 buffer 增长掺进基准
type discardWriter struct{ h http.Header }

func (d *discardWriter) Header() http.Header {
	if d.h == nil {
		d.h = make(http.Header, 4)
	}
	return d.h
}
func (d *discardWriter) Write(b []byte) (int, error) { return len(b), nil }
func (d *discardWriter) WriteHeader(int)             {}

// BenchmarkWriteJSONResponse 响应编码基准
//
// 按响应体大小分形状：小响应是 API 常态，编码器本身的固定开销占比最高；
// 大响应则由字节拷贝主导，正好能看出「多一次 buffer 拷贝」的代价
//
// [EN] Shape-split by body size: small responses are the common API case, where the
// encoder's fixed overhead dominates, while large ones are dominated by the byte copy and
// show the cost of the extra buffer copy
func BenchmarkWriteJSONResponse(b *testing.B) {
	small := map[string]any{"code": 0, "message": "ok"}

	large := make(map[string]any, 50)
	for i := 0; i < 50; i++ {
		large[strings.Repeat("k", 8)+string(rune('a'+i%26))+string(rune('0'+i/26))] =
			strings.Repeat("value", 10)
	}

	for _, shape := range []struct {
		name string
		data any
	}{
		{"small", small},
		{"large", large},
	} {
		b.Run(shape.name, func(b *testing.B) {
			w := &discardWriter{}
			b.ReportAllocs()
			for b.Loop() {
				WriteJSONResponse(w, http.StatusOK, shape.data)
			}
		})
	}
}

// legacyJSONEncoderPool 改造前的池子，原样保留供同二进制内 A/B 对比
var legacyJSONEncoderPool = sync.Pool{
	New: func() any { return json.NewEncoder(io.Discard) },
}

// legacyWriteJSONResponse 改造前的实现
func legacyWriteJSONResponse(w http.ResponseWriter, httpStatus int, data any) {
	w.Header().Set(constants.HeaderContentType, httpx.ContentTypeApplicationJSON)
	w.WriteHeader(httpStatus)

	encoder := legacyJSONEncoderPool.Get().(*json.Encoder)
	defer legacyJSONEncoderPool.Put(encoder)

	*encoder = *json.NewEncoder(w)
	_ = encoder.Encode(data)
}

// BenchmarkWriteJSONResponseLegacy 改造前实现的基准
//
// 与 BenchmarkWriteJSONResponse 在同一个二进制里跑，避免跨次运行的噪声
// [EN] Runs in the same binary as BenchmarkWriteJSONResponse, avoiding cross-run noise
func BenchmarkWriteJSONResponseLegacy(b *testing.B) {
	small := map[string]any{"code": 0, "message": "ok"}

	large := make(map[string]any, 50)
	for i := 0; i < 50; i++ {
		large[strings.Repeat("k", 8)+string(rune('a'+i%26))+string(rune('0'+i/26))] =
			strings.Repeat("value", 10)
	}

	for _, shape := range []struct {
		name string
		data any
	}{
		{"small", small},
		{"large", large},
	} {
		b.Run(shape.name, func(b *testing.B) {
			w := &discardWriter{}
			b.ReportAllocs()
			for b.Loop() {
				legacyWriteJSONResponse(w, http.StatusOK, shape.data)
			}
		})
	}
}

// noPoolWriteJSONResponse 完全不用池子的最简实现，用来验证池子到底值不值
func noPoolWriteJSONResponse(w http.ResponseWriter, httpStatus int, data any) {
	w.Header().Set(constants.HeaderContentType, httpx.ContentTypeApplicationJSON)
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(data)
}

// BenchmarkWriteJSONResponseNoPool 无池实现的基准
func BenchmarkWriteJSONResponseNoPool(b *testing.B) {
	small := map[string]any{"code": 0, "message": "ok"}
	large := make(map[string]any, 50)
	for i := 0; i < 50; i++ {
		large[strings.Repeat("k", 8)+string(rune('a'+i%26))+string(rune('0'+i/26))] =
			strings.Repeat("value", 10)
	}
	for _, shape := range []struct {
		name string
		data any
	}{{"small", small}, {"large", large}} {
		b.Run(shape.name, func(b *testing.B) {
			w := &discardWriter{}
			b.ReportAllocs()
			for b.Loop() {
				noPoolWriteJSONResponse(w, http.StatusOK, shape.data)
			}
		})
	}
}
