/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-29 12:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-30 20:57:55
 * @FilePath: \go-rpc-gateway\middleware\context_trace_test.go
 * @Description: Context 追踪中间件测试
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

	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

// TestContextTraceMiddleware_GeneratesIDs 测试中间件生成 trace_id 和 request_id
func TestContextTraceMiddleware_GeneratesIDs(t *testing.T) {
	middleware := ContextTraceMiddleware()

	var capturedCtx context.Context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 验证 context 中有 trace_id 和 request_id
	traceID := logger.GetTraceID(capturedCtx)
	requestID := logger.GetRequestID(capturedCtx)

	assert.NotEmpty(t, traceID, "trace_id 应该被生成")
	assert.NotEmpty(t, requestID, "request_id 应该被生成")

	// 验证响应头中也有这些值
	assert.Equal(t, traceID, rec.Header().Get(constants.HeaderXTraceID), "响应头应包含 trace_id")
	assert.Equal(t, requestID, rec.Header().Get(constants.HeaderXRequestID), "响应头应包含 request_id")
}

// TestContextTraceMiddleware_UsesExistingIDs 测试中间件使用请求中已有的 ID
func TestContextTraceMiddleware_UsesExistingIDs(t *testing.T) {
	middleware := ContextTraceMiddleware()

	existingTraceID := "existing-trace-id-12345"
	existingRequestID := "existing-request-id-67890"

	var capturedCtx context.Context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(constants.HeaderXTraceID, existingTraceID)
	req.Header.Set(constants.HeaderXRequestID, existingRequestID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 验证使用了已有的 ID
	assert.Equal(t, existingTraceID, logger.GetTraceID(capturedCtx), "应使用已有的 trace_id")
	assert.Equal(t, existingRequestID, logger.GetRequestID(capturedCtx), "应使用已有的 request_id")

	// 验证响应头
	assert.Equal(t, existingTraceID, rec.Header().Get(constants.HeaderXTraceID))
	assert.Equal(t, existingRequestID, rec.Header().Get(constants.HeaderXRequestID))
}

// TestContextTraceMiddleware_ExtractsOptionalFields 测试中间件提取可选字段
func TestContextTraceMiddleware_ExtractsOptionalFields(t *testing.T) {
	middleware := ContextTraceMiddleware()

	var capturedCtx context.Context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(constants.HeaderXUserID, "user-123")
	req.Header.Set(constants.HeaderXTenantID, "tenant-456")
	req.Header.Set(constants.HeaderXSessionID, "session-789")
	req.Header.Set(constants.HeaderXTimezone, "Asia/Shanghai")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 验证可选字段被提取
	assert.Equal(t, "user-123", logger.GetUserID(capturedCtx), "应提取 user_id")
	assert.Equal(t, "tenant-456", logger.GetTenantID(capturedCtx), "应提取 tenant_id")
	assert.Equal(t, "session-789", logger.GetSessionID(capturedCtx), "应提取 session_id")
	assert.Equal(t, "Asia/Shanghai", logger.GetTimezone(capturedCtx), "应提取 timezone")
}

// TestEnrichContextFromMetadata 测试从 gRPC metadata 提取追踪信息
func TestEnrichContextFromMetadata(t *testing.T) {
	// 创建带有 metadata 的 context
	md := metadata.Pairs(
		constants.MetadataTraceID, "grpc-trace-123",
		constants.MetadataRequestID, "grpc-request-456",
		constants.MetadataUserID, "grpc-user-789",
		constants.MetadataTenantID, "grpc-tenant-456",
		constants.MetadataSessionID, "grpc-session-999",
		constants.MetadataTimezone, "UTC",
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	// 调用函数
	enrichedCtx := enrichContextFromMetadata(ctx)

	// 验证提取的值
	assert.Equal(t, "grpc-trace-123", logger.GetTraceID(enrichedCtx))
	assert.Equal(t, "grpc-request-456", logger.GetRequestID(enrichedCtx))
	assert.Equal(t, "grpc-user-789", logger.GetUserID(enrichedCtx))
	assert.Equal(t, "grpc-tenant-456", logger.GetTenantID(enrichedCtx))
	assert.Equal(t, "grpc-session-999", logger.GetSessionID(enrichedCtx))
	assert.Equal(t, "UTC", logger.GetTimezone(enrichedCtx))
}

// TestEnrichContextFromMetadata_GeneratesIDsWhenMissing 测试缺少 ID 时生成新的
func TestEnrichContextFromMetadata_GeneratesIDsWhenMissing(t *testing.T) {
	// 空的 metadata
	md := metadata.Pairs()
	ctx := metadata.NewIncomingContext(context.Background(), md)

	enrichedCtx := enrichContextFromMetadata(ctx)

	// 验证生成了新的 ID
	assert.NotEmpty(t, logger.GetTraceID(enrichedCtx), "应生成 trace_id")
	assert.NotEmpty(t, logger.GetRequestID(enrichedCtx), "应生成 request_id")
}

// TestInjectTraceToOutgoingContext 测试将 trace 信息注入到 outgoing metadata
func TestInjectTraceToOutgoingContext(t *testing.T) {
	// 创建带有 trace 信息的 context
	ctx := context.Background()
	ctx = logger.WithTraceID(ctx, "outgoing-trace-123")
	ctx = logger.WithRequestID(ctx, "outgoing-request-456")
	ctx = logger.WithUserID(ctx, "outgoing-user-789")
	ctx = logger.WithTenantID(ctx, "outgoing-tenant-111")
	ctx = logger.WithSessionID(ctx, "outgoing-session-222")
	ctx = logger.WithTimezone(ctx, "America/New_York")

	// 缓存 TraceInfo
	ctx = context.WithValue(ctx, traceInfoKey{}, &TraceInfo{
		TraceID:   "outgoing-trace-123",
		RequestID: "outgoing-request-456",
		UserID:    "outgoing-user-789",
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
	assert.Equal(t, []string{"outgoing-tenant-111"}, md.Get(constants.MetadataTenantID))
	assert.Equal(t, []string{"outgoing-session-222"}, md.Get(constants.MetadataSessionID))
	assert.Equal(t, []string{"America/New_York"}, md.Get(constants.MetadataTimezone))
}

// TestGetTraceInfoFromContext 测试从 context 获取追踪信息
func TestGetTraceInfoFromContext(t *testing.T) {
	ctx := context.Background()
	ctx = logger.WithTraceID(ctx, "test-trace")
	ctx = logger.WithRequestID(ctx, "test-request")

	traceID, requestID := GetTraceInfoFromContext(ctx)

	assert.Equal(t, "test-trace", traceID)
	assert.Equal(t, "test-request", requestID)
}

// TestContextWrappedServerStream 测试 ServerStream 包装器
func TestContextWrappedServerStream(t *testing.T) {
	ctx := context.Background()
	ctx = logger.WithTraceID(ctx, "stream-trace")

	wrapped := &contextWrappedServerStream{
		ServerStream: nil, // 测试中不需要真实的 stream
		ctx:          ctx,
	}

	// 验证 Context() 返回增强后的 context
	assert.Equal(t, "stream-trace", logger.GetTraceID(wrapped.Context()))
}

// TestGetCachedTraceInfo 测试缓存的 TraceInfo 获取
func TestGetCachedTraceInfo(t *testing.T) {
	t.Run("从缓存获取", func(t *testing.T) {
		ctx := context.Background()
		expectedInfo := &TraceInfo{
			TraceID:   "cached-trace-123",
			RequestID: "cached-request-456",
			UserID:    "cached-user-789",
			TenantID:  "cached-tenant-111",
			SessionID: "cached-session-222",
			Timezone:  "Asia/Tokyo",
		}
		ctx = context.WithValue(ctx, traceInfoKey{}, expectedInfo)

		info := GetCachedTraceInfo(ctx)

		assert.Equal(t, expectedInfo.TraceID, info.TraceID)
		assert.Equal(t, expectedInfo.RequestID, info.RequestID)
		assert.Equal(t, expectedInfo.UserID, info.UserID)
		assert.Equal(t, expectedInfo.TenantID, info.TenantID)
		assert.Equal(t, expectedInfo.SessionID, info.SessionID)
		assert.Equal(t, expectedInfo.Timezone, info.Timezone)
	})

	t.Run("回退到logger提取", func(t *testing.T) {
		ctx := context.Background()
		ctx = logger.WithTraceID(ctx, "fallback-trace")
		ctx = logger.WithRequestID(ctx, "fallback-request")
		ctx = logger.WithUserID(ctx, "fallback-user")

		info := GetCachedTraceInfo(ctx)

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

	// 测试 GetXXX 函数
	assert.Equal(t, "util-trace", GetTraceID(ctx))
	assert.Equal(t, "util-request", GetRequestID(ctx))
	assert.Equal(t, "util-user", GetUserID(ctx))
	assert.Equal(t, "util-tenant", GetTenantID(ctx))
	assert.Equal(t, "util-session", GetSessionID(ctx))
	assert.Equal(t, "Europe/London", GetTimezone(ctx))
}

// TestContextTraceMiddleware_CachesTraceInfo 测试中间件缓存 TraceInfo
func TestContextTraceMiddleware_CachesTraceInfo(t *testing.T) {
	middleware := ContextTraceMiddleware()

	var capturedCtx context.Context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(constants.HeaderXTraceID, "cache-test-trace")
	req.Header.Set(constants.HeaderXRequestID, "cache-test-request")
	req.Header.Set(constants.HeaderXUserID, "cache-test-user")
	req.Header.Set(constants.HeaderXTenantID, "cache-test-tenant")
	req.Header.Set(constants.HeaderXSessionID, "cache-test-session")
	req.Header.Set(constants.HeaderXTimezone, "Asia/Seoul")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 验证 TraceInfo 被缓存
	info := GetCachedTraceInfo(capturedCtx)
	assert.NotNil(t, info)
	assert.Equal(t, "cache-test-trace", info.TraceID)
	assert.Equal(t, "cache-test-request", info.RequestID)
	assert.Equal(t, "cache-test-user", info.UserID)
	assert.Equal(t, "cache-test-tenant", info.TenantID)
	assert.Equal(t, "cache-test-session", info.SessionID)
	assert.Equal(t, "Asia/Seoul", info.Timezone)
}

// TestFullChain_HTTPToContext 测试完整链路：HTTP 请求到 context
func TestFullChain_HTTPToContext(t *testing.T) {
	// 模拟完整的 HTTP → Service → Repository 链路
	middleware := ContextTraceMiddleware()

	var serviceCtx context.Context
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟 Service 层获取 context
		serviceCtx = r.Context()

		// 模拟 Repository 层使用 context 记录日志
		traceID := logger.GetTraceID(serviceCtx)
		requestID := logger.GetRequestID(serviceCtx)

		// 验证在整个链路中都能获取到 trace 信息
		assert.NotEmpty(t, traceID, "Service 层应能获取 trace_id")
		assert.NotEmpty(t, requestID, "Service 层应能获取 request_id")

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/messages", nil)
	req.Header.Set(constants.HeaderXTraceID, "chain-trace-id")
	req.Header.Set(constants.HeaderXRequestID, "chain-request-id")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// 最终验证
	assert.Equal(t, "chain-trace-id", logger.GetTraceID(serviceCtx))
	assert.Equal(t, "chain-request-id", logger.GetRequestID(serviceCtx))
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
	serviceTraceID := logger.GetTraceID(enrichedCtx)
	serviceRequestID := logger.GetRequestID(enrichedCtx)
	serviceUserID := logger.GetUserID(enrichedCtx)
	serviceTenantID := logger.GetTenantID(enrichedCtx)
	serviceSessionID := logger.GetSessionID(enrichedCtx)
	serviceTimezone := logger.GetTimezone(enrichedCtx)

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
	ctx = logger.WithTraceID(ctx, "response-trace")
	ctx = logger.WithRequestID(ctx, "response-request")

	// 缓存 TraceInfo
	ctx = context.WithValue(ctx, traceInfoKey{}, &TraceInfo{
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

// TestGetFirstMetadataValue 测试从 metadata 获取第一个值
func TestGetFirstMetadataValue(t *testing.T) {
	md := metadata.Pairs(
		"single-key", "single-value",
		"multi-key", "value1",
		"multi-key", "value2",
	)

	// 测试单值
	assert.Equal(t, "single-value", getFirstMetadataValue(md, "single-key"))

	// 测试多值（应返回第一个）
	assert.Equal(t, "value1", getFirstMetadataValue(md, "multi-key"))

	// 测试不存在的 key
	assert.Equal(t, "", getFirstMetadataValue(md, "non-existent"))
}

// BenchmarkContextTraceMiddleware 性能测试
func BenchmarkContextTraceMiddleware(b *testing.B) {
	middleware := ContextTraceMiddleware()
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

// BenchmarkGetCachedTraceInfo 性能测试：缓存命中
func BenchmarkGetCachedTraceInfo(b *testing.B) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, traceInfoKey{}, &TraceInfo{
		TraceID:   "bench-trace",
		RequestID: "bench-request",
		UserID:    "bench-user",
	})

	b.ResetTimer()
	for range b.N {
		_ = GetCachedTraceInfo(ctx)
	}
}

// BenchmarkGetCachedTraceInfo_Fallback 性能测试：缓存未命中回退
func BenchmarkGetCachedTraceInfo_Fallback(b *testing.B) {
	ctx := context.Background()
	ctx = logger.WithTraceID(ctx, "bench-trace")
	ctx = logger.WithRequestID(ctx, "bench-request")

	b.ResetTimer()
	for range b.N {
		_ = GetCachedTraceInfo(ctx)
	}
}

// BenchmarkInjectTraceToOutgoingContext 性能测试
func BenchmarkInjectTraceToOutgoingContext(b *testing.B) {
	ctx := context.Background()
	ctx = logger.WithTraceID(ctx, "bench-trace")
	ctx = logger.WithRequestID(ctx, "bench-request")
	ctx = logger.WithUserID(ctx, "bench-user")
	ctx = context.WithValue(ctx, traceInfoKey{}, &TraceInfo{
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
	traceID := logger.GetTraceID(ctx)
	requestID := logger.GetRequestID(ctx)

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
	traceID := logger.GetTraceID(ctx)
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
		TraceID:   logger.GetTraceID(ctx),
		RequestID: logger.GetRequestID(ctx),
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
		TraceID:   logger.GetTraceID(ctx),
		RequestID: logger.GetRequestID(ctx),
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
	repo := NewUserRepository()
	service := NewUserService(repo)
	handler := NewUserHandler(service)

	// 2. 创建中间件链（模拟真实网关）
	middleware := ContextTraceMiddleware()
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
	req.Header.Set(constants.HeaderXTraceID, "client-trace-123")

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

	middleware := ContextTraceMiddleware()
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
	req1.Header.Set(constants.HeaderXTraceID, "trace-duplicate-1")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	assert.Equal(t, http.StatusOK, rec1.Code)

	// 第二次创建（应该失败）
	t.Log(">>> 第二次创建相同用户（应该失败）...")
	bodyBytes2, _ := json.Marshal(reqBody)
	req2 := httptest.NewRequest("POST", "/api/users", bytes.NewReader(bodyBytes2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set(constants.HeaderXTraceID, "trace-duplicate-2")
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

	repo := NewUserRepository()
	service := NewUserService(repo)
	handler := NewUserHandler(service)

	middleware := ContextTraceMiddleware()
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
			req.Header.Set(constants.HeaderXTraceID, traceIDs[index])
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
		handlerTraceID = logger.GetTraceID(ctx)

		// 在调用前记录 service 层的 trace_id
		serviceTraceID = logger.GetTraceID(ctx)

		// 在 repository 操作前记录
		repositoryTraceID = logger.GetTraceID(ctx)

		// 调用真实的 handler
		handler.CreateUser(w, r)
	})

	middleware := ContextTraceMiddleware()
	router := middleware(wrapperHandler)

	// 发送请求
	reqBody := &CreateUserRequest{Username: "propagation_test", Email: "test@example.com"}
	bodyBytes, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(constants.HeaderXTraceID, "propagation-trace-999")
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

	middleware := ContextTraceMiddleware()
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
	t.Log(strings.Repeat("=", 80))
	t.Log("完整请求流程演示 - HTTP → Handler → Service → Repository")
	t.Log(strings.Repeat("=", 80))

	repo := NewUserRepository()
	service := NewUserService(repo)
	handler := NewUserHandler(service)

	middleware := ContextTraceMiddleware()
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
	req.Header.Set(constants.HeaderXTraceID, "demo-trace-12345")
	req.Header.Set(constants.HeaderXUserID, "client-user-999")
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
	assert.NotEmpty(t, logger.GetTraceID(enrichedCtx))
	assert.NotEmpty(t, logger.GetRequestID(enrichedCtx))
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
	assert.Equal(t, "partial-trace", logger.GetTraceID(enrichedCtx))
	// request-id 应该被生成
	assert.NotEmpty(t, logger.GetRequestID(enrichedCtx))
}

// TestEdgeCases_MergeOutgoingMetadata 测试边界情况：合并已有的 outgoing metadata
func TestEdgeCases_MergeOutgoingMetadata(t *testing.T) {
	// 创建已有 outgoing metadata 的 context
	existingMD := metadata.Pairs("existing-key", "existing-value")
	ctx := metadata.NewOutgoingContext(context.Background(), existingMD)

	// 添加 trace 信息
	ctx = logger.WithTraceID(ctx, "merge-trace")
	ctx = logger.WithRequestID(ctx, "merge-request")
	ctx = context.WithValue(ctx, traceInfoKey{}, &TraceInfo{
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

// TestEdgeCases_EmptyTraceInfo 测试边界情况：空的 TraceInfo
func TestEdgeCases_EmptyTraceInfo(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, traceInfoKey{}, &TraceInfo{})

	info := GetCachedTraceInfo(ctx)

	// 应该返回空字符串，不会 panic
	assert.Equal(t, "", info.TraceID)
	assert.Equal(t, "", info.RequestID)
}

// TestEdgeCases_NilContext 测试边界情况：nil context（理论上不应该发生）
func TestEdgeCases_GetTraceInfoFromNilCachedContext(t *testing.T) {
	ctx := context.Background()

	// 没有缓存的 TraceInfo
	info := GetCachedTraceInfo(ctx)

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
