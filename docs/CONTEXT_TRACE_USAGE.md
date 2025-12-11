# Context Trace 工具方法使用指南

## 概述

`context_trace.go` 现在暴露了一组标准的工具方法，供其他组件使用，避免重复实现相同的逻辑。

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
```

### 3. 生成新 ID

```go
// 生成新的 TraceID
traceID := middleware.GenerateTraceID()

// 生成新的 RequestID
requestID := middleware.GenerateRequestID()
```

### 4. HTTP Header 操作

```go
// 从 HTTP Header 提取 trace 信息到 context
ctx = middleware.ExtractTraceFromHTTPHeader(ctx, r.Header)

// 将 context 中的 trace 信息注入到 HTTP Header
middleware.InjectTraceToHTTPHeader(ctx, w.Header())
```

### 5. 批量提取

```go
// 提取所有 trace 字段为 map
fields := middleware.ExtractAllTraceFields(ctx)
// 返回: map[string]string{
//   "trace_id": "...",
//   "request_id": "...",
//   "user_id": "...",
//   ...
// }

// 获取 TraceID 和 RequestID (快捷方法)
traceID, requestID := middleware.GetTraceInfoFromContext(ctx)
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

### 场景 3: 发起外部 HTTP 请求

```go
package client

import (
    "context"
    "net/http"
    "github.com/kamalyes/go-rpc-gateway/middleware"
)

func CallExternalAPI(ctx context.Context, url string) error {
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    
    // 将当前 context 中的 trace 信息传递到下游服务
    middleware.InjectTraceToHTTPHeader(ctx, req.Header)
    
    // 发起请求...
    resp, err := http.DefaultClient.Do(req)
    // ...
    return nil
}
```

### 场景 4: 发起 gRPC 调用

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
```

### 场景 5: 自定义业务逻辑中设置用户信息

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

1. **统一使用 middleware 提供的方法**：不要直接使用 `logger.GetTraceID(ctx)`，应该使用 `middleware.GetTraceID(ctx)`
2. **使用常量而非硬编码字符串**：所有 Header 名称、字段名称都应该使用 `constants` 包中的常量
3. **保持 Context 传递**：确保在整个调用链中正确传递 context
4. **日志记录时使用标准字段名**：使用 `constants.LogField*` 系列常量作为日志字段名

## 迁移指南

### 旧代码

```go
// ❌ 不推荐
traceID := logger.GetTraceID(ctx)
userID := r.Header.Get("X-User-ID")
log.Info("request", "trace_id", traceID)
```

### 新代码

```go
// ✅ 推荐
traceID := middleware.GetTraceID(ctx)
userID := r.Header.Get(constants.HeaderXUserID)
log.Info("request", constants.LogFieldTraceID, traceID)
```
