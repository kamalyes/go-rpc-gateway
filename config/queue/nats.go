/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-04-23 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-09-19 20:30:00
 * @FilePath: \go-rpc-gateway\config\queue\nats.go
 * @Description: NATS 消息队列配置
 *
 * 分层原则（对齐官方客户端参数面）：
 *   - 连接级字段与官方 nats.Options 一一对应（Timeout/ReconnectWait/ReconnectJitter/
 *     ReconnectBufSize/PingInterval/MaxPingsOut/FlushTimeout/DrainTimeout），
 *     由网关连接工厂直接接线；0 值表示不传该选项，走官方库默认
 *   - JetStream 流级字段与官方 nats.StreamConfig 的运维参数对应
 *     （Replicas/Storage/Retention/MaxAge/MaxBytes/DuplicatesWindow）：
 *     流名由业务侧决策（建流时自带），本配置只承载运维参数默认值，
 *     业务 AddStream 时按需取用
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package queue

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// JetStream 存储后端（对应官方 nats.StorageType）
const (
	NatsStorageFile   = "file"   // 文件存储（持久化，生产默认）
	NatsStorageMemory = "memory" // 内存存储（易失，低延迟场景）
)

// JetStream 保留策略（对应官方 nats.RetentionPolicy）
const (
	NatsRetentionLimits    = "limits"    // 基于限制保留（MaxAge/MaxBytes 约束）
	NatsRetentionInterest  = "interest"  // 基于兴趣保留（全部消费者确认后删除）
	NatsRetentionWorkQueue = "workqueue" // 工作队列（单个消费者确认即删除）
)

