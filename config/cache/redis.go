/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-13 19:54:14
 * @FilePath: \go-rpc-gateway\config\cache\redis.go
 * @Description: Redis 缓存配置
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package cache

import (
	"runtime"
	"time"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// defaultRedisPoolSize 默认连接池大小，跟随 go-redis 官方默认（10 × GOMAXPROCS）
// 硬编码小值（如 10）会在高并发时导致连接池信号量排队，成为吞吐瓶颈
var defaultRedisPoolSize = 10 * runtime.GOMAXPROCS(0)

// Redis 结构体用于配置 Redis 相关参数（增强版配置）
// 字段与 go-redis v9 UniversalOptions 对齐，通过 redis.go 映射到 UniversalOptions
type Redis struct {
	ModuleName string `mapstructure:"module-name" yaml:"module-name" json:"moduleName"` // 模块名
	// 连接 URL（redis:// 单机 / rediss:// TLS），优先于 Addr/Addrs
	URL string `mapstructure:"url" yaml:"url" json:"url"` // Redis 连接 URL，如 redis://user:pass@host:6379/0 或 rediss://host:6380
	// 兼容原有配置
	Addr string `mapstructure:"addr" yaml:"addr" json:"addr" validate:"url"` // Redis 数据服务器 IP 和端口（兼容旧版）
	// 新增增强配置
	Addrs                 []string      `mapstructure:"addrs" yaml:"addrs" json:"addrs"`                                                     // Redis服务器地址列表（集群模式）
	ClientName            string        `mapstructure:"client-name" yaml:"client-name" json:"clientName"`                                    // 客户端名称，执行 CLIENT SETNAME
	Protocol              int           `mapstructure:"protocol" yaml:"protocol" json:"protocol"`                                            // RESP 协议版本（2 或 3）
	Username              string        `mapstructure:"username" yaml:"username" json:"username"`                                            // 用户名
	Password              string        `mapstructure:"password" yaml:"password" json:"password"`                                            // 连接密码
	SentinelUsername      string        `mapstructure:"sentinel-username" yaml:"sentinel-username" json:"sentinelUsername"`                  // 哨兵认证用户名
	SentinelPassword      string        `mapstructure:"sentinel-password" yaml:"sentinel-password" json:"sentinelPassword"`                  // 哨兵认证密码
	DB                    int           `mapstructure:"db" yaml:"db" json:"db" validate:"min=0"`                                             // 指定连接的数据库，默认连数据库 0
	MaxRetries            int           `mapstructure:"max-retries" yaml:"max-retries" json:"maxRetries" validate:"min=0"`                   // 最大重试次数，最小值为 0
	MaxRedirects          int           `mapstructure:"max-redirects" yaml:"max-redirects" json:"maxRedirects" validate:"min=0"`             // 集群模式最大重定向次数
	PoolSize              int           `mapstructure:"pool-size" yaml:"pool-size" json:"poolSize" validate:"min=1"`                         // 连接池大小，最小值为 1
	MaxActiveConns        int           `mapstructure:"max-active-conns" yaml:"max-active-conns" json:"maxActiveConns" validate:"min=0"`     // 最大活跃连接数，0 表示不限制
	MinIdleConns          int           `mapstructure:"min-idle-conns" yaml:"min-idle-conns" json:"minIdleConns" validate:"min=0"`           // 最小空闲连接数，最小值为 0
	MaxIdleConns          int           `mapstructure:"max-idle-conns" yaml:"max-idle-conns" json:"maxIdleConns" validate:"min=0"`           // 最大空闲连接数，最小值为 0
	ReadBufferSize        int           `mapstructure:"read-buffer-size" yaml:"read-buffer-size" json:"readBufferSize" validate:"min=0"`     // 读缓冲区大小（字节），默认 32KB
	WriteBufferSize       int           `mapstructure:"write-buffer-size" yaml:"write-buffer-size" json:"writeBufferSize" validate:"min=0"`  // 写缓冲区大小（字节），默认 32KB
	MaxConnAge            time.Duration `mapstructure:"max-conn-age" yaml:"max-conn-age" json:"maxConnAge"`                                  // 连接最大存活时间
	DialTimeout           time.Duration `mapstructure:"dial-timeout" yaml:"dial-timeout" json:"dialTimeout"`                                 // 连接超时
	PoolTimeout           time.Duration `mapstructure:"pool-timeout" yaml:"pool-timeout" json:"poolTimeout"`                                 // 连接池超时
	IdleTimeout           time.Duration `mapstructure:"idle-timeout" yaml:"idle-timeout" json:"idleTimeout"`                                 // 空闲超时
	ReadTimeout           time.Duration `mapstructure:"read-timeout" yaml:"read-timeout" json:"readTimeout"`                                 // 读取超时
	WriteTimeout          time.Duration `mapstructure:"write-timeout" yaml:"write-timeout" json:"writeTimeout"`                              // 写入超时
	MinRetryBackoff       time.Duration `mapstructure:"min-retry-backoff" yaml:"min-retry-backoff" json:"minRetryBackoff" validate:"min=0"`  // 最小重试间隔
	MaxRetryBackoff       time.Duration `mapstructure:"max-retry-backoff" yaml:"max-retry-backoff" json:"maxRetryBackoff" validate:"min=0"`  // 最大重试间隔
	ContextTimeoutEnabled bool          `mapstructure:"context-timeout-enabled" yaml:"context-timeout-enabled" json:"contextTimeoutEnabled"` // 是否用 context 控制命令超时
	ReadOnly              bool          `mapstructure:"read-only" yaml:"read-only" json:"readOnly"`                                          // 集群模式只读（从节点读）
	ClusterMode           bool          `mapstructure:"cluster-mode" yaml:"cluster-mode" json:"clusterMode"`                                 // 是否集群模式（单地址集群也生效，映射到 go-redis IsClusterMode）
	MasterName            string        `mapstructure:"master-name" yaml:"master-name" json:"masterName"`                                    // 哨兵模式 master 名称，非空时启用哨兵模式
	RouteByLatency        bool          `mapstructure:"route-by-latency" yaml:"route-by-latency" json:"routeByLatency"`                      // 集群模式按延迟路由（只读从节点）
	RouteRandomly         bool          `mapstructure:"route-randomly" yaml:"route-randomly" json:"routeRandomly"`                           // 集群模式随机路由（只读从节点）
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
		r.Addr = configData.Addr
		r.Addrs = configData.Addrs
		r.ClientName = configData.ClientName
		r.Protocol = configData.Protocol
		r.Username = configData.Username
		r.Password = configData.Password
		r.SentinelUsername = configData.SentinelUsername
		r.SentinelPassword = configData.SentinelPassword
		r.DB = configData.DB
		r.MaxRetries = configData.MaxRetries
		r.MaxRedirects = configData.MaxRedirects
		r.PoolSize = configData.PoolSize
		r.MaxActiveConns = configData.MaxActiveConns
		r.MinIdleConns = configData.MinIdleConns
		r.MaxIdleConns = configData.MaxIdleConns
		r.ReadBufferSize = configData.ReadBufferSize
		r.WriteBufferSize = configData.WriteBufferSize
		r.MaxConnAge = configData.MaxConnAge
		r.DialTimeout = configData.DialTimeout
		r.PoolTimeout = configData.PoolTimeout
		r.IdleTimeout = configData.IdleTimeout
		r.ReadTimeout = configData.ReadTimeout
		r.WriteTimeout = configData.WriteTimeout
		r.MinRetryBackoff = configData.MinRetryBackoff
		r.MaxRetryBackoff = configData.MaxRetryBackoff
		r.ContextTimeoutEnabled = configData.ContextTimeoutEnabled
		r.ReadOnly = configData.ReadOnly
		r.ClusterMode = configData.ClusterMode
		r.MasterName = configData.MasterName
		r.RouteByLatency = configData.RouteByLatency
		r.RouteRandomly = configData.RouteRandomly
	}
}

