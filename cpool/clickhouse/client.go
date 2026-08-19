/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-28 15:20:16
 * @FilePath: \go-rpc-gateway\cpool\clickhouse\client.go
 * @Description: ClickHouse 数据库连接工厂函数
 * 支持原生 ClickHouse 连接和标准 database/sql 接口两种模式
 * 遵循纯工厂函数模式，不维护包级全局状态，由 Manager 统一管理连接生命周期
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package clickhouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-config/pkg/tsdb"
	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/cpool/database"
	gormch "gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// ensureClickHouseDatabase 确保目标数据库存在，不存在则自动创建
// 先连接到 ClickHouse 内置的 default 数据库，执行 CREATE DATABASE IF NOT EXISTS
func ensureClickHouseDatabase(ctx context.Context, cfg *tsdb.ClickHouse, log logger.ILogger) error {
	dbname := cfg.Dbname
	if dbname == "" || dbname == "default" {
		return nil
	}

	maintenanceOpts := buildClickHouseOptions(cfg)
	maintenanceOpts.Auth.Database = "default"

	maintenanceConn, err := clickhouse.Open(maintenanceOpts)
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
//   - cfg: 网关配置，包含 ClickHouse 连接参数
//   - log: 日志记录器
//
// 返回: *gorm.DB 实例，配置缺失或连接失败返回 nil
func NewClickHouseDB(ctx context.Context, cfg *gwconfig.Gateway, log logger.ILogger) *gorm.DB {
	if cfg == nil || cfg.ClickHouse == nil {
		log.WarnContext(ctx, "ClickHouse configuration not found, skipping gorm initialization")
		return nil
	}

	chCfg := cfg.ClickHouse

	if err := ensureClickHouseDatabase(ctx, chCfg, log); err != nil {
		log.ErrorContextKV(ctx, "ClickHouse database prepare failed", "host", chCfg.Host, "dbname", chCfg.Dbname, "err", err)
		return nil
	}

	// 复用 cpool/database 的 GormLogger，统一 SQL 日志走 go-logger
	database.SetContextLogger(log)

	logLevel := parseGormLogLevel(chCfg.LogLevel, chCfg.Debug)

	// 用 clickhouse.OpenDB(opts) 拿 *sql.DB，再注入 gorm clickhouse 驱动
	// 不能走 gormch.Open(dsn) 路径：该路径内部调用 clickhouse.ParseDSN，会把 DSN query 参数的值
	// 全部小写化（clickhouse_options.go default 分支 strings.ToLower），导致 session_timezone=UTC
	// 变成 session_timezone=utc，CH 服务端拒绝（code: 36 Invalid time zone: utc）
	// OpenDB 直接基于 Options 构建连接（clickhouse_std.go:124 setDefaults），Settings 原样保留，
	// session_timezone=UTC 大写不被篡改
	chOpts := buildClickHouseOptions(chCfg)
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

	// OpenDB 已设置默认连接池参数，这里按配置覆盖（与 initDB 风格一致）
	if chCfg.MaxIdleConns > 0 {
		chSqlDB.SetMaxIdleConns(chCfg.MaxIdleConns)
	}
	if chCfg.MaxOpenConns > 0 {
		chSqlDB.SetMaxOpenConns(chCfg.MaxOpenConns)
	}
	if chCfg.ConnMaxIdleTime > 0 {
		chSqlDB.SetConnMaxIdleTime(time.Duration(chCfg.ConnMaxIdleTime) * time.Second)
	}
	if chCfg.ConnMaxLifeTime > 0 {
		chSqlDB.SetConnMaxLifetime(time.Duration(chCfg.ConnMaxLifeTime) * time.Second)
	}

	log.InfoContextKV(ctx, "ClickHouse gorm connected successfully",
		"host", chCfg.Host,
		"port", chCfg.Port,
		"database", chCfg.Dbname,
	)

	return gormDB
}

// buildClickHouseOptions 根据 ClickHouse 配置构建原生驱动连接选项
// 包含地址、认证、压缩、超时、连接池、TLS 等配置
func buildClickHouseOptions(cfg *tsdb.ClickHouse) *clickhouse.Options {
	opts := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Database: cfg.Dbname,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		DialTimeout:     time.Duration(cfg.DialTimeout) * time.Second,
		ReadTimeout:     time.Duration(cfg.ReadTimeout) * time.Second,
		MaxIdleConns:    cfg.MaxIdleConns,
		MaxOpenConns:    cfg.MaxOpenConns,
		ConnMaxLifetime: time.Duration(cfg.ConnMaxLifeTime) * time.Second,
		Debug:           cfg.Debug,
		// 固定会话时区为 UTC：deposit_orders/withdraw_orders/game_trades 的时间列均为 naive DateTime64(3)（无时区），
		// clickhouse-go 会对这类列用会话时区作为 Location 解释（lib/column/datetime64.go WithLocation）。
		// 若不显式设置，会话时区取服务器默认值，导致前端传入的 UTC 时间参数与 naive 列比较时被错误偏移，
		// 窄窗口（如1天）查询直接命中错位返回空（player-rank 返回 {"items":[]} 即此问题）。
		// 设为 UTC 后，naive 时间列与前端 UTC 参数在同一时区下比较，结果与会话/服务器时区解耦。
		// 注意 setting 名必须是 session_timezone（CH 22.x+），用 "timezone" 会被服务端拒绝
		// （code: 115 Unknown setting 'timezone'），导致连接建立失败返回 nil → "clickhouse is unavailable"。
		Settings: clickhouse.Settings{
			"session_timezone": "UTC",
		},
	}

	// 启用 TLS 安全连接
	if cfg.Secure {
		opts.TLS = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	// HTTP 协议传输方式（默认为 Native TCP）
	if cfg.Protocol == "http" {
		opts.Protocol = clickhouse.HTTP
	}

	return opts
}

// parseGormLogLevel 将配置中的日志等级字符串映射为 GORM LogLevel
// debug 模式强制使用 Info 级别
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
