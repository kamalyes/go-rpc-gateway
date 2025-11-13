# 🎯 Go RPC Gateway 使用手册

## 📝 问题

**"我想使用这个框架快速开发微服务，应该怎么开始？"**

本文档将详细介绍如何使用 Go RPC Gateway 框架，从基础使用到高级功能，帮助您快速掌握这个企业级微服务网关框架。

---

## ✅ 四种使用方式

### 方式一：极简入口 (推荐给初学者)

**最快30秒上手，只需3行代码：**

```go
package main

import gateway "github.com/kamalyes/go-rpc-gateway"

func main() {
    gw, _ := gateway.NewGateway().
        WithSearchPath("./config").
        BuildAndStart()
    
    gw.WaitForShutdown()  // 等待关闭信号
}
```

**特点：**

- ✅ 只需 4 行代码
- ✅ 使用默认配置或自动发现配置文件
- ✅ 自动启动 HTTP(:8080) 和 gRPC(:9090)
- ✅ 自动启用健康检查、指标监控等功能
- ✅ 支持优雅关闭

**运行：**

```bash
go run main.go
```

**访问：**

- HTTP API: <http://localhost:8080>
- 健康检查: <http://localhost:8080/health>
- 指标监控: <http://localhost:8080/metrics>
- gRPC: localhost:9090

---

### 方式二：配置文件入口 (推荐给生产环境)

**1. 创建配置文件 `config.yaml`：**

```yaml
# 基础服务配置
name: my-gateway
version: v2.1.0
environment: development  # development, testing, production
debug: true

# HTTP/gRPC 端口配置  
http_server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

grpc:
  server:
    host: 0.0.0.0
    port: 9090

# 数据库配置 (可选)
mysql:
  enabled: true
  host: "localhost"
  port: 3306
  dbname: "mydb"
  username: "root"
  password: "password"
  max_idle_conns: 10
  max_open_conns: 100

# Redis 配置 (可选)
redis:
  enabled: true
  host: "localhost"
  port: 6379
  db: 0
  pool_size: 10

# MinIO 对象存储 (可选)
minio:
  enabled: true
  endpoint: "localhost:9000"
  access_key: "minioadmin"
  secret_key: "minioadmin"
  bucket_name: "my-bucket"

# 中间件配置
middleware:
  cors:
    enabled: true
    allowed_origins: ["*"]
  rate_limit:
    enabled: true
    rate: 100
    burst: 200
  logging:
    enabled: true
    level: info

# 功能特性配置
swagger:
  enabled: true
  ui_path: /swagger/
  title: My Gateway API
  
monitoring:
  enabled: true
  prometheus:
    enabled: true
    path: /metrics

health:
  enabled: true
  path: /health
```

**2. 创建 `main.go`：**

```go
package main

import gateway "github.com/kamalyes/go-rpc-gateway"

func main() {
    // 使用链式构建器创建网关
    gw, err := gateway.NewGateway().
        WithConfigPath("config.yaml").
        WithHotReload(nil).  // 启用配置热重载
        BuildAndStart()
    
    if err != nil {
        panic(err)
    }
    
    // 等待关闭信号
    gw.WaitForShutdown()
}
```

**特点：**

- ✅ 配置外部化，方便管理
- ✅ 支持数据库、Redis、MinIO 等企业级组件
- ✅ 支持多环境配置（开发、测试、生产）
- ✅ 支持配置热重载
- ✅ 链式构建器优雅API

**高级配置选项：**

```go
package main

import (
    gateway "github.com/kamalyes/go-rpc-gateway"
    goconfig "github.com/kamalyes/go-config"
)

func main() {
    // 更多配置选项
    gw, err := gateway.NewGateway().
        WithConfigPath("config.yaml").
        WithEnvironment(goconfig.EnvProduction).
        WithPrefix("gateway").     // 配置文件前缀
        WithHotReload(&goconfig.HotReloadConfig{
            Enabled:  true,
            Interval: 5 * time.Second,
            Debounce: 1 * time.Second,
        }).
        BuildAndStart()
    
    if err != nil {
        panic(err)
    }
    
    gw.WaitForShutdown()
}
```

---

### 方式三：功能特性入口 (推荐给复杂项目)

**完整的功能特性管理和路由注册：**

```go
package main

import (
    "net/http"
    
    gateway "github.com/kamalyes/go-rpc-gateway"
    "github.com/kamalyes/go-rpc-gateway/global"
    "github.com/kamalyes/go-rpc-gateway/server"
    "google.golang.org/grpc"
)

func main() {
    // 1. 创建网关 (构建但不启动)
    gw, err := gateway.NewGateway().
        WithConfigPath("config.yaml").
        Build()  // 只构建，不启动
    
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
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"message":"Hello World","version":"v2.1.0"}`))
    })
    
    // 4. 批量注册路由
    gw.RegisterHTTPRoutes(map[string]http.HandlerFunc{
        "/api/status":   statusHandler,
        "/api/info":     infoHandler,
        "/api/users":    usersHandler,
        "/api/products": productsHandler,
    })
    
    // 5. 启用功能特性
    gw.EnableFeature(server.FeaturePProf)      // 性能分析
    gw.EnableFeature(server.FeatureMonitoring) // 监控指标
    gw.EnableFeature(server.FeatureTracing)    // 链路追踪
    gw.EnableFeature(server.FeatureSwagger)    // API 文档
    gw.EnableFeature(server.FeatureHealth)     // 健康检查
    
    // 6. 启动服务
    if err := gw.Start(); err != nil {
        panic(err)
    }
    
    // 7. 等待关闭信号
    gw.WaitForShutdown()
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
    // 使用全局组件
    status := map[string]interface{}{
        "status":    "healthy",
        "timestamp": time.Now().Unix(),
    }
    
    // 检查数据库连接
    if global.DB != nil {
        sqlDB, err := global.DB.DB()
        if err == nil {
            if err := sqlDB.Ping(); err == nil {
                status["database"] = "connected"
            } else {
                status["database"] = "disconnected"
            }
        }
    }
    
    // 检查Redis连接
    if global.REDIS != nil {
        if err := global.REDIS.Ping(r.Context()).Err(); err == nil {
            status["redis"] = "connected"
        } else {
            status["redis"] = "disconnected"
        }
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(status)
}

