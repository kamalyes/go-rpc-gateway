/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-21 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-21 19:51:39
 * @FilePath: \go-rpc-gateway\cpool\grpc\client_test.go
 * @Description: gRPC 客户端连接池与初始化测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package grpc

import (
	"sync"
	"testing"
	"time"

	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	gwglobal "github.com/kamalyes/go-rpc-gateway/global"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

// resetConnPool 重置连接池（测试辅助函数）
func resetConnPool() {
	connPoolMu.Lock()
	connPool = make(map[string]*grpc.ClientConn)
	connPoolMu.Unlock()
}

// mockConn 创建一个用于测试的 *grpc.ClientConn（不实际连接）
// 注意：grpc.ClientConn 无法直接 mock，这里使用 nil 占位测试池的存取逻辑
// 对于需要真实连接的测试，使用 InitClient 的集成测试

func init() {
	// 确保全局日志器初始化
	_ = gwglobal.EnsureLoggerInitialized()
}

// ==================== GetConn / PutConn 测试 ====================

func TestGetConn_EmptyPool(t *testing.T) {
	resetConnPool()

	conn, ok := GetConn("nonexistent-service")
	assert.Nil(t, conn, "空连接池应返回 nil")
	assert.False(t, ok, "空连接池应返回 false")
}

func TestPutConn_AndGetConn(t *testing.T) {
	resetConnPool()

	// PutConn 存入 nil 连接（测试存取逻辑，不关心连接本身）
	PutConn("test-service", nil)

	conn, ok := GetConn("test-service")
	assert.Nil(t, conn, "存入 nil 连接应返回 nil")
	assert.True(t, ok, "已注册的服务名应返回 true")
}

func TestPutConn_Overwrite(t *testing.T) {
	resetConnPool()

	PutConn("test-service", nil)
	PutConn("test-service", nil) // 覆盖

	conn, ok := GetConn("test-service")
	assert.True(t, ok)
	assert.Nil(t, conn)
}

func TestPutConn_MultipleServices(t *testing.T) {
	resetConnPool()

	PutConn("service-a", nil)
	PutConn("service-b", nil)

	_, okA := GetConn("service-a")
	_, okB := GetConn("service-b")
	_, okC := GetConn("service-c")

	assert.True(t, okA, "service-a 应存在")
	assert.True(t, okB, "service-b 应存在")
	assert.False(t, okC, "service-c 不应存在")
}

func TestGetConn_ConcurrentAccess(t *testing.T) {
	resetConnPool()

	var wg sync.WaitGroup
	const goroutines = 100

	// 并发写入
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			PutConn("concurrent-service", nil)
		}(i)
	}
	wg.Wait()

	// 并发读取
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok := GetConn("concurrent-service")
			assert.True(t, ok)
		}()
	}
	wg.Wait()
}

func TestPutConn_ConcurrentDifferentKeys(t *testing.T) {
	resetConnPool()

	var wg sync.WaitGroup
	const goroutines = 50

	// 并发写入不同 key
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			PutConn("service-"+string(rune(idx)), nil)
		}(i)
	}
	wg.Wait()

	// 验证所有 key 都存在
	connPoolMu.RLock()
	count := len(connPool)
	connPoolMu.RUnlock()
	assert.Equal(t, goroutines, count, "应写入 %d 个不同的 key", goroutines)
}

// ==================== InitClient 测试 ====================

func TestInitClient_ServiceNotFound(t *testing.T) {
	resetConnPool()

	clients := map[string]*gwconfig.GRPCClient{}

	type mockClient struct{ Name string }
	client, ok := InitClient[mockClient](nil, clients, "nonexistent-service", func(ci grpc.ClientConnInterface) mockClient {
		return mockClient{Name: "test"}
	})

	assert.False(t, ok, "不存在的服务应返回 false")
	assert.Equal(t, mockClient{}, client, "不存在的服务应返回零值")
}

func TestInitClient_NilClientConfig(t *testing.T) {
	resetConnPool()

	clients := map[string]*gwconfig.GRPCClient{
		"test-service": nil,
	}

	type mockClient struct{ Name string }
	client, ok := InitClient[mockClient](nil, clients, "test-service", func(ci grpc.ClientConnInterface) mockClient {
		return mockClient{Name: "test"}
	})

	assert.False(t, ok, "nil 配置应返回 false")
	assert.Equal(t, mockClient{}, client)
}

