package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/kamalyes/go-cachex"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/osx"
	"github.com/kamalyes/go-wsc"
)

// Client 客户端连接信息
type Client struct {
	ID         string                 // 客户端唯一标识
	UserID     string                 // 用户ID
	UserType   wsc.UserType           // 用户类型：customer, agent, admin, bot, vip
	TicketID   string                 // 当前工单ID
	Role       wsc.UserRole           // 角色：customer, agent, admin
	WSClient   *wsc.Wsc               // WebSocket连接
	LastSeen   time.Time              // 最后活跃时间
	Status     wsc.UserStatus         // 状态：online, away, busy, offline, hidden
	Department wsc.Department         // 部门（客服专用）
	Skills     []wsc.Skill            // 技能标签（客服专用）
	MaxTickets int                    // 最大并发工单数（客服专用）
	NodeID     string                 // 所在节点ID（分布式支持）
	ClientType wsc.ClientType         // 客户端类型：web, mobile, desktop, api
	Metadata   map[string]interface{} // 扩展元数据
}

// NodeInfo 节点信息
type NodeInfo struct {
	ID          string         `json:"id"`          // 节点ID
	IPAddress   string         `json:"ip_address"`  // IP地址
	Port        int            `json:"port"`        // 端口
	Status      wsc.NodeStatus `json:"status"`      // 状态：active, inactive, starting, stopping
	LoadScore   float64        `json:"load_score"`  // 负载分数
	LastSeen    time.Time      `json:"last_seen"`   // 最后活跃时间
	Connections int            `json:"connections"` // 连接数
}

// Hub WebSocket连接管理中心（分布式版本）
type Hub struct {
	// 节点信息
	nodeID   string               // 当前节点ID
	nodeInfo *NodeInfo            // 当前节点信息
	nodes    map[string]*NodeInfo // 所有节点信息

	// 客户端管理
	clients       map[string]*Client   // 所有客户端连接 key: clientID
	userToClient  map[string]*Client   // 用户ID到客户端的映射
	agentClients  map[string]*Client   // 客服连接映射
	ticketClients map[string][]*Client // 工单相关客户端 key: ticketID

	// 消息通道
	register    chan *Client                 // 注册客户端
	unregister  chan *Client                 // 取消注册客户端
	broadcast   chan *Message                // 广播消息
	nodeMessage chan *wsc.DistributedMessage // 节点间消息通道

	// 分布式支持
	redisService cachex.CtxCache // Redis服务
	pubsubClient cachex.CtxCache // Redis发布订阅客户端

	// 欢迎消息提供者
	welcomeProvider wsc.WelcomeMessageProvider // 欢迎消息提供者接口

	// 统计信息
	stats struct {
		TotalConnections  int64     // 总连接数
		ActiveConnections int       // 活跃连接数
		MessagesSent      int64     // 发送消息数
		MessagesReceived  int64     // 接收消息数
		LastStatsUpdate   time.Time // 最后更新时间
	}

	// 并发控制
	mutex sync.RWMutex

	// 上下文控制
	ctx    context.Context
	cancel context.CancelFunc
}

// Message WebSocket消息结构
type Message struct {
	Type         wsc.MessageType        `json:"type"`                      // 消息类型: text, image, file, system, typing, read_receipt
	From         string                 `json:"from"`                      // 发送者ID
	To           string                 `json:"to"`                        // 接收者ID
	TicketID     string                 `json:"ticket_id"`                 // 工单ID
	Content      string                 `json:"content"`                   // 消息内容
	Data         map[string]interface{} `json:"data,omitempty"`            // 扩展数据
	CreateAt     time.Time              `json:"create_at"`                 // 创建时间
	MsgID        string                 `json:"msg_id"`                    // 消息ID
	SeqNo        int64                  `json:"seq_no"`                    // 消息序列号
	Priority     wsc.Priority           `json:"priority"`                  // 优先级
	ReplyToMsgID string                 `json:"reply_to_msg_id,omitempty"` // 回复的消息ID
	Status       wsc.MessageStatus      `json:"status"`                    // 消息状态: sent, delivered, read, failed
}

