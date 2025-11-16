# 🚀 WebSocket 快速开始指南

## 30 秒快速启动

### 1. 配置
```yaml
# config/gateway.yaml
wsc:
  enabled: true
  node_ip: "0.0.0.0"
  node_port: 8081
```

### 2. 启动
```go
gw, _ := gateway.NewGateway().
    WithConfigPath("./config.yaml").
    BuildAndStart()
```

✅ 完成！WebSocket 服务已启动，监听 `ws://0.0.0.0:8081`

---

## 常用模式速查

### 📌 连接事件
```go
gw.OnWebSocketClientConnect(func(ctx context.Context, client *wsc.Client) error {
    log.Printf("✓ 客户端已连接: %s", client.ID)
    return nil
})
```

### 📌 消息处理
```go
gw.OnWebSocketMessageReceived(func(ctx context.Context, client *wsc.Client, msg *wsc.HubMessage) error {
    log.Printf("📩 收到消息: %s", msg.Content)
    // 业务处理
    return nil
})
```

### 📌 断开连接
```go
gw.OnWebSocketClientDisconnect(func(ctx context.Context, client *wsc.Client, reason string) error {
    log.Printf("✗ 客户端断开: %s (原因: %s)", client.ID, reason)
    return nil
})
```

### 📌 广播消息
```go
hub := gw.GetWebSocketService().GetHub()
hub.Broadcast(context.Background(), &wsc.HubMessage{
    Type:    wsc.MessageTypeText,
    Content: "Hello all clients",
})
```

### 📌 点对点消息
```go
hub.SendToUser(context.Background(), "user123", &wsc.HubMessage{
    Type:    wsc.MessageTypeText,
    To:      "user123",
    Content: "Hello user123",
})
```

### 📌 工单消息
```go
hub.SendToTicket(context.Background(), "ticket_001", &wsc.HubMessage{
    Type:     wsc.MessageTypeText,
    TicketID: "ticket_001",
    Content:  "Ticket message",
})
```

### 📌 中间件（CORS + 认证）
```go
gw.
    // CORS
    UseWebSocketMiddleware(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Access-Control-Allow-Origin", "*")
            next.ServeHTTP(w, r)
        })
    }).
    // 认证
    UseWebSocketMiddleware(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := r.Header.Get("Authorization")
            if token == "" {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            next.ServeHTTP(w, r)
        })
    })
```

### 📌 事件驱动
```go
gw.
    OnWebSocketEvent("websocket.started", func(event *server.WebSocketEvent) {
        log.Println("✓ WebSocket 服务已启动")
    }).
    OnWebSocketEvent("client.connected", func(event *server.WebSocketEvent) {
        log.Printf("✓ 新客户端: %s", event.ClientID)
    }).
    OnWebSocketEvent("client.disconnected", func(event *server.WebSocketEvent) {
        log.Printf("✗ 客户端已断开: %s", event.ClientID)
    })
```

### 📌 统计信息
```go
stats := gw.GetWebSocketService().GetStats()
log.Printf("当前连接: %d", stats.CurrentConnections)
log.Printf("收到消息: %d", stats.TotalMessagesReceived)
log.Printf("发送消息: %d", stats.TotalMessagesSent)
```

### 📌 自定义拦截器
```go
type AuditInterceptor struct{}

func (a *AuditInterceptor) Name() string { return "audit" }
func (a *AuditInterceptor) Order() int { return 1 }
func (a *AuditInterceptor) Intercept(ctx context.Context, req interface{}, next InterceptorHandler) (interface{}, error) {
    log.Printf("📝 审计: %v", req)
    return next(ctx, req)
}

gw.AddWebSocketInterceptor(&AuditInterceptor{})
```

---

## 📊 配置参考

### 基础配置
```yaml
wsc:
  enabled: true              # 启用 WebSocket
  node_ip: "0.0.0.0"        # 监听 IP
  node_port: 8081           # 监听端口
  heartbeat_interval: 30    # 心跳间隔（秒）
  client_timeout: 90        # 客户端超时（秒）
  message_buffer_size: 256  # 消息缓冲大小
```

