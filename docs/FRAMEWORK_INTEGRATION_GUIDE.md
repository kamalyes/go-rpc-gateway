# go-rpc-gateway 框架集成指南

## 概述

本文档说明如何在 `go-rpc-gateway` 中集成自动 PB ↔ GORM Model 转换系统，实现零代码侵入的服务集成。

## 核心特性

✅ **自动转换** - 框架层面自动处理 PB ↔ GORM Model 转换  
✅ **零侵入** - 服务实现只需关注业务逻辑  
✅ **灵活扩展** - 支持自定义转换器和字段映射  
✅ **类型安全** - 编译期类型检查，运行时反射处理  
✅ **流支持** - 流式 gRPC 服务也支持自动转换  

## 架构图

```
┌─────────────────────────────────────────┐
│         gRPC 客户端请求                  │
└────────────────┬────────────────────────┘
                 │
                 ▼
    ┌────────────────────────────┐
    │ gRPC 一元/流拦截器         │
    │ (UnaryServerInterceptor)   │
    └────────────┬───────────────┘
                 │
                 ▼
    ┌────────────────────────────┐
    │ 自动 PB -> Model 转换       │
    │ (AutoModelConverter)       │
    └────────────┬───────────────┘
                 │
                 ▼
    ┌────────────────────────────┐
    │ gRPC 服务实现               │
    │ (仅处理业务逻辑)            │
    └────────────┬───────────────┘
                 │
                 ▼
    ┌────────────────────────────┐
    │ 自动 Model -> PB 转换       │
    │ (AutoModelConverter)       │
    └────────────┬───────────────┘
                 │
                 ▼
┌─────────────────────────────────────────┐
│      返回 PB 响应给客户端                 │
└─────────────────────────────────────────┘
```

## 集成步骤

### 步骤 1：在服务器初始化时启用中间件

在 `server/core.go` 或 `gateway.go` 中：

```go
import (
    "github.com/kamalyes/go-rpc-gateway/middleware"
)

// 配置转换中间件
conversionConfig := middleware.ConversionConfig{
    Enabled:        true,
    LogConversions: true,  // 开发环境启用日志，生产环境关闭
}

// 创建拦截器
unaryInterceptor := middleware.AutoModelConverterInterceptor(conversionConfig, logger)
streamInterceptor := middleware.StreamModelConverterInterceptor(conversionConfig, logger)

// 在 gRPC 服务器初始化时使用
opts := []grpc.ServerOption{
    grpc.UnaryInterceptor(unaryInterceptor),
    grpc.StreamInterceptor(streamInterceptor),
}

grpcServer := grpc.NewServer(opts...)
```

### 步骤 2：定义 Proto 文件

使用标准 Protocol Buffers 定义：

```protobuf
syntax = "proto3";
package link.v1;

import "google/api/annotations.proto";
import "google/protobuf/timestamp.proto";

message Link {
    int64 id = 1;
    string url = 2;
    string title = 3;
    string description = 4;
    google.protobuf.Timestamp created_at = 5;
    google.protobuf.Timestamp updated_at = 6;
}

message CreateLinkRequest {
    string url = 1;
    string title = 2;
    string description = 3;
}

message CreateLinkResponse {
    Link link = 1;
}

service LinkService {
    rpc CreateLink(CreateLinkRequest) returns (CreateLinkResponse) {
        option (google.api.http) = {
            post: "/v1/links"
            body: "*"
        };
    }
}
```

### 步骤 3：定义 GORM 模型

与 Proto 结构对应：

```go
// Link GORM 模型
type Link struct {
    ID          uint      `gorm:"primaryKey"`
    URL         string
    Title       string
    Description string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// TableName 指定表名
func (Link) TableName() string {
    return "links"
}
```

### 步骤 4：实现 gRPC 服务

**重要：服务实现只需处理业务逻辑，转换由框架自动处理**

