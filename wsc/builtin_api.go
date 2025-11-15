/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-15 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 00:20:47
 * @FilePath: \go-rpc-gateway\wsc\builtin_api.go
 * @Description: WSC内置API - 开箱即用的WebSocket通信HTTP API
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package wsc

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	gowsc "github.com/kamalyes/go-wsc"
)

// WSCBuiltinAPI WebSocket通信内置API处理器
// 提供开箱即用的HTTP API接口，业务无需编写任何代码
type WSCBuiltinAPI struct {
	adapter       *WSCAdapter
	enableSend    bool // 是否启用发送消息API
	enableBcast   bool // 是否启用广播API
	enableOnline  bool // 是否启用在线用户API
	enableStats   bool // 是否启用统计API
	authRequired  bool // 是否需要认证
	adminOnly     bool // 是否仅管理员可用
}

// WSCBuiltinAPIConfig 内置API配置
type WSCBuiltinAPIConfig struct {
	EnableSend    bool `json:"enable_send"`     // 启用发送API，默认true
	EnableBcast   bool `json:"enable_broadcast"` // 启用广播API，默认false (需要管理员权限)
	EnableOnline  bool `json:"enable_online"`   // 启用在线用户API，默认true
	EnableStats   bool `json:"enable_stats"`    // 启用统计API，默认true
	AuthRequired  bool `json:"auth_required"`   // 是否需要认证，默认true
	AdminOnly     bool `json:"admin_only"`      // 广播等敏感操作是否仅管理员，默认true
}

// DefaultWSCBuiltinAPIConfig 默认内置API配置
func DefaultWSCBuiltinAPIConfig() *WSCBuiltinAPIConfig {
	return &WSCBuiltinAPIConfig{
		EnableSend:   true,
		EnableBcast:  false, // 默认不启用广播（需要显式启用）
		EnableOnline: true,
		EnableStats:  true,
		AuthRequired: true,
		AdminOnly:    true,
	}
}

// NewWSCBuiltinAPI 创建内置API处理器
func NewWSCBuiltinAPI(adapter *WSCAdapter, config *WSCBuiltinAPIConfig) *WSCBuiltinAPI {
	if config == nil {
		config = DefaultWSCBuiltinAPIConfig()
	}

	return &WSCBuiltinAPI{
		adapter:      adapter,
		enableSend:   config.EnableSend,
		enableBcast:  config.EnableBcast,
		enableOnline: config.EnableOnline,
		enableStats:  config.EnableStats,
		authRequired: config.AuthRequired,
		adminOnly:    config.AdminOnly,
	}
}

// RegisterRoutes 注册所有内置API路由
func (api *WSCBuiltinAPI) RegisterRoutes(mux *http.ServeMux, basePath string) {
	if basePath == "" {
		basePath = "/api/wsc"
	}

	if api.enableSend {
		mux.HandleFunc(basePath+"/send", api.handleSendMessage)
		global.LOGGER.Info("   📤 Send Message: %s/send", basePath)
	}

	if api.enableBcast {
		mux.HandleFunc(basePath+"/broadcast", api.handleBroadcast)
		global.LOGGER.Info("   📢 Broadcast: %s/broadcast", basePath)
	}

	if api.enableOnline {
		mux.HandleFunc(basePath+"/online", api.handleOnlineUsers)
		global.LOGGER.Info("   👥 Online Users: %s/online", basePath)
	}

	if api.enableStats {
		mux.HandleFunc(basePath+"/stats", api.handleStats)
		global.LOGGER.Info("   📊 Statistics: %s/stats", basePath)
	}
}

// ==================== API处理器 ====================

// SendMessageRequest 发送消息请求
type SendMessageRequest struct {
	To       string                 `json:"to"`        // 接收者用户ID（必需）
	Type     gowsc.MessageType      `json:"type"`      // 消息类型（可选，默认text）
	Content  string                 `json:"content"`   // 消息内容（必需）
	Priority string                 `json:"priority"`  // 优先级（可选）
	Data     map[string]interface{} `json:"data"`      // 附加数据（可选）
}

// BroadcastRequest 广播请求
type BroadcastRequest struct {
	Type     gowsc.MessageType      `json:"type"`      // 消息类型（可选，默认notice）
	Content  string                 `json:"content"`   // 消息内容（必需）
	Priority string                 `json:"priority"`  // 优先级（可选）
	Data     map[string]interface{} `json:"data"`      // 附加数据（可选）
}

// APIResponse 通用API响应
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// handleSendMessage 处理发送消息API
// POST /api/wsc/send
// Body: { "to": "user123", "content": "Hello", "type": "text" }
func (api *WSCBuiltinAPI) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	// 只允许POST
	if r.Method != http.MethodPost {
		api.writeError(w, http.StatusMethodNotAllowed, "只支持POST请求")
		return
	}

	ctx := r.Context()

	// 认证检查
	if api.authRequired {
		userID, err := api.authenticate(ctx, r)
		if err != nil {
			api.writeError(w, http.StatusUnauthorized, "认证失败: "+err.Error())
			return
		}
		ctx = context.WithValue(ctx, gowsc.ContextKeyUserID, userID)
		ctx = context.WithValue(ctx, gowsc.ContextKeySenderID, userID)
	}

	// 解析请求
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.writeError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}

	// 验证参数
	if req.To == "" {
		api.writeError(w, http.StatusBadRequest, "缺少接收者ID")
		return
	}
	if req.Content == "" {
		api.writeError(w, http.StatusBadRequest, "缺少消息内容")
		return
	}

	// 构造消息
	msg := &HubMessage{
		Type:     req.Type,
		To:       req.To,
		Content:  req.Content,
		Data:     req.Data,
		CreateAt: time.Now(),
	}

	// 默认消息类型
	if msg.Type == "" {
		msg.Type = gowsc.MessageTypeText
	}

	// 发送消息
	if err := api.adapter.SendMessage(ctx, msg); err != nil {
		api.writeError(w, http.StatusInternalServerError, "发送失败: "+err.Error())
		return
	}

	api.writeSuccess(w, "消息已发送", map[string]interface{}{
		"to":   req.To,
		"type": msg.Type,
		"time": msg.CreateAt,
	})
}

