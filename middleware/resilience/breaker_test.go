/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-30 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-31 01:15:00
 * @FilePath: \go-rpc-gateway\middleware\resilience\breaker_test.go
 * @Description: 熔断器核心、管理器与 HTTP/gRPC 适配层测试
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package resilience

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	breakerconfig "github.com/kamalyes/go-config/pkg/breaker"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// ============================================================================
// gRPC 拦截器测试
// [EN] gRPC interceptor tests
// ============================================================================

// testGRPCMethod 测试用 gRPC 全限定方法名
const testGRPCMethod = "/user.v1.UserService/GetUser"

// newTestGRPCBreakerManager 创建测试用熔断器管理器（保护路径按 gRPC 全限定方法前缀配置）
// failureThreshold=2: 2 次失败触发熔断；volumeThreshold=0: 无最小请求量要求
func newTestGRPCBreakerManager(timeout time.Duration) *BreakerManager {
	return NewBreakerManager(&breakerconfig.CircuitBreaker{
		Enabled:          true,
		FailureThreshold: 2,
		SuccessThreshold: 1,
		VolumeThreshold:  0,
		Timeout:          int64(timeout),
		PreventionPaths:  []string{"/user.v1.UserService/"},
	})
}

// grpcCodeHandler 构造返回指定 gRPC code 的 unary handler（codes.OK 时返回 nil 错误）
func grpcCodeHandler(code codes.Code, called *bool) grpc.UnaryHandler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		*called = true
		return "ok", status.Error(code, "handler error")
	}
}

// grpcStreamCodeHandler 构造返回指定 gRPC code 的 stream handler
func grpcStreamCodeHandler(code codes.Code, called *bool) grpc.StreamHandler {
	return func(srv interface{}, ss grpc.ServerStream) error {
		*called = true
		return status.Error(code, "handler error")
	}
}

func TestIsBreakerFailureCode(t *testing.T) {
	cases := []struct {
		code codes.Code
		want bool
		why  string
	}{
		{codes.OK, false, "正常响应"},
		{codes.Canceled, false, "调用方主动取消"},
		{codes.Unknown, true, "handler panic 或非 status 错误，对外是 500"},
		{codes.InvalidArgument, false, "客户端参数错误"},
		{codes.DeadlineExceeded, true, "下游超时"},
		{codes.NotFound, false, "资源不存在"},
		{codes.AlreadyExists, false, "资源已存在"},
		{codes.PermissionDenied, false, "鉴权失败"},
		{codes.ResourceExhausted, false, "限流/配额背压，计入会形成正反馈"},
		{codes.FailedPrecondition, false, "业务前置条件不满足"},
		{codes.Aborted, false, "并发冲突"},
		{codes.OutOfRange, false, "参数越界"},
		{codes.Unimplemented, false, "确定性契约错误，重试无用"},
		{codes.Internal, true, "下游内部错误"},
		{codes.Unavailable, true, "下游不可用"},
		{codes.DataLoss, true, "不可恢复的数据损坏"},
		{codes.Unauthenticated, false, "未认证"},
	}

	for _, c := range cases {
		if got := IsBreakerFailureCode(c.code); got != c.want {
			t.Errorf("IsBreakerFailureCode(%s) = %v, want %v (%s)", c.code, got, c.want, c.why)
		}
	}
}

func TestUnaryServerBreakerInterceptor_MethodNotProtected(t *testing.T) {
	manager := newTestGRPCBreakerManager(time.Second)
	interceptor := UnaryServerBreakerInterceptor(manager)

	called := false
	_, err := interceptor(context.Background(), nil, newGRPCUnaryInfo("/other.v1.OtherService/Ping"),
		grpcCodeHandler(codes.Internal, &called))

	if !called {
		t.Error("未受保护方法应直通到 handler")
	}
	if status.Code(err) != codes.Internal {
		t.Errorf("未受保护方法应原样返回 handler 错误, got %s", status.Code(err))
	}
	// 未受保护方法不应创建断路器
	if len(manager.GetAllBreakers()) != 0 {
		t.Errorf("未受保护方法不应创建断路器, got %d", len(manager.GetAllBreakers()))
	}
}

func TestUnaryServerBreakerInterceptor_NilManager(t *testing.T) {
	interceptor := UnaryServerBreakerInterceptor(nil)

	called := false
	_, err := interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod),
		grpcCodeHandler(codes.OK, &called))

	if !called {
		t.Error("manager 为 nil 时应直通到 handler")
	}
	if err != nil {
		t.Errorf("manager 为 nil 时不应返回错误, got %v", err)
	}
}

