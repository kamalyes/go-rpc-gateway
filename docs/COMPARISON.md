# go-rpc-gateway vs go-zero

两套 Go 微服务方案的客观对比 —— 定位、设计哲学、能力矩阵,以及各自更合适的场景

> [EN] An objective comparison of two Go microservice stacks: positioning, design philosophy, capability matrix, and which workloads each suits better

本文不做"谁更好"的结论,因为**两者的分歧主要来自约束差异,不是优劣差异** 读完你应该能判断哪一套更贴合自己的部署形态与团队现状

---

## 1. 一句话定位

| | 定位 |
|---|---|
| **go-zero** | 社区微服务全家桶:`goctl` 代码生成 + api/zrpc 双引擎 + 内建服务治理(etcd 发现、熔断、降载),主打"工具链驱动、开箱即用" |
| **go-rpc-gateway** | 轻量 gRPC-Gateway 框架:gRPC-first,HTTP 由 proto 注解自动暴露,`cpool` 统一管理多类基础设施连接,主打"proto 即契约、单进程双协议" |

> [EN] go-zero is a batteries-included community stack driven by its `goctl` toolchain, with two engines (HTTP api and zrpc) and built-in governance. go-rpc-gateway is a gRPC-first framework where HTTP is derived from proto annotations, so one process serves both protocols from one contract

---

## 2. 核心分歧:双轨契约 vs 单一契约

这是两者最根本的区别,其余差异大多由它派生

