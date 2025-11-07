# 🚀 Go RPC Gateway

<div align="center">

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)]()
[![Go Report Card](https://img.shields.io/badge/go%20report-A+-brightgreen.svg)]()

**一个现代化的 gRPC-Gateway 框架，开箱即用的微服务网关解决方案**

集成了 [go-config](https://github.com/kamalyes/go-config) 和 [go-core](https://github.com/kamalyes/go-core) 架构，提供数据库、缓存、对象存储等企业级功能。

[快速开始](#-快速开始) • [配置文档](#️-配置文档) • [架构设计](#-架构设计) • [部署指南](#-部署指南) • [示例代码](examples/)

</div>

---

## ✨ 核心特性

<table>
<tr>
<td>

### 🏗️ **架构优化**
- 🔧 **模块化设计** - 可插拔的组件架构
- 🎯 **go-core 深度集成** - 自动初始化全局组件
- 🔄 **配置热重载** - 运行时动态更新配置
- 📊 **企业级监控** - Prometheus + OpenTelemetry

</td>
<td>

### �️ **安全与性能**
- 🚦 **智能限流** - 多算法支持（令牌桶、滑动窗口）
- 🔐 **请求签名** - HMAC-SHA256 安全验证
- 🛡️ **安全中间件** - CORS、安全头、防护机制
- ⚡ **高性能日志** - 基于 zap 的结构化日志

</td>
</tr>
</table>

### 🎪 **丰富的中间件生态**

| 类型 | 中间件 | 描述 |
|------|--------|------|
| **安全** | Security, CORS, Signature | 安全头设置、跨域支持、请求签名验证 |
| **监控** | Metrics, Tracing, Logging | Prometheus 指标、链路追踪、访问日志 |
| **控制** | RateLimit, Recovery, RequestID | 流量控制、异常恢复、请求追踪 |
| **扩展** | Custom Middleware | 支持自定义中间件开发 |

## 📦 快速安装

```bash
# 安装框架
go get github.com/kamalyes/go-rpc-gateway

# 或使用 Go Modules
go mod init your-project
go get github.com/kamalyes/go-rpc-gateway@latest
```

## � 快速开始

### 🎯 零配置启动

```go
package main

import (
    "context"
    "log"
    
    "github.com/kamalyes/go-rpc-gateway"
    "github.com/kamalyes/go-rpc-gateway/internal/server"
)

func main() {
    // 创建服务器实例
    srv, err := server.NewServer(nil) // 使用默认配置
    if err != nil {
        log.Fatal("创建服务器失败:", err)
    }

    // 注册你的 gRPC 服务
    srv.RegisterGRPCService(func(s *grpc.Server) {
        // pb.RegisterYourServiceServer(s, &yourServiceImpl{})
    })

    // 启动服务器
    log.Println("🚀 启动 Gateway 服务器...")
    if err := srv.Start(); err != nil {
        log.Fatal("启动失败:", err)
    }

    // 优雅关闭
    defer srv.Shutdown()
}
```

### 💻 命令行工具

```bash
# 构建项目
go build -o bin/gateway cmd/gateway/main.go

# 使用默认配置启动
./bin/gateway

# 指定配置文件启动
./bin/gateway -config config.yaml

# 开发模式启动（带详细日志）
./bin/gateway -log-level debug -log-dir ./logs

# 查看版本信息
./bin/gateway -version
```

### 📁 使用配置文件

<details>
<summary>点击展开配置文件示例</summary>

```go
// 1. 创建配置文件
configManager := config.NewConfigManager("config.yaml")

// 2. 创建服务器
srv, err := server.NewServerWithConfigManager(configManager)
if err != nil {
    log.Fatal(err)
}

// 3. 启动服务器
srv.Start()
```

</details>

## ⚙️ 配置文档

### 📋 完整配置示例

<details>
<summary>点击查看完整的 config.yaml 配置文件</summary>

```yaml
# ===========================================
# Go RPC Gateway 完整配置文件
# ===========================================

# 基础服务配置 (继承自 go-config)
server:
  name: go-rpc-gateway
  version: v1.0.0
  environment: development

# Gateway 核心配置
gateway:
  name: go-rpc-gateway
  version: v1.0.0
  debug: true
  
  # HTTP 服务配置
  http:
    host: 0.0.0.0
    port: 8080
    read_timeout: 30
    write_timeout: 30
    idle_timeout: 120
    max_header_bytes: 1048576  # 1MB
    
  # gRPC 服务配置
  grpc:
    host: 0.0.0.0
    port: 9090
    network: tcp
    enable_reflection: true
    max_recv_msg_size: 4194304    # 4MB
    max_send_msg_size: 4194304    # 4MB

  # 健康检查配置
  health_check:
    enabled: true
    path: /health

# 中间件配置
middleware:
  # CORS 跨域配置
  cors:
    enabled: true
    allow_origins: ["*"]
    allow_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allow_headers: ["*"]
    expose_headers: ["Content-Length"]
    allow_credentials: true
    max_age: 86400

  # 限流配置
  rate_limit:
    enabled: true
    algorithm: token_bucket    # token_bucket, sliding_window
    rate: 100                  # 每秒请求数
    burst: 200                 # 突发容量
    
  # 安全配置
  security:
    enabled: true
    frame_deny: true
    content_type_nosniff: true
    xss_protection: true

  # 请求签名验证
  signature:
    enabled: false
    secret_key: your-secret-key
    expire_duration: 300       # 5分钟
    algorithm: HMAC-SHA256

# 监控配置
monitoring:
  # Prometheus 指标
  metrics:
    enabled: true
    path: /metrics
    namespace: gateway
    subsystem: http
    
  # 链路追踪
  tracing:
    enabled: false
    service_name: go-rpc-gateway
    endpoint: http://jaeger:14268/api/traces

# 数据库配置 (go-config)
mysql:
  path: 127.0.0.1
  port: "3306"
  config: charset=utf8mb4&parseTime=True&loc=Local
  db-name: gateway_db
  username: root
  password: ""
  max-idle-conns: 10
  max-open-conns: 100

# Redis 配置 (go-config)
redis:
  db: 0
  addr: 127.0.0.1:6379
  password: ""
  pool-size: 100

# 日志配置
logging:
  level: info                  # debug, info, warn, error, fatal
  format: json                 # json, text
  output: ["stdout", "file"]
  file_path: logs/gateway.log
  max_size: 100               # MB
  max_backups: 10
  max_age: 30                 # days
  compress: true
```

</details>

### 🔧 环境变量配置

支持通过环境变量覆盖配置项：

```bash
# 基本配置
export GATEWAY_HOST=0.0.0.0
export GATEWAY_HTTP_PORT=8080
export GATEWAY_GRPC_PORT=9090

# 数据库配置
export MYSQL_HOST=localhost
export MYSQL_PASSWORD=your_password

# Redis 配置  
export REDIS_ADDR=localhost:6379
export REDIS_PASSWORD=your_password

# 启动服务
./bin/gateway
```

### 🎛️ 配置优先级

1. **命令行参数** (最高优先级)
2. **环境变量**
3. **配置文件**
4. **默认值** (最低优先级)

## 🏗 架构设计

### 📂 项目结构 (重构后)

```
go-rpc-gateway/
├── 🎯 cmd/                    # 应用程序入口
│   └── gateway/
│       └── main.go           # 主程序入口
├── 🏗️ internal/               # 内部包（不对外暴露）
│   ├── config/               # 配置管理
│   │   ├── config.go        # 配置结构定义
│   │   └── manager.go       # 配置管理器
│   └── server/              # 服务器实现 [已重构]
│       ├── server.go        # 🔧 核心结构定义
│       ├── core.go          # 🛠️ 组件初始化
│       ├── grpc.go          # 📡 gRPC 服务器
│       ├── http.go          # 🌐 HTTP 网关
│       ├── middleware_init.go # 🔌 中间件初始化
│       ├── lifecycle.go     # 🔄 生命周期管理
│       └── README.md        # 📖 重构说明文档
├── 🔌 middleware/             # 中间件生态系统
│   ├── manager.go           # 中间件管理器
│   ├── access.go           # 访问日志
│   ├── metrics.go          # 监控指标
│   ├── security.go         # 安全防护
│   ├── ratelimit.go        # 流量控制
│   ├── recovery.go         # 异常恢复
│   ├── signature.go        # 签名验证
│   └── types.go            # 类型定义
├── 🔄 response/              # 统一响应处理
│   └── response.go         # 响应格式化
├── 📚 examples/              # 使用示例
│   ├── basic/              # 基础示例
│   ├── quickstart/         # 快速开始
│   ├── with-config/        # 配置文件示例
│   ├── with-logs/          # 日志示例
│   └── config.yaml         # 示例配置
├── 📝 docs/                  # 文档目录
│   └── BUSINESS_HANDLER_GUIDE.md
├── 🛠️ build scripts          # 构建脚本
│   ├── build.sh            # Unix 构建脚本
│   ├── build.bat           # Windows 构建脚本
│   ├── start.sh            # Unix 启动脚本
│   └── start.bat           # Windows 启动脚本
└── 📋 config/                # 配置文件模板
    └── example.yaml        # 配置示例
```

### 🎯 设计原则

<table>
<tr>
<td width="25%">

**🔧 模块化设计**
- 单一职责原则
- 松耦合架构
- 可插拔组件

</td>
<td width="25%">

**⚙️ 配置驱动**
- 配置文件控制
- 热重载支持
- 环境变量覆盖

</td>
<td width="25%">

**🔌 中间件架构**
- 管道式处理
- 链式调用
- 自定义扩展

</td>
<td width="25%">

**🔍 可观测性**
- 结构化日志
- 指标收集
- 链路追踪

</td>
</tr>
</table>

### 🔄 重构亮点

| 文件 | 行数 | 职责 | 优势 |
|------|------|------|------|
| `server.go` | ~99 | 核心结构定义 | 清晰的接口设计 |
| `core.go` | ~140 | 组件初始化 | 统一的初始化流程 |
| `grpc.go` | ~63 | gRPC 服务管理 | 专注 gRPC 逻辑 |
| `http.go` | ~112 | HTTP 网关管理 | 专注 HTTP 逻辑 |
| `lifecycle.go` | ~108 | 生命周期管理 | 优雅启停控制 |

> 📊 **重构效果**: 原 506 行的单一文件拆分为 6 个专业模块，提高了代码的可读性和可维护性。

## 🔧 中间件系统

### 📦 内置中间件

<table>
<tr>
<th>类别</th>
<th>中间件</th>
<th>功能描述</th>
<th>配置示例</th>
</tr>
<tr>
<td rowspan="4"><strong>🛡️ 安全</strong></td>
<td><code>Security</code></td>
<td>安全头设置、XSS防护</td>
<td><code>security.enabled: true</code></td>
</tr>
<tr>
<td><code>CORS</code></td>
<td>跨域资源共享控制</td>
<td><code>cors.allow_origins: ["*"]</code></td>
</tr>
<tr>
<td><code>Signature</code></td>
<td>请求签名验证</td>
<td><code>signature.algorithm: HMAC-SHA256</code></td>
</tr>
<tr>
<td><code>RequestID</code></td>
<td>请求ID生成和追踪</td>
<td>自动启用</td>
</tr>
<tr>
<td rowspan="3"><strong>📊 监控</strong></td>
<td><code>Metrics</code></td>
<td>Prometheus指标收集</td>
<td><code>metrics.enabled: true</code></td>
</tr>
<tr>
<td><code>Tracing</code></td>
<td>OpenTelemetry链路追踪</td>
<td><code>tracing.enabled: true</code></td>
</tr>
<tr>
<td><code>Logging</code></td>
<td>结构化访问日志</td>
<td><code>logging.level: info</code></td>
</tr>
<tr>
<td rowspan="2"><strong>🚦 控制</strong></td>
<td><code>RateLimit</code></td>
<td>智能流量控制</td>
<td><code>rate_limit.rate: 100</code></td>
</tr>
<tr>
<td><code>Recovery</code></td>
<td>Panic异常恢复</td>
<td>自动启用</td>
</tr>
</table>

### 🎨 自定义中间件开发

```go
package middleware

import (
    "net/http"
    "time"
)

// CustomAuthMiddleware 自定义认证中间件
func CustomAuthMiddleware(secret string) HTTPMiddleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 1. 获取认证头
            token := r.Header.Get("Authorization")
            
            // 2. 验证逻辑
            if !isValidToken(token, secret) {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            
            // 3. 添加用户信息到上下文
            ctx := context.WithValue(r.Context(), "user_id", getUserID(token))
            r = r.WithContext(ctx)
            
            // 4. 继续处理
            next.ServeHTTP(w, r)
        })
    }
}

// 注册自定义中间件
func (m *Manager) RegisterCustomMiddleware(middleware HTTPMiddleware) {
    m.customMiddlewares = append(m.customMiddlewares, middleware)
}
```

### 🔀 中间件链配置

```yaml
# 中间件执行顺序配置
middleware:
  order:
    - RequestID      # 1. 生成请求ID
    - Recovery       # 2. 异常恢复
    - Logging        # 3. 访问日志
    - CORS           # 4. 跨域处理
    - Security       # 5. 安全头
    - RateLimit      # 6. 流量控制
    - Signature      # 7. 签名验证
    - Metrics        # 8. 指标收集
    - CustomAuth     # 9. 自定义认证
```

## 📊 监控与可观测性

### 📈 Prometheus 指标

<details>
<summary>📊 查看完整指标列表</summary>

```
# HTTP 请求指标
gateway_http_requests_total{method="GET", status="200", path="/api/v1/users"}
gateway_http_request_duration_seconds{method="GET", path="/api/v1/users"}
gateway_http_request_size_bytes{method="POST", path="/api/v1/users"} 
gateway_http_response_size_bytes{method="GET", path="/api/v1/users"}

# gRPC 请求指标  
gateway_grpc_requests_total{service="UserService", method="GetUser", status="OK"}
gateway_grpc_request_duration_seconds{service="UserService", method="GetUser"}

# 业务指标
gateway_active_connections_total
gateway_middleware_duration_seconds{middleware="rate_limit"}
gateway_database_connections_active
gateway_redis_operations_total{operation="GET", status="success"}
```

</details>

### 💊 健康检查

```bash
# 基础健康检查
curl http://localhost:8080/health
# 响应: {"status":"ok","service":"go-rpc-gateway","timestamp":1699123456}

# 详细健康检查 (包含依赖服务状态)
curl http://localhost:8080/health?detail=true
# 响应示例:
{
  "status": "ok",
  "service": "go-rpc-gateway", 
  "timestamp": 1699123456,
  "checks": {
    "database": {"status": "ok", "latency_ms": 2},
    "redis": {"status": "ok", "latency_ms": 1},
    "external_api": {"status": "warning", "latency_ms": 1500}
  }
}
```

### 📊 指标采集端点

```bash
# Prometheus 指标采集
curl http://localhost:8080/metrics

# 自定义指标查询
curl http://localhost:8080/metrics?format=json
```

### � 链路追踪

配置 OpenTelemetry 链路追踪：

```yaml
monitoring:
  tracing:
    enabled: true
    service_name: go-rpc-gateway
    endpoint: http://jaeger:14268/api/traces
    sampling_rate: 0.1  # 10% 采样率
```

## 🔒 安全特性

### 🔐 请求签名验证

<details>
<summary>📝 查看签名验证实现</summary>

```yaml
# 配置签名验证
middleware:
  signature:
    enabled: true
    secret_key: "your-256-bit-secret"
    expire_duration: 300  # 5分钟
    algorithm: HMAC-SHA256
    fields:
      - timestamp
      - request_id  
      - body_hash
```

**客户端签名生成示例:**

```go
func generateSignature(secretKey, method, uri, body string, timestamp int64) string {
    // 1. 构建签名字符串
    signString := fmt.Sprintf("%s\n%s\n%s\n%d", 
        method, uri, body, timestamp)
    
    // 2. HMAC-SHA256 签名
    h := hmac.New(sha256.New, []byte(secretKey))
    h.Write([]byte(signString))
    
    // 3. Base64 编码
    return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
```

**请求头设置:**

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -H "X-Timestamp: 1699123456" \
  -H "X-Signature: generated_signature_here" \
  -d '{"name":"test"}'
```

</details>

### 🛡️ 安全防护

| 安全特性 | 说明 | 配置 |
|----------|------|------|
| **XSS 防护** | 跨站脚本攻击防护 | `security.xss_protection: true` |
| **CSRF 保护** | 跨站请求伪造保护 | `security.csrf_protection: true` |
| **内容嗅探防护** | 防止MIME类型混淆攻击 | `security.content_type_nosniff: true` |
| **点击劫持防护** | X-Frame-Options头设置 | `security.frame_deny: true` |
| **HTTPS 强制** | 强制HTTPS重定向 | `security.force_https: true` |

## 🚀 部署指南

### 🐳 Docker 部署

<details>
<summary>📦 查看完整 Docker 配置</summary>

**多阶段构建 Dockerfile:**

```dockerfile
# ===========================================
# 多阶段构建，优化镜像大小
# ===========================================

# 构建阶段
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git ca-certificates tzdata

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o gateway cmd/gateway/main.go

# ===========================================
# 运行阶段
# ===========================================

FROM alpine:latest

# 安装必要的包
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 创建应用用户
RUN addgroup -g 1001 app && \
    adduser -u 1001 -G app -s /bin/sh -D app

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/gateway .
COPY --from=builder /app/config/example.yaml ./config.yaml

# 创建日志目录
RUN mkdir -p logs && chown -R app:app /app

# 切换到应用用户
USER app

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

# 暴露端口
EXPOSE 8080 9090

# 启动应用
CMD ["./gateway", "-config", "config.yaml"]
```

**Docker Compose 配置:**

```yaml
# docker-compose.yml
version: '3.8'

services:
  gateway:
    build: .
    ports:
      - "8080:8080"   # HTTP
      - "9090:9090"   # gRPC
    environment:
      - GATEWAY_ENV=production
      - MYSQL_HOST=mysql
      - REDIS_ADDR=redis:6379
    volumes:
      - ./logs:/app/logs
      - ./config/production.yaml:/app/config.yaml:ro
    depends_on:
      - mysql
      - redis
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: gateway123
      MYSQL_DATABASE: gateway_db
    ports:
      - "3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    restart: unless-stopped

volumes:
  mysql_data:
  redis_data:
```

</details>

### ☸️ Kubernetes 部署

<details>
<summary>🎛️ 查看 K8s 完整配置</summary>

**Deployment 配置:**

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: go-rpc-gateway
  labels:
    app: gateway
    version: v1.0.0
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
  selector:
    matchLabels:
      app: gateway
  template:
    metadata:
      labels:
        app: gateway
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      containers:
      - name: gateway
        image: your-registry/go-rpc-gateway:latest
        ports:
        - name: http
          containerPort: 8080
          protocol: TCP
        - name: grpc
          containerPort: 9090
          protocol: TCP
        env:
        - name: GATEWAY_ENV
          value: "production"
        - name: MYSQL_HOST
          value: "mysql-service"
        - name: REDIS_ADDR
          value: "redis-service:6379"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          limits:
            cpu: 500m
            memory: 512Mi
          requests:
            cpu: 100m
            memory: 128Mi
        volumeMounts:
        - name: config
          mountPath: /app/config.yaml
          subPath: config.yaml
          readOnly: true
        - name: logs
          mountPath: /app/logs
      volumes:
      - name: config
        configMap:
          name: gateway-config
      - name: logs
        emptyDir: {}

---
# Service 配置
apiVersion: v1
kind: Service
metadata:
  name: gateway-service
  labels:
    app: gateway
spec:
  type: ClusterIP
  ports:
  - name: http
    port: 8080
    targetPort: 8080
  - name: grpc
    port: 9090
    targetPort: 9090
  selector:
    app: gateway

---
# Ingress 配置
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gateway-ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  tls:
  - hosts:
    - gateway.example.com
    secretName: gateway-tls
  rules:
  - host: gateway.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: gateway-service
            port:
              number: 8080

---
# ConfigMap
apiVersion: v1
kind: ConfigMap
metadata:
  name: gateway-config
data:
  config.yaml: |
    gateway:
      name: go-rpc-gateway
      environment: production
      debug: false
    # ... 其他配置
```

</details>

### 🎯 生产环境最佳实践

<table>
<tr>
<th width="25%">🔧 性能优化</th>
<th width="25%">🛡️ 安全加固</th>
<th width="25%">📊 监控告警</th>
<th width="25%">🔄 高可用</th>
</tr>
<tr>
<td>

- 连接池调优
- 内存/CPU限制
- 垃圾回收优化
- 缓存策略

</td>
<td>

- HTTPS 强制
- 安全头设置
- 访问控制
- 敏感信息保护

</td>
<td>

- Prometheus 指标
- 日志聚合
- 告警规则
- 性能基线

</td>
<td>

- 多实例部署
- 负载均衡
- 健康检查
- 故障转移

</td>
</tr>
</table>

## 📚 完整示例

### 🎯 快速体验项目

```bash
# 1. 克隆项目
git clone https://github.com/kamalyes/go-rpc-gateway.git
cd go-rpc-gateway

# 2. 查看示例
ls examples/
# basic/          - 基础示例，零配置启动
# quickstart/     - 快速开始，5分钟上手
# with-config/    - 配置文件示例
# with-logs/      - 日志系统示例

# 3. 运行基础示例
cd examples/basic
go run main.go

# 4. 测试服务
curl http://localhost:8080/health
```

### 🎨 业务集成示例

<details>
<summary>💼 查看完整业务代码示例</summary>

```go
// examples/business/main.go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/kamalyes/go-rpc-gateway/internal/server"
    "github.com/kamalyes/go-rpc-gateway/internal/config"
    
    // 引入你的业务 proto
    pb "your-project/api/proto/user/v1"
)

// UserService 实现你的业务逻辑
type UserService struct {
    pb.UnimplementedUserServiceServer
    // 注入数据库、缓存等依赖
}

func (s *UserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
    // 实现业务逻辑
    return &pb.GetUserResponse{
        User: &pb.User{
            Id:    req.Id,
            Name:  "示例用户",
            Email: "user@example.com",
        },
    }, nil
}

func main() {
    // 1. 创建配置管理器
    configManager := config.NewConfigManager("config.yaml")
    
    // 2. 创建服务器
    srv, err := server.NewServerWithConfigManager(configManager)
    if err != nil {
        log.Fatal("创建服务器失败:", err)
    }

    // 3. 注册 gRPC 服务
    userService := &UserService{}
    srv.RegisterGRPCService(func(s *grpc.Server) {
        pb.RegisterUserServiceServer(s, userService)
    })

    // 4. 注册 HTTP 网关
    ctx := context.Background()
    err = srv.RegisterHTTPHandler(ctx, pb.RegisterUserServiceHandlerFromEndpoint)
    if err != nil {
        log.Fatal("注册HTTP处理器失败:", err)
    }

    // 5. 启动服务器
    go func() {
        log.Println("🚀 启动 Gateway 服务器...")
        if err := srv.Start(); err != nil {
            log.Fatal("启动失败:", err)
        }
    }()

    // 6. 优雅关闭
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Println("🛑 正在关闭服务器...")
    if err := srv.Shutdown(); err != nil {
        log.Printf("关闭服务器失败: %v", err)
    }
    log.Println("✅ 服务器已关闭")
}
```

</details>

### 🔗 相关链接

| 类型 | 链接 | 描述 |
|------|------|------|
| **📖 文档** | [完整文档](docs/) | 详细的使用文档和最佳实践 |
| **🎯 示例** | [examples/](examples/) | 各种场景的完整示例代码 |
| **🐛 问题反馈** | [GitHub Issues](https://github.com/kamalyes/go-rpc-gateway/issues) | Bug 报告和功能请求 |
| **💬 讨论区** | [GitHub Discussions](https://github.com/kamalyes/go-rpc-gateway/discussions) | 技术讨论和经验分享 |
| **📋 更新日志** | [CHANGELOG.md](CHANGELOG.md) | 版本更新记录 |

## 🤝 贡献指南

我们欢迎所有形式的贡献！请查看我们的 [贡献指南](CONTRIBUTING.md) 了解如何参与。

### 🏗️ 开发环境设置

```bash
# 1. Fork 项目并克隆
git clone https://github.com/your-username/go-rpc-gateway.git
cd go-rpc-gateway

# 2. 安装依赖
go mod download

# 3. 运行测试
go test ./...

# 4. 构建项目
./build.sh

# 5. 运行示例
./bin/gateway -config examples/config.yaml
```

### ✅ 提交规范

我们使用 [Conventional Commits](https://conventionalcommits.org/) 规范：

```
feat: 添加新的中间件支持
fix: 修复配置热重载问题
docs: 更新 README 文档
style: 代码格式化
refactor: 重构服务器启动逻辑
test: 添加中间件单元测试
chore: 更新依赖版本
```

## 📄 许可证

本项目采用 [MIT 许可证](LICENSE)，您可以自由使用、修改和分发。

---

## 🙏 致谢

感谢以下优秀的开源项目：

<table>
<tr>
<td align="center">
  <a href="https://github.com/kamalyes/go-config">
    <img src="https://via.placeholder.com/64x64.png?text=CONFIG" width="64" height="64">
    <br>
    <strong>go-config</strong>
  </a>
  <br>
  <sub>统一配置管理</sub>
</td>
<td align="center">
  <a href="https://github.com/kamalyes/go-core">
    <img src="https://via.placeholder.com/64x64.png?text=CORE" width="64" height="64">
    <br>
    <strong>go-core</strong>
  </a>
  <br>
  <sub>核心功能库</sub>
</td>
<td align="center">
  <a href="https://github.com/grpc-ecosystem/grpc-gateway">
    <img src="https://via.placeholder.com/64x64.png?text=gRPC" width="64" height="64">
    <br>
    <strong>grpc-gateway</strong>
  </a>
  <br>
  <sub>gRPC 网关</sub>
</td>
<td align="center">
  <a href="https://github.com/prometheus/client_golang">
    <img src="https://via.placeholder.com/64x64.png?text=PROM" width="64" height="64">
    <br>
    <strong>Prometheus</strong>
  </a>
  <br>
  <sub>监控指标</sub>
</td>
</tr>
</table>

---

<div align="center">

**⭐ 如果这个项目对您有帮助，请给我们一个 Star！**

[![Star History Chart](https://api.star-history.com/svg?repos=kamalyes/go-rpc-gateway&type=Date)](https://star-history.com/#kamalyes/go-rpc-gateway&Date)

</div>