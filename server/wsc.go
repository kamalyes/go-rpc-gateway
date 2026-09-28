/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-16 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-22 15:27:05
 * @FilePath: \go-rpc-gateway\server\wsc.go
 * @Description: WebSocket 集成层 - go-wsc 的薄封装
 * 职责：
 * 1. HTTP 服务器生命周期管理
 * 2. 应用层配置和依赖注入（SPI 存储装配 / PubSub / 连接 Token 鉴权器）
 * 3. 直接暴露 go-wsc Hub 的对外 API
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package server

import (
	"context"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/kamalyes/go-cachex"
	wscconfig "github.com/kamalyes/go-config/pkg/wsc"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/kamalyes/go-toolbox/pkg/netx"
	"github.com/kamalyes/go-wsc"
	gormadapter "github.com/kamalyes/go-wsc/adapter/gorm"
	redisadapter "github.com/kamalyes/go-wsc/adapter/redis"
	wschub "github.com/kamalyes/go-wsc/hub"
	"github.com/kamalyes/go-wsc/messaging"
	"github.com/kamalyes/go-wsc/models"
	"github.com/kamalyes/go-wsc/spi"
	"github.com/kamalyes/go-wsc/transport"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// ============================================================================
// WebSocketService 结构体
// ============================================================================

// WebSocketService WebSocket 服务 - go-wsc Hub 的薄封装
// 只负责：HTTP 服务器管理、应用层配置、SPI 装配编排
// 所有 WebSocket 功能直接使用 go-wsc Hub
type WebSocketService struct {
	hub            *wsc.Hub                   // go-wsc Hub 实例（直接暴露）
	upgrader       *transport.Upgrader        // WebSocket 升级器（传输域组件，实现 /ws HTTP handler）
	config         *wscconfig.WSC             // 配置
	httpServer     *http.Server               // HTTP 服务器
	tracingManager *middleware.TracingManager // 链路追踪管理器（可选）
	ctx            context.Context
	cancel         context.CancelFunc
	running        atomic.Bool
}

// ============================================================================
// 初始化
// ============================================================================

// NewWebSocketService 创建 WebSocket 服务
// 仅初始化配置、Hub 与存储装配，不启动 HTTP 服务器
func NewWebSocketService(cfg *wscconfig.WSC, tracingManager *middleware.TracingManager) (*WebSocketService, error) {
	// 1. 直接使用传入的配置创建 Hub
	hub := wsc.NewHub(cfg)
	if hub == nil {
		return nil, errors.NewError(errors.ErrCodeInternalServerError, "failed to create WebSocket Hub")
	}

	// 2. 验证基础设施依赖
	redisClient := global.GetRedis()
	if redisClient == nil {
		global.LOGGER.WarnMsg("⚠️  Redis 客户端未初始化,Hub 在线状态/统计/队列功能将受限")
		global.LOGGER.WarnMsg("⚠️  警告: 这将导致客户端连接时可能出现空指针错误!")
		os.Exit(1)
	}

	db := global.GetDB()
	if db == nil {
		global.LOGGER.ErrorMsg("❌ MySQL 数据库未初始化")
		return nil, errors.NewError(errors.ErrCodeInternalServerError, "MySQL database not initialized")
	}

	// 3. SPI 存储装配：Redis 侧（在线状态/统计/群组/负载）+ GORM 侧（消息记录/连接记录/连接质量）
	// 单后端部署时对应 hook 传 nil 即可；Redis 连通性由 spi.Initialize 预检（3s 超时）
	initCtx, initCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer initCancel()
	if err := spi.Initialize(initCtx, hub, redisClient, db,
		redisadapter.NewHooks(cfg.RedisRepository),
		gormadapter.NewHooks(cfg.Database),
	); err != nil {
		global.LOGGER.WithError(err).ErrorMsg("❌ SPI 存储装配失败")
		return nil, err
	}

	// 4. 离线消息处理器（Redis 队列 + RDBMS 双写的混合处理器）
	// 两半依赖分属不同适配器，由 messaging.InitializeOfflineQueue 统一组装注入
	if cfg.RedisRepository != nil && cfg.RedisRepository.OfflineMessage != nil {
		offlineCfg := cfg.RedisRepository.OfflineMessage
		if err := messaging.InitializeOfflineQueue(hub, messaging.OfflineDeps{
			Queue:  redisadapter.NewOfflineQueue(redisClient, offlineCfg.KeyPrefix, offlineCfg.QueueTTL),
			Store:  gormadapter.NewOfflineStoreFor(db, offlineCfg),
			Config: offlineCfg,
		}); err != nil {
			global.LOGGER.WithError(err).ErrorMsg("❌ 离线消息处理器装配失败")
			return nil, err
		}
	} else {
		global.LOGGER.WarnMsg("⚠️ 未配置 offline-message，离线消息能力未启用（用户离线期间消息不兜底）")
	}

	// 5. 初始化分布式 PubSub（启用跨节点消息路由，须在 Run() 前注入）
	// 注意：pubsub.enabled=false 时必须完全不挂 PubSub——否则 hub 会以空 namespace
	// 订阅裸频道（如 wsc_notify:node:<id>），与其他实例的带命名空间频道
	// （wsc_notify:pubsub:wsc_notify:node:<id>）完全错配，跨节点消息静默丢失
	// 节点间 gRPC 由 hub.Run() 按 node-grpc.enabled 自动初始化（gRPC 直连优先于 PubSub）
	if cfg.RedisRepository != nil && cfg.RedisRepository.PubSub != nil && cfg.RedisRepository.PubSub.GetEnabled() {
		pubsub := initDistributedPubSub(redisClient, cfg)
		hub.WithPubsub(pubsub)
	} else {
		global.LOGGER.WarnMsg("⚠️ 分布式 PubSub 未启用，跨节点消息路由将不可用")
	}

	// 6. 连接 Token 鉴权器（AES-GCM，未启用时 NewTokenAuthenticator 返回 nil）
	// 升级器与编排层共享同一鉴权器实例
	var authenticator spi.ConnectionAuthenticator
	if cfg.Security != nil {
		authenticator = transport.NewTokenAuthenticator(cfg.Security.ConnectionToken, hub.GetLogger())
		if authenticator != nil {
			hub.WithConnectionTokenDecoder(authenticator)
		}
	}

	// 7. WebSocket 升级器（传输域组件；Hub 隐式满足 transport.Registrar 端口）
	upgrader := transport.NewUpgrader(cfg, hub).
		WithLogger(hub.GetLogger()).
		WithNodeID(hub.NodeID()).
		WithHubContext(hub.Context())
	if authenticator != nil {
		upgrader = upgrader.WithAuthenticator(authenticator)
	}

	// 8. 启动 Hub 事件循环
	go hub.Run()

	// 9. 全局注册 Hub 实例
	global.WSCHUB = hub

	// 10. 创建服务实例
	ctx, cancel := context.WithCancel(context.Background())
	service := &WebSocketService{
		hub:            hub,
		upgrader:       upgrader,
		config:         cfg,
		tracingManager: tracingManager,
		ctx:            ctx,
		cancel:         cancel,
	}

	// 11. 使用 Console 展示服务配置
	cgInit := global.LOGGER.NewConsoleGroup()
	cgInit.Group("✅ WebSocket 服务已初始化")
	serviceConfig := map[string]interface{}{
		"节点IP":      cfg.NodeIP,
		"节点端口":      cfg.NodePort,
		"心跳间隔(秒)":   cfg.HeartbeatInterval,
		"消息缓冲区大小":   cfg.MessageBufferSize,
		"启用ACK":     cfg.EnableAck,
		"允许多端登录":    cfg.AllowMultiLogin,
		"每用户最大连接数":  cfg.MaxConnectionsPerUser,
		"启用客服模块":    cfg.EnableAgent,
		"启用观察者模块":   cfg.EnableObserver,
		"启用负载管理":    cfg.EnableWorkload,
		"启用连接Token": cfg.Security != nil && cfg.Security.ConnectionToken.IsEnabled(),
		"分布式PubSub": cfg.RedisRepository != nil && cfg.RedisRepository.PubSub != nil && cfg.RedisRepository.PubSub.GetEnabled(),
		"gRPC节点通信":  cfg.NodeGRPC.IsEnabled(),
	}
	if cfg.NodeGRPC.IsEnabled() {
		serviceConfig["gRPC监听地址"] = cfg.NodeGRPC.GetAddress()
	}
	cgInit.Table(serviceConfig)
	cgInit.GroupEnd()

	return service, nil
}

// initDistributedPubSub 创建分布式 PubSub 实例
// 从 WSC 配置的 redis-repository.pubsub 段构建 cachex.PubSubConfig，用于跨节点消息路由
// 调用方已保证 pubsub.enabled=true，此处直接全量读取配置
func initDistributedPubSub(redisClient redis.UniversalClient, cfg *wscconfig.WSC) *cachex.PubSub {
	pubsubCfg := cachex.DefaultPubSubConfig()
	if cfg.RedisRepository != nil && cfg.RedisRepository.PubSub != nil {
		ps := cfg.RedisRepository.PubSub
		pubsubCfg.Namespace = ps.GetNamespace()
		pubsubCfg.MaxRetries = ps.GetMaxRetries()
		pubsubCfg.RetryDelay = ps.GetRetryDelay()
		pubsubCfg.BufferSize = ps.GetBufferSize()
		pubsubCfg.PingInterval = ps.GetPingInterval()
		pubsubCfg.EnableCompression = ps.GetEnableCompression()
		pubsubCfg.CompressionMinSize = ps.GetCompressionMinSize()
	}
	return cachex.NewPubSub(redisClient,
		cachex.WithPubSubNamespace(pubsubCfg.Namespace),
		cachex.WithPubSubMaxRetries(pubsubCfg.MaxRetries),
		cachex.WithPubSubRetryDelay(pubsubCfg.RetryDelay),
		cachex.WithPubSubBufferSize(pubsubCfg.BufferSize),
		cachex.WithPubSubPingInterval(pubsubCfg.PingInterval),
		cachex.WithPubSubCompression(pubsubCfg.EnableCompression),
		cachex.WithPubSubCompressionMinSize(pubsubCfg.CompressionMinSize),
	)
}

// ============================================================================
// 生命周期管理
// ============================================================================

// Start 启动 WebSocket HTTP 服务器
func (ws *WebSocketService) Start() error {
	if ws.running.Load() {
		return nil
	}

	if !ws.config.Enabled {
		global.LOGGER.InfoMsg("⏭️  WebSocket 服务已禁用，跳过启动")
		return nil
	}

	// 创建 HTTP 路由（使用传输域升级器的 HandleWebSocketUpgrade）
	// 包装链路追踪：从升级请求提取 W3C traceparent，创建连接级 span，
	// 并将 OTel context 注入到 Client.Context，实现 WSC 全链路追踪
	mux := http.NewServeMux()
	upgradeHandler := middleware.WSCTracingWrapper(ws.tracingManager, ws.upgrader.HandleWebSocketUpgrade)
	mux.HandleFunc(ws.config.Path, upgradeHandler)

	ws.httpServer = &http.Server{
		Addr:         netx.JoinHostPort(ws.config.NodeIP, ws.config.NodePort),
		Handler:      mux,
		ReadTimeout:  ws.config.ReadTimeout,
		WriteTimeout: ws.config.WriteTimeout,
		IdleTimeout:  ws.config.IdleTimeout,
	}

	// 启动 HTTP 服务器
	go func() {
		listener, err := net.Listen(ws.config.Network, ws.httpServer.Addr)
		if err != nil {
			global.LOGGER.WithError(err).ErrorKV("❌ WebSocket 监听器创建失败",
				"network", ws.config.Network,
				"address", ws.httpServer.Addr)
			return
		}
		defer listener.Close() // 确保 listener 关闭，防止连接泄漏

		if err := ws.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			global.LOGGER.WithError(err).ErrorMsg("❌ WebSocket HTTP 服务器启动失败")
		}
	}()

	ws.running.Store(true)

	// 使用 Console 展示启动信息
	cgStart := global.LOGGER.NewConsoleGroup()
	cgStart.Group("✅ WebSocket 服务已启动")
	startupInfo := map[string]interface{}{
		"监听地址":        ws.httpServer.Addr,
		"网络类型":        ws.config.Network,
		"WebSocket路径": ws.config.Path,
		"服务状态":        "运行中",
	}
	cgStart.Table(startupInfo)
	cgStart.GroupEnd()

	return nil
}