func TestUnaryServerBreakerInterceptor_SuccessRecorded(t *testing.T) {
	manager := newTestGRPCBreakerManager(time.Second)
	interceptor := UnaryServerBreakerInterceptor(manager)

	called := false
	resp, err := interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod),
		grpcCodeHandler(codes.OK, &called))

	if err != nil || !called {
		t.Fatalf("成功请求应直通, err=%v called=%v", err, called)
	}
	if resp != "ok" {
		t.Errorf("应原样透传 handler 返回值, got %v", resp)
	}

	// 熔断 key 必须是 FullMethod
	snap := manager.GetBreaker(testGRPCMethod).Snapshot()
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

func TestUnaryServerBreakerInterceptor_DownstreamErrorOpensBreaker(t *testing.T) {
	// 判定为下游故障的 code，各自独立计数到阈值 2 后应打开熔断器
	for _, code := range []codes.Code{codes.Unknown, codes.DeadlineExceeded, codes.Internal, codes.Unavailable, codes.DataLoss} {
		manager := newTestGRPCBreakerManager(time.Second)
		interceptor := UnaryServerBreakerInterceptor(manager)

		for i := 0; i < 2; i++ {
			called := false
			_, _ = interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod), grpcCodeHandler(code, &called))
			if !called {
				t.Fatalf("code=%s 第 %d 次调用 handler 应被执行", code, i+1)
			}
		}

		snap := manager.GetBreaker(testGRPCMethod).Snapshot()
		if snap.State != BreakerOpen {
			t.Errorf("code=%s 达到失败阈值后熔断器应打开, got %s", code, snap.State)
		}
		if snap.FailedRequests != 2 {
			t.Errorf("code=%s FailedRequests 应为 2, got %d", code, snap.FailedRequests)
		}
	}
}

func TestUnaryServerBreakerInterceptor_ClientErrorKeepsBreakerClosed(t *testing.T) {
	// 客户端自身问题不应把下游熔断掉
	clientCodes := []codes.Code{
		codes.Canceled,
		codes.InvalidArgument,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.ResourceExhausted,
		codes.FailedPrecondition,
		codes.Aborted,
		codes.OutOfRange,
		codes.Unimplemented,
		codes.Unauthenticated,
	}

	for _, code := range clientCodes {
		manager := newTestGRPCBreakerManager(time.Second)
		interceptor := UnaryServerBreakerInterceptor(manager)

		// 远超失败阈值的调用次数
		for i := 0; i < 5; i++ {
			called := false
			_, err := interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod), grpcCodeHandler(code, &called))
			if status.Code(err) != code {
				t.Fatalf("code=%s 应原样返回 handler 错误, got %s", code, status.Code(err))
			}
		}

		snap := manager.GetBreaker(testGRPCMethod).Snapshot()
		if snap.State != BreakerClosed {
			t.Errorf("code=%s 属客户端问题，熔断器应保持 closed, got %s", code, snap.State)
		}
		if snap.FailedRequests != 0 {
			t.Errorf("code=%s 不应计入失败, FailedRequests got %d", code, snap.FailedRequests)
		}
		if snap.TotalRequests != 5 {
			t.Errorf("code=%s TotalRequests 应为 5, got %d", code, snap.TotalRequests)
		}
	}
}

func TestUnaryServerBreakerInterceptor_RejectWithUnavailable(t *testing.T) {
	manager := newTestGRPCBreakerManager(time.Second)
	interceptor := UnaryServerBreakerInterceptor(manager)

	// 2 次 Internal 触发熔断
	for i := 0; i < 2; i++ {
		called := false
		_, _ = interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod), grpcCodeHandler(codes.Internal, &called))
	}
	if st := manager.GetBreaker(testGRPCMethod).GetState(); st != BreakerOpen {
		t.Fatalf("2 次失败后熔断器应打开, got %s", st)
	}

	// 第 3 次应被熔断拒绝
	called := false
	resp, err := interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod), grpcCodeHandler(codes.OK, &called))

	if called {
		t.Error("熔断器打开时 handler 不应被调用")
	}
	if resp != nil {
		t.Errorf("熔断拒绝时不应返回响应, got %v", resp)
	}
	if status.Code(err) != codes.Unavailable {
		t.Errorf("熔断拒绝应返回 codes.Unavailable（ErrCodeCircuitBreakerOpen 映射）, got %s", status.Code(err))
	}
	// 错误消息来自错误码体系的标准消息，不是裸 status 字面量
	wantMsg := errors.ErrorCodeString(errors.ErrCodeCircuitBreakerOpen)
	if got := status.Convert(err).Message(); got != wantMsg {
		t.Errorf("熔断拒绝消息应为 %q, got %q", wantMsg, got)
	}
	// 拒绝的请求不应计入断路器统计（未触达 Record*）
	if snap := manager.GetBreaker(testGRPCMethod).Snapshot(); snap.TotalRequests != 2 {
		t.Errorf("被拒绝的请求不应计入统计, TotalRequests got %d", snap.TotalRequests)
	}
}

