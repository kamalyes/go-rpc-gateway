/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-08 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\observability\accesslog_types_test.go
 * @Description: 访问日志列类型测试：默认落库格式兼容性 + 编解码器注入
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package observability

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/kamalyes/go-toolbox/pkg/zipx"
)

// recordingCodec 记录调用次数的编解码器，用来断言注入真的生效
type recordingCodec struct {
	mu      sync.Mutex
	encodes int
	decodes int
}

func (c *recordingCodec) Encode(data []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.encodes++
	return append([]byte("ENC:"), data...), nil
}

func (c *recordingCodec) Decode(data []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.decodes++
	return []byte(strings.TrimPrefix(string(data), "ENC:")), nil
}

// TestAccessTextDefaultCodecMatchesLegacyFormat 未注入时的落库格式必须与改造前逐字节一致
//
// 这是整个改造里唯一不可回退的约束：审计库里已经有用 sqltypes.CompressedText 写进去的
// 数据（gzip + GZIP: 前缀）。默认编解码器一旦换成明文透传，升级后读历史行拿到的是一堆
// 压缩字节，而审计数据恰恰是保留期最长、最不该坏的那一份
//
// [EN] The one constraint in this refactor that cannot be relaxed: audit stores already hold
// data written through sqltypes.CompressedText (gzip plus a GZIP: prefix). If the default codec
// became a plain passthrough, legacy rows would read back as compressed bytes after the
// upgrade — and audit data is the longest-retained data there is
func TestAccessTextDefaultCodecMatchesLegacyFormat(t *testing.T) {
	const payload = `{"user":"alice","action":"login"}`

	got, err := AccessText(payload).Value()
	if err != nil {
		t.Fatalf("Value() 出错: %v", err)
	}

	want, err := zipx.GzipCompressWithPrefix([]byte(payload))
	if err != nil {
		t.Fatalf("zipx.GzipCompressWithPrefix 出错: %v", err)
	}

	if !reflect.DeepEqual(got, any(want)) {
		t.Errorf("默认落库格式与 zipx.GzipCompressWithPrefix 不一致\ngot  = %v\nwant = %v", got, want)
	}
}

// TestAccessTextReadsLegacyPlainRows 改造前未压缩的历史明文仍要能读出来
//
// zipx.GzipSmartDecompress 对无 GZIP: 前缀的数据原样返回，这条测试把这个兼容性
// 钉在访问日志这一层，而不是只信赖库的行为
func TestAccessTextReadsLegacyPlainRows(t *testing.T) {
	const plain = "legacy row written before compression"

	var text AccessText
	if err := text.Scan(plain); err != nil {
		t.Fatalf("Scan 明文出错: %v", err)
	}
	if string(text) != plain {
		t.Errorf("历史明文读出来变了\ngot  = %q\nwant = %q", string(text), plain)
	}
}

// TestAccessTextRoundTrip 默认编解码器下的写入—读取往返
func TestAccessTextRoundTrip(t *testing.T) {
	const payload = "响应体 with unicode 和 换行\n第二行"

	stored, err := AccessText(payload).Value()
	if err != nil {
		t.Fatalf("Value() 出错: %v", err)
	}

	var back AccessText
	if err := back.Scan(stored); err != nil {
		t.Fatalf("Scan 出错: %v", err)
	}
	if string(back) != payload {
		t.Errorf("往返后内容变了\ngot  = %q\nwant = %q", string(back), payload)
	}
}

// TestAccessTextEmptyIsNull 空串写 NULL、NULL 读回空串
//
// 与改造前 sqltypes.CompressedText 的空值语义一致：审计库里的空请求体是 NULL 而不是
// 一段「压缩后的空字符串」
func TestAccessTextEmptyIsNull(t *testing.T) {
	got, err := AccessText("").Value()
	if err != nil {
		t.Fatalf("Value() 出错: %v", err)
	}
	if got != nil {
		t.Errorf("空串应写 NULL，实际 %v", got)
	}

	text := AccessText("not empty")
	if err := text.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) 出错: %v", err)
	}
	if text != "" {
		t.Errorf("NULL 应读回空串，实际 %q", string(text))
	}
}

// TestAccessHeadersRoundTrip 请求头列的 JSON + 编解码往返
func TestAccessHeadersRoundTrip(t *testing.T) {
	headers := AccessHeaders{
		"Content-Type":  {"application/json"},
		"Accept":        {"application/json", "text/plain"},
		"X-Request-Id":  {"req-123"},
		"X-Empty-Value": {},
	}

	stored, err := headers.Value()
	if err != nil {
		t.Fatalf("Value() 出错: %v", err)
	}

	var back AccessHeaders
	if err := back.Scan(stored); err != nil {
		t.Fatalf("Scan 出错: %v", err)
	}
	if !reflect.DeepEqual(back.ToMap(), headers.ToMap()) {
		t.Errorf("往返后请求头变了\ngot  = %v\nwant = %v", back, headers)
	}
}

