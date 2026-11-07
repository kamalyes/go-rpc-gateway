/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-13 19:54:14
 * @FilePath: \go-rpc-gateway\config\cache\redis.go
 * @Description: Redis 缓存配置（单 DSN 方案）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package cache

import (
	"fmt"
	"net/url"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Redis Redis 连接配置（单 DSN）
//
// 连接信息、认证、超时、连接池参数全部写在 URL 一个字段里，由 go-redis 官方解析器解析
// （ParseURL / ParseClusterURL / ParseFailoverURL，见 cpool/redis）。
// 这些参数原先是 30 多个独立配置字段，与 go-redis 的 URL 查询参数一一重复，
// 收进 DSN 后配置面只剩两项，且参数名直接跟官方文档对齐，不再需要维护一份映射表
//
// ─── DSN 格式 ───────────────────────────────────────────────────────────────
//
//	redis://[用户名:密码@]主机:端口[/数据库号][?参数=值&...]     明文连接
//	rediss://[用户名:密码@]主机:端口[/数据库号][?参数=值&...]    TLS 连接
//
// ─── DSN 示例 ───────────────────────────────────────────────────────────────
//
//	单机，最简形式（端口默认 6379，库默认 0）：
//	  redis://127.0.0.1:6379
//
//	单机，带 ACL 用户名与密码，选 3 号库：
//	  redis://default:redis123456@127.0.0.1:6379/3
//
//	单机，只有密码没有用户名（Redis 6 以前的 requirepass）：
//	  redis://:redis123456@127.0.0.1:6379/0
//
//	单机，调连接池与超时：
//	  redis://default:pwd@127.0.0.1:6379/0?pool_size=200&min_idle_conns=10&dial_timeout=5s&read_timeout=3s&write_timeout=3s
//
//	TLS，跳过证书校验（自签证书环境）：
//	  rediss://default:pwd@redis.example.com:6380/0?skip_verify=true
//
//	集群，多节点用重复的 addr 参数追加（第一个节点写在 host 位）：
//	  redis://default:pwd@10.0.0.1:6379?addr=10.0.0.2:6379&addr=10.0.0.3:6379&route_by_latency=true
//
//	集群，只有一个入口地址（如云厂商的集群代理端点）——单地址无法自动判定，
//	需要显式加网关扩展参数 cluster=true：
//	  redis://default:pwd@redis-cluster.example.com:6379?cluster=true&max_redirects=3
//
//	哨兵，master_name 非空即走哨兵模式，host 位与 addr 参数是哨兵地址，
//	userinfo 是哨兵自己的认证，数据节点认证用 username/password 查询参数：
//	  redis://sentinel_user:sentinel_pwd@10.0.0.1:26379/0?master_name=mymaster&addr=10.0.0.2:26379&username=default&password=data_pwd
//
// ─── go-redis 原生查询参数 ──────────────────────────────────────────────────
//
// 由 go-redis v9 解析，未知参数会被直接拒绝（报 "redis: unexpected option"），填错不会被静默忽略。
// 时长类参数带单位走 time.ParseDuration，纯数字按秒解释；填 0 或负数表示禁用该项，留空走官方默认
//
//	三种模式都支持：
//	  protocol                 RESP 协议版本，2 或 3
//	  client_name              客户端名称，连上后执行 CLIENT SETNAME
//	  max_retries              命令最大重试次数
//	  min_retry_backoff        最小重试间隔
//	  max_retry_backoff        最大重试间隔
//	  dial_timeout             建连超时，官方默认 5s
//	  read_timeout             读超时，官方默认 3s
//	  write_timeout            写超时，默认取 read_timeout
//	  pool_fifo                连接池取连接用 FIFO（默认 LIFO）
//	  pool_size                每节点连接池大小，官方默认 10 × GOMAXPROCS
//	  pool_timeout             连接池满时的等待超时，官方默认 read_timeout + 1s
//	  min_idle_conns           最小空闲连接数
//	  max_idle_conns           最大空闲连接数
//	  max_active_conns         最大活跃连接数，0 不限制
//	  max_concurrent_dials     并发建连上限
//	  conn_max_idle_time       连接最大空闲时间（旧名 idle_timeout 仍可用），官方默认 30m
//	  conn_max_lifetime        连接最大存活时间（旧名 max_conn_age 仍可用）
//	  conn_max_lifetime_jitter 存活时间抖动，取值不超过 conn_max_lifetime
//	  skip_verify              跳过 TLS 证书校验，仅 rediss:// 生效
//
//	仅单机（redis:// 单地址且无 cluster/master_name）额外支持：
//	  db                       数据库号，等价于写在路径上的 /3
//
//	仅集群额外支持：
//	  addr                     追加节点地址，可重复出现
//	  max_redirects            MOVED/ASK 最大重定向次数
//	  read_only                允许从只读从节点读
//	  route_by_latency         按延迟路由到最快的从节点
//	  route_randomly           随机路由到从节点
//	  dialer_retries           建连重试次数
//	  dialer_retry_timeout     建连重试超时
//	  failing_timeout_seconds  节点标记失败后的冷却秒数
//
//	仅哨兵额外支持：
//	  master_name               哨兵 master 名称，非空即启用哨兵模式
//	  addr                      追加哨兵地址，可重复出现
//	  username / password       数据节点认证（userinfo 位置留给哨兵自身认证）
//	  replica_only              只连从节点
//	  use_disconnected_replicas 允许使用哨兵报告为断开的从节点
//	  route_by_latency          按延迟路由
//	  route_randomly            随机路由
//	  context_timeout_enabled   用 context 截止时间控制命令超时
//
// ─── 网关扩展查询参数 ───────────────────────────────────────────────────────
//
// go-redis 的 URL 解析器不认识这几项，由 cpool/redis 在交给驱动之前摘出来自行应用
// （摘取逻辑见 cpool/internal/dsnx）：
//
//	cluster                 单地址也按集群模式连接；多地址（重复 addr）会自动判定为集群，无需此项
//	read_buffer_size        每连接读缓冲区字节数，官方默认 32KB
//	write_buffer_size       每连接写缓冲区字节数，官方默认 32KB
//	context_timeout_enabled 用 context 截止时间控制命令超时（哨兵模式由 go-redis 原生支持，
//	                        单机与集群模式由网关补齐）
//
// [EN] Redis connection config, collapsed to a single DSN. Endpoint, credentials,
// timeouts and pool sizing all live in URL and are parsed by go-redis's own parsers
// (ParseURL / ParseClusterURL / ParseFailoverURL, see cpool/redis). These used to be 30-odd
// struct fields duplicating go-redis's URL query parameters one for one; folding them into
// the DSN leaves two fields and names every knob exactly as the upstream docs do, so there
// is no mapping table left to maintain. Unknown query parameters are rejected by go-redis
// rather than silently dropped, except for the four gateway extensions listed above, which
// cpool/redis strips before the driver sees the DSN
type Redis struct {
	ModuleName string `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`               // 模块名
	URL        string `mapstructure:"url" yaml:"url" json:"url" validate:"required,startswith=redis"` // 连接 DSN，格式与示例见类型注释
}

// NewRedis 创建一个新的 Redis 实例
func NewRedis(opt *Redis) *Redis {
	var redisInstance *Redis

	internal.LockFunc(func() {
		redisInstance = opt
	})
	return redisInstance
}

// Clone 返回 Redis 配置的副本
func (r *Redis) Clone() internal.Configurable {
	var cloned Redis
	if err := syncx.DeepCopy(&cloned, r); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Redis{}
	}
	return &cloned
}

