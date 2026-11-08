/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-17 05:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-17 05:00:00
 * @FilePath: \go-rpc-gateway\middleware\core\request_context_race_test.go
 * @Description: 配置热重载与请求路径并发读的竞态回归
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package core

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/global"
)

// TestRequestContextMiddlewareDuringConfigReload 热重载换配置时，请求路径读配置不得竞态
//
// 回归点：中间件原先每请求直接读 `global.GATEWAY.RequestContext`，而 gateway.go 的配置
// 变更回调里是 `global.GATEWAY = newConfig` —— 一个普通包级指针变量摆在「每请求读」与
// 「热重载写」之间。这不只是指针会不会被撕开：换上去那份 Gateway 的字段写入与指针写入
// 之间没有屏障，读侧可能看到指针已换、结构体字段还没可见，拿到一份半初始化的配置，
// 而这条路径决定的正是请求属性怎么提取
//
// 这个用例必须用 -race 跑才有意义：不带 -race 时它只验证重载期间中间件仍能正常服务
//
//	go test ./middleware/core/ -run TestRequestContextMiddlewareDuringConfigReload -race
//
// [EN] Regression: the middleware used to read global.GATEWAY.RequestContext directly on every
// request while the config-change callback in gateway.go did global.GATEWAY = newConfig — a
// plain package-level pointer between a per-request read and a hot-reload write. It is not only
// about whether the pointer tears: there is no barrier between the incoming Gateway's field
// writes and the pointer write, so a reader can observe the new pointer while the struct's
// fields are not yet visible and get a half-initialised config, on the very path that decides
// how request attributes are extracted.
//
// This test only carries weight under -race; without it the test merely checks that the
// middleware keeps serving across a reload.
func TestRequestContextMiddlewareDuringConfigReload(t *testing.T) {
	cfgA := gwconfig.Default()
	cfgB := gwconfig.Default()
	cfgB.Name = "reloaded"

	global.SetGatewayConfig(cfgA)
	t.Cleanup(func() { global.SetGatewayConfig(gwconfig.Default()) })

	handler := RequestContextMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// trace_id 必须一路可读：读侧拿到半初始化配置时，提取索引会是空的
		if GetTraceID(r.Context()) == "" {
			t.Error("请求上下文里没有 trace_id")
		}
		w.WriteHeader(http.StatusOK)
	}))

	const readers = 8
	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				req := httptest.NewRequest(http.MethodGet, "/v1/ping", nil)
				req.Header.Set("X-Trace-ID", "trace-from-header")
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Errorf("状态码 = %d, want 200", rec.Code)
					return
				}
			}
		}()
	}

	// 写侧：等价于配置变更回调里的那一行
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(stop)
		for i := 0; i < 2000; i++ {
			if i%2 == 0 {
				global.SetGatewayConfig(cfgB)
			} else {
				global.SetGatewayConfig(cfgA)
			}
		}
	}()

	wg.Wait()
}

// TestRequestContextMiddlewareWithoutGlobalConfig 全局配置为 nil 时不得 panic
//
// 回归点：原先 `global.GATEWAY.RequestContext` 在配置为 nil 时直接 panic
// 而 buildAttrIndex 本来就接受 nil 配置（返回一张空索引），所以这里只差一个 nil 保护
// 关停流程会把配置置空，在飞请求正好撞上这一下
//
// [EN] Regression: global.GATEWAY.RequestContext panicked outright when the config was nil,
// although buildAttrIndex already accepts a nil config and answers with an empty index, so only
// a nil guard was missing. Shutdown clears the config, which is exactly when in-flight requests
// hit this.
func TestRequestContextMiddlewareWithoutGlobalConfig(t *testing.T) {
	global.SetGatewayConfig(nil)
	global.GATEWAY = nil
	t.Cleanup(func() { global.SetGatewayConfig(gwconfig.Default()) })

	handler := RequestContextMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/ping", nil)
	handler.ServeHTTP(rec, req) // 原实现在这里 panic

	if rec.Code != http.StatusOK {
		t.Errorf("状态码 = %d, want 200", rec.Code)
	}
	// 取不到配置也要照常兜底生成 trace_id 并回写响应头
	if rec.Header().Get("X-Trace-Id") == "" && rec.Header().Get("X-Trace-ID") == "" {
		t.Error("配置缺失时仍应生成并回写 trace_id")
	}
}

// TestGetGatewayConfigFallsBackToVar 原子存储为空时回退读 GATEWAY 变量
//
// 仓库里有 8 处测试直接写 `global.GATEWAY = cfg`，这条回退是它们照旧可用的依据
// [EN] Eight tests in this repository assign global.GATEWAY = cfg directly; this fallback is
// what keeps them working.
func TestGetGatewayConfigFallsBackToVar(t *testing.T) {
	global.SetGatewayConfig(nil)
	t.Cleanup(func() { global.SetGatewayConfig(gwconfig.Default()) })

	legacy := gwconfig.Default()
	legacy.Name = "set-through-var"
	global.GATEWAY = legacy

	got := global.GetGatewayConfig()
	if got == nil || got.Name != "set-through-var" {
		t.Fatalf("回退失效：GetGatewayConfig() = %+v", got)
	}

	// SetGatewayConfig 之后原子存储成为权威来源，并同步写一次变量
	atomicCfg := gwconfig.Default()
	atomicCfg.Name = "set-through-atomic"
	global.SetGatewayConfig(atomicCfg)

	if got := global.GetGatewayConfig(); got.Name != "set-through-atomic" {
		t.Errorf("GetGatewayConfig() = %q, want set-through-atomic", got.Name)
	}
	if global.GATEWAY == nil || global.GATEWAY.Name != "set-through-atomic" {
		t.Error("SetGatewayConfig 未同步更新 GATEWAY 变量，直接读它的外部代码会看到旧配置")
	}
}
