package examples

import (
	"context"
	"fmt"
	"log"
	"time"

	gateway "github.com/kamalyes/go-rpc-gateway"
	wsc "github.com/kamalyes/go-wsc"
)

// ============================================================================
// 示例 1: 最简单的使用方式 - 开箱即用
// ============================================================================

// SimpleWebSocketExample 最简单的 WebSocket 示例
// 配置文件中启用 WebSocket，其他一切自动完成
func SimpleWebSocketExample() error {
	// 创建 Gateway 并启动 - WebSocket 自动启动
	gw, err := gateway.NewGateway().
		WithConfigPath("./config/gateway.yaml").
		BuildAndStart()

	if err != nil {
		return fmt.Errorf("启动失败: %w", err)
	}

	// 优雅关闭
	return gw.WaitForShutdown()
}

// ============================================================================
// 示例 2: 链式回调 - 事件驱动能力
// ============================================================================

// AdvancedWebSocketExample 高级 WebSocket 使用示例
func AdvancedWebSocketExample() error {
	// 创建 Gateway
	gw, err := gateway.NewGateway().
		WithConfigPath("./config/gateway.yaml").
		Build()

	if err != nil {
		return fmt.Errorf("构建失败: %w", err)
	}

	// ===== 链式注册回调 =====
	gw.
		// 1. 客户端连接回调
		OnWebSocketClientConnect(func(ctx context.Context, client *wsc.Client) error {
			fmt.Printf("[CONNECT] 客户端已连接: ID=%s, UserID=%s\n",
				client.ID, client.UserID)
			return nil
		}).

		// 2. 消息接收回调
		OnWebSocketMessageReceived(func(ctx context.Context, client *wsc.Client, msg *wsc.HubMessage) error {
			fmt.Printf("[MESSAGE-RECV] 来自 %s 的消息: %s\n", client.ID, msg.Content)
			return nil
		}).

		// 3. 客户端断开连接回调
		OnWebSocketClientDisconnect(func(ctx context.Context, client *wsc.Client, reason string) error {
			fmt.Printf("[DISCONNECT] 客户端已断开: ID=%s, 原因=%s\n", client.ID, reason)
			return nil
		}).

		// 4. 错误处理回调
		OnWebSocketError(func(ctx context.Context, err error, severity string) error {
			fmt.Printf("[ERROR-%s] %v\n", severity, err)
			return nil
		})

	// 启动
	if err := gw.Start(); err != nil {
		return fmt.Errorf("启动失败: %w", err)
	}

	// 优雅关闭
	return gw.WaitForShutdown()
}

// ============================================================================
// 示例 3: 直接操作 Hub - 高级功能
// ============================================================================