func TestUnaryServerBreakerInterceptor_HalfOpenRecovery(t *testing.T) {
	manager := newTestGRPCBreakerManager(50 * time.Millisecond)
	interceptor := UnaryServerBreakerInterceptor(manager)

	for i := 0; i < 2; i++ {
		called := false
		_, _ = interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod), grpcCodeHandler(codes.Unavailable, &called))
	}
	if st := manager.GetBreaker(testGRPCMethod).GetState(); st != BreakerOpen {
		t.Fatalf("2 次失败后熔断器应打开, got %s", st)
	}

	// 超时后首个请求应被放行并转入半开，成功后（successThreshold=1）恢复闭合
	time.Sleep(80 * time.Millisecond)

	called := false
	_, err := interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod), grpcCodeHandler(codes.OK, &called))
	if err != nil || !called {
		t.Fatalf("超时后应放行探测请求, err=%v called=%v", err, called)
	}
	if st := manager.GetBreaker(testGRPCMethod).GetState(); st != BreakerClosed {
		t.Errorf("半开状态下探测成功后应恢复 closed, got %s", st)
	}
}

func TestUnaryServerBreakerInterceptor_PerMethodIsolation(t *testing.T) {
	manager := newTestGRPCBreakerManager(time.Second)
	interceptor := UnaryServerBreakerInterceptor(manager)
	const otherMethod = "/user.v1.UserService/ListUsers"

	// 只打垮 GetUser
	for i := 0; i < 2; i++ {
		called := false
		_, _ = interceptor(context.Background(), nil, newGRPCUnaryInfo(testGRPCMethod), grpcCodeHandler(codes.Internal, &called))
	}

	// 同服务的另一个方法不应受影响
	called := false
	_, err := interceptor(context.Background(), nil, newGRPCUnaryInfo(otherMethod), grpcCodeHandler(codes.OK, &called))
	if err != nil || !called {
		t.Fatalf("熔断按 FullMethod 隔离，另一方法应正常放行, err=%v called=%v", err, called)
	}

	snapshots := manager.GetAllBreakerSnapshots()
	if len(snapshots) != 2 {
		t.Fatalf("应有 2 个断路器快照, got %d", len(snapshots))
	}
	if snapshots[testGRPCMethod].State != BreakerOpen {
		t.Errorf("%s 应为 open, got %s", testGRPCMethod, snapshots[testGRPCMethod].State)
	}
	if snapshots[otherMethod].State != BreakerClosed {
		t.Errorf("%s 应为 closed, got %s", otherMethod, snapshots[otherMethod].State)
	}
}

func TestStreamServerBreakerInterceptor_MethodNotProtected(t *testing.T) {
	manager := newTestGRPCBreakerManager(time.Second)
	interceptor := StreamServerBreakerInterceptor(manager)

	// 拦截器只读 info.FullMethod，不触达 stream，传 nil 即可
	called := false
	err := interceptor(nil, nil, newGRPCStreamInfo("/other.v1.OtherService/Watch"),
		grpcStreamCodeHandler(codes.Internal, &called))

	if !called {
		t.Error("未受保护方法应直通到 handler")
	}
	if status.Code(err) != codes.Internal {
		t.Errorf("未受保护方法应原样返回 handler 错误, got %s", status.Code(err))
	}
	if len(manager.GetAllBreakers()) != 0 {
		t.Errorf("未受保护方法不应创建断路器, got %d", len(manager.GetAllBreakers()))
	}
}

