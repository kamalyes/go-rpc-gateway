/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 22:55:49
 * @FilePath: \go-rpc-gateway\cpool\oss\minio.go
 * @Description: MinIO客户端，兼容Gateway结构
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package oss

import (
	"time"

	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Minio 初始化minio客户端
func Minio() *minio.Client {
	// 使用 global.GATEWAY 配置
	cfg := global.GATEWAY
	if cfg == nil {
		if global.LOGGER != nil {
			global.LOGGER.Warn("Gateway configuration not found")
		}
		return nil
	}

	// 检查MinIO配置
	if cfg.OSS == nil || cfg.OSS.Minio == nil {
		if global.LOGGER != nil {
			global.LOGGER.Warn("MinIO configuration not found")
		}
		return nil
	}

	minioCfg := cfg.OSS.Minio
	if minioCfg.Endpoint == "" {
		if global.LOGGER != nil {
			global.LOGGER.Warn("MinIO endpoint not configured")
		}
		return nil
	}

	// 创建MinIO客户端
	client, err := minio.New(minioCfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioCfg.AccessKey, minioCfg.SecretKey, ""),
		Secure: minioCfg.UseSSL,
	})
	if err != nil {
		if global.LOGGER != nil {
			global.LOGGER.ErrorKV("MinIO new client failed", "err", err)
		}
		return nil
	}

	// 检查服务状态
	_, err = client.HealthCheck(5 * time.Second)
	if err != nil {
		if global.LOGGER != nil {
			global.LOGGER.ErrorKV("MinIO connect ping failed", "err", err)
		}
		return nil
	}

	if global.LOGGER != nil {
		global.LOGGER.Info("MinIO client initialized successfully")
	}

	return client
}