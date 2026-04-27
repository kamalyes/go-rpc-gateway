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
	"database/sql"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-config/pkg/tsdb"
	"github.com/kamalyes/go-logger"
)

// NewClickHouse 创建 ClickHouse 原生连接（clickhouse-go/v2 驱动）
// 纯工厂函数，每次调用创建新连接，由调用方管理连接生命周期
// 参数:
//   - ctx: 上下文，用于超时控制和取消
//   - cfg: 网关配置，包含 ClickHouse 连接参数
//   - log: 日志记录器
//
// 返回: ClickHouse 原生连接实例，配置缺失或连接失败返回 nil
func NewClickHouse(ctx context.Context, cfg *gwconfig.Gateway, log logger.ILogger) clickhouse.Conn {
	if cfg == nil || cfg.ClickHouse == nil {
		log.WarnContext(ctx, "ClickHouse configuration not found, skipping initialization")
		return nil
	}

	chCfg := cfg.ClickHouse
	opts := buildClickHouseOptions(chCfg)

	conn, err := clickhouse.Open(opts)
	if err != nil {
		log.ErrorContextKV(ctx, "ClickHouse connection failed", "error", err)
		return nil
	}

	if err := conn.Ping(ctx); err != nil {
		log.ErrorContextKV(ctx, "ClickHouse ping failed", "error", err)
		conn.Close()
		return nil
	}

	log.InfoContextKV(ctx, "ClickHouse connected successfully",
		"host", chCfg.Host,
		"port", chCfg.Port,
		"database", chCfg.Dbname,
	)

	return conn
}

// NewClickHouseDB 创建 ClickHouse 标准库连接（database/sql 接口）
// 适用于需要使用标准 SQL 接口或兼容 database/sql 生态的场景
// 支持连接池配置（最大空闲连接、最大打开连接、连接生命周期等）
// 参数:
//   - ctx: 上下文，用于超时控制和取消
//   - cfg: 网关配置，包含 ClickHouse 连接参数
//   - log: 日志记录器
//
// 返回: *sql.DB 标准库连接实例，配置缺失或连接失败返回 nil
func NewClickHouseDB(ctx context.Context, cfg *gwconfig.Gateway, log logger.ILogger) *sql.DB {
	if cfg == nil || cfg.ClickHouse == nil {
		log.WarnContext(ctx, "ClickHouse configuration not found, skipping initialization")
		return nil
	}

	chCfg := cfg.ClickHouse
	dsn := buildClickHouseDSN(chCfg)

	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		log.ErrorContextKV(ctx, "ClickHouse sql.DB open failed", "error", err)
		return nil
	}

	// 配置连接池参数
	db.SetMaxIdleConns(chCfg.MaxIdleConns)
	db.SetMaxOpenConns(chCfg.MaxOpenConns)
	if chCfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(time.Duration(chCfg.ConnMaxIdleTime) * time.Second)
	}
	if chCfg.ConnMaxLifeTime > 0 {
		db.SetConnMaxLifetime(time.Duration(chCfg.ConnMaxLifeTime) * time.Second)
	}

	if err := db.PingContext(ctx); err != nil {
		log.ErrorContextKV(ctx, "ClickHouse sql.DB ping failed", "error", err)
		db.Close()
		return nil
	}

	log.InfoContextKV(ctx, "ClickHouse sql.DB connected successfully",
		"host", chCfg.Host,
		"port", chCfg.Port,
		"database", chCfg.Dbname,
	)

	return db
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
