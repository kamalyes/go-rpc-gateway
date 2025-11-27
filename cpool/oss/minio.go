/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 15:56:59
 * @FilePath: \go-rpc-gateway\cpool\oss\minio.go
 * @Description: MinIO客户端，兼容Gateway结构
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package oss

import (
	"context"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-logger"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"time"
)

// Minio 初始化minio客户端
func Minio(ctx context.Context, cfg *gwconfig.Gateway, log logger.ILogger) *minio.Client {
	if cfg.OSS == nil || cfg.OSS.Minio == nil {
		log.WarnContext(ctx, "MinIO configuration not found")
		return nil
	}

	minioCfg := cfg.OSS.Minio
	if minioCfg.Endpoint == "" {
		log.WarnContext(ctx, "MinIO endpoint not configured")
		return nil
	}

	// 创建MinIO客户端
	client, err := minio.New(minioCfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioCfg.AccessKey, minioCfg.SecretKey, ""),
		Secure: minioCfg.UseSSL,
	})
	if err != nil {
		log.ErrorContextKV(ctx, "MinIO new client failed", "endpoint", minioCfg.Endpoint, "err", err)
		return nil
	}

	// 检查服务状态
	_, err = client.HealthCheck(5 * time.Second)
	if err != nil {
		log.ErrorContextKV(ctx, "MinIO connect ping failed", "endpoint", minioCfg.Endpoint, "err", err)
		return nil
	}

	log.InfoContextKV(ctx, "MinIO client initialized successfully", "endpoint", minioCfg.Endpoint)

	return client
}
