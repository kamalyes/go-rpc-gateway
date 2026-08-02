# Server Package 重构说明

这个 `server` 包已经被重构成多个模块化的文件，以提高代码的可读性和可维护性。

## 文件结构

```bash
server/
├── server.go           # 主要的Server结构定义和构造函数
├── core.go            # 核心组件初始化（PoolManager、EndpointCollector）
├── grpc.go            # gRPC服务器初始化和启动逻辑
├── http.go            # HTTP服务器和网关初始化逻辑
├── middleware_init.go # 中间件管理器初始化
├── lifecycle.go       # 服务器生命周期管理（启动、停止等）
├── reload.go          # 配置热重载（ApplyConfig、ReloadHTTPGateway 等）
├── banner.go          # BannerManager 横幅显示
├── startup.go         # 启动展示统一模型与渲染入口
├── swagger.go         # Swagger 文档服务启用
├── swagger_embed.go   # Swagger 文件嵌入与端点解析
├── wsc.go             # WebSocket 服务（go-wsc 薄封装）
├── endpoint_utils.go  # API 端点信息聚合工具
└── README.md          # 本文件
```

## 各文件职责

### server.go

- `Server` 结构体定义
- 构造函数 `NewServer()`（使用全局 `global.GATEWAY` 配置）
- 访问器方法：`GetConfig`、`GetMiddlewareManager`、`GetBannerManager`、`GetPoolManager`、`GetWebSocketService`、`GetEndpointCollector`、`GetDataMasker`、`GetGRPCServer`、`GetEndpoint`、`GetGatewayMux`、`GetContext`、`GetDialOptions`
- 注册方法：`RegisterGRPCService`、`AddGrpcGatewayMiddleware`、`AddGrpcGatewayMiddlewareProvider`

### core.go

- 核心组件初始化：`initCore()`
- 绑定全局 `PoolManager`、初始化 `EndpointCollector`

### grpc.go

- gRPC服务器初始化：`initGRPCServer()`
- gRPC服务器启动：`startGRPCServer()`
- gRPC服务器停止：`stopGRPCServer()`

### http.go

- HTTP网关初始化：`initHTTPGateway()`
- HTTP网关重建：`RebuildHTTPGateway()`
- HTTP服务器启动：`startHTTPServer()`
- HTTP服务器停止：`stopHTTPServer()`
- 健康检查处理器：`healthCheckHandler()`
- HTTP路由注册：`RegisterHTTPRoute()`、`RegisterHTTPHandlerFunc()`
- 命名监听器：`initNamedListeners()`、`startNamedListeners()`、`stopNamedListeners()`

### middleware_init.go

- 中间件管理器初始化：`initMiddleware()`
- 健康检查管理器初始化：`initHealthManager()`
- 服务器组件初始化：`initServers()`
- 扩展指标采集注入：`injectMetricsCollectors()`

### lifecycle.go

- 服务器启动：`Start()`
- 服务器停止：`Stop()`
- 服务器重启：`Restart()`
- 优雅关闭：`Shutdown()`
- 状态检查：`IsRunning()`
- 等待运行：`Wait()`
- 等待关闭信号：`WaitForShutdown()`
- 一键启动：`Run()`

### reload.go

- 更新内存配置：`ApplyConfig(cfg)`
- 重建 HTTP 网关：`ReloadHTTPGateway(cfg, replay)`
- 重建 gRPC 服务器：`ReloadGRPCServer(cfg, registrars)`
- 重建 PProf 服务器：`ReloadPProfServer(cfg)`

### banner.go

- `BannerManager` 结构体定义
- 创建横幅管理器：`NewBannerManager(config)`
- 链式设置上下文：`WithContext(ctx)`
- 添加功能特性：`AddFeature(feature)`
- 关闭横幅：`PrintShutdownBanner()`、`PrintShutdownComplete()`

### startup.go

- 启动展示统一模型（`startupReport` 等）
- 启动前检查：`PrintStartupChecks()`
- 启动成功报告：`PrintStartupReport()`

### swagger.go

- 启用 Swagger 文档服务：`EnableSwagger()`

### swagger_embed.go

- `SwaggerFileProvider` 接口
- `EmbeddedSwaggerProvider` 嵌入式文件提供器
- `NewEmbeddedSwaggerProvider(files)`

### wsc.go

- `WebSocketService` 结构体（go-wsc Hub 薄封装）
- 创建服务：`NewWebSocketService(cfg)`
- 生命周期：`Start()`、`Stop()`、`IsRunning()`
- 访问器：`GetHub()`、`GetConfig()`
- 消息发送：`SendToUserWithRetry(ctx, userID, msg)`
- 回调注册：`OnClientConnect`、`OnClientDisconnect`、`OnMessageReceived`、`OnError`、`OnHeartbeatTimeout` 等

### endpoint_utils.go

- `EndpointCollector` 端点收集器
- `EndpointInfo` 端点信息结构
- 创建收集器：`NewEndpointCollector()`
- 端点操作：`AddEndpoint`、`GetAllEndpoints`、`Clear`
- Swagger 加载：`LoadEndpointsFromSwaggerFile`、`LoadEndpointsFromSwaggerFiles`、`CollectFromSwagger`
- 输出：`ToJSON()`、`CreateHTTPHandler()`
- 工具函数：`GenerateEndpointInfo(method, path, summary, operationID, tags)`

## 重构收益

1. **模块化**：每个文件专注于特定功能，代码更易理解
2. **可维护性**：修改特定功能时只需要关注对应的文件
3. **可测试性**：可以为每个模块编写独立的测试
4. **可扩展性**：添加新功能时可以创建新的模块文件
5. **代码复用**：各模块之间职责清晰，避免代码重复

## 使用示例

```go
// 创建服务器（使用全局 global.GATEWAY 配置）
server, err := NewServer()
if err != nil {
    log.Fatal(err)
}

// 注册gRPC服务
server.RegisterGRPCService(func(s *grpc.Server) {
    // 注册你的gRPC服务
})

// 启动服务器
if err := server.Start(); err != nil {
    log.Fatal(err)
}

// 优雅关闭
defer server.Shutdown()
```

这种模块化的设计使得代码更加清晰，每个文件的职责单一，便于团队协作和代码维护。
