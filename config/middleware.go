/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 20:08:06
 * @FilePath: \go-rpc-gateway\config\middleware.go
 * @Description: 中间件配置模块 - 集成 go-config、go-core、go-logger、go-toolbox
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package config

// MiddlewareConfig 中间件配置 - 集成 go-config、go-core、go-logger、go-toolbox
type MiddlewareConfig struct {
	// 安全类中间件
	Security  SecurityMiddlewareConfig `mapstructure:"security" yaml:"security" json:"security"`
	Signature SignatureConfig          `mapstructure:"signature" yaml:"signature" json:"signature"`

	// 控制类中间件
	RateLimit RateLimitConfig `mapstructure:"rate_limit" yaml:"rate_limit" json:"rate_limit"`
	Recovery  RecoveryConfig  `mapstructure:"recovery" yaml:"recovery" json:"recovery"`
	RequestID RequestIDConfig `mapstructure:"request_id" yaml:"request_id" json:"request_id"`

	// 监控类中间件 (使用已存在的配置，避免冲突)
	Metrics MetricsMiddlewareConfig `mapstructure:"metrics" yaml:"metrics" json:"metrics"`
	Logging LoggingConfig           `mapstructure:"logging" yaml:"logging" json:"logging"`
	Tracing TracingMiddlewareConfig `mapstructure:"tracing" yaml:"tracing" json:"tracing"`
	Health  HealthConfig            `mapstructure:"health" yaml:"health" json:"health"`

	// 体验类中间件
	I18n      I18nConfig      `mapstructure:"i18n" yaml:"i18n" json:"i18n"`
	AccessLog AccessLogConfig `mapstructure:"access_log" yaml:"access_log" json:"access_log"`

	// 开发类中间件
	PProf   PProfConfig   `mapstructure:"pprof" yaml:"pprof" json:"pprof"`
	Banner  BannerConfig  `mapstructure:"banner" yaml:"banner" json:"banner"`
	Swagger SwaggerConfig `mapstructure:"swagger" yaml:"swagger" json:"swagger"`
}

// SecurityMiddlewareConfig 安全中间件配置 (避免与security.go中的SecurityConfig冲突)
type SecurityMiddlewareConfig struct {
	Enabled               bool              `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Headers               map[string]string `mapstructure:"headers" yaml:"headers" json:"headers"`
	XSSProtection         bool              `mapstructure:"xss_protection" yaml:"xss_protection" json:"xss_protection"`
	ContentTypeNoSniff    bool              `mapstructure:"content_type_nosniff" yaml:"content_type_nosniff" json:"content_type_nosniff"`
	FrameOptions          string            `mapstructure:"frame_options" yaml:"frame_options" json:"frame_options"`
	ContentSecurityPolicy string            `mapstructure:"content_security_policy" yaml:"content_security_policy" json:"content_security_policy"`
	ReferrerPolicy        string            `mapstructure:"referrer_policy" yaml:"referrer_policy" json:"referrer_policy"`
	HSTSMaxAge            int               `mapstructure:"hsts_max_age" yaml:"hsts_max_age" json:"hsts_max_age"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled         bool                     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Algorithm       string                   `mapstructure:"algorithm" yaml:"algorithm" json:"algorithm"` // token_bucket, sliding_window
	Rate            int                      `mapstructure:"rate" yaml:"rate" json:"rate"`
	Burst           int                      `mapstructure:"burst" yaml:"burst" json:"burst"`
	WindowSize      int                      `mapstructure:"window_size" yaml:"window_size" json:"window_size"`
	KeyFunc         string                   `mapstructure:"key_func" yaml:"key_func" json:"key_func"` // ip, user, header
	CustomKeyHeader string                   `mapstructure:"custom_key_header" yaml:"custom_key_header" json:"custom_key_header"`
	Headers         RateLimitHeadersConfig   `mapstructure:"headers" yaml:"headers" json:"headers"`
	Whitelist       RateLimitWhitelistConfig `mapstructure:"whitelist" yaml:"whitelist" json:"whitelist"`
	Rules           []RateLimitRuleConfig    `mapstructure:"rules" yaml:"rules" json:"rules"`
}

// RateLimitHeadersConfig 限流响应头配置
type RateLimitHeadersConfig struct {
	Limit     string `mapstructure:"limit" yaml:"limit" json:"limit"`
	Remaining string `mapstructure:"remaining" yaml:"remaining" json:"remaining"`
	Reset     string `mapstructure:"reset" yaml:"reset" json:"reset"`
}

// RateLimitWhitelistConfig 限流白名单配置
type RateLimitWhitelistConfig struct {
	IPs     []string          `mapstructure:"ips" yaml:"ips" json:"ips"`
	Headers map[string]string `mapstructure:"headers" yaml:"headers" json:"headers"`
}

