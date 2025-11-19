/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-16 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-16 19:24:10
 * @FilePath: \go-rpc-gateway\server\wsc.go
 * @Description: WebSocket 集成层
 * 直接暴露 go-wsc Hub 的所有能力，不重复实现
 * 只负责：配置初始化、HTTP 升级、生命周期管理、回调链
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package server

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
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

// WebSocketService WebSocket 服务 - 包装 go-wsc Hub，提供集成能力
// 核心职责：
// 1. 配置初始化 -> Hub 创建
// 2. HTTP 升级处理 -> 客户端注册
// 3. 生命周期管理 -> Start/Stop
// 4. 回调链管理 -> 连接/消息事件
// 5. 直接委托 Hub API -> SendToUser/Broadcast/etc
type WebSocketService struct {
	// ===== 核心组件 =====
	hub        *wsc.Hub       // go-wsc Hub 实例（所有能力都来自这里）
	config     *wscconfig.WSC // go-config WSC 配置
	httpServer *http.Server   // HTTP 服务器

	// ===== 生命周期控制 =====
	ctx     context.Context
	cancel  context.CancelFunc
	running atomic.Bool // 使用 atomic 替代 RWMutex，更轻量级

	// ===== 回调链（仅用于用户自定义逻辑注入）=====
	connectCallbacks     []ClientConnectCallback
	disconnectCallbacks  []ClientDisconnectCallback
	messageRecvCallbacks []MessageReceivedCallback
	errorCallbacks       []ErrorCallback
	callbackMu           sync.RWMutex // 保护回调链的并发访问
}

// ============================================================================
// 初始化
// ============================================================================

