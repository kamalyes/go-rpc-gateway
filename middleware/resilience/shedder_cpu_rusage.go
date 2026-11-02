//go:build darwin || freebsd || netbsd || openbsd || dragonfly

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-09-21 18:50:17
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-09-23 21:30:05
 * @FilePath: \go-rpc-gateway\middleware\resilience\shedder_cpu_rusage.go
 * @Description: darwin/BSD CPU 使用率读取 - 退化为进程自身 rusage，无 cgroup 可读
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package resilience

import (
	"runtime"
	"syscall"
)

// newCPUStatReader 构造 darwin/BSD 下的 CPU 时间读取器
//
// 这些平台没有 cgroup，也没有纯 Go 能读到的整机 CPU 时间：
// darwin 的整机数据要走 Mach 的 host_processor_info（需要 cgo），
// 读不到就得引三方库，与「零新增三方依赖」冲突
// 因此这里退化为 getrusage(RUSAGE_SELF)：分子换成「本进程消耗的 CPU 时间」，
// 分母取 runtime.NumCPU()，得到的是本进程占满整机的程度
//
// 这个口径与 Linux 侧并不等价但方向一致：
//   - Linux cgroup：本 cgroup 消耗 ÷ 本 cgroup 配额
//   - 本实现：      本进程消耗 ÷ 本机核数
//
// 单进程容器里两者几乎重合，而在开发机上它的好处是不会被别的程序（编译、浏览器）
// 带跑偏——网关自己没吃 CPU 就不该降载
// 跨平台行为的代价写在 docs/SHEDDER.md，不在这里靠注释掩盖
//
// [EN] Builds the CPU time reader for darwin and the BSDs. These platforms have no cgroups
// and no host-wide CPU time reachable from pure Go — darwin needs Mach's host_processor_info
// through cgo — so the reader degrades to getrusage(RUSAGE_SELF): the numerator becomes the
// CPU time this process consumed and the denominator runtime.NumCPU(), yielding how much of
// the machine this process is occupying. That is not identical to the Linux cgroup ratio but
// points the same way, and the two nearly coincide in a single-process container. On a
// developer machine it has the added benefit of not being skewed by unrelated programs:
// if the gateway itself is not burning CPU, it should not shed
func newCPUStatReader() (cpuStatReader, string, bool) {
	reader := func() (int64, float64, error) {
		var usage syscall.Rusage
		if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
			return 0, 0, err
		}
		// Timeval.Nano 抹平了各 BSD 之间 Usec 字段宽度（int32/int64）的差异
		used := usage.Utime.Nano() + usage.Stime.Nano()
		cores := float64(runtime.NumCPU())
		if cores <= 0 {
			cores = 1
		}
		return used, cores, nil
	}
	if _, _, err := reader(); err != nil {
		return nil, cpuSourceUnsupported, false
	}
	return reader, cpuSourceRusage, true
}
