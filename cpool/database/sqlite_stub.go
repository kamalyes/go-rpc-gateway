//go:build !sqlite

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 15:56:59
 * @FilePath: \go-rpc-gateway\cpool\database\sqlite_stub.go
 * @Description: SQLite 驱动空实现
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

// sqlite_stub 在未启用 sqlite 构建标签时提供 gormSQLiteOpen 的空实现，
// 使 cpool/database 包可被 import 而不引入 mattn/go-sqlite3
// 需要 SQLite 支持时使用 -tags sqlite 构建

package database

import (
	"errors"

	"github.com/kamalyes/go-rpc-gateway/config/database"
	"gorm.io/gorm"
)

// gormSQLiteOpen SQLite 驱动未编译进二进制
// 需要时使用 -tags sqlite 重新构建以启用 SQLite 支持
func gormSQLiteOpen(c *database.Database) (*gorm.DB, error) {
	return nil, errors.New("sqlite driver not compiled in; rebuild with -tags sqlite to enable SQLite support")
}
