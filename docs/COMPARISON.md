# go-rpc-gateway vs go-zero

两套 Go 微服务方案的逐维度对比 —— 带并排代码,不只给结论

> [EN] A dimension-by-dimension comparison of two Go microservice stacks, with side-by-side code rather than conclusions alone

本文不做"谁更好"的判断,因为**两者的分歧主要来自约束差异,不是优劣差异** 目标是让你读完能判断哪一套更贴合自己的部署形态、对外 API 风格与团队现状

**go-zero 侧的代码是其规范用法的典型形态**(goctl 生成物与社区惯例),版本间细节可能有差异;go-rpc-gateway 侧的代码对应本仓库当前实现

---

## 目录

| 维度 | 小节 |
|---|---|
| 定位与核心分歧 | [1](#1-一句话定位) · [2](#2-核心分歧双轨契约-vs-单一契约) |
| 工程形态 | [3 项目结构](#3-项目结构) · [4 定义一个接口](#4-同一件事定义一个接口) · [5 代码生成](#5-代码生成两种思路) |
| 数据与转换 | [6 数据访问](#6-同一件事按条件分页查询) · [7 DTO 转换](#7-同一件事model--响应结构) |
| 横切能力 | [8 错误与 i18n](#8-同一件事返回一个业务错误) · [9 中间件](#9-同一件事挂一条中间件) · [10 配置](#10-配置) · [11 可观测](#11-可观测性) · [12 测试](#12-测试) |
| 运行时治理 | [13 三层保护](#13-服务治理三层保护的职责划分) · [14 服务发现与负载均衡](#14-服务发现与负载均衡含一个易忽略的部署前提) |
| 选型 | [15 扩展性](#15-扩展性与定制) · [16 生态](#16-生态与学习曲线) · [17 go-zero 更合适的场景](#17-go-zero-更合适的场景) · [18 迁移考量](#18-从-go-zero-迁移要先实测什么) · [19 怎么选](#19-怎么选) |

---

## 1. 一句话定位

| | 定位 |
|---|---|
| **go-zero** | 社区微服务全家桶:`goctl` 代码生成 + api/zrpc 双引擎 + 内建服务治理(etcd 发现、熔断、降载),主打"工具链驱动、开箱即用" |
| **go-rpc-gateway** | 轻量 gRPC-Gateway 框架:gRPC-first,HTTP 由 proto 注解自动暴露,统一连接池管理多类基础设施,主打"proto 即契约、单进程双协议" |

> [EN] go-zero is a batteries-included community stack driven by its `goctl` toolchain, with two engines (HTTP api and zrpc) and built-in governance. go-rpc-gateway is a gRPC-first framework where HTTP is derived from proto annotations, so one process serves both protocols from one contract

---

## 2. 核心分歧:双轨契约 vs 单一契约

这是两者最根本的区别,后续大部分差异都由它派生

```
go-zero 路线                           go-rpc-gateway 路线
─────────────────────────             ─────────────────────────────
.api 文件(HTTP 契约)                   .proto 文件(唯一契约)
.proto 文件(RPC 契约)                        │
        │                                    │
  goctl 生成 handler / logic           protoc 生成 PB
  / types / routes / client            HTTP 路由 = google.api.http 注解
        │                                    │
  手写 logic,HTTP 与 RPC               Server Reflection 自动注册
  两套 DTO 需人工对齐                   HTTP handler,零胶水代码
        │                                    │
  api 进程 + rpc 进程                   单进程双协议
```

| 维度 | go-zero | go-rpc-gateway |
|---|---|---|
| 契约数量 | 两套(api DSL + proto) | 一套(proto) |
| HTTP/RPC 一致性 | 人工对齐 | 天然一致,同一套 service 实现 |
| 改一个接口 | 改两处 | 改 proto 一处后重新生成 |
| 部署单元 | api 与 rpc 分离 | 单进程双协议 |

**代价对称地存在**:单一契约消除了对齐问题,但也强制你接受 proto 如果对外 API 本来就是 REST 风格、不打算用 gRPC,那 proto + grpc-gateway 这一层是净增的概念负担,go-zero 的 api DSL 更直接

> [EN] The trade-off cuts both ways. A single proto contract removes the alignment problem but forces proto on you. If your public API is REST-shaped and gRPC is not in the picture, go-zero's api DSL is the more direct tool

---

## 3. 项目结构

**go-zero**(goctl 生成,api 与 rpc 各一棵树)

```
order-api/                          order-rpc/
├── order.api                       ├── order.proto
├── etc/order-api.yaml              ├── etc/order.yaml
├── order.go                        ├── order.go
└── internal/                       └── internal/
    ├── config/config.go                ├── config/config.go
    ├── handler/                        ├── server/orderserver.go
    │   ├── routes.go   ← 生成          ├── logic/
    │   └── createorderhandler.go       │   └── createorderlogic.go
    ├── logic/                          └── svc/servicecontext.go
    │   └── createorderlogic.go     orderclient/            ← 生成的客户端
    ├── svc/servicecontext.go       └── order.go
    ├── types/types.go  ← 生成
    └── middleware/
```

**go-rpc-gateway**(单棵树,契约在协议中心)

```
order-service/
├── main.go                      入口,只做 bootstrap
├── bootstrap/                   启动编排
│   ├── server.go                Run() 的完整时序
│   ├── services.go              依赖注入 + gRPC 与 HTTP handler 注册
│   ├── pbmo.go                  Model ↔ PB 转换器注册
│   └── database.go              迁移 / 索引 / 表注释
├── models/                      数据模型
├── repository/                  数据访问
├── service/<module>/            业务实现(实现 proto 的 ServiceServer)
├── errcodes/                    错误码常量与状态码映射
├── migrations/                  自动迁移
├── locales/                     多语言文案
└── resources/                   分环境配置
```

两处结构性差异值得注意:

- **go-zero 的 `types` 与 `handler` 是生成物**,随 `.api` 重新生成;go-rpc-gateway 没有对应目录 —— 请求/响应结构体来自 PB,HTTP handler 由 grpc-gateway 注册
- **go-zero 有 `orderclient/`**(goctl 生成的 rpc 客户端,带 zrpc 封装);go-rpc-gateway 侧直接用 protoc 生成的 PB 客户端 + 连接池

---

## 4. 同一件事:定义一个接口

任务:`POST /v1/orders` 创建订单,同时要能被其他服务通过 gRPC 调用

### go-zero

**① 写 `.api`(HTTP 契约)**

```
type (
    CreateOrderReq {
        UserId int64  `json:"userId"`
        Amount string `json:"amount"`
    }
    CreateOrderResp {
        OrderId string `json:"orderId"`
    }
)

service order-api {
    @handler CreateOrder
    post /v1/orders (CreateOrderReq) returns (CreateOrderResp)
}
```

**② 写 `.proto`(RPC 契约,字段要和上面对齐)**

```protobuf
message CreateOrderRequest {
  int64  user_id = 1;
  string amount  = 2;
}
message CreateOrderResponse {
  string order_id = 1;
}
service Order {
  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse);
}
```

**③ `goctl api go` + `goctl rpc protoc` 生成骨架,然后手写两份 logic**

```go
// api 侧 logic
func (l *CreateOrderLogic) CreateOrder(req *types.CreateOrderReq) (*types.CreateOrderResp, error) {
    // 通常转发给 rpc
    resp, err := l.svcCtx.OrderRpc.CreateOrder(l.ctx, &order.CreateOrderRequest{
        UserId: req.UserId,   // ← 两套结构体逐字段搬运
        Amount: req.Amount,
    })
    if err != nil {
        return nil, err
    }
    return &types.CreateOrderResp{OrderId: resp.OrderId}, nil
}
```

```go
// rpc 侧 logic（真正的业务）
func (l *CreateOrderLogic) CreateOrder(in *order.CreateOrderRequest) (*order.CreateOrderResponse, error) {
    // 业务逻辑
}
```

### go-rpc-gateway

**① 只写 `.proto`,加 HTTP 注解**

```protobuf
import "google/api/annotations.proto";

service OrderService {
  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse) {
    option (google.api.http) = { post: "/v1/orders" body: "*" };
  }
}
```

**② 实现 protoc 生成的接口(一份)**

```go
type OrderService struct {
    pb.UnimplementedOrderServiceServer
    repo OrderRepository
}

func (s *OrderService) CreateOrder(
    ctx context.Context, req *pb.CreateOrderRequest,
) (*pb.CreateOrderResponse, error) {
    // 业务逻辑 —— gRPC 与 HTTP 走的都是这一个方法
}
```

**③ 注册两次(同一个实现)**

```go
// gRPC
g.gateway.RegisterService(func(srv *grpc.Server) {
    pb.RegisterOrderServiceServer(srv, g.orderSvc)
})

// HTTP：注意是 HandlerServer 后缀，直接挂实现，不需要 gRPC 连接自调
g.gateway.RegisterGatewayHandler(func(ctx context.Context, mux *runtime.ServeMux) error {
    return pb.RegisterOrderServiceHandlerServer(ctx, mux, g.orderSvc)
})
```

### 对照

| | go-zero | go-rpc-gateway |
|---|---|---|
| 契约文件 | 2 份(字段要人工对齐) | 1 份 |
| 手写 logic | 2 份(api 转发 + rpc 业务) | 1 份 |
| 请求结构体 | `types.CreateOrderReq` 与 `order.CreateOrderRequest` 两套 | 只有 PB |
| 字段搬运 | api logic 里逐字段赋值 | 无 |
| HTTP 路由声明 | `.api` 里的 `post /v1/orders` | proto 的 `google.api.http` 注解 |
| 进程 | 2 个 | 1 个 |

**反过来看**:go-zero 的两层结构在"api 层要做与 rpc 不同的事"时是优势 —— 比如对外做字段裁剪、聚合多个 rpc、做 BFF 层 go-rpc-gateway 的单一实现在这种场景下需要额外设计(再加一层 gateway 服务)

> [EN] The flip side: go-zero's two-layer split is an advantage when the HTTP layer genuinely differs from the RPC layer — field trimming for external consumers, aggregating several RPCs, a BFF. A single shared implementation needs an extra layer for that

---

## 5. 代码生成:两种思路

`goctl` 是**单一工具覆盖全链路**,成熟度高、文档齐全:

```bash
goctl api go     -api order.api -dir .      # HTTP 骨架
goctl rpc protoc order.proto --go_out=...   # RPC 骨架 + 客户端
goctl model mysql ddl -src order.sql -dir . # 带缓存的数据模型
goctl docker / kube                          # 部署清单
```

go-rpc-gateway 侧是**三层分工**:

| 层 | 职责 | 产物 |
|---|---|---|
| 项目级 | 从模板生成整个工程,交互式收集变量、条件性排除文件、生成后自动 `tidy`/`build`/`test` | 完整仓库 |
| 契约级 | protoc 流水线:PB → gRPC → HTTP 反向代理 → OpenAPI → struct tag 注入 | pb / gw / swagger |
| 实体级 | 往**已存在的文件**里幂等插入一个实体所需的全部条目 | 跨十余处的 patch |

第三层值得解释为什么需要:在一个成型的服务里加一个实体,改动散落在十余处 —— 表名常量、仓库工厂的字段与初始化项、迁移的模型清单、服务注册语句、跨包别名、转换器注册、多个语言文件的键 这类改动**纯机械但要求强一致性**,而且**漏一处的后果不对称**:

| 漏掉 | 后果 |
|---|---|
| 转换器注册 | 编译能过,**运行时**转换失败 |
| bootstrap 字段 | 直接编译失败(最容易发现的那种) |
| 某个语言文件的键 | 该语言静默回退,**没有任何报错** |

实现要点是**用 AST 只定位插入点的字节偏移,再做纯文本拼接**,而不是用 `go/printer` 重新输出整个文件 —— 后者会重排并丢失部分注释位置,破坏版权头与手工排版,diff 噪音巨大

> [EN] The third layer exists because adding one entity to an established service touches a dozen places, each mechanical but requiring strict consistency, with asymmetric failure modes. Locating insertion points via AST and splicing text keeps every byte outside the insertion point intact

**对照**:goctl 的 `model mysql` 从 DDL 生成带缓存的数据模型,这一项 go-rpc-gateway 侧没有等价物(它的数据层走泛型仓储基类,缓存由独立的缓存库处理)

---

## 6. 同一件事:按条件分页查询

### go-zero(goctl model + sqlx,缓存走 `CachedConn`)

```go
// goctl model mysql 生成的形态（带缓存）
func (m *defaultOrderModel) FindOne(ctx context.Context, id int64) (*Order, error) {
    orderIdKey := fmt.Sprintf("%s%v", cacheOrderIdPrefix, id)
    var resp Order
    err := m.QueryRowCtx(ctx, &resp, orderIdKey,
        func(ctx context.Context, conn sqlx.SqlConn, v any) error {
            query := fmt.Sprintf("select %s from %s where `id` = ? limit 1", orderRows, m.table)
            return conn.QueryRowCtx(ctx, v, query, id)
        })
    // ...
}

// 条件分页通常手写 SQL 拼接
func (m *customOrderModel) ListByPage(ctx context.Context, status string, page, size int) ([]*Order, error) {
    query := fmt.Sprintf("select %s from %s where 1=1", orderRows, m.table)
    args := []any{}
    if status != "" {
        query += " and `status` = ?"
        args = append(args, status)
    }
    query += " order by `created_at` desc limit ?, ?"
    args = append(args, (page-1)*size, size)
    var resp []*Order
    err := m.QueryRowsNoCacheCtx(ctx, &resp, query, args...)
    return resp, err
}
```

### go-rpc-gateway(泛型仓储 + 过滤器组)

```go
type WarehouseRepository struct {
    *CachedRepository[models.WarehouseModel]
}

func (r *WarehouseRepository) ListWithPagination(
    ctx context.Context, tenantID string, req *pb.WarehouseListRequest,
) ([]*models.WarehouseModel, *sqlrepo.Pagination32, error) {
    keywordGroup := sqlrepo.NewKeywordFilterGroup(
        req.GetKeyword().GetValue(), nil, []string{"name", "code"})

    mainGroup := sqlrepo.NewFilterGroup(sqlconstants.LOGIC_AND).
        AddFilter(sqlrepo.NewEqFilter("tenant_id", tenantID)).
        AddEqFilterIfNotEmpty("category", req.GetCategory()).
        AddEqFilterIfNotEmpty("status", req.GetStatus()).
        AddGroupIfNotEmpty(keywordGroup)

    q := sqlrepo.NewQuery().
        WithPaging(int(req.GetPageRequest().GetPage()), int(req.GetPageRequest().GetSize())).
        WithFilterGroup(mainGroup)

    // 排序字段走白名单，防注入
    q.AddSafeOrderFromSort(req.GetSort(), "sort_order", "ASC", WarehouseSortableFields)

    return r.ListWithPagination32(ctx, q)
}
```

### 对照

| | go-zero | go-rpc-gateway |
|---|---|---|
| 底层 | `sqlx`(原生 SQL) | GORM 封装的泛型仓储 |
| 条件查询 | 手写 SQL 拼接 + 参数数组 | 过滤器组链式组合 |
| 注入防护 | 依赖自觉(参数化要自己保证) | 强制参数化 + 排序白名单 |
| 缓存一致性 | **`CachedConn` 内建**(行级缓存 + 删除时自动失效) | 独立缓存库,需自己定策略 |
| 复杂 SQL | **直接写**(CTE / `FOR UPDATE` / `ON CONFLICT` / `LATERAL` 都没障碍) | 泛型基类表达不了,要退回 `Raw()` |

**这是 go-zero 明确更强的一项** `CachedConn` 的缓存一致性方案(查询自动读缓存、更新自动删键)是开箱的,而复杂 SQL 在原生 SQL 路线下毫无阻力 如果你的数据层大量使用窗口函数、CTE、悲观锁、upsert,泛型仓储会把你推回 `Raw()`,届时"消掉手写 SQL"的收益归零

> [EN] This is one dimension where go-zero is clearly stronger. `CachedConn` gives you row-cache coherence out of the box, and raw SQL imposes no ceiling on CTEs, pessimistic locks or upserts. A generic repository base pushes those back into `Raw()`

---

## 7. 同一件事:Model → 响应结构

### go-zero(逐字段赋值)

```go
func toOrderResp(o *model.Order) *types.OrderResp {
    return &types.OrderResp{
        OrderId:   o.Id,
        UserId:    o.UserId,
        Amount:    o.Amount,
        Status:    o.Status,
        CreatedAt: o.CreatedAt.Format(time.RFC3339),
        // 字段多了就线性增长
    }
}
```

### go-rpc-gateway(注册一次,反射转换)

```go
// bootstrap/pbmo.go —— 启动时注册一次
pbmo.Register[models.OrderModel, pb.OrderInfo]()

// 业务代码里
info, err := commonsvc.ToPB[models.OrderModel, pb.OrderInfo](ctx, order)
list, err := commonsvc.ToPBs[models.OrderModel, pb.OrderInfo](ctx, orders)
```

### 对照与一个诚实的边界

反射转换消除的是**同名机械拷贝**那一类 以下这些仍须手写,反射代劳不了:

```go
// 字典查表 + 按请求语言取名
EnergyType: optionPtr(dictOption("energy_type", row.EnergyType, locale)),
// 多值分割 + 去重 + URL 前缀重写
Images:     buildImageURLs(assetsBaseURL, row.Images),
// 单位与本地化格式化（中文「万公里」/ 英文千分位 km）
Mileage:    mileageDTO(row.Mileage, locale),
```

所以收益量级取决于**你的赋值里同名直拷占多大比例** 建议先统计再判断,不要默认"样板归零"

另外:**真正消除 HTTP 参数绑定样板的不是反射转换,是 grpc-gateway 的 query/path 自动绑定** go-zero 侧 `httpx.Parse` 也做同一件事,这一项两者接近

> [EN] Reflection removes same-name field copies, not computed assignments like dictionary lookups, localisation or unit formatting. Measure the ratio in your own code before assuming the boilerplate goes to zero

---

## 8. 同一件事:返回一个业务错误

### go-zero

错误码规范与 HTTP 映射需自行设计:

```go
// 自定义错误类型
type CodeError struct {
    Code int    `json:"code"`
    Msg  string `json:"msg"`
}

// 全局错误处理器（通常在 main 里设置一次）
httpx.SetErrorHandlerCtx(func(ctx context.Context, err error) (int, any) {
    switch e := err.(type) {
    case *CodeError:
        return http.StatusOK, Body{Code: e.Code, Msg: e.Msg}
    default:
        return http.StatusInternalServerError, nil
    }
})

// 业务代码
return nil, NewCodeError(10001, "订单不存在")   // 文案硬编码或自建 i18n
```

### go-rpc-gateway

错误码 → 状态码 → 多语言文案是一条内建链路:

```go
// 业务代码只有一行
return nil, ToGRPCError(ctx, errcodes.OrderNotFound)
```

框架完成:错误码 → 按请求语言取文案 → gRPC status 映射 → HTTP JSON 结构

**错误码字符串本身就是 i18n 键**,所以只有一处命名:

```go
OrderNotFound = "error.order_not_found"
```

```json
// locales/zh.json
{ "error": { "order_not_found": "订单不存在" } }
```

### 对照

| | go-zero | go-rpc-gateway |
|---|---|---|
| 错误码规范 | 自行设计 | 内建 |
| HTTP 状态码映射 | 自行实现 handler | 内建三态映射(HTTP / proto / gRPC code) |
| 多语言 | 无内建,自建 | 内建,按请求语言渲染 |
| 错误码与文案键 | 两处命名 | 一处(码即键) |

⚠️ **一个要注意的边界(对任何按后缀推断状态码的方案都成立)**:按后缀自动映射很方便(如 `_not_found` → `NotFound`),但未覆盖的后缀会兜底成 `Internal` 也就是说一个没显式登记的 `_already_exists` 会返回 **500 而不是 409**,且**没有任何报错提示** 这类"配置写了但不生效"的问题上线后毫无症状,只有真出故障时才暴露 —— 所以除自动映射覆盖的那一类,其余都应显式登记

---

## 9. 同一件事:挂一条中间件

### go-zero(HTTP 与 RPC 两套体系)

```go
// HTTP 侧
server := rest.MustNewServer(c.RestConf)
server.Use(middleware.NewAuthMiddleware().Handle)

// 单路由级
server.AddRoute(rest.Route{...}, rest.WithMiddleware(m.Handle))

// RPC 侧是另一套
rpcServer := zrpc.MustNewServer(c.RpcServerConf, register)
rpcServer.AddUnaryInterceptors(interceptor.Auth)
```

### go-rpc-gateway(一条链覆盖双协议)

```go
// 链由框架按固定顺序注入，业务 service 方法里看不到任何中间件调用
// tracing → request-context → 日志 → i18n → 监控 → 限流 → 降载 → 熔断 → 校验 → 压缩
```

### 对照

| | go-zero | go-rpc-gateway |
|---|---|---|
| 体系数量 | 2 套(rest 中间件 + zrpc 拦截器) | 1 条链,双协议共用 |
| 跨协议一致性 | 日志字段、错误结构需团队自行对齐 | 天然一致 |
| 自定义位置 | 显式 `Use`,顺序由调用方控制 | 顺序由框架定,扩展点有限 |

**这里的取舍是对称的**:go-zero 的显式注册给了你完全的顺序控制权;go-rpc-gateway 的固定链保证了一致性,但想在链中间插一个自己的拦截器就不如前者自由

---

## 10. 配置

| | go-zero | go-rpc-gateway |
|---|---|---|
| 加载 | `conf.MustLoad(file, &c)`,结构体由你定义 | 配置库驱动,按环境前缀 + 多格式 |
| 分环境 | 多份 yaml,启动参数指定 | 搜索路径 + 环境标识自动选择 |
| 热更新 | 需自行实现 | 内建热重载选项 |
| 基础设施连接 | 各组件独立配置与初始化 | 统一 Manager(MySQL/Redis/NATS/MQTT/ClickHouse/SMTP/gRPC) |

```go
// go-zero：结构体 + MustLoad
var c config.Config
conf.MustLoad(*configFile, &c)

// go-rpc-gateway：链式构建，含搜索路径与热重载
gw, err := gateway.NewGateway().
    WithSearchPath("resources").
    WithEnvironment(goconfig.GetEnvironment()).
    WithPrefix("gateway-order-service").
    WithHotReload(nil).
    Build()
```

统一连接池的收益是**少做一批选型决策**;代价是**连接类型受框架支持范围约束** —— 要接一个框架没覆盖的中间件,go-zero 那种各自独立接入反而更自由

---

## 11. 可观测性

| | go-zero | go-rpc-gateway |
|---|---|---|
| 链路追踪 | ✅ OTel 内建 | ✅ OTel stats handler |
| 指标 | ✅ Prometheus 内建(请求量、耗时、状态码) | ✅ Prometheus,另含连接池/熔断/降载指标 |
| 日志 | ✅ `logx`,结构化 | ✅ 结构化,trace_id 全链路贯通 |
| 治理指标 | 熔断/降载状态 | 熔断三态计数、降载 CPU/飞行数/承载上限/丢弃数 |

两者在基础三件套上接近 差异在**治理层的可观测粒度**:go-rpc-gateway 把降载的判定输入(CPU 千分比、飞行请求数、滑窗估出的承载上限、是否支持 CPU 采样)都导出成指标,这样过载时能回答"为什么开始丢"而不只是"丢了多少"

一个实现细节值得借鉴:**未启用某能力时不注册其指标**,而不是导出一组恒为 0 的时间序列 —— 后者会让面板和告警规则无法区分"没启用"与"启用了但为 0"

---

## 12. 测试

| | go-zero | go-rpc-gateway |
|---|---|---|
| HTTP 层测试 | `httptest` 打 handler,或起 `rest.Server` | 打 grpc-gateway 的 mux,或直接调 service 方法 |
| RPC 层测试 | 直接调 logic 结构体的方法 | 直接调 service 方法(与 HTTP 同一入口) |
| 契约测试 | api 与 proto 两份,需各自覆盖 | 一份 proto,service 方法测一次覆盖双协议 |
| mock | goctl 可生成 rpc 客户端,配合 gomock | PB 客户端接口,配合 gomock |

**单一契约在测试上是净收益**:同一个 service 方法的单测同时覆盖了 gRPC 与 HTTP 两条路径,不需要为"HTTP 层字段映射对不对"单独写一组断言

反过来,**go-zero 的两层结构让 api 层可以独立测**(比如只测字段裁剪与聚合逻辑),这在 BFF 场景下更顺手

---

## 13. 服务治理:三层保护的职责划分

两者都有限流、降载、熔断 值得说的是**它们回答的是不同问题,不能互相替代**:

```
限流(配额)  →  降载(过载自保)  →  熔断(下游故障)
```

| 层 | 回答 | 信号来源 |
|---|---|---|
| 限流 | 这个调用方超配额了吗 | 配置的 QPS / 令牌桶 |
| 降载 | **本进程还扛得住吗** | CPU + 飞行请求数 + RT 滑窗 |
| 熔断 | 下游还活着吗 | 下游返回的状态码 |

### 两处容易踩的链序问题

**降载必须在限流之后** 限流拒绝是微秒级返回的,若掺进 RT 滑窗会把最小平均 RT 压到接近 0,承载上限随之被低估,降载反而在系统还有余量时就开始丢请求

**降载必须在熔断之外(熔断在内层)** 降载拒绝通常返回 `Unavailable`,而它恰好落在熔断的失败集合里 若降载在内层会形成正反馈:

```
过载 → 降载拒绝 → 熔断把它记为下游失败 → 熔断打开 → 全量拒绝
```

更糟的是熔断通常按路径粒度,一次全局过载会把所有被保护路径的断路器一起打开,且退潮后要等熔断超时才恢复,远长于降载自身的冷却期

> [EN] Two ordering constraints matter. Shedding must sit inside rate limiting, or microsecond-fast limiter rejections pollute the response-time window and collapse the capacity estimate. Shedding must sit outside the breaker, or its own `Unavailable` rejections get recorded as downstream failures and the two form a positive feedback loop

### 自适应降载的判定需要两个条件同时成立

```
(CPU 超阈值 或 仍在冷却期) 且 (飞行请求数超过滑窗估出的承载上限) → 丢弃
```

只看 CPU 会误杀计算密集但吞吐正常的服务(CPU 90% 本身不是问题);只看飞行数会让 IO 密集型服务(长轮询、慢下游)自残 —— 它们天然高并发低 CPU

承载上限按 Little 定律从滑窗估出:`单桶最大通过量 × 每秒桶数 × 最小平均 RT` 取**最大**通过量与**最小**平均 RT,估的是"这台实例确实做到过的吞吐 × 确实达到过的最低延迟"

**容器里必须读 cgroup** 否则 CPU 使用率的分母取的是宿主机核数,1 核配额的容器永远到不了阈值 实现上按 cgroup v2 → cgroup v1 → `/proc/stat` → `getrusage` → 显式不支持 五级降级,且不支持采样时**刻意选择"当作健康"**:自适应判定静默、只留硬并发上限 最坏是降载不生效(还有限流与熔断兜底),而反向假设会在健康进程上凭空丢流量

### 熔断的失败判定不是"非 OK 即失败"

```
计入失败:  Unknown · DeadlineExceeded · Internal · Unavailable · DataLoss
不计失败:  InvalidArgument · NotFound · AlreadyExists · PermissionDenied
           Unauthenticated · FailedPrecondition · Canceled · ResourceExhausted
```

计入的那组正好是映射到 HTTP 5xx 的那组 两个值得说的判断:

- **`ResourceExhausted` 不计** 它是限流背压,计入会让"流量高峰 → 限流拒绝 → 熔断打开 → 下游明明健康却全拒"
- **`Unimplemented` 不计**(对"5xx 即故障"的有意偏离) 方法不存在是确定性契约错误,重试永远失败,熔断只会把信息量更大的 `Unimplemented` 换成笼统的 `Unavailable`,掩盖版本或部署不一致

---

## 14. 服务发现与负载均衡(含一个易忽略的部署前提)

| | go-zero | go-rpc-gateway |
|---|---|---|
| 服务发现 | ✅ 内建 etcd / consul | ⚠️ 依赖 K8s DNS 或静态地址 |
| 客户端负载均衡 | ✅ p2c 内建 | ✅ p2c + EWMA(需配合 Headless Service) |
| 非 K8s 部署 | **开箱可用** | 需自行解决地址发现 |

**这是 go-zero 明确更强的另一项** 如果部署在单机、虚拟机或传统 PaaS,etcd 服务发现是真正的加分项

### 对两个框架都成立的部署陷阱

**ClusterIP + gRPC 会让客户端负载均衡完全失效**

```
普通 Service(ClusterIP)  → 一个虚拟 IP
gRPC                     → 一条长连接(HTTP/2)多路复用所有请求
kube-proxy               → 按「连接」负载均衡,不按「请求」
```

客户端拨一次号,kube-proxy 把该 TCP 连接钉死在某个 pod,之后**所有请求都走那一个 pod** 此时客户端解析器只看到一个地址,`round_robin` 与 p2c 全部退化成"只有一个候选"——"某个 pod 过载、另一个空闲"根本观测不到

| 方案 | 做法 | 代价 |
|---|---|---|
| **Headless Service** | `clusterIP: None`,DNS 返回全部 pod IP 的 A 记录 | 最轻,改一行;但 DNS 解析器有最小重解析间隔,扩缩容感知偏慢 |
| 自定义 resolver | 监听 EndpointSlice 实时下发地址 | 自写 resolver + RBAC,换来秒级感知 |
| 服务网格 | sidecar 在 L7 按请求分发 | 新增控制面 + 每 pod sidecar;客户端侧 LB 实现随之作废 |

还有一个**容易漏掉的前提**:`grpc.Dial` 默认走 `passthrough` 解析器(单地址,客户端 LB 永不生效),而 `grpc.NewClient` 默认走 `dns` 解析器 这两个选哪个直接决定 LB 是否**有可能**生效 —— `grpc.Dial` 在新版 grpc-go 里已废弃,但存量代码里很常见

> [EN] Framework-agnostic and bites often: gRPC multiplexes over one long-lived connection while kube-proxy balances per connection, so a ClusterIP pins every request to one pod and client-side load balancing degenerates. A headless Service is the cheapest fix. Note also that `grpc.Dial` defaults to the `passthrough` resolver while `grpc.NewClient` defaults to `dns`

### 关于服务网格

上了网格,客户端侧的 p2c 实现就成了死代码(网格接管分发后客户端仍只看到一个 ClusterIP) 但有两层不被替代:

- **客户端熔断只被部分替代** 网格的 outlier detection 是按 host 摘除,而按"目标 + 方法"粒度的熔断能把故障隔离到方法级,且失败分类可以业务化
- **降载完全不可替代** 网格在进程外,看不到你进程的 CPU 与飞行请求数

**建议**:别只为了 gRPC 负载均衡上网格 —— Headless Service 就能解决 如果本来就需要 mTLS、统一观测、流量治理,网格是对的选择,gRPC LB 只是顺带的红利

---

## 15. 扩展性与定制

| | go-zero | go-rpc-gateway |
|---|---|---|
| 框架边界 | 清晰,约定明确 | 自研可控 |
| 深度定制 | 需绕开约定(或 fork) | 直接改框架层 |
| 插件体系 | ✅ goctl 插件(可自写生成器) | ❌ 无插件机制 |
| 升级成本 | 跟随社区,API 稳定性有保障 | 自己把控,也自己承担 |

**取舍很直接**:go-zero 的约定清晰意味着"按它的方式做很顺,偏离它就别扭";自研框架意味着"想怎么改都行,但没人替你维护"

如果你的业务有**跨服务一致的特殊横切需求**(特定的权限模型、租户隔离方式、审计要求),把它沉到框架层会消除大量重复代码 —— 这是自研路线最实在的收益 代价是这些定制成了你自己的维护负担,且新人要额外学

---

## 16. 生态与学习曲线

| | go-zero | go-rpc-gateway |
|---|---|---|
| GitHub star | **36k+** | 小众 |
| 官方文档 | ✅ 完整,中英双语,大量示例 | 仓库内 docs/ |
| 社区答案 | ✅ 搜得到 | 基本搜不到 |
| 教程/课程 | ✅ 多 | 无 |
| 招聘认知度 | ✅ 高,简历匹配 | 低 |
| 配套生态 | dtm(分布式事务)、go-queue、goctl 插件 | 自有库组(配置/日志/缓存/校验/错误/转换/构建) |

**这一行差距无法用技术手段弥补** 它影响的是:遇到问题能否搜到答案、新人上手是否有现成教程、招聘时简历匹配度 团队规模越大、流动性越高,这项权重越大

> [EN] This gap cannot be closed by engineering. It affects searchability of answers, availability of tutorials, and hiring signal — and its weight grows with team size and turnover

---

## 17. go-zero 更合适的场景

说清楚这一点,这篇对比才有参考价值:

| 场景 | 为什么 |
|---|---|
| **对外 API 是 REST 风格、不打算引入 gRPC** | proto + grpc-gateway 是净增的概念负担 |
| **非 K8s 部署**(单机、虚拟机、传统 PaaS) | 内建 etcd 服务发现是真加分项 |
| **需要分布式事务** | dtm 生态成熟,对方没有等价物 |
| **数据层重度使用复杂 SQL** | 原生 SQL 路线无天花板,`CachedConn` 的缓存一致性开箱可用 |
| **需要 BFF / 聚合层** | api 与 rpc 的两层结构正好匹配 |
| **团队规模大或流动性高** | 社区规模、文档成熟度、招聘认知度的价值随规模放大 |
| **需要自写代码生成器** | goctl 插件体系现成 |
| **存量 go-zero 代码多** | 迁移成本见下节 |

---

## 18. 从 go-zero 迁移要先实测什么

迁移成本的大头往往**不在"脱离 go-zero"** 建议迁移前先实测自己代码库的这几个量,再决定排期:

| 考察项 | 为什么关键 |
|---|---|
| **是否已有 proto 契约** | 若对外 API 只有 `.api` 而没有 proto,那不是"把两轨合一",而是**从零建一条契约轨** 这通常是最大的单项成本 |
| **是否真的用了 goctl 生成** | 如果 `.api` 只用来生成文档、handler 全是手写,那"生成的样板迁移后消失"这类收益并不存在 |
| **数据访问层的形态** | 统计 `FOR UPDATE`、CTE、`ON CONFLICT`、`LATERAL`、窗口函数的用量 —— 泛型仓储表达不了这些,只能退回 `Raw()`,届时"消掉手写 SQL"的收益归零 |
| **是否依赖 `CachedConn`** | 缓存一致性方案需要重新设计,这项容易被整体漏算 |
| **主键与软删除约定** | 字符串主键 vs 自增整型、状态列软删除 vs `deleted_at`、复合主键 vs 单 ID 假设 —— 不匹配时要么弃用泛型基类,要么做全量数据迁移 |
| **路由级元数据** | 若权限、审计语义挂在路由声明上,注意 `google.api.http` 注解**不承载授权语义**,需另找承载方式(proto 自定义 option 或策略引擎) |
| **测试资产规模** | 契约从 HTTP JSON 变成 proto 后断言需重写 这一项最容易被漏算,而它常常比业务代码本身还大 |
| **长连接能力** | WebSocket 在 proto + gateway 模型下没有对等物(gRPC streaming 也不等价,浏览器客户端走的是 WS 协议) |
| **DTO 样板的真实占比** | 反射转换只消除同名机械拷贝,计算型赋值仍须手写 先统计比例再判断收益量级 |

**一个务实的办法**:挑一个最小的服务端到端做完,测出三个速率 —— 「proto 行/路由·日」「仓储方法/日」「平移行/日」—— 再乘自己的分母 **在拿到实测速率前,任何人周估算都不可靠**

---

## 19. 怎么选

| 你的情况 | 建议 |
|---|---|
| K8s 原生 + gRPC 契约优先 + 需要多语言 | go-rpc-gateway 的取舍与之匹配 |
| 对外 REST 为主、不用 gRPC | go-zero 的 api DSL 更直接 |
| 非 K8s 部署、需要注册中心 | go-zero 内建 etcd |
| 需要分布式事务 | go-zero + dtm |
| 数据层重度复杂 SQL / 依赖缓存一致性 | go-zero 的 sqlx + CachedConn |
| 需要 BFF 聚合层 | go-zero 的两层结构 |
| 有跨服务一致的特殊横切需求 | 自研路线能把它沉到框架层 |
| 团队大、流动性高、依赖社区答案 | go-zero 的生态优势随规模放大 |
| 存量 go-zero 代码多且无 proto | 先按第 18 节实测,别按直觉估 |

**两套方案没有绝对优劣,只有与约束的匹配度** 真正该先确认的是:自己的部署形态、对外 API 风格、数据层复杂度、以及团队对社区生态的依赖程度

> [EN] Neither stack is strictly better. What decides it is your deployment shape, the style of your public API, how complex your data layer is, and how much your team relies on a large community. Measure before you estimate

---

## 延伸阅读

框架自身的实现细节:[快速开始](./QUICKSTART.md) · [Gateway 构建器](./GATEWAY-BUILDER.md) · [服务注册](./SERVICE-REGISTRATION.md) · [中间件链](./MIDDLEWARE.md) · [熔断](./BREAKER.md) · [自适应降载](./SHEDDER.md) · [gRPC 客户端](./GRPC-CLIENT.md) · [错误体系](./ERRORS.md) · [连接池](./CONNECTION-POOL.md)
