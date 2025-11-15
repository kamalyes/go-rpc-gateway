# WSC Package - WebSocket 通信组件

## 📦 概述

`wsc` 是 `go-rpc-gateway` 框架的企业级 WebSocket 通信组件，提供开箱即用的实时通信能力。

**核心特性**:
- ✅ WebSocket/SSE 双协议自动降级
- ✅ 配置驱动，无需编码即可启用
- ✅ 生产级用户信息追踪（IP、设备、地理位置等30+字段）
- ✅ 内置 REST API（发送/广播/统计/在线用户）
- ✅ 灵活的回调扩展机制
- ✅ 完整的类型安全保证

---

## 📁 包结构

```
wsc/
├── adapter.go          # WSC适配器 - 封装 go-wsc Hub
├── user_extractor.go   # 用户信息提取器 - 30+生产级字段
├── builtin_api.go      # 内置 REST API
└── README.md           # 本文档
```

---

## 🚀 快速开始

### 最简配置（3步启用）

**步骤 1**: 配置文件中启用 WSC

```yaml
# config/gateway-dev.yaml
wsc:
  enabled: true
  node_ip: "0.0.0.0"
  node_port: 8080
```

**步骤 2**: 代码中初始化

```go
gw, _ := gateway.NewGateway().
    WithConfigPath("./config/gateway-dev.yaml").
    Build()

gw.InitWSC()  // 自动注册所有路由
gw.Start()
```

**步骤 3**: 客户端连接

```javascript
const ws = new WebSocket('ws://localhost:8080/ws?user_id=user123');
```

**自动注册的路由**:
- `GET /ws` - WebSocket 连接
- `GET /sse` - SSE 连接
- `POST /api/wsc/send` - 发送消息
- `GET /api/wsc/online` - 在线用户
- `GET /api/wsc/stats` - 统计信息

---

---

## ⚙️ 完整配置参考

### 基础配置（必需）

```yaml
wsc:
  # === 基础配置 ===
  enabled: true                    # 是否启用 WSC 功能
  node_ip: "0.0.0.0"               # 节点 IP 地址
  node_port: 8080                  # 节点端口
  heartbeat_interval: 30           # 心跳间隔（秒）
  client_timeout: 90               # 客户端超时时间（秒）
  message_buffer_size: 256         # 消息缓冲区大小
  
  # WebSocket Origin 白名单
  websocket_origins:
    - "*"                          # 开发环境可用 *，生产环境建议指定域名
    # - "https://example.com"      # 生产环境示例
    # - "https://app.example.com"
```

### SSE 配置（可选）

```yaml
wsc:
  # === SSE 配置 ===
  sse_heartbeat: 30                # SSE 心跳间隔（秒）
  sse_timeout: 120                 # SSE 超时时间（秒）
  sse_message_buffer: 100          # SSE 消息缓冲区大小
```

### 分布式配置（可选）

```yaml
wsc:
  # === 分布式节点配置 ===
  distributed:
    enabled: false                 # 是否启用分布式模式
    node_discovery: "redis"        # 节点发现方式: redis | etcd | consul
    node_sync_interval: 30         # 节点同步间隔（秒）
    message_routing: "hash"        # 消息路由策略: hash | random | round-robin
    enable_load_balance: true      # 是否启用负载均衡
    health_check_interval: 10      # 健康检查间隔（秒）
    node_timeout: 60               # 节点超时时间（秒）
    cluster_name: "wsc-cluster"    # 集群名称
```

### Redis 配置（分布式消息）

```yaml
wsc:
  # === Redis 配置（用于分布式消息） ===
  redis:
    enabled: false                 # 是否启用 Redis
    addresses:
      - "localhost:6379"           # Redis 地址列表
    password: ""                   # 密码
    db: 0                          # 数据库编号
    pool_size: 10                  # 连接池大小
    min_idle_conns: 2              # 最小空闲连接
    max_retries: 3                 # 最大重试次数
    pubsub_channel: "wsc:pubsub"   # PubSub 频道
    key_prefix: "wsc:"             # Key 前缀
    message_ttl: 3600              # 消息 TTL（秒）
    
    # Redis 哨兵模式（可选）
    use_sentinel: false
    master_name: ""
    
    # Redis 集群模式（可选）
    use_cluster: false
```

### 群组/广播配置（可选）

```yaml
wsc:
  # === 群组/广播配置 ===
  group:
    enabled: false                 # 是否启用群组功能
    max_group_size: 500            # 最大群组人数
    max_groups_per_user: 100       # 每个用户最大群组数
    enable_broadcast: true         # 是否启用全局广播
    broadcast_rate_limit: 10       # 广播频率限制（次/分钟）
    group_cache_expire: 3600       # 群组缓存过期时间（秒）
    auto_create_group: false       # 是否自动创建群组
```

### 工单配置（客服场景）

