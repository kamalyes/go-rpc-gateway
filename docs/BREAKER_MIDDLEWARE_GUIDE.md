# 熔断中间件适配器使用指南

## 概述

熔断中间件适配器(`BreakerMiddlewareAdapter`)是一个在 `middleware` 模块下的适配器，将 `go-rpc-gateway/breaker` 模块的功能集成到标准的中间件框架中。

## 架构设计

```
go-config/pkg/breaker/
  └── breaker.go - CircuitBreaker 配置定义

go-rpc-gateway/breaker/
  ├── breaker.go - 核心断路器逻辑
  ├── manager.go - 路径级别的断路器管理
  ├── middleware.go - HTTP 中间件（已移到 middleware 模块）
  └── websocket.go - WebSocket 连接保护

go-rpc-gateway/middleware/
  └── breaker.go - 中间件适配器（新）
      ├── BreakerMiddlewareAdapter - 适配器主类
      ├── Middleware() - 中间件工厂函数
      └── 监控和统计方法
```

## 配置来源

熔断中间件的配置来自 `go-config/pkg/breaker/CircuitBreaker`：

```go
type CircuitBreaker struct {
    ModuleName          string        // 模块名称
    Enabled             bool          // 是否启用断路器
    FailureThreshold    int           // 失败阈值
    SuccessThreshold    int           // 成功阈值
    Timeout             time.Duration // 熔断后恢复时间
    VolumeThreshold     int           // 最小请求量阈值
    SlidingWindowSize   int           // 滑动窗口大小
    SlidingWindowBucket time.Duration // 滑动窗口桶大小
    PreventionPaths     []string      // 需要保护的路径
    ExcludePaths        []string      // 排除的路径
}
```

## 快速开始

### 方式 1：通过 Manager 使用（推荐）

```go
// Manager 会自动初始化熔断中间件适配器
manager, err := middleware.NewManager(config)
if err != nil {
    panic(err)
}

// 获取熔断中间件
breakerMW := manager.BreakerMiddleware()

// 应用到处理器
handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
})

protectedHandler := breakerMW(handler)
```

### 方式 2：直接创建适配器

```go
import "github.com/kamalyes/go-config/pkg/breaker"
import "github.com/kamalyes/go-rpc-gateway/middleware"

// 使用默认配置
adapter := middleware.NewBreakerMiddlewareAdapter(breaker.Default())

// 或使用自定义配置
customConfig := &breaker.CircuitBreaker{
    Enabled:           true,
    FailureThreshold:  5,
    SuccessThreshold:  2,
    VolumeThreshold:   10,
    Timeout:           30 * time.Second,
    PreventionPaths:   []string{"/api/"},
    ExcludePaths:      []string{"/health", "/metrics"},
}
adapter := middleware.NewBreakerMiddlewareAdapter(customConfig)

// 获取中间件并应用
mux := http.NewServeMux()
mux.Handle("/api/", adapter.Middleware()(handler))
```

## 核心方法

### Middleware()
返回 HTTP 中间件函数：

```go
func (a *BreakerMiddlewareAdapter) Middleware() func(http.Handler) http.Handler
```

### Enable/Disable
动态启用/禁用熔断器：

```go
adapter.Enable()   // 启用
adapter.Disable()  // 禁用
```

### GetStats()
获取统计信息：

```go
stats := adapter.GetStats()
fmt.Printf("Total Requests: %d\n", stats.TotalRequests)
fmt.Printf("Failed Requests: %d\n", stats.FailedRequests)
fmt.Printf("Blocked Requests: %d\n", stats.BlockedRequests)
fmt.Printf("Open Breakers: %d\n", stats.OpenBreakers)
```

### Reset()
重置所有统计信息：

```go
adapter.Reset()
```

## 工作流程

1. **请求到达** → 检查是否启用
2. **路径检查** → 检查路径是否需要保护
3. **熔断检查** → 获取该路径的断路器，检查状态
4. **状态判断**：
   - **Closed（正常）** → 允许请求通过
   - **Open（熔断）** → 拒绝请求，返回 503
   - **HalfOpen（半开）** → 允许请求，验证是否恢复
