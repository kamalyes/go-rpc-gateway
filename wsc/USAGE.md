# WSC 生产级使用指南

## 目录

- [快速开始](#快速开始)
- [生产高可用配置](#生产高可用配置)
  - [单机高性能配置](#单机高性能配置)
  - [分布式集群配置](#分布式集群配置)
  - [多可用区部署](#多可用区部署)
  - [容器化部署配置](#容器化部署配置)
- [详细用法](#详细用法)
  - [基础消息发送](#基础消息发送)
  - [广播消息](#广播消息)
  - [用户在线管理](#用户在线管理)
  - [自定义回调](#自定义回调)
  - [用户信息提取](#用户信息提取)
  - [内置 API 调用](#内置-api-调用)
- [高级特性](#高级特性)
  - [消息压缩](#消息压缩)
  - [消息加密](#消息加密)
  - [消息持久化](#消息持久化)
  - [离线消息推送](#离线消息推送)
  - [消息优先级](#消息优先级)
- [性能优化](#性能优化)
  - [连接池优化](#连接池优化)
  - [内存优化](#内存优化)
  - [CPU 优化](#cpu-优化)
  - [网络优化](#网络优化)
- [监控告警](#监控告警)
  - [指标收集](#指标收集)
  - [日志采集](#日志采集)
  - [告警规则](#告警规则)
- [故障处理](#故障处理)
  - [连接异常](#连接异常)
  - [消息丢失](#消息丢失)
  - [性能瓶颈](#性能瓶颈)
  - [内存泄漏](#内存泄漏)
- [最佳实践](#最佳实践)

---

## 快速开始

### 1. 启用 WSC 功能

在 `config/gateway-dev.yaml` 中配置:

```yaml
wsc:
  enabled: true
  port: 8081
  path: /ws
```

### 2. 在业务代码中使用

```go
// 发送消息给指定用户
gateway.SendMessage("user123", &wsc.HubMessage{
    Type: "notification",
    Data: map[string]interface{}{
        "title": "系统通知",
        "content": "您有新的订单",
    },
})

// 广播消息给所有在线用户
gateway.BroadcastMessage(&wsc.HubMessage{
    Type: "announcement",
    Data: map[string]interface{}{
        "content": "系统将在 5 分钟后维护",
    },
})
```

---

## 生产高可用配置

### 单机高性能配置

**适用场景**: 单节点部署,承载 10 万+ 并发连接

```yaml
wsc:
  enabled: true
  port: 8081
  path: /ws
  
  # 连接管理 - 单机优化
  max_connections: 100000           # 最大连接数
  read_buffer_size: 4096            # 读缓冲区 4KB
  write_buffer_size: 4096           # 写缓冲区 4KB
  
  # 心跳配置 - 保持连接存活
  heartbeat:
    enabled: true
    interval: 30s                   # 30 秒心跳间隔
    timeout: 90s                    # 90 秒超时断开
    
  # 消息队列 - 高吞吐量
  message_queue:
    size: 10000                     # 单连接消息队列 1 万
    worker_count: 8                 # 8 个消息处理协程
    
  # 性能优化
  compression:
    enabled: true                   # 启用压缩
    level: 1                        # 快速压缩 (1-9)
    min_size: 1024                  # >1KB 才压缩
    
  # 限流保护
  rate_limit:
    enabled: true
    requests_per_second: 100        # 每秒 100 条消息
    burst: 200                      # 突发 200 条
    
  # 资源限制
  limits:
    max_message_size: 1048576       # 单条消息 1MB
    max_frame_size: 1048576         # 单帧 1MB
    
  # 监控
  metrics:
    enabled: true
    port: 9090
    path: /metrics
    
  # 日志
  log:
    level: info
    format: json
    output: /var/log/wsc/wsc.log
    max_size: 100                   # 100MB 轮转
    max_backups: 10
    max_age: 30                     # 保留 30 天
```

**推荐硬件配置**:
- CPU: 8 核
- 内存: 16 GB
- 网络: 千兆网卡
- 磁盘: SSD (用于日志)

### 分布式集群配置

**适用场景**: 多节点集群,承载百万级并发连接,跨节点消息路由

```yaml
wsc:
  enabled: true
  port: 8081
  path: /ws
  
  # Redis 分布式支持
  distributed:
    enabled: true
    mode: redis                     # 使用 Redis 作为消息总线
    
  # Redis 配置
  redis:
    addrs:
      - "redis-master-1:6379"
      - "redis-master-2:6379"
      - "redis-master-3:6379"
    mode: cluster                   # Redis 集群模式
    
    # 高可用配置
    pool_size: 100                  # 连接池大小
    min_idle_conns: 10              # 最小空闲连接
    max_retries: 3                  # 重试次数
    dial_timeout: 5s
    read_timeout: 3s
    write_timeout: 3s
    pool_timeout: 4s
    
    # 哨兵模式 (可选,与 cluster 二选一)
    # mode: sentinel
    # master_name: mymaster
    # sentinel_addrs:
    #   - "sentinel-1:26379"
    #   - "sentinel-2:26379"
    #   - "sentinel-3:26379"
    
  # 连接管理
  max_connections: 50000            # 单节点 5 万连接
  
  # 心跳配置
  heartbeat:
    enabled: true
    interval: 30s
    timeout: 90s
    
  # 消息队列
  message_queue:
    size: 5000
    worker_count: 16                # 增加 worker 处理分布式消息
    
  # 性能优化
  compression:
    enabled: true
    level: 1
    min_size: 1024
    
  # 限流保护
  rate_limit:
    enabled: true
    requests_per_second: 100
    burst: 200
    
  # 监控
  metrics:
    enabled: true
    port: 9090
    path: /metrics
    labels:
      cluster: "prod-wsc"
      region: "us-west"
      node: "${NODE_ID}"            # 节点标识,用于区分不同实例
```

**集群拓扑**:
```
┌─────────────────────────────────────────────────────────┐
│                    Load Balancer                        │
│                  (Nginx/HAProxy)                        │
└─────────────┬──────────────┬──────────────┬────────────┘
              │              │              │
    ┌─────────▼──────┐ ┌────▼──────┐ ┌─────▼─────────┐
    │  WSC Node 1    │ │ WSC Node 2│ │  WSC Node 3   │
    │  10.0.1.10     │ │ 10.0.1.11 │ │  10.0.1.12    │
    └─────────┬──────┘ └────┬──────┘ └─────┬─────────┘
              │              │              │
              └──────────────┼──────────────┘
                             │
                   ┌─────────▼──────────┐
                   │  Redis Cluster     │
                   │  redis-1:6379      │
                   │  redis-2:6379      │
                   │  redis-3:6379      │
                   └────────────────────┘
```

**部署脚本 (Kubernetes)**:

```yaml
# wsc-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: wsc-gateway
spec:
  replicas: 3                       # 3 个副本
  selector:
    matchLabels:
      app: wsc-gateway
  template:
    metadata:
      labels:
        app: wsc-gateway
    spec:
      containers:
      - name: gateway
        image: your-registry/go-rpc-gateway:v1.0.0
        ports:
        - containerPort: 8081
          name: wsc
        - containerPort: 9090
          name: metrics
        env:
        - name: NODE_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        resources:
          requests:
            cpu: "2"
            memory: "4Gi"
          limits:
            cpu: "4"
            memory: "8Gi"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5

---
apiVersion: v1
kind: Service
metadata:
  name: wsc-gateway
spec:
  type: LoadBalancer
  ports:
  - port: 8081
    targetPort: 8081
    name: wsc
  selector:
    app: wsc-gateway
```

### 多可用区部署

**适用场景**: 跨地域高可用,容灾备份

```yaml
wsc:
  enabled: true
  port: 8081
  path: /ws
  
  # 分布式配置
  distributed:
    enabled: true
    mode: redis
    
  # Redis 多可用区配置
  redis:
    addrs:
      # 可用区 A
      - "redis-az-a-1:6379"
      - "redis-az-a-2:6379"
      # 可用区 B
      - "redis-az-b-1:6379"
      - "redis-az-b-2:6379"
      # 可用区 C
      - "redis-az-c-1:6379"
      - "redis-az-c-2:6379"
    mode: cluster
    
    # 高可用参数
    max_redirects: 3                # 集群重定向次数
    read_only: false
    route_by_latency: true          # 按延迟路由
    route_randomly: false
    
  # 连接管理
  max_connections: 30000            # 单节点降低连接数
  
  # 心跳配置 - 跨区网络不稳定时加大超时
  heartbeat:
    enabled: true
    interval: 45s                   # 增加心跳间隔
    timeout: 120s                   # 增加超时时间
    
  # 消息队列
  message_queue:
    size: 8000
    worker_count: 12
    
  # 监控 - 多可用区标签
  metrics:
    enabled: true
    port: 9090
    path: /metrics
    labels:
      cluster: "prod-wsc"
      availability_zone: "${AZ}"    # 可用区标识
      region: "${REGION}"
      node: "${NODE_ID}"
```

**拓扑架构**:
```
┌────────────────────────────────────────────────────────────┐
│                     Global DNS / CDN                       │
│                  (Route53 / CloudFlare)                    │
└──────────┬─────────────────────────┬──────────────────────┘
           │                         │
┌──────────▼───────────┐   ┌─────────▼──────────┐
│  Availability Zone A │   │ Availability Zone B│
│  ┌─────────────────┐ │   │ ┌─────────────────┐│
│  │ WSC Node 1/2    │ │   │ │ WSC Node 3/4    ││
│  │ Load Balancer   │ │   │ │ Load Balancer   ││
│  └────────┬────────┘ │   │ └────────┬────────┘│
│           │          │   │          │         │
│  ┌────────▼────────┐ │   │ ┌────────▼────────┐│
│  │ Redis Cluster   │ │   │ │ Redis Cluster   ││
│  │ (Replica Set)   │ │   │ │ (Replica Set)   ││
│  └─────────────────┘ │   │ └─────────────────┘│
└──────────────────────┘   └────────────────────┘
```

### 容器化部署配置

**Docker Compose 配置**:

```yaml
# docker-compose.yml
version: '3.8'

services:
  # WSC Gateway 服务
  wsc-gateway:
    image: your-registry/go-rpc-gateway:v1.0.0
    container_name: wsc-gateway
    ports:
      - "8081:8081"     # WSC 端口
      - "9090:9090"     # Metrics 端口
    environment:
      - NODE_ID=gateway-1
      - REGION=us-west
    volumes:
      - ./config:/app/config
      - ./logs:/var/log/wsc
    networks:
      - wsc-network
    depends_on:
      - redis-master
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 8G
        reservations:
          cpus: '2'
          memory: 4G
    restart: unless-stopped
    
  # Redis 主节点
  redis-master:
    image: redis:7-alpine
    container_name: redis-master
    ports:
      - "6379:6379"
    command: redis-server --appendonly yes --maxmemory 2gb --maxmemory-policy allkeys-lru
    volumes:
      - redis-data:/data
    networks:
      - wsc-network
    restart: unless-stopped
    
  # Prometheus 监控
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus-data:/prometheus
    networks:
      - wsc-network
    restart: unless-stopped
    
  # Grafana 可视化
  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana-data:/var/lib/grafana
    networks:
      - wsc-network
    restart: unless-stopped

volumes:
  redis-data:
  prometheus-data:
  grafana-data:

networks:
  wsc-network:
    driver: bridge
```

---

## 详细用法

### 基础消息发送

#### 发送给单个用户

```go
// 场景: 订单状态更新通知
func NotifyOrderStatus(userID string, orderID string, status string) error {
    msg := &wsc.HubMessage{
        Type: "order.status_changed",
        Data: map[string]interface{}{
            "order_id": orderID,
            "status": status,
            "timestamp": time.Now().Unix(),
        },
    }
    
    return gateway.SendMessage(userID, msg)
}
```

#### 发送给多个用户

```go
// 场景: 群聊消息
func SendGroupMessage(groupID string, senderID string, content string) error {
    // 1. 从数据库获取群成员
    members, err := db.GetGroupMembers(groupID)
    if err != nil {
        return err
    }
    
    // 2. 构造消息
    msg := &wsc.HubMessage{
        Type: "group.message",
        Data: map[string]interface{}{
            "group_id": groupID,
            "sender_id": senderID,
            "content": content,
            "timestamp": time.Now().Unix(),
        },
    }
    
    // 3. 批量发送
    var wg sync.WaitGroup
    for _, member := range members {
        wg.Add(1)
        go func(userID string) {
            defer wg.Done()
            gateway.SendMessage(userID, msg)
        }(member.UserID)
    }
    wg.Wait()
    
    return nil
}
```

### 广播消息

#### 全局广播

```go
// 场景: 系统维护公告
func BroadcastMaintenance(startTime time.Time, duration time.Duration) error {
    msg := &wsc.HubMessage{
        Type: "system.maintenance",
        Data: map[string]interface{}{
            "start_time": startTime.Unix(),
            "duration": duration.Seconds(),
            "message": "系统即将维护,请及时保存数据",
        },
    }
    
    return gateway.BroadcastMessage(msg)
}
```

#### 条件广播

```go
// 场景: 向特定地区用户广播促销信息
func BroadcastPromotion(region string, promotion Promotion) error {
    // 1. 获取该地区所有在线用户
    onlineUsers := gateway.GetOnlineUsers()
    
    // 2. 过滤符合条件的用户
    var targetUsers []string
    for _, userID := range onlineUsers {
        userInfo, _ := getUserInfo(userID)
        if userInfo.Region == region {
            targetUsers = append(targetUsers, userID)
        }
    }
    
    // 3. 批量发送
    msg := &wsc.HubMessage{
        Type: "promotion.new",
        Data: map[string]interface{}{
            "title": promotion.Title,
            "discount": promotion.Discount,
            "valid_until": promotion.ValidUntil.Unix(),
        },
    }
    
    for _, userID := range targetUsers {
        gateway.SendMessage(userID, msg)
    }
    
    return nil
}
```

### 用户在线管理

#### 获取在线用户列表

```go
// 场景: 管理后台查看在线用户
func GetOnlineUsersWithDetails() ([]UserOnlineInfo, error) {
    userIDs := gateway.GetOnlineUsers()
    
    var results []UserOnlineInfo
    for _, userID := range userIDs {
        // 从缓存或数据库获取用户详情
        userInfo, err := cache.GetUserInfo(userID)
        if err != nil {
            continue
        }
        
        results = append(results, UserOnlineInfo{
            UserID: userID,
            Username: userInfo.Username,
            ConnectedAt: userInfo.ConnectedAt,
            LastActive: userInfo.LastActive,
        })
    }
    
    return results, nil
}
```

#### 统计在线数据

```go
// 场景: 实时在线统计
func GetOnlineStats() (*OnlineStats, error) {
    stats := gateway.GetStats()
    
    return &OnlineStats{
        TotalConnections: stats.Connections,
        TotalUsers: stats.Users,
        MessagesSent: stats.MessagesSent,
        MessagesReceived: stats.MessagesReceived,
        Uptime: stats.Uptime,
    }, nil
}
```

### 自定义回调

#### 认证回调

```go
// 场景: JWT Token 认证
func SetupWSCAuth() {
    middleware.WSCCallbacks = &middleware.WSCCallbacks{
        OnAuthenticate: func(r *http.Request) (string, error) {
            // 1. 从请求头或查询参数获取 Token
            token := r.Header.Get("Authorization")
            if token == "" {
                token = r.URL.Query().Get("token")
            }
            
            if token == "" {
                return "", errors.New("missing token")
            }
            
            // 2. 验证 Token
            claims, err := jwt.ParseToken(token)
            if err != nil {
                return "", fmt.Errorf("invalid token: %w", err)
            }
            
            // 3. 检查用户状态
            userID := claims.UserID
            if !isUserActive(userID) {
                return "", errors.New("user is inactive")
            }
            
            // 4. 返回用户 ID
            return userID, nil
        },
    }
}
```

#### 连接生命周期回调

```go
// 场景: 记录用户上线/下线,更新在线状态
func SetupWSCLifecycle() {
    middleware.WSCCallbacks = &middleware.WSCCallbacks{
        OnConnect: func(userID string, r *http.Request) {
            // 1. 记录连接日志
            logger.Info("user connected", 
                zap.String("user_id", userID),
                zap.String("ip", getClientIP(r)),
            )
            
            // 2. 更新 Redis 在线状态
            cache.SetUserOnline(userID, true)
            
            // 3. 发送上线通知给好友
            friends, _ := db.GetUserFriends(userID)
            for _, friendID := range friends {
                gateway.SendMessage(friendID, &wsc.HubMessage{
                    Type: "user.online",
                    Data: map[string]interface{}{
                        "user_id": userID,
                        "timestamp": time.Now().Unix(),
                    },
                })
            }
        },
        
        OnDisconnect: func(userID string) {
            // 1. 记录断开日志
            logger.Info("user disconnected", 
                zap.String("user_id", userID),
            )
            
            // 2. 更新 Redis 在线状态
            cache.SetUserOnline(userID, false)
            
            // 3. 发送下线通知
            friends, _ := db.GetUserFriends(userID)
            for _, friendID := range friends {
                gateway.SendMessage(friendID, &wsc.HubMessage{
                    Type: "user.offline",
                    Data: map[string]interface{}{
                        "user_id": userID,
                        "timestamp": time.Now().Unix(),
                    },
                })
            }
        },
    }
}
```

#### 消息处理回调

```go
// 场景: 处理客户端发来的消息
func SetupWSCMessageHandler() {
    middleware.WSCCallbacks = &middleware.WSCCallbacks{
        OnMessage: func(userID string, msg *gowsc.Message) error {
            // 1. 解析消息
            var clientMsg ClientMessage
            if err := json.Unmarshal(msg.Data, &clientMsg); err != nil {
                return err
            }
            
            // 2. 根据消息类型处理
            switch clientMsg.Type {
            case "ping":
                // 心跳响应
                return gateway.SendMessage(userID, &wsc.HubMessage{
                    Type: "pong",
                    Data: map[string]interface{}{
                        "timestamp": time.Now().Unix(),
                    },
                })
                
            case "chat.send":
                // 聊天消息
                return handleChatMessage(userID, clientMsg)
                
            case "typing":
                // 输入状态
                return handleTypingStatus(userID, clientMsg)
                
            default:
                return fmt.Errorf("unknown message type: %s", clientMsg.Type)
            }
        },
    }
}
```

### 用户信息提取

#### 提取详细连接信息

```go
// 场景: 安全审计,提取用户完整连接信息
func SetupUserInfoExtractor() {
    middleware.UserInfoExtractor = func(r *http.Request) map[string]interface{} {
        return map[string]interface{}{
            // 基础信息
            "client_ip": getClientIP(r),
            "user_agent": r.Header.Get("User-Agent"),
            "referer": r.Header.Get("Referer"),
            
            // 设备信息
            "device_type": detectDeviceType(r),
            "device_model": extractDeviceModel(r),
            "os": extractOS(r),
            "os_version": extractOSVersion(r),
            "browser": extractBrowser(r),
            "browser_version": extractBrowserVersion(r),
            
            // 地理位置 (需要 GeoIP 库)
            "country": getCountry(r),
            "city": getCity(r),
            "latitude": getLatitude(r),
            "longitude": getLongitude(r),
            
            // 应用信息
            "app_version": r.Header.Get("X-App-Version"),
            "app_build": r.Header.Get("X-App-Build"),
            
            // 认证信息
            "auth_method": r.Header.Get("X-Auth-Method"),
            
            // 网络信息
            "connection_type": r.Header.Get("X-Connection-Type"), // wifi/4g/5g
            
            // 时间戳
            "connected_at": time.Now().Unix(),
        }
    }
}
```

### 内置 API 调用

#### REST API 使用

```bash
# 1. 发送消息给指定用户
curl -X POST http://localhost:8081/api/wsc/send \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "user_id": "user123",
    "type": "notification",
    "data": {
      "title": "新订单",
      "content": "您有一笔新订单待处理"
    }
  }'

# 2. 广播消息
curl -X POST http://localhost:8081/api/wsc/broadcast \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "type": "announcement",
    "data": {
      "content": "系统升级通知"
    }
  }'

# 3. 获取在线用户
curl -X GET http://localhost:8081/api/wsc/online \
  -H "Authorization: Bearer YOUR_TOKEN"

# 4. 获取统计数据
curl -X GET http://localhost:8081/api/wsc/stats \
  -H "Authorization: Bearer YOUR_TOKEN"
```

#### 在业务代码中调用内置 API

```go
// 场景: 定时任务中批量推送消息
func BatchPushNotifications() error {
    // 1. 获取待推送的通知
    notifications, err := db.GetPendingNotifications()
    if err != nil {
        return err
    }
    
    // 2. 使用内置 API 批量发送
    client := &http.Client{Timeout: 10 * time.Second}
    
    for _, notif := range notifications {
        payload := map[string]interface{}{
            "user_id": notif.UserID,
            "type": "notification",
            "data": notif.Data,
        }
        
        body, _ := json.Marshal(payload)
        req, _ := http.NewRequest("POST", 
            "http://localhost:8081/api/wsc/send",
            bytes.NewReader(body),
        )
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("Authorization", "Bearer "+getInternalToken())
        
        resp, err := client.Do(req)
        if err != nil {
            logger.Error("failed to send notification", zap.Error(err))
            continue
        }
        resp.Body.Close()
        
        // 3. 更新通知状态
        db.UpdateNotificationStatus(notif.ID, "sent")
    }
    
    return nil
}
```

---

## 高级特性

### 消息压缩

```yaml
# 配置启用压缩
wsc:
  compression:
    enabled: true
    level: 6              # 压缩级别 1-9,6 为平衡值
    min_size: 1024        # 消息大于 1KB 才压缩
```

```go
// 业务代码中发送大消息会自动压缩
func SendLargeData(userID string, data []byte) error {
    msg := &wsc.HubMessage{
        Type: "large.data",
        Data: data,  // 自动压缩
    }
    return gateway.SendMessage(userID, msg)
}
```

### 消息加密

```go
// 场景: 敏感数据加密传输
func SendEncryptedMessage(userID string, sensitiveData interface{}) error {
    // 1. 序列化数据
    plaintext, err := json.Marshal(sensitiveData)
    if err != nil {
        return err
    }
    
    // 2. 加密
    ciphertext, err := aes.Encrypt(plaintext, getEncryptionKey(userID))
    if err != nil {
        return err
    }
    
    // 3. 发送
    msg := &wsc.HubMessage{
        Type: "encrypted",
        Data: map[string]interface{}{
            "payload": base64.StdEncoding.EncodeToString(ciphertext),
            "algorithm": "AES-256-GCM",
        },
    }
    
    return gateway.SendMessage(userID, msg)
}
```

### 消息持久化

```go
// 场景: 离线消息存储
func SendWithPersistence(userID string, msg *wsc.HubMessage) error {
    // 1. 尝试发送
    err := gateway.SendMessage(userID, msg)
    
    // 2. 如果用户不在线,存储到数据库
    if err != nil {
        return db.SaveOfflineMessage(&OfflineMessage{
            UserID: userID,
            Type: msg.Type,
            Data: msg.Data,
            CreatedAt: time.Now(),
            ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // 7 天过期
        })
    }
    
    return nil
}

// 用户上线时推送离线消息
func PushOfflineMessages(userID string) error {
    messages, err := db.GetOfflineMessages(userID)
    if err != nil {
        return err
    }
    
    for _, msg := range messages {
        gateway.SendMessage(userID, &wsc.HubMessage{
            Type: msg.Type,
            Data: msg.Data,
        })
        
        // 删除已推送的消息
        db.DeleteOfflineMessage(msg.ID)
    }
    
    return nil
}
```

### 离线消息推送

```go
// 场景: 结合 APNs/FCM 推送离线消息
func SendWithFallback(userID string, msg *wsc.HubMessage) error {
    // 1. 尝试 WebSocket 发送
    err := gateway.SendMessage(userID, msg)
    if err == nil {
        return nil
    }
    
    // 2. 用户不在线,尝试推送通知
    userDevices, err := db.GetUserDevices(userID)
    if err != nil {
        return err
    }
    
    for _, device := range userDevices {
        switch device.Platform {
        case "ios":
            // APNs 推送
            apns.Send(&apns.Notification{
                DeviceToken: device.Token,
                Title: msg.Type,
                Body: fmt.Sprintf("%v", msg.Data),
            })
        case "android":
            // FCM 推送
            fcm.Send(&fcm.Message{
                Token: device.Token,
                Data: msg.Data,
            })
        }
    }
    
    // 3. 同时保存离线消息
    return db.SaveOfflineMessage(&OfflineMessage{
        UserID: userID,
        Type: msg.Type,
        Data: msg.Data,
        CreatedAt: time.Now(),
    })
}
```

### 消息优先级

```go
// 场景: 紧急消息优先发送
func SendWithPriority(userID string, msg *wsc.HubMessage, priority int) error {
    // 扩展消息结构包含优先级
    enrichedMsg := &wsc.HubMessage{
        Type: msg.Type,
        Data: map[string]interface{}{
            "priority": priority,  // 1-5, 5 最高
            "payload": msg.Data,
            "timestamp": time.Now().Unix(),
        },
    }
    
    return gateway.SendMessage(userID, enrichedMsg)
}

// 客户端根据优先级处理
// 高优先级消息: 弹窗、震动、声音提醒
// 低优先级消息: 仅更新列表
```

---

## 性能优化

### 连接池优化

```yaml
wsc:
  # 连接池配置
  max_connections: 100000
  
  # Redis 连接池优化
  redis:
    pool_size: 200              # 增大连接池
    min_idle_conns: 50          # 预热连接
    max_conn_age: 300s          # 连接最大存活时间
    pool_timeout: 5s
    idle_timeout: 300s
    idle_check_frequency: 60s
```

### 内存优化

```yaml
wsc:
  # 消息队列大小控制
  message_queue:
    size: 1000                  # 单连接队列大小
    worker_count: 4             # Worker 数量
    
  # 限制消息大小
  limits:
    max_message_size: 65536     # 64KB
    max_frame_size: 65536
    
  # 内存回收
  gc:
    enabled: true
    interval: 60s               # 每分钟触发一次 GC
```

```go
// 使用对象池减少内存分配
var messagePool = sync.Pool{
    New: func() interface{} {
        return &wsc.HubMessage{}
    },
}

func SendOptimizedMessage(userID string, msgType string, data interface{}) error {
    msg := messagePool.Get().(*wsc.HubMessage)
    defer messagePool.Put(msg)
    
    msg.Type = msgType
    msg.Data = data
    
    return gateway.SendMessage(userID, msg)
}
```

### CPU 优化

```yaml
wsc:
  # Worker 数量与 CPU 核心数匹配
  message_queue:
    worker_count: 16            # 16 核 CPU
    
  # 压缩级别降低
  compression:
    level: 1                    # 快速压缩,降低 CPU 消耗
```

```go
// 使用协程池限制并发
var workerPool = make(chan struct{}, 100)

func SendConcurrent(users []string, msg *wsc.HubMessage) {
    for _, userID := range users {
        workerPool <- struct{}{}
        go func(uid string) {
            defer func() { <-workerPool }()
            gateway.SendMessage(uid, msg)
        }(userID)
    }
}
```

### 网络优化

```yaml
wsc:
  # 缓冲区大小
  read_buffer_size: 8192        # 8KB 读缓冲
  write_buffer_size: 8192       # 8KB 写缓冲
  
  # 启用 TCP_NODELAY
  tcp_nodelay: true
  
  # 启用 Keep-Alive
  tcp_keepalive: true
  tcp_keepalive_period: 60s
```

---

## 监控告警

### 指标收集

```yaml
wsc:
  metrics:
    enabled: true
    port: 9090
    path: /metrics
    
    # 自定义标签
    labels:
      service: "wsc-gateway"
      environment: "production"
      region: "us-west"
```

**Prometheus 配置**:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'wsc-gateway'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 15s
    metrics_path: /metrics
```

**关键指标**:

```promql
# 在线连接数
wsc_connections_total

# 消息发送速率
rate(wsc_messages_sent_total[1m])

# 消息接收速率
rate(wsc_messages_received_total[1m])

# 消息延迟 (P99)
histogram_quantile(0.99, wsc_message_duration_seconds_bucket)

# 错误率
rate(wsc_errors_total[1m])

# 连接成功率
rate(wsc_connections_accepted_total[1m]) / rate(wsc_connections_attempted_total[1m])
```

### 日志采集

```yaml
wsc:
  log:
    level: info
    format: json
    output: /var/log/wsc/wsc.log
    
    # 日志轮转
    max_size: 100               # 100MB
    max_backups: 30             # 保留 30 个备份
    max_age: 90                 # 保留 90 天
    compress: true              # 压缩旧日志
```

**ELK 采集配置** (Filebeat):

```yaml
# filebeat.yml
filebeat.inputs:
- type: log
  enabled: true
  paths:
    - /var/log/wsc/*.log
  json.keys_under_root: true
  json.add_error_key: true
  fields:
    service: wsc-gateway
    environment: production

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
  index: "wsc-logs-%{+yyyy.MM.dd}"
```

### 告警规则

**Prometheus AlertManager 配置**:

```yaml
# alerts.yml
groups:
- name: wsc-alerts
  interval: 30s
  rules:
  
  # 连接数过高
  - alert: WSCHighConnections
    expr: wsc_connections_total > 80000
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "WSC 连接数过高"
      description: "当前连接数 {{ $value }},超过 80% 容量"
  
  # 消息延迟过高
  - alert: WSCHighLatency
    expr: histogram_quantile(0.99, wsc_message_duration_seconds_bucket) > 1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "WSC 消息延迟过高"
      description: "P99 延迟 {{ $value }} 秒"
  
  # 错误率过高
  - alert: WSCHighErrorRate
    expr: rate(wsc_errors_total[5m]) > 10
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "WSC 错误率过高"
      description: "每秒错误数 {{ $value }}"
  
  # Redis 连接失败
  - alert: WSCRedisConnectionFailed
    expr: wsc_redis_connection_errors_total > 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "WSC Redis 连接失败"
      description: "无法连接到 Redis,分布式功能异常"
```

---

## 故障处理

### 连接异常

**症状**: 用户频繁断开连接

**排查步骤**:

1. 检查心跳配置:
```yaml
wsc:
  heartbeat:
    interval: 30s
    timeout: 90s            # 确保超时时间足够
```

2. 检查负载均衡器超时:
```nginx
# Nginx 配置
location /ws {
    proxy_read_timeout 300s;    # 增加超时时间
    proxy_send_timeout 300s;
}
```

3. 检查网络质量:
```bash
# 查看连接统计
curl http://localhost:9090/metrics | grep wsc_connections
```

**解决方案**:
- 增加心跳间隔和超时时间
- 配置负载均衡器 WebSocket 支持
- 启用自动重连机制

### 消息丢失

**症状**: 消息未送达或延迟严重

**排查步骤**:

1. 检查消息队列状态:
```bash
curl http://localhost:9090/metrics | grep wsc_message_queue
```

2. 检查 Redis 连接:
```bash
redis-cli ping
redis-cli info clients
```

3. 查看日志:
```bash
grep "send message failed" /var/log/wsc/wsc.log
```

**解决方案**:
- 增大消息队列大小
- 增加 Worker 数量
- 启用消息持久化

### 性能瓶颈

**症状**: CPU/内存占用过高,响应缓慢

**排查步骤**:

1. CPU Profile:
```bash
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

2. 内存 Profile:
```bash
curl http://localhost:8080/debug/pprof/heap > mem.prof
go tool pprof mem.prof
```

3. Goroutine 泄漏:
```bash
curl http://localhost:8080/debug/pprof/goroutine > goroutine.prof
```

**解决方案**:
- 优化消息序列化 (使用 protobuf 替代 JSON)
- 使用对象池减少 GC 压力
- 限制并发 goroutine 数量

### 内存泄漏

**症状**: 内存持续增长不释放

**排查步骤**:

1. 检查连接清理:
```go
// 确保 OnDisconnect 回调清理资源
OnDisconnect: func(userID string) {
    // 清理缓存
    cache.DeleteUserSession(userID)
    // 清理消息队列
    messageQueue.Remove(userID)
}
```

2. 检查 goroutine 泄漏:
```bash
curl http://localhost:8080/debug/pprof/goroutine?debug=2
```

**解决方案**:
- 使用 context 控制 goroutine 生命周期
- 定期触发 GC
- 使用内存限制配置

---

## 最佳实践

### ✅ 推荐做法

1. **启用分布式模式**:生产环境必须使用 Redis 分布式,避免单点故障
2. **配置心跳检测**:避免僵尸连接占用资源
3. **启用消息压缩**:降低带宽消耗,建议 level=1 (快速压缩)
4. **设置限流保护**:防止恶意请求或消息轰炸
5. **使用回调验证**:在 OnAuthenticate 中严格校验用户身份
6. **监控关键指标**:Prometheus + Grafana 实时监控
7. **日志结构化**:使用 JSON 格式,便于 ELK 采集分析
8. **消息持久化**:重要消息存储到数据库,防止丢失
9. **优雅关闭**:处理 SIGTERM 信号,等待消息发送完成
10. **定期压测**:使用 k6/wrk 模拟高并发场景

### ❌ 避免做法

1. **不要在生产环境关闭认证**:OnAuthenticate 必须实现
2. **不要使用默认配置**:根据业务调整参数
3. **不要在回调中阻塞**:OnMessage 中不要执行耗时操作
4. **不要忽略错误**:SendMessage 失败要记录日志
5. **不要硬编码配置**:使用配置文件或环境变量
6. **不要忽略限流**:必须配置 rate_limit 防止滥用
7. **不要在内存中存储大量数据**:使用 Redis/数据库
8. **不要直接暴露内置 API**:必须加认证和鉴权
9. **不要频繁重连**:客户端需要退避重试策略
10. **不要忽略日志轮转**:避免磁盘被占满

---

## 常见问题

### Q1: 如何实现消息顺序性?

A: 对于同一用户的消息,WSC 保证顺序发送。如需全局顺序,建议:
- 使用消息 ID 和时间戳标记
- 客户端根据时间戳排序
- 使用 Redis Streams 保证顺序

### Q2: 如何处理大量离线消息?

A: 建议策略:
- 限制离线消息数量 (如最多 100 条)
- 设置过期时间 (如 7 天)
- 分页推送,避免一次性发送过多

### Q3: 如何实现跨集群消息路由?

A: WSC 通过 Redis Pub/Sub 实现:
- 节点 A 收到发送请求
- 发布消息到 Redis Channel
- 节点 B 订阅 Channel,接收并转发给目标用户

### Q4: 如何保证消息不丢失?

A: 多层保障:
1. 发送前检查连接状态
2. 发送失败存储到数据库
3. 用户上线后推送离线消息
4. 客户端确认机制 (ACK)

### Q5: 如何限制单个用户的连接数?

A: 在 OnAuthenticate 回调中检查:
```go
OnAuthenticate: func(r *http.Request) (string, error) {
    userID := extractUserID(r)
    
    // 检查现有连接数
    connections := gateway.GetUserConnections(userID)
    if len(connections) >= 3 {
        return "", errors.New("too many connections")
    }
    
    return userID, nil
}
```

---

**更多帮助**: 
- 查看 [README.md](./README.md) 了解配置详情
- 提交 Issue: https://github.com/kamalyes/go-rpc-gateway/issues
