# 📚 PBMO Advanced API 详细使用文档

## 🎯 概述

PBMO Advanced API 提供了一套高级的、易于使用的转换器系统，支持：

- **三层性能级别**：Basic → Optimized → UltraFast
- **智能脱敏机制**：自动发现 + 自定义规则 + 运行时注册
- **并发批量转换**：一行代码实现高性能批量处理
- **灵活校验系统**：struct tag 自动发现 + 手动配置
- **性能监控**：完整的统计信息和指标

---

## 🚀 快速开始

### ✅ 推荐写法：三种创建方式

```go
// 方式1：通用创建器（推荐用于复杂场景）
converter := pbmo.NewAdvancedConverter(&pb.User{}, &User{},
    pbmo.WithPerformanceLevel(pbmo.OptimizedLevel),
    pbmo.WithDesensitization(true, true),
    pbmo.WithValidation(true, true),
)

// 方式2：便利构造器（推荐用于特定性能级别）
converter := pbmo.NewOptimizedAdvancedConverter(&pb.User{}, &User{},
    pbmo.WithDesensitization(true, true),
)

// 方式3：超级简易批量转换（推荐用于一次性转换）
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
    pbmo.FastMode(),
)
```

### ❌ 不推荐写法

```go
// ❌ 直接使用基础转换器处理大批量数据
converter := pbmo.NewBidiConverter(&pb.User{}, &User{})
for _, pb := range pbUsers {  // 性能差，无并发
    var user User
    converter.ConvertPBToModel(pb, &user)
    users = append(users, user)
}

// ❌ 手动实现并发转换
var wg sync.WaitGroup
semaphore := make(chan struct{}, 10)
// ... 50+ 行复杂的并发处理代码

// ❌ 忽略错误处理
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers)
// 没有检查 result.Errors 和 result.Failed
```

---

## 🎛️ 性能级别选择

### 1. BasicLevel (基线) - BidiConverter

```go
// ✅ 适用场景：功能完整性优先
converter := pbmo.NewBasicAdvancedConverter(&pb.User{}, &User{})

// 特点：
// - 完整的反射机制支持
// - 最好的兼容性
// - 性能：~130ns/op
```

### 2. OptimizedLevel (推荐生产) - OptimizedBidiConverter

```go
// ✅ 适用场景：生产环境推荐
converter := pbmo.NewOptimizedAdvancedConverter(&pb.User{}, &User{},
    pbmo.WithDesensitization(true, true),
    pbmo.WithValidation(true, true),
)

// 特点：
// - 16x 性能提升
// - 生产级稳定性
// - 性能：~8ns/op
// - 内存优化
```

### 3. UltraFastLevel (超高性能) - UltraFastConverter

```go
// ✅ 适用场景：极致性能要求
converter := pbmo.NewUltraFastAdvancedConverter(&pb.User{}, &User{})

// 特点：
// - 极致性能优化
// - 等同于 OptimizedLevel 性能
// - 适合高频调用场景
```

---

## 🔒 脱敏功能使用

### ✅ 推荐写法：自动发现 + 灵活扩展

```go
// 1. struct tag 自动发现（最推荐）
type User struct {
    Name     string `desensitize:"name"`
    Email    string `desensitize:"email"`
    Phone    string `desensitize:"phone"`
    BankCard string `desensitize:"bankCard"`
    Custom   string `desensitize:"custom:mask(2,6,*)"`
}

// 2. 运行时注册自定义类型（推荐用于扩展）
converter.RegisterDesensitizationType("businessId", "custom")
converter.RegisterDesensitizationType("socialId", "identityCard")

// 3. 注册自定义解析器（推荐用于复杂规则）
converter.RegisterCustomParser("range", func(tag string, rule *pbmo.DesensitizeRule) error {
    if strings.HasPrefix(tag, "range:") {
        // 解析 range:1-3 格式
        rangeStr := strings.TrimPrefix(tag, "range:")
        parts := strings.Split(rangeStr, "-")
        rule.Type = "range"
        rule.Config = map[string]string{
            "start": parts[0],
            "end":   parts[1],
        }
    }
    return nil
})

// 使用示例
converter := pbmo.NewAdvancedConverter(&pb.User{}, &User{},
    pbmo.WithDesensitization(true, true),  // 启用自动发现
)
```