// Stop 停止 WebSocket 服务
func (ws *WebSocketService) Stop() error {
	if !ws.running.Load() {
		return nil
	}

	ctx := context.Background()
	global.LOGGER.InfoContext(ctx, "🛑 停止 WebSocket 服务...")

	ws.cancel()

	if ws.httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ws.httpServer.Shutdown(shutdownCtx)
	}

	if ws.hub != nil {
		ws.hub.Shutdown()
	}

	ws.running.Store(false)
	global.LOGGER.InfoContext(ctx, "✅ WebSocket 服务已停止")

	return nil
}

// IsRunning 检查服务是否运行中
func (ws *WebSocketService) IsRunning() bool {
	return ws.running.Load()
}

// ============================================================================
// 访问器方法
// ============================================================================

// GetHub 获取底层 go-wsc Hub 实例
// 用于需要 go-wsc 的高级 API 的场景
func (ws *WebSocketService) GetHub() *wsc.Hub {
	return ws.hub
}

// GetConfig 获取 WSC 配置
func (ws *WebSocketService) GetConfig() *wscconfig.WSC {
	return ws.config
}

// SendToUserWithRetry 带重试的发送消息并返回结果
// 返回详细的发送结果，适用于需要同步处理结果的场景（如ACK、批量统计）
func (ws *WebSocketService) SendToUserWithRetry(ctx context.Context, userID string, msg *models.HubMessage) *models.SendResult {
	return ws.hub.SendToUserWithRetry(ctx, userID, msg)
}

