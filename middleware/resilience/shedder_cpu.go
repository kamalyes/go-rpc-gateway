/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-09-20 18:10:33
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-09-23 21:25:19
 * @FilePath: \go-rpc-gateway\middleware\resilience\shedder_cpu.go
 * @Description: CPU 使用率采样 - 进程级单例采样器，平台相关实现见 shedder_cpu_*.go
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package resilience

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/kamalyes/go-rpc-gateway/global"
)

// CPU 采样参数
//
// 250ms 的采样间隔与 0.95 的 EWMA 系数配合，使使用率曲线的响应时间在 1s 量级：
// 更短会让单次 GC 或一次突发请求把使用率顶到阈值之上，更长则在真正过载时反应太慢
//
// [EN] Sampling parameters. A 250ms interval combined with a 0.95 EWMA factor gives the
// usage curve a response time on the order of a second: sampling faster lets a single GC
// cycle or burst push usage over the threshold, sampling slower reacts too late to a real
// overload
const (
	cpuSampleInterval = 250 * time.Millisecond
	cpuSampleBeta     = 0.95
)

// cpuStatReader 读取「累计消耗的 CPU 时间」与「可用核数」
//
// 两个返回值构成使用率的分子与分母：
//   - usedNanos 是单调递增的累计占用时间，采样器自己做差分
//   - cores 是本进程实际可用的核数（容器里是 CPU 配额，不是宿主机核数）
//
// 平台实现见 shedder_cpu_linux.go（cgroup v2 → cgroup v1 → /proc/stat）与
// shedder_cpu_rusage.go（darwin/BSD 走进程自身 rusage）
//
// [EN] Reads the cumulative CPU time consumed and the number of cores available. The two
// form the numerator and denominator of the usage ratio: usedNanos is a monotonically
// increasing total that the sampler differentiates itself, and cores is what this process
// may actually use — inside a container that is the CPU quota, not the host core count
type cpuStatReader func() (usedNanos int64, cores float64, err error)

// cpuSampler 进程级 CPU 使用率采样器
//
// 使用率是进程全局信号，因此采样器也是进程级单例：每个降载器各自起一个采样协程
// 只会重复读同一批 sysfs/proc 文件，还会让不同降载器看到不同的使用率
//
// [EN] Process-wide CPU usage sampler. Usage is a process-global signal, so the sampler is a
// process-wide singleton: giving every shedder its own sampling goroutine would re-read the
// same sysfs/proc files and let different shedders observe different usage values
type cpuSampler struct {
	usage     atomic.Int64 // 平滑后的使用率，千分比 0-1000
	supported bool         // 当前平台/环境是否支持采样
	source    string       // 数据来源，用于启动日志
}

var (
	cpuSamplerOnce sync.Once
	cpuSamplerInst *cpuSampler
)

// startCPUSampler 启动（或复用）进程级采样器
//
// 采样协程随进程生命周期存在，不提供停止接口：
// 它只做两次小文件读 + 一次原子写，代价远低于为它维护一套生命周期与重启逻辑；
// 网关热加载会重建降载器，但采样器复用同一个实例，不会越积越多
//
// [EN] Starts, or reuses, the process-wide sampler. The sampling goroutine lives for the
// life of the process and is deliberately not stoppable: it performs two small file reads
// and one atomic store per tick, which costs far less than maintaining a lifecycle for it.
// A gateway hot-reload rebuilds the shedder but reuses this very sampler, so goroutines
// never accumulate
func startCPUSampler() *cpuSampler {
	cpuSamplerOnce.Do(func() {
		sampler := &cpuSampler{}
		reader, source, ok := newCPUStatReader()
		if !ok {
			sampler.source = cpuSourceUnsupported
			cpuSamplerInst = sampler
			if global.LOGGER != nil {
				global.LOGGER.WithFields(map[string]interface{}{
					"source": cpuSourceUnsupported,
				}).WarnMsg("CPU usage sampling is unavailable on this platform, adaptive shedding will stay inactive (max-concurrency still applies)")
			}
			return
		}
		sampler.supported = true
		sampler.source = source
		cpuSamplerInst = sampler
		go sampler.loop(reader)
		if global.LOGGER != nil {
			global.LOGGER.WithFields(map[string]interface{}{
				"source":   source,
				"interval": cpuSampleInterval.String(),
			}).InfoMsg("CPU usage sampler started")
		}
	})
	return cpuSamplerInst
}

// loop 采样循环
//
// 每个周期读一次累计占用时间，与上一周期做差分后除以「经过时间 × 核数」得到使用率，
// 再做一次 EWMA 平滑写入原子变量；热路径只需要一次原子读
//
// [EN] Sampling loop. Each tick reads the cumulative CPU time, divides the delta by elapsed
// time times core count, smooths the result with an EWMA and stores it atomically; the hot
// path only ever performs one atomic load
func (s *cpuSampler) loop(reader cpuStatReader) {
	prevUsed, _, err := reader()
	prevAt := time.Now()
	if err != nil {
		prevUsed = 0
	}

	ticker := time.NewTicker(cpuSampleInterval)
	defer ticker.Stop()

	for range ticker.C {
		used, cores, err := reader()
		if err != nil || cores <= 0 {
			continue
		}
		now := time.Now()
		elapsed := now.Sub(prevAt)
		delta := used - prevUsed
		prevUsed, prevAt = used, now

		// delta < 0 说明计数器被重置（容器重建、配额热更新），本轮丢弃
		if elapsed <= 0 || delta < 0 {
			continue
		}

		permille := int64(float64(delta) * 1000 / (float64(elapsed) * cores))
		if permille < 0 {
			permille = 0
		}
		if permille > 1000 {
			permille = 1000
		}

		prev := s.usage.Load()
		// 首次采样直接采用，避免从 0 慢慢爬升导致过载初期判不出来
		if prev == 0 {
			s.usage.Store(permille)
			continue
		}
		s.usage.Store(int64(float64(prev)*cpuSampleBeta + float64(permille)*(1-cpuSampleBeta)))
	}
}

// CPUUsage 返回最近一次采样的 CPU 使用率（千分比 0-1000）与当前平台是否支持采样
//
// supported 为 false 时 usage 恒为 0，调用方必须按「没有 CPU 信号」处理而不是按「CPU 空闲」处理
// 首次调用会惰性启动采样协程，首个采样周期（250ms）内返回 0
//
// [EN] Returns the most recent CPU usage sample in per-mille and whether this platform
// supports sampling at all. When supported is false the value is always 0 and callers must
// treat it as "no CPU signal" rather than "CPU idle". The first call lazily starts the
// sampling goroutine, and 0 is returned during the first 250ms interval
func CPUUsage() (int64, bool) {
	sampler := startCPUSampler()
	if sampler == nil || !sampler.supported {
		return 0, false
	}
	return sampler.usage.Load(), true
}

// CPUUsageSource 返回 CPU 使用率的数据来源标识（cgroup2 / cgroup1 / procstat / rusage / unsupported）
//
// [EN] Returns an identifier for where CPU usage is read from
func CPUUsageSource() string {
	sampler := startCPUSampler()
	if sampler == nil {
		return cpuSourceUnsupported
	}
	return sampler.source
}

// CPU 数据来源标识
//
// [EN] CPU usage source identifiers
const (
	cpuSourceCgroupV2    = "cgroup2"
	cpuSourceCgroupV1    = "cgroup1"
	cpuSourceProcStat    = "procstat"
	cpuSourceRusage      = "rusage"
	cpuSourceUnsupported = "unsupported"
)
