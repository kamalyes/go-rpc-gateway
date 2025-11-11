# 功能管理机制

## 概述

统一的功能管理机制提供了一种优雅的方式来启用和管理各种框架功能，避免为每个功能编写单独的 Enable 方法。

## 当前状态

### ✅ 已完整实现（5个核心功能）
- **SwaggerFeature**: Swagger API 文档功能
  - 自动从 `server.config.Swagger` 读取配置
  - 支持自定义配置
  - 完整的 Enable/IsEnabled 接口
  
- **MonitoringFeature**: 监控功能（Prometheus/Grafana）
  - 自动从 `server.config.Monitoring` 读取配置
  - 注册 `/metrics` 端点
  - 支持自定义指标配置

- **HealthFeature**: 健康检查功能
  - 自动从 `server.config.Health` 读取配置
  - 支持 Redis/MySQL 健康检查
  - 可配置多个检查端点

- **PProfFeature**: 性能分析功能
  - 自动从 `server.config.Pprof` 读取配置
  - 注册标准 pprof 端点
  - 支持自定义路径前缀

- **TracingFeature**: 分布式链路追踪功能（Jaeger/OpenTelemetry）
  - 自动从 `server.config.Jaeger` 读取配置
  - 集成 OpenTelemetry
  - 支持多种采样策略

## 设计模式

### 核心组件

1. **FeatureType**: 功能类型枚举
2. **FeatureEnabler**: 功能启用器接口
3. **FeatureManager**: 统一的功能管理器
4. **具体功能实现**: 如 SwaggerFeature, MonitoringFeature 等

### 架构层次

```
Gateway (门面层)
    ↓
Server (核心层)
    ↓
FeatureManager (功能管理器)
    ↓
FeatureEnabler (功能实现)
```

## 使用方式

### 方式一：使用配置文件自动启用

```yaml
# config.yaml
swagger:
  enabled: true
  json_path: "/swagger.json"
  ui_path: "/swagger"
  title: "My API"

monitoring:
  enabled: true
  metrics:
    enabled: true
    endpoint: "/metrics"

health:
  enabled: true
  path: "/health"
  redis:
    enabled: true
    path: "/health/redis"

pprof:
  enabled: true
  path_prefix: "/debug/pprof"

jaeger:
  enabled: true
  service_name: "my-service"
  endpoint: "http://localhost:14268/api/traces"
  sampling:
    type: "probabilistic"
    param: 0.1
```

```go
// Go 代码 - 自动使用配置文件中的设置
gw, _ := gateway.New(cfg)

// 启用 Swagger
gw.EnableSwagger()

// 启用监控
gw.EnableMonitoring()

// 启用健康检查
gw.EnableHealth()

// 启用性能分析
gw.EnablePProf()

// 启用链路追踪
gw.EnableTracing()
```

### 方式二：使用自定义配置

```go
swaggerConfig := &swagger.Swagger{
    Enabled:  true,
    JSONPath: "/api/swagger.json",
    UIPath:   "/docs",
    Title:    "Custom API",
}

gw.EnableSwaggerWithConfig(swaggerConfig)
```

### 方式三：使用通用接口

```go
// 启用 Swagger
gw.EnableFeature(server.FeatureSwagger)

// 启用其他功能
gw.EnableFeature(server.FeatureMonitoring)
gw.EnableFeature(server.FeatureTracing)

// 检查功能状态
if gw.IsFeatureEnabled(server.FeatureSwagger) {
    fmt.Println("Swagger is enabled")
}
```

### 方式四：批量启用

```go
// Server 层提供
server.GetFeatureManager().EnableAll()
```

## 添加新功能

### 快速启用已准备的功能

所有功能的框架代码已在 `server/features.go` 中准备好（注释状态）。要启用某个功能，只需3步：

#### 步骤 1: 在 Server 层实现 EnableXXXWithConfig 方法

例如，为 Monitoring 创建 `server/monitoring.go`：

```go
package server

import (
    gomonitoring "github.com/kamalyes/go-config/pkg/monitoring"
    "github.com/kamalyes/go-rpc-gateway/middleware"
)

func (s *Server) EnableMonitoringWithConfig(config *gomonitoring.Monitoring) error {
    if !config.Enabled {
        return nil
    }
    
    // 创建并注册 Monitoring 中间件
    monitoringMiddleware := middleware.NewMonitoringMiddleware(config)
    
    // 注册 HTTP 路由
    if config.Prometheus != nil && config.Prometheus.Enabled {
        s.RegisterHTTPRoute(config.Metrics.Endpoint, monitoringMiddleware.MetricsHandler())
    }
    
    return nil
}
```

#### 步骤 2: 取消注释 features.go 中的代码

在 `server/features.go` 中：

1. 取消 import 中的注释：
```go
import (
    gomonitoring "github.com/kamalyes/go-config/pkg/monitoring"  // 取消注释
)
```