```go
type LinkServiceServer struct {
    db *gorm.DB
    pb.UnimplementedLinkServiceServer
}

// 创建链接 - 框架自动处理 PB ↔ Model 转换
func (s *LinkServiceServer) CreateLink(ctx context.Context, req *pb.CreateLinkRequest) (*pb.CreateLinkResponse, error) {
    // 业务逻辑：验证、存储数据库等
    
    // 创建 GORM 模型
    link := &Link{
        URL:         req.GetUrl(),
        Title:       req.GetTitle(),
        Description: req.GetDescription(),
    }
    
    // 保存到数据库
    if err := s.db.Create(link).Error; err != nil {
        return nil, status.Errorf(codes.Internal, "failed to create link: %v", err)
    }
    
    // 返回模型 - 框架自动转换为 PB
    // 注意：这里返回的是 *pb.CreateLinkResponse，框架自动处理转换
    pbLink := &pb.Link{
        Id:          int64(link.ID),
        Url:         link.URL,
        Title:       link.Title,
        Description: link.Description,
        CreatedAt:   timestamppb.New(link.CreatedAt),
        UpdatedAt:   timestamppb.New(link.UpdatedAt),
    }
    
    return &pb.CreateLinkResponse{Link: pbLink}, nil
}
```

### 步骤 5：注册服务

在 `gateway.go` 中：

```go
// 初始化服务
linkService := &LinkServiceServer{
    db: dbPool,
}

// 注册 gRPC 服务 - 框架自动处理所有转换
gateway.RegisterService(func(server *grpc.Server) {
    pb.RegisterLinkServiceServer(server, linkService)
})
```

## 自动转换支持的类型

### 基本类型转换

| PB 类型 | GORM 类型 | 说明 |
|--------|----------|------|
| `string` | `string` | 直接赋值 |
| `int32/int64` | `uint` | ID 字段自动转换 |
| `bool` | `bool` | 直接赋值 |
| `float/double` | `float64` | 直接赋值 |
| `bytes` | `[]byte` | 直接赋值 |

### 特殊类型转换

| PB 类型 | GORM 类型 | 说明 |
|--------|----------|------|
| `google.protobuf.Timestamp` | `time.Time` | 自动双向转换 |
| `google.protobuf.Duration` | `time.Duration` | 自动双向转换 |
| `google.protobuf.StringValue` | `*string` | 指针处理 |
| 嵌套 Message | 嵌套结构体 | 递归转换 |
| `repeated T` | `[]T` | 切片转换 |

### 自定义字段映射

对于字段名不匹配的情况，使用标签：

```go
type User struct {
    ID        uint   `pb:"user_id"`      // PB 字段名：user_id
    FirstName string `pb:"first_name"`
    LastName  string `pb:"last_name"`
}
```

## 高级用法

### 自定义转换器

对于特殊需求，实现 `ModelConverter` 接口：

```go
// 实现接口
type Link struct {
    ID          uint
    URL         string
    // ...
}

func (l *Link) ToPB() interface{} {
    return &pb.Link{
        Id:  int64(l.ID),
        Url: l.URL,
        // ...
    }
}

func (l *Link) FromPB(pb interface{}) error {
    pbLink := pb.(*pb.Link)
    l.ID = uint(pbLink.Id)
    l.URL = pbLink.Url
    // ...
    return nil
}
```

### 注册表方式

对于更复杂的场景，使用 `ConversionRegistry`：

```go
// 创建注册表
registry := middleware.NewConversionRegistry(logger)

// 注册自定义转换器
registry.RegisterPBToModelConverter("CreateLinkRequest", func(pb interface{}) (interface{}, error) {
    req := pb.(*pb.CreateLinkRequest)
    return &Link{
        URL:         req.Url,
        Title:       req.Title,
        Description: req.Description,
    }, nil
})

registry.RegisterModelToPBConverter("Link", func(model interface{}) (interface{}, error) {
    link := model.(*Link)
    return &pb.Link{
        Id:    int64(link.ID),
        Url:   link.URL,
        Title: link.Title,
        // ...
    }, nil
})
```

## 完整集成示例

### 1. Proto 定义

