# 🚀 快速开始指南

> 5分钟快速上手 Go RPC Gateway

---

## 📦 安装

```bash
go get github.com/kamalyes/go-rpc-gateway
```

**系统要求**:
- Go 1.23+
- 支持 Linux / macOS / Windows

---

## 🎯 三种启动方式

### 方式一：极简启动 (30秒)

适合快速体验和开发测试。

```go
package main

import gateway "github.com/kamalyes/go-rpc-gateway"

func main() {
    // 使用默认配置启动
    gw, _ := gateway.NewGateway().
        WithSearchPath("./config").  // 自动查找配置文件
        BuildAndStart()
    
    gw.WaitForShutdown()  // 等待 Ctrl+C
}
```

**访问服务**:
- HTTP: http://localhost:8080
- gRPC: localhost:9090
- 健康检查: http://localhost:8080/health

---

### 方式二：配置文件启动 (推荐)

适合生产环境部署。

**1. 创建配置文件** `config/gateway.yaml`:

```yaml
name: my-gateway
version: v1.0.0
environment: development

http_server:
  port: 8080

grpc:
  server:
    port: 9090

# 数据库配置
mysql:
  enabled: true
  host: localhost
  port: 3306
  username: root
  password: password
  dbname: mydb

# Redis 配置
redis:
  enabled: true
  host: localhost
  port: 6379
```

**2. 启动代码**:

```go
package main

import gateway "github.com/kamalyes/go-rpc-gateway"

func main() {
    gw, err := gateway.NewGateway().
        WithConfigPath("./config/gateway.yaml").
        WithHotReload(nil).  // 启用配置热重载
        BuildAndStart()
    
    if err != nil {
        panic(err)
    }
    
    gw.WaitForShutdown()
}
```

---

### 方式三：完整功能开发

适合复杂业务场景。

```go
package main

import (
    "context"
    "encoding/json"
    "net/http"
    
    gateway "github.com/kamalyes/go-rpc-gateway"
    "github.com/kamalyes/go-rpc-gateway/global"
    "github.com/kamalyes/go-rpc-gateway/server"
    "google.golang.org/grpc"
)

type User struct {
    ID   uint   `json:"id" gorm:"primaryKey"`
    Name string `json:"name"`
}

func main() {
    // 1. 构建网关
    gw, err := gateway.NewGateway().
        WithConfigPath("./config/gateway.yaml").
        WithEnvironment(gateway.EnvDevelopment).
        Build()  // 先构建，后续手动启动
    
    if err != nil {
        panic(err)
    }
    
    // 2. 注册 gRPC 服务
    gw.RegisterService(func(s *grpc.Server) {
        // pb.RegisterYourServiceServer(s, &yourService{})
    })
    
    // 3. 注册 HTTP 路由
    gw.RegisterHTTPRoutes(map[string]http.HandlerFunc{
        "/api/users":  handleUsers,
        "/api/health": handleHealth,
    })
    
    // 4. 启用功能特性
    gw.EnableFeature(server.FeaturePProf)      // 性能分析
    gw.EnableFeature(server.FeatureMonitoring) // Prometheus 监控
    gw.EnableFeature(server.FeatureTracing)    // 链路追踪
    gw.EnableFeature(server.FeatureSwagger)    // API 文档
    
    // 5. 数据库迁移 (如果启用了数据库)
    if global.DB != nil {
        global.DB.AutoMigrate(&User{})
    }
    
    // 6. 启动服务
    if err := gw.Start(); err != nil {
        panic(err)
    }
    
    gw.WaitForShutdown()
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
    if global.DB == nil {
        http.Error(w, "Database not available", 500)
        return
    }
    
    var users []User
    global.DB.Find(&users)
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(users)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
    status := map[string]interface{}{
        "status":  "healthy",
        "version": "v1.0.0",
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(status)
}
```

---

## 🎨 访问服务

启动成功后，可以访问以下端点：

| 服务 | 地址 | 说明 |
|------|------|------|
| **HTTP API** | http://localhost:8080 | RESTful API |
| **gRPC** | localhost:9090 | gRPC 服务 |
| **健康检查** | http://localhost:8080/health | 服务健康状态 |
| **监控指标** | http://localhost:8080/metrics | Prometheus 指标 |
| **API 文档** | http://localhost:8080/swagger/ | Swagger UI |
| **性能分析** | http://localhost:8080/debug/pprof/ | PProf 工具 |

---

## 📚 下一步

- 📖 [配置指南](CONFIG_GUIDE.md) - 详细配置说明
- 🏗️ [架构设计](ARCHITECTURE.md) - 了解系统架构
- 🔧 [API 参考](API_REFERENCE.md) - 完整 API 文档
- 🚀 [部署指南](DEPLOYMENT.md) - 生产环境部署
- 💡 [最佳实践](BEST_PRACTICES.md) - 开发建议

---

## ❓ 常见问题

### Q: 如何修改端口？

A: 在配置文件中设置：

```yaml
http_server:
  port: 3000

grpc:
  server:
    port: 50051
```

### Q: 如何使用全局数据库连接？

A: 配置数据库后，全局变量自动初始化：

```go
import "github.com/kamalyes/go-rpc-gateway/global"

// 直接使用
var users []User
global.DB.Find(&users)
```

### Q: 如何启用 HTTPS？

A: 配置 TLS 证书：

```yaml
security:
  tls:
    enabled: true
    cert_file: "cert.pem"
    key_file: "key.pem"
```

### Q: 如何添加自定义初始化逻辑？

A: 实现 `Initializer` 接口并注册：

```go
type MyInitializer struct{}

func (i *MyInitializer) Name() string { return "MyComponent" }
func (i *MyInitializer) Priority() int { return 20 }
func (i *MyInitializer) Initialize(ctx context.Context, cfg *gwconfig.Gateway) error {
    // 你的初始化逻辑
    return nil
}
func (i *MyInitializer) Cleanup() error { return nil }
func (i *MyInitializer) HealthCheck() error { return nil }

// 在 global/initializer.go 中注册
chain.Register(&MyInitializer{})
```

---

**🎉 现在您已经掌握了基础用法，开始构建您的微服务吧！**
