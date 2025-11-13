/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 07:50:57
 * @FilePath: \go-rpc-gateway\cpool\redis\redis.go
 * @Description: Redis连接客户端，兼容Gateway结构
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package redis

import (
	"context"

	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-logger"
	"github.com/redis/go-redis/v9"
)

// Redis 初始化redis客户端
func Redis(cfg *gwconfig.Gateway, log logger.ILogger) *redis.Client {
	if cfg == nil {
		if log != nil {
			log.Warn("Gateway configuration not found")
		}
		return nil
	}

	// 检查Redis配置
	if cfg.Cache == nil {
		if log != nil {
			log.Warn("Redis configuration not found")
		}
		return nil
	}

	// 使用配置创建Redis客户端
	redisCfg := cfg.Cache.Redis
	if redisCfg.Addr == "" {
		if log != nil {
			log.Warn("Redis address not configured")
		}
		return nil
	}

	db := 0
	if redisCfg.DB >= 0 && redisCfg.DB <= 15 {
		db = redisCfg.DB
	}

	client := redis.NewClient(&redis.Options{
		Addr:             redisCfg.Addr,
		Password:         redisCfg.Password,
		DB:               db,
		MaxRetries:       redisCfg.MaxRetries,
		PoolSize:         redisCfg.PoolSize,
		MinIdleConns:     redisCfg.MinIdleConns,
		DisableIndentity: true, // 禁用客户端身份标识，避免 maint_notifications 错误
	})

	// 测试连接
	pong, err := client.Ping(context.Background()).Result()
	if err != nil {
		if log != nil {
			log.ErrorKV("Redis connect ping failed", "err", err)
		}
		return nil
	}

	if log != nil {
		log.InfoKV("Redis connect ping response", "pong", pong)
	}

	return client
}
