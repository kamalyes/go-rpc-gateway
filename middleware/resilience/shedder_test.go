/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-09-26 21:33:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-09-29 23:10:27
 * @FilePath: \go-rpc-gateway\middleware\resilience\shedder_test.go
 * @Description: 自适应降载核心、滑窗、管理器与 HTTP/gRPC 适配层测试
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package resilience

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	shedderconfig "github.com/kamalyes/go-config/pkg/shedder"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ============================================================================
// 测试辅助
//
// 所有判定都通过 allowAt/allowStreamAt/passAt 这几个带显式时间的入口驱动，
// 测试里不出现 time.Sleep，也不依赖真实 CPU 负载——否则在 CI 的共享机器上必然偶发失败
// ============================================================================

// fakeCPU 可控的 CPU 使用率来源
type fakeCPU struct {
	usage     atomic.Int64
	supported bool
}

func (f *fakeCPU) read() (int64, bool) {
	return f.usage.Load(), f.supported
}

// newTestShedderConfig 测试用降载配置：窗口 1s / 10 桶（单桶 100ms），冷却 1s
func newTestShedderConfig() *shedderconfig.Shedder {
	return &shedderconfig.Shedder{
		Enabled:       true,
		CPUThreshold:  900,
		WindowMS:      1000,
		WindowBuckets: 10,
		CooldownMS:    1000,
		MinRTMS:       1000,
		FlyingBeta:    0.9,
	}
}

// newTestShedder 创建测试用降载器，返回降载器与可控 CPU 源
func newTestShedder(t *testing.T, cpuPermille int64, cpuSupported bool, maxConcurrency int64) (*Shedder, *fakeCPU) {
	t.Helper()
	cpu := &fakeCPU{supported: cpuSupported}
	cpu.usage.Store(cpuPermille)
	cfg := newTestShedderConfig()
	cfg.MaxConcurrency = maxConcurrency
	return NewShedderWithCPUFunc(cfg, cpu.read), cpu
}

// sumWindowPass 统计滑窗所有桶的通过量之和（与时间片无关，避免测试依赖时序）
func sumWindowPass(w *shedderWindow) int64 {
	var total int64
	for i := range w.buckets {
		if w.buckets[i].epoch.Load() >= 0 {
			total += w.buckets[i].pass.Load()
		}
	}
	return total
}

// ============================================================================
// 响应时间滑窗
// ============================================================================

func TestShedderWindow_MaxPassAndMinRT(t *testing.T) {
	base := time.Now()
	w := newShedderWindow(time.Second, 10, base) // 单桶 100ms

	// 第 0 个时间片：2 次通过，平均 RT 20ms
	w.record(base, 10*time.Millisecond)
	w.record(base.Add(10*time.Millisecond), 30*time.Millisecond)
	// 第 1 个时间片：1 次通过，平均 RT 5ms
	w.record(base.Add(150*time.Millisecond), 5*time.Millisecond)

	st := w.stat(base.Add(250 * time.Millisecond))
	if st.MaxPass != 2 {
		t.Errorf("单桶最大通过量应为 2, got %d", st.MaxPass)
	}
	if st.MinRT != 5000 {
		t.Errorf("最小平均 RT 应为 5000 微秒, got %d", st.MinRT)
	}
}

func TestShedderWindow_SkipsCurrentBucket(t *testing.T) {
	base := time.Now()
	w := newShedderWindow(time.Second, 10, base)

	w.record(base, 10*time.Millisecond)

	// 还在第 0 个时间片内：当前桶未写满，不应参与聚合
	st := w.stat(base.Add(50 * time.Millisecond))
	if st.MaxPass != 0 || st.MinRT != 0 {
		t.Errorf("当前未写满的桶不应参与聚合, got MaxPass=%d MinRT=%d", st.MaxPass, st.MinRT)
	}
}

func TestShedderWindow_StaleBucketExpires(t *testing.T) {
	base := time.Now()
	w := newShedderWindow(time.Second, 10, base)

	w.record(base, 10*time.Millisecond)

	// 整窗滚过之后，第 0 个时间片已经出窗
	st := w.stat(base.Add(1150 * time.Millisecond))
	if st.MaxPass != 0 {
		t.Errorf("滚出窗口的桶不应参与聚合, got MaxPass=%d", st.MaxPass)
	}
}

func TestShedderWindow_BucketReuseResetsCounters(t *testing.T) {
	base := time.Now()
	w := newShedderWindow(time.Second, 10, base)

	// 第 0 个时间片写 3 次，RT 100ms
	for i := 0; i < 3; i++ {
		w.record(base, 100*time.Millisecond)
	}
	// 第 10 个时间片复用同一个桶（10 % 10 == 0），只写 1 次，RT 2ms
	w.record(base.Add(1010*time.Millisecond), 2*time.Millisecond)

	st := w.stat(base.Add(1150 * time.Millisecond))
	if st.MaxPass != 1 {
		t.Errorf("桶复用时应清零旧计数, got MaxPass=%d", st.MaxPass)
	}
	if st.MinRT != 2000 {
		t.Errorf("桶复用时应清零旧 RT 累加, got MinRT=%d", st.MinRT)
	}
}

