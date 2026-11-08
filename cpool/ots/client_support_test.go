//go:build ots

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 23:40:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 23:40:00
 * @FilePath: \go-rpc-gateway\cpool\ots\client_support_test.go
 * @Description: Tablestore 客户端构造与健康检查测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package ots

import (
	"context"
	"testing"

	otsconfig "github.com/kamalyes/go-rpc-gateway/config/ots"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	cfg := otsconfig.DefaultOTS()
	cfg.Enabled = true
	cfg.Endpoint = "example.cn-hangzhou.ots.aliyuncs.com"
	cfg.InstanceName = "example"

	client, err := NewClient(&cfg)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NoError(t, client.HealthCheck(context.Background()))
	require.NoError(t, client.Close())
}

func TestNewClientRejectsDisabledConfig(t *testing.T) {
	cfg := otsconfig.DefaultOTS()
	_, err := NewClient(&cfg)
	require.Error(t, err)
}
