# 🚀 PBMO 快速开始指南

> **30秒上手 Protocol Buffer Model Object Converter！**

## 🎯 什么是 PBMO？

PBMO 是为 Go RPC Gateway 设计的高性能双向转换工具，让 Protocol Buffer 和 GORM Model 之间的转换变得极其简单。

**核心优势：**

- 🚄 超快速度：单次转换仅需 3μs
- 🔄 双向转换：PB ↔ Model 完全支持  
- 🛡️ 安全可靠：自动处理 nil 指针和类型转换
- 📊 智能监控：内置性能指标和日志

## 🚀 30秒快速开始

### 第一步：定义数据结构

```go
// protobuf 定义 (user.proto)
message User {
  uint64 id = 1;
  string name = 2;
  string email = 3;
  int32 age = 4;
  bool is_active = 5;
}

// GORM Model
type User struct {
    ID       uint   `gorm:"primarykey"`
    Name     string `gorm:"size:100"`
    Email    string `gorm:"uniqueIndex"`
    Age      int32
    IsActive bool
}
```

### 第二步：创建转换器

```go
package main

import (
    "fmt"
    "github.com/kamalyes/go-rpc-gateway/pbmo"
    pb "your-project/proto"  // 你的 proto 包
)

func main() {
    // 创建转换器（一次创建，重复使用）
    converter := pbmo.NewBidiConverter(&pb.User{}, &User{})
    
    // 🎉 就这么简单！
}
```

### 第三步：开始转换

```go
// PB → Model 转换
pbUser := &pb.User{
    Name:     "张三",
    Email:    "zhangsan@example.com", 
    Age:      25,
    IsActive: true,
}

var user User
err := converter.ConvertPBToModel(pbUser, &user)
if err != nil {
    panic(err)
}

fmt.Printf("转换成功: %+v\n", user)
// 输出: {ID:0 Name:张三 Email:zhangsan@example.com Age:25 IsActive:true}
```

```go
// Model → PB 转换
user.ID = 123
var pbResult pb.User
err = converter.ConvertModelToPB(&user, &pbResult)
if err != nil {
    panic(err)
}

fmt.Printf("反向转换: %+v\n", pbResult)
// 输出: {Id:123 Name:张三 Email:zhangsan@example.com Age:25 IsActive:true}
```

## 🎊 恭喜！你已经掌握了基础用法

## 🔥 进阶功能

### 增强转换器（推荐生产使用）

```go
import "github.com/kamalyes/go-logger"

// 带日志和监控的转换器
logger := logger.Default()
enhancedConverter := pbmo.NewEnhancedBidiConverter(
    &pb.User{}, &User{}, logger,
)

// 自动记录日志和性能指标
err := enhancedConverter.ConvertPBToModelWithLog(pbUser, &user)

// 查看统计信息
metrics := enhancedConverter.GetMetrics()
fmt.Printf("成功率: %.2f%%\n", 
    float64(metrics.SuccessfulConversions) / float64(metrics.TotalConversions) * 100)
```

### 安全转换器（处理复杂嵌套）

```go
// 安全处理 nil 指针和深度嵌套
safeConverter := pbmo.NewSafeConverter(&pb.User{}, &User{})

// 链式安全访问（类似 JavaScript 的 ?. 操作符）
value := safeConverter.SafeFieldAccess(obj, "Profile", "Address", "City")
if value.IsValid() {
    city := value.String("默认城市")
}
```

### 批量转换

```go
var users []User
var pbUsers []*pb.User

// 高效批量转换
err := converter.BatchConvertPBToModel(pbUsers, &users)
if err != nil {
    fmt.Printf("批量转换失败: %v\n", err)
}
```

## 📋 支持的类型转换

| PB 类型 | GORM 类型 | 说明 |
|---------|----------|------|
| `string` | `string` | 直接映射 |
| `int32/int64` | `int/uint` | 自动转换 |
| `bool` | `bool` | 直接映射 |
| `double` | `float64` | 精度保持 |
| `google.protobuf.Timestamp` | `time.Time` | 时间转换 ⭐ |
| `repeated T` | `[]T` | 切片转换 |
| 嵌套消息 | 嵌套结构体 | 递归转换 |

## ⚡ 性能对比

| 转换器类型 | 性能 | 适用场景 |
|----------|------|---------|
| BidiConverter | 130ns/op | 高频转换，性能要求极高 |
| EnhancedConverter | 200ns/op | 生产环境，需要监控和日志 |
| SafeConverter | 150ns/op | 复杂嵌套，安全要求高 |
| 标准反射 | 2260ns/op | 原始方法（不推荐） |

## 🎯 最佳实践

### ✅ 推荐做法

1. **重复使用转换器实例**

   ```go
   // ✅ 正确：一次创建，多次使用
   converter := pbmo.NewBidiConverter(&pb.User{}, &User{})
   // 在循环中使用 converter
   ```

2. **使用增强转换器进行生产部署**

   ```go
   // ✅ 正确：生产环境推荐
   converter := pbmo.NewEnhancedBidiConverter(&pb.User{}, &User{}, logger)
   ```

3. **为复杂嵌套使用安全转换器**

   ```go
   // ✅ 正确：处理可能为 nil 的嵌套字段
   safeConverter := pbmo.NewSafeConverter(&pb.ComplexMessage{}, &ComplexModel{})
   ```

### ❌ 避免做法

1. **不要频繁创建转换器**

   ```go
   // ❌ 错误：每次都创建新实例
   for _, pb := range pbList {
       converter := pbmo.NewBidiConverter(&pb.User{}, &User{})  // 浪费！
   }
   ```

2. **不要忽略错误处理**

   ```go
   // ❌ 错误：忽略转换错误
   converter.ConvertPBToModel(pb, &model)  // 没有检查 err
   ```

## 🔗 相关链接

- 📖 [完整文档](./README.md)
- 🎯 [最佳实践指南](./BEST_PRACTICES.md) - 常见场景处理和性能优化 ⭐
- 📚 [使用示例大全](./USAGE_EXAMPLES.md)
- 🛡️ [安全转换器指南](./SAFE_CONVERTER_GUIDE.md)
- 📊 [性能优化说明](./PERFORMANCE_OPTIMIZATION.md)
- 🔧 [集成总结](./INTEGRATION_SUMMARY.md)

---

**🎉 恭喜！你现在可以高效地使用 PBMO 进行 Protocol Buffer 和 GORM Model 之间的转换了！**
