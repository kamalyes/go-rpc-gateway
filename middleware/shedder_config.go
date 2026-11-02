/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-09-22 17:20:55
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-09-26 18:33:07
 * @FilePath: \go-rpc-gateway\middleware\shedder_config.go
 * @Description: 自适应降载配置适配 - 配置类型取自 go-config/pkg/shedder，本文件只补毫秒换算与非法值回落
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"time"

	shedderconfig "github.com/kamalyes/go-config/pkg/shedder"
)

// ============================================================================
// 降载配置的本地适配
// [EN] Local adapters for the shedding configuration
// ============================================================================
//
// 配置类型本身在 go-config 的 pkg/shedder（shedderconfig.Shedder），
// 与熔断配置下沉到 pkg/breaker 的惯例一致，YAML 侧对应 middleware.shedder 子树
// 本文件只提供两类 go-config 没有的东西：
//
//  1. 毫秒到 time.Duration 的换算（shedderWindowSpan / shedderCooldown / shedderMinRT）
//  2. normalizeShedderConfig：非法值回落默认
//
// 不把它们加进 go-config 的原因：这些约束（桶数要截到毫秒总数、beta 必须落在 (0,1)）
// 全部由本仓库的降载实现决定，配置库不该知道消费方的内部约束；
// 一旦加进去，别的使用方即使用别的算法也会被这套约束绑住
//
// [EN] The configuration type itself lives in go-config's pkg/shedder
// (shedderconfig.Shedder), mirroring how the breaker config sits in pkg/breaker, and is read
// from the middleware.shedder YAML subtree. This file only adds what go-config deliberately
// does not carry: the millisecond-to-Duration conversions, and the fallback of invalid values
// to defaults. Those constraints (buckets capped at the window's millisecond count, beta
// confined to (0,1)) follow from this repository's shedder implementation, and a
// configuration library has no business knowing a consumer's internal invariants — pushing
// them upstream would bind every other consumer to them as well

// normalizeShedderConfig 返回补齐默认值后的配置副本，入参为 nil 时返回一份默认配置
//
// 只在构造降载器时调用一次，不在热路径上；非法值一律回落默认而不是报错，
// 保证一份写错的配置不会让网关起不来——降载是保护手段，不该成为启动阻塞项
//
// 返回副本而不是就地改写入参：入参通常就是 cfg.Middleware.Shedder 本身，
// 就地改写会把截断后的桶数、回落后的阈值写回全局配置树，
// 让配置导出这类诊断手段看到的不再是使用方写下的那份配置
// 浅拷贝即可：ExcludePaths 只被读取，归一化不会改动切片内容
//
// [EN] Returns a normalized copy, or a fresh default when cfg is nil. Called once while the
// shedder is built, never on the hot path. Invalid values fall back instead of failing,
// because a typo in the shedder config must not keep the gateway from starting — shedding is
// a safety net, not a startup gate. A copy rather than an in-place fix: the argument is
// normally cfg.Middleware.Shedder itself, and writing clamped buckets or substituted
// thresholds back into the global config tree would make a config dump show something other
// than what the operator wrote. A shallow copy suffices, since ExcludePaths is only read
func normalizeShedderConfig(cfg *shedderconfig.Shedder) *shedderconfig.Shedder {
	def := shedderconfig.Default()
	if cfg == nil {
		return def
	}

	out := *cfg
	if out.ModuleName == "" {
		out.ModuleName = def.ModuleName
	}
	// CPU 阈值超出 (0,1000] 视为未配置：0 会让降载在空载时也触发，>1000 则永远不触发
	if out.CPUThreshold <= 0 || out.CPUThreshold > 1000 {
		out.CPUThreshold = def.CPUThreshold
	}
	if out.MaxConcurrency < 0 {
		out.MaxConcurrency = def.MaxConcurrency
	}
	if out.WindowMS <= 0 {
		out.WindowMS = def.WindowMS
	}
	if out.WindowBuckets <= 0 {
		out.WindowBuckets = def.WindowBuckets
	}
	// 桶数不能超过毫秒总数，否则每桶不足 1ms，桶序号会在同一毫秒内反复轮转
	if int64(out.WindowBuckets) > out.WindowMS {
		out.WindowBuckets = int(out.WindowMS)
	}
	if out.CooldownMS <= 0 {
		out.CooldownMS = def.CooldownMS
	}
	if out.MinRTMS <= 0 {
		out.MinRTMS = def.MinRTMS
	}
	// beta 必须落在 (0,1)：0 让 EWMA 退化成瞬时值，1 让它永远不更新
	if out.FlyingBeta <= 0 || out.FlyingBeta >= 1 {
		out.FlyingBeta = def.FlyingBeta
	}
	return &out
}

// shedderWindowSpan 返回滑窗总长度
//
// 三个换算函数都是包级函数而不是方法：shedderconfig.Shedder 是 go-config 的类型，
// 本包无法为它定义方法，也不该为了三行换算在配置库里加 API
//
// [EN] Returns the sliding window span. These three conversions are package functions rather
// than methods because shedderconfig.Shedder belongs to go-config and methods cannot be
// attached from here — nor should three lines of arithmetic become API in a config library
func shedderWindowSpan(cfg *shedderconfig.Shedder) time.Duration {
	return time.Duration(cfg.WindowMS) * time.Millisecond
}

// shedderCooldown 返回降载冷却时长
//
// [EN] Returns the shedding cool-off duration
func shedderCooldown(cfg *shedderconfig.Shedder) time.Duration {
	return time.Duration(cfg.CooldownMS) * time.Millisecond
}

// shedderMinRT 返回滑窗无样本时的兜底响应时间
//
// [EN] Returns the fallback response time used when the window holds no sample
func shedderMinRT(cfg *shedderconfig.Shedder) time.Duration {
	return time.Duration(cfg.MinRTMS) * time.Millisecond
}
