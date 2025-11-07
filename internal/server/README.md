# Server Package 重构说明

这个 `server` 包已经被重构成多个模块化的文件，以提高代码的可读性和可维护性。

## 文件结构

```
internal/server/
├── server.go           # 主要的Server结构定义和构造函数
├── core.go            # 核心组件初始化（数据库、Redis、日志等）
├── grpc.go            # gRPC服务器初始化和启动逻辑
├── http.go            # HTTP服务器和网关初始化逻辑
├── middleware_init.go # 中间件管理器初始化
├── lifecycle.go       # 服务器生命周期管理（启动、停止等）
└── README.md          # 本文件
```

## 各文件职责

### server.go
- `Server` 结构体定义
- 构造函数 `NewServer` 和 `NewServerWithConfigManager`
- 基本的公共方法如 `GetConfig`、`RegisterGRPCService` 等

### core.go
- 核心组件初始化：`initCore()`
- 数据库初始化：`initDatabase()`
- Redis初始化：`initRedis()`
- 日志初始化：`initLogger()`
- 其他go-core组件初始化
- 配置热重载回调：`onConfigChanged()`

### grpc.go
- gRPC服务器初始化：`initGRPCServer()`
- gRPC服务器启动：`startGRPCServer()`
- gRPC服务器停止：`stopGRPCServer()`

### http.go
- HTTP网关初始化：`initHTTPGateway()`
- HTTP服务器启动：`startHTTPServer()`
- HTTP服务器停止：`stopHTTPServer()`
- 健康检查处理器：`healthCheckHandler()`
- HTTP路由注册：`RegisterHTTPRoute()`、`RegisterHTTPHandler()`

### middleware_init.go
- 中间件管理器初始化：`initMiddleware()`
- 服务器组件初始化：`initServers()`

### lifecycle.go
- 服务器启动：`Start()`
- 服务器停止：`Stop()`
- 服务器重启：`Restart()`
- 优雅关闭：`Shutdown()`
- 状态检查：`IsRunning()`
- 等待运行：`Wait()`

## 重构收益

1. **模块化**：每个文件专注于特定功能，代码更易理解
2. **可维护性**：修改特定功能时只需要关注对应的文件
3. **可测试性**：可以为每个模块编写独立的测试
4. **可扩展性**：添加新功能时可以创建新的模块文件
5. **代码复用**：各模块之间职责清晰，避免代码重复

## 使用示例

```go
// 创建服务器
server, err := NewServer(config)
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