// Validate 验证 Redis 配置的有效性
func (r *Redis) Validate() error {
	// 处理地址列表
	if len(r.Addrs) == 0 && r.Addr != "" {
		r.Addrs = []string{r.Addr}
	}
	if len(r.Addrs) == 0 {
		r.Addrs = []string{"127.0.0.1:6379"}
	}

	// 设置默认值
	if r.MaxRetries < 0 {
		r.MaxRetries = 3
	}
	if r.PoolSize <= 0 {
		r.PoolSize = defaultRedisPoolSize
	}
	if r.MinIdleConns < 0 {
		r.MinIdleConns = 0
	}
	if r.MaxIdleConns < 0 {
		r.MaxIdleConns = 0
	}
	if r.MaxConnAge <= 0 {
		r.MaxConnAge = 30 * time.Minute
	}
	if r.DialTimeout <= 0 {
		r.DialTimeout = 5 * time.Second
	}
	if r.PoolTimeout <= 0 {
		r.PoolTimeout = 4 * time.Second
	}
	if r.IdleTimeout <= 0 {
		r.IdleTimeout = 5 * time.Minute
	}
	if r.ReadTimeout <= 0 {
		r.ReadTimeout = 3 * time.Second
	}
	if r.WriteTimeout <= 0 {
		r.WriteTimeout = 3 * time.Second
	}
	if r.MinRetryBackoff <= 0 {
		r.MinRetryBackoff = 8 * time.Millisecond
	}
	if r.MaxRetryBackoff <= 0 {
		r.MaxRetryBackoff = 512 * time.Millisecond
	}

	return internal.ValidateStruct(r)
}

