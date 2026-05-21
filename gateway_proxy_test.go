/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-21 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-21 19:55:50
 * @FilePath: \go-rpc-gateway\gateway_proxy_test.go
 * @Description: 测试 RegisterProxyHandlerByServiceName 连接池复用逻辑
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package gateway

import (
	"context"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	grpcpool "github.com/kamalyes/go-rpc-gateway/cpool/grpc"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/server"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

func init() {
	_ = global.EnsureLoggerInitialized()
}

// newTestGateway 创建一个用于测试的 Gateway 实例（不启动服务）
func newTestGateway(grpcClients map[string]*gwconfig.GRPCClient) *Gateway {
	// 使用 Default() 获取完整配置，然后覆盖 GRPC.Clients
	cfg := gwconfig.Default()
	cfg.GRPC.Clients = grpcClients

	// 设置全局配置（NewServer 依赖 global.GATEWAY）
	global.GATEWAY = cfg

	srv, err := server.NewServer()
	if err != nil {
		// 如果 NewServer 失败，使用最小化 Server（仅设置 gwMux）
		srv = &server.Server{}
	}

	return &Gateway{
		Server:        srv,
		gatewayConfig: cfg,
		ctx:           context.Background(),
	}
}

// ==================== GetGRPCEndpoint 测试 ====================

func TestGetGRPCEndpoint_NilConfig(t *testing.T) {
	gw := &Gateway{
		gatewayConfig: nil,
		ctx:           context.Background(),
	}
	_, ok := gw.GetGRPCEndpoint("test-service")
	assert.False(t, ok, "nil 配置应返回 false")
}

func TestGetGRPCEndpoint_NilGRPC(t *testing.T) {
	gw := &Gateway{
		gatewayConfig: &gwconfig.Gateway{GRPC: nil},
		ctx:           context.Background(),
	}
	_, ok := gw.GetGRPCEndpoint("test-service")
	assert.False(t, ok, "nil GRPC 应返回 false")
}

func TestGetGRPCEndpoint_NilClients(t *testing.T) {
	gw := &Gateway{
		gatewayConfig: &gwconfig.Gateway{GRPC: &gwconfig.GRPC{Clients: nil}},
		ctx:           context.Background(),
	}
	_, ok := gw.GetGRPCEndpoint("test-service")
	assert.False(t, ok, "nil Clients 应返回 false")
}

func TestGetGRPCEndpoint_NotFound(t *testing.T) {
	gw := newTestGateway(map[string]*gwconfig.GRPCClient{})
	_, ok := gw.GetGRPCEndpoint("nonexistent")
	assert.False(t, ok, "不存在的服务应返回 false")
}

func TestGetGRPCEndpoint_NilClientConfig(t *testing.T) {
	gw := newTestGateway(map[string]*gwconfig.GRPCClient{
		"test-service": nil,
	})
	_, ok := gw.GetGRPCEndpoint("test-service")
	assert.False(t, ok, "nil 客户端配置应返回 false")
}

func TestGetGRPCEndpoint_EmptyEndpoints(t *testing.T) {
	gw := newTestGateway(map[string]*gwconfig.GRPCClient{
		"test-service": {Endpoints: []string{}},
	})
	_, ok := gw.GetGRPCEndpoint("test-service")
	assert.False(t, ok, "空端点列表应返回 false")
}

func TestGetGRPCEndpoint_Valid(t *testing.T) {
	gw := newTestGateway(map[string]*gwconfig.GRPCClient{
		"test-service": {Endpoints: []string{"localhost:9000"}},
	})
	endpoint, ok := gw.GetGRPCEndpoint("test-service")
	assert.True(t, ok)
	assert.Equal(t, "localhost:9000", endpoint)
}

func TestGetGRPCEndpoint_MultipleEndpoints(t *testing.T) {
	gw := newTestGateway(map[string]*gwconfig.GRPCClient{
		"test-service": {Endpoints: []string{"localhost:9000", "localhost:9001"}},
	})
	endpoint, ok := gw.GetGRPCEndpoint("test-service")
	assert.True(t, ok)
	assert.Equal(t, "localhost:9000", endpoint, "应返回第一个端点")
}

// ==================== getGRPCEndpointWithConfig 测试 ====================

func TestGetGRPCEndpointWithConfig_NilConfig(t *testing.T) {
	gw := &Gateway{
		gatewayConfig: nil,
		ctx:           context.Background(),
	}
	_, _, ok := gw.getGRPCEndpointWithConfig("test-service")
	assert.False(t, ok)
}

func TestGetGRPCEndpointWithConfig_Valid(t *testing.T) {
	cfg := &gwconfig.GRPCClient{Endpoints: []string{"localhost:9000"}}
	gw := newTestGateway(map[string]*gwconfig.GRPCClient{
		"test-service": cfg,
	})
	endpoint, clientCfg, ok := gw.getGRPCEndpointWithConfig("test-service")
	assert.True(t, ok)
	assert.Equal(t, "localhost:9000", endpoint)
	assert.Same(t, cfg, clientCfg, "应返回原始配置对象")
}

// ==================== RegisterProxyHandlerByServiceName 测试 ====================

func TestRegisterProxyHandlerByServiceName_ServiceNotFound(t *testing.T) {
	gw := newTestGateway(map[string]*gwconfig.GRPCClient{})

	err := gw.RegisterProxyHandlerByServiceName("nonexistent-service", func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
		return nil
	})

	assert.Error(t, err, "不存在的服务应返回错误")
	assert.Contains(t, err.Error(), "not found")
}

