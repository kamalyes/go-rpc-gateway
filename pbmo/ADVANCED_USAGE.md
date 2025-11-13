# 🚀 PBMO 高级 API 使用指南

> 提供超级简化的傻瓜式API，让复杂操作变得简单

## 📋 目录

- [自动校验发现机制](#自动校验发现机制)
- [傻瓜式校验配置](#傻瓜式校验配置)
- [超级简易批量转换](#超级简易批量转换)
- [临时禁用功能](#临时禁用功能)
- [性能模式选择](#性能模式选择)

## 🔍 自动校验发现机制

### 基于 Struct Tag 的自动发现

PBMO 会自动扫描你的结构体标签，无需手动注册校验规则！

```go
// 1. 在 Model 结构体中添加 validate tag
type User struct {
    ID    uint   `json:"id" gorm:"primary_key"`
    Name  string `json:"name" validate:"required,min=2,max=50"`  // 自动发现！
    Email string `json:"email" validate:"required,email"`        // 自动发现！
    Age   int    `json:"age" validate:"min=0,max=120"`          // 自动发现！
}

// 2. 创建转换器时自动应用规则
converter := pbmo.NewAdvancedConverter(&pb.User{}, &User{})
// ✅ 校验规则已自动注册，无需手动配置！
```

### 发现机制的生效场景

```go
// 🎯 场景1: 创建转换器时自动发现
func NewUserService() *UserService {
    // 自动扫描 User 结构体的 validate tag
    converter := pbmo.NewAdvancedConverter(&pb.User{}, &User{})
    
    return &UserService{converter: converter}
}

// 🎯 场景2: 转换时自动校验
func (s *UserService) CreateUser(pbUser *pb.User) error {
    var user User
    
    // 转换时自动应用已发现的校验规则
    if err := s.converter.ConvertPBToModel(pbUser, &user); err != nil {
        return err  // 自动校验失败
    }
    
    return s.userRepo.Create(&user)
}

// 🎯 场景3: 批量转换时的自动校验
func (s *UserService) BatchCreate(pbUsers []*pb.User) error {
    result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers)
    
    if len(result.Errors) > 0 {
        return fmt.Errorf("校验失败: %v", result.Errors)
    }
    
    return s.userRepo.BatchCreate(result.Data)
}
```

## 🎯 傻瓜式校验配置

### 超级简单的校验规则定义

```go
// ❌ 之前：复杂的手动注册
converter.RegisterValidationRules("User",
    pbmo.FieldRule{
        Name:     "Name",
        Required: true,
        MinLen:   2,
        MaxLen:   50,
    },
    pbmo.FieldRule{
        Name:    "Email", 
        Pattern: `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
    },
)

// ✅ 现在：傻瓜式配置
converter := pbmo.NewAdvancedConverter(&pb.User{}, &User{},
    pbmo.WithEasyValidation("User",
        pbmo.EasyRule{Field: "Name", Required: true, MinLen: 2, MaxLen: 50},
        pbmo.EasyRule{Field: "Email", Email: true},  // 自动邮箱正则
        pbmo.EasyRule{Field: "Age", Min: 0, Max: 120},
    ),
)

// 🌟 最简单：直接用 struct tag（推荐）
type User struct {
    Name  string `validate:"required,min=2,max=50"`
    Email string `validate:"required,email"`
    Age   int    `validate:"min=0,max=120"`
}

converter := pbmo.NewAdvancedConverter(&pb.User{}, &User{})
// 自动发现并应用所有校验规则！
```

### 预定义的常用规则

```go
// 内置的快捷规则
pbmo.EasyRule{Field: "Email", Email: true}        // 邮箱格式
pbmo.EasyRule{Field: "Phone", Phone: true}        // 手机号格式
pbmo.EasyRule{Field: "URL", URL: true}           // URL 格式
pbmo.EasyRule{Field: "Password", Strong: true}    // 强密码
```

## 🚀 超级简易批量转换

### 一行代码完成复杂的并发转换

```go
// ❌ 之前：复杂的并发处理代码（50+ 行）
results := make([][]User, 0, (len(pbUsers)+batchSize-1)/batchSize)
errs := make([]error, 0, (len(pbUsers)+batchSize-1)/batchSize)
semaphore := make(chan struct{}, maxGoroutines)
var wg sync.WaitGroup
var mu sync.Mutex
// ... 50多行复杂代码

// ✅ 现在：一行搞定！
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers)

// 🎯 带选项的高级用法
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
    pbmo.FastMode(),                    // 快速模式
    pbmo.WithTimeout(1*time.Minute),    // 超时设置
    pbmo.NoValidation(),                // 临时禁用校验
)

// 📊 查看转换结果
fmt.Printf("成功: %d, 失败: %d, 耗时: %v\n", 
    result.Success, result.Failed, result.Elapsed)

if len(result.Errors) > 0 {
    log.Printf("转换错误: %v", result.Errors)
}
```

### 预设的性能模式

```go
// 🏃‍♂️ 快速模式：适合大量数据，性能优先
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
    pbmo.FastMode(),  // 自动配置：更多协程 + 更大批次 + 禁用校验
)

