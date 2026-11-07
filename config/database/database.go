/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 15:17:09
 * @FilePath: \go-rpc-gateway\config\database\database.go
 * @Description: 数据库配置（单 DSN 方案）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package database

import (
	"time"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// DBType 数据库类型
type DBType string

// 支持的数据库类型
const (
	DBTypeMySQL       DBType = "mysql"
	DBTypePostgreSQL  DBType = "postgres"
	DBTypeSQLite      DBType = "sqlite"
	DBTypeCockroachDB DBType = "cockroachdb"
)

// Database 数据库统一配置（单 DSN）
// 连接信息只需一个 DSN，连接池参数与 GORM 行为开关可选配置
//
// DSN 的格式由 Type 选中的驱动决定，四种类型互不相同——MySQL 用的根本不是 URL 形式，
// SQLite 的 DSN 就是一个文件路径。所以这里按类型分别列出可直接粘贴的示例
//
// ─── mysql ──────────────────────────────────────────────────────────────────
//
// go-sql-driver/mysql 的专有格式：[用户名[:密码]@][协议(地址)]/数据库名[?参数=值&...]
// 注意主机要包在 tcp(...) 里，不是 URL
//
//	基础（parseTime=True 必带，否则 DATETIME 列扫不进 time.Time）：
//	  gateway:mysql_pwd@tcp(127.0.0.1:3306)/gateway?charset=utf8mb4&parseTime=True&loc=Local
//
//	指定时区为 UTC（loc 的值要 URL 转义，Asia/Shanghai 写成 Asia%2FShanghai）：
//	  gateway:mysql_pwd@tcp(127.0.0.1:3306)/gateway?charset=utf8mb4&parseTime=True&loc=UTC
//
//	带超时与 TLS：
//	  gateway:mysql_pwd@tcp(mysql.example.com:3306)/gateway?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s&readTimeout=10s&writeTimeout=10s&tls=true
//
//	Unix socket：
//	  gateway:mysql_pwd@unix(/tmp/mysql.sock)/gateway?charset=utf8mb4&parseTime=True
//
// ─── postgres ───────────────────────────────────────────────────────────────
//
// 两种写法都能用（gorm.io/driver/postgres 底层是 pgx）：URL 形式、或空格分隔的 keyword/value 形式
//
//	URL 形式，关闭 TLS（本地开发）：
//	  postgres://postgres:postgres@127.0.0.1:5432/gateway?sslmode=disable
//
//	URL 形式，强制 TLS 并固定时区：
//	  postgres://gateway:pg_pwd@pg.example.com:5432/gateway?sslmode=require&TimeZone=UTC
//
//	URL 形式，带连接与语句超时：
//	  postgres://gateway:pg_pwd@pg.example.com:5432/gateway?sslmode=require&connect_timeout=5&statement_timeout=30000
//
//	keyword/value 形式（密码含 @ : / 等特殊字符时更省事，不用转义）：
//	  host=127.0.0.1 port=5432 user=gateway password=p@ss:w/rd dbname=gateway sslmode=disable TimeZone=UTC
//
//	走 PgBouncer 等连接池中间件：PgBouncer 在 transaction 模式下会把同一客户端的
//	连续语句分派到不同后端连接上，预编译语句缓存因此失效并报
//	"prepared statement does not exist"，所以要让 pgx 退回简单查询协议，
//	同时把下面的 PrepareStmt 置为 false：
//	  postgres://gateway:pg_pwd@pgbouncer.example.com:6432/gateway?sslmode=disable&default_query_exec_mode=simple_protocol
//
// ─── cockroachdb ────────────────────────────────────────────────────────────
//
// CockroachDB 兼容 PostgreSQL 线路协议，走同一个驱动，DSN 也是 postgres URL 形式
//
//	自建集群，默认 26257 端口：
//	  postgres://gateway:crdb_pwd@crdb.example.com:26257/gateway?sslmode=verify-full&sslrootcert=/etc/certs/ca.crt
//
//	CockroachDB Cloud（集群名通过 options 参数传入，= 要写成 %3D）：
//	  postgres://gateway:crdb_pwd@free-tier.gcp-us-central1.cockroachlabs.cloud:26257/gateway?sslmode=verify-full&options=--cluster%3Dmy-cluster-1234
//
// ─── sqlite ─────────────────────────────────────────────────────────────────
//
// DSN 就是数据库文件路径，也接受 file: URI 形式
//
//	相对路径文件：
//	  data/gateway.db
//
//	file: URI，自动建库并开启共享缓存：
//	  file:data/gateway.db?cache=shared&mode=rwc
//
//	内存库（进程退出即丢，适合跑测试）：
//	  file::memory:?cache=shared
//
// [EN] The DSN format is decided by the driver that Type selects, and the four differ
// materially: MySQL's is not a URL at all, and SQLite's is just a file path. The examples
// above are per type and ready to paste
type Database struct {
	ModuleName string `mapstructure:"module-name" yaml:"module-name" json:"moduleName"` // 模块名称
	Enabled    bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`            // 是否启用数据库
	Type       DBType `mapstructure:"type" yaml:"type" json:"type"`                     // 数据库类型 (postgres/mysql/sqlite/cockroachdb)
	DSN        string `mapstructure:"dsn" yaml:"dsn" json:"dsn"`                        // 连接字符串，格式按 Type 而异，可粘贴的示例见类型注释

	// 连接池参数
	MaxIdleConns    int           `mapstructure:"max-idle-conns" yaml:"max-idle-conns" json:"maxIdleConns"`            // 最大空闲连接数
	MaxOpenConns    int           `mapstructure:"max-open-conns" yaml:"max-open-conns" json:"maxOpenConns"`            // 最大打开连接数
	ConnMaxIdleTime time.Duration `mapstructure:"conn-max-idle-time" yaml:"conn-max-idle-time" json:"connMaxIdleTime"` // 连接最大空闲时间
	ConnMaxLifetime time.Duration `mapstructure:"conn-max-lifetime" yaml:"conn-max-lifetime" json:"connMaxLifetime"`   // 连接最大存活时间

	// GORM 行为开关
	SkipDefaultTransaction    bool   `mapstructure:"skip-default-transaction" yaml:"skip-default-transaction" json:"skipDefaultTransaction"`              // 跳过默认事务
	PrepareStmt               bool   `mapstructure:"prepare-stmt" yaml:"prepare-stmt" json:"prepareStmt"`                                                 // 预编译语句
	IgnoreRecordNotFoundError bool   `mapstructure:"ignore-record-not-found-error" yaml:"ignore-record-not-found-error" json:"ignoreRecordNotFoundError"` // 忽略记录未找到错误
	SlowThreshold             int    `mapstructure:"slow-threshold" yaml:"slow-threshold" json:"slowThreshold"`                                           // 慢查询阈值（毫秒）
	LogLevel                  string `mapstructure:"log-level" yaml:"log-level" json:"logLevel"`                                                          // SQL日志等级 (silent/error/warn/info)
}

// DefaultDatabaseConfig 返回默认数据库配置
func DefaultDatabaseConfig() *Database {
	return &Database{
		ModuleName:      "database",
		Enabled:         false,
		Type:            DBTypePostgreSQL,
		DSN:             "postgres://postgres:postgres@127.0.0.1:5432/gateway?sslmode=disable",
		MaxIdleConns:    10,
		MaxOpenConns:    100,
		ConnMaxIdleTime: 30 * time.Minute,
		ConnMaxLifetime: time.Hour,
		SlowThreshold:   200,
		LogLevel:        "warn",
	}
}

// Get 返回配置接口
func (d *Database) Get() any {
	return d
}

// Set 设置配置数据
func (d *Database) Set(data any) {
	if cfg, ok := data.(*Database); ok {
		*d = *cfg
	}
}

// Clone 返回配置的副本
func (d *Database) Clone() internal.Configurable {
	var cloned Database
	if err := syncx.DeepCopy(&cloned, d); err != nil {
		return &Database{}
	}
	return &cloned
}

// Validate 验证配置
func (d *Database) Validate() error {
	if !d.Enabled {
		return nil
	}
	if d.DSN == "" {
		d.DSN = "postgres://postgres:postgres@127.0.0.1:5432/gateway?sslmode=disable"
	}
	if d.Type == "" {
		d.Type = DBTypePostgreSQL
	}
	if d.MaxIdleConns <= 0 {
		d.MaxIdleConns = 10
	}
	if d.MaxOpenConns <= 0 {
		d.MaxOpenConns = 100
	}
	if d.ConnMaxIdleTime <= 0 {
		d.ConnMaxIdleTime = 30 * time.Minute
	}
	if d.ConnMaxLifetime <= 0 {
		d.ConnMaxLifetime = time.Hour
	}
	return internal.ValidateStruct(d)
}
