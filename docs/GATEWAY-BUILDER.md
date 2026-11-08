# Gateway 构建器

## 概述

`GatewayBuilder` 使用链式调用 API 构建 Gateway 实例。源码位于 [gateway.go:GatewayBuilder](../gateway.go)。

## 链式 API

### NewGateway()

> 源码：[gateway.go:NewGateway()](../gateway.go)

```go
gw, err := gateway.NewGateway().
    WithSearchPath("resources").
    WithEnvironment(goconfig.EnvProduction).
    WithPrefix("gateway-my-service").
    WithHotReload(nil).
    Build()
```

### 构建选项

| 方法 | 说明 | 源码 |
|------|------|------|
| `WithConfigPath(path)` | 直接指定配置文件路径 | [gateway.go](../gateway.go) |
| `WithSearchPath(path)` | 设置配置文件搜索目录 | [gateway.go](../gateway.go) |
| `WithEnvironment(env)` | 设置运行环境 | [gateway.go](../gateway.go) |
| `WithPrefix(prefix)` | 设置配置文件前缀 | [gateway.go](../gateway.go) |
| `WithPattern(pattern)` | 设置文件匹配模式 | [gateway.go](../gateway.go) |
| `WithHotReload(config)` | 启用热更新（nil 使用默认） | [gateway.go](../gateway.go) |
| `WithContext(ctx)` | 设置上下文 | [gateway.go](../gateway.go) |
| `WithContextOptions(opts)` | 设置上下文选项 | [gateway.go](../gateway.go) |
| `Silent()` | 静默启动（不显示 banner） | [gateway.go](../gateway.go) |
| `WithGrpcGatewayMiddleware(mw)` | 添加 gRPC-Gateway 中间件 | [gateway.go](../gateway.go) |

### 构建方法

| 方法 | 说明 | 源码 |
|------|------|------|
| `Build()` | 构建 Gateway（不启动） | [gateway.go](../gateway.go) |
| `BuildAndStart(ctx ...context.Context)` | 构建并启动 | [gateway.go](../gateway.go) |
| `MustBuildAndStart(ctx ...context.Context)` | 构建并启动（失败 panic） | [gateway.go](../gateway.go) |

## 配置发现策略

`Build()` 内部根据设置自动选择配置发现策略，优先级从高到低：

```mermaid
flowchart TD
    BUILD["Build()"] --> CHECK_PATTERN{"设置了 WithPattern?"}
    CHECK_PATTERN -->|是| PATTERN["模式匹配, 如 gateway-*.yaml"]
    CHECK_PATTERN -->|否| CHECK_PREFIX{"设置了 WithPrefix?"}
    CHECK_PREFIX -->|是| PREFIX["前缀发现, 如 gateway-core.yaml"]
    CHECK_PREFIX -->|否| CHECK_SEARCH{"设置了 WithSearchPath?"}
    CHECK_SEARCH -->|是| AUTO["自动发现, 搜索目录下所有配置"]
    CHECK_SEARCH -->|否| CHECK_PATH{"设置了 WithConfigPath?"}
    CHECK_PATH -->|是| DIRECT["直接使用, 指定路径"]
    CHECK_PATH -->|否| ERROR["返回 ErrInvalidConfiguration"]

    style PATTERN fill:#c8e6c9
    style PREFIX fill:#c8e6c9
    style AUTO fill:#c8e6c9
    style DIRECT fill:#c8e6c9
    style ERROR fill:#ffcdd2
```

> 源码：[gateway.go:Build()](../gateway.go) 中的 switch-case 逻辑

## 示例

### 使用前缀发现（最常用）

```go
gw, err := gateway.NewGateway().
    WithSearchPath("resources").
    WithEnvironment(goconfig.GetEnvironment()).
    WithPrefix("gateway-my-service").
    WithHotReload(nil).
    Build()
```

配置文件位置：`resources/gateway-my-service.yaml`

### 使用模式匹配

```go
gw, err := gateway.NewGateway().
    WithSearchPath("resources").
    WithPattern("gateway-*.yaml").
    WithEnvironment(goconfig.EnvProduction).
    Build()
```

### 直接指定路径

```go
gw, err := gateway.NewGateway().
    WithConfigPath("/etc/myapp/config.yaml").
    WithEnvironment(goconfig.EnvProduction).
    Build()
```

### 构建并启动

