/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 07:32:38
 * @FilePath: \go-rpc-gateway\global\global.go
 * @Description: 全局变量和配置管理 - 基于go-config的重构版本
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package global

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/snowflake"
	goconfig "github.com/kamalyes/go-config"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/cpool"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	GATEWAY         *gwconfig.Gateway                      // 网关配置
	LOGGER          logger.ILogger                       // 日志器
	POOL_MANAGER    *cpool.Manager                       // 连接池管理器
	CONFIG_MANAGER  *goconfig.IntegratedConfigManager      // 统一配置管理器
	CTX             context.Context                        // 全局上下文
	CANCEL          context.CancelFunc                     // 上下文取消函数
	Node            *snowflake.Node                        // 雪花算法节点（用于分布式ID生成）
	LOG             logger.ILogger                       // 日志器别名（兼容旧代码）
	DB              *gorm.DB                            // 数据库连接（暂未初始化）
	REDIS           *redis.Client                       // Redis连接（暂未初始化）
	MinIO           *minio.Client                       // MinIO连接（暂未初始化）
	GPerFix 	    string = "gw_"                         // 全局表前缀
)

// InitializeGatewayWithConfigPath 基于配置文件路径初始化 Gateway（使用go-config）
func InitializeGatewayWithConfigPath(configPath string) error {
	LOGGER.Info("🚀 开始基于配置文件初始化 Gateway: %s\n", configPath)
	
	// 创建配置实例
	config := &gwconfig.Gateway{}
	
	// 使用go-config创建并启动配置管理器
	manager, err := goconfig.CreateAndStartIntegratedManager(
		config, 
		configPath, 
		goconfig.GetEnvironment(),
	)
	if err != nil {
		return fmt.Errorf("创建配置管理器失败: %w", err)
	}
	
	// 设置全局变量
	CONFIG_MANAGER = manager
	GATEWAY = manager.GetConfig().(*gwconfig.Gateway)
	
	// 初始化全局上下文
	CTX, CANCEL = context.WithCancel(context.Background())
	
	// 注册配置变更回调
	if err := registerConfigCallbacks(); err != nil {
		return fmt.Errorf("注册配置回调失败: %w", err)
	}
	
	// 初始化其他组件
	if err := initializeComponents(); err != nil {
		return fmt.Errorf("初始化组件失败: %w", err)
	}
	
	LOGGER.Info("✅ Gateway 初始化成功: %s\n", GATEWAY.Name)
	return nil
}

// InitializeGatewayWithAutoDiscovery 基于自动发现初始化 Gateway
func InitializeGatewayWithAutoDiscovery(searchPath string) error {
	LOGGER.Info("🔍 开始自动发现配置初始化 Gateway: %s\n", searchPath)
	
	// 获取当前环境
	env := goconfig.GetEnvironment()
	LOGGER.Info("🌍 当前环境: %s\n", env)
	
	// 扫描并显示所有可用的配置文件
	configs, err := goconfig.ScanAndDisplayConfigs(searchPath, env)
	if err != nil {
		return fmt.Errorf("扫描配置文件失败: %w", err)
	}
	
	// 创建配置实例
	config := &gwconfig.Gateway{}
	
	// 使用自动发现创建集成管理器
	manager, err := goconfig.CreateAndStartIntegratedManagerWithAutoDiscovery(
		config, searchPath, env, "gateway")
	if err != nil {
		return fmt.Errorf("自动发现创建配置管理器失败: %w", err)
	}
	
	// 设置全局变量
	CONFIG_MANAGER = manager
	GATEWAY = manager.GetConfig().(*gwconfig.Gateway)
	
	// 初始化全局上下文
	CTX, CANCEL = context.WithCancel(context.Background())
	
	// 注册配置变更回调
	if err := registerConfigCallbacks(); err != nil {
		return fmt.Errorf("注册配置回调失败: %w", err)
	}
	
	// 初始化其他组件
	if err := initializeComponents(); err != nil {
		return fmt.Errorf("初始化组件失败: %w", err)
	}
	
	LOGGER.Info("✅ Gateway 自动发现初始化成功: %s (找到%d个配置文件)\n", 
		GATEWAY.Name, len(configs))
	return nil
}

