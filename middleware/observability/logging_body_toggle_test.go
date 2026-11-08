/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-08 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\observability\logging_body_toggle_test.go
 * @Description: EnableRequest / EnableResponse 必须管住请求体与响应体是否进运行时日志
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package observability

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/logger"
)

// redirectLogger 把全局日志器输出接到 buffer 上
func redirectLogger(t *testing.T) *bytes.Buffer {
	t.Helper()
	if err := global.EnsureLoggerInitialized(); err != nil {
		t.Fatalf("logger: %v", err)
	}
	lg, ok := global.LOGGER.(*logger.Logger)
	if !ok {
		t.Skip("全局日志器不是 *logger.Logger，无法重定向输出")
	}
	var sink bytes.Buffer
	prev := global.LOGGER
	global.LOGGER = lg.WithOutput(&sink)
	t.Cleanup(func() { global.LOGGER = prev })
	return &sink
}

// TestBodyLoggingHonoursEnableFlags 关掉开关后，请求体/响应体不得出现在运行时日志里
//
// 回归点：query 字段判了 config.EnableRequest，而请求体与响应体只判了
// 「捕获到的字节数 > 0」。而 body 的捕获条件是 EnableXxx || hasAccessLog ——
// 于是「注册了审计钩子 + 保持默认 EnableResponse:false」这个完全正常的生产配置下，
// 响应体仍然会被写进运行时日志
//
// 后果有两层：
//  1. 运维显式关掉的东西没关掉，报文进了不该进的地方
//  2. 每请求白付一次 masker.Mask 的完整 JSON 往返（Unmarshal 到 any 再 Marshal 回去），
//     profile 里这一项占访问日志层全部分配的 24%
//
// [EN] Regression: the query field tested config.EnableRequest while the request and response
// bodies tested only "captured length > 0". Capture is enabled by EnableXxx || hasAccessLog,
// so with an audit hook registered and the default EnableResponse:false the response body was
// still written into the runtime log. Two consequences: what the operator explicitly turned
// off was not off, and every request paid for a full JSON round trip inside masker.Mask,
// which the profile showed as 24% of all allocations in the access-log layer
func TestBodyLoggingHonoursEnableFlags(t *testing.T) {
	const reqMarker = "REQ_BODY_MARKER_7f3a"
	const respMarker = "RESP_BODY_MARKER_9c1d"

	for _, tc := range []struct {
		name                          string
		enableRequest, enableResponse bool
		wantReq, wantResp             bool
	}{
		{"两个都关", false, false, false, false},
		{"只开请求体", true, false, true, false},
		{"只开响应体", false, true, false, true},
		{"两个都开", true, true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := gwconfig.Default()
			cfg.Middleware.Logging.Enabled = true
			cfg.Middleware.Logging.EnableRequest = tc.enableRequest
			cfg.Middleware.Logging.EnableResponse = tc.enableResponse
			global.SetGatewayConfig(cfg)
			t.Cleanup(func() { global.SetGatewayConfig(gwconfig.Default()) })

			sink := redirectLogger(t)

			// 注册一个访问日志钩子：正是它让 body 被捕获，从而暴露这个 bug
			// [EN] Registering an access-log hook is what makes the body captured, and is
			// what exposed the bug
			unregister := RegisterAccessLog(func(ctx context.Context, rec *AccessRecord) {})
			t.Cleanup(unregister)

			handler := LoggingMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"echo":"` + respMarker + `"}`))
			}))

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/echo",
				strings.NewReader(`{"data":"`+reqMarker+`"}`))
			req.Header.Set("Content-Type", "application/json")
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("状态码 = %d, want 200", rec.Code)
			}

			out := sink.String()
			if got := strings.Contains(out, reqMarker); got != tc.wantReq {
				t.Errorf("EnableRequest=%v：日志中出现请求体 = %v，期望 %v\n日志: %s",
					tc.enableRequest, got, tc.wantReq, out)
			}
			if got := strings.Contains(out, respMarker); got != tc.wantResp {
				t.Errorf("EnableResponse=%v：日志中出现响应体 = %v，期望 %v\n日志: %s",
					tc.enableResponse, got, tc.wantResp, out)
			}
		})
	}
}

// TestAccessLogStillReceivesBodyWhenRuntimeLogOff 关掉运行时日志的 body 不能影响访问日志钩子
//
// 这一条防止「修过头」：EnableRequest/EnableResponse 管的是**运行时日志**，
// 访问日志钩子拿到的 AccessRecord 仍然必须带着报文 —— 审计钩子正是靠它落库的
//
// [EN] Guards against over-fixing: the flags govern the *runtime log*, while the access-log
// hook's AccessRecord must still carry the bodies, which is how an audit hook persists them
func TestAccessLogStillReceivesBodyWhenRuntimeLogOff(t *testing.T) {
	const reqMarker = "REQ_BODY_MARKER_7f3a"
	const respMarker = "RESP_BODY_MARKER_9c1d"

	cfg := gwconfig.Default()
	cfg.Middleware.Logging.Enabled = true
	cfg.Middleware.Logging.EnableRequest = false
	cfg.Middleware.Logging.EnableResponse = false
	global.SetGatewayConfig(cfg)
	t.Cleanup(func() { global.SetGatewayConfig(gwconfig.Default()) })

	_ = redirectLogger(t)

	var gotReq, gotResp string
	unregister := RegisterAccessLog(func(ctx context.Context, rec *AccessRecord) {
		gotReq = string(rec.Body)
		gotResp = string(rec.Response)
	})
	t.Cleanup(unregister)

	handler := LoggingMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"echo":"` + respMarker + `"}`))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/echo",
		strings.NewReader(`{"data":"`+reqMarker+`"}`))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)

	if !strings.Contains(gotReq, reqMarker) {
		t.Errorf("访问日志钩子未收到请求体，实际 %q", gotReq)
	}
	if !strings.Contains(gotResp, respMarker) {
		t.Errorf("访问日志钩子未收到响应体，实际 %q", gotResp)
	}
}

