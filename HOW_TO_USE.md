# 🎯 Go RPC Gateway 使用手册

## 📝 问题

**"我想使用这个框架快速开发微服务，应该怎么开始？"**

## ✅ 三种使用方式

### 方式一：极简入口 (推荐给初学者)

创建 `main.go`:

```go
package main

import gateway "github.com/kamalyes/go-rpc-gateway"

func main() {
    gw, _ := gateway.New()
    gw.Start()
}
```

**特点：**

- ✅ 只需 3 行代码
- ✅ 使用默认配置
- ✅ 自动启动 HTTP(:8080) 和 gRPC(:9090)
- ✅ 自动启用健康检查、指标监控等功能

**运行：**

```bash
go run main.go
```

**访问：**

- 健康检查: <http://localhost:8080/health>
- 指标监控: <http://localhost:8080/metrics>

---

### 方式二：配置文件入口 (推荐给生产环境)

**1. 创建配置文件 `config.yaml`：**

```yaml
# 基础服务配置
server:
  name: my-gateway
  version: v1.0.0
  environment: development

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

# 日志配置
zap:
  level: info
  format: json
```

**2. 创建 `main.go`：**

```go
package main

import (
    gateway "github.com/kamalyes/go-rpc-gateway"
    "github.com/kamalyes/go-rpc-gateway/config"
)

func main() {
    // 加载配置文件
    configManager, err := config.NewConfigManager("config.yaml")
    if err != nil {
        panic(err)
    }
    
    // 获取网关配置
    cfg := configManager.GetGatewayConfig()
    
    // 创建网关
    gw, err := gateway.New(cfg)
    if err != nil {
        panic(err)
    }
    
    gw.Start()
}
```

**特点：**

- ✅ 配置外部化，方便管理
- ✅ 支持数据库、Redis、MinIO 等企业级组件
- ✅ 支持多环境配置（开发、测试、生产）

---

### 方式三：完整功能入口 (推荐给复杂项目)

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
        // pb.RegisterUserServiceServer(s, &userService{})
        // pb.RegisterProductServiceServer(s, &productService{})
    })
    
    // 3. 注册 HTTP 路由
    gw.RegisterHTTPRoute("/api/hello", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"message":"Hello World"}`))
    })
    
    // 4. 批量注册路由
    gw.RegisterHTTPRoutes(map[string]http.HandlerFunc{
        "/api/status": statusHandler,
        "/api/info":   infoHandler,
    })
    
    // 5. 启用功能特性
    gw.EnablePProf()      // 性能分析
    gw.EnableMonitoring() // 监控指标
    gw.EnableTracing()    // 链路追踪
    
    // 6. 启动服务
    if err := gw.Start(); err != nil {
        panic(err)
    }
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
    // 使用全局组件
    if global.DB != nil {
        // 数据库操作
    }
    
    if global.REDIS != nil {
        // Redis 操作
        global.REDIS.Ping(r.Context())
    }
    
    w.Write([]byte(`{"status":"ok"}`))
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte(`{"service":"my-service","version":"1.0.0"}`))
}
```

**特点：**

- ✅ 完整的 gRPC + HTTP 服务
- ✅ 使用全局组件 (DB, Redis, MinIO)
- ✅ 支持性能分析
- ✅ 结构化的代码组织

---

## � 核心 API 说明

### 创建网关

```go
// 方式1: 默认配置
gw, _ := gateway.New()

// 方式2: 使用配置对象
cfg := config.DefaultGatewayConfig()
gw, _ := gateway.New(cfg)

// 方式3: 通过配置管理器
configManager, _ := config.NewConfigManager("config.yaml")
cfg := configManager.GetGatewayConfig()
gw, _ := gateway.New(cfg)
```

### 注册服务

```go
// 注册 gRPC 服务
gw.RegisterService(func(s *grpc.Server) {
    pb.RegisterYourServiceServer(s, &yourService{})
})

// 注册单个 HTTP 路由
gw.RegisterHTTPRoute("/api/hello", handlerFunc)

// 注册多个 HTTP 路由
gw.RegisterHTTPRoutes(map[string]http.HandlerFunc{
    "/api/v1/users":    usersHandler,
    "/api/v1/products": productsHandler,
})

// 注册 HTTP 处理器
gw.RegisterHandler("/custom", customHandler)
```

### 启用功能特性

```go
// 启用性能分析
gw.EnablePProf()

// 启用监控指标
gw.EnableMonitoring()

// 启用链路追踪
gw.EnableTracing()

// 启用健康检查
gw.EnableHealth()

// 启用 Swagger 文档
gw.EnableSwagger()

// 检查功能状态
if gw.IsPProfEnabled() {
    // pprof 已启用
}
```

### 启动和停止

```go
// 启动服务 (带 banner)
gw.Start()

// 静默启动
gw.StartSilent()

// 带 banner 启动
gw.StartWithBanner()

// 停止服务
gw.Stop()
```

### 使用全局组件

```go
import "github.com/kamalyes/go-rpc-gateway/global"

// 使用数据库
if global.DB != nil {
    var users []User
    global.DB.Find(&users)
}

// 使用 Redis
if global.REDIS != nil {
    global.REDIS.Set(ctx, "key", "value", 0)
    val := global.REDIS.Get(ctx, "key").Val()
}

// 使用 MinIO
if global.MinIO != nil {
    global.MinIO.PutObject(ctx, bucket, objectName, reader, size, opts)
}

// 使用日志
if global.LOGGER != nil {
    global.LOGGER.Info("message")
    global.LOGGER.InfoKV("message", "key", "value")
}
```

