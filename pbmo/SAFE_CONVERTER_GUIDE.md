# SafeConverter - 安全转换器最佳实践指南

## 概述

SafeConverter 是基于 `go-toolbox/safe` 模块的 SafeAccess 特性构建的安全转换工具，提供：

- ✅ 链式安全访问（类似 JavaScript 的可选链 `?.`）
- ✅ nil 指针自动处理
- ✅ 详细的错误上下文信息
- ✅ 灵活的批量转换结果追踪

## 快速开始

### 1. 基础使用

```go
// 创建安全转换器
converter := pbmo.NewSafeConverter(&pb.User{}, &User{})

// 安全转换（自动处理 nil）
var user User
if err := converter.SafeConvertPBToModel(pbUser, &user); err != nil {
    // err 包含详细的操作信息
    log.Printf("Conversion failed: %v", err)
}
```

### 2. 安全字段访问

```go
// 链式安全字段访问
// 类似 JavaScript: obj?.profile?.address?.city

city := converter.SafeFieldAccess(user, "Profile", "Address", "City").
    String("Unknown")

// 即使中间有 nil 字段也能安全处理
// 如果任何中间字段为 nil，返回默认值 "Unknown"
```

### 3. 批量转换与追踪

```go
// 获取详细的批量转换结果
result := converter.SafeBatchConvertPBToModel(pbUsers, &users)

// 检查整体统计
fmt.Printf("Success: %d/%d (%.2f%%)\n",
    result.SuccessCount,
    result.SuccessCount + result.FailureCount,
    float64(result.SuccessCount) * 100 / float64(len(pbUsers)))

// 逐个检查转换结果
for _, item := range result.Results {
    if item.Success {
        // item.Value 包含转换后的对象
        user := item.Value.(*User)
    } else {
        // item.Error 包含失败原因
        log.Printf("Item %d failed: %v", item.Index, item.Error)
    }
}
```

## 使用场景

### 场景 1: 深层嵌套字段访问

**问题**: 访问多层嵌套字段时，任何中间字段为 nil 都会导致 panic

```go
// ❌ 危险的做法
city := user.Profile.Address.City // 如果 Profile 为 nil 就会 panic

// ✅ 使用 SafeConverter
city := converter.SafeFieldAccess(user, "Profile", "Address", "City").
    String("Unknown") // 安全，返回 "Unknown"
```

### 场景 2: 处理可能为 nil 的批量数据

**问题**: 批量转换时，如果某个项目为 nil，整个操作会失败

```go
// ❌ 基础转换会在第一个 nil 处失败
if err := converter.BatchConvertPBToModel(pbUsers, &users); err != nil {
    // 无法知道哪个项目失败
}

// ✅ SafeConverter 继续处理，收集所有结果
result := converter.SafeBatchConvertPBToModel(pbUsers, &users)

for _, item := range result.Results {
    if !item.Success {
        log.Printf("Item %d: %v", item.Index, item.Error)
    }
}
```

### 场景 3: 配置对象的安全访问

```go
// 深层配置访问
type Config struct {
    Server *struct {
        Database *struct {
            Connection *struct {
                Host string
            }
        }
    }
}

converter := pbmo.NewSafeConverter(&Config{}, &Config{})

// 任何层级为 nil 都能安全处理
host := converter.SafeFieldAccess(config, "Server", "Database", "Connection", "Host").
    String("localhost")
```

## 高级特性

### OrElse - 提供备选值

```go
// 链式调用 OrElse 提供备选对象
address := converter.SafeFieldAccess(user, "Profile", "Address").
    OrElse(&Address{
        City: "Unknown",
        Country: "Unknown",
    }).
    Value().(*Address)
```

### IfPresent - 有条件执行

```go
// 值存在时执行回调
converter.SafeFieldAccess(user, "Email").
    IfPresent(func(v interface{}) {
        email := v.(string)
        // 处理 email
    })
```

### Filter - 条件过滤

```go
// 满足条件才返回有效值
converter.SafeFieldAccess(user, "Age").
    Filter(func(v interface{}) bool {
        return v.(int) >= 18
    }).
    Int(0) // 年龄 >= 18 才返回，否则默认值 0
```

## 错误处理

### 错误类型

SafeConverter 使用 ConversionError 类型提供详细信息：

```go
type ConversionError struct {
    Message    string // 错误消息
    Operation  string // 操作名称
    SourceType string // 源类型
    TargetType string // 目标类型
}
```

### 错误信息示例

```go
err := converter.SafeConvertPBToModel(nil, &user)
// Error: [SafeConvertPBToModel] pb message cannot be nil (from nil to *User)

err := converter.SafeConvertModelToPB(nil, &pbUser)
// Error: [SafeConvertModelToPB] model cannot be nil (from nil to *PBUser)
```

## 性能考虑

### 反射开销

SafeConverter 使用反射进行字段访问，性能特点：

