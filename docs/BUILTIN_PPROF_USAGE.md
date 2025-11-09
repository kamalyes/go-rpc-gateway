# Go RPC Gateway - 内置 PProf 功能

## 简介

go-rpc-gateway 现在内置了强大的 pprof 性能分析功能！你不再需要手动配置复杂的中间件，只需要一行代码就可以启用完整的性能分析功能。

## 🚀 快速开始 (30秒启用pprof)

### 最简单的用法

```go
package main

import (
    "log"
    "github.com/kamalyes/go-rpc-gateway"
)

func main() {
    // 创建Gateway
    gw, err := gateway.New()
    if err != nil {
        log.Fatal(err)
    }

    // 一键启用pprof! 🎉
    gw.EnablePProf()

    // 启动服务
    gw.Start()
}
```

就这么简单！现在你可以访问：
- 🌐 **Web界面**: http://localhost:8080/
- 📊 **PProf**: http://localhost:8080/debug/pprof/
- 🔐 **认证token**: `gateway-pprof-2024` (默认)

## 💡 使用方式

### 1. 环境变量配置 (推荐)

```bash
# 设置自定义认证token
export PPROF_TOKEN="your-secret-token"

# 运行程序
go run main.go
```

### 2. 代码中指定token

```go
gw.EnablePProfWithToken("my-custom-token")
```

### 3. 开发环境专用

```go
// 只允许本地访问，使用开发专用token
gw.EnablePProfForDevelopment()
```

### 4. 完全自定义配置

```go
gw.EnablePProfWithOptions(gateway.PProfOptions{
    Enabled:     true,
    AuthToken:   "production-token",
    PathPrefix:  "/admin/debug/pprof",      // 自定义路径
    AllowedIPs:  []string{"192.168.1.0/24"}, // IP白名单  
    DevModeOnly: false,
})
```

## 🧪 内置性能测试场景

基于你原始的pprof代码，我们内置了丰富的性能测试场景：

### GC 测试场景
- **小对象GC** - 创建10万个小对象，测试小对象GC性能
- **大对象GC** - 创建1000个1MB对象，测试大对象GC性能  
- **高CPU GC** - 多goroutine密集计算，测试高CPU使用下的GC
- **循环对象GC** - 创建循环引用对象，测试复杂引用的GC
- **生命周期GC** - 测试短/长生命周期对象的GC表现
- **复杂结构GC** - 二叉树等复杂数据结构的GC测试
- **并发GC** - 多goroutine并发创建对象的GC测试

### 其他测试场景
- **内存分配测试** - 各种大小内存块的分配测试
- **CPU密集测试** - CPU密集型计算和递归测试
- **Goroutine测试** - 大量goroutine创建和管理测试
- **互斥锁测试** - 互斥锁竞争和性能测试

## 📊 使用pprof分析

启用pprof后，你可以使用标准的Go工具进行性能分析：

### 1. 命令行分析

```bash
# CPU性能分析 (30秒采样)
curl -H "Authorization: Bearer your-token" \
     "http://localhost:8080/debug/pprof/profile?seconds=30" -o cpu.prof
go tool pprof cpu.prof

# 内存分析
curl -H "Authorization: Bearer your-token" \
     "http://localhost:8080/debug/pprof/heap" -o heap.prof
go tool pprof heap.prof

# Goroutine分析
curl -H "Authorization: Bearer your-token" \
     "http://localhost:8080/debug/pprof/goroutine" -o goroutine.prof
go tool pprof goroutine.prof
```

### 2. Web界面分析

```bash
# 启动pprof web界面
go tool pprof -http=:8081 cpu.prof
```

### 3. 实时性能测试

访问 http://localhost:8080/ 在Web界面中：
1. 运行内置的性能测试场景
2. 立即查看对应的pprof数据
3. 使用go tool pprof分析结果

## 🔒 安全特性

- ✅ **Token认证** - 支持Bearer token和query参数认证
- ✅ **IP白名单** - 限制特定IP地址访问
- ✅ **访问日志** - 记录所有pprof访问请求
- ✅ **环境控制** - 可配置只在开发环境启用

## 🎯 在现有项目中集成

如果你已经有一个Gateway项目，只需要添加一行代码：

```go
// 在你现有的Gateway代码中
gw, err := gateway.New(yourConfig)
if err != nil {
    log.Fatal(err)
}

// 添加你的业务路由
gw.RegisterHTTPRoute("/api/users", handleUsers)
gw.RegisterHTTPRoute("/api/orders", handleOrders)

// 最后启用pprof (不影响现有功能)
gw.EnablePProf() // 👈 就这一行！

gw.Start()
```

## 📈 最佳实践

### 开发环境
```go
gw.EnablePProfForDevelopment()
```

### 测试环境  
```go
gw.EnablePProfWithToken(os.Getenv("PPROF_TOKEN"))
```

### 生产环境
```go
// 默认禁用，需要时临时启用
if os.Getenv("ENABLE_PPROF") == "true" {
    gw.EnablePProfWithOptions(gateway.PProfOptions{
        Enabled:    true,
        AuthToken:  os.Getenv("PPROF_SECRET_TOKEN"),
        PathPrefix: "/admin/debug/pprof",  // 隐藏路径
        AllowedIPs: []string{"10.0.0.0/8"}, // 只允许内网
    })
}
```

## 🔧 API 方法总结

| 方法 | 说明 |
|------|------|
| `EnablePProf()` | 一键启用，使用默认配置 |
| `EnablePProfWithToken(token)` | 指定认证token启用 |
| `EnablePProfForDevelopment()` | 开发环境专用配置 |
| `EnablePProfWithOptions(options)` | 完全自定义配置 |
| `IsPProfEnabled()` | 检查是否已启用 |
| `GetPProfEndpoints()` | 获取所有可用端点信息 |

## 🎉 总结

现在你可以在任何go-rpc-gateway项目中，用一行代码启用完整的pprof功能！

- ✅ **零配置启用** - `gw.EnablePProf()`
- ✅ **内置测试场景** - 基于你的原始代码
- ✅ **Web管理界面** - 美观的Web界面管理
- ✅ **企业级安全** - 认证、IP控制、日志记录
- ✅ **生产就绪** - 灵活的环境配置

让性能分析变得前所未有的简单！🚀