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
// 使用 redis.NewUniversalClient 根据地址数量自动选择底层 client 类型：
//   - 单地址 → *redis.Client（单机）
//   - 多地址 → *redis.ClusterClient（集群）
//
// 返回 redis.UniversalClient 接口，统一单机与集群调用方式
func Redis(ctx context.Context, cfg *gwconfig.Gateway, log logger.ILogger) redis.UniversalClient {
	// 先检查配置是否存在，再检查是否启用，避免 cfg.Cache 为 nil 时 panic
	if cfg.Cache == nil {
		if log != nil {
			log.WarnContext(ctx, "Redis configuration not found")
		}
		return nil
	}

	if !cfg.Cache.Enabled {
		return nil
	}

	// 使用配置创建Redis客户端
	redisCfg := cfg.Cache.Redis

	// 统一地址列表：优先使用 Addrs（集群），回退到 Addr（单机兼容旧配置）
	addrs := redisCfg.Addrs
	if len(addrs) == 0 {
		if redisCfg.Addr == "" {
			if log != nil {
				log.WarnContext(ctx, "Redis address not configured")
			}
			return nil
		}
		addrs = []string{redisCfg.Addr}
	}

	// 构建通用配置，字段与 go-config Redis 结构体一一映射
	// MaxConnAge → ConnMaxLifetime, IdleTimeout → ConnMaxIdleTime（go-redis 字段名不同但语义一致）
	opts := &redis.UniversalOptions{
		Addrs:                 addrs,
		ClientName:            redisCfg.ClientName,
		Protocol:              redisCfg.Protocol,
		Username:              redisCfg.Username,
		Password:              redisCfg.Password,
		SentinelUsername:      redisCfg.SentinelUsername,
		SentinelPassword:      redisCfg.SentinelPassword,
		DB:                    redisCfg.DB,
		MaxRetries:            redisCfg.MaxRetries,
		MaxRedirects:          redisCfg.MaxRedirects,
		PoolSize:              redisCfg.PoolSize,
		MaxActiveConns:        redisCfg.MaxActiveConns,
		MinIdleConns:          redisCfg.MinIdleConns,
		MaxIdleConns:          redisCfg.MaxIdleConns,
		ReadBufferSize:        redisCfg.ReadBufferSize,
		WriteBufferSize:       redisCfg.WriteBufferSize,
		ConnMaxLifetime:       redisCfg.MaxConnAge,
		ConnMaxIdleTime:       redisCfg.IdleTimeout,
		DialTimeout:           redisCfg.DialTimeout,
		PoolTimeout:           redisCfg.PoolTimeout,
		ReadTimeout:           redisCfg.ReadTimeout,
		WriteTimeout:          redisCfg.WriteTimeout,
		MinRetryBackoff:       redisCfg.MinRetryBackoff,
		MaxRetryBackoff:       redisCfg.MaxRetryBackoff,
		ContextTimeoutEnabled: redisCfg.ContextTimeoutEnabled,
		ReadOnly:              redisCfg.ReadOnly,
		RouteByLatency:        redisCfg.RouteByLatency,
		RouteRandomly:         redisCfg.RouteRandomly,
		MasterName:            redisCfg.MasterName,
		IsClusterMode:         redisCfg.ClusterMode,
		DisableIdentity:       true, // 禁用 CLIENT SETINFO，避免 maint_notifications 错误
	}

	client := redis.NewUniversalClient(opts)

	// 测试连接
	if err := client.Ping(ctx).Err(); err != nil {
		if log != nil {
			log.ErrorContextKV(ctx, "Redis connection failed", "addrs", addrs, "err", err)
		}
		return nil
	}

	return client
}
