package database

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/kamalyes/go-config/pkg/database"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	gologger "github.com/kamalyes/go-logger"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Gorm 初始化数据库并产生数据库全局变量
func Gorm(cfg *gwconfig.Gateway, log gologger.ILogger) *gorm.DB {
	if cfg == nil {
		return nil
	}

	// 根据配置的数据库类型选择对应的初始化方法
	if cfg.Database != nil && cfg.Database.Type != "" {
		switch cfg.Database.Type {
		case database.DBTypeMySQL:
			return GormMySQL(cfg, log)
		case database.DBTypePostgreSQL:
			return GormPostgreSQL(cfg, log)
		case database.DBTypeSQLite:
			return GormSQLite(cfg, log)
		default:
			return GormMySQL(cfg, log) // 默认使用 MySQL
		}
	}

	// 默认尝试MySQL
	return GormMySQL(cfg, log)
}

// GormMySQL 初始化MySQL数据库
func GormMySQL(cfg *gwconfig.Gateway, log gologger.ILogger) *gorm.DB {
	if cfg == nil || cfg.Database == nil || cfg.Database.MySQL == nil {
		if log != nil {
			log.Error("MySQL config not found")
		}
		return nil
	}

	config := cfg.Database.MySQL
	return initDB(config, database.DBTypeMySQL, log, func(dsn string) (*gorm.DB, error) {
		return gorm.Open(mysql.New(mysql.Config{DSN: dsn}), gormConfig(config.LogLevel))
	})
}

// GormPostgreSQL 初始化PostgreSQL数据库
func GormPostgreSQL(cfg *gwconfig.Gateway, log gologger.ILogger) *gorm.DB {
	if cfg == nil || cfg.Database == nil || cfg.Database.PostgreSQL == nil {
		if log != nil {
			log.Error("PostgreSQL config not found")
		}
		return nil
	}

	config := cfg.Database.PostgreSQL
	return initDB(config, database.DBTypePostgreSQL, log, func(dsn string) (*gorm.DB, error) {
		return gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), gormConfig(config.LogLevel))
	})
}

// GormSQLite 连接SQLite数据库
func GormSQLite(cfg *gwconfig.Gateway, log gologger.ILogger) *gorm.DB {
	if cfg == nil || cfg.Database == nil || cfg.Database.SQLite == nil {
		if log != nil {
			log.Error("SQLite config not found")
		}
		return nil
	}

	config := cfg.Database.SQLite
	return initDB(config, database.DBTypeSQLite, log, func(dsn string) (*gorm.DB, error) {
		return gorm.Open(sqlite.Open(config.DbPath), gormConfig(config.LogLevel))
	})
}

// initDB 初始化数据库连接
func initDB(provider database.DatabaseProvider, dbType database.DBType, log gologger.ILogger, openFunc func(string) (*gorm.DB, error)) *gorm.DB {
	host := provider.GetHost()
	if dbType != database.DBTypeSQLite && host == "" {
		if log != nil {
			log.Error("Database host is empty")
		}
		return nil
	}

	dsn := buildDSN(provider, dbType)
	db, err := openFunc(dsn)
	if err != nil {
		if log != nil {
			log.ErrorKV(fmt.Sprintf("%s database startup error", dbType), "err", err)
		}
		os.Exit(0)
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
	password := url.QueryEscape(provider.GetPassword()) // 只对密码进行编码
	dbname := provider.GetDBName()
	port := provider.GetPort()
	configString := provider.GetConfig() // 配置字符串不编码

	var dsn string
	switch dbType {
	case database.DBTypeMySQL:
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s", user, password, host, port, dbname, configString)
	case database.DBTypePostgreSQL:
		// PostgreSQL需要对所有参数进行适当编码
		hostEscaped := url.QueryEscape(host)
		userEscaped := url.QueryEscape(user)
		passwordEscaped := url.QueryEscape(provider.GetPassword())
		dbnameEscaped := url.QueryEscape(dbname)
		portEscaped := url.QueryEscape(port)
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s %s", hostEscaped, userEscaped, passwordEscaped, dbnameEscaped, portEscaped, configString)
	case database.DBTypeSQLite:
		dsn = provider.GetDBName() // SQLite使用DbPath
	}
	return dsn
}

// gormConfig 根据配置决定是否开启日志
func gormConfig(logLevel string) *gorm.Config {
	config := &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	}
	// Debug模式：显示所有SQL语句，包括参数值和执行时间
	config.Logger = gormlogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		gormlogger.Config{
			SlowThreshold:             100 * time.Millisecond, // 慢查询阈值
			LogLevel:                  gormlogger.Info,        // 记录所有SQL
			IgnoreRecordNotFoundError: false,                  // 不忽略记录未找到错误
			Colorful:                  true,                   // 彩色输出
		},
	)
	return config
}
