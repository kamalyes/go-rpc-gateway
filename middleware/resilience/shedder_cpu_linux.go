//go:build linux

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-09-21 17:35:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-09-23 21:28:51
 * @FilePath: \go-rpc-gateway\middleware\resilience\shedder_cpu_linux.go
 * @Description: Linux CPU 使用率读取 - cgroup v2 → cgroup v1 → /proc/stat 三级回落
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package resilience

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

// cgroup 与 proc 文件路径
//
// 容器内开启了 cgroup namespace（Docker/containerd/k8s 默认），进程自己的 cgroup 就挂在
// /sys/fs/cgroup 根上，因此直接读固定路径即可，不需要先解析 /proc/self/cgroup
//
// [EN] cgroup and proc paths. With a cgroup namespace in place — the default for Docker,
// containerd and Kubernetes — the process's own cgroup is mounted at the /sys/fs/cgroup
// root, so these fixed paths can be read directly without first parsing /proc/self/cgroup
const (
	cgroupV2CPUStat     = "/sys/fs/cgroup/cpu.stat"
	cgroupV2CPUMax      = "/sys/fs/cgroup/cpu.max"
	cgroupV1CPUAcctA    = "/sys/fs/cgroup/cpuacct/cpuacct.usage"
	cgroupV1CPUAcctB    = "/sys/fs/cgroup/cpu,cpuacct/cpuacct.usage"
	cgroupV1QuotaA      = "/sys/fs/cgroup/cpu/cpu.cfs_quota_us"
	cgroupV1PeriodA     = "/sys/fs/cgroup/cpu/cpu.cfs_period_us"
	cgroupV1QuotaB      = "/sys/fs/cgroup/cpu,cpuacct/cpu.cfs_quota_us"
	cgroupV1PeriodB     = "/sys/fs/cgroup/cpu,cpuacct/cpu.cfs_period_us"
	procStatPath        = "/proc/stat"
	procStatJiffyNanos  = 10_000_000 // USER_HZ 固定为 100，一个 jiffy = 10ms
	microsToNanosFactor = 1000
)

// newCPUStatReader 构造 Linux 下的 CPU 时间读取器
//
// 三级回落，顺序即优先级：
//  1. cgroup v2 —— 容器内最准确：usage_usec 只统计本 cgroup，cpu.max 给出真实配额
//  2. cgroup v1 —— 老内核/老 runtime 上的等价物（cpuacct.usage + cfs_quota/cfs_period）
//  3. /proc/stat —— 裸机或没挂 cgroup 文件系统时的兜底，统计的是整机使用率
//
// 前两级拿到的是「本进程组消耗的 CPU 时间 ÷ 本进程组的配额」，这正是降载需要的信号：
// 同宿主机上别的容器把宿主机 CPU 打满，不该让本服务开始丢自己的请求
// 第三级退化为整机使用率，属于明确的精度损失，日志里用 source 标明以便排查
//
// [EN] Builds the Linux CPU time reader with a three-level fallback, in priority order:
// cgroup v2 (most accurate inside a container — usage_usec counts only this cgroup and
// cpu.max gives the real quota), cgroup v1 (the equivalent on older kernels), and
// /proc/stat (bare metal, or no cgroup filesystem mounted). The first two levels yield
// "CPU time this process group consumed, over this process group's quota", which is exactly
// the signal shedding needs: another container saturating the host must not make this
// service start dropping its own requests. The third level degrades to host-wide usage,
// a deliberate loss of precision that the source label makes visible in the logs
func newCPUStatReader() (cpuStatReader, string, bool) {
	// cgroup v2
	if _, err := os.Stat(cgroupV2CPUStat); err == nil {
		reader := func() (int64, float64, error) {
			used, err := readCgroupV2Usage()
			if err != nil {
				return 0, 0, err
			}
			return used, readCgroupV2Cores(), nil
		}
		if _, _, err := reader(); err == nil {
			return reader, cpuSourceCgroupV2, true
		}
	}

	// cgroup v1：两种常见挂载布局
	for _, layout := range [][3]string{
		{cgroupV1CPUAcctA, cgroupV1QuotaA, cgroupV1PeriodA},
		{cgroupV1CPUAcctB, cgroupV1QuotaB, cgroupV1PeriodB},
	} {
		usagePath, quotaPath, periodPath := layout[0], layout[1], layout[2]
		if _, err := os.Stat(usagePath); err != nil {
			continue
		}
		reader := func() (int64, float64, error) {
			used, err := readInt64File(usagePath)
			if err != nil {
				return 0, 0, err
			}
			return used, readCgroupV1Cores(quotaPath, periodPath), nil
		}
		if _, _, err := reader(); err == nil {
			return reader, cpuSourceCgroupV1, true
		}
	}

	// /proc/stat 兜底
	if _, _, err := readProcStatUsage(); err == nil {
		return readProcStatUsage, cpuSourceProcStat, true
	}

	return nil, cpuSourceUnsupported, false
}

