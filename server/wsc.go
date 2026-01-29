/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-16 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-22 15:27:05
 * @FilePath: \go-rpc-gateway\server\wsc.go
 * @Description: WebSocket 集成层 - go-wsc 的薄封装
 * 职责：
 * 1. HTTP 升级处理
 * 2. 配置初始化
 * 3. 生命周期管理
 * 4. 直接暴露 go-wsc Hub 的所有 API
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	wscconfig "github.com/kamalyes/go-config/pkg/wsc"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/metadata"
	"github.com/kamalyes/go-wsc"
)

// ============================================================================
// WebSocketService 结构体
// ============================================================================

// WebSocketService WebSocket 服务 - go-wsc Hub 的薄封装
// 只负责：HTTP 升级、配置管理、生命周期
// 所有 WebSocket 功能直接使用 go-wsc Hub
type WebSocketService struct {
	hub        *wsc.Hub       // go-wsc Hub 实例（直接暴露）
	config     *wscconfig.WSC // 配置
	httpServer *http.Server   // HTTP 服务器
	ctx        context.Context
	cancel     context.CancelFunc
	running    atomic.Bool
}

// ============================================================================
// 初始化
// ============================================================================

// NewWebSocketService 创建 WebSocket 服务
// 仅初始化配置和 Hub，不启动 HTTP 服务器
func NewWebSocketService(cfg *wscconfig.WSC) (*WebSocketService, error) {
	// 直接使用传入的配置创建 Hub
	hub := wsc.NewHub(cfg)
	if hub == nil {
		return nil, errors.NewError(errors.ErrCodeInternalServerError, "failed to create WebSocket Hub")
	}

	// 🔥 关键修复:初始化 Hub 的所有内部仓库(避免空指针)
	redisClient := global.GetRedis()
	if redisClient == nil {
		global.LOGGER.WarnMsg("⚠️  Redis 客户端未初始化,Hub 在线状态/统计/队列功能将受限")
		global.LOGGER.WarnMsg("⚠️  警告: 这将导致客户端连接时可能出现空指针错误!")
		os.Exit(1)
	}

	// 验证 Redis 连接
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		global.LOGGER.ErrorKV("❌ Redis 连接测试失败,WebSocket 功能将受限",
			"error", err)
		os.Exit(1)
	}

	// 获取 Hub 的 Logger
	hubLogger := hub.GetLogger()

	// 在线状态仓库 (TTL固定为心跳间隔的3倍)
	cfg.RedisRepository.OnlineStatus.TTL = time.Duration(cfg.HeartbeatInterval) * time.Second * 3
	onlineStatusRepo := wsc.NewRedisOnlineStatusRepository(redisClient, cfg.RedisRepository.OnlineStatus)
	hub.SetOnlineStatusRepository(onlineStatusRepo)

	// 统计仓库
	statsRepo := wsc.NewRedisHubStatsRepository(redisClient, cfg.RedisRepository.Stats)
	hub.SetHubStatsRepository(statsRepo)

	// 负载管理仓库
	workloadRepo := wsc.NewRedisWorkloadRepository(redisClient, cfg.RedisRepository.Workload, hubLogger)
	hub.SetWorkloadRepository(workloadRepo)

	// 2. 获取 MySQL/GORM 数据库并初始化 MySQL 仓库
	db := global.GetDB()
	if db == nil {
		global.LOGGER.WarnMsg("⚠️  MySQL 数据库未初始化,Hub 消息记录功能将受限")
		os.Exit(1)
	}

	// 消息记录仓库 (MySQL GORM)
	messageRecordRepo := wsc.NewMessageRecordRepository(db, cfg.Database.MessageRecord, hubLogger)
	hub.SetMessageRecordRepository(messageRecordRepo)

	// 连接记录仓库 (MySQL GORM)
	connectionRecordRepo := wsc.NewConnectionRecordRepository(db, cfg.Database.ConnectionRecord, hubLogger)
	hub.SetConnectionRecordRepository(connectionRecordRepo)

	// 🔥 离线消息处理器
	offlineHandler := wsc.NewHybridOfflineMessageHandler(redisClient, db, cfg.RedisRepository.OfflineMessage, hubLogger)
	hub.SetOfflineMessageHandler(offlineHandler)

	// 使用 Console 展示仓库初始化信息
	cg := global.LOGGER.NewConsoleGroup()
	cg.Group("✅ WebSocket Hub 仓库初始化")

	// Redis 仓库配置
	redisConfig := []map[string]interface{}{
		{
			"仓库类型":   "在线状态",
			"Key前缀":  cfg.RedisRepository.OnlineStatus.KeyPrefix,
			"TTL(秒)": cfg.RedisRepository.OnlineStatus.TTL.Seconds(),
		},
		{
			"仓库类型":    "统计数据",
			"Key前缀":   cfg.RedisRepository.Stats.KeyPrefix,
			"TTL(小时)": cfg.RedisRepository.Stats.TTL.Hours(),
		},
		{
			"仓库类型":  "工作负载",
			"Key前缀": cfg.RedisRepository.Workload.KeyPrefix,
		},
	}
	cg.Table(redisConfig)

	// 离线消息配置
	offlineConfig := map[string]interface{}{
		"Key前缀":     cfg.RedisRepository.OfflineMessage.KeyPrefix,
		"队列TTL(小时)": cfg.RedisRepository.OfflineMessage.QueueTTL.Hours(),
		"自动存储":      cfg.RedisRepository.OfflineMessage.AutoStore,
		"自动推送":      cfg.RedisRepository.OfflineMessage.AutoPush,
		"最大消息数":     cfg.RedisRepository.OfflineMessage.MaxCount,
	}
	cg.Table(offlineConfig)

	cg.Info("✅ MySQL 消息记录仓库已初始化")
	cg.Info("✅ MySQL 连接记录仓库已初始化")
	cg.Info("✅ ShortFlake ID 生成器已初始化 (Hub NodeID: %s, WorkerID: %d)", hub.GetNodeID(), hub.GetWorkerID())
	cg.GroupEnd()

	ctx, cancel = context.WithCancel(context.Background())

	service := &WebSocketService{
		hub:    hub,
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	// 启动 Hub 事件循环（go-wsc 的核心消息处理）
	go hub.Run()

	// 全局注册 Hub 实例
	global.WSCHUB = hub

	// 使用 Console 展示服务配置
	cgInit := global.LOGGER.NewConsoleGroup()
	cgInit.Group("✅ WebSocket 服务已初始化")

	serviceConfig := map[string]interface{}{
		"节点IP":    cfg.NodeIP,
		"节点端口":    cfg.NodePort,
		"心跳间隔(秒)": cfg.HeartbeatInterval,
		"消息缓冲区大小": cfg.MessageBufferSize,
		"启用ACK":   cfg.EnableAck,
	}
	cgInit.Table(serviceConfig)
	cgInit.GroupEnd()

	return service, nil
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

	// 创建 HTTP 路由
	mux := http.NewServeMux()
	mux.HandleFunc(ws.config.Path, ws.handleWebSocketUpgrade)

	ws.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", ws.config.NodeIP, ws.config.NodePort),
		Handler:      mux,
		ReadTimeout:  ws.config.ReadTimeout,
		WriteTimeout: ws.config.WriteTimeout,
		IdleTimeout:  ws.config.IdleTimeout,
	}

	// 从配置中获取网络类型（默认值应该在配置层面处理）
	go func() {
		listener, err := net.Listen(ws.config.Network, ws.httpServer.Addr)
		if err != nil {
			global.LOGGER.WithError(err).ErrorKV("❌ WebSocket 监听器创建失败",
				"network", ws.config.Network,
				"address", ws.httpServer.Addr)
			return
		}
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
// HTTP WebSocket 升级处理
// ============================================================================

// configureUpgrader 配置 WebSocket 升级器
func (ws *WebSocketService) configureUpgrader() *websocket.Upgrader {
	upgrader := wsc.DefaultUpgrader
	upgrader.ReadBufferSize = 1024
	upgrader.WriteBufferSize = 1024

	if ws.config != nil {
		if ws.config.MessageBufferSize > 0 {
			upgrader.ReadBufferSize = int(ws.config.MessageBufferSize)
			upgrader.WriteBufferSize = int(ws.config.MessageBufferSize)
		}

		// 自定义 Origin 检查
		if len(ws.config.WebSocketOrigins) > 0 {
			upgrader.CheckOrigin = ws.createOriginChecker()
		}
	}

	return &upgrader
}

// createOriginChecker 创建 Origin 检查器
func (ws *WebSocketService) createOriginChecker() func(*http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		for _, allowedOrigin := range ws.config.WebSocketOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				return true
			}
		}
		return false
	}
}

