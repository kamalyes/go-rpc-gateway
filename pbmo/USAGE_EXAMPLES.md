# 📚 PBMO 使用示例大全

> 包含各种转换器的具体用法和实际场景示例

## 目录

- [基础转换器示例](#基础转换器示例)
- [增强转换器示例](#增强转换器示例) 
- [安全转换器示例](#安全转换器示例)
- [服务集成示例](#服务集成示例)
- [批量转换示例](#批量转换示例)
- [性能优化示例](#性能优化示例)
- [实际项目集成](#实际项目集成)

## 基础转换器示例

### 🔧 简单数据类型转换

```go
package main

import (
    "fmt"
    "time"
    "github.com/kamalyes/go-rpc-gateway/pbmo"
    "google.golang.org/protobuf/types/known/timestamppb"
)

// 用户模型
type User struct {
    ID        uint      `gorm:"primarykey"`
    Name      string    `gorm:"size:100"`
    Email     string    `gorm:"uniqueIndex"`
    Age       int32
    IsActive  bool
    CreatedAt time.Time
    UpdatedAt time.Time
}

// 产品模型
type Product struct {
    ID          uint    `gorm:"primarykey"`
    Title       string  `gorm:"size:200"`
    Description string  `gorm:"type:text"`
    Price       float64 
    InStock     bool
    CreatedAt   time.Time
}

func BasicConverterExample() {
    // 创建转换器
    userConverter := pbmo.NewBidiConverter(&pb.User{}, &User{})
    
    // 测试数据
    pbUser := &pb.User{
        Id:        123,
        Name:      "张三",
        Email:     "zhangsan@example.com",
        Age:       30,
        IsActive:  true,
        CreatedAt: timestamppb.New(time.Now()),
    }
    
    // PB → Model 转换
    var user User
    if err := userConverter.ConvertPBToModel(pbUser, &user); err != nil {
        fmt.Printf("转换失败: %v\n", err)
        return
    }
    
    fmt.Printf("PB→Model: %+v\n", user)
    
    // 修改数据
    user.Age = 31
    user.UpdatedAt = time.Now()
    
    // Model → PB 转换  
    var pbResult pb.User
    if err := userConverter.ConvertModelToPB(&user, &pbResult); err != nil {
        fmt.Printf("反向转换失败: %v\n", err)
        return
    }
    
    fmt.Printf("Model→PB: %+v\n", pbResult)
}
```

### 🏗️ 嵌套结构转换

```go
// 用户档案
type UserProfile struct {
    ID       uint   `gorm:"primarykey"`
    UserID   uint   `gorm:"index"`
    Bio      string `gorm:"type:text"`
    Avatar   string `gorm:"size:500"`
    Website  string `gorm:"size:200"`
}

// 完整用户信息
type UserWithProfile struct {
    User
    Profile *UserProfile `gorm:"foreignKey:UserID"`
}

func NestedConverterExample() {
    converter := pbmo.NewBidiConverter(&pb.UserWithProfile{}, &UserWithProfile{})
    
    // 测试嵌套数据
    pbUserProfile := &pb.UserWithProfile{
        User: &pb.User{
            Id:       456,
            Name:     "李四", 
            Email:    "lisi@example.com",
            Age:      28,
            IsActive: true,
        },
        Profile: &pb.UserProfile{
            Bio:     "热爱编程的开发者",
            Avatar:  "https://example.com/avatar.jpg",
            Website: "https://lisi.dev",
        },
    }
    
    var userWithProfile UserWithProfile
    if err := converter.ConvertPBToModel(pbUserProfile, &userWithProfile); err != nil {
        fmt.Printf("嵌套转换失败: %v\n", err)
        return
    }
    
    fmt.Printf("嵌套转换成功: %+v\n", userWithProfile)
    if userWithProfile.Profile != nil {
        fmt.Printf("档案信息: %+v\n", *userWithProfile.Profile)
    }
}
```

## 增强转换器示例

### 📊 带监控的生产级转换

```go
package main

import (
    "context"
    "fmt"
    "github.com/kamalyes/go-rpc-gateway/pbmo"
    "github.com/kamalyes/go-logger"
)

func EnhancedConverterExample() {
    // 创建日志实例
    logger := logger.NewLogger(
        logger.WithLevel(logger.DebugLevel),
        logger.WithConsole(true),
    )
    
    // 创建增强转换器
    converter := pbmo.NewEnhancedBidiConverter(
        &pb.User{}, 
        &User{}, 
        logger,
    )
    
    // 模拟批量转换场景
    testUsers := []*pb.User{
        {Id: 1, Name: "用户1", Email: "user1@test.com", Age: 25},
        {Id: 2, Name: "用户2", Email: "user2@test.com", Age: 30}, 
        {Id: 3, Name: "用户3", Email: "user3@test.com", Age: 35},
    }
    
    var users []User
    
    // 逐个转换并监控
    for i, pbUser := range testUsers {
        var user User
        if err := converter.ConvertPBToModelWithLog(pbUser, &user); err != nil {
            logger.Error("转换失败 [%d]: %v", i, err)
            continue
        }
        users = append(users, user)
    }
    
    // 查看性能指标
    metrics := converter.GetMetrics()
    
    fmt.Printf("=== 转换统计 ===\n")
    fmt.Printf("总转换次数: %d\n", metrics.TotalConversions)
    fmt.Printf("成功次数: %d\n", metrics.SuccessfulConversions)
    fmt.Printf("失败次数: %d\n", metrics.FailedConversions) 
    fmt.Printf("成功率: %.2f%%\n", 
        float64(metrics.SuccessfulConversions)/float64(metrics.TotalConversions)*100)
    fmt.Printf("平均耗时: %v\n", metrics.AverageDuration)
    
    if metrics.LastError != nil {
        fmt.Printf("最后错误: %v\n", metrics.LastError)
    }
    
    // 定期报告指标
    converter.ReportMetrics()
}

// gRPC 服务中的使用示例
type UserService struct {
    pb.UnimplementedUserServiceServer
    converter *pbmo.EnhancedBidiConverter
    logger    logger.ILogger
}

func NewUserService(logger logger.ILogger) *UserService {
    return &UserService{
        converter: pbmo.NewEnhancedBidiConverter(&pb.User{}, &User{}, logger),
        logger:    logger,
    }
}

func (s *UserService) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
    // 转换请求
    var user User
    if err := s.converter.ConvertPBToModelWithLog(req.User, &user); err != nil {
        s.logger.Error("创建用户转换失败: %v", err)
        return nil, err  // 已经是 gRPC status error
    }
    
    // 保存到数据库
    // db.Create(&user)
    
    // 转换响应
    var pbUser pb.User
    if err := s.converter.ConvertModelToPBWithLog(&user, &pbUser); err != nil {
        s.logger.Error("响应转换失败: %v", err)
        return nil, err
    }
    
    return &pbUser, nil
}
```

### 🎯 自定义转换器

```go
func CustomTransformerExample() {
    converter := pbmo.NewEnhancedBidiConverter(&pb.Product{}, &Product{}, logger)
    
    // 注册自定义字段转换器
    converter.RegisterTransformer("Price", func(v interface{}) interface{} {
        // 将分转换为元
        if cents, ok := v.(int64); ok {
            return float64(cents) / 100.0
        }
        // 将元转换为分
        if yuan, ok := v.(float64); ok {
            return int64(yuan * 100)
        }
        return v
    })
    
    // 测试自定义转换
    pbProduct := &pb.Product{
        Id:          789,
        Title:       "测试商品",
        Description: "这是一个测试商品",
        PriceCents:  1999, // 19.99 元，以分为单位
        InStock:     true,
    }
    
    var product Product
    if err := converter.ConvertPBToModelWithLog(pbProduct, &product); err != nil {
        fmt.Printf("自定义转换失败: %v\n", err)
        return
    }
    
    fmt.Printf("自定义转换结果: %+v\n", product)
    fmt.Printf("价格转换: %d分 → %.2f元\n", pbProduct.PriceCents, product.Price)
}
```

## 安全转换器示例

### 🛡️ 处理复杂嵌套和 nil 指针

```go
// 复杂嵌套结构
type Address struct {
    ID       uint   `gorm:"primarykey"`
    Street   string `gorm:"size:200"`
    City     string `gorm:"size:100"`
    Province string `gorm:"size:100"`
    Country  string `gorm:"size:100"`
    ZipCode  string `gorm:"size:20"`
}

type Company struct {
    ID      uint    `gorm:"primarykey"`
    Name    string  `gorm:"size:200"`
    Address *Address `gorm:"foreignKey:CompanyID"`
}

type Employee struct {
    ID        uint     `gorm:"primarykey"`
    Name      string   `gorm:"size:100"`
    Company   *Company `gorm:"foreignKey:CompanyID"`
    HomeAddr  *Address `gorm:"foreignKey:HomeAddressID"`
    WorkAddr  *Address `gorm:"foreignKey:WorkAddressID"`
}

func SafeConverterExample() {
    // 创建安全转换器
    converter := pbmo.NewSafeConverter(&pb.Employee{}, &Employee{})
    
    // 测试部分数据缺失的情况
    pbEmployee := &pb.Employee{
        Id:   1001,
        Name: "安全测试员工",
        Company: &pb.Company{
            Id:   2001,
            Name: "测试公司",
            // Address 故意为 nil
        },
        // HomeAddr 故意为 nil
        WorkAddr: &pb.Address{
            Street:   "工作街道123号",
            City:     "工作城市",
            Province: "工作省份",
            Country:  "中国",
            ZipCode:  "100001",
        },
    }
    
    // 安全转换
    var employee Employee
    if err := converter.SafeConvertPBToModel(pbEmployee, &employee); err != nil {
        fmt.Printf("安全转换失败: %v\n", err)
        return
    }
    
    fmt.Printf("安全转换成功: %+v\n", employee)
    
    // 使用链式安全访问
    cityValue := converter.SafeFieldAccess(pbEmployee, "Company", "Address", "City")
    if cityValue.IsValid() {
        fmt.Printf("公司城市: %s\n", cityValue.String("未知"))
    } else {
        fmt.Printf("公司地址信息不完整，无法获取城市\n")
    }
    
    // 测试工作地址（应该存在）
    workCityValue := converter.SafeFieldAccess(pbEmployee, "WorkAddr", "City")
    if workCityValue.IsValid() {
        fmt.Printf("工作城市: %s\n", workCityValue.String("未知"))
    }
    
    // 测试不存在的字段路径
    phoneValue := converter.SafeFieldAccess(pbEmployee, "Contact", "Phone")
    fmt.Printf("电话号码存在: %t\n", phoneValue.IsValid())
}

// 安全批量转换示例
func SafeBatchConverterExample() {
    converter := pbmo.NewSafeConverter(&pb.User{}, &User{})
    
    // 测试数据（包含一些有问题的数据）
    pbUsers := []*pb.User{
        {Id: 1, Name: "正常用户1", Email: "user1@test.com", Age: 25},
        nil, // nil 用户
        {Id: 2, Name: "正常用户2", Email: "user2@test.com", Age: 30},
        {Id: 3, Name: "", Email: "invalid", Age: -5}, // 无效数据
        {Id: 4, Name: "正常用户4", Email: "user4@test.com", Age: 35},
    }
    
    var users []User
    result := converter.SafeBatchConvertPBToModel(pbUsers, &users)
    
    fmt.Printf("=== 安全批量转换结果 ===\n")
    fmt.Printf("总数: %d\n", len(pbUsers))
    fmt.Printf("成功: %d\n", result.SuccessCount)
    fmt.Printf("失败: %d\n", result.FailureCount)
    
    // 查看详细结果
    for _, item := range result.Results {
        if item.Success {
            user := item.Value.(*User)
            fmt.Printf("✅ [%d] 成功: %s\n", item.Index, user.Name)
        } else {
            fmt.Printf("❌ [%d] 失败: %v\n", item.Index, item.Error)
        }
    }
    
    fmt.Printf("成功转换的用户数: %d\n", len(users))
}
```

## 服务集成示例

### 🔧 完整的 gRPC 服务集成

```go
package service

import (
    "context"
    "github.com/kamalyes/go-rpc-gateway/pbmo"
    "github.com/kamalyes/go-logger"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

type UserManagementService struct {
    pb.UnimplementedUserManagementServiceServer
    userService      *pbmo.ServiceIntegration
    profileService   *pbmo.ServiceIntegration
    logger           logger.ILogger
}

func NewUserManagementService(logger logger.ILogger) *UserManagementService {
    service := &UserManagementService{
        logger: logger,
    }
    
    // 创建用户服务集成
    service.userService = pbmo.NewServiceIntegration(
        &pb.User{}, &User{}, logger,
    )
    
    // 注册用户校验规则
    service.userService.RegisterValidationRules("User",
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
        pbmo.FieldRule{
            Name: "Age",
            Min:  1,
            Max:  150,
        },
    )
    
    // 创建档案服务集成
    service.profileService = pbmo.NewServiceIntegration(
        &pb.UserProfile{}, &UserProfile{}, logger,
    )
    
    // 注册档案校验规则
    service.profileService.RegisterValidationRules("UserProfile",
        pbmo.FieldRule{
            Name:   "Bio",
            MaxLen: 1000,
        },
        pbmo.FieldRule{
            Name:    "Website",
            Pattern: `^https?://[^\s]+$`,
        },
    )
    
    return service
}

// 创建用户
func (s *UserManagementService) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
    var user User
    
    // 转换并校验用户数据
    if err := s.userService.ConvertAndValidatePBToModel(req.User, &user); err != nil {
        s.logger.Error("创建用户数据转换失败: %v", err)
        return nil, err // 已经是 gRPC error
    }
    
    // 保存到数据库
    if err := s.saveUserToDB(&user); err != nil {
        return nil, s.userService.HandleError(err, "CreateUser.SaveDB")
    }
    
    // 转换响应
    var pbUser pb.User
    if err := s.userService.ConvertModelToPBWithLog(&user, &pbUser); err != nil {
        return nil, err
    }
    
    return &pb.CreateUserResponse{User: &pbUser}, nil
}

// 批量创建用户
func (s *UserManagementService) BatchCreateUsers(ctx context.Context, req *pb.BatchCreateUsersRequest) (*pb.BatchCreateUsersResponse, error) {
    var users []User
    var results []*pb.BatchCreateResult
    
    // 批量转换
    for i, pbUser := range req.Users {
        var user User
        result := &pb.BatchCreateResult{
            Index: int32(i),
        }
        
        // 转换并校验
        if err := s.userService.ConvertAndValidatePBToModel(pbUser, &user); err != nil {
            result.Success = false
            result.ErrorMessage = err.Error()
            results = append(results, result)
            continue
        }
        
        // 保存到数据库
        if err := s.saveUserToDB(&user); err != nil {
            result.Success = false  
            result.ErrorMessage = err.Error()
            results = append(results, result)
            continue
        }
        
        // 转换响应
        var pbUserResult pb.User
        if err := s.userService.ConvertModelToPBWithLog(&user, &pbUserResult); err != nil {
            result.Success = false
            result.ErrorMessage = err.Error()
            results = append(results, result)
            continue
        }
        
        result.Success = true
        result.User = &pbUserResult
        results = append(results, result)
        users = append(users, user)
    }
    
    return &pb.BatchCreateUsersResponse{
        Results:      results,
        SuccessCount: int32(len(users)),
        TotalCount:   int32(len(req.Users)),
    }, nil
}

// 更新用户档案
func (s *UserManagementService) UpdateUserProfile(ctx context.Context, req *pb.UpdateUserProfileRequest) (*pb.UpdateUserProfileResponse, error) {
    var profile UserProfile
    
    // 转换并校验档案数据
    if err := s.profileService.ConvertAndValidatePBToModel(req.Profile, &profile); err != nil {
        return nil, err
    }
    
    // 更新数据库
    if err := s.updateProfileInDB(&profile); err != nil {
        return nil, s.profileService.HandleError(err, "UpdateUserProfile.UpdateDB")
    }
    
    // 转换响应
    var pbProfile pb.UserProfile
    if err := s.profileService.ConvertModelToPBWithLog(&profile, &pbProfile); err != nil {
        return nil, err
    }
    
    return &pb.UpdateUserProfileResponse{Profile: &pbProfile}, nil
}

// 辅助方法
func (s *UserManagementService) saveUserToDB(user *User) error {
    // 模拟数据库保存
    s.logger.Info("保存用户到数据库: %s", user.Name)
    return nil
}

func (s *UserManagementService) updateProfileInDB(profile *UserProfile) error {
    // 模拟数据库更新
    s.logger.Info("更新用户档案: %s", profile.Bio)
    return nil
}

// 定期报告性能指标
func (s *UserManagementService) ReportMetrics() {
    s.userService.ReportMetrics()
    s.profileService.ReportMetrics()
}
```

## 批量转换示例

### 📦 高效批量处理

```go
func BatchConversionExamples() {
    converter := pbmo.NewBidiConverter(&pb.User{}, &User{})
    
    // 生成测试数据
    testPBUsers := make([]*pb.User, 1000)
    for i := 0; i < 1000; i++ {
        testPBUsers[i] = &pb.User{
            Id:       uint64(i + 1),
            Name:     fmt.Sprintf("用户%d", i+1),
            Email:    fmt.Sprintf("user%d@test.com", i+1),
            Age:      int32(20 + i%50),
            IsActive: i%2 == 0,
        }
    }
    
    fmt.Printf("准备批量转换 %d 个用户...\n", len(testPBUsers))
    
    // 方式1: 标准批量转换
    start := time.Now()
    var users1 []User
    if err := converter.BatchConvertPBToModel(testPBUsers, &users1); err != nil {
        fmt.Printf("批量转换失败: %v\n", err)
        return
    }
    duration1 := time.Since(start)
    fmt.Printf("标准批量转换: %d个用户, 耗时: %v\n", len(users1), duration1)
    
    // 方式2: 增强转换器批量转换
    enhancedConverter := pbmo.NewEnhancedBidiConverter(&pb.User{}, &User{}, logger)
    
    start = time.Now()
    var users2 []User
    result := enhancedConverter.ConvertPBToModelBatchSafe(testPBUsers, &users2)
    duration2 := time.Since(start)
    
    fmt.Printf("增强批量转换: 成功%d个, 失败%d个, 耗时: %v\n", 
        result.SuccessCount, result.FailureCount, duration2)
    
    // 方式3: 安全转换器批量转换
    safeConverter := pbmo.NewSafeConverter(&pb.User{}, &User{})
    
    start = time.Now()
    var users3 []User
    safeResult := safeConverter.SafeBatchConvertPBToModel(testPBUsers, &users3)
    duration3 := time.Since(start)
    
    fmt.Printf("安全批量转换: 成功%d个, 失败%d个, 耗时: %v\n",
        safeResult.SuccessCount, safeResult.FailureCount, duration3)
    
    // 性能比较
    fmt.Printf("\\n=== 性能比较 ===\\n")
    fmt.Printf("标准转换: %.2f ns/op\\n", float64(duration1.Nanoseconds())/float64(len(testPBUsers)))
    fmt.Printf("增强转换: %.2f ns/op\\n", float64(duration2.Nanoseconds())/float64(len(testPBUsers)))
    fmt.Printf("安全转换: %.2f ns/op\\n", float64(duration3.Nanoseconds())/float64(len(testPBUsers)))
}
```

## 性能优化示例

### ⚡ 转换器重用和预分配

```go
// 转换器池管理
type ConverterPool struct {
    userConverter    *pbmo.BidiConverter
    productConverter *pbmo.BidiConverter
    orderConverter   *pbmo.EnhancedBidiConverter
}

func NewConverterPool(logger logger.ILogger) *ConverterPool {
    return &ConverterPool{
        userConverter:    pbmo.NewBidiConverter(&pb.User{}, &User{}),
        productConverter: pbmo.NewBidiConverter(&pb.Product{}, &Product{}),
        orderConverter:   pbmo.NewEnhancedBidiConverter(&pb.Order{}, &Order{}, logger),
    }
}

// 高性能批量转换
func (cp *ConverterPool) HighPerformanceBatchConvert() {
    const batchSize = 10000
    
    // 预分配切片容量
    users := make([]User, 0, batchSize)
    products := make([]Product, 0, batchSize)
    
    // 生成测试数据
    pbUsers := make([]*pb.User, batchSize)
    pbProducts := make([]*pb.Product, batchSize)
    
    for i := 0; i < batchSize; i++ {
        pbUsers[i] = &pb.User{
            Id:   uint64(i),
            Name: fmt.Sprintf("User%d", i),
        }
        pbProducts[i] = &pb.Product{
            Id:    uint64(i),
            Title: fmt.Sprintf("Product%d", i),
        }
    }
    
    // 基准测试
    start := time.Now()
    
    // 并发转换
    var wg sync.WaitGroup
    wg.Add(2)
    
    go func() {
        defer wg.Done()
        cp.userConverter.BatchConvertPBToModel(pbUsers, &users)
    }()
    
    go func() {
        defer wg.Done()
        cp.productConverter.BatchConvertPBToModel(pbProducts, &products)
    }()
    
    wg.Wait()
    
    duration := time.Since(start)
    totalOps := len(pbUsers) + len(pbProducts)
    
    fmt.Printf("并发批量转换: %d个对象, 耗时: %v\n", totalOps, duration)
    fmt.Printf("平均性能: %.2f ns/op\n", float64(duration.Nanoseconds())/float64(totalOps))
}

// 内存优化示例
func MemoryOptimizedConversion() {
    converter := pbmo.NewBidiConverter(&pb.User{}, &User{})
    
    const totalUsers = 100000
    const batchSize = 1000
    
    fmt.Printf("开始内存优化批量转换 %d 个用户...\n", totalUsers)
    
    var totalProcessed int
    start := time.Now()
    
    // 分批处理，避免内存占用过大
    for i := 0; i < totalUsers; i += batchSize {
        end := i + batchSize
        if end > totalUsers {
            end = totalUsers
        }
        
        // 生成这一批的测试数据
        batchPBUsers := make([]*pb.User, end-i)
        for j := 0; j < end-i; j++ {
            batchPBUsers[j] = &pb.User{
                Id:   uint64(i + j),
                Name: fmt.Sprintf("BatchUser%d", i+j),
            }
        }
        
        // 转换这一批
        var batchUsers []User
        if err := converter.BatchConvertPBToModel(batchPBUsers, &batchUsers); err != nil {
            fmt.Printf("批次转换失败: %v\n", err)
            continue
        }
        
        totalProcessed += len(batchUsers)
        
        // 模拟处理（这里可以保存到数据库等）
        // processBatch(batchUsers)
        
        if (i+batchSize)%10000 == 0 {
            fmt.Printf("已处理: %d/%d (%.1f%%)\n", 
                totalProcessed, totalUsers, 
                float64(totalProcessed)/float64(totalUsers)*100)
        }
    }
    
    duration := time.Since(start)
    fmt.Printf("内存优化转换完成: %d个用户, 耗时: %v\n", totalProcessed, duration)
    fmt.Printf("平均性能: %.2f ns/op\n", float64(duration.Nanoseconds())/float64(totalProcessed))
}
```

## 实际项目集成

### 🏭 完整的微服务项目示例

```go
// main.go
package main

import (
    "context"
    "net"
    "net/http"
    
    "github.com/kamalyes/go-rpc-gateway"
    "github.com/kamalyes/go-rpc-gateway/pbmo"
    "github.com/kamalyes/go-logger"
    "google.golang.org/grpc"
)

type Application struct {
    gateway        *gateway.Gateway
    userService    *UserService
    productService *ProductService
    logger         logger.ILogger
}

func NewApplication() *Application {
    // 创建日志
    logger := logger.NewLogger(
        logger.WithLevel(logger.InfoLevel),
        logger.WithConsole(true),
        logger.WithFile("logs/app.log"),
    )
    
    // 创建网关
    gw, err := gateway.NewGateway().
        WithConfigPath("config/gateway.yaml").
        WithEnvironment(gateway.EnvDevelopment).
        WithHotReload(nil).
        Build()
    
    if err != nil {
        logger.Fatal("创建网关失败: %v", err)
    }
    
    return &Application{
        gateway:        gw,
        userService:    NewUserService(logger),
        productService: NewProductService(logger),
        logger:         logger,
    }
}

func (app *Application) Run() error {
    // 注册 gRPC 服务
    app.gateway.RegisterService(func(s *grpc.Server) {
        pb.RegisterUserServiceServer(s, app.userService)
        pb.RegisterProductServiceServer(s, app.productService)
    })
    
    // 注册 HTTP 路由
    app.gateway.RegisterHTTPRoutes(map[string]http.HandlerFunc{
        "/api/health": app.healthCheck,
        "/api/metrics": app.metricsHandler,
    })
    
    // 启动定期指标报告
    go app.startMetricsReporting()
    
    // 启动服务
    app.logger.Info("应用启动成功")
    return app.gateway.Start()
}

func (app *Application) healthCheck(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}

func (app *Application) metricsHandler(w http.ResponseWriter, r *http.Request) {
    // 收集转换器指标
    userMetrics := app.userService.GetMetrics()
    productMetrics := app.productService.GetMetrics()
    
    response := map[string]interface{}{
        "user_service": map[string]interface{}{
            "total_conversions":      userMetrics.TotalConversions,
            "successful_conversions": userMetrics.SuccessfulConversions,
            "failed_conversions":     userMetrics.FailedConversions,
            "average_duration":       userMetrics.AverageDuration.String(),
        },
        "product_service": map[string]interface{}{
            "total_conversions":      productMetrics.TotalConversions,
            "successful_conversions": productMetrics.SuccessfulConversions,
            "failed_conversions":     productMetrics.FailedConversions,
            "average_duration":       productMetrics.AverageDuration.String(),
        },
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func (app *Application) startMetricsReporting() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            app.userService.ReportMetrics()
            app.productService.ReportMetrics()
        }
    }
}

func main() {
    app := NewApplication()
    if err := app.Run(); err != nil {
        app.logger.Fatal("应用运行失败: %v", err)
    }
}
```

这个示例大全展示了 PBMO 的各种使用场景，从简单的基础转换到复杂的生产级集成。每个示例都包含完整的代码和详细说明，可以直接在项目中使用或作为参考。