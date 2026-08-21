//go:build sqlite

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 15:56:59
 * @FilePath: \go-rpc-gateway\cpool\database\sqlite_support.go.go
 * @Description: 提供真实的 SQLite 驱动接入
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
// sqlite_support 仅在启用 sqlite 构建标签（-tags sqlite）时编译，
// 提供真实的 SQLite 驱动接入。gorm.io/driver/sqlite 会传递性引入
// mattn/go-sqlite3（CGO，约 10MB），故默认不编译进生产二进制

package database

import (
	"context"

	"github.com/kamalyes/go-config/pkg/database"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	gologger "github.com/kamalyes/go-logger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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