```yaml
wsc:
  # === 工单配置 ===
  ticket:
    enabled: true                  # 是否启用工单功能
    max_tickets_per_agent: 10      # 每个客服最大工单数
    auto_assign: true              # 是否自动分配工单
    assign_strategy: "load-balance" # 分配策略: random | load-balance | skill-based
    ticket_timeout: 1800           # 工单超时（秒）
    enable_queueing: true          # 是否启用排队
    queue_timeout: 300             # 排队超时（秒）
    notify_timeout: 30             # 通知超时（秒）
    enable_transfer: true          # 是否启用工单转接
    transfer_max_times: 3          # 最大转接次数
    enable_offline_message: true   # 是否启用离线消息
    offline_message_expire: 86400  # 离线消息过期时间（秒）
```

### 性能优化配置

```yaml
wsc:
  # === 性能配置 ===
  performance:
    max_connections_per_node: 10000 # 每个节点最大连接数
    read_buffer_size: 4            # 读缓冲区大小（KB）
    write_buffer_size: 4           # 写缓冲区大小（KB）
    enable_compression: false      # 是否启用压缩（大消息场景）
    compression_level: 6           # 压缩级别（1-9）
    enable_metrics: true           # 是否启用性能指标
    metrics_interval: 60           # 指标采集间隔（秒）
    enable_slow_log: true          # 是否启用慢日志
    slow_log_threshold: 1000       # 慢日志阈值（毫秒）
```

### 安全配置

```yaml
wsc:
  # === 安全配置 ===
  security:
    enable_auth: true              # 是否启用认证
    enable_encryption: false       # 是否启用加密（TLS）
    enable_rate_limit: true        # 是否启用限流
    max_message_size: 1024         # 最大消息大小（KB）
    
    # 允许的用户类型
    allowed_user_types:
      - "customer"
      - "agent"
      - "admin"
    
    # IP 黑白名单
    blocked_ips: []                # IP 黑名单
    whitelist_ips: []              # IP 白名单
    enable_ip_whitelist: false     # 是否启用 IP 白名单
    
    # 认证配置
    token_expiration: 3600         # Token 过期时间（秒）
    max_login_attempts: 5          # 最大登录尝试次数
    login_lock_duration: 300       # 登录锁定时长（秒）
```

### 中间件限流配置

```yaml
middleware:
  rate-limit:
    enabled: true
    routes:
      # WebSocket 连接限流
      - path: "/ws"
        requests-per-second: 50    # 每秒最大连接数
        burst-size: 100            # 突发容量
        per-user: true             # 按用户限流
      
      # SSE 连接限流
      - path: "/sse"
        requests-per-second: 30
        burst-size: 60
        per-user: true
      
      # API 限流
      - path: "/api/wsc/send"
        requests-per-second: 100
        burst-size: 200
        per-user: true
      
      - path: "/api/wsc/broadcast"
        requests-per-second: 10    # 广播限流（严格）
        burst-size: 20
        per-user: false
```

---

---

## 📖 使用指南

### 1. 基础使用（推荐）

#### 1.1 启用 WSC 功能

```go
package main

import gateway "github.com/kamalyes/go-rpc-gateway"

func main() {
    gw, err := gateway.NewGateway().
        WithConfigPath("./config/gateway-dev.yaml").
        Build()
    if err != nil {
        panic(err)
    }

    // 启用 WSC - 自动注册所有路由
    if err := gw.InitWSC(); err != nil {
        panic(err)
    }

    gw.Start()
    gw.WaitForShutdown()
}
```

#### 1.2 发送消息（服务端）

```go
import (
    "context"
    "github.com/kamalyes/go-rpc-gateway/wsc"
    gowsc "github.com/kamalyes/go-wsc"
)

// 在 HTTP 处理器中发送消息
func sendMessageHandler(gw *gateway.Gateway) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        
        msg := &wsc.HubMessage{
            Type:    gowsc.MessageTypeText,
            To:      "user123",              // 接收者 ID
            Content: "Hello from server!",
            Data: map[string]interface{}{   // 可选的附加数据
                "timestamp": time.Now().Unix(),
                "extra": "metadata",
            },
        }
        
        if err := gw.SendMessage(ctx, msg); err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        
        w.WriteHeader(http.StatusOK)
    }
}
```

#### 1.3 广播消息

```go
func broadcastHandler(gw *gateway.Gateway) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        
        msg := &wsc.HubMessage{
            Type:    gowsc.MessageTypeNotice,
            Content: "系统公告：服务器将于10分钟后维护",
        }
        
        if err := gw.BroadcastMessage(ctx, msg); err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        
        w.WriteHeader(http.StatusOK)
    }
}
```

#### 1.4 获取在线用户

```go
func onlineUsersHandler(gw *gateway.Gateway) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        users := gw.GetOnlineUsers()
        
        json.NewEncoder(w).Encode(map[string]interface{}{
            "count": len(users),
            "users": users,
        })
    }
}
```

#### 1.5 获取统计信息

```go
func statsHandler(gw *gateway.Gateway) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        stats := gw.GetWSCStats()
        json.NewEncoder(w).Encode(stats)
    }
}
```

---

### 2. 高级用法 - 自定义回调

#### 2.1 注册生命周期回调

