/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-02-01 23:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 01:26:00
 * @FilePath: \go-rpc-gateway\cpool\grpc\watcher.go
 * @Description: 下游服务发版感知：连接事件驱动 + 契约指纹兜底
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package grpc

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	gwglobal "github.com/kamalyes/go-rpc-gateway/global"
	"google.golang.org/grpc/connectivity"
)

// =============================================================================
// 默认参数
// =============================================================================

const (
	// DefaultWatchInterval 契约指纹兜底轮询的默认间隔
	//
	// 这个值只决定"最坏情况下多久收敛"：发版通常伴随连接重建，由事件路径秒级捕获，
	// 轮询是给捕获不到的情况兜底（见 ServiceWatcher 的说明）。定在 60s 是因为
	// 一轮的成本是一次 reflection 往返 + 若干次 SHA256，摊到 60s 可以忽略，
	// 再压短只会放大对下游 reflection 的压力而换不到什么
	// [EN] Only bounds the worst-case convergence time: a deploy usually rebuilds the
	// connection and is caught by the event path within seconds. 60s because one round costs
	// a single reflection round trip plus a few hashes, and shortening it mostly just adds
	// load to the downstream reflection service
	DefaultWatchInterval = 60 * time.Second

	// DefaultWatchMinInterval 同一服务两次 reconcile 之间的最小间隔
	//
	// 滚动发布期间连接状态会反复抖动，没有这个下限的话每次 Ready 上升沿都要打一次
	// reflection。注意是"合并"而不是"丢弃"：抖动窗口内的触发会被推迟到下限之后执行，
	// 丢弃则可能把真正的那次变更一起丢掉
	// [EN] Connection state flaps throughout a rolling deploy. Triggers inside the window are
	// coalesced and deferred, never dropped — dropping could discard the real change
	DefaultWatchMinInterval = 2 * time.Second

	// watchJitterRatio 兜底轮询的抖动比例（±10%）
	//
	// 多个网关副本是同时启动的，不抖动就会同一时刻一起打下游的 reflection
	// [EN] Gateway replicas start together and would otherwise hit reflection in lockstep
	watchJitterRatio = 0.1

	// DefaultWatchReconcileTimeout 单次 reconcile 的默认超时
	//
	// 必须有：reflection 走的是 stream，TCP 连着但服务端不回包（线程池打满、死锁、
	// 中间件吞包）时，没有 deadline 的 Recv 会一直阻塞。worker 是每服务一个 goroutine，
	// 它卡住就意味着这个服务从此既收不到事件也不再轮询 —— 静默失联，
	// 比直接报错难查得多
	// [EN] Required: reflection runs over a stream, and a Recv without a deadline blocks
	// forever when the peer is connected but never replies. One stuck worker means that
	// service silently stops being watched altogether
	DefaultWatchReconcileTimeout = 15 * time.Second

	// watchMaxBackoffSteps reconcile 连续失败时退避的最大倍数
	// 2^4 = 16 倍间隔封顶，避免一个长期挂掉的服务被反复探测
	// [EN] Caps backoff at 16x so a long-dead service is not probed forever at full rate
	watchMaxBackoffSteps = 4
)

// =============================================================================
// ServiceWatcher
// =============================================================================

// ServiceWatchOptions ServiceWatcher 的构造参数
type ServiceWatchOptions struct {
	// MuxFn 返回当前的 ServeMux，必填
	//
	// 必须是函数而不是 *runtime.ServeMux：RebuildHTTPGateway 会换掉整个 mux，
	// 持有旧指针的 watcher 会把路由注册到一个已经不在服务链路上的 mux 上，
	// 表现为"日志说注册成功了，但请求还是 404"
	// [EN] A function, not a pointer: RebuildHTTPGateway swaps the mux, and a watcher holding
	// the old one would register routes into a mux no longer serving traffic
	MuxFn func() *runtime.ServeMux

	// Services 需要监视的服务名（配置中的 key），为空则不启动
	Services []string

	// Interval 契约指纹兜底轮询间隔，<=0 表示只依赖连接事件，不轮询
	Interval time.Duration

	// MinInterval 同一服务两次 reconcile 的最小间隔，<=0 用默认值
	MinInterval time.Duration

	// ReconcileTimeout 单次 reconcile 的超时，<=0 用默认值
	ReconcileTimeout time.Duration

	// IsHealthy 可选的健康判定，仅用于跳过"兜底轮询"对已知不可用服务的无谓探测
	//
	// 只作用于轮询，不作用于事件触发：事件（连接 Ready 上升沿、外部 Trigger）本身就
	// 代表"刚刚发生了变化，现在就该看一眼"，而健康状态是另一条路径以固定间隔刷新的，
	// 必然滞后。用它过滤事件就会出现这种丢失：连接刚 Ready，watchConn 立即投递信号，
	// 但健康检查还要几秒才把该服务标回 healthy —— 信号被消费掉又被丢弃，
	// 真正的那次变更要等到下一轮兜底轮询才被发现
	// [EN] Applies to the fallback poll only, never to event triggers. Health state is
	// refreshed by another path at a fixed interval and therefore lags: right after a
	// connection turns Ready the watcher signals immediately, while the service is still
	// marked unhealthy for a few seconds. Filtering events on it would consume and discard
	// that signal, deferring a real change to the next poll
	//
	// 不传也能正确工作：reconcile 自己会失败并退避
	// [EN] Optional; without it reconcile simply fails and backs off
	IsHealthy func(serviceName string) bool

	// OnChange 可选回调，仅在契约确实发生变化并完成重注册后触发
	OnChange func(serviceName string, routes []string)
}

