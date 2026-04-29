/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 15:56:59
 * @FilePath: \go-rpc-gateway\cpool\database\client.go
 * @Description: 提供数据库连接初始化和管理功能
 * 支持普通连接和持久化模式
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

	"github.com/go-sql-driver/mysql"
	"github.com/kamalyes/go-config/pkg/database"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	gologger "github.com/kamalyes/go-logger"
	mysqldriver "gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// contextLogger 包级日志器实例，供 GormLogger 在记录 SQL 日志时使用
var contextLogger gologger.ILogger

// Gorm 初始化数据库并产生数据库全局变量
// 根据配置中的数据库类型（MySQL/PostgreSQL/SQLite/CockroachDB）自动选择对应的初始化方法
// 参数:
//   - ctx: 上下文，用于超时控制和取消
//   - cfg: 网关配置，包含数据库连接参数
//   - log: 日志记录器
//
// 返回: GORM 数据库实例，如果数据库未启用或初始化失败返回 nil
func Gorm(ctx context.Context, cfg *gwconfig.Gateway, log gologger.ILogger) *gorm.DB {
	// 检查数据库是否启用
	if !cfg.Database.Enabled {
		return nil
	}

	// 保存logger到包级变量供GormLogger使用
	contextLogger = log

	// 根据配置的数据库类型选择对应的初始化方法
	if cfg.Database.Type != "" {
		switch cfg.Database.Type {
		case database.DBTypeMySQL:
			return GormMySQL(ctx, cfg, log)
		case database.DBTypePostgreSQL:
			return GormPostgreSQL(ctx, cfg, log)
		case database.DBTypeSQLite:
			return GormSQLite(ctx, cfg, log)
		case database.DBTypeCockroachDB:
			return GormCockroachDB(ctx, cfg, log)
		default:
			return GormMySQL(ctx, cfg, log) // 默认使用 MySQL
		}
	}

	// 默认尝试MySQL
	return GormMySQL(ctx, cfg, log)
}

// GormMySQL 初始化MySQL数据库
// 使用 MySQL 官方驱动，通过 DSN 方式连接
func GormMySQL(ctx context.Context, cfg *gwconfig.Gateway, log gologger.ILogger) *gorm.DB {
	if cfg == nil || cfg.Database == nil || cfg.Database.MySQL == nil {
		log.ErrorContext(ctx, "MySQL config not found")
		return nil
	}

	config := cfg.Database.MySQL
	// 使用 initDB 通用初始化方法，传入 MySQL 专属的 GORM 打开函数
	return initDB(ctx, config, database.DBTypeMySQL, log, func(dsn string) (*gorm.DB, error) {
		return gorm.Open(mysqldriver.New(mysqldriver.Config{DSN: dsn}), gormConfig(config))
	})
}

// GormPostgreSQL 初始化PostgreSQL数据库
// 使用 PostgreSQL 驱动，启用 PreferSimpleProtocol 以提升性能
func GormPostgreSQL(ctx context.Context, cfg *gwconfig.Gateway, log gologger.ILogger) *gorm.DB {
	if cfg == nil || cfg.Database == nil || cfg.Database.PostgreSQL == nil {
		log.ErrorContext(ctx, "PostgreSQL config not found")
		return nil
	}

	config := cfg.Database.PostgreSQL
	// 使用 initDB 通用初始化方法，传入 PostgreSQL 专属的 GORM 打开函数
	return initDB(ctx, config, database.DBTypePostgreSQL, log, func(dsn string) (*gorm.DB, error) {
		return gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), gormConfig(config))
	})
}

// GormSQLite 连接SQLite数据库
// SQLite 为文件型数据库，使用 DbPath 指定数据库文件路径
func GormSQLite(ctx context.Context, cfg *gwconfig.Gateway, log gologger.ILogger) *gorm.DB {
	if cfg == nil || cfg.Database == nil || cfg.Database.SQLite == nil {
		log.ErrorContext(ctx, "SQLite config not found")
		return nil
	}

	config := cfg.Database.SQLite
	// SQLite 直接使用文件路径打开，不需要 DSN
	return initDB(ctx, config, database.DBTypeSQLite, log, func(dsn string) (*gorm.DB, error) {
		return gorm.Open(sqlite.Open(config.DbPath), gormConfig(config))
	})
}

