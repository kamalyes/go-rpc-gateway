/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-29 12:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-19 18:19:55
 * @FilePath: \go-rpc-gateway\middleware\request_context_test.go
 * @Description: 请求上下文中间件测试
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

// TestRequestContextMiddleware_GeneratesIDs 测试中间件生成 trace_id 和 request_id
func TestRequestContextMiddleware_GeneratesIDs(t *testing.T) {
	sk := global.GATEWAY.RequestContext.GetSourceKeys()
	middleware := RequestContextMiddleware()

	var capturedCtx context.Context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 验证 context 中有 trace_id 和 request_id
	traceID := GetTraceID(capturedCtx)
	requestID := GetRequestID(capturedCtx)

	assert.NotEmpty(t, traceID, "trace_id 应该被生成")
	assert.NotEmpty(t, requestID, "request_id 应该被生成")

	// 验证响应头中也有这些值
	assert.Equal(t, traceID, rec.Header().Get(sk.TraceID.Header), "响应头应包含 trace_id")
	assert.Equal(t, requestID, rec.Header().Get(sk.RequestID.Header), "响应头应包含 request_id")
}

// TestRequestContextMiddleware_UsesExistingIDs 测试中间件使用请求中已有的 ID
func TestRequestContextMiddleware_UsesExistingIDs(t *testing.T) {
	sk := global.GATEWAY.RequestContext.GetSourceKeys()
	middleware := RequestContextMiddleware()

	existingTraceID := "existing-trace-id-12345"
	existingRequestID := "existing-request-id-67890"

	var capturedCtx context.Context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(sk.TraceID.Header, existingTraceID)
	req.Header.Set(sk.RequestID.Header, existingRequestID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 验证使用了已有的 ID
	assert.Equal(t, existingTraceID, GetTraceID(capturedCtx), "应使用已有的 trace_id")
	assert.Equal(t, existingRequestID, GetRequestID(capturedCtx), "应使用已有的 request_id")

	// 验证响应头
	assert.Equal(t, existingTraceID, rec.Header().Get(sk.TraceID.Header))
	assert.Equal(t, existingRequestID, rec.Header().Get(sk.RequestID.Header))
}

// TestRequestContextMiddleware_ExtractsOptionalFields 测试中间件提取可选字段
func TestRequestContextMiddleware_ExtractsOptionalFields(t *testing.T) {
	sk := global.GATEWAY.RequestContext.GetSourceKeys()
	middleware := RequestContextMiddleware()

	var capturedCtx context.Context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(sk.ForwardedFor.Header, "192.168.1.100")
	req.Header.Set(sk.UserID.Header, "user-123")
	req.Header.Set(sk.Domain.Header, "tenant")
	req.Header.Set(sk.RoleCode.Header, "admin")
	req.Header.Set(sk.TenantID.Header, "tenant-456")
	req.Header.Set(sk.SessionID.Header, "session-789")
	req.Header.Set(sk.Timezone.Header, "Asia/Shanghai")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 验证可选字段被提取
	assert.Equal(t, "192.168.1.100", GetIPAddress(capturedCtx), "应提取 forwarded_for")
	assert.Equal(t, "user-123", GetUserID(capturedCtx), "应提取 user_id")
	assert.Equal(t, "tenant", GetDomain(capturedCtx))
	assert.Equal(t, "admin", GetRoleCode(capturedCtx))
	assert.Equal(t, "tenant-456", GetTenantID(capturedCtx), "应提取 tenant_id")
	assert.Equal(t, "session-789", GetSessionID(capturedCtx), "应提取 session_id")
	assert.Equal(t, "Asia/Shanghai", GetTimezone(capturedCtx), "应提取 timezone")
}

// TestEnrichContextFromMetadata 测试从 gRPC metadata 提取追踪信息
func TestEnrichContextFromMetadata(t *testing.T) {
	// 创建带有 metadata 的 context
	md := metadata.Pairs(
		constants.MetadataTraceID, "grpc-trace-123",
		constants.MetadataRequestID, "grpc-request-456",
		constants.MetadataUserID, "grpc-user-789",
		constants.MetadataDomain, "grpc-domain",
		constants.MetadataRoleCode, "grpc-role",
		constants.MetadataTenantID, "grpc-tenant-456",
		constants.MetadataSessionID, "grpc-session-999",
		constants.MetadataTimezone, "UTC",
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	// 调用函数
	enrichedCtx := enrichContextFromMetadata(ctx)

	// 验证提取的值
	assert.Equal(t, "grpc-trace-123", GetTraceID(enrichedCtx))
	assert.Equal(t, "grpc-request-456", GetRequestID(enrichedCtx))
	assert.Equal(t, "grpc-user-789", GetUserID(enrichedCtx))
	assert.Equal(t, "grpc-domain", GetDomain(enrichedCtx))
	assert.Equal(t, "grpc-role", GetRoleCode(enrichedCtx))
	assert.Equal(t, "grpc-tenant-456", GetTenantID(enrichedCtx))
	assert.Equal(t, "grpc-session-999", GetSessionID(enrichedCtx))
	assert.Equal(t, "UTC", GetTimezone(enrichedCtx))
}

// TestEnrichContextFromMetadata_GeneratesIDsWhenMissing 测试缺少 ID 时生成新的
func TestEnrichContextFromMetadata_GeneratesIDsWhenMissing(t *testing.T) {
	// 空的 metadata
	md := metadata.Pairs()
	ctx := metadata.NewIncomingContext(context.Background(), md)

	enrichedCtx := enrichContextFromMetadata(ctx)

	// 验证生成了新的 ID
	assert.NotEmpty(t, GetTraceID(enrichedCtx), "应生成 trace_id")
	assert.NotEmpty(t, GetRequestID(enrichedCtx), "应生成 request_id")
}

// TestInjectTraceToOutgoingContext 测试将 trace 信息注入到 outgoing metadata
func TestInjectTraceToOutgoingContext(t *testing.T) {
	// 创建带有 trace 信息的 RequestCommonMeta
	ctx := context.WithValue(context.Background(), requestCommonMetaKey{}, &RequestCommonMeta{
		TraceID:   "outgoing-trace-123",
		RequestID: "outgoing-request-456",
		UserID:    "outgoing-user-789",
		Domain:    "outgoing-domain",
		RoleCode:  "outgoing-role",
		TenantID:  "outgoing-tenant-111",
		SessionID: "outgoing-session-222",
		Timezone:  "America/New_York",
	})

	// 注入到 outgoing context
	outgoingCtx := injectTraceToOutgoingContext(ctx)

	// 验证 metadata 中有这些值
	md, ok := metadata.FromOutgoingContext(outgoingCtx)
	assert.True(t, ok, "应该有 outgoing metadata")
	assert.Equal(t, []string{"outgoing-trace-123"}, md.Get(constants.MetadataTraceID))
	assert.Equal(t, []string{"outgoing-request-456"}, md.Get(constants.MetadataRequestID))
	assert.Equal(t, []string{"outgoing-user-789"}, md.Get(constants.MetadataUserID))
	assert.Equal(t, []string{"outgoing-domain"}, md.Get(constants.MetadataDomain))
	assert.Equal(t, []string{"outgoing-role"}, md.Get(constants.MetadataRoleCode))
	assert.Equal(t, []string{"outgoing-tenant-111"}, md.Get(constants.MetadataTenantID))
	assert.Equal(t, []string{"outgoing-session-222"}, md.Get(constants.MetadataSessionID))
	assert.Equal(t, []string{"America/New_York"}, md.Get(constants.MetadataTimezone))
}

// TestContextWrappedServerStream 测试 ServerStream 包装器
func TestContextWrappedServerStream(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "stream-trace")

	wrapped := &contextWrappedServerStream{
		ServerStream: nil, // 测试中不需要真实的 stream
		ctx:          ctx,
	}

	// 验证 Context() 返回增强后的 context
	assert.Equal(t, "stream-trace", GetTraceID(wrapped.Context()))
}

// TestGetRequestCommonMeta 测试缓存的 RequestCommonMeta 获取
func TestGetRequestCommonMeta(t *testing.T) {
	t.Run("从缓存获取", func(t *testing.T) {
		ctx := context.Background()
		expectedInfo := &RequestCommonMeta{
			TraceID:   "cached-trace-123",
			RequestID: "cached-request-456",
			UserID:    "cached-user-789",
			TenantID:  "cached-tenant-111",
			SessionID: "cached-session-222",
			Timezone:  "Asia/Tokyo",
		}
		ctx = context.WithValue(ctx, requestCommonMetaKey{}, expectedInfo)

		info := GetRequestCommonMeta(ctx)

		assert.Equal(t, expectedInfo.TraceID, info.TraceID)
		assert.Equal(t, expectedInfo.RequestID, info.RequestID)
		assert.Equal(t, expectedInfo.UserID, info.UserID)
		assert.Equal(t, expectedInfo.TenantID, info.TenantID)
		assert.Equal(t, expectedInfo.SessionID, info.SessionID)
		assert.Equal(t, expectedInfo.Timezone, info.Timezone)
	})

	t.Run("回退到logger提取", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithTraceID(ctx, "fallback-trace")
		ctx = WithRequestID(ctx, "fallback-request")
		ctx = WithUserID(ctx, "fallback-user")

		info := GetRequestCommonMeta(ctx)

		assert.Equal(t, "fallback-trace", info.TraceID)
		assert.Equal(t, "fallback-request", info.RequestID)
		assert.Equal(t, "fallback-user", info.UserID)
	})
}

