# 连接池管理

## 概述

`cpool.Manager` 是所有连接的唯一管理者，统一管理数据库、Redis、ClickHouse、NATS、MQTT、SMTP 等客户端连接的生命周期。

> 源码：[cpool/manager.go](../cpool/manager.go)

## 连接管理架构

```mermaid
flowchart TD
    CFG["Gateway 配置"] --> MGR["PoolManager, cpool.Manager"]

    subgraph POOLS["连接池（按配置启用）"]
        DB["Database, MySQL / PostgreSQL / SQLite / CockroachDB"]
        REDIS["Redis, go-redis"]
        CH["ClickHouse, 时序数据库"]
        NATS["NATS / JetStream, 消息队列"]
        MQTT["MQTT, IoT 消息"]
        SMTP["SMTP, 邮件发送"]
    end

    MGR --> POOLS

    subgraph GLOBAL_REF["全局便捷引用"]
        G_DB["global.DB"]
        G_REDIS["global.REDIS"]
        G_CH["GetClickHouse()"]
        G_NATS["GetNats()"]
    end

    DB --> G_DB
    REDIS --> G_REDIS
    CH --> G_CH
    NATS --> G_NATS

    subgraph HEALTH["健康检查"]
        HC["manager.HealthCheck()"]
        HC_DB["database: true"]
        HC_REDIS["redis: true"]
    end

    MGR --> HEALTH

    style MGR fill:#e3f2fd
    style POOLS fill:#fff3e0
    style GLOBAL_REF fill:#e8f5e9
    style HEALTH fill:#fce4ec
```

## PoolManager 接口

> 源码：[cpool/manager.go:PoolManager](../cpool/manager.go)

```go
type PoolManager interface {
    Initialize(ctx context.Context, cfg *gwconfig.Gateway) error

    GetDB() *gorm.DB
    GetRedis() redis.UniversalClient
    GetMQTT() mqtt.Client
    GetSnowflake() *snowflake.Node
    GetSMTP() smtp.MailHandler
    GetClickHouse() *gorm.DB
    GetNats() *natsclient.NatsConn
    GetI18n() interface{}

    SetDB(db *gorm.DB)
    SetRedis(rdb redis.UniversalClient)
    SetMQTT(mqtt mqtt.Client)
    SetSnowflake(node *snowflake.Node)
    SetSMTP(smtp smtp.MailHandler)
    SetClickHouse(conn *gorm.DB)
    SetNats(conn *natsclient.NatsConn)
    SetI18n(i18n interface{})

    Close() error
    HealthCheck() map[string]bool
}
```

## 创建 Manager

```go
manager := cpool.NewManager(logger)
if err := manager.Initialize(ctx, cfg); err != nil {
    return err
}
defer manager.Close()
```

Manager 初始化时自动根据配置启用对应连接：

```yaml
database:
  enabled: true
cache:
  enabled: true
clickhouse:
  enabled: true
nats:
  enabled: true
mqtt:
  enabled: true
smtp:
  enabled: true
```

连接信息统一走单 DSN：每个连接池只有一个 `dsn`（Redis 与 NATS 叫 `url`）字段，
地址与认证写在里面。驱动原生支持的参数由官方解析器解析；MQTT、SMTP 等
驱动不支持的查询参数由网关解析后接入对应选项。NATS 不支持查询参数，其超时与
重连选项仍保留独立字段。每种 DSN 的完整参数表与可粘贴示例见对应配置类型注释：

| 模块 | 配置键 | DSN 字段 | 参数表与示例 |
| --- | --- | --- | --- |
| 数据库 | `database` | `dsn` | [config/database/database.go](../config/database/database.go) 的 `Database` 类型注释 |
| Redis | `cache.redis` | `url` | [config/cache/redis.go](../config/cache/redis.go) 的 `Redis` 类型注释 |
| ClickHouse | `clickhouse` | `dsn` | [config/tsdb/clickhouse.go](../config/tsdb/clickhouse.go) 的 `ClickHouse` 类型注释 |
| NATS | `nats` | `url` | [config/queue/nats.go](../config/queue/nats.go) 的 `Nats` 类型注释 |
| MQTT | `mqtt` | `dsn` | [config/queue/mqtt.go](../config/queue/mqtt.go) 的 `Mqtt` 类型注释 |
| SMTP | `smtp` | `dsn` | [config/smtp/smtp.go](../config/smtp/smtp.go) 的 `Smtp` 类型注释 |

