/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-28 15:18:59
 * @FilePath: \go-rpc-gateway\cpool\nats\client.go
 * @Description: NATS 连接工厂函数
 * 支持普通连接和 JetStream 持久化消息流两种模式
 * 集成 go-natsx 易用性封装，提供泛型发布/订阅、批量流式消费等高级功能
 * 遵循纯工厂函数模式，不维护包级全局状态，由 Manager 统一管理连接生命周期
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package nats

import (
	"context"
	"time"

	queue "github.com/kamalyes/go-config/pkg/queue"
	"github.com/kamalyes/go-logger"
	natsx "github.com/kamalyes/go-natsx"
	"github.com/nats-io/nats.go"
)

// NatsConn 封装 NATS 连接及其关联资源
// 将底层连接、JetStream 上下文和 go-natsx 客户端绑定在一起，避免分散管理
type NatsConn struct {
	Conn      *nats.Conn            // NATS 底层连接实例
	JetStream nats.JetStreamContext // JetStream 上下文（启用 JetStream 时非 nil）
	Client    *natsx.Client         // go-natsx 易用性封装客户端（启用时非 nil）
}

// NewNats 创建 NATS 连接
// 纯工厂函数，每次调用创建新连接，由调用方管理连接生命周期
// 支持通过函数式选项配置 JetStream、go-natsx 客户端、WorkerPool 等
// 参数:
//   - ctx: 上下文，用于超时控制和取消
//   - log: 日志记录器
//   - opts: 函数式选项，配置连接参数和高级功能
//
// 返回: NatsConn 封装实例，配置缺失或连接失败返回 nil
func NewNats(ctx context.Context, log logger.ILogger, opts *queue.Nats) *NatsConn {
	if opts == nil {
		log.WarnContext(ctx, "NATS configuration not found, skipping initialization")
		return nil
	}

	natsOpts := buildNatsOptions(opts, log)

	nc, err := nats.Connect(opts.URL, natsOpts...)
	if err != nil {
		log.ErrorContextKV(ctx, "NATS connection failed", "error", err)
		return nil
	}

	result := &NatsConn{Conn: nc}

	client, err := natsx.NewClient(nc, log)
	if err != nil {
		log.ErrorContextKV(ctx, "NATS go-natsx client creation failed", "error", err)
		nc.Close()
		return nil
	}
	result.Client = client

	// 如果配置了 JetStream，通过 go-natsx 客户端启用
	if opts.JetStream {
		if err := client.EnableJetStream(); err != nil {
			log.ErrorContextKV(ctx, "NATS JetStream initialization failed", "error", err)
			nc.Close()
			return nil
		}
		js := client.JetStream()
		result.JetStream = js

		log.InfoContextKV(ctx, "NATS JetStream enabled",
			"stream", opts.StreamName,
		)
	}

	// 如果配置了 WorkerPool，初始化全局消费者池
	if opts.WorkerPoolSize > 0 {
		client.InitWorkerPool(opts.WorkerPoolSize, opts.WorkerQueueSize)
		log.InfoContextKV(ctx, "NATS WorkerPool initialized",
			"workers", opts.WorkerPoolSize,
			"queueSize", opts.WorkerQueueSize,
		)
	}

	return result
}

// buildNatsOptions 根据 NATS 配置构建连接选项列表
// 包含客户端名称、重连策略、超时设置、各类事件回调处理器等
func buildNatsOptions(cfg *queue.Nats, log logger.ILogger) []nats.Option {
	opts := []nats.Option{
		nats.Name(cfg.Name),
		nats.ReconnectWait(time.Duration(cfg.ReconnectWait) * time.Second),
		nats.MaxReconnects(cfg.MaxReconnects),
		nats.Timeout(time.Duration(cfg.ConnectTimeout) * time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				log.WarnContextKV(context.Background(), "NATS disconnected",
					"url", nc.ConnectedUrl(),
					"error", err,
				)
			}
		}),
		// 重连成功回调
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.InfoContextKV(context.Background(), "NATS reconnected",
				"url", nc.ConnectedUrl(),
			)
		}),
		// 连接关闭回调
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.InfoContext(context.Background(), "NATS connection closed")
		}),
		// 错误回调
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			log.ErrorContextKV(context.Background(), "NATS error",
				"url", nc.ConnectedUrl(),
				"subject", sub.Subject,
				"error", err,
			)
		}),
	}

	// 如果配置了用户名和密码，添加用户信息认证
	if cfg.Username != "" && cfg.Password != "" {
		opts = append(opts, nats.UserInfo(cfg.Username, cfg.Password))
	}

	// 如果配置了 Token，添加 Token 认证
	if cfg.Token != "" {
		opts = append(opts, nats.Token(cfg.Token))
	}

	return opts
}