// HubDirectAccessExample 直接访问 Hub 的示例
func HubDirectAccessExample() error {
	gw, err := gateway.NewGateway().
		WithConfigPath("./config/gateway.yaml").
		BuildAndStart()

	if err != nil {
		return fmt.Errorf("启动失败: %w", err)
	}

	// 获取 WebSocket 服务
	wsSvc := gw.GetWebSocketService()
	if wsSvc == nil || !wsSvc.IsRunning() {
		return fmt.Errorf("WebSocket 服务未启用")
	}

	// 启动后台任务（模拟消息推送）
	go func() {
		time.Sleep(2 * time.Second)

		// 广播消息给所有连接的客户端
		gw.BroadcastWebSocketMessage(context.Background(), &wsc.HubMessage{
			Type:     wsc.MessageTypeText,
			Content:  "欢迎使用 go-rpc-gateway WebSocket!",
			CreateAt: time.Now(),
		})

		// 5 秒后发送统计信息
		time.Sleep(3 * time.Second)

		stats := wsSvc.GetStats()
		fmt.Printf("\n📊 WebSocket 统计信息:\n")
		for key, value := range stats {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}()

	return gw.WaitForShutdown()
}

// ============================================================================
// 示例 4: 完整的实时通信应用
// ============================================================================

// ChatApplicationExample 完整的聊天应用示例
func ChatApplicationExample() error {
	gw, err := gateway.NewGateway().
		WithConfigPath("./config/gateway.yaml").
		Build()

	if err != nil {
		return fmt.Errorf("构建失败: %w", err)
	}

	gw.
		OnWebSocketClientConnect(func(ctx context.Context, client *wsc.Client) error {
			log.Printf("用户 %s 上线\n", client.UserID)

			// 通知其他用户
			gw.BroadcastWebSocketMessage(ctx, &wsc.HubMessage{
				Type:     wsc.MessageTypeText,
				From:     "SYSTEM",
				Content:  fmt.Sprintf("用户 %s 已上线", client.UserID),
				CreateAt: time.Now(),
			})

			return nil
		}).

		OnWebSocketMessageReceived(func(ctx context.Context, client *wsc.Client, msg *wsc.HubMessage) error {
			// 如果指定了接收者，进行点对点消息
			if msg.To != "" {
				return gw.SendToWebSocketUser(ctx, msg.To, msg)
			}

			// 否则广播
			gw.BroadcastWebSocketMessage(ctx, msg)
			return nil
		}).

		OnWebSocketClientDisconnect(func(ctx context.Context, client *wsc.Client, reason string) error {
			log.Printf("用户 %s 离线\n", client.UserID)

			// 通知其他用户
			gw.BroadcastWebSocketMessage(ctx, &wsc.HubMessage{
				Type:     wsc.MessageTypeText,
				From:     "SYSTEM",
				Content:  fmt.Sprintf("用户 %s 已离线", client.UserID),
				CreateAt: time.Now(),
			})

			return nil
		})

	// 启动
	if err := gw.Start(); err != nil {
		return fmt.Errorf("启动失败: %w", err)
	}

	return gw.WaitForShutdown()
}

// ============================================================================
// 示例 5: 消息推送 API 使用
// ============================================================================

// MessagePushExample 消息推送 API 使用示例
func MessagePushExample() error {
	gw, err := gateway.NewGateway().
		WithConfigPath("./config/gateway.yaml").
		BuildAndStart()

	if err != nil {
		return fmt.Errorf("启动失败: %w", err)
	}

	// 启动后台任务（模拟推送）
	go func() {
		time.Sleep(2 * time.Second)
		ctx := context.Background()

		// 1. 发送给特定用户
		msg := &wsc.HubMessage{
			Type:     wsc.MessageTypeText,
			From:     "admin",
			To:       "user123",
			Content:  "Hello from admin",
			CreateAt: time.Now(),
		}

		if err := gw.SendToWebSocketUser(ctx, "user123", msg); err != nil {
			log.Printf("Failed to send message: %v", err)
		} else {
			log.Printf("Message sent successfully")
		}

		// 2. 发送给特定用户（带 ACK）
		ack, err := gw.SendToWebSocketUserWithAck(ctx, "user123", msg, 5*time.Second, 3)
		if err != nil {
			log.Printf("Failed to send with ACK: %v", err)
		} else {
			log.Printf("Message delivered, ACK: %+v", ack)
		}

		// 3. 广播消息
		gw.BroadcastWebSocketMessage(ctx, &wsc.HubMessage{
			Type:     wsc.MessageTypeText,
			From:     "admin",
			Content:  "Server announcement",
			CreateAt: time.Now(),
		})

		// 4. 获取在线用户
		users := gw.GetWebSocketOnlineUsers()
		log.Printf("Online users: %v", users)

		// 5. 获取在线用户数
		count := gw.GetWebSocketOnlineUserCount()
		log.Printf("Online user count: %d", count)
	}()

	return gw.WaitForShutdown()
}

// ============================================================================
// 配置文件示例 (gateway.yaml)
// ============================================================================

/*
配置文件示例: ./config/gateway.yaml

gateway:
  name: "Go RPC Gateway with WebSocket"
  version: "1.0.0"
  environment: "development"
  enabled: true

  http:
    host: "0.0.0.0"
    port: 8080

  grpc:
    server:
      host: "0.0.0.0"
      port: 9090

  wsc:
    enabled: true
    node_ip: "0.0.0.0"
    node_port: 8081
    heartbeat_interval: 30
    client_timeout: 90
    message_buffer_size: 256
    websocket_origins:
      - "http://localhost:3000"
      - "http://localhost:5173"

    # 安全配置
    security:
      enable_auth: true
      enable_encryption: false
      enable_rate_limit: true
      max_message_size: 1024

    # 性能配置
    performance:
      max_connections_per_node: 10000
      read_buffer_size: 4
      write_buffer_size: 4
      enable_compression: false
      enable_metrics: true

    # 分布式配置
    distributed:
      enabled: false

    redis:
      enabled: false
*/