### ❌ 不推荐写法

```go
// ❌ 硬编码脱敏规则
func customDesensitize(data interface{}) {
    // 大量 if/switch 硬编码逻辑
    switch v := data.(type) {
    case *User:
        v.Email = maskEmail(v.Email)  // 不灵活，难维护
        v.Phone = maskPhone(v.Phone)
    }
}

// ❌ 忽略脱敏配置
converter := pbmo.NewAdvancedConverter(&pb.User{}, &User{})
// 没有启用脱敏，敏感数据可能泄露
```

---

## ✅ 校验功能使用

### ✅ 推荐写法：多层次校验策略

```go
// 1. struct tag 自动校验（最推荐）
type User struct {
    Name  string `validate:"required,min=2,max=50"`
    Email string `validate:"required,email"`
    Age   int    `validate:"min=0,max=120"`
}

// 2. 编程式配置校验
converter := pbmo.NewAdvancedConverter(&pb.User{}, &User{},
    pbmo.WithEasyValidation("User",
        pbmo.EasyRule{Field: "Name", Required: true, MinLen: 2, MaxLen: 50},
        pbmo.EasyRule{Field: "Email", Required: true, Email: true},
    ),
)

// 3. 临时禁用校验（推荐用于性能敏感场景）
func BulkImport(pbUsers []*pb.User) error {
    restore := converter.TemporaryDisableValidation()
    defer restore()  // 确保函数结束时恢复
    
    // 批量导入时禁用校验，提升性能
    result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
        pbmo.FastMode(),
    )
    return processResult(result)
}
```

### ❌ 不推荐写法

```go
// ❌ 全局禁用校验
converter := pbmo.NewAdvancedConverter(&pb.User{}, &User{},
    pbmo.WithValidation(false, false),  // 安全风险
)

// ❌ 不检查校验结果
var user User
converter.ConvertPBToModel(pb, &user)
// 没有检查转换是否成功，可能有校验错误

// ❌ 手动实现校验逻辑
func validateUser(user *User) error {
    if user.Name == "" {  // 重复造轮子
        return errors.New("name required")
    }
    // ... 大量手动校验代码
}
```

---

## 🚄 并发批量转换

### ✅ 推荐写法：SuperEasyBatchConvert

```go
// 1. 一行代码搞定（最推荐）
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers)

// 2. 性能模式选择
// 🏃‍♂️ 大数据量，性能优先
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
    pbmo.FastMode(),  // 自动优化：更多协程+更大批次+禁用校验
)

// 🛡️ 重要数据，安全优先
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
    pbmo.SafeMode(),  // 自动配置：较少协程+较小批次+启用校验
)

// 🔒 安全模式，带脱敏
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
    pbmo.SecureMode(),  // 启用校验+脱敏
)

// 3. 精确控制
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
    pbmo.WithConcurrency(8, 200, 30*time.Second),
    pbmo.WithTimeout(1*time.Minute),
    pbmo.NoValidation(),
)

// 4. 错误处理（推荐）
if result.Failed > 0 {
    log.Printf("转换失败: %d/%d", result.Failed, len(pbUsers))
    for _, err := range result.Errors {
        log.Printf("错误: %v", err)
    }
}

// 5. 性能监控（推荐）
log.Printf("转换完成: 成功=%d, 失败=%d, 耗时=%v, 平均=%v/op", 
    result.Success, result.Failed, result.Elapsed,
    result.Elapsed/time.Duration(len(pbUsers)))
```

### ❌ 不推荐写法