## 数据库 — MySQL / PostgreSQL / SQLite / CockroachDB

> 源码：[cpool/database/client.go](../cpool/database/client.go)

支持四种数据库驱动，由 `database.Gorm` 根据配置 `type` 字段分发：MySQL 走
`mysqldriver.New`，SQLite 走 `gormSQLiteOpen`（需 `-tags sqlite`），postgres 与
cockroachdb 走 `gormPostgresOpen`（需 `-tags postgres`，CockroachDB 兼容 PostgreSQL 协议）。

DSN 的格式由 `type` 决定，四种互不相同——MySQL 用的不是 URL 形式，SQLite 的 DSN 就是文件路径：

```yaml
database:
  enabled: true
  type: "mysql"            # mysql | postgres | sqlite | cockroachdb
  # mysql：go-sql-driver 专有格式，主机包在 tcp(...) 里，parseTime=True 必带
  dsn: "gateway:mysql_pwd@tcp(127.0.0.1:3306)/gateway?charset=utf8mb4&parseTime=True&loc=Local"
  # postgres / cockroachdb：
  #   dsn: "postgres://gateway:pg_pwd@127.0.0.1:5432/gateway?sslmode=disable&TimeZone=UTC"
  # sqlite：
  #   dsn: "file:data/gateway.db?cache=shared&mode=rwc"

  # 连接池参数：database/sql 层的设置，不在 DSN 里
  max-open-conns: 100
  max-idle-conns: 10
  conn-max-idle-time: 30m
  conn-max-lifetime: 1h

  # GORM 行为开关
  log-level: "warn"        # silent | error | warn | info
  slow-threshold: 200      # 慢查询阈值（毫秒）
  prepare-stmt: false      # 走 PgBouncer transaction 模式时必须为 false
```

> 每种类型的完整示例（含 TLS、Unix socket、PgBouncer、CockroachDB Cloud、内存库）见
> [config/database/database.go](../config/database/database.go) 的 `Database` 类型注释

获取连接：

```go
db := gwglobal.GetDB()
db.Create(&user)
db.Find(&users)
```

## Redis

> 源码：[cpool/redis/redis.go](../cpool/redis/redis.go)

连接模式由 URL 自己表达，不需要额外开关：带 `master_name` 走哨兵，出现重复的
`addr` 参数或带 `cluster=true` 走集群，其余走单机。三种模式分别交给 go-redis 的
`ParseFailoverURL` / `ParseClusterURL` / `ParseURL` 解析，判定结果会写进启动日志的
`mode` 字段。

```yaml
cache:
  enabled: true
  type: "redis"
  default-ttl: 30m
  key-prefix: "cache:"
  redis:
    # 单机
    url: "redis://default:redis_pwd@127.0.0.1:6379/0?pool_size=200&dial_timeout=5s&read_timeout=3s"
    # 集群（多节点用重复的 addr 追加）
    #   url: "redis://default:pwd@10.0.0.1:6379?addr=10.0.0.2:6379&addr=10.0.0.3:6379&route_by_latency=true"
    # 集群（只有一个入口地址时要显式声明）
    #   url: "redis://default:pwd@redis-cluster.example.com:6379?cluster=true&max_redirects=3"
    # 哨兵（master_name 非空即哨兵模式）
    #   url: "redis://sentinel_user:sentinel_pwd@10.0.0.1:26379/0?master_name=mymaster&addr=10.0.0.2:26379&username=default&password=data_pwd"
    # TLS
    #   url: "rediss://default:pwd@redis.example.com:6380/0?skip_verify=true"
```

go-redis 的 URL 解析器会**拒绝**不认识的查询参数（报 `redis: unexpected option`），
所以参数名写错是启动即失败，不会静默忽略。四个网关扩展参数（`cluster`、
`read_buffer_size`、`write_buffer_size`、`context_timeout_enabled`）由
[cpool/redis/redis.go](../cpool/redis/redis.go) 在交给驱动之前摘出来自行应用。

