/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-22 15:27:05
 * @FilePath: \go-rpc-gateway\cpool\database\client.go
 * @Description: 提供数据库连接初始化和管理功能（单 DSN 方案）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/config/database"
	gologger "github.com/kamalyes/go-toolbox/pkg/logger"
	mysqldriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// contextLogger 包级日志器实例，供 GormLogger 在记录 SQL 日志时使用
var contextLogger gologger.ILogger

// SetContextLogger 设置包级日志器，供 GormLogger 使用
func SetContextLogger(log gologger.ILogger) {
	contextLogger = log
}

// Gorm 按配置初始化数据库连接（单 DSN）
// 根据配置中的数据库类型自动选择驱动，连接失败直接退出（数据库为关键依赖）
func Gorm(ctx context.Context, cfg *gwconfig.Gateway, log gologger.ILogger) *gorm.DB {
	if cfg == nil || cfg.Database == nil {
		if log != nil {
			log.ErrorContext(ctx, "Database config not found")
		}
		return nil
	}
	if !cfg.Database.Enabled {
		return nil
	}

	c := cfg.Database
	if c.DSN == "" {
		if log != nil {
			log.ErrorContext(ctx, "Database DSN is empty")
		}
		return nil
	}

	log.DebugContextKV(ctx, "Opening database connection", "type", c.Type)

	var (
		db  *gorm.DB
		err error
	)
	switch c.Type {
	case database.DBTypeMySQL:
		db, err = gorm.Open(mysqldriver.New(mysqldriver.Config{DSN: c.DSN}), gormConfig(c))
	case database.DBTypeSQLite:
		db, err = gormSQLiteOpen(c)
	default: // postgres / cockroachdb（CockroachDB 兼容 PostgreSQL 协议）
		db, err = gormPostgresOpen(c)
	}

	if err != nil {
		if log != nil {
			log.ErrorContextKV(ctx, fmt.Sprintf("%s database connection failed", c.Type), "err", err)
		}
		os.Exit(1)
		return nil
	}

	// 获取底层 sql.DB 实例以配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		if log != nil {
			log.ErrorContextKV(ctx, fmt.Sprintf("%s database handle failed", c.Type), "err", err)
		}
		os.Exit(1)
		return nil
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		if log != nil {
			log.ErrorContextKV(ctx, fmt.Sprintf("%s database ping failed", c.Type), "err", err)
		}
		os.Exit(1)
		return nil
	}

	// 连接池参数
	sqlDB.SetMaxIdleConns(c.MaxIdleConns)
	sqlDB.SetMaxOpenConns(c.MaxOpenConns)
	if c.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(c.ConnMaxIdleTime)
	}
	if c.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(c.ConnMaxLifetime)
	}

	return db
}

// gormConfig 根据数据库配置构建 GORM 配置
// 包含性能优化选项、命名策略和自定义日志记录器
func gormConfig(c *database.Database) *gorm.Config {
	config := &gorm.Config{
		// 性能优化配置
		SkipDefaultTransaction: c.SkipDefaultTransaction,
		PrepareStmt:            c.PrepareStmt,
		// 命名策略
		NamingStrategy: schema.NamingStrategy{
			SingularTable: false,
		},
	}

	// 使用自定义的JSON格式Logger,支持trace_id自动注入
	config.Logger = NewGormLogger(
		gormlogger.Config{
			SlowThreshold:             time.Duration(c.SlowThreshold) * time.Millisecond, // 慢查询阈值
			LogLevel:                  parseGormLogLevel(c.LogLevel),                     // SQL日志等级（silent/error/warn/info）
			IgnoreRecordNotFoundError: c.IgnoreRecordNotFoundError,                       // 是否忽略记录未找到错误
			Colorful:                  false,                                             // 使用JSON格式,不需要彩色
		},
	)
	return config
}

// parseGormLogLevel 解析配置的 SQL 日志等级为 gorm logger 级别
// 空串与未知值兜底 warn
func parseGormLogLevel(level string) gormlogger.LogLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "silent":
		return gormlogger.Silent
	case "error":
		return gormlogger.Error
	case "info":
		return gormlogger.Info
	default: // "warn"、空串、未知值
		return gormlogger.Warn
	}
}

