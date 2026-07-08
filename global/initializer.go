/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-15 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 00:00:00
 * @FilePath: \go-rpc-gateway\global\initializer.go
 * @Description: 统一初始化器 - 消除分散的初始化逻辑
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package global

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/bwmarrin/snowflake"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/cpool"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/osx"
)

// Initializer 初始化器接口 - 统一初始化流程
type Initializer interface {
	// Name 初始化器名称
	Name() string

	// Priority 优先级（数字越小优先级越高）
	Priority() int

	// Initialize 初始化组件
	Initialize(ctx context.Context, cfg *gwconfig.Gateway) error

	// Cleanup 清理资源
	Cleanup() error

	// HealthCheck 健康检查
	HealthCheck() error
}

// InitializerTimeout 可选接口，允许初始化器自定义初始化超时时间
// 未实现此接口的初始化器将使用 defaultInitTimeout
// 用于避免单个初始化器因网络问题（如数据库拒绝连接但拨号不返回）导致整个初始化链卡死
type InitializerTimeout interface {
	InitTimeout() time.Duration
}

// defaultInitTimeout 单个初始化器的默认最大超时时间
const defaultInitTimeout = 15 * time.Second

// getInitTimeout 获取初始化器的超时时间，未实现 InitializerTimeout 接口则使用默认值
func getInitTimeout(init Initializer) time.Duration {
	if t, ok := init.(InitializerTimeout); ok {
		if timeout := t.InitTimeout(); timeout > 0 {
			return timeout
		}
	}
	return defaultInitTimeout
}

// InitializerChain 初始化器链 - 管理所有初始化器
type InitializerChain struct {
	initializers []Initializer
	initialized  map[string]bool
	mu           sync.RWMutex
}

// NewInitializerChain 创建初始化器链
func NewInitializerChain() *InitializerChain {
	return &InitializerChain{
		initializers: make([]Initializer, 0),
		initialized:  make(map[string]bool),
	}
}

// Register 注册初始化器
func (c *InitializerChain) Register(init Initializer) *InitializerChain {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.initializers = append(c.initializers, init)

	// 按优先级排序
	sort.Slice(c.initializers, func(i, j int) bool {
		return c.initializers[i].Priority() < c.initializers[j].Priority()
	})

	return c
}

// InitializeAll 按顺序初始化所有组件
func (c *InitializerChain) InitializeAll(ctx context.Context, cfg *gwconfig.Gateway) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 使用 Console Group 展示初始化流程
	var cg *logger.ConsoleGroup
	if LOGGER != nil {
		cg = LOGGER.NewConsoleGroup()
		cg.Group("🔧 系统组件初始化")
		initTimer := cg.Time("总初始化耗时")
		defer initTimer.End()
	}

	for _, init := range c.initializers {
		name := init.Name()

		if cg != nil {
			cg.Info("→ 正在初始化: %s", name)
		} else {
			fmt.Printf("🔧 初始化 %s...\n", name)
		}

		// 为单个初始化器设置最大超时时间，避免网络问题（如目标机器拒绝连接但拨号不返回）导致整个初始化链卡死
		// 注意：超时后 Initialize 可能在后台 goroutine 中继续执行，但 InitializeAll 会立即返回错误
		timeout := getInitTimeout(init)
		initCtx, cancel := context.WithTimeout(ctx, timeout)

		done := make(chan error, 1)
		go func() {
			done <- init.Initialize(initCtx, cfg)
		}()

		var err error
		select {
		case err = <-done:
			// 初始化完成（成功或失败）
		case <-initCtx.Done():
			if initCtx.Err() == context.DeadlineExceeded {
				err = fmt.Errorf("初始化 %s 超时（耗时超过 %s）", name, timeout)
			} else {
				err = fmt.Errorf("初始化 %s 被取消: %w", name, initCtx.Err())
			}
		}
		cancel()

		if err != nil {
			if cg != nil {
				cg.GroupEnd()
			}
			return fmt.Errorf("初始化 %s 失败: %w", name, err)
		}

		c.initialized[name] = true

		if cg != nil {
			cg.Info("✅ %s 初始化完成", name)
		} else {
			fmt.Printf("✅ %s 初始化完成\n", name)
		}
	}

	if cg != nil {
		// 展示初始化摘要
		summary := map[string]interface{}{
			"已初始化组件": len(c.initialized),
			"总组件数":   len(c.initializers),
			"初始化状态":  "✅ 全部成功",
		}
		cg.Table(summary)
		cg.GroupEnd()
	}

	return nil
}

