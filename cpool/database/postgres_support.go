//go:build postgres

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-22 15:27:05
 * @FilePath: \go-rpc-gateway\cpool\database\postgres_support.go
 * @Description: PostgreSQL 驱动接入
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// postgres_support 仅在启用 postgres 构建标签（-tags postgres）时编译，
// 提供真实的 PostgreSQL 驱动接入。gorm.io/driver/postgres 会传递性引入
// jackc/pgx 全家桶（约 3MB），未使用 PostgreSQL 的服务默认不编译进生产二进制

package database

import (
	"github.com/kamalyes/go-rpc-gateway/config/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// gormPostgresOpen 打开 PostgreSQL 数据库
// CockroachDB 兼容 PostgreSQL 协议，走同一驱动
func gormPostgresOpen(c *database.Database) (*gorm.DB, error) {
	return gorm.Open(postgres.New(postgres.Config{DSN: c.DSN, PreferSimpleProtocol: true}), gormConfig(c))
}