```go
import (
    "github.com/kamalyes/go-rpc-gateway/middleware"
    gowsc "github.com/kamalyes/go-wsc"
)

func main() {
    gw, _ := gateway.NewGateway().
        WithConfigPath("./config/gateway-dev.yaml").
        Build()

    // 创建回调配置
    callbacks := &middleware.WSCCallbacks{
        // 客户端连接时
        OnClientConnect: func(ctx context.Context, client *gowsc.Client) error {
            log.Printf("[连接] 用户: %s, IP: %v", client.UserID, ctx.Value("remote_ip"))
            
            // 返回 error 会拒绝连接
            if isBlocked(client.UserID) {
                return fmt.Errorf("用户已被封禁")
            }
            return nil
        },

        // 客户端断开时
        OnClientDisconnect: func(ctx context.Context, client *gowsc.Client) {
            log.Printf("[断开] 用户: %s, 在线时长: %v", 
                client.UserID, time.Since(client.LastSeen))
        },

        // 收到消息时
        OnMessageReceived: func(ctx context.Context, client *gowsc.Client, msg *gowsc.HubMessage) bool {
            log.Printf("[消息] %s -> %s: %s", msg.From, msg.To, msg.Content)
            
            // 敏感词过滤
            if containsBadWords(msg.Content) {
                log.Printf("[过滤] 消息包含敏感词")
                return false  // 阻止消息传递
            }
            
            return true  // 允许消息传递
        },

        // 发送消息前
        OnMessageSend: func(ctx context.Context, msg *gowsc.HubMessage) error {
            // 可以修改消息内容或添加元数据
            if msg.Data == nil {
                msg.Data = make(map[string]interface{})
            }
            msg.Data["server_timestamp"] = time.Now().Unix()
            
            return nil
        },

        // 广播前
        OnBroadcast: func(ctx context.Context, msg *gowsc.HubMessage) error {
            // 记录广播日志
            log.Printf("[广播] 类型: %s, 内容: %s", msg.Type, msg.Content)
            return nil
        },

        // 自定义认证
        AuthenticateUser: func(r *http.Request) (string, gowsc.UserType, error) {
            token := r.Header.Get("Authorization")
            if token == "" {
                return "", "", fmt.Errorf("缺少认证token")
            }
            
            // 验证 JWT token（示例）
            userID, err := validateJWT(token)
            if err != nil {
                return "", "", err
            }
            
            // 从 token 中提取用户类型
            userType := gowsc.UserTypeCustomer
            if isAdmin(userID) {
                userType = gowsc.UserTypeAdmin
            }
            
            return userID, userType, nil
        },

        // 错误处理
        OnError: func(ctx context.Context, err error, source string) {
            log.Printf("[错误] 来源: %s, 错误: %v", source, err)
            // 可以发送到监控系统
            sendToMonitoring(source, err)
        },
    }

    // 使用回调启用 WSC
    if err := gw.Server.EnableWSCWithCallbacks(callbacks); err != nil {
        panic(err)
    }

    gw.Start()
    gw.WaitForShutdown()
}
```

---

### 3. 用户信息提取器

#### 3.1 基础使用

```go
import "github.com/kamalyes/go-rpc-gateway/wsc"

// 创建提取器
extractor := wsc.NewUserInfoExtractor()

// 在认证回调中使用
callbacks := &middleware.WSCCallbacks{
    AuthenticateUser: func(r *http.Request) (string, gowsc.UserType, error) {
        // 提取详细用户信息
        userInfo, err := extractor.ExtractUserInfo(r)
        if err != nil {
            return "", "", err
        }
        
        // 记录详细连接信息（生产环境推荐）
        log.Printf("[连接详情] 用户: %s, 真实IP: %s, 设备: %s, 浏览器: %s, 系统: %s",
            userInfo.UserID,
            userInfo.RealIP,
            userInfo.DeviceType,
            userInfo.Browser,
            userInfo.OSName,
        )
        
        // 存储到数据库（可选）
        saveConnectionLog(userInfo)
        
        return userInfo.UserID, userInfo.UserType, nil
    },
}
```

#### 3.2 集成 GeoIP（地理位置）

```go
import "github.com/oschwald/geoip2-golang"

// 创建 GeoIP 数据库读取器
db, _ := geoip2.Open("GeoLite2-City.mmdb")
defer db.Close()

// 创建提取器并添加 GeoIP 查询
extractor := wsc.NewUserInfoExtractor().
    WithGeoIPLookup(func(ip string) (country, region, city, isp string, lat, lon float64) {
        ipAddr := net.ParseIP(ip)
        record, err := db.City(ipAddr)
        if err != nil {
            return
        }
        
        country = record.Country.Names["zh-CN"]
        if len(record.Subdivisions) > 0 {
            region = record.Subdivisions[0].Names["zh-CN"]
        }
        city = record.City.Names["zh-CN"]
        lat = record.Location.Latitude
        lon = record.Location.Longitude
        
        return
    })
```

#### 3.3 集成 User-Agent 解析

```go
import "github.com/mssola/user_agent"

extractor := wsc.NewUserInfoExtractor().
    WithDeviceExtractor(func(uaString string) (platform, browser, os, device string) {
        ua := user_agent.New(uaString)
        
        browser, _ = ua.Browser()
        platform = ua.Platform()
        os = ua.OS()
        
        if ua.Mobile() {
            device = "mobile"
        } else if ua.Tablet() {
            device = "tablet"
        } else {
            device = "desktop"
        }
        
        return
    })
```

