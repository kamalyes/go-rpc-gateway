/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-16 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-01 19:41:17
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
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	wscconfig "github.com/kamalyes/go-config/pkg/wsc"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-wsc"
)

// ============================================================================
// 类型定义
// ============================================================================

// ClientConnectCallback 客户端连接回调
type ClientConnectCallback func(ctx context.Context, client *wsc.Client) error

// ClientDisconnectCallback 客户端断开连接回调
type ClientDisconnectCallback func(ctx context.Context, client *wsc.Client, reason string) error

// MessageReceivedCallback 消息接收回调
type MessageReceivedCallback func(ctx context.Context, client *wsc.Client, msg *wsc.HubMessage) error

// ErrorCallback 错误处理回调
type ErrorCallback func(ctx context.Context, err error, severity string) error

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

	// 回调列表
	connectCallbacks    []ClientConnectCallback
	disconnectCallbacks []ClientDisconnectCallback
	messageCallbacks    []MessageReceivedCallback
	errorCallbacks      []ErrorCallback
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

	ctx, cancel := context.WithCancel(context.Background())

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

	global.LOGGER.InfoKV("✅ WebSocket 服务已初始化",
		"node_ip", cfg.NodeIP,
		"node_port", cfg.NodePort,
		"heartbeat_interval_sec", cfg.HeartbeatInterval,
		"message_buffer_size", cfg.MessageBufferSize,
		"enable_ack", cfg.EnableAck)

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

	if ws.config == nil || !ws.config.Enabled {
		global.LOGGER.InfoMsg("⏭️  WebSocket 服务已禁用，跳过启动")
		return nil
	}

	// 创建 HTTP 路由
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.handleWebSocketUpgrade)

	ws.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", ws.config.NodeIP, ws.config.NodePort),
		Handler:      mux,
		ReadTimeout:  ws.config.ReadTimeout,
		WriteTimeout: ws.config.WriteTimeout,
		IdleTimeout:  ws.config.IdleTimeout,
	}

	// 从配置中获取网络类型,默认 tcp4 避免绑定到 IPv6
	network := ws.config.Network
	if network == "" {
		network = "tcp4" // 默认使用 tcp4
		global.LOGGER.WarnMsg("⚠️ WebSocket network 配置为空，使用默认值 tcp4")
	} else {
		global.LOGGER.InfoKV("WebSocket 网络配置", "network", network)
	}

	go func() {
		listener, err := net.Listen(network, ws.httpServer.Addr)
		if err != nil {
			global.LOGGER.WithError(err).ErrorKV("❌ WebSocket 监听器创建失败", "network", network, "address", ws.httpServer.Addr)
			return
		}
		if err := ws.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			global.LOGGER.WithError(err).ErrorMsg("❌ WebSocket HTTP 服务器启动失败")
		}
	}()

	ws.running.Store(true)
	global.LOGGER.InfoKV("✅ WebSocket 服务已启动",
		"address", ws.httpServer.Addr,
		"network", network,
		"path", "/ws")

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

// handleWebSocketUpgrade 处理 WebSocket 升级请求
// 此函数只负责：升级连接 -> 创建客户端 -> 注册到 Hub
// 所有消息处理都由 go-wsc Hub 完成
func (ws *WebSocketService) handleWebSocketUpgrade(w http.ResponseWriter, r *http.Request) {
	// 基于 go-wsc 的默认升级器，配置缓冲区大小
	upgrader := wsc.DefaultUpgrader
	upgrader.ReadBufferSize = 1024
	upgrader.WriteBufferSize = 1024

	// 从配置中获取缓冲区大小（如果有）
	if ws.config != nil {
		if ws.config.MessageBufferSize > 0 {
			upgrader.ReadBufferSize = int(ws.config.MessageBufferSize)
			upgrader.WriteBufferSize = int(ws.config.MessageBufferSize)
		}

		// 自定义 Origin 检查
		if len(ws.config.WebSocketOrigins) > 0 {
			upgrader.CheckOrigin = func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				for _, allowedOrigin := range ws.config.WebSocketOrigins {
					if allowedOrigin == "*" || allowedOrigin == origin {
						return true
					}
				}
				return false
			}
		}
	}

	// 升级连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		global.LOGGER.WithError(err).WarnMsg("WebSocket 升级失败")
		return
	}

	// 🔧 从请求中提取客户端属性
	clientID, userID, userType := ws.extractClientAttributes(r)

	// 转换为 wsc.UserType
	var clientUserType wsc.UserType
	switch userType {
	case "customer":
		clientUserType = wsc.UserTypeCustomer
	case "agent":
		clientUserType = wsc.UserTypeAgent
	case "admin":
		clientUserType = wsc.UserTypeAdmin
	case "bot":
		clientUserType = wsc.UserTypeBot
	case "vip":
		clientUserType = wsc.UserTypeVIP
	default:
		clientUserType = wsc.UserTypeCustomer // 默认为客户
	}

	client := &wsc.Client{
		ID:       clientID,
		UserID:   userID,
		UserType: clientUserType,
		Conn:     conn,
		LastSeen: time.Now(),
		Status:   wsc.UserStatusOnline,
		SendChan: make(chan []byte, ws.config.MessageBufferSize),
		Context:  context.WithValue(r.Context(), wsc.ContextKeySenderID, userID),
	}

	// 🔥 关键修复：先启动客户端写入 goroutine，再注册到 Hub
	// 这样可以避免在注册和启动 write goroutine 之间收到消息时导致消息丢失
	go func() {
		defer ws.hub.Unregister(client)
		defer func() {
			if client.Conn != nil {
				client.Conn.Close()
			}
		}()

		// 注册到 Hub（go-wsc 接管后续所有处理）
		ws.hub.Register(client)

		// 执行连接回调
		if err := ws.executeConnectCallbacks(ws.ctx, client); err != nil {
			ws.executeErrorCallbacks(ws.ctx, err, "error")
			return
		}

		// 处理消息循环
		ws.handleMessageLoop(client)
	}()
}

