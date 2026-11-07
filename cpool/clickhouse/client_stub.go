//go:build !clickhouse

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-28 15:20:16
 * @FilePath: \go-rpc-gateway\cpool\clickhouse\client_stub.go
 * @Description: ClickHouse 驱动空实现
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// client_stub 在未启用 clickhouse 构建标签时提供 NewClickHouseDB 的空实现，
// 使 cpool/clickhouse 包可被 import 而不引入 ClickHouse/clickhouse-go/v2
// 及 lz4、segmentio/asm 等传递依赖
// 需要 ClickHouse 支持时使用 -tags clickhouse 构建

package clickhouse

import (
	"context"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-toolbox/pkg/logger"
	"gorm.io/gorm"
)

// NewClickHouseDB ClickHouse 驱动未编译进二进制
// 需要时使用 -tags clickhouse 重新构建以启用 ClickHouse 支持
func NewClickHouseDB(ctx context.Context, cfg *gwconfig.Gateway, log logger.ILogger) *gorm.DB {
	if cfg == nil || cfg.ClickHouse == nil || !cfg.ClickHouse.Enabled {
		return nil
	}
	log.WarnContext(ctx, "ClickHouse driver not compiled in; rebuild with -tags clickhouse to enable ClickHouse support")
	return nil
}
