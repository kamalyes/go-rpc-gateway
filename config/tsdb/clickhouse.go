/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-23 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-23 19:58:55
 * @FilePath: \go-rpc-gateway\config\tsdb\clickhouse.go
 * @Description: ClickHouse数据库配置（单 DSN 方案）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package tsdb

import (
	"fmt"
	"net/url"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// ClickHouse ClickHouse 数据库配置（单 DSN）
//
// 连接信息、认证、协议、TLS、压缩、超时、连接池参数全部写在 DSN 一个字段里，
// 由 clickhouse-go 官方解析器 clickhouse.ParseDSN 解析（见 cpool/clickhouse）。
// 保留的字段只有 GORM 行为开关——那些是 ORM 层的事，连接 DSN 表达不了
//
// ─── DSN 格式 ───────────────────────────────────────────────────────────────
//
//	clickhouse://[用户名:密码@]主机:端口[/数据库名][?参数=值&...]   原生 TCP 协议，端口默认 9000
//	http://[用户名:密码@]主机:端口[/数据库名][?参数=值&...]         HTTP 协议，端口默认 8123
//	https://[用户名:密码@]主机:端口[/数据库名][?参数=值&...]        HTTPS，必须同时带 secure=true
//
// ─── DSN 示例 ───────────────────────────────────────────────────────────────
//
//	本地开发，最简形式（用户名默认 default，库默认 default）：
//	  clickhouse://127.0.0.1:9000/default
//
//	带认证与超时，LZ4 压缩：
//	  clickhouse://default:ch_pwd@127.0.0.1:9000/metrics?dial_timeout=10s&read_timeout=20s&compress=lz4
//
//	多节点轮询（alt_hosts 追加备用节点，连接策略用 connection_open_strategy）：
//	  clickhouse://default:ch_pwd@ch-1:9000/metrics?alt_hosts=ch-2:9000,ch-3:9000&connection_open_strategy=round_robin
//
//	连接池与生命周期（max_idle_conns 官方默认 5，max_open_conns 默认 max_idle_conns+5）：
//	  clickhouse://default:ch_pwd@127.0.0.1:9000/metrics?max_idle_conns=5&max_open_conns=20&conn_max_lifetime=1h&conn_max_idle_time=5m
//
//	TLS，自签证书跳过校验：
//	  clickhouse://default:ch_pwd@ch.example.com:9440/metrics?secure=true&skip_verify=true
//
//	HTTP 协议（走负载均衡器或只开了 8123 的环境）：
//	  http://default:ch_pwd@127.0.0.1:8123/metrics?dial_timeout=10s
//
//	ClickHouse Cloud（HTTPS + TLS）：
//	  https://default:ch_pwd@xxx.clickhouse.cloud:8443/metrics?secure=true
//
//	覆盖会话级 ClickHouse setting（驱动不认识的参数一律当 setting 下发，
//	所以服务端设置名写错会在建连时报 "code: 115 Unknown setting"）：
//	  clickhouse://default:ch_pwd@127.0.0.1:9000/metrics?session_timezone=Asia/Shanghai&max_execution_time=60
//
// ─── clickhouse-go 原生查询参数 ─────────────────────────────────────────────
//
// 由 clickhouse-go v2 的 Options.fromDSN 解析（clickhouse_options.go）：
//
//	username / password        认证信息，等价于写在 userinfo 位置
//	database                   数据库名，等价于写在路径上
//	hosts                      节点地址列表，逗号分隔，插在 host 位之前
//	alt_hosts                  备用节点地址列表，逗号分隔，追加在 host 位之后
//	connection_open_strategy    连接选择策略：in_order | round_robin | random
//	dial_timeout               建连超时，官方默认 30s
//	read_timeout               单次读超时，官方默认 300s
//	secure                     启用 TLS，不带值（?secure）即为 true
//	skip_verify                跳过 TLS 证书校验，不带值即为 true
//	tls_server_name            TLS SNI 名称，必须与 secure=true 同时出现
//	compress                   压缩方式：true（等价 lz4）| lz4 | zstd | gzip | deflate | br | none
//	compress_level             压缩级别，单独给级别不会启用压缩
//	max_compression_buffer     压缩缓冲区上限（字节），官方默认 10485760
//	max_open_conns             最大打开连接数，官方默认 max_idle_conns + 5
//	max_idle_conns             最大空闲连接数，官方默认 5
//	conn_max_lifetime          连接最大存活时间，官方默认 1h
//	block_buffer_size          数据块缓冲区个数，官方默认 2
//	debug                      驱动调试日志，同时把 GORM 的 SQL 日志抬到 info 级
//	http_proxy                 HTTP 代理地址，仅 HTTP 协议生效
//	http_path                  HTTP 请求附加路径，仅 HTTP 协议生效
//	client_info_product        上报给服务端的客户端产品标识，形如 name/version
//	其余任何参数               一律当 ClickHouse 会话 setting 原样下发
//
// ─── 网关扩展查询参数 ───────────────────────────────────────────────────────
//
// clickhouse-go 的 Options 没有这项（它只有 ConnMaxLifetime），
// 不摘出来会被当成服务端 setting 下发并报 Unknown setting，
// 所以由 cpool/clickhouse 摘出后作用到 database/sql 层（见 cpool/internal/dsnx）：
//
//	conn_max_idle_time         连接最大空闲时间，作用于 sql.DB.SetConnMaxIdleTime
//
// 另外 cpool/clickhouse 在 DSN 没写 session_timezone 时会补一个 UTC 默认值，
// 原因见 cpool/clickhouse/client_support.go 里 defaultSessionTimezone 的注释
//
// [EN] ClickHouse config, collapsed to a single DSN. Endpoint, credentials, protocol, TLS,
// compression, timeouts and pool sizing all live in DSN and are parsed by clickhouse-go's
// own clickhouse.ParseDSN (see cpool/clickhouse); the only fields kept are GORM behaviour
// switches, which belong to the ORM layer and cannot be expressed in a connection DSN.
// Any parameter the driver does not recognise is forwarded to the server as a session
// setting, so a misspelled setting name fails at connect time with "code: 115 Unknown
// setting" rather than being silently dropped
type ClickHouse struct {
	ModuleName string `mapstructure:"module-name" yaml:"module-name" json:"moduleName"` // 模块名称
	Enabled    bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`            // 是否启用
	DSN        string `mapstructure:"dsn" yaml:"dsn" json:"dsn" validate:"required"`    // 连接 DSN，格式与示例见类型注释

	// GORM 行为开关（ORM 层配置，DSN 表达不了）
	LogLevel                                 string `mapstructure:"log-level" yaml:"log-level" json:"logLevel" validate:"required"`                                                                                     // SQL 日志等级 (silent/error/warn/info)
	SlowThreshold                            int    `mapstructure:"slow-threshold" yaml:"slow-threshold" json:"slowThreshold"`                                                                                          // 慢查询阈值（毫秒）
	IgnoreRecordNotFoundError                bool   `mapstructure:"ignore-record-not-found-error" yaml:"ignore-record-not-found-error" json:"ignoreRecordNotFoundError"`                                                // 是否忽略ErrRecordNotFound错误
	SkipDefaultTransaction                   bool   `mapstructure:"skip-default-transaction" yaml:"skip-default-transaction" json:"skipDefaultTransaction"`                                                             // 跳过默认事务
	PrepareStmt                              bool   `mapstructure:"prepare-stmt" yaml:"prepare-stmt" json:"prepareStmt"`                                                                                                // 预编译语句
	DisableForeignKeyConstraintWhenMigrating bool   `mapstructure:"disable-foreign-key-constraint-when-migrating" yaml:"disable-foreign-key-constraint-when-migrating" json:"disableForeignKeyConstraintWhenMigrating"` // 禁用自动创建外键约束（ClickHouse不支持外键）
	DisableNestedTransaction                 bool   `mapstructure:"disable-nested-transaction" yaml:"disable-nested-transaction" json:"disableNestedTransaction"`                                                       // 禁用嵌套事务
	AllowGlobalUpdate                        bool   `mapstructure:"allow-global-update" yaml:"allow-global-update" json:"allowGlobalUpdate"`                                                                            // 允许全局更新
	QueryFields                              bool   `mapstructure:"query-fields" yaml:"query-fields" json:"queryFields"`                                                                                                // 执行查询时选择所有字段
	CreateBatchSize                          int    `mapstructure:"create-batch-size" yaml:"create-batch-size" json:"createBatchSize"`                                                                                  // 批量创建大小
	SingularTable                            bool   `mapstructure:"singular-table" yaml:"singular-table" json:"singularTable"`                                                                                          // 使用单数表名
}

func (c *ClickHouse) GetModuleName() string              { return c.ModuleName }
func (c *ClickHouse) GetDSN() string                     { return c.DSN }
func (c *ClickHouse) GetSlowThreshold() int              { return c.SlowThreshold }
func (c *ClickHouse) GetIgnoreRecordNotFoundError() bool { return c.IgnoreRecordNotFoundError }
func (c *ClickHouse) GetSkipDefaultTransaction() bool    { return c.SkipDefaultTransaction }
func (c *ClickHouse) GetPrepareStmt() bool               { return c.PrepareStmt }
func (c *ClickHouse) GetDisableForeignKeyConstraintWhenMigrating() bool {
	return c.DisableForeignKeyConstraintWhenMigrating
}
func (c *ClickHouse) GetDisableNestedTransaction() bool { return c.DisableNestedTransaction }
func (c *ClickHouse) GetAllowGlobalUpdate() bool        { return c.AllowGlobalUpdate }
func (c *ClickHouse) GetQueryFields() bool              { return c.QueryFields }
func (c *ClickHouse) GetCreateBatchSize() int           { return c.CreateBatchSize }
func (c *ClickHouse) GetSingularTable() bool            { return c.SingularTable }

func (c *ClickHouse) Clone() internal.Configurable {
	var cloned ClickHouse
	if err := syncx.DeepCopy(&cloned, c); err != nil {
		// 如果深拷贝失败，返回空配置
		return &ClickHouse{}
	}
	return &cloned
}
func (c *ClickHouse) Get() interface{} { return c }
func (c *ClickHouse) Set(data interface{}) {
	if cfg, ok := data.(*ClickHouse); ok {
		*c = *cfg
	}
}

// Validate 验证 ClickHouse 配置的有效性
//
// 未启用时不校验 DSN：Enabled=false 的模块允许留空配置
// DSN 只校验「能不能解析、scheme 对不对」，查询参数交给 clickhouse-go 在建连时报错——参数名由驱动定义，在这里复制一份白名单必然跟着版本漂移
//
// [EN] Validate skips the DSN entirely when the module is disabled, and otherwise only
// checks that it parses and carries a known scheme; query parameters are left for
// clickhouse-go to reject at connect time, since duplicating its parameter list here would
// drift against the driver version
func (c *ClickHouse) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.DSN == "" {
		return fmt.Errorf("clickhouse dsn is empty")
	}

	u, err := url.Parse(c.DSN)
	if err != nil {
		return fmt.Errorf("clickhouse dsn is invalid: %w", err)
	}
	switch u.Scheme {
	case "clickhouse", "http", "https":
	default:
		return fmt.Errorf("clickhouse dsn scheme must be clickhouse, http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("clickhouse dsn has no host: %s", c.DSN)
	}

	return internal.ValidateStruct(c)
}

// DefaultClickHouse 创建默认ClickHouse配置
func DefaultClickHouse() *ClickHouse {
	return &ClickHouse{
		ModuleName: "clickhouse",
		Enabled:    false,
		// 默认 DSN 显式写出压缩与超时：ClickHouse 是分析型负载，
		// 走官方默认的 30s 建连 / 300s 读超时会让故障节点拖着请求不返回
		DSN:                                      "clickhouse://default@127.0.0.1:9000/default?dial_timeout=10s&read_timeout=20s&compress=lz4&max_idle_conns=5&max_open_conns=20&conn_max_lifetime=1h&conn_max_idle_time=5m",
		LogLevel:                                 "info",
		SlowThreshold:                            200,   // ClickHouse 分析型查询相对较慢，阈值设置较大
		IgnoreRecordNotFoundError:                false, //
		SkipDefaultTransaction:                   true,  // ClickHouse不支持传统事务
		PrepareStmt:                              false, // ClickHouse对预编译语句支持有限
		DisableForeignKeyConstraintWhenMigrating: true,  // ClickHouse不支持外键
		DisableNestedTransaction:                 true,
		AllowGlobalUpdate:                        false,
		QueryFields:                              true,
		CreateBatchSize:                          1000, // ClickHouse适合大批量写入
		SingularTable:                            true,
	}
}

// NewClickHouseConfig 创建一个新的ClickHouse配置实例
func NewClickHouseConfig(opt *ClickHouse) *ClickHouse {
	var clickhouseInstance *ClickHouse
	internal.LockFunc(func() {
		clickhouseInstance = opt
	})
	return clickhouseInstance
}

// WithModuleName 设置模块名称
func (c *ClickHouse) WithModuleName(moduleName string) *ClickHouse {
	c.ModuleName = moduleName
	return c
}

// WithEnabled 设置是否启用ClickHouse
func (c *ClickHouse) WithEnabled(enabled bool) *ClickHouse {
	c.Enabled = enabled
	return c
}

// IsEnabled 返回是否启用ClickHouse
func (c *ClickHouse) IsEnabled() bool {
	return c.Enabled
}

// WithDSN 设置连接 DSN，格式与示例见 ClickHouse 类型注释
func (c *ClickHouse) WithDSN(dsn string) *ClickHouse {
	c.DSN = dsn
	return c
}

// WithLogLevel 设置SQL日志等级
func (c *ClickHouse) WithLogLevel(logLevel string) *ClickHouse {
	c.LogLevel = logLevel
	return c
}

// WithSlowThreshold 设置慢查询阈值（毫秒）
func (c *ClickHouse) WithSlowThreshold(slowThreshold int) *ClickHouse {
	c.SlowThreshold = slowThreshold
	return c
}

// WithIgnoreRecordNotFoundError 设置是否忽略记录未找到错误
func (c *ClickHouse) WithIgnoreRecordNotFoundError(ignore bool) *ClickHouse {
	c.IgnoreRecordNotFoundError = ignore
	return c
}

// WithSkipDefaultTransaction 设置是否跳过默认事务
func (c *ClickHouse) WithSkipDefaultTransaction(skip bool) *ClickHouse {
	c.SkipDefaultTransaction = skip
	return c
}

// WithPrepareStmt 设置是否启用预编译语句
func (c *ClickHouse) WithPrepareStmt(prepare bool) *ClickHouse {
	c.PrepareStmt = prepare
	return c
}

// WithCreateBatchSize 设置批量创建大小
func (c *ClickHouse) WithCreateBatchSize(size int) *ClickHouse {
	c.CreateBatchSize = size
	return c
}

// WithSingularTable 设置是否使用单数表名
func (c *ClickHouse) WithSingularTable(singular bool) *ClickHouse {
	c.SingularTable = singular
	return c
}