func TestShedderWindow_FixedMemory(t *testing.T) {
	base := time.Now()
	w := newShedderWindow(time.Second, 10, base)

	// 跨越 1000 个时间片、写 1 万次，桶数必须恒定
	for i := 0; i < 10000; i++ {
		w.record(base.Add(time.Duration(i)*time.Millisecond), time.Millisecond)
	}

	if len(w.buckets) != 10 || cap(w.buckets) != 10 {
		t.Errorf("滑窗内存必须固定为 10 个桶, got len=%d cap=%d", len(w.buckets), cap(w.buckets))
	}
}

func TestShedderWindow_BucketsPerSecond(t *testing.T) {
	w := newShedderWindow(5*time.Second, 50, time.Now())
	if got := w.bucketsPerSecond(); got != 10 {
		t.Errorf("5s/50 桶应为每秒 10 个桶, got %v", got)
	}
}

// ============================================================================
// 降载核心判定
// ============================================================================

func TestShedder_NoDropWithoutCPUSignal(t *testing.T) {
	// CPU 采样不可用（darwin/windows 的降级路径）：自适应判定必须整体静默
	s, _ := newTestShedder(t, 1000, false, 0)
	s.storeAvgFlying(1000)
	base := s.start

	for i := 0; i < 5; i++ {
		if !s.allowAt(base.Add(10 * time.Millisecond)) {
			t.Fatalf("CPU 信号不可用时不应丢弃请求, 第 %d 次被丢弃", i+1)
		}
	}
	if snap := s.Snapshot(); snap.DroppedRequests != 0 {
		t.Errorf("CPU 信号不可用时丢弃数应为 0, got %d", snap.DroppedRequests)
	}
}

func TestShedder_NoDropWhenCPUBelowThreshold(t *testing.T) {
	s, _ := newTestShedder(t, 500, true, 0)
	s.storeAvgFlying(1000)
	base := s.start

	for i := 0; i < 5; i++ {
		if !s.allowAt(base.Add(10 * time.Millisecond)) {
			t.Fatalf("CPU 未超阈值时不应丢弃请求, 第 %d 次被丢弃", i+1)
		}
	}
}

func TestShedder_DropWhenCPUOverThresholdAndOverCapacity(t *testing.T) {
	s, _ := newTestShedder(t, 950, true, 0)
	base := s.start
	// 滑窗为空，承载上限取下界 1；把平滑飞行数抬到上限之上
	s.storeAvgFlying(10)

	// 第 1 个请求：飞行数 1，未超上限 1，放行
	if !s.allowAt(base.Add(10 * time.Millisecond)) {
		t.Fatal("飞行数未超承载上限时应放行")
	}
	// 第 2 个请求：飞行数 2 > 1，且 CPU 超阈值，丢弃
	if s.allowAt(base.Add(10 * time.Millisecond)) {
		t.Fatal("CPU 超阈值且飞行数超承载上限时应丢弃")
	}

	snap := s.Snapshot()
	if snap.DroppedRequests != 1 {
		t.Errorf("丢弃数应为 1, got %d", snap.DroppedRequests)
	}
	if snap.TotalRequests != 2 {
		t.Errorf("判定次数应为 2, got %d", snap.TotalRequests)
	}
	if !snap.Shedding {
		t.Error("丢弃后应进入降载冷却期")
	}
	// 被丢弃的请求必须归还飞行计数，否则计数会越积越高
	if s.InFlight() != 1 {
		t.Errorf("丢弃请求后飞行数应回到 1, got %d", s.InFlight())
	}
}

func TestShedder_CapacityFromWindowPreventsFalseDrop(t *testing.T) {
	s, _ := newTestShedder(t, 950, true, 0)
	base := s.start

	// 直接灌滑窗：3 个时间片各 20 次通过、RT 50ms
	// 承载上限 = 20 × 10 桶/秒 × 0.05s = 10
	for epoch := 0; epoch < 3; epoch++ {
		at := base.Add(time.Duration(epoch) * 100 * time.Millisecond)
		for i := 0; i < 20; i++ {
			s.window.record(at, 50*time.Millisecond)
		}
	}
	now := base.Add(350 * time.Millisecond)
	if got := s.maxFlight(now); got != 10 {
		t.Fatalf("承载上限应为 10, got %v", got)
	}

	// 平滑飞行数 5 < 10：即使 CPU 超阈值、飞行数一路涨到 10 也不该丢
	// 这正是只看 CPU 会误杀的场景——系统吃紧但确实在以它的最佳吞吐干活
	s.storeAvgFlying(5)
	for i := 0; i < 10; i++ {
		if !s.allowAt(now) {
			t.Fatalf("飞行数未超承载上限时不应丢弃, 第 %d 次被丢弃", i+1)
		}
	}
	if s.InFlight() != 10 {
		t.Fatalf("应有 10 个请求在飞, got %d", s.InFlight())
	}

	// 平滑飞行数抬到 20 > 10，第 11 个请求的瞬时飞行数也超过 10
	s.storeAvgFlying(20)
	if s.allowAt(now) {
		t.Fatal("飞行数与平滑值同时超过承载上限时应丢弃")
	}
}

func TestShedder_RequiresBothAvgAndInstantOverCapacity(t *testing.T) {
	s, _ := newTestShedder(t, 950, true, 0)
	base := s.start

	// 平滑值很高但瞬时飞行数只有 1（未超上限 1）：不丢
	s.storeAvgFlying(100)
	if !s.allowAt(base) {
		t.Fatal("瞬时飞行数未超上限时不应丢弃")
	}
	// 瞬时飞行数升到 2 但平滑值压回 0：同样不丢
	s.storeAvgFlying(0)
	if !s.allowAt(base) {
		t.Fatal("平滑飞行数未超上限时不应丢弃")
	}
}

