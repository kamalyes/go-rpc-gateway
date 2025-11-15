# 🔧 统一初始化器指南 (InitializerChain)

> 自动化组件初始化 · 依赖管理 · 健康检查

---

## 🎯 设计理念

### 为什么需要 InitializerChain？

**传统初始化的问题**:

```go
// ❌ 问题代码：初始化逻辑分散，依赖顺序隐藏
func initializeComponents() error {
    initializeLogger()
    initializeDatabase()
    initializeRedis()
    initializeSnowflake()
    bindResourcesToGlobal()
    // 顺序错了会导致初始化失败
    // 添加新组件需要手动处理依赖
}
```

**InitializerChain 的优势**:

```go
// ✅ 优化代码：基于优先级自动排序，清晰可维护
chain := global.GetDefaultInitializerChain()
chain.Register(&LoggerInitializer{})      // Priority: 1
chain.Register(&ContextInitializer{})     // Priority: 2
chain.Register(&SnowflakeInitializer{})   // Priority: 5
chain.Register(&PoolManagerInitializer{}) // Priority: 10
chain.Register(&MyInitializer{})          // Priority: 20

// 自动按优先级初始化，支持健康检查和优雅清理
chain.InitializeAll(ctx, cfg)
```

---

## 🏗️ 核心架构

### 初始化流程

```
┌─────────────────────────────────────────────────────────────┐
│                    InitializerChain                         │
├─────────────────────────────────────────────────────────────┤
│  1. 注册所有初始化器                                          │
│     └─> Register(&Initializer{}) × N                        │
├─────────────────────────────────────────────────────────────┤
│  2. 按优先级自动排序                                          │
│     └─> sort.Slice(initializers, by Priority)              │
├─────────────────────────────────────────────────────────────┤
│  3. 顺序初始化                                               │
│     ├─> Logger (1) → Context (2) → Snowflake (5)            │
│     └─> PoolManager (10) → Custom (20+)                     │
├─────────────────────────────────────────────────────────────┤
│  4. 健康检查                                                 │
│     └─> HealthCheckAll() → map[string]error                 │
├─────────────────────────────────────────────────────────────┤
│  5. 优雅清理 (逆序)                                          │
│     └─> CleanupAll() → Custom → Pool → ... → Logger         │
└─────────────────────────────────────────────────────────────┘
```

---

## 📖 Initializer 接口

所有初始化器都需要实现以下接口：

```go
type Initializer interface {
    // Name 初始化器名称 (用于日志和错误信息)
    Name() string

    // Priority 优先级 (数字越小越先执行)
    // 推荐范围：
    //   1-10:   基础组件 (Logger, Context)
    //   11-50:  基础设施 (Database, Redis, MinIO)
    //   51-100: 业务组件 (Cache, Queue)
    //   101+:   应用逻辑
    Priority() int

    // Initialize 初始化组件
    Initialize(ctx context.Context, cfg *gwconfig.Gateway) error

    // Cleanup 清理资源 (逆序调用)
    Cleanup() error

    // HealthCheck 健康检查
    HealthCheck() error
}
```

---

## 🔨 内置初始化器

### 1. LoggerInitializer (Priority: 1)

**职责**: 初始化全局日志器

```go
type LoggerInitializer struct{}

func (i *LoggerInitializer) Name() string { return "Logger" }
func (i *LoggerInitializer) Priority() int { return 1 }

func (i *LoggerInitializer) Initialize(ctx context.Context, cfg *gwconfig.Gateway) error {
    if err := global.EnsureLoggerInitialized(); err != nil {
        return err
    }
    
    level := logger.INFO
    if cfg != nil && cfg.Debug {
        level = logger.DEBUG
    }
    
    global.LOGGER = logger.CreateSimpleLogger(level)
    global.LOG = global.LOGGER
    return nil
}
```

**全局变量**:
- `global.LOGGER` - 主日志器
- `global.LOG` - 别名

---

### 2. ContextInitializer (Priority: 2)

**职责**: 初始化全局上下文