5. **记录结果** → 根据响应状态码记录成功或失败
6. **更新统计** → 更新断路器状态和统计信息

## 状态转换图

```
         Closed (正常)
            ↓ (失败超过阈值)
         Open (熔断)
            ↓ (超时后)
      Half-Open (半开)
            ↓ (成功 2 次)
         Closed (正常)
            ↑ (失败)
         Open (熔断)
```

## 监控和调试

### 获取健康状态
```go
status := adapter.GetHealthStatus()
fmt.Printf("System Health: %s\n", status)
```

### 获取详细统计
```go
detailedStats := adapter.GetBreakerStats()
for path, stats := range detailedStats {
    fmt.Printf("Path: %s, Stats: %+v\n", path, stats)
}
```

### 定期监控
```go
go func() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        stats := adapter.GetStats()
        fmt.Printf("Health: %s, Open: %d, Blocked: %d\n",
            adapter.GetHealthStatus(),
            stats.OpenBreakers,
            stats.BlockedRequests)
    }
}()
```

## 配置示例

### YAML 配置

```yaml
middleware:
  breaker:
    module-name: "http-breaker"
    enabled: true
    failure-threshold: 5      # 5 次失败后打开
    success-threshold: 2      # 2 次成功后关闭
    volume-threshold: 10      # 最少 10 个请求
    timeout: 30s              # 30 秒后尝试恢复
    prevention-paths:
      - "/api/"
      - "/service/"
    exclude-paths:
      - "/health"
      - "/metrics"
      - "/status"
```

### Go 代码配置

```go
config := &breaker.CircuitBreaker{
    ModuleName:        "api-breaker",
    Enabled:           true,
    FailureThreshold:  5,
    SuccessThreshold:  2,
    VolumeThreshold:   10,
    Timeout:           30 * time.Second,
    PreventionPaths:   []string{"/api/"},
    ExcludePaths:      []string{"/health"},
}
```

## 集成到 Manager 的中间件链

当通过 `Manager` 使用时，熔断中间件会自动添加到中间件链中：

```go
// 获取默认中间件链（包含熔断中间件）
middlewares := manager.GetDefaultMiddlewares()

// 或生产环境中间件链（也包含熔断）
middlewares := manager.GetProductionMiddlewares()

// 应用所有中间件
finalHandler := middleware.ApplyMiddlewares(handler, middlewares...)
```

## 常见问题

### Q: 如何排除特定路径不受保护？
A: 在 `ExcludePaths` 配置中添加路径。即使路径在 `PreventionPaths` 中，如果也在 `ExcludePaths` 中，也不会受保护。

### Q: 熔断器打开后多久会尝试恢复？
A: 由 `Timeout` 配置决定，默认 30 秒。

### Q: 如何区分不同的故障源（如不同的 API 端点）？
A: Manager 会为每个受保护的路径创建单独的断路器实例。

### Q: 是否可以在运行时更改配置？
A: 可以通过 `Enable()`/`Disable()` 动态启用/禁用。但要更改阈值等参数，需要创建新的适配器。

## 性能考量

- 每个请求都会进行断路器状态检查（O(1) 操作）
- 定期收集指标（默认 10 秒间隔）
- 使用读写锁保护并发访问
- 内存占用：每个受保护的路径约 1KB

## 最佳实践

1. **保护关键 API** - 重点保护外部 API 调用
2. **合理设置阈值** - 根据业务量调整失败阈值
3. **监控告警** - 定期检查 `GetHealthStatus()`
4. **默认排除健康检查** - 避免健康检查触发熔断
5. **记录统计信息** - 用于分析和优化

## 相关文件

- 配置定义：`go-config/pkg/breaker/breaker.go`
- 核心实现：`go-rpc-gateway/breaker/breaker.go`
- 管理器：`go-rpc-gateway/breaker/manager.go`
- 中间件适配器：`go-rpc-gateway/middleware/breaker.go`
- 集成示例：`go-rpc-gateway/middleware/breaker_integration_example.go`