```go
gw, err := gateway.NewGateway().
    WithSearchPath("resources").
    WithPrefix("gateway-my-service").
    WithEnvironment(goconfig.GetEnvironment()).
    BuildAndStart()
```

## 初始化链

`Build()` 内部自动执行 `InitializerChain`，按优先级初始化组件：

```mermaid
flowchart TD
    BUILD["Build()"] --> INIT["InitializerChain.InitializeAll()"]

    subgraph CHAIN["初始化链（按优先级）"]
        I1["① LoggerInitializer, 优先级 1, → global.LOGGER"]
        I2["② ContextInitializer, 优先级 2, → global.CTX / CANCEL"]
        I3["③ SnowflakeInitializer, 优先级 5, → global.Node"]
        I4["④ PoolManagerInitializer, 优先级 10, → global.DB / REDIS + PoolManager"]
        I1 --> I2 --> I3 --> I4
    end

    INIT --> CHAIN
    CHAIN --> SERVER["server.NewServer()"]
    SERVER --> READY["Gateway 就绪"]

    style I1 fill:#e3f2fd
    style I2 fill:#e8f5e9
    style I3 fill:#fff9c4
    style I4 fill:#fce4ec
```

> 源码：[initializer.go:GetDefaultInitializerChain()](../global/initializer.go)

| 优先级 | 初始化器 | 说明 | 源码 |
|--------|---------|------|------|
| 1 | LoggerInitializer | 日志器 | [initializer.go](../global/initializer.go) |
| 2 | ContextInitializer | 全局上下文 | [initializer.go](../global/initializer.go) |
| 5 | SnowflakeInitializer | 雪花 ID 生成器 | [initializer.go](../global/initializer.go) |
| 10 | PoolManagerInitializer | 连接池管理器 | [initializer.go](../global/initializer.go) |

初始化完成后，以下全局变量自动可用：

```go
gwglobal.LOGGER    // 日志器
gwglobal.DB        // 数据库连接（便捷引用）
gwglobal.REDIS     // Redis 连接（便捷引用）
gwglobal.Node      // 雪花 ID 节点

// 其余连接都挂在 PoolManager 上，没有包级便捷变量
pm := gwglobal.GetPoolManager()
pm.GetClickHouse()
pm.GetNats()
```

## 热更新

启用热更新后，配置文件变更会自动触发回调：

```go
gw, err := gateway.NewGateway().
    WithSearchPath("resources").
    WithPrefix("gateway-my-service").
    WithHotReload(nil).  // nil 使用默认配置（3秒轮询）
    Build()
```

默认热更新配置：
- 轮询间隔：3 秒
- 变更后自动重新初始化日志器
- 配置变更回调优先级：-100（高优先级）

> 源码：[gateway.go:registerGlobalConfigCallbacks()](../gateway.go)

## Gateway 实例方法

构建完成后，Gateway 实例提供以下核心方法：

### 服务注册

| 方法 | 说明 | 源码 |
|------|------|------|
| `RegisterService(fn)` | 注册 gRPC 服务 | [gateway.go](../gateway.go) |
| `RegisterGatewayHandler(fn)` | 注册 gRPC-Gateway Handler（本地调用） | [gateway.go](../gateway.go) |
| `RegisterProxyHandler(fn, endpoint, dialOpts...)` | 注册代理处理器（远程 endpoint） | [gateway.go](../gateway.go) |
| `RegisterProxyHandlerByServiceName(serviceName, fn)` | 通过服务名注册代理处理器（共享连接） | [gateway.go](../gateway.go) |
| `GetGRPCEndpoint(serviceName)` | 获取 gRPC 客户端端点地址 | [gateway.go](../gateway.go) |
| `AutoRegister()` | 自动注册所有 gRPC 客户端与 HTTP Handler | [gateway.go](../gateway.go) |
| `AutoRegisterWithHealthCheck(checker)` | 自动注册（带健康检查器） | [gateway.go](../gateway.go) |

### HTTP 路由

| 方法 | 说明 | 源码 |
|------|------|------|
| `RegisterHandler(pattern, handler)` | 注册自定义 HTTP 路由 | [gateway.go](../gateway.go) |
| `RegisterHTTPRoute(pattern, fn)` | 注册 HTTP 路由（便捷） | [gateway.go](../gateway.go) |
| `RegisterHTTPRoutes(routes)` | 批量注册 HTTP 路由 | [gateway.go](../gateway.go) |

