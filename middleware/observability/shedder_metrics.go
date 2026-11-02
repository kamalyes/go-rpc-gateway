/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-09-25 18:27:11
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-09-28 19:05:23
 * @FilePath: \go-rpc-gateway\middleware\observability\shedder_metrics.go
 * @Description: 降载指标采集器 - 按需在 scrape 时读取降载器快照
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

// ShedderStat 降载统计信息
//
// 与 BreakerStat 对应，但没有 Path 标签：降载器是进程唯一实例（见 ShedderManager 的说明），
// 加一个恒为同一值的标签只会凭空抬高指标基数
//
// [EN] Load-shedding statistics, the counterpart of BreakerStat. There is no path label
// because the shedder is a single process-wide instance (see ShedderManager); a label whose
// value never varies would only inflate metric cardinality
type ShedderStat struct {
	Shedding        bool
	CPUUsage        int64
	CPUSupported    bool
	CPUThreshold    int64
	InFlight        int64
	AvgFlying       float64
	MaxFlight       float64
	MaxPass         int64
	MinRTMicros     int64
	MaxConcurrency  int64
	TotalRequests   int64
	DroppedRequests int64
}

// shedderCollector 降载指标采集器
//
// 采集函数在 Prometheus scrape 时按需调用，不起后台轮询协程，与 breakerCollector 一致
//
// [EN] Load-shedding collector. The stats function is invoked on demand when Prometheus
// scrapes, with no background polling goroutine, exactly as breakerCollector does
type shedderCollector struct {
	statFn func() *ShedderStat

	sheddingDesc       *prometheus.Desc
	cpuUsageDesc       *prometheus.Desc
	cpuSupportedDesc   *prometheus.Desc
	cpuThresholdDesc   *prometheus.Desc
	inFlightDesc       *prometheus.Desc
	avgFlyingDesc      *prometheus.Desc
	maxFlightDesc      *prometheus.Desc
	maxPassDesc        *prometheus.Desc
	minRTDesc          *prometheus.Desc
	maxConcurrencyDesc *prometheus.Desc
	totalDesc          *prometheus.Desc
	droppedDesc        *prometheus.Desc
}

// newShedderCollector 创建降载指标采集器
//
// [EN] Creates the load-shedding collector
func newShedderCollector(statFn func() *ShedderStat) *shedderCollector {
	return &shedderCollector{
		statFn: statFn,
		sheddingDesc: prometheus.NewDesc(
			"gateway_shedder_shedding",
			"降载器是否处于降载状态（1=正在降载, 0=正常）",
			nil, nil,
		),
		cpuUsageDesc: prometheus.NewDesc(
			"gateway_shedder_cpu_permille",
			"降载器观测到的 CPU 使用率（千分比 0-1000）",
			nil, nil,
		),
		cpuSupportedDesc: prometheus.NewDesc(
			"gateway_shedder_cpu_supported",
			"当前平台是否支持 CPU 采样（0 表示自适应判定静默，仅硬并发上限生效）",
			nil, nil,
		),
		cpuThresholdDesc: prometheus.NewDesc(
			"gateway_shedder_cpu_threshold_permille",
			"降载的 CPU 使用率阈值（千分比 0-1000）",
			nil, nil,
		),
		inFlightDesc: prometheus.NewDesc(
			"gateway_shedder_in_flight",
			"当前飞行请求数（未完成的一元请求）",
			nil, nil,
		),
		avgFlyingDesc: prometheus.NewDesc(
			"gateway_shedder_avg_flying",
			"飞行请求数的指数移动平均",
			nil, nil,
		),
		maxFlightDesc: prometheus.NewDesc(
			"gateway_shedder_max_flight",
			"滑窗估出的承载上限（最大吞吐 × 最小平均响应时间）",
			nil, nil,
		),
		maxPassDesc: prometheus.NewDesc(
			"gateway_shedder_window_max_pass",
			"响应时间滑窗内单桶最大通过量",
			nil, nil,
		),
		minRTDesc: prometheus.NewDesc(
			"gateway_shedder_window_min_rt_microseconds",
			"响应时间滑窗内最小平均响应时间（微秒, 0 表示无样本）",
			nil, nil,
		),
		maxConcurrencyDesc: prometheus.NewDesc(
			"gateway_shedder_max_concurrency",
			"配置的硬并发上限（0 表示不限）",
			nil, nil,
		),
		totalDesc: prometheus.NewDesc(
			"gateway_shedder_requests_total",
			"降载器累计判定的请求数",
			nil, nil,
		),
		droppedDesc: prometheus.NewDesc(
			"gateway_shedder_dropped_total",
			"降载器累计丢弃的请求数",
			nil, nil,
		),
	}
}

