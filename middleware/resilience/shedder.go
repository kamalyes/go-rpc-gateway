/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-09-23 17:08:30
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-09-28 21:50:36
 * @FilePath: \go-rpc-gateway\middleware\resilience\shedder.go
 * @Description: 自适应降载核心 - CPU/飞行请求/响应时间三信号判定，以及 HTTP 中间件与 gRPC 拦截器适配
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package resilience

import (
	"context"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	gcmiddleware "github.com/kamalyes/go-rpc-gateway/config/middleware"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// shedderDropResp 预分配的降载 503 JSON 响应体（与熔断的 breakerOpenResp 同理，拒绝路径零分配）
//
// [EN] Pre-allocated rejection body, mirroring the breaker's breakerOpenResp so that a shed
// request allocates nothing
var shedderDropResp = []byte(`{"code":503,"message":"Service overloaded (load shedding)","success":false}` + "\n")

// shedderRetryAfter 降载响应的 Retry-After 头
//
// 取 1 秒而不是更长：降载是秒级自愈的瞬时过载保护，等太久等于把一次抖动变成一次故障
// 固定字符串常量，避免每次拒绝都做一次整数格式化
//
// [EN] Retry-After for a shed response, fixed at one second: shedding protects against
// transient overload that heals within seconds, and a longer hint would turn a blip into an
// outage. A constant string avoids formatting an integer on every rejection
const shedderRetryAfter = "1"

// Shedder 自适应降载器 - 核心判定逻辑
//
// 判定依据三个本地信号，全部在进程内取得，不做任何网络调用：
// 降载的全部价值是在过载时以最低代价做出决定，一次 Redis 往返就把这个价值抵消掉了
//
// 1. CPU 使用率 —— 由进程级采样器提供（见 shedder_cpu.go），判断系统是否真的吃紧
// 2. 飞行请求数 —— 原子计数器，判断当前堆积了多少未完成请求
// 3. 响应时间滑窗 —— 固定内存环形桶（见 shedder_window.go），估出系统的处理能力上限
//
// 组合方式沿用 go-zero 自适应降载的核心思想，但按本仓库风格重新实现：
//
//	(CPU 超阈值 或 仍在冷却期) 且 (飞行请求数超过滑窗估出的承载上限) → 丢弃
//
// 两个条件必须同时成立，缺一不可：
// - 只看 CPU：计算密集但吞吐正常的服务会被误杀，CPU 跑满 90% 本身不是问题
// - 只看飞行请求数：IO 密集型服务（长轮询、慢下游）天然堆积大量飞行请求而 CPU 很低，
// 此时丢请求纯属自残；这也是本实现坚持要 CPU 信号作为前置条件的原因
//
// 热路径（Allow/Pass/Fail）完全无锁：状态用 atomic.Bool，计数用 atomic.Int64，
// 飞行请求数的 EWMA 用 atomic.Uint64 存 float64 位模式，与 Breaker 的取向一致
//
// [EN] Adaptive load shedder. It decides from three local signals — CPU usage from the
// process-wide sampler, the in-flight request count, and a fixed-memory response-time
// window — and never makes a network call, because the whole point of shedding is to decide
// as cheaply as possible while overloaded, and one Redis round trip cancels that out. The
// combination follows the core idea of go-zero's adaptive shedder, reimplemented in this
// repository's style: a request is dropped only when the CPU is over threshold (or the
// shedder is still cooling off) AND the in-flight count exceeds the capacity the window
// estimates. Both conditions are required: CPU alone would punish a compute-heavy service
// that is keeping up, while in-flight alone would punish an IO-bound service that naturally
// holds many pending requests at low CPU. The hot path takes no lock at all
type Shedder struct {
	// 不变配置（构造后只读）
	cpuThreshold   int64         // CPU 阈值，千分比
	maxConcurrency int64         // 硬并发上限，0 表示不限
	cooldown       time.Duration // 冷却时长
	fallbackRTMcs  int64         // 滑窗无样本时的兜底 RT，微秒
	flyingBeta     float64       // 飞行请求数 EWMA 系数
	cpuUsageFn     func() (int64, bool)
	window         *shedderWindow
	bucketsPerSec  float64
	start          time.Time // 时间基准（带单调读数）

	// 运行时状态（全原子）
	inFlight  atomic.Int64  // 当前飞行请求数
	avgFlying atomic.Uint64 // 飞行请求数的 EWMA，存 float64 位模式
	hot       atomic.Bool   // 是否处于降载冷却期
	droppedAt atomic.Int64  // 最近一次丢弃距 start 的纳秒数
	total     atomic.Int64  // 累计判定次数
	dropped   atomic.Int64  // 累计丢弃次数
}

// NewShedder 创建降载器（CPU 信号取自进程级采样器）
//
// [EN] Creates a shedder backed by the process-wide CPU sampler
func NewShedder(cfg *gcmiddleware.Shedder) *Shedder {
	return NewShedderWithCPUFunc(cfg, CPUUsage)
}

// NewShedderWithCPUFunc 创建降载器并指定 CPU 使用率来源
//
// cpuUsageFn 返回 (千分比, 该平台是否支持采样)：
// 第二个返回值为 false 时自适应判定整体静默，只保留 max-concurrency 硬上限，
// 这正是 darwin/windows 上的降级路径——读不到 CPU 时既不 panic，也不把缺失当成 0%
// 需要接自己的 CPU 来源（如监控 agent 推下来的使用率）时用这个构造函数
//
// [EN] Creates a shedder with an explicit CPU usage source. The function returns per-mille
// usage plus whether this platform supports sampling at all; when it reports false the
// adaptive path goes silent and only the max-concurrency ceiling remains, which is the
// degradation path on darwin and windows — no panic, and a missing reading is never treated
// as 0%. Use this constructor to plug in a different CPU source, such as usage pushed down
// by a monitoring agent
func NewShedderWithCPUFunc(cfg *gcmiddleware.Shedder, cpuUsageFn func() (int64, bool)) *Shedder {
	cfg = normalizeShedderConfig(cfg)
	if cpuUsageFn == nil {
		cpuUsageFn = func() (int64, bool) { return 0, false }
	}

	start := time.Now()
	window := newShedderWindow(shedderWindowSpan(cfg), cfg.WindowBuckets, start)
	s := &Shedder{
		cpuThreshold:   cfg.CPUThreshold,
		maxConcurrency: cfg.MaxConcurrency,
		cooldown:       shedderCooldown(cfg),
		fallbackRTMcs:  shedderMinRT(cfg).Microseconds(),
		flyingBeta:     cfg.FlyingBeta,
		cpuUsageFn:     cpuUsageFn,
		window:         window,
		bucketsPerSec:  window.bucketsPerSecond(),
		start:          start,
	}
	return s
}

// Allow 判定是否放行一个一元请求
//
// 返回 true 时调用方必须在请求结束后恰好调用一次 Pass 或 Fail，否则飞行请求数会永久偏高，
// 降载器会越来越倾向于丢请求；三个适配层都用 defer 保证这一点，连 handler panic 也不漏
// 返回 false 表示已被降载拒绝，飞行计数已经回退，调用方不要再调用 Pass/Fail
//
// [EN] Decides whether to admit one unary request. On true the caller must call exactly one
// of Pass or Fail when the request finishes, otherwise the in-flight count stays
// permanently inflated and the shedder drifts towards dropping everything; all three
// adapters guarantee this with defer, including when the handler panics. On false the
// request has been shed, the in-flight count is already rolled back, and the caller must not
// call Pass or Fail
func (s *Shedder) Allow() bool {
	return s.allowAt(time.Now())
}

// allowAt 带显式时间的放行判定（测试用确定性入口）
//
// 先占位再判定：直接 Load 判断会让多个 goroutine 同时看到「还有余量」从而冲破硬上限，
// 占位后拒绝则回退，代价是一次多余的原子加减，换来 max-concurrency 是真正的上限
// 回退走裸 inFlight.Add(-1) 而不是 settle：被拒绝的请求不应参与飞行数 EWMA
//
// [EN] Admission decision with an explicit clock, the deterministic entry point used by
// tests. It reserves a slot before deciding: a plain load-then-check lets concurrent
// goroutines all observe spare capacity and breach the hard ceiling. A rejection rolls the
// slot back with a raw decrement rather than through settle, because a shed request must not
// feed the in-flight EWMA
func (s *Shedder) allowAt(now time.Time) bool {
	s.total.Add(1)
	flying := s.inFlight.Add(1)

	// 1) 硬并发上限：平台无关、确定性，CPU 读不到时唯一仍然有效的保护
	// 刻意不进入冷却期——硬上限本身是自限的，飞行数降下来就自然放行，
	// 把它和自适应判定的冷却状态耦合只会让两套语义互相干扰
	if s.maxConcurrency > 0 && flying > s.maxConcurrency {
		s.inFlight.Add(-1)
		s.dropped.Add(1)
		return false
	}

	// 2) 自适应判定
	if s.shouldDrop(now, flying) {
		s.inFlight.Add(-1)
		s.dropped.Add(1)
		return false
	}

	return true
}

// AllowStream 判定是否放行一条新建的流
//
// 与 Allow 的区别是流不占用飞行计数、也不产生 RT 样本，只读当前状态做一次判定：
// 承载上限由「最大通过量 × 最小平均 RT」估出（Little 定律），而一条长连接流的
// 生命周期与它消耗的资源无关——一条挂一小时的流会把飞行计数钉死在高位，
// 让 maxFlight 估计彻底失效，最后把一元请求连带全部丢掉
// 因此流只借用一元流量估出的容量判定「现在能不能再建一条流」，不反向污染这个估计
// 代价是纯流式服务拿不到自适应信号，这类服务应改用 max-concurrency 硬上限
//
// [EN] Decides whether to admit a newly established stream. Unlike Allow, a stream occupies
// no in-flight slot and produces no RT sample; the check is read-only. Capacity is estimated
// from max pass times min average RT (Little's law), but a long-lived stream's lifetime says
// nothing about the resources it uses — a stream held open for an hour would pin the
// in-flight count high, invalidate the maxFlight estimate and end up shedding all unary
// traffic as well. So a stream borrows the capacity estimate derived from unary traffic to
// answer "may one more stream be opened right now" without polluting that estimate in
// return. The cost is that a purely streaming service gets no adaptive signal and should
// rely on the max-concurrency ceiling instead
func (s *Shedder) AllowStream() bool {
	return s.allowStreamAt(time.Now())
}

// allowStreamAt 带显式时间的建流判定（测试用确定性入口）
//
// [EN] Stream admission with an explicit clock, used by tests
func (s *Shedder) allowStreamAt(now time.Time) bool {
	s.total.Add(1)
	flying := s.inFlight.Load()

	if s.maxConcurrency > 0 && flying >= s.maxConcurrency {
		s.dropped.Add(1)
		return false
	}
	if s.shouldDrop(now, flying) {
		s.dropped.Add(1)
		return false
	}
	return true
}

// Pass 记录一个正常完成的请求：归还飞行计数并把响应时间计入滑窗
//
// [EN] Records a normally completed request: releases the in-flight slot and feeds the
// response time into the window
func (s *Shedder) Pass(rt time.Duration) {
	s.passAt(time.Now(), rt)
}

// passAt 带显式时间的完成记录（测试用确定性入口）
//
// [EN] Completion recording with an explicit clock, used by tests
func (s *Shedder) passAt(now time.Time, rt time.Duration) {
	s.settle()
	s.window.record(now, rt)
}

// Fail 记录一个异常结束的请求：只归还飞行计数，不计入滑窗
//
// 失败请求不进滑窗是有意的：滑窗要估的是「系统健康时能稳定承载多少并发」，
// 把超时、500 这些样本混进去会同时抬高 RT、抬高通过量，算出一个系统根本达不到的承载上限，
// 反过来让降载在真正该动手的时候不动手
//
// [EN] Records an abnormally finished request: releases the in-flight slot without feeding
// the window. Excluding failures is deliberate — the window estimates how much concurrency
// the system sustains while healthy, and folding timeouts and 500s into it would inflate
// both the RT and the pass count, producing a capacity the system cannot actually reach and
// keeping the shedder idle exactly when it should act
func (s *Shedder) Fail() {
	s.settle()
}

// settle 归还一个飞行计数并更新飞行数 EWMA
//
// EWMA 只在请求结束时更新（与 go-zero 一致）：在入口更新会把还没开始执行的请求算进去，
// 让曲线跟着瞬时流量抖动 只在出口更新使 avgFlying 比 flying 略微滞后，于是
// 流量陡增时它涨得慢（多收一些请求），流量陡降时它落得慢（少收一些请求），整体更稳
// Load/计算/Store 之间存在良性竞争：并发结束的请求可能互相覆盖一次更新，
// 对一个本就用来平滑的指数移动平均没有实际影响，换来的是热路径无锁
//
// [EN] Releases one in-flight slot and updates the in-flight EWMA. The EWMA is updated only
// on completion, as in go-zero: updating on admission would count requests that have not
// started and make the curve track instantaneous traffic. Updating on exit leaves avgFlying
// slightly behind flying, so it rises slowly when traffic spikes (admitting a little more)
// and falls slowly when traffic drops (admitting a little less), which is steadier overall.
// The load-compute-store sequence races benignly: concurrently finishing requests may
// overwrite one update, which is immaterial for a value whose job is smoothing, and in
// exchange the hot path stays lock-free
func (s *Shedder) settle() {
	flying := s.inFlight.Add(-1)
	// 兜底：Pass/Fail 多于 Allow（调用方记账出错）会把计数带到负数，
	// 负的飞行数会让降载永久失效，这里纠回 0；CAS 失败说明已有并发更新，交给它处理
	if flying < 0 {
		s.inFlight.CompareAndSwap(flying, 0)
		flying = 0
	}
	prev := s.loadAvgFlying()
	s.storeAvgFlying(prev*s.flyingBeta + float64(flying)*(1-s.flyingBeta))
}

// shouldDrop 自适应判定：系统吃紧（或仍在冷却期）且吞吐已超承载上限
//
// [EN] Adaptive decision: the system is strained (or still cooling off) and throughput has
// already passed the estimated ceiling
func (s *Shedder) shouldDrop(now time.Time, flying int64) bool {
	if !s.systemOverloaded() && !s.stillHot(now) {
		return false
	}
	if !s.overCapacity(now, flying) {
		return false
	}
	s.markDropped(now)
	return true
}

// systemOverloaded CPU 是否超过阈值
//
// 平台不支持采样时返回 false，即自适应判定永不触发；这是刻意选择的失败方向：
// 读不到 CPU 就认为系统健康，最坏情况是降载不起作用（退回限流与熔断保护），
// 而反过来假设过载会在一个完全健康的服务上凭空开始丢流量
//
// [EN] Whether CPU usage is over the threshold. When the platform cannot sample it returns
// false, so the adaptive path never fires — a deliberate choice of failure direction: an
// unreadable CPU means "assume healthy", whose worst case is shedding being inert while rate
// limiting and the breaker still protect the service, whereas assuming overload would start
// dropping traffic on a perfectly healthy process
func (s *Shedder) systemOverloaded() bool {
	usage, supported := s.cpuUsageFn()
	return supported && usage >= s.cpuThreshold
}

// stillHot 是否仍处于降载冷却期
//
// 冷却是防抖的关键：CPU 使用率是 250ms 采样 + EWMA 的滞后信号，一旦开始丢请求，
// 负载下降会先反映在飞行请求数上、过几百毫秒才反映到 CPU 上 没有冷却就会出现
// 「丢 → CPU 回落 → 全放 → 立刻又堆积 → 再丢」的高频抖动，客户端看到的是
// 成功与 503 交替出现，重试也不断把流量打回来
// 冷却期内 CPU 条件视为仍然成立，但承载上限条件依然要单独满足：
// 这保证了冷却不等于「无脑拒绝一秒」，负载真的降下来了就立刻恢复放行
//
// [EN] Whether the shedder is still cooling off. The cool-off is what prevents flapping: CPU
// usage is a lagging signal (250ms sampling plus an EWMA), so once requests start being
// dropped the falling load shows up in the in-flight count first and in CPU only hundreds of
// milliseconds later. Without a cool-off the shedder oscillates — drop, CPU recovers, admit
// everything, instantly pile up again, drop — and the client sees successes and 503s
// alternating while its retries keep feeding the spike back in. During the cool-off the CPU
// condition counts as still met, but the capacity condition must independently hold too, so
// cooling off never degenerates into "reject blindly for a second": once load genuinely
// drops, traffic flows again immediately
func (s *Shedder) stillHot(now time.Time) bool {
	if !s.hot.Load() {
		return false
	}
	if now.Sub(s.start)-time.Duration(s.droppedAt.Load()) < s.cooldown {
		return true
	}
	s.hot.Store(false)
	return false
}

// markDropped 进入降载冷却期
//
// [EN] Enters the shedding cool-off window
func (s *Shedder) markDropped(now time.Time) {
	s.droppedAt.Store(int64(now.Sub(s.start)))
	s.hot.Store(true)
}

// overCapacity 当前吞吐是否已超过承载上限
//
// 平滑值与瞬时值必须同时超限：
// - 只看平滑值：流量退潮后它还挂在高位，会继续多丢几百毫秒
// - 只看瞬时值：一次正常的突发（比如客户端批量重试）就会被当成过载
//
// [EN] Whether throughput has passed the ceiling. Both the smoothed and the instantaneous
// in-flight count must exceed it: the smoothed value alone stays high after traffic recedes
// and would keep shedding for hundreds of extra milliseconds, while the instantaneous value
// alone would read an ordinary burst, such as a client's batched retries, as an overload
func (s *Shedder) overCapacity(now time.Time, flying int64) bool {
	maxFlight := s.maxFlight(now)
	return s.loadAvgFlying() > maxFlight && float64(flying) > maxFlight
}

// maxFlight 由滑窗估出的承载上限（Little 定律：并发 = 吞吐 × 驻留时间）
//
//	最大吞吐 = 单桶最大通过量 × 每秒桶数
//	承载上限 = 最大吞吐 × 最小平均响应时间
//
// 取「最大通过量」与「最小平均 RT」而不是各自的平均值，是为了估出系统状态最好时的能力：
// 这两个值描述的是「这台实例确实做到过的吞吐」与「它确实达到过的最低延迟」，
// 二者相乘就是它曾经稳定承载过的并发量，超过这个数就说明请求在排队而不是在被处理
// 下界取 1，避免滑窗还没有样本（刚启动、或前一轮全是失败请求）时把上限算成 0 而全盘拒绝
//
// [EN] The concurrency ceiling estimated from the window via Little's law: concurrency
// equals throughput times residence time, where throughput is the maximum per-bucket pass
// count scaled to a second and residence time is the minimum average response time. Taking
// the maximum pass count and the minimum average RT rather than their averages estimates the
// system at its best: together they describe throughput this instance has actually achieved
// at the lowest latency it has actually reached, and their product is the concurrency it has
// sustained — beyond it, requests are queueing rather than being served. The result is
// floored at 1 so that an empty window (just started, or a round of nothing but failures)
// cannot compute a ceiling of zero and reject everything
func (s *Shedder) maxFlight(now time.Time) float64 {
	st := s.window.stat(now)
	minRTMcs := st.MinRT
	if minRTMcs <= 0 {
		minRTMcs = s.fallbackRTMcs
	}
	maxQPS := float64(st.MaxPass) * s.bucketsPerSec
	return math.Max(1, maxQPS*float64(minRTMcs)/float64(time.Second/time.Microsecond))
}

// loadAvgFlying 读取飞行请求数的 EWMA
//
// [EN] Reads the in-flight EWMA
func (s *Shedder) loadAvgFlying() float64 {
	return math.Float64frombits(s.avgFlying.Load())
}

// storeAvgFlying 写入飞行请求数的 EWMA
//
// [EN] Stores the in-flight EWMA
func (s *Shedder) storeAvgFlying(v float64) {
	s.avgFlying.Store(math.Float64bits(v))
}

// Shedding 当前是否处于降载状态（冷却期内）
//
// [EN] Whether the shedder is currently shedding, i.e. inside its cool-off window
func (s *Shedder) Shedding() bool {
	return s.stillHot(time.Now())
}

// InFlight 当前飞行请求数
//
// [EN] The current in-flight request count
func (s *Shedder) InFlight() int64 {
	return s.inFlight.Load()
}

// ShedderSnapshot 降载器快照（强类型，便于指标采集）
//
// [EN] Strongly typed shedder snapshot for metrics collection
type ShedderSnapshot struct {
	Shedding        bool    // 是否处于降载冷却期
	CPUUsage        int64   // CPU 使用率千分比；CPUSupported 为 false 时无意义
	CPUSupported    bool    // 当前平台是否支持 CPU 采样
	CPUThreshold    int64   // CPU 阈值千分比
	InFlight        int64   // 当前飞行请求数
	AvgFlying       float64 // 飞行请求数的 EWMA
	MaxFlight       float64 // 滑窗估出的承载上限
	MaxPass         int64   // 滑窗内单桶最大通过量
	MinRTMicros     int64   // 滑窗内最小平均响应时间，微秒；0 表示无样本
	MaxConcurrency  int64   // 硬并发上限，0 表示不限
	TotalRequests   int64   // 累计判定次数
	DroppedRequests int64   // 累计丢弃次数
}

// Snapshot 获取强类型快照（全原子读取，无锁）
//
// [EN] Returns a strongly typed snapshot; all reads are atomic and lock-free
func (s *Shedder) Snapshot() ShedderSnapshot {
	now := time.Now()
	usage, supported := s.cpuUsageFn()
	st := s.window.stat(now)
	return ShedderSnapshot{
		Shedding:        s.hot.Load(),
		CPUUsage:        usage,
		CPUSupported:    supported,
		CPUThreshold:    s.cpuThreshold,
		InFlight:        s.inFlight.Load(),
		AvgFlying:       s.loadAvgFlying(),
		MaxFlight:       s.maxFlight(now),
		MaxPass:         st.MaxPass,
		MinRTMicros:     st.MinRT,
		MaxConcurrency:  s.maxConcurrency,
		TotalRequests:   s.total.Load(),
		DroppedRequests: s.dropped.Load(),
	}
}

// Reset 重置降载器运行时状态（配置不变）
//
// 不在热路径上；飞行计数一并清零，因此只应在没有在途请求时调用（如测试、热加载后）
//
// [EN] Resets the shedder's runtime state, leaving configuration intact. Not on the hot
// path; it clears the in-flight count too, so it should only be called when no request is
// in flight, such as from tests or after a reload
func (s *Shedder) Reset() {
	s.window.reset()
	s.inFlight.Store(0)
	s.storeAvgFlying(0)
	s.hot.Store(false)
	s.droppedAt.Store(0)
	s.total.Store(0)
	s.dropped.Store(0)

	if global.LOGGER != nil {
		global.LOGGER.InfoMsg("Load shedder reset")
	}
}

// ============================================================================
// ShedderManager 降载管理器
// [EN] ShedderManager
// ============================================================================

// ShedderManager 降载管理器 - 持有进程唯一的降载器实例与路径排除规则
//
// 与 BreakerManager「每个路径一个断路器」的模型刻意不同：降载依赖的 CPU 使用率与
// 飞行请求数都是进程级信号，拆成每路径一份既拿不到更细的信号，还会让每条路径各自
// 按同一份 CPU 读数独立判定，等于把阈值放大了 N 倍——N 条路径都只用到 1/N 的容量时，
// 整个进程已经满载但没有任何一条路径认为自己该降载
//
// 路径策略也与熔断相反：熔断用 prevention-paths 白名单（只保护明确列出的路径），
// 降载用 exclude-paths 黑名单（默认保护全部流量） 过载是全进程现象，
// 漏保护一条高流量路径就等于没有保护；而健康检查、指标这类必须永远可达的端点数量有限、
// 可以穷举，正适合做排除
//
// [EN] Load-shedding manager, holding the single process-wide shedder and the path exclusion
// rules. The per-path model used by BreakerManager is deliberately not followed: CPU usage
// and in-flight count are process-level signals, so splitting them per path would gain no
// resolution while letting every path judge the same CPU reading independently, effectively
// multiplying the threshold by the number of paths — with N paths each using 1/N of the
// capacity the process is already saturated and no single path thinks it should shed. The
// path policy is inverted too: the breaker uses a prevention-paths allowlist, the shedder an
// exclude-paths denylist that protects all traffic by default, because overload is a
// process-wide phenomenon and missing one high-traffic path means no protection at all,
// while the endpoints that must stay reachable are few and enumerable
type ShedderManager struct {
	shedder         *Shedder
	config          *gcmiddleware.Shedder
	excludePrefixes []string
}

// NewShedderManager 创建降载管理器
//
// [EN] Creates a load-shedding manager
func NewShedderManager(cfg *gcmiddleware.Shedder) *ShedderManager {
	cfg = normalizeShedderConfig(cfg)

	// 排除规则按前缀匹配，不用熔断那套精确 map：/debug/pprof/heap 这类子路径必须
	// 能被 /debug 一条规则覆盖，精确匹配做不到这件事
	// 列表长度是个位数且构造时固定，线性扫描比 map 查找更快也更省内存
	prefixes := make([]string, 0, len(cfg.ExcludePaths))
	for _, p := range cfg.ExcludePaths {
		if p != "" {
			prefixes = append(prefixes, p)
		}
	}

	return &ShedderManager{
		shedder:         NewShedder(cfg),
		config:          cfg,
		excludePrefixes: prefixes,
	}
}

// Shedder 获取降载器实例
//
// [EN] Returns the shedder instance
func (m *ShedderManager) Shedder() *Shedder {
	if m == nil {
		return nil
	}
	return m.shedder
}

// Config 获取生效的降载配置（已补齐默认值）
//
// [EN] Returns the effective configuration, with defaults filled in
func (m *ShedderManager) Config() *gcmiddleware.Shedder {
	if m == nil {
		return nil
	}
	return m.config
}

// IsPathProtected 检查路径是否需要降载保护（默认全部保护，命中排除前缀则放行）
//
// [EN] Whether a path is subject to shedding; everything is protected unless it matches an
// exclude prefix
func (m *ShedderManager) IsPathProtected(path string) bool {
	if m == nil || m.shedder == nil {
		return false
	}
	for _, prefix := range m.excludePrefixes {
		if strings.HasPrefix(path, prefix) {
			return false
		}
	}
	return true
}

// Snapshot 获取降载器快照
//
// [EN] Returns the shedder snapshot
func (m *ShedderManager) Snapshot() ShedderSnapshot {
	if m == nil || m.shedder == nil {
		return ShedderSnapshot{}
	}
	return m.shedder.Snapshot()
}

// Reset 重置降载器运行时状态
//
// [EN] Resets the shedder's runtime state
func (m *ShedderManager) Reset() {
	if m == nil || m.shedder == nil {
		return
	}
	m.shedder.Reset()
}

// ============================================================================
// HTTP 中间件适配
// [EN] HTTP middleware adapter
// ============================================================================

// ShedderHTTPMiddleware 创建降载 HTTP 中间件（使用共享的 ShedderManager）
//
// 被降载时返回 503 + 预分配响应体（零分配）+ Retry-After: 1，错误码语义与 gRPC 侧一致
// 记账放在 defer 里而不是 ServeHTTP 之后：handler panic 时若不归还飞行计数，
// 这个计数会永久偏高，几次 panic 就能让降载器把全部流量当成过载丢掉
// panic 路径按 Fail 记（只归还计数、不进滑窗），并且不做 recover——
// recover 后再 panic 会污染 RecoveryMiddleware 看到的调用栈，得不偿失
//
// [EN] Creates the shedding HTTP middleware over a shared ShedderManager. A shed request
// gets a 503 with the pre-allocated body (zero allocations) and Retry-After: 1, matching the
// gRPC side's semantics. Accounting lives in a defer rather than after ServeHTTP: if a
// panicking handler never released its in-flight slot the count would stay inflated for
// good, and a handful of panics would be enough for the shedder to read all traffic as
// overload. The panic path is recorded as a failure — slot released, window untouched — and
// deliberately does not recover, since recovering and re-panicking would corrupt the stack
// that RecoveryMiddleware reports
func ShedderHTTPMiddleware(manager *ShedderManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 未启用或路径被排除：直通，不占用飞行计数
			if manager == nil || !manager.IsPathProtected(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			shedder := manager.Shedder()
			if !shedder.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", shedderRetryAfter)
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write(shedderDropResp)
				return
			}

			start := time.Now()
			rw := core.NewResponseWriter(w)
			completed := false
			defer func() {
				// completed 为 false 说明 handler panic 了，只归还计数
				if !completed || rw.IsServerError() {
					shedder.Fail()
				} else {
					shedder.Pass(time.Since(start))
				}
				rw.Release()
			}()

			next.ServeHTTP(rw, r)
			completed = true
		})
	}
}