func TestShedder_MaxConcurrencyHardCap(t *testing.T) {
	// CPU 不可采样，只靠硬上限保护（darwin/windows 上唯一生效的那条）
	s, _ := newTestShedder(t, 0, false, 2)
	base := s.start

	if !s.allowAt(base) || !s.allowAt(base) {
		t.Fatal("飞行数未达硬上限时应放行")
	}
	if s.allowAt(base) {
		t.Fatal("飞行数达到硬上限时应丢弃")
	}
	if s.InFlight() != 2 {
		t.Errorf("硬上限丢弃后飞行数应保持 2, got %d", s.InFlight())
	}

	// 一个请求完成后应重新放行
	s.passAt(base, 10*time.Millisecond)
	if !s.allowAt(base) {
		t.Fatal("有请求完成后应重新放行")
	}
}

func TestShedder_HardCapDoesNotEnterCooldown(t *testing.T) {
	s, _ := newTestShedder(t, 0, false, 1)
	base := s.start

	if !s.allowAt(base) {
		t.Fatal("首个请求应放行")
	}
	if s.allowAt(base) {
		t.Fatal("达到硬上限应丢弃")
	}
	// 硬上限是自限的，不该把自适应判定的冷却状态拖进来
	if s.stillHot(base) {
		t.Error("硬上限丢弃不应进入降载冷却期")
	}
}

func TestShedder_CooldownKeepsSheddingAfterCPURecovers(t *testing.T) {
	s, cpu := newTestShedder(t, 950, true, 0)
	base := s.start
	s.storeAvgFlying(10)

	// 在第 500ms 触发一次丢弃
	dropAt := base.Add(500 * time.Millisecond)
	if !s.allowAt(dropAt) {
		t.Fatal("首个请求应放行")
	}
	if s.allowAt(dropAt) {
		t.Fatal("CPU 超阈值且超承载上限时应丢弃")
	}

	// CPU 立刻回落：冷却期内仍应继续丢，避免「丢→回落→全放→再堆积」的抖动
	cpu.usage.Store(100)
	if s.allowAt(base.Add(900 * time.Millisecond)) {
		t.Error("冷却期内 CPU 回落也应继续降载")
	}

	// 冷却期从「最近一次丢弃」起算，上面那次丢弃把计时重置到了第 900ms，
	// 因此恢复时间是 900ms + 1s 冷却；取第 2000ms 验证恢复
	if !s.allowAt(base.Add(2000 * time.Millisecond)) {
		t.Error("冷却期结束且 CPU 已回落时应恢复放行")
	}
	if s.stillHot(base.Add(2000 * time.Millisecond)) {
		t.Error("冷却期结束后不应仍处于降载状态")
	}
}

func TestShedder_CooldownStillRequiresOverCapacity(t *testing.T) {
	s, cpu := newTestShedder(t, 950, true, 0)
	base := s.start
	s.storeAvgFlying(10)

	if !s.allowAt(base) {
		t.Fatal("首个请求应放行")
	}
	if s.allowAt(base) {
		t.Fatal("应触发一次丢弃以进入冷却期")
	}

	// 冷却期内但负载已经退潮（平滑飞行数压到 0）：必须立刻恢复放行，
	// 冷却不等于「无脑拒绝一整秒」
	cpu.usage.Store(100)
	s.storeAvgFlying(0)
	if !s.allowAt(base.Add(100 * time.Millisecond)) {
		t.Error("冷却期内负载退潮后应立刻恢复放行")
	}
}

func TestShedder_PassRecordsWindowFailDoesNot(t *testing.T) {
	s, _ := newTestShedder(t, 0, false, 0)
	base := s.start

	if !s.allowAt(base) {
		t.Fatal("请求应放行")
	}
	s.passAt(base, 20*time.Millisecond)
	if got := sumWindowPass(s.window); got != 1 {
		t.Errorf("Pass 应把样本计入滑窗, got %d", got)
	}
	if s.InFlight() != 0 {
		t.Errorf("Pass 后飞行数应归零, got %d", s.InFlight())
	}

	if !s.allowAt(base) {
		t.Fatal("请求应放行")
	}
	s.Fail()
	if got := sumWindowPass(s.window); got != 1 {
		t.Errorf("Fail 不应把样本计入滑窗, got %d", got)
	}
	if s.InFlight() != 0 {
		t.Errorf("Fail 后飞行数应归零, got %d", s.InFlight())
	}
}

func TestShedder_SettleNeverLeavesNegativeInFlight(t *testing.T) {
	s, _ := newTestShedder(t, 0, false, 0)
	// 记账出错（Fail 多于 Allow）时必须纠回 0：负的飞行数会让降载永久失效
	s.Fail()
	if got := s.InFlight(); got != 0 {
		t.Errorf("飞行数不应为负, got %d", got)
	}
}

