/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 11:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 11:00:00
 * @FilePath: \go-rpc-gateway\config\kafka\consumer.go
 * @Description: Kafka 消费者配置
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package kafka

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Consumer Kafka 消费者配置
//
// 与 Producer 分开维护：消费者只关心「订阅哪个 Topic、属于哪个组、从哪个位点开始、
// 拉取批量与提交节奏」，不携带确认级别与压缩算法
//
// 消费模型二选一
//   - 配置 group-id：同一消费组内自动分配分区，partition 字段被忽略
//   - 不配置 group-id：必须指定 partition，直接消费固定分区
//
// [EN] Consumer is the Kafka consumer configuration, kept separate from Producer. Pick one
// consumption model: set group-id for group-based partition assignment (the partition field is
// then ignored), or leave group-id empty and set partition to consume a fixed partition.
type Consumer struct {
	// === 连接级 ===
	Brokers       []string `mapstructure:"brokers" yaml:"brokers" json:"brokers" validate:"required,min=1"`      // Broker 地址列表
	ClientID      string   `mapstructure:"client-id" yaml:"client-id" json:"clientId"`                           // 客户端标识，便于服务端审计
	DialTimeout   int      `mapstructure:"dial-timeout" yaml:"dial-timeout" json:"dialTimeout" validate:"min=0"` // 建连超时（秒，0=默认 5s）
	SASLMechanism string   `mapstructure:"sasl-mechanism" yaml:"sasl-mechanism" json:"saslMechanism"`            // SASL 机制：plain / scram-sha-256 / scram-sha-512
	Username      string   `mapstructure:"username" yaml:"username" json:"username"`                             // SASL 用户名
	Password      string   `mapstructure:"password" yaml:"password" json:"password"`                             // SASL 密码
	TLS           TLS      `mapstructure:"tls" yaml:"tls" json:"tls"`                                            // TLS 配置

	// === 消费 ===
	Topic          string `mapstructure:"topic" yaml:"topic" json:"topic" validate:"required"`                           // 订阅主题
	GroupID        string `mapstructure:"group-id" yaml:"group-id" json:"groupId"`                                       // 消费者组 ID
	Partition      int    `mapstructure:"partition" yaml:"partition" json:"partition" validate:"min=0"`                  // 固定分区，未配置 GroupID 时使用
	Offset         string `mapstructure:"offset" yaml:"offset" json:"offset"`                                            // 起始位点：earliest / latest
	MinBytes       int    `mapstructure:"min-bytes" yaml:"min-bytes" json:"minBytes" validate:"min=0"`                   // 单次拉取最小字节（0=默认 1）
	MaxBytes       int    `mapstructure:"max-bytes" yaml:"max-bytes" json:"maxBytes" validate:"min=0"`                   // 单次拉取最大字节（0=默认 10MB）
	MaxWait        int    `mapstructure:"max-wait" yaml:"max-wait" json:"maxWait" validate:"min=0"`                      // 单次拉取最长等待（毫秒，0=默认 500ms）
	CommitInterval int    `mapstructure:"commit-interval" yaml:"commit-interval" json:"commitInterval" validate:"min=0"` // 自动提交间隔（毫秒，0=手动提交）
	QueueCapacity  int    `mapstructure:"queue-capacity" yaml:"queue-capacity" json:"queueCapacity" validate:"min=0"`    // 拉取队列长度（0=默认 100）

	ModuleName string `mapstructure:"module-name" yaml:"module-name" json:"moduleName"` // 模块名称
}

// NewConsumer 创建一个新的消费者配置实例
func NewConsumer(opt *Consumer) *Consumer {
	var consumer *Consumer
	internal.LockFunc(func() {
		consumer = opt
	})
	return consumer
}

// Clone 返回消费者配置的副本
func (c *Consumer) Clone() internal.Configurable {
	var cloned Consumer
	if err := syncx.DeepCopy(&cloned, c); err != nil {
		return &Consumer{}
	}
	return &cloned
}

// Get 返回消费者配置的所有字段
func (c *Consumer) Get() interface{} {
	return c
}

// Set 更新消费者配置的字段
func (c *Consumer) Set(data interface{}) {
	if configData, ok := data.(*Consumer); ok {
		*c = *configData
	}
}