func TestStreamServerBreakerInterceptor_NilManager(t *testing.T) {
	interceptor := StreamServerBreakerInterceptor(nil)

	called := false
	if err := interceptor(nil, nil, newGRPCStreamInfo(testGRPCMethod), grpcStreamCodeHandler(codes.OK, &called)); err != nil {
		t.Errorf("manager 为 nil 时不应返回错误, got %v", err)
	}
	if !called {
		t.Error("manager 为 nil 时应直通到 handler")
	}
}

func TestStreamServerBreakerInterceptor_SuccessRecorded(t *testing.T) {
	manager := newTestGRPCBreakerManager(time.Second)
	interceptor := StreamServerBreakerInterceptor(manager)

	called := false
	if err := interceptor(nil, nil, newGRPCStreamInfo(testGRPCMethod), grpcStreamCodeHandler(codes.OK, &called)); err != nil {
		t.Fatalf("成功的流不应返回错误, got %v", err)
	}

	// 计数粒度是「一条流」：建流时检查，流结束时记一次
	snap := manager.GetBreaker(testGRPCMethod).Snapshot()
	if snap.TotalRequests != 1 {
		t.Errorf("一条成功的流应记 1 次请求, got %d", snap.TotalRequests)
	}
	if snap.FailedRequests != 0 {
		t.Errorf("成功的流不应计入失败, got %d", snap.FailedRequests)
	}
}

func TestStreamServerBreakerInterceptor_FailureOpensBreakerAndRejects(t *testing.T) {
	manager := newTestGRPCBreakerManager(time.Second)
	interceptor := StreamServerBreakerInterceptor(manager)

	// 2 条失败的流触发熔断
	for i := 0; i < 2; i++ {
		called := false
		_ = interceptor(nil, nil, newGRPCStreamInfo(testGRPCMethod), grpcStreamCodeHandler(codes.Unavailable, &called))
		if !called {
			t.Fatalf("第 %d 条流的 handler 应被执行", i+1)
		}
	}
	if st := manager.GetBreaker(testGRPCMethod).GetState(); st != BreakerOpen {
		t.Fatalf("2 条失败流后熔断器应打开, got %s", st)
	}

	// 第 3 条流应在建流阶段就被拒绝
	called := false
	err := interceptor(nil, nil, newGRPCStreamInfo(testGRPCMethod), grpcStreamCodeHandler(codes.OK, &called))
	if called {
		t.Error("熔断器打开时 handler 不应被调用")
	}
	if status.Code(err) != codes.Unavailable {
		t.Errorf("熔断拒绝应返回 codes.Unavailable, got %s", status.Code(err))
	}
}

func TestStreamServerBreakerInterceptor_ClientErrorKeepsBreakerClosed(t *testing.T) {
	manager := newTestGRPCBreakerManager(time.Second)
	interceptor := StreamServerBreakerInterceptor(manager)

	for i := 0; i < 5; i++ {
		called := false
		_ = interceptor(nil, nil, newGRPCStreamInfo(testGRPCMethod), grpcStreamCodeHandler(codes.InvalidArgument, &called))
	}

	snap := manager.GetBreaker(testGRPCMethod).Snapshot()
	if snap.State != BreakerClosed {
		t.Errorf("InvalidArgument 属客户端问题，熔断器应保持 closed, got %s", snap.State)
	}
	if snap.FailedRequests != 0 {
		t.Errorf("InvalidArgument 不应计入失败, got %d", snap.FailedRequests)
	}
}

func TestBreakerExcludeMethod_GRPC(t *testing.T) {
	manager := NewBreakerManager(&breakerconfig.CircuitBreaker{
		Enabled:          true,
		FailureThreshold: 2,
		SuccessThreshold: 1,
		VolumeThreshold:  0,
		Timeout:          int64(time.Second),
		PreventionPaths:  []string{"/user.v1.UserService/"},
		ExcludePaths:     []string{"/user.v1.UserService/Health"},
	})
	interceptor := UnaryServerBreakerInterceptor(manager)

	called := false
	_, _ = interceptor(context.Background(), nil, newGRPCUnaryInfo("/user.v1.UserService/Health"),
		grpcCodeHandler(codes.Internal, &called))

	if !called {
		t.Error("排除方法应直通到 handler")
	}
	// 排除方法不应创建断路器
	if len(manager.GetAllBreakers()) != 0 {
		t.Errorf("排除方法不应创建断路器, got %d", len(manager.GetAllBreakers()))
	}
}
