# Constants Package

这个包包含了Go RPC Gateway项目中使用的所有常量定义，避免硬编码并提高代码的可维护性。

## 文件结构

```
constants/
├── gateway.go          # 网关相关常量
├── headers.go         # HTTP头部常量
└── README.md          # 本文件
```

## 常量分类

### gateway.go - 网关常量

定义了网关的基础配置常量：

- `DefaultServiceName` - 默认服务名称
- `DefaultHealthPath` - 健康检查路径
- `DefaultMetricsPath` - 监控指标路径  
- `DefaultDebugPath` - 调试路径

### headers.go - HTTP头部常量

#### 标准请求头
- Content-Type、Authorization、User-Agent等标准HTTP头

#### 自定义请求头
- X-Request-Id、X-Trace-Id、X-Real-IP等追踪相关头部
- X-Device-Id、X-App-Version等设备和应用相关头部
- X-Timestamp、X-Signature等安全相关头部

#### 安全响应头
- X-Frame-Options、X-Content-Type-Options等安全防护头部
- Strict-Transport-Security、Content-Security-Policy等

#### MIME类型常量
- application/json、application/xml等常用MIME类型
- 图片、文本等各种媒体类型

## 使用方式

```go
import "github.com/kamalyes/go-rpc-gateway/constants"

// 使用网关常量
serviceName := constants.DefaultServiceName
healthPath := constants.DefaultHealthPath

// 使用HTTP头部常量
w.Header().Set(constants.HeaderContentType, constants.MimeApplicationJSON)
requestID := r.Header.Get(constants.HeaderXRequestID)
```

## 设计原则

1. **集中管理**: 所有常量统一定义，避免分散在各处
2. **语义清晰**: 常量名称具有明确的语义，易于理解
3. **分类组织**: 按功能分类，便于查找和维护
4. **避免硬编码**: 替换代码中的魔数和硬编码字符串

## 添加新常量

当需要添加新常量时：

1. 选择合适的文件（或创建新文件）
2. 遵循现有的命名规范
3. 添加适当的注释说明
4. 更新相关文档

例如：
```go
// HTTP 状态相关常量
const (
    StatusOK              = "200"
    StatusBadRequest      = "400" 
    StatusInternalError   = "500"
)
```