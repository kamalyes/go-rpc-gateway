/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-15 11:37:08
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
func Redis(ctx context.Context, cfg *gwconfig.Gateway, log logger.ILogger) *redis.Client {
	// 检查缓存是否启用
	if !cfg.Cache.Enabled {
		return nil
	}

	// 检查Redis配置
	if cfg.Cache == nil {
		if log != nil {
			log.WarnContext(ctx, "Redis configuration not found")
		}
		return nil
	}

	// 使用配置创建Redis客户端
	redisCfg := cfg.Cache.Redis
	if redisCfg.Addr == "" {
		if log != nil {
			log.WarnContext(ctx, "Redis address not configured")
		}
		return nil
	}

	db := 0
	if redisCfg.DB >= 0 && redisCfg.DB <= 15 {
		db = redisCfg.DB
	}

	client := redis.NewClient(&redis.Options{
		Addr:             redisCfg.Addr,
		Username:         redisCfg.Username,
		Password:         redisCfg.Password,
		DB:               db,
		MaxRetries:       redisCfg.MaxRetries,
		PoolSize:         redisCfg.PoolSize,
		MinIdleConns:     redisCfg.MinIdleConns,
		MaxIdleConns:     redisCfg.MaxIdleConns,
		PoolTimeout:      redisCfg.PoolTimeout,
		DialTimeout:      redisCfg.DialTimeout,
		WriteTimeout:     redisCfg.WriteTimeout,
		ReadTimeout:      redisCfg.ReadTimeout,
		MaxRetryBackoff:  redisCfg.MaxRetryBackoff,
		MinRetryBackoff:  redisCfg.MinRetryBackoff,
		DisableIndentity: true, // 禁用客户端身份标识，避免 maint_notifications 错误
	})

	// 测试连接
	pong, err := client.Ping(ctx).Result()
	if err != nil {
		log.ErrorContextKV(ctx, "Redis connection failed", "addr", redisCfg.Addr, "db", db, "err", err)
		return nil
	}

	if log != nil {
		log.InfoContextKV(ctx, "Redis connect ping response", "pong", pong)
	}

	return client
}
