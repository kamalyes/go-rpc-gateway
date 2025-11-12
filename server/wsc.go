package server

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/kamalyes/go-cachex"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/osx"
	"github.com/kamalyes/go-wsc"
)

// Client 客户端连接信息
type Client struct {
	ID       string                 // 客户端唯一标识
	UserID   string                 // 用户ID
	RoomID   string                 // 房间ID（工单ID）
	Role     string                 // 角色：customer, agent, admin
	WSClient *wsc.Wsc               // WebSocket客户端连接
	LastSeen time.Time              // 最后活跃时间
	Metadata map[string]interface{} // 额外元数据
}

// Hub WebSocket连接管理中心
type Hub struct {
	// 客户端管理
	clients      map[string]*Client   // 所有客户端连接 key: clientID
	userToClient map[string]*Client   // 用户ID到客户端的映射
	roomClients  map[string][]*Client // 房间客户端列表 key: roomID

	// 消息通道
	register   chan *Client  // 注册客户端
	unregister chan *Client  // 取消注册客户端
	broadcast  chan *Message // 广播消息

	// 并发控制
	mutex sync.RWMutex

	// 上下文控制
	ctx    context.Context
	cancel context.CancelFunc

	// 服务
	redisService cachex.CtxCache // Redis服务（可选）
}

// Message WebSocket消息结构
type Message struct {
	Type     string                 `json:"type"`
	From     string                 `json:"from"`
	To       string                 `json:"to"`
	RoomID   string                 `json:"room_id"`
	Content  string                 `json:"content"`
	Data     map[string]interface{} `json:"data,omitempty"`
	CreateAt time.Time              `json:"create_at"`
	MsgID    string                 `json:"msg_id"`
}

// NewHub 创建新的WebSocket Hub
func NewHub(redisService cachex.CtxCache) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		clients:      make(map[string]*Client),
		userToClient: make(map[string]*Client),
		roomClients:  make(map[string][]*Client),
		register:     make(chan *Client, 256),
		unregister:   make(chan *Client, 256),
		broadcast:    make(chan *Message, 1024),
		redisService: redisService,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Run 启动Hub
func (h *Hub) Run() {
	global.LOGGER.Info("WebSocket Hub 启动运行...")

	ticker := time.NewTicker(30 * time.Second) // 心跳检查间隔
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			log.Println("WebSocket Hub 正在关闭...")
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

	// 添加到房间
	if client.RoomID != "" {
		h.roomClients[client.RoomID] = append(h.roomClients[client.RoomID], client)
	}

	global.LOGGER.Info("客户端注册成功: ID=%s, UserID=%s, RoomID=%s, Role=%s",
		client.ID, client.UserID, client.RoomID, client.Role)

	// 发送欢迎消息
	welcomeMsg := &Message{
		Type:     "welcome",
		From:     "system",
		To:       client.UserID,
		Content:  "连接成功",
		Data:     map[string]interface{}{"client_id": client.ID, "status": "connected"},
		CreateAt: time.Now(),
		MsgID:    osx.HashUnixMicroCipherText(),
	}

	h.sendMessageToClient(client, welcomeMsg)
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

	// 从房间中删除
	if client.RoomID != "" {
		if clients, exists := h.roomClients[client.RoomID]; exists {
			for i, c := range clients {
				if c.ID == client.ID {
					h.roomClients[client.RoomID] = append(clients[:i], clients[i+1:]...)
					break
				}
			}
			// 如果房间为空，删除房间
			if len(h.roomClients[client.RoomID]) == 0 {
				delete(h.roomClients, client.RoomID)
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
	case message.RoomID != "": // 房间消息
		if clients, exists := h.roomClients[message.RoomID]; exists {
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

// SendToRoom 发送消息到房间
func (h *Hub) SendToRoom(roomID string, message *Message) {
	message.RoomID = roomID
	if message.MsgID == "" {
		message.MsgID = osx.HashUnixMicroCipherText()
	}
	if message.CreateAt.IsZero() {
		message.CreateAt = time.Now()
	}

	select {
	case h.broadcast <- message:
	default:
		global.LOGGER.Info("发送消息失败，广播通道已满: 目标房间 %s", roomID)
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

// GetRoomClients 获取房间中的客户端
func (h *Hub) GetRoomClients(roomID string) []*Client {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if clients, exists := h.roomClients[roomID]; exists {
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
	log.Println("正在关闭 WebSocket Hub...")

	h.cancel() // 停止Run循环

	h.mutex.Lock()
	defer h.mutex.Unlock()

	// 关闭所有客户端连接
	for _, client := range h.clients {
		client.WSClient.Close()
	}

	log.Println("WebSocket Hub 已关闭")
}

// NewClient 创建新的WebSocket客户端
func NewClient(userID, role, roomID, wsURL string) (*Client, error) {
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
		ID:       clientID,
		UserID:   userID,
		Role:     role,
		RoomID:   roomID,
		WSClient: wsClient,
		LastSeen: time.Now(),
		Metadata: make(map[string]interface{}),
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
