/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 10:31:11
 * @FilePath: \go-rpc-gateway\config\queue\mqtt.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package queue

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Mqtt 结构体用于配置 MQTT 相关参数
type Mqtt struct {
	Endpoint             string `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint"        validate:"required,url"`                           // Mqtt 代理服务器端点地址
	ClientID             string `mapstructure:"client-id" yaml:"client-id" json:"clientId"`                                                        // 客户端标识符
	ProtocolVersion      uint   `mapstructure:"protocol-version" yaml:"protocol-version" json:"protocolVersion" validate:"required"`               // Mqtt 协议版本号，4 是 3.1.1，3 是 3.1
	KeepAlive            int    `mapstructure:"keep-alive" yaml:"keep-alive" json:"keepAlive"      validate:"required,min=1"`                      // 保活时间间隔，最小值为 1 秒
	MaxReconnectInterval int    `mapstructure:"max-reconnect-interval" yaml:"max-reconnect-interval" json:"maxReconnectInterval" validate:"min=1"` // 最大连接间隔时间，单位：秒，最小值为 1 秒
	PingTimeout          int    `mapstructure:"ping-timeout" yaml:"ping-timeout" json:"pingTimeout"      validate:"min=1"`                         // ping 超时时间，单位：秒，最小值为 1 秒
	WriteTimeout         int    `mapstructure:"write-timeout" yaml:"write-timeout" json:"writeTimeout"     validate:"min=1"`                       // 写超时时间，单位：秒，最小值为 1 秒
	ConnectTimeout       int    `mapstructure:"connect-timeout" yaml:"connect-timeout" json:"connectTimeout"   validate:"min=1"`                   // 连接超时时间，单位：秒，最小值为 1 秒
	Username             string `mapstructure:"username" yaml:"username" json:"username"`                                                          // 连接到代理服务器的用户名
	Password             string `mapstructure:"password" yaml:"password" json:"password"`                                                          // 密码
	CleanSession         bool   `mapstructure:"clean-session" yaml:"clean-session" json:"cleanSession"`                                            // 设置客户端掉线服务端是否清除 session
	AutoReconnect        bool   `mapstructure:"auto-reconnect" yaml:"auto-reconnect" json:"autoReconnect"`                                         // 断开后是否重新连接
	WillTopic            string `mapstructure:"will-topic" yaml:"will-topic" json:"willTopic"`                                                     // 遗言发送的 topic
	ModuleName           string `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`                                                  // 模块名称
}

// NewMqtt 创建一个新的 Mqtt 实例
func NewMqtt(opt *Mqtt) *Mqtt {
	var mqttInstance *Mqtt

	internal.LockFunc(func() {
		mqttInstance = opt
	})
	return mqttInstance
}

// Clone 返回 Mqtt 配置的副本
func (m *Mqtt) Clone() internal.Configurable {
	var cloned Mqtt
	if err := syncx.DeepCopy(&cloned, m); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Mqtt{}
	}
	return &cloned
}

// Get 返回 Mqtt 配置的所有字段
func (m *Mqtt) Get() interface{} {
	return m
}

// Set 更新 Mqtt 配置的字段
func (m *Mqtt) Set(data interface{}) {
	if configData, ok := data.(*Mqtt); ok {
		m.ModuleName = configData.ModuleName
		m.Endpoint = configData.Endpoint
		m.ClientID = configData.ClientID
		m.Username = configData.Username
		m.Password = configData.Password
		m.ProtocolVersion = configData.ProtocolVersion
		m.CleanSession = configData.CleanSession
		m.AutoReconnect = configData.AutoReconnect
		m.KeepAlive = configData.KeepAlive
		m.MaxReconnectInterval = configData.MaxReconnectInterval
		m.PingTimeout = configData.PingTimeout
		m.WriteTimeout = configData.WriteTimeout
		m.ConnectTimeout = configData.ConnectTimeout
		m.WillTopic = configData.WillTopic
	}
}

// Validate 验证 Mqtt 配置的有效性
func (m *Mqtt) Validate() error {
	return internal.ValidateStruct(m)
}

// DefaultMqtt 返回默认Mqtt配置
func DefaultMqtt() Mqtt {
	return Mqtt{
		ModuleName:           "mqtt",
		Endpoint:             "tcp://127.0.0.1:1883",
		ClientID:             "go-config-mqtt-client",
		ProtocolVersion:      4,   // MQTT 3.1.1
		KeepAlive:            60,  // 60秒
		MaxReconnectInterval: 300, // 5分钟
		PingTimeout:          10,  // 10秒
		WriteTimeout:         10,  // 10秒
		ConnectTimeout:       30,  // 30秒
		Username:             "mqtt_user",
		Password:             "mqtt_password",
		CleanSession:         true,
		AutoReconnect:        true,
		WillTopic:            "",
	}
}

// Default 返回默认Mqtt配置的指针，支持链式调用
func Default() *Mqtt {
	config := DefaultMqtt()
	return &config
}

// WithModuleName 设置模块名称
func (m *Mqtt) WithModuleName(moduleName string) *Mqtt {
	m.ModuleName = moduleName
	return m
}

// WithEndpoint 设置MQTT代理服务器端点地址
func (m *Mqtt) WithEndpoint(endpoint string) *Mqtt {
	m.Endpoint = endpoint
	return m
}

// WithClientID 设置客户端标识符
func (m *Mqtt) WithClientID(clientID string) *Mqtt {
	m.ClientID = clientID
	return m
}

// WithProtocolVersion 设置MQTT协议版本
func (m *Mqtt) WithProtocolVersion(protocolVersion uint) *Mqtt {
	m.ProtocolVersion = protocolVersion
	return m
}

// WithKeepAlive 设置保活时间间隔
func (m *Mqtt) WithKeepAlive(keepAlive int) *Mqtt {
	m.KeepAlive = keepAlive
	return m
}

// WithMaxReconnectInterval 设置最大重连间隔
func (m *Mqtt) WithMaxReconnectInterval(maxReconnectInterval int) *Mqtt {
	m.MaxReconnectInterval = maxReconnectInterval
	return m
}

// WithPingTimeout 设置ping超时时间
func (m *Mqtt) WithPingTimeout(pingTimeout int) *Mqtt {
	m.PingTimeout = pingTimeout
	return m
}

// WithWriteTimeout 设置写超时时间
func (m *Mqtt) WithWriteTimeout(writeTimeout int) *Mqtt {
	m.WriteTimeout = writeTimeout
	return m
}

// WithConnectTimeout 设置连接超时时间
func (m *Mqtt) WithConnectTimeout(connectTimeout int) *Mqtt {
	m.ConnectTimeout = connectTimeout
	return m
}

// WithUsername 设置用户名
func (m *Mqtt) WithUsername(username string) *Mqtt {
	m.Username = username
	return m
}

// WithPassword 设置密码
func (m *Mqtt) WithPassword(password string) *Mqtt {
	m.Password = password
	return m
}

// WithCleanSession 设置是否清除会话
func (m *Mqtt) WithCleanSession(cleanSession bool) *Mqtt {
	m.CleanSession = cleanSession
	return m
}

// WithAutoReconnect 设置是否自动重连
func (m *Mqtt) WithAutoReconnect(autoReconnect bool) *Mqtt {
	m.AutoReconnect = autoReconnect
	return m
}

// WithWillTopic 设置遗言主题
func (m *Mqtt) WithWillTopic(willTopic string) *Mqtt {
	m.WillTopic = willTopic
	return m
}