> 全部可用参数（三种模式各自支持哪些）与可粘贴示例见
> [config/cache/redis.go](../config/cache/redis.go) 的 `Redis` 类型注释

获取连接：

```go
rdb := gwglobal.GetRedis()
rdb.Set(ctx, "key", "value", 10*time.Minute)
val, err := rdb.Get(ctx, "key").Result()
```

## ClickHouse

> 源码：[cpool/clickhouse/client_support.go](../cpool/clickhouse/client_support.go)（需 `-tags clickhouse`）

```yaml
clickhouse:
  enabled: true
  dsn: "clickhouse://default:ch_pwd@127.0.0.1:9000/analytics?dial_timeout=10s&read_timeout=20s&compress=lz4&max_idle_conns=5&max_open_conns=20&conn_max_lifetime=1h&conn_max_idle_time=5m"
  # HTTP 协议：  dsn: "http://default:ch_pwd@127.0.0.1:8123/analytics"
  # ClickHouse Cloud： dsn: "https://default:ch_pwd@xxx.clickhouse.cloud:8443/analytics?secure=true"

  # GORM 行为开关（ORM 层，DSN 表达不了）
  log-level: "info"
  slow-threshold: 200
  skip-default-transaction: true     # ClickHouse 不支持传统事务
  singular-table: true
  create-batch-size: 1000
```

DSN 里驱动不认识的参数会被当成 ClickHouse 会话 setting 原样下发，所以设置名写错会在
建连时报 `code: 115 Unknown setting`。两个例外由
[cpool/clickhouse/client_support.go](../cpool/clickhouse/client_support.go) 处理：
`conn_max_idle_time` 被摘出来作用到 `sql.DB.SetConnMaxIdleTime`（`clickhouse.Options`
没有这个字段），`session_timezone` 在 DSN 未写时补 `UTC`（naive `DateTime64` 列用会话时区
解释，不固定会让 UTC 查询参数被偏移）。

> 全部可用参数与可粘贴示例见 [config/tsdb/clickhouse.go](../config/tsdb/clickhouse.go) 的
> `ClickHouse` 类型注释

获取连接（返回 `*gorm.DB`，由 `NewClickHouseDB` 创建，内部基于 gorm clickhouse 驱动，并复用 `cpool/database` 的 `GormLogger` 统一 SQL 日志）：

```go
chDB := gwglobal.GetPoolManager().GetClickHouse()
rows, err := chDB.Raw("SELECT * FROM events LIMIT 10").Rows()
defer rows.Close()
```

## NATS / JetStream

> 源码：[cpool/nats/client_support.go](../cpool/nats/client_support.go)（需 `-tags mq`）

```yaml
nats:
  enabled: true
  # 认证信息写在 URL 里；多节点逗号分隔
  url: "nats://gateway:nats_pwd@127.0.0.1:4222"
  # Token 认证（只给用户名不给密码即按 Token 解释）：
  #   url: "nats://s3cr3t_token@nats.example.com:4222"
  # 集群： url: "nats://gateway:pwd@nats-1:4222,nats://gateway:pwd@nats-2:4222"
  name: "my-service"

  # 连接级参数：nats.go 不解析 URL 查询参数，只能逐项传 nats.Option，所以保持独立字段
  connect-timeout: 10       # 秒
  reconnect-wait: 2         # 秒
  max-reconnects: 10

  # JetStream 流级运维参数（流名由业务建流时决策）
  jet-stream: true
  replicas: 1
  storage: "file"           # file | memory
  retention: "limits"       # limits | interest | workqueue
```

认证信息只认 URL：官方客户端的 `connectProto` 一旦发现 URL 带 userinfo，就完全忽略
`nats.Options` 上的 `User`/`Password`/`Token`，两处都配时 URL 静默取胜，所以原先的
`username` / `password` / `token` 三个字段已移除。

> 完整 URL 格式与可粘贴示例见 [config/queue/nats.go](../config/queue/nats.go) 的 `Nats` 类型注释

获取连接：