// GormCockroachDB 初始化CockroachDB数据库
// CockroachDB兼容PostgreSQL协议，使用postgres驱动
func GormCockroachDB(ctx context.Context, cfg *gwconfig.Gateway, log gologger.ILogger) *gorm.DB {
	if cfg == nil || cfg.Database == nil || cfg.Database.CockroachDB == nil {
		log.ErrorContext(ctx, "CockroachDB config not found")
		return nil
	}

	config := cfg.Database.CockroachDB
	openFunc := func(dsn string) (*gorm.DB, error) {
		return gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), gormConfig(config))
	}
	if err := ensureCockroachDatabase(ctx, config, log, openFunc); err != nil {
		log.ErrorContextKV(ctx, "CockroachDB database prepare failed", "host", config.GetHost(), "dbname", config.GetDBName(), "err", err)
		os.Exit(1)
		return nil
	}

	return initDB(ctx, config, database.DBTypeCockroachDB, log, openFunc)
}

// initDB 初始化数据库连接的通用方法
// 通过 DatabaseProvider 接口和 openFunc 回调实现多数据库类型的统一初始化流程
// 包括：构建 DSN、打开连接、配置连接池参数
// 参数:
//   - ctx: 上下文
//   - provider: 数据库配置提供者（实现 DatabaseProvider 接口）
//   - dbType: 数据库类型
//   - log: 日志记录器
//   - openFunc: GORM 数据库打开函数（不同数据库类型有不同的打开方式）
func initDB(ctx context.Context, provider database.DatabaseProvider, dbType database.DBType, log gologger.ILogger, openFunc func(string) (*gorm.DB, error)) *gorm.DB {
	host := provider.GetHost()
	port := provider.GetPort()
	// SQLite 不需要主机地址，其他数据库类型必须提供
	if dbType != database.DBTypeSQLite && host == "" {
		log.ErrorContext(ctx, "Database host is empty")
		return nil
	}

	dsn := buildDSN(provider, dbType)
	log.DebugContextKV(ctx, "Opening database connection", "type", dbType, "host", host, "port", port, "dbname", provider.GetDBName())

	// 使用传入的 openFunc 打开数据库连接
	db, err := openFunc(dsn)
	if err != nil {
		// 数据库连接失败，直接退出程序（数据库为关键依赖）
		log.ErrorContextKV(ctx, fmt.Sprintf("%s database connection failed", dbType), "host", host, "dbname", provider.GetDBName(), "err", err)
		os.Exit(1)
		return nil
	}

	// 获取底层 sql.DB 实例以配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		log.ErrorContextKV(ctx, fmt.Sprintf("%s database handle failed", dbType), "host", host, "dbname", provider.GetDBName(), "err", err)
		os.Exit(1)
		return nil
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		log.ErrorContextKV(ctx, fmt.Sprintf("%s database ping failed", dbType), "host", host, "dbname", provider.GetDBName(), "err", err)
		os.Exit(1)
		return nil
	}

	// 设置连接池参数，根据不同的数据库类型从 provider 中读取配置
	// 各数据库类型的连接池参数配置逻辑相同，只是类型断言不同
	if mysql, ok := provider.(*database.MySQL); ok {
		sqlDB.SetMaxIdleConns(mysql.MaxIdleConns)
		sqlDB.SetMaxOpenConns(mysql.MaxOpenConns)
		if mysql.ConnMaxIdleTime > 0 {
			sqlDB.SetConnMaxIdleTime(time.Duration(mysql.ConnMaxIdleTime) * time.Second)
		}
		if mysql.ConnMaxLifeTime > 0 {
			sqlDB.SetConnMaxLifetime(time.Duration(mysql.ConnMaxLifeTime) * time.Second)
		}
	} else if postgres, ok := provider.(*database.PostgreSQL); ok {
		sqlDB.SetMaxIdleConns(postgres.MaxIdleConns)
		sqlDB.SetMaxOpenConns(postgres.MaxOpenConns)
		if postgres.ConnMaxIdleTime > 0 {
			sqlDB.SetConnMaxIdleTime(time.Duration(postgres.ConnMaxIdleTime) * time.Second)
		}
		if postgres.ConnMaxLifeTime > 0 {
			sqlDB.SetConnMaxLifetime(time.Duration(postgres.ConnMaxLifeTime) * time.Second)
		}
	} else if sqlite, ok := provider.(*database.SQLite); ok {
		sqlDB.SetMaxIdleConns(sqlite.MaxIdleConns)
		sqlDB.SetMaxOpenConns(sqlite.MaxOpenConns)
		if sqlite.ConnMaxIdleTime > 0 {
			sqlDB.SetConnMaxIdleTime(time.Duration(sqlite.ConnMaxIdleTime) * time.Second)
		}
		if sqlite.ConnMaxLifeTime > 0 {
			sqlDB.SetConnMaxLifetime(time.Duration(sqlite.ConnMaxLifeTime) * time.Second)
		}
	} else if cockroachdb, ok := provider.(*database.CockroachDB); ok {
		sqlDB.SetMaxIdleConns(cockroachdb.MaxIdleConns)
		sqlDB.SetMaxOpenConns(cockroachdb.MaxOpenConns)
		if cockroachdb.ConnMaxIdleTime > 0 {
			sqlDB.SetConnMaxIdleTime(time.Duration(cockroachdb.ConnMaxIdleTime) * time.Second)
		}
		if cockroachdb.ConnMaxLifeTime > 0 {
			sqlDB.SetConnMaxLifetime(time.Duration(cockroachdb.ConnMaxLifeTime) * time.Second)
		}
	}

	return db
}

