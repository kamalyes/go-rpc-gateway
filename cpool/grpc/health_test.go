/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-08-06 18:15:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-08-06 18:15:00
 * @FilePath: \go-rpc-gateway\cpool\grpc\health_test.go
 * @Description: 健康检查器全量测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	gwglobal "github.com/kamalyes/go-rpc-gateway/global"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// =============================================================================
// 测试辅助
// =============================================================================

// startTestGRPCServer 启动带 reflection 的测试 gRPC 服务器，返回地址和停止函数
func startTestGRPCServer(t *testing.T) (addr string, stop func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	reflection.Register(server)
	go server.Serve(lis)
	return lis.Addr().String(), server.Stop
}

// startTestGRPCServerWithStreamInterceptor 启动带流拦截器的测试 gRPC 服务器
// 拦截器对所有流请求返回指定错误，用于模拟服务端拒绝 stream
func startTestGRPCServerWithStreamInterceptor(t *testing.T, rejectErr error) (addr string, stop func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.StreamInterceptor(func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return rejectErr
	}))
	reflection.Register(server)
	go server.Serve(lis)
	return lis.Addr().String(), server.Stop
}

// waitForConnReady 等待 ClientConn 进入 Ready 状态
func waitForConnReady(t *testing.T, conn *grpc.ClientConn, timeout time.Duration) {
	t.Helper()
	conn.Connect()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for conn.GetState() != connectivity.Ready {
		if !conn.WaitForStateChange(ctx, conn.GetState()) {
			t.Fatalf("ClientConn 未在 %v 内就绪，当前状态: %s", timeout, conn.GetState())
		}
	}
}

// findFreePort 获取一个空闲 TCP 端口（不监听）
func findFreePort(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	lis.Close()
	return addr
}

// mockClientStream 用于拦截器测试的 mock 流
type mockClientStream struct {
	ctx context.Context
}

func (m *mockClientStream) Header() (metadata.MD, error) { return nil, nil }
func (m *mockClientStream) Trailer() metadata.MD         { return nil }
func (m *mockClientStream) CloseSend() error             { return nil }
func (m *mockClientStream) Context() context.Context     { return m.ctx }
func (m *mockClientStream) SendMsg(any) error            { return nil }
func (m *mockClientStream) RecvMsg(any) error            { return nil }

// =============================================================================
// NewHealthChecker 测试
// =============================================================================

func TestNewHealthChecker_Empty(t *testing.T) {
	hc := NewHealthChecker()
	assert.Empty(t, hc.clients)
	assert.Nil(t, hc.onRecover)
}

// =============================================================================
// Register 测试
// =============================================================================

func TestRegister_UnreachableEndpoint(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	hc := NewHealthChecker()
	hc.Register("svc", nil, "localhost:59999") // 不可达端口

	healthy, exists := hc.GetServiceHealth("svc")
	assert.True(t, exists)
	assert.False(t, healthy, "不可达端口应标记为不健康")
}

func TestRegister_ReachableEndpoint(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	hc := NewHealthChecker()
	hc.Register("svc", nil, lis.Addr().String())

	healthy, exists := hc.GetServiceHealth("svc")
	assert.True(t, exists)
	assert.True(t, healthy, "可达端口应标记为健康")
}

func TestRegister_StoresConn(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	hc := NewHealthChecker()
	hc.Register("svc", conn, lis.Addr().String())

	hc.mu.RLock()
	h := hc.clients["svc"]
	hc.mu.RUnlock()
	assert.NotNil(t, h)
	assert.Equal(t, conn, h.conn)
}

func TestRegister_OverwriteExisting(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	hc := NewHealthChecker()
	hc.Register("svc", nil, "localhost:59999")
	hc.Register("svc", nil, lis.Addr().String())

	healthy, _ := hc.GetServiceHealth("svc")
	assert.True(t, healthy, "重新注册后应反映新端点状态")
}

// =============================================================================
// IsHealthy / GetServiceHealth 测试
// =============================================================================

func TestIsHealthy_NotRegistered(t *testing.T) {
	hc := NewHealthChecker()
	assert.False(t, hc.IsHealthy("nonexistent"))
}

func TestIsHealthy_Healthy(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	hc := NewHealthChecker()
	hc.Register("svc", nil, lis.Addr().String())
	assert.True(t, hc.IsHealthy("svc"))
}

