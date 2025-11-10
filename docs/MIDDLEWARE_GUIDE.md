# 🛡️ 中间件使用指南

## 📖 概述

go-rpc-gateway 内置了丰富的中间件生态系统，提供安全、监控、控制、体验和开发等多个维度的功能。本文档详细介绍每个中间件的配置方法、使用场景和最佳实践。

## 🏗️ 中间件架构

### 执行顺序

中间件按照以下顺序执行（可配置）：

```
Request → Security → RateLimit → RequestID → Signature → Tracing → Logging → I18n → Business Handler
```

### 配置结构

```yaml
middleware:
  # 各中间件配置
  rate_limit: {...}
  access_log: {...}
  signature: {...}
  # ... 其他中间件
```

## 🛡️ 安全类中间件

### 1. Security 中间件

**功能：** 提供基础安全防护，包括安全头设置、XSS防护、CSP策略等。

#### 配置示例

```yaml
middleware:
  security:
    enabled: true
    # 安全头配置
    headers:
      x_frame_options: "DENY"
      x_content_type_options: "nosniff" 
      x_xss_protection: "1; mode=block"
      strict_transport_security: "max-age=31536000; includeSubDomains"
      content_security_policy: "default-src 'self'"
      referrer_policy: "strict-origin-when-cross-origin"
    # XSS 防护
    xss_protection: true
    # 内容类型检测
    content_type_nosniff: true
```

#### 代码示例

```go
// 启用 Security 中间件
gw, _ := gateway.New()

// 自定义安全配置
securityConfig := &middleware.SecurityConfig{
    Enabled: true,
    Headers: map[string]string{
        "X-Frame-Options":           "DENY",
        "X-Content-Type-Options":    "nosniff",
        "X-XSS-Protection":          "1; mode=block",
        "Strict-Transport-Security": "max-age=31536000; includeSubDomains",
    },
}

gw.RegisterMiddleware(middleware.SecurityMiddleware(securityConfig))
```

#### 最佳实践

- ✅ 在生产环境启用所有安全头
- ✅ 根据业务需求调整 CSP 策略
- ✅ 定期更新安全配置
- ❌ 不要在开发环境使用过严格的策略

### 2. CORS 中间件

**功能：** 处理跨域资源共享(CORS)请求。

#### 配置示例

```yaml
middleware:
  cors:
    enabled: true
    allowed_origins: 
      - "https://example.com"
      - "https://*.example.com"
    allowed_methods: 
      - "GET"
      - "POST"
      - "PUT"
      - "DELETE"
      - "OPTIONS"
    allowed_headers: 
      - "Content-Type"
      - "Authorization"
      - "X-Requested-With"
    exposed_headers:
      - "X-Total-Count"
    allow_credentials: true
    max_age: 3600
```

#### 代码示例

```go
// 使用 go-config 的 CORS 配置
corsConfig := &cors.Cors{
    Enabled: true,
    AllowedOrigins: []string{"https://example.com"},
    AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
    AllowedHeaders: []string{"*"},
    AllowCredentials: true,
    MaxAge: 3600,
}

gw.RegisterCORS(corsConfig)
```

### 3. Signature 中间件

**功能：** 提供 HMAC-SHA256 请求签名验证，防止请求篡改和重放攻击。

#### 签名算法

```
signature = base64(hmac-sha256(secret, string_to_sign))

string_to_sign = HTTP_METHOD + "\n" +
                 REQUEST_URI + "\n" +
                 TIMESTAMP + "\n" +
                 NONCE + "\n" +
                 BODY_HASH
```

#### 配置示例

```yaml
middleware:
  signature:
    enabled: true
    algorithm: "hmac-sha256"
    secret_key: "your-super-secret-key-32-chars!"
    ttl: 300  # 签名有效期 5分钟
    skip_paths:
      - "/health"
      - "/metrics"
      - "/debug/pprof/*"
    headers:
      signature: "X-Signature"
      timestamp: "X-Timestamp" 
      nonce: "X-Nonce"
```

