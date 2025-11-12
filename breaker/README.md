# CircuitBreaker (断路器) 模块使用指南

## 模块结构

```
go-config/pkg/breaker/
  └── breaker.go            # 配置定义（CircuitBreaker、WebSocketBreaker）

go-rpc-gateway/breaker/
  ├── breaker.go            # 核心业务逻辑（Breaker 结构体）
  ├── manager.go            # 管理器（Manager 用于管理多个 Breaker 实例）
  ├── middleware.go         # HTTP 中间件
  └── websocket.go          # WebSocket 适配器和连接池
```

## 配置管理（go-config）

断路器配置已下沉到 `go-config` 模块管理，支持两种配置类型：

### 1. CircuitBreaker（HTTP 断路器配置）

```go
type CircuitBreaker struct {
    ModuleName          string        // 模块名称
    Enabled             bool          // 是否启用
    FailureThreshold    int           // 失败阈值（触发熔断）
    SuccessThreshold    int           // 成功阈值（触发恢复）
    Timeout             time.Duration // 熔断后恢复等待时间
    VolumeThreshold     int           // 最小请求量阈值
    SlidingWindowSize   int           // 滑动窗口大小
    SlidingWindowBucket time.Duration // 滑动窗口桶大小
    PreventionPaths     []string      // 需要保护的路径
    ExcludePaths        []string      // 排除的路径
}
```

### 2. WebSocketBreaker（WebSocket 断路器配置）

```go
type WebSocketBreaker struct {
    ModuleName          string        // 模块名称
    Enabled             bool          // 是否启用
    FailureThreshold    int           // 失败阈值
    SuccessThreshold    int           // 成功阈值
    Timeout             time.Duration // 熔断恢复时间
    MaxRetries          int           // 最大重试次数
    RetryBackoffFactor  float64       // 重试退避因子
    HealthCheckInterval time.Duration // 健康检查间隔
    MessageQueueSize    int           // 消息队列大小
}
```

## 核心组件使用

### 1. Breaker（断路器核心）

```go
import "github.com/kamalyes/go-rpc-gateway/breaker"

// 创建一个断路器
b := breaker.New(
    5,                  // failureThreshold
    2,                  // successThreshold
    10,                 // volumeThreshold
    30 * time.Second,   // timeout
)

// 检查是否允许请求
if !b.Allow() {
    // 熔断器打开，拒绝请求
    return fmt.Errorf("circuit breaker open")
}

// 记录成功
b.RecordSuccess()

// 记录失败
b.RecordFailure()

// 获取状态和统计信息
state := b.GetState()      // 返回 State: Closed/Open/HalfOpen
stats := b.GetStats()      // 返回详细统计信息

// 重置断路器
b.Reset()
```

### 2. Manager（断路器管理器）

```go
// 创建管理器（管理多个 Breaker 实例）
manager := breaker.NewManager(
    5,                  // failureThreshold
    2,                  // successThreshold
    10,                 // volumeThreshold
    30 * time.Second,   // timeout
    []string{"/api/"},  // preventionPaths
    []string{"/health"}, // excludePaths
)

// 获取或创建特定路径的断路器
breaker := manager.GetBreaker("/api/users")

// 检查路径是否需要保护
protected := manager.IsPathProtected("/api/users")  // true
protected := manager.IsPathProtected("/health")     // false

// 获取所有断路器统计信息
stats := manager.GetStats()

// 获取健康状态
health := manager.GetHealthStatus()
// 返回: {
//   "is_healthy": false,
//   "total_breakers": 5,
//   "open_breakers": 1,
//   "half_open_breakers": 0,
//   "closed_breakers": 4
// }

// 统计各状态的断路器数量
openCount := manager.CountOpenBreakers()
halfOpenCount := manager.CountHalfOpenBreakers()
closedCount := manager.CountClosedBreakers()

// 重置特定路径的断路器
manager.ResetBreaker("/api/users")

// 重置所有断路器
manager.ResetAllBreakers()
```

### 3. HTTP 中间件

```go
import (
    "net/http"
    "github.com/kamalyes/go-rpc-gateway/breaker"
)

// 创建管理器
manager := breaker.NewManager(5, 2, 10, 30*time.Second, []string{"/api/"}, []string{"/health"})

// 将中间件应用到 HTTP 处理器
mux := http.NewServeMux()
mux.HandleFunc("/api/users", breaker.HTTPMiddleware(manager)(userHandler))
mux.HandleFunc("/health", healthHandler) // 不会经过断路器保护

http.ListenAndServe(":8080", mux)
```

### 4. WebSocket 连接保护

