//go:build !mq

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-28 15:18:59
 * @FilePath: \go-rpc-gateway\cpool\nats\client_stub.go
 * @Description: NATS 连接工厂空实现
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// client_stub 在未启用 mq 构建标签时提供 NewNats 的空实现，
// nats.Connect 不可达，链接器据此裁剪 nats.go 协议实现
// 需要 NATS 支持时使用 -tags mq 构建

package nats

import (
	"context"

	queue "github.com/kamalyes/go-rpc-gateway/config/queue"
	"github.com/kamalyes/go-toolbox/pkg/logger"
)

// NewNats NATS 客户端未编译进二进制
// 需要时使用 -tags mq 重新构建以启用 NATS 支持
func NewNats(ctx context.Context, log logger.ILogger, opts *queue.Nats) *NatsConn {
	if opts == nil {
		log.WarnContext(ctx, "NATS configuration not found, skipping initialization")
		return nil
	}
	log.WarnContext(ctx, "NATS client not compiled in; rebuild with -tags mq to enable NATS support")
	return nil
}