#### 3.4 提取的用户信息字段

```go
type UserConnectionInfo struct {
    // 基础身份（必需）
    ClientID string       // 客户端唯一ID
    UserID   string       // 用户ID
    UserType gowsc.UserType // 用户类型
    Role     gowsc.UserRole // 角色
    
    // 网络信息（自动提取）
    RemoteIP     string   // 客户端IP
    RealIP       string   // 真实IP（处理代理）
    ForwardedFor string   // X-Forwarded-For
    Protocol     string   // ws/wss/sse
    TLSVersion   string   // TLS版本
    
    // HTTP 请求信息
    UserAgent    string   // User-Agent
    Origin       string   // Origin
    Referer      string   // Referer
    AcceptLang   string   // 接受的语言
    
    // 客户端信息（需要集成解析器）
    Platform     string   // 平台（iOS/Android/Windows）
    Browser      string   // 浏览器
    OSName       string   // 操作系统
    DeviceType   string   // 设备类型（mobile/tablet/desktop）
    DeviceModel  string   // 设备型号
    AppVersion   string   // App版本
    
    // 地理位置信息（需要集成 GeoIP）
    Country      string   // 国家
    Region       string   // 省/州
    City         string   // 城市
    ISP          string   // 运营商
    Latitude     float64  // 纬度
    Longitude    float64  // 经度
    
    // 认证信息
    Token        string   // 认证Token
    SessionID    string   // 会话ID
    AuthMethod   string   // 认证方式
    
    // 业务信息
    Department   gowsc.Department // 部门（客服）
    Tags         []string         // 用户标签
    
    // 连接状态
    ConnectedAt  time.Time  // 连接时间
    Status       string     // 状态
    
    // 扩展元数据
    Metadata     map[string]interface{}
    CustomFields map[string]interface{}
}
```

---

### 4. 内置 REST API

WSC 提供开箱即用的 REST API，无需编写任何代码。

#### 4.1 API 端点

| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | `/api/wsc/send` | 发送消息给指定用户 | 可选 |
| POST | `/api/wsc/broadcast` | 广播消息（默认禁用） | 需要 |
| GET | `/api/wsc/online` | 获取在线用户列表 | 可选 |
| GET | `/api/wsc/stats` | 获取统计信息 | 可选 |

#### 4.2 发送消息 API

**请求**:
```bash
POST /api/wsc/send
Content-Type: application/json
Authorization: Bearer <token>

{
  "to": "user123",
  "type": "text",
  "content": "Hello, User!",
  "data": {
    "extra": "metadata"
  }
}
```

**响应**:
```json
{
  "success": true,
  "message": "消息已发送",
  "data": {
    "to": "user123",
    "type": "text",
    "time": "2025-11-15T10:30:00Z"
  }
}
```

#### 4.3 广播消息 API

**请求**:
```bash
POST /api/wsc/broadcast
Content-Type: application/json
Authorization: Bearer <admin_token>

{
  "type": "notice",
  "content": "系统维护通知"
}
```

**响应**:
```json
{
  "success": true,
  "message": "广播已发送",
  "data": {
    "type": "notice",
    "time": "2025-11-15T10:30:00Z"
  }
}
```

#### 4.4 在线用户 API

**请求**:
```bash
GET /api/wsc/online
```

**响应**:
```json
{
  "success": true,
  "data": {
    "count": 10,
    "users": ["user1", "user2", "user3"]
  }
}
```

#### 4.5 统计信息 API

**请求**:
```bash
GET /api/wsc/stats
```

**响应**:
```json
{
  "success": true,
  "data": {
    "node_id": "node-1",
    "websocket_count": 100,
    "sse_count": 20,
    "total_connections": 120,
    "messages_sent": 5000,
    "messages_received": 4800
  }
}
```

#### 4.6 自定义 API 配置

```go
import "github.com/kamalyes/go-rpc-gateway/wsc"

// 自定义 API 配置
apiConfig := &wsc.WSCBuiltinAPIConfig{
    EnableSend:    true,   // 启用发送API
    EnableBcast:   true,   // 启用广播API（默认false）
    EnableOnline:  true,   // 启用在线用户API
    EnableStats:   true,   // 启用统计API
    AuthRequired:  true,   // 需要认证
    AdminOnly:     true,   // 广播等敏感操作需要管理员
}

// 创建并注册 API
adapter := wsc.NewWSCAdapter(config)
api := wsc.NewWSCBuiltinAPI(adapter, apiConfig)

// 注册到自定义路径
mux := http.NewServeMux()
api.RegisterRoutes(mux, "/custom/wsc")  // 默认 "/api/wsc"
```

---

---

## 🔧 推荐写法与最佳实践

### 1. 生产环境配置模板