func TestShedder_Snapshot(t *testing.T) {
	s, _ := newTestShedder(t, 880, true, 64)
	base := s.start

	if !s.allowAt(base) {
		t.Fatal("请求应放行")
	}
	s.passAt(base, 30*time.Millisecond)

	snap := s.Snapshot()
	if snap.CPUUsage != 880 || !snap.CPUSupported {
		t.Errorf("快照应反映注入的 CPU 源, got usage=%d supported=%v", snap.CPUUsage, snap.CPUSupported)
	}
	if snap.CPUThreshold != 900 {
		t.Errorf("快照 CPU 阈值应为 900, got %d", snap.CPUThreshold)
	}
	if snap.MaxConcurrency != 64 {
		t.Errorf("快照硬上限应为 64, got %d", snap.MaxConcurrency)
	}
	if snap.TotalRequests != 1 || snap.DroppedRequests != 0 {
		t.Errorf("快照计数不符, got total=%d dropped=%d", snap.TotalRequests, snap.DroppedRequests)
	}
	if snap.InFlight != 0 {
		t.Errorf("快照飞行数应为 0, got %d", snap.InFlight)
	}
	if snap.MaxFlight < 1 {
		t.Errorf("承载上限下界应为 1, got %v", snap.MaxFlight)
	}
}

func TestShedder_Reset(t *testing.T) {
	s, _ := newTestShedder(t, 950, true, 0)
	base := s.start
	s.storeAvgFlying(10)
	_ = s.allowAt(base)
	_ = s.allowAt(base)

	s.Reset()

	snap := s.Snapshot()
	if snap.TotalRequests != 0 || snap.DroppedRequests != 0 || snap.InFlight != 0 {
		t.Errorf("Reset 后计数应全部归零, got %+v", snap)
	}
	if snap.Shedding {
		t.Error("Reset 后不应处于降载状态")
	}
	if snap.AvgFlying != 0 {
		t.Errorf("Reset 后平滑飞行数应归零, got %v", snap.AvgFlying)
	}
	if got := sumWindowPass(s.window); got != 0 {
		t.Errorf("Reset 后滑窗应清空, got %d", got)
	}
}

func TestShedder_ConcurrentAllowSettleNoLeak(t *testing.T) {
	// 并发正确性测试：只断言「账一定平」，不断言任何与时间/调度相关的结果，
	// 因此在 -race 与任意 CPU 数下都是稳定的
	s, _ := newTestShedder(t, 0, false, 0)

	const workers, iterations = 64, 100
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(w int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if !s.Allow() {
					continue
				}
				if (w+i)%2 == 0 {
					s.Pass(time.Millisecond)
				} else {
					s.Fail()
				}
			}
		}(w)
	}
	wg.Wait()

	if got := s.InFlight(); got != 0 {
		t.Errorf("所有请求结束后飞行数应归零, got %d", got)
	}
	snap := s.Snapshot()
	if snap.TotalRequests != workers*iterations {
		t.Errorf("判定次数应为 %d, got %d", workers*iterations, snap.TotalRequests)
	}
	if snap.DroppedRequests != 0 {
		t.Errorf("CPU 信号不可用且无硬上限时不应丢弃, got %d", snap.DroppedRequests)
	}
}

// ============================================================================
// ShedderManager
// ============================================================================

func TestShedderManager_DefaultExcludePaths(t *testing.T) {
	m := NewShedderManager(&shedderconfig.Shedder{Enabled: true, ExcludePaths: shedderconfig.Default().ExcludePaths})

	protected := []string{"/api/v1/users", "/", "/v1/messages/send"}
	for _, p := range protected {
		if !m.IsPathProtected(p) {
			t.Errorf("默认应保护全部业务路径, %s 未被保护", p)
		}
	}

	// 健康检查、指标、pprof 必须永远放行
	excluded := []string{"/health", "/healthz", "/readyz", "/metrics", "/debug/pprof/heap"}
	for _, p := range excluded {
		if m.IsPathProtected(p) {
			t.Errorf("%s 应被排除在降载之外", p)
		}
	}
}

func TestShedderManager_ExcludeIsPrefixMatch(t *testing.T) {
	m := NewShedderManager(&shedderconfig.Shedder{Enabled: true, ExcludePaths: []string{"/internal/"}})

	if m.IsPathProtected("/internal/debug/vars") {
		t.Error("排除规则应按前缀匹配，子路径也要排除")
	}
	if !m.IsPathProtected("/internal2/x") {
		t.Error("前缀不匹配的路径仍应受保护")
	}
}

func TestShedderManager_NilSafe(t *testing.T) {
	var m *ShedderManager
	if m.IsPathProtected("/api") {
		t.Error("nil 管理器不应保护任何路径")
	}
	if m.Shedder() != nil {
		t.Error("nil 管理器的降载器应为 nil")
	}
	if m.Config() != nil {
		t.Error("nil 管理器的配置应为 nil")
	}
	if snap := m.Snapshot(); snap.TotalRequests != 0 {
		t.Error("nil 管理器的快照应为零值")
	}
	m.Reset() // 不应 panic
}

func TestShedderManager_NormalizesConfig(t *testing.T) {
	// 非法配置必须回落默认而不是让网关起不来
	def := shedderconfig.Default()
	m := NewShedderManager(&shedderconfig.Shedder{Enabled: true, CPUThreshold: 5000, WindowBuckets: -1, FlyingBeta: 2})
	cfg := m.Config()
	if cfg.CPUThreshold != def.CPUThreshold {
		t.Errorf("越界的 CPU 阈值应回落默认, got %d", cfg.CPUThreshold)
	}
	if cfg.WindowBuckets != def.WindowBuckets {
		t.Errorf("非法桶数应回落默认, got %d", cfg.WindowBuckets)
	}
	if cfg.FlyingBeta != def.FlyingBeta {
		t.Errorf("越界的 beta 应回落默认, got %v", cfg.FlyingBeta)
	}
}

