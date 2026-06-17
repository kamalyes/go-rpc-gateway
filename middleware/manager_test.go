/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 05:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 05:00:00
 * @FilePath: \go-rpc-gateway\middleware\manager_test.go
 * @Description: 中间件管理器热重载 - 跨配置保留项与旧实例资源回收
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	gcmiddleware "github.com/kamalyes/go-rpc-gateway/config/middleware"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/logger"
)

// newManagerTestConfig 一份可用的网关配置，日志写到 io.Discard
func newManagerTestConfig(t testing.TB) *gwconfig.Gateway {
	t.Helper()

	cfg := gwconfig.Default()
	global.GATEWAY = cfg
	if err := global.EnsureLoggerInitialized(); err != nil {
		t.Fatalf("logger: %v", err)
	}
	if lg, ok := global.LOGGER.(*logger.Logger); ok {
		global.LOGGER = lg.WithOutput(io.Discard)
	}
	return cfg
}

// TestUpdateConfigKeepsPreRateLimitMiddlewares 热重载不得丢掉限流前自定义中间件
//
// 回归点：UpdateConfig 用新配置整建一个管理器再 *m = *next 换上去，而只把
// dynamicRateLimit 与 dynamicSignature 带了过去 preRateLimitMiddlewares 的文档用途
// 就是放认证中间件（见 AddPreRateLimitMiddleware），丢掉等于热重载之后认证从 HTTP 链上
// 消失，而且不报错不告警 —— 表现是重载后接口突然不需要凭据
//
// [EN] Regression: UpdateConfig builds a whole new manager and swaps it in with *m = *next
// while carrying over only dynamicRateLimit and dynamicSignature. The documented purpose of
// preRateLimitMiddlewares is authentication middleware (see AddPreRateLimitMiddleware), so
// losing it means authentication silently disappears from the HTTP chain after a reload, with
// no error and no warning: endpoints simply stop requiring credentials.
func TestUpdateConfigKeepsPreRateLimitMiddlewares(t *testing.T) {
	cfg := newManagerTestConfig(t)

	m, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })

	// 一个站在认证位置上的标记中间件：命中就在响应头上留痕
	var marker HTTPMiddleware = func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Auth-Ran", "1")
			next.ServeHTTP(w, r)
		})
	}
	m.AddPreRateLimitMiddleware(marker)

	ranBefore := authMarkerRan(t, m)
	if !ranBefore {
		t.Fatal("注册之后标记中间件就没在链上，用例前提不成立")
	}

	newCfg := gwconfig.Default()
	newCfg.Name = "reloaded"
	if err := m.UpdateConfig(newCfg); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}

	if !authMarkerRan(t, m) {
		t.Error("热重载之后限流前自定义中间件从链上消失了：认证会被静默跳过")
	}
}

// authMarkerRan 走一遍当前链，看标记中间件是否执行
func authMarkerRan(t *testing.T, m *Manager) bool {
	t.Helper()

	handler := ApplyMiddlewares(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), m.GetMiddlewares()...)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/ping", nil))
	return rec.Header().Get("X-Auth-Ran") == "1"
}

// TestUpdateConfigReclaimsOldLimiter 热重载不得每次泄一个限流清理协程
//
// 回归点：FixedWindowLimiter 在构造函数里就 go cleanup()，它的 Stop 一直是写好的
// 但没有任何调用方 UpdateConfig 把旧管理器整个丢掉，于是每换一次配置就留下一个
// 常驻协程加一个 ticker
//
// 这里用协程数而不是去看 limiter 内部状态：stopChan 在另一个包里且未导出，
// 而「有没有泄漏」本来就该按进程里多出来几个协程来判
//
// [EN] Regression: FixedWindowLimiter starts go cleanup() in its constructor and its Stop was
// written but had no caller, while UpdateConfig dropped the whole old manager — so every config
// change left one resident goroutine and one ticker behind.
//
// Goroutine count is used rather than the limiter's internals because stopChan lives in another
// package and is unexported, and "did it leak" is properly judged by how many goroutines the
// process gained.
func TestUpdateConfigReclaimsOldLimiter(t *testing.T) {
	cfg := newManagerTestConfig(t)
	cfg.RateLimit.Enabled = true
	cfg.RateLimit.Strategy = gcmiddleware.StrategyFixedWindow

	m, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })

	// 先让首个 limiter 的协程起来，再取基线
	settleGoroutines()
	before := runtime.NumGoroutine()

	const reloads = 20
	for i := 0; i < reloads; i++ {
		next := gwconfig.Default()
		next.RateLimit.Enabled = true
		next.RateLimit.Strategy = gcmiddleware.StrategyFixedWindow
		if err := m.UpdateConfig(next); err != nil {
			t.Fatalf("第 %d 次 UpdateConfig: %v", i, err)
		}
	}

	settleGoroutines()
	after := runtime.NumGoroutine()

	// 每次重载泄一个的话增量就是 reloads 量级；留一半的宽容度吸收运行时噪声
	// [EN] Leaking one per reload puts the delta at the order of reloads; half of that is left
	// as slack to absorb runtime noise
	if grew := after - before; grew > reloads/2 {
		t.Errorf("%d 次热重载后协程数由 %d 增至 %d（+%d），旧限流器的清理协程没有被回收",
			reloads, before, after, grew)
	}
}

// settleGoroutines 给已被要求退出的协程一点时间真的退出
func settleGoroutines() {
	for i := 0; i < 50; i++ {
		runtime.Gosched()
		time.Sleep(2 * time.Millisecond)
	}
}

// TestManagerCloseIsSafeToRepeat Close 要能重复调用，也要容忍 nil 接收者
//
// 关停路径与热重载路径都会调它，重复或在半初始化状态下调用都不应 panic
func TestManagerCloseIsSafeToRepeat(t *testing.T) {
	cfg := newManagerTestConfig(t)

	m, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Errorf("首次 Close 返回错误: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Errorf("重复 Close 返回错误: %v", err)
	}

	var nilManager *Manager
	if err := nilManager.Close(); err != nil {
		t.Errorf("nil 接收者 Close 返回错误: %v", err)
	}
}

// TestUpdateConfigOnNilManager nil 管理器上调用 UpdateConfig 不应 panic（原有行为，守住）
func TestUpdateConfigOnNilManager(t *testing.T) {
	var nilManager *Manager
	if err := nilManager.UpdateConfig(gwconfig.Default()); err != nil {
		t.Errorf("nil 接收者 UpdateConfig 返回错误: %v", err)
	}
}
