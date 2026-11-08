/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-17 06:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-17 06:30:00
 * @FilePath: \go-rpc-gateway\middleware\observability\accesslog_test.go
 * @Description: 访问日志测试：请求头脱敏 + 钩子注册表槽位回收与并发安全
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kamalyes/go-rpc-gateway/middleware/core"
)

// TestMaskHeadersRedactsCredentials 凭据类请求头不得明文进入访问日志
//
// 回归点：请求体与响应体都过了 masker.Mask，唯独请求头是
// sqltypes.CompressedTextMap(r.Header) 原样入库 —— Authorization、Cookie、X-Token、
// 签名头全以明文落在审计库里，而审计库恰恰是会被导出、被下游分析、保留期最长的那一份
//
// [EN] Regression: request and response bodies both went through masker.Mask while the headers
// went in verbatim as sqltypes.CompressedTextMap(r.Header), so Authorization, Cookie, X-Token
// and the signature headers landed in the audit store in clear — and the audit store is
// precisely the data that gets exported, analysed downstream and retained longest.
func TestMaskHeadersRedactsCredentials(t *testing.T) {
	header := http.Header{}
	header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiJ9.super-secret-payload")
	header.Set("Cookie", "session=abcdef123456; theme=dark")
	header.Set("X-Token", "tk_live_0123456789abcdef")
	header.Set("X-Signature", "9f86d081884c7d659a2feaa0c55ad015")
	header.Set("X-Api-Key", "ak_prod_zzz")
	header.Set("Content-Type", "application/json")
	header.Set("User-Agent", "curl/8.4.0")
	header.Set("X-Trace-Id", "trace-abc")

	masked := maskHeaders(header)
	if masked == nil {
		t.Fatal("maskHeaders 返回 nil，请求头整列丢失")
	}

	flat := flattenHeaders(masked)

	// 凭据原文一个都不许出现
	for _, leak := range []string{
		"super-secret-payload",
		"abcdef123456",
		"tk_live_0123456789abcdef",
		"9f86d081884c7d659a2feaa0c55ad015",
		"ak_prod_zzz",
	} {
		if strings.Contains(flat, leak) {
			t.Errorf("凭据原文仍在访问日志里: %q\n%s", leak, flat)
		}
	}

	// 排障要用的非敏感头必须留着，否则审计日志失去诊断价值
	for _, keep := range []string{"application/json", "curl/8.4.0", "trace-abc"} {
		if !strings.Contains(flat, keep) {
			t.Errorf("非敏感头被过度脱敏，缺少 %q:\n%s", keep, flat)
		}
	}

	// 键名要保留，只换值
	for _, key := range []string{"Authorization", "Cookie", "X-Token"} {
		if _, ok := masked[key]; !ok {
			t.Errorf("脱敏后丢掉了键名 %q", key)
		}
	}
}

// TestMaskHeadersDoesNotAliasRequestHeader 返回的 map 不得是请求 header 的别名
//
// 回归点：CompressedTextMap(r.Header) 是类型转换，不复制底层 map，等于把请求自己的
// header map 交给可能异步消费的钩子
func TestMaskHeadersDoesNotAliasRequestHeader(t *testing.T) {
	header := http.Header{}
	header.Set("X-Trace-Id", "trace-abc")

	masked := maskHeaders(header)
	if masked == nil {
		t.Fatal("maskHeaders 返回 nil")
	}

	header.Set("X-Trace-Id", "mutated-after-capture")
	if got := masked["X-Trace-Id"]; len(got) > 0 && got[0] == "mutated-after-capture" {
		t.Error("返回的 map 是请求 header 的别名：捕获之后的改动会反映到已入库的记录上")
	}
}

// TestMaskHeadersEmpty 空请求头返回 nil（对应 DB 里的 NULL）
func TestMaskHeadersEmpty(t *testing.T) {
	if got := maskHeaders(nil); got != nil {
		t.Errorf("空 header 应返回 nil，实际 %v", got)
	}
	if got := maskHeaders(http.Header{}); got != nil {
		t.Errorf("空 header 应返回 nil，实际 %v", got)
	}
}

