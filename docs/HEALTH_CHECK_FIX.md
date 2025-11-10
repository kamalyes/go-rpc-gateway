# 🔍 健康检查无法访问问题 - 完整排查指南

## ❓ 你的问题

使用以下代码启动服务后，无法访问 `/health` 端点：

```go
gw, err := gateway.NewWithConfigFile("config.yaml")
if err != nil {
    log.Fatalf("使用配置文件创建Gateway失败: %v", err)
}

// 启动服务
go func() {
    if err := gw.Start(); err != nil {
        log.Printf("服务器启动失败: %v", err)
    }
}()
```

## 🎯 解决方案

### 步骤 1：检查你的配置文件

**问题根源：** 配置文件中 `health_check.enabled` 必须显式设置为 `true`

#### ✅ 正确的配置

在你的 `config.yaml` 中确保包含以下内容：

```yaml
gateway:
  health_check:
    enabled: true        # ⚠️ 必须显式设置为 true
    path: "/health"      # 健康检查路径
    interval: 30         # 可选：检查间隔
```

#### ❌ 错误的配置

```yaml
# 错误1：缺少 health_check 配置
gateway:
  name: "service"
  # 没有 health_check 配置

# 错误2：enabled 未设置或设置为 false
gateway:
  health_check:
    # enabled 未设置（默认为 false）
    path: "/health"

# 错误3：path 缺少前导斜杠
gateway:
  health_check:
    enabled: true
    path: "health"  # 错误：应该是 "/health"
```

### 步骤 2：使用最小配置文件测试

创建一个最小的 `config-minimal.yaml`：

```yaml
gateway:
  health_check:
    enabled: true
    path: "/health"
```

然后测试：

```bash
go run main.go
curl http://localhost:8080/health
```

### 步骤 3：使用诊断工具

我已经创建了一个诊断工具，运行它来检查配置：

```bash
# 在 examples 目录下
go run diagnose-health.go config.yaml
```

这个工具会：
- ✅ 检查配置文件是否存在
- ✅ 检查健康检查配置是否正确
- ✅ 自动启动服务并测试端点
- ✅ 显示详细的错误信息和解决方案

### 步骤 4：在代码中添加配置检查

修改你的 `main.go`：

```go
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	gateway "github.com/kamalyes/go-rpc-gateway"
	"github.com/kamalyes/go-core/pkg/global"
)

func main() {
	// 1. 使用配置文件创建Gateway实例
	gw, err := gateway.NewWithConfigFile("config.yaml")
	if err != nil {
		log.Fatalf("使用配置文件创建Gateway失败: %v", err)
	}

	// 2. 检查健康检查配置
	config := gw.GetConfig()
	global.LOGGER.Info("=== 配置检查 ===")
	global.LOGGER.InfoKV("健康检查",
		"enabled", config.Gateway.HealthCheck.Enabled,
		"path", config.Gateway.HealthCheck.Path,
	)

	if !config.Gateway.HealthCheck.Enabled {
		global.LOGGER.Fatal("❌ 健康检查未启用！请在 config.yaml 中设置 gateway.health_check.enabled: true")
	}

	// 3. 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 4. 启动服务器
	go func() {
		if err := gw.Start(); err != nil {
			log.Printf("服务器启动失败: %v", err)
		}
	}()

	global.LOGGER.InfoKV("服务已启动",
		"health_url", "http://localhost:8080/health",
		"metrics_url", "http://localhost:8080/metrics",
	)

	// 5. 等待关闭信号
	<-sigChan
	log.Println("🛑 正在优雅关闭服务器...")

	if err := gw.Stop(); err != nil {
		log.Printf("关闭服务器时出错: %v", err)
	}
	log.Println("✅ 服务器已成功关闭")
}
```

## 📝 完整的配置文件示例

### 方案 A：最小配置（推荐测试使用）

`config-minimal.yaml`:
```yaml
gateway:
  http:
    port: 8080
  grpc:
    port: 9090
  health_check:
    enabled: true
    path: "/health"
```

### 方案 B：标准配置（推荐开发使用）

`config-standard.yaml`:
```yaml
gateway:
  name: "IM-Push-Service"
  version: "v1.0.0"
  environment: "development"
  debug: true

  http:
    host: "0.0.0.0"
    port: 8080
    read_timeout: 30
    write_timeout: 30

  grpc:
    host: "0.0.0.0"
    port: 9090
    enable_reflection: true

  health_check:
    enabled: true
    path: "/health"
    interval: 30

monitoring:
  metrics:
    enabled: true
    path: "/metrics"

middleware:
  access_log:
    enabled: true
  rate_limit:
    enabled: true
    rate: 100
```

### 方案 C：生产配置

`config-production.yaml`:
```yaml
gateway:
  name: "IM-Push-Service"
  version: "v1.0.0"
  environment: "production"
  debug: false

  http:
    host: "0.0.0.0"
    port: 8080
    read_timeout: 30
    write_timeout: 30
    idle_timeout: 120

  grpc:
    host: "0.0.0.0"
    port: 9090
    enable_reflection: false

  health_check:
    enabled: true
    path: "/health"
    interval: 30
    
    redis:
      enabled: true
      host: "localhost"
      port: 6379
      timeout: 5
    
    mysql:
      enabled: true
      host: "localhost"
      port: 3306
      database: "im_service"
      timeout: 5

security:
  tls:
    enabled: true
    cert_file: "certs/server.crt"
    key_file: "certs/server.key"

monitoring:
  metrics:
    enabled: true
    path: "/metrics"
  tracing:
    enabled: true

# 数据库配置
mysql:
  host: "localhost"
  port: 3306
  dbname: "im_service"
  username: "root"
  password: "password"

# Redis 配置
redis:
  host: "localhost"
  port: 6379
```