// CleanupAll 清理所有组件（逆序）
func (c *InitializerChain) CleanupAll() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error

	// 逆序清理
	ctx := context.Background()
	for i := len(c.initializers) - 1; i >= 0; i-- {
		init := c.initializers[i]
		name := init.Name()

		if !c.initialized[name] {
			continue
		}

		if LOGGER != nil {
			LOGGER.InfoContext(ctx, "🧹 清理 %s...", name)
		}

		if err := init.Cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("清理 %s 失败: %w", name, err))
		} else {
			if LOGGER != nil {
				LOGGER.InfoContext(ctx, "✅ %s 清理完成", name)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("清理过程中发生错误: %v", errs)
	}

	return nil
}

// HealthCheckAll 检查所有组件健康状态
func (c *InitializerChain) HealthCheckAll() map[string]error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make(map[string]error)

	for _, init := range c.initializers {
		name := init.Name()
		if c.initialized[name] {
			results[name] = init.HealthCheck()
		}
	}

	return results
}

// ==================== 具体初始化器实现 ====================

// LoggerInitializer 日志器初始化器
type LoggerInitializer struct{}

func (i *LoggerInitializer) Name() string       { return "Logger" }
func (i *LoggerInitializer) Priority() int      { return 1 } // 最高优先级
func (i *LoggerInitializer) HealthCheck() error { return nil }

// Initialize 初始化日志器
// 注意：此方法会根据配置重新创建 logger，替换掉临时 logger
func (i *LoggerInitializer) Initialize(ctx context.Context, cfg *gwconfig.Gateway) error {
	isFirstInit := LOGGER == nil

	LOGGER = cfg.Middleware.Logging.ToLoggerInstance()
	LOG = LOGGER // 兼容别名

	// 根据初始化状态输出不同日志
	msg := mathx.IF(isFirstInit, "🔄 Logger reconfigured with settings from config file", "ℹ️ [INFO] Logger initialized successfully with go-logger")
	LOGGER.InfoContext(ctx, msg)
	return nil
}

func (i *LoggerInitializer) Cleanup() error {
	// 日志器不需要特别清理
	return nil
}

// SnowflakeInitializer Snowflake ID生成器初始化器
type SnowflakeInitializer struct{}

func (i *SnowflakeInitializer) Name() string  { return "Snowflake" }
func (i *SnowflakeInitializer) Priority() int { return 5 }

func (i *SnowflakeInitializer) Initialize(ctx context.Context, cfg *gwconfig.Gateway) error {
	// 使用默认节点ID 1
	// TODO: 未来可以从环境变量或配置中读取节点ID
	nodeID := int64(1)

	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		return fmt.Errorf("创建Snowflake节点失败: %w", err)
	}

	Node = node
	return nil
}

func (i *SnowflakeInitializer) Cleanup() error {
	Node = nil
	return nil
}

func (i *SnowflakeInitializer) HealthCheck() error {
	if Node == nil {
		return fmt.Errorf("Snowflake节点未初始化")
	}
	// 尝试生成一个ID
	_ = Node.Generate()
	return nil
}

// PoolManagerInitializer 连接池管理器初始化器
type PoolManagerInitializer struct{}

func (i *PoolManagerInitializer) Name() string  { return "PoolManager" }
func (i *PoolManagerInitializer) Priority() int { return 10 }

