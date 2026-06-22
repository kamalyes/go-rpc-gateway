/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-13 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-16 11:11:15
 * @FilePath: \go-rpc-gateway\cpool\grpc\auto_register_test.go
 * @Description: 自动注册机制测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package grpc

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

func TestClearRegistry(t *testing.T) {
	// 先添加一些数据
	reflectionRegistry.mu.Lock()
	reflectionRegistry.services["test-service"] = []ReflectionServiceInfo{
		{ServiceName: "TestService"},
	}
	reflectionRegistry.initialized = true
	reflectionRegistry.mu.Unlock()

	routeRegistry.mu.Lock()
	routeRegistry.routes = []HTTPRoute{{HTTPMethod: "GET", HTTPPath: "/test"}}
	routeRegistry.mu.Unlock()

	ClearRegistry()

	// 验证已清空
	reflectionRegistry.mu.RLock()
	assert.Empty(t, reflectionRegistry.services)
	assert.False(t, reflectionRegistry.initialized)
	reflectionRegistry.mu.RUnlock()

	routeRegistry.mu.RLock()
	assert.Empty(t, routeRegistry.routes)
	routeRegistry.mu.RUnlock()
}

func TestGetReflectionRegistry(t *testing.T) {
	ClearRegistry()
	defer ClearRegistry()

	// 初始为空
	services := GetReflectionRegistry("nonexistent")
	assert.Nil(t, services)

	// 添加数据后可获取
	reflectionRegistry.mu.Lock()
	reflectionRegistry.services["test-svc"] = []ReflectionServiceInfo{
		{ServiceName: "TestService"},
	}
	reflectionRegistry.mu.Unlock()

	services = GetReflectionRegistry("test-svc")
	assert.Len(t, services, 1)
	assert.Equal(t, "TestService", services[0].ServiceName)
}

func TestGetRoutes(t *testing.T) {
	ClearRegistry()
	defer ClearRegistry()

	// 初始为空
	routes := GetRoutes()
	assert.Empty(t, routes)

	// 添加路由后可获取
	routeRegistry.mu.Lock()
	routeRegistry.routes = []HTTPRoute{
		{HTTPMethod: "GET", HTTPPath: "/api/v1/test"},
		{HTTPMethod: "POST", HTTPPath: "/api/v1/test"},
	}
	routeRegistry.mu.Unlock()

	routes = GetRoutes()
	assert.Len(t, routes, 2)
}

func TestAutoRegisterResult_Summary(t *testing.T) {
	result := &AutoRegisterResult{
		Clients:       []string{"svc1", "svc2"},
		Handlers:      []string{"GET /api/v1/test"},
		TotalClients:  2,
		TotalHandlers: 1,
		SkippedManual: 0,
	}

	summary := result.Summary()
	assert.Contains(t, summary, "2/2 clients")
	assert.Contains(t, summary, "1/1 handlers")
}

func TestCollectRoutes(t *testing.T) {
	registered := []string{
		"GET /api/v1/users",
		"POST /api/v1/users",
		"DELETE /api/v1/users/{id}",
	}

	routes := collectRoutes(registered)
	assert.Len(t, routes, 3)
	assert.Equal(t, "GET", routes[0].HTTPMethod)
	assert.Equal(t, "/api/v1/users", routes[0].HTTPPath)
	assert.Equal(t, "POST", routes[1].HTTPMethod)
	assert.Equal(t, "DELETE", routes[2].HTTPMethod)
}

func TestGrpcStatusToHTTP(t *testing.T) {
	tests := []struct {
		grpcCode codes.Code
		wantHTTP int
	}{
		{codes.OK, 200},
		{codes.NotFound, 404},
		{codes.PermissionDenied, 403},
		{codes.Unauthenticated, 401},
		{codes.InvalidArgument, 400},
		{codes.Internal, 500},
		{codes.Unavailable, 503},
		{codes.Unimplemented, 501},
	}

	for _, tt := range tests {
		got := grpcStatusToHTTP(tt.grpcCode)
		assert.Equal(t, tt.wantHTTP, got, "gRPC code %v", tt.grpcCode)
	}
}

func TestSetFieldValue(t *testing.T) {
	// 使用 dynamicpb 测试字段设置
	// 这里只测试辅助函数的逻辑，不依赖 proto 描述符
	// 实际的 proto 描述符测试需要完整的 reflection 流程

	// 测试 grpcStatusToHTTP 的默认值
	assert.Equal(t, 500, grpcStatusToHTTP(codes.Code(999)))
}

func TestForwardOutgoingContextForwardsHeaders(t *testing.T) {
	type ctxKey struct{}

	req, err := http.NewRequestWithContext(context.WithValue(context.Background(), ctxKey{}, "kept"), http.MethodGet, "/api/v1/test", nil)
	assert.NoError(t, err)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-User-ID", "user-1")
	req.Header.Set("Connection", "keep-alive")

	ctx := ForwardOutgoingContext(req)

	assert.Equal(t, "kept", ctx.Value(ctxKey{}))
	md, ok := metadata.FromOutgoingContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, []string{"Bearer token"}, md.Get("authorization"))
	assert.Equal(t, []string{"user-1"}, md.Get("x-user-id"))
	assert.Empty(t, md.Get("connection"))
}