#### 客户端实现示例

```go
// 客户端签名生成示例
func generateSignature(method, uri, timestamp, nonce, body, secretKey string) string {
    // 构造待签名字符串
    stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", 
        method, uri, timestamp, nonce, hashBody(body))
    
    // HMAC-SHA256 签名
    h := hmac.New(sha256.New, []byte(secretKey))
    h.Write([]byte(stringToSign))
    
    // Base64 编码
    return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// 发送带签名的请求
func sendRequest(url, body string) {
    timestamp := strconv.FormatInt(time.Now().Unix(), 10)
    nonce := generateUUID()
    signature := generateSignature("POST", "/api/users", timestamp, nonce, body, secretKey)
    
    req, _ := http.NewRequest("POST", url, strings.NewReader(body))
    req.Header.Set("X-Signature", signature)
    req.Header.Set("X-Timestamp", timestamp)
    req.Header.Set("X-Nonce", nonce)
    req.Header.Set("Content-Type", "application/json")
    
    // 发送请求...
}
```

#### 最佳实践

- ✅ 使用足够长的密钥（至少32字符）
- ✅ 客户端使用 HTTPS 传输
- ✅ 合理设置签名有效期
- ✅ 实现 Nonce 防重放机制
- ❌ 不要在日志中记录签名和密钥

## 🚦 控制类中间件

### 1. RateLimit 中间件

**功能：** 提供多种限流算法，包括令牌桶和滑动窗口。

#### 令牌桶算法配置

```yaml
middleware:
  rate_limit:
    enabled: true
    algorithm: "token_bucket"
    rate: 100        # 每秒生成令牌数
    burst: 10        # 令牌桶容量
    key_func: "ip"   # 限流键：ip, user, header
    headers:
      limit: "X-RateLimit-Limit"
      remaining: "X-RateLimit-Remaining"  
      reset: "X-RateLimit-Reset"
    # 自定义键提取
    custom_key_header: "X-User-ID"
```

#### 滑动窗口算法配置

```yaml
middleware:
  rate_limit:
    enabled: true
    algorithm: "sliding_window"
    rate: 1000           # 窗口期间最大请求数
    window_size: 3600    # 窗口大小（秒）
    precision: 100       # 精度分片数
```

#### 代码示例

```go
// 自定义限流器
type CustomRateLimiter struct {
    // 实现限流逻辑
}

func (r *CustomRateLimiter) Allow(key string) bool {
    // 检查是否允许请求
    return true
}

func (r *CustomRateLimiter) Tokens(key string) (int, int, time.Time) {
    // 返回：当前令牌数，最大令牌数，重置时间
    return 10, 100, time.Now().Add(time.Hour)
}

// 注册自定义限流器
rateLimitConfig := &middleware.RateLimitConfig{
    Enabled: true,
    Algorithm: "custom",
}

gw.RegisterRateLimiter(&CustomRateLimiter{}, rateLimitConfig)
```

#### 高级配置

```yaml
middleware:
  rate_limit:
    enabled: true
    # 分层限流配置
    rules:
      # 全局限流
      - path: "/*"
        rate: 1000
        burst: 100
        algorithm: "token_bucket"
      # API 限流  
      - path: "/api/*"
        rate: 500
        burst: 50
        algorithm: "sliding_window"
      # 特殊端点限流
      - path: "/api/upload"
        rate: 10
        burst: 5
        algorithm: "token_bucket"
    # 白名单
    whitelist:
      ips: ["127.0.0.1", "::1"]
      headers: 
        X-Admin-Token: "admin-secret"
```

### 2. Recovery 中间件

**功能：** 捕获并处理 panic，防止服务器崩溃。

#### 配置示例

```yaml
middleware:
  recovery:
    enabled: true
    # 错误响应配置
    error_response:
      status_code: 500
      message: "Internal Server Error"
      include_stack: false  # 生产环境设为 false
    # 日志配置
    log_stack: true
    log_level: "error"
```

#### 代码示例