```go
natsConn := gwglobal.GetPoolManager().GetNats()
natsConn.Conn.Publish("subject", data)

// 使用 JetStream（需配置 jetstream-enabled）
js := natsConn.JetStream
js.Publish("subject", data)
```

`NatsConn` 封装结构（由 `NewNats` 创建，只持有 `nats-io/nats.go` 官方类型，将底层连接与 JetStream 上下文绑定在一起）：

```go
type NatsConn struct {
    Conn      *nats.Conn            // NATS 底层连接实例
    JetStream nats.JetStreamContext // JetStream 上下文（启用 JetStream 时非 nil）
}
```

> 连接池与 JetStream 都取自官方客户端，不再经第三方封装
> 原先由封装层提供的 `GetNatsX()`、泛型发布/订阅与 WorkerPool 已移除；
> 配置项 `worker-pool-size` / `worker-queue-size` 也已从配置结构体里删除——
> 它们曾保留着只为在启动时告警，留一个填了必然不生效的配置项没有意义

## MQTT

> 源码：[cpool/mqtt/mqtt_support.go](../cpool/mqtt/mqtt_support.go)

使用 `-tags mq` 构建，并显式设置 `enabled: true`。默认关闭，配置缺省时不连接。
启用但未带构建标签、DSN 无效或首次连接失败时，`Manager.Initialize` 返回错误并释放
已经创建的连接；成功后通过 `GetMQTT()` 获取官方 Paho 客户端，`Close()` 负责断开并停止重连。

```yaml
mqtt:
  enabled: true
  dsn: "tcp://gateway:mqtt_pwd@mqtt:1883?client_id=my-service&keep_alive=60s&clean_session=true&auto_reconnect=true"
  # TLS：ssl://gateway:mqtt_pwd@mqtt:8883?client_id=my-service
  # WebSocket：ws://mqtt:8083/mqtt?client_id=my-service
```

Paho 没有连接选项的 DSN 查询解析器，网关把 `client_id`、`protocol_version`、
`keep_alive`、`clean_session`、`auto_reconnect`、各超时与遗言参数映射到 Paho setter。
未配置 `will_topic` 时不启用遗言。完整参数、默认值与 URL 转义规则见
[config/queue/mqtt.go](../config/queue/mqtt.go)。原来的 `endpoint`、`client-id`、
`username/password` 及独立超时字段已移除，迁移时须改写为 `dsn` 并设置 `enabled: true`。

```go
mqttClient := gwglobal.GetPoolManager().GetMQTT()
if mqttClient == nil {
    return fmt.Errorf("MQTT is disabled")
}
token := mqttClient.Publish("topic", 0, false, payload)
if !token.WaitTimeout(5 * time.Second) {
    return fmt.Errorf("MQTT publish timed out")
}
if err := token.Error(); err != nil {
    return err
}
```

## SMTP 邮件

> 源码：[cpool/smtp/smtp.go](../cpool/smtp/smtp.go)

```yaml
smtp:
  enabled: true
  # 用户名里的 @ 必须写成 %40，否则会被当成 userinfo 与主机的分隔符
  dsn: "smtp://noreply%40example.com:app_password@smtp.example.com:587"
  # 465 隐式 TLS：  dsn: "smtps://noreply%40example.com:pwd@smtp.example.com:465"
  # 强制 STARTTLS 且跳过自签证书校验：
  #   dsn: "smtp://noreply%40corp.lan:pwd@mail.corp.lan:587?starttls=true&skip_verify=true"
  # 本地中继，无需认证：  dsn: "smtp://127.0.0.1:25"

  # 以下是邮件内容相关字段，不属于连接参数
  from-address: "noreply@example.com"
  to-addresses: ["admin@example.com"]
  headers:
    X-Gateway: "go-rpc-gateway"
```

传输加密方式由 scheme 与 `starttls` 参数共同决定：`smtps://` 走隐式 TLS
（`SendWithTLS`），`smtp://` + `starttls=true` 走显式 STARTTLS（`SendWithStartTLS`，
这条路径才吃得到 `skip_verify`），单纯 `smtp://` 走 `net/smtp` 的默认投递——
服务端声明支持 STARTTLS 就自动升级，不支持就明文。

