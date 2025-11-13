# engine-ads-link-service 迁移到 go-rpc-gateway - 简化版本

## 🎯 核心原则

**只需维护 PB 文件，HTTP 和 gRPC 端点自动生成！**

```
commonapis/pb/link.proto (维护此文件)
           ↓
         protoc
           ↓
commonapis/pb/link.pb.go (自动生成)
commonapis/pb/link_grpc.pb.go (自动生成)
           ↓
go-rpc-gateway/services/adslink/grpc/server.go (实现 gRPC 服务)
           ↓
gateway.RegisterService() (一句话注册)
           ↓
✅ HTTP + gRPC 端点自动可用
```

---

## 📋 迁移步骤（简化）

### 第 1 步：准备 PB 定义（commonapis 中）

**文件：** `commonapis/pb/link.proto`

```protobuf
syntax = "proto3";

package link.api.v1;

import "google/api/annotations.proto";
import "google/protobuf/timestamp.proto";

service LinkService {
  rpc CreateLink(CreateLinkRequest) returns (CreateLinkResponse) {
    option (google.api.http) = {
      post: "/v1/links"
      body: "*"
    };
  }

  rpc GetLink(GetLinkRequest) returns (LinkResponse) {
    option (google.api.http) = {
      get: "/v1/links/{short_code}"
    };
  }

  rpc ListLinks(ListLinksRequest) returns (ListLinksResponse) {
    option (google.api.http) = {
      get: "/v1/links"
    };
  }

  rpc UpdateLink(UpdateLinkRequest) returns (LinkResponse) {
    option (google.api.http) = {
      patch: "/v1/links/{id}"
      body: "*"
    };
  }

  rpc DeleteLink(DeleteLinkRequest) returns (DeleteLinkResponse) {
    option (google.api.http) = {
      delete: "/v1/links/{id}"
    };
  }
}

message CreateLinkRequest {
  string url = 1;
  string title = 2;
  string description = 3;
  google.protobuf.Timestamp expires_at = 4;
}

message CreateLinkResponse {
  int64 id = 1;
  string url = 2;
  string short_code = 3;
  string title = 4;
  google.protobuf.Timestamp created_at = 5;
}

message GetLinkRequest {
  string short_code = 1;
}

message LinkResponse {
  int64 id = 1;
  string url = 2;
  string short_code = 3;
  string title = 4;
  string description = 5;
  int64 click_count = 6;
  google.protobuf.Timestamp created_at = 7;
  google.protobuf.Timestamp updated_at = 8;
}

message ListLinksRequest {
  int32 page = 1;
  int32 page_size = 2;
}

message ListLinksResponse {
  repeated LinkResponse links = 1;
  int64 total = 2;
  int32 page = 3;
  int32 page_size = 4;
}

message UpdateLinkRequest {
  int64 id = 1;
  string title = 2;
  string description = 3;
}

message DeleteLinkRequest {
  int64 id = 1;
}

message DeleteLinkResponse {
  bool success = 1;
}
```

**重点：** `google.api.http` 注解定义 HTTP 端点映射，**无需手动代码**！

---

### 第 2 步：实现 gRPC 服务（go-rpc-gateway 中）

**文件结构：**
```
go-rpc-gateway/services/adslink/
├── models/
│   └── link.go              (数据模型 - 从 engine-ads-link-service 复制)
├── service/
│   └── link.go              (业务逻辑 - 从 engine-ads-link-service 复制)
└── grpc/
    └── server.go            (gRPC 服务实现 - 新增)
```

**文件：** `go-rpc-gateway/services/adslink/grpc/server.go`

```go
package grpc

import (
	"context"
	pb "github.com/Divine-Dragon-Voyage/commonapis/pb"
	"github.com/kamalyes/go-rpc-gateway/services/adslink/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LinkServiceServer struct {
	pb.UnimplementedLinkServiceServer
	svc service.LinkService
}

func NewLinkServiceServer(svc service.LinkService) *LinkServiceServer {
	return &LinkServiceServer{svc: svc}
}

// 实现每个 RPC 方法
func (s *LinkServiceServer) CreateLink(ctx context.Context, req *pb.CreateLinkRequest) (*pb.CreateLinkResponse, error) {
	// 从 engine-ads-link-service 中的 server 层复制逻辑
	link := &service.LinkModel{
		URL:       req.Url,
		Title:     req.Title,
		ShortCode: generateShortCode(),
	}
	result, err := s.svc.CreateLink(ctx, link)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.CreateLinkResponse{
		Id:        int64(result.ID),
		Url:       result.URL,
		ShortCode: result.ShortCode,
	}, nil
}

// ... 其他方法 (GetLink, ListLinks, UpdateLink, DeleteLink)
```

**重点：** 只需实现 5 个方法，直接转发到 service 层！

---

### 第 3 步：初始化和注册（main.go 或启动代码）

**文件：** `cmd/main.go` 或 `services/adslink/init.go`