// DefaultRedisConfig 返回默认Redis配置
func DefaultRedisConfig() Redis {
	return Redis{
		ModuleName:            "redis",
		Addr:                  "127.0.0.1:6379",
		Addrs:                 []string{"127.0.0.1:6379"},
		Username:              "default",
		Password:              "redis123456",
		DB:                    0,
		MaxRetries:            3,
		PoolSize:              defaultRedisPoolSize,
		MinIdleConns:          0,
		MaxIdleConns:          20,
		MaxConnAge:            30 * time.Minute,
		DialTimeout:           5 * time.Second,
		PoolTimeout:           4 * time.Second,
		IdleTimeout:           5 * time.Minute,
		ReadTimeout:           3 * time.Second,
		WriteTimeout:          3 * time.Second,
		MinRetryBackoff:       8 * time.Millisecond,
		MaxRetryBackoff:       512 * time.Millisecond,
		ContextTimeoutEnabled: true,
		ClusterMode:           false,
	}
}

// DefaultRedisConfig 返回默认Redis配置的指针，支持链式调用
func DefaultRedis() *Redis {
	config := DefaultRedisConfig()
	return &config
}

// WithModuleName 设置模块名称
func (r *Redis) WithModuleName(moduleName string) *Redis {
	r.ModuleName = moduleName
	return r
}

// WithURL 设置Redis连接URL（redis:// 或 rediss://），优先于 Addr/Addrs
func (r *Redis) WithURL(url string) *Redis {
	r.URL = url
	return r
}

// WithAddr 设置Redis地址（单实例）
func (r *Redis) WithAddr(addr string) *Redis {
	r.Addr = addr
	if r.Addrs == nil {
		r.Addrs = []string{addr}
	} else if len(r.Addrs) == 1 {
		r.Addrs[0] = addr
	} else {
		r.Addrs = []string{addr}
	}
	return r
}

// WithAddrs 设置Redis地址列表（集群模式）
func (r *Redis) WithAddrs(addrs []string) *Redis {
	r.Addrs = addrs
	if len(addrs) > 0 {
		r.Addr = addrs[0]
	}
	return r
}

// WithUsername 设置用户名
func (r *Redis) WithUsername(username string) *Redis {
	r.Username = username
	return r
}

// WithPassword 设置密码
func (r *Redis) WithPassword(password string) *Redis {
	r.Password = password
	return r
}

// WithDB 设置数据库编号
func (r *Redis) WithDB(db int) *Redis {
	r.DB = db
	return r
}

// WithMaxRetries 设置最大重试次数
func (r *Redis) WithMaxRetries(maxRetries int) *Redis {
	r.MaxRetries = maxRetries
	return r
}

// WithPoolSize 设置连接池大小
func (r *Redis) WithPoolSize(poolSize int) *Redis {
	r.PoolSize = poolSize
	return r
}