```yaml
# config/gateway-prod.yaml
wsc:
  # 基础配置
  enabled: true
  node_ip: "0.0.0.0"
  node_port: 8080
  heartbeat_interval: 30
  client_timeout: 90
  message_buffer_size: 512          # 生产环境建议512+
  
  # Origin 白名单（必须指定）
  websocket_origins:
    - "https://yourdomain.com"
    - "https://app.yourdomain.com"
  
  # SSE 配置
  sse_heartbeat: 30
  sse_timeout: 120
  sse_message_buffer: 200
  
  # 性能优化
  performance:
    max_connections_per_node: 10000
    read_buffer_size: 8
    write_buffer_size: 8
    enable_compression: true         # 大消息场景启用
    compression_level: 6
    enable_metrics: true            # 启用监控
    metrics_interval: 60
    enable_slow_log: true
    slow_log_threshold: 500         # 500ms
  
  # 安全配置
  security:
    enable_auth: true               # 必须启用
    enable_encryption: true         # 生产环境必须TLS
    enable_rate_limit: true
    max_message_size: 512           # 512KB
    allowed_user_types:
      - "customer"
      - "agent"
    token_expiration: 7200          # 2小时
    max_login_attempts: 5
    login_lock_duration: 600        # 10分钟

# 中间件限流（必需）
middleware:
  rate-limit:
    enabled: true
    routes:
      - path: "/ws"
        requests-per-second: 100
        burst-size: 200
        per-user: true
      - path: "/api/wsc/send"
        requests-per-second: 200
        burst-size: 400
        per-user: true
      - path: "/api/wsc/broadcast"
        requests-per-second: 5      # 严格限制广播
        burst-size: 10
        per-user: false
```

---

### 2. 推荐的代码结构

#### 2.1 项目结构

```
your-project/
├── main.go                 # 入口文件
├── config/
│   ├── gateway-dev.yaml   # 开发环境配置
│   └── gateway-prod.yaml  # 生产环境配置
├── internal/
│   ├── auth/              # 认证模块
│   │   └── jwt.go
│   ├── wsc/               # WSC 业务逻辑
│   │   ├── callbacks.go   # 回调实现
│   │   ├── handlers.go    # 消息处理
│   │   └── monitor.go     # 监控
│   └── models/            # 数据模型
└── pkg/
    └── utils/
```

#### 2.2 认证模块（推荐）

```go
// internal/auth/jwt.go
package auth

import (
    "fmt"
    "github.com/golang-jwt/jwt/v5"
    gowsc "github.com/kamalyes/go-wsc"
)

type JWTAuth struct {
    secretKey []byte
}

func NewJWTAuth(secretKey string) *JWTAuth {
    return &JWTAuth{secretKey: []byte(secretKey)}
}

// ValidateToken 验证 JWT token
func (a *JWTAuth) ValidateToken(tokenString string) (string, gowsc.UserType, error) {
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method")
        }
        return a.secretKey, nil
    })
    
    if err != nil || !token.Valid {
        return "", "", fmt.Errorf("invalid token")
    }
    
    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok {
        return "", "", fmt.Errorf("invalid claims")
    }
    
    userID := claims["user_id"].(string)
    userType := gowsc.UserType(claims["user_type"].(string))
    
    return userID, userType, nil
}
```

#### 2.3 WSC 回调模块（推荐）