// ============================================================================
// gRPC 拦截器适配
// [EN] gRPC interceptor adapters
// ============================================================================

// shedderDropGRPCErr 预构建的降载拒绝错误（与 HTTP 侧 shedderDropResp 同理，拒绝路径零分配）
//
// 错误码取 errors.ErrCodeServiceOverloaded(4005)，经 AppError.ToGRPCError() 得到
// codes.Unavailable（三态映射见 docs/ERRORS.md），与 HTTP 侧的 503 对应同一个错误码
//
// 为什么是 Unavailable 而不是 ResourceExhausted：
// 1. 两侧状态码必须一致 grpc-gateway 把 ResourceExhausted 映射成 HTTP 429，
// 而降载语义是「服务端自己忙不过来」，不是「调用方超了配额」；429 会让调用方
// 去查自己的限额而不是退避，HTTP 直连与经 gateway 代理还会拿到不同状态码
// 2. Unavailable 对调用方是正确的指引：gRPC 规范里 Unavailable 明确是
// 「瞬时不可用，应带退避重试」，ResourceExhausted 指向配额而非容量
// 3. 被下游熔断器观测到是期望行为而非隐患 调用方的熔断器因为一个过载的服务打开，
// 正是要的负反馈（流量减少 → 过载缓解）；真正要避免的正反馈是
// 「自己的拒绝被自己的熔断器计为下游故障」，那是靠拦截器链顺序解决的：
// 降载挂在熔断之外，本进程的熔断器永远看不到这个错误码（见 server/grpc.go）
//
// 同样不走 errors.ToGRPCError(ctx, bizCode) 的 i18n 路径：那条路径给业务错误码用，
// 降载拒绝是框架层决定，也不该在过载时再付一次翻译查表的代价
//
// [EN] Pre-built rejection error. The code is ErrCodeServiceOverloaded(4005), which maps to
// codes.Unavailable and to HTTP 503, the same code the HTTP middleware returns. It is
// Unavailable rather than ResourceExhausted because: both transports must agree, and
// grpc-gateway maps ResourceExhausted to HTTP 429, which tells the caller to check its quota
// rather than back off, and would make a direct HTTP call and a proxied one disagree;
// Unavailable is the correct guidance per the gRPC spec, meaning "transiently unavailable,
// retry with backoff", while ResourceExhausted points at quota rather than capacity; and a
// downstream breaker observing it is the desired negative feedback, not a hazard — the
// feedback loop to avoid is a shedder's own rejection being counted as a downstream failure
// by its own breaker, which is solved by chain order instead: shedding sits outside the
// breaker, so this process's breaker never sees this code (see server/grpc.go). The i18n
// path is skipped for the same reason as in the breaker
var shedderDropGRPCErr = errors.NewError(errors.ErrCodeServiceOverloaded, "").ToGRPCError()

