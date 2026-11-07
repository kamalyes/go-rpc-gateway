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
//
// ─── URL 格式 ───────────────────────────────────────────────────────────────
//
//	nats://[用户名:密码@]主机:端口              明文连接，端口默认 4222
//	nats://[Token@]主机:端口                    Token 鉴权（userinfo 只有用户名没有密码时按 Token 解释）
//	tls://[用户名:密码@]主机:端口               TLS 连接
//	多节点用逗号分隔，客户端按配置顺序尝试并在其中重连
//
// 认证信息写在 URL 里，不再有独立的 username/password/token 配置字段：
// 官方客户端的 connectProto 一旦发现 URL 带 userinfo，就完全忽略 Options 上的
// User/Password/Token（nats.go connectProto 的 if u != nil 分支），两处都配时
// URL 静默取胜——留着那三个字段只会制造「配了没生效」的排查成本
//
// ─── URL 示例 ───────────────────────────────────────────────────────────────
//
//	本地开发，无认证：
//	  nats://127.0.0.1:4222
//
//	用户名密码认证：
//	  nats://gateway:nats_pwd@nats.example.com:4222
//
//	Token 认证（只给用户名、不给密码即为 Token）：
//	  nats://s3cr3t_token@nats.example.com:4222
//
//	TLS：
//	  tls://gateway:nats_pwd@nats.example.com:4222
//
//	集群多节点（逗号分隔，客户端在这几个节点间重连）：
//	  nats://gateway:nats_pwd@nats-1:4222,nats://gateway:nats_pwd@nats-2:4222,nats://gateway:nats_pwd@nats-3:4222
//
// ─── 为什么超时项没有收进 URL ───────────────────────────────────────────────
//
// nats.go 不解析 URL 查询参数，连接级参数只能通过 nats.Option 函数传入
// （nats.Timeout / nats.ReconnectWait / nats.PingInterval 等），
// 所以下面这些字段保持独立配置项，由 cpool/nats 逐项接线
//
// [EN] Credentials live in the URL and there are no separate username/password/token
// fields any more: the official client's connectProto ignores the Options-level
// User/Password/Token entirely as soon as the URL carries userinfo, so a config that sets
// both has the URL win silently — keeping those fields only buys "I configured it and it
// did nothing" debugging. The timeout fields below stay as struct fields because nats.go
// parses no URL query parameters at all; connection-level settings can only arrive through
// nats.Option functions, which cpool/nats wires up one by one
type Nats struct {
	Enabled bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`         // 是否启用
	URL     string `mapstructure:"url" yaml:"url" json:"url" validate:"required"` // 连接 URL，可带认证信息与多地址，格式与示例见类型注释
	Name    string `mapstructure:"name" yaml:"name" json:"name"`                  // 客户端名称，用于服务端识别

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

	ModuleName string `mapstructure:"module-name" yaml:"module-name" json:"moduleName"` // 模块名称
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
