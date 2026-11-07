/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 10:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 10:30:00
 * @FilePath: \go-rpc-gateway\cpool\database\open.go
 * @Description: 严格返回错误的数据库连接入口
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/kamalyes/go-rpc-gateway/config/database"
	gologger "github.com/kamalyes/go-toolbox/pkg/logger"
	mysqldriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Open 按数据库配置打开连接，失败时返回具体错误
//
// 与 Gorm 的分工
//
//   - Gorm 是网关启动路径上的宽松入口：失败只写日志与 stderr 并返回 nil，由调用链决定降级还是退出
//   - Open 给需要感知失败的业务代码使用，例如多数据库实例注册、迁移工具与测试
//
// [EN] Open opens a database connection and returns a concrete error. Gorm stays the lenient
// entry point used by gateway startup; Open is for code that needs to observe the failure,
// such as named database registration, migration tools and tests.
func Open(ctx context.Context, cfg *database.Database, log gologger.ILogger) (*gorm.DB, error) {
	if cfg == nil {
		return nil, errors.New("数据库配置为空")
	}
	if !cfg.Enabled {
		return nil, errors.New("数据库未启用")
	}
	if cfg.DSN == "" {
		return nil, errors.New("数据库 DSN 为空")
	}

	var (
		db  *gorm.DB
		err error
	)
	switch cfg.Type {
	case database.DBTypeMySQL:
		db, err = gorm.Open(mysqldriver.New(mysqldriver.Config{DSN: cfg.DSN}), gormConfig(cfg))
	case database.DBTypeSQLite:
		db, err = gormSQLiteOpen(cfg)
	default: // postgres / cockroachdb（CockroachDB 兼容 PostgreSQL 协议）
		db, err = gormPostgresOpen(cfg)
	}
	if err != nil {
		return nil, fmt.Errorf("%s 数据库连接失败: %w", cfg.Type, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("%s 数据库句柄获取失败: %w", cfg.Type, err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("%s 数据库连通性检查失败: %w", cfg.Type, err)
	}
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if log != nil {
		log.DebugContextKV(ctx, "数据库连接已建立", "type", cfg.Type, "max_open", cfg.MaxOpenConns)
	}
	return db, nil
}