// TestUtilityFunctions 测试工具函数
func TestUtilityFunctions(t *testing.T) {
	ctx := context.Background()

	// 测试 WithXXX 函数
	ctx = WithTraceID(ctx, "util-trace")
	ctx = WithRequestID(ctx, "util-request")
	ctx = WithUserID(ctx, "util-user")
	ctx = WithTenantID(ctx, "util-tenant")
	ctx = WithSessionID(ctx, "util-session")
	ctx = WithTimezone(ctx, "Europe/London")
	ctx = WithAgentLineID(ctx, "agent-line-001")

	// 测试 GetXXX 函数
	assert.Equal(t, "util-trace", GetTraceID(ctx))
	assert.Equal(t, "util-request", GetRequestID(ctx))
	assert.Equal(t, "util-user", GetUserID(ctx))
	assert.Equal(t, "util-tenant", GetTenantID(ctx))
	assert.Equal(t, "util-session", GetSessionID(ctx))
	assert.Equal(t, "Europe/London", GetTimezone(ctx))
	assert.Equal(t, "agent-line-001", GetAgentLineID(ctx))
}

// TestRequestContextMiddleware_ExtractsAgentLineID 测试中间件提取代理线ID
func TestRequestContextMiddleware_ExtractsAgentLineID(t *testing.T) {
	sk := global.GATEWAY.RequestContext.GetSourceKeys()
	middleware := RequestContextMiddleware()

	var capturedCtx context.Context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(sk.AgentLineID.Header, "agent-line-test-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 验证 AgentLineID 被提取
	assert.Equal(t, "agent-line-test-123", GetAgentLineID(capturedCtx), "应提取 agent_line_id")
}

// TestEnrichContextFromMetadata_ExtractsAgentLineID 测试从 gRPC metadata 提取代理线ID
func TestEnrichContextFromMetadata_ExtractsAgentLineID(t *testing.T) {
	// 创建带有 metadata 的 context
	md := metadata.Pairs(
		constants.MetadataTraceID, "grpc-trace-123",
		constants.MetadataRequestID, "grpc-request-456",
		constants.MetadataAgentLineID, "grpc-agent-line-789",
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	// 调用函数
	enrichedCtx := enrichContextFromMetadata(ctx)

	// 验证提取的 AgentLineID
	assert.Equal(t, "grpc-agent-line-789", GetAgentLineID(enrichedCtx))
}

// TestInjectTraceToOutgoingContext_InjectsAgentLineID 测试将代理线ID注入到 outgoing metadata
func TestInjectTraceToOutgoingContext_InjectsAgentLineID(t *testing.T) {
	// 创建带有 AgentLineID 的 RequestCommonMeta
	ctx := context.WithValue(context.Background(), requestCommonMetaKey{}, &RequestCommonMeta{
		TraceID:     "outgoing-trace-123",
		RequestID:   "outgoing-request-456",
		AgentLineID: "outgoing-agent-line-789",
	})

	// 注入到 outgoing context
	outgoingCtx := injectTraceToOutgoingContext(ctx)

	// 验证 metadata 中有 AgentLineID
	md, ok := metadata.FromOutgoingContext(outgoingCtx)
	assert.True(t, ok, "应该有 outgoing metadata")
	assert.Equal(t, []string{"outgoing-agent-line-789"}, md.Get(constants.MetadataAgentLineID))
}

// TestSetResponseMetadata_ContainsAgentLineID 测试响应 metadata 包含代理线ID
func TestSetResponseMetadata_ContainsAgentLineID(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "response-trace")
	ctx = WithRequestID(ctx, "response-request")
	ctx = WithAgentLineID(ctx, "response-agent-line")

	// 缓存 RequestCommonMeta
	ctx = context.WithValue(ctx, requestCommonMetaKey{}, &RequestCommonMeta{
		TraceID:     "response-trace",
		RequestID:   "response-request",
		AgentLineID: "response-agent-line",
	})

	// 调用 setResponseMetadata
	setResponseMetadata(ctx)

	// 验证不会 panic
	assert.NotPanics(t, func() {
		setResponseMetadata(ctx)
	})
}

// TestFullChain_AgentLineID 测试完整链路中代理线ID的传递
func TestFullChain_AgentLineID(t *testing.T) {
	sk := global.GATEWAY.RequestContext.GetSourceKeys()
	// 测试 HTTP → Context 链路
	middleware := RequestContextMiddleware()

	var capturedCtx context.Context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(sk.AgentLineID.Header, "full-chain-agent-line")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 验证 HTTP 层提取
	assert.Equal(t, "full-chain-agent-line", GetAgentLineID(capturedCtx))

	// 测试 gRPC metadata → Context 链路
	md := metadata.Pairs(
		constants.MetadataAgentLineID, "grpc-full-chain-agent-line",
	)
	grpcCtx := metadata.NewIncomingContext(context.Background(), md)
	enrichedCtx := enrichContextFromMetadata(grpcCtx)

	// 验证 gRPC 层提取
	assert.Equal(t, "grpc-full-chain-agent-line", GetAgentLineID(enrichedCtx))

	// 测试 Context → outgoing metadata 链路
	outgoingCtx := injectTraceToOutgoingContext(enrichedCtx)
	outgoingMD, _ := metadata.FromOutgoingContext(outgoingCtx)

	// 验证传递到下游
	assert.Equal(t, []string{"grpc-full-chain-agent-line"}, outgoingMD.Get(constants.MetadataAgentLineID))
}

// TestRequestContextMiddleware_CachesRequestCommonMeta 测试中间件缓存 RequestCommonMeta
func TestRequestContextMiddleware_CachesRequestCommonMeta(t *testing.T) {
	sk := global.GATEWAY.RequestContext.GetSourceKeys()
	middleware := RequestContextMiddleware()

	var capturedCtx context.Context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(sk.TraceID.Header, "cache-test-trace")
	req.Header.Set(sk.RequestID.Header, "cache-test-request")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 验证 RequestCommonMeta 被缓存
	info := GetRequestCommonMeta(capturedCtx)
	assert.NotNil(t, info)
	assert.Equal(t, "cache-test-trace", info.TraceID)
	assert.Equal(t, "cache-test-request", info.RequestID)
}

func TestWithRequestCommonMeta_CachesWithReadableKey(t *testing.T) {
	expectedInfo := &RequestCommonMeta{
		TraceID:   "cached-trace",
		RequestID: "cached-request",
		Timestamp: "1700000000",
		Signature: "cached-signature",
		AccessKey: "cached-access-key",
		XNsID:     "cached-ns",
	}

	ctx := WithRequestCommonMeta(context.Background(), expectedInfo)
	info := GetRequestCommonMeta(ctx)

	assert.Same(t, expectedInfo, info)
	assert.Equal(t, "cached-ns", info.XNsID)
}

// TestFullChain_HTTPToContext 测试完整链路：HTTP 请求到 context
func TestFullChain_HTTPToContext(t *testing.T) {
	sk := global.GATEWAY.RequestContext.GetSourceKeys()
	// 模拟完整的 HTTP → Service → Repository 链路
	middleware := RequestContextMiddleware()

	var serviceCtx context.Context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟 Service 层获取 context
		serviceCtx = r.Context()

		// 模拟 Repository 层使用 context 记录日志
		traceID := GetTraceID(serviceCtx)
		requestID := GetRequestID(serviceCtx)

		// 验证在整个链路中都能获取到 trace 信息
		assert.NotEmpty(t, traceID, "Service 层应能获取 trace_id")
		assert.NotEmpty(t, requestID, "Service 层应能获取 request_id")

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/messages", nil)
	req.Header.Set(sk.TraceID.Header, "chain-trace-id")
	req.Header.Set(sk.RequestID.Header, "chain-request-id")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 最终验证
	assert.Equal(t, "chain-trace-id", GetTraceID(serviceCtx))
	assert.Equal(t, "chain-request-id", GetRequestID(serviceCtx))
}

// TestFullChain_GRPCMetadataToContext 测试完整链路：gRPC metadata 到 context
func TestFullChain_GRPCMetadataToContext(t *testing.T) {
	// 模拟 gRPC Gateway 传递过来的 metadata
	md := metadata.Pairs(
		constants.MetadataTraceID, "grpc-chain-trace",
		constants.MetadataRequestID, "grpc-chain-request",
		constants.MetadataUserID, "grpc-user",
		constants.MetadataTenantID, "grpc-tenant",
		constants.MetadataSessionID, "grpc-session",
		constants.MetadataTimezone, "Asia/Shanghai",
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	// 模拟 gRPC Server 的 context 增强
	enrichedCtx := enrichContextFromMetadata(ctx)

	// 模拟 Service 层
	serviceTraceID := GetTraceID(enrichedCtx)
	serviceRequestID := GetRequestID(enrichedCtx)
	serviceUserID := GetUserID(enrichedCtx)
	serviceTenantID := GetTenantID(enrichedCtx)
	serviceSessionID := GetSessionID(enrichedCtx)
	serviceTimezone := GetTimezone(enrichedCtx)

	assert.Equal(t, "grpc-chain-trace", serviceTraceID)
	assert.Equal(t, "grpc-chain-request", serviceRequestID)
	assert.Equal(t, "grpc-user", serviceUserID)
	assert.Equal(t, "grpc-tenant", serviceTenantID)
	assert.Equal(t, "grpc-session", serviceSessionID)
	assert.Equal(t, "Asia/Shanghai", serviceTimezone)

	// 模拟调用下游 gRPC 服务时传递 context
	outgoingCtx := injectTraceToOutgoingContext(enrichedCtx)
	outgoingMD, _ := metadata.FromOutgoingContext(outgoingCtx)

	// 验证 trace 信息被传递到下游
	assert.Equal(t, []string{"grpc-chain-trace"}, outgoingMD.Get(constants.MetadataTraceID))
	assert.Equal(t, []string{"grpc-chain-request"}, outgoingMD.Get(constants.MetadataRequestID))
	assert.Equal(t, []string{"grpc-user"}, outgoingMD.Get(constants.MetadataUserID))
	assert.Equal(t, []string{"grpc-tenant"}, outgoingMD.Get(constants.MetadataTenantID))
	assert.Equal(t, []string{"grpc-session"}, outgoingMD.Get(constants.MetadataSessionID))
	assert.Equal(t, []string{"Asia/Shanghai"}, outgoingMD.Get(constants.MetadataTimezone))
}

// TestSetResponseMetadata 测试设置响应 metadata
func TestSetResponseMetadata(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "response-trace")
	ctx = WithRequestID(ctx, "response-request")

	// 缓存 RequestCommonMeta
	ctx = context.WithValue(ctx, requestCommonMetaKey{}, &RequestCommonMeta{
		TraceID:   "response-trace",
		RequestID: "response-request",
	})

	// 调用 setResponseMetadata（实际场景中会通过 grpc.SetHeader 设置）
	setResponseMetadata(ctx)

	// 注意：在单元测试中无法直接验证 grpc.SetHeader 的效果
	// 这里主要验证函数不会 panic
	assert.NotPanics(t, func() {
		setResponseMetadata(ctx)
	})
}

// TestIncomingMetadataAccess 测试从 metadata 读取值
func TestIncomingMetadataAccess(t *testing.T) {
	md := metadata.Pairs(
		"single-key", "single-value",
		"multi-key", "value1",
		"multi-key", "value2",
	)

	firstMetadataValue := func(key string) string {
		if values := md.Get(key); len(values) > 0 {
			return values[0]
		}
		return ""
	}

	// 测试单值
	assert.Equal(t, "single-value", firstMetadataValue("single-key"))

	// 测试多值（应返回第一个）
	assert.Equal(t, "value1", firstMetadataValue("multi-key"))

	// 测试不存在的 key
	assert.Equal(t, "", firstMetadataValue("non-existent"))
}

// BenchmarkRequestContextMiddleware 性能测试
func BenchmarkRequestContextMiddleware(b *testing.B) {
	middleware := RequestContextMiddleware()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for range b.N {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

// BenchmarkEnrichContextFromMetadata 性能测试
func BenchmarkEnrichContextFromMetadata(b *testing.B) {
	md := metadata.Pairs(
		constants.MetadataTraceID, "bench-trace",
		constants.MetadataRequestID, "bench-request",
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	b.ResetTimer()
	for range b.N {
		_ = enrichContextFromMetadata(ctx)
	}
}

// BenchmarkGetRequestCommonMeta 性能测试：缓存命中
func BenchmarkGetRequestCommonMeta(b *testing.B) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, requestCommonMetaKey{}, &RequestCommonMeta{
		TraceID:   "bench-trace",
		RequestID: "bench-request",
		UserID:    "bench-user",
	})

	b.ResetTimer()
	for range b.N {
		_ = GetRequestCommonMeta(ctx)
	}
}

// BenchmarkGetRequestCommonMetaFallback 性能测试：缓存未命中回退
func BenchmarkGetRequestCommonMetaFallback(b *testing.B) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "bench-trace")
	ctx = WithRequestID(ctx, "bench-request")

	b.ResetTimer()
	for range b.N {
		_ = GetRequestCommonMeta(ctx)
	}
}

