/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 11:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 11:00:00
 * @FilePath: \go-rpc-gateway\config\kafka\producer.go
 * @Description: Kafka 生产者配置
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package kafka

import (
	"fmt"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Producer Kafka 生产者配置
//
// 与 Kafka（网关既有配置）分开维护：生产者只关心「连接、确认级别、压缩与攒批」，
// 不绑定具体 Topic——lm-api 与 channel-service 都存在一条连接发往多个 Topic 的场景，
// 目标 Topic 由 Send 调用显式传入
//
// 连接参数（Brokers/SASL/TLS）与消费者各自定义一份，保持两个结构都能独立使用；
// 公共校验放在 common.go，字段变化时两处同步
//
// [EN] Producer is the Kafka producer configuration, kept separate from the gateway's legacy
// Kafka struct. It deliberately binds no topic: both lm-api and channel-service publish to
// multiple topics over one connection, so the target topic is passed to Send explicitly.
// Connection fields are defined on both Producer and Consumer so either can be used
// standalone; shared validation lives in common.go.
type Producer struct {
	// === 连接级 ===
	Brokers       []string `mapstructure:"brokers" yaml:"brokers" json:"brokers" validate:"required,min=1"`      // Broker 地址列表
	ClientID      string   `mapstructure:"client-id" yaml:"client-id" json:"clientId"`                           // 客户端标识，便于服务端审计
	DialTimeout   int      `mapstructure:"dial-timeout" yaml:"dial-timeout" json:"dialTimeout" validate:"min=0"` // 建连超时（秒，0=默认 5s）
	SASLMechanism string   `mapstructure:"sasl-mechanism" yaml:"sasl-mechanism" json:"saslMechanism"`            // SASL 机制：plain / scram-sha-256 / scram-sha-512
	Username      string   `mapstructure:"username" yaml:"username" json:"username"`                             // SASL 用户名
	Password      string   `mapstructure:"password" yaml:"password" json:"password"`                             // SASL 密码
	TLS           TLS      `mapstructure:"tls" yaml:"tls" json:"tls"`                                            // TLS 配置

	// === 生产 ===
	RequiredAcks int    `mapstructure:"required-acks" yaml:"required-acks" json:"requiredAcks"`                  // 确认级别：0 不等待 / 1 主副本 / -1 全部副本
	Compression  string `mapstructure:"compression" yaml:"compression" json:"compression"`                       // 压缩算法：none / gzip / snappy / lz4 / zstd
	BatchSize    int    `mapstructure:"batch-size" yaml:"batch-size" json:"batchSize" validate:"min=0"`          // 批量条数（0=默认 100）
	BatchTimeout int    `mapstructure:"batch-timeout" yaml:"batch-timeout" json:"batchTimeout" validate:"min=0"` // 批量等待时间（毫秒，0=默认 100ms）
	WriteTimeout int    `mapstructure:"write-timeout" yaml:"write-timeout" json:"writeTimeout" validate:"min=0"` // 单次写入超时（秒，0=默认 10s）
	Async        bool   `mapstructure:"async" yaml:"async" json:"async"`                                         // 异步发送，吞吐更高但错误不直接返回

	ModuleName string `mapstructure:"module-name" yaml:"module-name" json:"moduleName"` // 模块名称
}

// NewProducer 创建一个新的生产者配置实例
func NewProducer(opt *Producer) *Producer {
	var producer *Producer
	internal.LockFunc(func() {
		producer = opt
	})
	return producer
}

// Clone 返回生产者配置的副本
func (p *Producer) Clone() internal.Configurable {
	var cloned Producer
	if err := syncx.DeepCopy(&cloned, p); err != nil {
		return &Producer{}
	}
	return &cloned
}

// Get 返回生产者配置的所有字段
func (p *Producer) Get() interface{} {
	return p
}

// Set 更新生产者配置的字段
func (p *Producer) Set(data interface{}) {
	if configData, ok := data.(*Producer); ok {
		*p = *configData
	}
}

// Validate 校验生产者配置
func (p *Producer) Validate() error {
	if err := internal.ValidateStruct(p); err != nil {
		return err
	}
	if err := validateConnection(p.Brokers, p.SASLMechanism, p.Username, p.Password, p.TLS); err != nil {
		return err
	}
	if p.RequiredAcks != 0 && p.RequiredAcks != 1 && p.RequiredAcks != -1 {
		return fmt.Errorf("Kafka required-acks 只支持 0、1、-1，当前为 %d", p.RequiredAcks)
	}
	if !isSupportedCompression(p.Compression) {
		return fmt.Errorf("Kafka 压缩算法不支持: %s", p.Compression)
	}
	return nil
}

// BrokersList 返回清洗后的 broker 地址列表
func (p *Producer) BrokersList() []string {
	return normalizeBrokers(p.Brokers)
}

// DefaultProducerConfig 返回默认生产者配置
func DefaultProducerConfig() Producer {
	return Producer{
		Brokers:       []string{"127.0.0.1:9092"},
		ClientID:      "go-rpc-gateway",
		DialTimeout:   5,
		SASLMechanism: SASLMechanismPlain,
		RequiredAcks:  -1,
		Compression:   CompressionNone,
		BatchSize:     100,
		BatchTimeout:  100,
		WriteTimeout:  10,
		ModuleName:    "kafka-producer",
	}
}

// DefaultProducer 返回默认生产者配置的指针，支持链式调用
func DefaultProducer() *Producer {
	config := DefaultProducerConfig()
	return &config
}

// WithBrokers 设置 broker 地址列表
func (p *Producer) WithBrokers(brokers ...string) *Producer {
	p.Brokers = brokers
	return p
}

// WithClientID 设置客户端标识
func (p *Producer) WithClientID(clientID string) *Producer {
	p.ClientID = clientID
	return p
}

// WithDialTimeout 设置建连超时（秒）
func (p *Producer) WithDialTimeout(seconds int) *Producer {
	p.DialTimeout = seconds
	return p
}

// WithSASL 设置 SASL 鉴权
func (p *Producer) WithSASL(mechanism, username, password string) *Producer {
	p.SASLMechanism = mechanism
	p.Username = username
	p.Password = password
	return p
}

// WithTLS 设置 TLS 配置
func (p *Producer) WithTLS(tlsConfig TLS) *Producer {
	p.TLS = tlsConfig
	return p
}

// WithRequiredAcks 设置确认级别
func (p *Producer) WithRequiredAcks(acks int) *Producer {
	p.RequiredAcks = acks
	return p
}

// WithCompression 设置压缩算法
func (p *Producer) WithCompression(compression string) *Producer {
	p.Compression = compression
	return p
}

// WithBatch 设置批量条数与等待时间（毫秒）
func (p *Producer) WithBatch(size, timeoutMillis int) *Producer {
	p.BatchSize = size
	p.BatchTimeout = timeoutMillis
	return p
}

// WithWriteTimeout 设置单次写入超时（秒）
func (p *Producer) WithWriteTimeout(seconds int) *Producer {
	p.WriteTimeout = seconds
	return p
}

// WithAsync 设置异步发送
func (p *Producer) WithAsync(async bool) *Producer {
	p.Async = async
	return p
}

// WithModuleName 设置模块名称
func (p *Producer) WithModuleName(moduleName string) *Producer {
	p.ModuleName = moduleName
	return p
}
