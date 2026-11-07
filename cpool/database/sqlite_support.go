//go:build sqlite

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 15:56:59
 * @FilePath: \go-rpc-gateway\cpool\database\sqlite_support.go
 * @Description: SQLite 驱动接入
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

// sqlite_support 仅在启用 sqlite 构建标签（-tags sqlite）时编译，
// 提供真实的 SQLite 驱动接入。gorm.io/driver/sqlite 会传递性引入
// mattn/go-sqlite3（CGO，约 10MB），故默认不编译进生产二进制

package database

import (
	"github.com/kamalyes/go-rpc-gateway/config/database"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// gormSQLiteOpen 打开 SQLite 数据库
// SQLite 为文件型数据库，DSN 即数据库文件路径
func gormSQLiteOpen(c *database.Database) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(c.DSN), gormConfig(c))
}
