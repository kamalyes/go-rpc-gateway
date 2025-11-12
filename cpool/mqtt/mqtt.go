/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2024-08-09 09:22:00
 * @FilePath: \go-rpc-gateway\mqtt\mqtt.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package mqtt

import (
	"time"

	pahoMqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/kamalyes/go-rpc-gateway/global"
)

// DefaultMqtt 创建默认的mqtt客户端
func DefaultMqtt(clientId string) *pahoMqtt.Client {
	global.LOGGER.Info("MQTT开始连接......")
	config := global.GATEWAY.Mqtt
	global.LOGGER.Info("MQTT连接地址：" + config.Endpoint)
	opts := pahoMqtt.NewClientOptions().AddBroker(config.Endpoint).SetClientID(clientId)
	// 设置mqtt协议版本 4是3.1.1，3是3.1
	opts.SetProtocolVersion(config.ProtocolVersion)
	// 客户端掉线服务端不清除session
	opts.SetCleanSession(config.CleanSession)
	// 设置断开后重新连接
	opts.SetAutoReconnect(config.AutoReconnect)
	// 保活时间
	opts.SetKeepAlive(time.Duration(config.KeepAlive) * time.Second)
	// 用户名和密码
	opts.SetUsername(config.Username)
	opts.SetPassword(config.Password)
	// 最大重连间隔
	opts.SetMaxReconnectInterval(time.Duration(config.MaxReconnectInterval) * time.Second)
	// 最大ping超时时间
	opts.SetPingTimeout(time.Duration(config.PingTimeout) * time.Second)
	// 最大写超时时间
	opts.SetWriteTimeout(time.Duration(config.WriteTimeout) * time.Second)
	// 最大连接超时时间
	opts.SetConnectTimeout(time.Duration(config.ConnectTimeout) * time.Second)
	// 设置遗言
	opts.SetWill(config.WillTopic, clientId, 1, false)
	client := pahoMqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		global.LOGGER.ErrorKV("MQTT连接异常", "mqtt_error", token.Error())
	}
	return &client
}

// Mqtt 连接和订阅
func Mqtt(clientId string, onConn pahoMqtt.OnConnectHandler, onLost pahoMqtt.ConnectionLostHandler, reConn pahoMqtt.ReconnectHandler) *pahoMqtt.Client {
	global.LOGGER.Info("MQTT开始连接......")
	config := global.GATEWAY.Mqtt
	global.LOGGER.Info("MQTT连接地址：" + config.Endpoint)
	opts := pahoMqtt.NewClientOptions().AddBroker(config.Endpoint).SetClientID(clientId)
	// 设置mqtt协议版本 4是3.1.1，3是3.1
	opts.SetProtocolVersion(config.ProtocolVersion)
	// 客户端掉线服务端不清除session
	opts.SetCleanSession(config.CleanSession)
	// 设置断开后重新连接
	opts.SetAutoReconnect(config.AutoReconnect)
	// 保活时间
	opts.SetKeepAlive(time.Duration(config.KeepAlive) * time.Second)
	// 用户名和密码
	opts.SetUsername(config.Username)
	opts.SetPassword(config.Password)
	// 最大重连间隔
	opts.SetMaxReconnectInterval(time.Duration(config.MaxReconnectInterval) * time.Second)
	// 最大ping超时时间
	opts.SetPingTimeout(time.Duration(config.PingTimeout) * time.Second)
	// 最大写超时时间
	opts.SetWriteTimeout(time.Duration(config.WriteTimeout) * time.Second)
	// 最大连接超时时间
	opts.SetConnectTimeout(time.Duration(config.ConnectTimeout) * time.Second)
	// 设置遗言
	opts.SetWill(config.WillTopic, clientId, 1, false)
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
