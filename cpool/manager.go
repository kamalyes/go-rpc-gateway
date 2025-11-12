/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 21:49:01
 * @FilePath: \go-rpc-gateway\cpool\manager.go
 * @Description: 连接池管理器，统一管理数据库、Redis、OSS等客户端连接
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package cpool

import (
	"context"
	"fmt"
	"sync"

	"github.com/bwmarrin/snowflake"
	"github.com/casbin/casbin/v2"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	cachex "github.com/kamalyes/go-cachex"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	logger "github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/cpool/database"
	"github.com/kamalyes/go-rpc-gateway/cpool/oss"
	"github.com/kamalyes/go-rpc-gateway/cpool/redis"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/minio/minio-go/v7"
	redisClient "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// PoolManager 连接池管理器接口
type PoolManager interface {
	// 初始化所有连接池
	Initialize(ctx context.Context, cfg *gwconfig.Gateway) error
	
	// 获取数据库连接
	GetDB() *gorm.DB
	
	// 获取Redis客户端
	GetRedis() *redisClient.Client
	
	// 获取缓存客户端
	GetCache() cachex.CtxCache
	
	// 获取MinIO客户端
	GetMinIO() *minio.Client
	
	// 获取MQTT客户端
	GetMQTT() mqtt.Client
	
	// 获取雪花ID生成器
	GetSnowflake() *snowflake.Node
	
	// 获取Casbin执行器
	GetCasbin() casbin.IEnforcer
	
	// 关闭所有连接
	Close() error
	
	// 检查连接状态
	HealthCheck() map[string]bool
}

// Manager 连接池管理器实现
type Manager struct {
	cfg    *gwconfig.Gateway
	logger *logger.Logger
	
	// 连接实例
	db        *gorm.DB
	redis     *redisClient.Client
	cache     cachex.CtxCache
	minio     *minio.Client
	mqtt      mqtt.Client
	snowflake *snowflake.Node
	casbin    casbin.IEnforcer
	
	// 状态管理
	initialized bool
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewManager 创建新的连接池管理器
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Initialize 初始化所有连接池
func (m *Manager) Initialize(ctx context.Context, cfg *gwconfig.Gateway) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.initialized {
		return fmt.Errorf("pool manager already initialized")
	}
	
	m.cfg = cfg
	m.logger = global.LOGGER
	
	if m.logger == nil {
		return fmt.Errorf("global logger not initialized")
	}
	
	// 设置全局配置
	global.GATEWAY = cfg
	
	// 初始化各个连接池
	if err := m.initDatabase(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	
	if err := m.initRedis(); err != nil {
		return fmt.Errorf("failed to initialize redis: %w", err)
	}
	
	if err := m.initCache(); err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}
	
	if err := m.initMinIO(); err != nil {
		return fmt.Errorf("failed to initialize minio: %w", err)
	}
	
	if err := m.initMQTT(); err != nil {
		return fmt.Errorf("failed to initialize mqtt: %w", err)
	}
	
	if err := m.initSnowflake(); err != nil {
		return fmt.Errorf("failed to initialize snowflake: %w", err)
	}
	
	if err := m.initCasbin(); err != nil {
		return fmt.Errorf("failed to initialize casbin: %w", err)
	}
	
	m.initialized = true
	m.logger.Info("Connection pool manager initialized successfully")
	
	return nil
}

// initDatabase 初始化数据库连接
func (m *Manager) initDatabase() error {
	db := database.Gorm()
	if db != nil {
		m.db = db
		global.DB = db
		m.logger.Info("Database initialized successfully")
	} else {
		m.logger.Warn("Failed to initialize database")
	}
	
	return nil
}

// initRedis 初始化Redis连接
func (m *Manager) initRedis() error {
	// 检查 Redis 配置是否存在
	rdb := redis.Redis()
	if rdb != nil {
		m.redis = rdb
		global.REDIS = rdb
		m.logger.Info("Redis initialized successfully")
	} else {
		m.logger.Warn("Failed to initialize Redis")
	}
	
	return nil
}

// initCache 初始化缓存
func (m *Manager) initCache() error {
	// 如果有Redis，使用Redis作为缓存后端
	if m.redis != nil {
		// 这里可以初始化基于Redis的缓存
		m.logger.Info("Cache will use Redis as backend")
	}
	return nil
}

// initMinIO 初始化MinIO客户端
func (m *Manager) initMinIO() error {
	// 检查 MinIO 配置是否存在  
	minio := oss.Minio()
	if minio != nil {
		m.minio = minio
		global.MinIO = minio
		m.logger.Info("MinIO initialized successfully")
	} else {
		m.logger.Warn("Failed to initialize MinIO")
	}
	
	return nil
}

// initMQTT 初始化MQTT客户端
func (m *Manager) initMQTT() error {
	// MQTT客户端初始化暂时跳过，等待具体实现
	m.logger.Info("MQTT initialization skipped (not implemented)")
	return nil
}

// initSnowflake 初始化雪花ID生成器
func (m *Manager) initSnowflake() error {
	// 使用默认节点ID 1
	node, err := snowflake.NewNode(1)
	if err != nil {
		m.logger.ErrorKV("Failed to create snowflake node", "error", err)
		return nil // 非关键组件，不阻止启动
	}
	
	m.snowflake = node
	global.Node = node
	m.logger.Info("Snowflake ID generator initialized successfully")
	
	return nil
}

// initCasbin 初始化权限管理
func (m *Manager) initCasbin() error {
	// Casbin初始化暂时跳过，等待具体实现
	m.logger.Info("Casbin initialization skipped (not implemented)")
	return nil
}

// initJWT 初始化JWT管理器
// Getter methods
func (m *Manager) GetDB() *gorm.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.db
}