---

## 🎯 实际项目结构

建议的项目结构：

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
├── model/               # 数据模型
│   └── user.go
└── go.mod
```

### 完整项目示例

**main.go**:

```go
package main

import (
    "your-project/handler"
    "your-project/service"
    
    gateway "github.com/kamalyes/go-rpc-gateway"
    "github.com/kamalyes/go-rpc-gateway/config"
    "google.golang.org/grpc"
)

func main() {
    // 加载配置
    configManager, err := config.NewConfigManager("config.yaml")
    if err != nil {
        panic(err)
    }
    
    cfg := configManager.GetGatewayConfig()
    
    // 创建网关
    gw, err := gateway.New(cfg)
    if err != nil {
        panic(err)
    }
    
    // 创建服务实例
    userSvc := &service.UserService{}
    
    // 注册 gRPC 服务
    gw.RegisterService(func(s *grpc.Server) {
        pb.RegisterUserServiceServer(s, userSvc)
    })
    
    // 注册 HTTP API
    apiHandler := &handler.APIHandler{UserService: userSvc}
    gw.RegisterHTTPRoutes(map[string]http.HandlerFunc{
        "/api/users":     apiHandler.GetUsers,
        "/api/users/new": apiHandler.CreateUser,
    })
    
    // 启用监控功能
    gw.EnablePProf()
    gw.EnableMonitoring()
    
    // 启动服务
    if err := gw.Start(); err != nil {
        panic(err)
    }
}
```

**service/user_service.go**:

```go
package service

import (
    "context"
    
    "your-project/model"
    "github.com/kamalyes/go-rpc-gateway/global"
    pb "your-project/proto"
)

type UserService struct {
    pb.UnimplementedUserServiceServer
}

func (s *UserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
    var user model.User
    
    // 使用全局数据库连接
    if err := global.DB.First(&user, req.Id).Error; err != nil {
        return nil, err
    }
    
    return &pb.GetUserResponse{
        User: &pb.User{
            Id:    user.ID,
            Name:  user.Name,
            Email: user.Email,
        },
    }, nil
}

func (s *UserService) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
    user := &model.User{
        Name:  req.Name,
        Email: req.Email,
    }
    
    if err := global.DB.Create(user).Error; err != nil {
        return nil, err
    }
    
    return &pb.CreateUserResponse{
        User: &pb.User{
            Id:    user.ID,
            Name:  user.Name,
            Email: user.Email,
        },
    }, nil
}
```

**handler/api_handler.go**:

```go
package handler

import (
    "encoding/json"
    "net/http"
    
    "your-project/service"
    "github.com/kamalyes/go-rpc-gateway/global"
)

type APIHandler struct {
    UserService *service.UserService
}

func (h *APIHandler) GetUsers(w http.ResponseWriter, r *http.Request) {
    var users []model.User
    
    if err := global.DB.Find(&users).Error; err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(users)
}

func (h *APIHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name  string `json:"name"`
        Email string `json:"email"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    user := &model.User{
        Name:  req.Name,
        Email: req.Email,
    }
    
    if err := global.DB.Create(user).Error; err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}
```

---

## 💻 命令行工具

构建和运行项目：

```bash
# 初始化项目
go mod init your-project
go get github.com/kamalyes/go-rpc-gateway

# 构建项目
go build -o bin/app main.go

# 运行项目
./bin/app

# 开发模式运行
go run main.go
```

## ✅ 测试服务

启动后测试服务：

```bash
# 检查健康状态
curl http://localhost:8080/health

# 查看指标监控
curl http://localhost:8080/metrics

# 测试 API
curl http://localhost:8080/api/users

# 性能分析 (如果启用了 PProf)
curl http://localhost:8080/debug/pprof/
```

## 🔗 相关资源

- [完整示例代码](./examples/) - 查看更多使用示例
- [配置文档](./docs/CONFIG_ANALYSIS.md) - 详细配置说明
- [中间件指南](./docs/MIDDLEWARE_GUIDE.md) - 中间件使用说明
- [部署指南](./docs/DEPLOYMENT.md) - 生产环境部署

## ❓ 常见问题

### Q: 如何自定义端口?

A: 在 `config.yaml` 中设置:

```yaml
server:
  http:
    port: 3000
  grpc:
    port: 50051
```

### Q: 如何启用数据库?

A: 在配置文件中添加数据库配置:

```yaml
mysql:
  host: "localhost"
  port: 3306
  username: "root"
  password: "password"
  dbname: "mydb"
```

### Q: 如何添加自定义中间件?

A: 目前通过服务器层面添加，未来版本会支持网关层面的中间件注册

### Q: 如何查看所有配置项?

A: 查看 [完整配置示例](./examples/config-complete.yaml)

## 🆘 获取帮助

- 查看示例代码: `examples/` 目录
- 阅读详细文档: `docs/` 目录
- 提交 Issue: GitHub Issues

---

**现在开始使用 Go RPC Gateway 构建你的微服务吧！** 🚀