// ============================================================================
// 应用层回调方法 - 直接暴露 go-wsc Hub 的回调
// ============================================================================

// OnClientConnect 注册客户端连接回调
// 在客户端成功建立连接时调用
// 链路追踪已自动注入：若 TracingManager 可用，会在回调执行前自动创建 wsc.client.connect span
//
// 参数:
//   - callback: 客户端连接回调函数，接收 ctx, client, record 参数
//
// 示例:
//
//	ws.OnClientConnect(func(ctx context.Context, client *models.Client, record *models.ConnectionRecord) error {
//	    log.Printf("客户端连接: %s", client.ID)
//	    return nil
//	})
func (ws *WebSocketService) OnClientConnect(callback wschub.ClientConnectCallback) {
	if ws.tracingManager != nil && ws.tracingManager.IsEnabled() {
		ws.hub.SetClientConnectCallback(func(ctx context.Context, client *models.Client, record *models.ConnectionRecord) error {
			_, span := middleware.WSCStartSpan(ws.tracingManager, client.Context, "wsc.client.connect",
				attribute.String("wsc.client.id", client.ID),
				attribute.String("wsc.user.id", client.UserID),
			)
			defer span.End()

			err := callback(ctx, client, record)
			if err != nil {
				span.RecordError(err)
			}
			return err
		})
	} else {
		ws.hub.SetClientConnectCallback(callback)
	}
}

