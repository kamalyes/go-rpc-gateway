/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-16 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-22 15:27:05
 * @FilePath: \go-rpc-gateway\server\wsc.go
 * @Description: WebSocket 集成层 - go-wsc 的薄封装
 * 职责：
 * 1. HTTP 服务器生命周期管理
 * 2. 应用层配置和依赖注入
 * 3. 直接暴露 go-wsc Hub 的所有 API
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

	wscconfig "github.com/kamalyes/go-config/pkg/wsc"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-wsc"
)

// ============================================================================
// WebSocketService 结构体
// ============================================================================

// WebSocketService WebSocket 服务 - go-wsc Hub 的薄封装
// 只负责：HTTP 服务器管理、应用层配置
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
	// 1. 直接使用传入的配置创建 Hub
	hub := wsc.NewHub(cfg)
	if hub == nil {
		return nil, errors.NewError(errors.ErrCodeInternalServerError, "failed to create WebSocket Hub")
	}

	// 2. 验证 Redis 连接
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

	// 3. 初始化所有仓库（使用 go-wsc 提供的便捷方法）
	if err := hub.InitializeRepositories(redisClient, db); err != nil {
		global.LOGGER.WithError(err).ErrorMsg("❌ 仓库初始化失败")
		return nil, err
	}

	// 4. 启动 Hub 事件循环
	go hub.Run()

	// 5. 全局注册 Hub 实例
	global.WSCHUB = hub

	// 6. 创建服务实例
	ctx, cancel := context.WithCancel(context.Background())
	service := &WebSocketService{
		hub:    hub,
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	// 7. 使用 Console 展示服务配置
	cgInit := global.LOGGER.NewConsoleGroup()
	cgInit.Group("✅ WebSocket 服务已初始化")
	serviceConfig := map[string]interface{}{
		"节点IP":     cfg.NodeIP,
		"节点端口":     cfg.NodePort,
		"心跳间隔(秒)":  cfg.HeartbeatInterval,
		"消息缓冲区大小":  cfg.MessageBufferSize,
		"启用ACK":    cfg.EnableAck,
		"允许多端登录":   cfg.AllowMultiLogin,
		"每用户最大连接数": cfg.MaxConnectionsPerUser,
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

	// 创建 HTTP 路由（使用 go-wsc Hub 的 HandleWebSocketUpgrade 方法）
	mux := http.NewServeMux()
	mux.HandleFunc(ws.config.Path, ws.hub.HandleWebSocketUpgrade)

	ws.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", ws.config.NodeIP, ws.config.NodePort),
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
//	    // 更新数据库、清理缓存等
//	})
func (ws *WebSocketService) OnHeartbeatTimeout(callback wsc.HeartbeatTimeoutCallback) {
	ws.hub.OnHeartbeatTimeout(callback)
}

// OnHeartbeatReport 注册心跳上报回调函数
// 当收到客户端心跳消息时会调用此回调
//
// 参数:
//   - callback: 心跳上报回调函数，接收 client 参数
//
// 示例:
//
//	ws.OnHeartbeatReport(func(client *wsc.Client) {
//	    log.Printf("收到客户端 %s 心跳上报", client.ID)
//	    // 更新业务层在线状态、记录心跳日志等
//	})
func (ws *WebSocketService) OnHeartbeatReport(callback wsc.HeartbeatReportCallback) {
	ws.hub.OnHeartbeatReport(callback)
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
//	ws.OnBeforeHeartbeat(func(client *wsc.Client) bool {
//	    // 校验或预处理
//	    return true
//	})
func (ws *WebSocketService) OnBeforeHeartbeat(callback wsc.BeforeHeartbeatCallback) {
	ws.hub.OnBeforeHeartbeat(callback)
}

// OnAfterHeartbeat 注册心跳处理后回调函数
// 在心跳处理完成后调用
//
// 参数:
//   - callback: 心跳处理后回调函数，接收 client 参数
//
// 示例:
//
//	ws.OnAfterHeartbeat(func(client *wsc.Client) {
//	    // 心跳处理完成后逻辑
//	})
func (ws *WebSocketService) OnAfterHeartbeat(callback wsc.AfterHeartbeatCallback) {
	ws.hub.OnAfterHeartbeat(callback)
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

// initWebSocket 初始化 WebSocket 服务
func (s *Server) initWebSocket() error {
	// 检查 WebSocket 是否启用
	if !s.config.WSC.Enabled {
		global.LOGGER.DebugMsg("WebSocket 服务未启用，跳过初始化")
		return nil
	}

	// 创建 WebSocket 服务
	wsSvc, err := NewWebSocketService(s.config.WSC)
	if err != nil {
		return errors.NewErrorf(errors.ErrCodeInternalServerError, "failed to create WebSocket service: %v", err)
	}

	s.webSocketService = wsSvc
	return nil
}
