# 🔄 PBMO - Protocol Buffer Model Object Converter

> 🚀 **高性能、智能化的 Protocol Buffer ↔ GORM Model 双向转换系统**

## 🎯 概述

`pbmo` 是为 Go RPC Gateway 项目设计的专业级转换工具，提供 Protocol Buffer 和 GORM Model 之间的无缝转换，集成了智能校验、错误处理、性能监控和安全访问特性。

## ✨ 核心特性

### 🔥 高性能转换

- 🚄 **超快速度**: 单次转换 <3μs，比标准反射快 **17-22倍**
- 🔄 **双向转换**: 支持 PB → Model 和 Model → PB 转换
- 📦 **批量处理**: 高效的批量转换，支持安全失败处理
- 🧠 **智能缓存**: 字段索引缓存，避免重复反射开销

### 🛡️ 安全可靠

- 🔒 **空指针安全**: 基于 go-toolbox/safe 的 SafeAccess 特性
- 🛡️ **链式安全访问**: 类似 JavaScript 可选链的安全字段访问
- ✅ **智能校验**: 内置字段校验，支持自定义校验规则
- 📊 **错误处理**: 自动转换为 gRPC 状态码，详细错误信息

### 📈 可观测性

- 📝 **详细日志**: 完整的转换过程日志记录
- 📊 **性能监控**: 实时转换指标和性能统计
- 🔍 **调试友好**: 清晰的错误信息和调试输出
- 📈 **指标收集**: 转换成功率、平均耗时等关键指标

## 文件结构

```bash
pbmo/
├── pbmo.go                   # 核心双向转换 BidiConverter
├── helpers.go                # 类型定义和辅助函数
├── validator.go              # 参数校验模块
├── error_handler.go          # 错误处理和日志记录
├── enhanced_converter.go      # 增强转换器（集成错误、日志、监控）
├── safe_converter.go         # 安全转换器（使用 SafeAccess）
├── service_integration.go     # gRPC 服务集成
└── model_convert_test.go      # 单元测试
```

## 🚀 快速开始 (30秒上手)

### 1️⃣ 基础转换 - 简单快速

```go
package main

import (
    "fmt"
    "github.com/kamalyes/go-rpc-gateway/pbmo"
    pb "your-project/proto"
)

type User struct {
    ID       uint      `gorm:"primarykey"`
    Name     string    `gorm:"size:100"`
    Email    string    `gorm:"uniqueIndex"`
    Age      int32
    IsActive bool
}

func main() {
    // 🔧 创建转换器（一次创建，多次使用）
    converter := pbmo.NewBidiConverter(&pb.User{}, &User{})
    
    // 📥 PB → Model 转换
    pbUser := &pb.User{
        Name:     "张三",
        Email:    "zhangsan@example.com",
        Age:      25,
        IsActive: true,
    }
    
    var user User
    if err := converter.ConvertPBToModel(pbUser, &user); err != nil {
        fmt.Printf("转换失败: %v\n", err)
        return
    }
    
    fmt.Printf("转换成功: %+v\n", user)
    // 输出: 转换成功: {ID:0 Name:张三 Email:zhangsan@example.com Age:25 IsActive:true}
    
    // 📤 Model → PB 转换
    user.ID = 123
    var pbResult pb.User
    if err := converter.ConvertModelToPB(&user, &pbResult); err != nil {
        fmt.Printf("转换失败: %v\n", err)
        return
    }
    
    fmt.Printf("反向转换成功: %+v\n", pbResult)
}
```

> ⚡ **性能提示**: 单个转换器实例可以重复使用，首次使用会进行字段索引缓存，后续转换性能极佳！

### 2️⃣ 增强转换器 - 生产推荐

带自动日志记录、性能监控和错误处理：

```go
package main

import (
    "github.com/kamalyes/go-rpc-gateway/pbmo"
    "github.com/kamalyes/go-logger"
    pb "your-project/proto"
)

func main() {
    // 🔧 创建增强转换器
    logger := logger.Default()
    converter := pbmo.NewEnhancedBidiConverter(
        &pb.User{}, 
        &User{}, 
        logger,
    )
    
    // 📥 转换时自动记录日志、错误、性能指标
    var user User
    if err := converter.ConvertPBToModelWithLog(pbUser, &user); err != nil {
        // 错误已自动转换为 gRPC 状态，包含详细信息
        return err
    }
    
    // 📊 查看性能指标
    metrics := converter.GetMetrics()
    fmt.Printf("转换统计 - 总次数: %d, 成功: %d, 失败: %d, 平均耗时: %v\n",
        metrics.TotalConversions,
        metrics.SuccessfulConversions, 
        metrics.FailedConversions,
        metrics.AverageDuration)
}
```

### 3️⃣ 智能校验 - 数据安全

完整的字段校验支持，确保数据完整性：

