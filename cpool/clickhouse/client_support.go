//go:build clickhouse

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-28 15:20:16
 * @FilePath: \go-rpc-gateway\cpool\clickhouse\client_support.go
 * @Description: ClickHouse 数据库连接工厂函数（单 DSN）
 * 连接参数全部来自配置里的 DSN，由 clickhouse-go 官方解析器解析
 * 遵循纯工厂函数模式，不维护包级全局状态，由 Manager 统一管理连接生命周期
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

// client_support 仅在启用 clickhouse 构建标签（-tags clickhouse）时编译。
// ClickHouse/clickhouse-go/v2 连带 gorm clickhouse 驱动、lz4、segmentio/asm、
// ch-go 等传递依赖（约 5MB），未使用 ClickHouse 的服务默认不编译进生产二进制

package clickhouse

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/cpool/database"
	"github.com/kamalyes/go-rpc-gateway/cpool/internal/dsnx"
	"github.com/kamalyes/go-toolbox/pkg/logger"
	gormch "gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// extConnMaxIdleTime 网关扩展查询参数：连接最大空闲时间
// clickhouse.Options 只有 ConnMaxLifetime，没有对应的空闲时间字段，
// 这项得落到 database/sql 层的 SetConnMaxIdleTime。不摘出来的话 ParseDSN 会把它
// 当成 ClickHouse 会话 setting 原样下发，服务端直接报 code: 115 Unknown setting，连都连不上
//
// [EN] A gateway-level parameter: clickhouse.Options has ConnMaxLifetime but no idle-time
// counterpart, so this one lands on database/sql's SetConnMaxIdleTime. Left in the DSN,
// ParseDSN would forward it to the server as a session setting and the connection would
// fail outright with code: 115 Unknown setting
const extConnMaxIdleTime = "conn_max_idle_time"

// sessionTimezoneSetting / defaultSessionTimezone 会话时区设置
//
// DSN 没写 session_timezone 时固定补 UTC：deposit_orders/withdraw_orders/game_trades
// 的时间列均为 naive DateTime64(3)（无时区），clickhouse-go 会用会话时区作为 Location
// 解释这类列（lib/column/datetime64.go WithLocation）。不显式设置时会话时区取服务器默认值，
// 导致前端传入的 UTC 时间参数与 naive 列比较时被错误偏移，窄窗口（如 1 天）查询
// 直接命中错位返回空（player-rank 返回 {"items":[]} 即此问题）。
// 设为 UTC 后，naive 时间列与前端 UTC 参数在同一时区下比较，结果与会话/服务器时区解耦
//
// 注意 setting 名必须是 session_timezone（CH 22.x+），用 "timezone" 会被服务端拒绝
// （code: 115 Unknown setting 'timezone'），导致连接建立失败返回 nil → "clickhouse is unavailable"
//
// 这里是「补默认值」而不是「强制覆盖」：DSN 里显式写了 session_timezone 就按写的来，
// 跨时区部署的业务需要这个出口
//
// [EN] When the DSN omits session_timezone, UTC is supplied. The order and trade tables
// hold naive DateTime64(3) columns with no timezone, and clickhouse-go interprets those
// using the session timezone as their Location; without an explicit setting the session
// takes the server default, so UTC parameters from the frontend get shifted against naive
// columns and a narrow window silently returns nothing. The setting name must be
// session_timezone (CH 22.x and later) — plain "timezone" is rejected and the connection
// never establishes. This supplies a default rather than forcing a value: a DSN that states
// session_timezone keeps it, which is the escape hatch cross-timezone deployments need
const (
	sessionTimezoneSetting  = "session_timezone"
	defaultSessionTimezone  = "UTC"
	maintenanceDatabaseName = "default" // ClickHouse 内置库，建目标库时先连它
)

