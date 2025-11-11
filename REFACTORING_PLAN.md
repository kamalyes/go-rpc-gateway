# Go RPC Gateway 架构设计

## 项目概览

go-rpc-gateway 是一个企业级 gRPC 网关框架，集成了四大核心库：
1. **go-config** - 统一配置管理
2. **go-core** - 企业级组件（数据库、缓存、存储等）  
3. **go-logger** - 高性能结构化日志
4. **go-toolbox** - 常用工具函数集

---

## 架构设计

## 核心特性

### 🏗️ 重构后的模块化架构

```
go-rpc-gateway/
├── 🎯 gateway.go               # 主网关入口
├── 🏗️ server/                 # 服务器核心 [已重构]
│   ├── server.go              # 核心结构定义
│   ├── grpc.go                # gRPC 服务器
│   ├── http.go                # HTTP 网关
│   └── lifecycle.go           # 生命周期管理
├── 🔌 middleware/             # 中间件生态系统
│   ├── manager.go             # 中间件管理器
│   ├── metrics.go             # Prometheus 监控
│   ├── tracing.go             # OpenTelemetry 链路追踪
│   ├── security.go            # 安全防护
│   ├── ratelimit.go           # 流量控制
│   └── logging.go             # 结构化日志
├── ⚙️ config/                 # 配置管理
│   ├── gateway.go             # 网关配置
│   └── manager.go             # 配置管理器
└── 📚 docs/                   # 文档系统
```

### 🚀 企业级特性

✅ **已实现的功能**:
- **四大核心库集成** - go-config, go-core, go-logger, go-toolbox
- **中间件系统** - 15+ 内置中间件，支持自定义扩展
- **性能分析** - 内置 PProf 支持，支持性能测试场景
- **监控指标** - Prometheus 指标收集
- **链路追踪** - OpenTelemetry 集成
- **多语言支持** - 19种语言 i18n
- **配置热重载** - 运行时配置更新
- **健康检查** - 多组件状态监控

### 🎯 设计原则

| 原则 | 描述 | 实现方式 |
|------|------|----------|
| **模块化** | 单一职责，松耦合 | 按功能分离模块 |
| **配置驱动** | 外部化配置管理 | go-config 统一管理 |
| **中间件架构** | 可插拔组件设计 | 中间件管理器 |
| **可观测性** | 完整监控体系 | Metrics + Tracing + Logging |
| **类型安全** | 编译时检查 | Protocol Buffers + 强类型配置 |
