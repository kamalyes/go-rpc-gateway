/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-23 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-23 19:58:55
 * @FilePath: \go-rpc-gateway\config\tsdb\clickhouse.go
 * @Description: ClickHouse数据库配置
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package tsdb

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// ClickHouse ClickHouse数据库配置
type ClickHouse struct {
	ModuleName                               string `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`                                                                                                   // 模块名称
	Enabled                                  bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                                                                                                              // 是否启用
	Host                                     string `mapstructure:"host" yaml:"host" json:"host"            validate:"required"`                                                                                        // 数据库 IP 地址
	Port                                     string `mapstructure:"port" yaml:"port" json:"port"            validate:"required"`                                                                                        // 端口（原生协议默认9000，HTTP协议默认8123）
	Config                                   string `mapstructure:"config" yaml:"config" json:"config"          validate:"required"`                                                                                    // 后缀配置 例如: dial_timeout=10s&read_timeout=20s
	LogLevel                                 string `mapstructure:"log-level" yaml:"log-level" json:"logLevel"       validate:"required"`                                                                               // SQL 日志等级
	SlowThreshold                            int    `mapstructure:"slow-threshold" yaml:"slow-threshold" json:"slowThreshold"`                                                                                          // 慢查询阈值（毫秒）
	IgnoreRecordNotFoundError                bool   `mapstructure:"ignore-record-not-found-error" yaml:"ignore-record-not-found-error" json:"ignoreRecordNotFoundError"`                                                // 是否忽略ErrRecordNotFound错误
	Dbname                                   string `mapstructure:"db-name" yaml:"db-name" json:"dbName"         validate:"required"`                                                                                   // 数据库名称
	Username                                 string `mapstructure:"username" yaml:"username" json:"username"        validate:"required"`                                                                                // 数据库用户名
	Password                                 string `mapstructure:"password" yaml:"password" json:"password"`                                                                                                           // 数据库密码（ClickHouse允许为空）
	Protocol                                 string `mapstructure:"protocol" yaml:"protocol" json:"protocol"`                                                                                                           // 协议类型：native（默认）或 http
	Cluster                                  string `mapstructure:"cluster" yaml:"cluster" json:"cluster"`                                                                                                              // ClickHouse集群名称（集群模式使用）
	Compress                                 bool   `mapstructure:"compress" yaml:"compress" json:"compress"`                                                                                                           // 是否启用压缩
	Debug                                    bool   `mapstructure:"debug" yaml:"debug" json:"debug"`                                                                                                                    // 是否启用调试模式
	Secure                                   bool   `mapstructure:"secure" yaml:"secure" json:"secure"`                                                                                                                 // 是否启用TLS安全连接
	DialTimeout                              int    `mapstructure:"dial-timeout" yaml:"dial-timeout" json:"dialTimeout" validate:"min=0"`                                                                               // 连接超时（秒）
	ReadTimeout                              int    `mapstructure:"read-timeout" yaml:"read-timeout" json:"readTimeout" validate:"min=0"`                                                                               // 读取超时（秒）
	MaxIdleConns                             int    `mapstructure:"max-idle-conns" yaml:"max-idle-conns" json:"maxIdleConns"  validate:"min=0"`                                                                         // 最大空闲连接数
	MaxOpenConns                             int    `mapstructure:"max-open-conns" yaml:"max-open-conns" json:"maxOpenConns"  validate:"min=0"`                                                                         // 最大连接数
	ConnMaxIdleTime                          int    `mapstructure:"conn-max-idle-time" yaml:"conn-max-idle-time" json:"connMaxIdleTime" validate:"min=0"`                                                               // 连接最大空闲时间 单位：秒
	ConnMaxLifeTime                          int    `mapstructure:"conn-max-life-time" yaml:"conn-max-life-time" json:"connMaxLifeTime" validate:"min=0"`                                                               // 连接最大生命周期 单位：秒
	SkipDefaultTransaction                   bool   `mapstructure:"skip-default-transaction" yaml:"skip-default-transaction" json:"skipDefaultTransaction"`                                                             // 跳过默认事务
	PrepareStmt                              bool   `mapstructure:"prepare-stmt" yaml:"prepare-stmt" json:"prepareStmt"`                                                                                                // 预编译语句
	DisableForeignKeyConstraintWhenMigrating bool   `mapstructure:"disable-foreign-key-constraint-when-migrating" yaml:"disable-foreign-key-constraint-when-migrating" json:"disableForeignKeyConstraintWhenMigrating"` // 禁用自动创建外键约束（ClickHouse不支持外键）
	DisableNestedTransaction                 bool   `mapstructure:"disable-nested-transaction" yaml:"disable-nested-transaction" json:"disableNestedTransaction"`                                                       // 禁用嵌套事务
	AllowGlobalUpdate                        bool   `mapstructure:"allow-global-update" yaml:"allow-global-update" json:"allowGlobalUpdate"`                                                                            // 允许全局更新
	QueryFields                              bool   `mapstructure:"query-fields" yaml:"query-fields" json:"queryFields"`                                                                                                // 执行查询时选择所有字段
	CreateBatchSize                          int    `mapstructure:"create-batch-size" yaml:"create-batch-size" json:"createBatchSize"`                                                                                  // 批量创建大小
	SingularTable                            bool   `mapstructure:"singular-table" yaml:"singular-table" json:"singularTable"`                                                                                          // 使用单数表名
}

