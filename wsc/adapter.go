/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-13 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 12:00:00
 * @FilePath: \go-rpc-gateway\wsc\adapter.go
 * @Description: WebSocket通信适配器（轻量级封装 go-wsc Hub）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package wsc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	wscconfig "github.com/kamalyes/go-config/pkg/wsc"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	gowsc "github.com/kamalyes/go-wsc"
)

// HubMessage 复用 go-wsc Hub 的消息结构（导出供外部使用）
type HubMessage = gowsc.HubMessage

// WSCAdapter WebSocket 通信适配器（轻量级封装 go-wsc Hub）
// 职责：仅作为 go-wsc Hub 与 go-rpc-gateway 的桥接层
// 限流、鉴权等功能由 go-rpc-gateway 的中间件处理
type WSCAdapter struct {
	hub      *gowsc.Hub
	upgrader websocket.Upgrader
	enabled  bool
}

// NewWSCAdapter 创建实时通信服务（直接使用 go-config/pkg/gowsc.WSC 配置）
func NewWSCAdapter(config *wscconfig.WSC) *WSCAdapter {
	if config == nil || !config.Enabled {
		return &WSCAdapter{enabled: false}
	}

	// 创建 Hub 配置（直接使用 go-config 配置）
	hubConfig := &gowsc.HubConfig{
		NodeIP:            config.NodeIP,
		NodePort:          config.NodePort,
		HeartbeatInterval: time.Duration(config.HeartbeatInterval) * time.Second,
		ClientTimeout:     time.Duration(config.ClientTimeout) * time.Second,
		MessageBufferSize: config.MessageBufferSize,
		SSEHeartbeat:      time.Duration(config.SSEHeartbeat) * time.Second,
		SSETimeout:        time.Duration(config.SSETimeout) * time.Second,
		SSEMessageBuffer:  config.SSEMessageBuffer,
	}

	hub := gowsc.NewHub(hubConfig)
	go hub.Run()

	service := &WSCAdapter{
		hub: hub,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				if len(config.WebSocketOrigins) == 0 {
					return true
				}
				origin := r.Header.Get("Origin")
				for _, allowed := range config.WebSocketOrigins {
					if origin == allowed || allowed == "*" {
						return true
					}
				}
				return false
			},
		},
		enabled: true,
	}

	global.LOGGER.Info("✅ 实时通信服务已启动 (基于 go-wsc Hub)")
	return service
}

// HandleWebSocket 处理 WebSocket 连接
func (s *WSCAdapter) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !s.enabled {
		http.Error(w, "实时通信服务未启用", http.StatusServiceUnavailable)
		return
	}

	// 从上下文提取用户信息
	ctx := r.Context()
	userID, _ := ctx.Value(gowsc.ContextKeyUserID).(string)
	if userID == "" {
		userID = r.URL.Query().Get("user_id")
	}

	if userID == "" {
		http.Error(w, "缺少用户ID", http.StatusUnauthorized)
		return
	}

	// 升级连接
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		global.LOGGER.Error("WebSocket 升级失败: %v", err)
		return
	}

	// 创建客户端
	client := &gowsc.Client{
		ID:         generateClientID(),
		UserID:     userID,
		UserType:   extractUserType(ctx, r),
		Role:       extractUserRole(ctx, r),
		TicketID:   r.URL.Query().Get("ticket_id"),
		Conn:       conn,
		LastSeen:   time.Now(),
		Status:     gowsc.UserStatusOnline,
		Department: extractDepartment(ctx, r),
		NodeID:     s.hub.GetNodeID(),
		ClientType: gowsc.ClientTypeWeb,
		SendChan:   make(chan []byte, 256),
		Context:    createClientContext(ctx, userID),
		Metadata:   make(map[string]interface{}),
	}

	// 注册到Hub
	s.hub.Register(client)

	// 读取客户端消息
	go s.handleClientRead(client)
}

