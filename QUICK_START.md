# 🚀 快速开始指南

本指南帮助你在 5 分钟内上手 Go RPC Gateway 框架。

## 📦 安装

```bash
go get github.com/kamalyes/go-rpc-gateway
```

## 🎯 三种使用方式

### 1️⃣ 极简启动 (30秒上手)

创建 `main.go`:

```go
package main

import gateway "github.com/kamalyes/go-rpc-gateway"

func main() {
    gw, _ := gateway.New()
    gw.Start()
}
```

运行:

```bash
go run main.go
```

访问:

- HTTP: <http://localhost:8080>
- gRPC: localhost:9090
- 健康检查: <http://localhost:8080/health>
- 指标监控: <http://localhost:8080/metrics>

### 2️⃣ 使用配置文件 (推荐)

1. **创建 config.yaml**

```yaml
# 基础服务配置
server:
  name: my-gateway
  version: v1.0.0

# HTTP/gRPC 端口配置  
server:
  http:
    port: 8080
  grpc:
    port: 9090

# 数据库配置 (可选)
mysql:
  host: "localhost"
  port: 3306
  dbname: "mydb"
  username: "root"
  password: "password"

# Redis 配置 (可选)
redis:
  host: "localhost"
  port: 6379
```

2. **创建 main.go**

```go
package main

import (
    gateway "github.com/kamalyes/go-rpc-gateway"
    "github.com/kamalyes/go-rpc-gateway/config"
)

func main() {
    configManager, err := config.NewConfigManager("config.yaml")
    if err != nil {
        panic(err)
    }
    
    cfg := configManager.GetGatewayConfig()
    
    gw, err := gateway.New(cfg)
    if err != nil {
        panic(err)
    }
    
    gw.Start()
}
```

### 3️⃣ 完整应用开发

```go
package main

import (
    "net/http"
    
    gateway "github.com/kamalyes/go-rpc-gateway"
    "github.com/kamalyes/go-rpc-gateway/global"
    "google.golang.org/grpc"
)

func main() {
    // 1. 创建网关
    gw, err := gateway.New()
    if err != nil {
        panic(err)
    }
    
    // 2. 注册 gRPC 服务
    gw.RegisterService(func(s *grpc.Server) {
        // pb.RegisterYourServiceServer(s, &yourService{})
    })
    
    // 3. 注册 HTTP 路由
    gw.RegisterHTTPRoute("/api/hello", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"message":"Hello World"}`))
    })
    
    // 4. 启用功能特性
    gw.EnablePProf()      // 性能分析
    gw.EnableMonitoring() // 监控指标
    gw.EnableTracing()    // 链路追踪
    
    // 5. 启动服务
    if err := gw.Start(); err != nil {
        panic(err)
    }
}
```

## 🎨 框架特性

### 开箱即用的功能

✅ **自动初始化**

- HTTP/gRPC 双协议服务器
- 健康检查端点
- Prometheus 指标监控
- 结构化日志系统

✅ **企业级组件** (通过配置文件启用)

- MySQL/PostgreSQL 数据库 (GORM)
- Redis 缓存 (单机/集群/哨兵)
- MinIO 对象存储
- RabbitMQ 消息队列
- Consul 服务发现

✅ **15+ 内置中间件**

- 访问日志 (go-logger)
- 限流控制 (令牌桶)
- CORS 跨域
- 请求签名验证
- 恢复捕获 (Panic Recovery)
- 请求 ID 追踪
- 多语言支持 (19种语言)
- 链路追踪 (OpenTelemetry)

### 核心 API

```go
// 创建网关
gw, _ := gateway.New()                              // 默认配置

// 注册服务
gw.RegisterService(func(s *grpc.Server) {})         // gRPC 服务
gw.RegisterHTTPRoute("/path", handlerFunc)          // HTTP 路由
gw.RegisterHTTPRoutes(map[string]http.HandlerFunc{}) // 批量路由

// 启用功能特性
gw.EnablePProf()                                    // 启用 pprof
gw.EnableMonitoring()                               // 启用监控
gw.EnableTracing()                                  // 启用追踪
gw.EnableHealth()                                   // 启用健康检查

// 启动和停止
gw.Start()                                          // 启动 (带 banner)
gw.StartSilent()                                    // 静默启动
gw.Stop()                                           // 优雅关闭
```

## 🔧 配置说明

### 最小配置 (config.yaml)

```yaml
server:
  http:
    port: 8080
  grpc:
    port: 9090