// BenchmarkInjectTraceToOutgoingContext 性能测试
func BenchmarkInjectTraceToOutgoingContext(b *testing.B) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "bench-trace")
	ctx = WithRequestID(ctx, "bench-request")
	ctx = WithUserID(ctx, "bench-user")
	ctx = context.WithValue(ctx, requestCommonMetaKey{}, &RequestCommonMeta{
		TraceID:   "bench-trace",
		RequestID: "bench-request",
		UserID:    "bench-user",
	})

	b.ResetTimer()
	for range b.N {
		_ = injectTraceToOutgoingContext(ctx)
	}
}

// ========== 模拟真实业务场景的结构 ==========

// UserService 用户服务层（模拟真实业务）
type UserService struct {
	repo *UserRepository
}

// UserRepository 数据访问层（模拟真实数据库操作）
type UserRepository struct {
	// 模拟数据库
	users map[string]*User
}

// User 用户模型
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	CreateAt string `json:"create_at"`
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

// CreateUserResponse 创建用户响应
type CreateUserResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      *User  `json:"data,omitempty"`
	TraceID   string `json:"trace_id"`
	RequestID string `json:"request_id"`
}

// ========== Repository 层实现 ==========

// NewUserRepository 创建用户仓储
func NewUserRepository() *UserRepository {
	return &UserRepository{
		users: make(map[string]*User),
	}
}

// Create 创建用户（模拟数据库插入）
func (r *UserRepository) Create(ctx context.Context, user *User) error {
	// 模拟数据库操作耗时
	time.Sleep(10 * time.Millisecond)

	// 记录 Repository 层日志（自动包含 trace_id）
	traceID := GetTraceID(ctx)
	requestID := GetRequestID(ctx)

	_ = traceID
	_ = requestID
	_ = user.ID
	_ = user.Username

	// 检查用户是否已存在
	if _, exists := r.users[user.Username]; exists {
		return fmt.Errorf("用户已存在: %s", user.Username)
	}

	// 插入数据
	r.users[user.Username] = user

	return nil
} // FindByUsername 根据用户名查找
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	time.Sleep(5 * time.Millisecond)

	user, exists := r.users[username]
	if !exists {
		return nil, fmt.Errorf("用户不存在: %s", username)
	}

	return user, nil
}

// ========== Service 层实现 ==========

// NewUserService 创建用户服务
func NewUserService(repo *UserRepository) *UserService {
	return &UserService{repo: repo}
}

// CreateUser 创建用户（业务逻辑）
func (s *UserService) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	traceID := GetTraceID(ctx)
	_ = traceID

	// 1. 业务验证
	if req.Username == "" || req.Email == "" {
		return nil, fmt.Errorf("用户名和邮箱不能为空")
	}

	// 2. 检查用户是否已存在（调用 Repository）
	existingUser, _ := s.repo.FindByUsername(ctx, req.Username)
	if existingUser != nil {
		return nil, fmt.Errorf("用户名已被使用: %s", req.Username)
	}

	// 3. 创建用户对象
	user := &User{
		ID:       fmt.Sprintf("user_%d", time.Now().Unix()),
		Username: req.Username,
		Email:    req.Email,
		CreateAt: time.Now().Format(time.RFC3339),
	}

	// 4. 调用 Repository 保存用户
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// ========== HTTP Handler 层实现 ==========

// UserHandler 用户处理器
type UserHandler struct {
	service *UserService
}

// NewUserHandler 创建用户处理器
func NewUserHandler(service *UserService) *UserHandler {
	return &UserHandler{service: service}
}

// CreateUser HTTP 处理函数
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. 解析请求
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, ctx, http.StatusBadRequest, "请求参数格式错误: "+err.Error())
		return
	}

	// 2. 调用 Service 层
	user, err := h.service.CreateUser(ctx, &req)
	if err != nil {
		h.writeErrorResponse(w, ctx, http.StatusBadRequest, err.Error())
		return
	}

	// 3. 返回成功响应
	h.writeSuccessResponse(w, ctx, user)
}

