# PBMO 字段映射指南

## 概述

本文档说明如何正确配置 Go 模型以支持 `pbmo.BidiConverter` 在 Protobuf 和 Go Model 之间进行自动转换。

## 核心原则

`pbmo.BidiConverter` 通过 **字段名称匹配** 来进行转换，而不是通过 JSON 标签。因此：

- ✅ **Go 模型字段名必须与 Protobuf 字段名完全一致**
- ❌ **JSON 标签不影响 pbmo 转换**
- ✅ **使用 GORM column 标签来映射数据库列名**

## 正确的映射示例

### Protobuf 定义

```protobuf
message QuickReplyGroup {
  int64 group_id = 1;                            // 分组ID
  int64 parent_id = 2;                           // 父分组ID
  string name = 3;                               // 分组名称
  string owner_id = 7;                           // 所有者ID
  google.protobuf.Timestamp created_at = 10;     // 创建时间
}

message QuickReply {
  int64 reply_id = 1;                            // 回复ID
  int64 group_id = 2;                            // 分组ID
  string title = 3;                              // 回复标题
  string owner_id = 7;                           // 所有者ID
}
```

### Go Model 定义（正确）

```go
// QuickReplyGroupModel 快捷回复分组 - 字段名称与proto完全匹配以支持PBMO自动转换
type QuickReplyGroupModel struct {
    // Go 字段名: GroupId (匹配 protobuf)
    // 数据库列名: id (通过 gorm:"column:id")
    GroupId     int64  `json:"group_id" gorm:"column:id;primaryKey;autoIncrement"`
    
    // 其他字段同理
    ParentId    int64  `json:"parent_id" gorm:"type:bigint;index"`
    Name        string `json:"name" gorm:"type:varchar(200);not null"`
    
    // 注意：OwnerId 不是 OwnerID
    OwnerId     string `json:"owner_id" gorm:"column:owner_id;type:varchar(64);index"`
    
    CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// QuickReplyModel 快捷回复实体 - 字段名称与proto完全匹配以支持PBMO自动转换
type QuickReplyModel struct {
    ReplyId   int64  `json:"reply_id" gorm:"column:id;primaryKey;autoIncrement"`
    GroupId   int64  `json:"group_id" gorm:"type:bigint;not null;index"`
    Title     string `json:"title" gorm:"type:varchar(500);not null"`
    OwnerId   string `json:"owner_id" gorm:"column:owner_id;type:varchar(64);index"`
}
```

### 错误示例（❌ 不要这样做）

```go
// ❌ 错误：字段名不匹配 protobuf
type QuickReplyGroupModel struct {
    Id       int64  `json:"group_id" gorm:"primaryKey"`  // ❌ Id != GroupId
    OwnerID  string `json:"owner_id"`                     // ❌ OwnerID != OwnerId
}

// ❌ 错误：依赖 JSON 标签
// pbmo 不使用 JSON 标签进行匹配！
```

## 字段命名规则

### Protobuf → Go 字段名转换

| Protobuf 字段 | Protobuf Go 生成 | Go Model 字段名 | 说明 |
|--------------|-----------------|----------------|------|
| `group_id` | `GroupId` | `GroupId` | ✅ 完全一致 |
| `reply_id` | `ReplyId` | `ReplyId` | ✅ 完全一致 |
| `owner_id` | `OwnerId` | `OwnerId` | ✅ 注意是 Id 不是 ID |
| `parent_id` | `ParentId` | `ParentId` | ✅ 完全一致 |
| `created_at` | `CreatedAt` | `CreatedAt` | ✅ 完全一致 |

### 数据库列名映射

当 Go 字段名与数据库列名不同时，使用 `gorm:"column:列名"`:

```go
// Go 字段名: GroupId (匹配 protobuf)
// 数据库列名: id (实际数据库表中的列)
GroupId int64 `gorm:"column:id;primaryKey"`

// Go 字段名: ReplyId (匹配 protobuf)
// 数据库列名: id (实际数据库表中的列)
ReplyId int64 `gorm:"column:id;primaryKey"`

// Go 字段名: OwnerId (匹配 protobuf)
// 数据库列名: owner_id (保持一致)
OwnerId string `gorm:"column:owner_id;type:varchar(64)"`
```

## Repository 层注意事项

Repository 层的过滤器字段名应该使用 **数据库列名**，而不是 Go 字段名：

```go
// ✅ 正确：使用数据库列名
filter := repository.NewEqFilter("id", replyID)           // 数据库列是 id
filter := repository.NewEqFilter("group_id", groupID)     // 数据库列是 group_id
filter := repository.NewEqFilter("owner_id", ownerID)     // 数据库列是 owner_id

// ❌ 错误：使用 Go 字段名
filter := repository.NewEqFilter("ReplyId", replyID)      // 数据库没有 ReplyId 列
filter := repository.NewEqFilter("GroupId", groupID)      // 数据库没有 GroupId 列
```