- 单次字段访问：~1-2μs（因反射开销）
- 链式访问（n 层）：~n-2n μs
- 批量转换：通过预分配内存优化

### 优化建议

1. **缓存 SafeFieldAccess 结果**
   ```go
   // 如果频繁访问同一字段，缓存结果
   cityValue := converter.SafeFieldAccess(user, "Profile", "Address", "City")
   for i := 0; i < 1000; i++ {
       city := cityValue.String("Unknown")
   }
   ```

2. **批量转换预分配**
   ```go
   users := make([]User, len(pbUsers)) // 预分配
   result := converter.SafeBatchConvertPBToModel(pbUsers, &users)
   ```

3. **选择合适的转换器**
   - 高性能要求：使用 `BidiConverter`
   - 需要日志和监控：使用 `EnhancedConverter`
   - 处理 nil 指针：使用 `SafeConverter`

## 最佳实践

### ✅ 推荐

1. **对可能为 nil 的字段使用 SafeConverter**
   ```go
   value := converter.SafeFieldAccess(obj, "OptionalField").String("default")
   ```

2. **批量转换时检查结果**
   ```go
   result := converter.SafeBatchConvertPBToModel(items, &output)
   if result.FailureCount > 0 {
       log.Printf("Partial failure: %d/%d", result.FailureCount, result.SuccessCount+result.FailureCount)
   }
   ```

3. **在 gRPC 服务中使用统一的错误处理**
   ```go
   if err := converter.SafeConvertPBToModel(req, &model); err != nil {
       if convErr, ok := err.(*ConversionError); ok {
           return nil, status.Errorf(codes.InvalidArgument, 
               "Invalid %s: %s", convErr.SourceType, convErr.Message)
       }
   }
   ```

4. **为关键操作添加日志**
   ```go
   result := converter.SafeBatchConvertPBToModel(items, &output)
   logger.Info("Batch conversion completed: %d success, %d failed",
       result.SuccessCount, result.FailureCount)
   ```

### ❌ 避免

1. **不要忽视批量转换的失败项**
   ```go
   // ❌ 错误：忽视失败
   converter.SafeBatchConvertPBToModel(items, &output)
   
   // ✅ 正确：检查失败
   result := converter.SafeBatchConvertPBToModel(items, &output)
   if result.FailureCount > 0 { /* 处理 */ }
   ```

2. **不要过度链式调用**
   ```go
   // ❌ 反射链过长，性能下降
   value := converter.SafeFieldAccess(obj, "A", "B", "C", "D", "E", "F", "G").String()
   
   // ✅ 分步获取
   a := converter.SafeFieldAccess(obj, "A").Value()
   value := converter.SafeFieldAccess(a, "B", "C", "D").String()
   ```

3. **不要混淆转换器的职责**
   ```go
   // ❌ 混淆使用
   enhanced := NewEnhancedBidiConverter(...)
   result := enhanced.SafeBatchConvertPBToModel(...) // SafeConverter 方法！
   
   // ✅ 选择合适的转换器
   safe := NewSafeConverter(...)
   result := safe.SafeBatchConvertPBToModel(...)
   ```

## 与其他转换器的选择

| 场景 | 推荐转换器 | 原因 |
|-----|----------|------|
| 高性能转换 | BidiConverter | 最小开销，直接反射 |
| 监控性能和日志 | EnhancedConverter | 集成日志和指标 |
| 处理 nil 指针 | SafeConverter | 安全的字段访问 |
| 完整的服务集成 | ServiceIntegration | 统一管理转换、校验、错误 |

## 故障排除

### 问题 1: SafeAccess 返回 nil 值

```go
value := converter.SafeFieldAccess(obj, "Field").Value() // nil?

// 检查字段是否有效
if !converter.SafeFieldAccess(obj, "Field").IsValid() {
    log.Println("Field is nil or invalid")
}
```

### 问题 2: 批量转换的某项失败不知道原因

```go
result := converter.SafeBatchConvertPBToModel(items, &output)

// 检查失败项的详细错误
for _, item := range result.Results {
    if !item.Success {
        log.Printf("[Item %d] %v", item.Index, item.Error)
    }
}
```

### 问题 3: 性能下降

```go
// 如果使用了很多字段链式访问，考虑优化

// ❌ 低效：每次都重新反射链
for range items {
    value := converter.SafeFieldAccess(obj, "A", "B", "C").String()
}

// ✅ 高效：缓存字段访问
cached := converter.SafeFieldAccess(obj, "A", "B", "C")
for range items {
    value := cached.String()
}
```

## 总结

SafeConverter 是处理 nil 指针和深层对象访问的最佳选择：

- ✅ 自动处理 nil，避免 panic
- ✅ 链式访问提高代码可读性
- ✅ 详细的错误信息便于调试
- ✅ 灵活的批量转换结果追踪

根据你的具体场景选择合适的转换器组合，充分利用 pbmo 的强大功能。
