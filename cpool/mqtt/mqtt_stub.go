//go:build !mq

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 07:49:19
 * @FilePath: \go-rpc-gateway\cpool\mqtt\mqtt_stub.go
 * @Description: MQTT 客户端工厂空实现
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// mqtt_stub 在未启用 mq 构建标签时提供 DefaultMqtt / Mqtt 的空实现，
// paho.mqtt.golang 的客户端创建逻辑不可达，由链接器裁剪协议实现
// 需要 MQTT 支持时使用 -tags mq 构建

package mqtt

import (
	"context"

	pahoMqtt "github.com/eclipse/paho.mqtt.golang"
	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	"github.com/kamalyes/go-rpc-gateway/global"
	gologger "github.com/kamalyes/go-toolbox/pkg/logger"
)

// DefaultMqtt MQTT 客户端未编译进二进制
// 需要时使用 -tags mq 重新构建以启用 MQTT 支持
func DefaultMqtt(ctx context.Context, cfg *gwconfig.Gateway, log gologger.ILogger) *pahoMqtt.Client {
	global.LOGGER.WarnContext(ctx, "MQTT client not compiled in; rebuild with -tags mq to enable MQTT support")
	return nil
}

// Mqtt MQTT 客户端未编译进二进制
// 需要时使用 -tags mq 重新构建以启用 MQTT 支持
func Mqtt(ctx context.Context, cfg *gwconfig.Gateway, log gologger.ILogger, onConn pahoMqtt.OnConnectHandler, onLost pahoMqtt.ConnectionLostHandler, reConn pahoMqtt.ReconnectHandler) *pahoMqtt.Client {
	global.LOGGER.WarnContext(ctx, "MQTT client not compiled in; rebuild with -tags mq to enable MQTT support")
	return nil
}