// NewHub 创建新的分布式WebSocket Hub
func NewHub(redisService cachex.CtxCache, nodeIP string, nodePort int, welcomeProvider wsc.WelcomeMessageProvider) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	nodeID := fmt.Sprintf("node-%s-%d-%d", nodeIP, nodePort, time.Now().Unix())

	hub := &Hub{
		nodeID: nodeID,
		nodeInfo: &NodeInfo{
			ID:          nodeID,
			IPAddress:   nodeIP,
			Port:        nodePort,
			Status:      wsc.NodeStatusActive,
			LastSeen:    time.Now(),
			Connections: 0,
		},
		nodes:           make(map[string]*NodeInfo),
		clients:         make(map[string]*Client),
		userToClient:    make(map[string]*Client),
		agentClients:    make(map[string]*Client),
		ticketClients:   make(map[string][]*Client),
		register:        make(chan *Client, 256),
		unregister:      make(chan *Client, 256),
		broadcast:       make(chan *Message, 1024),
		nodeMessage:     make(chan *wsc.DistributedMessage, 1024),
		redisService:    redisService,
		pubsubClient:    redisService, // 使用相同的Redis连接
		welcomeProvider: welcomeProvider,
		ctx:             ctx,
		cancel:          cancel,
	}

	// 启动分布式相关服务
	go hub.startNodeDiscovery()
	go hub.startPubSubListener()
	go hub.startNodeHeartbeat()

	go hub.startNodeHeartbeat()

	return hub
}

// GetNodeInfo 获取当前节点信息
func (h *Hub) GetNodeInfo() *NodeInfo {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// 更新当前连接数
	nodeInfo := *h.nodeInfo
	nodeInfo.Connections = len(h.clients)
	nodeInfo.LastSeen = time.Now()
	nodeInfo.LoadScore = h.calculateLoadScore()

	return &nodeInfo
}

// GetAllNodes 获取所有节点信息
func (h *Hub) GetAllNodes() map[string]*NodeInfo {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	result := make(map[string]*NodeInfo)
	for id, node := range h.nodes {
		result[id] = node
	}

	return result
}

// SendToOtherNodes 发送消息到其他节点
func (h *Hub) SendToOtherNodes(message *Message) {
	distMsg := &wsc.DistributedMessage{
		Type:      wsc.OperationTypeMessage,
		NodeID:    h.nodeID,
		TargetID:  message.To,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"message": message,
		},
	}

	// 发送到节点消息通道
	select {
	case h.nodeMessage <- distMsg:
	default:
		global.LOGGER.Info("节点消息通道已满，无法发送跨节点消息")
	}
}

// IsUserOnCurrentNode 检查用户是否在当前节点
func (h *Hub) IsUserOnCurrentNode(userID string) bool {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	_, exists := h.userToClient[userID]
	return exists
}

// GetNodeStats 获取节点统计信息
func (h *Hub) GetNodeStats() map[string]interface{} {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return map[string]interface{}{
		"node_id":            h.nodeID,
		"total_connections":  h.stats.TotalConnections,
		"active_connections": h.stats.ActiveConnections,
		"active_tickets":     len(h.ticketClients),
		"online_agents":      len(h.agentClients),
		"messages_sent":      h.stats.MessagesSent,
		"messages_received":  h.stats.MessagesReceived,
		"last_stats_update":  h.stats.LastStatsUpdate,
		"load_score":         h.calculateLoadScore(),
		"node_status":        h.nodeInfo.Status,
		"connected_nodes":    len(h.nodes),
	}
}

// Shutdown 关闭Hub（增强版）

// startNodeDiscovery 启动节点发现服务
func (h *Hub) startNodeDiscovery() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.registerNode()
			h.discoverNodes()
		}
	}
}

// startPubSubListener 启动Redis发布订阅监听
func (h *Hub) startPubSubListener() {
	// Redis发布订阅监听逻辑
	// 这里需要实现Redis pub/sub监听
	for {
		select {
		case <-h.ctx.Done():
			return
		case msg := <-h.nodeMessage:
			h.handleNodeMessage(msg)
		}
	}
}

// startNodeHeartbeat 启动节点心跳
func (h *Hub) startNodeHeartbeat() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.sendHeartbeat()
		}
	}
}

// registerNode 注册当前节点到Redis
func (h *Hub) registerNode() {
	h.mutex.Lock()
	h.nodeInfo.LastSeen = time.Now()
	h.nodeInfo.Connections = len(h.clients)
	h.nodeInfo.LoadScore = h.calculateLoadScore()
	nodeData, _ := json.Marshal(h.nodeInfo)
	h.mutex.Unlock()

	// 注册节点信息到Redis
	key := fmt.Sprintf("ws:nodes:%s", h.nodeID)
	h.redisService.Set(context.Background(), []byte(key), nodeData)
}