// NewWebSocketService 创建 WebSocket 服务
// 仅初始化配置和 Hub，不启动 HTTP 服务器
func NewWebSocketService(cfg *wscconfig.WSC) (*WebSocketService, error) {
	if cfg == nil {
		return nil, errors.NewError(errors.ErrCodeInvalidConfiguration, "WSC config is nil")
	}

	// 使用 Safe 方式检查配置
	cfgSafe := cfg.Safe()
	if !cfgSafe.Enabled() {
		global.LOGGER.InfoMsg("⏭️  WebSocket 服务已禁用")
		return nil, errors.NewError(errors.ErrCodeInvalidConfiguration, "WebSocket is disabled")
	}

	// 创建 Hub 配置 - 优先使用传入配置，没有的字段使用默认值
	// 使用 go-config 的 Safe 访问器，已经内置了默认值逻辑
	hubConfig := cfg.
		WithNodeIP(cfgSafe.NodeIP()).
		WithNodePort(cfgSafe.NodePort()).
		WithHeartbeatInterval(cfgSafe.HeartbeatInterval()).
		WithClientTimeout(cfgSafe.ClientTimeout()).
		WithMessageBufferSize(cfgSafe.MessageBufferSize())
	// 检查性能配置 - 如果Group配置不存在，创建并设置消息记录
	if hubConfig.Group == nil {
		perfSafe := cfgSafe.Performance()
		enableMetrics := perfSafe.Field("EnableMetrics").Bool(true)
		hubConfig = hubConfig.WithGroup(wscconfig.DefaultGroup().
			Enable().
			WithMessageRecord(enableMetrics))
	}

	// 检查分布式/ACK 配置 - 如果Ticket配置不存在，根据分布式配置设置ACK
	if hubConfig.Ticket == nil {
		distSafe := cfgSafe.Distributed()
		redisSafe := cfgSafe.Redis()
		if distSafe.Field("Enabled").Bool(false) && redisSafe.Field("Enabled").Bool(false) {
			hubConfig = hubConfig.WithTicket(wscconfig.DefaultTicket().
				Enable().
				WithAck(true, 5000, 3))
		}
	}

	// 创建 Hub
	hub := wsc.NewHub(hubConfig)
	if hub == nil {
		return nil, errors.NewError(errors.ErrCodeInternalServerError, "failed to create WebSocket Hub")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 启动 Hub 事件循环（go-wsc 的核心消息处理）
	go hub.Run()

	service := &WebSocketService{
		hub:    hub,
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	global.LOGGER.InfoKV("✅ WebSocket 服务已初始化",
		"node_ip", hubConfig.NodeIP,
		"node_port", hubConfig.NodePort,
		"heartbeat_interval_sec", cfgSafe.HeartbeatInterval(30),
		"message_buffer_size", cfgSafe.MessageBufferSize(256),
		"enable_ack", hubConfig.Ticket != nil && hubConfig.Ticket.EnableAck)

	return service, nil
}

// ============================================================================
// 回调链管理
// ============================================================================

// OnClientConnect 添加客户端连接回调
func (ws *WebSocketService) OnClientConnect(cb ClientConnectCallback) *WebSocketService {
	ws.callbackMu.Lock()
	defer ws.callbackMu.Unlock()
	ws.connectCallbacks = append(ws.connectCallbacks, cb)
	return ws
}

// OnClientDisconnect 添加客户端断开连接回调
func (ws *WebSocketService) OnClientDisconnect(cb ClientDisconnectCallback) *WebSocketService {
	ws.callbackMu.Lock()
	defer ws.callbackMu.Unlock()
	ws.disconnectCallbacks = append(ws.disconnectCallbacks, cb)
	return ws
}

// OnMessageReceived 添加消息接收回调
func (ws *WebSocketService) OnMessageReceived(cb MessageReceivedCallback) *WebSocketService {
	ws.callbackMu.Lock()
	defer ws.callbackMu.Unlock()
	ws.messageRecvCallbacks = append(ws.messageRecvCallbacks, cb)
	return ws
}

// OnError 添加错误处理回调
func (ws *WebSocketService) OnError(cb ErrorCallback) *WebSocketService {
	ws.callbackMu.Lock()
	defer ws.callbackMu.Unlock()
	ws.errorCallbacks = append(ws.errorCallbacks, cb)
	return ws
}

// ============================================================================
// 执行回调链的辅助方法
// ============================================================================

func (ws *WebSocketService) executeConnectCallbacks(ctx context.Context, client *wsc.Client) error {
	ws.callbackMu.RLock()
	callbacks := ws.connectCallbacks
	ws.callbackMu.RUnlock()

	for _, cb := range callbacks {
		if err := cb(ctx, client); err != nil {
			return err
		}
	}
	return nil
}

func (ws *WebSocketService) executeDisconnectCallbacks(ctx context.Context, client *wsc.Client, reason string) error {
	ws.callbackMu.RLock()
	callbacks := ws.disconnectCallbacks
	ws.callbackMu.RUnlock()

	for _, cb := range callbacks {
		if err := cb(ctx, client, reason); err != nil {
			return err
		}
	}
	return nil
}

func (ws *WebSocketService) executeMessageReceivedCallbacks(ctx context.Context, client *wsc.Client, msg *wsc.HubMessage) error {
	ws.callbackMu.RLock()
	callbacks := ws.messageRecvCallbacks
	ws.callbackMu.RUnlock()

	for _, cb := range callbacks {
		if err := cb(ctx, client, msg); err != nil {
			return err
		}
	}
	return nil
}

func (ws *WebSocketService) executeErrorCallbacks(ctx context.Context, err error, severity string) error {
	ws.callbackMu.RLock()
	callbacks := ws.errorCallbacks
	ws.callbackMu.RUnlock()

	for _, cb := range callbacks {
		if cbErr := cb(ctx, err, severity); cbErr != nil {
			return cbErr
		}
	}
	return nil
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
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 启动 HTTP 服务器
	go func() {
		if err := ws.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			ws.executeErrorCallbacks(ws.ctx, err, "error")
			global.LOGGER.WithError(err).ErrorMsg("❌ WebSocket HTTP 服务器启动失败")
		}
	}()

	ws.running.Store(true)
	global.LOGGER.InfoKV("✅ WebSocket 服务已启动",
		"address", ws.httpServer.Addr,
		"path", "/ws")

	return nil
}

// Stop 停止 WebSocket 服务
func (ws *WebSocketService) Stop() error {
	if !ws.running.Load() {
		return nil
	}

	global.LOGGER.InfoMsg("🛑 停止 WebSocket 服务...")

	ws.cancel()

	if ws.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ws.httpServer.Shutdown(ctx)
	}

	if ws.hub != nil {
		ws.hub.Shutdown()
	}

	ws.running.Store(false)
	global.LOGGER.InfoMsg("✅ WebSocket 服务已停止")

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
// 此函数只负责：升级连接 -> 创建客户端 -> 注册到 Hub -> 管理生命周期
// 所有消息处理都由 go-wsc Hub 完成
func (ws *WebSocketService) handleWebSocketUpgrade(w http.ResponseWriter, r *http.Request) {
	// 创建升级器
	upgrader := &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// 检查 Origin
			if ws.config != nil && ws.config.WebSocketOrigins != nil && len(ws.config.WebSocketOrigins) > 0 {
				origin := r.Header.Get("Origin")
				for _, allowedOrigin := range ws.config.WebSocketOrigins {
					if allowedOrigin == "*" || allowedOrigin == origin {
						return true
					}
				}
				return false
			}
			return true
		},
	}

	// 升级连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		ws.executeErrorCallbacks(ws.ctx, err, "warning")
		return
	}

	// 🔧 优先从 URL 查询参数获取，其次从 Header 获取
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

	// 注册到 Hub（go-wsc 接管后续所有处理）
	ws.hub.Register(client)
	defer ws.hub.Unregister(client)

	// 执行连接回调
	if err := ws.executeConnectCallbacks(ws.ctx, client); err != nil {
		ws.executeErrorCallbacks(ws.ctx, err, "error")
	}

	// 处理消息循环
	// 注意：这里是简化版，go-wsc Hub 有更复杂的实现
	for {
		select {
		case <-ws.ctx.Done():
			return
		default:
		}

		_, data, err := client.Conn.ReadMessage()
		if err != nil {
			// 执行断开连接回调
			_ = ws.executeDisconnectCallbacks(ws.ctx, client, "read_error")
			return
		}

		client.LastSeen = time.Now()

		// 创建消息对象
		msg := &wsc.HubMessage{
			From:     client.UserID,
			Content:  string(data),
			Type:     wsc.MessageTypeText,
			CreateAt: time.Now(),
		}

		// 执行消息接收回调
		if err := ws.executeMessageReceivedCallbacks(ws.ctx, client, msg); err != nil {
			ws.executeErrorCallbacks(ws.ctx, err, "warning")
			continue
		}

		// 路由消息给 Hub（Hub 处理 SendToUser/Broadcast 等逻辑）
		if msg.To != "" {
			// 发送给特定用户
			_ = ws.hub.SendToUser(ws.ctx, msg.To, msg)
		} else if msg.TicketID != "" {
			// 发送给特定凭证
			_ = ws.hub.SendToTicket(ws.ctx, msg.TicketID, msg)
		} else {
			// 广播给所有
			ws.hub.Broadcast(ws.ctx, msg)
		}
	}
}