func TestRegisterProxyHandlerByServiceName_CreateNewConnection(t *testing.T) {
	gw := newTestGateway(map[string]*gwconfig.GRPCClient{
		"test-new-conn-service": {Endpoints: []string{"localhost:59995"}},
	})

	registerCalled := false
	err := gw.RegisterProxyHandlerByServiceName("test-new-conn-service", func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
		registerCalled = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, registerCalled, "注册函数应被调用")

	// 验证连接已存入连接池
	_, ok := grpcpool.GetConn("test-new-conn-service")
	assert.True(t, ok, "新连接应存入连接池")
}

func TestRegisterProxyHandlerByServiceName_ReuseFromPool(t *testing.T) {
	// 先通过连接池存入一个连接
	serviceName := "test-reuse-pool-service"
	grpcpool.PutConn(serviceName, nil)

	gw := newTestGateway(map[string]*gwconfig.GRPCClient{
		serviceName: {Endpoints: []string{"localhost:59994"}},
	})

	registerCalled := false
	err := gw.RegisterProxyHandlerByServiceName(serviceName, func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
		registerCalled = true
		assert.Nil(t, conn, "从连接池获取的连接应为 nil（我们存入的是 nil）")
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, registerCalled, "注册函数应被调用")
}

func TestRegisterProxyHandlerByServiceName_MultipleHandlersSameService(t *testing.T) {
	serviceName := "test-multi-handler-service"

	gw := newTestGateway(map[string]*gwconfig.GRPCClient{
		serviceName: {Endpoints: []string{"localhost:59993"}},
	})

	// 第一个 handler 注册
	callCount1 := 0
	err := gw.RegisterProxyHandlerByServiceName(serviceName, func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
		callCount1++
		return nil
	})
	assert.NoError(t, err)

	// 第二个 handler 注册（应复用连接池中的连接）
	callCount2 := 0
	err = gw.RegisterProxyHandlerByServiceName(serviceName, func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
		callCount2++
		return nil
	})
	assert.NoError(t, err)

	assert.Equal(t, 1, callCount1, "第一个注册函数应被调用一次")
	assert.Equal(t, 1, callCount2, "第二个注册函数应被调用一次")

	// 验证 proxyHandlerRegistrations 中有两条记录
	assert.Len(t, gw.proxyHandlerRegistrations, 2, "应有两条注册记录")
}

