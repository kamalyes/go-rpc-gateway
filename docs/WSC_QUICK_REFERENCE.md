# WebSocket 快速参考

## 3 分钟快速开始

### 1️⃣ 配置启用

```yaml
# config.yaml
wsc:
  enabled: true
  node_ip: "0.0.0.0"
  node_port: 8080
  websocket_origins:
    - "*"
```

### 2️⃣ 初始化中间件

```go
import (
    wscconfig "github.com/kamalyes/go-config/pkg/wsc"
    "github.com/kamalyes/go-rpc-gateway/middleware"
)

// 从配置创建中间件
wscMiddleware := middleware.NewWSCMiddleware(&middleware.WSCConfig{
    Config: wscConfig,
    Callbacks: &middleware.WSCCallbacks{
        AuthenticateUser: func(r *http.Request) (userID string, userType gowsc.UserType, err error) {
            userID = r.URL.Query().Get("user_id")  // 或从 Header/Token 获取
            return userID, gowsc.UserTypeCustomer, nil
        },
    },
})

// 注册到路由器
wscMiddleware.RegisterRoutes(router)
```

### 3️⃣ 注册 API

```go
import "github.com/kamalyes/go-rpc-gateway/handlers"

wsAPI := handlers.NewWebSocketAPI(wscMiddleware.GetAdapter())
apiV1 := router.Group("/api/v1")
wsAPI.RegisterRoutes(apiV1)
```

完成！✅

---

## 常用操作

### 发送消息（代码）

```go
adapter := wscMiddleware.GetAdapter()

msg := &gowsc.HubMessage{
    From:     "user1",
    To:       "user2",
    Type:     gowsc.MessageTypeText,
    Content:  "Hello!",
    CreateAt: time.Now(),
}

adapter.SendMessage(ctx, msg)
```

### 广播消息（代码）

```go
msg := &gowsc.HubMessage{
    From:     "system",
    Type:     gowsc.MessageTypeNotice,
    Content:  "System update!",
    CreateAt: time.Now(),
}

adapter.Broadcast(ctx, msg)
```

### 发送消息（HTTP）

```bash
curl -X POST http://localhost:8080/api/v1/websocket/send \
  -H "X-User-ID: user1" \
  -d '{"to":"user2","content":"Hi"}'
```

### 广播消息（HTTP）

```bash
curl -X POST http://localhost:8080/api/v1/websocket/broadcast \
  -d '{"content":"System announcement","type":"notice"}'
```

### 查询在线用户

```bash
curl http://localhost:8080/api/v1/websocket/online
```

### 获取统计信息

```bash
curl http://localhost:8080/wsc/stats
```

---

## WebSocket 客户端示例

### JavaScript

```javascript
// 连接
const ws = new WebSocket('ws://localhost:8080/ws?user_id=alice');

ws.onopen = () => console.log('Connected');

// 发送消息
ws.send(JSON.stringify({
    to: 'bob',
    type: 'text',
    content: 'Hello Bob!',
    from: 'alice',
    create_at: new Date().toISOString()
}));

// 接收消息
ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    console.log('Message from', msg.from, ':', msg.content);
};

ws.onclose = () => console.log('Disconnected');
```

### Python

```python
import asyncio
import websockets
import json
from datetime import datetime

async def websocket_client():
    uri = "ws://localhost:8080/ws?user_id=alice"
    async with websockets.connect(uri) as ws:
        # 发送消息
        msg = {
            "to": "bob",
            "type": "text",
            "content": "Hello Bob!",
            "from": "alice",
            "create_at": datetime.now().isoformat()
        }
        await ws.send(json.dumps(msg))
        
        # 接收消息
        async for message in ws:
            data = json.loads(message)
            print(f"Message from {data['from']}: {data['content']}")

asyncio.run(websocket_client())
```

---

## 自定义回调示例

### 消息验证

```go
OnMessageReceived: func(ctx context.Context, client *gowsc.Client, msg *gowsc.HubMessage) bool {
    // 内容校验
    if len(msg.Content) > 1000 {
        log.Warn("Message too long")
        return false
    }
    
    // 发送者身份验证
    if msg.From != client.UserID {
        log.Warn("Sender mismatch")
        return false
    }
    
    return true  // 允许
},
```

### 日志记录

```go
OnMessageSend: func(ctx context.Context, msg *gowsc.HubMessage) error {
    // 记录消息到数据库
    db.Create(&Message{
        FromUserID: msg.From,
        ToUserID:   msg.To,
        Content:    msg.Content,
        Type:       string(msg.Type),
        CreatedAt:  msg.CreateAt,
    })
    return nil
},
```

### 用户状态同步

```go
OnClientConnect: func(ctx context.Context, client *gowsc.Client) error {
    // 更新用户在线状态
    db.Model(&User{}).Where("id=?", client.UserID).Update("status", "online")
    
    // 通知其他用户
    msg := &gowsc.HubMessage{
        Type:    gowsc.MessageTypeSystem,
        From:    "system",
        Content: fmt.Sprintf("%s is online", client.UserID),
    }
    wscAdapter.Broadcast(ctx, msg)
    
    return nil
},

OnClientDisconnect: func(ctx context.Context, client *gowsc.Client) {
    db.Model(&User{}).Where("id=?", client.UserID).Update("status", "offline")
},
```

---

## API 响应格式

### 成功

```json
{
    "success": true,
    "message": "Operation successful",
    "data": {...}
}
```

### 失败

```json
{
    "success": false,
    "error": "Error message",
    "message": null
}
```

---

## 常见问题

| 问题 | 解决方案 |
|------|--------|
| 连接被拒绝 | 确保 URL 中包含 `user_id` 参数 |
| CORS 错误 | 在配置中添加 `websocket_origins` |
| 消息未送达 | 检查目标用户是否在线：`GET /api/v1/websocket/online` |
| 高延迟 | 增加 `heartbeat_interval`，减少 `message_buffer_size` |
| 连接断开 | 检查 `client_timeout` 设置 |

---

## 内置端点速查

| 路径 | 方法 | 说明 |
|------|------|------|
| `/ws` | WS | WebSocket 连接 |
| `/sse` | HTTP | Server-Sent Events |
| `/api/v1/websocket/send` | POST | 发送消息 |
| `/api/v1/websocket/broadcast` | POST | 广播消息 |
| `/api/v1/websocket/online` | GET | 在线用户 |
| `/api/v1/websocket/stats` | GET | 统计信息 |
| `/wsc/stats` | GET | WSC 统计 |
| `/wsc/online` | GET | WSC 在线用户 |

---

## 更多资源

- 📖 完整文档：[WSC_INTEGRATION_GUIDE.md](./WSC_INTEGRATION_GUIDE.md)
- 🔗 go-wsc 库：https://github.com/kamalyes/go-wsc
- 📝 示例代码：[examples/websocket/](../examples/websocket/)
