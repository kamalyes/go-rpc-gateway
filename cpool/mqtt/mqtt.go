package mqtt

import (
	"context"

	paho "github.com/eclipse/paho.mqtt.golang"
	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-toolbox/pkg/logger"
)

// DefaultMqtt keeps the legacy factory signature. Prefer NewClient for error handling.
func DefaultMqtt(ctx context.Context, cfg *gwconfig.Gateway, log logger.ILogger) *paho.Client {
	return Mqtt(ctx, cfg, log, nil, nil, nil)
}

// Mqtt connects with callbacks, returning nil on failure or when disabled.
// Prefer NewClient when the caller needs a connection error.
func Mqtt(ctx context.Context, cfg *gwconfig.Gateway, log logger.ILogger, onConn paho.OnConnectHandler, onLost paho.ConnectionLostHandler, reConn paho.ReconnectHandler) *paho.Client {
	if cfg == nil {
		return nil
	}
	client, err := connect(ctx, cfg.Mqtt, log, onConn, onLost, reConn)
	if err != nil {
		if log != nil {
			log.ErrorContextKV(ctx, "Failed to initialize MQTT", "error", err)
		}
		return nil
	}
	if client == nil {
		return nil
	}
	return &client
}