func TestRegisterProxyHandlerByServiceName_RegisterFuncError(t *testing.T) {
	serviceName := "test-register-error-service"

	gw := newTestGateway(map[string]*gwconfig.GRPCClient{
		serviceName: {Endpoints: []string{"localhost:59992"}},
	})

	err := gw.RegisterProxyHandlerByServiceName(serviceName, func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
		return assert.AnError
	})

	assert.Error(t, err, "注册函数返回错误时应传播")
	assert.Equal(t, assert.AnError, err)
}

func TestRegisterProxyHandlerByServiceName_RegisterFuncErrorWithPooledConn(t *testing.T) {
	serviceName := "test-pool-register-error-service"
	grpcpool.PutConn(serviceName, nil)

	gw := newTestGateway(map[string]*gwconfig.GRPCClient{
		serviceName: {Endpoints: []string{"localhost:59991"}},
	})

	err := gw.RegisterProxyHandlerByServiceName(serviceName, func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
		return assert.AnError
	})

	assert.Error(t, err, "使用连接池连接时注册函数返回错误也应传播")
	assert.Equal(t, assert.AnError, err)
}

// ==================== ConnHandlerRegisterFunc 类型测试 ====================

func TestConnHandlerRegisterFunc_Type(t *testing.T) {
	var fn ConnHandlerRegisterFunc = func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
		return nil
	}

	err := fn(context.Background(), nil, nil)
	assert.NoError(t, err)
}

// ==================== proxyHandlerRegistration 结构测试 ====================

func TestProxyHandlerRegistration_ConnBased(t *testing.T) {
	reg := proxyHandlerRegistration{
		connRegisterFunc: func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
			return nil
		},
		conn:        nil,
		serviceName: "test-service",
		clientCfg:   &gwconfig.GRPCClient{Endpoints: []string{"localhost:9000"}},
	}

	assert.NotNil(t, reg.connRegisterFunc)
	assert.Equal(t, "test-service", reg.serviceName)
	assert.NotNil(t, reg.clientCfg)
}

func TestProxyHandlerRegistration_EndpointBased(t *testing.T) {
	reg := proxyHandlerRegistration{
		registerFunc: func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error {
			return nil
		},
		endpoint: "localhost:9000",
		dialOpts: []grpc.DialOption{},
	}

	assert.NotNil(t, reg.registerFunc)
	assert.Equal(t, "localhost:9000", reg.endpoint)
}

// ==================== 集成测试：InitClient + RegisterProxyHandlerByServiceName ====================

func TestIntegration_InitClientThenRegisterProxy(t *testing.T) {
	serviceName := "test-integration-service"

	clients := map[string]*gwconfig.GRPCClient{
		serviceName: {Endpoints: []string{"localhost:59990"}},
	}

	// 1. 先通过 InitClient 创建连接（模拟 setupGRPCClients 的行为）
	type mockClient struct{ Name string }
	client, ok := grpcpool.InitClient[mockClient](nil, clients, serviceName, func(ci grpc.ClientConnInterface) mockClient {
		return mockClient{Name: "integrated"}
	})
	assert.True(t, ok, "InitClient 应成功")
	assert.Equal(t, mockClient{Name: "integrated"}, client)

	// 2. 再通过 RegisterProxyHandlerByServiceName 注册代理（应复用连接）
	gw := newTestGateway(clients)

	registerCalled := false
	err := gw.RegisterProxyHandlerByServiceName(serviceName, func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
		registerCalled = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, registerCalled, "代理注册函数应被调用")

	// 3. 验证连接池中只有一个连接
	_, poolOk := grpcpool.GetConn(serviceName)
	assert.True(t, poolOk, "连接池中应有该服务的连接")
}