```go
// internal/wsc/callbacks.go
package wsc

import (
    "context"
    "log"
    "time"
    
    "your-project/internal/auth"
    "github.com/kamalyes/go-rpc-gateway/middleware"
    gowsc "github.com/kamalyes/go-wsc"
)

type CallbackHandler struct {
    jwtAuth     *auth.JWTAuth
    db          *gorm.DB
    monitor     *Monitor
}

func NewCallbackHandler(jwtAuth *auth.JWTAuth, db *gorm.DB) *CallbackHandler {
    return &CallbackHandler{
        jwtAuth: jwtAuth,
        db:      db,
        monitor: NewMonitor(),
    }
}

// GetCallbacks 获取所有回调配置
func (h *CallbackHandler) GetCallbacks() *middleware.WSCCallbacks {
    return &middleware.WSCCallbacks{
        OnClientConnect:    h.OnClientConnect,
        OnClientDisconnect: h.OnClientDisconnect,
        OnMessageReceived:  h.OnMessageReceived,
        OnMessageSend:      h.OnMessageSend,
        AuthenticateUser:   h.AuthenticateUser,
        OnError:           h.OnError,
    }
}

// OnClientConnect 连接回调
func (h *CallbackHandler) OnClientConnect(ctx context.Context, client *gowsc.Client) error {
    // 记录连接日志
    log.Printf("[WSC] 用户连接: %s, 类型: %s", client.UserID, client.UserType)
    
    // 检查黑名单
    if h.isBlocked(client.UserID) {
        return fmt.Errorf("用户已被封禁")
    }
    
    // 检查并发连接数
    if h.getConnectionCount(client.UserID) >= 5 {
        return fmt.Errorf("超过最大连接数限制")
    }
    
    // 记录到数据库
    h.saveConnectionLog(client)
    
    // 更新监控指标
    h.monitor.IncrementConnections()
    
    return nil
}

// OnClientDisconnect 断开回调
func (h *CallbackHandler) OnClientDisconnect(ctx context.Context, client *gowsc.Client) {
    duration := time.Since(client.LastSeen)
    log.Printf("[WSC] 用户断开: %s, 在线时长: %v", client.UserID, duration)
    
    // 更新在线状态
    h.updateUserStatus(client.UserID, "offline")
    
    // 更新监控指标
    h.monitor.DecrementConnections()
}

// OnMessageReceived 消息接收回调
func (h *CallbackHandler) OnMessageReceived(ctx context.Context, client *gowsc.Client, msg *gowsc.HubMessage) bool {
    // 敏感词过滤
    if h.containsBadWords(msg.Content) {
        log.Printf("[WSC] 消息被过滤: %s -> %s", msg.From, msg.To)
        return false
    }
    
    // 消息审计
    h.auditMessage(msg)
    
    // 更新监控指标
    h.monitor.IncrementMessagesReceived()
    
    return true
}

// OnMessageSend 消息发送回调
func (h *CallbackHandler) OnMessageSend(ctx context.Context, msg *gowsc.HubMessage) error {
    // 添加服务器时间戳
    if msg.Data == nil {
        msg.Data = make(map[string]interface{})
    }
    msg.Data["server_time"] = time.Now().Unix()
    
    // 更新监控指标
    h.monitor.IncrementMessagesSent()
    
    return nil
}

// AuthenticateUser 认证回调
func (h *CallbackHandler) AuthenticateUser(r *http.Request) (string, gowsc.UserType, error) {
    // 从 Header 获取 token
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        return "", "", fmt.Errorf("缺少认证信息")
    }
    
    // 去除 Bearer 前缀
    token := strings.TrimPrefix(authHeader, "Bearer ")
    
    // 验证 JWT
    userID, userType, err := h.jwtAuth.ValidateToken(token)
    if err != nil {
        return "", "", fmt.Errorf("认证失败: %v", err)
    }
    
    return userID, userType, nil
}

// OnError 错误回调
func (h *CallbackHandler) OnError(ctx context.Context, err error, source string) {
    log.Printf("[WSC错误] 来源: %s, 错误: %v", source, err)
    
    // 发送到监控系统（如 Sentry）
    h.monitor.ReportError(source, err)
}

// === 辅助方法 ===

func (h *CallbackHandler) isBlocked(userID string) bool {
    // 从数据库或缓存检查
    var blocked bool
    h.db.Raw("SELECT blocked FROM users WHERE id = ?", userID).Scan(&blocked)
    return blocked
}

func (h *CallbackHandler) getConnectionCount(userID string) int {
    // 从 Redis 获取当前连接数
    return 0  // 实现略
}

func (h *CallbackHandler) saveConnectionLog(client *gowsc.Client) {
    // 保存连接日志到数据库
    // 实现略
}

func (h *CallbackHandler) containsBadWords(content string) bool {
    // 敏感词检测
    // 实现略
    return false
}

func (h *CallbackHandler) auditMessage(msg *gowsc.HubMessage) {
    // 消息审计
    // 实现略
}
```

#### 2.4 监控模块（推荐）

```go
// internal/wsc/monitor.go
package wsc

import (
    "sync/atomic"
    "github.com/prometheus/client_golang/prometheus"
)

type Monitor struct {
    connections      int64
    messagesSent     int64
    messagesReceived int64
    
    // Prometheus 指标
    connectionsGauge   prometheus.Gauge
    messagesSentCounter prometheus.Counter
    messagesRecvCounter prometheus.Counter
}

func NewMonitor() *Monitor {
    m := &Monitor{
        connectionsGauge: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "wsc_connections_total",
            Help: "Total number of WSC connections",
        }),
        messagesSentCounter: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "wsc_messages_sent_total",
            Help: "Total number of messages sent",
        }),
        messagesRecvCounter: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "wsc_messages_received_total",
            Help: "Total number of messages received",
        }),
    }
    
    // 注册 Prometheus 指标
    prometheus.MustRegister(m.connectionsGauge)
    prometheus.MustRegister(m.messagesSentCounter)
    prometheus.MustRegister(m.messagesRecvCounter)
    
    return m
}

func (m *Monitor) IncrementConnections() {
    atomic.AddInt64(&m.connections, 1)
    m.connectionsGauge.Inc()
}

func (m *Monitor) DecrementConnections() {
    atomic.AddInt64(&m.connections, -1)
    m.connectionsGauge.Dec()
}

func (m *Monitor) IncrementMessagesSent() {
    atomic.AddInt64(&m.messagesSent, 1)
    m.messagesSentCounter.Inc()
}

func (m *Monitor) IncrementMessagesReceived() {
    atomic.AddInt64(&m.messagesReceived, 1)
    m.messagesRecvCounter.Inc()
}

func (m *Monitor) ReportError(source string, err error) {
    // 发送到 Sentry 或其他错误追踪系统
    // 实现略
}
```

#### 2.5 主程序集成（推荐）