// ============================================================================
// HTTP 中间件
// ============================================================================

// newTestShedderManager 创建测试用降载管理器，并把它的降载器换成可控 CPU 源的实例
func newTestShedderManager(t *testing.T, cpuPermille int64, cpuSupported bool, maxConcurrency int64) (*ShedderManager, *Shedder) {
	t.Helper()
	cfg := newTestShedderConfig()
	cfg.MaxConcurrency = maxConcurrency
	cfg.ExcludePaths = []string{"/health", "/metrics"}
	m := NewShedderManager(cfg)

	shedder, _ := newTestShedder(t, cpuPermille, cpuSupported, maxConcurrency)
	m.shedder = shedder
	return m, shedder
}

func TestShedderHTTPMiddleware_PathExcluded(t *testing.T) {
	m, s := newTestShedderManager(t, 0, false, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	ShedderHTTPMiddleware(m)(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("被排除路径应直通返回 200, got %d", rr.Code)
	}
	if snap := s.Snapshot(); snap.TotalRequests != 0 {
		t.Errorf("被排除路径不应进入降载判定, got %d", snap.TotalRequests)
	}
}

func TestShedderHTTPMiddleware_NilManager(t *testing.T) {
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	ShedderHTTPMiddleware(nil)(handler).ServeHTTP(rr, req)

	if !called || rr.Code != http.StatusOK {
		t.Errorf("manager 为 nil 时应直通, called=%v code=%d", called, rr.Code)
	}
}

func TestShedderHTTPMiddleware_SuccessRecorded(t *testing.T) {
	m, s := newTestShedderManager(t, 0, false, 0)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	ShedderHTTPMiddleware(m)(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("正常请求应返回 200, got %d", rr.Code)
	}
	snap := s.Snapshot()
	if snap.TotalRequests != 1 || snap.DroppedRequests != 0 {
		t.Errorf("计数不符, got total=%d dropped=%d", snap.TotalRequests, snap.DroppedRequests)
	}
	if snap.InFlight != 0 {
		t.Errorf("请求结束后飞行数应归零, got %d", snap.InFlight)
	}
	if got := sumWindowPass(s.window); got != 1 {
		t.Errorf("成功请求应计入滑窗, got %d", got)
	}
}

func TestShedderHTTPMiddleware_ServerErrorCountsAsFail(t *testing.T) {
	m, s := newTestShedderManager(t, 0, false, 0)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	ShedderHTTPMiddleware(m)(handler).ServeHTTP(rr, req)

	if s.InFlight() != 0 {
		t.Errorf("请求结束后飞行数应归零, got %d", s.InFlight())
	}
	// 5xx 不进滑窗：否则会把系统达不到的承载上限算进来
	if got := sumWindowPass(s.window); got != 0 {
		t.Errorf("5xx 不应计入滑窗, got %d", got)
	}
}

func TestShedderHTTPMiddleware_RejectWith503(t *testing.T) {
	// 硬上限设为 1 并预先占用一个飞行位，使下一个请求必被降载
	m, s := newTestShedderManager(t, 0, false, 1)
	if !s.allowAt(s.start) {
		t.Fatal("预占飞行位失败")
	}

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	ShedderHTTPMiddleware(m)(handler).ServeHTTP(rr, req)

	if called {
		t.Error("被降载的请求不应触达 handler")
	}
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("被降载应返回 503, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("降载响应 Content-Type 应为 application/json, got %q", ct)
	}
	if ra := rr.Header().Get("Retry-After"); ra != shedderRetryAfter {
		t.Errorf("降载响应应带 Retry-After=%s, got %q", shedderRetryAfter, ra)
	}
	if !bytes.Equal(rr.Body.Bytes(), shedderDropResp) {
		t.Errorf("降载响应体应为预分配的 JSON, got %q", rr.Body.String())
	}
}

func TestShedderHTTPMiddleware_PanicReleasesInFlight(t *testing.T) {
	m, s := newTestShedderManager(t, 0, false, 0)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	func() {
		// 真实链路里由最外层的 RecoveryMiddleware 兜住，这里自己接一下
		defer func() { _ = recover() }()
		ShedderHTTPMiddleware(m)(handler).ServeHTTP(rr, req)
	}()

	// handler panic 时若不归还飞行计数，几次 panic 就能让降载器把全部流量当成过载
	if s.InFlight() != 0 {
		t.Errorf("handler panic 后飞行数应归零, got %d", s.InFlight())
	}
	if got := sumWindowPass(s.window); got != 0 {
		t.Errorf("panic 的请求不应计入滑窗, got %d", got)
	}
}

// ============================================================================
// gRPC 一元拦截器
// ============================================================================

func TestUnaryServerShedderInterceptor_NilManager(t *testing.T) {
	interceptor := UnaryServerShedderInterceptor(nil)

	called := false
	if _, err := interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod), grpcCodeHandler(codes.OK, &called)); err != nil {
		t.Errorf("manager 为 nil 时不应返回错误, got %v", err)
	}
	if !called {
		t.Error("manager 为 nil 时应直通到 handler")
	}
}