func TestInitClient_EmptyEndpoints(t *testing.T) {
	resetConnPool()

	clients := map[string]*gwconfig.GRPCClient{
		"test-service": {Endpoints: []string{}},
	}

	type mockClient struct{ Name string }
	client, ok := InitClient[mockClient](nil, clients, "test-service", func(ci grpc.ClientConnInterface) mockClient {
		return mockClient{Name: "test"}
	})

	assert.False(t, ok, "空端点列表应返回 false")
	assert.Equal(t, mockClient{}, client)
}

func TestInitClient_ReuseExistingConnection(t *testing.T) {
	resetConnPool()

	// 先存入一个连接到连接池
	PutConn("test-service", nil)

	// InitClient 应复用连接池中的连接
	clients := map[string]*gwconfig.GRPCClient{
		"test-service": {
			Endpoints: []string{"localhost:9999"},
		},
	}

	factoryCalled := false
	type mockClient struct{ Name string }
	client, ok := InitClient[mockClient](nil, clients, "test-service", func(ci grpc.ClientConnInterface) mockClient {
		factoryCalled = true
		return mockClient{Name: "from-pool"}
	})

	assert.True(t, ok, "复用连接应返回 true")
	assert.True(t, factoryCalled, "factory 应被调用")
	assert.Equal(t, mockClient{Name: "from-pool"}, client)
}

func TestInitClient_CreateNewConnection(t *testing.T) {
	resetConnPool()

	// 使用一个不会实际连接的端口
	clients := map[string]*gwconfig.GRPCClient{
		"test-service": {
			Endpoints: []string{"localhost:59999"},
		},
	}

	type mockClient struct{ Name string }
	client, ok := InitClient[mockClient](nil, clients, "test-service", func(ci grpc.ClientConnInterface) mockClient {
		return mockClient{Name: "new-conn"}
	})

	assert.True(t, ok, "创建新连接应返回 true")
	assert.Equal(t, mockClient{Name: "new-conn"}, client)

	// 验证连接已存入连接池
	_, poolOk := GetConn("test-service")
	assert.True(t, poolOk, "新连接应存入连接池")
}

func TestInitClient_WithHealthChecker(t *testing.T) {
	resetConnPool()

	hc := NewHealthChecker()

	clients := map[string]*gwconfig.GRPCClient{
		"test-service": {
			Endpoints: []string{"localhost:59998"},
		},
	}

	type mockClient struct{ Name string }
	client, ok := InitClient[mockClient](hc, clients, "test-service", func(ci grpc.ClientConnInterface) mockClient {
		return mockClient{Name: "with-hc"}
	})

	assert.True(t, ok)
	assert.Equal(t, mockClient{Name: "with-hc"}, client)

	// 验证健康检查器已注册该服务
	_, exists := hc.GetServiceHealth("test-service")
	assert.True(t, exists, "健康检查器应注册该服务")
}

func TestInitClient_MultipleCallsSameService(t *testing.T) {
	resetConnPool()

	clients := map[string]*gwconfig.GRPCClient{
		"test-service": {
			Endpoints: []string{"localhost:59997"},
		},
	}

	type mockClient struct{ Name string }

	// 第一次调用：创建新连接
	client1, ok1 := InitClient[mockClient](nil, clients, "test-service", func(ci grpc.ClientConnInterface) mockClient {
		return mockClient{Name: "first"}
	})
	assert.True(t, ok1)
	assert.Equal(t, mockClient{Name: "first"}, client1)

	// 第二次调用：应复用连接池中的连接
	client2, ok2 := InitClient[mockClient](nil, clients, "test-service", func(ci grpc.ClientConnInterface) mockClient {
		return mockClient{Name: "second"}
	})
	assert.True(t, ok2)
	assert.Equal(t, mockClient{Name: "second"}, client2)

	// 连接池中应只有一个连接
	connPoolMu.RLock()
	count := len(connPool)
	connPoolMu.RUnlock()
	assert.Equal(t, 1, count, "同一服务应只创建一个连接")
}

// ==================== BuildDialOptions 测试 ====================

