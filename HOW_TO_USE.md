# 🎯 如何将框架给别人使用 - 入口编写指南

## 📝 问题
**"我想将这个框架给别人用直接开发，应该怎么写入口？"**

## ✅ 答案：提供三种使用方式

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
- 健康检查: http://localhost:8080/health
- 指标监控: http://localhost:8080/metrics

---

### 方式二：配置文件入口 (推荐给生产环境)

**1. 创建配置文件 `config.yaml`：**

```yaml
# 基础配置
gateway:
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

**2. 创建 `main.go`：**

```go
package main

import gateway "github.com/kamalyes/go-rpc-gateway"

func main() {
    // 使用配置文件
    gw, err := gateway.NewWithConfigFile("config.yaml")
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
    "context"
    "net/http"
    
    "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
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
        // pb.RegisterUserServiceServer(s, &userService{})
        // pb.RegisterProductServiceServer(s, &productService{})
    })
    
    // 3. 注册 HTTP 路由
    gw.RegisterHTTPRoute("/api/hello", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"message":"Hello World"}`))
    })
    
    // 批量注册
    gw.RegisterHTTPRoutes(map[string]http.HandlerFunc{
        "/api/status": statusHandler,
        "/api/info":   infoHandler,
    })
    
    // 4. 注册 gRPC-Gateway 转换器
    gw.Server.RegisterHTTPHandler(context.Background(), func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error {
        // return pb.RegisterUserServiceHandlerFromEndpoint(ctx, mux, endpoint, opts)
        return nil
    })
    
    // 5. 启用性能分析 (可选)
    gw.EnablePProfWithToken("your-secret-token")
    
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

## 📦 提供给用户的文件结构

建议为用户提供以下文件：

```
your-framework/
├── template/              # 📁 模板目录
│   ├── quickstart.go      # 极简模板
│   ├── main.go            # 标准模板
│   ├── advanced.go        # 高级模板
│   ├── with-database.go   # 数据库集成模板
│   ├── config.yaml        # 配置文件模板
│   └── README.md          # 模板使用说明
│
├── examples/              # 📁 示例目录
│   ├── integration-demo/  # 完整集成演示
│   └── config-complete.yaml  # 完整配置示例
│
├── docs/                  # 📁 文档目录
│   ├── QUICK_START.md     # 快速开始
│   ├── CONFIG_ANALYSIS.md # 配置说明
│   └── MIDDLEWARE_GUIDE.md # 中间件文档
│
├── README.md              # 主说明文档
└── go.mod
```

## 🚀 用户使用流程

### 第一步：安装框架

```bash
go get github.com/kamalyes/go-rpc-gateway
```

### 第二步：选择模板

**新手用户：**
```bash
# 复制极简模板
cp template/quickstart.go main.go
go run main.go
```

**一般用户：**
```bash
# 复制标准模板
cp template/main.go main.go
cp template/config.yaml config.yaml
# 编辑 config.yaml
go run main.go
```

**高级用户：**
```bash
# 复制高级模板
cp template/advanced.go main.go
cp template/config.yaml config.yaml
# 根据需要修改
go run main.go
```

### 第三步：添加业务逻辑

在模板基础上添加自己的：
- gRPC 服务定义 (proto 文件)
- 业务逻辑实现
- HTTP 路由处理
- 数据库模型

### 第四步：配置和部署

编辑 `config.yaml` 配置数据库、Redis 等，然后部署：

```bash
# 编译
go build -o myapp main.go

# 运行
./myapp

# 或使用 Docker
docker build -t myapp .
docker run -p 8080:8080 -p 9090:9090 myapp
```

---

## 📚 核心 API 说明

### 创建网关

```go
// 方式1: 默认配置
gw, _ := gateway.New()

// 方式2: 使用配置文件
gw, _ := gateway.NewWithConfigFile("config.yaml")

// 方式3: 自定义配置
config := &gateway.Config{ /* ... */ }
gw, _ := gateway.New(config)
```

### 注册服务

```go
// 注册 gRPC 服务
gw.RegisterService(func(s *grpc.Server) {
    pb.RegisterYourServiceServer(s, &yourService{})
})

// 注册 HTTP 路由
gw.RegisterHTTPRoute("/api/path", handlerFunc)

// 批量注册 HTTP 路由
gw.RegisterHTTPRoutes(map[string]http.HandlerFunc{
    "/api/v1/users":    usersHandler,
    "/api/v1/products": productsHandler,
})

// 注册 gRPC-Gateway 处理器
gw.Server.RegisterHTTPHandler(ctx, handlerRegisterFunc)
```

### 启用功能

```go
// 启用 pprof 性能分析
gw.EnablePProf()

// 启用带认证的 pprof
gw.EnablePProfWithToken("secret-token")

// 开发环境 pprof
gw.EnablePProfForDevelopment()
```

### 启动和停止

```go
// 启动 (带 banner)
gw.Start()

// 静默启动
gw.StartSilent()

// 停止服务
gw.Stop()
```

### 使用全局组件

```go
import "github.com/kamalyes/go-core/pkg/global"

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
global.LOGGER.Info("message")
global.LOGGER.InfoKV("message", "key", "value")
```

---

## 🎯 给别人用的建议

### 1. 提供清晰的模板

已创建的模板文件在 `template/` 目录：
- ✅ `quickstart.go` - 最简单
- ✅ `main.go` - 标准模板
- ✅ `advanced.go` - 完整功能
- ✅ `with-database.go` - 数据库集成
- ✅ `config.yaml` - 配置模板

### 2. 提供完整文档

- ✅ `QUICK_START.md` - 快速开始指南
- ✅ `template/README.md` - 模板使用说明
- ✅ `README.md` - 完整文档

### 3. 提供示例代码

- ✅ `examples/integration-demo/` - 集成演示
- ✅ `examples/complete-integration/` - 完整示例

### 4. 提供一键启动脚本

创建 `start.sh`:
```bash
#!/bin/bash
echo "🚀 启动 Go RPC Gateway..."
go run main.go
```

创建 `Makefile`:
```makefile
.PHONY: run
run:
	go run main.go

.PHONY: build
build:
	go build -o gateway main.go

.PHONY: docker
docker:
	docker build -t go-rpc-gateway .
	docker run -p 8080:8080 -p 9090:9090 go-rpc-gateway
```

---

## ✅ 总结

**给别人使用这个框架，你需要：**

1. ✅ 提供简单的入口模板 → 已创建在 `template/` 目录
2. ✅ 提供配置文件模板 → `template/config.yaml`
3. ✅ 提供快速开始文档 → `QUICK_START.md`
4. ✅ 提供完整示例代码 → `examples/` 目录
5. ✅ 提供 API 文档 → 本文档

**用户只需三步：**
```bash
# 1. 安装
go get github.com/kamalyes/go-rpc-gateway

# 2. 复制模板
cp template/main.go main.go
cp template/config.yaml config.yaml

# 3. 运行
go run main.go
```

**就这么简单！** 🎉

---

## 📞 后续支持

用户使用过程中可能需要：
- 📖 查看 `docs/` 目录的详细文档
- 💡 参考 `examples/` 目录的示例
- ❓ 查看 FAQ 常见问题
- 🐛 提交 Issue 获取帮助

---

**现在你的框架已经准备好给别人使用了！** 🚀
