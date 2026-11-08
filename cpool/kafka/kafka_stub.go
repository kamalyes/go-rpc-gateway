//go:build !kafka

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 23:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 23:30:00
 * @FilePath: \go-rpc-gateway\cpool\kafka\kafka_stub.go
 * @Description: Kafka 客户端空实现
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// kafka_stub 在未启用 kafka 构建标签时提供空实现，
// 使 cpool/kafka 包可被 import 而不引入 segmentio/kafka-go 及其传递依赖
// 需要 Kafka 支持时使用 -tags kafka 构建

package kafka

import (
	"context"
	"errors"

	kafkaconfig "github.com/kamalyes/go-rpc-gateway/config/kafka"
)

// ErrNotEnabled Kafka 支持未编译进二进制
var ErrNotEnabled = errors.New("Kafka 支持未编译进二进制，请使用 -tags kafka 重新构建")

// Producer Kafka 生产者占位
type Producer struct{}

// NewProducer Kafka 支持未编译进二进制
func NewProducer(_ *kafkaconfig.Producer) (*Producer, error) {
	return nil, ErrNotEnabled
}

// Close 空实现
func (p *Producer) Close() error { return nil }

// HealthCheck Kafka 支持未编译进二进制
func (p *Producer) HealthCheck(context.Context) error { return ErrNotEnabled }

// Consumer Kafka 消费者占位
type Consumer struct{}

// NewConsumer Kafka 支持未编译进二进制
func NewConsumer(_ *kafkaconfig.Consumer) (*Consumer, error) {
	return nil, ErrNotEnabled
}

// Close 空实现
func (c *Consumer) Close() error { return nil }

// HealthCheck Kafka 支持未编译进二进制
func (c *Consumer) HealthCheck(context.Context) error { return ErrNotEnabled }
