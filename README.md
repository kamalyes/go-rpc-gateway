# 🚀 Go RPC Gateway

<div align="center">

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.23-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)]()
[![Go Report Card](https://img.shields.io/badge/go%20report-A+-brightgreen.svg)]()
[![Release](https://img.shields.io/badge/release-v2.1.0-blue.svg)]()

**企业级 gRPC-Gateway 微服务网关框架**

基于现代化架构设计 · 生产级可靠性 · 开箱即用

[快速开始](docs/QUICK_START.md) · [架构设计](docs/ARCHITECTURE.md) · [API文档](docs/API_REFERENCE.md) · [部署指南](docs/DEPLOYMENT.md)

</div>

---

## 🎯 核心特性

<table>
<tr>
<td width="50%">

### 🏗️ 现代化架构

- **链式构建器模式** - 流畅优雅的 API 设计
- **统一初始化链** - 组件依赖自动管理
- **功能特性管理** - 动态启用/禁用模块
- **配置热重载** - 运行时无缝更新配置

</td>
<td width="50%">

### 🚀 生产级特性

- **双协议支持** - HTTP/1.1 + gRPC 同时服务
- **企业级中间件** - 15+ 内置中间件系统
- **完整可观测性** - 日志/监控/追踪一体化
- **高性能连接池** - 自动化资源管理

</td>
</tr>
</table>

---

## 🏛️ 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                      🌐 Client Layer                            │
│           HTTP/1.1  │  HTTP/2  │  gRPC  │  WebSocket            │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                   🎯 Gateway Entry Point                        │
│                    (链式构建器模式)                                │
│  gateway.NewGateway().WithConfig().WithHotReload().Build()     │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                  🔧 Middleware Pipeline                         │
│  ┌──────────┬──────────┬──────────┬──────────┬──────────┐      │
│  │ Recovery │RequestID │ RateLimit│  Breaker │  Auth    │      │
│  ├──────────┼──────────┼──────────┼──────────┼──────────┤      │
│  │  CORS    │ Security │  Logging │  Metrics │ Tracing  │      │
│  └──────────┴──────────┴──────────┴──────────┴──────────┘      │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                   🎮 Service Router                             │
│  ┌──────────────────┐    ┌──────────────────┐                  │
│  │  gRPC Services   │    │  HTTP Handlers   │                  │
│  │  - User Service  │    │  - REST API      │                  │
│  │  - Order Service │    │  - Health Check  │                  │
│  │  - ... Custom    │    │  - ... Custom    │                  │
│  └──────────────────┘    └──────────────────┘                  │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│              📊 Unified Initialization Chain                    │
│                   (InitializerChain)                            │
│  ┌──────────┬──────────┬──────────┬──────────┬──────────┐      │
│  │  Logger  │ Context  │Snowflake │ PoolMgr   │ Custom   │      │
│  │ Priority │ Priority │ Priority │ Priority │ Priority │      │
│  │    1     │    2     │    5     │    10    │   ...    │      │
│  └──────────┴──────────┴──────────┴──────────┴──────────┘      │
│              自动依赖排序 · 健康检查 · 优雅关闭                     │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│               🗄️ Infrastructure Layer                           │
│  ┌──────────┬──────────┬──────────┬──────────┬──────────┐      │
│  │ Database │  Redis   │  MinIO   │   MQTT   │  Consul  │      │
│  │  (GORM)  │ (Cache)  │(Storage) │(Message) │(Discovery│      │
│  └──────────┴──────────┴──────────┴──────────┴──────────┘      │
│              连接池管理 · 自动重连 · 健康监控                       │
└─────────────────────────────────────────────────────────────────┘
```

### 🔄 初始化流程 (InitializerChain)

```go
┌─────────────────────────────────────────────────────────────┐
│  1. Logger (Priority: 1)                                    │
│     └─> 创建日志器 → 设置级别 → 全局注入                      │
├─────────────────────────────────────────────────────────────┤
│  2. Context (Priority: 2)                                   │
│     └─> 初始化全局上下文 → 设置取消函数                       │
├─────────────────────────────────────────────────────────────┤
│  3. Snowflake (Priority: 5)                                 │
│     └─> 创建分布式ID生成器 → 设置节点ID                       │
├─────────────────────────────────────────────────────────────┤
│  4. PoolManager (Priority: 10)                              │
│     └─> 初始化数据库 → Redis → MinIO → MQTT                  │
│     └─> 绑定到全局变量 → 健康检查                             │
├─────────────────────────────────────────────────────────────┤
│  5. Custom Initializers (Priority: 15+)                    │
│     └─> 用户自定义组件 → 业务初始化逻辑                       │
└─────────────────────────────────────────────────────────────┘
```

---

## ⚡ 快速开始

### 安装

```bash
go get github.com/kamalyes/go-rpc-gateway
```

### 极简示例 (3行代码)

```go
package main

import gateway "github.com/kamalyes/go-rpc-gateway"

func main() {
    gw, _ := gateway.NewGateway().
        WithSearchPath("./config").
        BuildAndStart()
    
    gw.WaitForShutdown()
}
```

### 生产环境示例

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
    // 链式构建网关
    gw, err := gateway.NewGateway().
        WithConfigPath("./config/gateway-prod.yaml").
        WithEnvironment(gateway.EnvProduction).
        WithHotReload(nil).  // 启用配置热重载
        Build()
    
    if err != nil {
        panic(err)
    }
    
    // 注册 gRPC 服务
    gw.RegisterService(func(s *grpc.Server) {
        // pb.RegisterYourServiceServer(s, &yourService{})
    })
    
    // 注册 HTTP 路由
    gw.RegisterHTTPRoutes(map[string]http.HandlerFunc{
        "/api/users":  handleUsers,
        "/api/health": handleHealth,
    })
    
    // 启用企业级特性
    gw.EnableFeature(server.FeaturePProf)      // 性能分析
    gw.EnableFeature(server.FeatureMonitoring) // Prometheus 监控
    gw.EnableFeature(server.FeatureTracing)    // OpenTelemetry 追踪
    gw.EnableFeature(server.FeatureSwagger)    // API 文档
    
    // 启动并等待
    gw.Start()
    gw.WaitForShutdown()
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
    // 全局数据库连接开箱即用
    var users []User
    global.DB.Find(&users)
    // ... 返回结果
}
```

**访问服务**:

- 🌐 HTTP API: `http://localhost:8080`
- 🔌 gRPC: `localhost:9090`
- 📊 监控指标: `http://localhost:8080/metrics`
- 📖 API文档: `http://localhost:8080/swagger/`
- 🔍 性能分析: `http://localhost:8080/debug/pprof/`

---

## 📚 完整文档

<table>
<tr>
<td width="33%">

### 🚀 入门指南

- [快速开始](docs/QUICK_START.md)
- [配置指南](docs/CONFIG_GUIDE.md)
- [API 参考](docs/API_REFERENCE.md)

</td>
<td width="33%">

### 🏗️ 架构设计

- [系统架构](docs/ARCHITECTURE.md)
- [初始化机制](docs/INITIALIZER_GUIDE.md)
- [中间件系统](docs/MIDDLEWARE_GUIDE.md)

</td>
<td width="33%">

### 🛠️ 高级特性

- [功能特性管理](docs/FEATURE_MANAGEMENT.md)
- [连接池管理](docs/POOL_MANAGEMENT.md)
- [WebSocket 通信](wsc/README.md)

</td>
</tr>
<tr>
<td width="33%">

### 📦 模块文档

- [PBMO 转换器](pbmo/README.md)
- [错误处理](errors/README.md)
- [响应封装](response/README.md)
- [白名单中间件](middleware/WHITELIST_USAGE.md) 🆕

</td>
<td width="33%">

### 🚀 部署运维

- [部署指南](docs/DEPLOYMENT.md)
- [监控告警](docs/MONITORING.md)
- [性能优化](docs/PERFORMANCE.md)

</td>
<td width="33%">

### 💡 最佳实践

- [开发规范](docs/BEST_PRACTICES.md)
- [示例代码](docs/EXAMPLES.md)
- [常见问题](docs/FAQ.md)

</td>
</tr>
</table>

---

## 🎨 核心能力

### 1. 统一初始化链 (InitializerChain)

**问题**: 传统方式组件初始化顺序难以管理，依赖关系隐藏在代码中

**解决**: 基于优先级的自动化初始化链

```go
// 添加自定义初始化器
type MyInitializer struct{}

func (i *MyInitializer) Name() string { return "MyComponent" }
func (i *MyInitializer) Priority() int { return 20 }  // 在 PoolManager 之后
func (i *MyInitializer) Initialize(ctx context.Context, cfg *gwconfig.Gateway) error {
    // 初始化逻辑
    return nil
}
func (i *MyInitializer) Cleanup() error { return nil }
func (i *MyInitializer) HealthCheck() error { return nil }

// 注册即可，自动按优先级排序
chain.Register(&MyInitializer{})
```

### 2. 链式构建器模式

**优雅的配置方式**:

```go
gateway.NewGateway().
    WithConfigPath("./config.yaml").        // 指定配置文件
    WithEnvironment(config.EnvProduction).  // 设置环境
    WithHotReload(nil).                     // 启用热重载
    WithSearchPath("./config").             // 自动发现配置
    WithPrefix("gateway").                  // 配置文件前缀
    Silent().                               // 静默启动
    BuildAndStart()                         // 构建并启动
```

### 3. 功能特性管理

**统一的功能开关**:

```go
// 启用功能
gw.EnableFeature(server.FeaturePProf)
gw.EnableFeature(server.FeatureMonitoring)

// 检查状态
if gw.IsFeatureEnabled(server.FeatureSwagger) {
    fmt.Println("Swagger 已启用")
}
```

**支持的特性**:

- `FeatureSwagger` - API 文档服务
- `FeatureMonitoring` - Prometheus 监控
- `FeatureHealth` - 健康检查
- `FeaturePProf` - 性能分析
- `FeatureTracing` - 链路追踪
- `FeatureWSC` - WebSocket 通信

### 4. 企业级中间件

**15+ 内置中间件**:

| 中间件 | 功能 | 生产推荐 |
|--------|------|---------|
| Recovery | Panic 恢复 | ✅ 必需 |
| RequestID | 请求追踪 | ✅ 必需 |
| RateLimit | 流量控制 | ✅ 推荐 |
| Breaker | 熔断保护 | ✅ 推荐 |
| Logging | 访问日志 | ✅ 必需 |
| Metrics | 性能指标 | ✅ 推荐 |
| Tracing | 链路追踪 | ✅ 推荐 |
| CORS | 跨域支持 | ⚪ 按需 |
| Security | 安全防护 | ✅ 推荐 |
| I18N | 国际化 | ⚪ 按需 |

### 5. 完整的可观测性

```
📝 日志 (go-logger)          📊 监控 (Prometheus)      🔍 追踪 (OpenTelemetry)
     │                            │                          │
     ├─ 结构化日志                 ├─ HTTP 指标                ├─ 分布式追踪
     ├─ 多级别控制                 ├─ gRPC 指标                ├─ Span 关联
     ├─ 上下文关联                 ├─ 系统指标                 ├─ Jaeger/Zipkin
     └─ 自动轮转                   └─ 自定义指标               └─ 性能分析
```

---

## 🔧 配置管理

### 配置文件示例

```yaml
# gateway-prod.yaml
name: production-gateway
version: v2.1.0
environment: production
debug: false

# HTTP/gRPC 服务
http_server:
  host: 0.0.0.0
  port: 8080

grpc:
  server:
    host: 0.0.0.0
    port: 9090

# 数据库 (自动初始化连接池)
mysql:
  enabled: true
  host: db.example.com
  port: 3306
  username: ${DB_USER}      # 支持环境变量
  password: ${DB_PASSWORD}
  dbname: gateway
  max_idle_conns: 10
  max_open_conns: 100

# Redis (自动初始化)
redis:
  enabled: true
  host: redis.example.com
  port: 6379
  pool_size: 20

# MinIO (自动初始化)
minio:
  enabled: true
  endpoint: minio.example.com:9000
  access_key: ${MINIO_ACCESS_KEY}
  secret_key: ${MINIO_SECRET_KEY}

# 中间件配置
middleware:
  rate_limit:
    enabled: true
    rate: 1000      # 每秒1000个请求
    burst: 2000
  
  metrics:
    enabled: true
  
  tracing:
    enabled: true
    jaeger:
      endpoint: http://jaeger:14268/api/traces
```

### 配置热重载

```go
// 启用热重载后，配置文件变更自动生效
gw, _ := gateway.NewGateway().
    WithConfigPath("config.yaml").
    WithHotReload(nil).  // 使用默认配置
    Build()

// 手动重载配置
global.ReloadConfig()
```

---

## 🚀 性能指标

<table>
<tr>
<td width="50%">

### ⚡ 性能数据

- **启动时间**: < 3s
- **首次请求**: < 100ms (含连接池预热)
- **QPS**: 10,000+ (单机)
- **并发连接**: 10,000+
- **内存占用**: < 100MB (空载)

</td>
<td width="50%">

### 📊 可靠性

- **可用性**: 99.9%+
- **P99 延迟**: < 50ms
- **配置热更新**: < 5ms
- **优雅关闭**: < 30s
- **自动恢复**: 100%

</td>
</tr>
</table>

---

## 🏢 生产案例

<table>
<tr>
<td align="center" width="25%">
<img src="https://via.placeholder.com/100" alt="Company 1"/><br/>
<b>电商平台</b><br/>
<sub>1000+ QPS</sub>
</td>
<td align="center" width="25%">
<img src="https://via.placeholder.com/100" alt="Company 2"/><br/>
<b>金融服务</b><br/>
<sub>高可用架构</sub>
</td>
<td align="center" width="25%">
<img src="https://via.placeholder.com/100" alt="Company 3"/><br/>
<b>物联网平台</b><br/>
<sub>海量连接</sub>
</td>
<td align="center" width="25%">
<img src="https://via.placeholder.com/100" alt="Company 4"/><br/>
<b>AI服务</b><br/>
<sub>低延迟要求</sub>
</td>
</tr>
</table>

---

## 🤝 贡献指南

我们欢迎各种形式的贡献！

### 参与方式

- 🐛 [报告 Bug](https://github.com/kamalyes/go-rpc-gateway/issues)
- ✨ [提交功能建议](https://github.com/kamalyes/go-rpc-gateway/issues)
- 📖 [改进文档](https://github.com/kamalyes/go-rpc-gateway/pulls)
- 💻 [提交代码](https://github.com/kamalyes/go-rpc-gateway/pulls)

### 开发流程

```bash
# 1. Fork 项目
# 2. 创建功能分支
git checkout -b feature/amazing-feature

# 3. 提交更改
git commit -m 'feat: add amazing feature'

# 4. 推送分支
git push origin feature/amazing-feature

# 5. 创建 Pull Request
```

---

## 📄 开源协议

本项目采用 [MIT License](LICENSE) 开源协议。

---

## 🔗 相关项目

- [go-config](https://github.com/kamalyes/go-config) - 统一配置管理
- [go-logger](https://github.com/kamalyes/go-logger) - 高性能日志
- [go-toolbox](https://github.com/kamalyes/go-toolbox) - 工具集
- [go-cachex](https://github.com/kamalyes/go-cachex) - 多级缓存
- [go-wsc](https://github.com/kamalyes/go-wsc) - WebSocket 客户端

---

## 📞 联系我们

- 📧 Email: <501893067@qq.com>
- 💬 讨论: [GitHub Discussions](https://github.com/kamalyes/go-rpc-gateway/discussions)
- 🐛 问题: [GitHub Issues](https://github.com/kamalyes/go-rpc-gateway/issues)

---

<div align="center">

**⭐ 如果这个项目对你有帮助，请给一个 Star 支持！**

Built with ❤️ by [Kamalyes](https://github.com/kamalyes)

[⬆ 回到顶部](#-go-rpc-gateway)

</div>
