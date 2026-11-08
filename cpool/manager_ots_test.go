//go:build ots

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 23:35:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 23:35:00
 * @FilePath: \go-rpc-gateway\cpool\manager_ots_test.go
 * @Description: 管理器 Tablestore 命名实例测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package cpool

import (
	"testing"

	otsconfig "github.com/kamalyes/go-rpc-gateway/config/ots"
	"github.com/kamalyes/go-toolbox/pkg/logger"
	"github.com/stretchr/testify/require"
)

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
