# engine-ads-link-service 迁移到 go-rpc-gateway 战略分析

## 📋 概览

本文档分析了 `commonpkgs`、`commonapis` 和 `engine-ads-link-service` 三个项目的架构，提出将 `engine-ads-link-service` 平滑迁移到 `go-rpc-gateway` 框架的详细方案。

---

## 1️⃣ 项目现状分析

### 1.1 engine-ads-link-service 项目结构

```
engine-ads-link-service/
├── admin-server/          # 后台管理服务
├── api/                   # HTTP API 定义
├── cmd/                   # 主程序入口
├── config/                # 配置文件
├── deployments/           # 部署脚本
├── docs/                  # 文档
├── i18n/                  # 国际化
├── migrations/            # 数据库迁移
├── pb/                    # Protocol Buffer 定义
├── persist/               # 数据持久化层
├── server/                # 核心服务实现
└── version/               # 版本管理
```

**关键依赖：**
- `commonapis` - 共享的 API 和 gRPC 定义
- `commonpkgs` - 共享的通用工具包
- `gorm.io/gorm` - ORM 框架
- `grpc-ecosystem/grpc-gateway/v2` - gRPC 网关
- `prometheus/client_golang` - 指标收集
- `opentelemetry/*` - 链路追踪

### 1.2 commonpkgs 项目结构

```
commonpkgs/
├── middleware/            # 中间件集合
│   ├── auth/            # 认证中间件
│   ├── logging/         # 日志中间件
│   └── ...
├── pkg/                 # 工具包
│   ├── errors/          # 错误处理
│   ├── response/        # 响应格式
│   ├── validators/      # 验证器
│   └── ...
└── utils/               # 工具函数
    ├── jwt/            # JWT 工具
    ├── encryption/     # 加密工具
    └── ...
```

**提供的功能：**
- ✅ 中间件组件
- ✅ 错误处理标准化
- ✅ 响应格式标准化
- ✅ 认证和授权工具
- ✅ 日志记录工具
- ✅ 数据验证器

### 1.3 commonapis 项目结构

```
commonapis/
├── api/                 # HTTP API 定义
│   └── *.proto        # API 规范
└── pb/                  # 生成的代码
    └── *.pb.go        # Protocol Buffer
```

**提供的定义：**
- 共享的 gRPC 服务定义
- 共享的消息类型
- 跨服务通信协议

---

## 2️⃣ go-rpc-gateway 现有能力

### 核心特性

| 特性 | 状态 | 备注 |
|------|------|------|
| gRPC 服务注册 | ✅ | `Gateway.RegisterService()` |
| HTTP 路由 | ✅ | `Gateway.RegisterHTTPRoute()` |
| 数据库连接池 | ✅ | `cpool.Manager` 支持 GORM |
| Redis 连接 | ✅ | `cpool.Manager` 支持 Redis |
| 中间件系统 | ✅ | 完整的中间件栈 |
| JWT 认证 | ✅ | `cpool.jwt` |
| 日志管理 | ✅ | `global.LOGGER` |
| 配置管理 | ✅ | `go-config` 热加载 |
| 健康检查 | ✅ | Built-in |
| Prometheus 指标 | ✅ | `middleware.metrics` |
| 链路追踪 | ✅ | `middleware.tracing` (Jaeger) |
| PProf 性能分析 | ✅ | Built-in |
| Swagger 文档 | ✅ | `EnableSwagger()` |

### 缺失或需增强

| 功能 | 建议 |
|------|------|
| MQTT 支持 | 🔧 需完成 (已有框架) |
| Casbin 权限 | 🔧 需完成 (已有框架) |
| 缓存层 | 🔧 需完成 |
| 多租户支持 | ⚠️ 需设计 |
| 管理后台服务 | ⚠️ 需单独实现 |

---

## 3️⃣ 迁移路线图

### 阶段 1：准备与分析（1-2 周）

**目标：** 建立基础框架，准备迁移环境

#### 1.1 代码审计
```bash
# 分析 engine-ads-link-service 的核心功能
✓ 数据模型（在 persist/ 中）
✓ 业务逻辑（在 server/ 中）
✓ API 定义（在 api/ 和 pb/ 中）
✓ 中间件依赖（commonpkgs 中）
✓ 配置需求（config/ 中）
```

#### 1.2 创建适配层
在 `go-rpc-gateway` 中创建新的服务模块：