```go
// ❌ 手动实现并发（复杂且易错）
func manualConcurrentConvert(pbUsers []*pb.User) []User {
    var users []User
    var mu sync.Mutex
    var wg sync.WaitGroup
    semaphore := make(chan struct{}, 10)
    
    for _, pb := range pbUsers {
        wg.Add(1)
        go func(pb *pb.User) {
            defer wg.Done()
            // 获取信号量
            semaphore <- struct{}{}
            defer func() { <-semaphore }()
            
            // 转换逻辑
            var user User
            converter.ConvertPBToModel(pb, &user)
            
            // 线程安全地添加结果
            mu.Lock()
            users = append(users, user)
            mu.Unlock()
        }(pb)
    }
    
    wg.Wait()
    return users  // 50+ 行代码，SuperEasyBatchConvert 一行搞定
}

// ❌ 忽略超时控制
result := pbmo.SuperEasyBatchConvert[*pb.User, User](hugeDataSet)
// 没有设置超时，可能长时间阻塞

// ❌ 不检查结果
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers)
users := result.Data  // 没有检查 Errors 和 Failed 字段
```

---

## 📊 性能监控和调优

### ✅ 推荐写法：全面监控

```go
// 1. 转换器统计信息
stats := converter.GetStats()
log.Printf("转换器状态: %+v", stats)

// 2. 性能信息
perfInfo := converter.GetPerformanceInfo()
log.Printf("性能级别: %s - %s", 
    perfInfo["level_name"], perfInfo["description"])

// 3. 批量转换结果监控
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers)
if result.Success > 0 {
    avgTime := result.Elapsed / time.Duration(result.Success)
    log.Printf("转换性能: %v/op", avgTime)
}

// 4. 动态调优
if result.Elapsed > 5*time.Second {
    // 转换太慢，调整并发配置
    converter.UpdateConcurrencyConfig(
        runtime.NumCPU()*2,  // 增加协程
        200,                 // 增加批次大小
        60*time.Second,      // 增加超时时间
    )
}

// 5. 内存使用监控
var m runtime.MemStats
runtime.ReadMemStats(&m)
log.Printf("内存使用: %.2f MB", float64(m.Alloc)/1024/1024)
```

### ❌ 不推荐写法

```go
// ❌ 忽略性能监控
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers)
// 没有监控转换性能和结果

// ❌ 固定配置，不调优
converter := pbmo.NewAdvancedConverter(&pb.User{}, &User{})
// 使用默认配置，不根据实际情况调整
```

---

## 🎯 实际应用场景

### 场景1：微服务 API 转换

```go
// ✅ 推荐：服务级别的转换器管理
type UserService struct {
    converter *pbmo.AdvancedConverter
}

func NewUserService() *UserService {
    return &UserService{
        converter: pbmo.NewOptimizedAdvancedConverter(&pb.User{}, &User{},
            pbmo.WithDesensitization(true, true),  // API 响应脱敏
            pbmo.WithValidation(true, true),       // 数据校验
        ),
    }
}

// 单个用户转换
func (s *UserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*User, error) {
    pbUser, err := s.fetchUserFromDB(req.Id)
    if err != nil {
        return nil, err
    }
    
    var user User
    if err := s.converter.ConvertPBToModel(pbUser, &user); err != nil {
        return nil, fmt.Errorf("转换用户数据失败: %w", err)
    }
    
    return &user, nil
}

// 批量用户转换
func (s *UserService) ListUsers(ctx context.Context, req *pb.ListUsersRequest) ([]User, error) {
    pbUsers, err := s.fetchUsersFromDB(req.Filters)
    if err != nil {
        return nil, err
    }
    
    // 智能选择转换策略
    switch {
    case len(pbUsers) <= 10:
        // 小批量：直接转换
        users := make([]User, 0, len(pbUsers))
        for _, pb := range pbUsers {
            var user User
            if err := s.converter.ConvertPBToModel(pb, &user); err != nil {
                return nil, err
            }
            users = append(users, user)
        }
        return users, nil
        
    default:
        // 大批量：并发转换
        result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
            pbmo.SafeMode(),  // 保持校验和脱敏
        )
        
        if result.Failed > 0 {
            return nil, fmt.Errorf("批量转换失败: %d/%d", 
                result.Failed, len(pbUsers))
        }
        
        return result.Data, nil
    }
}
```