```go
// 自定义恢复处理器
func customRecoveryHandler(c *gin.Context, err interface{}) {
    // 记录错误日志
    logger.Error("Panic recovered", 
        zap.Any("error", err),
        zap.String("path", c.Request.URL.Path),
        zap.String("method", c.Request.Method),
    )
    
    // 返回错误响应
    c.JSON(http.StatusInternalServerError, gin.H{
        "error": "Internal Server Error",
        "code":  500,
    })
}

// 注册自定义恢复中间件
gw.RegisterMiddleware(middleware.RecoveryWithHandler(customRecoveryHandler))
```

### 3. RequestID 中间件

**功能：** 为每个请求生成唯一ID，用于链路追踪。

#### 配置示例

```yaml
middleware:
  request_id:
    enabled: true
    header: "X-Request-ID"      # 请求头名称
    generator: "uuid"           # 生成器类型：uuid, nanoid, snowflake
    # UUID 配置
    uuid_version: 4
    # NanoID 配置  
    nanoid_alphabet: "0123456789abcdefghijklmnopqrstuvwxyz"
    nanoid_length: 21
    # Snowflake 配置
    snowflake_machine_id: 1
```

#### 代码示例

```go
// 自定义 ID 生成器
type CustomIDGenerator struct{}

func (g *CustomIDGenerator) Generate() string {
    // 实现自定义 ID 生成逻辑
    return fmt.Sprintf("custom-%d", time.Now().UnixNano())
}

// 在处理器中获取 Request ID
func handler(w http.ResponseWriter, r *http.Request) {
    requestID := middleware.GetRequestID(r.Context())
    
    // 使用 Request ID 进行日志记录
    logger.Info("Processing request", 
        zap.String("request_id", requestID),
        zap.String("path", r.URL.Path),
    )
}
```

## 📊 监控类中间件

### 1. Metrics 中间件

**功能：** 收集 Prometheus 指标，包括请求计数、延迟、错误率等。

#### 配置示例

```yaml
middleware:
  metrics:
    enabled: true
    path: "/metrics"
    port: 8081                    # 独立端口
    namespace: "gateway"          # 指标命名空间
    subsystem: "http"            # 指标子系统
    # 内置指标配置
    builtin_metrics:
      requests_total: true        # 请求总数
      request_duration: true      # 请求延迟
      request_size: true          # 请求大小
      response_size: true         # 响应大小
      active_requests: true       # 活跃请求数
    # 标签配置
    labels:
      - "method"      # HTTP 方法
      - "path"        # 请求路径
      - "status"      # 状态码
      - "version"     # API 版本
    # 路径标签化配置
    path_mapping:
      "/api/users/*": "/api/users/{id}"
      "/api/orders/*": "/api/orders/{id}"
```

#### 自定义指标

```go
// 注册自定义指标
var (
    customCounter = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "gateway",
            Subsystem: "custom", 
            Name:      "events_total",
            Help:      "Total number of custom events",
        },
        []string{"event_type"},
    )
    
    customHistogram = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "gateway",
            Subsystem: "custom",
            Name:      "operation_duration_seconds",
            Help:      "Duration of operations",
            Buckets:   prometheus.DefBuckets,
        },
        []string{"operation"},
    )
)

func init() {
    prometheus.MustRegister(customCounter)
    prometheus.MustRegister(customHistogram)
}

// 在业务代码中使用
func businessHandler(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    
    // 业务逻辑
    processBusinessLogic()
    
    // 记录指标
    customCounter.WithLabelValues("user_created").Inc()
    customHistogram.WithLabelValues("user_creation").Observe(time.Since(start).Seconds())
}
```

### 2. Logging 中间件

**功能：** 结构化访问日志记录。

#### 配置示例