// registerConfigCallbacks 注册配置变更回调
func registerConfigCallbacks() error {
	if CONFIG_MANAGER == nil {
		return fmt.Errorf("配置管理器未初始化")
	}
	
	// 注册配置变更回调
	err := CONFIG_MANAGER.RegisterConfigCallback(func(ctx context.Context, event goconfig.CallbackEvent) error {
		if newConfig, ok := event.NewValue.(*gwconfig.Gateway); ok {
			LOGGER.Info("📋 配置已更新: %s\n", newConfig.Name)
			GATEWAY = newConfig
			
			// 重新初始化日志器（如果日志配置发生变化）
			if err := initializeLogger(); err != nil {
				LOGGER.Info("❌ 重新初始化日志器失败: %v\n", err)
			}
			
			LOGGER.Info("🔄 配置热更新完成\n")
		}
		return nil
	}, goconfig.CallbackOptions{
		ID:       "gateway_config_handler",
		Types:    []goconfig.CallbackType{goconfig.CallbackTypeConfigChanged},
		Priority: goconfig.CallbackPriorityHigh,
		Async:    false,
		Timeout:  5 * time.Second,
	})
	
	if err != nil {
		return fmt.Errorf("注册配置变更回调失败: %w", err)
	}
	
	// 注册环境变更回调
	err = CONFIG_MANAGER.RegisterEnvironmentCallback("gateway_env_handler", 
		func(oldEnv, newEnv goconfig.EnvironmentType) error {
			LOGGER.Info("🌍 环境变更: %s -> %s\n", oldEnv, newEnv)
			return nil
		}, goconfig.CallbackPriorityHigh, false)
	
	if err != nil {
		return fmt.Errorf("注册环境变更回调失败: %w", err)
	}
	
	return nil
}

// initializeComponents 初始化其他组件
func initializeComponents() error {
	// 初始化日志器
	if err := initializeLogger(); err != nil {
		return fmt.Errorf("初始化日志器失败: %w", err)
	}
	
	// 初始化连接池管理器
	if err := initializePoolManager(); err != nil {
		return fmt.Errorf("初始化连接池管理器失败: %w", err)
	}
	
	// 初始化Snowflake节点（用于分布式ID生成）
	if err := initializeSnowflakeNode(); err != nil {
		return fmt.Errorf("初始化Snowflake节点失败: %w", err)
	}
	
	// 从pool manager中绑定全局资源
	if err := bindPoolResourcesToGlobal(); err != nil {
		return fmt.Errorf("绑定池资源到全局失败: %w", err)
	}
	
	return nil
}

// initializeLogger 初始化日志器
func initializeLogger() error {
	if GATEWAY == nil {
		return fmt.Errorf("GATEWAY 配置为空")
	}
	
	// 根据配置设置日志级别
	level := logger.INFO
	if GATEWAY.Debug {
		level = logger.DEBUG
	}
	
	// 如果已存在日志器，更新级别；否则创建新的
	if LOGGER != nil {
		// 这里可以添加重新配置日志器的逻辑
		LOGGER.Info("🔄 更新日志器配置: level=%s, debug=%t\n", level.String(), GATEWAY.Debug)
	} else {
		// 创建新的日志器
		LOGGER = logger.CreateSimpleLogger(level)
		if LOGGER == nil {
			return fmt.Errorf("创建日志器失败")
		}
		LOGGER.Info("📝 日志器初始化完成: level=%s, debug=%t\n", level.String(), GATEWAY.Debug)
	}
	
	return nil
}

// initializeSnowflakeNode 初始化Snowflake节点用于分布式ID生成
func initializeSnowflakeNode() error {
	// 使用节点ID 1（可以从配置中读取）
	var err error
	Node, err = snowflake.NewNode(1)
	if err != nil {
		return fmt.Errorf("创建Snowflake节点失败: %w", err)
	}
	LOGGER.Info("❄️  Snowflake节点初始化完成\n")
	return nil
}

// initializePoolManager 初始化连接池管理器及其所有资源
func initializePoolManager() error {
	if GATEWAY == nil {
		return fmt.Errorf("GATEWAY 配置为空")
	}
	
	if LOGGER == nil {
		return fmt.Errorf("LOGGER 未初始化")
	}
	
	// 创建连接池管理器（注入 logger）
	manager := cpool.NewManager(LOGGER)
	
	// 初始化 Manager（这会初始化所有连接池）
	if err := manager.Initialize(CTX, GATEWAY); err != nil {
		return fmt.Errorf("初始化 Pool Manager 失败: %w", err)
	}
	
	// 将 Manager 的资源绑定到全局变量
	if db := manager.GetDB(); db != nil {
		DB = db
	}
	if rdb := manager.GetRedis(); rdb != nil {
		REDIS = rdb
	}
	if minio := manager.GetMinIO(); minio != nil {
		MinIO = minio
	}
	if node := manager.GetSnowflake(); node != nil {
		Node = node
	}
	
	POOL_MANAGER = manager
	LOGGER.Info("✅ 连接池管理器初始化完成\n")
	
	return nil
}

