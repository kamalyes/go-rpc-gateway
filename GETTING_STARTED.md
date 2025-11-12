# 🚀 go-rpc-gateway 快速开始

## 立即体验

### 1. 克隆项目

```bash
git clone https://github.com/kamalyes/go-rpc-gateway.git
cd go-rpc-gateway
```

### 2. 一键启动（3种方式任选）

#### 方式1：使用默认配置启动

```bash
go run cmd/gateway/main.go
```

#### 方式2：使用配置文件启动

```bash
go run cmd/gateway/main.go -config=config.yaml
```

#### 方式3：自定义端口启动

```bash
go run cmd/gateway/main.go -port=8888 -grpc-port=9999
```

### 3. 验证服务

启动成功后，你会看到：

```
🚀 正在启动服务...
⏳ 等待服务启动...
✅ HTTP服务启动成功: http://0.0.0.0:8080
✅ gRPC服务启动成功: 0.0.0.0:9090
✅ 健康检查端点: http://0.0.0.0:8080/health
✅ Swagger文档: http://0.0.0.0:8080/swagger
```

立即测试：

```bash
# 健康检查
curl http://localhost:8080/health

# 查看API文档
open http://localhost:8080/swagger
```

## 🎯 核心特性一览

| 特性 | 状态 | 说明 |
|------|------|------|
| 🌐 HTTP/gRPC双协议 | ✅ | 同时支持HTTP和gRPC请求 |
| 🔄 协议转换 | ✅ | HTTP ↔ gRPC 自动转换 |
| 🛡️ 中间件系统 | ✅ | 认证、限流、CORS、恢复等 |
| 📊 健康检查 | ✅ | 内置健康检查端点 |
| 📚 Swagger文档 | ✅ | 自动生成API文档 |
| 🔧 配置管理 | ✅ | YAML配置文件支持 |
| 📝 日志记录 | ✅ | 结构化日志输出 |
| ⚡ 高性能 | ✅ | 优化的网络和内存使用 |

## 📋 命令行选项

```bash
go run cmd/gateway/main.go [选项]

选项:
  -config string     配置文件路径 (默认 "config.yaml")
  -port int         HTTP服务端口 (默认 8080)
  -grpc-port int    gRPC服务端口 (默认 9090)
  -help            显示帮助信息
```

## 🔧 配置说明

### 最小配置（config.yaml）

```yaml
server:
  http:
    port: 8080
  grpc:
    port: 9090
```

### 完整配置

参考 `config.yaml` 文件获取所有配置选项。

## 🏗️ 项目结构

```
go-rpc-gateway/
├── cmd/gateway/          # 主程序入口
│   └── main.go          # 可执行主文件  
├── config.yaml          # 配置文件示例
├── examples/            # 使用示例
├── middleware/          # 中间件
├── server/             # 服务器核心
├── errors/             # 错误定义
└── README.md           # 项目文档
```

## 🎮 使用示例

### 添加你的gRPC服务

1. 将你的 `.proto` 文件放到 `proto/` 目录
2. 生成Go代码：

```bash
protoc --go_out=. --go-grpc_out=. your-service.proto
```

3. 在 `main.go` 中注册你的服务

### 自定义中间件

```go
// 添加自定义中间件
gw.Use(middleware.CustomMiddleware())
```

## 📞 获取帮助

- 📖 完整文档: [README.md](./README.md)
- 💻 代码示例: [examples/](./examples/)
- 🐛 问题反馈: [GitHub Issues](https://github.com/kamalyes/go-rpc-gateway/issues)

---

**🎉 恭喜！你已经成功启动了 go-rpc-gateway！**