// HandleSSE 处理 SSE 连接
func (s *WSCAdapter) HandleSSE(w http.ResponseWriter, r *http.Request) {
	if !s.enabled {
		http.Error(w, "实时通信服务未启用", http.StatusServiceUnavailable)
		return
	}

	// 提取用户信息
	ctx := r.Context()
	userID, _ := ctx.Value(gowsc.ContextKeyUserID).(string)
	if userID == "" {
		userID = r.URL.Query().Get("user_id")
	}

	if userID == "" {
		http.Error(w, "缺少用户ID", http.StatusBadRequest)
		return
	}

	// 检查SSE支持
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 创建 SSE 连接
	conn := &gowsc.SSEConnection{
		UserID:     userID,
		Writer:     w,
		Flusher:    flusher,
		MessageCh:  make(chan *gowsc.HubMessage, 100),
		CloseCh:    make(chan struct{}),
		LastActive: time.Now(),
		Context:    createClientContext(ctx, userID),
	}

	// 注册到Hub
	s.hub.RegisterSSE(conn)
	defer s.hub.UnregisterSSE(userID)

	// 发送连接成功消息
	fmt.Fprintf(w, "data: {\"type\":\"connected\",\"user_id\":\"%s\"}\n\n", userID)
	flusher.Flush()

	// 监听消息
	for {
		select {
		case <-r.Context().Done():
			return
		case <-conn.CloseCh:
			return
		case msg := <-conn.MessageCh:
			s.sendSSEMessage(w, flusher, msg)
		}
	}
}

// SendMessage 发送消息（自动选择WebSocket或SSE）
func (s *WSCAdapter) SendMessage(ctx context.Context, msg *gowsc.HubMessage) error {
	if !s.enabled {
		return errors.ErrWSCNotEnabled
	}
	return s.hub.SendToUser(ctx, msg.To, msg)
}

// Broadcast 广播消息
func (s *WSCAdapter) Broadcast(ctx context.Context, msg *gowsc.HubMessage) {
	if s.enabled {
		s.hub.Broadcast(ctx, msg)
	}
}

// GetOnlineUsers 获取在线用户列表
func (s *WSCAdapter) GetOnlineUsers() []string {
	if !s.enabled {
		return []string{}
	}
	return s.hub.GetOnlineUsers()
}

// GetStats 获取统计信息
func (s *WSCAdapter) GetStats() map[string]interface{} {
	if !s.enabled {
		return map[string]interface{}{"enabled": false}
	}
	return s.hub.GetStats()
}

// Shutdown 关闭服务
func (s *WSCAdapter) Shutdown() {
	if s.enabled && s.hub != nil {
		s.hub.Shutdown()
	}
}

// === 内部方法 ===

func (s *WSCAdapter) handleClientRead(client *gowsc.Client) {
	defer func() {
		client.Conn.Close()
		s.hub.Unregister(client)
	}()

	for {
		var msg gowsc.HubMessage
		if err := client.Conn.ReadJSON(&msg); err != nil {
			return
		}

		client.LastSeen = time.Now()

		// 从客户端上下文自动填充发送者
		if msg.From == "" {
			msg.From = client.UserID
		}
		if msg.CreateAt.IsZero() {
			msg.CreateAt = time.Now()
		}

		// 发送到Hub
		if msg.To != "" {
			s.hub.SendToUser(client.Context, msg.To, &msg)
		} else if msg.TicketID != "" {
			s.hub.SendToTicket(client.Context, msg.TicketID, &msg)
		}
	}
}

func (s *WSCAdapter) sendSSEMessage(w http.ResponseWriter, f http.Flusher, msg *gowsc.HubMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", string(data))
	f.Flush()
}

// === 辅助函数 ===

func generateClientID() string {
	return fmt.Sprintf("client_%d", time.Now().UnixNano())
}

func extractUserType(ctx context.Context, r *http.Request) gowsc.UserType {
	if userType, ok := ctx.Value("user_type").(string); ok {
		return gowsc.UserType(userType)
	}
	userType := r.URL.Query().Get("user_type")
	if userType != "" {
		return gowsc.UserType(userType)
	}
	return gowsc.UserTypeCustomer
}

func extractUserRole(ctx context.Context, r *http.Request) gowsc.UserRole {
	if role, ok := ctx.Value("role").(string); ok {
		return gowsc.UserRole(role)
	}
	role := r.URL.Query().Get("role")
	if role != "" {
		return gowsc.UserRole(role)
	}
	return gowsc.UserRoleCustomer
}

func extractDepartment(ctx context.Context, r *http.Request) gowsc.Department {
	if dept, ok := ctx.Value("department").(string); ok {
		return gowsc.Department(dept)
	}
	return gowsc.DepartmentGeneral
}

func createClientContext(ctx context.Context, userID string) context.Context {
	ctx = context.WithValue(ctx, gowsc.ContextKeyUserID, userID)
	ctx = context.WithValue(ctx, gowsc.ContextKeySenderID, userID)
	return ctx
}

// GetNodeID 获取节点ID
func (s *WSCAdapter) GetNodeID() string {
	if !s.enabled || s.hub == nil {
		return ""
	}
	return s.hub.GetNodeID()
}