// handleMessageLoop 处理客户端消息循环
func (ws *WebSocketService) handleMessageLoop(client *wsc.Client) {
	for {
		select {
		case <-ws.ctx.Done():
			_ = ws.executeDisconnectCallbacks(ws.ctx, client, "context_done")
			return
		default:
		}
		// 读取消息
		messageType, data, err := client.Conn.ReadMessage()
		if err != nil {
			// WebSocket 连接错误，执行断开连接回调
			_ = ws.executeDisconnectCallbacks(ws.ctx, client, "read_error")
			return
		}

		// 更新最后活跃时间
		client.LastSeen = time.Now()

		// 根据 WebSocket 消息类型处理
		switch messageType {
		case 1: // TextMessage
			ws.handleTextMessage(client, data)
		case 2: // BinaryMessage
			ws.handleBinaryMessage(client, data)
		case 8: // CloseMessage
			_ = ws.executeDisconnectCallbacks(ws.ctx, client, "close_message")
			return
		case 9: // PingMessage
			// 响应 Pong
			_ = client.Conn.WriteMessage(10, nil)
		case 10: // PongMessage
			// 忽略 Pong 消息
		default:
			global.LOGGER.DebugKV("收到未知类型的消息", "type", messageType)
		}
	}
}

// handleTextMessage 处理文本消息
func (ws *WebSocketService) handleTextMessage(client *wsc.Client, data []byte) {
	// 尝试解析为 JSON 格式的 HubMessage
	var msg wsc.HubMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		// 不是 JSON 格式，当作纯文本处理
		msg = wsc.HubMessage{
			ID:          fmt.Sprintf("text_%s_%d", client.UserID, time.Now().UnixNano()),
			Sender:      client.UserID,
			SenderType:  client.UserType,
			Content:     string(data),
			MessageType: wsc.MessageTypeText,
			CreateAt:    time.Now(),
			Priority:    wsc.PriorityNormal,
			Status:      wsc.MessageStatusSent,
		}
	} else {
		// 是 JSON 格式，补充必要字段
		if msg.Sender == "" {
			msg.Sender = client.UserID
		}
		if msg.SenderType == "" {
			msg.SenderType = client.UserType
		}
		if msg.CreateAt.IsZero() {
			msg.CreateAt = time.Now()
		}
		if msg.MessageType == "" {
			msg.MessageType = wsc.MessageTypeText
		}
		if msg.ID == "" {
			msg.ID = fmt.Sprintf("json_%s_%d", client.UserID, time.Now().UnixNano())
		}
		if msg.Priority == "" {
			msg.Priority = wsc.PriorityNormal
		}
		if msg.Status == "" {
			msg.Status = wsc.MessageStatusSent
		}
	}

	// 处理心跳消息
	if msg.MessageType == wsc.MessageTypeHeartbeat {
		ws.handleHeartbeatMessage(client)
		return
	}

	// 执行消息接收回调
	if err := ws.executeMessageReceivedCallbacks(ws.ctx, client, &msg); err != nil {
		ws.executeErrorCallbacks(ws.ctx, err, "warning")
	}

	// 🔥 关键修复：将消息转发到 Hub 的 broadcast 队列
	if msg.Receiver != "" {
		// 点对点消息
		if err := ws.hub.SendToUser(ws.ctx, msg.Receiver, &msg); err != nil {
			global.LOGGER.WarnKV("消息发送失败",
				"message_id", msg.ID,
				"sender", msg.Sender,
				"receiver", msg.Receiver,
				"error", err,
			)
			ws.executeErrorCallbacks(ws.ctx, err, "error")
		}
	} else {
		// 广播消息（没有指定接收者）
		ws.hub.Broadcast(ws.ctx, &msg)
	}
}

