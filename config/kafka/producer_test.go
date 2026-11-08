/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 12:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 12:30:00
 * @FilePath: \go-rpc-gateway\config\kafka\producer_test.go
 * @Description: Kafka 生产者配置测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package kafka

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProducerValidate(t *testing.T) {
	cfg := DefaultProducerConfig()
	require.NoError(t, cfg.Validate())

	cases := []struct {
		name   string
		mutate func(*Producer)
	}{
		{"空 brokers", func(p *Producer) { p.Brokers = nil }},
		{"非法确认级别", func(p *Producer) { p.RequiredAcks = 2 }},
		{"非法压缩算法", func(p *Producer) { p.Compression = "brotli" }},
		{"SASL 只配置用户名", func(p *Producer) { p.Username = "user" }},
		{"SASL 机制不支持", func(p *Producer) { p.Username = "user"; p.Password = "pwd"; p.SASLMechanism = "oauth" }},
		{"TLS 只配置证书", func(p *Producer) { p.TLS.CertFile = "/tmp/cert.pem" }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			invalid := DefaultProducerConfig()
			testCase.mutate(&invalid)
			require.Error(t, invalid.Validate())
		})
	}
}

func TestProducerDoesNotBindTopic(t *testing.T) {
	// 生产者配置只承载连接与发送参数，目标 Topic 由 Send 显式传入
	_, ok := reflect.TypeOf(Producer{}).FieldByName("Topic")
	require.False(t, ok, "生产者配置不应绑定 Topic")
}

func TestProducerBrokersList(t *testing.T) {
	cfg := DefaultProducerConfig()
	cfg.Brokers = []string{" broker-1:9092 ", "", "broker-2:9092"}
	require.Equal(t, []string{"broker-1:9092", "broker-2:9092"}, cfg.BrokersList())
}

func TestProducerCloneAndSet(t *testing.T) {
	cfg := DefaultProducerConfig()
	cfg.Brokers = []string{"broker-1:9092"}
	cloned, ok := cfg.Clone().(*Producer)
	require.True(t, ok)
	require.NotSame(t, &cfg, cloned)
	cloned.Brokers[0] = "broker-2:9092"
	require.Equal(t, "broker-1:9092", cfg.Brokers[0], "Clone 必须是深拷贝")

	var target Producer
	target.Set(&cfg)
	require.Equal(t, cfg.Brokers, target.Brokers)
}
