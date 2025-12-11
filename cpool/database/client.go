package database

import (
	"context"
	"errors"
	"fmt"
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
	"os"
	"strings"
	"time"
)

// contextLogger 存储在context中的logger实例
var contextLogger gologger.ILogger

// Gorm 初始化数据库并产生数据库全局变量
func Gorm(ctx context.Context, cfg *gwconfig.Gateway, log gologger.ILogger) *gorm.DB {
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
		default:
			return GormMySQL(ctx, cfg, log) // 默认使用 MySQL
		}
	}

	// 默认尝试MySQL
	return GormMySQL(ctx, cfg, log)
}

// GormMySQL 初始化MySQL数据库
func GormMySQL(ctx context.Context, cfg *gwconfig.Gateway, log gologger.ILogger) *gorm.DB {
	if cfg == nil || cfg.Database == nil || cfg.Database.MySQL == nil {
		if log != nil {
			log.ErrorContext(ctx, "MySQL config not found")
		}
		return nil
	}

	config := cfg.Database.MySQL
	return initDB(ctx, config, database.DBTypeMySQL, log, func(dsn string) (*gorm.DB, error) {
		return gorm.Open(mysqldriver.New(mysqldriver.Config{DSN: dsn}), gormConfig(config.LogLevel))
	})
}

// GormPostgreSQL 初始化PostgreSQL数据库
func GormPostgreSQL(ctx context.Context, cfg *gwconfig.Gateway, log gologger.ILogger) *gorm.DB {
	if cfg == nil || cfg.Database == nil || cfg.Database.PostgreSQL == nil {
		if log != nil {
			log.ErrorContext(ctx, "PostgreSQL config not found")
		}
		return nil
	}

	config := cfg.Database.PostgreSQL
	return initDB(ctx, config, database.DBTypePostgreSQL, log, func(dsn string) (*gorm.DB, error) {
		return gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), gormConfig(config.LogLevel))
	})
}

// GormSQLite 连接SQLite数据库
func GormSQLite(ctx context.Context, cfg *gwconfig.Gateway, log gologger.ILogger) *gorm.DB {
	if cfg == nil || cfg.Database == nil || cfg.Database.SQLite == nil {
		if log != nil {
			log.ErrorContext(ctx, "SQLite config not found")
		}
		return nil
	}

	config := cfg.Database.SQLite
	return initDB(ctx, config, database.DBTypeSQLite, log, func(dsn string) (*gorm.DB, error) {
		return gorm.Open(sqlite.Open(config.DbPath), gormConfig(config.LogLevel))
	})
}

// initDB 初始化数据库连接
func initDB(ctx context.Context, provider database.DatabaseProvider, dbType database.DBType, log gologger.ILogger, openFunc func(string) (*gorm.DB, error)) *gorm.DB {
	host := provider.GetHost()
	if dbType != database.DBTypeSQLite && host == "" {
		if log != nil {
			log.ErrorContext(ctx, "Database host is empty")
		}
		return nil
	}

	dsn := buildDSN(provider, dbType)
	db, err := openFunc(dsn)
	if err != nil {
		log.ErrorContextKV(ctx, fmt.Sprintf("%s database connection failed", dbType), "host", host, "dbname", provider.GetDBName(), "err", err)
		os.Exit(1)
		return nil
	}

	sqlDB, _ := db.DB()

	// 设置连接池参数，直接从provider获取
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
	}

	return db
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
	case database.DBTypePostgreSQL:
		// PostgreSQL DSN 格式
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s %s",
			host, user, password, dbname, port, configString)
	case database.DBTypeSQLite:
		dsn = provider.GetDBName() // SQLite使用DbPath
	}
	return dsn
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

// gormConfig 根据配置决定是否开启日志
func gormConfig(logLevel string) *gorm.Config {
	config := &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	}
	// 使用自定义的JSON格式Logger,支持trace_id自动注入
	config.Logger = NewGormLogger(
		gormlogger.Config{
			SlowThreshold:             100 * time.Millisecond, // 慢查询阈值
			LogLevel:                  gormlogger.Info,        // 记录所有SQL
			IgnoreRecordNotFoundError: false,                  // 不忽略记录未找到错误
			Colorful:                  false,                  // 使用JSON格式,不需要彩色
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

// LogMode 实现gormlogger.Interface接口
func (l *GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.Config.LogLevel = level
	return &newLogger
}

// Info 实现gormlogger.Interface接口
func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.Config.LogLevel >= gormlogger.Info && contextLogger != nil {
		contextLogger.InfoContextKV(ctx, msg, "data", fmt.Sprintf("%v", data))
	}
}

// Warn 实现gormlogger.Interface接口
func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.Config.LogLevel >= gormlogger.Warn && contextLogger != nil {
		contextLogger.WarnContextKV(ctx, msg, "data", fmt.Sprintf("%v", data))
	}
}

// Error 实现gormlogger.Interface接口
func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.Config.LogLevel >= gormlogger.Error && contextLogger != nil {
		contextLogger.ErrorContextKV(ctx, msg, "data", fmt.Sprintf("%v", data))
	}
}

// Trace 实现gormlogger.Interface接口 - 记录SQL执行
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.Config.LogLevel <= gormlogger.Silent || contextLogger == nil {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	switch {
	case err != nil && l.Config.LogLevel >= gormlogger.Error && (!errors.Is(err, gormlogger.ErrRecordNotFound) || !l.Config.IgnoreRecordNotFoundError):
		// SQL错误 - 显示完整信息
		contextLogger.ErrorContextKV(ctx, "❌ SQL Error",
			"ms", fmt.Sprintf("%.2f", float64(elapsed.Nanoseconds())/1e6),
			"rows", rows,
			"error", err.Error(),
			"sql", sql,
		)
	case elapsed > l.Config.SlowThreshold && l.Config.SlowThreshold != 0 && l.Config.LogLevel >= gormlogger.Warn:
		// 慢查询 - 显示详细信息
		contextLogger.WarnContextKV(ctx, "🐌 SLOW SQL",
			"ms", fmt.Sprintf("%.2f", float64(elapsed.Nanoseconds())/1e6),
			"threshold", fmt.Sprintf("%.0f", float64(l.Config.SlowThreshold.Nanoseconds())/1e6),
			"rows", rows,
			"sql", sql,
		)
	case l.Config.LogLevel >= gormlogger.Info:
		// 正常SQL
		contextLogger.InfoContextKV(ctx, "SQL",
			"ms", fmt.Sprintf("%.2f", float64(elapsed.Nanoseconds())/1e6),
			"rows", rows,
			"sql", sql,
		)
	}
}
