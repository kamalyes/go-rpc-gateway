/**
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-18 13:25:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-18 17:32:18
 * @FilePath: \go-rpc-gateway\middleware\observability_test.go
 * @Description: 可观测性管理器测试 - 验证 registry 暴露、自定义 Collector、限流拒绝记录
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kamalyes/go-config/pkg/monitoring"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// newTestMonitoringConfig 创建测试用监控配置（已启用 metrics）
func newTestMonitoringConfig() *monitoring.Monitoring {
	return &monitoring.Monitoring{
		Enabled: true,
		Metrics: &monitoring.Metrics{
			Enabled: true,
			Buckets: []float64{0.1, 0.5, 1},
		},
	}
}

func TestNewMetricsManager_Disabled(t *testing.T) {
	cfg := &monitoring.Monitoring{
		Enabled: true,
		Metrics: &monitoring.Metrics{Enabled: false},
	}
	if mm := NewMetricsManager(cfg); mm != nil {
		t.Error("metrics 未启用时应返回 nil")
	}
}

func TestNewMetricsManager_RegistryContainsRuntime(t *testing.T) {
	mm := NewMetricsManager(newTestMonitoringConfig())
	if mm == nil {
		t.Fatal("启用时应返回非 nil MetricsManager")
	}
	// 验证 registry 包含 Go runtime 指标
	families, err := mm.registry.Gather()
	if err != nil {
		t.Fatalf("Gather 失败: %v", err)
	}
	names := make(map[string]bool)
	for _, f := range families {
		names[f.GetName()] = true
	}
	for _, expected := range []string{"go_goroutines", "process_resident_memory_bytes"} {
		if !names[expected] {
			t.Errorf("registry 应包含 runtime 指标 %s", expected)
		}
	}
}

func TestMetricsHandler_ExposesAllMetrics(t *testing.T) {
	mm := NewMetricsManager(newTestMonitoringConfig())

	// 注入连接池采集函数
	mm.SetPoolHealthFn(func() map[string]bool {
		return map[string]bool{"database": true, "redis": false}
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	mm.Handler().ServeHTTP(rr, req)

	body := rr.Body.String()
	// 验证 runtime 指标存在（修复 bug 后应保留）
	if !strings.Contains(body, "go_goroutines") {
		t.Error("指标输出应包含 go_goroutines（runtime 指标）")
	}
	// 验证 process 指标存在（修复 bug 后应保留）
	if !strings.Contains(body, "process_resident_memory_bytes") {
		t.Error("指标输出应包含 process_resident_memory_bytes（process 指标）")
	}
	// 验证业务指标存在（修复 bug 后应暴露，自定义 Collector 按需输出）
	if !strings.Contains(body, "gateway_pool_health") {
		t.Error("指标输出应包含 gateway_pool_health（业务指标）")
	}
}

func TestPoolCollector(t *testing.T) {
	mm := NewMetricsManager(newTestMonitoringConfig())
	mm.SetPoolHealthFn(func() map[string]bool {
		return map[string]bool{"database": true, "redis": false}
	})

	families, err := mm.registry.Gather()
	if err != nil {
		t.Fatalf("Gather 失败: %v", err)
	}

	var found bool
	var dbVal, redisVal float64
	for _, f := range families {
		if f.GetName() == "gateway_pool_health" {
			found = true
			for _, m := range f.GetMetric() {
				component := ""
				for _, l := range m.GetLabel() {
					if l.GetName() == "component" {
						component = l.GetValue()
					}
				}
				switch component {
				case "database":
					dbVal = m.GetGauge().GetValue()
				case "redis":
					redisVal = m.GetGauge().GetValue()
				}
			}
		}
	}

	if !found {
		t.Fatal("未找到 gateway_pool_health 指标")
	}
	if dbVal != 1 {
		t.Errorf("database 健康值应为 1, got %v", dbVal)
	}
	if redisVal != 0 {
		t.Errorf("redis 不健康值应为 0, got %v", redisVal)
	}
}

func TestPoolCollector_NilFn(t *testing.T) {
	mm := NewMetricsManager(newTestMonitoringConfig())
	// 未注入采集函数，不应 panic 且不输出指标
	families, err := mm.registry.Gather()
	if err != nil {
		t.Fatalf("Gather 失败: %v", err)
	}
	for _, f := range families {
		if f.GetName() == "gateway_pool_health" {
			if len(f.GetMetric()) > 0 {
				t.Error("未注入采集函数时不应输出 pool 指标")
			}
		}
	}
}

func TestBreakerCollector(t *testing.T) {
	mm := NewMetricsManager(newTestMonitoringConfig())
	mm.SetBreakerStatsFn(func() []BreakerStat {
		return []BreakerStat{
			{Path: "/api/users", State: "closed", TotalRequests: 100, FailedRequests: 5, FailureCount: 1, SuccessCount: 0},
			{Path: "/api/orders", State: "open", TotalRequests: 50, FailedRequests: 50, FailureCount: 10, SuccessCount: 0},
		}
	})

	families, err := mm.registry.Gather()
	if err != nil {
		t.Fatalf("Gather 失败: %v", err)
	}

	var stateFamily, totalFamily, failedFamily bool
	var openStateVal float64
	for _, f := range families {
		switch f.GetName() {
		case "gateway_breaker_state":
			stateFamily = true
			for _, m := range f.GetMetric() {
				for _, l := range m.GetLabel() {
					if l.GetName() == "path" && l.GetValue() == "/api/orders" {
						openStateVal = m.GetGauge().GetValue()
					}
				}
			}
		case "gateway_breaker_requests_total":
			totalFamily = true
		case "gateway_breaker_failed_requests_total":
			failedFamily = true
		}
	}

	if !stateFamily {
		t.Error("未找到 gateway_breaker_state 指标")
	}
	if !totalFamily {
		t.Error("未找到 gateway_breaker_requests_total 指标")
	}
	if !failedFamily {
		t.Error("未找到 gateway_breaker_failed_requests_total 指标")
	}
	if openStateVal != 1 {
		t.Errorf("open 状态值应为 1, got %v", openStateVal)
	}
}

func TestWSCCollector(t *testing.T) {
	mm := NewMetricsManager(newTestMonitoringConfig())
	mm.SetWSCStatsFn(func() *WSCStats {
		return &WSCStats{
			TotalClients:     10,
			WebSocketClients: 8,
			SSEClients:       2,
			OnlineUsers:      5,
			MessagesSent:     1000,
			MessagesReceived: 2000,
			BroadcastsSent:   100,
			QueuedMessages:   3,
			Uptime:           60000, // 毫秒
		}
	})

	families, err := mm.registry.Gather()
	if err != nil {
		t.Fatalf("Gather 失败: %v", err)
	}

	names := make(map[string]bool)
	for _, f := range families {
		names[f.GetName()] = true
	}
	for _, expected := range []string{
		"gateway_wsc_total_clients",
		"gateway_wsc_websocket_clients",
		"gateway_wsc_online_users",
		"gateway_wsc_messages_sent_total",
		"gateway_wsc_messages_received_total",
		"gateway_wsc_uptime_seconds",
	} {
		if !names[expected] {
			t.Errorf("未找到 WebSocket 指标 %s", expected)
		}
	}
}

func TestWSCCollector_NilStats(t *testing.T) {
	mm := NewMetricsManager(newTestMonitoringConfig())
	mm.SetWSCStatsFn(func() *WSCStats { return nil })

	// 不应 panic
	if _, err := mm.registry.Gather(); err != nil {
		t.Fatalf("Gather 失败: %v", err)
	}
}

func TestRecordRateLimitRejected(t *testing.T) {
	mm := NewMetricsManager(newTestMonitoringConfig())

	mm.RecordRateLimitRejected("token_bucket")
	mm.RecordRateLimitRejected("token_bucket")
	mm.RecordRateLimitRejected("leaky_bucket")

	tbVal := testutil.ToFloat64(mm.rateLimitRejected.WithLabelValues("token_bucket"))
	if tbVal != 2 {
		t.Errorf("token_bucket 拒绝数应为 2, got %v", tbVal)
	}
	lbVal := testutil.ToFloat64(mm.rateLimitRejected.WithLabelValues("leaky_bucket"))
	if lbVal != 1 {
		t.Errorf("leaky_bucket 拒绝数应为 1, got %v", lbVal)
	}
}

func TestRecordRateLimitRejected_NilManager(t *testing.T) {
	// nil MetricsManager 不应 panic
	var mm *MetricsManager
	mm.RecordRateLimitRejected("token_bucket")
}

func TestSetters_NilManager(t *testing.T) {
	// nil MetricsManager 的 setter 不应 panic
	var mm *MetricsManager
	mm.SetPoolHealthFn(func() map[string]bool { return nil })
	mm.SetBreakerStatsFn(func() []BreakerStat { return nil })
	mm.SetWSCStatsFn(func() *WSCStats { return nil })
}
