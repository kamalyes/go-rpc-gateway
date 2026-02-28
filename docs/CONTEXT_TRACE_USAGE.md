# Context Trace 工具方法使用指南

## 概述

`context_trace.go` 提供了一组标准的工具方法，供其他组件使用，实现 HTTP → gRPC → Service → Repository 全链路追踪

## 常用方法

### 1. 从 Context 读取信息

```go
import "github.com/kamalyes/go-rpc-gateway/middleware"

// 获取 TraceID
traceID := middleware.GetTraceID(ctx)

// 获取 RequestID  
requestID := middleware.GetRequestID(ctx)

// 获取 UserID
userID := middleware.GetUserID(ctx)

// 获取 TenantID
tenantID := middleware.GetTenantID(ctx)

// 获取 SessionID
sessionID := middleware.GetSessionID(ctx)

// 获取 Timezone
timezone := middleware.GetTimezone(ctx)
```

### 2. 向 Context 写入信息

```go
// 设置 TraceID
ctx = middleware.WithTraceID(ctx, "trace-123")

// 设置 RequestID
ctx = middleware.WithRequestID(ctx, "request-456")

// 设置 UserID
ctx = middleware.WithUserID(ctx, "user-789")

// 设置 TenantID
ctx = middleware.WithTenantID(ctx, "tenant-abc")

// 设置 SessionID
ctx = middleware.WithSessionID(ctx, "session-xyz")

// 设置 Timezone
ctx = middleware.WithTimezone(ctx, "Asia/Shanghai")
```

### 3. 批量获取

```go
// 获取 TraceID 和 RequestID (快捷方法)
traceID, requestID := middleware.GetTraceInfoFromContext(ctx)

// 获取完整的追踪信息（包含所有字段）
traceInfo := middleware.GetCachedTraceInfo(ctx)
// traceInfo.TraceID
// traceInfo.RequestID
// traceInfo.UserID
// traceInfo.TenantID
// traceInfo.SessionID
// traceInfo.Timezone
```

## 使用场景示例

### 场景 1: Service 层调用

```go
package service

import (
    "context"
    "github.com/kamalyes/go-rpc-gateway/middleware"
)

type UserService struct{}

func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
    // 获取当前请求的 trace 信息
    traceID := middleware.GetTraceID(ctx)
    requestID := middleware.GetRequestID(ctx)
    
    // 记录日志时使用
    log.InfoContext(ctx, "Getting user",
        "trace_id", traceID,
        "request_id", requestID,
        "user_id", userID,
    )
    
    // 业务逻辑...
    return user, nil
}
```

### 场景 2: Repository 层调用

```go
package repository

import (
    "context"
    "github.com/kamalyes/go-rpc-gateway/middleware"
)

type UserRepository struct{}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*User, error) {
    // 在数据库查询中传递 trace 信息
    traceID := middleware.GetTraceID(ctx)
    
    // 可以将 trace_id 记录到慢查询日志中
    if duration > slowThreshold {
        log.WarnContext(ctx, "Slow query detected",
            "trace_id", traceID,
            "query", query,
            "duration", duration,
        )
    }
    
    return user, nil
}
```

### 场景 3: 发起 gRPC 调用

```go
// gRPC Client 拦截器已经自动处理，无需手动操作
// 只需要确保使用了 UnaryClientContextInterceptor 和 StreamClientContextInterceptor

import (
	"google.golang.org/grpc"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

conn, err := grpc.Dial(
	target,
	grpc.WithUnaryInterceptor(middleware.UnaryClientContextInterceptor()),
	grpc.WithStreamInterceptor(middleware.StreamClientContextInterceptor()),
)

// 之后的 gRPC 调用会自动传递 context 中的 trace 信息
client := pb.NewUserServiceClient(conn)
resp, err := client.GetUser(ctx, &pb.GetUserRequest{ID: "123"})
```

### 场景 4: 自定义业务逻辑中设置用户信息

```go
package auth

import (
	"context"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

func AuthenticateUser(ctx context.Context, token string) (context.Context, error) {
	// 验证 token...
	user, err := validateToken(token)
	if err != nil {
		return ctx, err
	}
	
	// 将用户信息注入到 context
	ctx = middleware.WithUserID(ctx, user.ID)
	ctx = middleware.WithTenantID(ctx, user.TenantID)
	
	return ctx, nil
}
```

### 场景 5: 在 HTTP Handler 中使用

```go
package handler

import (
	"net/http"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

func UserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// 读取追踪信息
	traceID := middleware.GetTraceID(ctx)
	userID := middleware.GetUserID(ctx)
	
	// 业务逻辑...
	
	// 响应头已由 ContextTraceMiddleware 自动设置
	// 无需手动设置 X-Trace-Id 和 X-Request-Id
}
```

## 中间件配置

### HTTP 中间件

在 HTTP 服务器中使用 `ContextTraceMiddleware`：

```go
import (
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

// 配置中间件
router.Use(middleware.ContextTraceMiddleware())
```

该中间件会自动：
1. 从 HTTP Header 提取或生成 `trace_id` 和 `request_id`
2. 将追踪信息存入 context
3. 设置响应头返回 `X-Trace-Id` 和 `X-Request-Id`

### gRPC Server 拦截器

在 gRPC 服务器中使用拦截器：

```go
import (
	"google.golang.org/grpc"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

server := grpc.NewServer(
	grpc.UnaryInterceptor(middleware.UnaryServerContextInterceptor()),
	grpc.StreamInterceptor(middleware.StreamServerContextInterceptor()),
)
```

拦截器会自动：
1. 从 gRPC metadata 提取追踪信息
2. 将追踪信息存入 context
3. 设置响应 metadata 返回追踪信息

### gRPC Client 拦截器

在 gRPC 客户端中使用拦截器：