```go
// go-rpc-gateway/services/adslink/
├── models/          # 数据模型（从 engine-ads-link-service/persist 迁移）
├── handler/         # HTTP 处理器
├── service/         # 业务逻辑
└── middleware.go    # 服务特定的中间件
```

#### 1.3 建立依赖关系
```go
// go-rpc-gateway/services/adslink/service.go
import (
    "github.com/Divine-Dragon-Voyage/commonpkgs/middleware"
    "github.com/Divine-Dragon-Voyage/commonapis/pb"
)
```

---

### 阶段 2：数据模型迁移（1-2 周）

**目标：** 将所有数据模型适配到 go-rpc-gateway

#### 2.1 模型文件迁移

**从：** `engine-ads-link-service/persist/models/`
**到：** `go-rpc-gateway/services/adslink/models/`

```go
// go-rpc-gateway/services/adslink/models/link.go
package models

import "gorm.io/gorm"

type LinkModel struct {
    ID        uint      `gorm:"primaryKey"`
    Url       string    `gorm:"index"`
    ShortCode string    `gorm:"unique"`
    CreatedAt time.Time
    UpdatedAt time.Time
    DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (LinkModel) TableName() string {
    return "links"
}
```

#### 2.2 数据库初始化

利用 `go-rpc-gateway` 的 `cpool.Manager`：

```go
// 在 gateway.Build() 中，PoolManager 已自动初始化
db := gateway.GetDB()  // 获取 GORM 连接

// 运行迁移
db.AutoMigrate(&models.LinkModel{})
```

---

### 阶段 3：服务逻辑迁移（2-3 周）

**目标：** 迁移核心业务逻辑

#### 3.1 创建服务接口

```go
// go-rpc-gateway/services/adslink/service/service.go
package service

import (
    "context"
    "github.com/kamalyes/go-rpc-gateway/services/adslink/models"
)

type LinkService interface {
    CreateLink(ctx context.Context, url string) (*models.LinkModel, error)
    GetLink(ctx context.Context, shortCode string) (*models.LinkModel, error)
    ListLinks(ctx context.Context, page, pageSize int) ([]*models.LinkModel, int64, error)
    UpdateLink(ctx context.Context, link *models.LinkModel) error
    DeleteLink(ctx context.Context, id uint) error
}

type linkService struct {
    db *gorm.DB
}

func NewLinkService(db *gorm.DB) LinkService {
    return &linkService{db: db}
}

// 实现接口方法...
```

#### 3.2 集成 commonpkgs 中间件

```go
// go-rpc-gateway/services/adslink/middleware.go
package adslink

import (
    "github.com/Divine-Dragon-Voyage/commonpkgs/middleware"
)

// 使用 commonpkgs 提供的中间件
var middlewares = []middleware.Middleware{
    middleware.LoggingMiddleware(),
    middleware.AuthMiddleware(),
    middleware.ValidationMiddleware(),
}
```

---

### 阶段 4：API 实现（2-3 周）

**目标：** 实现 HTTP 和 gRPC API

#### 4.1 HTTP 处理器

```go
// go-rpc-gateway/services/adslink/handler/link.go
package handler

import (
    "github.com/gin-gonic/gin"
    "github.com/kamalyes/go-rpc-gateway/services/adslink/service"
)

type LinkHandler struct {
    service service.LinkService
}

func NewLinkHandler(svc service.LinkService) *LinkHandler {
    return &LinkHandler{service: svc}
}

// CreateLink 创建短链
// @Summary 创建短链
// @Description 根据长链创建短链
// @Tags links
// @Accept json
// @Produce json
// @Param request body CreateLinkRequest true "请求体"
// @Success 200 {object} CreateLinkResponse
// @Router /api/v1/links [post]
func (h *LinkHandler) CreateLink(c *gin.Context) {
    var req CreateLinkRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    link, err := h.service.CreateLink(c.Request.Context(), req.URL)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, gin.H{"data": link})
}
```

#### 4.2 gRPC 服务实现

```go
// go-rpc-gateway/services/adslink/grpc/server.go
package grpc

import (
    "context"
    pb "github.com/Divine-Dragon-Voyage/commonapis/pb"
    "github.com/kamalyes/go-rpc-gateway/services/adslink/service"
)

type LinkServiceServer struct {
    pb.UnimplementedLinkServiceServer
    svc service.LinkService
}

func NewLinkServiceServer(svc service.LinkService) *LinkServiceServer {
    return &LinkServiceServer{svc: svc}
}

func (s *LinkServiceServer) CreateLink(ctx context.Context, req *pb.CreateLinkRequest) (*pb.CreateLinkResponse, error) {
    link, err := s.svc.CreateLink(ctx, req.Url)
    if err != nil {
        return nil, err
    }

    return &pb.CreateLinkResponse{
        ShortCode: link.ShortCode,
        Url: link.Url,
    }, nil
}
```