func TestUnaryServerShedderInterceptor_MethodExcluded(t *testing.T) {
	cfg := newTestShedderConfig()
	cfg.ExcludePaths = []string{"/user.v1.UserService/"}
	m := NewShedderManager(cfg)
	interceptor := UnaryServerShedderInterceptor(m)

	called := false
	if _, err := interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod), grpcCodeHandler(codes.OK, &called)); err != nil {
		t.Errorf("被排除方法不应返回错误, got %v", err)
	}
	if !called {
		t.Error("被排除方法应直通到 handler")
	}
	if snap := m.Snapshot(); snap.TotalRequests != 0 {
		t.Errorf("被排除方法不应进入降载判定, got %d", snap.TotalRequests)
	}
}

func TestUnaryServerShedderInterceptor_SuccessRecorded(t *testing.T) {
	m, s := newTestShedderManager(t, 0, false, 0)
	interceptor := UnaryServerShedderInterceptor(m)

	called := false
	resp, err := interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod), grpcCodeHandler(codes.OK, &called))
	if err != nil {
		t.Fatalf("成功调用不应返回错误, got %v", err)
	}
	if !called || resp != "ok" {
		t.Errorf("应直通到 handler 并返回其响应, called=%v resp=%v", called, resp)
	}
	if s.InFlight() != 0 {
		t.Errorf("调用结束后飞行数应归零, got %d", s.InFlight())
	}
	if got := sumWindowPass(s.window); got != 1 {
		t.Errorf("成功调用应计入滑窗, got %d", got)
	}
}

func TestUnaryServerShedderInterceptor_FailureCodesNotRecorded(t *testing.T) {
	// 与熔断共用 IsBreakerFailureCode：下游故障码只归还飞行计数，不污染容量估计
	for _, code := range []codes.Code{codes.Internal, codes.DeadlineExceeded, codes.Unavailable, codes.Unknown, codes.DataLoss} {
		m, s := newTestShedderManager(t, 0, false, 0)
		interceptor := UnaryServerShedderInterceptor(m)

		called := false
		_, _ = interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod), grpcCodeHandler(code, &called))

		if s.InFlight() != 0 {
			t.Errorf("%s: 调用结束后飞行数应归零, got %d", code, s.InFlight())
		}
		if got := sumWindowPass(s.window); got != 0 {
			t.Errorf("%s: 下游故障码不应计入滑窗, got %d", code, got)
		}
	}
}

func TestUnaryServerShedderInterceptor_ClientErrorCodesRecorded(t *testing.T) {
	// 客户端侧错误是「服务端正常工作并给出了答案」，应作为有效样本计入容量估计
	for _, code := range []codes.Code{codes.InvalidArgument, codes.NotFound, codes.PermissionDenied, codes.ResourceExhausted} {
		m, s := newTestShedderManager(t, 0, false, 0)
		interceptor := UnaryServerShedderInterceptor(m)

		called := false
		_, _ = interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod), grpcCodeHandler(code, &called))

		if got := sumWindowPass(s.window); got != 1 {
			t.Errorf("%s: 客户端侧错误应计入滑窗, got %d", code, got)
		}
	}
}

func TestUnaryServerShedderInterceptor_RejectWithUnavailable(t *testing.T) {
	m, s := newTestShedderManager(t, 0, false, 1)
	if !s.allowAt(s.start) {
		t.Fatal("预占飞行位失败")
	}
	interceptor := UnaryServerShedderInterceptor(m)

	called := false
	resp, err := interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod), grpcCodeHandler(codes.OK, &called))

	if called {
		t.Error("被降载的调用不应触达 handler")
	}
	if resp != nil {
		t.Errorf("被降载时响应应为 nil, got %v", resp)
	}
	if got := status.Code(err); got != codes.Unavailable {
		t.Errorf("降载拒绝应返回 codes.Unavailable, got %s", got)
	}
	// 与 HTTP 侧 503 对应同一个错误码
	want := errors.NewError(errors.ErrCodeServiceOverloaded, "").ToGRPCError()
	if status.Convert(err).Message() != status.Convert(want).Message() {
		t.Errorf("降载拒绝消息应取自 ErrCodeServiceOverloaded, got %q want %q",
			status.Convert(err).Message(), status.Convert(want).Message())
	}
	if httpStatus := errors.NewError(errors.ErrCodeServiceOverloaded, "").GetHTTPStatus(); httpStatus != http.StatusServiceUnavailable {
		t.Errorf("ErrCodeServiceOverloaded 应映射到 HTTP 503, got %d", httpStatus)
	}
}

func TestUnaryServerShedderInterceptor_PanicReleasesInFlight(t *testing.T) {
	m, s := newTestShedderManager(t, 0, false, 0)
	interceptor := UnaryServerShedderInterceptor(m)

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		panic("boom")
	}
	func() {
		defer func() { _ = recover() }()
		_, _ = interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod), handler)
	}()

	if s.InFlight() != 0 {
		t.Errorf("handler panic 后飞行数应归零, got %d", s.InFlight())
	}
	if got := sumWindowPass(s.window); got != 0 {
		t.Errorf("panic 的调用不应计入滑窗, got %d", got)
	}
}

// ============================================================================
// gRPC 流式拦截器
// ============================================================================

func TestStreamServerShedderInterceptor_NilManager(t *testing.T) {
	interceptor := StreamServerShedderInterceptor(nil)

	called := false
	if err := interceptor(nil, nil, newGRPCStreamInfo(testGRPCMethod), grpcStreamCodeHandler(codes.OK, &called)); err != nil {
		t.Errorf("manager 为 nil 时不应返回错误, got %v", err)
	}
	if !called {
		t.Error("manager 为 nil 时应直通到 handler")
	}
}

