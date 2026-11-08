//go:build kafka

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 23:40:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 23:40:00
 * @FilePath: \go-rpc-gateway\cpool\kafka\kafka_support_test.go
 * @Description: Kafka 客户端构造与健康检查测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package kafka

import (
	"context"
	"testing"

	kafkaconfig "github.com/kamalyes/go-rpc-gateway/config/kafka"
	"github.com/stretchr/testify/require"
)

func TestNewProducer(t *testing.T) {
	cfg := kafkaconfig.DefaultProducerConfig()
	producer, err := NewProducer(&cfg)
	require.NoError(t, err)
	require.NotNil(t, producer)
	require.NoError(t, producer.HealthCheck(context.Background()))
	require.NoError(t, producer.Close())
}

func TestNewProducerRejectsInvalidConfig(t *testing.T) {
	cfg := kafkaconfig.DefaultProducerConfig()
	cfg.Brokers = nil
	_, err := NewProducer(&cfg)
	require.Error(t, err)
}

func TestNewConsumer(t *testing.T) {
	cfg := kafkaconfig.DefaultConsumerConfig()
	consumer, err := NewConsumer(&cfg)
	require.NoError(t, err)
	require.NotNil(t, consumer)
	require.NoError(t, consumer.HealthCheck(context.Background()))
	require.NoError(t, consumer.Close())
}

func TestNewConsumerRejectsInvalidConfig(t *testing.T) {
	cfg := kafkaconfig.DefaultConsumerConfig()
	cfg.Topic = ""
	_, err := NewConsumer(&cfg)
	require.Error(t, err)
}