// OnClientDisconnect 注册客户端断开连接回调
// 在客户端断开连接时调用
// 链路追踪已自动注入：若 TracingManager 可用，会在回调执行前自动创建 wsc.client.disconnect span
//
// 参数:
//   - callback: 客户端断开回调函数，接收 ctx, client, reason 参数
//
// 示例:
//
//	ws.OnClientDisconnect(func(ctx context.Context, client *models.Client, reason models.DisconnectReason) error {
//	    log.Printf("客户端断开: %s, 原因: %s", client.ID, reason)
//	    return nil
//	})
func (ws *WebSocketService) OnClientDisconnect(callback wschub.ClientDisconnectCallback) {
	if ws.tracingManager != nil && ws.tracingManager.IsEnabled() {
		ws.hub.SetClientDisconnectCallback(func(ctx context.Context, client *models.Client, reason models.DisconnectReason) error {
			_, span := middleware.WSCStartSpan(ws.tracingManager, client.Context, "wsc.client.disconnect",
				attribute.String("wsc.client.id", client.ID),
				attribute.String("wsc.user.id", client.UserID),
				attribute.String("wsc.disconnect.reason", string(reason)),
			)
			defer span.End()

			err := callback(ctx, client, reason)
			if err != nil {
				span.RecordError(err)
			}
			return err
		})
	} else {
		ws.hub.SetClientDisconnectCallback(callback)
	}
}