func ensureCockroachDatabase(ctx context.Context, provider database.DatabaseProvider, log gologger.ILogger, openFunc func(string) (*gorm.DB, error)) error {
	dbname := provider.GetDBName()
	if dbname == "" {
		return fmt.Errorf("cockroachdb db-name is empty")
	}

	maintenanceDSN := buildDSN(provider, database.DBTypeCockroachDB)
	maintenanceDB, err := openFunc(maintenanceDSN)
	if err != nil {
		return fmt.Errorf("connect maintenance database system failed: %w", err)
	}

	sqlDB, err := maintenanceDB.DB()
	if err != nil {
		return fmt.Errorf("get maintenance database handle failed: %w", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping maintenance database system failed: %w", err)
	}

	createSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", quoteIdentifier(dbname))
	if err := maintenanceDB.WithContext(ctx).Exec(createSQL).Error; err != nil {
		return fmt.Errorf("ensure database %q failed: %w", dbname, err)
	}

	log.InfoContextKV(ctx, "CockroachDB database is ready", "dbname", dbname)
	return nil
}

// buildDSN 构建数据库连接字符串
func buildDSN(provider database.DatabaseProvider, dbType database.DBType) string {
	host := provider.GetHost()
	user := provider.GetUsername()
	password := provider.GetPassword()
	dbname := provider.GetDBName()
	port := provider.GetPort()
	configString := provider.GetConfig()

	var dsn string
	switch dbType {
	case database.DBTypeMySQL:
		// 使用 mysql.Config 来安全构建 DSN,自动处理特殊字符
		cfg := mysql.Config{
			User:                 user,
			Passwd:               password,
			Net:                  "tcp",
			Addr:                 fmt.Sprintf("%s:%s", host, port),
			DBName:               dbname,
			Params:               parseConfigParams(configString),
			AllowNativePasswords: true,
		}
		dsn = cfg.FormatDSN()
	case database.DBTypePostgreSQL, database.DBTypeCockroachDB:
		// CockroachDB 兼容 PostgreSQL 协议，DSN格式与PostgreSQL相同
		parts := []string{
			pgKeyValue("host", host),
			pgKeyValue("user", user),
			pgKeyValue("password", password),
			pgKeyValue("dbname", dbname),
			pgKeyValue("port", port),
		}
		if configString != "" {
			parts = append(parts, configString)
		}
		dsn = strings.Join(parts, " ")
	case database.DBTypeSQLite:
		dsn = dbname // SQLite使用DbPath
	}
	return dsn
}

// pgKeyValue 构建 PostgreSQL 连接字符串键值对
func pgKeyValue(key, value string) string {
	return fmt.Sprintf("%s=%s", key, quoteConnStringValue(value))
}

// quoteConnStringValue 转义 PostgreSQL 连接字符串中的特殊字符
func quoteConnStringValue(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \\'") {
		return value
	}

	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	return "'" + escaped + "'"
}