// GormLogger 自定义GORM日志记录器,支持JSON格式和trace_id自动注入
type GormLogger struct {
	Config gormlogger.Config
}

// NewGormLogger 创建新的GORM日志记录器
func NewGormLogger(config gormlogger.Config) gormlogger.Interface {
	return &GormLogger{
		Config: config,
	}
}

// LogMode 实现 gormlogger.Interface 接口 - 设置日志级别
// 返回一个新的 GormLogger 实例，不影响原实例
func (l *GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.Config.LogLevel = level
	return &newLogger
}

// Info 实现 gormlogger.Interface 接口 - 记录 Info 级别日志
func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.Config.LogLevel >= gormlogger.Info && contextLogger != nil {
		contextLogger.InfoContextKV(ctx, msg, "data", fmt.Sprintf("%v", data))
	}
}

// Warn 实现 gormlogger.Interface 接口 - 记录 Warn 级别日志
func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.Config.LogLevel >= gormlogger.Warn && contextLogger != nil {
		contextLogger.WarnContextKV(ctx, msg, "data", fmt.Sprintf("%v", data))
	}
}

// Error 实现 gormlogger.Interface 接口 - 记录 Error 级别日志
func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.Config.LogLevel >= gormlogger.Error && contextLogger != nil {
		contextLogger.ErrorContextKV(ctx, msg, "data", fmt.Sprintf("%v", data))
	}
}

// Trace 实现 gormlogger.Interface 接口 - 记录 SQL 执行详情
// 根据执行结果和耗时分为四种情况：
// 1. 记录未找到 → 降级为 WARN
// 2. SQL 执行错误 → 记录为 ERROR
// 3. 慢查询（超过阈值）→ 记录为 WARN
// 4. 正常执行 → 记录为 INFO
//
// fc 是惰性闭包（postgres.Dialector.Explain，内含正则格式化），仅在真正进入打印分支时才调用
// （与 gorm 官方 logger 同款写法）：级别不足或非目标场景时零格式化开销
// —— warn 级别下正常 SQL 不进任何分支，fc 不执行，避免每条 SQL 白付一次正则格式化
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.Config.LogLevel <= gormlogger.Silent || contextLogger == nil {
		return
	}

	// 计算 SQL 执行耗时
	elapsed := time.Since(begin)

	switch {
	case err != nil && errors.Is(err, gormlogger.ErrRecordNotFound) && !l.Config.IgnoreRecordNotFoundError && l.Config.LogLevel >= gormlogger.Warn:
		// Record Not Found - 降级为WARN
		sql, rows := fc()
		contextLogger.WarnContextKV(
			ctx,
			"⚠️ Record Not Found",
			"ms", elapsed.Milliseconds(),
			"rows", rows,
			"sql", sql,
		)
	case err != nil && l.Config.LogLevel >= gormlogger.Error:
		// SQL错误 - 显示完整信息
		sql, rows := fc()
		contextLogger.ErrorContextKV(
			ctx,
			"❌ SQL Error",
			"ms", elapsed.Milliseconds(),
			"rows", rows,
			"error", err.Error(),
			"sql", sql,
		)
	case elapsed > l.Config.SlowThreshold && l.Config.SlowThreshold != 0 && l.Config.LogLevel >= gormlogger.Warn:
		// 慢查询 - 显示详细信息
		sql, rows := fc()
		contextLogger.WarnContextKV(
			ctx,
			"🐌 SLOW SQL",
			"ms", elapsed.Milliseconds(),
			"threshold", l.Config.SlowThreshold.Milliseconds(),
			"rows", rows,
			"sql", sql,
		)
	case l.Config.LogLevel >= gormlogger.Info:
		// 正常SQL执行
		sql, rows := fc()
		contextLogger.InfoContextKV(
			ctx,
			"SQL",
			"ms", elapsed.Milliseconds(),
			"rows", rows,
			"sql", sql,
		)
	}
}