// OnMessageReceived 注册消息接收回调
// 在接收到客户端消息时调用
// 链路追踪已自动注入：若 TracingManager 可用，会在回调执行前自动创建 wsc.message.receive span
//
// 参数:
//   - callback: 消息接收回调函数，接收 ctx, client, msg 参数
//
// 示例:
//
//	ws.OnMessageReceived(func(ctx context.Context, client *models.Client, msg *models.HubMessage) error {
//	    log.Printf("收到消息: %s", msg.ID)
//	    return nil
//	})
func (ws *WebSocketService) OnMessageReceived(callback messaging.MessageReceivedCallback) {
	if ws.tracingManager != nil && ws.tracingManager.IsEnabled() {
		ws.hub.SetMessageReceivedCallback(func(ctx context.Context, client *models.Client, msg *models.HubMessage) error {
			_, span := middleware.WSCStartSpan(ws.tracingManager, client.Context, "wsc.message.receive",
				attribute.String(constants.TracingAttrRPCMethod, "wsc.message.receive"),
				attribute.String("wsc.client.id", client.ID),
				attribute.String("wsc.user.id", client.UserID),
				attribute.String("wsc.message.id", msg.ID),
				attribute.String("wsc.message.type", string(msg.MessageType)),
			)
			defer span.End()

			err := callback(ctx, client, msg)
			if err != nil {
				span.RecordError(err)
			}
			return err
		})
	} else {
		ws.hub.SetMessageReceivedCallback(callback)
	}
}

// OnError 注册错误处理回调
// 在发生错误时调用
// 链路追踪已自动注入：若 TracingManager 可用，会在回调执行前自动创建 wsc.error span 并记录错误
//
// 参数:
//   - callback: 错误处理回调函数，接收 ctx, err, severity 参数
//
// 示例:
//
//	ws.OnError(func(ctx context.Context, err error, severity models.ErrorSeverity) error {
//	    log.Printf("错误: %v, 严重程度: %s", err, severity)
//	    return nil
//	})
func (ws *WebSocketService) OnError(callback messaging.ErrorCallback) {
	if ws.tracingManager != nil && ws.tracingManager.IsEnabled() {
		ws.hub.SetErrorCallback(func(ctx context.Context, err error, severity models.ErrorSeverity) error {
			_, span := middleware.WSCStartSpan(ws.tracingManager, ctx, "wsc.error",
				attribute.String("wsc.error.severity", string(severity)),
			)
			defer span.End()

			if err != nil {
				span.RecordError(err)
			}

			callbackErr := callback(ctx, err, severity)
			if callbackErr != nil {
				span.RecordError(callbackErr)
			}
			return callbackErr
		})
	} else {
		ws.hub.SetErrorCallback(callback)
	}
}

// ============================================================================
// Hub 级别回调方法 - 直接暴露 go-wsc Hub 的回调
// ============================================================================

// OnHeartbeatTimeout 注册心跳超时回调函数
// 当客户端心跳超时时会调用此回调
// 链路追踪已自动注入：若 TracingManager 可用，会在回调执行前自动创建 wsc.heartbeat.timeout span
//
// 参数:
//   - callback: 心跳超时回调函数，接收 clientID, userID, lastHeartbeat 参数
//
// 示例:
//
//	ws.OnHeartbeatTimeout(func(clientID, userID string, lastHeartbeat time.Time) {
//	    log.Printf("客户端 %s 心跳超时", clientID)
//	    // 更新数据库、清理缓存等
//	})
func (ws *WebSocketService) OnHeartbeatTimeout(callback wschub.HeartbeatTimeoutCallback) {
	if ws.tracingManager != nil && ws.tracingManager.IsEnabled() {
		ws.hub.SetHeartbeatTimeoutCallback(func(clientID, userID string, lastHeartbeat time.Time) {
			_, span := ws.tracingManager.GetTracer().Start(context.Background(), "wsc.heartbeat.timeout",
				oteltrace.WithAttributes(
					attribute.String("wsc.client.id", clientID),
					attribute.String("wsc.user.id", userID),
					attribute.String("wsc.heartbeat.last", lastHeartbeat.Format(time.RFC3339)),
				),
			)
			defer span.End()
			callback(clientID, userID, lastHeartbeat)
		})
	} else {
		ws.hub.SetHeartbeatTimeoutCallback(callback)
	}
}

