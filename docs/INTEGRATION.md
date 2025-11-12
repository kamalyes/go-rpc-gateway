# Go RPC Gateway - 四大核心库集成说明

本项目深度集成了四个核心库，构建了一个完整的企业级微服务网关框架。

## 🏗️ 四大核心库架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    Go RPC Gateway                               │
│                   (四大核心库集成)                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  📋 go-config        🔧 go-core         📝 go-logger  🧰 go-toolbox │
│  统一配置管理        企业级组件         结构化日志     工具函数集    │
│                                                                 │
│  • 多格式支持        • MySQL/PG        • 高性能Zap   • 加密/解密   │
│  • 热重载           • Redis集群        • 多输出      • ID生成     │  
│  • 环境变量         • MinIO存储        • 日志轮转    • 字符串工具  │
│  • 配置验证         • RabbitMQ         • 上下文      • 时间工具    │
│  • 分层配置         • Consul           • 性能优化    • 网络工具    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## 📋 go-config - 统一配置管理

**仓库**: [go-config](https://github.com/kamalyes/go-config)  
**版本**: v0.6.0

### 核心特性

- ✅ **多格式支持**: YAML, JSON, TOML, ENV
- ✅ **配置热重载**: 文件变化自动更新
- ✅ **环境变量**: `${VAR_NAME:default}` 语法
- ✅ **配置验证**: 类型和值校验
- ✅ **分层配置**: 环境特定配置覆盖

### 使用示例

```go
// 创建配置管理器
configManager, err := config.NewConfigManager("config/app.yaml")
if err != nil {
    log.Fatal(err)
}

// 监听配置变化
configManager.OnConfigChange(func() {
    log.Println("配置已更新，重新加载...")
})
```

## 🔧 go-core - 企业级组件

**仓库**: [go-core](https://github.com/kamalyes/go-core)  
**版本**: v0.15.6

### 支持的组件

#### 🗄️ 数据库

- **MySQL** 5.7+, 8.0+
- **PostgreSQL** 12+
- **SQLite** 3.x
- 读写分离、连接池、自动迁移

```go
import "github.com/kamalyes/go-rpc-gateway/global"

// 自动获取数据库连接
db := global.DB
if db != nil {
    var users []User
    db.Find(&users)
}
```

#### 🔄 缓存系统

- **Redis** 单机/集群/哨兵模式
- 连接池管理、故障转移

```go
// 自动获取Redis连接
redis := global.REDIS
if redis != nil {
    redis.Set(ctx, "key", "value", time.Hour)
}
```

#### 💾 对象存储

- **MinIO** (S3兼容)
- **阿里云OSS**
- **AWS S3**

```go
// 自动获取MinIO客户端
minio := global.MinIO
if minio != nil {
    minio.PutObject(ctx, "bucket", "object", reader, size, options)
}
```

#### 📨 消息队列

- **RabbitMQ**
- 消息持久化、死信队列

#### 🔍 服务发现

- **Consul**
- 健康检查、服务注册

## 📝 go-logger - 结构化日志

**仓库**: [go-logger](https://github.com/kamalyes/go-logger)  
**版本**: latest

### 核心特性

- ✅ **高性能**: 基于Zap，零分配设计
- ✅ **结构化**: JSON/文本格式
- ✅ **多输出**: 控制台、文件、远程服务
- ✅ **日志轮转**: 按大小、时间自动轮转
- ✅ **上下文**: 携带请求ID、用户信息等

### 使用示例

```go
import "github.com/kamalyes/go-logger/pkg/logger"

// 结构化日志记录
logger.Info("用户登录成功",
    logger.String("user_id", "123"),
    logger.String("ip", clientIP),
    logger.Duration("duration", loginTime),
    logger.Bool("is_admin", user.IsAdmin),
)
```

## 🧰 go-toolbox - 工具函数集

**仓库**: [go-toolbox](https://github.com/kamalyes/go-toolbox)  
**版本**: v0.11.62

### 工具分类

#### 🔐 加密安全

- **AES-256-GCM** 对称加密
- **RSA** 公钥加密  
- **HMAC-SHA256** 签名验证
- **安全随机数** 生成

```go
import "github.com/kamalyes/go-toolbox/pkg/crypto"

// HMAC签名验证
valid := crypto.ValidateHMAC(data, signature, secretKey)
```

#### 🆔 ID生成器

- **UUID v4**: 全球唯一标识符
- **ULID**: 字典序UUID
- **NanoID**: 短ID生成
- **雪花算法**: 分布式ID

```go
import "github.com/kamalyes/go-toolbox/pkg/random"

// 生成UUID
uuid := random.GenerateUUID()
```

#### 🔤 字符串工具

- 驼峰/蛇形转换
- 字符串模糊匹配
- SQL注入防护
- XSS过滤

#### ⏰ 时间工具

- 多时区支持
- 相对时间计算
- 时间格式化/解析

#### 🌐 网络工具

- IP地址验证
- URL解析/构建
- HTTP客户端工具

## 🚀 项目集成效果

### 开箱即用特性

1. **零配置启动**: 使用默认配置即可启动完整网关
2. **自动初始化**: 四大核心库自动集成和初始化
3. **企业级组件**: 数据库、缓存、存储等组件开箱即用
4. **统一配置**: 单一配置文件管理所有组件
5. **高性能日志**: 自动配置结构化日志系统
6. **安全工具**: 内置加密、签名验证等安全功能

### 运行示例

```bash
# 克隆项目
git clone https://github.com/kamalyes/go-rpc-gateway.git
cd go-rpc-gateway

# 运行集成演示
cd examples/integration-demo
go run main.go

# 测试API
curl http://localhost:8080/health      # 健康检查
curl http://localhost:8080/components  # 组件状态
```

### 配置示例

参考 `config/examples/complete-config.yaml` 查看完整配置。

## 📚 文档链接

- [完整README文档](../README.md)
- [架构设计文档](../docs/ARCHITECTURE.md)
- [配置示例](../config/examples/)
- [API文档](../docs/)

---

**四大核心库联合构建，企业级微服务网关框架** 🚀