```yaml
middleware:
  access_log:
    enabled: true
    format: "json"                # json, text, custom
    # 输出配置
    outputs:
      - type: "file"
        path: "/var/log/gateway/access.log"
        max_size: 100              # MB
        max_backups: 7
        max_age: 30               # 天
      - type: "stdout"
    # 字段配置
    fields:
      timestamp: true
      request_id: true
      remote_addr: true
      method: true
      uri: true
      protocol: true
      status_code: true
      response_size: true
      request_size: true
      user_agent: true
      referer: true
      latency: true
      # 自定义字段
      custom_fields:
        user_id: "X-User-ID"
        tenant: "X-Tenant"
    # 过滤配置
    filters:
      # 忽略的路径
      ignore_paths:
        - "/health"
        - "/metrics"
        - "/favicon.ico"
      # 忽略的状态码
      ignore_status_codes:
        - 404
      # 最小记录延迟
      min_latency: "1ms"
```

#### 自定义日志格式

```go
// 自定义日志字段提取器
func customFieldExtractor(r *http.Request, resp *http.Response, latency time.Duration) map[string]interface{} {
    return map[string]interface{}{
        "custom_field_1": r.Header.Get("X-Custom-Field"),
        "custom_field_2": extractFromContext(r.Context()),
        "business_metric": calculateBusinessMetric(r, resp),
    }
}

// 注册自定义访问日志中间件
loggingConfig := &middleware.LoggingConfig{
    Enabled: true,
    Format:  "custom",
    CustomFieldExtractor: customFieldExtractor,
}

gw.RegisterMiddleware(middleware.LoggingMiddleware(loggingConfig))
```

### 3. Tracing 中间件

**功能：** OpenTelemetry 链路追踪集成。

#### 配置示例

```yaml
middleware:
  tracing:
    enabled: true
    # 导出器配置
    exporter:
      type: "jaeger"              # jaeger, zipkin, otlp
      endpoint: "http://jaeger:14268/api/traces"
      # OTLP 配置
      otlp_endpoint: "http://otel-collector:4317"
      otlp_insecure: true
    # 采样配置
    sampler:
      type: "probability"         # always, never, probability, rate_limiting
      probability: 0.1            # 10% 采样率
      rate: 100                   # 每秒采样数
    # 资源配置
    resource:
      service_name: "go-rpc-gateway"
      service_version: "v1.0.0"
      environment: "production"
      attributes:
        team: "platform"
        region: "us-west-2"
```

#### 代码示例

```go
// 在处理器中创建子 Span
func businessHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // 创建子 span
    ctx, span := otel.Tracer("business").Start(ctx, "process_user_request")
    defer span.End()
    
    // 添加 span 属性
    span.SetAttributes(
        attribute.String("user_id", getUserID(r)),
        attribute.String("operation", "create_user"),
        attribute.Int("request_size", int(r.ContentLength)),
    )
    
    // 执行业务逻辑
    result, err := processUser(ctx, getUserFromRequest(r))
    if err != nil {
        span.SetStatus(codes.Error, err.Error())
        span.RecordError(err)
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // 记录成功
    span.SetStatus(codes.Ok, "User processed successfully")
    span.SetAttributes(attribute.String("result_id", result.ID))
    
    // 返回响应
    json.NewEncoder(w).Encode(result)
}
```

### 4. Health 中间件

**功能：** 健康检查端点，支持多组件检查。

#### 配置示例

```yaml
middleware:
  health:
    enabled: true
    path: "/health"
    # 组件检查配置
    checks:
      # Redis 检查
      redis:
        enabled: true
        name: "redis"
        timeout: "5s"
        connection:
          host: "redis"
          port: 6379
          password: ""
          database: 0
      # MySQL 检查  
      mysql:
        enabled: true
        name: "mysql"
        timeout: "5s"
        connection:
          host: "mysql"
          port: 3306
          username: "user"
          password: "password"
          database: "app"
      # 自定义检查
      custom:
        enabled: true
        name: "external_api"
        timeout: "10s"
        endpoint: "https://api.external.com/health"
    # 响应配置
    response:
      include_details: true        # 包含详细信息
      include_system_info: true    # 包含系统信息
      custom_fields:
        version: "v1.0.0"
        build_time: "2024-01-01T00:00:00Z"
```

#### 自定义健康检查