> 原先的 `pool-size` 字段已移除：`net/smtp` 每次发信自建一条连接、发完即关，
> 没有连接池可配

> 完整 DSN 格式与可粘贴示例见 [config/smtp/smtp.go](../config/smtp/smtp.go) 的 `Smtp` 类型注释

获取连接：

```go
smtpClient := gwglobal.GetPoolManager().GetSMTP()
err := smtpClient.SendEmail(ctx, []string{"user@example.com"}, "Subject", "Body")
err := smtpClient.SendEmailWithHTML(ctx, []string{"user@example.com"}, "Subject", "<h1>HTML</h1>")
```

MailHandler 接口：

```go
type MailHandler interface {
    SendEmail(ctx context.Context, to []string, subject, body string) error
    SendEmailWithHTML(ctx context.Context, to []string, subject, htmlBody string) error
    Close() error
}
```

`SmtpClient` 是 `MailHandler` 的实现，由 `NewSmtpClient(cfg *smtpconfig.Smtp, log logger.ILogger) (*SmtpClient, error)` 创建。

## gRPC 客户端连接池

> 源码：[cpool/grpc/client.go](../cpool/grpc/client.go)

gRPC 客户端连接**不走 `cpool.Manager`**，而是由 `cpool/grpc` 包自己维护一个按服务名缓存 `*grpc.ClientConn` 的包级连接池：

```go
var (
    connPool   = make(map[string]*grpc.ClientConn)
    connPoolMu sync.RWMutex
)
```

| 函数 | 签名 | 说明 |
|------|------|------|
| `GetConn` | `func(serviceName string) (*grpc.ClientConn, bool)` | 读取缓存的连接 |
| `PutConn` | `func(serviceName string, conn *grpc.ClientConn)` | 写入连接 |
| `InitClient[T]` | 见 [gRPC 客户端](./GRPC-CLIENT.md) | 命中缓存则复用，否则 `grpc.NewClient` 建连后入池 |

一个服务名对应一条 `ClientConn`，`InitClient` / `InitClientAny` 重复初始化同一服务会复用已有连接（日志 `♻️ xxx 复用连接池中的已有连接`）；`grpc.NewClient` 不会立即拨号，连接是懒建立的，真正的多路复用由 HTTP/2 在这条 `ClientConn` 内部完成——所以这里不需要「池大小」这类参数，`clientCfg.Endpoints[0]` 背后有多少后端由解析器（DNS / 服务发现）决定

### 三层韧性能力的分工

`ClientConn` 建立时注入的治理能力分三层，职责不重叠，不要互相替代：

| 层 | 组件 | 粒度 | 回答的问题 | 失败时的动作 |
|------|------|------|------|------|
| 连接级 | `HealthChecker`（`cpool/grpc/health.go`） | 服务名 | 这个下游的连接还能用吗 | 周期性检查 `ClientConn` 连通性状态（`ResetConnectBackoff` + `Connect` + `WaitForStateChange`，无 conn 时降级为 TCP 端口探测），不健康时拦截器短路返回 `Unavailable` |
| 调用级 | `ClientBreaker`（`cpool/grpc/breaker.go`） | `target + method`（默认） | 这个逻辑下游的这个方法还值不值得调用 | 打开后就地快速失败，不发请求、不等超时 |
| 后端级 | `p2c_ewma` 负载均衡（`cpool/grpc/balancer.go`） | 单个后端实例（SubConn） | 同一个下游的多个后端里，这次该发给谁 | 把流量拉向延迟更低、在途更少的后端 |

常见的混淆：熔断器的 `target` 是**拨号目标**（`cc.Target()`，即配置里的 endpoint），不是负载均衡后面的某个后端实例；「单个后端变慢」由 p2c 处理，「整个逻辑下游不值得调用」由熔断器处理

### 默认行为