func (i *PoolManagerInitializer) Initialize(ctx context.Context, cfg *gwconfig.Gateway) error {
	if LOGGER == nil {
		return fmt.Errorf("LOGGER 未初始化")
	}

	// 创建连接池管理器
	manager := cpool.NewManager(LOGGER)

	// 初始化 Manager（这会初始化所有连接池）
	if err := manager.Initialize(ctx, cfg); err != nil {
		return fmt.Errorf("初始化 Pool Manager 失败: %w", err)
	}

	POOL_MANAGER = manager

	// 将 Manager 的资源绑定到全局便捷引用变量
	// ClickHouse 和 NATS 不再存储为独立全局变量，通过 GetClickHouse()/GetNats() 从 PoolManager 获取
	DB = manager.GetDB()
	REDIS = manager.GetRedis()
	MinIO = manager.GetMinIO()

	return nil
}

func (i *PoolManagerInitializer) Cleanup() error {
	if POOL_MANAGER != nil {
		return POOL_MANAGER.Close()
	}
	return nil
}

func (i *PoolManagerInitializer) HealthCheck() error {
	if POOL_MANAGER == nil {
		return fmt.Errorf("连接池管理器未初始化")
	}

	status := POOL_MANAGER.HealthCheck()

	// 检查是否有失败的组件
	for name, healthy := range status {
		if !healthy {
			return fmt.Errorf("组件 %s 健康检查失败", name)
		}
	}

	return nil
}

// ContextInitializer 全局上下文初始化器
type ContextInitializer struct{}

func (i *ContextInitializer) Name() string       { return "Context" }
func (i *ContextInitializer) Priority() int      { return 2 }
func (i *ContextInitializer) HealthCheck() error { return nil }

func (i *ContextInitializer) Initialize(ctx context.Context, cfg *gwconfig.Gateway) error {
	// 初始化全局上下文
	CTX, CANCEL = context.WithCancel(context.Background())
	return nil
}

func (i *ContextInitializer) Cleanup() error {
	if CANCEL != nil {
		CANCEL()
	}
	return nil
}

// ServerNodeInitializer 服务节点标识初始化器
// 在 K8s 环境下用于识别处理请求的具体 Pod，便于分布式环境下的流量定位与问题排查
// 节点标识获取逻辑封装在 osx.GetServerNode() 中（与 osx.GetWorkerId 同源）
type ServerNodeInitializer struct{}

func (i *ServerNodeInitializer) Name() string       { return "ServerNode" }
func (i *ServerNodeInitializer) Priority() int      { return 3 } // 在 Context 之后、Snowflake 之前
func (i *ServerNodeInitializer) HealthCheck() error { return nil }

func (i *ServerNodeInitializer) Initialize(ctx context.Context, cfg *gwconfig.Gateway) error {
	// 通过 osx.GetServerNode 获取节点标识
	// 优先级: POD_NAME > HOSTNAME > os.Hostname() > 随机生成的标识
	SERVER_NODE = osx.GetServerNode()
	LOGGER.InfoContext(ctx, "🖥️ 服务节点标识已初始化: %s", SERVER_NODE)
	return nil
}

func (i *ServerNodeInitializer) Cleanup() error {
	SERVER_NODE = ""
	return nil
}

// ==================== 辅助函数 ====================

// GetDefaultInitializerChain 获取默认初始化器链
func GetDefaultInitializerChain() *InitializerChain {
	chain := NewInitializerChain()

	// 注册所有默认初始化器（按优先级自动排序）
	chain.Register(&LoggerInitializer{})
	chain.Register(&ContextInitializer{})
	chain.Register(&ServerNodeInitializer{})
	chain.Register(&SnowflakeInitializer{})
	chain.Register(&PoolManagerInitializer{})

	return chain
}

// InitializeWithDefaults 使用默认初始化器链初始化
func InitializeWithDefaults(ctx context.Context, cfg *gwconfig.Gateway) error {
	chain := GetDefaultInitializerChain()
	return chain.InitializeAll(ctx, cfg)
}

// CleanupWithDefaults 使用默认初始化器链清理
func CleanupWithDefaults() error {
	chain := GetDefaultInitializerChain()
	return chain.CleanupAll()
}
