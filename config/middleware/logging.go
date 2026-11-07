/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-04 13:20:28
 * @FilePath: \go-rpc-gateway\config\middleware\logging.go
 * @Description: 日志中间件配置模块
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"os"
	"time"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/logger"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Logging 日志中间件配置
type Logging struct {
	ModuleName           string            `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`                                 // 模块名称
	Enabled              bool              `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                                            // 是否启用日志
	Level                string            `mapstructure:"level" yaml:"level" json:"level"`                                                  // 日志级别 (debug, info, warn, error)
	Format               logger.FormatType `mapstructure:"format" yaml:"format" json:"format"`                                               // 日志格式 (json, text, xml, csv)
	Prefix               string            `mapstructure:"prefix" yaml:"prefix" json:"prefix"`                                               // 日志前缀（如：[WSC]）
	ShowCaller           bool              `mapstructure:"show-caller" yaml:"show-caller" json:"showCaller"`                                 // 是否显示调用者信息
	Colorful             bool              `mapstructure:"colorful" yaml:"colorful" json:"colorful"`                                         // 是否使用彩色输出
	TimeFormat           string            `mapstructure:"time-format" yaml:"time-format" json:"timeFormat"`                                 // 时间格式（如：2006-01-02 15:04:05.000）
	Output               logger.OutputType `mapstructure:"output" yaml:"output" json:"output"`                                               // 输出目标 (console, file, rotate, stdout, stderr)
	FilePath             string            `mapstructure:"file-path" yaml:"file-path" json:"filePath"`                                       // 日志文件路径
	MaxSize              int               `mapstructure:"max-size" yaml:"max-size" json:"maxSize"`                                          // 最大文件大小(MB)
	MaxBackups           int               `mapstructure:"max-backups" yaml:"max-backups" json:"maxBackups"`                                 // 最大备份文件数
	MaxAge               int               `mapstructure:"max-age" yaml:"max-age" json:"maxAge"`                                             // 最大保存天数
	Compress             bool              `mapstructure:"compress" yaml:"compress" json:"compress"`                                         // 是否压缩
	FilePermission       uint32            `mapstructure:"file-permission" yaml:"file-permission" json:"filePermission"`                     // 文件权限（十进制值，如：420 表示 0644）
	BufferSize           int               `mapstructure:"buffer-size" yaml:"buffer-size" json:"bufferSize"`                                 // 缓冲区大小(字节)
	SkipPaths            []string          `mapstructure:"skip-paths" yaml:"skip-paths" json:"skipPaths"`                                    // 跳过的路径
	EnableRequest        bool              `mapstructure:"enable-request" yaml:"enable-request" json:"enableRequest"`                        // 是否记录请求
	EnableResponse       bool              `mapstructure:"enable-response" yaml:"enable-response" json:"enableResponse"`                     // 是否记录响应
	MaxBodySize          int               `mapstructure:"max-body-size" yaml:"max-body-size" json:"maxBodySize"`                            // 最大日志体大小(字节)
	SensitiveMask        string            `mapstructure:"sensitive-mask" yaml:"sensitive-mask" json:"sensitiveMask"`                        // 敏感数据掩码
	SensitiveKeys        []string          `mapstructure:"sensitive-keys" yaml:"sensitive-keys" json:"sensitiveKeys"`                        // 敏感字段关键词
	SlowHTTPThreshold    int64             `mapstructure:"slow-http-threshold" yaml:"slow-http-threshold" json:"slowHttpThreshold"`          // HTTP慢请求阈值(毫秒)
	SlowGRPCThreshold    int64             `mapstructure:"slow-grpc-threshold" yaml:"slow-grpc-threshold" json:"slowGrpcThreshold"`          // GRPC慢请求阈值(毫秒)
	SlowStreamThreshold  int64             `mapstructure:"slow-stream-threshold" yaml:"slow-stream-threshold" json:"slowStreamThreshold"`    // 流式请求慢请求阈值(毫秒)
	LoggableContentTypes []string          `mapstructure:"loggable-content-types" yaml:"loggable-content-types" json:"loggableContentTypes"` // 可记录的 Content-Type
}

// GetLogLevel 获取 go-logger 的日志级别
func (l *Logging) GetLogLevel() string {
	return l.Level
}

// GetFormatterType 获取 go-logger 的格式化类型
func (l *Logging) GetFormatterType() logger.FormatType {
	return l.Format
}

// GetOutputType 获取 go-logger 的输出类型
func (l *Logging) GetOutputType() logger.OutputType {
	return l.Output
}

// DefaultLogging 创建默认日志配置
func DefaultLogging() *Logging {
	return &Logging{
		ModuleName:     "logging",
		Enabled:        true,
		Level:          "debug",
		Format:         logger.FormatJSON,
		Prefix:         "",
		ShowCaller:     false,
		Colorful:       true,
		TimeFormat:     time.RFC3339Nano,
		Output:         logger.OutputStdout,
		FilePath:       "/var/log/app.log",
		MaxSize:        100, // 100MB（会转换为字节）
		MaxBackups:     logger.DefaultMaxFiles,
		MaxAge:         28,
		Compress:       true,
		FilePermission: logger.DefaultFilePermission,
		BufferSize:     logger.DefaultBufferSize,
		SkipPaths:      []string{"/health", "/metrics", "/favicon.ico", "/ping", "/readiness", "/liveness"},
		EnableRequest:  true,
		EnableResponse: false,
		MaxBodySize:    2048,
		SensitiveMask:  "***REDACTED***",
		SensitiveKeys: []string{
			"password", "passwd", "token", "access_token", "refresh_token",
			"secret", "authorization", "api_key", "apikey",
			"mobile", "phone", "id_card", "credit_card",
		},
		SlowHTTPThreshold:    1000,
		SlowGRPCThreshold:    1000,
		SlowStreamThreshold:  5000,
		LoggableContentTypes: []string{"application/json", "application/xml", "application/x-www-form-urlencoded", "text/"},
	}
}

// Get 返回配置接口
func (l *Logging) Get() any {
	return l
}

// Set 设置配置数据
func (l *Logging) Set(data any) {
	if cfg, ok := data.(*Logging); ok {
		*l = *cfg
	}
}

// Clone 返回配置的副本
func (l *Logging) Clone() internal.Configurable {
	var cloned Logging
	if err := syncx.DeepCopy(&cloned, l); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Logging{}
	}
	return &cloned
}

// ========== Logging 链式调用方法 ==========

// WithModuleName 设置模块名称
func (l *Logging) WithModuleName(moduleName string) *Logging {
	l.ModuleName = moduleName
	return l
}

// WithEnabled 设置是否启用日志
func (l *Logging) WithEnabled(enabled bool) *Logging {
	l.Enabled = enabled
	return l
}

// EnableLogging 启用日志
func (l *Logging) EnableLogging() *Logging {
	l.Enabled = true
	return l
}

// WithLevel 设置日志级别
func (l *Logging) WithLevel(level string) *Logging {
	l.Level = level
	return l
}

// WithFormat 设置日志格式
func (l *Logging) WithFormat(format logger.FormatType) *Logging {
	l.Format = format
	return l
}

// WithTimeFormat 设置时间格式
func (l *Logging) WithTimeFormat(timeFormat string) *Logging {
	l.TimeFormat = timeFormat
	return l
}

// WithOutput 设置输出目标
func (l *Logging) WithOutput(output logger.OutputType) *Logging {
	l.Output = output
	return l
}

// WithFilePath 设置日志文件路径
func (l *Logging) WithFilePath(filePath string) *Logging {
	l.FilePath = filePath
	return l
}

// WithMaxSize 设置最大文件大小
func (l *Logging) WithMaxSize(maxSize int) *Logging {
	l.MaxSize = maxSize
	return l
}

// WithMaxBackups 设置最大备份文件数
func (l *Logging) WithMaxBackups(maxBackups int) *Logging {
	l.MaxBackups = maxBackups
	return l
}

// WithMaxAge 设置最大保存天数
func (l *Logging) WithMaxAge(maxAge int) *Logging {
	l.MaxAge = maxAge
	return l
}

// WithCompress 设置是否压缩
func (l *Logging) WithCompress(compress bool) *Logging {
	l.Compress = compress
	return l
}

// WithFilePermission 设置文件权限
func (l *Logging) WithFilePermission(permission uint32) *Logging {
	l.FilePermission = permission
	return l
}

// WithBufferSize 设置缓冲区大小
func (l *Logging) WithBufferSize(bufferSize int) *Logging {
	l.BufferSize = bufferSize
	return l
}

// WithSkipPaths 设置跳过的路径
func (l *Logging) WithSkipPaths(skipPaths []string) *Logging {
	l.SkipPaths = skipPaths
	return l
}

// AddSkipPath 添加跳过的路径
func (l *Logging) AddSkipPath(path string) *Logging {
	l.SkipPaths = append(l.SkipPaths, path)
	return l
}

// WithEnableRequest 设置是否记录请求
func (l *Logging) WithEnableRequest(enableRequest bool) *Logging {
	l.EnableRequest = enableRequest
	return l
}

// WithEnableResponse 设置是否记录响应
func (l *Logging) WithEnableResponse(enableResponse bool) *Logging {
	l.EnableResponse = enableResponse
	return l
}

// EnableRequestResponse 同时启用请求和响应记录
func (l *Logging) EnableRequestResponse() *Logging {
	l.EnableRequest = true
	l.EnableResponse = true
	return l
}

// Validate 验证配置
func (l *Logging) Validate() error {
	return internal.ValidateStruct(l)
}

// Enable 启用日志中间件
func (l *Logging) Enable() *Logging {
	return l.EnableLogging()
}

// Disable 禁用日志中间件
func (l *Logging) Disable() *Logging {
	l.Enabled = false
	return l
}

// IsEnabled 检查是否启用
func (l *Logging) IsEnabled() bool {
	return l.Enabled
}

// ToLoggerInstance 将 Logging 配置转换为 go-logger 的 Logger 实例
func (l *Logging) ToLoggerInstance() *logger.Logger {
	level, _ := logger.ParseLevel(l.Level)

	// 创建基础 logger 实例
	loggerInstance := logger.NewLogger().
		WithLevel(level).
		WithPrefix(l.Prefix).
		WithShowCaller(l.ShowCaller).
		WithColorful(l.Colorful).
		WithTimeFormat(l.TimeFormat).
		WithFormat(l.Format)

	// 使用 go-logger 提供的 CreateWriter 创建输出器
	writerConfig := l.toWriterConfig()
	writer, err := logger.CreateWriter(writerConfig)
	if err != nil {
		// 如果创建失败，使用默认的控制台输出
		writer = logger.NewConsoleWriter(logger.WithConsoleOutput(os.Stdout))
	}

	loggerInstance.WithOutput(writer)

	return loggerInstance
}

// toWriterConfig 将 Logging 配置转换为 go-logger 的 WriterConfig
func (l *Logging) toWriterConfig() *logger.WriterConfig {
	return &logger.WriterConfig{
		Type:       l.Output,
		FilePath:   l.FilePath,
		MaxSize:    mathx.IfNotZero(int64(l.MaxSize)*1024*1024, logger.DefaultMaxSize), // MB -> 字节
		MaxFiles:   mathx.IfNotZero(l.MaxBackups, 0),
		MaxAge:     mathx.IfNotZero(l.MaxAge, 0),
		Compress:   l.Compress,
		Permission: mathx.IfNotZero(os.FileMode(l.FilePermission), logger.DefaultFilePermission),
		BufferSize: mathx.IfNotZero(l.BufferSize, logger.DefaultBufferSize),
	}
}