// ServiceWatcher 感知下游服务发版，把新增的 proto 文件与 HTTP 路由热注册进 mux
//
// 为什么不是"把健康检查间隔调短"：
//
//	原有的感知路径只有 HealthChecker.onRecover，即 unhealthy → healthy 这一个边沿。
//	多副本滚动发布时，ClientConn 的聚合状态只要还有一个 subconn 是 Ready 就是 Ready，
//	整个发布过程网关这边从未判定过不健康 —— onRecover 永不触发，下游这一版新增的 RPC
//	和新加的 google.api.http 注解，网关的动态路由表完全看不见。把 3s 改成 1s 对这个
//	盲区没有任何帮助，因为它不是"探测得不够勤"，而是"根本没有那条边沿"
//
// 所以这里走两条互补的路：
//
//	连接事件（watchConn）：阻塞在 ClientConn.WaitForStateChange 上，Ready 的上升沿
//	立即触发 reconcile。零轮询开销，覆盖 Pod 重启、GOAWAY、网络抖动后的重连
//
//	指纹兜底（worker 的 ticker）：低频拉一次 reflection，只比对契约指纹，未变则什么
//	都不做。覆盖上面那个"全程 Ready"的盲区，也兜住事件路径漏掉的任何情况
//
// 两条路最终都收口到 reconcileService，由它做指纹比对；所以"触发得过于频繁"的代价
// 上限就是一次 reflection 往返，不会演变成重复注册
// [EN] Two complementary paths: a blocking WaitForStateChange watcher for connection rebuilds,
// and a low-frequency fingerprint poll for the blind spot where a multi-replica rolling deploy
// never leaves the Ready state. Both funnel into reconcileService, so an over-eager trigger
// costs one reflection round trip and never causes duplicate registration
//
// 注意能力边界：proto 文件内容变更（改字段、改类型）无法热加载，protoregistry.GlobalFiles
// 没有 Unregister。这种情况 watcher 能"发现"但不能"生效"，它会把服务标记为 stale
// （见 GetStaleServices），需要重启网关进程。能真正热生效的是新增 proto 文件与新增服务
// [EN] Changed proto files cannot be hot-reloaded (GlobalFiles has no Unregister); the watcher
// detects them and marks the service stale, but a restart is required. Newly added files and
// services do take effect at runtime
type ServiceWatcher struct {
	muxFn            func() *runtime.ServeMux
	services         []string
	interval         time.Duration
	minInterval      time.Duration
	reconcileTimeout time.Duration
	isHealthy        func(string) bool
	onChange         func(string, []string)

	// signals serviceName -> 触发通道，容量 1，多次触发自动合并
	signals map[string]chan struct{}

	ctx    context.Context
	cancel context.CancelFunc

	wg sync.WaitGroup

	// mu 保护 started/stopped，以及"检查 stopped 后再 wg.Add"这组操作
	//
	// Start 与 Stop 可能来自不同 goroutine（启动流程与关闭流程）。sync.WaitGroup
	// 不允许 Add 与 Wait 并发：Stop 已进入 Wait 之后 Start 再 Add 属于未定义行为，
	// 而且这些 goroutine 用的是已被取消的 ctx，也不会有人再收。用同一把锁把
	// "检查并置位" 与 "Add/取 cancel" 变成原子操作
	// [EN] Start and Stop may run on different goroutines, and WaitGroup forbids Add
	// concurrent with Wait. The lock makes the stopped check plus the Add (or cancel
	// hand-off) atomic, so no goroutine can be spawned after Stop starts waiting
	mu      sync.Mutex
	started bool
	stopped bool
}