```go
// main.go
package main

import (
    "log"
    
    gateway "github.com/kamalyes/go-rpc-gateway"
    "your-project/internal/auth"
    "your-project/internal/wsc"
)

func main() {
    // 创建 Gateway
    gw, err := gateway.NewGateway().
        WithConfigPath("./config/gateway-prod.yaml").
        WithEnvironment(goconfig.EnvProduction).
        WithHotReload(nil).
        Build()
    if err != nil {
        log.Fatal(err)
    }

    // 初始化认证
    jwtAuth := auth.NewJWTAuth("your-secret-key")
    
    // 初始化数据库（示例）
    db := initDatabase()
    
    // 创建 WSC 回调处理器
    callbackHandler := wsc.NewCallbackHandler(jwtAuth, db)
    
    // 使用回调启用 WSC
    if err := gw.Server.EnableWSCWithCallbacks(callbackHandler.GetCallbacks()); err != nil {
        log.Fatal(err)
    }

    // 注册业务路由
    registerBusinessRoutes(gw)

    // 启动
    if err := gw.Start(); err != nil {
        log.Fatal(err)
    }

    // 优雅关闭
    gw.WaitForShutdown()
}

func initDatabase() *gorm.DB {
    // 数据库初始化
    // 实现略
    return nil
}

func registerBusinessRoutes(gw *gateway.Gateway) {
    // 注册业务路由
    // 实现略
}
```

---

### 3. 安全配置建议

#### 3.1 必须配置项（生产环境）

```yaml
wsc:
  security:
    enable_auth: true              # ✅ 必须
    enable_encryption: true        # ✅ 必须（使用TLS）
    enable_rate_limit: true        # ✅ 必须
    
  websocket_origins:               # ✅ 必须指定
    - "https://yourdomain.com"
    # ❌ 生产环境禁止使用 "*"
```

#### 3.2 认证最佳实践

```go
// ✅ 推荐：使用 JWT 认证
AuthenticateUser: func(r *http.Request) (string, gowsc.UserType, error) {
    token := extractBearerToken(r)
    return validateJWT(token)
}

// ❌ 不推荐：从 URL 参数获取（不安全）
AuthenticateUser: func(r *http.Request) (string, gowsc.UserType, error) {
    userID := r.URL.Query().Get("user_id")  // 不安全
    return userID, gowsc.UserTypeCustomer, nil
}
```

#### 3.3 限流配置建议

```yaml
middleware:
  rate-limit:
    routes:
      # WebSocket 连接 - 严格限制
      - path: "/ws"
        requests-per-second: 50    # 每秒50个连接
        per-user: true             # 按用户限流
      
      # 发送消息 - 中等限制
      - path: "/api/wsc/send"
        requests-per-second: 200
        per-user: true
      
      # 广播 - 严格限制（仅管理员）
      - path: "/api/wsc/broadcast"
        requests-per-second: 5     # 严格限制
        per-user: false
```

---

### 4. 性能优化建议

#### 4.1 连接数优化

```yaml
wsc:
  performance:
    max_connections_per_node: 10000   # 根据服务器配置调整
    message_buffer_size: 512          # 高并发场景增大缓冲区
```

#### 4.2 大消息场景

```yaml
wsc:
  performance:
    enable_compression: true          # 启用压缩
    compression_level: 6              # 平衡压缩率和性能
  security:
    max_message_size: 2048            # 2MB（根据需求调整）
```

#### 4.3 心跳优化

```yaml
wsc:
  heartbeat_interval: 30              # 30秒（移动网络建议20-30）
  client_timeout: 90                  # 超时时间 = 心跳间隔 × 3
```

---

### 5. 监控与日志

#### 5.1 启用监控

```yaml
wsc:
  performance:
    enable_metrics: true
    metrics_interval: 60
    enable_slow_log: true
    slow_log_threshold: 500           # 500ms
```

#### 5.2 集成 Prometheus

```go
import (
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// 在 main.go 中注册 Prometheus 端点
gw.RegisterHTTPRoute("/metrics", promhttp.Handler())
```

#### 5.3 日志记录建议

```go
OnClientConnect: func(ctx context.Context, client *gowsc.Client) error {
    // ✅ 推荐：结构化日志
    log.WithFields(log.Fields{
        "event":     "client_connect",
        "user_id":   client.UserID,
        "user_type": client.UserType,
        "ip":        ctx.Value("remote_ip"),
        "timestamp": time.Now(),
    }).Info("客户端连接")
    
    return nil
}
```

---

### 6. 常见问题与解决方案

#### Q1: 如何处理大量并发连接？

**A**: 
1. 增大 `max_connections_per_node`
2. 增大 `message_buffer_size`
3. 启用分布式模式
4. 使用负载均衡

```yaml
wsc:
  performance:
    max_connections_per_node: 20000
    message_buffer_size: 1024
  distributed:
    enabled: true
    node_discovery: "redis"
```

#### Q2: 如何实现消息持久化？

**A**: 在回调中保存消息到数据库

```go
OnMessageReceived: func(ctx context.Context, client *gowsc.Client, msg *gowsc.HubMessage) bool {
    // 保存到数据库
    db.Create(&Message{
        From:    msg.From,
        To:      msg.To,
        Content: msg.Content,
        SentAt:  time.Now(),
    })
    return true
}
```

#### Q3: 如何实现离线消息？

**A**: 配置离线消息功能

```yaml
wsc:
  ticket:
    enable_offline_message: true
    offline_message_expire: 86400     # 24小时
```