// createClient 创建 WebSocket 客户端
func (ws *WebSocketService) createClient(r *http.Request, conn *websocket.Conn) *wsc.Client {
	clientID, userID, userType := ws.extractClientAttributes(r)
	clientUserType := ws.convertUserType(userType)

	// 使用 metadata 提取所有请求元数据
	requestMeta := metadata.ExtractRequestMetadata(r)
	metaMap := requestMeta.ToMap()

	return &wsc.Client{
		ID:         clientID,
		UserID:     userID,
		UserType:   clientUserType,
		ClientIP:   requestMeta.ClientIP, // 从 metadata 提取 ClientIP
		ClientType: wsc.ClientTypeWeb,    // 默认为 Web 类型
		Conn:       conn,
		LastSeen:   time.Now(),
		Status:     wsc.UserStatusOnline,
		SendChan:   make(chan []byte, ws.config.MessageBufferSize),
		Context:    context.WithValue(r.Context(), wsc.ContextKeySenderID, userID),
		Metadata:   metaMap,
	}
}

// convertUserType 转换用户类型字符串为 wsc.UserType
func (ws *WebSocketService) convertUserType(userType string) wsc.UserType {
	switch userType {
	case "customer":
		return wsc.UserTypeCustomer
	case "agent":
		return wsc.UserTypeAgent
	case "admin":
		return wsc.UserTypeAdmin
	case "bot":
		return wsc.UserTypeBot
	case "vip":
		return wsc.UserTypeVIP
	default:
		return wsc.UserTypeCustomer
	}
}

