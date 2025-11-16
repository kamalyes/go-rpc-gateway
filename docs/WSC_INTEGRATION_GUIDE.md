# WebSocket 通信（WSC）集成指南

## 概述

`go-rpc-gateway` 现已基于 `go-wsc` 库重构了 WebSocket 通信能力，所有使用该框架的人都能开箱即用实时通信功能。

## 架构

```
┌─────────────────────────────────────────────┐
│         Application Layer                   │
│   (使用 API 或直接调用中间件)                   │
└────────────────┬────────────────────────────┘
                 │
┌────────────────▼────────────────────────────┐
│     middleware/wsc.go (WSCMiddleware)       │
│   • 自定义回调机制                             │
│   • 认证、消息拦截                            │
│   • 路由注册 (/ws, /sse, /stats, /online)    │
└────────────────┬────────────────────────────┘
                 │
┌────────────────▼────────────────────────────┐
│     wsc/adapter.go (WSCAdapter)             │
│   • go-wsc Hub 的唯一适配层                   │
│   • WebSocket 和 SSE 协议处理                │
│   • 消息路由和用户管理                        │
└────────────────┬────────────────────────────┘
                 │
┌────────────────▼────────────────────────────┐
│     go-wsc (Hub)                            │
│   • 核心实时通信引擎                         │
│   • 连接管理、消息分发、聚类                  │
└─────────────────────────────────────────────┘
```

## 核心组件

### 1. WSCAdapter (`wsc/adapter.go`)

**职责**：`go-wsc Hub` 与 `go-rpc-gateway` 的唯一桥接层

**关键方法**：
```go
// 协议处理
HandleWebSocket(w http.ResponseWriter, r *http.Request)  // WebSocket 连接升级
HandleSSE(w http.ResponseWriter, r *http.Request)        // SSE 连接处理

// 消息操作
SendMessage(ctx context.Context, msg *HubMessage) error  // 发送单点消息
Broadcast(ctx context.Context, msg *HubMessage) error    // 广播消息

// 查询接口
GetOnlineUsers() []string                                 // 获取在线用户列表
GetStats() map[string]interface{}                         // 获取统计信息
GetNodeID() string                                        // 获取节点 ID
IsEnabled() bool                                          // 是否已启用
```

### 2. WSCMiddleware (`middleware/wsc.go`)

**职责**：统一的中间件层，支持自定义回调和路由注册

**特点**：
- ✅ 自定义回调：客户端连接/断开、消息拦截、认证
- ✅ 统一的路由注册机制
- ✅ 自动认证处理
- ✅ 内置错误处理

**回调接口**：
```go
type WSCCallbacks struct {
    OnClientConnect      // 客户端连接时
    OnClientDisconnect   // 客户端断开时
    OnMessageReceived    // 收到消息时
    OnMessageSend        // 发送消息前
    OnBroadcast          // 广播前
    AuthenticateUser     // 自定义认证
    OnError              // 错误处理
}
```

### 3. WebSocketAPI (`handlers/websocket.go`)

**职责**：HTTP API 层，提供 REST 接口

**内置 API 端点**：
```
POST   /websocket/send          # 发送单点消息
POST   /websocket/broadcast     # 广播消息
GET    /websocket/stats         # 获取统计信息
GET    /websocket/online        # 获取在线用户列表
```

## 快速开始

### 1. 配置文件设置

在你的 `config.yaml` 中启用 WebSocket：

```yaml
wsc:
  enabled: true
  node_ip: "0.0.0.0"
  node_port: 8080
  heartbeat_interval: 30        # 心跳间隔（秒）
  client_timeout: 60            # 客户端超时（秒）
  message_buffer_size: 256      # 消息缓冲大小
  websocket_origins:            # CORS 白名单
    - "*"
```

### 2. 初始化中间件

在 `server.go` 中：

```go
import (
    wscconfig "github.com/kamalyes/go-config/pkg/wsc"
    "github.com/kamalyes/go-rpc-gateway/middleware"
)

// 创建 WSC 中间件
wscMiddleware := middleware.NewWSCMiddleware(&middleware.WSCConfig{
    Config: wscConfig,  // 从配置加载
    Callbacks: &middleware.WSCCallbacks{
        AuthenticateUser: func(r *http.Request) (userID string, userType gowsc.UserType, err error) {
            // 实现自己的认证逻辑
            userID = r.URL.Query().Get("user_id")
            return userID, gowsc.UserTypeCustomer, nil
        },
    },
})

// 注册中间件
gateway.Use(wscMiddleware)
```

### 3. 注册 API 路由

```go
// 创建 WebSocket API 处理器
adapter := wscMiddleware.GetAdapter()
wsAPI := handlers.NewWebSocketAPI(adapter)

// 注册路由
apiGroup := router.Group("/api/v1")
wsAPI.RegisterRoutes(apiGroup)
```

## 使用示例

### 客户端连接（WebSocket）

```javascript
// 连接到 WebSocket
const ws = new WebSocket('ws://localhost:8080/ws?user_id=user123&user_type=customer');

ws.onopen = function() {
    console.log('Connected');
    
    // 发送消息给特定用户
    ws.send(JSON.stringify({
        to: 'user456',
        type: 'text',
        content: 'Hello!',
        from: 'user123',
        create_at: new Date().toISOString()
    }));
};

ws.onmessage = function(event) {
    const msg = JSON.parse(event.data);
    console.log('Received:', msg);
};
```

### 发送消息（HTTP API）