```go
import (
    "github.com/kamalyes/go-rpc-gateway/breaker"
    wsc "github.com/kamalyes/go-wsc"
)

// 创建管理器
manager := breaker.NewManager(5, 2, 10, 30*time.Second, []string{}, []string{})

// 创建连接池
pool := breaker.NewWSPool(manager)

// 创建并注册 WebSocket 连接
wsconn, err := wsc.NewWsc(url, options...)
if err != nil {
    log.Fatal(err)
}

protectedConn, err := pool.Register(
    "user_123",              // connectionID
    wsconn,
    3,                       // maxRetries
    2.0,                     // retryBackoffFactor
    10*time.Second,          // healthCheckInterval
)
if err != nil {
    log.Fatal(err)
}

// 发送消息（带重试和断路器保护）
err := protectedConn.SendMessage("Hello, World!")
err := protectedConn.SendBinaryMessage([]byte{0x01, 0x02})

// 异步发送（加入队列）
err := protectedConn.QueueMessage("Async message")

// 检查连接健康状态
isHealthy := protectedConn.IsHealthy()

// 获取连接统计信息
stats := protectedConn.GetStats()

// 关闭连接
pool.Unregister("user_123")

// 获取所有连接的统计信息
allStats := pool.GetAllStats()
```

## 工作原理

### 断路器状态机

```
Closed (关闭)
  ↓ (失败次数 >= failureThreshold && 请求数 >= volumeThreshold)
Open (打开)
  ↓ (等待 timeout 时间)
HalfOpen (半开)
  ↓ (成功次数 >= successThreshold)
Closed (关闭)
  ↓ (失败)
Open (打开)
```

### 工作流程

1. **Closed 状态**：正常工作，记录请求结果
2. **Open 状态**：
   - 立即拒绝请求（Allow() 返回 false）
   - 等待 timeout 时间后转入 HalfOpen
3. **HalfOpen 状态**：
   - 允许部分请求通过（Allow() 返回 true）
   - 成功次数达到阈值 → 转入 Closed
   - 失败 1 次 → 立即转入 Open

## 配置示例（YAML）

```yaml
# go-config 中的配置
breaker:
  - module-name: circuit_breaker
    enabled: true
    failure-threshold: 5
    success-threshold: 2
    timeout: 30s
    volume-threshold: 10
    sliding-window-size: 100
    sliding-window-bucket: 1s
    prevention-paths:
      - /api/
    exclude-paths:
      - /health
      - /metrics

# WebSocket 断路器配置
websocket-breaker:
  - module-name: websocket_breaker
    enabled: true
    failure-threshold: 5
    success-threshold: 2
    timeout: 30s
    max-retries: 3
    retry-backoff-factor: 2.0
    health-check-interval: 10s
    message-queue-size: 1000
```

## 监控和调试

获取断路器统计信息用于监控：

```go
// 单个断路器统计
stats := breaker.GetStats()
// 返回：
// {
//   "state": "closed",
//   "total_requests": 1000,
//   "failed_requests": 50,
//   "failure_rate": 5.0,
//   "failure_count": 2,
//   "success_count": 0,
//   "last_failure_time": "2025-11-12T10:30:45Z",
//   "last_success_time": "2025-11-12T10:30:50Z",
//   "last_state_change": "2025-11-12T10:00:00Z",
//   "uptime": "30m50s"
// }

// 管理器统计（所有路径）
allStats := manager.GetStats()
// 返回 map[路径]统计信息

// 健康状态
health := manager.GetHealthStatus()
// 返回：
// {
//   "is_healthy": true,
//   "total_breakers": 5,
//   "open_breakers": 0,
//   "half_open_breakers": 0,
//   "closed_breakers": 5
// }
```

## 最佳实践

1. **配置合理的阈值**
   - FailureThreshold：5-10（根据业务容错能力调整）
   - SuccessThreshold：2-3（快速恢复）
   - VolumeThreshold：10-20（避免误触发）

2. **区分保护路径**
   - PreventionPaths：需要保护的业务 API
   - ExcludePaths：健康检查、监控等不需要保护的路径

3. **监控告警**
   - 监控 is_healthy 状态
   - 告警 open_breakers 数量
   - 定期查看失败率

4. **优雅降级**
   - 在 Open 状态时返回友好的错误信息
   - 记录详细日志便于排查

## 总结

通过将断路器功能模块化和配置化：
- **配置下沉到 go-config**：统一管理，易于动态更新
- **核心逻辑在 breaker 模块**：独立维护，清晰职责
- **中间件和适配器**：灵活集成到 HTTP 和 WebSocket
- **完整的监控和管理能力**：便于生产环境运维