// RateLimitRuleConfig 限流规则配置
type RateLimitRuleConfig struct {
	Path      string `mapstructure:"path" yaml:"path" json:"path"`
	Rate      int    `mapstructure:"rate" yaml:"rate" json:"rate"`
	Burst     int    `mapstructure:"burst" yaml:"burst" json:"burst"`
	Algorithm string `mapstructure:"algorithm" yaml:"algorithm" json:"algorithm"`
}

// AccessLogConfig 访问日志配置
type AccessLogConfig struct {
	Enabled        bool              `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Format         string            `mapstructure:"format" yaml:"format" json:"format"` // json, text
	IncludeBody    bool              `mapstructure:"include_body" yaml:"include_body" json:"include_body"`
	IncludeHeaders bool              `mapstructure:"include_headers" yaml:"include_headers" json:"include_headers"`
	Outputs        []LogOutputConfig `mapstructure:"outputs" yaml:"outputs" json:"outputs"`
	Filters        LogFiltersConfig  `mapstructure:"filters" yaml:"filters" json:"filters"`
}

// SignatureConfig 请求签名配置
type SignatureConfig struct {
	Enabled   bool                   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Algorithm string                 `mapstructure:"algorithm" yaml:"algorithm" json:"algorithm"` // hmac-sha256
	SecretKey string                 `mapstructure:"secret_key" yaml:"secret_key" json:"secret_key"`
	SkipPaths []string               `mapstructure:"skip_paths" yaml:"skip_paths" json:"skip_paths"`
	TTL       int                    `mapstructure:"ttl" yaml:"ttl" json:"ttl"` // 签名有效期（秒）
	Headers   SignatureHeadersConfig `mapstructure:"headers" yaml:"headers" json:"headers"`
}

// SignatureHeadersConfig 签名相关头配置
type SignatureHeadersConfig struct {
	Signature string `mapstructure:"signature" yaml:"signature" json:"signature"`
	Timestamp string `mapstructure:"timestamp" yaml:"timestamp" json:"timestamp"`
	Nonce     string `mapstructure:"nonce" yaml:"nonce" json:"nonce"`
}

// RecoveryConfig 异常恢复配置
type RecoveryConfig struct {
	Enabled          bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	PrintStack       bool   `mapstructure:"print_stack" yaml:"print_stack" json:"print_stack"`
	LogStack         bool   `mapstructure:"log_stack" yaml:"log_stack" json:"log_stack"`
	LogLevel         string `mapstructure:"log_level" yaml:"log_level" json:"log_level"`
	ErrorStatusCode  int    `mapstructure:"error_status_code" yaml:"error_status_code" json:"error_status_code"`
	ErrorMessage     string `mapstructure:"error_message" yaml:"error_message" json:"error_message"`
	IncludeStack     bool   `mapstructure:"include_stack" yaml:"include_stack" json:"include_stack"`
	DisableStackAll  bool   `mapstructure:"disable_stack_all" yaml:"disable_stack_all" json:"disable_stack_all"`
	CustomRecoveryFn string `mapstructure:"custom_recovery_fn" yaml:"custom_recovery_fn" json:"custom_recovery_fn"`
}

// RequestIDConfig 请求ID配置 - 集成 go-toolbox 的 UUID 生成
type RequestIDConfig struct {
	Enabled            bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Header             string `mapstructure:"header" yaml:"header" json:"header"`
	Generator          string `mapstructure:"generator" yaml:"generator" json:"generator"` // uuid, nanoid, snowflake, custom
	UUIDVersion        int    `mapstructure:"uuid_version" yaml:"uuid_version" json:"uuid_version"`
	NanoidLength       int    `mapstructure:"nanoid_length" yaml:"nanoid_length" json:"nanoid_length"`
	NanoidAlphabet     string `mapstructure:"nanoid_alphabet" yaml:"nanoid_alphabet" json:"nanoid_alphabet"`
	SnowflakeMachineID int    `mapstructure:"snowflake_machine_id" yaml:"snowflake_machine_id" json:"snowflake_machine_id"`
}

// MetricsMiddlewareConfig 中间件级别的监控指标配置 (扩展已有的MetricsConfig)
type MetricsMiddlewareConfig struct {
	Enabled        bool                 `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Path           string               `mapstructure:"path" yaml:"path" json:"path"`
	Port           int                  `mapstructure:"port" yaml:"port" json:"port"`
	Namespace      string               `mapstructure:"namespace" yaml:"namespace" json:"namespace"`
	Subsystem      string               `mapstructure:"subsystem" yaml:"subsystem" json:"subsystem"`
	Labels         []string             `mapstructure:"labels" yaml:"labels" json:"labels"`
	PathMapping    map[string]string    `mapstructure:"path_mapping" yaml:"path_mapping" json:"path_mapping"`
	BuiltinMetrics BuiltinMetricsConfig `mapstructure:"builtin_metrics" yaml:"builtin_metrics" json:"builtin_metrics"`
}

