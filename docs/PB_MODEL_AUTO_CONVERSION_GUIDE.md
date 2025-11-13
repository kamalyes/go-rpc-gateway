# PB ↔ GORM Model 自动转换系统

## 🎯 核心理念

**零额外代码自动处理 PB 和 GORM Model 之间的转换**

```
请求到达
  ↓
自动 PB → GORM Model (转换器自动处理)
  ↓
Service 处理 (只需关心业务逻辑)
  ↓
自动 GORM Model → PB (转换器自动处理)
  ↓
响应返回
```

---

## 📋 设计原理

### 1. 字段自动匹配
- 同名字段自动匹配（Model.ID ↔ PB.id）
- 支持通过 `pb` tag 自定义映射
- 支持 snake_case 和 CamelCase 自动转换

### 2. 类型自动转换
- ✅ `time.Time` ↔ `*timestamppb.Timestamp`
- ✅ `int/int32/int64` 互相转换
- ✅ `float32/float64` 互相转换
- ✅ `string/[]byte` 互相转换
- ✅ 嵌套结构自动递归转换
- ✅ `*Type` ↔ `Type` 自动处理

### 3. 自动类型检测
- 反射自动检测源和目标类型
- 智能选择合适的转换策略
- 无需手动类型声明

---

## 🚀 使用方式

### 方式 1：直接使用转换函数（最简单）

**定义 GORM Model：**
```go
// go-rpc-gateway/services/adslink/models/link.go
package models

import (
	"time"
	"gorm.io/gorm"
)

type LinkModel struct {
	ID          uint           `gorm:"primaryKey" pb:"id"`
	URL         string         `gorm:"index" pb:"url"`
	ShortCode   string         `gorm:"unique" pb:"short_code"`
	Title       string         `pb:"title"`
	Description string         `pb:"description"`
	ClickCount  int64          `pb:"click_count"`
	CreatedAt   time.Time      `pb:"created_at"`
	UpdatedAt   time.Time      `pb:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" pb:"-"` // 忽略此字段
}

func (LinkModel) TableName() string {
	return "links"
}
```

**gRPC 服务实现（无需手动转换）：**
```go
// go-rpc-gateway/services/adslink/grpc/server.go
package grpc

import (
	"context"
	pb "github.com/Divine-Dragon-Voyage/commonapis/pb"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/kamalyes/go-rpc-gateway/services/adslink/models"
	"github.com/kamalyes/go-rpc-gateway/services/adslink/service"
	"github.com/kamalyes/go-rpc-gateway/utils/converters"
)

type LinkServiceServer struct {
	pb.UnimplementedLinkServiceServer
	svc service.LinkService
}

// CreateLink - 无需手动转换！
func (s *LinkServiceServer) CreateLink(ctx context.Context, req *pb.CreateLinkRequest) (*pb.CreateLinkResponse, error) {
	// 请求自动转换：PB → GORM Model
	link := &models.LinkModel{}
	if err := converters.ConvertPBToModel(req, link); err != nil {
		return nil, err
	}

	// 业务逻辑处理
	result, err := s.svc.CreateLink(ctx, link)
	if err != nil {
		return nil, err
	}

	// 响应自动转换：GORM Model → PB
	pbResp := &pb.CreateLinkResponse{}
	if err := converters.ConvertModelToPB(result, pbResp); err != nil {
		return nil, err
	}

	return pbResp, nil
}

// GetLink
func (s *LinkServiceServer) GetLink(ctx context.Context, req *pb.GetLinkRequest) (*pb.LinkResponse, error) {
	link, err := s.svc.GetLink(ctx, req.ShortCode)
	if err != nil {
		return nil, err
	}

	// 只需一行代码转换
	pbResp := &pb.LinkResponse{}
	converters.ConvertModelToPB(link, pbResp)

	return pbResp, nil
}

