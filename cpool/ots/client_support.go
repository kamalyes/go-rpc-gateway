//go:build ots

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 11:35:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 11:35:00
 * @FilePath: \go-rpc-gateway\cpool\ots\client.go
 * @Description: 阿里云 Tablestore 客户端
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// Package ots 提供阿里云 Tablestore（OTS）客户端
//
// 客户端只负责连接参数与超时配置，表、行、多元索引等业务操作通过 Raw 取到底层 SDK 后调用
//
// [EN] Package ots provides an Alibaba Cloud Tablestore client. The wrapper owns connection
// parameters and timeouts; table, row and search-index operations use Raw to reach the
// underlying SDK.
package ots

import (
	"context"
	"errors"
	"time"

	"github.com/aliyun/aliyun-tablestore-go-sdk/tablestore"
	otsconfig "github.com/kamalyes/go-rpc-gateway/config/ots"
)

const defaultHTTPTimeout = 10 * time.Second

// Client Tablestore 客户端
type Client struct {
	client *tablestore.TableStoreClient
}

// NewClient 创建 Tablestore 客户端
func NewClient(cfg *otsconfig.OTS) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("Tablestore 配置为空")
	}
	if !cfg.Enabled {
		return nil, errors.New("Tablestore 未启用")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	httpTimeout := defaultHTTPTimeout
	if cfg.HTTPTimeout > 0 {
		httpTimeout = time.Duration(cfg.HTTPTimeout) * time.Second
	}
	config := tablestore.NewDefaultTableStoreConfig()
	if cfg.RetryTimes > 0 {
		config.RetryTimes = uint(cfg.RetryTimes)
	}
	if cfg.MaxIdleConns > 0 {
		config.MaxIdleConnections = cfg.MaxIdleConns
	}
	config.HTTPTimeout.ConnectionTimeout = httpTimeout
	config.HTTPTimeout.RequestTimeout = httpTimeout

	client := tablestore.NewClientWithConfig(
		cfg.Endpoint,
		cfg.InstanceName,
		cfg.AccessKeyID,
		cfg.AccessKeySecret,
		cfg.SecurityToken,
		config,
	)
	return &Client{client: client}, nil
}

// Raw 返回底层 Tablestore 客户端
func (c *Client) Raw() *tablestore.TableStoreClient { return c.client }

// HealthCheck 检查客户端是否已初始化
// Tablestore SDK 没有轻量级连通性探测接口，真正的连通性由首次业务请求暴露，
// 这里只拦截「客户端未创建」这一种确定性错误
func (c *Client) HealthCheck(_ context.Context) error {
	if c == nil || c.client == nil {
		return errors.New("Tablestore 客户端未初始化")
	}
	return nil
}

// Close 释放客户端资源
// Tablestore SDK 客户端不持有需要显式关闭的资源
func (c *Client) Close() error { return nil }
