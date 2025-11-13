/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 07:49:19
 * @FilePath: \go-rpc-gateway\cpool\mqtt\mqtt.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package mqtt

import (
	"time"

	pahoMqtt "github.com/eclipse/paho.mqtt.golang"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	gologger "github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/global"
)

// DefaultMqtt 创建默认的mqtt客户端
func DefaultMqtt(cfg *gwconfig.Gateway, log gologger.ILogger) *pahoMqtt.Client {
	global.LOGGER.Info("MQTT连接地址：" + cfg.Mqtt.Endpoint)
	opts := pahoMqtt.NewClientOptions().AddBroker(cfg.Mqtt.Endpoint).SetClientID(cfg.Mqtt.ClientID)
	// 设置mqtt协议版本 4是3.1.1，3是3.1
	opts.SetProtocolVersion(cfg.Mqtt.ProtocolVersion)
	// 客户端掉线服务端不清除session
	opts.SetCleanSession(cfg.Mqtt.CleanSession)
	// 设置断开后重新连接
	opts.SetAutoReconnect(cfg.Mqtt.AutoReconnect)
	// 保活时间
	opts.SetKeepAlive(time.Duration(cfg.Mqtt.KeepAlive) * time.Second)
	// 用户名和密码
	opts.SetUsername(cfg.Mqtt.Username)
	opts.SetPassword(cfg.Mqtt.Password)
	// 最大重连间隔
	opts.SetMaxReconnectInterval(time.Duration(cfg.Mqtt.MaxReconnectInterval) * time.Second)
	// 最大ping超时时间
	opts.SetPingTimeout(time.Duration(cfg.Mqtt.PingTimeout) * time.Second)
	// 最大写超时时间
	opts.SetWriteTimeout(time.Duration(cfg.Mqtt.WriteTimeout) * time.Second)
	// 最大连接超时时间
	opts.SetConnectTimeout(time.Duration(cfg.Mqtt.ConnectTimeout) * time.Second)
	// 设置遗言
	opts.SetWill(cfg.Mqtt.WillTopic, cfg.Mqtt.ClientID, 1, false)
	client := pahoMqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		global.LOGGER.ErrorKV("MQTT连接异常", "mqtt_error", token.Error())
	}
	return &client
}

// Mqtt 连接和订阅
func Mqtt(cfg *gwconfig.Gateway, log gologger.ILogger, onConn pahoMqtt.OnConnectHandler, onLost pahoMqtt.ConnectionLostHandler, reConn pahoMqtt.ReconnectHandler) *pahoMqtt.Client {
	global.LOGGER.Info("MQTT开始连接......")
	global.LOGGER.Info("MQTT连接地址：" + cfg.Mqtt.Endpoint)
	opts := pahoMqtt.NewClientOptions().AddBroker(cfg.Mqtt.Endpoint).SetClientID(cfg.Mqtt.ClientID)
	// 设置mqtt协议版本 4是3.1.1，3是3.1
	opts.SetProtocolVersion(cfg.Mqtt.ProtocolVersion)
	// 客户端掉线服务端不清除session
	opts.SetCleanSession(cfg.Mqtt.CleanSession)
	// 设置断开后重新连接
	opts.SetAutoReconnect(cfg.Mqtt.AutoReconnect)
	// 保活时间
	opts.SetKeepAlive(time.Duration(cfg.Mqtt.KeepAlive) * time.Second)
	// 用户名和密码
	opts.SetUsername(cfg.Mqtt.Username)
	opts.SetPassword(cfg.Mqtt.Password)
	// 最大重连间隔
	opts.SetMaxReconnectInterval(time.Duration(cfg.Mqtt.MaxReconnectInterval) * time.Second)
	// 最大ping超时时间
	opts.SetPingTimeout(time.Duration(cfg.Mqtt.PingTimeout) * time.Second)
	// 最大写超时时间
	opts.SetWriteTimeout(time.Duration(cfg.Mqtt.WriteTimeout) * time.Second)
	// 最大连接超时时间
	opts.SetConnectTimeout(time.Duration(cfg.Mqtt.ConnectTimeout) * time.Second)
	// 设置遗言
	opts.SetWill(cfg.Mqtt.WillTopic, cfg.Mqtt.ClientID, 1, false)
	if onConn != nil {
		opts.SetOnConnectHandler(onConn)
	}
	if onLost == nil {
		opts.SetConnectionLostHandler(onLostHandler)
	} else {
		opts.SetConnectionLostHandler(onLost)
	}
	// 断线重连
	if reConn == nil {
		opts.SetReconnectingHandler(reConnHandler)
	} else {
		opts.SetReconnectingHandler(reConn)
	}
	client := pahoMqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		global.LOGGER.ErrorKV("MQTT连接异常", "mqtt_error", token.Error())
	}
	return &client
}

// 连接断开
func onLostHandler(client pahoMqtt.Client, err error) {
	global.LOGGER.Info("MQTT连接已经断开")
}

// 断线重连后重新回调
func reConnHandler(client pahoMqtt.Client, options *pahoMqtt.ClientOptions) {
	global.LOGGER.Info("MQTT开始重新连接")
}
