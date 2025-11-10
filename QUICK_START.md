# 🚀 快速开始指南

本指南帮助你快速上手 Go RPC Gateway 框架。

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
- HTTP: http://localhost:8080
- gRPC: localhost:9090
- 健康检查: http://localhost:8080/health
- 指标监控: http://localhost:8080/metrics

### 2️⃣ 使用配置文件 (推荐)

1. **复制配置模板**
```bash
cp template/config.yaml config.yaml
```

2. **创建 main.go**
```go
package main

import gateway "github.com/kamalyes/go-rpc-gateway"

func main() {
    gw, err := gateway.NewWithConfigFile("config.yaml")
    if err != nil {
        panic(err)
    }
    gw.Start()
}
```

3. **根据需要编辑 config.yaml**

### 3️⃣ 完整应用开发

参考 `template/advanced.go` 了解如何:
- ✅ 注册 gRPC 服务
- ✅ 添加 HTTP 路由
- ✅ 使用数据库和 Redis
- ✅ 启用性能分析
- ✅ 自定义中间件

## 🏗️ 项目结构建议

```
your-project/
├── main.go              # 入口文件
├── config.yaml          # 配置文件
├── proto/               # Protocol Buffers 定义
│   └── service.proto
├── service/             # 业务逻辑
│   └── user_service.go
├── handler/             # HTTP 处理器
│   └── api_handler.go
└── go.mod
```

## 📝 完整示例

```go
package main

import (
    "context"
    "net/http"
    
    gateway "github.com/kamalyes/go-rpc-gateway"
    "github.com/kamalyes/go-core/pkg/global"
    "google.golang.org/grpc"
)

func main() {
    // 1. 创建网关
    gw, err := gateway.NewWithConfigFile("config.yaml")
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
    
    // 4. 使用全局组件
    if global.DB != nil {
        global.LOGGER.Info("数据库已就绪")
    }
    
    if global.REDIS != nil {
        global.LOGGER.Info("Redis已就绪")
    }
    
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
gw, _ := gateway.NewWithConfigFile("config.yaml")   // 配置文件

// 注册服务
gw.RegisterService(func(s *grpc.Server) {})         // gRPC 服务
gw.RegisterHTTPRoute("/path", handlerFunc)          // HTTP 路由
gw.RegisterHTTPRoutes(map[string]http.HandlerFunc{}) // 批量路由

// 性能分析
gw.EnablePProf()                                    // 启用 pprof
gw.EnablePProfWithToken("token")                    // 带认证的 pprof
gw.EnablePProfForDevelopment()                      // 开发环境 pprof

// 启动和停止
gw.Start()                                          // 启动 (带 banner)
gw.StartSilent()                                    // 静默启动
gw.Stop()                                           // 优雅关闭
```

## 🔧 配置说明

### 最小配置 (config.yaml)

```yaml
gateway:
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
import "github.com/kamalyes/go-core/pkg/global"

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

## 📚 更多示例

查看 `template/` 目录:
- `quickstart.go` - 最简启动
- `main.go` - 标准模板
- `advanced.go` - 高级特性
- `with-database.go` - 数据库集成

查看 `examples/` 目录:
- `integration-demo/` - 四大核心库集成演示
- `complete-integration/` - 完整功能示例

## 🔗 相关链接

- [完整文档](./README.md)
- [配置指南](./docs/CONFIG_ANALYSIS.md)
- [中间件文档](./docs/MIDDLEWARE_GUIDE.md)
- [架构设计](./docs/ARCHITECTURE.md)
- [部署指南](./docs/DEPLOYMENT.md)

## ❓ 常见问题

### Q: 如何自定义端口?
A: 在 `config.yaml` 中设置:
```yaml
gateway:
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

### Q: 如何添加自定义中间件?
A: 使用中间件管理器:
```go
gw.GetMiddlewareManager().Use(yourMiddleware)
```

### Q: 如何查看所有配置项?
A: 查看 `examples/config-complete.yaml` 完整配置示例

## 🆘 获取帮助

- 查看示例代码: `examples/` 和 `template/`
- 阅读文档: `docs/`
- 提交 Issue: GitHub Issues

---

**现在开始构建你的微服务网关吧！** 🚀
