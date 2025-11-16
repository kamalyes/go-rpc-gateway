# go-wsc 完整集成指南

## 概述

`go-rpc-gateway` 已完整集成 `go-wsc` 的所有核心能力。集成采用**最小化包装**策略：仅提供 HTTP WebSocket 升级、配置初始化、回调链管理，所有消息处理能力**直接委托给 go-wsc Hub**。

## 核心设计

### WebSocketService 职责

| 职责 | 实现 |
|------|------|
| HTTP 升级 | `handleWebSocketUpgrade()` 处理 `/ws` 路由 |
| 配置初始化 | `NewWebSocketService()` 从 go-config 读取 WSC 配置 |
| 生命周期 | `Start()` / `Stop()` 管理 HTTP 服务器 |
| 回调链 | `OnClientConnect/Disconnect/MessageReceived/Error` |
| Hub 委托 | 所有消息 API 直接转发到 `go-wsc.Hub` |

### 状态管理优化

- `running` 字段使用 `atomic.Bool`（轻量级，无锁）
- 回调链使用 `sync.RWMutex`（仅保护并发访问）
- go-wsc Hub 自身有完整的并发控制

## 完整 API 清单

### 消息发送 API

```go
// 发送消息给特定用户
SendToUser(ctx context.Context, userID string, msg *wsc.HubMessage) error

// 发送消息给特定用户（带 ACK 和自动重试）
SendToUserWithAck(ctx context.Context, userID string, msg *wsc.HubMessage, 
                  timeout time.Duration, maxRetry int) (*wsc.AckMessage, error)

// 发送消息给特定凭证 ID（用于分布式场景）
SendToTicket(ctx context.Context, ticketID string, msg *wsc.HubMessage) error

// 发送消息给特定凭证 ID（带 ACK 和自动重试）
SendToTicketWithAck(ctx context.Context, ticketID string, msg *wsc.HubMessage, 
                    timeout time.Duration, maxRetry int) (*wsc.AckMessage, error)

// 广播消息给所有在线客户端
Broadcast(ctx context.Context, msg *wsc.HubMessage)
```

### 查询 API

```go
// 获取所有在线用户列表
GetOnlineUsers() []string

// 获取在线用户数量
GetOnlineUserCount() int

// 获取详细统计信息
GetStats() map[string]interface{}
```

### 事件回调 API

```go
// 客户端连接事件
OnClientConnect(cb ClientConnectCallback) *WebSocketService

// 客户端断开事件
OnClientDisconnect(cb ClientDisconnectCallback) *WebSocketService

// 接收消息事件
OnMessageReceived(cb MessageReceivedCallback) *WebSocketService

// 错误事件
OnError(cb ErrorCallback) *WebSocketService
```

### 访问器

```go
// 获取底层 go-wsc Hub 实例（高级操作）
GetHub() *wsc.Hub

// 获取 WSC 配置
GetConfig() *wscconfig.WSC
```

## Gateway 快捷 API

所有能力也暴露在 Gateway 层，可以直接调用：

```go
gw := gateway.NewGateway()...BuildAndStart()

// 发送消息给用户
gw.SendToWebSocketUser(ctx, "user123", msg)

// 发送带 ACK 的消息
ack, err := gw.SendToWebSocketUserWithAck(ctx, "user123", msg, timeout, retry)

// 广播
gw.BroadcastWebSocketMessage(ctx, msg)

// 查询
users := gw.GetWebSocketOnlineUsers()
count := gw.GetWebSocketOnlineUserCount()
stats := gw.GetWebSocketStats()
```

## 使用示例

### 基础用法

```go
wsSvc := gw.GetWebSocketService()
ctx := context.Background()

// 发送消息
msg := &wsc.HubMessage{
    From:     "admin",
    Content:  "Hello",
    Type:     wsc.MessageTypeText,
    CreateAt: time.Now(),
}

if err := wsSvc.SendToUser(ctx, "user123", msg); err != nil {
    log.Printf("Failed: %v", err)
}
```

### 带 ACK 的消息（确保投递）

```go
ack, err := wsSvc.SendToUserWithAck(ctx, "user123", msg, 5*time.Second, 3)
if err != nil {
    log.Printf("Failed: %v", err)
} else {
    log.Printf("ACK Status: %s", ack.Status)
}
```

### 事件处理

```go
wsSvc.
    OnClientConnect(func(ctx context.Context, client *wsc.Client) error {
        log.Printf("User %s connected", client.UserID)
        return nil
    }).
    OnClientDisconnect(func(ctx context.Context, client *wsc.Client, reason string) error {
        log.Printf("User %s disconnected: %s", client.UserID, reason)
        return nil
    }).
    OnMessageReceived(func(ctx context.Context, client *wsc.Client, msg *wsc.HubMessage) error {
        log.Printf("Message from %s: %s", client.UserID, msg.Content)
        return nil
    }).
    OnError(func(ctx context.Context, err error, severity string) error {
        log.Printf("Error [%s]: %v", severity, err)
        return nil
    })
```

