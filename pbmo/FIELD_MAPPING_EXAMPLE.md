# PBMO 字段映射功能使用指南

## 功能概述

当 Model 结构体和 Proto 结构体的字段名不一致时，可以使用字段映射功能自动完成转换，无需手动赋值。

## 使用场景

适用于第三方库结构体与自定义 Proto 定义字段名不匹配的情况，例如：

```go
// 第三方库结构体
type Client struct {
    ID          string    // 希望映射到 proto 的 ClientId
    UserID      string    // 希望映射到 proto 的 UserId
    NodeID      string    // 希望映射到 proto 的 NodeId
    ClientIP    string    // 希望映射到 proto 的 ClientIp
    ConnectedAt time.Time // 希望映射到 proto 的 ConnectTime
}

// Proto 定义
message WebSocketConnectionDetail {
    string client_id = 1;      // 对应 ClientId
    string user_id = 2;        // 对应 UserId
    string node_id = 3;        // 对应 NodeId
    string client_ip = 4;      // 对应 ClientIp
    string connect_time = 5;   // 对应 ConnectTime (需要手动转换时间格式)
}
```

## 使用方法

### 1. Struct Tag 方式（最推荐）✨

在 Model 结构体上使用 `pbmo` tag 声明映射关系：

```go
type Client struct {
    ID          string    `json:"id" pbmo:"ClientId"`           // 映射到 proto.ClientId
    UserID      string    `json:"user_id" pbmo:"UserId"`        // 映射到 proto.UserId
    NodeID      string    `json:"node_id" pbmo:"NodeId"`        // 映射到 proto.NodeId
    ClientIP    string    `json:"client_ip" pbmo:"ClientIp"`    // 映射到 proto.ClientIp
    ConnectedAt time.Time `json:"connected_at" pbmo:"ConnectTime"` // 映射到 proto.ConnectTime
}

// 创建转换器（自动读取 pbmo tag）
converter := pbmo.NewBidiConverter(
    &pb.WebSocketConnectionDetail{},
    &wsc.Client{},
).WithAutoTimeConversion(false)
```

✅ **优点**：

- 字段映射信息直接在结构体定义中
- 更直观易读
- 无需手动注册映射关系
- 自动生成文档

⚠️ **注意**：如果你使用的是第三方库的结构体（如 `wsc.Client`），无法修改其定义，则需要使用下面的方式。

### 2. 链式调用方式（适用于第三方库）

```go
converter := pbmo.NewBidiConverter(
    &pb.WebSocketConnectionDetail{},
    &wsc.Client{},
).WithAutoTimeConversion(false).  // 禁用自动时间转换
  WithFieldMapping("ID", "ClientId").          // Model字段名 -> Proto字段名
  WithFieldMapping("UserID", "UserId").
  WithFieldMapping("NodeID", "NodeId").
  WithFieldMapping("ClientIP", "ClientIp").
  WithFieldMapping("ConnectedAt", "ConnectTime")
```

### 3. 批量注册方式

```go
converter := pbmo.NewBidiConverter(
    &pb.WebSocketConnectionDetail{},
    &wsc.Client{},
)

// 批量注册字段映射
converter.RegisterFieldMapping(map[string]string{
    "ID":          "ClientId",
    "UserID":      "UserId",
    "NodeID":      "NodeId",
    "ClientIP":    "ClientIp",
    "ConnectedAt": "ConnectTime",
})
```

## 转换示例

### Model to Proto

```go
// 源数据 (wsc.Client)
client := &wsc.Client{
    ID:          "conn_123",
    UserID:      "user_456",
    NodeID:      "node_01",
    ClientIP:    "192.168.1.1",
    ConnectedAt: time.Now(),
}

// 目标对象
detail := &pb.WebSocketConnectionDetail{}

// 自动转换（使用字段映射）
if err := converter.ConvertModelToPB(client, detail); err != nil {
    return err
}

// 结果:
// detail.ClientId = "conn_123"
// detail.UserId = "user_456"
// detail.NodeId = "node_01"
// detail.ClientIp = "192.168.1.1"
// detail.ConnectTime = (需要手动处理时间格式)
```

### Proto to Model

字段映射支持双向转换，反向转换会自动应用：

```go
// 源数据
detail := &pb.WebSocketConnectionDetail{
    ClientId:    "conn_123",
    UserId:      "user_456",
    NodeId:      "node_01",
    ClientIp:    "192.168.1.1",
    ConnectTime: "2026-01-24T10:00:00Z",
}

// 目标对象
client := &wsc.Client{}

// 自动转换（反向映射自动生效）
if err := converter.ConvertPBToModel(detail, client); err != nil {
    return err
}

// 结果:
// client.ID = "conn_123"
// client.UserID = "user_456"
// client.NodeID = "node_01"
// client.ClientIP = "192.168.1.1"
```

## 完整示例

