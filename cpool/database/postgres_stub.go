//go:build !postgres

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-22 15:27:05
 * @FilePath: \go-rpc-gateway\cpool\database\postgres_stub.go
 * @Description: PostgreSQL 驱动空实现
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// postgres_stub 在未启用 postgres 构建标签时提供 gormPostgresOpen 的空实现，
// 使 cpool/database 包可被 import 而不引入 gorm.io/driver/postgres（jackc/pgx）
// 需要 PostgreSQL 支持时使用 -tags postgres 构建

package database

import (
	"errors"

	"github.com/kamalyes/go-rpc-gateway/config/database"
	"gorm.io/gorm"
)

// gormPostgresOpen PostgreSQL 驱动未编译进二进制
// 需要时使用 -tags postgres 重新构建以启用 PostgreSQL 支持
func gormPostgresOpen(c *database.Database) (*gorm.DB, error) {
	return nil, errors.New("postgres driver not compiled in; rebuild with -tags postgres to enable PostgreSQL support")
}