// Validate 校验消费者配置
func (c *Consumer) Validate() error {
	if err := internal.ValidateStruct(c); err != nil {
		return err
	}
	if err := validateConnection(c.Brokers, c.SASLMechanism, c.Username, c.Password, c.TLS); err != nil {
		return err
	}
	if strings.TrimSpace(c.Topic) == "" {
		return errors.New("Kafka 消费主题不能为空")
	}
	if c.GroupID == "" && c.Partition < 0 {
		return errors.New("Kafka 消费者必须配置 group-id 或 partition")
	}
	if c.Offset != "" && c.Offset != OffsetEarliest && c.Offset != OffsetLatest {
		return fmt.Errorf("Kafka 消费位点只支持 earliest 或 latest，当前为 %s", c.Offset)
	}
	if c.MinBytes > 0 && c.MaxBytes > 0 && c.MinBytes > c.MaxBytes {
		return fmt.Errorf("Kafka min-bytes(%d) 不能大于 max-bytes(%d)", c.MinBytes, c.MaxBytes)
	}
	if c.CommitInterval < 0 {
		return fmt.Errorf("Kafka commit-interval 不能为负数")
	}
	return nil
}

// BrokersList 返回清洗后的 broker 地址列表
func (c *Consumer) BrokersList() []string {
	return normalizeBrokers(c.Brokers)
}

// DefaultConsumerConfig 返回默认消费者配置
func DefaultConsumerConfig() Consumer {
	return Consumer{
		Brokers:        []string{"127.0.0.1:9092"},
		ClientID:       "go-rpc-gateway",
		DialTimeout:    5,
		SASLMechanism:  SASLMechanismPlain,
		Topic:          "default-topic",
		GroupID:        "default-group",
		Partition:      0,
		Offset:         OffsetLatest,
		MinBytes:       1,
		MaxBytes:       10 << 20,
		MaxWait:        500,
		CommitInterval: 1000,
		QueueCapacity:  100,
		ModuleName:     "kafka-consumer",
	}
}

// DefaultConsumer 返回默认消费者配置的指针，支持链式调用
func DefaultConsumer() *Consumer {
	config := DefaultConsumerConfig()
	return &config
}

// WithBrokers 设置 broker 地址列表
func (c *Consumer) WithBrokers(brokers ...string) *Consumer {
	c.Brokers = brokers
	return c
}

// WithClientID 设置客户端标识
func (c *Consumer) WithClientID(clientID string) *Consumer {
	c.ClientID = clientID
	return c
}

// WithDialTimeout 设置建连超时（秒）
func (c *Consumer) WithDialTimeout(seconds int) *Consumer {
	c.DialTimeout = seconds
	return c
}

// WithSASL 设置 SASL 鉴权
func (c *Consumer) WithSASL(mechanism, username, password string) *Consumer {
	c.SASLMechanism = mechanism
	c.Username = username
	c.Password = password
	return c
}

// WithTLS 设置 TLS 配置
func (c *Consumer) WithTLS(tlsConfig TLS) *Consumer {
	c.TLS = tlsConfig
	return c
}

// WithTopic 设置订阅主题
func (c *Consumer) WithTopic(topic string) *Consumer {
	c.Topic = topic
	return c
}

// WithGroupID 设置消费者组 ID
func (c *Consumer) WithGroupID(groupID string) *Consumer {
	c.GroupID = groupID
	return c
}

// WithPartition 设置固定分区
func (c *Consumer) WithPartition(partition int) *Consumer {
	c.Partition = partition
	return c
}

// WithOffset 设置起始位点
func (c *Consumer) WithOffset(offset string) *Consumer {
	c.Offset = offset
	return c
}

// WithFetch 设置单次拉取字节范围与最长等待（毫秒）
func (c *Consumer) WithFetch(minBytes, maxBytes, maxWaitMillis int) *Consumer {
	c.MinBytes = minBytes
	c.MaxBytes = maxBytes
	c.MaxWait = maxWaitMillis
	return c
}

// WithCommitInterval 设置自动提交间隔（毫秒，0=手动提交）
func (c *Consumer) WithCommitInterval(millis int) *Consumer {
	c.CommitInterval = millis
	return c
}

// WithQueueCapacity 设置拉取队列长度
func (c *Consumer) WithQueueCapacity(capacity int) *Consumer {
	c.QueueCapacity = capacity
	return c
}

// WithModuleName 设置模块名称
func (c *Consumer) WithModuleName(moduleName string) *Consumer {
	c.ModuleName = moduleName
	return c
}