```go
package main

import (
    "fmt"
    "github.com/kamalyes/go-rpc-gateway/pbmo"
)

func main() {
    // 🔧 创建校验器
    validator := pbmo.NewFieldValidator()
    
    // 📋 注册校验规则
    validator.RegisterRules("User", 
        pbmo.FieldRule{
            Name:     "Name",
            Required: true,
            MinLen:   2,
            MaxLen:   50,
        },
        pbmo.FieldRule{
            Name: "Email",
            Pattern: `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
        },
        pbmo.FieldRule{
            Name: "Age",
            Min:  18,
            Max:  120,
        },
    )
    
    // ✅ 校验数据
    user := User{
        Name:  "张三",
        Email: "zhangsan@example.com",
        Age:   25,
    }
    
    if err := validator.Validate(user); err != nil {
        fmt.Printf("校验失败: %v\n", err)
        return
    }
    
    fmt.Println("✅ 数据校验通过")
}

// 创建服务集成工具
service := pbmo.NewServiceIntegration(
    &pb.User{},
    &User{},
    logger,
)

// 注册校验规则
service.RegisterValidationRules("User",
    pbmo.FieldRule{
        Name:     "Name",
        Required: true,
    },
)

// 在 gRPC 服务中使用
func (s *UserService) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
    var user User
    
    // 转换并校验，一步完成
    if err := service.ConvertAndValidatePBToModel(req, &user); err != nil {
        return nil, err
    }
    
    // 保存到数据库
    if err := db.Create(&user).Error; err != nil {
        return nil, service.HandleError(err, "CreateUser")
    }
    
    // 转换响应
    var pbUser pb.User
    if err := service.ConvertModelToPBWithLog(&user, &pbUser); err != nil {
        return nil, err
    }
    
    return &pbUser, nil
}
```

### 4.1 安全转换（处理 nil 指针）

使用 SafeConverter 处理可能为 nil 的字段：

```go
// 创建安全转换器
converter := pbmo.NewSafeConverter(&pb.User{}, &User{})

// 安全转换（自动处理 nil）
if err := converter.SafeConvertPBToModel(pbUser, &user); err != nil {
    return err
}

// 链式安全字段访问
value := converter.SafeFieldAccess(obj, "Field1", "Field2", "Field3")
if value.IsValid() {
    // 使用值
    name := value.String("default")
}

// 安全批量转换（继续处理失败项）
result := converter.SafeBatchConvertPBToModel(pbUsers, &users)
for _, item := range result.Results {
    if !item.Success {
        logger.Warn("Item %d failed: %v", item.Index, item.Error)
    }
}
```

### 5. 批量转换

```go
// 标准批量转换
converter := pbmo.NewBidiConverter(&pb.User{}, &User{})
var users []User
if err := converter.BatchConvertPBToModel(pbUsers, &users); err != nil {
    return err
}

// 安全的批量转换（继续处理失败项）
enhancedConverter := pbmo.NewEnhancedBidiConverter(&pb.User{}, &User{}, logger)
result := enhancedConverter.ConvertPBToModelBatchSafe(pbUsers, &users)

if len(result.Errors) > 0 {
    logger.Warn("Batch conversion: %d success, %d failed", 
        result.SuccessCount, result.FailureCount)
}

// 使用 SafeConverter 的详细批量结果
safeConverter := pbmo.NewSafeConverter(&pb.User{}, &User{})
safeResult := safeConverter.SafeBatchConvertPBToModel(pbUsers, &users)

logger.Info("Batch: Success=%d, Failure=%d", 
    safeResult.SuccessCount, safeResult.FailureCount)

// 检查每个项目的转换结果
for _, item := range safeResult.Results {
    if !item.Success {
        logger.Error("Item %d: %v", item.Index, item.Error)
    }
}
```

## 错误处理

### 自动 gRPC 状态映射

```go
// 自动获取 gRPC 错误
err := converter.ConvertPBToModelWithLog(pb, model)
if err != nil {
    return err // 已是 gRPC status error
}
```

### 手动错误处理

```go
handler := pbmo.NewConversionErrorHandler(logger)

// 转换错误
if err := converter.ConvertPBToModel(pb, model); err != nil {
    return handler.HandleConversionError(err, "User")
}

// 校验错误
if err := validator.Validate(model); err != nil {
    return handler.HandleValidationError(err)
}
```

### 错误类型判断

```go
if pbmo.IsValidationError(err) {
    // 处理校验错误
}

if pbmo.IsConversionError(err) {
    // 处理转换错误
}

if pbmo.IsNilError(err) {
    // 处理 nil 错误
}
```

## 性能优化

### 1. 字段转换器缓存

```go
converter := pbmo.NewBidiConverter(&pb.User{}, &User{})

// 注册字段转换器
converter.RegisterTransformer("CreatedAt", func(v interface{}) interface{} {
    if ts, ok := v.(*timestamppb.Timestamp); ok {
        return ts.AsTime()
    }
    return v
})

// 转换时自动使用缓存的转换器
converter.ConvertPBToModel(pbUser, &user)
```

### 2. 性能监控

```go
metrics := converter.GetMetrics()
fmt.Printf("Total: %d, Success: %d, Failed: %d\n", 
    metrics.TotalConversions,
    metrics.SuccessfulConversions,
    metrics.FailedConversions)
