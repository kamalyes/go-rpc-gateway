//go:build !kafka

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 23:40:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 23:40:00
 * @FilePath: \go-rpc-gateway\cpool\kafka\kafka_stub_test.go
 * @Description: 未启用 kafka 构建标签时的空实现测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package kafka

import (
	"testing"

	kafkaconfig "github.com/kamalyes/go-rpc-gateway/config/kafka"
	"github.com/stretchr/testify/require"
)

func TestStubNotEnabled(t *testing.T) {
	producerConfig := kafkaconfig.DefaultProducerConfig()
	_, err := NewProducer(&producerConfig)
	require.ErrorIs(t, err, ErrNotEnabled)

	consumerConfig := kafkaconfig.DefaultConsumerConfig()
	_, err = NewConsumer(&consumerConfig)
	require.ErrorIs(t, err, ErrNotEnabled)
}
