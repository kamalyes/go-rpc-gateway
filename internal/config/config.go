/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 01:14:03
 * @FilePath: \go-rpc-gateway\internal\config\config.go
 * @Description: Gateway配置结构定义，基于go-config、go-core和go-logger深度集成
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package config

import (
	"fmt"
	"os"

	"github.com/fsnotify/fsnotify"
	goconfig "github.com/kamalyes/go-config"
	"github.com/kamalyes/go-config/pkg/cache"
	"github.com/kamalyes/go-config/pkg/cors"
	"github.com/kamalyes/go-config/pkg/env"
	"github.com/kamalyes/go-config/pkg/jwt"
	"github.com/kamalyes/go-config/pkg/register"
	zapconfig "github.com/kamalyes/go-config/pkg/zap"
	"github.com/kamalyes/go-core/pkg/global"
	"github.com/kamalyes/go-logger"
	"github.com/spf13/viper"
)

// 常量定义
const (
	DefaultServiceName = "go-rpc-gateway"
	HealthPath         = "/health"
	MetricsPath        = "/metrics"
)

// GatewayConfig Gateway配置结构，基于go-config简化配置管理
type GatewayConfig struct {
	// 基础配置完全使用go-config
	*goconfig.SingleConfig `mapstructure:",squash" yaml:",inline" json:",inline"`

	// Gateway特有的扩展配置
	Gateway    GatewaySettings    `mapstructure:"gateway" yaml:"gateway" json:"gateway"`
	Middleware MiddlewareConfig   `mapstructure:"middleware" yaml:"middleware" json:"middleware"`
	Monitoring MonitoringConfig   `mapstructure:"monitoring" yaml:"monitoring" json:"monitoring"`
	Security   SecurityConfig     `mapstructure:"security" yaml:"security" json:"security"`
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
	// 使用go-config标准的CORS配置，但保持自定义字段名兼容
	// CORS CORSMiddlewareConfig `mapstructure:"cors" yaml:"cors" json:"cors"`

	// 限流配置
	RateLimit RateLimitConfig `mapstructure:"rate_limit" yaml:"rate_limit" json:"rate_limit"`

	// 访问记录
	AccessLog AccessLogConfig `mapstructure:"access_log" yaml:"access_log" json:"access_log"`

	// 请求签名
	Signature SignatureConfig `mapstructure:"signature" yaml:"signature" json:"signature"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled    bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Algorithm  string `mapstructure:"algorithm" yaml:"algorithm" json:"algorithm"` // token_bucket, sliding_window
	Rate       int    `mapstructure:"rate" yaml:"rate" json:"rate"`
	Burst      int    `mapstructure:"burst" yaml:"burst" json:"burst"`
	WindowSize int    `mapstructure:"window_size" yaml:"window_size" json:"window_size"`
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

// DefaultGatewayConfig 返回默认Gateway配置，使用go-config的链式调用风格
func DefaultGatewayConfig() *GatewayConfig {
	// 创建go-config的默认单例配置，使用优雅的链式调用
	defaultSingleConfig := &goconfig.SingleConfig{
		// Server配置 - 使用链式调用
		Server: *register.Default().
			WithModuleName(DefaultServiceName).
			WithEndpoint(":8080").
			WithServerName(DefaultServiceName).
			WithDataDriver("memory").
			WithContextPath("/").
			WithLanguage("zh-cn"),

		// CORS配置 - 使用链式调用  
		Cors: *cors.Default().
			WithModuleName("cors").
			WithAllowedOrigins([]string{"*"}).
			WithAllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}).
			WithAllowedHeaders([]string{"*"}).
			WithMaxAge("86400").
			WithAllowedAllOrigins(true).
			WithAllowCredentials(true).
			WithOptionsResponseCode(200),

		// JWT配置 - 使用链式调用
		JWT: *jwt.Default().
			WithModuleName("jwt").
			WithSigningKey("go-rpc-gateway-default-key").
			WithExpiresTime(3600).
			WithBufferTime(300).
			WithUseMultipoint(false),

		// Cache配置 - 使用链式调用
		Cache: *cache.Default().
			WithModuleName("cache").
			WithType(cache.TypeMemory).
			WithEnabled(true).
			WithKeyPrefix("gateway:").
			WithSerializer("json"),

		// PProf配置 - 使用链式调用
		Pprof: *register.DefaultPProfConfig().
			WithEnabled(true).
			WithPathPrefix("/debug/pprof").
			WithRequireAuth(false),

		// Jaeger配置 - 使用链式调用
		Jaeger: *register.DefaultJaegerConfig().
			WithType("const").
			WithParam(1).
			WithLogSpans(false).
			WithEndpoint("http://localhost:14268/api/traces").
			WithService(DefaultServiceName).
			WithModuleName("jaeger"),

		// Zap日志配置 - 使用链式调用
		Zap: *zapconfig.Default().
			WithModuleName("zap").
			WithLevel("info").
			WithFormat("console").
			WithPrefix("[GO-RPC-GATEWAY]").
			WithDirector("logs").
			WithMaxSize(100).
			WithMaxAge(7).
			WithMaxBackups(5).
			WithCompress(true).
			WithShowLine(true).
			WithEncodeLevel("LowercaseColorLevelEncoder").
			WithStacktraceKey("stacktrace").
			WithLogInConsole(true).
			WithDevelopment(true),
	}
	
	return &GatewayConfig{
		SingleConfig: defaultSingleConfig,
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
			RateLimit: RateLimitConfig{
				Enabled:    true,
				Algorithm:  "token_bucket",
				Rate:       100,
				Burst:      200,
				WindowSize: 60,
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

// LoadConfig 加载配置文件并初始化全局组件
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

		// 初始化全局组件
		if err := cm.InitGlobalComponents(); err != nil {
			return fmt.Errorf("初始化全局组件失败: %w", err)
		}

		// 使用初始化后的logger记录日志
		global.LOGGER.InfoKV("配置文件加载成功", 
			"path", cm.configPath, 
			"environment", string(cm.environment))
	} else {
		// 即使没有配置文件，也要初始化全局组件
		if err := cm.InitGlobalComponents(); err != nil {
			return fmt.Errorf("初始化全局组件失败: %w", err)
		}
	}

	return nil
}

// InitGlobalComponents 初始化全局组件，集成go-core和go-logger
func (cm *ConfigManager) InitGlobalComponents() error {
	// 1. 设置全局配置到go-core
	global.VP = cm.viper
	global.CONFIG = cm.config.SingleConfig
	
	// 2. 初始化go-logger实例
	loggerConfig := logger.DefaultConfig()
	if cm.config.SingleConfig.Zap.Prefix != "" {
		loggerConfig.Prefix = cm.config.SingleConfig.Zap.Prefix
	}
	if cm.config.SingleConfig.Zap.Level != "" {
		if level, err := logger.ParseLevel(cm.config.SingleConfig.Zap.Level); err == nil {
			loggerConfig.Level = level
		}
	}
	
	// 创建并设置全局logger
	global.LOGGER = logger.NewLogger(loggerConfig)
	
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
		// 使用go-logger记录日志
		global.LOGGER.InfoKV("配置文件发生变化", "file", e.Name)

		// 重新加载配置
		if err := cm.LoadConfig(); err != nil {
			global.LOGGER.WithError(err).ErrorMsg("重新加载配置失败")
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