```go
package service

import (
    "time"
    "github.com/kamalyes/go-rpc-gateway/pbmo"
    "github.com/kamalyes/go-wsc"
    pb "your-project/pb/dashboard"
)

type DashboardService struct {
    wsConverter *pbmo.BidiConverter
}

func NewDashboardService() *DashboardService {
    // 创建转换器并配置字段映射
    wsConverter := pbmo.NewBidiConverter(
        &pb.WebSocketConnectionDetail{},
        &wsc.Client{},
    ).WithAutoTimeConversion(false).
      WithFieldMapping("ID", "ClientId").
      WithFieldMapping("UserID", "UserId").
      WithFieldMapping("NodeID", "NodeId").
      WithFieldMapping("ClientIP", "ClientIp").
      WithFieldMapping("ConnectedAt", "ConnectTime")

    return &DashboardService{
        wsConverter: wsConverter,
    }
}

func (s *DashboardService) GetConnectionDetail(clientID string) (*pb.WebSocketConnectionDetail, error) {
    // 从第三方库获取数据
    client := wsc.GetClientByID(clientID)
    
    // 使用转换器自动转换
    detail := &pb.WebSocketConnectionDetail{}
    if err := s.wsConverter.ConvertModelToPB(client, detail); err != nil {
        return nil, err
    }
    
    // 手动处理特殊字段（如时间格式）
    detail.ConnectTime = client.ConnectedAt.Format(time.RFC3339)
    detail.LastSeen = client.LastSeen.Format(time.RFC3339)
    detail.LastHeartbeat = client.LastHeartbeat.Format(time.RFC3339)
    
    return detail, nil
}
```

## 注意事项

1. **Struct Tag 优先级**：
   - 如果结构体有 `pbmo` tag，会优先使用 tag 定义的映射
   - `WithFieldMapping()` 会覆盖 tag 定义的映射

2. **字段映射参数顺序**：`WithFieldMapping(Model字段名, Proto字段名)`

3. **双向转换**：映射关系自动支持双向转换（Model↔Proto）

4. **时间字段**：时间类型需要手动处理格式转换（time.Time ↔ string）

5. **枚举类型**：枚举类型需要使用专门的 mapper 进行转换

6. **性能**：
   - Struct tag 在首次使用时会被缓存
   - 映射表查找为 O(1)
   - 不会显著影响性能

## 与手动赋值的对比

### 手动赋值方式（旧方法）

```go
detail := &pb.WebSocketConnectionDetail{
    ClientId:      conn.ID,
    UserId:        conn.UserID,
    NodeId:        conn.NodeID,
    ClientIp:      conn.ClientIP,
    ConnectTime:   conn.ConnectedAt.Format(time.RFC3339),
    LastSeen:      conn.LastSeen.Format(time.RFC3339),
    LastHeartbeat: conn.LastHeartbeat.Format(time.RFC3339),
}
```

❌ 缺点：

- 需要逐个手动赋值
- 代码冗长重复
- 易出错

### 使用字段映射（新方法）

```go
detail := &pb.WebSocketConnectionDetail{}
s.wsConverter.ConvertModelToPB(conn, detail)

// 仅手动处理时间格式
detail.ConnectTime = conn.ConnectedAt.Format(time.RFC3339)
detail.LastSeen = conn.LastSeen.Format(time.RFC3339)
detail.LastHeartbeat = conn.LastHeartbeat.Format(time.RFC3339)
```

✅ 优点：

- 自动映射大部分字段
- 代码简洁清晰
- 统一转换逻辑
- 易于维护

## API 参考

### Struct Tag

```go
type YourModel struct {
    FieldName Type `pbmo:"ProtoFieldName"`
}
```

在结构体字段上使用 `pbmo` tag 声明映射关系（自动识别，无需手动注册）

### WithFieldMapping

```go
func (bc *BidiConverter) WithFieldMapping(modelFieldName, pbFieldName string) *BidiConverter
```

注册单个字段映射（链式调用，会覆盖 struct tag）

### RegisterFieldMapping

```go
func (bc *BidiConverter) RegisterFieldMapping(mappings map[string]string)
```

批量注册字段映射（会覆盖 struct tag）

## 映射方式对比

| 方式 | 优先级 | 适用场景 | 优点 | 缺点 |
|------|--------|----------|------|------|
| **Struct Tag** | 低 | 自定义结构体 | 直观、易维护 | 无法用于第三方库 |
| **WithFieldMapping** | 高 | 第三方库结构体 | 灵活、可动态配置 | 需要手动注册 |
| **RegisterFieldMapping** | 高 | 批量映射 | 集中管理 | 配置分离 |

## 最佳实践

1. **自定义结构体**：优先使用 `pbmo` tag
2. **第三方库**：使用 `WithFieldMapping()` 或 `RegisterFieldMapping()`
3. **混合使用**：可以用 tag 定义基础映射，用 `WithFieldMapping()` 覆盖特殊情况
