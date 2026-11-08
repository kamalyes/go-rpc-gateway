//go:build !kafka

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 23:35:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 23:35:00
 * @FilePath: \go-rpc-gateway\cpool\manager_kafka_stub_test.go
 * @Description: 未启用 kafka 构建标签时的行为测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package cpool

import (
	"testing"

	kafkaconfig "github.com/kamalyes/go-rpc-gateway/config/kafka"
	kafkapool "github.com/kamalyes/go-rpc-gateway/cpool/kafka"
	"github.com/kamalyes/go-toolbox/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestKafkaNotCompiledIn(t *testing.T) {
	manager := NewManager(logger.NewEmptyLogger())
	t.Cleanup(func() { _ = manager.Close() })

	producerConfig := kafkaconfig.DefaultProducerConfig()
	require.ErrorIs(t, manager.AddKafkaProducer("events", &producerConfig), kafkapool.ErrNotEnabled)

	consumerConfig := kafkaconfig.DefaultConsumerConfig()
	require.ErrorIs(t, manager.AddKafkaConsumer("commands", &consumerConfig), kafkapool.ErrNotEnabled)
}