func infoHandler(w http.ResponseWriter, r *http.Request) {
    info := map[string]interface{}{
        "service": "my-service",
        "version": "v2.1.0",
        "environment": global.GATEWAY.Environment,
        "features": []string{
            "swagger", "monitoring", "tracing", "health", "pprof",
        },
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(info)
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
    // 使用全局数据库连接
    if global.DB == nil {
        http.Error(w, "Database not available", http.StatusServiceUnavailable)
        return
    }
    
    var users []User
    if err := global.DB.Find(&users).Error; err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(users)
}
```

**特点：**

- ✅ 完整的 gRPC + HTTP 服务
- ✅ 使用全局组件 (DB, Redis, MinIO)
- ✅ 支持性能分析和监控
- ✅ 结构化的代码组织
- ✅ 功能特性动态管理

---

### 方式四：企业级开发 (推荐给大型项目)

**完整的企业级项目结构：**

```go
package main

import (
    "context"
    "net/http"
    
    gateway "github.com/kamalyes/go-rpc-gateway"
    "github.com/kamalyes/go-rpc-gateway/global"
    "github.com/kamalyes/go-rpc-gateway/server"
    
    "your-project/internal/handler"
    "your-project/internal/service"
    "your-project/internal/model"
    pb "your-project/proto"
    "google.golang.org/grpc"
)

func main() {
    // 1. 创建网关
    gw, err := gateway.NewGateway().
        WithConfigPath("config/gateway.yaml").
        WithEnvironment(gateway.EnvProduction).
        WithHotReload(nil).
        Build()
    
    if err != nil {
        global.LOGGER.Fatal("Failed to create gateway: %v", err)
    }
    
    // 2. 数据库迁移
    if err := migrateDatabase(); err != nil {
        global.LOGGER.Fatal("Database migration failed: %v", err)
    }
    
    // 3. 初始化服务
    userService := service.NewUserService(global.DB, global.REDIS)
    productService := service.NewProductService(global.DB)
    
    // 4. 注册 gRPC 服务
    gw.RegisterService(func(s *grpc.Server) {
        pb.RegisterUserServiceServer(s, userService)
        pb.RegisterProductServiceServer(s, productService)
        
        // 注册健康检查服务
        grpc_health_v1.RegisterHealthServer(s, health.NewServer())
    })
    
    // 5. 注册 HTTP 网关处理器
    err = gw.RegisterHTTPHandler(context.Background(), 
        pb.RegisterUserServiceHandlerFromEndpoint)
    if err != nil {
        global.LOGGER.Fatal("Failed to register user service handler: %v", err)
    }
    
    err = gw.RegisterHTTPHandler(context.Background(), 
        pb.RegisterProductServiceHandlerFromEndpoint)
    if err != nil {
        global.LOGGER.Fatal("Failed to register product service handler: %v", err)
    }
    
    // 6. 创建HTTP处理器
    apiHandler := handler.NewAPIHandler(userService, productService)
    adminHandler := handler.NewAdminHandler(userService)
    
    // 7. 注册业务API路由
    gw.RegisterHTTPRoutes(map[string]http.HandlerFunc{
        // 业务API
        "/api/v1/users":           apiHandler.GetUsers,
        "/api/v1/users/create":    apiHandler.CreateUser,
        "/api/v1/products":        apiHandler.GetProducts,
        "/api/v1/products/create": apiHandler.CreateProduct,
        
        // 管理API
        "/admin/users":    adminHandler.ManageUsers,
        "/admin/stats":    adminHandler.GetStatistics,
        
        // 系统API
        "/api/version":    versionHandler,
        "/api/config":     configHandler,
    })
    
    // 8. 启用所有功能特性
    enableAllFeatures(gw)
    
    // 9. 启动服务
    if err := gw.Start(); err != nil {
        global.LOGGER.Fatal("Failed to start gateway: %v", err)
    }
    
    // 10. 等待关闭信号
    global.LOGGER.Info("Gateway started successfully, waiting for shutdown signal...")
    gw.WaitForShutdown()
}

func migrateDatabase() error {
    if global.DB == nil {
        return nil // 数据库未配置
    }
    
    // 自动迁移数据库表
    return global.DB.AutoMigrate(
        &model.User{},
        &model.Product{},
        &model.Order{},
        // ... 其他模型
    )
}

func enableAllFeatures(gw *gateway.Gateway) {
    features := []server.FeatureType{
        server.FeatureSwagger,
        server.FeatureMonitoring,
        server.FeatureHealth,
        server.FeaturePProf,
        server.FeatureTracing,
    }
    
    for _, feature := range features {
        if err := gw.EnableFeature(feature); err != nil {
            global.LOGGER.Warn("Failed to enable feature %s: %v", feature, err)
        } else {
            global.LOGGER.Info("Feature %s enabled successfully", feature)
        }
    }
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
    version := map[string]interface{}{
        "version":     "v2.1.0",
        "build_time":  buildTime,
        "git_commit":  gitCommit,
        "environment": global.GATEWAY.Environment,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(version)
}

func configHandler(w http.ResponseWriter, r *http.Request) {
    // 返回非敏感配置信息
    config := map[string]interface{}{
        "name":        global.GATEWAY.Name,
        "environment": global.GATEWAY.Environment,
        "debug":       global.GATEWAY.Debug,
        "features":    getEnabledFeatures(),
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(config)
}
```

**特点：**

- ✅ 完整的企业级架构
- ✅ 分层设计 (Handler -> Service -> Repository)
- ✅ 数据库迁移和管理
- ✅ gRPC + HTTP 网关 + 业务API
- ✅ 完整的错误处理和日志记录
- ✅ 生产级配置管理

---

## 🚀 核心 API 说明

### 🔧 网关创建与构建

#### NewGateway() - 创建构建器

```go
// 创建新的网关构建器
builder := gateway.NewGateway()
```

#### 配置方法 (链式调用)

```go
// 设置配置文件路径
builder.WithConfigPath("config.yaml")

// 设置配置搜索路径 (自动发现)
builder.WithSearchPath("./config")

// 设置环境
builder.WithEnvironment(goconfig.EnvProduction)

// 设置配置文件前缀
builder.WithPrefix("gateway")

// 设置文件匹配模式
builder.WithPattern("gateway-*.yaml")

// 启用配置热重载
builder.WithHotReload(nil)  // 使用默认配置

// 自定义热重载配置
builder.WithHotReload(&goconfig.HotReloadConfig{
    Enabled:  true,
    Interval: 5 * time.Second,
    Debounce: 1 * time.Second,
})

// 设置上下文选项
builder.WithContext(&goconfig.ContextKeyOptions{
    ConfigKey: "config",
    EnvKey:    "environment",
})

// 静默模式 (不显示启动banner)
builder.Silent()
```

#### 构建方法

```go
// 方式1: 构建但不启动
gw, err := builder.Build()
if err != nil {
    // 处理错误
}
// 手动启动
gw.Start()

// 方式2: 构建并立即启动
gw, err := builder.BuildAndStart()

// 方式3: 构建并启动 (失败时panic)
gw := builder.MustBuildAndStart()
```

### 📝 服务注册

#### gRPC 服务注册

```go
// 注册单个 gRPC 服务
gw.RegisterService(func(s *grpc.Server) {
    pb.RegisterUserServiceServer(s, &userService{})
})

// 注册多个 gRPC 服务
gw.RegisterService(func(s *grpc.Server) {
    pb.RegisterUserServiceServer(s, &userService{})
    pb.RegisterProductServiceServer(s, &productService{})
    pb.RegisterOrderServiceServer(s, &orderService{})
})
```

#### HTTP 路由注册

```go
// 注册单个 HTTP 路由
gw.RegisterHTTPRoute("/api/hello", helloHandler)

// 注册多个 HTTP 路由
gw.RegisterHTTPRoutes(map[string]http.HandlerFunc{
    "/api/users":    usersHandler,
    "/api/products": productsHandler,
    "/api/orders":   ordersHandler,
})

// 注册 HTTP 处理器
gw.RegisterHandler("/custom", customHandler)

// 注册 HTTP 网关处理器 (gRPC -> HTTP 转换)
gw.RegisterHTTPHandler(ctx, pb.RegisterUserServiceHandlerFromEndpoint)
```

### 🎛️ 功能特性管理

#### 启用功能特性

```go
// 启用单个功能
gw.EnableFeature(server.FeaturePProf)

// 使用自定义配置启用功能
swaggerConfig := &SwaggerConfig{
    Title:       "My API",
    Description: "My API Description",
    Version:     "v1.0.0",
    UIPath:      "/docs/",
}
gw.EnableFeatureWithConfig(server.FeatureSwagger, swaggerConfig)
```

#### 功能特性类型

```go
const (
    FeatureSwagger    FeatureType = "swagger"    // API 文档
    FeatureMonitoring FeatureType = "monitoring" // 监控指标
    FeatureHealth     FeatureType = "health"     // 健康检查
    FeaturePProf      FeatureType = "pprof"      // 性能分析
    FeatureTracing    FeatureType = "tracing"    // 链路追踪
)
```

#### 检查功能状态

```go
// 检查功能是否启用
if gw.IsFeatureEnabled(server.FeatureSwagger) {
    fmt.Println("Swagger is enabled")
}

// 便捷方法
if gw.IsSwaggerEnabled() {
    fmt.Println("Swagger is enabled")
}
if gw.IsMonitoringEnabled() {
    fmt.Println("Monitoring is enabled")
}
```

### 🔄 生命周期管理

#### 启动方法

```go
// 启动服务 (带 banner)
gw.Start()

// 静默启动 (不显示 banner)
gw.StartSilent()

// 带 banner 启动
gw.StartWithBanner()
```

#### 停止和关闭

```go
// 停止服务
gw.Stop()

// 优雅关闭
gw.Shutdown()

// 重启服务
gw.Restart()

// 等待关闭信号
gw.WaitForShutdown()
```

#### 状态检查

```go
// 检查运行状态
if gw.IsRunning() {
    fmt.Println("Gateway is running")
}

// 等待服务运行
gw.Wait()
```

### 💾 全局资源访问

#### 使用全局组件

```go
import "github.com/kamalyes/go-rpc-gateway/global"

// 使用数据库
if global.DB != nil {
    var users []User
    global.DB.Find(&users)
    
    // 创建记录
    user := &User{Name: "Alice", Email: "alice@example.com"}
    global.DB.Create(user)
}

// 使用 Redis
if global.REDIS != nil {
    // 设置值
    global.REDIS.Set(ctx, "key", "value", 0)
    
    // 获取值
    val := global.REDIS.Get(ctx, "key").Val()
    
    // 检查连接
    if err := global.REDIS.Ping(ctx).Err(); err != nil {
        // 处理连接错误
    }
}

// 使用 MinIO
if global.MinIO != nil {
    // 上传对象
    _, err := global.MinIO.PutObject(ctx, bucket, objectName, reader, size, opts)
    
    // 下载对象
    object, err := global.MinIO.GetObject(ctx, bucket, objectName, opts)
}

// 使用日志
if global.LOGGER != nil {
    global.LOGGER.Info("Information message")
    global.LOGGER.InfoKV("Structured message", "key", "value", "count", 123)
    global.LOGGER.Error("Error message: %v", err)
    global.LOGGER.WithError(err).ErrorMsg("Error occurred")
}

// 使用雪花ID生成器
if global.Node != nil {
    id := global.Node.Generate()
    fmt.Printf("Generated ID: %d\n", id.Int64())
}
```

#### 连接池管理

```go
// 获取连接池管理器
poolManager := gw.GetPoolManager()

// 获取特定连接
db := gw.GetDB()
redis := gw.GetRedis()
minio := gw.GetMinIO()
snowflake := gw.GetSnowflake()

// 健康检查所有连接
healthStatus := gw.HealthCheck()
for service, status := range healthStatus {
    fmt.Printf("%s: %v\n", service, status)
}
```

---

## 🎯 实际项目结构

### 📁 推荐的项目结构

```
your-project/
├── cmd/                     # 应用程序入口
│   └── main.go             # 主入口文件
├── config/                  # 配置文件
│   ├── gateway-dev.yaml    # 开发环境配置
│   ├── gateway-test.yaml   # 测试环境配置
│   └── gateway-prod.yaml   # 生产环境配置
├── internal/               # 内部包
│   ├── handler/            # HTTP 处理器
│   │   ├── api_handler.go
│   │   ├── admin_handler.go
│   │   └── health_handler.go
│   ├── service/            # 业务逻辑服务
│   │   ├── user_service.go
│   │   ├── product_service.go
│   │   └── order_service.go
│   ├── repository/         # 数据访问层
│   │   ├── user_repo.go
│   │   ├── product_repo.go
│   │   └── order_repo.go
│   ├── model/              # 数据模型
│   │   ├── user.go
│   │   ├── product.go
│   │   └── order.go
│   └── middleware/         # 自定义中间件
│       ├── auth.go
│       └── validation.go
├── proto/                  # Protocol Buffers 定义
│   ├── user.proto
│   ├── product.proto
│   └── common.proto
├── api/                    # 生成的 API 代码
│   └── v1/
│       ├── user.pb.go
│       ├── user_grpc.pb.go
│       └── user.pb.gw.go
├── docs/                   # 项目文档
│   ├── api.md
│   └── deployment.md
├── scripts/                # 构建和部署脚本
│   ├── build.sh
│   ├── deploy.sh
│   └── migrate.sh
├── docker/                 # Docker 相关文件
│   ├── Dockerfile
│   └── docker-compose.yml
├── go.mod
├── go.sum
└── README.md
```

### 🏗️ 完整项目示例

#### cmd/main.go - 应用入口

```go
package main

import (
    "context"
    "log"
    
    gateway "github.com/kamalyes/go-rpc-gateway"
    "github.com/kamalyes/go-rpc-gateway/global"
    "github.com/kamalyes/go-rpc-gateway/server"
    
    "your-project/internal/handler"
    "your-project/internal/service"
    "your-project/internal/repository"
    "your-project/internal/model"
    pb "your-project/api/v1"
    "google.golang.org/grpc"
)

func main() {
    // 创建网关
    gw, err := gateway.NewGateway().
        WithSearchPath("./config").
        WithPrefix("gateway").
        WithEnvironment(gateway.GetEnvironment()).
        WithHotReload(nil).
        Build()
    
    if err != nil {
        log.Fatalf("Failed to create gateway: %v", err)
    }
    
    // 数据库迁移
    if err := migrateDatabase(); err != nil {
        global.LOGGER.Fatal("Database migration failed: %v", err)
    }
    
    // 初始化仓库层
    userRepo := repository.NewUserRepository(global.DB)
    productRepo := repository.NewProductRepository(global.DB)
    
    // 初始化服务层
    userService := service.NewUserService(userRepo, global.REDIS)
    productService := service.NewProductService(productRepo)
    
    // 初始化处理器层
    apiHandler := handler.NewAPIHandler(userService, productService)
    
    // 注册 gRPC 服务
    gw.RegisterService(func(s *grpc.Server) {
        pb.RegisterUserServiceServer(s, userService)
        pb.RegisterProductServiceServer(s, productService)
    })
    
    // 注册 HTTP 网关处理器
    gw.RegisterHTTPHandler(context.Background(), 
        pb.RegisterUserServiceHandlerFromEndpoint)
    gw.RegisterHTTPHandler(context.Background(), 
        pb.RegisterProductServiceHandlerFromEndpoint)
    
    // 注册业务API路由
    gw.RegisterHTTPRoutes(map[string]http.HandlerFunc{
        "/api/v1/users/list":   apiHandler.ListUsers,
        "/api/v1/users/create": apiHandler.CreateUser,
        "/api/v1/products":     apiHandler.ListProducts,
    })
    
    // 启用功能特性
    gw.EnableFeature(server.FeatureSwagger)
    gw.EnableFeature(server.FeatureMonitoring)
    gw.EnableFeature(server.FeatureHealth)
    
    // 启动服务
    if err := gw.Start(); err != nil {
        log.Fatalf("Failed to start gateway: %v", err)
    }
    
    // 等待关闭信号
    gw.WaitForShutdown()
}

func migrateDatabase() error {
    if global.DB == nil {
        return nil
    }
    
    return global.DB.AutoMigrate(
        &model.User{},
        &model.Product{},
        &model.Order{},
    )
}
```

#### internal/service/user_service.go - 服务层

```go
package service

import (
    "context"
    "time"
    
    "github.com/redis/go-redis/v9"
    "your-project/internal/model"
    "your-project/internal/repository"
    pb "your-project/api/v1"
)

type UserService struct {
    pb.UnimplementedUserServiceServer
    userRepo repository.UserRepository
    redis    *redis.Client
}

func NewUserService(userRepo repository.UserRepository, redis *redis.Client) *UserService {
    return &UserService{
        userRepo: userRepo,
        redis:    redis,
    }
}

func (s *UserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
    // 先从缓存获取
    if s.redis != nil {
        cachedUser, err := s.getUserFromCache(ctx, req.Id)
        if err == nil && cachedUser != nil {
            return &pb.GetUserResponse{User: cachedUser}, nil
        }
    }
    
    // 从数据库获取
    user, err := s.userRepo.GetByID(ctx, req.Id)
    if err != nil {
        return nil, err
    }
    
    pbUser := &pb.User{
        Id:    user.ID,
        Name:  user.Name,
        Email: user.Email,
    }
    
    // 写入缓存
    if s.redis != nil {
        s.setUserToCache(ctx, pbUser)
    }
    
    return &pb.GetUserResponse{User: pbUser}, nil
}

func (s *UserService) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
    user := &model.User{
        Name:  req.Name,
        Email: req.Email,
    }
    
    if err := s.userRepo.Create(ctx, user); err != nil {
        return nil, err
    }
    
    pbUser := &pb.User{
        Id:    user.ID,
        Name:  user.Name,
        Email: user.Email,
    }
    
    return &pb.CreateUserResponse{User: pbUser}, nil
}

func (s *UserService) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
    users, total, err := s.userRepo.List(ctx, int(req.Page), int(req.PageSize))
    if err != nil {
        return nil, err
    }
    
    pbUsers := make([]*pb.User, len(users))
    for i, user := range users {
        pbUsers[i] = &pb.User{
            Id:    user.ID,
            Name:  user.Name,
            Email: user.Email,
        }
    }
    
    return &pb.ListUsersResponse{
        Users: pbUsers,
        Total: int32(total),
    }, nil
}

func (s *UserService) getUserFromCache(ctx context.Context, id int32) (*pb.User, error) {
    // 实现缓存获取逻辑
    key := fmt.Sprintf("user:%d", id)
    result := s.redis.Get(ctx, key)
    if result.Err() != nil {
        return nil, result.Err()
    }
    
    var user pb.User
    if err := json.Unmarshal([]byte(result.Val()), &user); err != nil {
        return nil, err
    }
    
    return &user, nil
}

func (s *UserService) setUserToCache(ctx context.Context, user *pb.User) {
    key := fmt.Sprintf("user:%d", user.Id)
    data, _ := json.Marshal(user)
    s.redis.Set(ctx, key, data, time.Hour)
}
```

#### internal/handler/api_handler.go - HTTP处理器

```go
package handler

import (
    "encoding/json"
    "net/http"
    "strconv"
    
    "your-project/internal/service"
    pb "your-project/api/v1"
)

type APIHandler struct {
    userService    *service.UserService
    productService *service.ProductService
}

func NewAPIHandler(userService *service.UserService, productService *service.ProductService) *APIHandler {
    return &APIHandler{
        userService:    userService,
        productService: productService,
    }
}

func (h *APIHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
    page := 1
    pageSize := 10
    
    if p := r.URL.Query().Get("page"); p != "" {
        if parsed, err := strconv.Atoi(p); err == nil {
            page = parsed
        }
    }
    
    if ps := r.URL.Query().Get("page_size"); ps != "" {
        if parsed, err := strconv.Atoi(ps); err == nil {
            pageSize = parsed
        }
    }
    
    req := &pb.ListUsersRequest{
        Page:     int32(page),
        PageSize: int32(pageSize),
    }
    
    resp, err := h.userService.ListUsers(r.Context(), req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "code": 0,
        "data": resp,
        "msg":  "success",
    })
}

