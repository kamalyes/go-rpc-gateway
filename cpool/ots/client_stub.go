//go:build !ots

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 23:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 23:30:00
 * @FilePath: \go-rpc-gateway\cpool\ots\client_stub.go
 * @Description: Tablestore 客户端空实现
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// client_stub 在未启用 ots 构建标签时提供空实现，
// 使 cpool/ots 包可被 import 而不引入阿里云 Tablestore SDK 及其传递依赖
// 需要 Tablestore 支持时使用 -tags ots 构建

package ots

import (
	"context"
	"errors"

	otsconfig "github.com/kamalyes/go-rpc-gateway/config/ots"
)

// ErrNotEnabled Tablestore 支持未编译进二进制
var ErrNotEnabled = errors.New("Tablestore 支持未编译进二进制，请使用 -tags ots 重新构建")

// Client Tablestore 客户端占位
type Client struct{}

// NewClient Tablestore 支持未编译进二进制
func NewClient(_ *otsconfig.OTS) (*Client, error) {
	return nil, ErrNotEnabled
}

// Close 空实现
func (c *Client) Close() error { return nil }

// HealthCheck Tablestore 支持未编译进二进制
func (c *Client) HealthCheck(context.Context) error { return ErrNotEnabled }