```go
// 实现健康检查接口
type CustomHealthChecker struct {
    name    string
    timeout time.Duration
}

func (c *CustomHealthChecker) Name() string {
    return c.name
}

func (c *CustomHealthChecker) Check(ctx context.Context) error {
    // 创建带超时的上下文
    checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
    defer cancel()
    
    // 执行具体检查逻辑
    if err := checkExternalService(checkCtx); err != nil {
        return fmt.Errorf("external service check failed: %w", err)
    }
    
    return nil
}

// 注册自定义健康检查
healthChecker := &CustomHealthChecker{
    name:    "external_api",
    timeout: 10 * time.Second,
}

gw.RegisterHealthChecker(healthChecker)
```

## 🌍 体验类中间件

### 1. I18n 中间件

**功能：** 国际化支持，目前支持19种语言。

#### 配置示例

```yaml
middleware:
  i18n:
    enabled: true
    default_language: "en"
    # 语言检测配置
    detection:
      sources:
        - "header"      # Accept-Language
        - "query"       # ?lang=en
        - "cookie"      # cookie中的语言设置
      header_name: "Accept-Language"
      query_param: "lang"
      cookie_name: "language"
    # 翻译文件配置
    translations:
      path: "./locales"
      format: "json"      # json, yaml
      fallback: true      # 启用回退机制
    # 支持的语言列表
    supported_languages:
      - "en"    # English
      - "zh"    # 中文简体
      - "zh-tw" # 中文繁体
      - "ja"    # 日本語
      - "ko"    # 한국어
      - "es"    # Español
      - "fr"    # Français
      - "de"    # Deutsch
      - "ru"    # Русский
      - "ar"    # العربية
      - "hi"    # हिन्दी
      - "pt"    # Português
      - "it"    # Italiano
      - "nl"    # Nederlands
      - "sv"    # Svenska
      - "tr"    # Türkçe
      - "th"    # ไทย
```

#### 翻译文件示例

```json
// locales/en.json
{
  "welcome": "Welcome to our service",
  "user": {
    "created": "User {{.name}} has been created successfully",
    "updated": "User information updated",
    "deleted": "User has been deleted",
    "not_found": "User not found",
    "errors": {
      "invalid_email": "Invalid email address",
      "weak_password": "Password is too weak"
    }
  },
  "validation": {
    "required": "This field is required",
    "min_length": "Minimum length is {{.min}} characters",
    "max_length": "Maximum length is {{.max}} characters"
  }
}
```

```json
// locales/zh.json
{
  "welcome": "欢迎使用我们的服务",
  "user": {
    "created": "用户 {{.name}} 创建成功",
    "updated": "用户信息已更新", 
    "deleted": "用户已删除",
    "not_found": "用户不存在",
    "errors": {
      "invalid_email": "邮箱地址无效",
      "weak_password": "密码强度不够"
    }
  },
  "validation": {
    "required": "此字段为必填项",
    "min_length": "最小长度为 {{.min}} 个字符", 
    "max_length": "最大长度为 {{.max}} 个字符"
  }
}
```

#### 代码使用示例

```go
// 在处理器中使用国际化
func createUserHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // 获取当前语言
    lang := middleware.GetLanguage(ctx)
    logger.Info("Current language", zap.String("lang", lang))
    
    // 简单消息翻译
    welcomeMsg := middleware.T(ctx, "welcome")
    
    // 带参数的消息翻译
    userData := map[string]interface{}{
        "name": "John",
    }
    userCreatedMsg := middleware.TWithMap(ctx, "user.created", userData)
    
    // 验证错误消息
    if email == "" {
        errorMsg := middleware.T(ctx, "user.errors.invalid_email")
        http.Error(w, errorMsg, http.StatusBadRequest)
        return
    }
    
    // 返回多语言响应
    response := map[string]interface{}{
        "message": userCreatedMsg,
        "welcome": welcomeMsg,
        "data": userData,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// 程序化设置语言
func setLanguageHandler(w http.ResponseWriter, r *http.Request) {
    newLang := r.URL.Query().Get("lang")
    if newLang != "" {
        // 设置新语言到上下文
        newCtx := middleware.SetLanguage(r.Context(), newLang)
        
        // 在新上下文中处理请求
        msg := middleware.T(newCtx, "welcome")
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{
            "message": msg,
            "language": newLang,
        })
    }
}
```