// TestMaskHeadersHandlesLargeCookie 带大 Cookie 的请求头不得因截断而整列丢失
//
// 请求头专用脱敏器的 MaxBodySize 是 64 KiB，而 global.DATAMASKER 用的是
// logging.max-body-size（默认 2048）。复用后者的话，一份带大 Cookie 的浏览器请求头
// 会在脱敏入口被截断成非法 JSON，解析不回来，于是整列丢失
//
// [EN] The header-specific masker allows 64 KiB while global.DATAMASKER uses
// logging.max-body-size (2048 by default). Reusing the latter would truncate a browser's
// headers with a large cookie into invalid JSON at the masking entry point, making them
// unparseable and losing the whole column.
func TestMaskHeadersHandlesLargeCookie(t *testing.T) {
	header := http.Header{}
	header.Set("Cookie", "session="+strings.Repeat("x", 4096))
	header.Set("X-Trace-Id", "trace-abc")

	masked := maskHeaders(header)
	if masked == nil {
		t.Fatal("带大 Cookie 的请求头被整列丢弃")
	}
	flat := flattenHeaders(masked)
	if strings.Contains(flat, strings.Repeat("x", 64)) {
		t.Error("大 Cookie 的值未被脱敏")
	}
	if !strings.Contains(flat, "trace-abc") {
		t.Error("大 Cookie 把其余请求头一起带走了")
	}
}

// flattenHeaders 把脱敏结果拼成一段文本，便于断言「某个原文不存在」
func flattenHeaders(h map[string][]string) string {
	var b strings.Builder
	for name, values := range h {
		b.WriteString(name)
		b.WriteString("=")
		b.WriteString(strings.Join(values, ","))
		b.WriteString("\n")
	}
	return b.String()
}

// TestCaptureAccessLogMasksHeaders 接线用例：真的走一遍 captureAccessLog，断言落到钩子里的
// 记录已经脱敏
//
// 上面几条只测 maskHeaders 这个函数本身，把 captureAccessLog 里的调用点换回
// sqltypes.CompressedTextMap(r.Header) 它们照样全绿 —— 守不住「接线被改回去」
// 从钩子收到的记录上断言，才真正覆盖调用点
//
// [EN] The tests above exercise maskHeaders itself and stay green if the call site inside
// captureAccessLog is reverted to sqltypes.CompressedTextMap(r.Header), so they cannot hold the
// wiring. This one asserts on the record the hook receives, which does cover the call site.
func TestCaptureAccessLogMasksHeaders(t *testing.T) {
	var got *AccessRecord
	unregister := RegisterAccessLog(func(ctx context.Context, rec *AccessRecord) {
		got = rec
	})
	t.Cleanup(unregister)

	r := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(`{"id":"x"}`))
	r.Header.Set("Authorization", "Bearer super-secret-payload")
	r.Header.Set("Cookie", "session=abcdef123456")
	r.Header.Set("X-Trace-Id", "trace-abc")

	rw := core.NewResponseWriter(httptest.NewRecorder())
	t.Cleanup(rw.Release)
	rw.WriteHeader(http.StatusOK)

	captureAccessLog(context.Background(), r, rw, time.Now(), 5*time.Millisecond,
		[]byte(`{"id":"x"}`), []byte(`{"ok":true}`), time.Second)

	if got == nil {
		t.Fatal("访问日志钩子没有收到记录")
	}

	flat := flattenHeaders(got.Headers)
	for _, leak := range []string{"super-secret-payload", "abcdef123456"} {
		if strings.Contains(flat, leak) {
			t.Errorf("落到钩子里的记录仍含凭据原文 %q:\n%s", leak, flat)
		}
	}
	if !strings.Contains(flat, "trace-abc") {
		t.Errorf("非敏感头被过度脱敏:\n%s", flat)
	}
}

// =============================================================================
// 钩子注册表：槽位回收与并发安全
// =============================================================================

// accessLogSnapshotLen 当前快照长度，仅测试使用
func accessLogSnapshotLen() int {
	return len(*accessLogEntries.Load())
}