// ensureClickHouseDatabase 确保目标数据库存在，不存在则自动创建
// 先连接到 ClickHouse 内置的 default 数据库，执行 CREATE DATABASE IF NOT EXISTS
func ensureClickHouseDatabase(ctx context.Context, opts *clickhouse.Options, log logger.ILogger) error {
	dbname := opts.Auth.Database
	if dbname == "" || dbname == maintenanceDatabaseName {
		return nil
	}

	// 浅拷贝一份只改库名：Auth 是值类型，改它不会动到原 Options；
	// Settings map 与 Compression 指针被两份 Options 共享，但建库这条路径只读不写
	//
	// [EN] A shallow copy with only the database name changed: Auth is a value type, so the
	// original Options is untouched. The Settings map and Compression pointer are shared
	// between the two, which is safe because this path only reads them
	maintenanceOpts := *opts
	maintenanceOpts.Auth.Database = maintenanceDatabaseName

	maintenanceConn, err := clickhouse.Open(&maintenanceOpts)
	if err != nil {
		return fmt.Errorf("connect to default database failed: %w", err)
	}
	defer maintenanceConn.Close()

	if err := maintenanceConn.Ping(ctx); err != nil {
		return fmt.Errorf("ping default database failed: %w", err)
	}

	createSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", dbname)
	if err := maintenanceConn.Exec(ctx, createSQL); err != nil {
		return fmt.Errorf("ensure database %q failed: %w", dbname, err)
	}

	log.InfoContextKV(ctx, "ClickHouse database is ready", "dbname", dbname)
	return nil
}

// NewClickHouseDB 创建 ClickHouse gorm 连接
// 适用于需要使用 gorm ORM 的场景，自动复用已有的连接池配置和数据库准备逻辑
// 参数:
//   - ctx: 上下文，用于超时控制和取消
//   - cfg: 网关配置，包含 ClickHouse 连接 DSN 与 GORM 行为开关
//   - log: 日志记录器
//
// 返回: *gorm.DB 实例，配置缺失或连接失败返回 nil
func NewClickHouseDB(ctx context.Context, cfg *gwconfig.Gateway, log logger.ILogger) *gorm.DB {
	if cfg == nil || cfg.ClickHouse == nil {
		log.WarnContext(ctx, "ClickHouse configuration not found, skipping gorm initialization")
		return nil
	}

	chCfg := cfg.ClickHouse
	if chCfg.DSN == "" {
		log.ErrorContext(ctx, "ClickHouse dsn is empty")
		return nil
	}

	chOpts, connMaxIdleTime, err := buildClickHouseOptions(chCfg.DSN)
	if err != nil {
		log.ErrorContextKV(ctx, "ClickHouse dsn parse failed", "dsn", redactDSN(chCfg.DSN), "err", err)
		return nil
	}

	if err := ensureClickHouseDatabase(ctx, chOpts, log); err != nil {
		log.ErrorContextKV(ctx, "ClickHouse database prepare failed", "dsn", redactDSN(chCfg.DSN), "err", err)
		return nil
	}

	// 复用 cpool/database 的 GormLogger，统一 SQL 日志走 go-logger
	database.SetContextLogger(log)

	logLevel := parseGormLogLevel(chCfg.LogLevel, chOpts.Debug)

	// 用 clickhouse.OpenDB(opts) 拿 *sql.DB，再注入 gorm clickhouse 驱动，
	// 而不是走 gormch.Open(dsn)：后者在内部自己调 ParseDSN 建连接，拿不到我们这边
	// 补过 session_timezone、摘过扩展参数的那份 Options——补的默认值会丢，
	// conn_max_idle_time 会被当成服务端 setting 下发
	//
	// [EN] OpenDB(opts) yields the *sql.DB that the gorm driver then wraps, instead of
	// gormch.Open(dsn): that path calls ParseDSN itself and so never sees the Options this
	// function has already fixed up, losing the session_timezone default and forwarding
	// conn_max_idle_time to the server as a setting
	chSqlDB := clickhouse.OpenDB(chOpts)

	gormDB, err := gorm.Open(gormch.New(gormch.Config{Conn: chSqlDB}), &gorm.Config{
		SkipDefaultTransaction:                   chCfg.SkipDefaultTransaction,
		PrepareStmt:                              chCfg.PrepareStmt,
		DisableForeignKeyConstraintWhenMigrating: chCfg.DisableForeignKeyConstraintWhenMigrating,
		DisableNestedTransaction:                 chCfg.DisableNestedTransaction,
		AllowGlobalUpdate:                        chCfg.AllowGlobalUpdate,
		QueryFields:                              chCfg.QueryFields,
		CreateBatchSize:                          chCfg.CreateBatchSize,
		NamingStrategy:                           schema.NamingStrategy{SingularTable: chCfg.SingularTable},
		Logger: database.NewGormLogger(
			gormlogger.Config{
				SlowThreshold:             time.Duration(chCfg.SlowThreshold) * time.Millisecond,
				LogLevel:                  logLevel,
				IgnoreRecordNotFoundError: chCfg.IgnoreRecordNotFoundError,
				Colorful:                  false,
			},
		),
	})
	if err != nil {
		log.ErrorContextKV(ctx, "ClickHouse gorm open failed", "error", err)
		return nil
	}

	// 连接池的其余参数（max_idle_conns/max_open_conns/conn_max_lifetime）由 OpenDB
	// 直接从 Options 读走，只有 conn_max_idle_time 没有 Options 字段，得在这里补上
	//
	// [EN] OpenDB reads the rest of the pool settings straight off the Options; only
	// conn_max_idle_time has no Options field and has to be applied here
	if connMaxIdleTime > 0 {
		chSqlDB.SetConnMaxIdleTime(connMaxIdleTime)
	}

	log.InfoContextKV(ctx, "ClickHouse gorm connected successfully",
		"addr", strings.Join(chOpts.Addr, ","),
		"database", chOpts.Auth.Database,
	)

	return gormDB
}

