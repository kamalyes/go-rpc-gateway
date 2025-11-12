# 🔍 /health 端点无法访问的问题排查

## ❓ 问题描述

使用 `NewWithConfigFile` 创建网关后，`/health` 端点无法访问。

## 🔎 原因分析

问题出在**配置文件中的 HealthCheck 配置**。当使用配置文件时，如果没有正确配置健康检查选项，默认值可能不会被正确应用。

### 根本原因

1. **配置文件未设置健康检查** - 如果配置文件中没有 `health_check` 配置项
2. **Enabled 默认为 false** - YAML 解析时，未设置的布尔值默认为 `false`
3. **路径未配置** - 健康检查路径可能为空

## ✅ 解决方案

### 方案一：在配置文件中显式启用健康检查（推荐）

编辑你的 `config.yaml`，添加以下配置：

```yaml
# 网关配置
gateway:
  name: "go-rpc-gateway"
  version: "v1.0.0"
  environment: "development"
  debug: true
  
  # HTTP 服务器配置
  http:
    host: "0.0.0.0"
    port: 8080
    read_timeout: 30
    write_timeout: 30
    idle_timeout: 120
  
  # gRPC 服务器配置
  grpc:
    host: "0.0.0.0"
    port: 9090
    max_recv_msg_size: 4194304
    max_send_msg_size: 4194304
  
  # ⚠️ 重点：显式启用健康检查
  health_check:
    enabled: true          # 必须设置为 true
    path: "/health"        # 健康检查路径
    interval: 30           # 检查间隔（秒）
    
    # 可选：组件级健康检查
    redis:
      enabled: false       # 如果使用 Redis，设置为 true
      host: "localhost"
      port: 6379
      timeout: 5
    
    mysql:
      enabled: false       # 如果使用 MySQL，设置为 true
      host: "localhost"
      port: 3306
      database: "test"
      timeout: 5

# 监控配置（可选）
monitoring:
  metrics:
    enabled: true
    path: "/metrics"
```

### 方案二：最小配置文件

如果你只需要基本功能，使用最小配置：

```yaml
gateway:
  health_check:
    enabled: true
    path: "/health"
```

### 方案三：使用 New() 而不是 NewWithConfigFile()

如果不需要配置文件，直接使用默认配置：

```go
package main

import gateway "github.com/kamalyes/go-rpc-gateway"

func main() {
    // 使用默认配置（健康检查自动启用）
    gw, err := gateway.New()
    if err != nil {
        panic(err)
    }
    
    gw.Start()
}
```

### 方案四：代码中检查配置

在启动前检查配置是否正确：

```go
package main

import (
    "fmt"
    gateway "github.com/kamalyes/go-rpc-gateway"
)

func main() {
    gw, err := gateway.NewWithConfigFile("config.yaml")
    if err != nil {
        panic(err)
    }
    
    // 检查健康检查是否启用
    config := gw.GetConfig()
    fmt.Printf("健康检查启用: %v\n", config..HealthCheck.Enabled)
    fmt.Printf("健康检查路径: %s\n", config..HealthCheck.Path)
    
    if !config..HealthCheck.Enabled {
        fmt.Println("⚠️  警告：健康检查未启用！")
        fmt.Println("请在 config.yaml 中设置:")
        fmt.Println("gateway:")
        fmt.Println("  health_check:")
        fmt.Println("    enabled: true")
        fmt.Println("    path: \"/health\"")
    }
    
    gw.Start()
}
```

## 🧪 测试步骤

### 1. 创建完整的配置文件

```bash
# 复制模板配置文件
cp template/config.yaml config.yaml
```

### 2. 确保健康检查配置正确

编辑 `config.yaml`，确保包含：

```yaml
gateway:
  health_check:
    enabled: true
    path: "/health"
```

### 3. 启动服务

```bash
go run main.go
```

### 4. 测试健康检查端点

```bash
# 方式1: 使用 curl
curl http://localhost:8080/health

# 方式2: 使用浏览器
# 打开 http://localhost:8080/health

# 方式3: 使用 PowerShell
Invoke-WebRequest -Uri http://localhost:8080/health
```

### 预期响应

```json
{
  "status": "healthy",
  "timestamp": "2024-11-10T12:00:00Z",
  "version": "v1.0.0",
  "service": "go-rpc-gateway",
  "components": {
    "database": "not_configured",
    "redis": "not_configured"
  }
}
```

## 📋 完整的测试示例

创建 `test-health.go`：