### 批量操作示例

```go
// 批量更新状态
func BatchUpdateStatus(ctx context.Context, replyIDs []int64, status Status) error {
    filter := &repository.Filter{
        Field:    "id",        // ✅ 数据库列名
        Operator: constant.OP_IN,
        Value:    replyIDs,
    }
    updates := map[string]interface{}{
        "status": status,      // ✅ 数据库列名
    }
    return r.UpdateFieldsByFilters(ctx, updates, filter)
}

// 移动到分组
func MoveToGroup(ctx context.Context, replyIDs []int64, targetGroupID int64) error {
    filter := &repository.Filter{
        Field:    "id",        // ✅ 按 id 过滤
        Operator: constant.OP_IN,
        Value:    replyIDs,
    }
    updates := map[string]interface{}{
        "group_id": targetGroupID,  // ✅ 更新 group_id 列
    }
    return r.UpdateFieldsByFilters(ctx, updates, filter)
}
```

## PBMO 转换验证

### 初始化转换器

```go
// 创建双向转换器
groupConverter := pbmo.NewBidiConverter(
    &quickreplyApis.QuickReplyGroup{},  // Protobuf 消息
    &models.QuickReplyGroupModel{},     // Go 模型
)

replyConverter := pbmo.NewBidiConverter(
    &quickreplyApis.QuickReply{},
    &models.QuickReplyModel{},
)
```

### Model → Protobuf 转换

```go
// 从数据库查询到的 model
model := &models.QuickReplyGroupModel{
    GroupId:   123,
    ParentId:  456,
    Name:      "常用回复",
    OwnerId:   "agent_001",
}

// 转换为 protobuf
pbGroup := &quickreplyApis.QuickReplyGroup{}
err := groupConverter.ConvertModelToPB(model, pbGroup)

// 结果：
// pbGroup.GroupId = 123      ✅
// pbGroup.ParentId = 456     ✅
// pbGroup.Name = "常用回复"   ✅
// pbGroup.OwnerId = "agent_001" ✅
```

### Protobuf → Model 转换

```go
// 从请求接收的 protobuf
pbReq := &quickreplyApis.CreateQuickReplyRequest{
    GroupId: 123,
    Title:   "欢迎语",
    OwnerId: "agent_001",
}

// 转换为 model
model := &models.QuickReplyModel{}
// 注意：通常手动创建 model，因为请求对象可能不完全匹配

model.GroupId = pbReq.GroupId
model.Title = pbReq.Title
model.OwnerId = pbReq.OwnerId
```

## 常见问题

### Q1: 为什么 JSON 响应中字段值都是零值？

**A:** 字段名不匹配。检查 Go 模型字段名是否与 protobuf 字段名完全一致。

```go
// ❌ 错误
type Model struct {
    Id int64 `json:"group_id"`  // Go字段是 Id，protobuf是 GroupId
}

// ✅ 正确
type Model struct {
    GroupId int64 `json:"group_id"`  // 完全匹配
}
```

### Q2: OwnerID 还是 OwnerId？

**A:** 使用 `OwnerId`（Id 不是 ID）。Protobuf 生成的 Go 代码使用 `OwnerId`。

```go
// Protobuf 生成的代码
type QuickReplyGroup struct {
    OwnerId string  // ✅ 是 OwnerId，不是 OwnerID
}
```

### Q3: 数据库列名和 Go 字段名不一致怎么办？

**A:** 使用 `gorm:"column:实际列名"` 标签。

```go
// Go 字段名匹配 protobuf，数据库列名使用 column 指定
GroupId int64 `gorm:"column:id"`  // Go: GroupId, DB: id
```

### Q4: Repository 过滤器应该用哪个名字？

**A:** 使用数据库列名，不是 Go 字段名。

```go
// ✅ 正确
filter := repository.NewEqFilter("id", value)
filter := repository.NewEqFilter("group_id", value)

// ❌ 错误
filter := repository.NewEqFilter("GroupId", value)
filter := repository.NewEqFilter("ReplyId", value)
```

## 检查清单

使用此清单验证您的模型配置：

- [ ] Go 模型字段名与 protobuf 字段名完全一致
- [ ] 使用正确的大小写（`OwnerId` 不是 `OwnerID`）
- [ ] 主键字段使用 `gorm:"column:id"` 指定数据库列名
- [ ] Repository 过滤器使用数据库列名
- [ ] 批量更新操作使用正确的数据库列名
- [ ] 模型文件添加了注释说明 "字段名称与proto完全匹配以支持PBMO自动转换"
---

**最后更新**: 2025-11-19