func TestBuildDialOptions_DefaultConfig(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		Endpoints: []string{"localhost:9999"},
	}

	opts := BuildDialOptions(cfg, "test-service", nil)
	assert.NotEmpty(t, opts, "应返回非空的 dial options")
}

func TestBuildDialOptions_WithCompression(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		Endpoints:         []string{"localhost:9999"},
		EnableCompression: true,
		CompressionType:   gwconfig.GRPCCompressGzip,
	}

	opts := BuildDialOptions(cfg, "test-service", nil)
	assert.NotEmpty(t, opts)
}

func TestBuildDialOptions_WithLoadBalance(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		Endpoints:         []string{"localhost:9999"},
		EnableLoadBalance: true,
		LoadBalancePolicy: "round_robin",
	}

	opts := BuildDialOptions(cfg, "test-service", nil)
	assert.NotEmpty(t, opts)
}

func TestBuildDialOptions_WithNetwork(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		Endpoints: []string{"localhost:9999"},
		Network:   "tcp4",
	}

	opts := BuildDialOptions(cfg, "test-service", nil)
	assert.NotEmpty(t, opts)
}

func TestBuildDialOptions_WithCustomKeepalive(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		Endpoints:        []string{"localhost:9999"},
		KeepaliveTime:    30,
		KeepaliveTimeout: 10,
	}

	opts := BuildDialOptions(cfg, "test-service", nil)
	assert.NotEmpty(t, opts)
}

func TestBuildDialOptions_WithCustomMessageSize(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		Endpoints:      []string{"localhost:9999"},
		MaxRecvMsgSize: 32 * 1024 * 1024,
		MaxSendMsgSize: 32 * 1024 * 1024,
	}

	opts := BuildDialOptions(cfg, "test-service", nil)
	assert.NotEmpty(t, opts)
}

func TestBuildDialOptions_WithCustomWindowSize(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		Endpoints:             []string{"localhost:9999"},
		InitialWindowSize:     2 << 20,
		InitialConnWindowSize: 2 << 21,
	}

	opts := BuildDialOptions(cfg, "test-service", nil)
	assert.NotEmpty(t, opts)
}

func TestBuildDialOptions_WithConnectionTimeout(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		Endpoints:         []string{"localhost:9999"},
		Network:           "tcp4",
		ConnectionTimeout: 60,
	}

	opts := BuildDialOptions(cfg, "test-service", nil)
	assert.NotEmpty(t, opts)
}

func TestBuildDialOptions_WithHealthChecker(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		Endpoints: []string{"localhost:9999"},
	}

	hc := NewHealthChecker()
	opts := BuildDialOptions(cfg, "test-service", hc)
	assert.NotEmpty(t, opts)
}

func TestBuildDialOptions_NilHealthChecker(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		Endpoints: []string{"localhost:9999"},
	}

	opts := BuildDialOptions(cfg, "test-service", nil)
	assert.NotEmpty(t, opts)
}

func TestBuildDialOptions_DefaultLoadBalancePolicy(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		Endpoints:         []string{"localhost:9999"},
		EnableLoadBalance: true,
		LoadBalancePolicy: "", // 应默认 round_robin
	}

	opts := BuildDialOptions(cfg, "test-service", nil)
	assert.NotEmpty(t, opts)
}

func TestBuildDialOptions_DefaultCompressionType(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		Endpoints:         []string{"localhost:9999"},
		EnableCompression: true,
		CompressionType:   "", // 应默认 gzip
	}

	opts := BuildDialOptions(cfg, "test-service", nil)
	assert.NotEmpty(t, opts)
}

// ==================== BuildEndpointMap 测试 ====================

func TestBuildEndpointMap_Empty(t *testing.T) {
	endpoints := BuildEndpointMap(nil)
	assert.Empty(t, endpoints)
}

func TestBuildEndpointMap_NilClient(t *testing.T) {
	clients := map[string]*gwconfig.GRPCClient{
		"service-a": nil,
	}

	endpoints := BuildEndpointMap(clients)
	assert.Empty(t, endpoints, "nil 客户端应被跳过")
}

func TestBuildEndpointMap_EmptyEndpoints(t *testing.T) {
	clients := map[string]*gwconfig.GRPCClient{
		"service-a": {Endpoints: []string{}},
	}

	endpoints := BuildEndpointMap(clients)
	assert.Empty(t, endpoints, "空端点列表应被跳过")
}