// 为ClickHouse配置实现DatabaseProvider接口
func (c *ClickHouse) GetHost() string                    { return c.Host }
func (c *ClickHouse) GetPort() string                    { return c.Port }
func (c *ClickHouse) GetDBName() string                  { return c.Dbname }
func (c *ClickHouse) GetUsername() string                { return c.Username }
func (c *ClickHouse) GetPassword() string                { return c.Password }
func (c *ClickHouse) GetConfig() string                  { return c.Config }
func (c *ClickHouse) GetModuleName() string              { return c.ModuleName }
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
func (c *ClickHouse) SetCredentials(username, password string) {
	c.Username, c.Password = username, password
}
func (c *ClickHouse) SetHost(host string)     { c.Host = host }
func (c *ClickHouse) SetPort(port string)     { c.Port = port }
func (c *ClickHouse) SetDBName(dbName string) { c.Dbname = dbName }

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
func (c *ClickHouse) Validate() error { return internal.ValidateStruct(c) }

// DefaultClickHouse 创建默认ClickHouse配置
func DefaultClickHouse() *ClickHouse {
	return &ClickHouse{
		ModuleName:                               "clickhouse",
		Enabled:                                  false,
		Host:                                     "localhost",
		Port:                                     "9000",
		Config:                                   "dial_timeout=10s&read_timeout=20s",
		LogLevel:                                 "info",
		SlowThreshold:                            200, // ClickHouse 分析型查询相对较慢，阈值设置较大
		IgnoreRecordNotFoundError:                false,
		Dbname:                                   "default",
		Username:                                 "default",
		Password:                                 "",
		Protocol:                                 "native",
		Cluster:                                  "",
		Compress:                                 true,
		Debug:                                    false,
		Secure:                                   false,
		DialTimeout:                              10, // 10秒
		ReadTimeout:                              20, // 20秒
		MaxIdleConns:                             5,
		MaxOpenConns:                             20,
		ConnMaxIdleTime:                          300,   // 5分钟
		ConnMaxLifeTime:                          3600,  // 1小时
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

// IsEnabled 检查ClickHouse是否启用
func (c *ClickHouse) IsEnabled() bool {
	return c != nil && c.Enabled
}

// WithHost 设置数据库主机地址
func (c *ClickHouse) WithHost(host string) *ClickHouse {
	c.Host = host
	return c
}

// WithPort 设置数据库端口
func (c *ClickHouse) WithPort(port string) *ClickHouse {
	c.Port = port
	return c
}

// WithConfig 设置数据库配置参数
func (c *ClickHouse) WithConfig(config string) *ClickHouse {
	c.Config = config
	return c
}

// WithLogLevel 设置日志级别
func (c *ClickHouse) WithLogLevel(logLevel string) *ClickHouse {
	c.LogLevel = logLevel
	return c
}

// WithDbname 设置数据库名称
func (c *ClickHouse) WithDbname(dbname string) *ClickHouse {
	c.Dbname = dbname
	return c
}

// WithUsername 设置数据库用户名
func (c *ClickHouse) WithUsername(username string) *ClickHouse {
	c.Username = username
	return c
}

// WithPassword 设置数据库密码
func (c *ClickHouse) WithPassword(password string) *ClickHouse {
	c.Password = password
	return c
}

// WithProtocol 设置通信协议 (native 或 http)
func (c *ClickHouse) WithProtocol(protocol string) *ClickHouse {
	c.Protocol = protocol
	return c
}

// WithCluster 设置集群名称
func (c *ClickHouse) WithCluster(cluster string) *ClickHouse {
	c.Cluster = cluster
	return c
}

// WithCompress 设置是否启用压缩
func (c *ClickHouse) WithCompress(compress bool) *ClickHouse {
	c.Compress = compress
	return c
}

// WithDebug 设置是否启用调试模式
func (c *ClickHouse) WithDebug(debug bool) *ClickHouse {
	c.Debug = debug
	return c
}

// WithSecure 设置是否启用TLS安全连接
func (c *ClickHouse) WithSecure(secure bool) *ClickHouse {
	c.Secure = secure
	return c
}

// WithDialTimeout 设置连接超时（秒）
func (c *ClickHouse) WithDialTimeout(dialTimeout int) *ClickHouse {
	c.DialTimeout = dialTimeout
	return c
}

// WithReadTimeout 设置读取超时（秒）
func (c *ClickHouse) WithReadTimeout(readTimeout int) *ClickHouse {
	c.ReadTimeout = readTimeout
	return c
}

// WithMaxIdleConns 设置最大空闲连接数
func (c *ClickHouse) WithMaxIdleConns(maxIdleConns int) *ClickHouse {
	c.MaxIdleConns = maxIdleConns
	return c
}

// WithMaxOpenConns 设置最大连接数
func (c *ClickHouse) WithMaxOpenConns(maxOpenConns int) *ClickHouse {
	c.MaxOpenConns = maxOpenConns
	return c
}

// WithConnMaxIdleTime 设置连接最大空闲时间
func (c *ClickHouse) WithConnMaxIdleTime(connMaxIdleTime int) *ClickHouse {
	c.ConnMaxIdleTime = connMaxIdleTime
	return c
}

// WithConnMaxLifeTime 设置连接最大生命周期
func (c *ClickHouse) WithConnMaxLifeTime(connMaxLifeTime int) *ClickHouse {
	c.ConnMaxLifeTime = connMaxLifeTime
	return c
}