// quoteIdentifier 转义 PostgreSQL 连接字符串中的标识符
func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

// parseConfigParams 解析配置字符串为参数 map
func parseConfigParams(configString string) map[string]string {
	params := make(map[string]string)
	if configString == "" {
		return params
	}

	// 分割配置字符串 (格式: key1=value1&key2=value2)
	pairs := strings.Split(configString, "&")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			params[kv[0]] = kv[1]
		}
	}
	return params
}

// gormConfig 根据数据库配置构建 GORM 配置
// 包含性能优化选项、命名策略和自定义日志记录器
func gormConfig(provider database.DatabaseProvider) *gorm.Config {
	// 从 DatabaseProvider 接口读取所有 GORM 相关配置
	slowThreshold := provider.GetSlowThreshold()
	ignoreRecordNotFoundError := provider.GetIgnoreRecordNotFoundError()
	skipDefaultTransaction := provider.GetSkipDefaultTransaction()
	prepareStmt := provider.GetPrepareStmt()
	disableForeignKeyConstraintWhenMigrating := provider.GetDisableForeignKeyConstraintWhenMigrating()
	disableNestedTransaction := provider.GetDisableNestedTransaction()
	allowGlobalUpdate := provider.GetAllowGlobalUpdate()
	queryFields := provider.GetQueryFields()
	createBatchSize := provider.GetCreateBatchSize()
	singularTable := provider.GetSingularTable()

	config := &gorm.Config{
		// 性能优化配置
		SkipDefaultTransaction:                   skipDefaultTransaction,
		PrepareStmt:                              prepareStmt,
		DisableForeignKeyConstraintWhenMigrating: disableForeignKeyConstraintWhenMigrating,
		DisableNestedTransaction:                 disableNestedTransaction,
		AllowGlobalUpdate:                        allowGlobalUpdate,
		QueryFields:                              queryFields,
		CreateBatchSize:                          createBatchSize,
		// 命名策略
		NamingStrategy: schema.NamingStrategy{
			SingularTable: singularTable,
		},
	}

	// 使用自定义的JSON格式Logger,支持trace_id自动注入
	config.Logger = NewGormLogger(
		gormlogger.Config{
			SlowThreshold:             time.Duration(slowThreshold) * time.Millisecond, // 从配置读取慢查询阈值
			LogLevel:                  gormlogger.Info,                                 // 记录所有SQL
			IgnoreRecordNotFoundError: ignoreRecordNotFoundError,                       // 从配置读取是否忽略记录未找到错误
			Colorful:                  false,                                           // 使用JSON格式,不需要彩色
		},
	)
	return config
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
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.Config.LogLevel <= gormlogger.Silent || contextLogger == nil {
		return
	}

	// 计算 SQL 执行耗时
	elapsed := time.Since(begin)
	// 获取 SQL 语句和影响行数
	sql, rows := fc()

	switch {
	case err != nil && errors.Is(err, gormlogger.ErrRecordNotFound) && l.Config.LogLevel >= gormlogger.Warn:
		// Record Not Found - 降级为WARN
		contextLogger.WarnContextKV(
			ctx,
			"⚠️ Record Not Found",
			"ms", elapsed.Milliseconds(),
			"rows", rows,
			"sql", sql,
		)
	case err != nil && l.Config.LogLevel >= gormlogger.Error:
		// SQL错误 - 显示完整信息
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
		contextLogger.InfoContextKV(
			ctx,
			"SQL",
			"ms", elapsed.Milliseconds(),
			"rows", rows,
			"sql", sql,
		)
	}
}