```go
func main() {
	// 构建网关
	gw, err := gateway.NewGateway().
		WithConfigPath("./config/gateway-dev.yaml").
		Build()
	if err != nil {
		panic(err)
	}

	// 初始化 AdsLink 服务
	db := gw.GetDB()
	db.AutoMigrate(&LinkModel{})
	
	linkService := service.NewLinkService(db)
	linkServer := grpc.NewLinkServiceServer(linkService)

	// 注册 gRPC 服务 - 就这一行！
	gw.RegisterService(func(s *grpc.Server) {
		pb.RegisterLinkServiceServer(s, linkServer)
	})

	// 启动网关
	gw.Start()
	gw.WaitForShutdown()
}
```

**仅需 1 句注册代码！**

---

## ✅ 自动生成的 HTTP 端点

| 方法 | HTTP 路径 | 来源 |
|------|---------|------|
| CreateLink | `POST /v1/links` | Proto 中的 `post: "/v1/links"` |
| GetLink | `GET /v1/links/{short_code}` | Proto 中的 `get: "/v1/links/{short_code}"` |
| ListLinks | `GET /v1/links` | Proto 中的 `get: "/v1/links"` |
| UpdateLink | `PATCH /v1/links/{id}` | Proto 中的 `patch: "/v1/links/{id}"` |
| DeleteLink | `DELETE /v1/links/{id}` | Proto 中的 `delete: "/v1/links/{id}"` |

**零额外代码，完全自动！**

---

## 📊 代码行数对比

### 原始方式（engine-ads-link-service）
```
router.go        (路由定义)          150 行
handler.go       (HTTP 处理器)       300 行
middleware.go    (中间件)            200 行
service.go       (业务逻辑)          400 行
model.go         (数据模型)          100 行
────────────────────────────────────
总计                               1150 行
```

### go-rpc-gateway 方式
```
link.proto       (PB 定义)           100 行
server.go        (gRPC 实现)         200 行  ← 自动映射到 HTTP
service.go       (业务逻辑)          400 行  ← 可复用
model.go         (数据模型)          100 行  ← 可复用
────────────────────────────────────
总计                                800 行  (-30% 代码)

且没有：
❌ 路由管理代码
❌ HTTP handler 模板代码
❌ 中间件重复代码
❌ 手动参数绑定代码
```

---

## 🚀 迁移检查清单

### 前期准备
- [ ] 审查 PB 定义是否完整（来自 commonapis）
- [ ] 确认所有 API 端点都有 `google.api.http` 注解
- [ ] 列出所有需要迁移的数据模型

### 代码迁移
- [ ] 复制 `persist/models/` 到 `go-rpc-gateway/services/adslink/models/`
- [ ] 复制 `server/` 业务逻辑到 `go-rpc-gateway/services/adslink/service/`
- [ ] 创建 `grpc/server.go` 实现 gRPC 服务
- [ ] 创建 `init.go` 或修改 `main.go` 进行服务注册
- [ ] 更新配置文件（数据库、日志等）

### 测试
- [ ] 编译检查：`go build ./...`
- [ ] gRPC 测试：`grpcurl` 或 Postman
- [ ] HTTP 测试：`curl` 或 Postman
- [ ] 数据库迁移：验证模型自动迁移

### 部署
- [ ] 更新 Docker 镜像
- [ ] 配置环境变量
- [ ] 监控和告警

---

## 💡 关键优势

| 方面 | 收益 |
|------|------|
| **代码行数** | -30%（减少样板代码） |
| **路由管理** | ✅ 无需手动（PB 自动映射） |
| **中间件** | ✅ 统一管理（gateway 框架） |
| **文档** | ✅ 自动生成（Swagger） |
| **配置** | ✅ 热加载（go-config） |
| **监控** | ✅ 内置（Prometheus + Jaeger） |
| **扩展性** | ✅ 模块化（易新增服务） |

---

## 📝 总结

**核心概念：**
```
┌─────────────────────────┐
│  commonapis/pb/xxx.proto │ ← 只需维护这个
└────────────┬────────────┘
             │
      protoc 自动生成
             │
    ┌────────▼──────────┐
    │ link.pb.go       │
    │ link_grpc.pb.go  │
    └────────┬──────────┘
             │
    ┌────────▼──────────────────────┐
    │ gRPC 服务实现 (server.go)    │
    │ 只需实现 5 个方法             │
    └────────┬──────────────────────┘
             │
    ┌────────▼──────────────────────┐
    │ gateway.RegisterService()     │
    │ 一句话注册                   │
    └────────┬──────────────────────┘
             │
      ✅ HTTP 自动可用
      ✅ gRPC 自动可用
      ✅ 端点自动映射
      ✅ 文档自动生成
```

**一句话总结：** 
将 engine-ads-link-service 迁移到 go-rpc-gateway，只需：
1. 维护 PB 文件（commonapis）
2. 实现 gRPC 服务（go-rpc-gateway）
3. 一句话注册（RegisterService）

**无需：** 路由、HTTP handler、中间件重复代码 ✅
