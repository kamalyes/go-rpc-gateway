/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-15 13:10:50
 * @FilePath: \go-rpc-gateway\global\global.go
 * @Description: 全局变量和配置管理 - 基于go-config的重构版本
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package global

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/kamalyes/go-rpc-gateway/config"
	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/cpool"
	"github.com/kamalyes/go-toolbox/pkg/desensitize"
	"github.com/kamalyes/go-toolbox/pkg/logger"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// gatewayConfig 网关配置的权威存储
//
// 为什么要原子指针而不是直接用下面的 GATEWAY 变量：这份配置会热重载 ——
// gateway.go 的配置变更回调会整体换掉它，而请求路径上每个请求都要读一次
// （RequestContextMiddleware 每请求读 RequestContext 子树建提取索引）
// 一个普通的包级指针变量摆在这两者之间就是数据竞态，race detector 实测可复现：
//
//	WARNING: DATA RACE
//	Read at ...  middleware/core/request_context.go:160
//	Previous write at ...  gateway.go
//
// 这不只是「指针会不会被撕开」的问题 换上去的那份 Gateway，它的字段写入与指针写入之间
// 没有任何屏障，读侧完全可能看到指针已经换了、而结构体字段还没可见，
// 读到一份半初始化的配置 —— 而这条路径决定的是请求属性怎么提取
//
// [EN] Why an atomic pointer rather than the plain GATEWAY variable below: this config is
// hot-reloadable — the config-change callbacks in gateway.go replace it wholesale — while every
// request reads it on the request path (RequestContextMiddleware reads the RequestContext
// subtree per request to build its extraction index). A plain package-level pointer sitting
// between those two is a data race, reproducible under the race detector.
//
// It is not merely a question of whether the pointer can tear. There is no barrier between the
// field writes of the incoming Gateway and the write of the pointer itself, so a reader can
// perfectly well observe the new pointer while the struct's fields are not yet visible, and
// read a half-initialised config — on the very path that decides how request attributes are
// extracted.
var gatewayConfig atomic.Pointer[gwconfig.Gateway]

var (
	// GATEWAY 网关配置
	//
	// 仅为兼容保留：它是导出的，外部代码可能直接读它，所以 SetGatewayConfig 仍会同步写一次
	// 仓库内部一律走 GetGatewayConfig / GetConfig，不要新增对这个变量的直接读取 ——
	// 直接读它就是把上面那条竞态重新引进来
	//
	// [EN] Kept for compatibility only: it is exported and external code may read it directly,
	// so SetGatewayConfig keeps writing it. Everything inside this repository goes through
	// GetGatewayConfig or GetConfig; do not add new direct reads of this variable, because a
	// direct read is exactly how the race above comes back.
	GATEWAY        *gwconfig.Gateway
	LOGGER         logger.ILogger                          // 日志器
	POOL_MANAGER   *cpool.Manager                          // 连接池管理器（所有连接的唯一管理者）
	CONFIG_MANAGER *config.IntegratedConfigManager         // 统一配置管理器
	CTX            context.Context                         // 全局上下文
	CANCEL         context.CancelFunc                      // 上下文取消函数
	Node           *snowflake.Node                         // 雪花算法节点（用于分布式ID生成）
	LOG            logger.ILogger                          // 日志器别名（兼容旧代码）
	DB             *gorm.DB                                // 数据库连接（便捷引用，实际由 PoolManager 管理）
	REDIS          redis.UniversalClient                   // Redis连接（便捷引用，实际由 PoolManager 管理）
	DATAMASKER     *desensitize.DataMasker                 // 数据脱敏器
	SERVER_NODE    string                                  // 当前服务节点标识（K8s 环境下为 Pod 名称），用于响应头和 gRPC metadata 透传
	GPerFix        string                          = "gw_" // 全局表前缀
)

