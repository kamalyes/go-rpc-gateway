//go:build mq

/* Copyright (c) 2024 by kamalyes, All Rights Reserved. */

package mqtt

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/kamalyes/go-rpc-gateway/config/queue"
	"github.com/kamalyes/go-rpc-gateway/cpool/internal/dsnx"
	"github.com/kamalyes/go-toolbox/pkg/logger"
)

// buildOptions removes gateway parameters before handing the broker URL to Paho.
// A WebSocket handshake must not receive MQTT options as URL parameters.
func buildOptions(dsn string) (*paho.ClientOptions, error) {
	keys := []string{"client_id", "protocol_version", "clean_session", "auto_reconnect",
		"keep_alive", "max_reconnect_interval", "ping_timeout", "write_timeout", "connect_timeout",
		"will_topic", "will_payload", "will_qos", "will_retained"}
	u, ext, err := dsnx.Split(dsn, keys...)
	if err != nil {
		return nil, fmt.Errorf("invalid MQTT DSN URL") // Parse errors can contain credentials.
	}
	if u.Hostname() == "" || u.Port() == "" || u.Fragment != "" || u.Opaque != "" {
		return nil, fmt.Errorf("MQTT DSN requires a host and explicit port, without a fragment")
	}
	websocket := u.Scheme == "ws" || u.Scheme == "wss"
	switch u.Scheme {
	case "tcp", "ssl", "tls", "tcps", "ws", "wss":
	default:
		return nil, fmt.Errorf("unsupported MQTT DSN scheme")
	}
	// URL.Query silently discards malformed escapes, so validate the original query.
	original, _ := url.Parse(dsn)
	query, err := url.ParseQuery(original.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("invalid MQTT DSN query encoding")
	}
	for _, key := range keys {
		if len(query[key]) > 1 {
			return nil, fmt.Errorf("duplicate MQTT DSN parameter: %s", key)
		}
	}
	if !websocket && (u.RawQuery != "" || (u.Path != "" && u.Path != "/")) {
		return nil, fmt.Errorf("MQTT TCP/TLS DSN contains an unknown parameter or path")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid MQTT port")
	}

	opts := paho.NewClientOptions()
	clientID := ext.String("client_id", "go-config-mqtt-client")
	if ext.Has("client_id") {
		clientID = query.Get("client_id") // Preserve an explicitly empty ID.
	}
	protocol := ext.Int("protocol_version", 4)
	clean := ext.Bool("clean_session", true)
	autoReconnect := ext.Bool("auto_reconnect", true)
	keepAlive := ext.Duration("keep_alive", 60*time.Second)
	reconnect := ext.Duration("max_reconnect_interval", 5*time.Minute)
	ping := ext.Duration("ping_timeout", 10*time.Second)
	write := ext.Duration("write_timeout", 10*time.Second)
	connect := ext.Duration("connect_timeout", 30*time.Second)
	willTopic := ext.String("will_topic", "")
	willPayload := clientID
	if ext.Has("will_payload") {
		willPayload = query.Get("will_payload")
	}
	willQos := ext.Int("will_qos", 1)
	willRetained := ext.Bool("will_retained", false)
	if err := ext.Err(); err != nil {
		return nil, fmt.Errorf("invalid MQTT DSN parameter value")
	}
	if protocol != 3 && protocol != 4 {
		return nil, fmt.Errorf("MQTT protocol_version must be 3 or 4")
	}
	if !clean && clientID == "" {
		return nil, fmt.Errorf("MQTT clean_session=false requires a client_id")
	}
	if keepAlive < 0 || keepAlive > 65535*time.Second || keepAlive%time.Second != 0 || reconnect <= 0 || ping <= 0 || write < 0 || connect <= 0 {
		return nil, fmt.Errorf("invalid MQTT timeout or keep_alive range")
	}
	if willQos < 0 || willQos > 2 || strings.ContainsAny(willTopic, "+#\x00") {
		return nil, fmt.Errorf("invalid MQTT will_topic or will_qos")
	}
	if willTopic == "" && (ext.Has("will_payload") || ext.Has("will_qos") || ext.Has("will_retained")) {
		return nil, fmt.Errorf("MQTT will options require will_topic")
	}
	opts.SetClientID(clientID).SetProtocolVersion(uint(protocol)).SetCleanSession(clean).
		SetAutoReconnect(autoReconnect).SetKeepAlive(keepAlive).SetMaxReconnectInterval(reconnect).
		SetPingTimeout(ping).SetWriteTimeout(write).SetConnectTimeout(connect).
		SetDialer(&net.Dialer{Timeout: connect})
	if u.User != nil {
		opts.SetUsername(u.User.Username())
		password, _ := u.User.Password()
		opts.SetPassword(password)
		u.User = nil // Keep credentials out of transport errors and broker logs.
	}
	if willTopic != "" {
		opts.SetWill(willTopic, willPayload, byte(willQos), willRetained)
	}
	opts.AddBroker(u.String())
	return opts, nil
}

// NewClient returns a connected client, or an error; disabled configurations return nil.
func NewClient(ctx context.Context, cfg *queue.Mqtt, log logger.ILogger) (paho.Client, error) {
	return connect(ctx, cfg, log, nil, nil, nil)
}

func connect(ctx context.Context, cfg *queue.Mqtt, log logger.ILogger, onConn paho.OnConnectHandler, onLost paho.ConnectionLostHandler, reConn paho.ReconnectHandler) (paho.Client, error) {
	if !cfg.IsEnabled() {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	opts, err := buildOptions(cfg.DSN)
	if err != nil {
		return nil, err
	}
	if onConn != nil {
		opts.SetOnConnectHandler(onConn)
	}
	if onLost != nil {
		opts.SetConnectionLostHandler(onLost)
	} else if log != nil {
		opts.SetConnectionLostHandler(func(_ paho.Client, err error) {
			log.WarnContextKV(context.Background(), "MQTT connection lost", "error", err)
		})
	}
	if reConn != nil {
		opts.SetReconnectingHandler(reConn)
	} else if log != nil {
		opts.SetReconnectingHandler(func(_ paho.Client, _ *paho.ClientOptions) {
			log.InfoContext(context.Background(), "MQTT reconnecting")
		})
	}
	client := paho.NewClient(opts)
	token := client.Connect()
	select {
	case <-ctx.Done():
		// Disconnect also stops an in-flight connection or reconnect attempt.
		client.Disconnect(0)
		return nil, ctx.Err()
	case <-token.Done():
		if err := token.Error(); err != nil {
			client.Disconnect(0)
			return nil, fmt.Errorf("connect MQTT: %w", err)
		}
		if err := ctx.Err(); err != nil {
			client.Disconnect(0)
			return nil, err
		}
		return client, nil
	}
}