// discoverNodes 发现其他节点
func (h *Hub) discoverNodes() {
	// 简化实现，实际中需要实现Redis扫描逻辑
	// 这里只是占位符，避免编译错误
}

// handleNodeMessage 处理节点间消息
func (h *Hub) handleNodeMessage(msg *wsc.DistributedMessage) {
	switch msg.Type {
	case wsc.OperationTypeMessage:
		// 处理跨节点用户消息
		if messageData, ok := msg.Data["message"]; ok {
			if message, ok := messageData.(*Message); ok {
				h.handleCrossNodeMessage(message)
			}
		}
	case wsc.OperationTypeSync:
		// 处理节点同步
		h.handleNodeSync(msg)
	}
}

// handleCrossNodeMessage 处理跨节点消息
func (h *Hub) handleCrossNodeMessage(message *Message) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// 检查目标用户是否在当前节点
	if client, exists := h.userToClient[message.To]; exists {
		h.sendMessageToClient(client, message)
	}
}

// handleNodeSync 处理节点同步
func (h *Hub) handleNodeSync(msg *wsc.DistributedMessage) {
	// 实现节点同步逻辑
}

// sendHeartbeat 发送心跳
func (h *Hub) sendHeartbeat() {
	heartbeat := &wsc.DistributedMessage{
		Type:      wsc.OperationTypeHeartbeat,
		NodeID:    h.nodeID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"connections": len(h.clients),
			"load_score":  h.calculateLoadScore(),
		},
	}

	// 发送到本地消息通道
	select {
	case h.nodeMessage <- heartbeat:
	default:
		// 通道满了，跳过这次心跳
	}
}

// calculateLoadScore 计算负载分数
func (h *Hub) calculateLoadScore() float64 {
	connections := len(h.clients)
	// 简单的负载计算：连接数/1000
	return float64(connections) / 1000.0
}

// Run 启动Hub
func (h *Hub) Run() {
	global.LOGGER.Info("WebSocket Hub 启动运行...")

	ticker := time.NewTicker(30 * time.Second) // 心跳检查间隔
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			global.LOGGER.Info("WebSocket Hub 正在关闭...")
			return

		case client := <-h.register:
			h.handleRegister(client)
		case client := <-h.unregister:
			h.handleUnregister(client)
		case message := <-h.broadcast:
			h.handleBroadcast(message)
		case <-ticker.C:
			h.checkHeartbeat()
		}
	}
}

// handleRegister 处理客户端注册
func (h *Hub) handleRegister(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// 如果用户已经有连接，关闭旧连接
	if existingClient, exists := h.userToClient[client.UserID]; exists {
		global.LOGGER.Info("用户 %s 已存在连接，关闭旧连接", client.UserID)
		existingClient.WSClient.Close()
		h.removeClientUnsafe(existingClient)
	}

	// 添加新客户端
	h.clients[client.ID] = client
	h.userToClient[client.UserID] = client

	// 如果是客服，添加到客服映射
	if client.UserType == wsc.UserTypeAgent || client.UserType == wsc.UserTypeBot {
		h.agentClients[client.UserID] = client
	}

	// 添加到工单映射
	if client.TicketID != "" {
		h.ticketClients[client.TicketID] = append(h.ticketClients[client.TicketID], client)
	}

	// 更新统计信息
	h.stats.TotalConnections++
	h.stats.ActiveConnections = len(h.clients)
	h.stats.LastStatsUpdate = time.Now()

	global.LOGGER.Info("客户端注册成功: ID=%s, UserID=%s, TicketID=%s, Role=%s",
		client.ID, client.UserID, client.TicketID, client.Role)

	// 发送动态欢迎消息
	h.SendWelcomeMessage(client)
}

// handleUnregister 处理客户端注销
func (h *Hub) handleUnregister(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.removeClientUnsafe(client)
}

