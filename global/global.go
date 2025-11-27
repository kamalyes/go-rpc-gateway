/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-20 13:26:06
 * @FilePath: \go-rpc-gateway\global\global.go
 * @Description: 全局变量和配置管理 - 基于go-config的重构版本
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package global

import (
	"context"
	"fmt"
	"github.com/bwmarrin/snowflake"
	goconfig "github.com/kamalyes/go-config"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/cpool"
	"github.com/kamalyes/go-toolbox/pkg/safe"
	gowsc "github.com/kamalyes/go-wsc"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"time"
)

var (
	GATEWAY        *gwconfig.Gateway                         // 网关配置
	LOGGER         logger.ILogger                            // 日志器
	POOL_MANAGER   *cpool.Manager                            // 连接池管理器
	CONFIG_MANAGER *goconfig.IntegratedConfigManager         // 统一配置管理器
	CTX            context.Context                           // 全局上下文
	CANCEL         context.CancelFunc                        // 上下文取消函数
	WSCHUB         *gowsc.Hub                                // 全局WebSocket服务实例
	Node           *snowflake.Node                           // 雪花算法节点（用于分布式ID生成）
	LOG            logger.ILogger                            // 日志器别名（兼容旧代码）
	DB             *gorm.DB                                  // 数据库连接（暂未初始化）
	REDIS          *redis.Client                             // Redis连接（暂未初始化）
	MinIO          *minio.Client                             // MinIO连接（暂未初始化）
	GPerFix        string                            = "gw_" // 全局表前缀
)

// EnsureLoggerInitialized 确保全局日志器被正确初始化
func EnsureLoggerInitialized() error {
	// 如果全局日志器已经初始化，直接返回
	if LOGGER != nil {
		return nil
	}

	// 使用 go-logger 创建一个新的日志器实例
	newLogger := logger.CreateSimpleLogger(logger.DEBUG)
	if newLogger == nil {
		return fmt.Errorf("failed to create logger instance")
	}

	// 将新创建的 logger 赋值给全局变量
	LOGGER = newLogger
	LOG = newLogger // 兼容别名

	ctx := context.Background()
	LOGGER.InfoContext(ctx, "Logger initialized successfully with go-logger")
	return nil
}

// CleanupGlobal 清理全局资源
func CleanupGlobal() {
	ctx := context.Background()
	LOGGER.InfoContext(ctx, "🧹 开始清理全局资源")

	if CANCEL != nil {
		CANCEL()
	}

	// 关闭连接池管理器
	if POOL_MANAGER != nil {
		if err := POOL_MANAGER.Close(); err != nil {
			LOGGER.InfoContext(ctx, "❌ 关闭连接池管理器失败: %v", err)
		} else {
			LOGGER.InfoContext(ctx, "✅ 连接池管理器已关闭")
		}
	}

	// 停止配置管理器
	if CONFIG_MANAGER != nil {
		if err := CONFIG_MANAGER.Stop(); err != nil {
			LOGGER.InfoContext(ctx, "❌ 停止配置管理器失败: %v", err)
		} else {
			LOGGER.InfoContext(ctx, "✅ 配置管理器已停止")
		}
	}

	// 清理全局变量
	GATEWAY = nil
	CONFIG_MANAGER = nil
	POOL_MANAGER = nil
	REDIS = nil
	DB = nil
	MinIO = nil
	Node = nil
	CTX = nil
	CANCEL = nil
	WSCHUB = nil

	if LOGGER != nil {
		LOGGER.InfoContext(ctx, "✅ 全局资源清理完成")
		LOGGER = nil
	}
}

// GetConfig 获取当前配置
func GetConfig() *gwconfig.Gateway {
	return GATEWAY
}

// GetLogger 获取日志器
func GetLogger() logger.ILogger {
	return LOGGER
}

// GetPoolManager 获取连接池管理器
func GetPoolManager() *cpool.Manager {
	return POOL_MANAGER
}

// GetContext 获取全局上下文
func GetContext() context.Context {
	return CTX
}

// GetDB 获取数据库连接
func GetDB() *gorm.DB {
	return DB
}

// GetRedis 获取Redis连接
func GetRedis() *redis.Client {
	return REDIS
}

// GetMinIO 获取MinIO连接
func GetMinIO() *minio.Client {
	return MinIO
}

// GetSnowflakeNode 获取雪花算法节点
func GetSnowflakeNode() *snowflake.Node {
	return Node
}

// GetWebSocketService 获取全局WebSocket服务实例
func GetWebSocketService() *gowsc.Hub {
	return WSCHUB
}

// GetGatewayConfig 获取网关配置
func GetGatewayConfig() *gwconfig.Gateway {
	return GATEWAY
}

// GetConfigManager 获取配置管理器
func GetConfigManager() *goconfig.IntegratedConfigManager {
	return CONFIG_MANAGER
}

// ============================================================================
// WSC 消息归档配置访问函数
// ============================================================================

// GetWSCArchiveDays 获取消息归档天数阈值（默认3天）
// 从配置 WSC.Jobs.Tasks["message-archive"].Params["archive_days"] 读取
func GetWSCArchiveDays() int {
	return safe.Safe(GATEWAY).
		Field("WSC").
		Field("Jobs").
		Field("Tasks").
		Field("message-archive").
		Field("Params").
		Field("archive_days").
		Int(3)
}

// GetWSCArchiveRetentionDays 获取归档保留天数（默认60天）
// 从配置 WSC.Jobs.Tasks["message-archive"].Params["retention_days"] 读取
func GetWSCArchiveRetentionDays() int {
	return safe.Safe(GATEWAY).
		Field("WSC").
		Field("Jobs").
		Field("Tasks").
		Field("message-archive").
		Field("Params").
		Field("retention_days").
		Int(60)
}

// IsInitialized 检查是否已初始化
func IsInitialized() bool {
	return GATEWAY != nil && LOGGER != nil && CONFIG_MANAGER != nil
}

// ReloadConfig 手动重新加载配置
func ReloadConfig() error {
	if CONFIG_MANAGER == nil {
		return fmt.Errorf("配置管理器未初始化")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 通过热重载器进行配置重载
	if err := CONFIG_MANAGER.GetHotReloader().Reload(ctx); err != nil {
		return fmt.Errorf("重新加载配置失败: %w", err)
	}

	LOGGER.InfoContext(ctx, "🔄 配置重新加载成功")
	return nil
}

// GetEnvironment 获取当前环境
func GetEnvironment() goconfig.EnvironmentType {
	if CONFIG_MANAGER != nil {
		return CONFIG_MANAGER.GetEnvironment()
	}
	return goconfig.GetEnvironment()
}