#### 4.3 在 Gateway 中注册服务

```go
// 在应用启动代码中（如 main.go 或 cmd/main.go）
package main

import (
    "github.com/kamalyes/go-rpc-gateway/gateway"
    "github.com/kamalyes/go-rpc-gateway/services/adslink/grpc"
    "github.com/kamalyes/go-rpc-gateway/services/adslink/handler"
    "github.com/kamalyes/go-rpc-gateway/services/adslink/service"
    pb "github.com/Divine-Dragon-Voyage/commonapis/pb"
)

func main() {
    // 构建网关
    gw, err := gateway.NewGateway().
        WithConfigPath("./config/gateway-dev.yaml").
        Build()
    if err != nil {
        panic(err)
    }

    // 获取数据库和日志
    db := gw.GetDB()
    logger := global.LOGGER

    // 初始化服务
    linkService := service.NewLinkService(db)
    linkHandler := handler.NewLinkHandler(linkService)
    linkGRPCServer := grpc.NewLinkServiceServer(linkService)

    // 注册 gRPC 服务
    gw.RegisterService(func(s *grpc.Server) {
        pb.RegisterLinkServiceServer(s, linkGRPCServer)
    })

    // 注册 HTTP 路由
    gw.RegisterHTTPRoute("/api/v1/links", func(w http.ResponseWriter, r *http.Request) {
        switch r.Method {
        case http.MethodPost:
            linkHandler.CreateLink(w, r)
        case http.MethodGet:
            linkHandler.ListLinks(w, r)
        }
    })

    // 启动网关
    if err := gw.Start(); err != nil {
        panic(err)
    }

    gw.WaitForShutdown()
}
```

---

### 阶段 5：配置与部署（1-2 周）

**目标：** 完整的配置和部署方案

#### 5.1 配置文件示例

```yaml
# go-rpc-gateway/config/adslink-dev.yaml
name: "AdsLink Service"
environment: "development"
debug: true

http_server:
  host: "0.0.0.0"
  port: 8080

grpc_server:
  host: "0.0.0.0"
  port: 9090

database:
  driver: "mysql"
  dsn: "user:password@tcp(localhost:3306)/adslink?parseTime=true"
  max_idle_conns: 10
  max_open_conns: 100
  log_level: "warn"

redis:
  addr: "localhost:6379"
  db: 0
  password: ""

jwt:
  signing_key: "your-secret-key"
  expires_time: 86400

logger:
  level: "debug"
  format: "json"

swagger:
  enabled: true
  ui_path: "/swagger"

features:
  health: true
  prometheus: true
  pprof: true
  jaeger: true
```

#### 5.2 Docker 部署

```dockerfile
# Dockerfile
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY . .

RUN go build -o adslink-service ./cmd/main.go

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/adslink-service .
COPY --from=builder /app/config ./config

EXPOSE 8080 9090

CMD ["./adslink-service"]
```

---

## 4️⃣ 迁移检查清单

### 前期准备
- [ ] 完整审计 engine-ads-link-service 代码
- [ ] 列出所有外部依赖
- [ ] 确认数据模型和关系
- [ ] 备份生产数据库
- [ ] 建立迁移分支

### 代码迁移
- [ ] 创建 `services/adslink/` 模块结构
- [ ] 迁移所有数据模型
- [ ] 迁移业务逻辑层
- [ ] 实现 HTTP 处理器
- [ ] 实现 gRPC 服务
- [ ] 迁移数据库迁移脚本
- [ ] 迁移国际化文件 (i18n)
- [ ] 集成 commonpkgs 中间件

### 集成与测试
- [ ] 单元测试（所有服务层）
- [ ] 集成测试（API 端点）
- [ ] 数据库测试（迁移脚本）
- [ ] 性能测试（负载测试）
- [ ] 安全审计

### 部署与验证
- [ ] 开发环境部署
- [ ] 测试环境部署
- [ ] 验收测试
- [ ] 灰度发布（金丝雀部署）
- [ ] 监控和告警配置
- [ ] 文档更新