// ============================================================================
// 直接暴露 go-wsc Hub API（不重复实现）
// ============================================================================

// SendToUser 发送消息给特定用户
// 直接委托给 go-wsc Hub
func (ws *WebSocketService) SendToUser(ctx context.Context, userID string, msg *wsc.HubMessage) error {
	if ws.hub == nil {
		return errors.NewError(errors.ErrCodeInternalServerError, "WebSocket Hub not initialized")
	}
	return ws.hub.SendToUser(ctx, userID, msg)
}

// SendToUserWithAck 发送消息给特定用户（带 ACK）
// 直接委托给 go-wsc Hub
func (ws *WebSocketService) SendToUserWithAck(ctx context.Context, userID string, msg *wsc.HubMessage, timeout time.Duration, maxRetry int) (*wsc.AckMessage, error) {
	if ws.hub == nil {
		return nil, errors.NewError(errors.ErrCodeInternalServerError, "WebSocket Hub not initialized")
	}
	return ws.hub.SendToUserWithAck(ctx, userID, msg, timeout, maxRetry)
}

// SendToTicket 发送消息给特定凭证
// 直接委托给 go-wsc Hub
func (ws *WebSocketService) SendToTicket(ctx context.Context, ticketID string, msg *wsc.HubMessage) error {
	if ws.hub == nil {
		return errors.NewError(errors.ErrCodeInternalServerError, "WebSocket Hub not initialized")
	}
	return ws.hub.SendToTicket(ctx, ticketID, msg)
}

// SendToTicketWithAck 发送消息给特定凭证（带 ACK）
// 直接委托给 go-wsc Hub
func (ws *WebSocketService) SendToTicketWithAck(ctx context.Context, ticketID string, msg *wsc.HubMessage, timeout time.Duration, maxRetry int) (*wsc.AckMessage, error) {
	if ws.hub == nil {
		return nil, errors.NewError(errors.ErrCodeInternalServerError, "WebSocket Hub not initialized")
	}
	return ws.hub.SendToTicketWithAck(ctx, ticketID, msg, timeout, maxRetry)
}

// Broadcast 广播消息给所有客户端
// 直接委托给 go-wsc Hub
func (ws *WebSocketService) Broadcast(ctx context.Context, msg *wsc.HubMessage) {
	if ws.hub != nil {
		ws.hub.Broadcast(ctx, msg)
	}
}

// GetOnlineUsers 获取所有在线用户列表
// 直接委托给 go-wsc Hub
func (ws *WebSocketService) GetOnlineUsers() []string {
	if ws.hub == nil {
		return []string{}
	}
	return ws.hub.GetOnlineUsers()
}

// GetOnlineUserCount 获取在线用户数量
// 直接委托给 go-wsc Hub
func (ws *WebSocketService) GetOnlineUserCount() int {
	if ws.hub == nil {
		return 0
	}
	return len(ws.hub.GetOnlineUsers())
}

// GetStats 获取 WebSocket 统计信息
// 直接委托给 go-wsc Hub
func (ws *WebSocketService) GetStats() map[string]interface{} {
	if ws.hub == nil {
		return map[string]interface{}{}
	}
	return ws.hub.GetStats()
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
