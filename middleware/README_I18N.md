# i18n 国际化中间件 - 业务层级API

这是一个重新设计的i18n国际化中间件，提供了更好的业务层级API，类似于go-i18n库的使用方式。

## 特性

- 📁 **文件配置支持** - 从JSON文件加载翻译消息
- 🔀 **多种语言检测** - 支持HTTP头、查询参数、Cookie检测
- 📝 **模板数据支持** - 支持动态数据插值
- 🔄 **语言回退机制** - 自动回退到默认语言
- 🎯 **业务层级API** - 简洁易用的函数接口

## 核心API函数

### 1. 基础翻译函数

```go
// 通过键获取消息
message := middleware.GetMsgByKey(ctx, "welcome")

// 等价于
message := middleware.T(ctx, "welcome")
```

### 2. 模板数据翻译函数

```go
// 使用map模板数据获取消息
userData := map[string]interface{}{
    "name": "张三",
    "age":  25,
}
message := middleware.GetMsgWithMap(ctx, "user.created", userData)

// 等价于
message := middleware.TWithMap(ctx, "user.created", userData)
```

### 3. 语言管理函数

```go
// 获取当前语言
language := middleware.GetLanguage(ctx)

// 设置语言并返回新的context
newCtx := middleware.SetLanguage(ctx, "zh")
```

## 配置文件格式

### English (locales/en.json)
```json
{
  "welcome": "Welcome",
  "user.created": "User {{.name}} created successfully",
  "user.info": "User {{.name}} is {{.age}} years old",
  "validation.min_length": "Minimum length is {{.min}} characters"
}
```

### 中文 (locales/zh.json)
```json
{
  "welcome": "欢迎",
  "user.created": "用户 {{.name}} 创建成功",
  "user.info": "用户 {{.name}} 今年 {{.age}} 岁",
  "validation.min_length": "最小长度为 {{.min}} 个字符"
}
```

## 使用示例

### 1. 基础使用

```go
func helloHandler(w http.ResponseWriter, r *http.Request) {
    // 获取翻译消息
    message := middleware.GetMsgByKey(r.Context(), "welcome")
    language := middleware.GetLanguage(r.Context())
    
    fmt.Fprintf(w, "Message: %s, Language: %s", message, language)
}
```

### 2. 模板数据使用

```go
func userHandler(w http.ResponseWriter, r *http.Request) {
    userData := map[string]interface{}{
        "name": "张三",
        "age":  25,
    }
    
    // 创建消息
    createMsg := middleware.GetMsgWithMap(r.Context(), "user.created", userData)
    
    // 信息消息
    infoMsg := middleware.GetMsgWithMap(r.Context(), "user.info", userData)
    
    fmt.Fprintf(w, "Create: %s Info: %s", createMsg, infoMsg)
}
```

### 3. 验证消息示例

```go
func validationHandler(w http.ResponseWriter, r *http.Request) {
    // 使用模板数据
    minData := map[string]interface{}{"min": 8}
    maxData := map[string]interface{}{"max": 50}
    
    minMsg := middleware.GetMsgWithMap(r.Context(), "validation.min_length", minData)
    maxMsg := middleware.GetMsgWithMap(r.Context(), "validation.max_length", maxData)
    
    // 输出: "最小长度为 8 个字符" (中文) 或 "Minimum length is 8 characters" (英文)
}
```

### 4. 动态语言切换

```go
func switchLanguageHandler(w http.ResponseWriter, r *http.Request) {
    // 切换到中文
    ctx := middleware.SetLanguage(r.Context(), "zh")
    
    // 使用新语言获取消息
    message := middleware.GetMsgByKey(ctx, "welcome")
    // 输出: "欢迎"
}
```

## 中间件配置

i18n 中间件基于 `go-config` 的 `gci18n.I18N` 配置与 `go-i18n` 的 `Manager`。先创建 Manager，再构造中间件：

```go
import (
    gci18n "github.com/kamalyes/go-config/pkg/i18n"
    "github.com/kamalyes/go-rpc-gateway/middleware"
)

// cfg.Middleware.I18N 为 *gci18n.I18N 类型，字段包括：
//   DefaultLanguage  默认语言
//   LanguageHeader   语言检测头名（如 Accept-Language）
//   LanguageParam    查询参数与 Cookie 名（如 lang）
//   CookieName       Cookie 名（为空时回退到 LanguageParam）
//   DetectionOrder   检测顺序（header / query / cookie / default）
//   ParseAcceptLanguage / ResolveLanguage 语言解析方法
manager, err := middleware.NewI18nManager(cfg.Middleware.I18N)
if err != nil {
    // ...
}

// 构造 HTTP 中间件（返回 MiddlewareFunc）
i18nMiddleware := middleware.I18nWithManager(manager)
// 或使用默认配置（panic if init failed）：
// i18nMiddleware := middleware.I18n()

mux.Handle("/api", i18nMiddleware(http.HandlerFunc(handler)))
```

> 由 `Manager.I18nMiddleware()` 暴露：启用时使用 `NewManager` 中创建的 `i18nManager`，未启用时直通。gRPC 拦截器为 `UnaryServerI18nInterceptor(manager)` 与 `StreamServerI18nInterceptor(manager)`。

## 语言检测优先级

1. **Query参数** - `?lang=zh`
2. **HTTP头** - `Accept-Language: zh-CN,zh;q=0.9,en;q=0.8`
3. **Cookie** - `lang=zh`
4. **默认语言** - 配置中的默认语言

## 与传统i18n库的对比

| 功能 | 传统方式 | 我们的方式 |
|------|----------|------------|
| 获取消息 | `I18n.Localize(&i18n.LocalizeConfig{MessageID: key})` | `GetMsgByKey(ctx, key)` |
| 模板数据 | `I18n.Localize(&i18n.LocalizeConfig{MessageID: key, TemplateData: data})` | `GetMsgWithMap(ctx, key, data)` |
| 上下文传递 | 需要手动管理bundle和localizer | 自动从HTTP上下文获取 |
| 错误处理 | 需要手动检查错误 | 自动回退到key或默认语言 |

## 高级功能

### 自定义消息加载器

```go
type customLoader struct {
    // 自定义实现
}

func (c *customLoader) LoadMessages(language string) (map[string]string, error) {
    // 从数据库、Redis等加载消息
    return messages, nil
}

config.MessageLoader = &customLoader{}
```

### 本地化错误

```go
// 创建本地化错误
err := middleware.NewLocalizedError(ctx, "validation.required")

// 输出错误消息时会自动翻译
fmt.Println(err.Error()) // "此字段为必填项" (中文) 或 "This field is required" (英文)
```

这个重新设计的i18n中间件提供了更好的业务层级抽象，使得国际化功能更易用和维护。