### 2. Access 中间件

**功能：** 访问日志记录，支持多种输出格式。

#### 配置示例

```yaml
middleware:
  access:
    enabled: true
    # 记录级别
    level: "info"           # info, debug, warn, error
    # 输出格式
    format: "json"          # json, text, combined, custom
    # 字段配置
    fields:
      basic: true           # 基础字段
      headers: true         # 请求头
      body: false          # 请求体（谨慎开启）
      response: true        # 响应信息
    # 过滤器
    filters:
      ignore_paths:
        - "/health"
        - "/metrics"
        - "/favicon.ico"
      ignore_user_agents:
        - "kube-probe/*"
        - "Prometheus/*"
      min_latency: "10ms"   # 最小记录延迟
    # 输出目标
    outputs:
      - type: "file"
        path: "/var/log/access.log"
        rotate: true
        max_size: "100MB"
        max_age: "30d"
      - type: "console"
        colored: true
      - type: "syslog"
        network: "udp"
        address: "localhost:514"
```

## 🔧 开发类中间件

### 1. PProf 中间件

**功能：** 集成 Go 性能分析工具。

#### 配置示例

```yaml
middleware:
  pprof:
    enabled: true
    path_prefix: "/debug/pprof"
    # 安全配置
    auth:
      enabled: true
      token: "debug-secret-token"
      allowed_ips:
        - "127.0.0.1"
        - "::1"
        - "10.0.0.0/8"
    # 场景配置
    scenarios:
      cpu_profile: true      # CPU 分析
      heap_profile: true     # 内存分析
      goroutine: true        # 协程分析
      block: true           # 阻塞分析
      mutex: true           # 锁竞争分析
      trace: true           # 执行追踪
    # 自动采集配置
    auto_collect:
      enabled: true
      interval: "5m"        # 采集间隔
      cpu_duration: "30s"   # CPU 分析时长
      output_dir: "/tmp/pprof"
```

#### 使用示例

```bash
# 获取 CPU 分析
curl "http://localhost:8080/debug/pprof/profile?seconds=30" -H "Authorization: Bearer debug-secret-token" > cpu.prof

# 分析 CPU 性能
go tool pprof cpu.prof

# 获取内存分析
curl "http://localhost:8080/debug/pprof/heap" -H "Authorization: Bearer debug-secret-token" > heap.prof

# 分析内存使用
go tool pprof heap.prof

# 查看所有 Goroutines
curl "http://localhost:8080/debug/pprof/goroutine?debug=2" -H "Authorization: Bearer debug-secret-token"
```

### 2. Banner 中间件

**功能：** 显示服务启动信息和横幅。

#### 配置示例

```yaml
middleware:
  banner:
    enabled: true
    # 横幅模板
    template: |
      ╔══════════════════════════════════════════════════════════════╗
      ║                    🚀 Go RPC Gateway                         ║
      ║                                                              ║
      ║  Version: {{.Version}}                                       ║
      ║  Build:   {{.BuildTime}}                                     ║
      ║  Go:      {{.GoVersion}}                                     ║
      ║                                                              ║
      ║  HTTP:    http://{{.HTTPHost}}:{{.HTTPPort}}                 ║
      ║  gRPC:    {{.GRPCHost}}:{{.GRPCPort}}                        ║
      ║  Health:  http://{{.HTTPHost}}:{{.HTTPPort}}/health          ║
      ║  Metrics: http://{{.HTTPHost}}:{{.HTTPPort}}/metrics         ║
      ║                                                              ║
      ║  Environment: {{.Environment}}                               ║
      ║  Debug Mode:  {{.Debug}}                                     ║
      ╚══════════════════════════════════════════════════════════════╝
    # 颜色配置
    colors:
      enabled: true
      title: "cyan"
      info: "green"
      warning: "yellow"
      error: "red"
    # 显示配置
    show_system_info: true    # 显示系统信息
    show_middleware: true     # 显示启用的中间件
    show_routes: false        # 显示注册的路由
```