func (m *Manager) GetRedis() *redisClient.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.redis
}

func (m *Manager) GetCache() cachex.CtxCache {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cache
}

func (m *Manager) GetMinIO() *minio.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.minio
}

func (m *Manager) GetMQTT() mqtt.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mqtt
}

func (m *Manager) GetSnowflake() *snowflake.Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snowflake
}

func (m *Manager) GetCasbin() casbin.IEnforcer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.casbin
}

// Close 关闭所有连接
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.initialized {
		return nil
	}
	
	var errs []error
	
	// 关闭数据库连接
	if m.db != nil {
		if sqlDB, err := m.db.DB(); err == nil {
			if err := sqlDB.Close(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close database: %w", err))
			}
		}
	}
	
	// 关闭Redis连接
	if m.redis != nil {
		if err := m.redis.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close redis: %w", err))
		}
	}
	
	// 关闭MQTT连接
	if m.mqtt != nil {
		if m.mqtt.IsConnected() {
			m.mqtt.Disconnect(250)
		}
	}
	
	// 取消上下文
	if m.cancel != nil {
		m.cancel()
	}
	
	m.initialized = false
	
	if len(errs) > 0 {
		return fmt.Errorf("errors closing connections: %v", errs)
	}
	
	m.logger.Info("Connection pool manager closed successfully")
	return nil
}

// HealthCheck 检查所有连接的健康状态
func (m *Manager) HealthCheck() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	status := make(map[string]bool)
	
	// 检查数据库
	if m.db != nil {
		if sqlDB, err := m.db.DB(); err == nil {
			status["database"] = sqlDB.Ping() == nil
		} else {
			status["database"] = false
		}
	}
	
	// 检查Redis
	if m.redis != nil {
		ctx := context.Background()
		_, err := m.redis.Ping(ctx).Result()
		status["redis"] = err == nil
	}
	
	// 检查MinIO
	if m.minio != nil {
		_, err := m.minio.HealthCheck(3)
		status["minio"] = err == nil
	}
	
	// 检查MQTT
	if m.mqtt != nil {
		status["mqtt"] = m.mqtt.IsConnected()
	}
	
	return status
}