### CORS 配置
```yaml
wsc:
  websocket_origins:
    - "http://localhost:3000"
    - "http://localhost:5173"
    - "https://example.com"
```

### 性能配置
```yaml
wsc:
  performance:
    max_connections_per_node: 10000
    read_buffer_size: 4
    write_buffer_size: 4
    enable_compression: false
    enable_metrics: true
```

### 安全配置
```yaml
wsc:
  security:
    enable_auth: true
    enable_rate_limit: true
    max_message_size: 1024        # KB
    allowed_user_types:
      - "customer"
      - "agent"
      - "admin"
```

### 群组配置
```yaml
wsc:
  group:
    enabled: true
    max_group_size: 500
    max_groups_per_user: 100
```

### 工单配置
```yaml
wsc:
  ticket:
    enabled: true
    max_tickets_per_agent: 10
    auto_assign: true
    ticket_timeout: 1800
```

---

## 🔗 API 速查表

### Gateway 方法

| 方法 | 链式 | 用途 |
|-----|------|------|
| `GetWebSocketService()` | ✗ | 获取 WebSocket 服务 |
| `IsWebSocketEnabled()` | ✗ | 检查启用状态 |
| `OnWebSocketClientConnect()` | ✓ | 连接回调 |
| `OnWebSocketClientDisconnect()` | ✓ | 断开回调 |
| `OnWebSocketMessageReceived()` | ✓ | 消息接收回调 |
| `OnWebSocketMessageSent()` | ✓ | 消息发送回调 |
| `OnWebSocketError()` | ✓ | 错误处理 |
| `UseWebSocketMiddleware()` | ✓ | 中间件 |
| `OnWebSocketEvent()` | ✓ | 事件订阅 |
| `AddWebSocketInterceptor()` | ✓ | 拦截器 |

### Hub 方法

| 方法 | 功能 |
|-----|------|
| `Broadcast(ctx, msg)` | 广播消息给所有连接 |
| `SendToUser(ctx, userID, msg)` | 点对点消息 |
| `SendToTicket(ctx, ticketID, msg)` | 工单消息 |
| `BroadcastToGroup(ctx, groupID, msg)` | 群组消息 |
| `Shutdown()` | 关闭 Hub |
| `GetClients()` | 获取所有客户端 |
| `GetClient(clientID)` | 获取单个客户端 |

---

## ✅ 检查清单

启动应用前：
- [ ] `wsc.enabled: true` 已在配置中设置
- [ ] `node_port` 未被占用
- [ ] 必要的中间件已添加

运行时调试：
- [ ] `gw.IsWebSocketEnabled()` 返回 true
- [ ] WebSocket 服务已启动（日志中有 "WebSocket server listening" 消息）
- [ ] 客户端可以连接到 `ws://host:port`

---

## 🐛 常见问题

**Q: WebSocket 服务没有启动？**
```
A: 检查配置: wsc.enabled: true
```

**Q: 客户端连接被拒绝？**
```
A: 检查 CORS 配置 websocket_origins
A: 检查认证中间件是否配置正确
```

**Q: 看不到连接日志？**
```
A: 添加: OnWebSocketEvent("client.connected", ...)
A: 或 OnWebSocketClientConnect(...)
```

**Q: 消息没有被接收？**
```
A: 检查 OnWebSocketMessageReceived 是否注册
A: 检查消息格式是否正确
```

---

## 📚 详细文档

- 📖 **完整指南**: `WEBSOCKET-INTEGRATION-GUIDE.md`
- 📋 **完成报告**: `WEBSOCKET-INTEGRATION-COMPLETION-REPORT.md`
- 🏗️ **架构文档**: `WEBSOCKET-INTEGRATION-ARCHITECTURE.md`
- 💡 **使用示例**: `examples/websocket_example.go`

---

**准备好了？开始使用 WebSocket 吧！** 🚀
