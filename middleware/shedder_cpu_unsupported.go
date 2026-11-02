//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-09-21 19:02:38
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-09-23 21:33:27
 * @FilePath: \go-rpc-gateway\middleware\shedder_cpu_unsupported.go
 * @Description: 其余平台（windows/js/plan9 等）无 CPU 使用率来源，显式降级
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

// newCPUStatReader 其余平台一律返回「不支持」
//
// windows 可以走 GetProcessTimes，但本仓库目前只在 linux 容器与 darwin 开发机上运行，
// 为一个没人部署的平台写一套 syscall 适配并不划算；需要时按 shedder_cpu_rusage.go 的
// 同一套接口补一个文件即可，降载器其余部分不用改
// 返回 false 后降载器只保留 max-concurrency 硬上限这一条平台无关的保护，
// 自适应判定整体静默——不会 panic，也不会因为读不到 CPU 就把使用率当成 0% 或 100%
//
// [EN] Every other platform reports "unsupported". Windows could use GetProcessTimes, but
// this repository only runs on Linux containers and darwin development machines, so writing
// a syscall adapter for a platform nobody deploys is not worth it; when it is needed, adding
// one file against the same interface as shedder_cpu_rusage.go is enough and the rest of the
// shedder stays untouched. With false returned, the shedder keeps only its platform-neutral
// max-concurrency ceiling and silences the adaptive path — it neither panics nor pretends
// the missing reading means 0% or 100%
func newCPUStatReader() (cpuStatReader, string, bool) {
	return nil, cpuSourceUnsupported, false
}
