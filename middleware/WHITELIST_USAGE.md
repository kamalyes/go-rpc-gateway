# Whitelist Middleware 使用指南

## 概述

通用白名单中间件提供了灵活的规则匹配机制，支持多种匹配模式、IP/CIDR 过滤和优先级控制。

## 核心特性

- ✅ **多种匹配规则** - 前缀、后缀、精确、正则、自定义
- ✅ **IP/CIDR 支持** - 基于客户端 IP 地址或网段过滤
- ✅ **优先级控制** - 数字越小优先级越高
- ✅ **线程安全** - 使用 RWMutex 保护
- ✅ **Builder 模式** - 链式调用，优雅配置
- ✅ **预设规则集** - 常用场景开箱即用

## 快速开始

### 1. 基础用法

```go
import "github.com/kamalyes/go-rpc-gateway/middleware"

// 创建白名单管理器
whitelist := middleware.NewWhitelistManager()

// 使用 Builder 模式注册规则
middleware.NewRuleBuilder(whitelist).
    AddPathPrefix("/public/", "公开资源").
    AddExactPath("GET", "/api/version", "版本信息").
    AddPathSuffix(".html", "静态 HTML 页面").
    Build()

// 检查是否在白名单中
if whitelist.IsWhitelisted("GET", "/public/images/logo.png") {
    // 允许访问
}
```

### 2. 使用预设规则

```go
var common middleware.CommonRules

whitelist := middleware.NewWhitelistManager()
whitelist.Register(common.HealthCheck())
whitelist.Register(common.Metrics())
whitelist.Register(common.Swagger())
whitelist.Register(common.StaticFiles("/static/"))
whitelist.Register(common.PublicAPI("/api/v1/public/"))
```

### 3. 高级规则

#### 正则表达式匹配

```go
middleware.NewRuleBuilder(whitelist).
    AddRegex(`^/api/v\d+/public/.*`, "所有版本的公开 API").
    Build()
```

#### 自定义匹配逻辑

```go
middleware.NewRuleBuilder(whitelist).
    AddCustom(func(method, path string) bool {
        // 只允许 GET 请求的图片文件
        return method == "GET" && 
               (strings.HasSuffix(path, ".jpg") || 
                strings.HasSuffix(path, ".png"))
    }, "图片资源", 100).
    Build()
```

#### IP 地址白名单

```go
middleware.NewRuleBuilder(whitelist).
    AddIP([]string{
        "127.0.0.1",
        "192.168.1.100",
        "10.0.0.50",
    }, "内部服务器").
    Build()
```

#### CIDR 网段白名单

```go
middleware.NewRuleBuilder(whitelist).
    AddCIDR([]string{
        "192.168.0.0/16",    // 内网
        "10.0.0.0/8",         // 内网
        "172.16.0.0/12",      // 内网
    }, "内网 IP 段").
    Build()
```

#### 指定优先级

```go
middleware.NewRuleBuilder(whitelist).
    AddPathPrefixWithPriority("/admin/", "管理后台", 1).  // 最高优先级
    AddPathPrefix("/api/", "API 接口", 50).
    Build()
```

## 规则类型

### PathPrefixRule - 路径前缀匹配

```go
AddPathPrefix("/test/", "测试页面")
// 匹配: /test/foo, /test/bar/baz
```

### ExactPathRule - 精确路径匹配

```go
AddExactPath("POST", "/v1/install", "安装接口")
// 仅匹配: POST /v1/install
```

### PathSuffixRule - 路径后缀匹配

```go
AddPathSuffix(".css", "CSS 样式表")
// 匹配: /static/style.css, /theme.css
```

### RegexRule - 正则表达式匹配

```go
AddRegex(`^/api/v[0-9]+/users/\d+$`, "用户详情 API")
// 匹配: /api/v1/users/123, /api/v2/users/456
```

### MethodRule - HTTP 方法匹配

```go
AddMethods([]string{"OPTIONS", "HEAD"}, "预检请求")
// 匹配: OPTIONS /any/path, HEAD /any/path
```