// TestAccessHeadersNilIsNull nil map 写 NULL、NULL 读回 nil
func TestAccessHeadersNilIsNull(t *testing.T) {
	got, err := AccessHeaders(nil).Value()
	if err != nil {
		t.Fatalf("Value() 出错: %v", err)
	}
	if got != nil {
		t.Errorf("nil map 应写 NULL，实际 %v", got)
	}

	headers := AccessHeaders{"X-A": {"b"}}
	if err := headers.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) 出错: %v", err)
	}
	if headers != nil {
		t.Errorf("NULL 应读回 nil，实际 %v", headers)
	}
}

// TestSetAccessLogCodecIsUsedAndRestores 注入的编解码器要真的接到 Value/Scan 上，
// 恢复函数要把它摘干净
//
// 这条测试守的是「接口白给了」这种失败：类型上留了注入点，但 Value/Scan 里仍然直接
// 调 zipx，注入与不注入的落库结果完全一样
func TestSetAccessLogCodecIsUsedAndRestores(t *testing.T) {
	codec := &recordingCodec{}
	restore := SetAccessLogCodec(codec)

	stored, err := AccessText("payload").Value()
	if err != nil {
		t.Fatalf("Value() 出错: %v", err)
	}
	if want := "ENC:payload"; string(stored.([]byte)) != want {
		t.Errorf("注入的编解码器没接上\ngot  = %q\nwant = %q", stored, want)
	}

	var back AccessText
	if err := back.Scan(stored); err != nil {
		t.Fatalf("Scan 出错: %v", err)
	}
	if string(back) != "payload" {
		t.Errorf("注入的编解码器往返后内容变了: %q", string(back))
	}
	if codec.encodes != 1 || codec.decodes != 1 {
		t.Errorf("编解码器调用次数不对: encodes=%d decodes=%d，期望各 1 次", codec.encodes, codec.decodes)
	}

	restore()
	restore() // 幂等：重复调用只生效一次

	// 恢复后回到默认 gzip 格式
	after, err := AccessText("payload").Value()
	if err != nil {
		t.Fatalf("恢复后 Value() 出错: %v", err)
	}
	want, err := zipx.GzipCompressWithPrefix([]byte("payload"))
	if err != nil {
		t.Fatalf("zipx.GzipCompressWithPrefix 出错: %v", err)
	}
	if !reflect.DeepEqual(after, any(want)) {
		t.Error("恢复函数没有把编解码器还原成默认 gzip")
	}
}

// TestPlainAccessLogCodec 透传编解码器下落库内容就是原文
func TestPlainAccessLogCodec(t *testing.T) {
	restore := SetAccessLogCodec(PlainAccessLogCodec{})
	defer restore()

	const payload = `{"k":"v"}`
	stored, err := AccessText(payload).Value()
	if err != nil {
		t.Fatalf("Value() 出错: %v", err)
	}
	if string(stored.([]byte)) != payload {
		t.Errorf("透传编解码器下落库内容应为原文\ngot  = %q\nwant = %q", stored, payload)
	}
}

// TestAccessLogColumnTypesDeclareGormDataType 两个列类型都必须声明 GormDataType
//
// 回归点：DataType 为空时 gorm 的 schema 解析会把字段误判成关联关系，AccessRecord
// 直接当 gorm 模型用的调用方会在 parseRelation 上报错 —— 而这个坑在编译期看不出来，
// 只在建表/首次写入时炸
//
// [EN] Regression guard: with an empty DataType gorm's schema parse mistakes the field for a
// relation and a caller using AccessRecord as a gorm model fails in parseRelation — a trap
// invisible at compile time that only fires on migration or the first write
func TestAccessLogColumnTypesDeclareGormDataType(t *testing.T) {
	if got := AccessText("").GormDataType(); got != "string" {
		t.Errorf("AccessText.GormDataType() = %q，期望 \"string\"", got)
	}
	if got := AccessHeaders(nil).GormDataType(); got != "string" {
		t.Errorf("AccessHeaders.GormDataType() = %q，期望 \"string\"", got)
	}
}

// TestAccessLogScanRejectsUnsupportedType 不认识的数据库值要报错而不是静默吞掉
func TestAccessLogScanRejectsUnsupportedType(t *testing.T) {
	var text AccessText
	if err := text.Scan(42); err == nil {
		t.Error("Scan(int) 应报错")
	}

	var headers AccessHeaders
	if err := headers.Scan(42); err == nil {
		t.Error("Scan(int) 应报错")
	}
}