// 🛡️ 安全模式：适合重要数据，安全优先
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
    pbmo.SafeMode(),  // 自动配置：较少协程 + 较小批次 + 启用校验
)

// ⚖️ 自定义模式：精确控制
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
    pbmo.WithConcurrency(8, 200, 30*time.Second),  // 8协程, 200批次, 30秒超时
)
```

## 🎛️ 临时禁用功能

### 灵活的运行时控制

```go
// 创建转换器（默认启用校验）
converter := pbmo.NewAdvancedConverter(&pb.User{}, &User{})

// 🚫 临时禁用校验（比如批量导入时）
restore := converter.TemporaryDisableValidation()

// 在禁用期间进行转换
var users []User
converter.BatchConvertPBToModel(pbUsers, &users)  // 无校验，更快

// ✅ 恢复校验
restore()  // 调用返回的函数恢复原状态

// 📊 检查当前状态
if converter.IsValidationEnabled() {
    fmt.Println("校验已启用")
}
```

### 作用域式禁用

```go
func (s *Service) BulkImport(pbUsers []*pb.User) error {
    // 批量导入时临时禁用校验以提升性能
    restore := s.converter.TemporaryDisableValidation()
    defer restore()  // 确保函数结束时恢复
    
    // 快速批量转换
    result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
        pbmo.FastMode(),
    )
    
    return s.repo.BatchInsert(result.Data)
}

func (s *Service) CreateUser(pbUser *pb.User) error {
    // 单个用户创建保持校验启用
    var user User
    return s.converter.ConvertPBToModel(pbUser, &user)
}
```

## 📈 实用的使用模式

### 模式1：自适应转换器

```go
type UserConverter struct {
    *pbmo.AdvancedConverter
}

func NewUserConverter() *UserConverter {
    return &UserConverter{
        AdvancedConverter: pbmo.NewAdvancedConverter(&pb.User{}, &User{}),
    }
}

// 智能转换：根据数据量自动选择策略
func (c *UserConverter) SmartConvert(pbUsers []*pb.User) ([]User, error) {
    switch {
    case len(pbUsers) == 1:
        // 单个转换
        var user User
        err := c.ConvertPBToModel(pbUsers[0], &user)
        return []User{user}, err
        
    case len(pbUsers) < 100:
        // 小批量：安全模式
        result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers, 
            pbmo.SafeMode())
        return result.Data, nil
        
    default:
        // 大批量：快速模式
        result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers, 
            pbmo.FastMode())
        return result.Data, nil
    }
}
```

### 模式2：配置驱动的转换器

```go
type ConversionConfig struct {
    ValidationEnabled bool          `yaml:"validation_enabled"`
    MaxGoroutines    int           `yaml:"max_goroutines"`
    BatchSize        int           `yaml:"batch_size"`
    Timeout          time.Duration `yaml:"timeout"`
}

func NewConfigDrivenConverter(config *ConversionConfig) *pbmo.AdvancedConverter {
    return pbmo.NewAdvancedConverter(&pb.User{}, &User{},
        pbmo.WithValidation(config.ValidationEnabled, true),
        pbmo.WithConcurrency(
            config.MaxGoroutines, 
            config.BatchSize, 
            config.Timeout,
        ),
    )
}
```

### 模式3：监控和指标

```go
func (c *UserConverter) ConvertWithMetrics(pbUsers []*pb.User) ([]User, error) {
    start := time.Now()
    
    // 获取转换器状态
    stats := c.GetStats()
    log.Printf("转换开始 - 配置: %+v", stats)
    
    // 执行转换
    result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers)
    
    // 记录指标
    log.Printf("转换完成 - 成功: %d, 失败: %d, 耗时: %v", 
        result.Success, result.Failed, time.Since(start))
    
    return result.Data, nil
}
```

## 🔧 高级配置示例

```go
// 完整配置示例
converter := pbmo.NewAdvancedConverter(&pb.User{}, &User{},
    // 校验配置
    pbmo.WithValidation(true, true),  // 启用校验 + 自动发现
    
    // 并发配置
    pbmo.WithConcurrency(
        runtime.NumCPU()*2,  // 协程数
        500,                 // 批次大小
        2*time.Minute,       // 超时时间
    ),
    
    // 手动校验规则（补充自动发现）
    pbmo.WithEasyValidation("User",
        pbmo.EasyRule{Field: "CustomField", Required: true},
    ),
)
```

## 🎯 最佳实践总结

1. **优先使用 struct tag 自动发现** - 最简单，最直观
2. **大数据量使用 SuperEasyBatchConvert** - 一行代码搞定并发
3. **灵活使用 TemporaryDisableValidation** - 性能敏感时临时禁用
4. **选择合适的性能模式** - FastMode/SafeMode 根据场景选择
5. **监控转换指标** - 使用 GetStats() 和结果统计

🎉 **现在你可以用最少的代码实现最复杂的转换需求！**