// Describe 实现 prometheus.Collector
//
// [EN] Implements prometheus.Collector
func (c *shedderCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.sheddingDesc
	ch <- c.cpuUsageDesc
	ch <- c.cpuSupportedDesc
	ch <- c.cpuThresholdDesc
	ch <- c.inFlightDesc
	ch <- c.avgFlyingDesc
	ch <- c.maxFlightDesc
	ch <- c.maxPassDesc
	ch <- c.minRTDesc
	ch <- c.maxConcurrencyDesc
	ch <- c.totalDesc
	ch <- c.droppedDesc
}

// Collect 实现 prometheus.Collector
//
// [EN] Implements prometheus.Collector
func (c *shedderCollector) Collect(ch chan<- prometheus.Metric) {
	if c.statFn == nil {
		return
	}
	stat := c.statFn()
	if stat == nil {
		return
	}

	gauge := func(desc *prometheus.Desc, value float64) {
		if m, err := prometheus.NewConstMetric(desc, prometheus.GaugeValue, value); err == nil {
			ch <- m
		}
	}
	counter := func(desc *prometheus.Desc, value float64) {
		if m, err := prometheus.NewConstMetric(desc, prometheus.CounterValue, value); err == nil {
			ch <- m
		}
	}

	gauge(c.sheddingDesc, boolToFloat(stat.Shedding))
	gauge(c.cpuSupportedDesc, boolToFloat(stat.CPUSupported))
	// CPU 采样不可用时不导出使用率：导出一个 0 会被读成「CPU 空闲」，
	// 而真实含义是「这个平台没有这个信号」，后者由 cpu_supported 单独表达
	if stat.CPUSupported {
		gauge(c.cpuUsageDesc, float64(stat.CPUUsage))
	}
	gauge(c.cpuThresholdDesc, float64(stat.CPUThreshold))
	gauge(c.inFlightDesc, float64(stat.InFlight))
	gauge(c.avgFlyingDesc, stat.AvgFlying)
	gauge(c.maxFlightDesc, stat.MaxFlight)
	gauge(c.maxPassDesc, float64(stat.MaxPass))
	gauge(c.minRTDesc, float64(stat.MinRTMicros))
	gauge(c.maxConcurrencyDesc, float64(stat.MaxConcurrency))
	counter(c.totalDesc, float64(stat.TotalRequests))
	counter(c.droppedDesc, float64(stat.DroppedRequests))
}

// boolToFloat 布尔值转指标值
//
// [EN] Converts a boolean into a metric value
func boolToFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

// RegisterShedderCollector 注册降载指标采集器
//
// 与 SetBreakerStatsFn 这类「注入采集函数」的写法略有不同：降载指标是自带注册入口的
// Collector，由调用方在组件就绪后注册一次，不在 MetricsManager 上常驻一个字段——
// 未启用降载时注册表里连指标名都不会出现，而不是导出一组恒为 0 的时间序列
// 重复注册只会返回 prometheus.AlreadyRegisteredError，不 panic，便于热加载场景安全重入
//
// [EN] Registers the load-shedding collector. This differs slightly from the "inject a stats
// function" style of SetBreakerStatsFn: the shedder's metrics come as a self-registering
// collector that the caller registers once the component is ready, instead of occupying a
// permanent field on MetricsManager — when shedding is disabled the metric names never
// appear in the registry at all, rather than exporting a set of series pinned at zero.
// Registering twice returns prometheus.AlreadyRegisteredError instead of panicking, so a
// hot-reload can safely re-enter
func (mm *MetricsManager) RegisterShedderCollector(statFn func() *ShedderStat) error {
	if mm == nil || mm.registry == nil || statFn == nil {
		return nil
	}
	return mm.registry.Register(newShedderCollector(statFn))
}
