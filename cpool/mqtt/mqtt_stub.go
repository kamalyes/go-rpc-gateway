//go:build !mq

/* Copyright (c) 2026 by kamalyes, All Rights Reserved. */

package mqtt

import (
	"context"
	"fmt"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/kamalyes/go-rpc-gateway/config/queue"
	"github.com/kamalyes/go-toolbox/pkg/logger"
)

// NewClient fails explicitly when MQTT is enabled without the mq build tag.
func NewClient(ctx context.Context, cfg *queue.Mqtt, log logger.ILogger) (paho.Client, error) {
	return connect(ctx, cfg, log, nil, nil, nil)
}

func connect(_ context.Context, cfg *queue.Mqtt, _ logger.ILogger, _ paho.OnConnectHandler, _ paho.ConnectionLostHandler, _ paho.ReconnectHandler) (paho.Client, error) {
	if !cfg.IsEnabled() {
		return nil, nil
	}
	return nil, fmt.Errorf("MQTT client not compiled in; rebuild with -tags mq to enable MQTT support")
}