// handleBroadcast 处理广播API
// POST /api/wsc/broadcast
// Body: { "content": "System Notice", "type": "notice" }
func (api *WSCBuiltinAPI) handleBroadcast(w http.ResponseWriter, r *http.Request) {
	// 只允许POST
	if r.Method != http.MethodPost {
		api.writeError(w, http.StatusMethodNotAllowed, "只支持POST请求")
		return
	}

	ctx := r.Context()

	// 认证检查（广播需要管理员权限）
	if api.authRequired || api.adminOnly {
		userID, err := api.authenticate(ctx, r)
		if err != nil {
			api.writeError(w, http.StatusUnauthorized, "认证失败: "+err.Error())
			return
		}

		// 检查是否为管理员
		if api.adminOnly && !api.isAdmin(ctx, r) {
			api.writeError(w, http.StatusForbidden, "需要管理员权限")
			return
		}

		ctx = context.WithValue(ctx, gowsc.ContextKeyUserID, userID)
	}

	// 解析请求
	var req BroadcastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.writeError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}

	// 验证参数
	if req.Content == "" {
		api.writeError(w, http.StatusBadRequest, "缺少消息内容")
		return
	}

	// 构造消息
	msg := &HubMessage{
		Type:     req.Type,
		Content:  req.Content,
		Data:     req.Data,
		CreateAt: time.Now(),
	}

	// 默认消息类型
	if msg.Type == "" {
		msg.Type = gowsc.MessageTypeNotice
	}

	// 广播消息
	api.adapter.Broadcast(ctx, msg)

	api.writeSuccess(w, "广播已发送", map[string]interface{}{
		"type": msg.Type,
		"time": msg.CreateAt,
	})
}

// handleOnlineUsers 处理在线用户API
// GET /api/wsc/online
func (api *WSCBuiltinAPI) handleOnlineUsers(w http.ResponseWriter, r *http.Request) {
	// 只允许GET
	if r.Method != http.MethodGet {
		api.writeError(w, http.StatusMethodNotAllowed, "只支持GET请求")
		return
	}

	ctx := r.Context()

	// 认证检查（可选）
	if api.authRequired {
		_, err := api.authenticate(ctx, r)
		if err != nil {
			api.writeError(w, http.StatusUnauthorized, "认证失败: "+err.Error())
			return
		}
	}

	// 获取在线用户
	users := api.adapter.GetOnlineUsers()

	api.writeSuccess(w, "获取成功", map[string]interface{}{
		"count": len(users),
		"users": users,
	})
}

// handleStats 处理统计信息API
// GET /api/wsc/stats
func (api *WSCBuiltinAPI) handleStats(w http.ResponseWriter, r *http.Request) {
	// 只允许GET
	if r.Method != http.MethodGet {
		api.writeError(w, http.StatusMethodNotAllowed, "只支持GET请求")
		return
	}

	ctx := r.Context()

	// 认证检查（可选）
	if api.authRequired {
		_, err := api.authenticate(ctx, r)
		if err != nil {
			api.writeError(w, http.StatusUnauthorized, "认证失败: "+err.Error())
			return
		}
	}

	// 获取统计信息
	stats := api.adapter.GetStats()

	api.writeSuccess(w, "获取成功", stats)
}

// ==================== 辅助方法 ====================

// authenticate 认证用户
func (api *WSCBuiltinAPI) authenticate(ctx context.Context, r *http.Request) (string, error) {
	// 从上下文获取
	if userID, ok := ctx.Value(gowsc.ContextKeyUserID).(string); ok && userID != "" {
		return userID, nil
	}

	// 从Header获取
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID, nil
	}

	// 从Token解析（简化版，实际应使用JWT等）
	if token := r.Header.Get("Authorization"); token != "" {
		// TODO: 解析JWT token获取userID
		// 这里简化处理，实际应该调用认证服务
		return "user_from_token", nil
	}

	return "", errors.ErrUserAuthNotFound
}

// isAdmin 检查是否为管理员
func (api *WSCBuiltinAPI) isAdmin(ctx context.Context, r *http.Request) bool {
	// 从上下文获取
	if role, ok := ctx.Value("role").(string); ok {
		return role == "admin" || role == string(gowsc.UserRoleAdmin)
	}

	// 从Header获取
	if role := r.Header.Get("X-User-Role"); role != "" {
		return role == "admin"
	}

	return false
}

// writeSuccess 写入成功响应
func (api *WSCBuiltinAPI) writeSuccess(w http.ResponseWriter, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	}

	json.NewEncoder(w).Encode(resp)
}

// writeError 写入错误响应
func (api *WSCBuiltinAPI) writeError(w http.ResponseWriter, statusCode int, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := APIResponse{
		Success: false,
		Error:   errMsg,
	}

	json.NewEncoder(w).Encode(resp)
}

