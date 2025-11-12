/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 21:55:30
 * @FilePath: \go-rpc-gateway\cpool\database\client.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package database

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/kamalyes/go-config/pkg/database"
	"github.com/kamalyes/go-rpc-gateway/global"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Gorm 初始化数据库并产生数据库全局变量
func Gorm() *gorm.DB {
	// 使用 global.GATEWAY 配置
	cfg := global.GATEWAY
	if cfg == nil {
		return nil
	}

	// 根据配置的数据库类型选择对应的初始化方法
	if cfg.Database != nil && cfg.Database.Type != "" {
		switch cfg.Database.Type {
		case database.DBTypeMySQL:
			return GormMySQL()
		case database.DBTypePostgreSQL:
			return GormPostgreSQL()
		case database.DBTypeSQLite:
			return GormSQLite()
		default:
			return GormMySQL() // 默认使用 MySQL
		}
	}
	
	// 默认尝试MySQL
	return GormMySQL()
}

// GormMySQL 初始化MySQL数据库
func GormMySQL() *gorm.DB {
	cfg := global.GATEWAY
	if cfg == nil || cfg.Database == nil || cfg.Database.MySQL == nil {
		global.LOGGER.Error("MySQL config not found")
		return nil
	}
	
	config := cfg.Database.MySQL
	return initDB(config, database.DBTypeMySQL, func(dsn string) (*gorm.DB, error) {
		return gorm.Open(mysql.New(mysql.Config{DSN: dsn}), gormConfig(config.LogLevel))
	})
}

// GormPostgreSQL 初始化PostgreSQL数据库
func GormPostgreSQL() *gorm.DB {
	cfg := global.GATEWAY
	if cfg == nil || cfg.Database == nil || cfg.Database.PostgreSQL == nil {
		global.LOGGER.Error("PostgreSQL config not found")
		return nil
	}
	
	config := cfg.Database.PostgreSQL
	return initDB(config, database.DBTypePostgreSQL, func(dsn string) (*gorm.DB, error) {
		return gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), gormConfig(config.LogLevel))
	})
}

// GormSQLite 连接SQLite数据库
func GormSQLite() *gorm.DB {
	cfg := global.GATEWAY
	if cfg == nil || cfg.Database == nil || cfg.Database.SQLite == nil {
		global.LOGGER.Error("SQLite config not found")
		return nil
	}
	
	config := cfg.Database.SQLite
	return initDB(config, database.DBTypeSQLite, func(dsn string) (*gorm.DB, error) {
		return gorm.Open(sqlite.Open(config.DbPath), gormConfig(config.LogLevel))
	})
}

// initDB 初始化数据库连接
func initDB(provider database.DatabaseProvider, dbType database.DBType, openFunc func(string) (*gorm.DB, error)) *gorm.DB {
	host := provider.GetHost()
	if dbType != database.DBTypeSQLite && host == "" {
		global.LOGGER.Error("Database host is empty")
		return nil
	}

	dsn := buildDSN(provider, dbType)
	db, err := openFunc(dsn)
	if err != nil {
		global.LOGGER.ErrorKV(fmt.Sprintf("%s database startup error", dbType), "err", err)
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
	host := url.QueryEscape(provider.GetHost())
	user := url.QueryEscape(provider.GetUsername())
	password := url.QueryEscape(provider.GetPassword())
	dbname := url.QueryEscape(provider.GetDBName())
	port := url.QueryEscape(provider.GetPort())
	configString := url.QueryEscape(provider.GetConfig())

	var dsn string
	switch dbType {
	case database.DBTypeMySQL:
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s", user, password, host, port, dbname, configString)
	case database.DBTypePostgreSQL:
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s %s", host, user, password, dbname, port, configString)
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

	switch logLevel {
	case "silent", "Silent":
		config.Logger = logger.Default.LogMode(logger.Silent)
	case "error", "Error":
		config.Logger = logger.Default.LogMode(logger.Error)
	case "warn", "Warn":
		config.Logger = logger.Default.LogMode(logger.Warn)
	case "info", "Info":
		config.Logger = logger.Default.LogMode(logger.Info)
	default:
		config.Logger = logger.Default.LogMode(logger.Error)
	}
	return config
}