### 场景2：数据迁移工具

```go
// ✅ 推荐：高性能数据迁移
func MigrateUsers(sourceDB, targetDB *sql.DB) error {
    const batchSize = 1000
    
    // 数据迁移时使用超高性能模式
    converter := pbmo.NewUltraFastAdvancedConverter(&pb.User{}, &User{})
    
    offset := 0
    for {
        // 分批读取数据
        pbUsers, err := fetchUsersFromSource(sourceDB, offset, batchSize)
        if err != nil {
            return err
        }
        if len(pbUsers) == 0 {
            break
        }
        
        // 临时禁用校验，提升迁移性能
        restore := converter.TemporaryDisableValidation()
        
        // 超高性能批量转换
        result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
            pbmo.FastMode(),                // 最大性能模式
            pbmo.WithTimeout(5*time.Minute), // 长时间超时
        )
        
        restore() // 恢复校验设置
        
        if result.Failed > 0 {
            log.Printf("批次迁移部分失败: %d/%d", result.Failed, len(pbUsers))
            // 记录失败数据，继续处理
        }
        
        // 保存到目标数据库
        if err := saveUsersToTarget(targetDB, result.Data); err != nil {
            return err
        }
        
        log.Printf("迁移进度: %d 用户已处理", offset+len(pbUsers))
        offset += batchSize
    }
    
    return nil
}
```

### 场景3：消息队列处理

```go
// ✅ 推荐：消息队列批量处理
type MessageProcessor struct {
    converter *pbmo.AdvancedConverter
}

func NewMessageProcessor() *MessageProcessor {
    return &MessageProcessor{
        converter: pbmo.NewOptimizedAdvancedConverter(&pb.UserEvent{}, &UserEvent{},
            pbmo.WithDesensitization(true, true),  // 消息脱敏
        ),
    }
}

func (p *MessageProcessor) ProcessBatch(messages []*pb.UserEvent) error {
    // 根据消息数量选择处理策略
    switch {
    case len(messages) <= 50:
        // 小批量：安全模式
        result := pbmo.SuperEasyBatchConvert[*pb.UserEvent, UserEvent](messages,
            pbmo.SafeMode(),
        )
        return p.handleResult(result)
        
    default:
        // 大批量：快速模式
        result := pbmo.SuperEasyBatchConvert[*pb.UserEvent, UserEvent](messages,
            pbmo.FastMode(),
            pbmo.WithTimeout(30*time.Second),
        )
        return p.handleResult(result)
    }
}

func (p *MessageProcessor) handleResult(result *pbmo.ConversionResult[UserEvent]) error {
    // 处理转换结果
    if result.Failed > 0 {
        log.Printf("消息转换失败: %d/%d", result.Failed, 
            result.Success+result.Failed)
        
        // 失败消息发送到死信队列
        for _, err := range result.Errors {
            p.sendToDeadLetter(err)
        }
    }
    
    // 处理成功的消息
    for _, event := range result.Data {
        if err := p.processEvent(&event); err != nil {
            log.Printf("事件处理失败: %v", err)
        }
    }
    
    return nil
}
```

---

## ⚡ 性能基准和选择指南

### 性能对比

| 转换器 | 单次转换 | 批量转换(100) | 批量转换(1000) | 推荐场景 |
|--------|----------|---------------|----------------|----------|
| BasicLevel | 130ns/op | 15μs | 150μs | 功能完整性优先 |
| OptimizedLevel | 8ns/op | 1μs | 10μs | 生产环境推荐 |
| UltraFastLevel | 8ns/op | 1μs | 10μs | 极致性能要求 |
| SuperEasyBatch | - | 0.5μs | 5μs | 一次性大批量 |

### 选择指南