// Get 返回 Redis 配置的所有字段
func (r *Redis) Get() interface{} {
	return r
}

// Set 更新 Redis 配置的字段
func (r *Redis) Set(data interface{}) {
	if configData, ok := data.(*Redis); ok {
		r.ModuleName = configData.ModuleName
		r.URL = configData.URL
	}
}

// Validate 验证 Redis 配置的有效性
//
// 只做「能不能解析、scheme 对不对」这一层校验，查询参数交给 go-redis 在建连时报错：
// 参数名与语义都由 go-redis 定义，在这里复制一份白名单必然跟着版本漂移
//
// [EN] Validate only checks that the DSN parses and carries a redis scheme; query
// parameters are left for go-redis to reject at connect time, since duplicating its
// parameter whitelist here would drift against the driver version
func (r *Redis) Validate() error {
	if r.URL == "" {
		return fmt.Errorf("redis url is empty")
	}

	u, err := url.Parse(r.URL)
	if err != nil {
		return fmt.Errorf("redis url is invalid: %w", err)
	}
	if u.Scheme != "redis" && u.Scheme != "rediss" {
		return fmt.Errorf("redis url scheme must be redis or rediss, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("redis url has no host: %s", r.URL)
	}

	return internal.ValidateStruct(r)
}

// DefaultRedisConfig 返回默认Redis配置
func DefaultRedisConfig() Redis {
	return Redis{
		ModuleName: "redis",
		URL:        "redis://default:redis123456@127.0.0.1:6379/0",
	}
}

// DefaultRedis 返回默认Redis配置的指针，支持链式调用
func DefaultRedis() *Redis {
	config := DefaultRedisConfig()
	return &config
}

// WithModuleName 设置模块名称
func (r *Redis) WithModuleName(moduleName string) *Redis {
	r.ModuleName = moduleName
	return r
}

// WithURL 设置 Redis 连接 DSN，格式与示例见 Redis 类型注释
func (r *Redis) WithURL(url string) *Redis {
	r.URL = url
	return r
}
