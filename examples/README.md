# Go RPC Gateway - Examples Guide

这个目录包含了Go RPC Gateway的完整使用示例，从基础入门到企业级部署。

## 🏗️ 项目架构概览

Go RPC Gateway是一个现代化的gRPC-Gateway框架，提供：

- **🔧 模块化设计**: 基于go-config和go-core的企业级架构
- **🔌 丰富中间件**: CORS、限流、认证、日志、监控等
- **📊 性能分析**: 内置PProf支持，完整的性能测试场景
- **🛡️ 安全特性**: 签名验证、IP白名单、安全头设置
- **📈 可观测性**: Prometheus指标、链路追踪、结构化日志
- **⚙️ 配置驱动**: 支持配置文件和环境变量
- **🔄 热重载**: 运行时配置更新

## 📚 示例列表

### 1. 快速入门 (`01-quickstart/`)

最简单的Gateway使用方式，适合初学者。

**功能特点:**
- 使用默认配置创建Gateway
- 注册简单的HTTP路由
- 基础的健康检查
- 优雅关闭

**运行方式:**
```bash
cd examples/01-quickstart
go run main.go
```

**测试端点:**
- `http://localhost:8080/api/hello`
- `http://localhost:8080/api/health`
- `http://localhost:8080/health`
- `http://localhost:8080/metrics`

### 2. 配置文件使用 (`02-with-config/`)

展示如何使用配置文件来管理Gateway的各种设置。

**功能特点:**
- 完整的YAML配置文件
- 数据库、Redis、MinIO配置
- 中间件配置
- 监控和日志配置

**运行方式:**
```bash
cd examples/02-with-config
go run main.go
```

**配置文件:** `config.yaml` - 包含详细的配置说明

### 3. 中间件功能演示 (`03-middleware/`)

深入展示Gateway的中间件功能。

**功能特点:**
- CORS跨域处理
- 限流控制
- 访问日志记录
- 异常恢复
- 请求ID追踪
- 安全头设置

**运行方式:**
```bash
cd examples/03-middleware
go run main.go
```

**测试端点:**
- `/api/test/cors` - CORS测试
- `/api/test/rate-limit` - 限流测试
- `/api/test/error` - 错误处理测试
- `/api/test/panic` - Panic恢复测试

### 4. PProf性能分析 (`04-pprof/`)

完整的性能分析功能演示。

**功能特点:**
- 启用PProf性能分析
- 内存分配测试
- CPU密集型测试
- Goroutine管理测试
- GC性能测试
- 后台负载生成器

**运行方式:**
```bash
cd examples/04-pprof
go run main.go
```

**性能分析:**
- 访问 `http://localhost:8080/` 查看完整的PProf仪表板
- 使用认证token: `pprof-demo-2024`

**命令行分析:**
```bash
# CPU性能分析
curl -H "Authorization: Bearer pprof-demo-2024" "http://localhost:8080/debug/pprof/profile?seconds=30" -o cpu.prof
go tool pprof cpu.prof

# 内存分析
curl -H "Authorization: Bearer pprof-demo-2024" "http://localhost:8080/debug/pprof/heap" -o heap.prof
go tool pprof heap.prof
```

### 5. gRPC服务集成 (`05-grpc/`)

展示如何集成gRPC服务到Gateway中。

**功能特点:**
- 模拟gRPC服务注册
- gRPC到HTTP的转换
- RESTful API设计
- 用户和产品服务示例

**运行方式:**
```bash
cd examples/05-grpc
go run main.go
```

**API端点:**
- `GET /api/v1/users` - 用户列表
- `POST /api/v1/users` - 创建用户
- `GET /api/v1/products` - 产品列表
- `GET /api/v1/services/status` - 服务状态

### 6. 企业级完整示例 (`06-enterprise/`)

生产环境级别的完整示例，包含所有企业级功能。