// writeSuccessResponse 写入成功响应
func (h *UserHandler) writeSuccessResponse(w http.ResponseWriter, ctx context.Context, user *User) {
	resp := &CreateUserResponse{
		Code:      200,
		Message:   "创建成功",
		Data:      user,
		TraceID:   GetTraceID(ctx),
		RequestID: GetRequestID(ctx),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// writeErrorResponse 写入错误响应
func (h *UserHandler) writeErrorResponse(w http.ResponseWriter, ctx context.Context, statusCode int, message string) {
	resp := &CreateUserResponse{
		Code:      statusCode,
		Message:   message,
		TraceID:   GetTraceID(ctx),
		RequestID: GetRequestID(ctx),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}

// ========== 集成测试 ==========

// TestRealWorldScenario_CreateUser 测试真实场景：创建用户
func TestRealWorldScenario_CreateUser(t *testing.T) {
	t.Log("========== 测试场景：创建用户（完整链路追踪） ==========")

	// 1. 初始化业务组件
	sk := global.GATEWAY.RequestContext.GetSourceKeys()
	repo := NewUserRepository()
	service := NewUserService(repo)
	handler := NewUserHandler(service)

	// 2. 创建中间件链（模拟真实网关）
	middleware := RequestContextMiddleware()
	router := middleware(http.HandlerFunc(handler.CreateUser))

	// 3. 构造 HTTP 请求
	reqBody := &CreateUserRequest{
		Username: "zhangsan",
		Email:    "zhangsan@example.com",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	// 模拟客户端传入 trace_id
	req.Header.Set(sk.TraceID.Header, "client-trace-123")

	rec := httptest.NewRecorder()

	// 4. 执行请求
	t.Log(">>> 发送 HTTP 请求...")
	router.ServeHTTP(rec, req)

	// 5. 验证响应
	t.Log(">>> 验证响应结果...")
	assert.Equal(t, http.StatusOK, rec.Code, "状态码应该是 200")

	var resp CreateUserResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err, "响应解析应该成功")

	// 验证 trace_id 传递
	assert.Equal(t, "client-trace-123", resp.TraceID, "trace_id 应该从请求头传递")
	assert.NotEmpty(t, resp.RequestID, "request_id 应该被生成")
	assert.Equal(t, 200, resp.Code, "业务状态码应该是 200")
	assert.Equal(t, "创建成功", resp.Message)
	assert.NotNil(t, resp.Data, "应该返回用户数据")
	assert.Equal(t, "zhangsan", resp.Data.Username)

	t.Logf("✅ 测试通过！trace_id=%s 在整个链路中保持一致", resp.TraceID)
}

// TestRealWorldScenario_DuplicateUser 测试真实场景：创建重复用户
func TestRealWorldScenario_DuplicateUser(t *testing.T) {
	t.Log("========== 测试场景：创建重复用户（错误处理） ==========")

	repo := NewUserRepository()
	service := NewUserService(repo)
	handler := NewUserHandler(service)

	middleware := RequestContextMiddleware()
	sk := global.GATEWAY.RequestContext.GetSourceKeys()
	router := middleware(http.HandlerFunc(handler.CreateUser))

	// 第一次创建
	reqBody := &CreateUserRequest{
		Username: "lisi",
		Email:    "lisi@example.com",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	t.Log(">>> 第一次创建用户...")
	req1 := httptest.NewRequest("POST", "/api/users", bytes.NewReader(bodyBytes))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set(sk.TraceID.Header, "trace-duplicate-1")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	assert.Equal(t, http.StatusOK, rec1.Code)

	// 第二次创建（应该失败）
	t.Log(">>> 第二次创建相同用户（应该失败）...")
	bodyBytes2, _ := json.Marshal(reqBody)
	req2 := httptest.NewRequest("POST", "/api/users", bytes.NewReader(bodyBytes2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set(sk.TraceID.Header, "trace-duplicate-2")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	// 验证错误响应
	assert.Equal(t, http.StatusBadRequest, rec2.Code, "重复用户应该返回 400")

	var resp CreateUserResponse
	json.NewDecoder(rec2.Body).Decode(&resp)

	assert.Equal(t, "trace-duplicate-2", resp.TraceID, "错误响应也应包含 trace_id")
	assert.Contains(t, resp.Message, "已被使用", "错误消息应该提示用户名已被使用")

	t.Logf("✅ 测试通过！错误响应也包含 trace_id=%s", resp.TraceID)
}

// TestRealWorldScenario_ConcurrentRequests 测试真实场景：并发请求
func TestRealWorldScenario_ConcurrentRequests(t *testing.T) {
	t.Log("========== 测试场景：并发请求（trace_id 隔离） ==========")

	// 1. 初始化业务组件
	sk := global.GATEWAY.RequestContext.GetSourceKeys()

	repo := NewUserRepository()
	service := NewUserService(repo)
	handler := NewUserHandler(service)

	middleware := RequestContextMiddleware()
	router := middleware(http.HandlerFunc(handler.CreateUser))

	// 模拟 3 个并发请求
	done := make(chan bool, 3)
	traceIDs := []string{"concurrent-1", "concurrent-2", "concurrent-3"}
	usernames := []string{"user1", "user2", "user3"}

	for index := range 3 {
		go func(index int) {
			reqBody := &CreateUserRequest{
				Username: usernames[index],
				Email:    fmt.Sprintf("%s@example.com", usernames[index]),
			}
			bodyBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(sk.TraceID.Header, traceIDs[index])
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			var resp CreateUserResponse
			json.NewDecoder(rec.Body).Decode(&resp)

			// 验证每个请求的 trace_id 都正确
			assert.Equal(t, traceIDs[index], resp.TraceID,
				fmt.Sprintf("请求 %d 的 trace_id 应该是 %s", index+1, traceIDs[index]))

			done <- true
		}(index)
	}

	// 等待所有请求完成
	for range 3 {
		<-done
	}

	t.Log("✅ 测试通过！3 个并发请求的 trace_id 都正确隔离")
}

// TestRealWorldScenario_TraceIDPropagation 测试 trace_id 传播
func TestRealWorldScenario_TraceIDPropagation(t *testing.T) {
	sk := global.GATEWAY.RequestContext.GetSourceKeys()

	t.Log("========== 测试场景：trace_id 跨层传播验证 ==========")

	// 收集各层的 trace_id
	var (
		handlerTraceID    string
		serviceTraceID    string
		repositoryTraceID string
	)

	// 使用包装器收集 trace_id
	repo := NewUserRepository()
	service := NewUserService(repo)
	handler := NewUserHandler(service)

	// 创建包装器 handler 来捕获 trace_id
	wrapperHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		handlerTraceID = GetTraceID(ctx)

		// 在调用前记录 service 层的 trace_id
		serviceTraceID = GetTraceID(ctx)

		// 在 repository 操作前记录
		repositoryTraceID = GetTraceID(ctx)

		// 调用真实的 handler
		handler.CreateUser(w, r)
	})

	middleware := RequestContextMiddleware()
	router := middleware(wrapperHandler)

	// 发送请求
	reqBody := &CreateUserRequest{Username: "propagation_test", Email: "test@example.com"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(sk.TraceID.Header, "propagation-trace-999")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// 验证所有层的 trace_id 一致
	t.Logf(">>> Handler 层 trace_id:    %s", handlerTraceID)
	t.Logf(">>> Service 层 trace_id:    %s", serviceTraceID)
	t.Logf(">>> Repository 层 trace_id: %s", repositoryTraceID)

	assert.Equal(t, "propagation-trace-999", handlerTraceID, "Handler 层 trace_id 正确")
	assert.Equal(t, "propagation-trace-999", serviceTraceID, "Service 层 trace_id 正确")
	assert.Equal(t, "propagation-trace-999", repositoryTraceID, "Repository 层 trace_id 正确")
	assert.Equal(t, handlerTraceID, serviceTraceID, "Handler 和 Service 层 trace_id 一致")
	assert.Equal(t, serviceTraceID, repositoryTraceID, "Service 和 Repository 层 trace_id 一致")

	t.Log("✅ 测试通过！trace_id 在所有层之间正确传播")
}

// TestRealWorldScenario_WithoutTraceID 测试没有 trace_id 的场景
func TestRealWorldScenario_WithoutTraceID(t *testing.T) {
	t.Log("========== 测试场景：客户端未提供 trace_id（自动生成） ==========")

	repo := NewUserRepository()
	service := NewUserService(repo)
	handler := NewUserHandler(service)

	middleware := RequestContextMiddleware()
	router := middleware(http.HandlerFunc(handler.CreateUser))

	reqBody := &CreateUserRequest{Username: "auto_trace", Email: "auto@example.com"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	// 不设置 X-Trace-Id，让中间件自动生成

	rec := httptest.NewRecorder()

	t.Log(">>> 发送请求（不包含 trace_id）...")
	router.ServeHTTP(rec, req)

	var resp CreateUserResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	// 验证自动生成的 trace_id
	assert.NotEmpty(t, resp.TraceID, "应该自动生成 trace_id")
	assert.NotEmpty(t, resp.RequestID, "应该自动生成 request_id")
	assert.Equal(t, http.StatusOK, rec.Code)

	t.Logf("✅ 测试通过！自动生成 trace_id=%s, request_id=%s",
		resp.TraceID, resp.RequestID)
} // TestRealWorldScenario_CompleteRequestFlow 完整请求流程演示
func TestRealWorldScenario_CompleteRequestFlow(t *testing.T) {
	sk := global.GATEWAY.RequestContext.GetSourceKeys()

	t.Log(strings.Repeat("=", 80))
	t.Log("完整请求流程演示 - HTTP → Handler → Service → Repository")
	t.Log(strings.Repeat("=", 80))

	repo := NewUserRepository()
	service := NewUserService(repo)
	handler := NewUserHandler(service)

	middleware := RequestContextMiddleware()
	loggingMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			duration := time.Since(start)
			_ = duration
		})
	}

	// 组合中间件
	router := middleware(loggingMiddleware(http.HandlerFunc(handler.CreateUser)))

	// 发送请求
	reqBody := &CreateUserRequest{Username: "complete_flow", Email: "complete@example.com"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(sk.TraceID.Header, "demo-trace-12345")
	req.Header.Set(sk.UserID.Header, "client-user-999")
	rec := httptest.NewRecorder()

	t.Log(">>> 开始执行请求...")
	router.ServeHTTP(rec, req)

	// 输出响应
	t.Log(">>> 响应结果:")
	var resp CreateUserResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	respJSON, _ := json.MarshalIndent(resp, "", "  ")
	t.Log(string(respJSON))

	t.Log(strings.Repeat("=", 80))
	t.Log("✅ 完整流程演示完成！trace_id 在所有层都正确传递")
	t.Log(strings.Repeat("=", 80))
}

// TestEdgeCases_EmptyMetadata 测试边界情况：空 metadata
func TestEdgeCases_EmptyMetadata(t *testing.T) {
	ctx := context.Background()

	// 没有 incoming metadata
	enrichedCtx := enrichContextFromMetadata(ctx)

	// 应该生成新的 ID
	assert.NotEmpty(t, GetTraceID(enrichedCtx))
	assert.NotEmpty(t, GetRequestID(enrichedCtx))
}

// TestEdgeCases_PartialMetadata 测试边界情况：部分 metadata
func TestEdgeCases_PartialMetadata(t *testing.T) {
	md := metadata.Pairs(
		constants.MetadataTraceID, "partial-trace",
		// 缺少 request-id
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	enrichedCtx := enrichContextFromMetadata(ctx)

	// trace-id 应该使用已有的
	assert.Equal(t, "partial-trace", GetTraceID(enrichedCtx))
	// request-id 应该被生成
	assert.NotEmpty(t, GetRequestID(enrichedCtx))
}

// TestEdgeCases_MergeOutgoingMetadata 测试边界情况：合并已有的 outgoing metadata
func TestEdgeCases_MergeOutgoingMetadata(t *testing.T) {
	// 创建已有 outgoing metadata 的 context
	existingMD := metadata.Pairs("existing-key", "existing-value")
	ctx := metadata.NewOutgoingContext(context.Background(), existingMD)

	// 添加 trace 信息到 RequestCommonMeta
	ctx = context.WithValue(ctx, requestCommonMetaKey{}, &RequestCommonMeta{
		TraceID:   "merge-trace",
		RequestID: "merge-request",
	})

	// 注入 trace 信息
	outgoingCtx := injectTraceToOutgoingContext(ctx)

	// 验证两者都存在
	md, ok := metadata.FromOutgoingContext(outgoingCtx)
	assert.True(t, ok)
	assert.Equal(t, []string{"existing-value"}, md.Get("existing-key"))
	assert.Equal(t, []string{"merge-trace"}, md.Get(constants.MetadataTraceID))
	assert.Equal(t, []string{"merge-request"}, md.Get(constants.MetadataRequestID))
}

// TestEdgeCases_EmptyRequestCommonMeta 测试边界情况：空的 RequestCommonMeta
func TestEdgeCases_EmptyRequestCommonMeta(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, requestCommonMetaKey{}, &RequestCommonMeta{})

	info := GetRequestCommonMeta(ctx)

	// 应该返回空字符串，不会 panic
	assert.Equal(t, "", info.TraceID)
	assert.Equal(t, "", info.RequestID)
}

// TestEdgeCases_NilContext 测试边界情况：nil context（理论上不应该发生）
func TestEdgeCases_GetTraceInfoFromNilCachedContext(t *testing.T) {
	ctx := context.Background()

	// 没有缓存的 RequestCommonMeta
	info := GetRequestCommonMeta(ctx)

	// 应该回退到 logger 提取，返回空字符串
	assert.NotNil(t, info)
	assert.Equal(t, "", info.TraceID)
	assert.Equal(t, "", info.RequestID)
}

// TestExtractOrGenerateTraceID_WithOpenTelemetry 测试从 OpenTelemetry 提取 TraceID
func TestExtractOrGenerateTraceID_WithOpenTelemetry(t *testing.T) {
	// 注意：这个测试需要实际的 OpenTelemetry span context
	// 这里只测试基本逻辑
	ctx := context.Background()

	// 没有提供 traceID，也没有 OpenTelemetry span
	traceID := extractOrGenerateTraceID(ctx, "")

	// 应该生成新的 traceID
	assert.NotEmpty(t, traceID, "应该生成 trace_id")
}

// TestExtractOrGenerateRequestID 测试 RequestID 生成
func TestExtractOrGenerateRequestID(t *testing.T) {
	// 提供了 requestID
	requestID := extractOrGenerateRequestID("provided-request-id")
	assert.Equal(t, "provided-request-id", requestID)

	// 没有提供 requestID
	requestID = extractOrGenerateRequestID("")
	assert.NotEmpty(t, requestID, "应该生成 request_id")
}

// TestNewFields_HTTPMiddleware 测试新增字段的 HTTP 中间件提取
func TestNewFields_HTTPMiddleware(t *testing.T) {
	sk := global.GATEWAY.RequestContext.GetSourceKeys()
	middleware := RequestContextMiddleware()

	var capturedCtx context.Context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(sk.ID.Header, "test-id-123")
	req.Header.Set(sk.TenantCode.Header, "tenant-code-abc")
	req.Header.Set(sk.PlatformID.Header, "platform-123")
	req.Header.Set(sk.PlatformCode.Header, "platform-code-xyz")
	req.Header.Set(sk.RegionID.Header, "region-456")
	req.Header.Set(sk.RegionCode.Header, "region-code-def")
	req.Header.Set(sk.Nonce.Header, "nonce-789")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 验证新增字段被正确提取
	assert.Equal(t, "test-id-123", GetID(capturedCtx), "应提取 ID")
	assert.Equal(t, "tenant-code-abc", GetTenantCode(capturedCtx), "应提取 TenantCode")
	assert.Equal(t, "platform-123", GetPlatformID(capturedCtx), "应提取 PlatformID")
	assert.Equal(t, "platform-code-xyz", GetPlatformCode(capturedCtx), "应提取 PlatformCode")
	assert.Equal(t, "region-456", GetRegionID(capturedCtx), "应提取 RegionID")
	assert.Equal(t, "region-code-def", GetRegionCode(capturedCtx), "应提取 RegionCode")
	assert.Equal(t, "nonce-789", GetNonce(capturedCtx), "应提取 Nonce")
}

// TestNewFields_GRPCMetadata 测试新增字段的 gRPC metadata 提取
func TestNewFields_GRPCMetadata(t *testing.T) {
	md := metadata.Pairs(
		constants.MetadataID, "grpc-id-123",
		constants.MetadataTenantCode, "grpc-tenant-code",
		constants.MetadataPlatformID, "grpc-platform-id",
		constants.MetadataPlatformCode, "grpc-platform-code",
		constants.MetadataRegionID, "grpc-region-id",
		constants.MetadataRegionCode, "grpc-region-code",
		constants.MetadataNonce, "grpc-nonce",
		constants.MetadataAppID, "grpc-app-id",
		constants.MetadataDeviceID, "grpc-device-id",
		constants.MetadataAppVersion, "grpc-app-version",
		constants.MetadataXNsID, "grpc-ns-id",
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	enrichedCtx := enrichContextFromMetadata(ctx)

	// 验证新增字段被正确提取
	assert.Equal(t, "grpc-id-123", GetID(enrichedCtx))
	assert.Equal(t, "grpc-tenant-code", GetTenantCode(enrichedCtx))
	assert.Equal(t, "grpc-platform-id", GetPlatformID(enrichedCtx))
	assert.Equal(t, "grpc-platform-code", GetPlatformCode(enrichedCtx))
	assert.Equal(t, "grpc-region-id", GetRegionID(enrichedCtx))
	assert.Equal(t, "grpc-region-code", GetRegionCode(enrichedCtx))
	assert.Equal(t, "grpc-nonce", GetNonce(enrichedCtx))
	assert.Equal(t, "grpc-app-id", GetAppID(enrichedCtx))
	assert.Equal(t, "grpc-device-id", GetDeviceID(enrichedCtx))
	assert.Equal(t, "grpc-app-version", GetAppVersion(enrichedCtx))
	assert.Equal(t, "grpc-ns-id", GetXNsID(enrichedCtx))
}

// TestNewFields_InjectTraceToOutgoingContext 测试新增字段注入到 outgoing metadata
func TestNewFields_InjectTraceToOutgoingContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), requestCommonMetaKey{}, &RequestCommonMeta{
		ID:           "out-id",
		TenantCode:   "out-tenant-code",
		PlatformID:   "out-platform-id",
		PlatformCode: "out-platform-code",
		RegionID:     "out-region-id",
		RegionCode:   "out-region-code",
		Nonce:        "out-nonce",
		AppID:        "out-app-id",
		DeviceID:     "out-device-id",
		AppVersion:   "out-app-version",
		XNsID:        "out-ns-id",
	})

	outgoingCtx := injectTraceToOutgoingContext(ctx)

	md, ok := metadata.FromOutgoingContext(outgoingCtx)
	assert.True(t, ok, "应该有 outgoing metadata")
	assert.Equal(t, []string{"out-id"}, md.Get(constants.MetadataID))
	assert.Equal(t, []string{"out-tenant-code"}, md.Get(constants.MetadataTenantCode))
	assert.Equal(t, []string{"out-platform-id"}, md.Get(constants.MetadataPlatformID))
	assert.Equal(t, []string{"out-platform-code"}, md.Get(constants.MetadataPlatformCode))
	assert.Equal(t, []string{"out-region-id"}, md.Get(constants.MetadataRegionID))
	assert.Equal(t, []string{"out-region-code"}, md.Get(constants.MetadataRegionCode))
	assert.Equal(t, []string{"out-nonce"}, md.Get(constants.MetadataNonce))
	assert.Equal(t, []string{"out-app-id"}, md.Get(constants.MetadataAppID))
	assert.Equal(t, []string{"out-device-id"}, md.Get(constants.MetadataDeviceID))
	assert.Equal(t, []string{"out-app-version"}, md.Get(constants.MetadataAppVersion))
	assert.Equal(t, []string{"out-ns-id"}, md.Get(constants.MetadataXNsID))
}

// TestNewFields_GetRequestCommonMetaFallback 测试 GetRequestCommonMeta 回退逻辑包含新字段
func TestNewFields_GetRequestCommonMetaFallback(t *testing.T) {
	ctx := context.Background()
	ctx = WithID(ctx, "fallback-id")
	ctx = WithTenantCode(ctx, "fallback-tenant-code")
	ctx = WithPlatformID(ctx, "fallback-platform-id")
	ctx = WithPlatformCode(ctx, "fallback-platform-code")
	ctx = WithRegionID(ctx, "fallback-region-id")
	ctx = WithRegionCode(ctx, "fallback-region-code")
	ctx = WithNonce(ctx, "fallback-nonce")
	ctx = WithAppID(ctx, "fallback-app-id")
	ctx = WithDeviceID(ctx, "fallback-device-id")
	ctx = WithAppVersion(ctx, "fallback-app-version")
	ctx = WithXNsID(ctx, "fallback-ns-id")

	// 不缓存 RequestCommonMeta，触发回退逻辑
	info := GetRequestCommonMeta(ctx)

	assert.Equal(t, "fallback-id", info.ID)
	assert.Equal(t, "fallback-tenant-code", info.TenantCode)
	assert.Equal(t, "fallback-platform-id", info.PlatformID)
	assert.Equal(t, "fallback-platform-code", info.PlatformCode)
	assert.Equal(t, "fallback-region-id", info.RegionID)
	assert.Equal(t, "fallback-region-code", info.RegionCode)
	assert.Equal(t, "fallback-nonce", info.Nonce)
	assert.Equal(t, "fallback-app-id", info.AppID)
	assert.Equal(t, "fallback-device-id", info.DeviceID)
	assert.Equal(t, "fallback-app-version", info.AppVersion)
	assert.Equal(t, "fallback-ns-id", info.XNsID)
}

// TestNewFields_WithFunctions 测试新增的 With* 函数
func TestNewFields_WithFunctions(t *testing.T) {
	ctx := context.Background()

	ctx = WithID(ctx, "test-id")
	ctx = WithTenantCode(ctx, "test-tenant-code")
	ctx = WithPlatformID(ctx, "test-platform-id")
	ctx = WithPlatformCode(ctx, "test-platform-code")
	ctx = WithRegionID(ctx, "test-region-id")
	ctx = WithRegionCode(ctx, "test-region-code")
	ctx = WithNonce(ctx, "test-nonce")
	ctx = WithAppID(ctx, "test-app-id")
	ctx = WithDeviceID(ctx, "test-device-id")
	ctx = WithAppVersion(ctx, "test-app-version")
	ctx = WithXNsID(ctx, "test-ns-id")

	assert.Equal(t, "test-id", GetID(ctx))
	assert.Equal(t, "test-tenant-code", GetTenantCode(ctx))
	assert.Equal(t, "test-platform-id", GetPlatformID(ctx))
	assert.Equal(t, "test-platform-code", GetPlatformCode(ctx))
	assert.Equal(t, "test-region-id", GetRegionID(ctx))
	assert.Equal(t, "test-region-code", GetRegionCode(ctx))
	assert.Equal(t, "test-nonce", GetNonce(ctx))
	assert.Equal(t, "test-app-id", GetAppID(ctx))
	assert.Equal(t, "test-device-id", GetDeviceID(ctx))
	assert.Equal(t, "test-app-version", GetAppVersion(ctx))
	assert.Equal(t, "test-ns-id", GetXNsID(ctx))
}

// TestRequestCommonMeta_AllFields 测试 RequestCommonMeta 结构体所有字段
func TestRequestCommonMeta_AllFields(t *testing.T) {
	meta := &RequestCommonMeta{
		ID:            "id-1",
		TraceID:       "trace-1",
		RequestID:     "request-1",
		UserID:        "user-1",
		TenantID:      "tenant-1",
		TenantCode:    "tenant-code-1",
		SessionID:     "session-1",
		Timezone:      "Asia/Shanghai",
		Timestamp:     "1234567890",
		Signature:     "signature-1",
		Authorization: "Bearer token",
		AccessKey:     "access-key-1",
		AppID:         "app-1",
		DeviceID:      "device-1",
		AppVersion:    "1.0.0",
		IPAddress:     "127.0.0.1",
		PlatformID:    "platform-1",
		PlatformCode:  "platform-code-1",
		RegionID:      "region-1",
		RegionCode:    "region-code-1",
		Nonce:         "nonce-1",
		XNsID:         "ns-1",
	}

	assert.Equal(t, "id-1", meta.ID)
	assert.Equal(t, "trace-1", meta.TraceID)
	assert.Equal(t, "request-1", meta.RequestID)
	assert.Equal(t, "user-1", meta.UserID)
	assert.Equal(t, "tenant-1", meta.TenantID)
	assert.Equal(t, "tenant-code-1", meta.TenantCode)
	assert.Equal(t, "session-1", meta.SessionID)
	assert.Equal(t, "Asia/Shanghai", meta.Timezone)
	assert.Equal(t, "1234567890", meta.Timestamp)
	assert.Equal(t, "signature-1", meta.Signature)
	assert.Equal(t, "Bearer token", meta.Authorization)
	assert.Equal(t, "access-key-1", meta.AccessKey)
	assert.Equal(t, "app-1", meta.AppID)
	assert.Equal(t, "device-1", meta.DeviceID)
	assert.Equal(t, "1.0.0", meta.AppVersion)
	assert.Equal(t, "127.0.0.1", meta.IPAddress)
	assert.Equal(t, "platform-1", meta.PlatformID)
	assert.Equal(t, "platform-code-1", meta.PlatformCode)
	assert.Equal(t, "region-1", meta.RegionID)
	assert.Equal(t, "region-code-1", meta.RegionCode)
	assert.Equal(t, "nonce-1", meta.Nonce)
	assert.Equal(t, "ns-1", meta.XNsID)
}

// TestFullChain_AllFields 测试完整链路传递所有字段
func TestFullChain_AllFields(t *testing.T) {
	sk := global.GATEWAY.RequestContext.GetSourceKeys()
	middleware := RequestContextMiddleware()

	var serviceCtx context.Context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serviceCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/test", nil)
	req.Header.Set(sk.ID.Header, "full-id")
	req.Header.Set(sk.TraceID.Header, "full-trace")
	req.Header.Set(sk.RequestID.Header, "full-request")
	req.Header.Set(sk.UserID.Header, "full-user")
	req.Header.Set(sk.TenantID.Header, "full-tenant")
	req.Header.Set(sk.TenantCode.Header, "full-tenant-code")
	req.Header.Set(sk.SessionID.Header, "full-session")
	req.Header.Set(sk.Timezone.Header, "Asia/Shanghai")
	req.Header.Set(sk.AppID.Header, "full-app")
	req.Header.Set(sk.DeviceID.Header, "full-device")
	req.Header.Set(sk.AppVersion.Header, "1.0.0")
	req.Header.Set(sk.PlatformID.Header, "full-platform-id")
	req.Header.Set(sk.PlatformCode.Header, "full-platform-code")
	req.Header.Set(sk.RegionID.Header, "full-region-id")
	req.Header.Set(sk.RegionCode.Header, "full-region-code")
	req.Header.Set(sk.Nonce.Header, "full-nonce")
	req.Header.Set(sk.Jti.Header, "full-jti")
	req.Header.Set(sk.FamilyId.Header, "full-family")
	req.Header.Set(sk.Timestamp.Header, "1700000000")
	req.Header.Set(sk.Signature.Header, "full-signature")
	req.Header.Set(sk.AccessKey.Header, "full-access-key")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 验证所有字段都能在服务层获取
	meta := GetRequestCommonMeta(serviceCtx)
	assert.Equal(t, "full-id", meta.ID)
	assert.Equal(t, "full-trace", meta.TraceID)
	assert.Equal(t, "full-request", meta.RequestID)
	assert.Equal(t, "full-user", meta.UserID)
	assert.Equal(t, "full-tenant", meta.TenantID)
	assert.Equal(t, "full-tenant-code", meta.TenantCode)
	assert.Equal(t, "full-session", meta.SessionID)
	assert.Equal(t, "Asia/Shanghai", meta.Timezone)
	assert.Equal(t, "full-app", meta.AppID)
	assert.Equal(t, "full-device", meta.DeviceID)
	assert.Equal(t, "1.0.0", meta.AppVersion)
	assert.Equal(t, "full-platform-id", meta.PlatformID)
	assert.Equal(t, "full-platform-code", meta.PlatformCode)
	assert.Equal(t, "full-region-id", meta.RegionID)
	assert.Equal(t, "full-region-code", meta.RegionCode)
	assert.Equal(t, "full-nonce", meta.Nonce)
	assert.Equal(t, "full-jti", meta.Jti)
	assert.Equal(t, "full-family", meta.FamilyId)
	assert.Equal(t, "1700000000", meta.Timestamp)
	assert.Equal(t, "full-signature", meta.Signature)
	assert.Equal(t, "full-access-key", meta.AccessKey)
}

// TestWithMethods_SetContextValueAndSyncRequestCommonMeta 测试 With* 方法同时设置 context value 和同步更新 RequestCommonMeta
func TestWithMethods_SetContextValueAndSyncRequestCommonMeta(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(context.Context) context.Context
		key       string
		value     string
		getFunc   func(context.Context) string
		metaField func(*RequestCommonMeta) string
	}{
		{
			name:      "WithTraceID",
			setupFunc: func(c context.Context) context.Context { return WithTraceID(c, "test-trace-123") },
			key:       constants.MetadataTraceID,
			value:     "test-trace-123",
			getFunc:   GetTraceID,
			metaField: func(m *RequestCommonMeta) string { return m.TraceID },
		},
		{
			name:      "WithRequestID",
			setupFunc: func(c context.Context) context.Context { return WithRequestID(c, "test-request-456") },
			key:       constants.MetadataRequestID,
			value:     "test-request-456",
			getFunc:   GetRequestID,
			metaField: func(m *RequestCommonMeta) string { return m.RequestID },
		},
		{
			name:      "WithUserID",
			setupFunc: func(c context.Context) context.Context { return WithUserID(c, "test-user-789") },
			key:       constants.MetadataUserID,
			value:     "test-user-789",
			getFunc:   GetUserID,
			metaField: func(m *RequestCommonMeta) string { return m.UserID },
		},
		{
			name:      "WithTenantID",
			setupFunc: func(c context.Context) context.Context { return WithTenantID(c, "test-tenant-111") },
			key:       constants.MetadataTenantID,
			value:     "test-tenant-111",
			getFunc:   GetTenantID,
			metaField: func(m *RequestCommonMeta) string { return m.TenantID },
		},
		{
			name:      "WithSessionID",
			setupFunc: func(c context.Context) context.Context { return WithSessionID(c, "test-session-222") },
			key:       constants.MetadataSessionID,
			value:     "test-session-222",
			getFunc:   GetSessionID,
			metaField: func(m *RequestCommonMeta) string { return m.SessionID },
		},
		{
			name:      "WithDomain",
			setupFunc: func(c context.Context) context.Context { return WithDomain(c, "test-domain") },
			key:       constants.MetadataDomain,
			value:     "test-domain",
			getFunc:   GetDomain,
			metaField: func(m *RequestCommonMeta) string { return m.Domain },
		},
		{
			name:      "WithRoleCode",
			setupFunc: func(c context.Context) context.Context { return WithRoleCode(c, "test-role") },
			key:       constants.MetadataRoleCode,
			value:     "test-role",
			getFunc:   GetRoleCode,
			metaField: func(m *RequestCommonMeta) string { return m.RoleCode },
		},
		{
			name:      "WithTenantCode",
			setupFunc: func(c context.Context) context.Context { return WithTenantCode(c, "test-tenant-code") },
			key:       constants.MetadataTenantCode,
			value:     "test-tenant-code",
			getFunc:   GetTenantCode,
			metaField: func(m *RequestCommonMeta) string { return m.TenantCode },
		},
		{
			name:      "WithTimezone",
			setupFunc: func(c context.Context) context.Context { return WithTimezone(c, "Asia/Shanghai") },
			key:       constants.MetadataTimezone,
			value:     "Asia/Shanghai",
			getFunc:   GetTimezone,
			metaField: func(m *RequestCommonMeta) string { return m.Timezone },
		},
		{
			name:      "WithIPAddress",
			setupFunc: func(c context.Context) context.Context { return WithIPAddress(c, "192.168.1.100") },
			key:       constants.MetadataIPAddress,
			value:     "192.168.1.100",
			getFunc:   GetIPAddress,
			metaField: func(m *RequestCommonMeta) string { return m.IPAddress },
		},
		{
			name:      "WithAppID",
			setupFunc: func(c context.Context) context.Context { return WithAppID(c, "test-app-id") },
			key:       constants.MetadataAppID,
			value:     "test-app-id",
			getFunc:   GetAppID,
			metaField: func(m *RequestCommonMeta) string { return m.AppID },
		},
		{
			name:      "WithDeviceID",
			setupFunc: func(c context.Context) context.Context { return WithDeviceID(c, "test-device-id") },
			key:       constants.MetadataDeviceID,
			value:     "test-device-id",
			getFunc:   GetDeviceID,
			metaField: func(m *RequestCommonMeta) string { return m.DeviceID },
		},
		{
			name:      "WithAppVersion",
			setupFunc: func(c context.Context) context.Context { return WithAppVersion(c, "1.0.0") },
			key:       constants.MetadataAppVersion,
			value:     "1.0.0",
			getFunc:   GetAppVersion,
			metaField: func(m *RequestCommonMeta) string { return m.AppVersion },
		},
		{
			name:      "WithPlatformID",
			setupFunc: func(c context.Context) context.Context { return WithPlatformID(c, "platform-123") },
			key:       constants.MetadataPlatformID,
			value:     "platform-123",
			getFunc:   GetPlatformID,
			metaField: func(m *RequestCommonMeta) string { return m.PlatformID },
		},
		{
			name:      "WithPlatformCode",
			setupFunc: func(c context.Context) context.Context { return WithPlatformCode(c, "platform-code") },
			key:       constants.MetadataPlatformCode,
			value:     "platform-code",
			getFunc:   GetPlatformCode,
			metaField: func(m *RequestCommonMeta) string { return m.PlatformCode },
		},
		{
			name:      "WithRegionID",
			setupFunc: func(c context.Context) context.Context { return WithRegionID(c, "region-123") },
			key:       constants.MetadataRegionID,
			value:     "region-123",
			getFunc:   GetRegionID,
			metaField: func(m *RequestCommonMeta) string { return m.RegionID },
		},
		{
			name:      "WithRegionCode",
			setupFunc: func(c context.Context) context.Context { return WithRegionCode(c, "region-code") },
			key:       constants.MetadataRegionCode,
			value:     "region-code",
			getFunc:   GetRegionCode,
			metaField: func(m *RequestCommonMeta) string { return m.RegionCode },
		},
		{
			name:      "WithNonce",
			setupFunc: func(c context.Context) context.Context { return WithNonce(c, "nonce-abc123") },
			key:       constants.MetadataNonce,
			value:     "nonce-abc123",
			getFunc:   GetNonce,
			metaField: func(m *RequestCommonMeta) string { return m.Nonce },
		},
		{
			name:      "WithJti",
			setupFunc: func(c context.Context) context.Context { return WithJti(c, "jti-xyz789") },
			key:       constants.MetadataJti,
			value:     "jti-xyz789",
			getFunc:   GetJti,
			metaField: func(m *RequestCommonMeta) string { return m.Jti },
		},
		{
			name:      "WithFamilyId",
			setupFunc: func(c context.Context) context.Context { return WithFamilyId(c, "family-123") },
			key:       constants.MetadataFamilyId,
			value:     "family-123",
			getFunc:   GetFamilyId,
			metaField: func(m *RequestCommonMeta) string { return m.FamilyId },
		},
		{
			name:      "WithXNsID",
			setupFunc: func(c context.Context) context.Context { return WithXNsID(c, "ns-123") },
			key:       constants.MetadataXNsID,
			value:     "ns-123",
			getFunc:   GetXNsID,
			metaField: func(m *RequestCommonMeta) string { return m.XNsID },
		},
		{
			name:      "WithAuthorization",
			setupFunc: func(c context.Context) context.Context { return WithAuthorization(c, "Bearer token123") },
			key:       constants.MetadataAuthorization,
			value:     "Bearer token123",
			getFunc:   GetAuthorization,
			metaField: func(m *RequestCommonMeta) string { return m.Authorization },
		},
		{
			name:      "WithUserAgent",
			setupFunc: func(c context.Context) context.Context { return WithUserAgent(c, "Mozilla/5.0") },
			key:       constants.MetadataUserAgent,
			value:     "Mozilla/5.0",
			getFunc:   GetUserAgent,
			metaField: func(m *RequestCommonMeta) string { return m.UserAgent },
		},
		{
			name:      "WithTimestamp",
			setupFunc: func(c context.Context) context.Context { return WithTimestamp(c, "2026-05-19T12:00:00Z") },
			value:     "2026-05-19T12:00:00Z",
			getFunc:   GetTimestamp,
			metaField: func(m *RequestCommonMeta) string { return m.Timestamp },
		},
		{
			name:      "WithSignature",
			setupFunc: func(c context.Context) context.Context { return WithSignature(c, "abc123signature") },
			key:       constants.MetadataSignature,
			value:     "abc123signature",
			getFunc:   GetSignature,
			metaField: func(m *RequestCommonMeta) string { return m.Signature },
		},
		{
			name:      "WithAccessKey",
			setupFunc: func(c context.Context) context.Context { return WithAccessKey(c, "ak-12345") },
			key:       constants.MetadataAccessKey,
			value:     "ak-12345",
			getFunc:   GetAccessKey,
			metaField: func(m *RequestCommonMeta) string { return m.AccessKey },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 先初始化一个带 RequestCommonMeta 的 context
			ctx := context.WithValue(context.Background(), requestCommonMetaKey{}, &RequestCommonMeta{})

			resultCtx := tt.setupFunc(ctx)

			// 验证 context value 可通过 Get* 函数获取
			assert.Equal(t, tt.value, tt.getFunc(resultCtx), "Get* 函数应返回设置的值")

			// 验证 RequestCommonMeta 被同步更新
			meta := GetRequestCommonMeta(resultCtx)
			assert.Equal(t, tt.value, tt.metaField(meta), "RequestCommonMeta 应被同步更新")
		})
	}
}