func TestBuildEndpointMap_ValidClients(t *testing.T) {
	clients := map[string]*gwconfig.GRPCClient{
		"service-a": {Endpoints: []string{"localhost:9000"}},
		"service-b": {Endpoints: []string{"localhost:9001", "localhost:9002"}},
	}

	endpoints := BuildEndpointMap(clients)

	assert.Equal(t, "localhost:9000", endpoints["service-a"])
	assert.Equal(t, "localhost:9001", endpoints["service-b"], "应取第一个端点")
	assert.Len(t, endpoints, 2)
}

func TestBuildEndpointMap_MixedClients(t *testing.T) {
	clients := map[string]*gwconfig.GRPCClient{
		"service-a": {Endpoints: []string{"localhost:9000"}},
		"service-b": nil,
		"service-c": {Endpoints: []string{}},
	}

	endpoints := BuildEndpointMap(clients)

	assert.Len(t, endpoints, 1, "只有 service-a 有效")
	assert.Equal(t, "localhost:9000", endpoints["service-a"])
}

// ==================== buildTLSConfig 测试 ====================

func TestBuildTLSConfig_Disabled(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		EnableTLS: false,
	}

	creds := buildTLSConfig(cfg, "test-service")
	assert.NotNil(t, creds, "禁用 TLS 应返回 insecure credentials")
}

func TestBuildTLSConfig_Enabled(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		EnableTLS: true,
	}

	creds := buildTLSConfig(cfg, "test-service")
	assert.NotNil(t, creds, "启用 TLS 应返回 TLS credentials")
}

func TestBuildTLSConfig_WithInvalidCAFile(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		EnableTLS: true,
		TLSCAFile: "/nonexistent/ca.pem",
	}

	creds := buildTLSConfig(cfg, "test-service")
	assert.NotNil(t, creds, "无效 CA 文件应仍返回 credentials（InsecureSkipVerify=true）")
}

func TestBuildTLSConfig_WithInvalidCertFiles(t *testing.T) {
	cfg := &gwconfig.GRPCClient{
		EnableTLS:   true,
		TLSCertFile: "/nonexistent/cert.pem",
		TLSKeyFile:  "/nonexistent/key.pem",
	}

	creds := buildTLSConfig(cfg, "test-service")
	assert.NotNil(t, creds, "无效证书文件应仍返回 credentials")
}

// ==================== DefaultHealthCheckInterval 测试 ====================

func TestDefaultHealthCheckInterval(t *testing.T) {
	assert.Equal(t, 3*time.Second, DefaultHealthCheckInterval, "默认健康检查间隔应为 3 秒")
}

// ==================== 连接池并发安全测试 ====================

func TestConnPool_ConcurrentPutAndGet(t *testing.T) {
	resetConnPool()

	var wg sync.WaitGroup
	const iterations = 200

	// 并发写入和读取
	for i := 0; i < iterations; i++ {
		wg.Add(2)

		// 写入
		go func(idx int) {
			defer wg.Done()
			PutConn("service-"+string(rune(idx%10)), nil)
		}(i)

		// 读取
		go func(idx int) {
			defer wg.Done()
			_, _ = GetConn("service-" + string(rune(idx%10)))
		}(i)
	}

	wg.Wait()
	// 如果没有 panic 或 race condition，测试通过
}

func TestConnPool_ConcurrentInitClient(t *testing.T) {
	resetConnPool()

	clients := map[string]*gwconfig.GRPCClient{
		"concurrent-service": {
			Endpoints: []string{"localhost:59996"},
		},
	}

	var wg sync.WaitGroup
	const goroutines = 10
	results := make([]bool, goroutines)

	type mockClient struct{ Name string }

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, ok := InitClient[mockClient](nil, clients, "concurrent-service", func(ci grpc.ClientConnInterface) mockClient {
				return mockClient{Name: "concurrent"}
			})
			results[idx] = ok
		}(i)
	}

	wg.Wait()

	// 所有调用都应成功
	for i, ok := range results {
		assert.True(t, ok, "goroutine %d 的 InitClient 应成功", i)
	}

	// 连接池中应只有一个连接
	connPoolMu.RLock()
	count := len(connPool)
	connPoolMu.RUnlock()
	assert.Equal(t, 1, count, "并发 InitClient 应只创建一个连接")
}
