/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 12:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 12:30:00
 * @FilePath: \go-rpc-gateway\config\kafka\consumer_test.go
 * @Description: Kafka 消费者配置测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package kafka

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConsumerValidate(t *testing.T) {
	cfg := DefaultConsumerConfig()
	require.NoError(t, cfg.Validate())

	cases := []struct {
		name   string
		mutate func(*Consumer)
	}{
		{"空 brokers", func(c *Consumer) { c.Brokers = nil }},
		{"空主题", func(c *Consumer) { c.Topic = " " }},
		{"非法位点", func(c *Consumer) { c.Offset = "middle" }},
		{"拉取范围颠倒", func(c *Consumer) { c.MinBytes = 1024; c.MaxBytes = 512 }},
		{"负提交间隔", func(c *Consumer) { c.CommitInterval = -1 }},
		{"SASL 只配置密码", func(c *Consumer) { c.Password = "pwd" }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			invalid := DefaultConsumerConfig()
			testCase.mutate(&invalid)
			require.Error(t, invalid.Validate())
		})
	}
}

func TestConsumerAllowsFixedPartitionWithoutGroup(t *testing.T) {
	cfg := DefaultConsumerConfig()
	cfg.GroupID = ""
	cfg.Partition = 0
	require.NoError(t, cfg.Validate())
}

func TestConsumerRejectsNegativePartitionWithoutGroup(t *testing.T) {
	cfg := DefaultConsumerConfig()
	cfg.GroupID = ""
	cfg.Partition = -1
	require.Error(t, cfg.Validate())
}

func TestConsumerBrokersList(t *testing.T) {
	cfg := DefaultConsumerConfig()
	cfg.Brokers = []string{" broker-1:9092 ", "", "broker-2:9092"}
	require.Equal(t, []string{"broker-1:9092", "broker-2:9092"}, cfg.BrokersList())
}