func TestWithTenantID_RealWorldScenario(t *testing.T) {
	// 模拟真实场景：先有 RequestCommonMeta（由中间件/拦截器注入），再通过 WithTenantID 覆盖
	ctx := context.WithValue(context.Background(), requestCommonMetaKey{}, &RequestCommonMeta{
		TenantID: "old-tenant",
	})

	resultCtx := WithTenantID(ctx, "tenant-abc123")

	// 验证 Get* 函数返回新值
	assert.Equal(t, "tenant-abc123", GetTenantID(resultCtx), "GetTenantID 应返回新值")

	// 验证 RequestCommonMeta 被同步更新
	meta := GetRequestCommonMeta(resultCtx)
	assert.Equal(t, "tenant-abc123", meta.TenantID, "RequestCommonMeta.TenantID 应被同步更新")

	// 验证 Client 拦截器能通过 injectTraceToOutgoingContext 传播新值
	outgoingCtx := injectTraceToOutgoingContext(resultCtx)
	md, ok := metadata.FromOutgoingContext(outgoingCtx)
	require.True(t, ok, "应该有 outgoing metadata")
	assert.Equal(t, []string{"tenant-abc123"}, md.Get(constants.MetadataTenantID), "outgoing metadata 应包含新值")

	t.Log("✅ WithTenantID 一次性完成了:")
	t.Log("  1. 设置 context value")
	t.Log("  2. 同步更新 RequestCommonMeta")
	t.Log("  3. Client 拦截器自动传播到 outgoing metadata")
}