// removeClientUnsafe 移除客户端（非线程安全，需要在锁内调用）
func (h *Hub) removeClientUnsafe(client *Client) {
	// 从客户端映射中删除
	delete(h.clients, client.ID)
	delete(h.userToClient, client.UserID)

	// 如果是客服，从客服映射中删除
	if client.UserType == wsc.UserTypeAgent || client.UserType == wsc.UserTypeBot {
		delete(h.agentClients, client.UserID)
	}

	// 从工单映射中删除
	if client.TicketID != "" {
		if clients, exists := h.ticketClients[client.TicketID]; exists {
			for i, c := range clients {
				if c.ID == client.ID {
					h.ticketClients[client.TicketID] = append(clients[:i], clients[i+1:]...)
					break
				}
			}
			// 如果工单无客户端，删除工单映射
			if len(h.ticketClients[client.TicketID]) == 0 {
				delete(h.ticketClients, client.TicketID)
			}
		}
	}

	global.LOGGER.Info("客户端注销成功: ID=%s, UserID=%s", client.ID, client.UserID)
}

// handleBroadcast 处理消息广播
func (h *Hub) handleBroadcast(message *Message) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	switch {
	case message.To != "": // 点对点消息
		if client, exists := h.userToClient[message.To]; exists {
			h.sendMessageToClient(client, message)
		} else {
			global.LOGGER.Info("用户 %s 不在线，无法发送消息", message.To)
		}
	case message.TicketID != "": // 工单消息
		if clients, exists := h.ticketClients[message.TicketID]; exists {
			for _, client := range clients {
				// 不发送给自己
				if client.UserID == message.From {
					continue
				}
				h.sendMessageToClient(client, message)
			}
		}
	default: // 广播给所有客户端
		for _, client := range h.clients {
			h.sendMessageToClient(client, message)
		}
	}
}

// sendMessageToClient 发送消息到特定客户端
func (h *Hub) sendMessageToClient(client *Client, message *Message) {
	messageData, err := json.Marshal(message)
	if err != nil {
		global.LOGGER.Info("序列化消息失败: %v", err)
		return
	}

	if err := client.WSClient.SendTextMessage(string(messageData)); err != nil {
		global.LOGGER.Info("发送消息失败，客户端 %s: %v", client.ID, err)
		// 连接可能已断开，移除客户端
		h.unregister <- client
	}
}

// checkHeartbeat 检查客户端心跳
func (h *Hub) checkHeartbeat() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	now := time.Now()
	for clientID, client := range h.clients {
		if now.Sub(client.LastSeen) > 90*time.Second { // 90秒无心跳则断开
			global.LOGGER.Info("客户端 %s 心跳超时，断开连接", clientID)
			client.WSClient.Close()
			delete(h.clients, clientID)
			h.removeClientUnsafe(client)
		}
	}
}

// SendToUser 发送消息给特定用户
func (h *Hub) SendToUser(userID string, message *Message) {
	message.To = userID
	if message.MsgID == "" {
		message.MsgID = osx.HashUnixMicroCipherText()
	}
	if message.CreateAt.IsZero() {
		message.CreateAt = time.Now()
	}

	select {
	case h.broadcast <- message:
	default:
		global.LOGGER.Info("发送消息失败，广播通道已满: 目标用户 %s", userID)
	}
}

// SendToTicket 发送消息到工单
func (h *Hub) SendToTicket(ticketID string, message *Message) {
	message.TicketID = ticketID
	if message.MsgID == "" {
		message.MsgID = osx.HashUnixMicroCipherText()
	}
	if message.CreateAt.IsZero() {
		message.CreateAt = time.Now()
	}

	select {
	case h.broadcast <- message:
	default:
		global.LOGGER.Info("发送消息失败，广播通道已满: 目标工单 %s", ticketID)
	}
}

// Broadcast 广播消息给所有客户端
func (h *Hub) Broadcast(message *Message) {
	if message.MsgID == "" {
		message.MsgID = osx.HashUnixMicroCipherText()
	}
	if message.CreateAt.IsZero() {
		message.CreateAt = time.Now()
	}

	select {
	case h.broadcast <- message:
	default:
		global.LOGGER.Info("发送消息失败，广播通道已满")
	}
}

// GetOnlineUsers 获取在线用户列表
func (h *Hub) GetOnlineUsers() map[string]*Client {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	result := make(map[string]*Client)
	for userID, client := range h.userToClient {
		result[userID] = client
	}
	return result
}

