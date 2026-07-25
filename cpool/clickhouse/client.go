/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-28 15:20:16
 * @FilePath: \go-rpc-gateway\cpool\clickhouse\client.go
 * @Description: ClickHouse 数据库连接工厂函数
 * 支持原生 ClickHouse 连接和标准 database/sql 接口两种模式
 * 遵循纯工厂函数模式，不维护包级全局状态，由 Manager 统一管理连接生命周期
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package clickhouse

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-config/pkg/tsdb"
	"github.com/kamalyes/go-logger"
	gormch "gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ensureClickHouseDatabase 确保目标数据库存在，不存在则自动创建
// 先连接到 ClickHouse 内置的 default 数据库，执行 CREATE DATABASE IF NOT EXISTS
func ensureClickHouseDatabase(ctx context.Context, cfg *tsdb.ClickHouse, log logger.ILogger) error {
	dbname := cfg.Dbname
	if dbname == "" || dbname == "default" {
		return nil
	}

	maintenanceOpts := buildClickHouseOptions(cfg)
	maintenanceOpts.Auth.Database = "default"

	maintenanceConn, err := clickhouse.Open(maintenanceOpts)
	if err != nil {
		return fmt.Errorf("connect to default database failed: %w", err)
	}
	defer maintenanceConn.Close()

	if err := maintenanceConn.Ping(ctx); err != nil {
		return fmt.Errorf("ping default database failed: %w", err)
	}

	createSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", dbname)
	if err := maintenanceConn.Exec(ctx, createSQL); err != nil {
		return fmt.Errorf("ensure database %q failed: %w", dbname, err)
	}

	log.InfoContextKV(ctx, "ClickHouse database is ready", "dbname", dbname)
	return nil
}

// NewClickHouseDB 创建 ClickHouse gorm 连接
// 适用于需要使用 gorm ORM 的场景，自动复用已有的连接池配置和数据库准备逻辑
// 参数:
//   - ctx: 上下文，用于超时控制和取消
//   - cfg: 网关配置，包含 ClickHouse 连接参数
//   - log: 日志记录器
//
// 返回: *gorm.DB 实例，配置缺失或连接失败返回 nil
func NewClickHouseDB(ctx context.Context, cfg *gwconfig.Gateway, log logger.ILogger) *gorm.DB {
	if cfg == nil || cfg.ClickHouse == nil {
		log.WarnContext(ctx, "ClickHouse configuration not found, skipping gorm initialization")
		return nil
	}

	chCfg := cfg.ClickHouse

	if err := ensureClickHouseDatabase(ctx, chCfg, log); err != nil {
		log.ErrorContextKV(ctx, "ClickHouse database prepare failed", "host", chCfg.Host, "dbname", chCfg.Dbname, "err", err)
		return nil
	}

	dsn := buildClickHouseDSN(chCfg)

	logLevel := gormlogger.Warn
	if chCfg.Debug {
		logLevel = gormlogger.Info
	}

	gormDB, err := gorm.Open(gormch.Open(dsn), &gorm.Config{
		SkipDefaultTransaction:                   true,
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		log.ErrorContextKV(ctx, "ClickHouse gorm open failed", "error", err)
		return nil
	}

	sqlDB, err := gormDB.DB()
	if err == nil {
		sqlDB.SetMaxIdleConns(chCfg.MaxIdleConns)
		sqlDB.SetMaxOpenConns(chCfg.MaxOpenConns)
		if chCfg.ConnMaxIdleTime > 0 {
			sqlDB.SetConnMaxIdleTime(time.Duration(chCfg.ConnMaxIdleTime) * time.Second)
		}
		if chCfg.ConnMaxLifeTime > 0 {
			sqlDB.SetConnMaxLifetime(time.Duration(chCfg.ConnMaxLifeTime) * time.Second)
		}
	}

	log.InfoContextKV(ctx, "ClickHouse gorm connected successfully",
		"host", chCfg.Host,
		"port", chCfg.Port,
		"database", chCfg.Dbname,
	)

	return gormDB
}

// buildClickHouseOptions 根据 ClickHouse 配置构建原生驱动连接选项
// 包含地址、认证、压缩、超时、连接池、TLS 等配置
func buildClickHouseOptions(cfg *tsdb.ClickHouse) *clickhouse.Options {
	opts := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Database: cfg.Dbname,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		DialTimeout:     time.Duration(cfg.DialTimeout) * time.Second,
		ReadTimeout:     time.Duration(cfg.ReadTimeout) * time.Second,
		MaxIdleConns:    cfg.MaxIdleConns,
		MaxOpenConns:    cfg.MaxOpenConns,
		ConnMaxLifetime: time.Duration(cfg.ConnMaxLifeTime) * time.Second,
		Debug:           cfg.Debug,
	}

	// 启用 TLS 安全连接
	if cfg.Secure {
		opts.TLS = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	// HTTP 协议传输方式（默认为 Native TCP）
	if cfg.Protocol == "http" {
		opts.Protocol = clickhouse.HTTP
	}

	return opts
}

// buildClickHouseDSN 根据 ClickHouse 配置构建 DSN 连接字符串
// 支持原生协议和 HTTP 协议，支持安全连接和压缩选项
func buildClickHouseDSN(cfg *tsdb.ClickHouse) string {
	protocol := "clickhouse"
	if cfg.Protocol == "http" {
		protocol = "http"
	}

	dsn := fmt.Sprintf("%s://%s:%s@%s:%s/%s",
		protocol,
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Dbname,
	)

	if cfg.Secure {
		dsn += "?secure=true"
	}
	if cfg.Compress {
		if cfg.Secure {
			dsn += "&compress=true"
		} else {
			dsn += "?compress=true"
		}
	}

	return dsn
}