// buildClickHouseOptions 解析 DSN 为原生驱动连接选项
// 返回连接选项、网关扩展参数 conn_max_idle_time 的取值，以及解析错误
//
// 地址、认证、协议、TLS、压缩、超时、连接池全部由 clickhouse.ParseDSN 解析，
// 这里只做两件 ParseDSN 做不到的事：摘走网关扩展参数、补 session_timezone 默认值
//
// [EN] buildClickHouseOptions turns a DSN into native driver options, returning them
// alongside the value of the gateway's conn_max_idle_time parameter. Addresses,
// credentials, protocol, TLS, compression, timeouts and pool sizing all come from
// clickhouse.ParseDSN; this function only does the two things ParseDSN cannot — strip the
// gateway's own parameters and supply the session_timezone default
func buildClickHouseOptions(dsn string) (*clickhouse.Options, time.Duration, error) {
	u, ext, err := dsnx.Split(dsn, extConnMaxIdleTime)
	if err != nil {
		return nil, 0, err
	}

	connMaxIdleTime := ext.Duration(extConnMaxIdleTime, 0)
	if err := ext.Err(); err != nil {
		return nil, 0, err
	}

	opts, err := clickhouse.ParseDSN(u.String())
	if err != nil {
		return nil, 0, fmt.Errorf("parse clickhouse dsn failed: %w", err)
	}

	if opts.Settings == nil {
		opts.Settings = clickhouse.Settings{}
	}
	if _, ok := opts.Settings[sessionTimezoneSetting]; !ok {
		opts.Settings[sessionTimezoneSetting] = defaultSessionTimezone
	}

	return opts, connMaxIdleTime, nil
}

// parseGormLogLevel 将配置中的日志等级字符串映射为 GORM LogLevel
// DSN 里带 debug=true 时强制使用 Info 级别
func parseGormLogLevel(level string, debug bool) gormlogger.LogLevel {
	if debug {
		return gormlogger.Info
	}
	switch strings.ToLower(level) {
	case "silent":
		return gormlogger.Silent
	case "error":
		return gormlogger.Error
	case "info":
		return gormlogger.Info
	default:
		return gormlogger.Warn
	}
}

// redactDSN 抹掉 DSN 里的密码，只保留可用于定位问题的 scheme、用户名与地址
// 解析失败时返回固定占位串而不是原串，避免把密码写进日志
//
// [EN] redactDSN strips the password, keeping the scheme, username and address that are
// useful for diagnosis. A DSN that fails to parse yields a fixed placeholder rather than
// the original string, so a password never reaches the log
func redactDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "<unparsable dsn>"
	}
	if u.User != nil {
		if _, hasPwd := u.User.Password(); hasPwd {
			u.User = url.User(u.User.Username())
		}
	}
	return u.Redacted()
}