func (h *APIHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    var req pb.CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    resp, err := h.userService.CreateUser(r.Context(), &req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "code": 0,
        "data": resp.User,
        "msg":  "User created successfully",
    })
}
```

---

## 💻 命令行工具和脚本

### 🛠️ 开发脚本

#### scripts/build.sh - 构建脚本

```bash
#!/bin/bash

# Go RPC Gateway 构建脚本

set -e

# 项目信息
PROJECT_NAME="your-project"
VERSION=$(git describe --tags --always --dirty)
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse HEAD)

# 构建参数
LDFLAGS="-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}"

echo "Building ${PROJECT_NAME}..."
echo "Version: ${VERSION}"
echo "Build Time: ${BUILD_TIME}"
echo "Git Commit: ${GIT_COMMIT}"

# 清理旧文件
rm -rf bin/

# 构建应用
go build -ldflags "${LDFLAGS}" -o bin/${PROJECT_NAME} cmd/main.go

echo "Build completed successfully!"
echo "Binary: bin/${PROJECT_NAME}"
```

#### scripts/dev.sh - 开发脚本

```bash
#!/bin/bash

# 开发模式启动脚本

export ENVIRONMENT=development
export CONFIG_PATH=./config/gateway-dev.yaml

echo "Starting in development mode..."
echo "Environment: ${ENVIRONMENT}"
echo "Config: ${CONFIG_PATH}"