**功能特点:**
- 多版本API支持 (v1, v2)
- 完整的业务API
- 管理和监控API
- 企业级配置
- 后台任务管理
- 优雅关闭
- 性能监控

**运行方式:**
```bash
cd examples/06-enterprise
go run main.go
```

**业务API:**
- `/api/v1/users` - 用户管理
- `/api/v1/orders` - 订单管理  
- `/api/v1/products` - 产品管理
- `/api/v2/users` - 增强版用户API
- `/api/v2/analytics` - 数据分析

**管理API:**
- `/admin/health/detailed` - 详细健康检查
- `/admin/config` - 配置信息
- `/admin/metrics/summary` - 指标摘要
- `/admin/performance` - 性能报告

## 🚀 快速开始

### 环境要求

- Go 1.21+
- 可选: MySQL, Redis, MinIO (用于完整功能测试)

### 安装依赖

```bash
# 在项目根目录
go mod download
```

### 运行示例

选择任意示例目录：

```bash
# 例如运行快速入门示例
cd examples/01-quickstart
go run main.go
```

### 构建和部署

```bash
# 构建所有示例
./build.sh

# 运行构建后的程序
./bin/gateway -config examples/02-with-config/config.yaml
```

## 🔧 配置说明

### 基础配置结构

```yaml
app:
  name: "gateway-name"
  version: "1.0.0"
  env: "development|production"

gateway:
  name: "Gateway Display Name"
  debug: true|false
  
  http:
    host: "0.0.0.0"
    port: 8080
    
  grpc:
    host: "0.0.0.0"
    port: 9090

middleware:
  cors:
    enabled: true
    allow_origins: ["*"]
    
  rate_limit:
    enabled: true
    rate: 1000
    burst: 2000
```

### 环境变量

支持的环境变量：

- `PPROF_TOKEN` - PProf认证令牌
- `DB_PASSWORD` - 数据库密码
- `REDIS_PASSWORD` - Redis密码
- `JWT_SECRET_KEY` - JWT密钥

## 📊 监控和指标

### 内置端点

- `/health` - 基础健康检查
- `/metrics` - Prometheus格式指标
- `/debug/pprof/` - 性能分析 (需启用)

### 自定义指标

Gateway自动收集：
- 请求数量和延迟
- 错误率统计
- 内存和CPU使用率
- Goroutine数量
- GC统计

## 🛡️ 安全特性

### 认证授权

- JWT Token认证
- 签名验证 (HMAC-SHA256)
- IP白名单控制

### 安全头

自动设置安全响应头：
- X-Frame-Options
- X-Content-Type-Options
- X-XSS-Protection
- Strict-Transport-Security

## 🔍 故障排查

### 常见问题

1. **端口冲突**
   - 修改配置文件中的端口设置
   - 检查系统是否有其他服务占用端口

2. **配置文件不存在**
   - 检查配置文件路径
   - 参考示例配置文件

3. **依赖连接失败**
   - 检查MySQL/Redis连接配置
   - 确保依赖服务正常运行

### 日志分析

```bash
# 查看详细日志
go run main.go -log-level=debug

# 指定日志目录
go run main.go -log-dir=./logs
```

## 📈 性能优化

### 推荐设置

生产环境推荐配置：

```yaml
gateway:
  debug: false
  
middleware:
  rate_limit:
    rate: 2000
    burst: 5000
    
zap:
  level: "info"
  format: "json"
  log_in_console: false
```

### 监控指标

关注以下关键指标：
- 响应时间 P95/P99
- 错误率
- 内存使用率
- GC频率和耗时

## 🤝 贡献

欢迎提交示例和改进：

1. Fork 项目
2. 创建新的示例目录
3. 添加完整的README说明
4. 提交Pull Request

## 📞 支持

如有问题，请查看：
- [项目文档](../docs/)
- [Issue追踪](https://github.com/kamalyes/go-rpc-gateway/issues)
- [讨论区](https://github.com/kamalyes/go-rpc-gateway/discussions)