## 🔧 自定义中间件开发

### 1. 实现 HTTP 中间件

```go
// HTTP 中间件接口
type HTTPMiddleware func(http.Handler) http.Handler

// 自定义中间件示例
func CustomMiddleware(config *CustomConfig) HTTPMiddleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 前置处理
            start := time.Now()
            
            // 添加自定义头
            w.Header().Set("X-Custom-Header", config.HeaderValue)
            
            // 验证逻辑
            if !isValid(r) {
                http.Error(w, "Invalid request", http.StatusBadRequest)
                return
            }
            
            // 调用下一个中间件/处理器
            next.ServeHTTP(w, r)
            
            // 后置处理
            duration := time.Since(start)
            log.Printf("Request processed in %v", duration)
        })
    }
}
```

### 2. 实现 gRPC 拦截器

```go
// gRPC 一元拦截器
func CustomUnaryInterceptor(config *CustomConfig) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        // 前置处理
        start := time.Now()
        
        // 添加元数据
        if md, ok := metadata.FromIncomingContext(ctx); ok {
            md.Set("custom-header", config.HeaderValue)
            ctx = metadata.NewIncomingContext(ctx, md)
        }
        
        // 调用处理器
        resp, err := handler(ctx, req)
        
        // 后置处理
        duration := time.Since(start)
        log.Printf("gRPC call %s processed in %v", info.FullMethod, duration)
        
        return resp, err
    }
}

// gRPC 流拦截器
func CustomStreamInterceptor(config *CustomConfig) grpc.StreamServerInterceptor {
    return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
        // 包装流
        wrappedStream := &CustomStreamWrapper{
            ServerStream: stream,
            config:       config,
        }
        
        return handler(srv, wrappedStream)
    }
}
```

### 3. 注册自定义中间件

```go
// 在 Gateway 中注册
func main() {
    gw, _ := gateway.New()
    
    // 注册 HTTP 中间件
    customConfig := &CustomConfig{
        HeaderValue: "custom-value",
    }
    gw.RegisterMiddleware(CustomMiddleware(customConfig))
    
    // 注册 gRPC 拦截器
    gw.RegisterGRPCInterceptor(
        CustomUnaryInterceptor(customConfig),
        CustomStreamInterceptor(customConfig),
    )
    
    gw.Start()
}
```

## 📋 最佳实践

### 1. 中间件顺序

推荐的中间件执行顺序：

1. **Recovery** - 优先捕获 panic
2. **RequestID** - 尽早生成请求ID
3. **Logging** - 记录请求开始
4. **Tracing** - 启动链路追踪
5. **Security** - 安全检查
6. **CORS** - 跨域处理
7. **RateLimit** - 流量控制
8. **Signature** - 签名验证
9. **I18n** - 国际化处理
10. **Metrics** - 指标收集
11. **Business** - 业务处理

### 2. 性能优化

- ✅ 避免在中间件中执行耗时操作
- ✅ 使用上下文传递数据
- ✅ 合理设置超时时间
- ✅ 缓存重复计算结果
- ❌ 不要在中间件中阻塞

### 3. 错误处理

- ✅ 优雅处理中间件错误
- ✅ 记录详细错误日志
- ✅ 返回有意义的错误信息
- ❌ 不要泄露内部错误细节

### 4. 配置管理

- ✅ 使用配置文件管理中间件参数
- ✅ 支持环境变量覆盖
- ✅ 提供合理的默认值
- ❌ 避免在代码中硬编码配置

---

更多信息请参考 [架构设计文档](ARCHITECTURE.md) 和 [GitHub Issues](https://github.com/kamalyes/go-rpc-gateway/issues)。