# 使用 air 进行热重载 (需要安装 github.com/cosmtrek/air)
if command -v air &> /dev/null; then
    air
else
    echo "Air not found, starting with go run..."
    go run cmd/main.go
fi
```

#### scripts/generate.sh - 代码生成脚本

```bash
#!/bin/bash

# Protocol Buffers 代码生成脚本

set -e

echo "Generating Protocol Buffers code..."

# 检查 protoc 是否安装
if ! command -v protoc &> /dev/null; then
    echo "protoc is required but not installed."
    exit 1
fi

# 生成 Go 代码
protoc \
    --proto_path=proto \
    --go_out=api/v1 \
    --go_opt=paths=source_relative \
    --go-grpc_out=api/v1 \
    --go-grpc_opt=paths=source_relative \
    --grpc-gateway_out=api/v1 \
    --grpc-gateway_opt=paths=source_relative \
    proto/*.proto

echo "Code generation completed!"
```

### 📦 部署脚本

#### scripts/deploy.sh - 部署脚本

```bash
#!/bin/bash

# 生产部署脚本

set -e

ENVIRONMENT=${1:-production}
VERSION=${2:-latest}

echo "Deploying to ${ENVIRONMENT}..."

# 构建 Docker 镜像
docker build -t your-registry/${PROJECT_NAME}:${VERSION} .

# 推送到镜像仓库
docker push your-registry/${PROJECT_NAME}:${VERSION}

# 使用 kubectl 部署到 Kubernetes
kubectl set image deployment/${PROJECT_NAME} \
    ${PROJECT_NAME}=your-registry/${PROJECT_NAME}:${VERSION} \
    -n ${ENVIRONMENT}

# 等待部署完成
kubectl rollout status deployment/${PROJECT_NAME} -n ${ENVIRONMENT}

echo "Deployment completed!"
```

---

## ✅ 测试服务

### 🧪 启动后测试

```bash
# 检查服务状态
curl http://localhost:8080/health

# 查看指标监控
curl http://localhost:8080/metrics

# 测试 gRPC 服务 (使用 grpcurl)
grpcurl -plaintext localhost:9090 list
grpcurl -plaintext localhost:9090 your.package.UserService/GetUser

# 测试 HTTP API
curl -X GET "http://localhost:8080/api/v1/users?page=1&page_size=10"

# 创建用户
curl -X POST "http://localhost:8080/api/v1/users/create" \
     -H "Content-Type: application/json" \
     -d '{"name":"Alice","email":"alice@example.com"}'

# 查看 API 文档 (如果启用了 Swagger)
curl http://localhost:8080/swagger/

# 性能分析 (如果启用了 PProf)
curl http://localhost:8080/debug/pprof/
go tool pprof http://localhost:8080/debug/pprof/profile
```

### 🔍 监控和调试

#### Prometheus 指标查询

```bash
# 查看 HTTP 请求总数
curl "http://localhost:8080/metrics" | grep http_requests_total

# 查看请求处理时间
curl "http://localhost:8080/metrics" | grep http_request_duration

# 查看 gRPC 服务指标
curl "http://localhost:8080/metrics" | grep grpc_server

# 查看数据库连接池指标
curl "http://localhost:8080/metrics" | grep database_connections
```

#### 日志查询示例

```bash
# 查看应用日志 (如果使用 JSON 格式)
tail -f app.log | jq '.'

# 过滤错误日志
tail -f app.log | jq 'select(.level=="error")'

# 查看特定请求的日志
tail -f app.log | jq 'select(.request_id=="your-request-id")'
```

---

## 🔗 相关资源

### 📚 文档链接

- [🏗️ 架构设计](./docs/ARCHITECTURE.md) - 系统架构详细说明
- [⚙️ 配置分析](./docs/CONFIG_ANALYSIS.md) - 配置文件详细解释
- [🔌 中间件指南](./docs/MIDDLEWARE_GUIDE.md) - 中间件开发指南
- [📦 部署指南](./docs/DEPLOYMENT.md) - 生产环境部署指南
- [🔧 重构计划](./REFACTORING_PLAN.md) - 项目重构历程

### 🎯 示例项目

- [基础 API 服务](./examples/basic-api/) - 简单的 RESTful API 示例
- [gRPC + HTTP 混合](./examples/grpc-http/) - gRPC 和 HTTP 的混合服务
- [微服务网关](./examples/microservice-gateway/) - 完整的微服务网关示例
- [企业级应用](./examples/enterprise-app/) - 包含完整基础设施的企业应用

### 🔗 核心依赖

- [kamalyes/go-config](https://github.com/kamalyes/go-config) - 统一配置管理库
- [kamalyes/go-logger](https://github.com/kamalyes/go-logger) - 高性能日志库
- [kamalyes/go-toolbox](https://github.com/kamalyes/go-toolbox) - 工具函数集
- [kamalyes/go-cachex](https://github.com/kamalyes/go-cachex) - 多级缓存库
- [kamalyes/go-wsc](https://github.com/kamalyes/go-wsc) - WebSocket 客户端

---

## 🔄 PBMO - Protocol Buffer 模型转换

### 概述

Go RPC Gateway 内置了强大的 **PBMO (Protocol Buffer Model Object)** 转换系统，提供 Protocol Buffer 和 GORM Model 之间的高性能双向转换。

**核心优势：**


- 🚄 **极致性能**: 单次转换仅需 3μs，比标准反射快 17-22倍
- 🔄 **双向转换**: 完全支持 PB ↔ Model 转换
- 🛡️ **安全可靠**: 自动处理 nil 指针和类型转换
- ✅ **智能校验**: 内置字段校验和自定义规则
- 📊 **可观测性**: 详细日志和性能监控

### 30秒快速上手

#### 1. 基础转换

```go
import "github.com/kamalyes/go-rpc-gateway/pbmo"

// 定义 GORM 模型
type User struct {
    ID       uint   `gorm:"primarykey"`
    Name     string `gorm:"size:100"`
    Email    string `gorm:"uniqueIndex"`
    Age      int32
    IsActive bool
}

func quickStart() {
    // 创建转换器（一次创建，重复使用）
    converter := pbmo.NewBidiConverter(&pb.User{}, &User{})
    
    // PB → Model 转换
    pbUser := &pb.User{
        Name:     "张三",
        Email:    "zhangsan@example.com",
        Age:      25,
        IsActive: true,
    }
    
    var user User
    if err := converter.ConvertPBToModel(pbUser, &user); err != nil {
        panic(err)
    }
    
    // Model → PB 转换
    user.ID = 123
    var pbResult pb.User
    if err := converter.ConvertModelToPB(&user, &pbResult); err != nil {
        panic(err)
    }
    
    fmt.Printf("转换成功: %+v\n", pbResult)
}
```

#### 2. 生产级转换（推荐）

```go
// 带日志和性能监控的增强转换器
converter := pbmo.NewEnhancedBidiConverter(
    &pb.User{}, &User{}, logger,
)

// 转换时自动记录日志和性能指标
if err := converter.ConvertPBToModelWithLog(pbUser, &user); err != nil {
    return err // 已自动转换为 gRPC status error
}

// 查看性能统计
metrics := converter.GetMetrics()
fmt.Printf("转换成功率: %.2f%%\n", 
    float64(metrics.SuccessfulConversions) / float64(metrics.TotalConversions) * 100)
```

#### 3. 安全转换（处理复杂嵌套）

```go
// 安全处理 nil 指针和深度嵌套
safeConverter := pbmo.NewSafeConverter(&pb.UserProfile{}, &UserProfile{})

// 链式安全访问（类似 JavaScript 的 ?. 操作符）
value := safeConverter.SafeFieldAccess(obj, "Profile", "Address", "City")
if value.IsValid() {
    city := value.String("默认城市")
}
```

### gRPC 服务集成

在实际的 gRPC 服务中使用 PBMO：

```go
type UserService struct {
    pb.UnimplementedUserServiceServer
    converter *pbmo.EnhancedBidiConverter
    logger    logger.ILogger
}

func NewUserService(logger logger.ILogger) *UserService {
    converter := pbmo.NewEnhancedBidiConverter(&pb.User{}, &User{}, logger)
    
    // 注册校验规则
    converter.RegisterValidationRules("User",
        pbmo.FieldRule{
            Name:     "Name",
            Required: true,
            MinLen:   2,
            MaxLen:   50,
        },
        pbmo.FieldRule{
            Name:    "Email", 
            Pattern: `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
        },
    )
    
    return &UserService{
        converter: converter,
        logger:    logger,
    }
}

func (s *UserService) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
    var user User
    
    // 转换并校验，一步完成
    if err := s.converter.ConvertAndValidatePBToModel(req.User, &user); err != nil {
        return nil, err
    }
    
    // 保存到数据库
    if err := global.DB.Create(&user).Error; err != nil {
        return nil, s.converter.HandleError(err, "CreateUser")
    }
    
    // 转换响应
    var pbUser pb.User
    if err := s.converter.ConvertModelToPBWithLog(&user, &pbUser); err != nil {
        return nil, err
    }
    
    return &pbUser, nil
}
```

### 支持的类型转换

| PB 类型 | GORM 类型 | 说明 |
|---------|----------|------|
| `string` | `string` | 直接映射 |
| `int32/int64` | `int/uint` | 自动转换 |
| `bool` | `bool` | 直接映射 |
| `double` | `float64` | 精度保持 |
| `google.protobuf.Timestamp` | `time.Time` | 时间转换 ⭐ |
| `repeated T` | `[]T` | 切片转换 |
| 嵌套消息 | 嵌套结构体 | 递归转换 |

### 性能对比

| 转换方法 | 性能 | 适用场景 |
|---------|------|---------|
| **PBMO BidiConverter** | 130ns/op | 高频转换，性能要求极高 |
| **PBMO EnhancedConverter** | 200ns/op | 生产环境，需要监控和日志 |
| **PBMO SafeConverter** | 150ns/op | 复杂嵌套，安全要求高 |
| 手动转换 | 50-100ns/op | 简单场景，无复杂逻辑 |
| 标准反射 | 2260ns/op | 原始方法（不推荐） |

### 最佳实践

#### ✅ 推荐做法

```go
// 1. 重复使用转换器实例
converter := pbmo.NewEnhancedBidiConverter(&pb.User{}, &User{}, logger)

// 2. 使用服务集成进行生产部署
service := pbmo.NewServiceIntegration(&pb.User{}, &User{}, logger)
service.RegisterValidationRules("User", rules...)

// 3. 为复杂嵌套使用安全转换器
safeConverter := pbmo.NewSafeConverter(&pb.ComplexMessage{}, &ComplexModel{})
```

#### ❌ 避免做法

```go
// ❌ 不要频繁创建转换器
for _, pb := range pbList {
    converter := pbmo.NewBidiConverter(&pb.User{}, &User{})  // 浪费！
}

// ❌ 不要忽略错误处理
converter.ConvertPBToModel(pb, &model)  // 没有检查 err
```

### 详细文档

- 📖 [PBMO 完整指南](./pbmo/README.md)
- 🚀 [快速开始](./pbmo/QUICK_START.md)  
- 📚 [使用示例大全](./pbmo/USAGE_EXAMPLES.md)
- 🛡️ [安全转换器指南](./pbmo/SAFE_CONVERTER_GUIDE.md)
- 📊 [性能优化说明](./pbmo/PERFORMANCE_OPTIMIZATION.md)

---

## ❓ 常见问题

### Q: 如何自定义端口配置?

A: 在配置文件中设置端口:

```yaml
http_server:
  host: 0.0.0.0
  port: 3000

grpc:
  server:
    host: 0.0.0.0
    port: 50051
```

### Q: 如何启用数据库连接?

A: 在配置文件中添加数据库配置:

```yaml
mysql:
  enabled: true
  host: "localhost"
  port: 3306
  username: "root"
  password: "password"
  dbname: "mydb"
  max_idle_conns: 10
  max_open_conns: 100
```

数据库连接会自动创建并可通过 `global.DB` 访问。

### Q: 如何添加自定义中间件?

A: 创建中间件函数并注册:

```go
func CustomMiddleware() middleware.MiddlewareFunc {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 前置处理
            global.LOGGER.Info("Request started: %s", r.URL.Path)
            
            // 调用下一个中间件或处理器
            next.ServeHTTP(w, r)
            
            // 后置处理
            global.LOGGER.Info("Request completed: %s", r.URL.Path)
        })
    }
}
```

### Q: 如何实现认证和授权?

A: 使用内置的认证中间件或自定义:

```yaml
middleware:
  auth:
    enabled: true
    jwt:
      secret: "your-jwt-secret"
      expire: 24h
```

或创建自定义认证:

```go
func AuthMiddleware() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        
        // 验证 token 逻辑
        if !validateToken(token) {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }
        
        // 继续处理请求
        next.ServeHTTP(w, r)
    }
}
```

### Q: 如何配置 HTTPS?

A: 在配置文件中启用 TLS:

```yaml
security:
  tls:
    enabled: true
    cert_file: "path/to/cert.pem"
    key_file: "path/to/key.pem"
```

### Q: 如何进行数据库迁移?

A: 在应用启动时执行迁移:

```go
func migrateDatabase() error {
    if global.DB == nil {
        return nil
    }
    
    // 自动迁移
    return global.DB.AutoMigrate(
        &model.User{},
        &model.Product{},
        // ... 其他模型
    )
}
```

### Q: 如何配置日志输出?

A: 在配置文件中设置日志配置:

```yaml
middleware:
  logging:
    enabled: true
    level: info      # debug, info, warn, error
    format: json     # json, text
    output: stdout   # stdout, stderr, file
    file_path: ./logs/app.log
```

### Q: 如何实现分布式追踪?

A: 启用追踪功能:

```yaml
monitoring:
  tracing:
    enabled: true
    jaeger:
      endpoint: "http://localhost:14268/api/traces"
    # 或使用 Zipkin
    zipkin:
      endpoint: "http://localhost:9411/api/v2/spans"
```

```go
// 在代码中启用
gw.EnableFeature(server.FeatureTracing)
```

### Q: 如何处理跨域问题?

A: 配置 CORS 中间件:

```yaml
middleware:
  cors:
    enabled: true
    allowed_origins: ["*"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allowed_headers: ["*"]
    expose_headers: ["Content-Length"]
    max_age: 86400
```

### Q: 如何实现限流?

A: 启用限流中间件:

```yaml
middleware:
  rate_limit:
    enabled: true
    rate: 100        # 每秒允许的请求数
    burst: 200       # 突发请求数
    window: 1s       # 时间窗口
```

---

## 🆘 获取帮助

- 📖 **详细文档**: 查看 `docs/` 目录下的详细文档
- 🔍 **示例代码**: 参考 `examples/` 目录下的示例项目
- 🐛 **问题反馈**: [GitHub Issues](https://github.com/kamalyes/go-rpc-gateway/issues)
- 💬 **讨论交流**: <501893067@qq.com>ons](https://github.com/kamalyes/go-rpc-gateway/discussions)
- 📫 **邮件支持**: 501893067@qq.com

---

**🎉 现在开始使用 Go RPC Gateway 构建你的微服务吧！** 🚀


从最简单的3行代码开始，逐步构建你的企业级微服务应用。框架的链式构建器设计让你可以从简单开始，随着项目复杂度的增加，逐步添加更多功能特性。