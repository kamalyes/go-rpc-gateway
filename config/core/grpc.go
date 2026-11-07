/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-13 11:53:15
 * @FilePath: \go-rpc-gateway\config\core\grpc.go
 * @Description: GRPC配置
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package core

import (
	"fmt"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// GRPC GRPC服务配置（包含服务端和多个客户端）
type GRPC struct {
	Server  *GRPCServer            `mapstructure:"server" yaml:"server" json:"server"`    // GRPC服务端配置
	Clients map[string]*GRPCClient `mapstructure:"clients" yaml:"clients" json:"clients"` // GRPC客户端配置（key为服务名）
}

// GRPCCompressType GRPC压缩算法类型
type GRPCCompressType string

const (
	GRPCCompressGzip   GRPCCompressType = "gzip"   // Gzip压缩
	GRPCCompressSnappy GRPCCompressType = "snappy" // Snappy压缩（高速度）
	GRPCCompressZstd   GRPCCompressType = "zstd"   // Zstandard压缩（高压缩率）
)

// String 实现 Stringer 接口，返回压缩算法类型的字符串表示
func (c GRPCCompressType) String() string {
	return string(c)
}

// GRPCServer GRPC服务端配置
type GRPCServer struct {
	Enable             bool             `mapstructure:"enable" yaml:"enable" json:"enable"`                                         // 是否启用GRPC服务
	Host               string           `mapstructure:"host" yaml:"host" json:"host"`                                               // 主机地址
	Port               int              `mapstructure:"port" yaml:"port" json:"port"`                                               // 端口
	Network            string           `mapstructure:"network" yaml:"network" json:"network"`                                      // 网络类型 (tcp, tcp4, tcp6, unix)
	MaxRecvMsgSize     int              `mapstructure:"max-recv-msg-size" yaml:"max-recv-msg-size" json:"maxRecvMsgSize"`           // 最大接收消息大小(字节)
	MaxSendMsgSize     int              `mapstructure:"max-send-msg-size" yaml:"max-send-msg-size" json:"maxSendMsgSize"`           // 最大发送消息大小(字节)
	KeepaliveTime      int              `mapstructure:"keepalive-time" yaml:"keepalive-time" json:"keepaliveTime"`                  // Keepalive时间(秒)
	KeepaliveTimeout   int              `mapstructure:"keepalive-timeout" yaml:"keepalive-timeout" json:"keepaliveTimeout"`         // Keepalive超时(秒)
	ConnectionTimeout  int              `mapstructure:"connection-timeout" yaml:"connection-timeout" json:"connectionTimeout"`      // 连接超时(秒)
	EnableReflection   bool             `mapstructure:"enable-reflection" yaml:"enable-reflection" json:"enableReflection"`         // 是否启用反射
	EnableProtobufResp bool             `mapstructure:"enable-protobuf-resp" yaml:"enable-protobuf-resp" json:"enableProtobufResp"` // 是否启用protobuf格式响应（grpc-gateway场景下默认返回JSON，开启后返回原始protobuf）
	EnableCompression  bool             `mapstructure:"enable-compression" yaml:"enable-compression" json:"enableCompression"`      // 是否启用压缩
	CompressionType    GRPCCompressType `mapstructure:"compression-type" yaml:"compression-type" json:"compressionType"`            // 压缩算法类型 (gzip, snappy, zstd)
	CompressionLevel   int              `mapstructure:"compression-level" yaml:"compression-level" json:"compressionLevel"`         // 压缩级别 (gzip: 1-9, zstd: 1-22, snappy: 忽略此值)
	MinCompressSize    int              `mapstructure:"min-compress-size" yaml:"min-compress-size" json:"minCompressSize"`          // 最小压缩大小(字节)，小于此大小不压缩
	Endpoint           string           `mapstructure:"-" yaml:"-" json:""`                                                         // 完整的服务端点地址（自动计算）
}

// GRPCClient GRPC客户端配置
type GRPCClient struct {
	ServiceName           string           `mapstructure:"service-name" yaml:"service-name" json:"serviceName"`                                   // 服务名称
	Endpoints             []string         `mapstructure:"endpoints" yaml:"endpoints" json:"endpoints"`                                           // 服务端点列表
	Network               string           `mapstructure:"network" yaml:"network" json:"network"`                                                 // 网络类型 (tcp, tcp4, tcp6, unix)
	MaxRecvMsgSize        int              `mapstructure:"max-recv-msg-size" yaml:"max-recv-msg-size" json:"maxRecvMsgSize"`                      // 最大接收消息大小(字节)
	MaxSendMsgSize        int              `mapstructure:"max-send-msg-size" yaml:"max-send-msg-size" json:"maxSendMsgSize"`                      // 最大发送消息大小(字节)
	KeepaliveTime         int              `mapstructure:"keepalive-time" yaml:"keepalive-time" json:"keepaliveTime"`                             // Keepalive时间(秒)
	KeepaliveTimeout      int              `mapstructure:"keepalive-timeout" yaml:"keepalive-timeout" json:"keepaliveTimeout"`                    // Keepalive超时(秒)
	ConnectionTimeout     int              `mapstructure:"connection-timeout" yaml:"connection-timeout" json:"connectionTimeout"`                 // 连接超时(秒)
	RetryTimes            int              `mapstructure:"retry-times" yaml:"retry-times" json:"retryTimes"`                                      // 重试次数
	EnableLoadBalance     bool             `mapstructure:"enable-load-balance" yaml:"enable-load-balance" json:"enableLoadBalance"`               // 是否启用负载均衡
	LoadBalancePolicy     string           `mapstructure:"load-balance-policy" yaml:"load-balance-policy" json:"loadBalancePolicy"`               // 负载均衡策略
	EnableBreaker         bool             `mapstructure:"enable-breaker" yaml:"enable-breaker" json:"enableBreaker"`                             // 是否启用客户端熔断（默认关闭，未启用时拦截器不入链）
	BreakerKeyMode        string           `mapstructure:"breaker-key-mode" yaml:"breaker-key-mode" json:"breakerKeyMode"`                        // 客户端熔断键粒度 (target_method / target / method)，非法值由使用方回落默认
	EnableTLS             bool             `mapstructure:"enable-tls" yaml:"enable-tls" json:"enableTls"`                                         // 是否启用TLS
	TLSCertFile           string           `mapstructure:"tls-cert-file" yaml:"tls-cert-file" json:"tlsCertFile"`                                 // TLS证书文件
	TLSKeyFile            string           `mapstructure:"tls-key-file" yaml:"tls-key-file" json:"tlsKeyFile"`                                    // TLS密钥文件
	TLSCAFile             string           `mapstructure:"tls-ca-file" yaml:"tls-ca-file" json:"tlsCaFile"`                                       // TLS CA文件
	InitialWindowSize     int32            `mapstructure:"initial-window-size" yaml:"initial-window-size" json:"initialWindowSize"`               // HTTP/2 初始窗口大小（字节）
	InitialConnWindowSize int32            `mapstructure:"initial-conn-window-size" yaml:"initial-conn-window-size" json:"initialConnWindowSize"` // HTTP/2 初始连接窗口大小（字节）
	WaitForReady          bool             `mapstructure:"wait-for-ready" yaml:"wait-for-ready" json:"waitForReady"`                              // 是否等待连接就绪
	EnableCompression     bool             `mapstructure:"enable-compression" yaml:"enable-compression" json:"enableCompression"`                 // 是否启用压缩
	CompressionType       GRPCCompressType `mapstructure:"compression-type" yaml:"compression-type" json:"compressionType"`                       // 压缩算法类型 (gzip, snappy, zstd)
}

// DefaultGRPC 创建默认GRPC配置
func DefaultGRPC() *GRPC {
	return &GRPC{
		Server:  DefaultGRPCServer(),
		Clients: make(map[string]*GRPCClient),
	}
}

// DefaultGRPCServer 创建默认GRPC服务端配置
func DefaultGRPCServer() *GRPCServer {
	g := &GRPCServer{
		Enable:             false, // 默认不启用，需要显式配置
		Host:               "0.0.0.0",
		Port:               9090,
		Network:            "tcp4",          // 默认使用 tcp4 强制 IPv4
		MaxRecvMsgSize:     4 * 1024 * 1024, // 4MB
		MaxSendMsgSize:     4 * 1024 * 1024, // 4MB
		KeepaliveTime:      30,
		KeepaliveTimeout:   10,
		ConnectionTimeout:  5,
		EnableReflection:   true,
		EnableProtobufResp: false,            // 默认返回JSON（grpc-gateway场景）
		EnableCompression:  false,            // 默认不启用压缩，需显式开启
		CompressionType:    GRPCCompressGzip, // 默认Gzip压缩
		CompressionLevel:   5,                // 平衡速度和压缩率
		MinCompressSize:    1024,             // 1KB以下不压缩
	}
	internal.CallAfterLoad(g) // 自动调用 AfterLoad 钩子
	return g
}

// BeforeLoad GRPC Server 配置加载前的钩子
func (g *GRPCServer) BeforeLoad() error {
	return nil
}

// AfterLoad GRPC Server 配置加载后的钩子 - 计算 Endpoint
func (g *GRPCServer) AfterLoad() error {
	g.Endpoint = fmt.Sprintf("%s:%d", g.Host, g.Port)
	return nil
}

// GetEndpoint 获取GRPC服务端点地址
func (g *GRPCServer) GetEndpoint() string {
	if g.Endpoint == "" {
		internal.CallAfterLoad(g)
	}
	return g.Endpoint
}

// DefaultGRPCClient 创建默认GRPC客户端配置
func DefaultGRPCClient(serviceName string, endpoints []string) *GRPCClient {
	return &GRPCClient{
		ServiceName:           serviceName,
		Endpoints:             endpoints,
		Network:               "tcp4",           // 默认使用 tcp4 强制 IPv4
		MaxRecvMsgSize:        16 * 1024 * 1024, // 16MB（优化后）
		MaxSendMsgSize:        16 * 1024 * 1024, // 16MB（优化后）
		KeepaliveTime:         10,               // 10秒（优化连接保活）
		KeepaliveTimeout:      3,                // 3秒
		ConnectionTimeout:     30,
		RetryTimes:            3,
		EnableLoadBalance:     true,
		LoadBalancePolicy:     "round_robin",
		EnableBreaker:         false,           // 默认关闭：客户端熔断阈值要按下游实际情况调，默认开启等于替使用方做决定
		BreakerKeyMode:        "target_method", // 故障隔离到方法级：只按 method 会让不同下游互相牵连，只按 target 会让一个慢方法打死整个下游
		EnableTLS:             false,
		InitialWindowSize:     1 << 20, // 1MB 初始窗口（HTTP/2 多路复用优化）
		InitialConnWindowSize: 1 << 21, // 2MB 连接窗口（HTTP/2 多路复用优化）
		WaitForReady:          false,   // 快速失败，不等待连接就绪
	}
}

// Clone 返回配置的副本
func (g *GRPC) Clone() internal.Configurable {
	var cloned GRPC
	if err := syncx.DeepCopy(&cloned, g); err != nil {
		// 如果深拷贝失败，返回空配置
		return &GRPC{}
	}
	// 重新计算 Server Endpoint
	if cloned.Server != nil {
		internal.CallAfterLoad(cloned.Server)
	}
	return &cloned
}

// Clone 返回GRPC服务端配置的副本
func (g *GRPCServer) Clone() *GRPCServer {
	var cloned GRPCServer
	if err := syncx.DeepCopy(&cloned, g); err != nil {
		// 如果深拷贝失败，返回空配置
		return &GRPCServer{}
	}
	internal.CallAfterLoad(&cloned) // 重新计算 Endpoint
	return &cloned
}

// Clone 返回GRPC客户端配置的副本
func (g *GRPCClient) Clone() *GRPCClient {
	var cloned GRPCClient
	if err := syncx.DeepCopy(&cloned, g); err != nil {
		// 如果深拷贝失败，返回空配置
		return &GRPCClient{}
	}
	return &cloned
}

// Get 返回配置接口
func (g *GRPC) Get() interface{} {
	return g
}

// Set 设置配置数据
func (g *GRPC) Set(data interface{}) {
	if cfg, ok := data.(*GRPC); ok {
		*g = *cfg
	}
}

// Validate 验证配置
func (g *GRPC) Validate() error {
	if err := internal.ValidateStruct(g); err != nil {
		return err
	}
	if g.Server != nil {
		if err := g.Server.Validate(); err != nil {
			return err
		}
	}
	for _, client := range g.Clients {
		if err := client.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate 验证GRPC服务端配置
func (g *GRPCServer) Validate() error {
	return internal.ValidateStruct(g)
}

// Validate 验证GRPC客户端配置
func (g *GRPCClient) Validate() error {
	return internal.ValidateStruct(g)
}

// WithHost 设置主机地址
func (g *GRPCServer) WithHost(host string) *GRPCServer {
	g.Host = host
	internal.CallAfterLoad(g) // 重新计算 Endpoint
	return g
}

// WithPort 设置端口
func (g *GRPCServer) WithPort(port int) *GRPCServer {
	g.Port = port
	internal.CallAfterLoad(g) // 重新计算 Endpoint
	return g
}

// WithMaxMsgSize 设置最大消息大小
func (g *GRPCServer) WithMaxMsgSize(maxRecv, maxSend int) *GRPCServer {
	g.MaxRecvMsgSize = maxRecv
	g.MaxSendMsgSize = maxSend
	return g
}

// WithKeepalive 设置Keepalive配置
func (g *GRPCServer) WithKeepalive(time, timeout int) *GRPCServer {
	g.KeepaliveTime = time
	g.KeepaliveTimeout = timeout
	return g
}

// EnableReflectionService 启用反射服务
func (g *GRPCServer) EnableReflectionService() *GRPCServer {
	g.EnableReflection = true
	return g
}

// DisableReflectionService 禁用反射服务
func (g *GRPCServer) DisableReflectionService() *GRPCServer {
	g.EnableReflection = false
	return g
}

// EnableProtobufResponse 启用protobuf格式响应
func (g *GRPCServer) EnableProtobufResponse() *GRPCServer {
	g.EnableProtobufResp = true
	return g
}

// DisableProtobufResponse 禁用protobuf格式响应（返回JSON）
func (g *GRPCServer) DisableProtobufResponse() *GRPCServer {
	g.EnableProtobufResp = false
	return g
}

// WithCompression 设置压缩配置
func (g *GRPCServer) WithCompression(compressType GRPCCompressType, level, minCompressSize int) *GRPCServer {
	g.EnableCompression = true
	g.CompressionType = compressType
	g.CompressionLevel = level
	g.MinCompressSize = minCompressSize
	return g
}

// EnableCompressionService 启用压缩
func (g *GRPCServer) EnableCompressionService() *GRPCServer {
	g.EnableCompression = true
	return g
}

// DisableCompressionService 禁用压缩
func (g *GRPCServer) DisableCompressionService() *GRPCServer {
	g.EnableCompression = false
	return g
}

// WithCompressionType 设置压缩算法类型
func (g *GRPCServer) WithCompressionType(compressType GRPCCompressType) *GRPCServer {
	g.CompressionType = compressType
	return g
}

// WithCompressionLevel 设置压缩级别
func (g *GRPCServer) WithCompressionLevel(level int) *GRPCServer {
	g.CompressionLevel = level
	return g
}

// WithMinCompressSize 设置最小压缩大小
func (g *GRPCServer) WithMinCompressSize(size int) *GRPCServer {
	g.MinCompressSize = size
	return g
}

// AddClient 添加GRPC客户端配置
func (g *GRPC) AddClient(name string, client *GRPCClient) *GRPC {
	if g.Clients == nil {
		g.Clients = make(map[string]*GRPCClient)
	}
	g.Clients[name] = client
	return g
}

// GetClient 获取GRPC客户端配置
func (g *GRPC) GetClient(name string) *GRPCClient {
	if g.Clients == nil {
		return nil
	}
	return g.Clients[name]
}

// WithConnectionTimeout 设置连接超时
func (g *GRPCClient) WithConnectionTimeout(timeout int) *GRPCClient {
	g.ConnectionTimeout = timeout
	return g
}

// WithRetry 设置重试次数
func (g *GRPCClient) WithRetry(retryTimes int) *GRPCClient {
	g.RetryTimes = retryTimes
	return g
}

// EnableLoadBalancing 启用负载均衡
func (g *GRPCClient) EnableLoadBalancing(policy string) *GRPCClient {
	g.EnableLoadBalance = true
	g.LoadBalancePolicy = policy
	return g
}

// EnableClientBreaker 启用客户端熔断
// keyMode 为空时保留当前值（默认 target_method），合法值 target_method / target / method
// 本包只存字符串，合法性校验留给使用方：go-config 不能依赖 go-rpc-gateway（会成环）
func (g *GRPCClient) EnableClientBreaker(keyMode string) *GRPCClient {
	g.EnableBreaker = true
	if keyMode != "" {
		g.BreakerKeyMode = keyMode
	}
	return g
}

// DisableClientBreaker 禁用客户端熔断
func (g *GRPCClient) DisableClientBreaker() *GRPCClient {
	g.EnableBreaker = false
	return g
}

// WithTLS 设置TLS配置
func (g *GRPCClient) WithTLS(certFile, keyFile, caFile string) *GRPCClient {
	g.EnableTLS = true
	g.TLSCertFile = certFile
	g.TLSKeyFile = keyFile
	g.TLSCAFile = caFile
	return g
}

// WithEndpoints 设置服务端点
func (g *GRPCClient) WithEndpoints(endpoints []string) *GRPCClient {
	g.Endpoints = endpoints
	return g
}

// AddEndpoint 添加服务端点
func (g *GRPCClient) AddEndpoint(endpoint string) *GRPCClient {
	g.Endpoints = append(g.Endpoints, endpoint)
	return g
}

// WithNetwork 设置网络类型
func (g *GRPCClient) WithNetwork(network string) *GRPCClient {
	g.Network = network
	return g
}

// WithHTTP2WindowSize 设置 HTTP/2 窗口大小（优化多路复用性能）
func (g *GRPCClient) WithHTTP2WindowSize(initialWindow, connWindow int32) *GRPCClient {
	g.InitialWindowSize = initialWindow
	g.InitialConnWindowSize = connWindow
	return g
}

// WithWaitForReady 设置是否等待连接就绪
func (g *GRPCClient) WithWaitForReady(wait bool) *GRPCClient {
	g.WaitForReady = wait
	return g
}

// WithClientCompression 设置客户端压缩配置
func (g *GRPCClient) WithClientCompression(compressType GRPCCompressType) *GRPCClient {
	g.EnableCompression = true
	g.CompressionType = compressType
	return g
}

// EnableClientCompression 启用客户端压缩
func (g *GRPCClient) EnableClientCompression() *GRPCClient {
	g.EnableCompression = true
	return g
}

// DisableClientCompression 禁用客户端压缩
func (g *GRPCClient) DisableClientCompression() *GRPCClient {
	g.EnableCompression = false
	return g
}
