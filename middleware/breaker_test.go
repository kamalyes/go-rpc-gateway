/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-30 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-31 01:15:00
 * @FilePath: \go-rpc-gateway\middleware\breaker_test.go
 * @Description: 熔断器核心与管理器测试
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	breakerconfig "github.com/kamalyes/go-config/pkg/breaker"
)

// newTestBreakerManager 创建测试用熔断器管理器
// failureThreshold=2: 2 次失败触发熔断；volumeThreshold=0: 无最小请求量要求
func newTestBreakerManager() *BreakerManager {
	return NewBreakerManager(&breakerconfig.CircuitBreaker{
		Enabled:          true,
		FailureThreshold: 2,
		SuccessThreshold: 1,
		VolumeThreshold:  0,
		Timeout:          int64(time.Second),
		PreventionPaths:  []string{"/api"},
	})
}

func TestBreakerHTTPMiddleware_PathNotProtected(t *testing.T) {
	manager := newTestBreakerManager()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public", nil)

	BreakerHTTPMiddleware(manager)(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("未受保护路径应直通返回 200, got %d", rr.Code)
	}
	// 未受保护路径不应创建断路器
	if len(manager.GetAllBreakers()) != 0 {
		t.Errorf("未受保护路径不应创建断路器, got %d", len(manager.GetAllBreakers()))
	}
}

func TestBreakerHTTPMiddleware_SuccessRecorded(t *testing.T) {
	manager := newTestBreakerManager()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)

	BreakerHTTPMiddleware(manager)(handler).ServeHTTP(rr, req)

	b := manager.GetBreaker("/api/users")
	snap := b.Snapshot()
	if snap.TotalRequests != 1 {
		t.Errorf("成功请求后 TotalRequests 应为 1, got %d", snap.TotalRequests)
	}
	if snap.FailedRequests != 0 {
		t.Errorf("成功请求后 FailedRequests 应为 0, got %d", snap.FailedRequests)
	}
	if snap.State != BreakerClosed {
		t.Errorf("成功请求后状态应为 closed, got %s", snap.State)
	}
}

func TestBreakerHTTPMiddleware_FailureRecorded(t *testing.T) {
	manager := newTestBreakerManager()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)

	BreakerHTTPMiddleware(manager)(handler).ServeHTTP(rr, req)

	b := manager.GetBreaker("/api/users")
	snap := b.Snapshot()
	if snap.FailedRequests != 1 {
		t.Errorf("失败请求后 FailedRequests 应为 1, got %d", snap.FailedRequests)
	}
	// 1 次失败未达阈值，状态应仍为 closed
	if snap.State != BreakerClosed {
		t.Errorf("1 次失败未达阈值，状态应为 closed, got %s", snap.State)
	}
}

func TestBreakerHTTPMiddleware_BreakerOpen(t *testing.T) {
	manager := newTestBreakerManager() // failureThreshold=2
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mw := BreakerHTTPMiddleware(manager)

	// 发送 2 次失败请求触发熔断
	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
		mw(handler).ServeHTTP(rr, req)
	}

	// 验证熔断器已打开
	b := manager.GetBreaker("/api/users")
	if b.GetState() != BreakerOpen {
		t.Fatalf("2 次失败后熔断器应打开, got %s", b.GetState())
	}

	// 第 3 次请求应被熔断，返回 503 JSON
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	mw(handler).ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("熔断器打开后应返回 503, got %d", rr.Code)
	}
	// 验证返回 JSON 格式错误响应
	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Errorf("熔断响应应为 JSON 格式: %v", err)
	}
	if resp["code"] != float64(503) {
		t.Errorf("熔断响应 code 应为 503, got %v", resp["code"])
	}
}

func TestBreakerHTTPMiddleware_ExcludePath(t *testing.T) {
	manager := NewBreakerManager(&breakerconfig.CircuitBreaker{
		Enabled:          true,
		FailureThreshold: 2,
		SuccessThreshold: 1,
		VolumeThreshold:  0,
		Timeout:          int64(time.Second),
		PreventionPaths:  []string{"/api"},
		ExcludePaths:     []string{"/api/health"},
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	BreakerHTTPMiddleware(manager)(handler).ServeHTTP(rr, req)

	// 排除路径不应创建断路器
	if len(manager.GetAllBreakers()) != 0 {
		t.Errorf("排除路径不应创建断路器, got %d", len(manager.GetAllBreakers()))
	}
}

func TestBreakerSnapshot(t *testing.T) {
	b := NewBreaker(2, 1, 0, 1*time.Second)

	b.RecordFailure()
	b.RecordFailure()

	snap := b.Snapshot()
	if snap.State != BreakerOpen {
		t.Errorf("2 次失败后状态应为 open, got %s", snap.State)
	}
	if snap.TotalRequests != 2 {
		t.Errorf("TotalRequests 应为 2, got %d", snap.TotalRequests)
	}
	if snap.FailedRequests != 2 {
		t.Errorf("FailedRequests 应为 2, got %d", snap.FailedRequests)
	}
}

func TestBreakerManagerGetAllBreakerSnapshots(t *testing.T) {
	manager := newTestBreakerManager()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 访问两个不同路径，创建两个断路器
	for _, path := range []string{"/api/users", "/api/orders"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		BreakerHTTPMiddleware(manager)(handler).ServeHTTP(rr, req)
	}

	snapshots := manager.GetAllBreakerSnapshots()
	if len(snapshots) != 2 {
		t.Fatalf("应有 2 个断路器快照, got %d", len(snapshots))
	}
	for path, snap := range snapshots {
		if snap.TotalRequests != 1 {
			t.Errorf("路径 %s 的 TotalRequests 应为 1, got %d", path, snap.TotalRequests)
		}
		if snap.State != BreakerClosed {
			t.Errorf("路径 %s 的状态应为 closed, got %s", path, snap.State)
		}
	}
}