// BuiltinMetricsConfig 内置指标配置
type BuiltinMetricsConfig struct {
	RequestsTotal   bool `mapstructure:"requests_total" yaml:"requests_total" json:"requests_total"`
	RequestDuration bool `mapstructure:"request_duration" yaml:"request_duration" json:"request_duration"`
	RequestSize     bool `mapstructure:"request_size" yaml:"request_size" json:"request_size"`
	ResponseSize    bool `mapstructure:"response_size" yaml:"response_size" json:"response_size"`
	ActiveRequests  bool `mapstructure:"active_requests" yaml:"active_requests" json:"active_requests"`
}

// LoggingConfig 日志配置 - 集成 go-logger
type LoggingConfig struct {
	Enabled      bool              `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Level        string            `mapstructure:"level" yaml:"level" json:"level"`
	Format       string            `mapstructure:"format" yaml:"format" json:"format"` // json, text, custom
	Outputs      []LogOutputConfig `mapstructure:"outputs" yaml:"outputs" json:"outputs"`
	Fields       LogFieldsConfig   `mapstructure:"fields" yaml:"fields" json:"fields"`
	Filters      LogFiltersConfig  `mapstructure:"filters" yaml:"filters" json:"filters"`
	CustomFields map[string]string `mapstructure:"custom_fields" yaml:"custom_fields" json:"custom_fields"`
}

// LogOutputConfig 日志输出配置
type LogOutputConfig struct {
	Type       string `mapstructure:"type" yaml:"type" json:"type"` // file, stdout, syslog
	Path       string `mapstructure:"path" yaml:"path" json:"path"`
	MaxSize    int    `mapstructure:"max_size" yaml:"max_size" json:"max_size"`
	MaxBackups int    `mapstructure:"max_backups" yaml:"max_backups" json:"max_backups"`
	MaxAge     int    `mapstructure:"max_age" yaml:"max_age" json:"max_age"`
	Colored    bool   `mapstructure:"colored" yaml:"colored" json:"colored"`
	Network    string `mapstructure:"network" yaml:"network" json:"network"`
	Address    string `mapstructure:"address" yaml:"address" json:"address"`
}

// LogFieldsConfig 日志字段配置
type LogFieldsConfig struct {
	Timestamp      bool `mapstructure:"timestamp" yaml:"timestamp" json:"timestamp"`
	RequestID      bool `mapstructure:"request_id" yaml:"request_id" json:"request_id"`
	RemoteAddr     bool `mapstructure:"remote_addr" yaml:"remote_addr" json:"remote_addr"`
	Method         bool `mapstructure:"method" yaml:"method" json:"method"`
	URI            bool `mapstructure:"uri" yaml:"uri" json:"uri"`
	Protocol       bool `mapstructure:"protocol" yaml:"protocol" json:"protocol"`
	StatusCode     bool `mapstructure:"status_code" yaml:"status_code" json:"status_code"`
	ResponseSize   bool `mapstructure:"response_size" yaml:"response_size" json:"response_size"`
	RequestSize    bool `mapstructure:"request_size" yaml:"request_size" json:"request_size"`
	UserAgent      bool `mapstructure:"user_agent" yaml:"user_agent" json:"user_agent"`
	Referer        bool `mapstructure:"referer" yaml:"referer" json:"referer"`
	Latency        bool `mapstructure:"latency" yaml:"latency" json:"latency"`
	IncludeBody    bool `mapstructure:"include_body" yaml:"include_body" json:"include_body"`
	IncludeHeaders bool `mapstructure:"include_headers" yaml:"include_headers" json:"include_headers"`
}

// LogFiltersConfig 日志过滤配置
type LogFiltersConfig struct {
	IgnorePaths       []string `mapstructure:"ignore_paths" yaml:"ignore_paths" json:"ignore_paths"`
	IgnoreStatusCodes []int    `mapstructure:"ignore_status_codes" yaml:"ignore_status_codes" json:"ignore_status_codes"`
	IgnoreUserAgents  []string `mapstructure:"ignore_user_agents" yaml:"ignore_user_agents" json:"ignore_user_agents"`
	MinLatency        string   `mapstructure:"min_latency" yaml:"min_latency" json:"min_latency"`
}
