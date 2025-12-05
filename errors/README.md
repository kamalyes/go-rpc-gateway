# 错误管理系统使用指南

## 📋 概述

go-rpc-gateway 采用**纯错误码模式**的企业级错误管理系统，所有错误消息集中管理，代码中只使用错误码。

## 🎯 核心优势

1. **统一管理**：所有错误码和消息集中定义
2. **类型安全**：使用常量避免字符串硬编码
3. **自动映射**：错误码自动转换为 HTTP/gRPC 状态码
4. **国际化友好**：消息与代码分离，便于多语言支持
5. **错误链支持**：保留原始错误信息

## 📖 使用方式

### 1. 创建新错误（无上下文）

```go
// 直接使用预定义错误
return errors.ErrTokenExpired

// 使用错误码创建（无details）
return errors.NewError(errors.ErrCodeInvalidConfiguration, "")

// 使用错误码创建（带details）
return errors.NewError(errors.ErrCodeResourceNotFound, userID)
```

### 2. 包装现有错误（推荐）

```go
// ✅ 推荐：纯错误码模式（不传消息）
if err := someOperation(); err != nil {
    return errors.WrapWithContext(err, errors.ErrCodeOperationFailed)
}

// ✅ 备选：需要额外上下文时
if err := someOperation(); err != nil {
    return errors.Wrapf(err, errors.ErrCodeOperationFailed, "additional context")
}

// ❌ 避免：不要在代码中硬编码消息
if err := someOperation(); err != nil {
    return errors.Wrapf(err, errors.ErrCodeOperationFailed, "操作失败") // 不推荐
}
```

### 3. 格式化错误（带参数）

```go
// 当需要动态参数时
return errors.NewErrorf(errors.ErrCodeResourceNotFound, "user_id: %s", userID)
```

## 🔧 错误码分类

### 网关核心错误 (1000-1999)
```go
errors.ErrCodeGatewayNotInitialized  // 1001: Gateway not initialized
errors.ErrCodeInvalidConfiguration   // 1002: Invalid configuration
errors.ErrCodeServiceUnavailable     // 1003: Service unavailable
```

### 认证授权错误 (2000-2999)
```go
errors.ErrCodeUnauthorized           // 2001: Unauthorized
errors.ErrCodeTokenExpired           // 2004: Token expired
errors.ErrCodeAccountLoginElsewhere  // 2103: 账号已在其他地方登录
```

### 请求处理错误 (3000-3999)
```go
errors.ErrCodeBadRequest             // 3001: Bad request
errors.ErrCodeNotFound               // 3002: Not found
errors.ErrCodeInvalidParameter       // 3006: Invalid parameter
```

### 配置和特性错误 (1100-1199)
```go
errors.ErrCodeInvalidConfigType      // 1101: 无效的配置类型
errors.ErrCodeFeatureNotRegistered   // 1102: 特性未注册
errors.ErrCodeMiddlewareInitFailed   // 1104: 中间件初始化失败
```

## 💡 最佳实践

### ✅ 推荐做法

```go
// 1. 使用纯错误码
func CreateServer() (*Server, error) {
    baseServer, err := NewServer()
    if err != nil {
        return nil, errors.WrapWithContext(err, errors.ErrCodeServerCreationFailed)
    }
    return baseServer, nil
}

// 2. 使用预定义错误
func ValidateToken(token string) error {
    if token == "" {
        return errors.ErrInvalidToken
    }
    return nil
}

// 3. 检查错误码
if errors.IsErrorCode(err, errors.ErrCodeTokenExpired) {
    // 处理 token 过期
}
```

### ❌ 避免做法

```go
// ❌ 不要硬编码消息
return errors.NewError(errors.ErrCodeOperationFailed, "创建服务器失败")

// ❌ 不要使用 fmt.Errorf
return fmt.Errorf("token expired")

// ❌ 不要使用 errors.New
return errors.New("服务不可用")
```