// GetTicketClients 获取工单中的客户端
func (h *Hub) GetTicketClients(ticketID string) []*Client {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if clients, exists := h.ticketClients[ticketID]; exists {
		// 返回副本以避免并发问题
		result := make([]*Client, len(clients))
		copy(result, clients)
		return result
	}
	return nil
}

// GetClientCount 获取连接数统计
func (h *Hub) GetClientCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.clients)
}

// Shutdown 关闭Hub
func (h *Hub) Shutdown() {
	global.LOGGER.Info("正在关闭 WebSocket Hub...")

	h.cancel() // 停止Run循环

	h.mutex.Lock()
	defer h.mutex.Unlock()

	// 关闭所有客户端连接
	for _, client := range h.clients {
		client.WSClient.Close()
	}

	global.LOGGER.Info("WebSocket Hub 已关闭")
}

// NewClient 创建新的WebSocket客户端
func NewClient(userID string, userType wsc.UserType, role wsc.UserRole, ticketID, wsURL string) (*Client, error) {
	clientID := osx.HashUnixMicroCipherText()

	// 创建WebSocket连接
	wsClient := wsc.New(wsURL)

	// 配置WebSocket客户端
	config := wsc.NewDefaultConfig()
	config.AutoReconnect = true
	config.MinRecTime = 2 * time.Second
	config.MaxRecTime = 30 * time.Second
	config.MessageBufferSize = 1024
	wsClient.SetConfig(config)

	client := &Client{
		ID:         clientID,
		UserID:     userID,
		UserType:   userType,
		Role:       role,
		TicketID:   ticketID,
		WSClient:   wsClient,
		LastSeen:   time.Now(),
		Status:     wsc.UserStatusOnline,
		ClientType: wsc.ClientTypeWeb, // 默认为Web客户端
		Metadata:   make(map[string]interface{}),
	}

	// 设置回调函数
	wsClient.OnConnected(func() {
		global.LOGGER.Info("客户端 %s 连接成功", clientID)
		client.LastSeen = time.Now()
	})

	wsClient.OnDisconnected(func(err error) {
		global.LOGGER.Info("客户端 %s 连接断开: %v", clientID, err)
	})

	wsClient.OnTextMessageReceived(func(message string) {
		global.LOGGER.Info("客户端 %s 收到消息: %s", clientID, message)
		client.LastSeen = time.Now()
		// 这里可以添加消息处理逻辑
	})

	wsClient.OnConnectError(func(err error) {
		global.LOGGER.Info("客户端 %s 连接错误: %v", clientID, err)
	})

	return client, nil
}

// ConnectClient 连接客户端到WebSocket
func (c *Client) Connect() error {
	c.WSClient.Connect()
	return nil
}

// SendWelcomeMessage 发送欢迎消息
func (h *Hub) SendWelcomeMessage(client *Client) {
	if h.welcomeProvider == nil {
		return
	}

	// 准备扩展数据
	extraData := map[string]interface{}{
		"client_id":      client.ID,
		"time":           time.Now().Format("2006-01-02 15:04:05"),
		"active_tickets": h.getActiveTicketsCount(),
		"online_users":   len(h.clients),
		"node_id":        h.nodeID,
	}

	// 从提供者获取欢迎消息
	welcomeMsg, enabled, err := h.welcomeProvider.GetWelcomeMessage(
		client.UserID,
		client.Role,
		client.UserType,
		client.TicketID,
		extraData,
	)

	if err != nil {
		global.LOGGER.Error("获取欢迎消息失败: %v", err)
		return
	}

	if !enabled || welcomeMsg == nil {
		return
	}

	// 创建消息对象
	message := &Message{
		Type:     welcomeMsg.MessageType,
		From:     "system",
		To:       client.UserID,
		TicketID: client.TicketID,
		Content:  welcomeMsg.Content,
		Data:     welcomeMsg.Data,
		CreateAt: time.Now(),
		MsgID:    osx.HashUnixMicroCipherText(),
		Status:   wsc.MessageStatusSent,
		Priority: welcomeMsg.Priority,
	}

	// 添加标题到Data中
	if message.Data == nil {
		message.Data = make(map[string]interface{})
	}
	message.Data["title"] = welcomeMsg.Title
	// 发送消息给客户端
	h.sendMessageToClient(client, message)
}

// getActiveTicketsCount 获取活跃工单数量
func (h *Hub) getActiveTicketsCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return len(h.ticketClients)
}