func TestIsHealthy_Unhealthy(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	hc := NewHealthChecker()
	hc.Register("svc", nil, "localhost:59999")
	assert.False(t, hc.IsHealthy("svc"))
}

func TestGetServiceHealth_NotRegistered(t *testing.T) {
	hc := NewHealthChecker()
	healthy, exists := hc.GetServiceHealth("nonexistent")
	assert.False(t, healthy)
	assert.False(t, exists)
}

func TestGetServiceHealth_Registered(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	hc := NewHealthChecker()
	hc.Register("svc", nil, lis.Addr().String())
	healthy, exists := hc.GetServiceHealth("svc")
	assert.True(t, exists)
	assert.True(t, healthy)
}

// =============================================================================
// GetHealthStatus 测试
// =============================================================================

func TestGetHealthStatus_Empty(t *testing.T) {
	hc := NewHealthChecker()
	assert.Empty(t, hc.GetHealthStatus())
}

func TestGetHealthStatus_MultipleServices(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	hc := NewHealthChecker()
	hc.Register("healthy-svc", nil, lis.Addr().String())
	hc.Register("unhealthy-svc", nil, "localhost:59999")

	statusMap := hc.GetHealthStatus()
	assert.Len(t, statusMap, 2)
	assert.True(t, statusMap["healthy-svc"])
	assert.False(t, statusMap["unhealthy-svc"])
}

// =============================================================================
// SetOnRecover 测试
// =============================================================================

func TestSetOnRecover_SetsCallback(t *testing.T) {
	hc := NewHealthChecker()
	called := false
	hc.SetOnRecover(func(name string) { called = true })

	hc.mu.RLock()
	cb := hc.onRecover
	hc.mu.RUnlock()
	require.NotNil(t, cb)
	cb("test")
	assert.True(t, called)
}

func TestSetOnRecover_OverwriteCallback(t *testing.T) {
	hc := NewHealthChecker()
	first := false
	hc.SetOnRecover(func(name string) { first = true })

	second := false
	hc.SetOnRecover(func(name string) { second = true })

	hc.mu.RLock()
	cb := hc.onRecover
	hc.mu.RUnlock()
	cb("test")
	assert.False(t, first, "第一个回调不应被调用")
	assert.True(t, second, "第二个回调应被调用")
}

// =============================================================================
// checkEndpointHealth 测试
// =============================================================================

func TestCheckEndpointHealth_Reachable(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	hc := NewHealthChecker()
	hc.Register("svc", nil, "localhost:59999") // 初始不健康
	hc.checkEndpointHealth("svc", lis.Addr().String())

	healthy, _ := hc.GetServiceHealth("svc")
	assert.True(t, healthy, "可达端口应标记为健康")
}

func TestCheckEndpointHealth_Unreachable(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	hc := NewHealthChecker()
	hc.Register("svc", nil, lis.Addr().String()) // 初始健康
	hc.checkEndpointHealth("svc", "localhost:59999")

	healthy, _ := hc.GetServiceHealth("svc")
	assert.False(t, healthy, "不可达端口应标记为不健康")
}

func TestCheckEndpointHealth_NotRegistered(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	hc := NewHealthChecker()
	// 不应 panic
	hc.checkEndpointHealth("nonexistent", "localhost:59999")
}

func TestCheckEndpointHealth_UpdatesLastCheck(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	hc := NewHealthChecker()
	hc.Register("svc", nil, lis.Addr().String())

	before := time.Now()
	hc.checkEndpointHealth("svc", lis.Addr().String())
	after := time.Now()

	hc.mu.RLock()
	h := hc.clients["svc"]
	hc.mu.RUnlock()
	h.mu.RLock()
	lastCheck := h.lastCheck
	h.mu.RUnlock()
	assert.True(t, lastCheck.After(before.Add(-time.Millisecond)))
	assert.True(t, lastCheck.Before(after.Add(time.Millisecond)))
}

// =============================================================================
// checkConnReady 测试
// =============================================================================

func TestCheckConnReady_Ready(t *testing.T) {
	addr, stop := startTestGRPCServer(t)
	defer stop()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	conn.Connect()
	assert.True(t, checkConnReady(conn, 5*time.Second), "Ready 状态的 ClientConn 应返回 true")
}

