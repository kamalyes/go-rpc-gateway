# 🏗️ 系统架构文档

> Go RPC Gateway 深度架构解析

---

## 📋 目录

- [架构概览](#架构概览)
- [核心组件](#核心组件)
- [初始化流程](#初始化流程)
- [请求处理流程](#请求处理流程)
- [设计模式](#设计模式)
- [扩展性设计](#扩展性设计)

---

## 🏛️ 架构概览

Go RPC Gateway 采用分层架构设计，将系统分为 6 个核心层次：

```
┌─────────────────────────────────────────────────────────────────┐
│                      🌐 客户端层 (Client Layer)                  │
│           HTTP/1.1  │  HTTP/2  │  gRPC  │  WebSocket            │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                   🎯 网关入口层 (Gateway Layer)                  │
│                    链式构建器模式 (Builder Pattern)               │
│      NewGateway() → WithConfig() → WithFeatures() → Build()    │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                  🔧 中间件层 (Middleware Layer)                  │
│  Recovery │ RequestID │ RateLimit │ Breaker │ Auth │ ...       │
│                   责任链模式 (Chain of Responsibility)            │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│                   🎮 路由层 (Routing Layer)                      │
│         gRPC Services ←→ HTTP Handlers ←→ Custom Routes        │
│                      适配器模式 (Adapter Pattern)                │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│              📊 初始化层 (Initialization Layer)                  │
│                   InitializerChain (优先级管理)                  │
│     Logger → Context → Snowflake → PoolManager → Custom        │
└────────────────────────────┬────────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────────┐
│               🗄️ 基础设施层 (Infrastructure Layer)               │
│     Database │  Redis  │  MinIO  │  MQTT  │  Consul │ ...     │
│                   连接池管理 (Pool Management)                   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 🧩 核心组件

### 1. Gateway (网关核心)

**文件**: `gateway.go`

**职责**:
- 提供链式构建器 API
- 管理网关生命周期
- 协调各组件初始化
- 处理优雅关闭

**核心结构**:

```go
type GatewayBuilder struct {
    configPath     string
    searchPath     string
    prefix         string
    environment    string
    hotReload      *config.HotReloadConfig
    silent         bool
    autoStart      bool
}

func (b *GatewayBuilder) Build() (*server.Server, error)
func (b *GatewayBuilder) BuildAndStart() (*server.Server, error)
```

**设计亮点**:
- ✅ 流畅的 API 设计 (Method Chaining)
- ✅ 参数可选 (Optional Parameters)
- ✅ 默认值智能推断
- ✅ 错误提前发现

---

### 2. Server (服务器核心)

**文件**: `server/server.go`, `server/http.go`, `server/grpc.go`

**职责**:
- 管理 HTTP/gRPC 双协议服务
- 功能特性动态控制
- 路由注册和分发
- 健康检查管理

**核心结构**:

```go
type Server struct {
    config          *gwconfig.Gateway
    httpServer      *http.Server
    grpcServer      *grpc.Server
    middlewareChain *middleware.Manager
    enabledFeatures map[FeatureType]bool
}

func (s *Server) Start() error
func (s *Server) EnableFeature(feature FeatureType)
func (s *Server) RegisterService(registrar func(*grpc.Server))
func (s *Server) RegisterHTTPRoutes(routes map[string]http.HandlerFunc)
```

**特性管理**:

```go
type FeatureType string

const (
    FeatureSwagger    FeatureType = "swagger"
    FeatureMonitoring FeatureType = "monitoring"
    FeatureHealth     FeatureType = "health"
    FeaturePProf      FeatureType = "pprof"
    FeatureTracing    FeatureType = "tracing"
    FeatureWSC        FeatureType = "wsc"
)
```

---

### 3. InitializerChain (初始化链)

**文件**: `global/initializer.go`

**职责**:
- 基于优先级的组件初始化
- 依赖关系自动管理
- 健康检查统一接口
- 资源清理逆序执行

**核心接口**:

```go
type Initializer interface {
    Name() string
    Priority() int
    Initialize(ctx context.Context, cfg *gwconfig.Gateway) error
    Cleanup() error
    HealthCheck() error
}

type InitializerChain struct {
    initializers []Initializer
}

func (c *InitializerChain) Register(i Initializer)
func (c *InitializerChain) InitializeAll(ctx context.Context, cfg *gwconfig.Gateway) error
func (c *InitializerChain) HealthCheckAll() map[string]error
func (c *InitializerChain) CleanupAll() error
```

**内置初始化器**:

| 初始化器 | 优先级 | 职责 | 依赖 |
|---------|--------|------|------|
| LoggerInitializer | 1 | 初始化日志器 | - |
| ContextInitializer | 2 | 创建全局上下文 | Logger |
| SnowflakeInitializer | 5 | 初始化ID生成器 | Logger |
| PoolManagerInitializer | 10 | 初始化连接池 | Logger, Context |

**执行流程**:

```
注册阶段:
  Register(&LoggerInitializer{})      → Priority: 1
  Register(&ContextInitializer{})     → Priority: 2
  Register(&SnowflakeInitializer{})   → Priority: 5
  Register(&PoolManagerInitializer{}) → Priority: 10
  Register(&CustomInitializer{})      → Priority: 20
  
排序阶段:
  sort.Slice(initializers, func(i, j int) bool {
      return initializers[i].Priority() < initializers[j].Priority()
  })
  
执行阶段:
  for _, init := range initializers {
      if err := init.Initialize(ctx, cfg); err != nil {
          return err  // 快速失败
      }
  }
  
清理阶段 (逆序):
  for i := len(initializers) - 1; i >= 0; i-- {
      init.Cleanup()
  }
```

---

### 4. Middleware Manager (中间件管理器)

**文件**: `middleware/manager.go`

**职责**:
- 中间件注册和链接
- 执行顺序控制
- HTTP/gRPC 双协议适配

**核心结构**:

```go
type Manager struct {
    middlewares []func(http.Handler) http.Handler
    config      *gwconfig.Gateway
}

func (m *Manager) Use(middleware func(http.Handler) http.Handler)
func (m *Manager) Chain(handler http.Handler) http.Handler
```

**内置中间件**:

```
执行顺序 (从外到内):
  1. Recovery        → Panic 捕获
  2. RequestID       → 请求ID生成
  3. Logging         → 访问日志
  4. Metrics         → 指标收集
  5. Tracing         → 链路追踪
  6. Security        → 安全防护
  7. CORS            → 跨域处理
  8. RateLimit       → 流量控制
  9. Breaker         → 熔断保护
 10. Auth            → 身份认证
 11. I18N            → 国际化
 ... (自定义中间件)
 
请求流向:
  Client → Recovery → RequestID → ... → Handler → ... → Client
```

---

### 5. Connection Pool Manager (连接池管理器)

**文件**: `cpool/manager.go`

**职责**:
- 统一管理数据库/Redis/MinIO/MQTT连接
- 自动初始化和健康检查
- 连接池生命周期管理

**核心结构**:

```go
type Manager struct {
    db    *gorm.DB
    redis *redis.Client
    minio *minio.Client
    mqtt  mqtt.Client
    logger logger.Logger
}

func (m *Manager) Initialize(ctx context.Context, cfg *gwconfig.Gateway) error
func (m *Manager) GetDB() *gorm.DB
func (m *Manager) GetRedis() *redis.Client
func (m *Manager) HealthCheck() map[string]bool
func (m *Manager) Close() error
```

**初始化逻辑**:

```go
if cfg.Mysql.Enabled {
    m.db = initDatabase(cfg.Mysql)
    global.DB = m.db  // 绑定到全局
}

if cfg.Redis.Enabled {
    m.redis = initRedis(cfg.Redis)
    global.REDIS = m.redis
}

if cfg.Minio.Enabled {
    m.minio = initMinIO(cfg.Minio)
    global.MinIO = m.minio
}
```

---

## 🔄 初始化流程

### 完整初始化时序图

```
┌─────────┐    ┌──────────┐    ┌────────────┐    ┌──────────┐
│ Gateway │    │  Server  │    │ InitChain  │    │PoolMgr   │
└────┬────┘    └─────┬────┘    └──────┬─────┘    └─────┬────┘
     │               │                │                │
     │ Build()       │                │                │
     ├──────────────>│                │                │
     │               │                │                │
     │               │ NewServer()    │                │
     │               ├───────────────>│                │
     │               │                │                │
     │               │ InitializeAll()│                │
     │               ├───────────────>│                │
     │               │                │                │
     │               │                │ Logger (P:1)   │
     │               │                ├───────────────>│
     │               │                │ ✅ global.LOG   │
     │               │                │                │
     │               │                │ Context (P:2)  │
     │               │                ├───────────────>│
     │               │                │ ✅ global.CTX   │
     │               │                │                │
     │               │                │ Snowflake(P:5) │
     │               │                ├───────────────>│
     │               │                │ ✅ global.Node  │
     │               │                │                │
     │               │                │ PoolMgr (P:10) │
     │               │                ├───────────────>│
     │               │                │  Initialize()  │
     │               │                │  ├────────────>│
     │               │                │  │ Init DB     │
     │               │                │  │ Init Redis  │
     │               │                │  │ Init MinIO  │
     │               │                │  │ Bind Global │
     │               │                │  <────────────┤
     │               │                │ ✅ global.DB    │
     │               │                │ ✅ global.REDIS │
     │               │                │                │
     │               │ ✅ Initialize完成│                │
     │               <───────────────┤                │
     │               │                │                │
     │ ✅ Server     │                │                │
     <───────────────┤                │                │
     │               │                │                │
     │ Start()       │                │                │
     ├──────────────>│                │                │
     │               │ StartHTTP()    │                │
     │               │ StartgRPC()    │                │
     │               │ ✅ Running     │                │
     <───────────────┤                │                │
     │               │                │                │
```

### 初始化步骤详解

**Step 1: 网关构建**

```go
gw, err := gateway.NewGateway().
    WithConfigPath("config.yaml").
    WithHotReload(nil).
    Build()
```

- 解析配置文件
- 创建 Server 实例
- 触发初始化链

**Step 2: 初始化链执行**

```go
chain := global.GetDefaultInitializerChain()
ctx, cancel := context.WithTimeout(global.CTX, 30*time.Second)
defer cancel()

err := chain.InitializeAll(ctx, global.GATEWAY)
```

- 按优先级顺序初始化
- 超时控制 (30秒)
- 错误快速失败

**Step 3: 组件绑定**

```go
// 由 PoolManagerInitializer 自动执行
global.DB = manager.GetDB()
global.REDIS = manager.GetRedis()
global.MinIO = manager.GetMinIO()
global.Node = snowflakeNode
```

**Step 4: 服务启动**

```go
gw.Start()
```

- 启动 HTTP 服务器 (端口 8080)
- 启动 gRPC 服务器 (端口 9090)
- 启用功能特性
- 注册信号处理

---

## 🚦 请求处理流程

### HTTP 请求流程

```
Client Request
     │
     ▼
┌─────────────────────────────────┐
│  HTTP Server (:8080)            │
│  http.Server.Handler            │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  Middleware Chain               │
│  ┌──────────────────────┐       │
│  │ 1. Recovery          │       │
│  │ 2. RequestID         │       │
│  │ 3. Logging           │       │
│  │ 4. Metrics           │       │
│  │ 5. Tracing           │       │
│  │ 6. Security          │       │
│  │ 7. CORS              │       │
│  │ 8. RateLimit         │       │
│  │ 9. Breaker           │       │
│  │ 10. Auth             │       │
│  └──────────────────────┘       │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  Router (http.ServeMux)         │
│  - gRPC Gateway Handlers        │
│  - Custom HTTP Handlers         │
│  - Feature Handlers             │
│    (swagger, pprof, health)     │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  Business Handler               │
│  - 数据库操作 (global.DB)        │
│  - 缓存读写 (global.REDIS)      │
│  - 对象存储 (global.MinIO)      │
│  - 业务逻辑处理                  │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  Response                        │
│  - JSON/Protobuf 编码           │
│  - 统一响应格式                  │
│  - 错误码转换                    │
└─────────────────────────────────┘
     │
     ▼
Client Response
```

### gRPC 请求流程

```
gRPC Client
     │
     ▼
┌─────────────────────────────────┐
│  gRPC Server (:9090)            │
│  grpc.Server                    │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  gRPC Interceptors              │
│  - UnaryInterceptor             │
│  - StreamInterceptor            │
│  - Recovery                     │
│  - Logging                      │
│  - Metrics                      │
│  - Tracing                      │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  Service Implementation         │
│  pb.RegisterXXXServer(s, impl)  │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  Business Logic                 │
│  - global.DB 数据库访问         │
│  - global.REDIS 缓存             │
│  - PBMO 模型转换                │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│  Protobuf Response              │
└─────────────────────────────────┘
     │
     ▼
gRPC Client Response
```

---

## 🎨 设计模式

### 1. Builder Pattern (构建器模式)

**应用**: `GatewayBuilder`

**优势**:
- 参数可选，接口清晰
- 链式调用，流畅优雅
- 默认值推断，简化使用

```go
gateway.NewGateway().
    WithConfigPath("config.yaml").  // 可选
    WithEnvironment("production").   // 可选
    WithHotReload(nil).             // 可选
    Silent().                        // 可选
    BuildAndStart()
```

---

### 2. Chain of Responsibility (责任链模式)

**应用**: `InitializerChain`, `MiddlewareChain`

**优势**:
- 解耦发送者和接收者
- 动态添加/删除处理器
- 顺序可控

```go
// 初始化链
chain.Register(&LoggerInitializer{})
chain.Register(&ContextInitializer{})
chain.InitializeAll()

// 中间件链
manager.Use(RecoveryMiddleware())
manager.Use(LoggingMiddleware())
manager.Chain(handler)
```

---

### 3. Strategy Pattern (策略模式)

**应用**: `Initializer` 接口

**优势**:
- 算法族独立变化
- 避免条件语句
- 易于扩展

```go
type Initializer interface {
    Initialize(ctx, cfg) error
    Cleanup() error
    HealthCheck() error
}

// 不同策略实现
&LoggerInitializer{}
&DatabaseInitializer{}
&CacheInitializer{}
```

---

### 4. Adapter Pattern (适配器模式)

**应用**: gRPC-Gateway, WSC Adapter

**优势**:
- 接口转换
- 复用现有代码
- 解耦不兼容接口

```go
// gRPC → HTTP 适配
pb.RegisterUserServiceHandlerFromEndpoint(ctx, mux, endpoint)

// WSC Hub 适配
adapter := wsc.NewAdapter(hubManager, extractor)
```

---

### 5. Singleton Pattern (单例模式)

**应用**: 全局变量 `global.*`

**优势**:
- 唯一实例
- 全局访问点
- 延迟初始化

```go
var (
    DB     *gorm.DB         // 单例数据库
    REDIS  *redis.Client    // 单例Redis
    Node   *snowflake.Node  // 单例ID生成器
    CTX    context.Context  // 单例上下文
)
```

---

## 🔌 扩展性设计

### 1. 自定义初始化器

```go
type MyInitializer struct{}

func (i *MyInitializer) Priority() int { return 15 }
func (i *MyInitializer) Initialize(ctx, cfg) error {
    // 自定义初始化逻辑
}

// 注册
chain.Register(&MyInitializer{})
```

### 2. 自定义中间件

```go
func CustomMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 前置处理
            next.ServeHTTP(w, r)
            // 后置处理
        })
    }
}

// 注册
manager.Use(CustomMiddleware())
```

### 3. 自定义功能特性

```go
const FeatureMyCustom FeatureType = "my_custom"

func enableMyCustomFeature(s *Server) error {
    // 功能启用逻辑
    return nil
}

// 注册
server.RegisterFeature(FeatureMyCustom, enableMyCustomFeature)
```

---

## 📊 性能优化

### 1. 连接池复用

```go
// 连接池配置
mysql:
  max_idle_conns: 10   # 空闲连接
  max_open_conns: 100  # 最大连接
  conn_max_lifetime: 3600s

redis:
  pool_size: 20
  min_idle_conns: 5
```

### 2. 并发控制

```go
// 使用 errgroup 并发初始化
g, ctx := errgroup.WithContext(ctx)
g.Go(func() error { return initDB() })
g.Go(func() error { return initRedis() })
err := g.Wait()
```

### 3. 优雅关闭

```go
// 30秒优雅关闭
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

httpServer.Shutdown(ctx)
grpcServer.GracefulStop()
poolManager.Close()
```

---

**📚 相关文档**:
- [快速开始](QUICK_START.md)
- [初始化器指南](INITIALIZER_GUIDE.md)
- [中间件指南](MIDDLEWARE_GUIDE.md)
- [配置指南](CONFIG_GUIDE.md)
