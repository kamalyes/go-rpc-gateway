/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 22:06:19
 * @FilePath: \go-rpc-gateway\cpool\redis\redis.go
 * @Description: Redis连接客户端，兼容Gateway结构
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package redis

import (
	"context"

	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/redis/go-redis/v9"
)

// Redis 初始化redis客户端
func Redis() *redis.Client {
	// 使用 global.GATEWAY 配置
	cfg := global.GATEWAY
	if cfg == nil {
		if global.LOGGER != nil {
			global.LOGGER.Warn("Gateway configuration not found")
		}
		return nil
	}

	// 检查Redis配置
	if cfg.Cache == nil {
		if global.LOGGER != nil {
			global.LOGGER.Warn("Redis configuration not found")
		}
		return nil
	}

	// 使用配置创建Redis客户端
	redisCfg := cfg.Cache.Redis
	if redisCfg.Addr == "" {
		if global.LOGGER != nil {
			global.LOGGER.Warn("Redis address not configured")
		}
		return nil
	}

	db := 0
	if redisCfg.DB >= 0 && redisCfg.DB <= 15 {
		db = redisCfg.DB
	}

	client := redis.NewClient(&redis.Options{
		Addr:         redisCfg.Addr,
		Password:     redisCfg.Password,
		DB:           db,
		MaxRetries:   redisCfg.MaxRetries,
		PoolSize:     redisCfg.PoolSize,
		MinIdleConns: redisCfg.MinIdleConns,
	})

	// 测试连接
	pong, err := client.Ping(context.Background()).Result()
	if err != nil {
		if global.LOGGER != nil {
			global.LOGGER.ErrorKV("Redis connect ping failed", "err", err)
		}
		return nil
	}

	if global.LOGGER != nil {
		global.LOGGER.InfoKV("Redis connect ping response", "pong", pong)
	}

	return client
}