```go
// ✅ 数据量 < 10：直接转换
var user User
converter.ConvertPBToModel(pbUser, &user)

// ✅ 数据量 10-100：SafeMode
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
    pbmo.SafeMode(),
)

// ✅ 数据量 100-1000：FastMode
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
    pbmo.FastMode(),
)

// ✅ 数据量 > 1000：分批处理
for i := 0; i < len(pbUsers); i += 1000 {
    end := i + 1000
    if end > len(pbUsers) {
        end = len(pbUsers)
    }
    
    batch := pbUsers[i:end]
    result := pbmo.SuperEasyBatchConvert[*pb.User, User](batch,
        pbmo.FastMode(),
    )
    // 处理结果...
}
```

---

## 🚨 常见错误和解决方案

### 1. 内存泄漏

```go
// ❌ 错误：不释放转换器
func ProcessUsers() {
    for _, pb := range pbUsers {
        converter := pbmo.NewAdvancedConverter(&pb.User{}, &User{})
        // ... 每次创建新的转换器，内存泄漏
    }
}

// ✅ 正确：复用转换器
var converter = pbmo.NewAdvancedConverter(&pb.User{}, &User{})

func ProcessUsers() {
    // 复用全局转换器
    result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers)
}
```

### 2. 并发安全问题

```go
// ❌ 错误：并发访问转换器配置
go func() {
    converter.UpdateConcurrencyConfig(16, 200, 60*time.Second)
}()
go func() {
    stats := converter.GetStats()  // 可能读取到不一致的状态
}()

// ✅ 正确：转换器本身是线程安全的
go func() {
    converter.UpdateConcurrencyConfig(16, 200, 60*time.Second)
}()
go func() {
    stats := converter.GetStats()  // 安全的并发读取
}()
```

### 3. 性能配置错误

```go
// ❌ 错误：过度并发
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
    pbmo.WithConcurrency(1000, 10, 1*time.Second),  // 过多协程，超时太短
)

// ✅ 正确：合理配置
result := pbmo.SuperEasyBatchConvert[*pb.User, User](pbUsers,
    pbmo.WithConcurrency(runtime.NumCPU()*2, 100, 30*time.Second),
)
```

---

## 📋 最佳实践清单

### ✅ DO（推荐做法）

1. **优先使用 SuperEasyBatchConvert** - 一行代码解决大部分需求
2. **根据数据量选择性能模式** - SafeMode(小)/FastMode(大)
3. **启用自动脱敏和校验** - 基于 struct tag 的自动发现
4. **复用转换器实例** - 避免重复创建
5. **监控转换性能** - 使用 GetStats() 和结果统计
6. **处理转换错误** - 检查 result.Errors 和 result.Failed
7. **使用临时禁用功能** - 性能敏感时临时禁用校验
8. **选择合适的性能级别** - OptimizedLevel 适合大多数场景

### ❌ DON'T（不推荐做法）

1. **不要手动实现并发转换** - 复杂且易错
2. **不要忽略错误处理** - 可能导致数据不一致
3. **不要过度并发** - 协程数量要合理
4. **不要全局禁用校验** - 存在安全风险
5. **不要忽略超时设置** - 可能导致长时间阻塞
6. **不要重复创建转换器** - 造成内存浪费
7. **不要忽略性能监控** - 无法发现性能问题
8. **不要硬编码脱敏规则** - 缺乏灵活性

---

## 🎉 总结

PBMO Advanced API 通过以下设计实现了**简单易用**和**功能强大**的完美平衡：

- **一行代码批量转换**：`SuperEasyBatchConvert` 解决 90% 的使用场景
- **三层性能级别**：Basic/Optimized/UltraFast 满足不同性能需求
- **智能脱敏系统**：自动发现 + 运行时扩展
- **灵活校验机制**：struct tag + 编程配置 + 临时禁用
- **完整错误处理**：详细的错误信息和统计数据
- **性能监控**：全面的指标和调优建议

**现在，你可以用最少的代码实现最复杂的转换需求！** 🚀
