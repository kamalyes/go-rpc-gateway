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

// 使用错误码创建（消息取自码表）
return errors.NewError(errors.ErrCodeInvalidConfiguration)

// 带详情
return errors.NewError(errors.ErrCodeResourceNotFound, errors.WithDetails(userID))
return errors.NewError(errors.ErrCodeResourceNotFound, errors.WithDetailsf("user_id: %s", userID))
```

### 2. 包装现有错误（推荐）

```go
// ✅ 推荐：纯错误码模式（不传消息）
if err := someOperation(); err != nil {
    return errors.Wrap(err, errors.ErrCodeOperationFailed)
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
errors.ErrCodeTokenMalformed         // 2101: Malformed token
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
        return nil, errors.Wrap(err, errors.ErrCodeServerCreationFailed)
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
return errors.NewError(errors.ErrCodeOperationFailed, errors.WithMessage("创建服务器失败"))

// ❌ 不要使用 fmt.Errorf
return fmt.Errorf("token expired")

// ❌ 不要使用 errors.New
return errors.New("服务不可用")
```

## 🔍 错误信息获取

```go
appErr := errors.NewError(errors.ErrCodeTokenExpired)

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

## 🌐 业务错误码与国际化

业务错误码这一套原先住在独立的 `go-bizerror` 仓库，现已并入本包：码表、映射、内置文案、
构造器与收集器都在这里，下游只依赖本包即可，不需要再引一个错误码库

> 两套码不要混：`ErrCode*` 是网关层的数值码（`ErrorCode`，进响应包络的 `code`），
> `BizErrCode*` 是业务语义串（同时就是 i18n key）。前者描述「网关怎么回」，
> 后者描述「业务出了什么事」，由注册表把后者映射到前者

### 通用业务错误码

`bizcodes.go` 里有 15 个跨服务通用码，码值本身就是 i18n key：

| 常量 | 码值 | gRPC | 网关码 |
|---|---|---|---|
| `BizErrCodeInvalidArgument` | `error.invalid_argument` | InvalidArgument | `ErrCodeBadRequest` |
| `BizErrCodeUnauthenticated` | `error.unauthenticated` | Unauthenticated | `ErrCodeUnauthorized` |
| `BizErrCodePermissionDenied` | `error.permission_denied` | PermissionDenied | `ErrCodeForbidden` |
| `BizErrCodeNotFound` | `error.not_found` | NotFound | `ErrCodeNotFound` |
| `BizErrCodeConflict` | `error.conflict` | AlreadyExists | `ErrCodeConflict` |
| `BizErrCodeValidationFailed` | `error.validation_failed` | InvalidArgument | `ErrCodeBadRequest` |
| `BizErrCodeInvalidParameter` | `error.invalid_parameter` | InvalidArgument | `ErrCodeBadRequest` |
| `BizErrCodeCannotBeSelf` | `error.cannot_be_self` | FailedPrecondition | `ErrCodeBadRequest` |
| `BizErrCodeDatabaseError` | `error.database_error` | Internal | `ErrCodeInternalServerError` |
| `BizErrCodeDatabaseOperationFailed` | `error.database_operation_failed` | Internal | `ErrCodeInternalServerError` |
| `BizErrCodeConversionFailed` | `error.conversion_failed` | Internal | `ErrCodeInternalServerError` |
| `BizErrCodeInternalError` | `error.internal` | Internal | `ErrCodeInternalServerError` |
| `BizErrCodeRepositoryNotInitialized` | `error.repository_not_initialized` | Internal | `ErrCodeInternalServerError` |
| `BizErrCodeUnsupportedOperation` | `error.unsupported_operation` | Unimplemented | `ErrCodeInternalServerError` |
| `BizErrCodeExternalServiceError` | `error.external_service_error` | Internal | `ErrCodeInternalServerError` |

这 15 个在包 `init()` 里已注册好，**零注册即可用**。

### 一行直出：具名构造器

每个通用码都有一个同名构造器，省掉 `ToGRPCError(ctx, BizErrCodeXxx)` 的两段式写法：

```go
return nil, errors.NotFound(ctx)
return nil, errors.Unauthenticated(ctx)
return nil, errors.DatabaseOperationFailed(ctx)  // 别返回原始 err，表名和 SQL 会漏给调用方
```

> 构造器是函数，同包的 `ErrNotFound` 之类是 `*AppError` 值，靠括号区分。
> 有一条测试直接读码表源码，保证每加一个 `BizErrCode*` 就必须有对应构造器。

### 要带变量或原因：链式构造器

```go
// 服务自注册的码也走这里，不必在服务里再包一层
return errors.Of(errcodes.BizErrCodeQuotaExceeded).
    With("limit", limit).
    Cause(err).            // 写入 {reason}；err 为 nil 时不改变链，调用方不必判空
    Err(ctx)

// 只要文案不要 gRPC 包装（写日志或塞响应体）
msg := errors.Of(errcodes.BizErrCodeQuotaExceeded).With("limit", limit).Message(ctx)
```

`*Chain` 的所有方法对 nil 接收者安全：`Err` 返回 nil，`Message` 返回空串。

### 注册服务自己的码

```go
// 推荐：gRPC 码与网关码一起注册，二者是一个决定的两面
errors.RegisterMany(map[string]errors.CodeMapping{
    "error.quota_exceeded":  {codes.ResourceExhausted, errors.ErrCodeTooManyRequests},
    "error.order_cancelled": {codes.FailedPrecondition, errors.ErrCodeBadRequest},
})
errors.Register("error.payment_failed", errors.CodeMapping{codes.Internal, errors.ErrCodeInternalServerError})

// 只给 gRPC 码时，网关码按 gRPC 码推导（不会留零值——那等于 ErrCodeOK，
// 会让一个错误在包络里显示成成功）
errors.RegisterBizCode("error.payment_failed", codes.Internal)
errors.RegisterBizCodeMap(map[string]codes.Code{"error.user_not_found": codes.NotFound})

// 查找顺序：显式注册 → _not_found 后缀规则 → 兜底 Internal
m := errors.Mapping("error.widget_not_found")        // {NotFound, ErrCodeNotFound}
errors.MapBizCodeToGRPCCode("error.quota_exceeded")  // codes.ResourceExhausted
errors.MapBizCodeToGatewayCode("error.quota_exceeded")
```

> ⚠️ **除 `_not_found` 结尾的码之外必须显式注册**：其余一律兜底 `Internal`，
> 也就是说漏登记的 `_already_exists` 会变成 **500 而不是 409**，而且没有任何报错提示。

注册表带读写锁：注册在 bootstrap 期发生，查找在每个请求的错误路径上发生，
而 Go 对并发读写 map 是直接 fatal，不是返回错误。

### 转换为 gRPC 错误

> 包级 `errors.ToGRPCError(ctx, bizCode)` 与 `AppError.ToGRPCError()` 方法不是一回事。

```go
err := errors.ToGRPCError(ctx, errors.BizErrCodeNotFound)
err := errors.ToGRPCErrorWithTemplate(ctx, errors.BizErrCodeInvalidParameter,
    map[string]interface{}{"reason": "age 必须为正数"})
err := errors.ToGRPCErrorWithCause(ctx, errors.BizErrCodeInvalidParameter, dbErr) // cause 为 nil 时退化
```

### 文案解析：三层兜底

```go
msg := errors.NewI18nError(ctx, errors.BizErrCodeNotFound)
msg := errors.NewI18nErrorWithTemplate(ctx, errors.BizErrCodeInvalidParameter,
    map[string]interface{}{"reason": "连接被重置"})
msg := errors.BuiltinMessage("zh", errors.BizErrCodeNotFound) // "资源未找到"；未收录返回空串
```

| 层 | 来源 |
|---|---|
| 1 | 服务自己的 locales（同名 key 优先） |
| 2 | 本包 `builtin_messages.go` 的内置文案（中英两套） |
| 3 | 错误码本身，保证返回值永不为空 |

第 2 层是「新服务零配置即有可读错误消息」的保证 —— 没有它，还没写 locales 的服务
会把 `error.not_found` 这样的内部标识符直接返给前端。内置文案兜底时模板一样会套上，
否则调用方会收到字面量 `{reason}`。

### 从错误里取可读消息

两个函数名字相近、输入不同，别用错：

```go
// 收 error：取链上最内层 gRPC status 的 Message
// 用标准库的 stderrors.As 而不是 status.FromError —— 后者在遇到 %w 包裹的 status 时，
// 会把外层 error 的完整文案覆盖进 Message，于是提取结果带上 "rpc error: code = ..." 信封
msg := errors.ExtractRpcErrorMsg(err)

// 收字符串：从网关包络 {code,msg} 里取 msg；不是 JSON 则返回原串
msg := errors.ExtractEnvelopeMsg(raw)
```

### 逐字段违规项收集

标签表达不了的业务规则校验用它 —— 跨字段、跨记录、要查库、或一条规则牵涉整个集合
（阶梯价要逐档递增这类）。能用标签说清的（必填、长度、区间、格式、枚举合法性）
交给 proto 上的 `validate:"..."`，由校验拦截器统一执行，业务代码里不该再写一遍。

```go
v := errors.NewViolations()
for i, tier := range req.GetTiers() {
    if tier.GetMinAmount() <= prev {
        v.Addf("tiers", i, "minAmount", "must_ascend", "阶梯下限必须逐档递增")
    }
}
if v.Has() {
    return nil, v.Err(ctx, errors.BizErrCodeValidationFailed)
}
```

收集而不是遇错即返：一次提交里有三处填错时逐条返回，会让使用者改一处、提交一次、
再被拒一次。`Violation.Rule` 是机器可读的稳定串，前端按它定位到具体输入框；
`Msg` 可本地化，`Rule` 不变。`IndexedField("tiers", 2, "minAmount")` 拼出 `tiers[2].minAmount`，
格式必须稳定，前端靠它定位到具体那一行。

明细走日志而不是 `status.WithDetails`：本仓库的错误统一由 `ToGRPCError` 构造、
由网关包络成 `{code,msg,data}`，两套机制混用会让同一个接口有两种错误形状。

```go
// 日志出口由网关在初始化与热重载时注入；未注入时明细不记录，但错误照常返回
errors.SetViolationLogger(global.LOGGER)
```

> 本包**不导入** `global`：那条边会经 `cpool` 把 MySQL 驱动、Redis、NATS、MQTT 与
> ClickHouse 拖进一个只做错误码映射的包（实测传递依赖 404 个包）。错误码是最底层的
> 契约，应该谁都能引用它，而不是它反过来引用连接池，所以日志器由上层注入

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
    ErrYourNewError = NewError(ErrCodeYourNewError)
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
│ 使用 Wrap │  ← 推荐：纯错误码
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
        return nil, errors.NewError(errors.ErrCodeInvalidConfiguration,
            errors.WithDetails("global GATEWAY config is not initialized"))
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
```

## 📝 总结

- ✅ **使用错误码**，不要硬编码消息
- ✅ **使用 Wrap**，保留原始错误
- ✅ **使用预定义错误变量**，代码更简洁
- ✅ **集中管理消息**，便于维护和国际化
- ❌ **避免 fmt.Errorf**
- ❌ **避免 errors.New**
- ❌ **避免硬编码消息字符串**

业务错误码那一半：

- ✅ **用具名构造器**（`errors.NotFound(ctx)`），不要在每个服务里重新包一层
- ✅ **要带变量或原因时走链式**（`errors.Of(code).With(...).Cause(err).Err(ctx)`）
- ✅ **注册服务自己的码时 gRPC 码与网关码一起给**（`Register` / `RegisterMany`）
- ✅ **跨字段、跨记录的校验用 `Violations` 一次收齐**，不要遇错即返
- ❌ **别漏登记非 `_not_found` 结尾的码** —— 会静默兜底成 500
- ❌ **别把 `ExtractRpcErrorMsg`（收 error）和 `ExtractEnvelopeMsg`（收字符串）用混**