// handleWebSocketUpgrade 处理 WebSocket 升级请求
// 此函数只负责：升级连接 -> 创建客户端 -> 注册到 Hub
// 所有消息处理都由 go-wsc Hub 完成
func (ws *WebSocketService) handleWebSocketUpgrade(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()

	// 提取客户端属性
	clientID, userID, userType := ws.extractClientAttributes(r)

	// 记录 WebSocket 升级请求开始（包含完整的请求信息）
	global.LOGGER.InfoContextKV(ctx, "[WebSocket] 升级请求",
		"method", r.Method,
		"path", r.URL.Path,
		"query", r.URL.RawQuery,
		"client_id", clientID,
		"user_id", userID,
		"user_type", userType,
		"remote_addr", r.RemoteAddr,
		"user_agent", r.Header.Get("User-Agent"),
		"origin", r.Header.Get("Origin"),
		"sec_websocket_key", r.Header.Get("Sec-WebSocket-Key"),
		"sec_websocket_version", r.Header.Get("Sec-WebSocket-Version"),
		"sec_websocket_protocol", r.Header.Get("Sec-WebSocket-Protocol"),
		"connection", r.Header.Get("Connection"),
		"upgrade", r.Header.Get("Upgrade"),
	)

	// 配置并升级 WebSocket 连接
	upgrader := ws.configureUpgrader()
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// 记录升级失败日志
		global.LOGGER.WithError(err).ErrorContextKV(ctx, "[WebSocket] 升级失败",
			"client_id", clientID,
			"user_id", userID,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err.Error(),
			"upgrade_failed", true,
		)
		return
	}

	// 记录升级成功日志（升级后响应已发送，记录连接信息）
	global.LOGGER.InfoContextKV(ctx, "[WebSocket] 升级成功",
		"client_id", clientID,
		"user_id", userID,
		"user_type", userType,
		"status_code", 101, // WebSocket 升级成功状态码固定为 101
		"protocol", conn.Subprotocol(),
		"remote_addr", conn.RemoteAddr().String(),
		"local_addr", conn.LocalAddr().String(),
		"duration_ms", time.Since(start).Milliseconds(),
		"upgrade_success", true,
	)

	// 创建客户端
	client := ws.createClient(r, conn)

	// 注册到 Hub（go-wsc 接管后续所有处理，包括消息读取）
	ws.hub.Register(client)

	// 记录客户端注册成功日志
	global.LOGGER.InfoContextKV(ctx, "[WebSocket] 客户端注册成功",
		"client_id", client.ID,
		"user_id", client.UserID,
		"user_type", string(client.UserType),
	)
}

// extractClientAttributes 从请求中提取客户端属性
// 优先从 URL 查询参数获取，其次从 Header 获取
// 返回: clientID, userID, userType
func (ws *WebSocketService) extractClientAttributes(r *http.Request) (string, string, string) {
	query := r.URL.Query()

	// 获取 Client ID
	clientID := query.Get("client_id")
	if clientID == "" {
		clientID = r.Header.Get("X-Client-ID")
	}
	if clientID == "" {
		clientID = fmt.Sprintf("client_%d", time.Now().UnixNano())
	}

	// 获取 User ID (优先使用查询参数中的 user_id)
	userID := query.Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-User-ID")
	}
	if userID == "" {
		userID = clientID
	}

	// 获取 User Type (从查询参数)
	userType := query.Get("user_type")
	if userType == "" {
		userType = r.Header.Get("X-User-Type")
	}

	return clientID, userID, userType
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
func (ws *WebSocketService) SendToUserWithRetry(ctx context.Context, userID string, msg *wsc.HubMessage) *wsc.SendResult {
	return ws.hub.SendToUserWithRetry(ctx, userID, msg)
}

// ============================================================================
// 应用层回调方法 - 直接暴露 go-wsc Hub 的回调
// ============================================================================