```
go-zero 路线                           go-rpc-gateway 路线
─────────────────────────             ─────────────────────────────
.api 文件(HTTP 契约)                   .proto 文件(唯一契约)
.proto 文件(RPC 契约)                        │
        │                                    │
  goctl 生成 handler / logic           protoc 生成 PB
  / types / client 骨架                 HTTP 路由 = google.api.http 注解
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
| DTO 转换 | `types` ↔ model 手写赋值 | 反射转换,注册一次 |
| 部署单元 | api 与 rpc 分离 | 单进程双协议 |
| 改一个接口 | 改两处 | 改 proto 一处后重新生成 |

**代价对称地存在**:单一契约要求你接受 proto 作为唯一真理 如果你的对外 API 本来就是 REST 风格、不打算用 gRPC,那 proto + grpc-gateway 这一层是净增的概念负担,go-zero 的 api DSL 更直接

> [EN] The trade-off cuts both ways. A single proto contract removes the HTTP/RPC alignment problem, but it also forces proto on you. If your public API is REST-shaped and gRPC is not in the picture, go-zero's api DSL is the more direct tool

---

## 3. 功能矩阵

| 能力 | go-zero | go-rpc-gateway |
|---|---|---|
| gRPC 服务 | ✅ zrpc | ✅ 核心能力 |
| HTTP 服务 | ✅ 独立 rest 引擎 | ✅ grpc-gateway,与 gRPC 同进程 |
| 代码生成 | ✅ goctl(api / rpc / model) | ✅ 三层:项目脚手架 + protoc 流水线 + 实体级生成 |
| 服务发现 | ✅ 内建 etcd / consul | ⚠️ 依赖 K8s DNS 或静态地址 |
| 限流 | ✅ 内建 | ✅ 令牌桶(本地无锁)+ 滑动窗口(Redis) |
| 熔断 | ✅ breaker | ✅ HTTP + gRPC 服务端 + gRPC 客户端三处 |
| 自适应降载 | ✅ shedder | ✅ CPU + 飞行请求数 + RT 滑窗三信号 |
| 负载均衡 | ✅ p2c 内建 | ✅ p2c + EWMA(需配合 Headless Service,见第 5 节) |
| 链路追踪 | ✅ OTel | ✅ OTel stats handler |
| 参数校验 | ✅ 内建 | ✅ 拦截器 + proto 注入 tag |
| 统一错误码 | ⚠️ 需自行约定 | ✅ 错误码 → 状态码 → 多语言文案一体 |
| 国际化 | ❌ 无内建 | ✅ 拦截器按请求语言渲染 |
| 基础设施连接池 | ⚠️ 各组件独立接入 | ✅ 统一 Manager(MySQL/Redis/NATS/MQTT/ClickHouse/SMTP/gRPC) |
| 分布式 ID | ⚠️ 无内建 | ✅ Snowflake / ShortFlake |
| 分布式定时任务 | ⚠️ 需引入第三方 | ✅ 可内嵌业务进程 |
| 分布式事务 | ✅ dtm 生态 | ❌ 无 |
| **社区规模** | **✅ 36k+ star,文档成熟,招聘认知度高** | **⚠️ 小众** |

最后一行是**真实且无法通过技术手段弥补的差距** 它影响的是:遇到问题能否搜到答案、新人上手是否有现成教程、招聘时简历匹配度

> [EN] The last row is a real gap that no amount of engineering closes. It affects searchability of answers, availability of tutorials, and hiring signal

---

## 4. 服务治理:三层保护的职责划分

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

go-rpc-gateway 的实现把它们按 `限流 → 降载 → 熔断` 排列,两处顺序都是必须的:

**降载必须在限流之后** 限流拒绝是微秒级返回的,若掺进 RT 滑窗会把最小平均 RT 压到接近 0,承载上限随之被低估,降载反而在系统还有余量时就开始丢请求

**降载必须在熔断之外(熔断在内层)** 降载拒绝通常返回 `Unavailable`,而它恰好落在熔断的失败集合里 若降载在内层会形成正反馈:

```
过载 → 降载拒绝 → 熔断把它记为下游失败 → 熔断打开 → 全量拒绝
```

更糟的是熔断通常按路径粒度,一次全局过载会把所有被保护路径的断路器一起打开,且退潮后要等熔断超时才恢复,远长于降载自身的冷却期

> [EN] Two ordering constraints matter. Shedding must sit inside rate limiting, otherwise microsecond-fast limiter rejections pollute the response-time window and make the capacity estimate collapse. Shedding must sit outside the breaker, otherwise its own `Unavailable` rejections are observed by the breaker as downstream failures and the two form a positive feedback loop

### 自适应降载的判定需要两个条件同时成立

```
(CPU 超阈值 或 仍在冷却期) 且 (飞行请求数超过滑窗估出的承载上限) → 丢弃
```

只看 CPU 会误杀计算密集但吞吐正常的服务(CPU 90% 本身不是问题);只看飞行数会让 IO 密集型服务(长轮询、慢下游)自残 —— 它们天然高并发低 CPU

承载上限按 Little 定律从滑窗估出:`单桶最大通过量 × 每秒桶数 × 最小平均 RT` 取**最大**通过量与**最小**平均 RT,估的是"这台实例确实做到过的吞吐 × 确实达到过的最低延迟"

**容器里必须读 cgroup** 否则 CPU 使用率的分母取的是宿主机核数,1 核配额的容器永远到不了阈值 go-rpc-gateway 的实现按 cgroup v2 → cgroup v1 → `/proc/stat` → `getrusage` → 显式不支持 五级降级,且不支持采样时**刻意选择"当作健康"**:自适应判定静默、只留硬并发上限 最坏是降载不生效(还有限流与熔断兜底),而反向假设会在健康进程上凭空丢流量

---

## 5. 一个与框架无关、但极易被忽略的部署前提

**ClusterIP + gRPC 会让客户端负载均衡完全失效** 这条对两个框架都成立,值得单独讲:

```
普通 Service(ClusterIP)  → 一个虚拟 IP
gRPC                     → 一条长连接(HTTP/2)多路复用所有请求
kube-proxy               → 按「连接」负载均衡,不按「请求」
```

客户端拨一次号,kube-proxy 把该 TCP 连接钉死在某个 pod,之后**所有请求都走那一个 pod** 此时客户端解析器只看到一个地址,`round_robin` 与 p2c 全部退化成"只有一个候选"——"某个 pod 过载、另一个空闲"根本观测不到

### 解法与取舍

| 方案 | 做法 | 代价 |
|---|---|---|
| **Headless Service** | `clusterIP: None`,DNS 返回全部 pod IP 的 A 记录 | 最轻,改一行配置;但 DNS 解析器有最小重解析间隔,扩缩容感知偏慢 |
| 自定义 resolver | 监听 EndpointSlice 实时下发地址 | 要自写 resolver + RBAC,换来秒级感知 |
| 服务网格 | sidecar 在 L7 按请求分发 | 新增控制面 + 每 pod sidecar;客户端侧 LB 实现随之作废 |

还有一个**容易漏掉的前提**:`grpc.Dial` 默认走 `passthrough` 解析器(单地址,客户端 LB 永不生效),而 `grpc.NewClient` 默认走 `dns` 解析器 这两个选哪个直接决定 LB 是否**有可能**生效 —— `grpc.Dial` 在新版 grpc-go 里已废弃,但存量代码里很常见

> [EN] This one is framework-agnostic and bites often. gRPC multiplexes over a single long-lived connection while kube-proxy balances per connection, so a ClusterIP pins every request to one pod and client-side load balancing degenerates. A headless Service is the cheapest fix. Also note `grpc.Dial` defaults to the `passthrough` resolver (single address, no client LB) whereas `grpc.NewClient` defaults to `dns`

### 关于服务网格

如果上网格,客户端侧的 p2c 实现就成了死代码(网格接管分发后客户端仍只看到一个 ClusterIP) 但有两层不被替代:

- **客户端熔断只被部分替代** 网格的 outlier detection 是按 host 摘除,而按"目标 + 方法"粒度的熔断能把故障隔离到方法级,且失败分类可以业务化
- **降载完全不可替代** 网格在进程外,看不到你进程的 CPU 与飞行请求数

**建议**:别只为了 gRPC 负载均衡上网格 —— Headless Service 就能解决 如果本来就需要 mTLS、统一观测、流量治理,网格是对的选择,gRPC LB 只是顺带的红利

---

## 6. 代码生成:两种思路

go-zero 的 `goctl` 是**单一工具覆盖全链路**(api / rpc / model / 插件体系),成熟度高、文档齐全

go-rpc-gateway 侧是**三层分工**:

| 层 | 职责 |
|---|---|
| 项目级 | 从模板生成整个工程,交互式收集变量、条件性排除文件、生成后自动 `tidy`/`build`/`test` |
| 契约级 | protoc 流水线:PB → gRPC → HTTP 反向代理 → OpenAPI → struct tag 注入 |
| 实体级 | 往**已存在的文件**里幂等插入一个实体所需的全部条目 |

第三层值得解释一下为什么需要:在一个成型的服务里加一个实体,改动散落在十余处 —— 表名常量、仓库工厂的字段与初始化项、迁移的模型清单、服务注册语句、跨包别名、转换器注册、多个语言文件的键 这类改动的特点是**纯机械但强一致性**,而且**漏一处的后果不对称**:

- 漏转换器注册 → 编译能过,运行时转换失败
- 漏 bootstrap 字段 → 直接编译失败
- 漏某个语言文件 → 该语言静默回退,没有任何报错

所以它适合用确定性生成而不是手工(也不适合交给非确定性的 AI 提示词) 实现上的要点是**用 AST 只定位插入点的字节偏移,再做纯文本拼接**,而不是用 `go/printer` 重新输出整个文件 —— 后者会重排并丢失部分注释位置,导致版权头与手工排版被破坏、diff 噪音巨大

> [EN] The third layer exists because adding one entity to an established service touches a dozen places, each mechanical but requiring strict consistency, and the cost of missing one is asymmetric. Locating insertion points with the AST and then splicing text keeps every byte outside the insertion point intact, which `go/printer` round-tripping does not

---

## 7. 国际化与错误码

go-zero 没有内建 i18n,错误码规范也需自行设计(`httpx` 的错误处理器要自己写)

go-rpc-gateway 把三者合成一条链路,业务代码只写一行:

```go
return nil, ToGRPCError(ctx, ErrCodeSomethingNotFound)
```

框架完成:错误码 → 按请求语言取文案 → gRPC status 映射 → HTTP JSON 结构

一个值得借鉴的约定是**让错误码字符串本身就是 i18n 键**(如 `error.order_not_found`),这样只有一处命名,不存在"错误码与文案键漂移"的问题

⚠️ 但要注意**后缀自动映射的边界**:按后缀推断状态码(如 `_not_found` → `NotFound`)很方便,但未覆盖的后缀会兜底成 `Internal` 也就是说一个没显式登记的 `_already_exists` 会返回 **500 而不是 409**,且**没有任何报错提示** 这类"配置写了但不生效"的问题上线后毫无症状,只有真出故障时才发现 —— 所以除自动映射覆盖的那一类,其余错误码都应显式登记

---

## 8. go-zero 更合适的场景

说清楚这一点才有参考价值:

- **对外 API 是 REST 风格、不打算引入 gRPC** —— 这时 proto + grpc-gateway 是净增的概念负担
- **非 K8s 部署**(单机、虚拟机、传统 PaaS)—— go-zero 内建 etcd 服务发现是真正的加分项,而 go-rpc-gateway 假定了 K8s DNS
- **需要分布式事务** —— dtm 生态是成熟方案,go-rpc-gateway 侧没有对等物
- **团队规模较大或流动性高** —— 社区规模、文档成熟度、招聘认知度的价值随团队规模放大
- **存量 go-zero 代码多** —— 迁移成本见下节

---

## 9. 从 go-zero 迁移要考虑什么

迁移成本的大头往往不在"脱离 go-zero",而在以下几项 建议迁移前先实测自己代码库的这几个量:

| 考察项 | 为什么关键 |
|---|---|
| **是否已有 proto 契约** | 若对外 API 只有 `.api` 文件而没有 proto,那不是"把两轨合一",而是**从零建一条契约轨** 这通常是最大的单项成本 |
| **是否真的用了 goctl 生成** | 如果 `.api` 只用来生成文档、handler 全是手写,那"生成的样板迁移后消失"这类收益并不存在 |
| **数据访问层的形态** | raw SQL 与 ORM 封装是范式冲突 统计一下 `FOR UPDATE`、CTE、`ON CONFLICT`、`LATERAL` 的用量 —— 泛型仓储基类表达不了这些,只能退回 `Raw()`,届时"消掉手写 SQL"的收益归零 |
| **主键与软删除约定** | 字符串主键 vs 自增整型、状态列软删除 vs `deleted_at`、复合主键 vs 单 ID 假设 —— 不匹配时要么弃用泛型基类,要么做全量数据迁移 |
| **路由级元数据** | 若权限、审计等语义挂在路由声明上,注意 `google.api.http` 注解**不承载授权语义**,这部分需要另找承载方式(proto 自定义 option 或策略引擎) |
| **测试资产规模** | 接口契约从 HTTP JSON 变成 proto 后,断言需要重写 这一项最容易被漏算,而它常常比业务代码本身还大 |
| **长连接能力** | WebSocket 在 proto + gateway 模型下没有对等物(gRPC streaming 也不等价,浏览器客户端走的是 WS 协议) |
| **DTO 样板的真实占比** | 反射转换能消除的是**同名机械拷贝**那一类 字典查表、本地化、单位格式化这类计算型赋值仍须手写 先统计一下两者比例,再判断收益量级 |

**一个务实的办法**:挑一个最小的服务端到端做完,测出三个速率——「proto 行/路由·日」「仓储方法/日」「平移行/日」——再乘自己的分母 在拿到实测速率前,任何人周估算都不可靠

---

## 10. 怎么选

| 你的情况 | 建议 |
|---|---|
| K8s 原生 + gRPC 契约优先 + 需要多语言 | go-rpc-gateway 的取舍与之匹配 |
| 对外 REST 为主、不用 gRPC | go-zero 的 api DSL 更直接 |
| 非 K8s 部署、需要注册中心 | go-zero 内建 etcd |
| 需要分布式事务 | go-zero + dtm |
| 团队大、流动性高、依赖社区答案 | go-zero 的生态优势随规模放大 |
| 存量 go-zero 代码多且无 proto | 先按第 9 节实测,别按直觉估 |

**两套方案没有绝对优劣,只有与约束的匹配度** 真正该先确认的是自己的部署形态、对外 API 风格、以及团队对社区生态的依赖程度

> [EN] Neither stack is strictly better. What decides it is your deployment shape, the style of your public API, and how much your team relies on a large community. Measure before you estimate

---

## 延伸阅读

框架自身的实现细节:[熔断](./BREAKER.md) · [自适应降载](./SHEDDER.md) · [中间件链](./MIDDLEWARE.md) · [gRPC 客户端](./GRPC-CLIENT.md) · [错误体系](./ERRORS.md) · [快速开始](./QUICKSTART.md)