// WithMinIdleConns 设置最小空闲连接数
func (r *Redis) WithMinIdleConns(minIdleConns int) *Redis {
	r.MinIdleConns = minIdleConns
	return r
}

// WithMaxConnAge 设置连接最大存活时间
func (r *Redis) WithMaxConnAge(maxConnAge time.Duration) *Redis {
	r.MaxConnAge = maxConnAge
	return r
}

// WithPoolTimeout 设置连接池超时
func (r *Redis) WithPoolTimeout(poolTimeout time.Duration) *Redis {
	r.PoolTimeout = poolTimeout
	return r
}

// WithIdleTimeout 设置空闲超时
func (r *Redis) WithIdleTimeout(idleTimeout time.Duration) *Redis {
	r.IdleTimeout = idleTimeout
	return r
}

// WithReadTimeout 设置读取超时
func (r *Redis) WithReadTimeout(readTimeout time.Duration) *Redis {
	r.ReadTimeout = readTimeout
	return r
}

// WithWriteTimeout 设置写入超时
func (r *Redis) WithWriteTimeout(writeTimeout time.Duration) *Redis {
	r.WriteTimeout = writeTimeout
	return r
}

// WithMinRetryBackoff 设置最小重试间隔
func (r *Redis) WithMinRetryBackoff(minRetryBackoff time.Duration) *Redis {
	r.MinRetryBackoff = minRetryBackoff
	return r
}

// WithMaxRetryBackoff 设置最大重试间隔
func (r *Redis) WithMaxRetryBackoff(maxRetryBackoff time.Duration) *Redis {
	r.MaxRetryBackoff = maxRetryBackoff
	return r
}

// WithClusterMode 设置是否集群模式
func (r *Redis) WithClusterMode(clusterMode bool) *Redis {
	r.ClusterMode = clusterMode
	return r
}

// WithMasterName 设置哨兵模式 master 名称
func (r *Redis) WithMasterName(masterName string) *Redis {
	r.MasterName = masterName
	return r
}

// WithRouteByLatency 设置集群模式按延迟路由
func (r *Redis) WithRouteByLatency(routeByLatency bool) *Redis {
	r.RouteByLatency = routeByLatency
	return r
}

// WithRouteRandomly 设置集群模式随机路由
func (r *Redis) WithRouteRandomly(routeRandomly bool) *Redis {
	r.RouteRandomly = routeRandomly
	return r
}

// WithClientName 设置客户端名称
func (r *Redis) WithClientName(clientName string) *Redis {
	r.ClientName = clientName
	return r
}

// WithProtocol 设置 RESP 协议版本
func (r *Redis) WithProtocol(protocol int) *Redis {
	r.Protocol = protocol
	return r
}

// WithSentinelUsername 设置哨兵认证用户名
func (r *Redis) WithSentinelUsername(username string) *Redis {
	r.SentinelUsername = username
	return r
}

// WithSentinelPassword 设置哨兵认证密码
func (r *Redis) WithSentinelPassword(password string) *Redis {
	r.SentinelPassword = password
	return r
}

// WithMaxRedirects 设置集群最大重定向次数
func (r *Redis) WithMaxRedirects(maxRedirects int) *Redis {
	r.MaxRedirects = maxRedirects
	return r
}

// WithMaxActiveConns 设置最大活跃连接数
func (r *Redis) WithMaxActiveConns(maxActiveConns int) *Redis {
	r.MaxActiveConns = maxActiveConns
	return r
}

// WithReadBufferSize 设置读缓冲区大小
func (r *Redis) WithReadBufferSize(readBufferSize int) *Redis {
	r.ReadBufferSize = readBufferSize
	return r
}

// WithWriteBufferSize 设置写缓冲区大小
func (r *Redis) WithWriteBufferSize(writeBufferSize int) *Redis {
	r.WriteBufferSize = writeBufferSize
	return r
}

// WithContextTimeoutEnabled 设置是否用 context 控制命令超时
func (r *Redis) WithContextTimeoutEnabled(enabled bool) *Redis {
	r.ContextTimeoutEnabled = enabled
	return r
}

// WithReadOnly 设置集群模式只读
func (r *Redis) WithReadOnly(readOnly bool) *Redis {
	r.ReadOnly = readOnly
	return r
}