| 能力 | 默认 | 开启方式 |
|------|------|------|
| 健康检查 | 传入 `healthChecker` 时启用 | `InitClient(healthChecker, ...)` + `StartPeriodicCheck` |
| 客户端熔断 | **关闭** | 按服务配 `enable-breaker: true`（可选 `breaker-key-mode`）；需要自定义阈值或与服务端共享 `BreakerManager` 时改用 `grpcpool.EnableClientBreaker(cfg, keyMode)`，必须在初始化客户端之前调用，且优先于配置 |
| 负载均衡 | `round_robin`（`enable-load-balance: true` 时） | `load-balance-policy: "p2c"` 或 `"p2c_ewma"` |

熔断未启用时拦截器根本不加入链；p2c 未选用时只是注册在 gRPC 的 balancer 注册表里不被使用；两者都不改变既有部署的行为

各项的算法细节、配置字段与导出符号见 [gRPC 客户端](./GRPC-CLIENT.md) 的「负载均衡」与「客户端熔断」两节

## 多数据库实例

`database` 是默认实例，`databases` 按名字追加主库、控制库、巡检库等多实例：

```yaml
database:
  enabled: true
  type: postgres
  dsn: "postgres://gateway:pwd@127.0.0.1:5432/gateway?sslmode=disable"
databases:
  control:
    enabled: true
    type: postgres
    dsn: "postgres://gateway:pwd@127.0.0.1:5432/control?sslmode=disable"
```

```go
db := manager.GetDatabase("control")
names := manager.DatabaseNames() // ["control", "default"]
```

命名实例不能叫 `default`，默认实例始终由 `database` 配置，取用统一走
`GetDatabase("default")` 或既有的 `GetDB()`

也可以不经过配置文件，运行时直接注册：

```go
err := manager.AddDatabase(ctx, "inspect", &database.Database{Enabled: true, Type: database.DBTypePostgreSQL, DSN: dsn})
```

## Kafka 生产者与消费者

配置类型在 `config/kafka`，生产与消费分开定义：

```yaml
kafka-producers:
  events:
    brokers: ["kafka-1:9092", "kafka-2:9092"]
    required-acks: -1
    compression: snappy
kafka-consumers:
  commands:
    brokers: ["kafka-1:9092"]
    topic: "channel.command"
    group-id: "channel-service"
    offset: latest
```

**生产者不绑定 Topic**：一条连接可以发往多个 Topic，目标 Topic 在发送时传入，
这与 lm-api、channel-service 存在多 Topic 的场景一致

```go
producer := manager.GetKafkaProducer("events")
err := producer.Send(ctx, "channel.event", key, payload, nil)
err = producer.SendJSON(ctx, "channel.history", key, event, nil)
```

消费者按名字取用；`commit-interval: 0` 时关闭自动提交，使用 `Commit` 手动提交位点

## Tablestore

```yaml
ots:
  enabled: true
  endpoint: "example.cn-hangzhou.ots.aliyuncs.com"
  instance-name: "example"
  http-timeout: 10
```

```go
client := manager.GetOTS("default")
raw := client.Raw() // 底层 Tablestore SDK 客户端
```

## 自定义池注册

对象存储（MinIO / S3 / R2）这类可选 SDK 不进入框架主模块：服务自行创建客户端后
注册给 Manager，统一参与关闭与健康检查

```go
if err := manager.RegisterPool("oss", ossClient); err != nil { // ossClient 实现 Close() error
    return err
}
defer manager.UnregisterPool("oss")
```

实现了 `HealthCheck(ctx context.Context) error` 的自定义池会按实际结果上报，
否则注册成功即视为健康

## 健康检查

```go
status := manager.HealthCheck()
// status = map[string]bool{
//     "database":              true,
//     "database.control":       true,
//     "redis":                  true,
//     "mqtt":                   true,
//     "clickhouse":             true,
//     "nats":                   true,
//     "kafka.producer.events":  true,
//     "kafka.consumer.commands": true,
//     "ots.default":            true,
//     "pool.oss":               true,
// }
```

仅已初始化（非 nil）的连接才会出现在返回 map 中。

## 关闭所有连接

```go
if err := manager.Close(); err != nil {
    logger.Error("Failed to close pool manager: %v", err)
}
```

## 下一步

- [全局变量与初始化器](./GLOBAL.md) — 了解 PoolManager 如何被自动初始化
- [gRPC 客户端](./GRPC-CLIENT.md) — 了解 gRPC 客户端连接管理