// EnsureLoggerInitialized 确保全局日志器被正确初始化
func EnsureLoggerInitialized() error {
	// 如果全局日志器已经初始化，直接返回
	if LOGGER != nil {
		return nil
	}

	// 使用 go-logger 创建一个新的日志器实例
	newLogger := logger.New()
	if newLogger == nil {
		return fmt.Errorf("failed to create logger instance")
	}

	// 将新创建的 logger 赋值给全局变量
	LOGGER = newLogger
	LOG = newLogger // 兼容别名
	return nil
}

// CleanupGlobal 清理全局资源
// 连接实例由 PoolManager 统一管理，此处只需关闭 PoolManager 即可释放所有连接
func CleanupGlobal() {
	ctx := context.Background()
	LOGGER.InfoContext(ctx, "🧹 开始清理全局资源")

	if CANCEL != nil {
		CANCEL()
	}

	// 关闭连接池管理器（会自动关闭所有连接：PG、Redis 等）
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

	// 清理全局变量（便捷引用置空即可，实际连接已由 PoolManager.Close() 释放）
	gatewayConfig.Store(nil)
	GATEWAY = nil
	CONFIG_MANAGER = nil
	POOL_MANAGER = nil
	REDIS = nil
	DB = nil
	Node = nil
	CTX = nil
	CANCEL = nil
	SERVER_NODE = ""

	if LOGGER != nil {
		LOGGER.InfoContext(ctx, "✅ 全局资源清理完成")
		LOGGER = nil
	}
}

// SetGatewayConfig 设置网关配置
//
// 启动与热重载都必须走这里：原子存储是权威来源，GATEWAY 变量同步更新只为兼容
// 外部直接读它的代码 两者顺序不能反 —— 先存原子，读侧才不会在新旧之间看到空档
//
// [EN] Both startup and hot reload must go through here: the atomic store is the source of
// truth and the GATEWAY variable is updated alongside it only for code that reads it directly.
// The order matters: the atomic is stored first so readers never see a gap between old and new.
func SetGatewayConfig(cfg *gwconfig.Gateway) {
	gatewayConfig.Store(cfg)
	GATEWAY = cfg
}

// GetConfig 获取当前配置
func GetConfig() *gwconfig.Gateway {
	return GetGatewayConfig()
}

// GetLogger 获取日志器
func GetLogger() logger.ILogger {
	return LOGGER
}

// GetPoolManager 获取连接池管理器（所有连接的唯一管理者）
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
func GetRedis() redis.UniversalClient {
	return REDIS
}

// GetSnowflakeNode 获取雪花算法节点
func GetSnowflakeNode() *snowflake.Node {
	return Node
}

// GetServerNode 获取当前服务节点标识（K8s 环境下为 Pod 名称）
// 用于 HTTP 响应头 X-Server-Node 和 gRPC metadata x-server-node 透传
func GetServerNode() string {
	return SERVER_NODE
}

// GetGatewayConfig 获取网关配置
//
// 原子存储为空时回退读 GATEWAY：单元测试与「只用中间件、不起整个网关」的场景里，
// 直接 `global.GATEWAY = cfg` 是既有写法，这条回退让它们照旧可用
// 这一支只在从未调用过 SetGatewayConfig 时才会走到，所以正常生命周期里不会去读那个变量
//
// [EN] Falls back to GATEWAY when the atomic is empty: assigning global.GATEWAY = cfg directly
// is the established pattern in unit tests and in setups that use the middlewares without
// starting the whole gateway, and this keeps them working. The branch is only reached when
// SetGatewayConfig was never called, so the normal lifecycle never reads that variable.
func GetGatewayConfig() *gwconfig.Gateway {
	if cfg := gatewayConfig.Load(); cfg != nil {
		return cfg
	}
	return GATEWAY
}

// GetConfigManager 获取配置管理器
func GetConfigManager() *config.IntegratedConfigManager {
	return CONFIG_MANAGER
}

// IsInitialized 检查是否已初始化
func IsInitialized() bool {
	return GetGatewayConfig() != nil && LOGGER != nil && CONFIG_MANAGER != nil
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
func GetEnvironment() config.EnvironmentType {
	if CONFIG_MANAGER != nil {
		return CONFIG_MANAGER.GetEnvironment()
	}
	return config.GetEnvironment()
}
