# PProf 性能分析中间件

## 概述

PProf 中间件为 go-rpc-gateway 提供了强大的性能分析功能，集成了 Go 标准库的 `net/http/pprof` 包，同时添加了认证、IP 白名单、日志记录等安全特性。

## 功能特性

- 🔒 **安全认证**: 支持 Token 认证，保护 pprof 端点安全
- 🌐 **IP 白名单**: 限制只有特定 IP 地址可以访问
- 📝 **访问日志**: 记录所有 pprof 访问请求
- ⚙️ **环境控制**: 支持开发/生产环境分别配置
- 🧪 **性能测试场景**: 内置多种性能测试场景，便于压测分析
- 🔧 **自定义处理器**: 支持注册自定义性能分析处理器
- ⏱️ **超时控制**: 可配置请求超时时间

## 内置 PProf 端点

| 端点 | 描述 |
|------|------|
| `/debug/pprof/` | 索引页面，显示所有可用的性能分析处理器链接 |
| `/debug/pprof/allocs` | 内存分配采样信息 |
| `/debug/pprof/block` | 阻塞同步原语的堆栈跟踪 |
| `/debug/pprof/cmdline` | 当前程序的命令行参数 |
| `/debug/pprof/goroutine` | 所有 goroutine 的堆栈跟踪 |
| `/debug/pprof/heap` | 活动对象的内存分配采样 |
| `/debug/pprof/mutex` | 互斥锁争用的堆栈跟踪 |
| `/debug/pprof/profile` | CPU 性能分析 (可用 `?seconds=30` 指定时间) |
| `/debug/pprof/threadcreate` | 创建新 OS 线程的堆栈跟踪 |
| `/debug/pprof/trace` | 程序执行跟踪 (可用 `?seconds=5` 指定时间) |
| `/debug/pprof/symbol` | 符号查找 (POST 请求) |

## 内置性能测试场景

### GC 测试场景

- `/debug/pprof/gc/small-objects` - 大量小对象 GC 测试
- `/debug/pprof/gc/large-objects` - 大对象 GC 测试  
- `/debug/pprof/gc/high-cpu` - 高 CPU 使用率 GC 测试
- `/debug/pprof/gc/cyclic-objects` - 循环引用对象 GC 测试
- `/debug/pprof/gc/short-lived-objects` - 短生命周期对象 GC 测试
- `/debug/pprof/gc/long-lived-objects` - 长生命周期对象 GC 测试
- `/debug/pprof/gc/complex-structure` - 复杂数据结构 GC 测试
- `/debug/pprof/gc/concurrent` - 并发 GC 测试

### 内存测试场景

- `/debug/pprof/memory/allocate` - 内存分配测试
- `/debug/pprof/memory/leak` - 内存泄漏模拟
- `/debug/pprof/memory/fragmentation` - 内存碎片化测试

### CPU 测试场景  

- `/debug/pprof/cpu/intensive` - CPU 密集型计算
- `/debug/pprof/cpu/recursive` - 递归计算测试

### 并发测试场景

- `/debug/pprof/goroutine/spawn` - 大量 Goroutine 创建
- `/debug/pprof/goroutine/leak` - Goroutine 泄漏模拟
- `/debug/pprof/mutex/contention` - 互斥锁竞争测试

### 清理场景

- `/debug/pprof/cleanup/all` - 清理所有持有对象并触发 GC

## 基础使用

### 1. 基本配置

```go
import "github.com/kamalyes/go-rpc-gateway/middleware"

// 创建默认配置
pprofConfig := middleware.DefaultPProfConfig()
pprofConfig.Enabled = true
pprofConfig.AuthToken = "your-secret-token"
pprofConfig.AllowedIPs = []string{"127.0.0.1", "::1"}
```

### 2. 集成到中间件管理器

```go
// 创建中间件管理器
manager, err := middleware.NewManager(nil, nil)
if err != nil {
    log.Fatal(err)
}

// 配置 pprof
manager.WithPProfConfig(pprofConfig)

// 获取中间件
pprofMiddleware := manager.PProfMiddleware()
```

### 3. 在 HTTP 服务器中使用

```go
mux := http.NewServeMux()
mux.HandleFunc("/", yourHandler)

// 应用中间件
handler := manager.HTTPMiddleware(mux)

server := &http.Server{
    Addr:    ":8080",
    Handler: handler,
}
```

### 4. 在 Gin 中使用

```go
router := gin.Default()

// 转换为 Gin 中间件
router.Use(func(c *gin.Context) {
    pprofMiddleware := manager.PProfMiddleware()
    handler := pprofMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        c.Next()
    }))
    handler.ServeHTTP(c.Writer, c.Request)
})
```

## 高级配置

### 完整配置示例