func TestCheckConnReady_NotReady(t *testing.T) {
	conn, err := grpc.NewClient("localhost:59999", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	conn.Connect()
	assert.False(t, checkConnReady(conn, connReadyTimeout), "不可达端口的 ClientConn 应返回 false")
}

// =============================================================================
// checkEndpointHealth 使用真实 ClientConn 测试
// =============================================================================

func TestCheckEndpointHealth_WithConn_Ready(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	addr, stop := startTestGRPCServer(t)
	defer stop()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	waitForConnReady(t, conn, 5*time.Second)

	hc := NewHealthChecker()
	hc.Register("svc", conn, addr) // endpoint 不影响，conn 非 nil 时用 ClientConn 状态

	healthy, _ := hc.GetServiceHealth("svc")
	assert.True(t, healthy, "ClientConn Ready 时应标记为健康")
}

func TestCheckEndpointHealth_WithConn_NotReady(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	conn, err := grpc.NewClient("localhost:59999", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	hc := NewHealthChecker()
	hc.Register("svc", conn, "localhost:59999")

	healthy, _ := hc.GetServiceHealth("svc")
	assert.False(t, healthy, "ClientConn 未就绪时应标记为不健康")
}

func TestCheckEndpointHealth_WithConn_StateTransition(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	freePort := findFreePort(t)

	conn, err := grpc.NewClient(freePort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	hc := NewHealthChecker()
	hc.Register("svc", conn, freePort)

	// 初始：端口未监听 → 不健康
	healthy, _ := hc.GetServiceHealth("svc")
	assert.False(t, healthy, "端口未监听时应不健康")

	// 启动服务器
	lis, err := net.Listen("tcp", freePort)
	require.NoError(t, err)
	server := grpc.NewServer()
	reflection.Register(server)
	go server.Serve(lis)
	defer server.Stop()

	// 重新检查 → 应变为健康
	hc.checkEndpointHealth("svc", freePort)
	healthy, _ = hc.GetServiceHealth("svc")
	assert.True(t, healthy, "服务器启动后重新检查应健康")
}

// =============================================================================
// StartPeriodicCheck 测试
// =============================================================================

func TestStartPeriodicCheck_ConnTransientFailure_MarksUnhealthy(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	addr, stop := startTestGRPCServer(t)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	waitForConnReady(t, conn, 5*time.Second)

	hc := NewHealthChecker()
	hc.Register("svc", conn, addr)
	require.True(t, hc.IsHealthy("svc"), "初始应健康")

	hc.StartPeriodicCheck(100*time.Millisecond, map[string]string{})
	t.Cleanup(func() {
		hc.SetOnRecover(nil)
		hc.mu.Lock()
		delete(hc.clients, "svc")
		hc.mu.Unlock()
	})

	// 停止服务器，模拟所有 Pod 宕机
	stop()

	// 等待 ClientConn 进入 TransientFailure，健康检查应标记为不健康
	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("服务停止后应标记为不健康")
		default:
		}
		if !hc.IsHealthy("svc") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.False(t, hc.IsHealthy("svc"), "TransientFailure 后应不健康")
}

func TestStartPeriodicCheck_EmptyEndpoints(t *testing.T) {
	hc := NewHealthChecker()
	hc.StartPeriodicCheck(100*time.Millisecond, map[string]string{})
	// 无端点时应直接返回，不启动 goroutine
}

func TestStartPeriodicCheck_StaysUnhealthy_NoCallback(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	hc := NewHealthChecker()
	hc.Register("svc", nil, "localhost:59999")

	callbackFired := make(chan string, 1)
	hc.SetOnRecover(func(name string) { callbackFired <- name })

	hc.StartPeriodicCheck(100*time.Millisecond, map[string]string{"svc": "localhost:59999"})
	t.Cleanup(func() {
		hc.SetOnRecover(nil)
		hc.mu.Lock()
		delete(hc.clients, "svc")
		hc.mu.Unlock()
	})

	select {
	case <-callbackFired:
		t.Fatal("服务持续不健康时不应触发恢复回调")
	case <-time.After(500 * time.Millisecond):
	}
}

func TestStartPeriodicCheck_AlreadyHealthy_NoCallback(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	hc := NewHealthChecker()
	hc.Register("svc", nil, lis.Addr().String()) // 初始即健康

	callbackFired := make(chan string, 1)
	hc.SetOnRecover(func(name string) { callbackFired <- name })

	hc.StartPeriodicCheck(100*time.Millisecond, map[string]string{"svc": lis.Addr().String()})
	t.Cleanup(func() {
		hc.SetOnRecover(nil)
		hc.mu.Lock()
		delete(hc.clients, "svc")
		hc.mu.Unlock()
	})

	select {
	case <-callbackFired:
		t.Fatal("服务一直健康时不应触发恢复回调")
	case <-time.After(500 * time.Millisecond):
	}
}

// TestStartPeriodicCheck_Recovery_WithReadyConn_TriggersCallback
// 核心测试：服务恢复健康且 ClientConn 已就绪时，恢复回调应被触发
func TestStartPeriodicCheck_Recovery_WithReadyConn_TriggersCallback(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	addr, stopServer := startTestGRPCServer(t)
	defer stopServer()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	waitForConnReady(t, conn, 5*time.Second)

	hc := NewHealthChecker()
	// 手动注册为不健康状态（模拟服务恢复前的状态）
	hc.mu.Lock()
	hc.clients["svc"] = &ClientHealth{
		conn:      conn,
		healthy:   false,
		lastCheck: time.Now(),
	}
	hc.mu.Unlock()

	callbackFired := make(chan string, 1)
	hc.SetOnRecover(func(name string) { callbackFired <- name })

	hc.StartPeriodicCheck(100*time.Millisecond, map[string]string{"svc": addr})
	t.Cleanup(func() {
		hc.SetOnRecover(nil)
		hc.mu.Lock()
		delete(hc.clients, "svc")
		hc.mu.Unlock()
	})

	select {
	case name := <-callbackFired:
		assert.Equal(t, "svc", name, "恢复回调应传递正确的服务名")
	case <-time.After(5 * time.Second):
		t.Fatal("服务恢复且 ClientConn 就绪时，恢复回调应被触发")
	}
}

// TestStartPeriodicCheck_Recovery_WithNotReadyConn_SkipsCallback
// 核心 Bug 验证：TCP 端口可达但 ClientConn 未就绪时，恢复回调不应被触发
// 这是本次修复的关键场景：避免在连接未 Ready 时触发 reflection 导致 stream.Send EOF
func TestStartPeriodicCheck_Recovery_WithNotReadyConn_SkipsCallback(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	// TCP 监听器（TCP 探测会成功）
	tcpLis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer tcpLis.Close()
	tcpAddr := tcpLis.Addr().String()

	// ClientConn 指向不可达端口（永远不会 Ready）
	conn, err := grpc.NewClient("localhost:59999", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	// 缩短超时，避免测试等待过久
	origTimeout := connReadyTimeout
	connReadyTimeout = 300 * time.Millisecond
	defer func() { connReadyTimeout = origTimeout }()

	hc := NewHealthChecker()
	hc.mu.Lock()
	hc.clients["svc"] = &ClientHealth{
		conn:      conn,
		healthy:   false,
		lastCheck: time.Now(),
	}
	hc.mu.Unlock()

	callbackFired := make(chan string, 1)
	hc.SetOnRecover(func(name string) { callbackFired <- name })

	hc.StartPeriodicCheck(100*time.Millisecond, map[string]string{"svc": tcpAddr})
	t.Cleanup(func() {
		hc.SetOnRecover(nil)
		hc.mu.Lock()
		delete(hc.clients, "svc")
		hc.mu.Unlock()
	})

	// 等待超过 connReadyTimeout，验证回调未被触发
	select {
	case <-callbackFired:
		t.Fatal("ClientConn 未就绪时不应触发恢复回调")
	case <-time.After(2 * time.Second):
		// 成功：回调未被触发
	}
}

// TestStartPeriodicCheck_Recovery_NilConn_TriggersCallback
// ClientConn 为 nil 时（无连接管理），TCP 探测成功即触发回调
func TestStartPeriodicCheck_Recovery_NilConn_TriggersCallback(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	hc := NewHealthChecker()
	hc.mu.Lock()
	hc.clients["svc"] = &ClientHealth{
		conn:      nil,
		healthy:   false,
		lastCheck: time.Now(),
	}
	hc.mu.Unlock()

	callbackFired := make(chan string, 1)
	hc.SetOnRecover(func(name string) { callbackFired <- name })

	hc.StartPeriodicCheck(100*time.Millisecond, map[string]string{"svc": lis.Addr().String()})
	t.Cleanup(func() {
		hc.SetOnRecover(nil)
		hc.mu.Lock()
		delete(hc.clients, "svc")
		hc.mu.Unlock()
	})

	select {
	case name := <-callbackFired:
		assert.Equal(t, "svc", name)
	case <-time.After(5 * time.Second):
		t.Fatal("conn 为 nil 时 TCP 探测成功应直接触发回调")
	}
}

// =============================================================================
// Close 测试
// =============================================================================

func TestClose_ClearsClients(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	hc := NewHealthChecker()
	hc.Register("svc-a", nil, "localhost:59999")
	hc.Register("svc-b", nil, "localhost:59998")

	err := hc.Close()
	assert.NoError(t, err)
	assert.Empty(t, hc.clients)
}

func TestClose_ClosesConn(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	addr, stop := startTestGRPCServer(t)
	defer stop()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	hc := NewHealthChecker()
	hc.Register("svc", conn, addr)

	err = hc.Close()
	assert.NoError(t, err)
	// 关闭后连接状态应为 Shutdown
	assert.Equal(t, connectivity.Shutdown, conn.GetState())
}

func TestClose_Empty(t *testing.T) {
	hc := NewHealthChecker()
	err := hc.Close()
	assert.NoError(t, err)
}

func TestClose_Idempotent(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	hc := NewHealthChecker()
	hc.Register("svc", nil, "localhost:59999")

	require.NoError(t, hc.Close())
	require.NoError(t, hc.Close(), "重复 Close 不应报错")
}

// =============================================================================
// EnsureServiceReady 测试
// =============================================================================

func TestEnsureServiceReady_NilClient(t *testing.T) {
	err := EnsureServiceReady(nil, nil, "svc")
	assert.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestEnsureServiceReady_Unhealthy(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	hc := NewHealthChecker()
	hc.Register("svc", nil, "localhost:59999") // 不健康

	err := EnsureServiceReady("fake-client", hc.IsHealthy, "svc")
	assert.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unavailable, st.Code())
}

func TestEnsureServiceReady_Healthy(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	hc := NewHealthChecker()
	hc.Register("svc", nil, lis.Addr().String())

	err = EnsureServiceReady("fake-client", hc.IsHealthy, "svc")
	assert.NoError(t, err)
}

func TestEnsureServiceReady_NilHealthChecker(t *testing.T) {
	err := EnsureServiceReady("fake-client", nil, "svc")
	assert.NoError(t, err, "无健康检查函数时应放行")
}

// =============================================================================
// ServiceGuard 测试
// =============================================================================

func TestNewServiceGuard(t *testing.T) {
	g := NewServiceGuard("svc")
	assert.Equal(t, "svc", g.serviceName)
}

func TestServiceGuard_WithServiceName(t *testing.T) {
	g := NewServiceGuard("")
	g = g.WithServiceName("new-svc")
	assert.Equal(t, "new-svc", g.serviceName)
}

func TestServiceGuard_WithClient(t *testing.T) {
	g := NewServiceGuard("svc")
	g = g.WithClient("my-client")
	assert.Equal(t, "my-client", g.client)
}

func TestServiceGuard_WithHealthChecker(t *testing.T) {
	g := NewServiceGuard("svc")
	fn := func(string) bool { return true }
	g = g.WithHealthChecker(fn)
	assert.NotNil(t, g.isHealthy)
}

func TestServiceGuard_Ensure_NilClient(t *testing.T) {
	g := NewServiceGuard("svc")
	err := g.Ensure()
	assert.Error(t, err)
}

func TestServiceGuard_Ensure_Healthy(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	hc := NewHealthChecker()
	hc.Register("svc", nil, lis.Addr().String())

	g := NewServiceGuard("svc").
		WithClient("fake-client").
		WithHealthChecker(hc.IsHealthy)
	assert.NoError(t, g.Ensure())
}

func TestServiceGuard_Ensure_Unhealthy(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	hc := NewHealthChecker()
	hc.Register("svc", nil, "localhost:59999")

	g := NewServiceGuard("svc").
		WithClient("fake-client").
		WithHealthChecker(hc.IsHealthy)
	err := g.Ensure()
	assert.Error(t, err)
}

func TestServiceGuard_Ensure_NilHealthChecker(t *testing.T) {
	g := NewServiceGuard("svc").WithClient("fake-client")
	assert.NoError(t, g.Ensure())
}

// =============================================================================
// UnaryClientHealthInterceptor 测试
// =============================================================================

func TestUnaryClientHealthInterceptor_NilChecker(t *testing.T) {
	interceptor := UnaryClientHealthInterceptor("svc", nil)
	invoked := false
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invoked = true
		return nil
	}
	err := interceptor(context.Background(), "/test/method", nil, nil, nil, invoker)
	assert.NoError(t, err)
	assert.True(t, invoked, "nil checker 应放行")
}

func TestUnaryClientHealthInterceptor_NotRegistered(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	hc := NewHealthChecker()
	interceptor := UnaryClientHealthInterceptor("nonexistent", hc)

	invoked := false
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invoked = true
		return nil
	}
	err := interceptor(context.Background(), "/test/method", nil, nil, nil, invoker)
	assert.NoError(t, err)
	assert.True(t, invoked, "未注册服务应放行")
}

func TestUnaryClientHealthInterceptor_Healthy(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	hc := NewHealthChecker()
	hc.Register("svc", nil, lis.Addr().String())
	interceptor := UnaryClientHealthInterceptor("svc", hc)

	invoked := false
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invoked = true
		return nil
	}
	err = interceptor(context.Background(), "/test/method", nil, nil, nil, invoker)
	assert.NoError(t, err)
	assert.True(t, invoked, "健康服务应放行")
}