// Nats 结构体用于配置 NATS 相关参数
//
// NATS 是一款高性能、轻量级的云原生消息系统，常用于：
//   - 分布式服务间低延迟消息广播
//   - 事件驱动架构中的发布/订阅
//   - 启用 JetStream 后支持消息持久化、重放
type Nats struct {
	Enabled  bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`         // 是否启用
	URL      string `mapstructure:"url" yaml:"url" json:"url" validate:"required"` // NATS 服务器地址，如 nats://127.0.0.1:4222
	Name     string `mapstructure:"name" yaml:"name" json:"name"`                  // 客户端名称，用于服务端识别
	Username string `mapstructure:"username" yaml:"username" json:"username"`      // 用户名（可选）
	Password string `mapstructure:"password" yaml:"password" json:"password"`      // 密码（可选）
	Token    string `mapstructure:"token" yaml:"token" json:"token"`               // Token 鉴权（可选）

	// === 连接级（对齐官方 nats.Options，0 值走官方库默认） ===
	ConnectTimeout   int   `mapstructure:"connect-timeout" yaml:"connect-timeout" json:"connectTimeout" validate:"min=1"` // 连接超时（秒），官方默认 2s
	ReconnectWait    int   `mapstructure:"reconnect-wait" yaml:"reconnect-wait" json:"reconnectWait" validate:"min=1"`    // 重连等待时间（秒），官方默认 2s
	ReconnectJitter  int   `mapstructure:"reconnect-jitter" yaml:"reconnect-jitter" json:"reconnectJitter"`               // 重连抖动上限（秒，0=官方默认 100ms）
	ReconnectBufSize int64 `mapstructure:"reconnect-buf-size" yaml:"reconnect-buf-size" json:"reconnectBufSize"`          // 重连期间发布缓冲区上限（字节，0=官方默认 8MB）
	MaxReconnects    int   `mapstructure:"max-reconnects" yaml:"max-reconnects" json:"maxReconnects"`                     // 最大重连次数，-1 表示无限
	PingInterval     int   `mapstructure:"ping-interval" yaml:"ping-interval" json:"pingInterval"`                        // 心跳 ping 间隔（秒，0=官方默认 120s）
	MaxPingsOut      int   `mapstructure:"max-pings-out" yaml:"max-pings-out" json:"maxPingsOut"`                         // 未回应 ping 上限（0=官方默认 2）
	FlushTimeout     int   `mapstructure:"flush-timeout" yaml:"flush-timeout" json:"flushTimeout"`                        // Flush 等待超时（秒，0=官方默认 10s）
	DrainTimeout     int   `mapstructure:"drain-timeout" yaml:"drain-timeout" json:"drainTimeout"`                        // Drain 排空超时（秒，0=官方默认 30s）

	// === JetStream 流级运维参数（对齐官方 nats.StreamConfig；流名由业务建流时决策） ===
	JetStream        bool   `mapstructure:"jet-stream" yaml:"jet-stream" json:"jetStream"`                      // 是否启用 JetStream 持久化
	Replicas         int    `mapstructure:"replicas" yaml:"replicas" json:"replicas"`                           // 副本数（0=单副本；超出集群规模时建流报错）
	Storage          string `mapstructure:"storage" yaml:"storage" json:"storage"`                              // 存储后端：file|memory（空=file）
	Retention        string `mapstructure:"retention" yaml:"retention" json:"retention"`                        // 保留策略：limits|interest|workqueue（空=limits）
	MaxAge           int64  `mapstructure:"max-age" yaml:"max-age" json:"maxAge"`                               // 消息最大保留时长（秒，0=不限）
	MaxBytes         int64  `mapstructure:"max-bytes" yaml:"max-bytes" json:"maxBytes"`                         // 流最大体积（字节，0=不限）
	DuplicatesWindow int64  `mapstructure:"duplicates-window" yaml:"duplicates-window" json:"duplicatesWindow"` // Publish 去重窗口（秒，0=官方默认 120s）

	ModuleName      string `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`                  // 模块名称
	WorkerPoolSize  int    `mapstructure:"worker-pool-size" yaml:"worker-pool-size" json:"workerPoolSize"`    // 全局消费者 WorkerPool 大小（0 表示不初始化）
	WorkerQueueSize int    `mapstructure:"worker-queue-size" yaml:"worker-queue-size" json:"workerQueueSize"` // 全局消费者 WorkerPool 任务队列大小（0 表示不初始化）
}

// NewNats 创建一个新的 Nats 实例
func NewNats(opt *Nats) *Nats {
	var natsInstance *Nats

	internal.LockFunc(func() {
		natsInstance = opt
	})
	return natsInstance
}

// Clone 返回 Nats 配置的副本
func (n *Nats) Clone() internal.Configurable {
	var cloned Nats
	if err := syncx.DeepCopy(&cloned, n); err != nil {
		return &Nats{}
	}
	return &cloned
}

// Get 返回 Nats 配置
func (n *Nats) Get() interface{} {
	return n
}

// Set 更新 Nats 配置的字段
func (n *Nats) Set(data interface{}) {
	if configData, ok := data.(*Nats); ok {
		n.ModuleName = configData.ModuleName
		n.Enabled = configData.Enabled
		n.URL = configData.URL
		n.Name = configData.Name
		n.Username = configData.Username
		n.Password = configData.Password
		n.Token = configData.Token
		n.ConnectTimeout = configData.ConnectTimeout
		n.ReconnectWait = configData.ReconnectWait
		n.ReconnectJitter = configData.ReconnectJitter
		n.ReconnectBufSize = configData.ReconnectBufSize
		n.MaxReconnects = configData.MaxReconnects
		n.PingInterval = configData.PingInterval
		n.MaxPingsOut = configData.MaxPingsOut
		n.FlushTimeout = configData.FlushTimeout
		n.DrainTimeout = configData.DrainTimeout
		n.JetStream = configData.JetStream
		n.Replicas = configData.Replicas
		n.Storage = configData.Storage
		n.Retention = configData.Retention
		n.MaxAge = configData.MaxAge
		n.MaxBytes = configData.MaxBytes
		n.DuplicatesWindow = configData.DuplicatesWindow
		n.WorkerPoolSize = configData.WorkerPoolSize
		n.WorkerQueueSize = configData.WorkerQueueSize
	}
}

// Validate 验证 Nats 配置的有效性
func (n *Nats) Validate() error {
	return internal.ValidateStruct(n)
}

// DefaultNats 返回默认 NATS 配置（连接级 0 值项由官方库默认兜底）
func DefaultNats() Nats {
	return Nats{
		ModuleName:       "nats",
		Enabled:          false,
		URL:              "nats://127.0.0.1:4222",
		Name:             "go-config-nats-client",
		ConnectTimeout:   10, // 10 秒
		ReconnectWait:    2,  // 2 秒
		MaxReconnects:    10,
		JetStream:        false,
		Replicas:         1,
		Storage:          NatsStorageFile,
		Retention:        NatsRetentionLimits,
		MaxAge:           0, // 不限
		MaxBytes:         0, // 不限
		DuplicatesWindow: 0, // 官方默认 2 分钟
		WorkerPoolSize:   4,
		WorkerQueueSize:  100,
	}
}

// DefaultNatsPtr 返回默认 NATS 配置的指针，支持链式调用
func DefaultNatsPtr() *Nats {
	config := DefaultNats()
	return &config
}

// WithModuleName 设置模块名称
func (n *Nats) WithModuleName(moduleName string) *Nats {
	n.ModuleName = moduleName
	return n
}

// WithEnabled 设置是否启用NATS
func (n *Nats) WithEnabled(enabled bool) *Nats {
	n.Enabled = enabled
	return n
}

// IsEnabled 检查NATS启用状态
func (n *Nats) IsEnabled() bool {
	return n != nil && n.Enabled
}

// WithURL 设置 NATS 服务器地址
func (n *Nats) WithURL(url string) *Nats {
	n.URL = url
	return n
}

// WithName 设置客户端名称
func (n *Nats) WithName(name string) *Nats {
	n.Name = name
	return n
}

// WithUsername 设置用户名
func (n *Nats) WithUsername(username string) *Nats {
	n.Username = username
	return n
}

// WithPassword 设置密码
func (n *Nats) WithPassword(password string) *Nats {
	n.Password = password
	return n
}

// WithToken 设置 Token
func (n *Nats) WithToken(token string) *Nats {
	n.Token = token
	return n
}

// WithConnectTimeout 设置连接超时时间（秒）
func (n *Nats) WithConnectTimeout(seconds int) *Nats {
	n.ConnectTimeout = seconds
	return n
}

// WithReconnectWait 设置重连等待时间（秒）
func (n *Nats) WithReconnectWait(seconds int) *Nats {
	n.ReconnectWait = seconds
	return n
}

// WithReconnectJitter 设置重连抖动上限（秒，0=官方默认）
func (n *Nats) WithReconnectJitter(seconds int) *Nats {
	n.ReconnectJitter = seconds
	return n
}

// WithReconnectBufSize 设置重连期间发布缓冲区上限（字节，0=官方默认）
func (n *Nats) WithReconnectBufSize(bytes int64) *Nats {
	n.ReconnectBufSize = bytes
	return n
}

// WithMaxReconnects 设置最大重连次数
func (n *Nats) WithMaxReconnects(max int) *Nats {
	n.MaxReconnects = max
	return n
}

// WithPingInterval 设置心跳 ping 间隔（秒，0=官方默认）
func (n *Nats) WithPingInterval(seconds int) *Nats {
	n.PingInterval = seconds
	return n
}

// WithMaxPingsOut 设置未回应 ping 上限（0=官方默认）
func (n *Nats) WithMaxPingsOut(max int) *Nats {
	n.MaxPingsOut = max
	return n
}

// WithFlushTimeout 设置 Flush 等待超时（秒，0=官方默认）
func (n *Nats) WithFlushTimeout(seconds int) *Nats {
	n.FlushTimeout = seconds
	return n
}

// WithDrainTimeout 设置 Drain 排空超时（秒，0=官方默认）
func (n *Nats) WithDrainTimeout(seconds int) *Nats {
	n.DrainTimeout = seconds
	return n
}

// WithJetStream 设置是否启用 JetStream
func (n *Nats) WithJetStream(enabled bool) *Nats {
	n.JetStream = enabled
	return n
}

// WithReplicas 设置 JetStream 副本数
func (n *Nats) WithReplicas(replicas int) *Nats {
	n.Replicas = replicas
	return n
}

// WithStorage 设置 JetStream 存储后端（file|memory）
func (n *Nats) WithStorage(storage string) *Nats {
	n.Storage = storage
	return n
}

// WithRetention 设置 JetStream 保留策略（limits|interest|workqueue）
func (n *Nats) WithRetention(retention string) *Nats {
	n.Retention = retention
	return n
}

// WithMaxAge 设置 JetStream 消息最大保留时长（秒，0=不限）
func (n *Nats) WithMaxAge(seconds int64) *Nats {
	n.MaxAge = seconds
	return n
}

// WithMaxBytes 设置 JetStream 流最大体积（字节，0=不限）
func (n *Nats) WithMaxBytes(bytes int64) *Nats {
	n.MaxBytes = bytes
	return n
}

// WithDuplicatesWindow 设置 JetStream Publish 去重窗口（秒，0=官方默认）
func (n *Nats) WithDuplicatesWindow(seconds int64) *Nats {
	n.DuplicatesWindow = seconds
	return n
}

// WithWorkerPoolSize 设置全局消费者 WorkerPool 大小
func (n *Nats) WithWorkerPoolSize(size int) *Nats {
	n.WorkerPoolSize = size
	return n
}

// WithWorkerQueueSize 设置全局消费者 WorkerPool 任务队列大小
func (n *Nats) WithWorkerQueueSize(size int) *Nats {
	n.WorkerQueueSize = size
	return n
}
