/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-15 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 10:00:00
 * @FilePath: \go-rpc-gateway\middleware\wsc.go
 * @Description: WebSocket通信中间件 - 自动启用+回调扩展
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"fmt"
	"net/http"

	wscconfig "github.com/kamalyes/go-config/pkg/wsc"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/wsc"
	gowsc "github.com/kamalyes/go-wsc"
)

// WSCCallbacks WebSocket通信回调接口
// 开发者可以实现这些回调来自定义WSC行为
type WSCCallbacks struct {
	// OnClientConnect 客户端连接时回调
	// 返回 error 会拒绝连接
	OnClientConnect func(ctx context.Context, client *gowsc.Client) error

	// OnClientDisconnect 客户端断开时回调
	OnClientDisconnect func(ctx context.Context, client *gowsc.Client)

	// OnMessageReceived 收到消息时回调
	// 返回 false 会阻止消息继续传递
	OnMessageReceived func(ctx context.Context, client *gowsc.Client, msg *gowsc.HubMessage) bool

	// OnMessageSend 发送消息前回调
	// 可以修改消息内容或返回 error 阻止发送
	OnMessageSend func(ctx context.Context, msg *gowsc.HubMessage) error

	// OnBroadcast 广播消息前回调
	OnBroadcast func(ctx context.Context, msg *gowsc.HubMessage) error

	// WelcomeMessage 生成欢迎消息（可选）
	// 当客户端连接成功后，会收到此消息
	WelcomeMessage func(ctx context.Context, client *gowsc.Client) *gowsc.HubMessage

	// AuthenticateUser 用户认证回调
	// 从请求中提取并验证用户信息
	// 返回 userID, userType, error
	AuthenticateUser func(r *http.Request) (userID string, userType gowsc.UserType, err error)

	// OnError 错误处理回调
	OnError func(ctx context.Context, err error, source string)
}

// WSCConfig WebSocket通信中间件配置
type WSCConfig struct {
	Config        *wscconfig.WSC // WSC基础配置
	Callbacks     *WSCCallbacks  // 回调函数
	WebSocketPath string         // WebSocket路由路径，默认 "/ws"
	SSEPath       string         // SSE路由路径，默认 "/sse"
	StatsPath     string         // 统计信息路径，默认 "/wsc/stats"
	OnlinePath    string         // 在线用户路径，默认 "/wsc/online"
}

// WSCMiddleware WebSocket通信中间件
type WSCMiddleware struct {
	adapter   *wsc.WSCAdapter
	config    *WSCConfig
	callbacks *WSCCallbacks
	enabled   bool
}

// NewWSCMiddleware 创建WebSocket通信中间件
func NewWSCMiddleware(config *WSCConfig) *WSCMiddleware {
	if config == nil || config.Config == nil || !config.Config.Enabled {
		return &WSCMiddleware{enabled: false}
	}

	// 设置默认路径
	if config.WebSocketPath == "" {
		config.WebSocketPath = "/ws"
	}
	if config.SSEPath == "" {
		config.SSEPath = "/sse"
	}
	if config.StatsPath == "" {
		config.StatsPath = "/wsc/stats"
	}
	if config.OnlinePath == "" {
		config.OnlinePath = "/wsc/online"
	}

	// 初始化回调
	if config.Callbacks == nil {
		config.Callbacks = &WSCCallbacks{}
	}

	// 创建适配器
	adapter := wsc.NewWSCAdapter(config.Config)
	if adapter == nil {
		return &WSCMiddleware{enabled: false}
	}

	middleware := &WSCMiddleware{
		adapter:   adapter,
		config:    config,
		callbacks: config.Callbacks,
		enabled:   true,
	}

	global.LOGGER.Info("✅ WSC中间件已初始化 [WebSocket=%s, SSE=%s]", 
		config.WebSocketPath, config.SSEPath)

	return middleware
}

// Name 返回中间件名称
func (m *WSCMiddleware) Name() string {
	return "wsc"
}

// IsEnabled 检查是否启用
func (m *WSCMiddleware) IsEnabled() bool {
	return m.enabled
}

// RegisterRoutes 注册路由（自动调用）
func (m *WSCMiddleware) RegisterRoutes(mux interface{}) error {
	if !m.enabled {
		return nil
	}

	// 支持 *http.ServeMux 和其他路由器
	if httpMux, ok := mux.(*http.ServeMux); ok {
		// WebSocket 路由
		httpMux.HandleFunc(m.config.WebSocketPath, m.handleWebSocket)
		// SSE 路由
		httpMux.HandleFunc(m.config.SSEPath, m.handleSSE)
		// 统计信息路由
		httpMux.HandleFunc(m.config.StatsPath, m.handleStats)
		// 在线用户路由
		httpMux.HandleFunc(m.config.OnlinePath, m.handleOnlineUsers)

		global.LOGGER.Info("📡 WSC路由已注册:")
		global.LOGGER.Info("   WebSocket: %s", m.config.WebSocketPath)
		global.LOGGER.Info("   SSE:       %s", m.config.SSEPath)
		global.LOGGER.Info("   Stats:     %s", m.config.StatsPath)
		global.LOGGER.Info("   Online:    %s", m.config.OnlinePath)
	}

	return nil
}