// ListLinks
func (s *LinkServiceServer) ListLinks(ctx context.Context, req *pb.ListLinksRequest) (*pb.ListLinksResponse, error) {
	models, total, err := s.svc.ListLinks(ctx, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, err
	}

	// 批量转换 Model → PB
	pbModels := make([]*pb.LinkResponse, len(models))
	for i, m := range models {
		pbModels[i] = &pb.LinkResponse{}
		converters.ConvertModelToPB(m, pbModels[i])
	}

	return &pb.ListLinksResponse{
		Links:    pbModels,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}
```

**就这么简单！无需手动写转换代码**

---

### 方式 2：实现 ModelConverter 接口（自动化）

**定义带转换能力的 Model：**
```go
// 可选：实现 ModelConverter 接口以获得自动转换
func (l *LinkModel) ToPB() interface{} {
	pbResp := &pb.LinkResponse{}
	converters.ConvertModelToPB(l, pbResp)
	return pbResp
}

func (l *LinkModel) FromPB(pb interface{}) error {
	pbReq := pb.(*pb.CreateLinkRequest)
	return converters.ConvertPBToModel(pbReq, l)
}
```

**拦截器会自动使用：**
```go
// 注册拦截器到 gateway
gw.RegisterService(func(s *grpc.Server) {
	// 自动添加转换拦截器
	s.ChainUnaryInterceptor(
		middleware.PBModelConverterInterceptor(global.LOGGER),
	)

	pb.RegisterLinkServiceServer(s, linkServer)
})
```

---

### 方式 3：使用通用转换助手（推荐）

```go
// 在 service layer 使用
converter := middleware.NewUniversalConverter(global.LOGGER)

// 请求转换
pbReq := req  // Protocol Buffer request
linkModel := &models.LinkModel{}
if err := converters.ConvertPBToModel(pbReq, linkModel); err != nil {
	return nil, err
}

// 业务处理
result, err := svc.CreateLink(ctx, linkModel)

// 响应转换
pbResp, err := converter.ConvertResponse(result, &pb.CreateLinkResponse{})
```

---

## 🔧 高级配置

### 自定义字段映射

```go
// 当 Model 和 PB 字段名不同时
type UserModel struct {
	ID        uint   `gorm:"primaryKey" pb:"user_id"`    // pb field 名为 user_id
	FullName  string `pb:"full_name"`
	CreatedAt time.Time
}

// 自动转换会优先使用 pb tag 中指定的字段名
```

### 自定义类型转换

```go
// 注册特殊类型转换函数
autoConverter := converters.NewAutoConverter()
autoConverter.RegisterTypeConverter("CustomType", func(v interface{}) interface{} {
	// 自定义转换逻辑
	return transformCustomType(v)
})
```

### 忽略字段

```go
type LinkModel struct {
	ID    uint   `pb:"-"`  // 忽略此字段，不进行转换
	URL   string `pb:"url"`
}
```

---

## 📊 对比示例

### 原始方式（需手动转换）
```go
func (s *LinkServiceServer) CreateLink(ctx context.Context, req *pb.CreateLinkRequest) (*pb.CreateLinkResponse, error) {
	// 手动转换 PB 到 Model
	link := &models.LinkModel{
		URL:         req.Url,
		Title:       req.Title,
		Description: req.Description,
		ShortCode:   generateShortCode(),
	}
	if req.ExpiresAt != nil {
		expiresAt := req.ExpiresAt.AsTime()
		link.ExpiresAt = &expiresAt
	}

	// 业务逻辑
	result, err := s.svc.CreateLink(ctx, link)
	if err != nil {
		return nil, err
	}

	// 手动转换 Model 到 PB
	return &pb.CreateLinkResponse{
		Id:        int64(result.ID),
		Url:       result.URL,
		ShortCode: result.ShortCode,
		Title:     result.Title,
		CreatedAt: timestamppb.New(result.CreatedAt),
	}, nil
}
// 总计：~30 行代码
```

### 自动转换方式
```go
func (s *LinkServiceServer) CreateLink(ctx context.Context, req *pb.CreateLinkRequest) (*pb.CreateLinkResponse, error) {
	// 自动转换
	link := &models.LinkModel{}
	converters.ConvertPBToModel(req, link)

	// 业务逻辑
	result, err := s.svc.CreateLink(ctx, link)
	if err != nil {
		return nil, err
	}

	// 自动转换
	pbResp := &pb.CreateLinkResponse{}
	converters.ConvertModelToPB(result, pbResp)

	return pbResp, nil
}
// 总计：~15 行代码 (-50%)
```

---

## ✅ 支持的转换类型

| 源类型 | 目标类型 | 自动支持 |
|--------|---------|--------|
| `time.Time` | `*timestamppb.Timestamp` | ✅ |
| `*timestamppb.Timestamp` | `time.Time` | ✅ |
| `int32` | `int64` | ✅ |
| `int64` | `int32` | ✅ |
| `uint` | `int64` | ✅ |
| `int` | `int32` | ✅ |
| `float32` | `float64` | ✅ |
| `float64` | `float32` | ✅ |
| `string` | `string` | ✅ |
| `[]byte` | `string` | ✅ |
| `*Type` | `Type` | ✅ |
| `Type` | `*Type` | ✅ |
| 嵌套结构 | 嵌套结构 | ✅ |
| 切片 | 切片 | ✅ |

---

## 🎓 最佳实践

### 1. 始终为 Model 字段添加 pb tag
```go
type LinkModel struct {
	ID    uint   `pb:"id"`
	URL   string `pb:"url"`
	// 没有 pb tag 将使用字段名自动匹配
}
```

### 2. 在 service layer 只处理 GORM Model
```go
// Service layer 接收 Model，返回 Model
type LinkService interface {
	CreateLink(ctx context.Context, link *LinkModel) (*LinkModel, error)
	GetLink(ctx context.Context, shortCode string) (*LinkModel, error)
}
```

### 3. 在 gRPC layer 进行 PB ↔ Model 转换
```go
// gRPC layer 处理 PB 和 Model 的转换
func (s *LinkServiceServer) CreateLink(ctx context.Context, req *pb.CreateLinkRequest) (*pb.CreateLinkResponse, error) {
	model := &LinkModel{}
	converters.ConvertPBToModel(req, model)  // PB → Model
	
	result, _ := s.svc.CreateLink(ctx, model)  // Service 逻辑
	
	pbResp := &pb.CreateLinkResponse{}
	converters.ConvertModelToPB(result, pbResp)  // Model → PB
	
	return pbResp, nil
}
```

### 4. 保持 Model 和 PB 字段对齐
```
LinkModel              LinkResponse (PB)
├── ID                ├── id
├── URL               ├── url
├── ShortCode         ├── short_code
├── Title             ├── title
└── Description       └── description
```

---

## 🚀 迁移 engine-ads-link-service 完整流程

### Step 1: 定义 PB（保持原样，在 commonapis）
```protobuf
// commonapis/pb/link.proto
service LinkService {
  rpc CreateLink(CreateLinkRequest) returns (CreateLinkResponse) {
    option (google.api.http) = {
      post: "/v1/links"
      body: "*"
    };
  }
  // ... 其他方法
}
```

### Step 2: 定义 Model（go-rpc-gateway）
```go
// go-rpc-gateway/services/adslink/models/link.go
type LinkModel struct {
	ID        uint   `gorm:"primaryKey" pb:"id"`
	URL       string `gorm:"index" pb:"url"`
	// ... 其他字段
}
```

### Step 3: 复制 Service（go-rpc-gateway）
```go
// go-rpc-gateway/services/adslink/service/link.go
type LinkService interface {
	CreateLink(ctx context.Context, link *LinkModel) (*LinkModel, error)
	// ... 其他方法
}
```

### Step 4: 实现 gRPC（go-rpc-gateway）
```go
// go-rpc-gateway/services/adslink/grpc/server.go
func (s *LinkServiceServer) CreateLink(ctx context.Context, req *pb.CreateLinkRequest) (*pb.CreateLinkResponse, error) {
	link := &LinkModel{}
	converters.ConvertPBToModel(req, link)
	
	result, err := s.svc.CreateLink(ctx, link)
	if err != nil {
		return nil, err
	}
	
	pbResp := &pb.CreateLinkResponse{}
	converters.ConvertModelToPB(result, pbResp)
	
	return pbResp, nil
}
```

### Step 5: 注册服务（go-rpc-gateway）
```go
// main.go
gw.RegisterService(func(s *grpc.Server) {
	pb.RegisterLinkServiceServer(s, grpc.NewLinkServiceServer(linkService))
})
```

**就这样，完全迁移完成！**

---

## 📈 性能考虑

- **反射开销**：仅在请求/响应时发生，业务逻辑层没有额外开销
- **内存**：转换器使用栈分配，无额外堆内存申请
- **缓存**：可添加转换规则缓存优化频繁转换

---

## 🔍 调试技巧

```go
// 启用转换日志
converter := middleware.NewUniversalConverter(global.LOGGER)
// 所有转换失败都会记录详细日志
```

---

## 总结

**核心优势：**
- ✅ 零重复代码
- ✅ 类型安全
- ✅ 自动化处理
- ✅ 易于维护
- ✅ 高效运行