### CustomRule - 自定义规则

```go
AddCustom(func(method, path string) bool {
    return method == "GET" && strings.Contains(path, "/public/")
}, "自定义公开资源", 100)
```

## 优先级说明

默认优先级（数字越小越优先）：

- **5** - IP/CIDR 规则（最高优先级）
- **10** - 系统级端点（健康检查、监控等）
- **50** - 精确路径匹配
- **80** - 公开 API
- **100** - 路径前缀匹配
- **150** - 路径后缀匹配
- **200** - 正则表达式匹配
- **300** - HTTP 方法匹配

## 完整示例

### 认证中间件集成

```go
// middleware/authorities.go

import gwmiddleware "github.com/kamalyes/go-rpc-gateway/middleware"

var authWhitelist = gwmiddleware.NewWhitelistManager()

func init() {
    gwmiddleware.NewRuleBuilder(authWhitelist).
        // 系统端点
        AddExactPath(http.MethodPost, "/v1/install", "安装接口").
        // 测试和调试
        AddPathPrefix("/test/", "测试页面").
        // 监控
        AddPathPrefix("/health", "健康检查").
        AddPathPrefix("/metrics", "监控指标").
        // 静态资源
        AddPathSuffix(".html", "HTML 页面").
        AddPathSuffix(".js", "JavaScript").
        AddPathSuffix(".css", "样式表").
        // 正则匹配
        AddRegex(`^/static/.*\.(jpg|png|gif|svg)$`, "图片资源").
        Build()
}

// 在中间件中使用
func (h gruntime.HandlerFunc) gruntime.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
        if authWhitelist.IsWhitelisted(r.Method, r.URL.Path) {
            h(w, r, pathParams)
            return
        }
        // 执行认证逻辑...
    }
}
```

### 动态添加规则

```go
// 在运行时添加新规则
middleware.RegisterAuthWhitelistRule(&gwmiddleware.PathPrefixRule{
    Prefix:      "/new-endpoint/",
    Description: "新增端点",
    Priority:    100,
})
```

## 调试和查看规则

```go
// 获取所有规则
rules := whitelist.GetRules()
for _, rule := range rules {
    fmt.Printf("规则: %s (优先级: %d)\n", 
        rule.Description(), rule.Priority())
}

// 清空所有规则
whitelist.Clear()
```

## 最佳实践

1. **合理设置优先级** - 精确匹配应该比模糊匹配优先级更高
2. **避免规则冲突** - 确保规则之间不会产生歧义
3. **使用 Builder 模式** - 链式调用更清晰
4. **利用预设规则** - 减少重复代码
5. **添加描述信息** - 方便调试和维护

## API 参考

### WhitelistManager

- `NewWhitelistManager()` - 创建管理器
- `Register(rule)` - 注册规则
- `IsWhitelisted(method, path)` - 检查是否在白名单
- `GetRules()` - 获取所有规则
- `Clear()` - 清空规则

### RuleBuilder

- `AddPathPrefix(prefix, desc)` - 添加前缀规则
- `AddExactPath(method, path, desc)` - 添加精确路径
- `AddPathSuffix(suffix, desc)` - 添加后缀规则
- `AddRegex(pattern, desc)` - 添加正则规则
- `AddMethods(methods, desc)` - 添加方法规则
- `AddCustom(func, desc, priority)` - 添加自定义规则
- `AddIP(ips, desc)` - 添加 IP 白名单
- `AddCIDR(cidrs, desc)` - 添加 CIDR 网段
- `Build()` - 完成构建

### 全局便捷方法

- `DefaultWhitelistManager()` - 获取默认管理器
- `RegisterWhitelistRule(rule)` - 注册到默认管理器
- `IsWhitelisted(method, path)` - 使用默认管理器检查
- `IsWhitelistedWithIP(method, path, ip)` - 检查（含 IP）
- `GetClientIP(r)` - 从 HTTP 请求提取客户端 IP
