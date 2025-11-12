# 🚀 Go RPC Gateway

<div align="center">

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.23-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)]()
[![Go Report Card](https://img.shields.io/badge/go%20report-A+-brightgreen.svg)]()
[![Coverage](https://img.shields.io/badge/coverage-85%25-brightgreen.svg)]()
[![Release](https://img.shields.io/badge/release-v1.0.0-blue.svg)]()

**🎯 企业级 gRPC-Gateway 微服务网关框架，深度集成五大核心库**

集成了 [go-config](https://github.com/kamalyes/go-config) 统一配置管理、[go-logger](https://github.com/kamalyes/go-logger) 结构化日志、[go-toolbox](https://github.com/kamalyes/go-toolbox) 工具集、[go-sqlbuilder](https://github.com/kamalyes/go-sqlbuilder) SQL构建器和 [go-wsc](https://github.com/kamalyes/go-wsc) WebSocket客户端，提供数据库、缓存、对象存储、消息队列等完整的微服务解决方案。

[🚀 快速开始](#-快速开始) • [⚙️ 配置文档](#️-配置文档) • [🏗️ 架构设计](#️-架构设计) • [📦 部署指南](#-部署指南) • [📚 示例代码](#-示例代码)

</div>

---

## 🎯 项目特色

<table>
<tr>
<th align="center">🏗️ 架构优势</th>
<th align="center">🔧 技术栈</th>
<th align="center">🚀 开箱即用</th>
</tr>
<tr>
<td>

• **go-config 统一配置** - 多源配置管理  
• **go-logger 结构化日志** - 高性能日志系统
• **go-toolbox 工具集** - 常用工具函数
• **go-sqlbuilder SQL构建器** - 类型安全的SQL构建
• **go-wsc WebSocket客户端** - 高性能WebSocket支持
• **中间件生态** - 15+ 内置中间件
• **云原生支持** - K8s/Docker 友好

</td>
<td>

• **gRPC/HTTP** - 双协议支持
• **Prometheus** - 指标监控
• **OpenTelemetry** - 链路追踪  
• **Zap Logger** - 结构化日志
• **多语言支持** - 19种语言i18n

</td>
<td>

• **零配置启动** - 默认配置可用
• **热重载配置** - 运行时更新
• **健康检查** - 多组件监控
• **性能分析** - 内置 pprof
• **安全防护** - 多层安全机制

</td>
</tr>
</table>

### 🏗️ 系统架构

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                               🚀 Go RPC Gateway                                          │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                         │
│  ┌─────────────┐    ┌──────────────┐    ┌─────────────┐    ┌──────────────┐          │
│  │   Gateway   │    │   Server     │    │ Middleware  │    │   Config     │          │
│  │  (Entry)    │────│   Manager    │────│  Manager    │────│  Manager     │          │
│  └─────────────┘    └──────────────┘    └─────────────┘    └──────────────┘          │
│                                                                                         │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│                              🌐 协议层                                                    │
│  ┌─────────────┐              │              ┌─────────────┐                          │
│  │ HTTP Server │◀─────────────┼─────────────▶│ gRPC Server │                          │
│  │ (:8080)     │         gRPC-Gateway        │ (:9090)     │                          │
│  └─────────────┘                             └─────────────┘                          │
│                                                                                         │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│                            🛡️ 中间件层                                                   │
│ ┌──────────────┬──────────────┬──────────────┬──────────────┬─────────────┐           │
│ │   Security   │  RateLimit   │   Logging    │   Metrics    │    i18n     │           │
│ │   CORS/Auth  │ Token Bucket │  go-logger   │ Prometheus   │ 19 Languages│           │
│ ├──────────────┼──────────────┼──────────────┼──────────────┼─────────────┤           │
│ │  Signature   │   Recovery   │  RequestID   │   Tracing    │   Health    │           │
│ │go-toolbox加密│  Panic Safe  │  UUID Track  │OpenTelemetry │ Components  │           │
│ └──────────────┴──────────────┴──────────────┴──────────────┴─────────────┘           │
│                                                                                         │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│                         🏗️ 五大核心库集成层                                              │
│ ┌───────────┬─────────────┬─────────────┬─────────────┬─────────────────────────────┐ │
│ │go-config  │ go-logger   │ go-toolbox  │go-sqlbuilder│          go-wsc             │ │
│ │统一配置管理│ 结构化日志   │ 工具函数集   │ SQL构建器   │      WebSocket客户端        │ │
│ │           │             │             │             │                             │ │
│ │• 多源配置  │• Zap日志     │• 加密/解密  │• 类型安全   │• 高性能连接                  │ │
│ │• 热重载    │• 多级别     │• 数据转换   │• SQL构建    │• 自动重连                    │ │
│ │• 环境变量  │• 日志轮转   │• JSON/XML   │• 查询构造   │• 消息处理                    │ │
│ │• 配置验证  │• 上下文     │• Base64     │• 条件构建   │• 协议支持                    │ │
│ │• 分层配置  │• 性能优化   │• 算法工具   │• 批量操作   │• 事件驱动                    │ │
│ └───────────┴─────────────┴─────────────┴─────────────┴─────────────────────────────┘ │
│                                                                                         │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│                           💾 基础设施层 (内置支持)                                       │
│ ┌──────────────┬──────────────┬──────────────┬──────────────┬─────────────┐           │
│ │   Database   │    Redis     │    MinIO     │   RabbitMQ   │   Consul    │           │
│ │ MySQL/Postgres│   Cache     │Object Storage│Message Queue │Service Mesh │           │
│ └──────────────┴──────────────┴──────────────┴──────────────┴─────────────┘           │
│                                                                                         │
└─────────────────────────────────────────────────────────────────────────────────────────┘
```

## ✨ 核心特性

### 🏗️ 四大核心库深度集成

#### 📋 go-config - 统一配置管理

- **多种配置源支持** - 支持 YAML、JSON、TOML、ENV 等多种配置格式
- **配置热重载** - 监听配置文件变化，运行时动态更新
- **环境变量覆盖** - 支持通过环境变量覆盖配置项
- **配置验证** - 内置配置格式和值的校验机制
- **分层配置** - 支持 base、dev、prod 等多环境配置

```go
// 使用 go-config 管理配置
configManager, err := config.NewConfigManager("config.yaml")
if err != nil {
    log.Fatal(err)
}

// 获取网关配置
gatewayConfig := configManager.GetGatewayConfig()
```

#### �️ go-sqlbuilder - 类型安全SQL构建器

- **类型安全** - 编译时SQL语法检查，避免运行时错误
- **链式调用** - 流畅的API设计，直观易用
- **多数据库支持** - MySQL、PostgreSQL、SQLite等主流数据库
- **高级查询** - 支持复杂查询、子查询、联表查询
- **批量操作** - 高效的批量插入、更新操作

```go
// go-sqlbuilder 类型安全的SQL构建
import "github.com/kamalyes/go-sqlbuilder"

// 构建查询
query := sqlbuilder.Select("id", "name", "email").
    From("users").
    Where("age > ?", 18).
    OrderBy("created_at DESC").
    Limit(10)

// 执行查询
sql, args := query.Build()
```

#### 🌐 go-wsc - 高性能WebSocket客户端

- **高性能连接** - 基于goroutine的高并发WebSocket客户端
- **自动重连** - 智能断线重连，保证连接稳定性
- **消息处理** - 支持JSON、二进制等多种消息格式
- **事件驱动** - 基于事件的消息处理机制
- **协议支持** - 完整的WebSocket协议实现

```go
// go-wsc 高性能WebSocket客户端
import "github.com/kamalyes/go-wsc"

// 创建WebSocket客户端
client := wsc.NewClient("ws://localhost:8080/ws")
client.OnMessage(func(msg []byte) {
    // 处理收到的消息
})
```

#### 📝 go-logger - 高性能日志系统

- **结构化日志** - 基于 Zap 的高性能结构化日志
- **多级别输出** - 支持 Debug、Info、Warn、Error、Fatal 等级别
- **多输出格式** - JSON、文本格式可选
- **日志轮转** - 支持按时间、大小进行日志轮转
- **上下文支持** - 支持携带请求ID、用户信息等上下文

```go
// 使用 go-logger 记录日志
import "github.com/kamalyes/go-logger/pkg/logger"

// 结构化日志记录
logger.Info("用户登录成功", 
    logger.String("user_id", "123"),
    logger.String("ip", "192.168.1.100"),
    logger.Duration("duration", time.Since(start)),
)
```

#### 🧰 go-toolbox - 常用工具集

- **加密解密** - AES、RSA、HMAC 等加密算法
- **ID生成器** - UUID、雪花算法、NanoID 等
- **数据转换** - JSON、XML、Base64 等格式转换
- **字符串工具** - 各种字符串处理函数
- **时间工具** - 时间格式化、解析、计算等
- **网络工具** - IP检查、URL解析等

```go
// 使用 go-toolbox 工具函数
import "github.com/kamalyes/go-toolbox/pkg/random"
import "github.com/kamalyes/go-toolbox/pkg/crypto"

// 生成随机ID
requestID := random.GenerateUUID()

// HMAC签名验证
valid := crypto.ValidateHMAC(data, signature, secretKey)
```

### 🏗️ 架构优势

- **🔧 模块化设计** - 可插拔的组件架构，支持自定义扩展
- **🎯 go-config 深度集成** - 统一配置管理，支持多种配置源
- **🔄 五大核心库集成** - go-config配置管理、go-logger日志、go-toolbox工具集、go-sqlbuilder SQL构建器、go-wsc WebSocket客户端
- **📊 企业级监控** - 集成 Prometheus + OpenTelemetry 完整可观测性
- **🔥 配置热重载** - 运行时动态更新配置，零停机变更

### 🛡️ 安全与性能

- **🚦 智能限流** - 支持令牌桶、滑动窗口等多种限流算法
- **🔐 请求签名** - 内置 HMAC-SHA256 安全验证机制
- **🛡️ 安全中间件** - CORS、安全头、XSS防护等多层安全机制
- **⚡ 高性能日志** - 基于 Zap 的结构化日志系统
- **🔍 性能分析** - 内置 pprof 性能分析工具

### 🌍 国际化与扩展

- **🌐 多语言支持** - 支持 19 种语言的国际化
- **📝 模板数据支持** - 支持动态数据插值和模板渲染
- **🔄 语言回退机制** - 自动回退到默认语言
- **🎪 丰富中间件** - 15+ 内置中间件，支持自定义中间件
- **📦 开箱即用** - 零配置启动，默认配置即可使用

## 🎪 中间件生态系统

| 分类 | 中间件 | 功能描述 | 配置复杂度 |
|------|--------|----------|------------|
| **🛡️ 安全** | Security | 安全头设置、XSS防护、CSP策略 | ⭐️⭐️ |
| | CORS | 跨域资源共享配置 | ⭐️ |
| | Signature | HMAC-SHA256 请求签名验证 | ⭐️⭐️⭐️ |
| **📊 监控** | Metrics | Prometheus 指标收集 | ⭐️⭐️ |
| | Logging | 结构化日志记录 | ⭐️⭐️ |
| | Tracing | OpenTelemetry 链路追踪 | ⭐️⭐️⭐️ |
| | Health | 健康检查 (Redis/MySQL/自定义) | ⭐️⭐️ |
| **🚦 控制** | RateLimit | 流量控制 (令牌桶/滑动窗口) | ⭐️⭐️⭐️ |
| | Recovery | 异常恢复处理 | ⭐️ |
| | RequestID | 请求链路追踪ID | ⭐️ |
| **🌍 体验** | I18n | 19种语言国际化 | ⭐️⭐️⭐️ |
| | Access | 访问日志记录 | ⭐️⭐️ |
| **🔧 开发** | PProf | 性能分析工具 | ⭐️⭐️ |
| | Banner | 服务启动横幅 | ⭐️ |

## � 快速上手示例

### 1️⃣ 最简示例 (零配置)

```go
package main

import "github.com/kamalyes/go-rpc-gateway"

func main() {
    // 🎯 创建网关 (自动集成四大核心库)
    gw, _ := gateway.New()
    
    // 🚀 启动服务
    gw.Start()
}
```

### 2️⃣ 完整集成示例

查看 `examples/integration-demo/main.go` 了解四大核心库的完整使用：

```bash
# 运行集成演示
cd examples/integration-demo
go run main.go

# 访问健康检查
curl http://localhost:8080/health

# 查看组件状态
curl http://localhost:8080/components
```

### 3️⃣ 配置文件示例

参考 `config/examples/complete-config.yaml` 查看完整的配置选项，包括：

- 🏗️ go-config 配置管理
- 💾 内置企业级组件 (数据库、Redis、MinIO 等)
- 📝 go-logger 日志配置
- 🧰 go-toolbox 工具配置

## �📦 依赖管理

本项目集成了以下核心依赖库：

### 🏗️ 五大核心库

| 库名称 | 版本 | 功能描述 | 仓库地址 |
|--------|------|----------|----------|
| **go-config** | v0.7.0 | 统一配置管理 | [go-config](https://github.com/kamalyes/go-config) |
| **go-logger** | latest | 结构化日志 | [go-logger](https://github.com/kamalyes/go-logger) |
| **go-toolbox** | v0.11.63 | 工具函数集 | [go-toolbox](https://github.com/kamalyes/go-toolbox) |
| **go-sqlbuilder** | latest | SQL构建器 | [go-sqlbuilder](https://github.com/kamalyes/go-sqlbuilder) |
| **go-wsc** | latest | WebSocket客户端 | [go-wsc](https://github.com/kamalyes/go-wsc) |

### ⚡ 核心依赖

| 依赖库 | 版本 | 用途 |
|--------|------|------|
| gRPC | v1.62.1 | RPC框架 |
| grpc-gateway/v2 | v2.19.1 | HTTP/gRPC转换 |
| Prometheus | v1.18.0 | 监控指标 |
| OpenTelemetry | v1.24.0 | 链路追踪 |
| Viper | v1.19.0 | 配置管理 |
| Zap | v1.27.0 | 高性能日志 |

### 🔧 企业级组件 (内置支持)

| 组件 | 功能描述 | 支持版本 |
|------|----------|----------|
| **数据库** | MySQL、PostgreSQL、SQLite | 多版本 |
| **缓存** | Redis 单机/集群/哨兵 | Redis 6+ |
| **对象存储** | MinIO、阿里云OSS、AWS S3 | 兼容S3 API |
| **消息队列** | RabbitMQ、Kafka | 多版本 |
| **服务发现** | Consul、Etcd | 最新版 |

## 📦 快速安装

### 方式一：Go Modules (推荐)

```bash
# 初始化项目
mkdir my-gateway && cd my-gateway
go mod init my-gateway

# 安装最新版本
go get github.com/kamalyes/go-rpc-gateway@latest

# 安装依赖
go mod tidy
```

### 方式二：直接克隆

```bash
# 克隆项目
git clone https://github.com/kamalyes/go-rpc-gateway.git
cd go-rpc-gateway

# 安装依赖
go mod download
```

## 🚀 快速开始

### 1️⃣ 极简启动 (30秒上手)

创建 `main.go`:

```go
package main

import "github.com/kamalyes/go-rpc-gateway"

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

### 2️⃣ 使用配置文件

创建 `config.yaml`:

```yaml
gateway:
  http:
    port: 8080
  grpc:
    port: 9090

# 启用数据库 (可选)
mysql:
  host: "localhost"
  port: 3306
  username: "root"
  password: "password"
  dbname: "mydb"

# 启用 Redis (可选)  
redis:
  host: "localhost"
  port: 6379
```

创建 `main.go`:

```go
package main

import (
    gateway "github.com/kamalyes/go-rpc-gateway"
    "github.com/kamalyes/go-rpc-gateway/config"
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
    
    gw.Start()
}
```

### 3️⃣ 完整功能示例

```go
package main

import (
    "context"
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

## ⚙️ 配置文档

### 📋 完整配置示例

## ⚙️ 配置文档

Go RPC Gateway 基于四大核心库提供了完整的配置管理体系，支持多种配置源和格式。

### 📋 配置文件结构

```yaml
# 完整配置示例 - config/app.yaml
app:
  name: "go-rpc-gateway"
  version: "v1.0.0"
  environment: "production"

# go-logger 日志配置
logger:
  level: "info"
  format: "json"
  output: ["stdout", "file"]

# 企业级组件配置
components:
  database:
    enabled: true
    driver: "mysql"
    password: "${DB_PASSWORD}"  # 支持环境变量
  redis:
    enabled: true
    addr: "localhost:6379"
  storage:
    enabled: true
    provider: "minio"

# 中间件配置  
middleware:
  security:
    enabled: true
    signature:
      secret_key: "${SIGNATURE_SECRET}"  # go-toolbox 加密
  rate_limit:
    enabled: true
    rate: 1000
```

### 🏗️ 四大核心库配置详解

#### 📋 go-config 配置管理

**特性**：

- 多种配置格式：YAML、JSON、TOML
- 环境变量支持：`${VAR_NAME:default}`
- 配置热重载：文件变化自动更新
- 配置验证：类型和值校验
- 分层配置：环境特定配置覆盖

**使用示例**：

```go
// 创建配置管理器
configManager, err := config.NewConfigManager("config/app.yaml")
if err != nil {
    log.Fatal(err)
}

// 获取配置
gatewayConfig := configManager.GetGatewayConfig()

// 监听配置变化
configManager.OnConfigChange(func() {
    log.Println("配置已更新")
})
```

#### 🔧 企业级组件配置

**支持的组件**：

**数据库**：

- MySQL 5.7+, 8.0+
- PostgreSQL 12+  
- SQLite 3.x
- 读写分离、连接池

```yaml
components:
  database:
    driver: "mysql"
    primary:
      host: "localhost"
      port: 3306
      username: "gateway"
      password: "${DB_PASSWORD}"
    replicas:  # 读写分离
      - host: "replica1.example.com"
```

**Redis 缓存**：

- 单机/集群/哨兵模式
- 连接池管理
- 故障转移

```yaml
components:
  redis:
    mode: "cluster"  # single, cluster, sentinel
    cluster:
      addrs: ["node1:6379", "node2:6379"]
      password: "${REDIS_PASSWORD}"
```

**对象存储**：

- MinIO、阿里云OSS、AWS S3
- 统一接口、多云支持

```yaml
components:
  storage:
    provider: "minio"  # minio, aliyun_oss, aws_s3
    minio:
      endpoint: "localhost:9000"
      access_key: "${MINIO_ACCESS_KEY}"
```

#### 📝 go-logger 日志配置

**高性能结构化日志**：

- 基于 Zap，零分配设计
- 多输出目标：控制台、文件、远程
- 自动日志轮转
- 上下文携带

```yaml
logger:
  level: "info"  # debug, info, warn, error, fatal
  format: "json"  # json, text
  output: ["stdout", "file"]
  file:
    path: "logs/gateway.log"
    max_size: 100  # MB
    max_backups: 10
    compress: true
```

**使用示例**：

```go
import "github.com/kamalyes/go-logger/pkg/logger"

// 结构化日志
logger.Info("用户登录",
    logger.String("user_id", "123"),
    logger.String("ip", clientIP),
    logger.Duration("duration", time.Since(start)),
)
```

#### 🧰 go-toolbox 工具集

**加密安全**：

- AES-256-GCM 对称加密
- RSA 公钥加密
- HMAC-SHA256 签名验证
- 安全随机数生成

```yaml
middleware:
  security:
    signature:
      enabled: true
      algorithm: "hmac_sha256"
      secret_key: "${SIGNATURE_SECRET}"
tools:
  crypto:
    default_algorithm: "aes_256_gcm"
```

**ID 生成器**：

- UUID v4：全球唯一
- ULID：字典序UUID
- 雪花算法：分布式ID
- NanoID：短ID生成

```yaml
tools:
  id_generator:
    default_type: "uuid"  # uuid, ulid, nanoid, snowflake
    snowflake:
      machine_id: 1
middleware:
  request_id:
    generator: "uuid"
```

### 🔧 完整配置示例

<details>
<summary>点击查看完整的 config.yaml 配置文件</summary>

```yaml
# ===========================================
# Go RPC Gateway 完整配置文件
# ===========================================

# 基础服务配置 (继承自 go-config)
server:
  name: go-rpc-gateway
  version: v1.0.0
  environment: development

# Gateway 核心配置
gateway:
  name: go-rpc-gateway
  version: v1.0.0
  debug: true
  
  # HTTP 服务配置
  http:
    host: 0.0.0.0
    port: 8080
    read_timeout: 30
    write_timeout: 30
    idle_timeout: 120
    max_header_bytes: 1048576  # 1MB
    
  # gRPC 服务配置
  grpc:
    host: 0.0.0.0
    port: 9090
    network: tcp
    enable_reflection: true
    max_recv_msg_size: 4194304    # 4MB
    max_send_msg_size: 4194304    # 4MB

  # 健康检查配置
  health_check:
    enabled: true
    path: /health

# 中间件配置
middleware:
  # CORS 跨域配置
  cors:
    enabled: true
    allow_origins: ["*"]
    allow_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allow_headers: ["*"]
    expose_headers: ["Content-Length"]
    allow_credentials: true
    max_age: 86400

  # 限流配置
  rate_limit:
    enabled: true
    algorithm: token_bucket    # token_bucket, sliding_window
    rate: 100                  # 每秒请求数
    burst: 200                 # 突发容量
    
  # 安全配置
  security:
    enabled: true
    frame_deny: true
    content_type_nosniff: true
    xss_protection: true

  # 请求签名验证
  signature:
    enabled: false
    secret_key: your-secret-key
    expire_duration: 300       # 5分钟
    algorithm: HMAC-SHA256

# 监控配置
monitoring:
  # Prometheus 指标
  metrics:
    enabled: true
    path: /metrics
    namespace: gateway
    subsystem: http
    
  # 链路追踪
  tracing:
    enabled: false
    service_name: go-rpc-gateway
    endpoint: http://jaeger:14268/api/traces

# 数据库配置 (go-config)
mysql:
  path: 127.0.0.1
  port: "3306"
  config: charset=utf8mb4&parseTime=True&loc=Local
  db-name: gateway_db
  username: root
  password: ""
  max-idle-conns: 10
  max-open-conns: 100

# Redis 配置 (go-config)
redis:
  db: 0
  addr: 127.0.0.1:6379
  password: ""
  pool-size: 100

# 日志配置
logging:
  level: info                  # debug, info, warn, error, fatal
  format: json                 # json, text
  output: ["stdout", "file"]
  file_path: logs/gateway.log
  max_size: 100               # MB
  max_backups: 10
  max_age: 30                 # days
  compress: true
```

</details>

### 🔧 环境变量配置

支持通过环境变量覆盖配置项：

```bash
# 基本配置
export GATEWAY_HOST=0.0.0.0
export GATEWAY_HTTP_PORT=8080
export GATEWAY_GRPC_PORT=9090

# 数据库配置
export MYSQL_HOST=localhost
export MYSQL_PASSWORD=your_password

# Redis 配置  
export REDIS_ADDR=localhost:6379
export REDIS_PASSWORD=your_password

# 启动服务
./bin/gateway
```

### 🎛️ 配置优先级

1. **命令行参数** (最高优先级)
2. **环境变量**
3. **配置文件**
4. **默认值** (最低优先级)

## 🏗 架构设计

### 📂 项目结构 (重构后)

```
go-rpc-gateway/
├── 🎯 cmd/                    # 应用程序入口
│   ├── gateway/
│   │   └── main.go           # 主程序入口 - 网关服务
│   ├── simple-gateway/
│   │   └── main.go           # 简单网关示例
│   └── test-adapter/
│       └── main.go           # 测试适配器
├── 🏗️ server/                 # 服务器实现 [已重构]
│   ├── server.go            # 🔧 核心结构定义
│   ├── core.go              # 🛠️ 组件初始化
│   ├── grpc.go              # 📡 gRPC 服务器
│   ├── http.go              # 🌐 HTTP 网关
│   ├── middleware_init.go   # 🔌 中间件初始化
│   ├── lifecycle.go         # 🔄 生命周期管理
│   ├── banner.go            # 🎨 启动横幅
│   └── README.md            # 📖 重构说明文档
├── 🔌 middleware/             # 中间件生态系统
│   ├── manager.go           # 中间件管理器
│   ├── access.go            # 访问日志
│   ├── observability.go     # 可观测性
│   ├── metrics.go           # 监控指标
│   ├── security.go          # 安全防护
│   ├── ratelimit.go         # 流量控制
│   ├── recovery.go          # 异常恢复
│   ├── logging.go           # 日志中间件
│   ├── signature.go         # 签名验证
│   ├── pprof.go             # 性能分析
│   ├── pprof_gateway.go     # PProf网关
│   ├── pprof_scenarios.go   # 性能测试场景
│   ├── requestid.go         # 请求ID
│   └── types.go             # 类型定义
├── � config/                 # 配置管理
│   ├── defaults.go          # 默认配置
│   ├── gateway.go           # 网关配置
│   ├── manager.go           # 配置管理器
│   ├── middleware.go        # 中间件配置
│   ├── monitoring.go        # 监控配置
│   └── security.go          # 安全配置
├── 🏷️ constants/              # 常量定义
│   ├── gateway.go           # 网关常量
│   └── headers.go           # HTTP头常量
├── � pbuf/                   # Protocol Buffers定义
│   ├── buf.gen.yaml         # Buf配置文件
│   ├── README.md            # Proto文档说明
│   └── common/              # 通用proto定义
│       ├── common.proto     # 通用消息定义
│       └── common.pb.go     # 生成的Go代码
├── 📚 examples/               # 使用示例
│   ├── 01-quickstart/       # 快速开始
│   ├── 02-with-config/      # 配置文件示例
│   ├── 03-middleware/       # 中间件示例
│   ├── 04-pprof/           # 性能分析示例
│   ├── 05-grpc/            # gRPC集成示例
│   ├── 06-enterprise/      # 企业级示例
│   ├── config/             # 配置模板
│   ├── demo/               # 演示程序
│   ├── docker/             # Docker配置
│   ├── k8s/                # Kubernetes配置
│   └── README.md           # 示例说明文档
├── 📝 docs/                   # 文档目录
│   ├── BUILTIN_PPROF_USAGE.md # 内置PProf使用指南
│   └── PPROF_MIDDLEWARE.md    # PProf中间件文档
├── 🛠️ scripts/               # 构建和工具脚本
│   ├── build-pprof.bat     # Windows PProf构建脚本
│   └── build-pprof.sh      # Unix PProf构建脚本
├── 🧪 tests/                 # 测试目录
│   └── performance/         # 性能测试
│       └── load-test.js     # 负载测试脚本
├── 🛠️ build scripts          # 构建脚本
│   ├── build.sh            # Unix 构建脚本
│   ├── build.bat           # Windows 构建脚本
│   ├── start.sh            # Unix 启动脚本
│   ├── start.bat           # Windows 启动脚本
│   ├── run-with-logs.sh    # Unix 日志启动脚本
│   └── run-with-logs.bat   # Windows 日志启动脚本
├── gateway.go               # 主要网关包导出
├── go.mod                   # Go模块定义
├── go.sum                   # 依赖校验文件
├── Makefile                 # Make构建脚本
└── README.md                # 项目说明文档
```

### 🎯 设计原则

<table>
<tr>
<td width="20%">

**🔧 模块化设计**

- 单一职责原则
- 松耦合架构
- 可插拔组件

</td>
<td width="20%">

**⚙️ 配置驱动**

- 配置文件控制
- 热重载支持
- 环境变量覆盖

</td>
<td width="20%">

**🔌 中间件架构**

- 管道式处理
- 链式调用
- 自定义扩展

</td>
<td width="20%">

**🔍 可观测性**

- 结构化日志
- 指标收集
- 链路追踪

</td>
<td width="20%">

**📦 类型安全**

- Protocol Buffers
- 统一响应格式
- 编译时检查

</td>
</tr>
</table>

### 🔄 重构亮点

| 组件 | 文件数 | 职责 | 优势 |
|------|--------|------|------|
| **Server核心** | 6个文件 | 服务器生命周期管理 | 模块化，易维护 |
| **Middleware** | 12个文件 | 中间件生态系统 | 功能完整，可插拔 |
| **Config管理** | 6个文件 | 配置管理和热重载 | 集中管理，类型安全 |
| **PBuf定义** | 2个文件 | Protocol Buffers | 标准化响应，类型安全 |
| **常量定义** | 2个文件 | 系统常量集中管理 | 避免硬编码，易维护 |

> 📊 **重构效果**: 原始单一文件拆分为专业化模块，提高了代码的可读性、可维护性和可测试性。

## 🔧 中间件系统

### 📦 内置中间件

<table>
<tr>
<th>类别</th>
<th>中间件</th>
<th>功能描述</th>
<th>配置示例</th>
</tr>
<tr>
<td rowspan="4"><strong>🛡️ 安全</strong></td>
<td><code>Security</code></td>
<td>安全头设置、XSS防护</td>
<td><code>security.enabled: true</code></td>
</tr>
<tr>
<td><code>CORS</code></td>
<td>跨域资源共享控制</td>
<td><code>cors.allow_origins: ["*"]</code></td>
</tr>
<tr>
<td><code>Signature</code></td>
<td>请求签名验证</td>
<td><code>signature.algorithm: HMAC-SHA256</code></td>
</tr>
<tr>
<td><code>RequestID</code></td>
<td>请求ID生成和追踪</td>
<td>自动启用</td>
</tr>
<tr>
<td rowspan="3"><strong>📊 监控</strong></td>
<td><code>Metrics</code></td>
<td>Prometheus指标收集</td>
<td><code>metrics.enabled: true</code></td>
</tr>
<tr>
<td><code>Tracing</code></td>
<td>OpenTelemetry链路追踪</td>
<td><code>tracing.enabled: true</code></td>
</tr>
<tr>
<td><code>Logging</code></td>
<td>结构化访问日志</td>
<td><code>logging.level: info</code></td>
</tr>
<tr>
<td rowspan="2"><strong>🚦 控制</strong></td>
<td><code>RateLimit</code></td>
<td>智能流量控制</td>
<td><code>rate_limit.rate: 100</code></td>
</tr>
<tr>
<td><code>Recovery</code></td>
<td>Panic异常恢复</td>
<td>自动启用</td>
</tr>
</table>

### 🎨 自定义中间件开发

```go
package middleware

import (
    "net/http"
    "time"
)

// CustomAuthMiddleware 自定义认证中间件
func CustomAuthMiddleware(secret string) HTTPMiddleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 1. 获取认证头
            token := r.Header.Get("Authorization")
            
            // 2. 验证逻辑
            if !isValidToken(token, secret) {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            
            // 3. 添加用户信息到上下文
            ctx := context.WithValue(r.Context(), "user_id", getUserID(token))
            r = r.WithContext(ctx)
            
            // 4. 继续处理
            next.ServeHTTP(w, r)
        })
    }
}

// 注册自定义中间件
func (m *Manager) RegisterCustomMiddleware(middleware HTTPMiddleware) {
    m.customMiddlewares = append(m.customMiddlewares, middleware)
}
```

### 🔀 中间件链配置

```yaml
# 中间件执行顺序配置
middleware:
  order:
    - RequestID      # 1. 生成请求ID
    - Recovery       # 2. 异常恢复
    - Logging        # 3. 访问日志
    - CORS           # 4. 跨域处理
    - Security       # 5. 安全头
    - RateLimit      # 6. 流量控制
    - Signature      # 7. 签名验证
    - Metrics        # 8. 指标收集
    - CustomAuth     # 9. 自定义认证
```

## 📊 监控与可观测性

### 📈 Prometheus 指标

<details>
<summary>📊 查看完整指标列表</summary>

```
# HTTP 请求指标
gateway_http_requests_total{method="GET", status="200", path="/api/v1/users"}
gateway_http_request_duration_seconds{method="GET", path="/api/v1/users"}
gateway_http_request_size_bytes{method="POST", path="/api/v1/users"} 
gateway_http_response_size_bytes{method="GET", path="/api/v1/users"}

# gRPC 请求指标  
gateway_grpc_requests_total{service="UserService", method="GetUser", status="OK"}
gateway_grpc_request_duration_seconds{service="UserService", method="GetUser"}

# 业务指标
gateway_active_connections_total
gateway_middleware_duration_seconds{middleware="rate_limit"}
gateway_database_connections_active
gateway_redis_operations_total{operation="GET", status="success"}
```

</details>

### 💊 健康检查

```bash
# 基础健康检查
curl http://localhost:8080/health
# 响应: {"status":"ok","service":"go-rpc-gateway","timestamp":1699123456}

# 详细健康检查 (包含依赖服务状态)
curl http://localhost:8080/health?detail=true
# 响应示例:
{
  "status": "ok",
  "service": "go-rpc-gateway", 
  "timestamp": 1699123456,
  "checks": {
    "database": {"status": "ok", "latency_ms": 2},
    "redis": {"status": "ok", "latency_ms": 1},
    "external_api": {"status": "warning", "latency_ms": 1500}
  }
}
```

### 📊 指标采集端点

```bash
# Prometheus 指标采集
curl http://localhost:8080/metrics

# 自定义指标查询
curl http://localhost:8080/metrics?format=json
```

### � 链路追踪

配置 OpenTelemetry 链路追踪：

```yaml
monitoring:
  tracing:
    enabled: true
    service_name: go-rpc-gateway
    endpoint: http://jaeger:14268/api/traces
    sampling_rate: 0.1  # 10% 采样率
```

## 🔒 安全特性

### 🔐 请求签名验证

<details>
<summary>📝 查看签名验证实现</summary>

```yaml
# 配置签名验证
middleware:
  signature:
    enabled: true
    secret_key: "your-256-bit-secret"
    expire_duration: 300  # 5分钟
    algorithm: HMAC-SHA256
    fields:
      - timestamp
      - request_id  
      - body_hash
```

**客户端签名生成示例:**

```go
func generateSignature(secretKey, method, uri, body string, timestamp int64) string {
    // 1. 构建签名字符串
    signString := fmt.Sprintf("%s\n%s\n%s\n%d", 
        method, uri, body, timestamp)
    
    // 2. HMAC-SHA256 签名
    h := hmac.New(sha256.New, []byte(secretKey))
    h.Write([]byte(signString))
    
    // 3. Base64 编码
    return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
```

**请求头设置:**

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -H "X-Timestamp: 1699123456" \
  -H "X-Signature: generated_signature_here" \
  -d '{"name":"test"}'
```

</details>

### 🛡️ 安全防护

| 安全特性 | 说明 | 配置 |
|----------|------|------|
| **XSS 防护** | 跨站脚本攻击防护 | `security.xss_protection: true` |
| **CSRF 保护** | 跨站请求伪造保护 | `security.csrf_protection: true` |
| **内容嗅探防护** | 防止MIME类型混淆攻击 | `security.content_type_nosniff: true` |
| **点击劫持防护** | X-Frame-Options头设置 | `security.frame_deny: true` |
| **HTTPS 强制** | 强制HTTPS重定向 | `security.force_https: true` |

## 🚀 部署指南

### 🐳 Docker 部署

<details>
<summary>📦 查看完整 Docker 配置</summary>

**多阶段构建 Dockerfile:**

```dockerfile
# ===========================================
# 多阶段构建，优化镜像大小
# ===========================================

# 构建阶段
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git ca-certificates tzdata

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o gateway cmd/gateway/main.go

# ===========================================
# 运行阶段
# ===========================================

FROM alpine:latest

# 安装必要的包
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 创建应用用户
RUN addgroup -g 1001 app && \
    adduser -u 1001 -G app -s /bin/sh -D app

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/gateway .
COPY --from=builder /app/config/example.yaml ./config.yaml

# 创建日志目录
RUN mkdir -p logs && chown -R app:app /app

# 切换到应用用户
USER app

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

# 暴露端口
EXPOSE 8080 9090

# 启动应用
CMD ["./gateway", "-config", "config.yaml"]
```

**Docker Compose 配置:**

```yaml
# docker-compose.yml
version: '3.8'

services:
  gateway:
    build: .
    ports:
      - "8080:8080"   # HTTP
      - "9090:9090"   # gRPC
    environment:
      - GATEWAY_ENV=production
      - MYSQL_HOST=mysql
      - REDIS_ADDR=redis:6379
    volumes:
      - ./logs:/app/logs
      - ./config/production.yaml:/app/config.yaml:ro
    depends_on:
      - mysql
      - redis
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: gateway123
      MYSQL_DATABASE: gateway_db
    ports:
      - "3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    restart: unless-stopped

volumes:
  mysql_data:
  redis_data:
```

</details>

### ☸️ Kubernetes 部署

<details>
<summary>🎛️ 查看 K8s 完整配置</summary>

**Deployment 配置:**

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: go-rpc-gateway
  labels:
    app: gateway
    version: v1.0.0
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
  selector:
    matchLabels:
      app: gateway
  template:
    metadata:
      labels:
        app: gateway
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      containers:
      - name: gateway
        image: your-registry/go-rpc-gateway:latest
        ports:
        - name: http
          containerPort: 8080
          protocol: TCP
        - name: grpc
          containerPort: 9090
          protocol: TCP
        env:
        - name: GATEWAY_ENV
          value: "production"
        - name: MYSQL_HOST
          value: "mysql-service"
        - name: REDIS_ADDR
          value: "redis-service:6379"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          limits:
            cpu: 500m
            memory: 512Mi
          requests:
            cpu: 100m
            memory: 128Mi
        volumeMounts:
        - name: config
          mountPath: /app/config.yaml
          subPath: config.yaml
          readOnly: true
        - name: logs
          mountPath: /app/logs
      volumes:
      - name: config
        configMap:
          name: gateway-config
      - name: logs
        emptyDir: {}

---
# Service 配置
apiVersion: v1
kind: Service
metadata:
  name: gateway-service
  labels:
    app: gateway
spec:
  type: ClusterIP
  ports:
  - name: http
    port: 8080
    targetPort: 8080
  - name: grpc
    port: 9090
    targetPort: 9090
  selector:
    app: gateway

---
# Ingress 配置
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gateway-ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  tls:
  - hosts:
    - gateway.example.com
    secretName: gateway-tls
  rules:
  - host: gateway.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: gateway-service
            port:
              number: 8080

---
# ConfigMap
apiVersion: v1
kind: ConfigMap
metadata:
  name: gateway-config
data:
  config.yaml: |
    gateway:
      name: go-rpc-gateway
      environment: production
      debug: false
    # ... 其他配置
```

</details>

### 🎯 生产环境最佳实践

<table>
<tr>
<th width="25%">🔧 性能优化</th>
<th width="25%">🛡️ 安全加固</th>
<th width="25%">📊 监控告警</th>
<th width="25%">🔄 高可用</th>
</tr>
<tr>
<td>

- 连接池调优
- 内存/CPU限制
- 垃圾回收优化
- 缓存策略

</td>
<td>

- HTTPS 强制
- 安全头设置
- 访问控制
- 敏感信息保护

</td>
<td>

- Prometheus 指标
- 日志聚合
- 告警规则
- 性能基线

</td>
<td>

- 多实例部署
- 负载均衡
- 健康检查
- 故障转移

</td>
</tr>
</table>

## 📚 完整示例

### 🎯 快速体验项目

```bash
# 1. 克隆项目
git clone https://github.com/kamalyes/go-rpc-gateway.git
cd go-rpc-gateway

# 2. 查看示例
ls examples/
# basic/          - 基础示例，零配置启动
# quickstart/     - 快速开始，5分钟上手
# with-config/    - 配置文件示例
# with-logs/      - 日志系统示例

# 3. 运行基础示例
cd examples/basic
go run main.go

# 4. 测试服务
curl http://localhost:8080/health
```

### 🎨 业务集成示例

<details>
<summary>💼 查看完整业务代码示例</summary>

```go
// examples/business/main.go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/kamalyes/go-rpc-gateway/internal/server"
    "github.com/kamalyes/go-rpc-gateway/internal/config"
    
    // 引入你的业务 proto
    pb "your-project/api/proto/user/v1"
)

// UserService 实现你的业务逻辑
type UserService struct {
    pb.UnimplementedUserServiceServer
    // 注入数据库、缓存等依赖
}

func (s *UserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
    // 实现业务逻辑
    return &pb.GetUserResponse{
        User: &pb.User{
            Id:    req.Id,
            Name:  "示例用户",
            Email: "user@example.com",
        },
    }, nil
}

func main() {
    // 1. 创建配置管理器
    configManager := config.NewConfigManager("config.yaml")
    
    // 2. 创建服务器
    srv, err := server.NewServerWithConfigManager(configManager)
    if err != nil {
        log.Fatal("创建服务器失败:", err)
    }

    // 3. 注册 gRPC 服务
    userService := &UserService{}
    srv.RegisterGRPCService(func(s *grpc.Server) {
        pb.RegisterUserServiceServer(s, userService)
    })

    // 4. 注册 HTTP 网关
    ctx := context.Background()
    err = srv.RegisterHTTPHandler(ctx, pb.RegisterUserServiceHandlerFromEndpoint)
    if err != nil {
        log.Fatal("注册HTTP处理器失败:", err)
    }

    // 5. 启动服务器
    go func() {
        log.Println("🚀 启动 Gateway 服务器...")
        if err := srv.Start(); err != nil {
            log.Fatal("启动失败:", err)
        }
    }()

    // 6. 优雅关闭
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Println("🛑 正在关闭服务器...")
    if err := srv.Shutdown(); err != nil {
        log.Printf("关闭服务器失败: %v", err)
    }
    log.Println("✅ 服务器已关闭")
}
```

</details>

### 🔗 相关链接

| 类型 | 链接 | 描述 |
|------|------|------|
| **📖 文档** | [完整文档](docs/) | 详细的使用文档和最佳实践 |
| **🎯 示例** | [examples/](examples/) | 各种场景的完整示例代码 |
| **🐛 问题反馈** | [GitHub Issues](https://github.com/kamalyes/go-rpc-gateway/issues) | Bug 报告和功能请求 |
| **💬 讨论区** | [GitHub Discussions](https://github.com/kamalyes/go-rpc-gateway/discussions) | 技术讨论和经验分享 |
| **📋 更新日志** | [CHANGELOG.md](CHANGELOG.md) | 版本更新记录 |

## 🤝 贡献指南

我们欢迎所有形式的贡献！请查看我们的 [贡献指南](CONTRIBUTING.md) 了解如何参与。

### 🏗️ 开发环境设置

```bash
# 1. Fork 项目并克隆
git clone https://github.com/your-username/go-rpc-gateway.git
cd go-rpc-gateway

# 2. 安装依赖
go mod download

# 3. 运行测试
go test ./...

# 4. 构建项目
./build.sh

# 5. 运行示例
./bin/gateway -config examples/config.yaml
```

### ✅ 提交规范

我们使用 [Conventional Commits](https://conventionalcommits.org/) 规范：

```
feat: 添加新的中间件支持
fix: 修复配置热重载问题
docs: 更新 README 文档
style: 代码格式化
refactor: 重构服务器启动逻辑
test: 添加中间件单元测试
chore: 更新依赖版本
```

## 📄 许可证

本项目采用 [MIT 许可证](LICENSE)，您可以自由使用、修改和分发。

---

## 🙏 致谢

感谢以下优秀的开源项目：

<table>
<tr>
<td align="center">
  <a href="https://github.com/kamalyes/go-config">
    <img src="https://via.placeholder.com/64x64.png?text=CONFIG" width="64" height="64">
    <br>
    <strong>go-config</strong>
  </a>
  <br>
  <sub>统一配置管理</sub>
</td>
<td align="center">
  <a href="https://github.com/kamalyes/go-sqlbuilder">
    <img src="https://via.placeholder.com/64x64.png?text=SQL" width="64" height="64">
    <br>
    <strong>go-sqlbuilder</strong>
  </a>
  <br>
  <sub>SQL构建器</sub>
</td>
<td align="center">
  <a href="https://github.com/kamalyes/go-wsc">
    <img src="https://via.placeholder.com/64x64.png?text=WSC" width="64" height="64">
    <br>
    <strong>go-wsc</strong>
  </a>
  <br>
  <sub>WebSocket客户端</sub>
</td>
<td align="center">
  <a href="https://github.com/grpc-ecosystem/grpc-gateway">
    <img src="https://via.placeholder.com/64x64.png?text=gRPC" width="64" height="64">
    <br>
    <strong>grpc-gateway</strong>
  </a>
  <br>
  <sub>gRPC 网关</sub>
</td>
<td align="center">
  <a href="https://github.com/prometheus/client_golang">
    <img src="https://via.placeholder.com/64x64.png?text=PROM" width="64" height="64">
    <br>
    <strong>Prometheus</strong>
  </a>
  <br>
  <sub>监控指标</sub>
</td>
</tr>
</table>

---

<div align="center">

**⭐ 如果这个项目对您有帮助，请给我们一个 Star！**

[![Star History Chart](https://api.star-history.com/svg?repos=kamalyes/go-rpc-gateway&type=Date)](https://star-history.com/#kamalyes/go-rpc-gateway&Date)

</div>