func TestStreamServerShedderInterceptor_StreamLifecycleNotAccounted(t *testing.T) {
	m, s := newTestShedderManager(t, 0, false, 0)
	interceptor := StreamServerShedderInterceptor(m)

	// 长连接流若占用飞行计数，会把 maxFlight 估计彻底打乱，因此流内不应看到飞行计数
	var inFlightInside int64
	called := false
	err := interceptor(nil, nil, newGRPCStreamInfo(testGRPCMethod), func(srv interface{}, ss grpc.ServerStream) error {
		called = true
		inFlightInside = s.InFlight()
		return nil
	})

	if err != nil {
		t.Fatalf("正常建流不应返回错误, got %v", err)
	}
	if !called {
		t.Fatal("应直通到 stream handler")
	}
	if inFlightInside != 0 {
		t.Errorf("流不应占用飞行计数, 流内 got %d", inFlightInside)
	}
	if s.InFlight() != 0 {
		t.Errorf("流结束后飞行数应为 0, got %d", s.InFlight())
	}
	if got := sumWindowPass(s.window); got != 0 {
		t.Errorf("流不应产生 RT 样本, got %d", got)
	}
	// 判定本身仍然计数，便于观测
	if snap := s.Snapshot(); snap.TotalRequests != 1 {
		t.Errorf("建流应计一次判定, got %d", snap.TotalRequests)
	}
}

func TestStreamServerShedderInterceptor_RejectWhenOverloaded(t *testing.T) {
	m, s := newTestShedderManager(t, 100, true, 0)
	base := s.start

	// 先在 CPU 正常时占住 2 个飞行位（模拟同时在跑的一元请求）
	if !s.allowAt(base) || !s.allowAt(base) {
		t.Fatal("预占飞行位失败")
	}
	// 再让 CPU 超阈值、平滑飞行数超过承载上限 1
	s.cpuUsageFn = func() (int64, bool) { return 950, true }
	s.storeAvgFlying(10)

	interceptor := StreamServerShedderInterceptor(m)
	called := false
	err := interceptor(nil, nil, newGRPCStreamInfo(testGRPCMethod), grpcStreamCodeHandler(codes.OK, &called))

	if called {
		t.Error("过载时不应建立新流")
	}
	if got := status.Code(err); got != codes.Unavailable {
		t.Errorf("建流被降载应返回 codes.Unavailable, got %s", got)
	}
	// 建流判定不占用飞行计数，被拒绝也不该改变它
	if s.InFlight() != 2 {
		t.Errorf("建流被拒绝不应改变飞行数, got %d", s.InFlight())
	}
}

// ============================================================================
// CPU 采样
// ============================================================================

func TestShedderCPUUsage_DegradesGracefully(t *testing.T) {
	// 不同平台结果不同（linux 读 cgroup、darwin/BSD 读 rusage、其余不支持），
	// 这里只断言「不 panic、取值合法、来源标识可读」这三件事
	usage, supported := CPUUsage()
	if supported {
		if usage < 0 || usage > 1000 {
			t.Errorf("CPU 使用率应在 0-1000 千分比区间, got %d", usage)
		}
	} else if usage != 0 {
		t.Errorf("不支持采样时使用率应为 0, got %d", usage)
	}

	source := CPUUsageSource()
	switch source {
	case cpuSourceCgroupV2, cpuSourceCgroupV1, cpuSourceProcStat, cpuSourceRusage, cpuSourceUnsupported:
	default:
		t.Errorf("未知的 CPU 数据来源标识: %q", source)
	}
	if supported == (source == cpuSourceUnsupported) {
		t.Errorf("来源标识与支持状态不一致, supported=%v source=%q", supported, source)
	}
}

func TestShedderNewWithCPUFunc_NilFuncIsSafe(t *testing.T) {
	s := NewShedderWithCPUFunc(newTestShedderConfig(), nil)
	s.storeAvgFlying(1000)
	// cpuUsageFn 为 nil 时按「不支持采样」处理，不应 panic 也不应丢请求
	for i := 0; i < 3; i++ {
		if !s.allowAt(s.start) {
			t.Fatal("CPU 源为 nil 时不应丢弃请求")
		}
	}
}

// ============================================================================
// 配置
// ============================================================================

// 配置类型来自 go-config 的 pkg/shedder，本仓库只保留毫秒换算与非法值回落
// （见 shedder_config.go），所以这一组测试盯的是这两件事，以及它们不改写入参
//
// [EN] The configuration type comes from go-config's pkg/shedder; this repository keeps only
// the millisecond conversions and the fallback of invalid values (shedder_config.go), so this
// group covers exactly those two, plus the guarantee that the input is left untouched

func TestNormalizeShedderConfig_NilUsesDefault(t *testing.T) {
	cfg := normalizeShedderConfig(nil)
	def := shedderconfig.Default()
	if cfg == nil {
		t.Fatal("入参为 nil 时应返回一份默认配置而不是 nil")
	}
	if cfg.ModuleName != def.ModuleName || cfg.CPUThreshold != def.CPUThreshold ||
		cfg.WindowMS != def.WindowMS || cfg.WindowBuckets != def.WindowBuckets {
		t.Errorf("入参为 nil 时应返回默认配置, got %+v", cfg)
	}
}