// handleHeartbeatMessage 处理心跳消息
func (ws *WebSocketService) handleHeartbeatMessage(client *wsc.Client) {
	// 更新心跳时间
	ws.hub.UpdateHeartbeat(client.ID)

	// 🔥 发送 pong 响应
	pongMsg := &wsc.HubMessage{
		ID:          fmt.Sprintf("pong_%s_%d", client.UserID, time.Now().UnixNano()),
		MessageType: wsc.MessageTypePong,
		Sender:      "system",
		Receiver:    client.UserID,
		CreateAt:    time.Now(),
		Priority:    wsc.PriorityNormal,
		Status:      wsc.MessageStatusSent,
	}

	// 添加错误处理和日志
	if err := ws.hub.SendToUser(ws.ctx, client.UserID, pongMsg); err != nil {
		global.LOGGER.WarnKV("心跳 pong 响应发送失败",
			"client_id", client.ID,
			"user_id", client.UserID,
			"error", err,
		)
	} else {
		global.LOGGER.DebugKV("心跳 pong 响应发送成功",
			"client_id", client.ID,
			"user_id", client.UserID,
			"pong_msg_id", pongMsg.ID,
		)
	}
}

// handleBinaryMessage 处理二进制消息
func (ws *WebSocketService) handleBinaryMessage(client *wsc.Client, data []byte) {
	msg := &wsc.HubMessage{
		ID:          fmt.Sprintf("binary_%s_%d", client.UserID, time.Now().UnixNano()),
		Sender:      client.UserID,
		SenderType:  client.UserType,
		Content:     string(data),
		MessageType: wsc.MessageTypeBinary,
		CreateAt:    time.Now(),
		Priority:    wsc.PriorityNormal,
		Status:      wsc.MessageStatusSent,
		Data: map[string]interface{}{
			"binary_length": len(data),
		},
	}

	// 执行消息接收回调
	if err := ws.executeMessageReceivedCallbacks(ws.ctx, client, msg); err != nil {
		ws.executeErrorCallbacks(ws.ctx, err, "warning")
	}

	// 🔥 关键修复：将二进制消息转发到 Hub
	if msg.Receiver != "" {
		if err := ws.hub.SendToUser(ws.ctx, msg.Receiver, msg); err != nil {
			global.LOGGER.WarnKV("二进制消息发送失败",
				"message_id", msg.ID,
				"sender", msg.Sender,
				"receiver", msg.Receiver,
				"error", err,
			)
			ws.executeErrorCallbacks(ws.ctx, err, "error")
		}
	}
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

// ============================================================================
// 回调注册方法
// ============================================================================

// OnClientConnect 注册客户端连接回调
func (ws *WebSocketService) OnClientConnect(cb ClientConnectCallback) {
	ws.connectCallbacks = append(ws.connectCallbacks, cb)
}

// OnClientDisconnect 注册客户端断开连接回调
func (ws *WebSocketService) OnClientDisconnect(cb ClientDisconnectCallback) {
	ws.disconnectCallbacks = append(ws.disconnectCallbacks, cb)
}

// OnMessageReceived 注册消息接收回调
func (ws *WebSocketService) OnMessageReceived(cb MessageReceivedCallback) {
	ws.messageCallbacks = append(ws.messageCallbacks, cb)
}

// OnError 注册错误处理回调
func (ws *WebSocketService) OnError(cb ErrorCallback) {
	ws.errorCallbacks = append(ws.errorCallbacks, cb)
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
func (ws *WebSocketService) OnHeartbeatTimeout(callback func(clientID, userID string, lastHeartbeat time.Time)) {
	ws.hub.OnHeartbeatTimeout(callback)
}

// SetHeartbeatConfig 设置心跳配置
//
// 参数:
//   - interval: 心跳检查间隔，建议30秒
//   - timeout: 心跳超时时间，建议90秒（interval的3倍）
//
// 示例:
//
//	ws.SetHeartbeatConfig(30*time.Second, 90*time.Second)
func (ws *WebSocketService) SetHeartbeatConfig(interval, timeout time.Duration) {
	ws.hub.SetHeartbeatConfig(interval, timeout)
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

// ============================================================================
// 回调执行方法（内部使用）
// ============================================================================

// executeConnectCallbacks 执行连接回调
func (ws *WebSocketService) executeConnectCallbacks(ctx context.Context, client *wsc.Client) error {
	for _, cb := range ws.connectCallbacks {
		if err := cb(ctx, client); err != nil {
			return err
		}
	}
	return nil
}

// executeDisconnectCallbacks 执行断开连接回调
func (ws *WebSocketService) executeDisconnectCallbacks(ctx context.Context, client *wsc.Client, reason string) error {
	for _, cb := range ws.disconnectCallbacks {
		if err := cb(ctx, client, reason); err != nil {
			return err
		}
	}
	return nil
}

// executeMessageReceivedCallbacks 执行消息接收回调
func (ws *WebSocketService) executeMessageReceivedCallbacks(ctx context.Context, client *wsc.Client, msg *wsc.HubMessage) error {
	for _, cb := range ws.messageCallbacks {
		if err := cb(ctx, client, msg); err != nil {
			return err
		}
	}
	return nil
}

// executeErrorCallbacks 执行错误处理回调
func (ws *WebSocketService) executeErrorCallbacks(ctx context.Context, err error, severity string) error {
	for _, cb := range ws.errorCallbacks {
		if cbErr := cb(ctx, err, severity); cbErr != nil {
			return cbErr
		}
	}
	return nil
}
