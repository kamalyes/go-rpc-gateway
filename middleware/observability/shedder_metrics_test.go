/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-09-26 21:33:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-09-29 23:10:27
 * @FilePath: \go-rpc-gateway\middleware\observability\shedder_metrics_test.go
 * @Description: 降载指标采集器测试
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// ============================================================================
// 指标采集器
// ============================================================================

func TestShedderCollector_Collect(t *testing.T) {
	stat := &ShedderStat{
		Shedding: true, CPUUsage: 920, CPUSupported: true, CPUThreshold: 900,
		InFlight: 7, AvgFlying: 6.5, MaxFlight: 5, MaxPass: 20, MinRTMicros: 50000,
		MaxConcurrency: 128, TotalRequests: 100, DroppedRequests: 3,
	}
	c := newShedderCollector(func() *ShedderStat { return stat })

	descs := make(chan *prometheus.Desc, 32)
	c.Describe(descs)
	close(descs)
	descCount := len(descs)
	if descCount != 12 {
		t.Errorf("应描述 12 个指标, got %d", descCount)
	}

	metrics := make(chan prometheus.Metric, 32)
	c.Collect(metrics)
	close(metrics)
	if len(metrics) != 12 {
		t.Errorf("CPU 可采样时应导出 12 个指标, got %d", len(metrics))
	}
}

func TestShedderCollector_OmitsCPUWhenUnsupported(t *testing.T) {
	// 导出一个 0 会被读成「CPU 空闲」，而真实含义是「这个平台没有这个信号」
	c := newShedderCollector(func() *ShedderStat {
		return &ShedderStat{CPUSupported: false}
	})

	metrics := make(chan prometheus.Metric, 32)
	c.Collect(metrics)
	close(metrics)
	if len(metrics) != 11 {
		t.Errorf("CPU 不可采样时应少导出使用率指标, got %d", len(metrics))
	}
}

func TestShedderCollector_NilStatFn(t *testing.T) {
	c := newShedderCollector(nil)
	metrics := make(chan prometheus.Metric, 4)
	c.Collect(metrics)
	close(metrics)
	if len(metrics) != 0 {
		t.Errorf("采集函数为 nil 时不应导出指标, got %d", len(metrics))
	}

	var mm *MetricsManager
	if err := mm.RegisterShedderCollector(func() *ShedderStat { return nil }); err != nil {
		t.Errorf("nil MetricsManager 注册应安全返回, got %v", err)
	}
}