2. 在 `registerBuiltinFeatures()` 中取消注册：
```go
func (fm *FeatureManager) registerBuiltinFeatures() {
    fm.enablers[FeatureSwagger] = &SwaggerFeature{server: fm.server}
    fm.enablers[FeatureMonitoring] = &MonitoringFeature{server: fm.server}  // 取消注释
}
```

3. 取消功能实现的注释：
找到 `MonitoringFeature` 的代码块，将整个 `/* ... */` 注释去掉。

#### 步骤 3: (可选) 在 Gateway 添加便捷方法

```go
// gateway.go
func (g *Gateway) EnableMonitoring() error {
    return g.Server.EnableFeature(server.FeatureMonitoring)
}

func (g *Gateway) EnableMonitoringWithConfig(config *monitoring.Monitoring) error {
    return g.Server.EnableFeatureWithConfig(server.FeatureMonitoring, config)
}
```

### 添加全新功能

如果要添加一个全新的功能（不在已准备列表中），按以下步骤：

#### 步骤 1: 定义功能类型

```go
// server/features.go
const (
    FeatureSwagger    FeatureType = "swagger"
    FeatureMonitoring FeatureType = "monitoring"  // 新增
)
```

### 步骤 2: 实现 FeatureEnabler 接口

```go
// MonitoringFeature Monitoring功能实现
type MonitoringFeature struct {
    server  *Server
    enabled bool
}

func (f *MonitoringFeature) Enable() error {
    // 从 server.config.Monitoring 读取配置
    if f.server.config.Monitoring.Enabled {
        return f.EnableWithConfig(&f.server.config.Monitoring)
    }
    // 使用默认配置
    return f.EnableWithConfig(defaultMonitoringConfig)
}

func (f *MonitoringFeature) EnableWithConfig(config interface{}) error {
    monitoringConfig, ok := config.(*monitoring.Monitoring)
    if !ok {
        return fmt.Errorf("invalid config type")
    }
    
    // 实际的启用逻辑
    if err := f.server.EnableMonitoringWithConfig(monitoringConfig); err != nil {
        return err
    }
    
    f.enabled = true
    return nil
}

func (f *MonitoringFeature) IsEnabled() bool {
    return f.enabled
}

func (f *MonitoringFeature) GetType() FeatureType {
    return FeatureMonitoring
}
```

### 步骤 3: 注册到 FeatureManager

```go
// server/features.go
func (fm *FeatureManager) registerBuiltinFeatures() {
    fm.enablers[FeatureSwagger] = &SwaggerFeature{server: fm.server}
    fm.enablers[FeatureMonitoring] = &MonitoringFeature{server: fm.server}  // 新增
}
```

### 步骤 4: (可选) 在 Gateway 添加便捷方法

```go
// gateway.go
func (g *Gateway) EnableMonitoring() error {
    return g.Server.EnableFeature(server.FeatureMonitoring)
}

func (g *Gateway) EnableMonitoringWithConfig(config *monitoring.Monitoring) error {
    return g.Server.EnableFeatureWithConfig(server.FeatureMonitoring, config)
}
```

## 优势

### 1. 统一管理
所有功能通过 FeatureManager 统一管理，代码结构清晰。

### 2. 配置驱动
自动从 go-config 读取配置，减少硬编码。

### 3. 易于扩展
添加新功能只需实现 FeatureEnabler 接口。

### 4. 类型安全
使用类型常量避免字符串错误。

### 5. 向后兼容
保留了便捷方法如 `EnableSwagger()`，同时提供通用接口。

## 配置优先级

```
EnableWithConfig (自定义配置)
    ↓
server.config.Swagger (配置文件)
    ↓
Default Config (默认配置)
```

## 最佳实践

1. **配置优先**: 优先使用配置文件管理功能开关
2. **便捷方法**: 常用功能提供便捷方法
3. **通用接口**: 动态场景使用通用接口
4. **错误处理**: 始终检查 Enable 的返回错误

## 示例：完整流程

```go
package main

import (
    "github.com/kamalyes/go-rpc-gateway/gateway"
    "github.com/kamalyes/go-rpc-gateway/server"
    "github.com/kamalyes/go-config/pkg/swagger"
)

func main() {
    // 1. 创建 Gateway
    gw, err := gateway.New()
    if err != nil {
        panic(err)
    }

    // 2. 方式一：使用配置文件
    gw.EnableSwagger()

    // 3. 方式二：自定义配置
    customSwagger := &swagger.Swagger{
        Enabled:  true,
        UIPath:   "/docs",
        Title:    "My API",
    }
    gw.EnableSwaggerWithConfig(customSwagger)

    // 4. 方式三：通用接口
    gw.EnableFeature(server.FeatureSwagger)

    // 5. 检查状态
    if gw.IsFeatureEnabled(server.FeatureSwagger) {
        println("✓ Swagger enabled")
    }

    // 6. 启动服务
    gw.Start()
}
```
