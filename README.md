<div align="center">

# 🚀 Go RPC Gateway

### 新一代企业级微服务网关框架 · 高性能 · 高可用 · 开箱即用

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.23-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)]()
[![Go Report Card](https://img.shields.io/badge/go%20report-A+-brightgreen.svg)]()
[![Release](https://img.shields.io/badge/release-v2.1.0-blue.svg)
</div>

---

## ✨ 为什么选择 Go RPC Gateway？

| 🎯 特性 | 💡 能力 | 📊 指标 | ✅ 优势 |
|---------|---------|---------|---------|
| **⚡ 极致性能** | 高并发处理<br/>快速启动<br/>低延迟响应<br/>内存高效 | **10,000+ QPS**<br/>启动 < 3s<br/>P99 < 50ms<br/>内存 < 100MB | 单机承载海量请求<br/>秒级快速部署<br/>用户体验流畅<br/>成本显著降低 |
| **🛡️ 生产可靠** | 高可用保障<br/>自动容错<br/>优雅关闭<br/>热更新 | **99.9%+ 可用性**<br/>熔断/限流<br/>< 30s 关闭<br/>零停机 | 业务持续稳定<br/>异常自动恢复<br/>平滑版本升级<br/>用户无感知 |
| **🎯 开箱即用** | 零配置启动<br/>链式 API<br/>自动管理<br/>丰富组件 | **3 行代码启动**<br/>流畅调用<br/>依赖自动化<br/>15+ 中间件 | 快速上手开发<br/>代码简洁优雅<br/>降低维护成本<br/>功能开箱可用 |
| **🏗️ 架构先进** | 分层设计<br/>优先级初始化<br/>功能插件化<br/>配置驱动 | **6 层架构**<br/>4 级优先级<br/>动态启用<br/>统一管理 | 职责清晰易懂<br/>启动顺序可靠<br/>灵活按需加载<br/>配置集中控制 |
| **🔌 完整集成** | 连接池管理<br/>全局变量<br/>多协议支持<br/>可观测性 | **DB/Redis/MinIO**<br/>6 个全局变量<br/>HTTP/gRPC/WS<br/>日志/监控/追踪 | 资源自动管理<br/>访问简单直接<br/>一套代码多用<br/>问题快速定位 |
| **📦 企业特性** | 中间件体系<br/>安全防护<br/>限流熔断<br/>国际化 | **15+ 中间件**<br/>认证/授权<br/>自动保护<br/>多语言支持 | 功能开箱即用<br/>安全合规保障<br/>系统稳定运行<br/>全球化部署 |

---

## 📖 系统架构

### 完整架构视图

```mermaid
flowchart TB
    %% 客户端层
    Client["🌐 客户端"]
    
    %% Gateway 入口
    Client --> |HTTP/gRPC/WebSocket| Gateway
    
    subgraph Gateway["🚀 Gateway 网关层"]
        direction TB
        Builder["GatewayBuilder\n链式构建"]
        Core["Gateway Core\ngateway.go"]
        Builder -.构建.-> Core
    end
    
    %% Server 层
    Gateway --> Server
    
    subgraph Server["🎯 Server 服务层"]
        direction TB
        
        subgraph Protocol["协议处理"]
            GRPC["gRPC Server\n:9090"]
            HTTP["HTTP Server\n:8080"]
            WS["WebSocket\n可选"]
        end
        
        subgraph Router["路由注册"]
            RPCReg["RPC 路由\nRegisterAllGRPCServices"]
            HTTPReg["HTTP 路由\nRegisterHTTPRoute"]
            GWMux["gRPC-Gateway Mux"]
            HTTPMux["HTTP Mux"]
        end
        
        GRPC --> RPCReg
        HTTP --> HTTPReg
        RPCReg --> GWMux
        HTTPReg --> HTTPMux
    end
    
    %% 中间件层
    Server --> MW
    
    subgraph MW["🔧 Middleware 中间件层\n✨ 可配置选择"]
        direction TB
        MWManager["MiddlewareManager"]
        
        subgraph ModeSelect["模式选择"]
            DevMode["Development 模式\nGetDevelopmentMiddlewares"]
            ProdMode["Production 模式\nGetDefaultMiddlewares"]
        end
        
        subgraph Core_MW["核心中间件"]
            Recovery["Recovery"]
            RequestID["RequestID"]
            Logging["Logging"]
        end
        
        subgraph Flow_MW["流量控制"]
            RateLimit["RateLimit"]
            Breaker["Breaker"]
        end
        
        subgraph Obs_MW["可观测性"]
            Metrics["Metrics"]
            Tracing["Tracing"]
        end
        
        MWManager --> ModeSelect
        ModeSelect --> Core_MW
        ModeSelect --> Flow_MW
        ModeSelect --> Obs_MW
    end
    
    %% 功能特性层
    Server --> Features
    
    subgraph Features["✨ Features 功能层"]
        direction LR
        FManager["FeatureManager"]
        
        subgraph DevTools["开发工具"]
            Swagger["Swagger UI"]
            PProf["PProf"]
        end
        
        subgraph Monitor["监控健康"]
            Health["Health"]
            Monitoring["Monitoring"]
        end
        
        FManager --> DevTools
        FManager --> Monitor
    end
    
    %% 初始化系统
    Gateway -.初始化.-> Init
    
    subgraph Init["📊 初始化系统"]
        direction TB
        Chain["InitializerChain\n优先级管理"]
        
        L1["① Logger\nP:1"]
        L2["② Context\nP:2"]
        L3["③ Snowflake\nP:5"]
        L4["④ PoolManager\nP:10"]
        
        Chain --> L1 --> L2 --> L3 --> L4
    end
    
    %% 连接池管理
    Init --> Pool
    
    subgraph Pool["🔌 连接池管理"]
        direction TB
        PManager["PoolManager"]
        
        subgraph Pools["连接池"]
            DB_Pool["Database\nGORM"]
            Redis_Pool["Redis\ngo-redis"]
            MinIO_Pool["MinIO\nS3"]
            MQTT_Pool["MQTT\n可选"]
        end
        
        PManager --> Pools
    end
    
    %% 配置管理
    Gateway -.配置.-> Config
    
    subgraph Config["⚙️ 配置管理"]
        direction TB
        CManager["ConfigManager"]
        GWConfig["Gateway Config"]
        HotReload["Hot Reload\n可选"]
        
        CManager --> GWConfig
        CManager -.监听.-> HotReload
    end
    
    %% 全局变量
    Pool --> Global
    Config --> Global
    
    subgraph Global["🌍 全局变量"]
        direction LR
        G_LOGGER["LOGGER"]
        G_CTX["CTX"]
        G_DB["DB"]
        G_REDIS["REDIS"]
        G_MinIO["MinIO"]
        G_Node["Node"]
    end
    
    %% 外部系统
    Pool --> External
    
    subgraph External["💾 外部系统"]
        direction LR
        Database[("MySQL\nPostgreSQL")]
        Cache[("Redis\nCluster")]
        Storage[("MinIO\nS3")]
        Queue[("MQTT\nKafka")]
    end
    
    %% 样式定义
    classDef gatewayStyle fill:#e3f2fd,stroke:#1976d2,stroke-width:3px
    classDef serverStyle fill:#fff3e0,stroke:#f57f17,stroke-width:3px
    classDef middlewareStyle fill:#f3e5f5,stroke:#7b1fa2,stroke-width:3px
    classDef bizStyle fill:#e1f5fe,stroke:#0277bd,stroke-width:3px
    classDef featureStyle fill:#e8f5e9,stroke:#388e3c,stroke-width:3px
    classDef initStyle fill:#fff9c4,stroke:#fbc02d,stroke-width:3px
    classDef poolStyle fill:#fce4ec,stroke:#c2185b,stroke-width:3px
    classDef configStyle fill:#e0f2f1,stroke:#00796b,stroke-width:3px
    classDef globalStyle fill:#f1f8e9,stroke:#689f38,stroke-width:3px
    classDef externalStyle fill:#fafafa,stroke:#9e9e9e,stroke-width:2px
    
    class Gateway gatewayStyle
    class Server serverStyle
    class MW middlewareStyle
    class BizInjection bizStyle
    class Features featureStyle
    class Init initStyle
    class Pool poolStyle
    class Config configStyle
    class Global globalStyle
    class External externalStyle
```

### 初始化流程

```mermaid
sequenceDiagram
    participant User as 用户代码
    participant GB as GatewayBuilder
    participant CM as ConfigManager
    participant IC as InitializerChain
    participant S as Server
    participant PM as PoolManager
    participant G as Global
    
    User->>+GB: NewGateway()
    Note over GB: 创建构建器
    
    User->>GB: WithConfigPath("./config.yaml")
    User->>GB: WithEnvironment(Production)
    User->>GB: Build()
    
    GB->>+CM: NewManager(config).BuildAndStart()
    CM->>CM: 加载配置文件
    CM->>CM: 应用默认值
    CM->>G: global.CONFIG_MANAGER = manager
    CM->>G: global.GATEWAY = config
    CM->>CM: 注册热更新回调
    CM-->>-GB: 返回 ConfigManager
    
    GB->>GB: initializeGlobalState()
    GB->>G: global.CTX, CANCEL = context.WithCancel()
    GB->>GB: registerGlobalConfigCallbacks()
    
    GB->>+IC: GetDefaultInitializerChain()
    Note over IC: 注册初始化器<br/>按优先级排序
    IC-->>-GB: 返回 InitializerChain
    
    GB->>+IC: InitializeAll(ctx, config)
    
    IC->>IC: ① LoggerInitializer (P:1)
    IC->>IC: EnsureLoggerInitialized()
    IC->>IC: CreateSimpleLogger(level)
    IC->>G: global.LOGGER = logger
    IC->>G: global.LOG = logger
    Note over IC,G: ✅ 日志器初始化完成
    
    IC->>IC: ② ContextInitializer (P:2)
    IC->>G: global.CTX, CANCEL = WithCancel()
    Note over IC,G: ✅ 上下文初始化完成
    
    IC->>IC: ③ SnowflakeInitializer (P:5)
    IC->>IC: snowflake.NewNode(nodeID)
    IC->>G: global.Node = node
    Note over IC,G: ✅ Snowflake初始化完成
    
    IC->>+PM: ④ PoolManagerInitializer (P:10)
    PM->>PM: NewManager(LOGGER)
    PM->>PM: Initialize(ctx, config)
    
    alt 配置启用 Database
        PM->>PM: 初始化 Database (GORM)
        PM->>G: global.DB = db
    end
    
    alt 配置启用 Redis
        PM->>PM: 初始化 Redis (go-redis)
        PM->>G: global.REDIS = redis
    end
    
    alt 配置启用 MinIO
        PM->>PM: 初始化 MinIO (S3)
        PM->>G: global.MinIO = minio
    end
    
    PM->>G: global.POOL_MANAGER = manager
    PM-->>-IC: 返回成功
    Note over IC,PM: ✅ 连接池管理器初始化完成
    
    IC-->>-GB: 初始化链执行完成
    
    GB->>+S: NewServer()
    S->>S: initCore()
    S->>S: initMiddleware()
    S->>S: initGRPCServer()
    S->>S: initHTTPGateway()
    S-->>-GB: 返回 Server
    
    GB->>GB: 创建 Gateway 实例
    GB->>GB: RegisterConfigCallbacks()
    GB-->>-User: 返回 Gateway
    
    User->>+S: gateway.Start()
    S->>S: startGRPCServer()
    Note over S: 监听 :9090
    S->>S: startHTTPServer()
    Note over S: 监听 :8080
    
    opt WebSocket 启用
        S->>S: startWebSocketService()
    end
    
    S-->>-User: ✅ 启动完成
```

---

### 📊 完整的可观测性

```mermaid
graph LR
    subgraph Logging["📝 日志系统"]
        L1[结构化日志]
        L2[多级别控制]
        L3[上下文关联]
    end
    
    subgraph Monitoring["📊 监控告警"]
        M1[Prometheus]
        M2[自定义指标]
        M3[实时告警]
    end
    
    subgraph Tracing["🔍 链路追踪"]
        T1[分布式追踪]
        T2[Jaeger/Zipkin]
        T3[性能分析]
    end
    
    style Logging fill:#e3f2fd,stroke:#1976d2,stroke-width:2px
    style Monitoring fill:#fff9c4,stroke:#f57f17,stroke-width:2px
    style Tracing fill:#e8f5e9,stroke:#388e3c,stroke-width:2px
```

## 📚 文档

完整文档请访问 [docs/](./docs/) 目录：

| 分类 | 文档 | 说明 |
|------|------|------|
| **入门** | [快速入门](./docs/QUICKSTART.md) | 5 分钟上手 |
| | [Gateway 构建器](./docs/GATEWAY-BUILDER.md) | 链式构建 API、配置发现、热更新 |
| | [服务注册](./docs/SERVICE-REGISTRATION.md) | gRPC + HTTP 双协议注册 |
| **核心** | [gRPC 客户端](./docs/GRPC-CLIENT.md) | InitClient 泛型、健康检查、负载均衡 |
| | [请求上下文](./docs/REQUEST-CONTEXT.md) | HTTP → gRPC → Service 全链路传递 |
| | [中间件系统](./docs/MIDDLEWARE.md) | 15+ 中间件：CORS、限流、熔断、签名等 |
| | [连接池管理](./docs/CONNECTION-POOL.md) | DB/Redis/MinIO/ClickHouse/NATS 统一管理 |
| | [全局变量与初始化器](./docs/GLOBAL.md) | InitializerChain、ID 生成器 |
| | [Server 内部机制](./docs/SERVER.md) | 生命周期、热重载、Swagger |
| **工具** | [错误体系](./docs/ERRORS.md) | ErrorCode、AppError、三态映射 |
| | [HTTP 响应工具](./docs/RESPONSE.md) | 统一 JSON 响应写入 |
| | [熔断器](./docs/BREAKER.md) | 断路器状态机、管理器 |

## 🤝 贡献与支持

### 参与贡献

我们欢迎所有形式的贡献！

```bash
# Fork 项目并创建功能分支
git checkout -b feature/amazing-feature

# 提交更改
git commit -m 'feat: add amazing feature'

# 推送并创建 Pull Request
git push origin feature/amazing-feature
```

- 🐛 [报告 Bug](https://github.com/kamalyes/go-rpc-gateway/issues)
- ✨ [功能建议](https://github.com/kamalyes/go-rpc-gateway/issues)
- 📖 [改进文档](https://github.com/kamalyes/go-rpc-gateway/pulls)
- 💻 [提交代码](https://github.com/kamalyes/go-rpc-gateway/pulls)

### 相关项目

- [go-config](https://github.com/kamalyes/go-config) - 统一配置管理
- [go-logger](https://github.com/kamalyes/go-logger) - 高性能日志
- [go-toolbox](https://github.com/kamalyes/go-toolbox) - 工具集
- [go-cachex](https://github.com/kamalyes/go-cachex) - 多级缓存
- [go-wsc](https://github.com/kamalyes/go-wsc) - WebSocket 客户端

### 联系我们

- 📧 Email: <501893067@qq.com>
- 💬 讨论: [GitHub Discussions](https://github.com/kamalyes/go-rpc-gateway/discussions)
- 🐛 问题: [GitHub Issues](https://github.com/kamalyes/go-rpc-gateway/issues)

---

## 📄 开源协议

本项目采用 [MIT License](LICENSE) 开源协议。

---

<div align="center">

**⭐ 如果这个项目对你有帮助，请给一个 Star 支持！**

Built with ❤️ by [Kamalyes](https://github.com/kamalyes)

[⬆ 回到顶部](#-go-rpc-gateway)

</div>