```go
pprofConfig := &middleware.PProfConfig{
    // 基础设置
    Enabled:     true,
    PathPrefix:  "/debug/pprof",
    
    // 安全设置
    RequireAuth: true,
    AuthToken:   os.Getenv("PPROF_TOKEN"),
    AllowedIPs: []string{
        "127.0.0.1",
        "::1",
        "10.0.0.0/8",
        "192.168.0.0/16",
    },
    
    // 环境控制
    DevModeOnly: true,  // 只在开发环境启用
    
    // 日志和超时
    EnableLogging: true,
    Logger:        logger,
    Timeout:       30,
    
    // 自定义处理器
    CustomHandlers: make(map[string]http.HandlerFunc),
}
```

### 生产环境配置建议

```go
func productionPProfConfig() *middleware.PProfConfig {
    return &middleware.PProfConfig{
        Enabled:     false, // 生产环境默认关闭
        PathPrefix:  "/admin/debug/pprof", // 非标准路径
        RequireAuth: true,
        AuthToken:   os.Getenv("PPROF_SECRET_TOKEN"), // 强制环境变量
        AllowedIPs: []string{
            "10.0.0.0/8",    // 仅允许内网
            "172.16.0.0/12",
            "192.168.0.0/16",
        },
        EnableLogging: true,
        DevModeOnly:   false,
        Timeout:       15, // 较短超时
    }
}
```

## 认证方式

### 1. Bearer Token (推荐)

```bash
curl -H "Authorization: Bearer your-secret-token" \
     http://localhost:8080/debug/pprof/
```

### 2. Query Parameter

```bash
curl http://localhost:8080/debug/pprof/?token=your-secret-token
```

## 性能分析工具使用

### 1. CPU 性能分析

```bash
# 生成 CPU profile (30秒)
curl -H "Authorization: Bearer your-token" \
     "http://localhost:8080/debug/pprof/profile?seconds=30" \
     -o cpu.prof

# 使用 go tool 分析
go tool pprof cpu.prof
```

### 2. 内存分析

```bash
# 获取 heap profile
curl -H "Authorization: Bearer your-token" \
     "http://localhost:8080/debug/pprof/heap" \
     -o heap.prof

# 分析内存使用
go tool pprof heap.prof
```

### 3. Goroutine 分析

```bash
# 获取 goroutine 信息
curl -H "Authorization: Bearer your-token" \
     "http://localhost:8080/debug/pprof/goroutine" \
     -o goroutine.prof

go tool pprof goroutine.prof
```

### 4. 执行跟踪

```bash
# 生成执行跟踪 (5秒)
curl -H "Authorization: Bearer your-token" \
     "http://localhost:8080/debug/pprof/trace?seconds=5" \
     -o trace.out

# 查看跟踪
go tool trace trace.out
```

## 自定义性能测试场景

### 注册自定义处理器

```go
pprofConfig := middleware.DefaultPProfConfig()

// 注册自定义内存测试
pprofConfig.RegisterCustomHandler("custom/memory-stress", func(w http.ResponseWriter, r *http.Request) {
    var data [][]byte
    for i := 0; i < 1000; i++ {
        data = append(data, make([]byte, 1024*1024)) // 1MB
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"message": "Custom memory stress test completed"}`))
})
```

### 使用内置测试场景

```go
// 创建并注册内置场景
scenarios := middleware.NewPProfScenarios()
scenarios.RegisterScenarios(pprofConfig)
```

## 安全注意事项

1. **生产环境**: 默认应该禁用 pprof，只在需要调试时临时启用
2. **认证**: 始终启用认证，使用强密码或复杂 token
3. **IP 白名单**: 限制只有可信 IP 地址可以访问
4. **路径隐藏**: 生产环境使用非标准路径
5. **日志监控**: 启用访问日志，监控异常访问

## 监控和告警

```go
// 记录 pprof 访问
pprofConfig.EnableLogging = true
pprofConfig.Logger = logger

// 日志示例
// {"level":"info","msg":"pprof access","method":"GET","path":"/debug/pprof/heap","client_ip":"127.0.0.1","status_code":200,"duration":"45.123ms"}
```

## 故障排查

### 常见问题

1. **403 Forbidden**: 检查 IP 白名单配置
2. **401 Unauthorized**: 检查认证 token 是否正确
3. **404 Not Found**: 确认 pprof 已启用且路径正确
4. **超时**: 调整 `Timeout` 配置

### 调试日志

```go
pprofConfig.EnableLogging = true
pprofConfig.Logger = logger.With(zap.String("component", "pprof"))
```

## 最佳实践

1. **开发环境**: 启用所有调试功能，无需认证
2. **测试环境**: 启用 pprof，启用认证，记录访问日志  
3. **生产环境**: 默认禁用，必要时临时启用，严格安全控制
4. **性能测试**: 使用内置场景进行各种性能测试
5. **监控集成**: 将 pprof 访问日志集成到监控系统

## 示例项目

查看 `examples/pprof_example.go` 获取完整的使用示例。

```bash
# 运行示例
cd examples
go run pprof_example.go

# 访问 pprof
curl -H "Authorization: Bearer demo-token-123" \
     http://localhost:8080/debug/pprof/
```