```go
type ContextInitializer struct{}

func (i *ContextInitializer) Name() string { return "Context" }
func (i *ContextInitializer) Priority() int { return 2 }

func (i *ContextInitializer) Initialize(ctx context.Context, cfg *gwconfig.Gateway) error {
    global.CTX, global.CANCEL = context.WithCancel(context.Background())
    return nil
}

func (i *ContextInitializer) Cleanup() error {
    if global.CANCEL != nil {
        global.CANCEL()
    }
    return nil
}
```

**全局变量**:
- `global.CTX` - 全局上下文
- `global.CANCEL` - 取消函数

---

### 3. SnowflakeInitializer (Priority: 5)

**职责**: 初始化分布式 ID 生成器

```go
type SnowflakeInitializer struct{}

func (i *SnowflakeInitializer) Name() string { return "Snowflake" }
func (i *SnowflakeInitializer) Priority() int { return 5 }

func (i *SnowflakeInitializer) Initialize(ctx context.Context, cfg *gwconfig.Gateway) error {
    nodeID := int64(1)  // 可从配置读取
    
    node, err := snowflake.NewNode(nodeID)
    if err != nil {
        return fmt.Errorf("创建Snowflake节点失败: %w", err)
    }
    
    global.Node = node
    return nil
}

func (i *SnowflakeInitializer) HealthCheck() error {
    if global.Node == nil {
        return fmt.Errorf("Snowflake节点未初始化")
    }
    _ = global.Node.Generate()  // 测试生成ID
    return nil
}
```

**全局变量**:
- `global.Node` - Snowflake 节点

---

### 4. PoolManagerInitializer (Priority: 10)

**职责**: 初始化所有连接池 (Database, Redis, MinIO, MQTT)

```go
type PoolManagerInitializer struct{}

func (i *PoolManagerInitializer) Name() string { return "PoolManager" }
func (i *PoolManagerInitializer) Priority() int { return 10 }

func (i *PoolManagerInitializer) Initialize(ctx context.Context, cfg *gwconfig.Gateway) error {
    manager := cpool.NewManager(global.LOGGER)
    
    if err := manager.Initialize(ctx, cfg); err != nil {
        return fmt.Errorf("初始化 Pool Manager 失败: %w", err)
    }
    
    global.POOL_MANAGER = manager
    
    // 绑定到全局变量
    global.DB = manager.GetDB()
    global.REDIS = manager.GetRedis()
    global.MinIO = manager.GetMinIO()
    
    return nil
}

func (i *PoolManagerInitializer) Cleanup() error {
    if global.POOL_MANAGER != nil {
        return global.POOL_MANAGER.Close()
    }
    return nil
}

func (i *PoolManagerInitializer) HealthCheck() error {
    if global.POOL_MANAGER == nil {
        return fmt.Errorf("连接池管理器未初始化")
    }
    
    status := global.POOL_MANAGER.HealthCheck()
    for name, healthy := range status {
        if !healthy {
            return fmt.Errorf("组件 %s 健康检查失败", name)
        }
    }
    
    return nil
}
```

**全局变量**:
- `global.POOL_MANAGER` - 连接池管理器
- `global.DB` - GORM 数据库连接
- `global.REDIS` - Redis 客户端
- `global.MinIO` - MinIO 客户端

---

## 🚀 自定义初始化器

### 示例：缓存初始化器

```go
package global

import (
    "context"
    "fmt"
    "github.com/kamalyes/go-cachex"
    gwconfig "github.com/kamalyes/go-config/pkg/gateway"
)

// CacheInitializer 缓存初始化器
type CacheInitializer struct{}

func (i *CacheInitializer) Name() string { return "Cache" }
func (i *CacheInitializer) Priority() int { return 15 }  // 在 PoolManager 之后

func (i *CacheInitializer) Initialize(ctx context.Context, cfg *gwconfig.Gateway) error {
    if REDIS == nil {
        return fmt.Errorf("Redis 未初始化，跳过缓存初始化")
    }
    
    // 创建基于 Redis 的缓存
    cache := cachex.NewRedisCache(REDIS)
    CACHE = cache
    
    LOGGER.Info("✅ 缓存初始化完成")
    return nil
}

func (i *CacheInitializer) Cleanup() error {
    CACHE = nil
    return nil
}

func (i *CacheInitializer) HealthCheck() error {
    if CACHE == nil {
        return fmt.Errorf("缓存未初始化")
    }
    
    // 测试缓存读写
    testKey := "health_check_test"
    if err := CACHE.Set(ctx, testKey, "ok", 1); err != nil {
        return fmt.Errorf("缓存写入失败: %w", err)
    }
    
    if _, err := CACHE.Get(ctx, testKey); err != nil {
        return fmt.Errorf("缓存读取失败: %w", err)
    }
    
    return nil
}

// 在 global.go 中添加全局变量
var CACHE cachex.CtxCache

// 在 GetDefaultInitializerChain 中注册
func GetDefaultInitializerChain() *InitializerChain {
    chain := NewInitializerChain()
    chain.Register(&LoggerInitializer{})
    chain.Register(&ContextInitializer{})
    chain.Register(&SnowflakeInitializer{})
    chain.Register(&PoolManagerInitializer{})
    chain.Register(&CacheInitializer{})  // 添加这行
    return chain
}
```

