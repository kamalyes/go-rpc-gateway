/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 15:38:25
 * @FilePath: \go-rpc-gateway\constants\middleware_pprof.go
 * @Description: 性能分析中间件相关常量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

// PProf 路径常量
const (
	PProfBasePath = "/debug/pprof"
)

// PProf 默认配置
const (
	// 默认是否启用
	PProfDefaultEnabled = false

	// 默认基础路径
	PProfDefaultBasePath = PProfBasePath

	// 默认认证token
	PProfDefaultAuthToken = "gateway-pprof-2024"

	// 默认CPU采样持续时间（秒）
	PProfDefaultCPUProfileDuration = 30

	// 默认内存采样率
	PProfDefaultMemProfileRate = 1

	// 默认阻塞采样率
	PProfDefaultBlockProfileRate = 1

	// 默认互斥锁采样比例
	PProfDefaultMutexProfileFraction = 1
)

// PProf 安全配置
const (
	// 开发环境
	PProfModeProduction  = "production"
	PProfModeDevelopment = "development"
	PProfModeDebug       = "debug"

	// 访问控制级别
	PProfAccessLevelPublic     = "public"     // 公开访问
	PProfAccessLevelInternal   = "internal"   // 内网访问
	PProfAccessLevelRestricted = "restricted" // 受限访问
)

// PProf 采样类型
const (
	PProfSampleTypeCPU       = "cpu"
	PProfSampleTypeHeap      = "heap"
	PProfSampleTypeGoroutine = "goroutine"
	PProfSampleTypeAllocs    = "allocs"
	PProfSampleTypeBlock     = "block"
	PProfSampleTypeMutex     = "mutex"
	PProfSampleTypeThreads   = "threadcreate"
)

// PProf 输出格式
const (
	PProfFormatText  = "text"
	PProfFormatSVG   = "svg"
	PProfFormatPDF   = "pdf"
	PProfFormatPNG   = "png"
	PProfFormatRaw   = "raw"
	PProfFormatProto = "proto"
)

// 压力测试场景
const (
	PProfScenarioBasic      = "basic"
	PProfScenarioCPU        = "cpu_intensive"
	PProfScenarioMemory     = "memory_intensive"
	PProfScenarioGoroutine  = "goroutine_leak"
	PProfScenarioAllocation = "allocation_heavy"
	PProfScenarioGC         = "gc_pressure"
)

// 默认压力测试配置
const (
	PProfScenarioDefaultIterations = 1000000
	PProfScenarioDefaultWorkers    = 4
	PProfScenarioDefaultDuration   = 30 // 秒
)

// 白名单IP（允许访问pprof的IP）
var PProfDefaultWhitelistIPs = []string{
	"127.0.0.1",
	"::1",
	"localhost",
	"10.0.0.0/8",     // 私有网络A类
	"172.16.0.0/12",  // 私有网络B类
	"192.168.0.0/16", // 私有网络C类
}