// OnHeartbeatReport 注册心跳上报回调函数
// 当收到客户端心跳消息时会调用此回调
//
// 参数:
//   - callback: 心跳上报回调函数，接收 client 参数
//
// 示例:
//
//	ws.OnHeartbeatReport(func(client *models.Client) {
//	    log.Printf("收到客户端 %s 心跳上报", client.ID)
//	    // 更新业务层在线状态、记录心跳日志等
//	})
func (ws *WebSocketService) OnHeartbeatReport(callback wschub.HeartbeatReportCallback) {
	ws.hub.SetHeartbeatReportCallback(callback)
}

// OnBeforeHeartbeat 注册心跳处理前回调函数
// 在心跳处理前调用，返回 false 则跳过后续心跳处理流程
//
// 参数:
//   - callback: 心跳处理前回调函数，接收 client 参数，返回 bool
//
// 返回:
//   - bool: true 继续处理心跳，false 跳过
//
// 示例:
//
//	ws.OnBeforeHeartbeat(func(client *models.Client) bool {
//	    // 校验或预处理
//	    return true
//	})
func (ws *WebSocketService) OnBeforeHeartbeat(callback wschub.BeforeHeartbeatCallback) {
	ws.hub.SetBeforeHeartbeatCallback(callback)
}

// OnAfterHeartbeat 注册心跳处理后回调函数
// 在心跳处理完成后调用
//
// 参数:
//   - callback: 心跳处理后回调函数，接收 client 参数
//
// 示例:
//
//	ws.OnAfterHeartbeat(func(client *models.Client) {
//	    // 心跳处理完成后逻辑
//	})
func (ws *WebSocketService) OnAfterHeartbeat(callback wschub.AfterHeartbeatCallback) {
	ws.hub.SetAfterHeartbeatCallback(callback)
}

// OnOfflineMessagePush 注册离线消息推送回调函数
// 当离线消息推送完成时会调用此回调，由上游决定是否删除消息
//
// 参数:
//   - callback: 离线消息推送回调函数，接收 userID, pushedMessageIDs, failedMessageIDs 参数
//
// 示例:
//
//	ws.OnOfflineMessagePush(func(userID string, pushedMessageIDs, failedMessageIDs []string) {
//	    log.Printf("用户 %s 推送完成，成功: %d, 失败: %d", userID, len(pushedMessageIDs), len(failedMessageIDs))
//	})
func (ws *WebSocketService) OnOfflineMessagePush(callback wschub.OfflineMessagePushCallback) {
	ws.hub.SetOfflineMessagePushCallback(callback)
}

// OnMessageSend 注册消息发送完成回调函数
// 当消息发送完成（无论成功还是失败）时会调用此回调
//
// 参数:
//   - callback: 消息发送回调函数，接收 msg 和 result 参数
//
// 示例:
//
//	ws.OnMessageSend(func(msg *models.HubMessage, result *models.SendResult) {
//	    if result.FinalError != nil {
//	        log.Printf("消息发送失败: %s, 错误: %v", msg.ID, result.FinalError)
//	    } else {
//	        log.Printf("消息发送成功: %s, 重试次数: %d", msg.ID, result.TotalRetries)
//	    }
//	})
func (ws *WebSocketService) OnMessageSend(callback messaging.MessageSendCallback) {
	ws.hub.SetMessageSendCallback(callback)
}

// initWebSocket 初始化 WebSocket 服务
func (s *Server) initWebSocket() error {
	// 检查 WebSocket 是否启用
	if !s.config.WSC.Enabled {
		global.LOGGER.DebugMsg("WebSocket 服务未启用，跳过初始化")
		return nil
	}

	// 获取链路追踪管理器（可能为 nil，WSC 内部会优雅处理）
	var tracingManager *middleware.TracingManager
	if s.middlewareManager != nil {
		tracingManager = s.middlewareManager.GetTracingManager()
	}

	// 创建 WebSocket 服务
	wsSvc, err := NewWebSocketService(s.config.WSC, tracingManager)
	if err != nil {
		return errors.NewErrorf(errors.ErrCodeInternalServerError, "failed to create WebSocket service: %v", err)
	}

	s.webSocketService = wsSvc
	return nil
}
