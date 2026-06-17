/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-17 06:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-17 06:30:00
 * @FilePath: \go-rpc-gateway\middleware\observability\logging_toggle_test.go
 * @Description: 日志中间件开关的运行期生效性
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package observability

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/logger"
)

// TestLoggingMiddlewareHonoursEnabledPerRequest Enabled 必须在请求路径上生效，两个方向都要能切
//
// 回归点：构造期读一次配置，Enabled 为 false 就返回透传、根本不装这一层；装上之后又再也
// 不看 reqConfig.Enabled 于是两个方向的热重载都失效：启动时关着的之后打开不生效
// （层没装），启动时开着的之后关掉也停不下来（层不看开关）
//
// [EN] Regression: the config was read once at construction, a pass-through was returned when
// Enabled was false so the layer was never installed, and once installed it never looked at
// reqConfig.Enabled again. Hot reload was broken in both directions.
func TestLoggingMiddlewareHonoursEnabledPerRequest(t *testing.T) {
	cfg := gwconfig.Default()
	global.SetGatewayConfig(cfg)
	t.Cleanup(func() { global.SetGatewayConfig(gwconfig.Default()) })

	var sink bytes.Buffer
	if err := global.EnsureLoggerInitialized(); err != nil {
		t.Fatalf("logger: %v", err)
	}
	if lg, ok := global.LOGGER.(*logger.Logger); ok {
		global.LOGGER = lg.WithOutput(&sink)
	} else {
		t.Skip("全局日志器不是 *logger.Logger，无法重定向输出")
	}

	// 构造期先关着：原实现在这里就返回透传，之后无论怎么改配置都不会再记日志
	cfg.Middleware.Logging.Enabled = false
	handler := LoggingMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	serve := func() {
		sink.Reset()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/ping", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("状态码 = %d, want 200", rec.Code)
		}
	}

	serve()
	if sink.Len() != 0 {
		t.Errorf("关着日志时仍有输出:\n%s", sink.String())
	}

	// 运行期打开：必须开始记日志
	cfg.Middleware.Logging.Enabled = true
	serve()
	if sink.Len() == 0 {
		t.Error("运行期打开日志之后仍然没有输出：Enabled 的变更没有生效")
	}

	// 运行期关掉：必须停下来
	cfg.Middleware.Logging.Enabled = false
	serve()
	if sink.Len() != 0 {
		t.Errorf("运行期关掉日志之后仍有输出:\n%s", sink.String())
	}
}