// recordShedderResult 按 handler 返回的 error 记录完成结果
//
// 直接复用熔断的 IsBreakerFailureCode：两者需要回答的是同一个问题——
// 「这次调用是服务端真的坏了，还是调用方自己的问题」 降载需要它是因为失败样本
// 不能进滑窗（见 Fail 的说明），而失败判定只有一份实现才不会在两处慢慢长歪
// 判定口径与 HTTP 侧的 rw.IsServerError() 对齐（都等价于 5xx）
//
// [EN] Records a completion based on the handler's error, reusing the breaker's
// IsBreakerFailureCode: both need to answer the same question — was this the server
// genuinely failing, or the caller's own mistake. The shedder needs it because failed
// samples must stay out of the window (see Fail), and keeping a single implementation is
// what stops the two classifications from drifting apart. The classification matches the
// HTTP side's rw.IsServerError(), both equivalent to 5xx
func recordShedderResult(s *Shedder, err error, rt time.Duration) {
	if IsBreakerFailureCode(status.Code(err)) {
		s.Fail()
		return
	}
	s.Pass(rt)
}

// UnaryServerShedderInterceptor 创建降载 gRPC 一元拦截器（复用共享的 ShedderManager）
//
// 与 HTTP 中间件共用同一个 Shedder 实例与同一套信号：降载判定的是整个进程的负载，
// HTTP 与 gRPC 两侧的请求消耗的是同一份 CPU，统计必须合在一起才有意义
// 排除规则对 gRPC 按方法全名（形如 /user.v1.UserService/GetUser）前缀匹配，
// 因此默认的 /health、/metrics 这类 HTTP 前缀不会误伤任何 gRPC 方法；
// 要排除某个服务的全部方法，配 /user.v1.UserService/ 即可
// manager 为 nil 时直通，便于未启用降载的场景安全调用
//
// [EN] Unary shedding interceptor, sharing one Shedder instance and one set of signals with
// the HTTP middleware: shedding judges whole-process load, requests on both transports burn
// the same CPU, and the statistics are only meaningful combined. Exclusions match a prefix
// of the fully qualified method name, so the default HTTP-shaped prefixes never touch a gRPC
// method; to exclude an entire service, configure /user.v1.UserService/. A nil manager
// passes through so callers that disabled shedding stay safe
func UnaryServerShedderInterceptor(manager *ShedderManager) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if manager == nil || !manager.IsPathProtected(info.FullMethod) {
			return handler(ctx, req)
		}

		shedder := manager.Shedder()
		// 过载：尽早拒绝，不触达 handler，也不打日志
		// （降载打开期间每条拒绝都打日志只会在最需要日志的时候把日志刷爆）
		if !shedder.Allow() {
			return nil, shedderDropGRPCErr
		}

		start := time.Now()
		completed := false
		defer func() {
			// completed 为 false 说明 handler panic 了，只归还飞行计数
			if !completed {
				shedder.Fail()
			}
		}()

		resp, err := handler(ctx, req)
		completed = true
		recordShedderResult(shedder, err, time.Since(start))
		return resp, err
	}
}

// StreamServerShedderInterceptor 创建降载 gRPC 流式拦截器（复用共享的 ShedderManager）
//
// 只在建流时判定一次（与限流、熔断的 Stream 粒度一致），且走 AllowStream：
// 流不占用飞行计数、不产生 RT 样本，理由见 Shedder.AllowStream
// 因此这里没有 Pass/Fail 调用，也就不存在长连接期间计数泄漏的问题
//
// [EN] Stream shedding interceptor. It decides once when the stream is established, matching
// the granularity of the rate-limit and breaker stream interceptors, and goes through
// AllowStream so the stream occupies no in-flight slot and produces no RT sample — see
// Shedder.AllowStream for why. There is consequently no Pass or Fail call here and no way
// for a long-lived stream to leak a slot
func StreamServerShedderInterceptor(manager *ShedderManager) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if manager == nil || !manager.IsPathProtected(info.FullMethod) {
			return handler(srv, ss)
		}

		if !manager.Shedder().AllowStream() {
			return shedderDropGRPCErr
		}

		return handler(srv, ss)
	}
}