```go
package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

	gateway "github.com/kamalyes/go-rpc-gateway"
)

func main() {
	// 1. 创建网关
	gw, err := gateway.NewWithConfigFile("config.yaml")
	if err != nil {
		panic(err)
	}

	// 2. 检查配置
	config := gw.GetConfig()
	fmt.Println("=== 配置检查 ===")
	fmt.Printf("健康检查启用: %v\n", config..HealthCheck.Enabled)
	fmt.Printf("健康检查路径: %s\n", config..HealthCheck.Path)
	fmt.Printf("HTTP 端口: %d\n", config..HTTP.Port)
	fmt.Println()

	// 3. 启动服务（在 goroutine 中）
	go func() {
		if err := gw.Start(); err != nil {
			panic(err)
		}
	}()

	// 4. 等待服务启动
	time.Sleep(2 * time.Second)

	// 5. 测试健康检查端点
	testHealthEndpoint(config..HTTP.Port, config..HealthCheck.Path)

	// 6. 保持运行
	select {}
}

func testHealthEndpoint(port int, path string) {
	url := fmt.Sprintf("http://localhost:%d%s", port, path)
	
	fmt.Println("=== 测试健康检查端点 ===")
	fmt.Printf("请求 URL: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		fmt.Println("\n可能的原因：")
		fmt.Println("1. 服务未启动")
		fmt.Println("2. 端口被占用")
		fmt.Println("3. 健康检查未启用")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ 读取响应失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 状态码: %d\n", resp.StatusCode)
	fmt.Printf("✅ 响应内容:\n%s\n", string(body))
}
```

运行测试：

```bash
go run test-health.go
```

## 🔧 常见问题

### Q1: 配置文件正确但仍无法访问

**检查项：**
```bash
# 1. 检查端口是否被占用
netstat -ano | findstr :8080

# 2. 检查防火墙
# Windows PowerShell (管理员)
New-NetFirewallRule -DisplayName "Go Gateway" -Direction Inbound -LocalPort 8080 -Protocol TCP -Action Allow

# 3. 尝试使用 127.0.0.1 而不是 localhost
curl http://127.0.0.1:8080/health
```

### Q2: 配置文件找不到

```go
// 使用绝对路径
gw, err := gateway.NewWithConfigFile("E:\\path\\to\\config.yaml")

// 或检查当前目录
import "os"
fmt.Println("当前目录:", os.Getwd())
```

### Q3: 健康检查总是返回 404

**可能原因：**
1. 配置文件中 `enabled: false`
2. 路径配置错误（如 `path: "health"` 而不是 `path: "/health"`）
3. 中间件拦截了请求

**解决：**
```yaml
gateway:
  health_check:
    enabled: true
    path: "/health"  # 注意前面的斜杠
```

## 📝 推荐的完整配置文件模板

创建 `config-full.yaml`：

```yaml
# ================================================
# Go RPC Gateway 完整配置文件
# ================================================

# 网关基础配置
gateway:
  name: "go-rpc-gateway"
  version: "v1.0.0"
  environment: "development"  # development, testing, production
  debug: true

  # HTTP 服务器
  http:
    host: "0.0.0.0"
    port: 8080
    read_timeout: 30
    write_timeout: 30
    idle_timeout: 120
    max_header_bytes: 1048576
    enable_gzip_compress: true

  # gRPC 服务器
  grpc:
    host: "0.0.0.0"
    port: 9090
    network: "tcp"
    max_recv_msg_size: 4194304  # 4MB
    max_send_msg_size: 4194304  # 4MB
    connection_timeout: 30
    keepalive_time: 60
    keepalive_timeout: 30
    enable_reflection: true

  # ⭐ 健康检查（重要）
  health_check:
    enabled: true               # 必须设置为 true
    path: "/health"            # 健康检查端点
    interval: 30               # 检查间隔（秒）
    
    # Redis 健康检查
    redis:
      enabled: false
      host: "localhost"
      port: 6379
      timeout: 5
    
    # MySQL 健康检查
    mysql:
      enabled: false
      host: "localhost"
      port: 3306
      database: "test"
      timeout: 5

# 中间件配置
middleware:
  # 访问日志
  access_log:
    enabled: true
    format: "json"
    include_body: false
    include_headers: false

  # 限流控制
  rate_limit:
    enabled: true
    algorithm: "token_bucket"
    rate: 100
    burst: 200
    window_size: 60

  # 签名验证
  signature:
    enabled: false
    algorithm: "hmac-sha256"
    skip_paths:
      - "/health"
      - "/metrics"
    ttl: 300

# 安全配置
security:
  # CORS
  cors:
    allowed_origins: ["*"]
    allowed_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allowed_headers: ["*"]
    max_age: "86400"
    allow_credentials: true

  # TLS (可选)
  tls:
    enabled: false
    cert_file: ""
    key_file: ""

# 监控配置
monitoring:
  # Prometheus 指标
  metrics:
    enabled: true
    path: "/metrics"

  # 链路追踪
  tracing:
    enabled: false
    service_name: "go-rpc-gateway"

# 日志配置
logger:
  level: "info"
  format: "console"
  prefix: "[GO-RPC-GATEWAY]"
  director: "logs"
  show_line: true
  encode_level: "LowercaseColorLevelEncoder"
  log_in_console: true

# 数据库配置（可选）
# mysql:
#   host: "localhost"
#   port: 3306
#   dbname: "test"
#   username: "root"
#   password: "password"

# Redis 配置（可选）
# redis:
#   host: "localhost"
#   port: 6379
#   password: ""
#   db: 0
```

## ✅ 总结

使用 `NewWithConfigFile` 时，**必须在配置文件中显式启用健康检查**：

```yaml
gateway:
  health_check:
    enabled: true    # ⚠️ 关键：必须设置为 true
    path: "/health"
```

如果不想配置文件，可以直接使用 `gateway.New()`，它会使用默认配置（健康检查已启用）。

---

**问题解决了吗？如果还有问题，请检查上述所有步骤！** 🎯
