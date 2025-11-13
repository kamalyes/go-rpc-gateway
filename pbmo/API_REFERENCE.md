# 📖 PBMO API 参考文档

> 完整的 Protocol Buffer Model Object Converter API 文档

## 目录

- [核心转换器](#核心转换器)
- [增强转换器](#增强转换器)
- [安全转换器](#安全转换器)
- [服务集成](#服务集成)
- [字段校验器](#字段校验器)
- [错误处理](#错误处理)
- [类型定义](#类型定义)
- [工具函数](#工具函数)

## 核心转换器

### BidiConverter

基础的双向转换器，提供高性能的 PB ↔ Model 转换。

#### 构造函数

```go
func NewBidiConverter(pbType, modelType interface{}) *BidiConverter
```

**参数：**


- `pbType`: Protocol Buffer 类型的实例（如 `&pb.User{}`）
- `modelType`: Model 类型的实例（如 `&User{}`）

**返回：** `*BidiConverter` 实例


**示例：**

```go
converter := pbmo.NewBidiConverter(&pb.User{}, &User{})
```

#### 方法

##### ConvertPBToModel

```go
func (bc *BidiConverter) ConvertPBToModel(pb interface{}, modelPtr interface{}) error
```

将 Protocol Buffer 消息转换为 Model。


**参数：**

- `pb`: Protocol Buffer 消息实例
- `modelPtr`: Model 指针，用于接收转换结果

**返回：** `error` - 转换错误，如果成功则为 nil


**性能：** ~130ns/op

**示例：**

```go
var user User
if err := converter.ConvertPBToModel(pbUser, &user); err != nil {
    return err
}
```

##### ConvertModelToPB

```go
func (bc *BidiConverter) ConvertModelToPB(model interface{}, pbPtr interface{}) error
```


将 Model 转换为 Protocol Buffer 消息。

**参数：**

- `model`: Model 实例或指针
- `pbPtr`: Protocol Buffer 消息指针，用于接收转换结果


**返回：** `error` - 转换错误，如果成功则为 nil

**性能：** ~101ns/op

**示例：**

```go
var pbUser pb.User
if err := converter.ConvertModelToPB(&user, &pbUser); err != nil {
    return err
}
```

##### BatchConvertPBToModel

```go

func (bc *BidiConverter) BatchConvertPBToModel(pbSlice interface{}, modelSlicePtr interface{}) error
```

批量转换 Protocol Buffer 消息列表为 Model 列表。

**参数：**


- `pbSlice`: Protocol Buffer 消息切片
- `modelSlicePtr`: Model 切片指针，用于接收转换结果

**返回：** `error` - 转换错误，如果成功则为 nil

**示例：**

```go
var users []User
if err := converter.BatchConvertPBToModel(pbUsers, &users); err != nil {
    return err
}
```

##### BatchConvertModelToPB


```go
func (bc *BidiConverter) BatchConvertModelToPB(modelSlice interface{}, pbSlicePtr interface{}) error
```

批量转换 Model 列表为 Protocol Buffer 消息列表。

**参数：**

- `modelSlice`: Model 切片
- `pbSlicePtr`: Protocol Buffer 消息切片指针，用于接收转换结果

**返回：** `error` - 转换错误，如果成功则为 nil


##### RegisterTransformer

```go
func (bc *BidiConverter) RegisterTransformer(field string, transformer func(interface{}) interface{})

```

为特定字段注册自定义转换函数。

**参数：**

- `field`: 字段名称
- `transformer`: 转换函数，接收原值并返回转换后的值

**示例：**

```go
// 价格从分转换为元
converter.RegisterTransformer("Price", func(v interface{}) interface{} {
    if cents, ok := v.(int64); ok {
        return float64(cents) / 100.0
    }
    return v
})
```

## 增强转换器


### EnhancedBidiConverter

带有日志记录、性能监控和错误处理的增强转换器。

#### 构造函数

```go

func NewEnhancedBidiConverter(pbType, modelType interface{}, log logger.ILogger) *EnhancedBidiConverter
```

**参数：**

- `pbType`: Protocol Buffer 类型的实例
- `modelType`: Model 类型的实例
- `log`: 日志实例，用于记录转换过程和错误

**返回：** `*EnhancedBidiConverter` 实例

**示例：**

```go
converter := pbmo.NewEnhancedBidiConverter(&pb.User{}, &User{}, logger)

```

#### 方法

##### ConvertPBToModelWithLog

```go
func (ebc *EnhancedBidiConverter) ConvertPBToModelWithLog(pb interface{}, modelPtr interface{}) error
```

带日志记录的 PB → Model 转换。

**特性：**

- 自动记录转换开始和结束
- 记录转换耗时
- 自动更新性能指标
- 转换错误自动转换为 gRPC status error

**返回：** `error` - 已处理的 gRPC status error

##### ConvertModelToPBWithLog

```go

func (ebc *EnhancedBidiConverter) ConvertModelToPBWithLog(model interface{}, pbPtr interface{}) error
```

带日志记录的 Model → PB 转换。

##### GetMetrics

```go
func (ebc *EnhancedBidiConverter) GetMetrics() *ConversionMetrics
```

获取转换性能指标。

**返回：** `*ConversionMetrics` 包含以下字段：

- `TotalConversions int64` - 总转换次数
- `SuccessfulConversions int64` - 成功转换次数
- `FailedConversions int64` - 失败转换次数
- `TotalDuration time.Duration` - 总耗时
- `AverageDuration time.Duration` - 平均耗时
- `LastError error` - 最后一次错误

##### ReportMetrics

```go
func (ebc *EnhancedBidiConverter) ReportMetrics()
```

报告当前性能指标到日志。

##### ConvertPBToModelBatchSafe

```go
func (ebc *EnhancedBidiConverter) ConvertPBToModelBatchSafe(pbSlice interface{}, modelSlicePtr interface{}) *BatchConversionResult
```

安全的批量转换，失败的项目不会影响其他项目。

**返回：** `*BatchConversionResult` 包含详细的转换结果

## 安全转换器

### SafeConverter

基于 go-toolbox/safe 的 SafeAccess 特性，提供安全的字段访问和转换。

#### 构造函数


```go
func NewSafeConverter(pbType, modelType interface{}) *SafeConverter
```

#### 方法

##### SafeConvertPBToModel

```go
func (sc *SafeConverter) SafeConvertPBToModel(pb interface{}, modelPtr interface{}) error
```

安全转换，自动处理 nil 指针。


**特性：**

- 自动检查 nil 指针
- 详细的错误信息
- 安全的字段访问


##### SafeFieldAccess

```go
func (sc *SafeConverter) SafeFieldAccess(obj interface{}, fieldPath ...string) *SafeValue
```

链式安全字段访问，类似 JavaScript 的可选链操作符。

**参数：**

- `obj`: 要访问的对象
- `fieldPath`: 字段路径，支持多层嵌套

**返回：** `*SafeValue` 安全值包装器

**示例：**

```go
// 安全访问 user.Profile.Address.City
value := converter.SafeFieldAccess(user, "Profile", "Address", "City")
if value.IsValid() {
    city := value.String("默认城市")
}
```

##### SafeBatchConvertPBToModel

```go
func (sc *SafeConverter) SafeBatchConvertPBToModel(pbSlice interface{}, modelSlicePtr interface{}) *SafeBatchResult
```

安全的批量转换，提供详细的每项结果。

**返回：** `*SafeBatchResult` 包含每个转换项的详细结果

### SafeValue

安全值包装器，提供类型安全的值提取。

#### 方法

```go
func (sv *SafeValue) IsValid() bool                    // 检查值是否有效
func (sv *SafeValue) String(defaultValue string) string   // 获取字符串值
func (sv *SafeValue) Int(defaultValue int) int           // 获取整数值
func (sv *SafeValue) Float64(defaultValue float64) float64 // 获取浮点数值
func (sv *SafeValue) Bool(defaultValue bool) bool        // 获取布尔值
```

## 服务集成

### ServiceIntegration

完整的 gRPC 服务集成解决方案，集成转换、校验、错误处理。


#### 构造函数

```go
func NewServiceIntegration(pbType, modelType interface{}, log logger.ILogger) *ServiceIntegration
```

#### 方法

##### RegisterValidationRules

```go
func (si *ServiceIntegration) RegisterValidationRules(typeName string, rules ...FieldRule)
```

注册字段校验规则。

**参数：**

- `typeName`: 类型名称
- `rules`: 校验规则列表

##### ConvertAndValidatePBToModel

```go
func (si *ServiceIntegration) ConvertAndValidatePBToModel(pb interface{}, modelPtr interface{}) error
```

转换并校验，一步完成。

##### HandleError

```go
func (si *ServiceIntegration) HandleError(err error, operation string) error
```

统一错误处理，自动转换为 gRPC status error。

## 字段校验器

### FieldValidator

字段校验器，支持多种校验规则。

#### 构造函数

```go
func NewFieldValidator() *FieldValidator
```

#### 方法

##### RegisterRules

```go
func (fv *FieldValidator) RegisterRules(typeName string, rules ...FieldRule)
```

##### Validate

```go
func (fv *FieldValidator) Validate(obj interface{}) error
```

执行字段校验。


### FieldRule

字段校验规则定义。

```go
type FieldRule struct {
    Name     string                 // 字段名称
    Required bool                   // 是否必填
    MinLen   int                    // 最小长度
    MaxLen   int                    // 最大长度
    Min      float64                // 最小值
    Max      float64                // 最大值
    Pattern  string                 // 正则表达式
    Custom   func(interface{}) error // 自定义校验函数
}
```

**示例：**

```go
rules := []pbmo.FieldRule{
    {
        Name:     "Name",
        Required: true,
        MinLen:   2,
        MaxLen:   50,
    },
    {
        Name:    "Email", 
        Pattern: `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
    },
    {
        Name: "Age",
        Min:  1,
        Max:  150,
    },
    {
        Name: "Password",
        Custom: func(v interface{}) error {
            pwd := v.(string)
            if len(pwd) < 8 {
                return errors.New("密码至少8位")
            }
            return nil
        },
    },
}
```

## 错误处理

### ConversionErrorHandler

转换错误处理器，提供统一的错误处理和日志记录。

#### 构造函数

```go
func NewConversionErrorHandler(log logger.ILogger) *ConversionErrorHandler
```

#### 方法

##### HandleConversionError

```go
func (ceh *ConversionErrorHandler) HandleConversionError(err error, context string) error
```

处理转换错误，自动转换为 gRPC status error。

##### HandleValidationError

```go
func (ceh *ConversionErrorHandler) HandleValidationError(err error) error
```

处理校验错误。

##### LogConversionStart

```go
func (ceh *ConversionErrorHandler) LogConversionStart(fromType, toType string)
```

记录转换开始日志。

##### LogConversionSuccess

```go
func (ceh *ConversionErrorHandler) LogConversionSuccess(fromType, toType string)
```

记录转换成功日志。

## 类型定义

### ConversionMetrics

转换性能指标。

```go
type ConversionMetrics struct {
    TotalConversions      int64         // 总转换次数
    SuccessfulConversions int64         // 成功转换次数
    FailedConversions     int64         // 失败转换次数
    TotalDuration         time.Duration // 总耗时
    AverageDuration       time.Duration // 平均耗时
    LastError             error         // 最后一次错误
}
```

### BatchConversionResult

批量转换结果。

```go
type BatchConversionResult struct {
    SuccessCount int                    // 成功数量
    FailureCount int                    // 失败数量
    Errors       []BatchConversionError // 错误列表
}

type BatchConversionError struct {
    Index   int   // 失败项索引
    Error   error // 错误信息
    PBValue interface{} // 原始 PB 值
}
```

### SafeBatchResult

安全批量转换结果。

```go
type SafeBatchResult struct {
    SuccessCount int                    // 成功数量
    FailureCount int                    // 失败数量
    Results      []SafeBatchResultItem  // 详细结果列表
}

type SafeBatchResultItem struct {
    Index   int         // 项目索引
    Success bool        // 是否成功
    Value   interface{} // 转换后的值（成功时）
    Error   error       // 错误信息（失败时）
}
```

## 工具函数

### 类型检查函数

```go
func IsValidationError(err error) bool   // 检查是否为校验错误
func IsConversionError(err error) bool   // 检查是否为转换错误
func IsNilError(err error) bool          // 检查是否为 nil 错误
```

### 类型名获取函数

```go
func getTypeName(t reflect.Type) string  // 获取类型名称
```

## 性能基准

### 转换性能

| 转换器 | PB→Model | Model→PB | 内存分配 |
|--------|---------|---------|---------|
| BidiConverter | 130ns/op | 101ns/op | 0 allocs |
| EnhancedConverter | 200ns/op | 180ns/op | 1 allocs |
| SafeConverter | 150ns/op | 130ns/op | 0 allocs |

### 批量转换性能

| 数据量 | BidiConverter | EnhancedConverter | SafeConverter |
|--------|--------------|------------------|---------------|
| 100 items | 12μs | 18μs | 15μs |
| 1,000 items | 120μs | 180μs | 150μs |
| 10,000 items | 1.2ms | 1.8ms | 1.5ms |

## 最佳实践

### 转换器创建

```go
// ✅ 正确：一次创建，重复使用
var userConverter = pbmo.NewBidiConverter(&pb.User{}, &User{})

// ❌ 错误：每次创建新实例
func convertUser(pb *pb.User) (*User, error) {
    converter := pbmo.NewBidiConverter(&pb.User{}, &User{}) // 浪费！
    // ...
}
```

### 错误处理

```go
// ✅ 正确：检查错误
if err := converter.ConvertPBToModel(pb, &model); err != nil {
    return nil, err
}

// ❌ 错误：忽略错误
converter.ConvertPBToModel(pb, &model) // 危险！
```

### 校验规则


```go
// ✅ 正确：在服务初始化时注册

func NewUserService() *UserService {
    service := pbmo.NewServiceIntegration(&pb.User{}, &User{}, logger)
    service.RegisterValidationRules("User", userRules...)

    return &UserService{service: service}
}


// ❌ 错误：在每次请求时注册
func (s *UserService) CreateUser(req *pb.CreateUserRequest) {
    s.service.RegisterValidationRules("User", rules...) // 浪费！

}
```


## 故障排除

### 常见错误

1. **类型不匹配**


   ```
   failed to convert field Name: cannot assign string to int32
   ```

   **解决：** 确保 PB 和 Model 字段类型兼容


2. **nil 指针错误**

   ```
   pb message cannot be nil
   ```

   **解决：** 使用 SafeConverter 或检查输入


3. **字段未找到**

   ```
   field "NonExistentField" not found in destination type
   ```

   **解决：** 确保字段名称匹配或使用 struct tag

### 调试技巧

1. **启用详细日志**

   ```go

   logger := logger.NewLogger(logger.WithLevel(logger.DebugLevel))
   converter := pbmo.NewEnhancedBidiConverter(&pb.User{}, &User{}, logger)
   ```

2. **使用性能监控**

   ```go
   metrics := converter.GetMetrics()
   if metrics.FailedConversions > 0 {
       log.Printf("转换失败率: %.2f%%", 
           float64(metrics.FailedConversions) / float64(metrics.TotalConversions) * 100)
   }
   ```

3. **使用安全转换器调试复杂嵌套**

   ```go
   safeConverter := pbmo.NewSafeConverter(&pb.Complex{}, &Complex{})
   result := safeConverter.SafeBatchConvertPBToModel(pbList, &modelList)
   for _, item := range result.Results {
       if !item.Success {
           log.Printf("Item %d failed: %v", item.Index, item.Error)
       }
   }
   ```

---

**📝 注意：** 本文档基于 PBMO v1.0.0 版本编写。如需最新信息，请查看源代码或 [GitHub 仓库](https://github.com/kamalyes/go-rpc-gateway/tree/master/pbmo)。