// TestRegisterAccessLogUnregisterReclaimsSlots 反注册必须真正移除槽位
//
// 回归点：原实现只把切片槽位置 nil，不回收也不收缩。插件式调用方反复
// 注册/反注册后，DispatchAccessLog 仍要遍历整段历史，切片无界增长
// [EN] Regression: unregister only set the slot to nil without reclaiming it. Repeated
// register/unregister cycles left DispatchAccessLog walking an ever-growing slice
func TestRegisterAccessLogUnregisterReclaimsSlots(t *testing.T) {
	baseLen := accessLogSnapshotLen()
	baseCount := accessLogCount.Load()

	const cycles = 64
	unregisters := make([]func(), 0, cycles)
	for i := 0; i < cycles; i++ {
		unregisters = append(unregisters, RegisterAccessLog(func(context.Context, *AccessRecord) {}))
	}

	if got := accessLogSnapshotLen(); got != baseLen+cycles {
		t.Fatalf("注册 %d 个钩子后快照长度应为 %d，实际 %d", cycles, baseLen+cycles, got)
	}
	if got := accessLogCount.Load(); got != baseCount+cycles {
		t.Fatalf("注册后计数应为 %d，实际 %d", baseCount+cycles, got)
	}

	for _, unregister := range unregisters {
		unregister()
	}

	if got := accessLogSnapshotLen(); got != baseLen {
		t.Errorf("全部反注册后快照长度应回到 %d，实际 %d（槽位未回收）", baseLen, got)
	}
	if got := accessLogCount.Load(); got != baseCount {
		t.Errorf("全部反注册后计数应回到 %d，实际 %d", baseCount, got)
	}

	// 再注册同样数量：长度仍应是 base+cycles，而不是在历史基础上继续增长
	for i := 0; i < cycles; i++ {
		unregisters[i] = RegisterAccessLog(func(context.Context, *AccessRecord) {})
	}
	if got := accessLogSnapshotLen(); got != baseLen+cycles {
		t.Errorf("再次注册 %d 个后长度应为 %d，实际 %d", cycles, baseLen+cycles, got)
	}
	for _, unregister := range unregisters {
		unregister()
	}
}

// TestRegisterAccessLogUnregisterIsIdempotent 重复调用反注册只生效一次
func TestRegisterAccessLogUnregisterIsIdempotent(t *testing.T) {
	baseLen := accessLogSnapshotLen()
	baseCount := accessLogCount.Load()

	unregister := RegisterAccessLog(func(context.Context, *AccessRecord) {})
	unregister()
	unregister()
	unregister()

	if got := accessLogSnapshotLen(); got != baseLen {
		t.Errorf("重复反注册后长度应为 %d，实际 %d", baseLen, got)
	}
	if got := accessLogCount.Load(); got != baseCount {
		t.Errorf("重复反注册后计数应为 %d，实际 %d", baseCount, got)
	}
}

// TestAccessLogRegistryConcurrent 注册/反注册/派发并发不应有数据竞争
//
// 覆盖写时复制快照的 CAS 循环：并发注册与反注册必须都成功发布，
// 且最终计数回到初始值
// [EN] Exercises the copy-on-write CAS loop: concurrent registrations and unregistrations
// must all publish, and the final count must return to the starting value
func TestAccessLogRegistryConcurrent(t *testing.T) {
	baseLen := accessLogSnapshotLen()
	baseCount := accessLogCount.Load()

	const (
		workers = 8
		rounds  = 100
	)

	stop := make(chan struct{})
	var dispatchWG sync.WaitGroup
	dispatchWG.Add(1)
	go func() {
		defer dispatchWG.Done()
		for {
			select {
			case <-stop:
				return
			default:
				DispatchAccessLog(context.Background(), &AccessRecord{})
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < rounds; j++ {
				unregister := RegisterAccessLog(func(context.Context, *AccessRecord) {})
				unregister()
			}
		}()
	}

	wg.Wait()
	close(stop)
	dispatchWG.Wait()

	if got := accessLogSnapshotLen(); got != baseLen {
		t.Errorf("并发结束后长度应回到 %d，实际 %d", baseLen, got)
	}
	if got := accessLogCount.Load(); got != baseCount {
		t.Errorf("并发结束后计数应回到 %d，实际 %d", baseCount, got)
	}
}