#### Q4: 如何限制单用户连接数？

**A**: 在连接回调中检查

```go
OnClientConnect: func(ctx context.Context, client *gowsc.Client) error {
    count := getConnectionCount(client.UserID)
    if count >= 5 {
        return fmt.Errorf("超过最大连接数限制")
    }
    return nil
}
```

---

---

## 📊 配置参数完整说明

| 配置项 | 类型 | 默认值 | 说明 | 建议值（生产） |
|--------|------|--------|------|----------------|
| **基础配置** |
| `enabled` | bool | false | 是否启用 WSC | true |
| `node_ip` | string | "0.0.0.0" | 节点 IP | "0.0.0.0" |
| `node_port` | int | 8080 | 节点端口 | 8080 |
| `heartbeat_interval` | int | 30 | 心跳间隔（秒） | 30 |
| `client_timeout` | int | 90 | 客户端超时（秒） | 90 |
| `message_buffer_size` | int | 256 | 消息缓冲区 | 512-1024 |
| `websocket_origins` | []string | ["*"] | 允许的 Origin | 指定域名列表 |
| **SSE 配置** |
| `sse_heartbeat` | int | 30 | SSE 心跳（秒） | 30 |
| `sse_timeout` | int | 120 | SSE 超时（秒） | 120 |
| `sse_message_buffer` | int | 100 | SSE 缓冲区 | 200 |
| **性能配置** |
| `max_connections_per_node` | int | 10000 | 最大连接数 | 根据服务器 |
| `read_buffer_size` | int | 4 | 读缓冲（KB） | 8 |
| `write_buffer_size` | int | 4 | 写缓冲（KB） | 8 |
| `enable_compression` | bool | false | 是否压缩 | true（大消息） |
| `compression_level` | int | 6 | 压缩级别 | 6 |
| `enable_metrics` | bool | true | 启用监控 | true |
| `slow_log_threshold` | int | 1000 | 慢日志阈值（ms） | 500 |
| **安全配置** |
| `enable_auth` | bool | true | 启用认证 | true |
| `enable_encryption` | bool | false | 启用加密 | true（TLS） |
| `enable_rate_limit` | bool | true | 启用限流 | true |
| `max_message_size` | int | 1024 | 最大消息（KB） | 512-2048 |
| `token_expiration` | int | 3600 | Token 过期（秒） | 7200 |

---

## 🔗 相关链接

- **上游依赖**:
  - [go-wsc](https://github.com/kamalyes/go-wsc) - WebSocket Hub 核心
  - [go-config](https://github.com/kamalyes/go-config) - 配置管理

- **框架文档**:
  - [go-rpc-gateway 主文档](../README.md)
  - [WSC 快速开始](../docs/WSC_QUICK_START.md)
  - [中间件指南](../docs/MIDDLEWARE_GUIDE.md)

- **第三方工具**:
  - [geoip2-golang](https://github.com/oschwald/geoip2-golang) - GeoIP 查询
  - [user_agent](https://github.com/mssola/user_agent) - User-Agent 解析
  - [jwt-go](https://github.com/golang-jwt/jwt) - JWT 认证

---

## 📝 版本历史

### v1.0.0 (2025-11-15)
- ✅ 独立 wsc 包
- ✅ 生产级用户信息提取器（30+字段）
- ✅ 内置 REST API
- ✅ 完整的回调机制
- ✅ 类型安全保证

---

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

**开发建议**:
1. Fork 项目
2. 创建特性分支
3. 提交代码
4. 创建 Pull Request

---

## 📄 许可证

Copyright (c) 2025 by kamalyes, All Rights Reserved.

---

## 💡 总结

### ✅ DO（推荐）

1. **配置管理**
   - 使用配置文件启用/禁用功能
   - 生产环境指定 Origin 白名单
   - 启用 TLS 加密
   - 配置合理的限流策略

2. **认证授权**
   - 使用 JWT 认证
   - 实现 `AuthenticateUser` 回调
   - 检查用户权限
   - 记录认证日志

3. **消息处理**
   - 使用回调过滤敏感内容
   - 记录消息审计日志
   - 实现消息持久化
   - 处理离线消息

4. **监控运维**
   - 启用 Prometheus 监控
   - 记录结构化日志
   - 配置慢日志阈值
   - 定期查看统计信息

5. **性能优化**
   - 根据并发量调整缓冲区
   - 大消息场景启用压缩
   - 合理配置心跳间隔
   - 使用连接池

### ❌ DON'T（不推荐）

1. **安全问题**
   - ❌ 生产环境使用 `origins: ["*"]`
   - ❌ 从 URL 参数获取敏感信息
   - ❌ 不启用 TLS 加密
   - ❌ 不配置限流

2. **性能问题**
   - ❌ 过小的缓冲区
   - ❌ 过短的心跳间隔
   - ❌ 不启用压缩（大消息）
   - ❌ 不限制连接数

3. **维护问题**
   - ❌ 不记录日志
   - ❌ 不启用监控
   - ❌ 硬编码配置
   - ❌ 不处理错误

---

**架构优势**: 模块化、配置驱动、生产就绪、易于扩展！🎉