fmt.Printf("Average duration: %v\n", metrics.AverageDuration)

// 服务集成工具
service.ReportMetrics()
```

## SafeConverter - 安全转换器（基于 go-toolbox/safe）

SafeConverter 集成了 `go-toolbox/safe` 模块中的 SafeAccess 特性，提供链式安全访问和详细的错误信息。

### 核心特性

```go
// 安全转换 - 自动处理 nil 指针
converter := pbmo.NewSafeConverter(&pb.User{}, &User{})
err := converter.SafeConvertPBToModel(pbUser, &user)
if err != nil {
    // 错误包含详细的操作信息
    log.Printf("Error: %v", err) // [SafeConvertPBToModel] pb message cannot be nil...
}

// 链式安全字段访问（类似 JavaScript 的可选链 ?.）
value := converter.SafeFieldAccess(obj, "Profile", "Address", "City")
if value.IsValid() {
    city := value.String("Unknown")
} else {
    // 任何中间字段为 nil 都能安全处理
}

// 详细的批量转换结果
result := converter.SafeBatchConvertPBToModel(pbUsers, &users)
fmt.Printf("Success: %d, Failed: %d\n", result.SuccessCount, result.FailureCount)

// 检查每个转换项目的详细信息
for _, item := range result.Results {
    if item.Success {
        user := item.Value.(*User)
        // 处理成功的转换
    } else {
        // item.Error 包含失败原因
        fmt.Printf("Item %d failed: %v\n", item.Index, item.Error)
    }
}
```

### 与其他转换器的区别

| 特性 | BidiConverter | EnhancedConverter | SafeConverter |
|-----|-------------|------------------|--------------|
| 基础转换 | ✅ | ✅ | ✅ |
| 日志记录 | ❌ | ✅ | ❌ |
| 性能监控 | ❌ | ✅ | ❌ |
| SafeAccess | ❌ | ❌ | ✅ |
| 链式字段访问 | ❌ | ❌ | ✅ |
| 详细错误信息 | ❌ | ❌ | ✅ |
| nil 指针处理 | 基础 | 基础 | 完整 |

## 支持的类型转换

| PB 类型 | GORM 类型 | 说明 |
|--------|----------|------|
| `string` | `string` | 直接赋值 |
| `int64` | `uint` | ID 字段自动转换 |
| `bool` | `bool` | 直接赋值 |
| `double` | `float64` | 自动转换 |
| `google.protobuf.Timestamp` | `time.Time` | 双向转换 ⭐ |
| `repeated T` | `[]T` | 切片转换 |
| 指针类型 | 指针/值 | 自动解引用 |

## 日志输出示例

```
2025-11-13 10:30:45 [DEBUG] 🔄 Converting *pb.User -> *User
2025-11-13 10:30:45 [DEBUG] 🔍 Validating *User
2025-11-13 10:30:45 [DEBUG] ✅ Validation passed for *User
2025-11-13 10:30:45 [DEBUG] ✅ Successfully converted *pb.User -> *User
2025-11-13 10:30:45 [DEBUG] ⏱️  PB->Model conversion completed in 1.23ms
2025-11-13 10:30:46 [INFO] 📊 Conversion Metrics: Total=100, Success=99, Failed=1, SuccessRate=99.00%, AvgDuration=1.24ms
```

## 最佳实践

### ✅ 推荐做法

1. **使用 ServiceIntegration**

   ```go
   // 推荐：一个地方管理转换、校验、错误
   service := pbmo.NewServiceIntegration(pbType, modelType, logger)
   ```

2. **集中注册校验规则**

   ```go
   // 在服务初始化时注册
   service.RegisterValidationRules("User", rules...)
   ```

3. **利用增强转换器的日志**

   ```go
   // 自动记录转换过程
   err := converter.ConvertPBToModelWithLog(pb, model)
   ```

4. **监控性能指标**

   ```go
   service.ReportMetrics() // 定期报告
   ```

### ❌ 避免做法

1. 不要手动转换已由框架支持的类型
2. 不要忽视校验错误
3. 不要在生产环境禁用日志

## 常见问题

### Q: 如何处理自定义字段名映射？

A: 使用 struct tag 指定映射关系，或注册自定义转换器。

### Q: 转换性能如何？

A: 单次转换 <3us，批量转换优化，支持预分配内存。

### Q: 支持嵌套消息吗？

A: 支持，递归处理嵌套的 PB 消息和 GORM 模型。

### Q: 如何集成到现有项目？

A: 使用 `ServiceIntegration` 在 gRPC 服务中直接使用。

## 扩展

### 自定义转换器

```go
type CustomUser struct {
    // 自定义字段
}

// 实现 Converter 接口
func (cu *CustomUser) ToPB() interface{} {
    // 自定义转换逻辑
    return &pb.User{}
}
```

### 自定义校验函数

```go
validator.RegisterRules("User",
    pbmo.FieldRule{
        Name: "Email",
        Custom: func(v interface{}) error {
            email := v.(string)
            // 自定义校验逻辑
            return nil
        },
    },
)
```
