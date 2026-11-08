/**
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-18 13:25:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-18 17:32:18
 * @FilePath: \go-rpc-gateway\middleware\observability\observability_test.go
 * @Description: 可观测性管理器测试 - 验证 registry 暴露、自定义 Collector、限流拒绝记录
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package observability

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gcmiddleware "github.com/kamalyes/go-rpc-gateway/config/middleware"
	"github.com/kamalyes/go-rpc-gateway/middleware/core"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// newTestMonitoringConfig 创建测试用监控配置（已启用 metrics）
func newTestMonitoringConfig() *gcmiddleware.Monitoring {
	return &gcmiddleware.Monitoring{
		Enabled: true,
		Metrics: &gcmiddleware.MonitoringMetrics{
			Enabled: true,
			Buckets: []float64{0.1, 0.5, 1},
		},
	}
}

func TestNewMetricsManager_Disabled(t *testing.T) {
	cfg := &gcmiddleware.Monitoring{
		Enabled: true,
		Metrics: &gcmiddleware.MonitoringMetrics{Enabled: false},
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
	// runtime 指标在所有平台都该有；process 指标只在支持 /proc 的平台上有
	// [EN] Runtime metrics must exist everywhere; process metrics only where /proc works
	expected := []string{"go_goroutines"}
	if processMetricsSupported() {
		expected = append(expected, "process_resident_memory_bytes")
	}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("registry 应包含指标 %s", name)
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
	// process 指标只在支持 /proc 的平台上存在
	// 不支持时 registerProcessCollector 会撤回采集器，断言它存在会在 macOS 上必红，
	// 而那不是缺陷 —— 缺陷是整个端点 500，那一点由本测试的其余断言覆盖
	//
	// [EN] Process metrics exist only where /proc is available
	// Where it is not, registerProcessCollector withdraws the collector, so asserting
	// their presence would always fail on macOS, and that is not the defect; the defect
	// was the endpoint returning 500, which the remaining assertions cover
	if processMetricsSupported() && !strings.Contains(body, "process_resident_memory_bytes") {
		t.Error("本平台支持 process 指标，输出里却没有 process_resident_memory_bytes")
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
}

// processMetricsSupported 探测本平台能否采集 process 指标
// 单独建一个 registry 试，不污染被测对象
//
// [EN] processMetricsSupported probes whether process metrics work on this platform,
// using a throwaway registry so the subject under test is untouched
func processMetricsSupported() bool {
	r := prometheus.NewRegistry()
	r.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{ReportErrors: true}))
	_, err := r.Gather()
	return err == nil
}

// recordingWriter 底层 writer，记录 Flush / Hijack / Push 是否真的被调到
type recordingWriter struct {
	http.ResponseWriter
	flushed  int
	hijacked int
	pushed   int
}

func (w *recordingWriter) Flush() { w.flushed++ }

func (w *recordingWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijacked++
	return nil, nil, nil
}

func (w *recordingWriter) Push(string, *http.PushOptions) error {
	w.pushed++
	return nil
}

// TestMetricsResponseWriterImplementsOptionalInterfaces 包装器自身必须实现这三个可选接口
//
// 不实现的后果不是「慢一点」：grpc-gateway 的 ForwardResponseStream 第一步就是
// w.(http.Flusher) 断言，失败时直接回 500；WebSocket 升级则拿不到裸连接
func TestMetricsResponseWriterImplementsOptionalInterfaces(t *testing.T) {
	rw := acquireMetricsResponseWriter(httptest.NewRecorder())
	defer releaseMetricsResponseWriter(rw)

	var w http.ResponseWriter = rw
	if _, ok := w.(http.Flusher); !ok {
		t.Error("metricsResponseWriter 未实现 http.Flusher：流式响应与 server-streaming 会失效")
	}
	if _, ok := w.(http.Hijacker); !ok {
		t.Error("metricsResponseWriter 未实现 http.Hijacker：WebSocket 升级会失败")
	}
	if _, ok := w.(http.Pusher); !ok {
		t.Error("metricsResponseWriter 未实现 http.Pusher")
	}
	if u, ok := w.(interface{ Unwrap() http.ResponseWriter }); !ok {
		t.Error("metricsResponseWriter 未实现 Unwrap：http.ResponseController 取不到底层 writer")
	} else if u.Unwrap() == nil {
		t.Error("Unwrap 返回了 nil")
	}
}

// TestMetricsResponseWriterForwardsThroughChain 真正要锁住的是「穿透整条链」
//
// 中间件链上 core.ResponseWriter 的 Flush 是逐层往内委托的（见 middleware/core/response_writer.go）
// 只要中间夹一层不实现 http.Flusher 的包装器，委托就在那里
// 断掉而且毫无报错 —— 所以这里按链上真实顺序叠起来验证调用到底
//
// 链上真实顺序见 middleware/manager.go GetMiddlewares：
// Logging(core) → Metrics(本层) → Shedder/Breaker(core)
func TestMetricsResponseWriterForwardsThroughChain(t *testing.T) {
	base := &recordingWriter{ResponseWriter: httptest.NewRecorder()}

	// Logging 层
	outer := core.NewResponseWriter(base)
	defer outer.Release()

	// 指标层（被测对象，夹在中间）
	mid := acquireMetricsResponseWriter(outer)
	defer releaseMetricsResponseWriter(mid)

	// Shedder / Breaker 层，也就是 grpc-gateway mux 实际拿到的那个 writer
	inner := core.NewResponseWriter(mid)
	defer inner.Release()

	var w http.ResponseWriter = inner

	flusher, ok := w.(http.Flusher)
	if !ok {
		t.Fatal("最内层 writer 不是 http.Flusher")
	}
	flusher.Flush()
	if base.flushed != 1 {
		t.Errorf("Flush 未穿透到底层：底层收到 %d 次，期望 1 次（委托链在指标层断掉）", base.flushed)
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		t.Fatal("最内层 writer 不是 http.Hijacker")
	}
	if _, _, err := hijacker.Hijack(); err != nil {
		t.Errorf("Hijack 未穿透到底层，返回 %v（WebSocket 升级会失败）", err)
	}
	if base.hijacked != 1 {
		t.Errorf("Hijack 未穿透到底层：底层收到 %d 次，期望 1 次", base.hijacked)
	}

	pusher, ok := w.(http.Pusher)
	if !ok {
		t.Fatal("最内层 writer 不是 http.Pusher")
	}
	if err := pusher.Push("/x", nil); err != nil {
		t.Errorf("Push 未穿透到底层，返回 %v", err)
	}
	if base.pushed != 1 {
		t.Errorf("Push 未穿透到底层：底层收到 %d 次，期望 1 次", base.pushed)
	}
}