// bindPoolResourcesToGlobal 从连接池管理器绑定资源到全局变量
func bindPoolResourcesToGlobal() error {
	if POOL_MANAGER == nil {
		return fmt.Errorf("连接池管理器未初始化")
	}
	
	// 资源已在 initializePoolManager 中直接绑定到全局变量
	// 这里只需确保它们是否已绑定
	if DB == nil {
		DB = POOL_MANAGER.GetDB()
	}
	if REDIS == nil {
		REDIS = POOL_MANAGER.GetRedis()
	}
	if MinIO == nil {
		MinIO = POOL_MANAGER.GetMinIO()
	}
	
	return nil
}

// MustInitializeGatewayWithConfigPath 必须成功初始化，失败时 panic
func MustInitializeGatewayWithConfigPath(configPath string) {
	if err := InitializeGatewayWithConfigPath(configPath); err != nil {
		panic(fmt.Sprintf("初始化 Gateway 失败: %v", err))
	}
}

// MustInitializeGatewayWithAutoDiscovery 必须成功初始化，失败时 panic
func MustInitializeGatewayWithAutoDiscovery(searchPath string) {
	if err := InitializeGatewayWithAutoDiscovery(searchPath); err != nil {
		panic(fmt.Sprintf("初始化 Gateway 失败: %v", err))
	}
}

// CleanupGlobal 清理全局资源
func CleanupGlobal() {
	LOGGER.Info("🧹 开始清理全局资源\n")
	
	if CANCEL != nil {
		CANCEL()
	}
	
	// 关闭连接池管理器
	if POOL_MANAGER != nil {
		if err := POOL_MANAGER.Close(); err != nil {
			LOGGER.Info("❌ 关闭连接池管理器失败: %v\n", err)
		} else {
			LOGGER.Info("✅ 连接池管理器已关闭\n")
		}
	}
	
	// 停止配置管理器
	if CONFIG_MANAGER != nil {
		if err := CONFIG_MANAGER.Stop(); err != nil {
			LOGGER.Info("❌ 停止配置管理器失败: %v\n", err)
		} else {
			LOGGER.Info("✅ 配置管理器已停止\n")
		}
	}
	
	// 清理全局变量
	GATEWAY = nil
	CONFIG_MANAGER = nil
	POOL_MANAGER = nil
	LOGGER = nil
	REDIS = nil
	DB = nil
	MinIO = nil
	Node = nil
	CTX = nil
	CANCEL = nil
	
	LOGGER.Info("✅ 全局资源清理完成\n")
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

// GetI18nManager 获取国际化管理器（通过 PoolManager 获取）
func GetI18nManager() interface{} {
	if POOL_MANAGER == nil {
		return nil
	}
	return POOL_MANAGER.GetI18n()
}

// GetTranslate 获取国际化管理器 (暂不支持)
// func GetTranslate() *locales.LanguageManager {
// 	return TRANSLATE
// }

// GetContext 获取全局上下文
func GetContext() context.Context {
	return CTX
}

// GetConfigManager 获取配置管理器
func GetConfigManager() *goconfig.IntegratedConfigManager {
	return CONFIG_MANAGER
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
	
	if err := CONFIG_MANAGER.ReloadConfig(ctx); err != nil {
		return fmt.Errorf("重新加载配置失败: %w", err)
	}
	
	LOGGER.Info("🔄 配置重新加载成功\n")
	return nil
}

// GetEnvironment 获取当前环境
func GetEnvironment() goconfig.EnvironmentType {
	if CONFIG_MANAGER != nil {
		return CONFIG_MANAGER.GetEnvironment()
	}
	return goconfig.GetEnvironment()
}

// SetEnvironment 设置环境
func SetEnvironment(env goconfig.EnvironmentType) error {
	if CONFIG_MANAGER != nil {
		return CONFIG_MANAGER.SetEnvironment(env)
	}
	return fmt.Errorf("配置管理器未初始化")
}

// GetConfigMetadata 获取配置元数据
func GetConfigMetadata() map[string]interface{} {
	if CONFIG_MANAGER != nil {
		return CONFIG_MANAGER.GetConfigMetadata()
	}
	return make(map[string]interface{})
}
