/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-26 23:42:05
 * @FilePath: \go-rpc-gateway\config\core\http.go
 * @Description: 服务器配置模块
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package core

import (
	"crypto/tls"
	"fmt"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// TLSVersion TLS 版本枚举
type TLSVersion string

const (
	TLSVersion10 TLSVersion = "TLS10" // TLS 1.0
	TLSVersion11 TLSVersion = "TLS11" // TLS 1.1
	TLSVersion12 TLSVersion = "TLS12" // TLS 1.2
	TLSVersion13 TLSVersion = "TLS13" // TLS 1.3
)

// ToUint16 将 TLSVersion 转换为 tls.Config 使用的 uint16 值
func (v TLSVersion) ToUint16() uint16 {
	switch v {
	case TLSVersion10:
		return tls.VersionTLS10
	case TLSVersion11:
		return tls.VersionTLS11
	case TLSVersion12:
		return tls.VersionTLS12
	case TLSVersion13:
		return tls.VersionTLS13
	default:
		return tls.VersionTLS12 // 默认 TLS 1.2
	}
}

// ClientAuthType 客户端认证类型枚举
type ClientAuthType string

const (
	NoClientCert               ClientAuthType = "NoClientCert"               // 不需要客户端证书
	RequestClientCert          ClientAuthType = "RequestClientCert"          // 请求客户端证书（可选）
	RequireAnyClientCert       ClientAuthType = "RequireAnyClientCert"       // 要求任意客户端证书
	VerifyClientCertIfGiven    ClientAuthType = "VerifyClientCertIfGiven"    // 如果提供则验证客户端证书
	RequireAndVerifyClientCert ClientAuthType = "RequireAndVerifyClientCert" // 要求并验证客户端证书
)

// ToTLSClientAuth 将 ClientAuthType 转换为 tls.ClientAuthType
func (c ClientAuthType) ToTLSClientAuth() tls.ClientAuthType {
	switch c {
	case NoClientCert:
		return tls.NoClientCert
	case RequestClientCert:
		return tls.RequestClientCert
	case RequireAnyClientCert:
		return tls.RequireAnyClientCert
	case VerifyClientCertIfGiven:
		return tls.VerifyClientCertIfGiven
	case RequireAndVerifyClientCert:
		return tls.RequireAndVerifyClientCert
	default:
		return tls.NoClientCert
	}
}

// HTTP/2 默认配置常量
const (
	DefaultHTTP2MaxConcurrentStreams  = 250     // 默认最大并发流数
	DefaultHTTP2MaxReadFrameSize      = 1 << 20 // 默认最大读取帧大小 (1MB)
	DefaultHTTP2InitialWindowSize     = 1 << 20 // 默认初始窗口大小 (1MB)
	DefaultHTTP2InitialConnWindowSize = 1 << 21 // 默认初始连接窗口大小 (2MB)
)

// HTTPServer HTTP服务器配置
type HTTPServer struct {
	ModuleName           string            `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`                                 // 模块名称
	Host                 string            `mapstructure:"host" yaml:"host" json:"host"`                                                     // 主机地址
	Port                 int               `mapstructure:"port" yaml:"port" json:"port"`                                                     // 端口
	Network              string            `mapstructure:"network" yaml:"network" json:"network"`                                            // 网络类型: tcp, tcp4, tcp6
	ReadTimeout          int               `mapstructure:"read-timeout" yaml:"read-timeout" json:"readTimeout"`                              // 读取超时(秒)
	ReadHeaderTimeout    int               `mapstructure:"read-header-timeout" yaml:"read-header-timeout" json:"readHeaderTimeout"`          // 读取请求头超时(秒)，防止慢速攻击
	WriteTimeout         int               `mapstructure:"write-timeout" yaml:"write-timeout" json:"writeTimeout"`                           // 写入超时(秒)
	IdleTimeout          int               `mapstructure:"idle-timeout" yaml:"idle-timeout" json:"idleTimeout"`                              // 空闲超时(秒)
	MaxHeaderBytes       int               `mapstructure:"max-header-bytes" yaml:"max-header-bytes" json:"maxHeaderBytes"`                   // 最大请求头字节数
	EnableTls            bool              `mapstructure:"enable-tls" yaml:"enable-tls" json:"enableTls"`                                    // 是否启用TLS
	TLS                  *TLS              `mapstructure:"tls" yaml:"tls" json:"tls"`                                                        // TLS配置
	Headers              map[string]string `mapstructure:"headers" yaml:"headers" json:"headers"`                                            // 自定义头部
	Endpoint             string            `mapstructure:"-" yaml:"-" json:""`                                                               // 完整的服务端点地址（自动计算，不从配置文件读取）
	EnableGzipCompress   bool              `mapstructure:"enable-gzip-compress" yaml:"enable-gzip-compress" json:"enableGzipCompress"`       // 是否启用Gzip压缩
	GzipCompressionLevel int               `mapstructure:"gzip-compression-level" yaml:"gzip-compression-level" json:"gzipCompressionLevel"` // Gzip压缩级别 (1-9, 默认5)
	GzipMinSize          int               `mapstructure:"gzip-min-size" yaml:"gzip-min-size" json:"gzipMinSize"`                            // Gzip最小压缩大小（字节），小于此大小不压缩（默认1024）
	GzipSkipPaths        []string          `mapstructure:"gzip-skip-paths" yaml:"gzip-skip-paths" json:"gzipSkipPaths"`                      // Gzip跳过的路径前缀列表
	GzipSkipExtensions   []string          `mapstructure:"gzip-skip-extensions" yaml:"gzip-skip-extensions" json:"gzipSkipExtensions"`       // Gzip跳过的文件扩展名列表
	EnableHTTP2          bool              `mapstructure:"enable-http2" yaml:"enable-http2" json:"enableHttp2"`                              // 是否启用HTTP/2（h2c）
	HTTP2                *HTTP2            `mapstructure:"http2" yaml:"http2" json:"http2"`                                                  // HTTP/2配置
}

// TLS TLS配置
type TLS struct {
	CertFile            string         `mapstructure:"cert-file" yaml:"cert-file" json:"certFile"`                                    // 证书文件路径
	KeyFile             string         `mapstructure:"key-file" yaml:"key-file" json:"keyFile"`                                       // 私钥文件路径
	CAFile              string         `mapstructure:"ca-file" yaml:"ca-file" json:"caFile"`                                          // CA文件路径
	MinVersion          TLSVersion     `mapstructure:"min-version" yaml:"min-version" json:"minVersion"`                              // 最小TLS版本
	PreferServerCiphers bool           `mapstructure:"prefer-server-ciphers" yaml:"prefer-server-ciphers" json:"preferServerCiphers"` // 优先使用服务器密码套件
	NextProtos          []string       `mapstructure:"next-protos" yaml:"next-protos" json:"nextProtos"`                              // ALPN协议列表（如: h2, http/1.1）
	InsecureSkipVerify  bool           `mapstructure:"insecure-skip-verify" yaml:"insecure-skip-verify" json:"insecureSkipVerify"`    // 跳过证书验证（仅用于开发环境）
	ClientAuth          ClientAuthType `mapstructure:"client-auth" yaml:"client-auth" json:"clientAuth"`                              // 客户端认证模式
}

// HTTP2 HTTP/2配置
type HTTP2 struct {
	MaxConcurrentStreams  uint32 `mapstructure:"max-concurrent-streams" yaml:"max-concurrent-streams" json:"maxConcurrentStreams"`      // 单个连接最大并发流数
	MaxReadFrameSize      uint32 `mapstructure:"max-read-frame-size" yaml:"max-read-frame-size" json:"maxReadFrameSize"`                // 最大读取帧大小
	InitialWindowSize     int32  `mapstructure:"initial-window-size" yaml:"initial-window-size" json:"initialWindowSize"`               // 初始窗口大小
	InitialConnWindowSize int32  `mapstructure:"initial-conn-window-size" yaml:"initial-conn-window-size" json:"initialConnWindowSize"` // 初始连接窗口大小
}

// DefaultHTTPServer 创建默认HTTP服务器配置
func DefaultHTTPServer() *HTTPServer {
	h := &HTTPServer{
		ModuleName:           "server",
		Host:                 "0.0.0.0",
		Port:                 8080,
		Network:              "tcp4",
		ReadTimeout:          30,
		ReadHeaderTimeout:    5, // 防止慢速攻击（Slowloris）
		WriteTimeout:         30,
		IdleTimeout:          60,
		MaxHeaderBytes:       1 << 20, // 1MB
		GzipCompressionLevel: 5,       // 平衡速度和压缩率
		GzipMinSize:          1024,    // 1KB，小于此大小不压缩
		GzipSkipPaths: []string{ // 默认跳过的路径
			"/ws",      // WebSocket
			"/metrics", // Prometheus指标
			"/health",  // 健康检查
			"/healthz", // 健康检查
			"/debug",   // 调试接口
		},
		GzipSkipExtensions: []string{ // 默认跳过的扩展名（已压缩或不适合压缩）
			".gz", ".zip", ".rar", ".7z", ".tar", // 压缩文件
			".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".ico", // 图片
			".mp4", ".avi", ".mov", ".wmv", ".flv", ".mkv", // 视频
			".mp3", ".wav", ".flac", ".aac", ".ogg", // 音频
			".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", // 文档
		},
		EnableTls: false,
		TLS: &TLS{
			CertFile:            "",
			KeyFile:             "",
			CAFile:              "",
			MinVersion:          TLSVersion12,
			PreferServerCiphers: true,
			NextProtos:          []string{"h2", "http/1.1"}, // 优先使用HTTP/2
			InsecureSkipVerify:  false,
			ClientAuth:          NoClientCert,
		},
		Headers: map[string]string{
			"x-server": "go-config",
		},
		EnableGzipCompress: true,
		EnableHTTP2:        true, // 默认启用HTTP/2
		HTTP2: &HTTP2{
			MaxConcurrentStreams:  DefaultHTTP2MaxConcurrentStreams,
			MaxReadFrameSize:      DefaultHTTP2MaxReadFrameSize,
			InitialWindowSize:     DefaultHTTP2InitialWindowSize,
			InitialConnWindowSize: DefaultHTTP2InitialConnWindowSize,
		},
	}
	internal.CallAfterLoad(h) // 自动调用 AfterLoad 钩子
	return h
}

// BeforeLoad 配置加载前的钩子函数
func (h *HTTPServer) BeforeLoad() error {
	// 这里可以添加加载前的预处理逻辑
	return nil
}

// AfterLoad 配置加载后的钩子函数 - 用于计算衍生字段
func (h *HTTPServer) AfterLoad() error {
	h.Endpoint = fmt.Sprintf("http://%s:%d", h.Host, h.Port)
	return nil
}

// GetEndpoint 获取服务端点地址（如果未设置则自动计算）
func (h *HTTPServer) GetEndpoint() string {
	if h.Endpoint == "" {
		internal.CallAfterLoad(h)
	}
	return h.Endpoint
}

// Clone 返回配置的副本
func (h *HTTPServer) Clone() internal.Configurable {
	var cloned HTTPServer
	if err := syncx.DeepCopy(&cloned, h); err != nil {
		// 如果深拷贝失败，返回空配置
		return &HTTPServer{}
	}
	internal.CallAfterLoad(&cloned) // 重新计算衍生字段
	return &cloned
}

// Get 返回配置接口
func (h *HTTPServer) Get() interface{} {
	return h
}

// Set 设置配置数据
func (h *HTTPServer) Set(data interface{}) {
	if cfg, ok := data.(*HTTPServer); ok {
		*h = *cfg
	}
}

// Validate 验证配置
func (h *HTTPServer) Validate() error {
	return internal.ValidateStruct(h)
}

// WithHost 设置主机地址
func (h *HTTPServer) WithHost(host string) *HTTPServer {
	h.Host = host
	internal.CallAfterLoad(h) // 重新计算 Endpoint
	return h
}

// WithPort 设置端口
func (h *HTTPServer) WithPort(port int) *HTTPServer {
	h.Port = port
	internal.CallAfterLoad(h) // 重新计算 Endpoint
	return h
}

// WithTimeouts 设置超时配置
func (h *HTTPServer) WithTimeouts(read, write, idle int) *HTTPServer {
	h.ReadTimeout = read
	h.WriteTimeout = write
	h.IdleTimeout = idle
	return h
}

// WithTLS 设置TLS配置
func (h *HTTPServer) WithTLS(certFile, keyFile, caFile string) *HTTPServer {
	if h.TLS == nil {
		h.TLS = &TLS{}
	}
	h.TLS.CertFile = certFile
	h.TLS.KeyFile = keyFile
	h.TLS.CAFile = caFile
	h.EnableTls = true
	return h
}

// EnableTLSService 启用TLS
func (h *HTTPServer) EnableTLSService() *HTTPServer {
	h.EnableTls = true
	return h
}

// DisableTLS 禁用TLS
func (h *HTTPServer) DisableTLS() *HTTPServer {
	h.EnableTls = false
	return h
}

// EnableGzip 启用Gzip压缩
func (h *HTTPServer) EnableGzip() *HTTPServer {
	h.EnableGzipCompress = true
	return h
}

// DisableGzip 禁用Gzip压缩
func (h *HTTPServer) DisableGzip() *HTTPServer {
	h.EnableGzipCompress = false
	return h
}

// AddHeader 添加自定义头部
func (h *HTTPServer) AddHeader(key, value string) *HTTPServer {
	if h.Headers == nil {
		h.Headers = make(map[string]string)
	}
	h.Headers[key] = value
	return h
}

// EnableHTTP2Service 启用HTTP/2
func (h *HTTPServer) EnableHTTP2Service() *HTTPServer {
	h.EnableHTTP2 = true
	return h
}

// DisableHTTP2 禁用HTTP/2
func (h *HTTPServer) DisableHTTP2() *HTTPServer {
	h.EnableHTTP2 = false
	return h
}

// WithHTTP2Config 设置HTTP/2配置
func (h *HTTPServer) WithHTTP2Config(maxStreams uint32, windowSize, connWindowSize int32) *HTTPServer {
	if h.HTTP2 == nil {
		h.HTTP2 = &HTTP2{}
	}
	h.HTTP2.MaxConcurrentStreams = maxStreams
	h.HTTP2.InitialWindowSize = windowSize
	h.HTTP2.InitialConnWindowSize = connWindowSize
	return h
}

// Listener 命名监听器配置，用于在同一进程中启动多个 HTTP 监听端口
// 典型场景：Ops 内网端口 + Tenant 公网端口
type Listener struct {
	Name     string `mapstructure:"name" yaml:"name" json:"name"`          // 监听器名称（如 "ops", "tenant"）
	Host     string `mapstructure:"host" yaml:"host" json:"host"`          // 主机地址
	Port     int    `mapstructure:"port" yaml:"port" json:"port"`          // 端口
	Network  string `mapstructure:"network" yaml:"network" json:"network"` // 网络类型: tcp, tcp4, tcp6
	Endpoint string `mapstructure:"-" yaml:"-" json:""`                    // 完整端点地址（自动计算）
}

// DefaultListener 创建默认监听器配置
func DefaultListener(name string) *Listener {
	l := &Listener{
		Name:    name,
		Host:    "0.0.0.0",
		Port:    8080,
		Network: "tcp4",
	}
	l.AfterLoad()
	return l
}

// AfterLoad 配置加载后的钩子 - 计算衍生字段
func (l *Listener) AfterLoad() error {
	l.Endpoint = fmt.Sprintf("http://%s:%d", l.Host, l.Port)
	return nil
}

// GetEndpoint 获取服务端点地址
func (l *Listener) GetEndpoint() string {
	if l.Endpoint == "" {
		l.AfterLoad()
	}
	return l.Endpoint
}

// WithListenerHost 设置监听器主机地址
func (l *Listener) WithListenerHost(host string) *Listener {
	l.Host = host
	l.AfterLoad()
	return l
}

// WithListenerPort 设置监听器端口
func (l *Listener) WithListenerPort(port int) *Listener {
	l.Port = port
	l.AfterLoad()
	return l
}