```bash
# 发送单点消息
curl -X POST http://localhost:8080/api/v1/websocket/send \
  -H "Content-Type: application/json" \
  -H "X-User-ID: user123" \
  -d '{
    "to": "user456",
    "content": "Hello from HTTP!",
    "type": "text",
    "data": {"priority": "high"}
  }'

# 广播消息
curl -X POST http://localhost:8080/api/v1/websocket/broadcast \
  -H "Content-Type: application/json" \
  -H "X-User-ID: admin" \
  -d '{
    "content": "System announcement",
    "type": "notice",
    "data": {"level": "warning"}
  }'

# 获取在线用户
curl http://localhost:8080/api/v1/websocket/online

# 获取统计信息
curl http://localhost:8080/api/v1/websocket/stats
```

### 自定义回调示例

```go
// 实现消息拦截
callbacks := &middleware.WSCCallbacks{
    OnMessageReceived: func(ctx context.Context, client *gowsc.Client, msg *gowsc.HubMessage) bool {
        // 内容审核
        if containsBadWords(msg.Content) {
            log.Warn("Bad content detected from", client.UserID)
            return false  // 阻止消息继续传递
        }
        
        // 记录消息
        saveMessageLog(msg)
        
        return true  // 允许消息继续传递
    },
    
    OnClientConnect: func(ctx context.Context, client *gowsc.Client) error {
        // 更新用户在线状态
        db.Model(&User{}).Where("id = ?", client.UserID).Update("online", true)
        
        // 发送欢迎消息
        welcome := &gowsc.HubMessage{
            Type:    gowsc.MessageTypeSystem,
            From:    "system",
            To:      client.UserID,
            Content: fmt.Sprintf("Welcome back, %s!", client.UserID),
        }
        hub.SendToUser(ctx, client.UserID, welcome)
        
        return nil
    },
    
    OnClientDisconnect: func(ctx context.Context, client *gowsc.Client) {
        // 更新用户离线状态
        db.Model(&User{}).Where("id = ?", client.UserID).Update("online", false)
        log.Info("User disconnected:", client.UserID)
    },
}

wscMiddleware := middleware.NewWSCMiddleware(&middleware.WSCConfig{
    Config:    wscConfig,
    Callbacks: callbacks,
})
```

## 内置 API 参考

### 1. 发送消息

**请求**：
```
POST /api/v1/websocket/send
Content-Type: application/json
X-User-ID: user123

{
    "to": "target_user_id",
    "content": "message content",
    "type": "text",                // optional, default: text
    "data": {                       // optional
        "priority": "high",
        "custom_field": "value"
    }
}
```

**响应**：
```json
{
    "success": true,
    "message": "Message sent successfully",
    "data": {
        "to": "target_user_id",
        "type": "text"
    }
}
```

### 2. 广播消息

**请求**：
```
POST /api/v1/websocket/broadcast
Content-Type: application/json

{
    "content": "broadcast message",
    "type": "notice",               // optional
    "data": {}                      // optional
}
```

**响应**：
```json
{
    "success": true,
    "message": "Broadcast sent successfully",
    "delivered_count": 42
}
```

### 3. 获取在线用户

**请求**：
```
GET /api/v1/websocket/online
```

**响应**：
```json
{
    "total": 42,
    "users": [
        {
            "user_id": "user123",
            "type": "websocket",
            "status": "connected",
            "connected_at": "2025-11-15T10:30:00Z",
            "last_ping": "2025-11-15T10:35:00Z"
        }
    ]
}
```

### 4. 获取统计信息

**请求**：
```
GET /api/v1/websocket/stats
```

**响应**：
```json
{
    "active_connections": 42,
    "connections_by_type": {
        "websocket": 40,
        "sse": 2
    },
    "last_updated": "2025-11-15T10:35:00Z"
}
```

## 关键特性

### ✅ 开箱即用
- 无需编写任何代码，仅需配置即可启用 WebSocket
- 提供现成的 HTTP API 和中间件

### ✅ 高度可定制
- 支持认证、消息拦截、连接生命周期回调
- 支持自定义错误处理

### ✅ 协议灵活性
- 支持 WebSocket 长连接
- 支持 SSE（Server-Sent Events）兼容降级
- 支持 HTTP 短连接 API

### ✅ 生产就绪
- 内置心跳机制和超时控制
- 自动消息缓冲和重试
- 完整的统计信息和在线用户管理

## 故障排查

### 问题 1：WebSocket 连接被拒绝

**原因**：缺少用户 ID

**解决方案**：确保在连接 URL 或 Header 中提供 `user_id`：
```
ws://localhost:8080/ws?user_id=user123
```

或者在 Header 中：
```
X-User-ID: user123
```

### 问题 2：跨域请求失败

**原因**：CORS 设置不正确

**解决方案**：在配置中设置 `websocket_origins`：
```yaml
wsc:
  websocket_origins:
    - "http://localhost:3000"
    - "https://app.example.com"
```

### 问题 3：消息发送失败

**原因**：目标用户不在线或消息内容无效

**解决方案**：检查目标用户是否在线：
```bash
curl http://localhost:8080/api/v1/websocket/online
```

## 性能优化建议

1. **调整缓冲区大小**：根据高峰消息量调整 `message_buffer_size`
2. **心跳间隔**：增加 `heartbeat_interval` 可降低服务器负载
3. **连接超时**：根据业务需求调整 `client_timeout`
4. **消息队列**：在高并发场景下考虑使用消息队列

## 更新日志

### v1.0.0 (2025-11-15)
- ✨ 基于 go-wsc 的完整重构
- 🎯 统一的中间件架构
- 📚 完整的 API 文档
- 🔧 自定义回调支持