// handleWebSocket 处理WebSocket连接
func (m *WSCMiddleware) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 认证用户
	userID, userType, err := m.authenticateUser(r)
	if err != nil {
		m.handleError(ctx, err, "WebSocket认证失败")
		http.Error(w, "认证失败: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// 更新上下文
	ctx = context.WithValue(ctx, gowsc.ContextKeyUserID, userID)
	ctx = context.WithValue(ctx, "user_type", string(userType))
	r = r.WithContext(ctx)

	// 连接前回调
	if m.callbacks.OnClientConnect != nil {
		client := &gowsc.Client{
			UserID:   userID,
			UserType: userType,
		}
		if err := m.callbacks.OnClientConnect(ctx, client); err != nil {
			m.handleError(ctx, err, "连接前回调失败")
			http.Error(w, "连接被拒绝: "+err.Error(), http.StatusForbidden)
			return
		}
	}

	// 委托给适配器处理
	m.adapter.HandleWebSocket(w, r)
}

// handleSSE 处理SSE连接
func (m *WSCMiddleware) handleSSE(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 认证用户
	userID, userType, err := m.authenticateUser(r)
	if err != nil {
		m.handleError(ctx, err, "SSE认证失败")
		http.Error(w, "认证失败: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// 更新上下文
	ctx = context.WithValue(ctx, gowsc.ContextKeyUserID, userID)
	ctx = context.WithValue(ctx, "user_type", string(userType))
	r = r.WithContext(ctx)

	// 委托给适配器处理
	m.adapter.HandleSSE(w, r)
}

// handleStats 处理统计信息请求
func (m *WSCMiddleware) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := m.adapter.GetStats()
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	// 简单的JSON序列化
	fmt.Fprintf(w, `{"status":"ok","data":%v}`, stats)
}

// handleOnlineUsers 处理在线用户请求
func (m *WSCMiddleware) handleOnlineUsers(w http.ResponseWriter, r *http.Request) {
	users := m.adapter.GetOnlineUsers()
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	fmt.Fprintf(w, `{"status":"ok","count":%d,"users":%v}`, len(users), users)
}

// authenticateUser 认证用户
func (m *WSCMiddleware) authenticateUser(r *http.Request) (string, gowsc.UserType, error) {
	// 优先使用自定义认证回调
	if m.callbacks.AuthenticateUser != nil {
		return m.callbacks.AuthenticateUser(r)
	}

	// 默认认证逻辑
	ctx := r.Context()
	
	// 从上下文获取
	if userID, ok := ctx.Value(gowsc.ContextKeyUserID).(string); ok && userID != "" {
		userType := gowsc.UserTypeCustomer
		if ut, ok := ctx.Value("user_type").(string); ok {
			userType = gowsc.UserType(ut)
		}
		return userID, userType, nil
	}

	// 从查询参数获取
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = r.Header.Get("X-User-ID")
	}

	if userID == "" {
		return "", "", errors.ErrUserIDMissing
	}

	userType := gowsc.UserTypeCustomer
	if ut := r.URL.Query().Get("user_type"); ut != "" {
		userType = gowsc.UserType(ut)
	}

	return userID, userType, nil
}

// handleError 处理错误
func (m *WSCMiddleware) handleError(ctx context.Context, err error, source string) {
	if m.callbacks.OnError != nil {
		m.callbacks.OnError(ctx, err, source)
	} else {
		global.LOGGER.Error("[WSC] %s: %v", source, err)
	}
}

// SendMessage 发送消息（带回调）
func (m *WSCMiddleware) SendMessage(ctx context.Context, msg *gowsc.HubMessage) error {
	if !m.enabled {
		return errors.ErrWSCNotEnabled
	}

	// 发送前回调
	if m.callbacks.OnMessageSend != nil {
		if err := m.callbacks.OnMessageSend(ctx, msg); err != nil {
			return err
		}
	}

	return m.adapter.SendMessage(ctx, msg)
}

// Broadcast 广播消息（带回调）
func (m *WSCMiddleware) Broadcast(ctx context.Context, msg *gowsc.HubMessage) error {
	if !m.enabled {
		return errors.ErrWSCNotEnabled
	}

	// 广播前回调
	if m.callbacks.OnBroadcast != nil {
		if err := m.callbacks.OnBroadcast(ctx, msg); err != nil {
			return err
		}
	}

	m.adapter.Broadcast(ctx, msg)
	return nil
}

// GetAdapter 获取底层适配器（供高级用户使用）
func (m *WSCMiddleware) GetAdapter() *wsc.WSCAdapter {
	return m.adapter
}

// GetOnlineUsers 获取在线用户
func (m *WSCMiddleware) GetOnlineUsers() []string {
	if !m.enabled {
		return []string{}
	}
	return m.adapter.GetOnlineUsers()
}

// GetStats 获取统计信息
func (m *WSCMiddleware) GetStats() map[string]interface{} {
	if !m.enabled {
		return map[string]interface{}{"enabled": false}
	}
	return m.adapter.GetStats()
}

// Shutdown 关闭WSC服务
func (m *WSCMiddleware) Shutdown() error {
	if m.enabled && m.adapter != nil {
		m.adapter.Shutdown()
	}
	return nil
}

// ==================== 便捷构建器 ====================

// WSCMiddlewareBuilder WSC中间件构建器
type WSCMiddlewareBuilder struct {
	config    *WSCConfig
	callbacks *WSCCallbacks
}

// NewWSCMiddlewareBuilder 创建WSC中间件构建器
func NewWSCMiddlewareBuilder(wscConfig *wscconfig.WSC) *WSCMiddlewareBuilder {
	return &WSCMiddlewareBuilder{
		config: &WSCConfig{
			Config:    wscConfig,
			Callbacks: &WSCCallbacks{},
		},
		callbacks: &WSCCallbacks{},
	}
}

// WithWebSocketPath 设置WebSocket路径
func (b *WSCMiddlewareBuilder) WithWebSocketPath(path string) *WSCMiddlewareBuilder {
	b.config.WebSocketPath = path
	return b
}

// WithSSEPath 设置SSE路径
func (b *WSCMiddlewareBuilder) WithSSEPath(path string) *WSCMiddlewareBuilder {
	b.config.SSEPath = path
	return b
}

// WithStatsPath 设置统计路径
func (b *WSCMiddlewareBuilder) WithStatsPath(path string) *WSCMiddlewareBuilder {
	b.config.StatsPath = path
	return b
}

// WithOnlinePath 设置在线用户路径
func (b *WSCMiddlewareBuilder) WithOnlinePath(path string) *WSCMiddlewareBuilder {
	b.config.OnlinePath = path
	return b
}

// OnClientConnect 设置客户端连接回调
func (b *WSCMiddlewareBuilder) OnClientConnect(
	callback func(ctx context.Context, client *gowsc.Client) error,
) *WSCMiddlewareBuilder {
	b.callbacks.OnClientConnect = callback
	return b
}

// OnClientDisconnect 设置客户端断开回调
func (b *WSCMiddlewareBuilder) OnClientDisconnect(
	callback func(ctx context.Context, client *gowsc.Client),
) *WSCMiddlewareBuilder {
	b.callbacks.OnClientDisconnect = callback
	return b
}

// OnMessageReceived 设置消息接收回调
func (b *WSCMiddlewareBuilder) OnMessageReceived(
	callback func(ctx context.Context, client *gowsc.Client, msg *gowsc.HubMessage) bool,
) *WSCMiddlewareBuilder {
	b.callbacks.OnMessageReceived = callback
	return b
}

// OnMessageSend 设置消息发送回调
func (b *WSCMiddlewareBuilder) OnMessageSend(
	callback func(ctx context.Context, msg *gowsc.HubMessage) error,
) *WSCMiddlewareBuilder {
	b.callbacks.OnMessageSend = callback
	return b
}

// OnBroadcast 设置广播回调
func (b *WSCMiddlewareBuilder) OnBroadcast(
	callback func(ctx context.Context, msg *gowsc.HubMessage) error,
) *WSCMiddlewareBuilder {
	b.callbacks.OnBroadcast = callback
	return b
}

// WithWelcomeMessage 设置欢迎消息生成器
func (b *WSCMiddlewareBuilder) WithWelcomeMessage(
	callback func(ctx context.Context, client *gowsc.Client) *gowsc.HubMessage,
) *WSCMiddlewareBuilder {
	b.callbacks.WelcomeMessage = callback
	return b
}

// WithAuthenticator 设置认证器
func (b *WSCMiddlewareBuilder) WithAuthenticator(
	callback func(r *http.Request) (userID string, userType gowsc.UserType, err error),
) *WSCMiddlewareBuilder {
	b.callbacks.AuthenticateUser = callback
	return b
}

// OnError 设置错误处理回调
func (b *WSCMiddlewareBuilder) OnError(
	callback func(ctx context.Context, err error, source string),
) *WSCMiddlewareBuilder {
	b.callbacks.OnError = callback
	return b
}

// Build 构建中间件
func (b *WSCMiddlewareBuilder) Build() *WSCMiddleware {
	b.config.Callbacks = b.callbacks
	return NewWSCMiddleware(b.config)
}

