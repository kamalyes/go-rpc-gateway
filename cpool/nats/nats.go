/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-04-28 15:18:59
 * @FilePath: \go-rpc-gateway\cpool\nats\nats.go
 * @Description: NATS 连接封装类型定义
 *
 * 只持有 nats.go 官方类型，不再经任何第三方封装：连接池与 JetStream 都取自 nats-io/nats.go 本身，连接的生命周期由 Manager 统一管理
 *
 * 连接工厂 NewNats 按 mq 构建标签拆分（client_support.go / client_stub.go）：
 * 未启用 -tags mq 时 NewNats 返回 nil，nats.Connect 不可达，
 * 由链接器裁剪掉 nats.go 协议实现，仅保留本文件引用的类型元数据
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package nats

import (
	"github.com/nats-io/nats.go"
)

// NatsConn 封装 NATS 连接及其关联资源
// 两个字段都是 nats.go 官方类型，底层连接与 JetStream 上下文绑定在一起，避免分散管理
type NatsConn struct {
	Conn      *nats.Conn            // NATS 底层连接实例
	JetStream nats.JetStreamContext // JetStream 上下文（启用 JetStream 时非 nil）
}