```protobuf
syntax = "proto3";
package product.v1;

import "google/api/annotations.proto";
import "google/protobuf/timestamp.proto";

message Product {
    int64 id = 1;
    string name = 2;
    string description = 3;
    double price = 4;
    int32 stock = 5;
    google.protobuf.Timestamp created_at = 6;
}

message CreateProductRequest {
    string name = 1;
    string description = 2;
    double price = 3;
    int32 stock = 4;
}

message CreateProductResponse {
    Product product = 1;
}

service ProductService {
    rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse) {
        option (google.api.http) = {
            post: "/v1/products"
            body: "*"
        };
    }
}
```

### 2. GORM 模型

```go
type Product struct {
    ID          uint      `gorm:"primaryKey"`
    Name        string
    Description string
    Price       float64
    Stock       int32
    CreatedAt   time.Time
}

func (Product) TableName() string {
    return "products"
}
```

### 3. 服务实现

```go
type ProductServiceServer struct {
    db *gorm.DB
    pb.UnimplementedProductServiceServer
}

func (s *ProductServiceServer) CreateProduct(ctx context.Context, 
    req *pb.CreateProductRequest) (*pb.CreateProductResponse, error) {
    
    product := &Product{
        Name:        req.GetName(),
        Description: req.GetDescription(),
        Price:       req.GetPrice(),
        Stock:       req.GetStock(),
    }
    
    if err := s.db.Create(product).Error; err != nil {
        return nil, status.Errorf(codes.Internal, "failed to create product: %v", err)
    }
    
    // 框架自动转换响应
    pbProduct := &pb.Product{
        Id:          int64(product.ID),
        Name:        product.Name,
        Description: product.Description,
        Price:       product.Price,
        Stock:       product.Stock,
        CreatedAt:   timestamppb.New(product.CreatedAt),
    }
    
    return &pb.CreateProductResponse{Product: pbProduct}, nil
}
```

### 4. 服务注册

```go
// 在 main.go 或初始化代码中
func setupGateway() {
    // 初始化服务
    productService := &ProductServiceServer{
        db: dbPool,
    }
    
    // 启用自动转换中间件
    convConfig := middleware.ConversionConfig{
        Enabled:        true,
        LogConversions: true,
    }
    
    unaryInterceptor := middleware.AutoModelConverterInterceptor(convConfig, logger)
    
    opts := []grpc.ServerOption{
        grpc.UnaryInterceptor(unaryInterceptor),
    }
    
    grpcServer := grpc.NewServer(opts...)
    
    // 注册服务
    pb.RegisterProductServiceServer(grpcServer, productService)
}
```

## 常见问题

### Q: 字段名称不匹配怎么办？

A: 使用标签或自定义转换器：

```go
type User struct {
    UserID    uint   `pb:"id"`           // 标签映射
    FirstName string                    // 自动驼峰转换
}
```

### Q: 如何处理复杂的业务逻辑转换？

A: 实现 `ModelConverter` 接口：

```go
func (u *User) ToPB() interface{} {
    // 自定义逻辑
    return &pb.User{
        Id:        int64(u.UserID),
        FirstName: u.FirstName,
    }
}
```

### Q: 流式 gRPC 服务如何转换？

A: 框架自动处理，无需额外代码。流拦截器自动处理消息转换。

### Q: 转换失败时会怎样？

A: 框架记录警告日志，返回原始值。使用日志级别 `DEBUG` 诊断转换问题。

## 性能考虑

- **反射开销**：使用 `sync.Map` 缓存类型信息减少反射开销
- **批量操作**：使用 `ConvertList()` 进行批量转换
- **日志级别**：生产环境设置 `LogConversions: false` 避免日志开销

## 最佳实践

1. **保持 Proto 简单** - 复杂的转换逻辑应该在服务中实现
2. **使用 Proto 作为契约** - 不要手动修改生成的 PB 代码
3. **遵循命名规范** - 使用驼峰命名确保自动转换工作
4. **测试转换** - 编写单元测试验证转换逻辑
5. **监控性能** - 监控转换延迟，必要时使用自定义转换器优化

## 下一步

- [Auto-Converter API 参考](../utils/converters/auto_converter.go)
- [中间件配置参考](../middleware/pb_model_converter.go)
- [服务集成示例](ADSLINK_INTEGRATION_EXAMPLE.md)
