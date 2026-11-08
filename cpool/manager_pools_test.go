/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 12:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 12:30:00
 * @FilePath: \go-rpc-gateway\cpool\manager_pools_test.go
 * @Description: 管理器的命名实例与自定义池测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package cpool

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	kafkaconfig "github.com/kamalyes/go-rpc-gateway/config/kafka"
	otsconfig "github.com/kamalyes/go-rpc-gateway/config/ots"
	"github.com/kamalyes/go-toolbox/pkg/logger"
	"github.com/stretchr/testify/require"
)

type fakePool struct {
	closed  atomic.Bool
	healthy atomic.Bool
}

func (f *fakePool) Close() error {
	f.closed.Store(true)
	return nil
}

func (f *fakePool) HealthCheck(context.Context) error {
	if f.healthy.Load() {
		return nil
	}
	return errors.New("unhealthy")
}

func TestRegisterPoolAndHealth(t *testing.T) {
	manager := NewManager(logger.NewEmptyLogger())
	t.Cleanup(func() { _ = manager.Close() })

	pool := &fakePool{}
	pool.healthy.Store(true)
	require.NoError(t, manager.RegisterPool("cachex", pool))

	got, ok := manager.GetPool("cachex")
	require.True(t, ok)
	require.Same(t, pool, got)
	require.Equal(t, []string{"cachex"}, manager.PoolNames())
	require.True(t, manager.HealthCheck()["pool.cachex"])

	pool.healthy.Store(false)
	require.False(t, manager.HealthCheck()["pool.cachex"])

	require.NoError(t, manager.UnregisterPool("cachex"))
	require.True(t, pool.closed.Load())
	_, ok = manager.GetPool("cachex")
	require.False(t, ok)
}

func TestManagerCloseClosesRegisteredPool(t *testing.T) {
	manager := NewManager(logger.NewEmptyLogger())
	pool := &fakePool{}
	require.NoError(t, manager.RegisterPool("custom", pool))
	require.NoError(t, manager.Close())
	require.True(t, pool.closed.Load())
}

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

func TestOTSPool(t *testing.T) {
	manager := NewManager(logger.NewEmptyLogger())
	t.Cleanup(func() { _ = manager.Close() })

	disabled := otsconfig.DefaultOTS()
	require.Error(t, manager.AddOTS("main", &disabled), "未启用的 OTS 配置必须被拒绝")

	enabled := otsconfig.DefaultOTS()
	enabled.Enabled = true
	enabled.Endpoint = "example.cn-hangzhou.ots.aliyuncs.com"
	enabled.InstanceName = "example"
	require.NoError(t, manager.AddOTS("main", &enabled))
	require.NotNil(t, manager.GetOTS("main"))
	require.Equal(t, []string{"main"}, manager.OTSNames())
	require.True(t, manager.HealthCheck()["ots.main"])

	require.NoError(t, manager.RemoveOTS("main"))
	require.Nil(t, manager.GetOTS("main"))
}

func TestDatabaseNamesWithoutInstances(t *testing.T) {
	manager := NewManager(logger.NewEmptyLogger())
	t.Cleanup(func() { _ = manager.Close() })
	require.Empty(t, manager.DatabaseNames())
}
