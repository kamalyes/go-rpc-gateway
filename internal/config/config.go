/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2024-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\internal\config\config.go
 * @Description: Gateway配置结构定义，基于go-config和go-core深度集成
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package config

import (
	"fmt"
	"os"

	"github.com/fsnotify/fsnotify"
	goconfig "github.com/kamalyes/go-config"
	"github.com/kamalyes/go-config/pkg/captcha"
	"github.com/kamalyes/go-config/pkg/database"
	"github.com/kamalyes/go-config/pkg/env"
	"github.com/kamalyes/go-config/pkg/jwt"
	"github.com/kamalyes/go-config/pkg/oss"
	"github.com/kamalyes/go-config/pkg/redis"
	"github.com/kamalyes/go-config/pkg/register"
	zapconfig "github.com/kamalyes/go-config/pkg/zap"
	"github.com/kamalyes/go-config/pkg/zero"
	"github.com/kamalyes/go-core/pkg/global"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// 常量定义
const (
	DefaultServiceName = "go-rpc-gateway"
	HealthPath         = "/health"
	MetricsPath        = "/metrics"
)

// GatewayConfig Gateway配置结构
type GatewayConfig struct {
	// 基础配置
	*goconfig.SingleConfig `mapstructure:",squash" yaml:",inline" json:",inline"`

	// Gateway特有配置
	Gateway GatewaySettings `mapstructure:"gateway" yaml:"gateway" json:"gateway"`

	// 中间件配置
	Middleware MiddlewareConfig `mapstructure:"middleware" yaml:"middleware" json:"middleware"`

	// 监控配置
	Monitoring MonitoringConfig `mapstructure:"monitoring" yaml:"monitoring" json:"monitoring"`

	// 安全配置
	Security SecurityConfig `mapstructure:"security" yaml:"security" json:"security"`
}

// GatewaySettings Gateway基础设置
type GatewaySettings struct {
	Name        string `mapstructure:"name" yaml:"name" json:"name"`
	Version     string `mapstructure:"version" yaml:"version" json:"version"`
	Environment string `mapstructure:"environment" yaml:"environment" json:"environment"`
	Debug       bool   `mapstructure:"debug" yaml:"debug" json:"debug"`

	// 服务器配置
	HTTP HTTPConfig `mapstructure:"http" yaml:"http" json:"http"`
	GRPC GRPCConfig `mapstructure:"grpc" yaml:"grpc" json:"grpc"`

	// 健康检查
	HealthCheck HealthCheckConfig `mapstructure:"health_check" yaml:"health_check" json:"health_check"`
}

// HTTPConfig HTTP服务器配置
type HTTPConfig struct {
	Host               string `mapstructure:"host" yaml:"host" json:"host"`
	Port               int    `mapstructure:"port" yaml:"port" json:"port"`
	ReadTimeout        int    `mapstructure:"read_timeout" yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout       int    `mapstructure:"write_timeout" yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout        int    `mapstructure:"idle_timeout" yaml:"idle_timeout" json:"idle_timeout"`
	MaxHeaderBytes     int    `mapstructure:"max_header_bytes" yaml:"max_header_bytes" json:"max_header_bytes"`
	EnableGzipCompress bool   `mapstructure:"enable_gzip_compress" yaml:"enable_gzip_compress" json:"enable_gzip_compress"`
}

// GRPCConfig GRPC服务器配置
type GRPCConfig struct {
	Host              string `mapstructure:"host" yaml:"host" json:"host"`
	Port              int    `mapstructure:"port" yaml:"port" json:"port"`
	Network           string `mapstructure:"network" yaml:"network" json:"network"`
	MaxRecvMsgSize    int    `mapstructure:"max_recv_msg_size" yaml:"max_recv_msg_size" json:"max_recv_msg_size"`
	MaxSendMsgSize    int    `mapstructure:"max_send_msg_size" yaml:"max_send_msg_size" json:"max_send_msg_size"`
	ConnectionTimeout int    `mapstructure:"connection_timeout" yaml:"connection_timeout" json:"connection_timeout"`
	KeepaliveTime     int    `mapstructure:"keepalive_time" yaml:"keepalive_time" json:"keepalive_time"`
	KeepaliveTimeout  int    `mapstructure:"keepalive_timeout" yaml:"keepalive_timeout" json:"keepalive_timeout"`
	EnableReflection  bool   `mapstructure:"enable_reflection" yaml:"enable_reflection" json:"enable_reflection"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled  bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Path     string `mapstructure:"path" yaml:"path" json:"path"`
	Interval int    `mapstructure:"interval" yaml:"interval" json:"interval"`
}

// MiddlewareConfig 中间件配置
type MiddlewareConfig struct {
	// 跨域配置
	CORS CORSMiddlewareConfig `mapstructure:"cors" yaml:"cors" json:"cors"`

	// 限流配置
	RateLimit RateLimitConfig `mapstructure:"rate_limit" yaml:"rate_limit" json:"rate_limit"`

	// 认证配置
	Auth AuthConfig `mapstructure:"auth" yaml:"auth" json:"auth"`

	// 访问记录
	AccessLog AccessLogConfig `mapstructure:"access_log" yaml:"access_log" json:"access_log"`

	// 请求签名
	Signature SignatureConfig `mapstructure:"signature" yaml:"signature" json:"signature"`
}

// CORSMiddlewareConfig 跨域中间件配置
type CORSMiddlewareConfig struct {
	Enabled          bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	AllowOrigins     []string `mapstructure:"allow_origins" yaml:"allow_origins" json:"allow_origins"`
	AllowMethods     []string `mapstructure:"allow_methods" yaml:"allow_methods" json:"allow_methods"`
	AllowHeaders     []string `mapstructure:"allow_headers" yaml:"allow_headers" json:"allow_headers"`
	ExposeHeaders    []string `mapstructure:"expose_headers" yaml:"expose_headers" json:"expose_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials" yaml:"allow_credentials" json:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age" yaml:"max_age" json:"max_age"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled    bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Algorithm  string `mapstructure:"algorithm" yaml:"algorithm" json:"algorithm"` // token_bucket, sliding_window
	Rate       int    `mapstructure:"rate" yaml:"rate" json:"rate"`
	Burst      int    `mapstructure:"burst" yaml:"burst" json:"burst"`
	WindowSize int    `mapstructure:"window_size" yaml:"window_size" json:"window_size"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Enabled   bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Type      string   `mapstructure:"type" yaml:"type" json:"type"` // jwt, basic, oauth2
	SkipPaths []string `mapstructure:"skip_paths" yaml:"skip_paths" json:"skip_paths"`
}

// AccessLogConfig 访问日志配置
type AccessLogConfig struct {
	Enabled        bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Format         string `mapstructure:"format" yaml:"format" json:"format"` // json, text
	IncludeBody    bool   `mapstructure:"include_body" yaml:"include_body" json:"include_body"`
	IncludeHeaders bool   `mapstructure:"include_headers" yaml:"include_headers" json:"include_headers"`
}

// SignatureConfig 请求签名配置
type SignatureConfig struct {
	Enabled   bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Algorithm string   `mapstructure:"algorithm" yaml:"algorithm" json:"algorithm"` // hmac-sha256
	SecretKey string   `mapstructure:"secret_key" yaml:"secret_key" json:"secret_key"`
	SkipPaths []string `mapstructure:"skip_paths" yaml:"skip_paths" json:"skip_paths"`
	TTL       int      `mapstructure:"ttl" yaml:"ttl" json:"ttl"` // 签名有效期（秒）
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Metrics MetricsConfig `mapstructure:"metrics" yaml:"metrics" json:"metrics"`
	Tracing TracingConfig `mapstructure:"tracing" yaml:"tracing" json:"tracing"`
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	Enabled   bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Path      string `mapstructure:"path" yaml:"path" json:"path"`
	Port      int    `mapstructure:"port" yaml:"port" json:"port"`
	Namespace string `mapstructure:"namespace" yaml:"namespace" json:"namespace"`
	Subsystem string `mapstructure:"subsystem" yaml:"subsystem" json:"subsystem"`
}

// TracingConfig 链路追踪配置
type TracingConfig struct {
	Enabled     bool    `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	ServiceName string  `mapstructure:"service_name" yaml:"service_name" json:"service_name"`
	Endpoint    string  `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint"`
	SampleRate  float64 `mapstructure:"sample_rate" yaml:"sample_rate" json:"sample_rate"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	TLS      TLSConfig      `mapstructure:"tls" yaml:"tls" json:"tls"`
	Security SecurityPolicy `mapstructure:"policy" yaml:"policy" json:"policy"`
}

// TLSConfig TLS配置
type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	CertFile string `mapstructure:"cert_file" yaml:"cert_file" json:"cert_file"`
	KeyFile  string `mapstructure:"key_file" yaml:"key_file" json:"key_file"`
	CAFile   string `mapstructure:"ca_file" yaml:"ca_file" json:"ca_file"`
}

// SecurityPolicy 安全策略
type SecurityPolicy struct {
	EnableCSRFProtection bool   `mapstructure:"enable_csrf_protection" yaml:"enable_csrf_protection" json:"enable_csrf_protection"`
	EnableXSSProtection  bool   `mapstructure:"enable_xss_protection" yaml:"enable_xss_protection" json:"enable_xss_protection"`
	ContentTypeNoSniff   bool   `mapstructure:"content_type_no_sniff" yaml:"content_type_no_sniff" json:"content_type_no_sniff"`
	FrameOptions         string `mapstructure:"frame_options" yaml:"frame_options" json:"frame_options"`
}

// DefaultGatewayConfig 返回默认Gateway配置
func DefaultGatewayConfig() *GatewayConfig {
	return &GatewayConfig{
		SingleConfig: &goconfig.SingleConfig{
			Server: register.Server{
				ServerName: DefaultServiceName,
				Addr:       ":8080",
				DataDriver: "mysql",
			},
			MySQL:      database.MySQL{},
			Redis:      redis.Redis{},
			Minio:      oss.Minio{},
			Zap:        zapconfig.Zap{Level: "info", Format: "console"},
			JWT:        jwt.JWT{},
			Captcha:    captcha.Captcha{},
			ZeroServer: zero.RpcServer{},
		},
		Gateway: GatewaySettings{
			Name:        DefaultServiceName,
			Version:     "v1.0.0",
			Environment: "development",
			Debug:       true,
			HTTP: HTTPConfig{
				Host:               "0.0.0.0",
				Port:               8080,
				ReadTimeout:        30,
				WriteTimeout:       30,
				IdleTimeout:        120,
				MaxHeaderBytes:     1048576,
				EnableGzipCompress: true,
			},
			GRPC: GRPCConfig{
				Host:              "0.0.0.0",
				Port:              9090,
				Network:           "tcp",
				MaxRecvMsgSize:    4194304,
				MaxSendMsgSize:    4194304,
				ConnectionTimeout: 30,
				KeepaliveTime:     60,
				KeepaliveTimeout:  30,
				EnableReflection:  true,
			},
			HealthCheck: HealthCheckConfig{
				Enabled:  true,
				Path:     HealthPath,
				Interval: 30,
			},
		},
		Middleware: MiddlewareConfig{
			CORS: CORSMiddlewareConfig{
				Enabled:          true,
				AllowOrigins:     []string{"*"},
				AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowHeaders:     []string{"*"},
				AllowCredentials: true,
				MaxAge:           86400,
			},
			RateLimit: RateLimitConfig{
				Enabled:    true,
				Algorithm:  "token_bucket",
				Rate:       100,
				Burst:      200,
				WindowSize: 60,
			},
			Auth: AuthConfig{
				Enabled:   false,
				Type:      "jwt",
				SkipPaths: []string{HealthPath, MetricsPath},
			},
			AccessLog: AccessLogConfig{
				Enabled:        true,
				Format:         "json",
				IncludeBody:    false,
				IncludeHeaders: false,
			},
			Signature: SignatureConfig{
				Enabled:   false,
				Algorithm: "hmac-sha256",
				SkipPaths: []string{HealthPath, MetricsPath},
				TTL:       300,
			},
		},
		Monitoring: MonitoringConfig{
			Metrics: MetricsConfig{
				Enabled:   true,
				Path:      MetricsPath,
				Port:      9100,
				Namespace: "gateway",
				Subsystem: "rpc",
			},
			Tracing: TracingConfig{
				Enabled:     false,
				ServiceName: DefaultServiceName,
				SampleRate:  0.1,
			},
		},
		Security: SecurityConfig{
			TLS: TLSConfig{
				Enabled: false,
			},
			Security: SecurityPolicy{
				EnableCSRFProtection: false,
				EnableXSSProtection:  true,
				ContentTypeNoSniff:   true,
				FrameOptions:         "DENY",
			},
		},
	}
}

// ConfigManager Gateway配置管理器，集成go-config功能
type ConfigManager struct {
	config      *GatewayConfig
	viper       *viper.Viper
	configPath  string
	environment env.EnvironmentType
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configPath string) (*ConfigManager, error) {
	manager := &ConfigManager{
		config:     DefaultGatewayConfig(),
		configPath: configPath,
		viper:      viper.New(),
	}

	// 设置环境
	envVar := os.Getenv("GO_ENV")
	if envVar != "" {
		manager.environment = env.EnvironmentType(envVar)
	} else {
		manager.environment = env.Dev
	}

	// 加载配置
	if err := manager.LoadConfig(); err != nil {
		return nil, err
	}

	return manager, nil
}

// LoadConfig 加载配置文件
func (cm *ConfigManager) LoadConfig() error {
	if cm.configPath != "" && fileExists(cm.configPath) {
		cm.viper.SetConfigFile(cm.configPath)

		if err := cm.viper.ReadInConfig(); err != nil {
			return fmt.Errorf("读取配置文件失败: %w", err)
		}

		// 将viper配置解析到结构体
		if err := cm.viper.Unmarshal(cm.config); err != nil {
			return fmt.Errorf("解析配置失败: %w", err)
		}

		// 设置全局viper实例
		global.VP = cm.viper

		// 设置全局配置
		global.CONFIG = cm.config.SingleConfig

		if global.LOG != nil {
			global.LOG.Info("配置文件加载成功",
				zap.String("path", cm.configPath),
				zap.String("environment", string(cm.environment)))
		}
	}

	return nil
}

// GetConfig 获取配置
func (cm *ConfigManager) GetConfig() *GatewayConfig {
	return cm.config
}

// GetEnvironment 获取当前环境
func (cm *ConfigManager) GetEnvironment() env.EnvironmentType {
	return cm.environment
}

// WatchConfig 监听配置变化
func (cm *ConfigManager) WatchConfig(callback func(*GatewayConfig)) {
	if cm.configPath == "" {
		return
	}

	cm.viper.WatchConfig()
	cm.viper.OnConfigChange(func(e fsnotify.Event) {
		if global.LOG != nil {
			global.LOG.Info("配置文件发生变化", zap.String("file", e.Name))
		}

		// 重新加载配置
		if err := cm.LoadConfig(); err != nil {
			if global.LOG != nil {
				global.LOG.Error("重新加载配置失败", zap.Error(err))
			}
			return
		}

		// 回调通知
		if callback != nil {
			callback(cm.config)
		}
	})
}

// fileExists 检查文件是否存在
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}