## 🔍 错误信息获取

```go
appErr := errors.NewError(errors.ErrCodeTokenExpired, "")

// 获取错误代码
code := appErr.GetCode()  // 2004

// 获取标准消息
msg := appErr.GetMessage()  // "Token expired"

// 获取详细信息
details := appErr.GetDetails()

// 获取 HTTP 状态码
status := appErr.GetHTTPStatus()  // 401

// 获取 gRPC 状态码
grpcStatus := appErr.GetStatusCode()  // Unauthenticated

// 转换为 Result 结构
result := appErr.ToResult()
```

## 🌍 添加新错误类型

### 1. 在 `code.go` 中添加错误码

```go
const (
    // 你的模块错误 (xxxx-xxxx)
    ErrCodeYourNewError ErrorCode = 5001
)
```

### 2. 在 `error.go` 中添加消息映射

```go
var errorMessages = map[ErrorCode]string{
    ErrCodeYourNewError: "Your error message",
}
```

### 3. 添加 HTTP 状态码映射

```go
var httpStatusMapping = map[ErrorCode]int{
    ErrCodeYourNewError: http.StatusBadRequest,
}
```

### 4. 添加 gRPC 状态码映射

```go
var statusCodeMapping = map[ErrorCode]commonapis.StatusCode{
    ErrCodeYourNewError: commonapis.StatusCode_InvalidArgument,
}
```

### 5. 添加预定义错误变量

```go
var (
    ErrYourNewError = NewError(ErrCodeYourNewError, "")
)
```

## 📊 错误处理流程

```
┌─────────────┐
│ 业务代码发生错误 │
└──────┬──────┘
       │
       ▼
┌─────────────────┐
│ 使用 WrapWithContext │  ← 推荐：纯错误码
│ 包装为 AppError    │
└──────┬──────────┘
       │
       ▼
┌──────────────────┐
│ 自动查找错误码消息  │
│ 自动映射 HTTP状态码 │
│ 自动映射 gRPC状态码 │
└──────┬───────────┘
       │
       ▼
┌─────────────┐
│ 返回给调用者   │
└─────────────┘
```

## 🎨 实际示例

```go
// server/server.go
func NewServer() (*Server, error) {
    cfg := global.GATEWAY
    if cfg == nil {
        // ✅ 纯错误码：自动使用 "Invalid configuration"
        return nil, errors.NewError(errors.ErrCodeInvalidConfiguration, "global GATEWAY config is not initialized")
    }
    
    server := &Server{
        config:     cfg,
        configSafe: goconfig.SafeConfig(cfg),
    }
    
    // 初始化核心组件
    if err := server.initCore(); err != nil {
        // ✅ 纯错误码：不传消息，自动使用 "内部服务器错误"
        return nil, errors.NewErrorf(errors.ErrCodeInternalServerError, "failed to init core: %v", err)
    }
    
    return server, nil
}

// cpool/jwt/jwt.go
func (j *JWT) checkRedisMultipointAuth(claims *CustomClaims, jsonStr string) error {
    var clis CustomClaims
    if err := json.Unmarshal([]byte(jsonStr), &clis); err != nil {
        // ✅ 纯错误码：自动使用 "解析Redis中的用户token时出错"
        return errors.WrapWithContext(err, errors.ErrCodeRedisParseError)
    }
    
    if clis.TokenId != "" && claims.TokenId != clis.TokenId {
        // ✅ 使用预定义错误
        return errors.ErrAccountLoginElsewhere
    }
    
    return nil
}
```

## 📝 总结

- ✅ **使用错误码**，不要硬编码消息
- ✅ **使用 WrapWithContext**，保留原始错误
- ✅ **使用预定义错误变量**，代码更简洁
- ✅ **集中管理消息**，便于维护和国际化
- ❌ **避免 fmt.Errorf**
- ❌ **避免 errors.New**
- ❌ **避免硬编码消息字符串**
