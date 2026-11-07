/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 13:26:00
 * @FilePath: \go-rpc-gateway\config\kafka\kafka.go
 * @Description:
 *
 * Copyright (j) 2024 by kamalyes, All Rights Reserved.
 */
package kafka

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Kafka 结构体
type Kafka struct {
	Brokers     string `mapstructure:"brokers" yaml:"brokers" json:"brokers"           validate:"required"`           // Kafka brokers地址
	Topic       string `mapstructure:"topic" yaml:"topic" json:"topic"             validate:"required"`               // Kafka主题
	GroupID     string `mapstructure:"group-id" yaml:"group-id" json:"groupId"`                                       // 消费者组ID
	Username    string `mapstructure:"username" yaml:"username" json:"username"`                                      // SASL用户名
	Password    string `mapstructure:"password" yaml:"password" json:"password"`                                      // SASL密码
	Partition   int    `mapstructure:"partition" yaml:"partition" json:"partition"`                                   // 分区号
	Offset      string `mapstructure:"offset" yaml:"offset" json:"offset"`                                            // 偏移量策略(earliest/latest)
	TryTimes    int    `mapstructure:"try-times" yaml:"try-times" json:"tryTimes"         validate:"required,min=1"`  // 尝试次数
	SyncESTopic string `mapstructure:"sync-es-topic" yaml:"sync-es-topic" json:"syncEsTopic"     validate:"required"` // 同步到 ES 的主题
	ModuleName  string `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`                              // 模块名称
}

// NewKafka 创建一个新的 Kafka 实例
func NewKafka(opt *Kafka) *Kafka {
	var kafkaInstance *Kafka

	internal.LockFunc(func() {
		kafkaInstance = opt
	})
	return kafkaInstance
}

// Clone 返回 Kafka 配置的副本
func (k *Kafka) Clone() internal.Configurable {
	var cloned Kafka
	if err := syncx.DeepCopy(&cloned, k); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Kafka{}
	}
	return &cloned
}

// Get 返回 Kafka 配置的所有字段
func (k *Kafka) Get() interface{} {
	return k
}

// Set 更新 Kafka 配置的字段
func (k *Kafka) Set(data interface{}) {
	if configData, ok := data.(*Kafka); ok {
		k.Brokers = configData.Brokers
		k.Topic = configData.Topic
		k.GroupID = configData.GroupID
		k.Username = configData.Username
		k.Password = configData.Password
		k.Partition = configData.Partition
		k.Offset = configData.Offset
		k.TryTimes = configData.TryTimes
		k.SyncESTopic = configData.SyncESTopic
		k.ModuleName = configData.ModuleName
	}
}

// Validate 检查 Kafka 配置的有效性
func (k *Kafka) Validate() error {
	return internal.ValidateStruct(k)
}

// DefaultKafka 返回默认Kafka配置
func DefaultKafka() Kafka {
	return Kafka{
		ModuleName:  "kafka",
		Brokers:     "127.0.0.1:9092",
		Topic:       "default-topic",
		GroupID:     "default-group",
		Username:    "kafka_user",
		Password:    "kafka_password",
		Partition:   0,
		Offset:      "latest",
		TryTimes:    3,
		SyncESTopic: "sync-es-topic",
	}
}

// Default 返回默认Kafka配置的指针，支持链式调用
func Default() *Kafka {
	config := DefaultKafka()
	return &config
}

// WithModuleName 设置模块名称
func (k *Kafka) WithModuleName(moduleName string) *Kafka {
	k.ModuleName = moduleName
	return k
}

// WithBrokers 设置Kafka brokers地址
func (k *Kafka) WithBrokers(brokers string) *Kafka {
	k.Brokers = brokers
	return k
}

// WithTryTimes 设置尝试次数
func (k *Kafka) WithTryTimes(tryTimes int) *Kafka {
	k.TryTimes = tryTimes
	return k
}

// WithSyncESTopic 设置同步ES的主题
func (k *Kafka) WithSyncESTopic(syncESTopic string) *Kafka {
	k.SyncESTopic = syncESTopic
	return k
}

// WithTopic 设置Kafka主题
func (k *Kafka) WithTopic(topic string) *Kafka {
	k.Topic = topic
	return k
}

// WithGroupID 设置消费者组ID
func (k *Kafka) WithGroupID(groupID string) *Kafka {
	k.GroupID = groupID
	return k
}

// WithUsername 设置SASL用户名
func (k *Kafka) WithUsername(username string) *Kafka {
	k.Username = username
	return k
}

// WithPassword 设置SASL密码
func (k *Kafka) WithPassword(password string) *Kafka {
	k.Password = password
	return k
}

// WithPartition 设置分区号
func (k *Kafka) WithPartition(partition int) *Kafka {
	k.Partition = partition
	return k
}

// WithOffset 设置偏移量策略
func (k *Kafka) WithOffset(offset string) *Kafka {
	k.Offset = offset
	return k
}