---

## 5️⃣ 架构对比

### 迁移前（原始架构）

```
┌─────────────────────────────────────────┐
│      engine-ads-link-service            │
├─────────────────────────────────────────┤
│  ├─ Custom Server Setup                 │
│  ├─ Custom HTTP Router                  │
│  ├─ Custom Middleware Stack             │
│  ├─ Custom Config Management            │
│  └─ Custom Pool Management              │
├─────────────────────────────────────────┤
│  ├─ API Layer        (api/)             │
│  ├─ Service Layer    (server/)          │
│  ├─ Persist Layer    (persist/)         │
│  └─ Proto Defs       (pb/)              │
├─────────────────────────────────────────┤
│  Deps: commonpkgs, commonapis           │
└─────────────────────────────────────────┘
```

### 迁移后（集成到 go-rpc-gateway）

```
┌──────────────────────────────────────────────┐
│         go-rpc-gateway (Framework)           │
├──────────────────────────────────────────────┤
│  ├─ Server Management                       │
│  ├─ Config Management                       │
│  ├─ Pool Management (cpool)                 │
│  ├─ Middleware Stack                        │
│  ├─ Observability (Metrics, Tracing)        │
│  ├─ Health Check                            │
│  ├─ Swagger / Docs                          │
│  └─ Graceful Shutdown                       │
├──────────────────────────────────────────────┤
│  services/adslink/                          │
│  ├─ models/       (数据模型)                │
│  ├─ service/      (业务逻辑)                │
│  ├─ handler/      (HTTP 处理)               │
│  ├─ grpc/         (gRPC 服务)               │
│  └─ middleware.go (特定中间件)              │
├──────────────────────────────────────────────┤
│  Deps: commonpkgs, commonapis, go-rpc-gateway│
└──────────────────────────────────────────────┘
```

---

## 6️⃣ 技术亮点 & 优势

### ✅ 统一框架的好处

| 方面 | 原始 | 迁移后 |
|------|------|--------|
| 配置管理 | 自定义 | go-config 热加载 |
| 池管理 | 分散 | 统一 cpool.Manager |
| 中间件 | 混乱 | 标准化栈 |
| 监控 | 手动 | Prometheus + Jaeger |
| 健康检查 | 无 | 内置 |
| 文档 | 无 | Swagger 自动生成 |
| 性能分析 | 无 | PProf 内置 |
| 扩展性 | 困难 | 模块化易扩展 |

### 🚀 迁移成本估计

| 活动 | 耗时 | 人力 |
|------|------|------|
| 代码审计 | 3-5 天 | 1 人 |
| 模型迁移 | 2-3 天 | 1 人 |
| 逻辑迁移 | 1-2 周 | 2 人 |
| API 实现 | 1-2 周 | 2 人 |
| 测试 | 1 周 | 2-3 人 |
| 部署/文档 | 3-5 天 | 1 人 |
| **总计** | **4-5 周** | **2-3 人** |

---

## 7️⃣ 潜在风险 & 缓解方案

### 🔴 高风险

| 风险 | 影响 | 缓解方案 |
|------|------|--------|
| 数据迁移错误 | 数据丢失 | 完整备份、灰度发布、回滚方案 |
| API 兼容性破裂 | 客户端失败 | 版本控制、向后兼容性测试 |
| 性能下降 | 用户体验 | 性能基准测试、负载测试 |

### 🟡 中等风险

| 风险 | 影响 | 缓解方案 |
|------|------|--------|
| 依赖版本冲突 | 编译失败 | 提前检查、go mod tidy |
| 配置不完整 | 启动失败 | 详细文档、配置验证 |
| 中间件调整 | 功能异常 | 单元测试、集成测试 |

---

## 8️⃣ 下一步建议

### 即期（本周）
1. 对本文档进行评审和补充
2. 与团队讨论并确认时间表
3. 建立专门的迁移分支
4. 开始代码审计

### 短期（2 周内）
1. 完成阶段 1-2 的工作
2. 建立开发环境
3. 运行初步测试

### 中期（4 周内）
1. 完成所有代码迁移
2. 通过完整的测试套件
3. 准备生产环境部署

### 长期（迁移后）
1. 监控性能指标
2. 收集团队反馈
3. 持续优化和改进

---

## 📞 支持与联系

如有任何问题，请联系项目维护者。

---

**文档版本：** 1.0  
**最后更新：** 2025-11-13  
**作者：** Architecture Team