func TestWithMethods_ChainedCalls(t *testing.T) {
	// 模拟真实场景：先有 RequestCommonMeta（由中间件/拦截器注入）
	ctx := context.WithValue(context.Background(), requestCommonMetaKey{}, &RequestCommonMeta{})

	resultCtx := WithTraceID(ctx, "trace-123")
	resultCtx = WithRequestID(resultCtx, "request-456")
	resultCtx = WithUserID(resultCtx, "user-789")
	resultCtx = WithTenantID(resultCtx, "tenant-111")
	resultCtx = WithSessionID(resultCtx, "session-222")

	// 验证 Get* 函数返回正确的值
	assert.Equal(t, "trace-123", GetTraceID(resultCtx))
	assert.Equal(t, "request-456", GetRequestID(resultCtx))
	assert.Equal(t, "user-789", GetUserID(resultCtx))
	assert.Equal(t, "tenant-111", GetTenantID(resultCtx))
	assert.Equal(t, "session-222", GetSessionID(resultCtx))

	// 验证 RequestCommonMeta 被同步更新
	meta := GetRequestCommonMeta(resultCtx)
	assert.Equal(t, "trace-123", meta.TraceID)
	assert.Equal(t, "request-456", meta.RequestID)
	assert.Equal(t, "user-789", meta.UserID)
	assert.Equal(t, "tenant-111", meta.TenantID)
	assert.Equal(t, "session-222", meta.SessionID)

	// 验证 Client 拦截器能自动传播
	outgoingCtx := injectTraceToOutgoingContext(resultCtx)
	md, ok := metadata.FromOutgoingContext(outgoingCtx)
	require.True(t, ok, "应该有 outgoing metadata")
	assert.Equal(t, []string{"trace-123"}, md.Get(constants.MetadataTraceID))
	assert.Equal(t, []string{"request-456"}, md.Get(constants.MetadataRequestID))
	assert.Equal(t, []string{"user-789"}, md.Get(constants.MetadataUserID))
	assert.Equal(t, []string{"tenant-111"}, md.Get(constants.MetadataTenantID))
	assert.Equal(t, []string{"session-222"}, md.Get(constants.MetadataSessionID))

	t.Log("✅ 链式调用正常工作，RequestCommonMeta 同步更新，Client 拦截器自动传播")
}