// NewServiceWatcher 创建服务发版监视器
// opts.MuxFn 为 nil 或 opts.Services 为空时返回 nil，调用方可直接忽略
func NewServiceWatcher(opts ServiceWatchOptions) *ServiceWatcher {
	if opts.MuxFn == nil || len(opts.Services) == 0 {
		return nil
	}

	minInterval := opts.MinInterval
	if minInterval <= 0 {
		minInterval = DefaultWatchMinInterval
	}

	reconcileTimeout := opts.ReconcileTimeout
	if reconcileTimeout <= 0 {
		reconcileTimeout = DefaultWatchReconcileTimeout
	}

	signals := make(map[string]chan struct{}, len(opts.Services))
	for _, name := range opts.Services {
		signals[name] = make(chan struct{}, 1)
	}

	return &ServiceWatcher{
		muxFn:            opts.MuxFn,
		services:         opts.Services,
		interval:         opts.Interval,
		minInterval:      minInterval,
		reconcileTimeout: reconcileTimeout,
		isHealthy:        opts.IsHealthy,
		onChange:         opts.OnChange,
		signals:          signals,
	}
}

// Start 启动监视，可重复调用（只生效一次）
// 传入的 ctx 结束等同于 Stop；Stop 之后再调用 Start 不会启动任何 goroutine
func (w *ServiceWatcher) Start(ctx context.Context) {
	if w == nil {
		return
	}

	w.mu.Lock()
	if w.started || w.stopped {
		w.mu.Unlock()
		return
	}
	w.started = true

	if ctx == nil {
		ctx = context.Background()
	}
	w.ctx, w.cancel = context.WithCancel(ctx)

	for _, name := range w.services {
		w.wg.Add(2)
		go w.watchConn(name)
		go w.worker(name)
	}
	w.mu.Unlock()

	if w.interval > 0 {
		gwglobal.LOGGER.Info("👀 服务发版监视已启动 (%d 个服务，连接事件 + %v 指纹兜底)", len(w.services), w.interval)
	} else {
		gwglobal.LOGGER.Info("👀 服务发版监视已启动 (%d 个服务，仅连接事件)", len(w.services))
	}
}

// Stop 停止监视并等待所有 goroutine 退出，可重复调用
//
// 一定要等 wg：不等的话测试之间会残留 goroutine 继续探测，
// 这正是 HealthChecker.StopPeriodicCheck 当初修掉的那类问题
// [EN] Waits on the WaitGroup; not waiting leaves probing goroutines across test boundaries,
// the same class of problem StopPeriodicCheck was added to fix
func (w *ServiceWatcher) Stop() {
	if w == nil {
		return
	}

	w.mu.Lock()
	alreadyStopped := w.stopped
	wasStarted := w.started
	w.stopped = true
	cancel := w.cancel
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	w.wg.Wait()

	if wasStarted && !alreadyStopped {
		gwglobal.LOGGER.Info("👀 服务发版监视已停止")
	}
}

// Trigger 立即触发一次指定服务的 reconcile（异步、合并）
// 供外部的发布回调使用：想做到秒级且准确，发布侧主动调它比任何探测都强
// [EN] For a publish-side hook: an explicit call beats any amount of probing
func (w *ServiceWatcher) Trigger(serviceName string) {
	if w == nil {
		return
	}
	w.signal(serviceName)
}

// =============================================================================
// 内部实现
// =============================================================================

// signal 向某服务的 worker 投递一次触发，通道满（已有待处理触发）则直接返回
func (w *ServiceWatcher) signal(serviceName string) {
	ch, ok := w.signals[serviceName]
	if !ok {
		return
	}
	select {
	case ch <- struct{}{}:
	default: // 已有待处理的触发，合并掉
	}
}