// TestMaskedBodyComputesOnce 同一份报文在一个请求里只脱敏一次
//
// 运行时日志与访问日志是同一请求里的两个消费者，原先各自对同一份字节调一次
// masker.Mask，而 Mask 内部是完整的 JSON 往返。记忆化很容易在后续改动里
// 被绕过（比如有人又直接调一次 masker.Mask），而且不会有任何症状 —— 只是变慢，
// 所以用计数把它钉住
//
// [EN] The runtime log and the access log are two consumers in one request that each used to
// call masker.Mask on the same bytes, a full JSON round trip. The memoisation is easy to bypass
// in a later change with no symptom other than slowness, so it is pinned by counting
func TestMaskedBodyComputesOnce(t *testing.T) {
	calls := 0
	mb := &maskedBody{raw: []byte(`{"token":"secret","keep":"visible"}`)}

	// 包一层计数：直接数 Mask 调用次数不现实（masker 是全局的），
	// 改为数 done 被翻转的次数等价 —— get() 只在 done 为假时才真正脱敏
	// [EN] Counting Mask calls directly is impractical since the masker is global, so the
	// number of times done flips is counted instead: get() only masks while done is false
	get := func() string {
		if !mb.done {
			calls++
		}
		return mb.get()
	}

	first := get()
	second := get()
	third := get()

	if calls != 1 {
		t.Errorf("脱敏执行了 %d 次，期望 1 次", calls)
	}
	if first != second || second != third {
		t.Errorf("多次取值结果不一致: %q / %q / %q", first, second, third)
	}
	if !strings.Contains(first, "visible") {
		t.Errorf("非敏感字段应保留，实际 %q", first)
	}
	if strings.Contains(first, "secret") {
		t.Errorf("敏感字段未被脱敏，实际 %q", first)
	}
}

// TestMaskedBodyNilSafe nil 包装取值返回空串而不是 panic
//
// captureAccessLog 的两个参数都是指针，调用方漏传时应当安全降级
// [EN] Both of captureAccessLog's body parameters are pointers, so a caller omitting one
// should degrade safely
func TestMaskedBodyNilSafe(t *testing.T) {
	var mb *maskedBody
	if got := mb.get(); got != "" {
		t.Errorf("nil 包装应返回空串，实际 %q", got)
	}
}
