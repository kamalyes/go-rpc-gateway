//go:build kafka

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 23:35:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 23:35:00
 * @FilePath: \go-rpc-gateway\cpool\manager_kafka_test.go
 * @Description: 管理器 Kafka 命名实例测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package cpool

import (
	"testing"

	kafkaconfig "github.com/kamalyes/go-rpc-gateway/config/kafka"
	"github.com/kamalyes/go-toolbox/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestKafkaProducerPool(t *testing.T) {
	manager := NewManager(logger.NewEmptyLogger())
	t.Cleanup(func() { _ = manager.Close() })

	cfg := kafkaconfig.DefaultProducerConfig()
	require.NoError(t, manager.AddKafkaProducer("events", &cfg))
	require.NotNil(t, manager.GetKafkaProducer("events"))
	require.Equal(t, []string{"events"}, manager.KafkaProducerNames())
	require.Error(t, manager.AddKafkaProducer("events", &cfg), "重复注册必须失败")
	require.True(t, manager.HealthCheck()["kafka.producer.events"])

	require.NoError(t, manager.RemoveKafkaProducer("events"))
	require.Nil(t, manager.GetKafkaProducer("events"))
}

func TestKafkaConsumerPool(t *testing.T) {
	manager := NewManager(logger.NewEmptyLogger())
	t.Cleanup(func() { _ = manager.Close() })

	cfg := kafkaconfig.DefaultConsumerConfig()
	require.NoError(t, manager.AddKafkaConsumer("commands", &cfg))
	require.NotNil(t, manager.GetKafkaConsumer("commands"))
	require.Equal(t, []string{"commands"}, manager.KafkaConsumerNames())
	require.Error(t, manager.AddKafkaConsumer("commands", &cfg), "重复注册必须失败")
	require.True(t, manager.HealthCheck()["kafka.consumer.commands"])

	require.NoError(t, manager.RemoveKafkaConsumer("commands"))
	require.Nil(t, manager.GetKafkaConsumer("commands"))
}
