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

func TestDatabaseNamesWithoutInstances(t *testing.T) {
	manager := NewManager(logger.NewEmptyLogger())
	t.Cleanup(func() { _ = manager.Close() })
	require.Empty(t, manager.DatabaseNames())
}