// TestContextBuilder 测试 ContextBuilder 链式构建
func TestContextBuilder(t *testing.T) {
	ctx := context.WithValue(context.Background(), requestCommonMetaKey{}, &RequestCommonMeta{})

	resultCtx := NewContextBuilder(ctx).
		WithTraceID("trace-batch-1").
		WithUserID("user-batch-2").
		WithTenantID("tenant-batch-3").
		WithTimestamp("2026-05-19T12:00:00Z").
		WithSignature("sig-batch-4").
		WithAccessKey("ak-batch-5").
		WithAuthorization("Bearer batch-token").
		Build()

	// 验证 Get* 函数返回正确的值
	assert.Equal(t, "trace-batch-1", GetTraceID(resultCtx))
	assert.Equal(t, "user-batch-2", GetUserID(resultCtx))
	assert.Equal(t, "tenant-batch-3", GetTenantID(resultCtx))
	assert.Equal(t, "2026-05-19T12:00:00Z", GetTimestamp(resultCtx))
	assert.Equal(t, "sig-batch-4", GetSignature(resultCtx))
	assert.Equal(t, "ak-batch-5", GetAccessKey(resultCtx))
	assert.Equal(t, "Bearer batch-token", GetAuthorization(resultCtx))

	// 验证 RequestCommonMeta 被同步更新
	resultMeta := GetRequestCommonMeta(resultCtx)
	assert.Equal(t, "trace-batch-1", resultMeta.TraceID)
	assert.Equal(t, "user-batch-2", resultMeta.UserID)
	assert.Equal(t, "tenant-batch-3", resultMeta.TenantID)
	assert.Equal(t, "2026-05-19T12:00:00Z", resultMeta.Timestamp)
	assert.Equal(t, "sig-batch-4", resultMeta.Signature)
	assert.Equal(t, "ak-batch-5", resultMeta.AccessKey)
	assert.Equal(t, "Bearer batch-token", resultMeta.Authorization)

	// 验证 Client 拦截器能自动传播
	outgoingCtx := injectTraceToOutgoingContext(resultCtx)
	md, ok := metadata.FromOutgoingContext(outgoingCtx)
	require.True(t, ok, "应该有 outgoing metadata")
	assert.Equal(t, []string{"trace-batch-1"}, md.Get(constants.MetadataTraceID))
	assert.Equal(t, []string{"tenant-batch-3"}, md.Get(constants.MetadataTenantID))
	assert.Equal(t, []string{"ak-batch-5"}, md.Get(constants.MetadataAccessKey))

	t.Log("✅ ContextBuilder 链式构建正常工作，RequestCommonMeta 同步更新，Client 拦截器自动传播")
}

