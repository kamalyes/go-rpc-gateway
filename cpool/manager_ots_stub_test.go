//go:build !ots

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 23:35:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 23:35:00
 * @FilePath: \go-rpc-gateway\cpool\manager_ots_stub_test.go
 * @Description: 未启用 ots 构建标签时的行为测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package cpool

import (
	"testing"

	otsconfig "github.com/kamalyes/go-rpc-gateway/config/ots"
	otspool "github.com/kamalyes/go-rpc-gateway/cpool/ots"
	"github.com/kamalyes/go-toolbox/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestOTSNotCompiledIn(t *testing.T) {
	manager := NewManager(logger.NewEmptyLogger())
	t.Cleanup(func() { _ = manager.Close() })

	cfg := otsconfig.DefaultOTS()
	cfg.Enabled = true
	cfg.Endpoint = "example.cn-hangzhou.ots.aliyuncs.com"
	cfg.InstanceName = "example"
	require.ErrorIs(t, manager.AddOTS("main", &cfg), otspool.ErrNotEnabled)
}