func TestNormalizeShedderConfig_InvalidFieldsFallBack(t *testing.T) {
	def := shedderconfig.Default()
	cfg := normalizeShedderConfig(&shedderconfig.Shedder{
		CPUThreshold:   5000, // 超出 (0,1000]
		MaxConcurrency: -1,
		WindowMS:       0,
		WindowBuckets:  -1,
		CooldownMS:     0,
		MinRTMS:        -5,
		FlyingBeta:     2, // 不在 (0,1)
	})
	if cfg.ModuleName != def.ModuleName {
		t.Errorf("模块名应回落默认, got %q", cfg.ModuleName)
	}
	if cfg.CPUThreshold != def.CPUThreshold || cfg.MaxConcurrency != def.MaxConcurrency {
		t.Errorf("阈值字段应回落默认, got cpu=%d max=%d", cfg.CPUThreshold, cfg.MaxConcurrency)
	}
	if cfg.WindowMS != def.WindowMS || cfg.WindowBuckets != def.WindowBuckets ||
		cfg.CooldownMS != def.CooldownMS || cfg.MinRTMS != def.MinRTMS {
		t.Errorf("时间字段应回落默认, got %+v", cfg)
	}
	if cfg.FlyingBeta != def.FlyingBeta {
		t.Errorf("beta 应回落默认, got %v", cfg.FlyingBeta)
	}
}

func TestNormalizeShedderConfig_KeepsValidFields(t *testing.T) {
	src := &shedderconfig.Shedder{
		ModuleName:     "shedder-test",
		Enabled:        true,
		CPUThreshold:   850,
		MaxConcurrency: 512,
		WindowMS:       3000,
		WindowBuckets:  30,
		CooldownMS:     800,
		MinRTMS:        600,
		FlyingBeta:     0.8,
		ExcludePaths:   []string{"/health", "/internal/"},
	}
	cfg := normalizeShedderConfig(src)
	if cfg.ModuleName != src.ModuleName || !cfg.Enabled {
		t.Errorf("合法的基础字段不应被改动, got %+v", cfg)
	}
	if cfg.CPUThreshold != 850 || cfg.MaxConcurrency != 512 {
		t.Errorf("合法的阈值字段不应被改动, got cpu=%d max=%d", cfg.CPUThreshold, cfg.MaxConcurrency)
	}
	if cfg.WindowMS != 3000 || cfg.WindowBuckets != 30 || cfg.CooldownMS != 800 || cfg.MinRTMS != 600 {
		t.Errorf("合法的时间字段不应被改动, got %+v", cfg)
	}
	if cfg.FlyingBeta != 0.8 {
		t.Errorf("合法的 beta 不应被改动, got %v", cfg.FlyingBeta)
	}
	if len(cfg.ExcludePaths) != 2 || cfg.ExcludePaths[0] != "/health" {
		t.Errorf("排除路径不应被改动, got %v", cfg.ExcludePaths)
	}
}

func TestNormalizeShedderConfig_ClampsBuckets(t *testing.T) {
	cfg := normalizeShedderConfig(&shedderconfig.Shedder{WindowMS: 10, WindowBuckets: 1000})
	if cfg.WindowBuckets != 10 {
		t.Errorf("桶数不应超过窗口毫秒数, got %d", cfg.WindowBuckets)
	}
}

// TestNormalizeShedderConfig_DoesNotMutateInput 归一化必须返回副本
// 入参通常就是 cfg.Middleware.Shedder 本身，就地改写会把回落后的值写回全局配置树
func TestNormalizeShedderConfig_DoesNotMutateInput(t *testing.T) {
	src := &shedderconfig.Shedder{Enabled: true, WindowMS: 10, WindowBuckets: 1000, FlyingBeta: 2}
	cfg := normalizeShedderConfig(src)
	if cfg == src {
		t.Fatal("应返回副本而不是入参本身")
	}
	if src.WindowBuckets != 1000 || src.FlyingBeta != 2 || src.CPUThreshold != 0 {
		t.Errorf("归一化不应改写入参, got %+v", src)
	}
}

func TestShedderDurationHelpers(t *testing.T) {
	cfg := &shedderconfig.Shedder{WindowMS: 3000, CooldownMS: 800, MinRTMS: 600}
	if shedderWindowSpan(cfg) != 3*time.Second ||
		shedderCooldown(cfg) != 800*time.Millisecond ||
		shedderMinRT(cfg) != 600*time.Millisecond {
		t.Errorf("毫秒到 Duration 的换算不符, got window=%v cooldown=%v minRT=%v",
			shedderWindowSpan(cfg), shedderCooldown(cfg), shedderMinRT(cfg))
	}
}

// TestShedderConfigDefault_SafeDefaults go-config 默认值里本仓库依赖的两条约定
// 默认关闭是因为降载会主动丢流量，默认排除健康检查/指标是因为它们被丢会把一次
// 本可自愈的过载变成重启风暴
func TestShedderConfigDefault_SafeDefaults(t *testing.T) {
	cfg := shedderconfig.Default()
	if cfg.Enabled {
		t.Error("降载默认应关闭：主动丢流量的策略必须由使用方显式开启")
	}
	if cfg.MaxConcurrency != 0 {
		t.Errorf("硬并发上限默认应为 0（不限）, got %d", cfg.MaxConcurrency)
	}
	for _, want := range []string{"/health", "/metrics", "/debug"} {
		found := false
		for _, p := range cfg.ExcludePaths {
			if p == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("默认排除路径应包含 %s, got %v", want, cfg.ExcludePaths)
		}
	}
}
