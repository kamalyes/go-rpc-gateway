//go:build !ots

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 23:40:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 23:40:00
 * @FilePath: \go-rpc-gateway\cpool\ots\client_stub_test.go
 * @Description: 未启用 ots 构建标签时的空实现测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package ots

import (
	"testing"

	otsconfig "github.com/kamalyes/go-rpc-gateway/config/ots"
	"github.com/stretchr/testify/require"
)

func TestStubNotEnabled(t *testing.T) {
	cfg := otsconfig.DefaultOTS()
	cfg.Enabled = true
	cfg.Endpoint = "example.cn-hangzhou.ots.aliyuncs.com"
	cfg.InstanceName = "example"

	_, err := NewClient(&cfg)
	require.ErrorIs(t, err, ErrNotEnabled)
}