// watchConn 阻塞式跟踪单个服务的连接状态，在 Ready 的上升沿触发 reconcile
//
// 初始的 wasReady 直接取当前状态，而不是从 false 起步：启动期连接通常已经 Ready，
// 从 false 起步会在启动瞬间凭空触发一次 reconcile
// [EN] wasReady starts from the current state rather than false: the connection is usually
// already Ready at startup, and starting from false would fire a reconcile for nothing
func (w *ServiceWatcher) watchConn(serviceName string) {
	defer w.wg.Done()

	var initialized bool
	var wasReady bool

	for {
		conn, ok := GetConn(serviceName)
		if !ok {
			// 连接还没进池（启动期竞态）：等一拍再看，不空转
			if !w.sleep(w.minInterval) {
				return
			}
			continue
		}

		state := conn.GetState()
		if !initialized {
			initialized = true
			wasReady = state == connectivity.Ready
		} else {
			switch {
			case state == connectivity.Ready && !wasReady:
				wasReady = true
				w.signal(serviceName)
			case state != connectivity.Ready && wasReady:
				wasReady = false
			}
		}

		// WaitForStateChange 阻塞到状态变化或 ctx 结束，不消耗 CPU
		if !conn.WaitForStateChange(w.ctx, state) {
			return
		}
	}
}

// worker 单个服务的 reconcile 循环
//
// 每个服务一个 worker，于是：同一服务的 reconcile 天然串行（无需额外的单飞），
// 不同服务之间并行（一个慢服务不拖累其他服务的收敛）
// [EN] One worker per service: reconciles of the same service are serialized without extra
// single-flight bookkeeping, while different services still converge in parallel
func (w *ServiceWatcher) worker(serviceName string) {
	defer w.wg.Done()

	var (
		last     time.Time
		failures int
	)

	for {
		var timer *time.Timer
		var tick <-chan time.Time
		if w.interval > 0 {
			timer = time.NewTimer(w.nextInterval(failures))
			tick = timer.C
		}

		var byEvent bool
		select {
		case <-w.ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-w.signals[serviceName]:
			byEvent = true
			if timer != nil {
				timer.Stop()
			}
		case <-tick:
		}

		// 合并抖动：距上次 reconcile 不足 minInterval 就等够再做，而不是丢弃
		if wait := w.minInterval - time.Since(last); wait > 0 && !last.IsZero() {
			if !w.sleep(wait) {
				return
			}
		}

		// 仅对兜底轮询做健康过滤，事件触发一律执行（理由见 ServiceWatchOptions.IsHealthy）
		if !byEvent && w.isHealthy != nil && !w.isHealthy(serviceName) {
			continue
		}

		last = time.Now()
		reconcileCtx, cancel := context.WithTimeout(w.ctx, w.reconcileTimeout)
		outcome, err := reconcileService(reconcileCtx, w.muxFn(), serviceName)
		cancel()
		if err != nil {
			if w.ctx.Err() != nil {
				return
			}
			if failures < watchMaxBackoffSteps {
				failures++
			}
			gwglobal.LOGGER.Warn("⚠️  服务 %s 契约比对失败（第 %d 次，将退避重试）: %v", serviceName, failures, err)
			continue
		}
		failures = 0

		if !outcome.Changed {
			continue
		}

		gwglobal.LOGGER.Info("🔄 服务 %s 契约变更已生效，注册 %d 个 HTTP 路由", serviceName, len(outcome.Routes))
		if len(outcome.Stale) > 0 {
			gwglobal.LOGGER.Error("⚠️  服务 %s 有 %d 个 proto 文件无法热加载，网关仍在使用旧契约，需重启进程: %v",
				serviceName, len(outcome.Stale), outcome.Stale)
		}
		if w.onChange != nil {
			w.onChange(serviceName, outcome.Routes)
		}
	}
}

// nextInterval 计算下一轮兜底轮询的等待时间：基础间隔 × 失败退避 × 抖动
func (w *ServiceWatcher) nextInterval(failures int) time.Duration {
	d := w.interval
	for i := 0; i < failures; i++ {
		d *= 2
	}
	jitter := 1 + (rand.Float64()*2-1)*watchJitterRatio
	d = time.Duration(float64(d) * jitter)
	if d < w.minInterval {
		d = w.minInterval
	}
	return d
}

// sleep 可被 ctx 打断的等待，返回 false 表示应退出
func (w *ServiceWatcher) sleep(d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-w.ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// =============================================================================
// 便捷构造
// =============================================================================

// WatchedServiceNames 从客户端配置中取出可监视的服务名
func WatchedServiceNames(clients map[string]*gwconfig.GRPCClient) []string {
	names := make([]string, 0, len(clients))
	for name, cfg := range clients {
		if cfg != nil && len(cfg.Endpoints) > 0 {
			names = append(names, name)
		}
	}
	return names
}