### 广播

```go
broadcastMsg := &wsc.HubMessage{
    From:     "system",
    Content:  "Announcement",
    Type:     wsc.MessageTypeText,
    CreateAt: time.Now(),
}
wsSvc.Broadcast(ctx, broadcastMsg)
```

## 配置

WebSocket 配置通过 go-config 的 WSC 模块管理：

```yaml
wsc:
  enabled: true
  node_ip: "0.0.0.0"
  node_port: 8081
  heartbeat_interval: 30
  client_timeout: 90
  message_buffer_size: 256
  
  websocket_origins:
    - "*"
  
  performance:
    enable_metrics: true
    read_buffer_size: 1024
    write_buffer_size: 1024
  
  distributed:
    enabled: false
  
  redis:
    enabled: false
```

## 分布式部署

### 基于 Ticket 的分布式消息

在多服务器部署中，使用 `SendToTicket` 实现跨节点消息路由：

```go
// ticket 可以是：用户ID、会话ID、连接ID 等
// go-wsc Hub 会根据分布式配置自动路由消息
msg := &wsc.HubMessage{
    From:     "backend",
    Content:  "Cross-server message",
    Type:     wsc.MessageTypeText,
    CreateAt: time.Now(),
}

if err := wsSvc.SendToTicket(ctx, "session_xyz", msg); err != nil {
    log.Printf("Failed: %v", err)
}
```

### Redis 配置（分布式 ACK 支持）

启用 Redis 配置后，go-wsc Hub 会自动支持分布式 ACK 和消息确认：

```yaml
distributed:
  enabled: true

redis:
  enabled: true
  host: "localhost"
  port: 6379
  # ... 其他 Redis 配置
```

## 性能优化

1. **原子操作**：`running` 状态使用 `atomic.Bool`，避免锁开销
2. **RWMutex**：回调链使用读写锁，多数时间无锁
3. **Hub 锁**：go-wsc Hub 内部使用高效的并发控制
4. **异步处理**：HTTP 服务器在 goroutine 中运行

## 扩展点

### 自定义客户端初始化

覆盖 `OnClientConnect` 回调：

```go
wsSvc.OnClientConnect(func(ctx context.Context, client *wsc.Client) error {
    // 验证用户
    if !isValidUser(client.UserID) {
        return fmt.Errorf("invalid user")
    }
    
    // 加载用户状态
    loadUserState(client.UserID)
    
    // 发送欢迎消息
    welcomeMsg := &wsc.HubMessage{...}
    return wsSvc.SendToUser(ctx, client.UserID, welcomeMsg)
})
```

### 消息过滤和处理

在 `OnMessageReceived` 中处理：

```go
wsSvc.OnMessageReceived(func(ctx context.Context, client *wsc.Client, msg *wsc.HubMessage) error {
    // 验证消息
    if len(msg.Content) > 10000 {
        return fmt.Errorf("message too long")
    }
    
    // 过滤敏感词
    msg.Content = filterContent(msg.Content)
    
    // 记录日志
    logMessage(client.UserID, msg)
    
    // 继续处理（客户端的消息路由）
    return nil
})
```

## 常见场景

### 1. 私聊系统

```go
msg := &wsc.HubMessage{
    From:    sender,
    To:      recipient,
    Content: messageBody,
    Type:    wsc.MessageTypeText,
}
wsSvc.SendToUser(ctx, recipient, msg)
```

### 2. 聊天群组

```go
// 广播给所有人
wsSvc.Broadcast(ctx, msg)
```

### 3. 实时通知

```go
// 带 ACK，确保投递
ack, _ := wsSvc.SendToUserWithAck(ctx, userID, notification, 30*time.Second, 5)
```

### 4. 在线状态查询

```go
onlineUsers := wsSvc.GetOnlineUsers()
count := wsSvc.GetOnlineUserCount()
stats := wsSvc.GetStats()
```

## 总结

- ✅ **完整暴露** go-wsc Hub 的所有能力
- ✅ **最小化包装** 只做必要的 HTTP 升级和配置初始化
- ✅ **开箱即用** 从 Gateway 直接调用 WebSocket 方法
- ✅ **灵活扩展** 通过回调链注入自定义业务逻辑
- ✅ **高性能** 使用 atomic 和高效并发控制
- ✅ **分布式支持** 通过 Ticket 和 Redis 支持多服务器部署