// OnClientConnect 注册客户端连接回调
// 在客户端成功建立连接时调用
//
// 参数:
//   - callback: 客户端连接回调函数，接收 ctx, client 参数
//
// 示例:
//
//	ws.OnClientConnect(func(ctx context.Context, client *wsc.Client) error {
//	    log.Printf("客户端连接: %s", client.ID)
//	    return nil
//	})
func (ws *WebSocketService) OnClientConnect(callback wsc.ClientConnectCallback) {
	ws.hub.OnClientConnect(callback)
}

// OnClientDisconnect 注册客户端断开连接回调
// 在客户端断开连接时调用
//
// 参数:
//   - callback: 客户端断开回调函数，接收 ctx, client, reason 参数
//
// 示例:
//
//	ws.OnClientDisconnect(func(ctx context.Context, client *wsc.Client, reason string) error {
//	    log.Printf("客户端断开: %s, 原因: %s", client.ID, reason)
//	    return nil
//	})
func (ws *WebSocketService) OnClientDisconnect(callback wsc.ClientDisconnectCallback) {
	ws.hub.OnClientDisconnect(callback)
}

// OnMessageReceived 注册消息接收回调
// 在接收到客户端消息时调用
//
// 参数:
//   - callback: 消息接收回调函数，接收 ctx, client, msg 参数
//
// 示例:
//
//	ws.OnMessageReceived(func(ctx context.Context, client *wsc.Client, msg *wsc.HubMessage) error {
//	    log.Printf("收到消息: %s", msg.ID)
//	    return nil
//	})
func (ws *WebSocketService) OnMessageReceived(callback wsc.MessageReceivedCallback) {
	ws.hub.OnMessageReceived(callback)
}

// OnError 注册错误处理回调
// 在发生错误时调用
//
// 参数:
//   - callback: 错误处理回调函数，接收 ctx, err, severity 参数
//
// 示例:
//
//	ws.OnError(func(ctx context.Context, err error, severity string) error {
//	    log.Printf("错误: %v, 严重程度: %s", err, severity)
//	    return nil
//	})
func (ws *WebSocketService) OnError(callback wsc.ErrorCallback) {
	ws.hub.OnError(callback)
}

// ============================================================================
// Hub 级别回调方法 - 直接暴露 go-wsc Hub 的回调
// ============================================================================

// OnHeartbeatTimeout 注册心跳超时回调函数
// 当客户端心跳超时时会调用此回调
//
// 参数:
//   - callback: 心跳超时回调函数，接收 clientID, userID, lastHeartbeat 参数
//
// 示例:
//
//	ws.OnHeartbeatTimeout(func(clientID, userID string, lastHeartbeat time.Time) {
//	    log.Printf("客户端 %s 心跳超时", clientID)
//	    更新数据库、清理缓存等
//	})
func (ws *WebSocketService) OnHeartbeatTimeout(callback wsc.HeartbeatTimeoutCallback) {
	ws.hub.OnHeartbeatTimeout(callback)
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
func (ws *WebSocketService) OnOfflineMessagePush(callback wsc.OfflineMessagePushCallback) {
	ws.hub.OnOfflineMessagePush(callback)
}

// OnMessageSend 注册消息发送完成回调函数
// 当消息发送完成（无论成功还是失败）时会调用此回调
//
// 参数:
//   - callback: 消息发送回调函数，接收 msg 和 result 参数
//
// 示例:
//
//	ws.OnMessageSend(func(msg *wsc.HubMessage, result *wsc.SendResult) {
//	    if result.FinalError != nil {
//	        log.Printf("消息发送失败: %s, 错误: %v", msg.ID, result.FinalError)
//	    } else {
//	        log.Printf("消息发送成功: %s, 重试次数: %d", msg.ID, result.TotalRetries)
//	    }
//	})
func (ws *WebSocketService) OnMessageSend(callback wsc.MessageSendCallback) {
	ws.hub.OnMessageSend(callback)
}

// OnQueueFull 注册队列满回调函数
// 当消息队列满时会调用此回调
//
// 参数:
//   - callback: 队列满回调函数，接收 msg, recipient, queueType, err 参数
//
// 示例:
//
//	ws.OnQueueFull(func(msg *wsc.HubMessage, recipient, queueType string, err *errorx.BaseError) {
//	    log.Printf("队列满: 接收者=%s, 类型=%s", recipient, queueType)
//	})
func (ws *WebSocketService) OnQueueFull(callback wsc.QueueFullCallback) {
	ws.hub.OnQueueFull(callback)
}

// UpdateHeartbeat 更新客户端心跳时间
//
// 参数:
//   - clientID: 客户端ID
//
// 示例:
//
//	ws.UpdateHeartbeat(client.ID)
func (ws *WebSocketService) UpdateHeartbeat(clientID string) {
	ws.hub.UpdateHeartbeat(clientID)
}