// readCgroupV2Usage 读取 cgroup v2 的累计 CPU 占用时间
// cpu.stat 形如：usage_usec 123456 / user_usec ... / system_usec ...
//
// [EN] Reads the cumulative CPU time from cgroup v2's cpu.stat
func readCgroupV2Usage() (int64, error) {
	data, err := os.ReadFile(cgroupV2CPUStat)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		rest, ok := strings.CutPrefix(line, "usage_usec")
		if !ok {
			continue
		}
		micros, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
		if err != nil {
			return 0, err
		}
		return micros * microsToNanosFactor, nil
	}
	return 0, os.ErrNotExist
}

// readCgroupV2Cores 读取 cgroup v2 的 CPU 配额核数
// cpu.max 形如 "max 100000"（无配额）或 "200000 100000"（配额 2 核）
//
// [EN] Reads the CPU quota from cgroup v2's cpu.max, expressed in cores
func readCgroupV2Cores() float64 {
	data, err := os.ReadFile(cgroupV2CPUMax)
	if err != nil {
		return fallbackCores()
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 || fields[0] == "max" {
		return fallbackCores()
	}
	quota, err1 := strconv.ParseFloat(fields[0], 64)
	period, err2 := strconv.ParseFloat(fields[1], 64)
	if err1 != nil || err2 != nil || quota <= 0 || period <= 0 {
		return fallbackCores()
	}
	return quota / period
}

// readCgroupV1Cores 读取 cgroup v1 的 CPU 配额核数
// quota 为 -1 表示未设置配额
//
// [EN] Reads the CPU quota from cgroup v1; a quota of -1 means unlimited
func readCgroupV1Cores(quotaPath, periodPath string) float64 {
	quota, err := readInt64File(quotaPath)
	if err != nil || quota <= 0 {
		return fallbackCores()
	}
	period, err := readInt64File(periodPath)
	if err != nil || period <= 0 {
		return fallbackCores()
	}
	return float64(quota) / float64(period)
}

// readProcStatUsage 从 /proc/stat 读取整机繁忙时间
//
// 首行形如：cpu  user nice system idle iowait irq softirq steal guest guest_nice
// 繁忙时间 = 总时间 - idle - iowait（iowait 期间 CPU 其实是空闲的，计入会高估使用率）
// 单位是 jiffy，按 USER_HZ=100 换算成纳秒；这个取值在所有主流发行版内核上都成立
//
// [EN] Reads host-wide busy time from /proc/stat. Busy equals total minus idle minus iowait,
// since the CPU is in fact idle while waiting on IO and counting it would overstate usage.
// Jiffies are converted assuming USER_HZ is 100, which holds on every mainstream kernel
func readProcStatUsage() (int64, float64, error) {
	data, err := os.ReadFile(procStatPath)
	if err != nil {
		return 0, 0, err
	}
	line, _, _ := strings.Cut(string(data), "\n")
	fields := strings.Fields(line)
	// 至少需要 cpu + user/nice/system/idle/iowait 五个数值字段
	if len(fields) < 6 || fields[0] != "cpu" {
		return 0, 0, os.ErrInvalid
	}

	var total, idle int64
	for i, field := range fields[1:] {
		value, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			return 0, 0, err
		}
		total += value
		// 下标 3 为 idle，4 为 iowait
		if i == 3 || i == 4 {
			idle += value
		}
	}
	return (total - idle) * procStatJiffyNanos, fallbackCores(), nil
}

// readInt64File 读取只含一个整数的文件
//
// [EN] Reads a file containing a single integer
func readInt64File(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
}

// fallbackCores 无配额时的核数
//
// runtime.NumCPU 在 Linux 上取的是 sched_getaffinity 的结果，已经反映了 cpuset 限制，
// 因此不需要再解析 cpuset.cpus
//
// [EN] Core count when no quota is set. On Linux runtime.NumCPU reflects sched_getaffinity
// and therefore already accounts for cpuset restrictions, so cpuset.cpus needs no parsing
func fallbackCores() float64 {
	n := runtime.NumCPU()
	if n <= 0 {
		return 1
	}
	return float64(n)
}