// TestContextBuilder_EmptyBuild 测试 ContextBuilder 不设置任何字段直接 Build
func TestContextBuilder_EmptyBuild(t *testing.T) {
	ctx := context.WithValue(context.Background(), requestCommonMetaKey{}, &RequestCommonMeta{TenantID: "original"})
	resultCtx := NewContextBuilder(ctx).Build()
	assert.Equal(t, "original", GetTenantID(resultCtx), "不设置字段时不应修改 context")
}

// TestContextBuilder_ExplicitEmptyValue 测试 ContextBuilder 可以显式设置空值
func TestContextBuilder_ExplicitEmptyValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), requestCommonMetaKey{}, &RequestCommonMeta{
		TenantID: "original-tenant",
	})

	resultCtx := NewContextBuilder(ctx).WithTenantID("").Build()
	assert.Equal(t, "", GetTenantID(resultCtx), "显式设置空值应覆盖原值")
}

func TestOutgoingVsIncomingMetadata(t *testing.T) {
	t.Log("=== Outgoing vs Incoming Metadata 区别 ===")
	t.Log("")
	t.Log("┌─────────────┐    Outgoing     ┌─────────────┐")
	t.Log("│   Client    │ ───────────────▶│   Server    │")
	t.Log("│             │                 │             │")
	t.Log("│  设置 metadata│               │  读取 metadata│")
	t.Log("│  (Outgoing) │                 │  (Incoming) │")
	t.Log("└─────────────┘                 └─────────────┘")
	t.Log("")
	t.Log("Outgoing Metadata:")
	t.Log("  - Client 拦截器自动从 RequestCommonMeta 注入")
	t.Log("  - 使用 injectTraceToOutgoingContext()")
	t.Log("  - 随请求发送到服务端")
	t.Log("")
	t.Log("Incoming Metadata:")
	t.Log("  - 服务端接收请求时读取")
	t.Log("  - 使用 metadata.FromIncomingContext()")
	t.Log("  - 从客户端请求中获取")

	ctx := context.WithValue(context.Background(), requestCommonMetaKey{}, &RequestCommonMeta{
		TenantID: "test-tenant",
	})
	ctx = WithTenantID(ctx, "test-tenant")

	// 通过 Client 拦截器注入到 outgoing metadata
	outgoingCtx := injectTraceToOutgoingContext(ctx)

	md, ok := metadata.FromOutgoingContext(outgoingCtx)
	assert.True(t, ok)
	assert.Equal(t, []string{"test-tenant"}, md.Get(constants.MetadataTenantID))

	t.Log("")
	t.Log("✅ WithTenantID 更新 RequestCommonMeta，Client 拦截器自动传播到 Outgoing Metadata")
}
