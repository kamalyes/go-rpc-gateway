/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-09-19 17:05:08
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-09-22 19:30:27
 * @FilePath: \go-rpc-gateway\middleware\resilience\shedder_window.go
 * @Description: 降载响应时间滑窗 - 固定内存环形桶，读写全原子无锁
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package resilience

import (
	"sync/atomic"
	"time"
)

// shedderBucket 滑窗桶 - 一个时间片内的通过量与响应时间累加
//
// epoch 是桶当前归属的时间片序号，用它替代「定时清零」：
// 桶被复用到新时间片时由第一个写入者 CAS 抢到清零权，省掉一个后台清理协程
//
// [EN] One sliding-window bucket, holding the pass count and accumulated response time of a
// single time slice. The epoch field records which slice the bucket currently belongs to and
// replaces timer-driven resets: the first writer to reach a recycled bucket wins a CAS and
// clears it, so no background sweeper goroutine is needed
type shedderBucket struct {
	epoch atomic.Int64 // 时间片序号，-1 表示从未写入
	pass  atomic.Int64 // 通过的请求数
	rtSum atomic.Int64 // 响应时间累加，微秒
}

// shedderWindow 响应时间滑窗 - 固定大小环形桶
//
// 内存固定为 buckets 个桶（默认 50 个，共 1.2KB 量级），不随流量增长，
// 也不保留任何按请求增长的切片：降载器自己必须是过载时最便宜的那部分代码
// 时间基准取构造时的 time.Time 并用 Sub 计算，走的是单调时钟读数，
// 不受 NTP 回拨影响，同时便于测试传入自定义 now
//
// [EN] Fixed-size ring of buckets for the response-time window. Memory is constant in the
// number of buckets (50 by default) and never grows with traffic — the shedder itself has to
// be the cheapest code in the process while the process is overloaded. Time is measured as a
// difference against the construction instant, which uses Go's monotonic reading and is
// therefore immune to wall-clock jumps, and also lets tests inject their own now
type shedderWindow struct {
	buckets  []shedderBucket
	interval time.Duration // 单桶时长
	start    time.Time     // 时间基准（带单调读数）
}

// newShedderWindow 创建滑窗
// window 为总长度，count 为桶数，单桶时长为 window/count
//
// [EN] Creates a sliding window spanning window across count buckets
func newShedderWindow(window time.Duration, count int, start time.Time) *shedderWindow {
	if count <= 0 {
		count = 1
	}
	interval := window / time.Duration(count)
	if interval <= 0 {
		interval = time.Millisecond
	}
	w := &shedderWindow{
		buckets:  make([]shedderBucket, count),
		interval: interval,
		start:    start,
	}
	// 初始化为 -1，与真实时间片序号（从 0 开始）区分开
	for i := range w.buckets {
		w.buckets[i].epoch.Store(-1)
	}
	return w
}

// epochAt 计算某一时刻所属的时间片序号
//
// [EN] Returns the time-slice index a given instant falls into
func (w *shedderWindow) epochAt(now time.Time) int64 {
	elapsed := now.Sub(w.start)
	if elapsed < 0 {
		return 0
	}
	return int64(elapsed / w.interval)
}

// record 记录一次通过的请求及其响应时间
//
// 无锁实现，桶轮转时存在一个已知且有界的误差：
// 抢到 CAS 的 goroutine 先置 epoch 再清零计数，如果另一个 goroutine 恰好在这两步之间
// 完成 Add，它的那一次样本会被清掉 最坏情况是每次桶轮转丢失少量样本（桶轮转
// 频率为每 interval 一次），对「最大通过量 / 最小平均 RT」这种统计量没有实际影响；
// 换来的是热路径上没有任何锁
//
// [EN] Records one passed request and its response time, lock-free. Bucket rotation carries
// a known, bounded error: the goroutine that wins the CAS stores the new epoch before
// zeroing the counters, so a sample added in between is lost. At worst a handful of samples
// are dropped once per bucket interval, which does not matter for statistics as coarse as
// "max pass" and "min average RT", and in exchange the hot path takes no lock
func (w *shedderWindow) record(now time.Time, rt time.Duration) {
	epoch := w.epochAt(now)
	b := &w.buckets[epoch%int64(len(w.buckets))]

	if cur := b.epoch.Load(); cur != epoch {
		if b.epoch.CompareAndSwap(cur, epoch) {
			b.pass.Store(0)
			b.rtSum.Store(0)
		}
	}

	rtMicros := rt.Microseconds()
	if rtMicros < 0 {
		rtMicros = 0
	}
	b.pass.Add(1)
	b.rtSum.Add(rtMicros)
}

// shedderWindowStat 滑窗聚合结果
//
// [EN] Aggregated sliding-window statistics
type shedderWindowStat struct {
	MaxPass int64 // 单桶最大通过量
	MinRT   int64 // 各桶平均响应时间的最小值，微秒；0 表示窗口内没有样本
}

// stat 聚合滑窗
//
// 取「单桶最大通过量」与「单桶最小平均 RT」而不是整窗平均，是为了估出系统
// 在最好状态下的处理能力：这两个值相乘即为系统曾经稳定承载过的并发量（见 Shedder.maxFlight）
// 当前正在写入的桶被跳过——它只累积了一部分时间，通过量必然偏低，
// 直接参与会让容量估计被低估，进而误判过载
//
// [EN] Aggregates the window. It takes the maximum pass count of any single bucket and the
// minimum average RT of any single bucket, rather than window-wide averages, in order to
// estimate the system's capacity at its best: the product of the two is the concurrency the
// system has actually sustained (see Shedder.maxFlight). The bucket currently being written
// is skipped because it has only accumulated part of its interval, so counting it would
// understate throughput and fake an overload
func (w *shedderWindow) stat(now time.Time) shedderWindowStat {
	current := w.epochAt(now)
	oldest := current - int64(len(w.buckets)) + 1

	var st shedderWindowStat
	for i := range w.buckets {
		b := &w.buckets[i]
		epoch := b.epoch.Load()
		// 跳过未写入过的桶、当前未写满的桶、以及已经滚出窗口的桶
		if epoch < 0 || epoch >= current || epoch < oldest {
			continue
		}
		// 兜底：序号必须与桶下标对齐，防止极端时钟跳变造成的别名读取
		if epoch%int64(len(w.buckets)) != int64(i) {
			continue
		}
		pass := b.pass.Load()
		if pass <= 0 {
			continue
		}
		if pass > st.MaxPass {
			st.MaxPass = pass
		}
		avgRT := b.rtSum.Load() / pass
		if st.MinRT == 0 || avgRT < st.MinRT {
			st.MinRT = avgRT
		}
	}
	return st
}

// bucketsPerSecond 每秒桶数，用于把「单桶最大通过量」换算成 QPS
//
// [EN] Buckets per second, used to turn a per-bucket pass count into a QPS figure
func (w *shedderWindow) bucketsPerSecond() float64 {
	return float64(time.Second) / float64(w.interval)
}

// reset 清空滑窗（仅用于 Reset 与测试，不在热路径上）
//
// [EN] Clears the window; used by Reset and by tests, never on the hot path
func (w *shedderWindow) reset() {
	for i := range w.buckets {
		w.buckets[i].epoch.Store(-1)
		w.buckets[i].pass.Store(0)
		w.buckets[i].rtSum.Store(0)
	}
}