---

## 💡 最佳实践

### 1. 优先级设置规范

```go
// 推荐的优先级分配
const (
    PriorityLogger     = 1   // 日志器 - 最优先
    PriorityContext    = 2   // 上下文
    PriorityConfig     = 3   // 配置管理
    PrioritySnowflake  = 5   // ID生成器
    PriorityDatabase   = 10  // 数据库
    PriorityRedis      = 11  // Redis
    PriorityCache      = 15  // 缓存
    PriorityQueue      = 20  // 消息队列
    PriorityService    = 50  // 业务服务
    PriorityCustom     = 100 // 自定义组件
)
```

### 2. 错误处理

```go
func (i *MyInitializer) Initialize(ctx context.Context, cfg *gwconfig.Gateway) error {
    // ❌ 错误：吞掉错误
    client, _ := NewClient()
    
    // ✅ 正确：返回详细错误
    client, err := NewClient()
    if err != nil {
        return fmt.Errorf("创建客户端失败: %w", err)
    }
    
    return nil
}
```

### 3. 健康检查实现

```go
func (i *MyInitializer) HealthCheck() error {
    // 检查组件是否初始化
    if MyComponent == nil {
        return fmt.Errorf("组件未初始化")
    }
    
    // 执行实际健康检查
    if err := MyComponent.Ping(); err != nil {
        return fmt.Errorf("健康检查失败: %w", err)
    }
    
    return nil
}
```

### 4. 资源清理

```go
func (i *MyInitializer) Cleanup() error {
    if MyComponent == nil {
        return nil  // 未初始化，无需清理
    }
    
    // 执行清理逻辑
    if err := MyComponent.Close(); err != nil {
        return fmt.Errorf("关闭组件失败: %w", err)
    }
    
    MyComponent = nil  // 清空引用
    return nil
}
```

---

## 🔍 调试和监控

### 查看初始化状态

```go
// 获取健康检查结果
healthStatus := global.InitChain.HealthCheckAll()

for name, err := range healthStatus {
    if err != nil {
        fmt.Printf("❌ %s: %v\n", name, err)
    } else {
        fmt.Printf("✅ %s: healthy\n", name)
    }
}
```

### 日志输出示例

```
🔧 初始化 Logger...
✅ Logger 初始化完成
🔧 初始化 Context...
✅ Context 初始化完成
🔧 初始化 Snowflake...
✅ Snowflake 初始化完成
🔧 初始化 PoolManager...
  └─> 初始化数据库...
  └─> 初始化 Redis...
  └─> 初始化 MinIO...
✅ PoolManager 初始化完成
```

---

## ❓ 常见问题

### Q: 如何控制初始化顺序？

A: 通过 `Priority()` 方法控制，数字越小越先执行。

### Q: 初始化失败会怎样？

A: `InitializeAll()` 会立即返回错误，已初始化的组件可通过 `CleanupAll()` 清理。

### Q: 可以动态添加初始化器吗？

A: 可以，但建议在 `BuildAndStart()` 之前注册完所有初始化器。

### Q: 如何处理循环依赖？

A: 通过优先级避免循环依赖。如果 A 依赖 B，则 B 的优先级应小于 A。

---

**📚 相关文档**:
- [架构设计](ARCHITECTURE.md)
- [连接池管理](POOL_MANAGEMENT.md)
- [配置指南](CONFIG_GUIDE.md)
