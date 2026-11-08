/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-07 00:20:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-07 00:56:18
 * @FilePath: \go-rpc-gateway\middleware\observability\health_manager_test.go
 * @Description: HealthManager 注册与检查的并发安全
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package observability

import (
	"context"
	"sync"
	"testing"
	"time"
)

// stubHealthChecker 固定返回 ok 的检查器，可带一点延迟模拟真实探测
type stubHealthChecker struct {
	name  string
	delay time.Duration
}

func (s *stubHealthChecker) Name() string { return s.name }

func (s *stubHealthChecker) Check(ctx context.Context) HealthStatus {
	if s.delay > 0 {
		select {
		case <-ctx.Done():
		case <-time.After(s.delay):
		}
	}
	return HealthStatus{Status: "ok", CheckedAt: time.Now()}
}

// TestHealthManagerConcurrentRegisterAndCheck 注册与检查并发不应触发数据竞争
//
// 回归点：RegisterChecker 直接 append，Check 同时 range 同一个 slice。
// 健康端点会被负载均衡器并发探测，而插件式注册也可能在运行期发生，
// 两者重叠时原实现是裸的 slice 并发读写
// [EN] Regression: RegisterChecker appended without a lock while Check ranged the same slice.
// The health endpoint is probed concurrently by load balancers and registration may happen at
// runtime in plugin-style setups, so the original code was an unsynchronized slice race
func TestHealthManagerConcurrentRegisterAndCheck(t *testing.T) {
	hm := NewHealthManager()

	const (
		writers = 16
		readers = 8
		rounds  = 50
	)

	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			<-start
			for j := 0; j < rounds; j++ {
				hm.RegisterChecker(&stubHealthChecker{name: "writer-checker"})
			}
		}(i)
	}

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < rounds; j++ {
				result := hm.Check(context.Background(), true)
				if result.Status != "ok" {
					t.Errorf("状态应为 ok，实际 %q", result.Status)
					return
				}
			}
		}()
	}

	close(start)
	wg.Wait()
}

// TestHealthManagerIgnoresNilChecker nil checker 不应注册，也不应让 Check 崩溃
func TestHealthManagerIgnoresNilChecker(t *testing.T) {
	hm := NewHealthManager()
	hm.RegisterChecker(nil)

	result := hm.Check(context.Background(), true)
	if len(result.Checks) != 0 {
		t.Errorf("nil checker 不应进入检查结果，实际 %d 项", len(result.Checks))
	}
}

// TestHealthManagerCheckDetailedEmpty 未注册检查器时 detailed 检查应返回 ok 且不阻塞
func TestHealthManagerCheckDetailedEmpty(t *testing.T) {
	hm := NewHealthManager()

	result := hm.Check(context.Background(), true)
	if result.Status != "ok" {
		t.Errorf("无检查器时应为 ok，实际 %q", result.Status)
	}
	if len(result.Checks) != 0 {
		t.Errorf("不应有检查项，实际 %d 项", len(result.Checks))
	}
}