```

### 启用数据库

```yaml
mysql:
  host: "localhost"
  port: 3306
  dbname: "mydb"
  username: "root"
  password: "password"
```

使用数据库:

```go
import "github.com/kamalyes/go-rpc-gateway/global"

// global.DB 自动初始化
var users []User
global.DB.Find(&users)
```

### 启用 Redis

```yaml
redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0
```

使用 Redis:

```go
global.REDIS.Set(ctx, "key", "value", 0)
val := global.REDIS.Get(ctx, "key").Val()
```

## 📚 示例项目

### 简单 API 服务

```go
package main

import (
    "encoding/json"
    "net/http"
    
    gateway "github.com/kamalyes/go-rpc-gateway"
    "github.com/kamalyes/go-rpc-gateway/global"
)

type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

func main() {
    gw, _ := gateway.New()
    
    // 注册 API 路由
    gw.RegisterHTTPRoutes(map[string]http.HandlerFunc{
        "/api/users": getUsersHandler,
        "/api/hello": helloHandler,
    })
    
    gw.Start()
}

func getUsersHandler(w http.ResponseWriter, r *http.Request) {
    users := []User{
        {ID: 1, Name: "Alice"},
        {ID: 2, Name: "Bob"},
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(users)
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
    response := map[string]string{
        "message": "Hello from Go RPC Gateway!",
        "version": "1.0.0",
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

### 带数据库的服务

```go
package main

import (
    "net/http"
    
    gateway "github.com/kamalyes/go-rpc-gateway"
    "github.com/kamalyes/go-rpc-gateway/config"
    "github.com/kamalyes/go-rpc-gateway/global"
)

type User struct {
    ID   uint   `gorm:"primaryKey"`
    Name string `gorm:"not null"`
}

func main() {
    // 加载配置
    configManager, _ := config.NewConfigManager("config.yaml")
    cfg := configManager.GetGatewayConfig()
    
    gw, _ := gateway.New(cfg)
    
    // 自动迁移数据库
    if global.DB != nil {
        global.DB.AutoMigrate(&User{})
    }
    
    // 注册路由
    gw.RegisterHTTPRoute("/api/users", usersHandler)
    
    gw.Start()
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
    if global.DB == nil {
        http.Error(w, "Database not available", 500)
        return
    }
    
    var users []User
    global.DB.Find(&users)
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(users)
}
```

## ❓ 常见问题

### Q: 如何自定义端口?

A: 在配置文件中设置:

```yaml
server:
  http:
    port: 3000
  grpc:
    port: 50051
```

### Q: 如何启用 HTTPS?

A: 配置 TLS:

```yaml
security:
  tls:
    enabled: true
    cert_file: "cert.pem"
    key_file: "key.pem"
```

### Q: 如何查看性能分析?

A: 启用 PProf 后访问:

```bash
# 启用 PProf
gw.EnablePProf()

# 访问性能分析
curl http://localhost:8080/debug/pprof/
```

### Q: 如何添加 gRPC 服务?

A: 注册 gRPC 服务:

```go
gw.RegisterService(func(s *grpc.Server) {
    pb.RegisterYourServiceServer(s, &yourService{})
})
```

### Q: 如何使用中间件?

A: 框架内置 15+ 中间件，通过配置文件启用:

```yaml
middleware:
  cors:
    enabled: true
  rate_limit:
    enabled: true
    rate: 100
```

## 🔗 相关链接

- [完整文档](./README.md)
- [使用手册](./HOW_TO_USE.md)
- [配置指南](./docs/CONFIG_ANALYSIS.md)
- [中间件文档](./docs/MIDDLEWARE_GUIDE.md)
- [架构设计](./docs/ARCHITECTURE.md)
- [部署指南](./docs/DEPLOYMENT.md)

## 🆘 获取帮助

- 查看示例代码: `examples/` 目录
- 阅读文档: `docs/` 目录  
- 提交 Issue: GitHub Issues
- 参与讨论: GitHub Discussions

---

**🎉 现在开始构建你的微服务网关吧！**

完成以上任一示例后，你已经掌握了 Go RPC Gateway 的基本用法，可以开始构建更复杂的应用了。