### 中间件

| 方法 | 说明 | 源码 |
|------|------|------|
| `AddGrpcGatewayMiddleware(mw)` | 添加 gRPC-Gateway 中间件 | [gateway.go](../gateway.go) |
| `AddGrpcGatewayMiddlewareProvider(fn)` | 添加中间件提供器 | [gateway.go](../gateway.go) |
| `RebuildHTTPGateway()` | 重建 HTTP Gateway | [gateway.go](../gateway.go) |
| `SetDynamicSignatureProvider(p)` | 设置动态签名提供器 | [gateway.go](../gateway.go) |
| `SetDynamicRateLimitProvider(p)` | 设置动态限流提供器 | [gateway.go](../gateway.go) |

### 配置与监听器

| 方法 | 说明 | 源码 |
|------|------|------|
| `GetConfig()` | 获取网关配置 | [gateway.go](../gateway.go) |
| `GetGatewayConfig()` | 获取网关配置（直接字段） | [gateway.go](../gateway.go) |
| `GetListener(name)` | 按名称获取监听器配置 | [gateway.go](../gateway.go) |
| `GetListenerEndpoint(name)` | 按名称获取监听器端点地址 | [gateway.go](../gateway.go) |
| `RegisterConfigCallbacks()` | 注册配置变更回调 | [gateway.go](../gateway.go) |
| `Context()` | 获取 Gateway 上下文 | [gateway.go](../gateway.go) |

### 生命周期

| 方法 | 说明 | 源码 |
|------|------|------|
| `Run()` | 启动并等待关闭信号 | [gateway.go](../gateway.go) |
| `Start()` | 启动（显示 banner） | [gateway.go](../gateway.go) |
| `StartSilent()` | 静默启动 | [gateway.go](../gateway.go) |
| `StartWithBanner()` | 启动并显示 banner | [gateway.go](../gateway.go) |
| `Stop()` | 停止服务 | [gateway.go](../gateway.go) |
| `WaitForShutdown()` | 等待关闭信号并优雅关闭 | [gateway.go](../gateway.go) |
| `OnShutdown(fn)` | 注册应用级关闭回调（Stop 前执行） | [gateway.go](../gateway.go) |

### 连接池与资源

| 方法 | 说明 | 源码 |
|------|------|------|
| `GetPoolManager()` | 获取连接池管理器 | [gateway.go](../gateway.go) |
| `GetDB()` | 获取数据库连接 | [gateway.go](../gateway.go) |
| `GetRedis()` | 获取 Redis 客户端 | [gateway.go](../gateway.go) |
| `GetSnowflake()` | 获取雪花 ID 生成器 | [gateway.go](../gateway.go) |
| `HealthCheck()` | 获取所有连接的健康状态 | [gateway.go](../gateway.go) |

### 诊断与信息

| 方法 | 说明 | 源码 |
|------|------|------|
| `PrintStartupInfo()` | 打印启动信息 | [gateway.go](../gateway.go) |
| `PrintShutdownInfo()` | 打印关闭信息 | [gateway.go](../gateway.go) |
| `PrintShutdownComplete()` | 打印关闭完成信息 | [gateway.go](../gateway.go) |
| `PrintAPIRegistrationSummary()` | 打印 API 注册汇总 | [gateway.go](../gateway.go) |

### WebSocket

| 方法 | 说明 | 源码 |
|------|------|------|

### 其他（继承自 Server）

| 方法 | 说明 | 源码 |
|------|------|------|
| — | Swagger 由配置 `swagger.enabled` 控制，没有对应方法 |  |

## 包级便捷函数

| 函数 | 说明 | 源码 |
|------|------|------|
| `QuickStart(configPath ...string)` | 快速启动（默认配置路径与自动发现） | [gateway.go](../gateway.go) |
| `QuickStartWithConfigFile(path)` | 使用指定配置文件快速启动 | [gateway.go](../gateway.go) |
| `QuickStartWithConfigFilePrefix(path, prefix)` | 使用指定配置文件和前缀快速启动 | [gateway.go](../gateway.go) |

## 下一步

- [服务注册](./SERVICE-REGISTRATION.md) — 了解如何注册 gRPC 和 HTTP 服务
- [全局变量与初始化器](./GLOBAL.md) — 了解初始化链和全局状态
