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

import "github.com/kamalyes/go-rpc-gateway/config/internal"

// Mqtt 配置使用单 DSN，默认关闭；启用后必须使用 -tags mq 构建。
//
// DSN 格式：
//
//	tcp://[用户名:密码@]主机:端口[?参数=值&...]
//	ssl://[用户名:密码@]主机:端口[?参数=值&...]
//	ws://[用户名:密码@]主机:端口/路径[?参数=值&...]
//	wss://[用户名:密码@]主机:端口/路径[?参数=值&...]
//
// tls://、tcps:// 与 ssl:// 等价。必须显式给出端口，userinfo 按 URL 规则转义。
// Paho 不解析连接选项的查询参数，以下参数由网关摘出并调用对应的 ClientOptions setter；
// WebSocket 剩余的查询参数原样交给服务端，TCP/TLS 的未知参数报错。
//
//	client_id               客户端 ID，默认 go-config-mqtt-client
//	protocol_version        3=MQTT 3.1，4=MQTT 3.1.1（默认 4）
//	clean_session           默认 true；false 时须给出非空 client_id
//	auto_reconnect          默认 true
//	keep_alive              默认 60s，0 禁用，最大 65535s，只接受整秒
//	max_reconnect_interval  默认 5m，必须大于 0
//	ping_timeout            默认 10s，必须大于 0
//	write_timeout           默认 10s，0 禁用
//	connect_timeout         默认 30s，必须大于 0（同时约束拨号与 MQTT 握手）
//	will_topic              默认不启用遗言；主题不可带通配符
//	will_payload            默认使用 client_id
//	will_qos                0/1/2，默认 1
//	will_retained           默认 false
//
// 时长支持 30s、5m，也支持纯数字秒。布尔参数接受 true/false，裸参数视为 true。
// 查询值中的 &、+、# 等字符需 URL 转义。网关参数不可重复。
//
// 示例：
//
//	tcp://127.0.0.1:1883?client_id=my-service
//	ssl://gateway:mqtt_pwd@mqtt.example.com:8883?client_id=my-service&connect_timeout=10s
//	ws://gateway:mqtt_pwd@mqtt.example.com:8083/mqtt?client_id=my-service&keep_alive=60s
//	tcp://127.0.0.1:1883?client_id=my-service&clean_session=false&will_topic=status%2Fmy-service&will_payload=offline&will_qos=1
//
// 原 endpoint、username/password、client-id 及超时等独立字段已移除，迁移时写入 DSN。
type Mqtt struct {
	Enabled    bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	DSN        string `mapstructure:"dsn" yaml:"dsn" json:"dsn" validate:"required_if=Enabled true"`
	ModuleName string `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`
}

// NewMqtt 创建一个新的 Mqtt 实例。
func NewMqtt(opt *Mqtt) *Mqtt {
	return opt
}

// Clone 返回 Mqtt 配置的副本。
func (m *Mqtt) Clone() internal.Configurable {
	if m == nil {
		return &Mqtt{}
	}
	cloned := *m
	return &cloned
}

// Get 返回 Mqtt 配置。
func (m *Mqtt) Get() interface{} { return m }

// Set 更新 Mqtt 配置。
func (m *Mqtt) Set(data interface{}) {
	if cfg, ok := data.(*Mqtt); ok && cfg != nil {
		*m = *cfg
	}
}

// Validate 验证 MQTT 配置，详细 DSN 参数在建连前验证。
func (m *Mqtt) Validate() error {
	if !m.IsEnabled() {
		return nil
	}
	return internal.ValidateStruct(m)
}

// DefaultMqtt 返回默认关闭的 MQTT 配置。
func DefaultMqtt() Mqtt {
	return Mqtt{ModuleName: "mqtt", DSN: "tcp://127.0.0.1:1883?client_id=go-config-mqtt-client"}
}

// Default 返回默认 MQTT 配置指针。
func Default() *Mqtt {
	cfg := DefaultMqtt()
	return &cfg
}

// IsEnabled 检查 MQTT 启用状态。
func (m *Mqtt) IsEnabled() bool { return m != nil && m.Enabled }

// WithEnabled 设置是否启用 MQTT。
func (m *Mqtt) WithEnabled(enabled bool) *Mqtt { m.Enabled = enabled; return m }

// WithDSN 设置 MQTT 连接 DSN。
func (m *Mqtt) WithDSN(dsn string) *Mqtt { m.DSN = dsn; return m }

// WithModuleName 设置模块名称。
func (m *Mqtt) WithModuleName(name string) *Mqtt { m.ModuleName = name; return m }