```go
import (
	"google.golang.org/grpc"
	"github.com/kamalyes/go-rpc-gateway/middleware"
)

conn, err := grpc.Dial(
	target,
	grpc.WithUnaryInterceptor(middleware.UnaryClientContextInterceptor()),
	grpc.WithStreamInterceptor(middleware.StreamClientContextInterceptor()),
)
```

拦截器会自动将 context 中的追踪信息注入到 gRPC metadata

## 常量定义

所有字段名和 Header 名称都已在 `constants` 包中定义：

### Header 常量 (constants/headers.go)

```go
constants.HeaderXTraceID    // "X-Trace-Id"
constants.HeaderXRequestID  // "X-Request-Id"
constants.HeaderXUserID     // "X-User-ID"
constants.HeaderXTenantID   // "X-Tenant-ID"
constants.HeaderXSessionID  // "X-Session-ID"
```

### gRPC Metadata 常量 (constants/metadata.go)

```go
constants.MetadataTraceID   // "x-trace-id"
constants.MetadataRequestID // "x-request-id"
constants.MetadataUserID    // "x-user-id"
constants.MetadataTenantID  // "x-tenant-id"
constants.MetadataSessionID // "x-session-id"
```

### 日志字段常量 (constants/metadata.go)

```go
constants.LogFieldTraceID    // "trace_id"
constants.LogFieldRequestID  // "request_id"
constants.LogFieldUserID     // "user_id"
constants.LogFieldTenantID   // "tenant_id"
constants.LogFieldError      // "error"
constants.LogFieldMethod     // "method"
constants.LogFieldPath       // "path"
// ... 更多字段
```

## 最佳实践

1. **统一使用 middleware 提供的方法**
   - 使用 `middleware.GetTraceID(ctx)` 而不是直接使用 `logger.GetTraceID(ctx)`
   - 保持 API 的一致性和可维护性

2. **使用常量而非硬编码字符串**
   - 所有 Header 名称使用 `constants.HeaderX*` 系列常量
   - 所有 Metadata 名称使用 `constants.Metadata*` 系列常量
   - 所有日志字段使用 `constants.LogField*` 系列常量

3. **保持 Context 传递**
   - 确保在整个调用链中正确传递 context
   - 不要创建新的空 context，使用传入的 context

4. **自动化优先**
   - HTTP 层使用 `ContextTraceMiddleware()` 自动处理
   - gRPC 层使用拦截器自动处理
   - 避免手动设置响应头或 metadata

5. **日志记录标准化**
   - 使用 `constants.LogField*` 常量作为日志字段名
   - 保持日志格式的一致性

## 架构说明

### 全链路追踪流程

```
HTTP Request
    ↓ (ContextTraceMiddleware 提取/生成 trace_id)
HTTP Handler
    ↓ (context 传递)
Service Layer (使用 middleware.GetTraceID 获取)
    ↓ (context 传递)
Repository Layer (使用 middleware.GetTraceID 获取)
    ↓ (context 传递)
gRPC Client (UnaryClientContextInterceptor 自动注入)
    ↓ (metadata 传递)
gRPC Server (UnaryServerContextInterceptor 自动提取)
    ↓ (context 传递)
Downstream Service
```

### Context 存储机制

追踪信息在 context 中有两层存储：

1. **缓存层**：`TraceInfo` 结构体（通过 `traceInfoKey` 存储）
   - 优点：一次查询获取所有字段，性能更好
   - 使用：`GetCachedTraceInfo(ctx)` 获取完整信息

2. **标准层**：使用 `go-logger` 的标准 ContextKey
   - 优点：与日志系统集成，兼容性好
   - 使用：`middleware.GetTraceID(ctx)` 等方法

推荐使用 `middleware.Get*` 系列方法，它们会优先使用缓存层，性能更优

## 常见问题

### Q1: 为什么要使用 middleware.GetTraceID 而不是 logger.GetTraceID？

A: `middleware.GetTraceID` 会优先从缓存的 `TraceInfo` 中获取，性能更好同时保持了 API 的一致性，便于后续维护

### Q2: 如何在发起外部 HTTP 请求时传递追踪信息？

A: 目前框架主要支持 gRPC 调用的自动传递如需在 HTTP 请求中传递，可以手动设置 Header：

```go
req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
req.Header.Set(constants.HeaderXTraceID, middleware.GetTraceID(ctx))
req.Header.Set(constants.HeaderXRequestID, middleware.GetRequestID(ctx))
```

### Q3: TraceInfo 缓存什么时候创建？

A: 在 HTTP 中间件 `ContextTraceMiddleware` 或 gRPC 拦截器中自动创建如果你在这些中间件之外使用，`GetCachedTraceInfo` 会回退到从 logger 中提取

### Q4: 如何在单元测试中使用？

A: 在测试中创建带有追踪信息的 context：

```go
ctx := context.Background()
ctx = middleware.WithTraceID(ctx, "test-trace-id")
ctx = middleware.WithRequestID(ctx, "test-request-id")
ctx = middleware.WithUserID(ctx, "test-user-id")

// 使用 ctx 进行测试
```

### Q5: 支持哪些追踪字段？

A: 当前支持以下字段：
- `TraceID`: 追踪 ID（必需）
- `RequestID`: 请求 ID（必需）
- `UserID`: 用户 ID（可选）
- `TenantID`: 租户 ID（可选）
- `SessionID`: 会话 ID（可选）
- `Timezone`: 时区（可选）

## 相关文档

- [go-logger 文档](https://github.com/kamalyes/go-logger) - 了解底层日志系统
- [OpenTelemetry 集成](https://opentelemetry.io/) - 分布式追踪标准
- [gRPC Metadata](https://grpc.io/docs/guides/metadata/) - gRPC 元数据传递机制