func TestUnaryClientHealthInterceptor_Unhealthy(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	hc := NewHealthChecker()
	hc.Register("svc", nil, "localhost:59999") // 不健康
	interceptor := UnaryClientHealthInterceptor("svc", hc)

	invoked := false
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invoked = true
		return nil
	}
	err := interceptor(context.Background(), "/test/method", nil, nil, nil, invoker)
	assert.Error(t, err)
	assert.False(t, invoked, "不健康服务应拦截，不调用 invoker")

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unavailable, st.Code())
}

// =============================================================================
// StreamClientHealthInterceptor 测试
// =============================================================================

func TestStreamClientHealthInterceptor_NilChecker(t *testing.T) {
	interceptor := StreamClientHealthInterceptor("svc", nil)
	invoked := false
	streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		invoked = true
		return &mockClientStream{ctx: ctx}, nil
	}
	stream, err := interceptor(context.Background(), nil, nil, "/test/method", streamer)
	assert.NoError(t, err)
	assert.NotNil(t, stream)
	assert.True(t, invoked, "nil checker 应放行")
}

func TestStreamClientHealthInterceptor_NotRegistered(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	hc := NewHealthChecker()
	interceptor := StreamClientHealthInterceptor("nonexistent", hc)

	invoked := false
	streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		invoked = true
		return &mockClientStream{ctx: ctx}, nil
	}
	stream, err := interceptor(context.Background(), nil, nil, "/test/method", streamer)
	assert.NoError(t, err)
	assert.NotNil(t, stream)
	assert.True(t, invoked, "未注册服务应放行")
}

func TestStreamClientHealthInterceptor_Healthy(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer lis.Close()

	hc := NewHealthChecker()
	hc.Register("svc", nil, lis.Addr().String())
	interceptor := StreamClientHealthInterceptor("svc", hc)

	invoked := false
	streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		invoked = true
		return &mockClientStream{ctx: ctx}, nil
	}
	stream, err := interceptor(context.Background(), nil, nil, "/test/method", streamer)
	assert.NoError(t, err)
	assert.NotNil(t, stream)
	assert.True(t, invoked, "健康服务应放行")
}

func TestStreamClientHealthInterceptor_Unhealthy(t *testing.T) {
	_ = gwglobal.EnsureLoggerInitialized()
	hc := NewHealthChecker()
	hc.Register("svc", nil, "localhost:59999") // 不健康
	interceptor := StreamClientHealthInterceptor("svc", hc)

	invoked := false
	streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		invoked = true
		return &mockClientStream{ctx: ctx}, nil
	}
	stream, err := interceptor(context.Background(), nil, nil, "/test/method", streamer)
	assert.Error(t, err)
	assert.Nil(t, stream)
	assert.False(t, invoked, "不健康服务应拦截，不调用 streamer")

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unavailable, st.Code())
}