## 🧪 测试步骤

### 1. 准备配置文件

```bash
# 复制简化配置
cp examples/config-simple.yaml config.yaml
```

### 2. 启动服务

```bash
go run main.go
```

### 3. 测试健康检查

```bash
# 方式 1: curl
curl http://localhost:8080/health

# 方式 2: PowerShell
Invoke-WebRequest -Uri http://localhost:8080/health

# 方式 3: 浏览器
# 访问 http://localhost:8080/health
```

### 4. 预期响应

```json
{
  "status": "healthy",
  "timestamp": "2024-11-10T12:00:00Z",
  "version": "v1.0.0",
  "service": "IM-Push-Service",
  "uptime": "5m30s"
}
```

## 🔧 常见问题排查

### Q1: 配置正确但仍然无法访问

**检查清单：**

```bash
# 1. 确认服务是否真的启动了
# 查看日志输出，应该看到类似：
# ✅ HTTP服务器已启动: http://0.0.0.0:8080
# ✅ 健康检查已启用: /health

# 2. 检查端口是否被占用
netstat -ano | findstr :8080

# 3. 尝试使用 127.0.0.1
curl http://127.0.0.1:8080/health

# 4. 检查防火墙
# Windows PowerShell (管理员)
New-NetFirewallRule -DisplayName "IM-Push" -Direction Inbound -LocalPort 8080 -Protocol TCP -Action Allow
```

### Q2: 服务启动但没有日志输出

**原因：** 使用 `go func()` 启动可能导致日志不可见

**解决：**

```go
// 方式 1: 不使用 goroutine（推荐简单场景）
if err := gw.Start(); err != nil {
    log.Fatalf("服务器启动失败: %v", err)
}

// 方式 2: 使用 channel 传递错误
errChan := make(chan error, 1)
go func() {
    errChan <- gw.Start()
}()

// 等待启动或错误
select {
case err := <-errChan:
    if err != nil {
        log.Fatalf("服务器启动失败: %v", err)
    }
case <-time.After(2 * time.Second):
    log.Println("服务器启动成功")
}
```

### Q3: 配置文件找不到

**检查当前目录：**

```go
import (
	"os"
	"path/filepath"
)

// 添加到 main 函数开头
currentDir, _ := os.Getwd()
log.Printf("当前目录: %s", currentDir)

configPath := "config.yaml"
absPath, _ := filepath.Abs(configPath)
log.Printf("配置文件路径: %s", absPath)

if _, err := os.Stat(configPath); os.IsNotExist(err) {
    log.Fatalf("配置文件不存在: %s", absPath)
}
```

### Q4: YAML 格式错误

**常见错误：**

```yaml
# 错误：缩进不正确
gateway:
health_check:  # 应该缩进
  enabled: true

# 错误：使用 Tab 而不是空格
gateway:
	health_check:  # YAML 不允许 Tab，必须用空格

# 错误：冒号后缺少空格
gateway:
  health_check:
    enabled:true  # 应该是 "enabled: true"
```

**验证 YAML：**

```bash
# 使用在线工具验证
# https://www.yamllint.com/

# 或使用 Python
python -c "import yaml; yaml.safe_load(open('config.yaml'))"
```

## 📊 对比：使用配置文件 vs 默认配置

### 使用默认配置（无配置文件）

```go
// 健康检查自动启用
gw, _ := gateway.New()
gw.Start()

// 访问 http://localhost:8080/health ✅ 工作正常
```

### 使用配置文件

```go
// 必须在配置文件中显式启用
gw, _ := gateway.NewWithConfigFile("config.yaml")
gw.Start()

// 访问 http://localhost:8080/health 
// ❌ 如果 config.yaml 中未设置 enabled: true
// ✅ 如果 config.yaml 中设置了 enabled: true
```

## 🎯 推荐的最佳实践

### 1. 开发环境

使用 `config-simple.yaml` 并添加必要配置：

```yaml
gateway:
  environment: "development"
  debug: true
  health_check:
    enabled: true
    path: "/health"
```

### 2. 测试环境

使用完整配置但禁用不必要的功能：

```yaml
gateway:
  environment: "test"
  debug: false
  health_check:
    enabled: true
    path: "/health"
    redis:
      enabled: true
    mysql:
      enabled: true
```

### 3. 生产环境

使用完整安全配置：

```yaml
gateway:
  environment: "production"
  debug: false
  health_check:
    enabled: true
    path: "/health"
security:
  tls:
    enabled: true
```

## 📁 文件清单

我已经为你创建了以下文件：

1. **examples/config-simple.yaml** - 最小配置（推荐开始使用）
2. **examples/diagnose-health.go** - 健康检查诊断工具
3. **examples/test-health-config.go** - 配置测试工具

## 🚀 快速修复

如果你现在就想让它工作，最快的方法：

```bash
# 1. 进入你的项目目录
cd your-project

# 2. 创建最小配置文件
cat > config.yaml << 'EOF'
gateway:
  health_check:
    enabled: true
    path: "/health"
EOF

# 3. 运行程序
go run main.go

# 4. 测试
curl http://localhost:8080/health
```

---

**问题解决了吗？** 如果还有问题，请运行诊断工具：

```bash
go run examples/diagnose-health.go config.yaml
```

它会告诉你具体哪里出了问题！🔍
