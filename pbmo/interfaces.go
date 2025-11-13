// Package pbmo provides Protocol Buffer Model Object conversion interfaces
package pbmo

import (
	"context"
	"reflect"
	"time"
)

// ===============================
// 核心转换器接口定义
// ===============================

// Converter 核心转换器接口
type Converter interface {
	// 基础转换方法
	ConvertPBToModel(pb interface{}, model interface{}) error
	ConvertModelToPB(model interface{}, pb interface{}) error

	// 批量转换方法
	BatchConvertPBToModel(pbSlice interface{}, modelSlicePtr interface{}) error
	BatchConvertModelToPB(modelSlice interface{}, pbSlicePtr interface{}) error

	// 切片转换方法
	ConvertSlice(src interface{}) (interface{}, error)

	// 转换器信息
	GetConverterInfo() ConverterInfo
}

// EnhancedConverter 增强转换器接口
type EnhancedConverter interface {
	Converter

	// 字段转换器注册
	RegisterTransformer(field string, transformer func(interface{}) interface{})

	// 字段映射注册
	RegisterFieldMapping(pbField, modelField string)

	// 获取转换统计
	GetConversionStats() ConversionStats
}

// SafeConverter 安全转换器接口 - 具体实现在safe_converter.go中
// type SafeConverter interface { ... } // 实际定义在safe_converter.go

// ValidatingConverter 校验转换器接口
type ValidatingConverter interface {
	Converter

	// 校验相关方法
	RegisterValidationRules(typeName string, rules ...FieldRule)
	ValidateAndConvert(pb interface{}, model interface{}) error
	GetValidationRules(typeName string) []FieldRule

	// 校验控制
	EnableValidation() ValidatingConverter
	DisableValidation() ValidatingConverter
	IsValidationEnabled() bool
}

// DesensitizingConverter 脱敏转换器接口
type DesensitizingConverter interface {
	Converter

	// 脱敏相关方法
	WithDesensitization(enabled bool) DesensitizingConverter
	RegisterDesensitizer(fieldName string, desensitizer Desensitizer)
	ApplyDesensitization(obj interface{}) error

	// 脱敏控制
	IsDesensitizationEnabled() bool
	GetDesensitizers() map[string]Desensitizer
}

// ConcurrentConverter 并发转换器接口
type ConcurrentConverter interface {
	Converter

	// 并发转换方法
	ConcurrentConvert(ctx context.Context, inputs []interface{}) *ConcurrentResult
	BatchConvertWithConcurrency(inputs interface{}, opts ...interface{}) *ConcurrentResult

	// 并发配置
	SetConcurrencyConfig(config ConcurrencyConfig)
	GetConcurrencyConfig() ConcurrencyConfig
}

// AdvancedConverterInterface 高级转换器接口 (组合所有功能) - 具体实现在advanced_api.go中
// type AdvancedConverterInterface interface { ... } // 实际定义在advanced_api.go// ===============================
// 脱敏相关接口
// ===============================

// Desensitizer 脱敏器接口
type Desensitizer interface {
	Desensitize(value string) string
	GetType() string
	GetDescription() string
}

// DesensitizationRule 脱敏规则接口
type DesensitizationRule interface {
	AppliesTo(fieldName string, fieldType reflect.Type) bool
	GetDesensitizer() Desensitizer
	GetPriority() int
}

// DesensitizationManager 脱敏管理器接口
type DesensitizationManager interface {
	RegisterRule(rule DesensitizationRule)
	RegisterDesensitizer(name string, desensitizer Desensitizer)
	ApplyDesensitization(obj interface{}) error
	GetAvailableDesensitizers() []string
}

// ===============================
// 服务集成接口
// ===============================

// ServiceIntegration 服务集成接口 - 具体实现在service_integration.go中
// type ServiceIntegration interface { ... } // 实际定义在service_integration.go// ===============================
// 结果和统计接口
// ===============================

// ConversionResult 转换结果接口 - 具体实现在advanced_api.go中
// type ConversionResult interface { ... } // 实际定义在advanced_api.go

// ConcurrentResult 并发转换结果接口
type ConcurrentResult interface {
	GetSuccessCount() int
	GetFailedCount() int
	GetErrors() []error
	GetData() []interface{}
	GetElapsed() time.Duration
	GetMetrics() ConcurrentMetrics
}

// MetricsCollector 指标收集器接口
type MetricsCollector interface {
	RecordConversion(success bool, elapsed time.Duration)
	RecordValidation(success bool, elapsed time.Duration)
	RecordDesensitization(success bool, elapsed time.Duration)
	GetMetrics() GlobalMetrics
	Reset()
}

// ===============================
// 配置和选项接口
// ===============================

// ConfigurableConverter 可配置转换器接口
type ConfigurableConverter interface {
	// 配置管理
	LoadConfig(config interface{}) error
	SaveConfig() (interface{}, error)
	ResetToDefaults()

	// 动态配置
	UpdateConfig(updates map[string]interface{}) error
	GetConfigValue(key string) interface{}
	SetConfigValue(key string, value interface{})
}

// PluginManager 插件管理器接口
type PluginManager interface {
	// 插件注册
	RegisterPlugin(name string, plugin Plugin)
	UnregisterPlugin(name string)

	// 插件生命周期
	EnablePlugin(name string) error
	DisablePlugin(name string) error

	// 插件查询
	GetPlugins() []PluginInfo
	IsPluginEnabled(name string) bool
}

// Plugin 插件接口
type Plugin interface {
	GetName() string
	GetVersion() string
	GetDescription() string
	Initialize() error
	Shutdown() error
	IsEnabled() bool
}

// ===============================
// 工厂接口
// ===============================

// ConverterFactory 转换器工厂接口
type ConverterFactory interface {
	// 基础转换器创建
	NewBidiConverter(pb, model interface{}) EnhancedConverter
	NewSafeConverter(pb, model interface{}) SafeConverter

	// 特殊转换器创建
	NewDesensitizingConverter(pb, model interface{}) DesensitizingConverter
	NewValidatingConverter(pb, model interface{}) ValidatingConverter
	NewConcurrentConverter(pb, model interface{}) ConcurrentConverter

	// 高级转换器创建 - AdvancedConverter定义在advanced_api.go中
	// NewAdvancedConverter(pb, model interface{}, opts ...AdvancedOption) *AdvancedConverter

	// 服务集成创建
	NewServiceIntegration() ServiceIntegration
	NewPluginManager() PluginManager
}

// ===============================
// 数据结构定义
// ===============================

// ConverterInfo 转换器信息
type ConverterInfo struct {
	Type        string                 `json:"type"`
	Version     string                 `json:"version"`
	Features    []string               `json:"features"`
	Performance PerformanceInfo        `json:"performance"`
	Config      map[string]interface{} `json:"config"`
}

// ConversionStats 转换统计
type ConversionStats struct {
	TotalConversions      int64         `json:"total_conversions"`
	SuccessfulConversions int64         `json:"successful_conversions"`
	FailedConversions     int64         `json:"failed_conversions"`
	AverageLatency        time.Duration `json:"average_latency"`
	LastConversion        time.Time     `json:"last_conversion"`
}

// PerformanceInfo 性能信息
type PerformanceInfo struct {
	BenchmarkScore   float64       `json:"benchmark_score"`
	AverageLatency   time.Duration `json:"average_latency"`
	ThroughputPerSec int64         `json:"throughput_per_sec"`
	MemoryUsage      int64         `json:"memory_usage"`
}

// ConversionMetrics 转换指标 - 具体实现在enhanced_converter.go中
// type ConversionMetrics struct { ... } // 实际定义在enhanced_converter.go

// ConcurrentMetrics 并发指标
type ConcurrentMetrics struct {
	TotalTasks     int           `json:"total_tasks"`
	CompletedTasks int           `json:"completed_tasks"`
	FailedTasks    int           `json:"failed_tasks"`
	Goroutines     int           `json:"goroutines"`
	TotalDuration  time.Duration `json:"total_duration"`
	AverageLatency time.Duration `json:"average_latency"`
}

// GlobalMetrics 全局指标
type GlobalMetrics struct {
	Conversions      ConversionStats      `json:"conversions"`
	Validations      ValidationStats      `json:"validations"`
	Desensitizations DesensitizationStats `json:"desensitizations"`
	SystemMetrics    SystemMetrics        `json:"system_metrics"`
}

// ValidationStats 校验统计
type ValidationStats struct {
	TotalValidations      int64         `json:"total_validations"`
	SuccessfulValidations int64         `json:"successful_validations"`
	FailedValidations     int64         `json:"failed_validations"`
	AverageLatency        time.Duration `json:"average_latency"`
}

// DesensitizationStats 脱敏统计
type DesensitizationStats struct {
	TotalDesensitizations      int64         `json:"total_desensitizations"`
	SuccessfulDesensitizations int64         `json:"successful_desensitizations"`
	FailedDesensitizations     int64         `json:"failed_desensitizations"`
	AverageLatency             time.Duration `json:"average_latency"`
	FieldsCovered              []string      `json:"fields_covered"`
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	CPUUsage       float64 `json:"cpu_usage"`
	MemoryUsage    int64   `json:"memory_usage"`
	GoroutineCount int     `json:"goroutine_count"`
	GCStats        GCStats `json:"gc_stats"`
}

// GCStats 垃圾回收统计
type GCStats struct {
	NumGC      uint32        `json:"num_gc"`
	PauseTotal time.Duration `json:"pause_total"`
	LastPause  time.Duration `json:"last_pause"`
	NextGC     uint64        `json:"next_gc"`
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Name         string                 `json:"name"`
	Type         string                 `json:"type"` // "grpc" or "rest"
	Version      string                 `json:"version"`
	Converter    string                 `json:"converter"`
	Metadata     map[string]interface{} `json:"metadata"`
	RegisteredAt time.Time              `json:"registered_at"`
}

// PluginInfo 插件信息
type PluginInfo struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	LoadedAt    time.Time `json:"loaded_at"`
}

// ConcurrencyConfig 并发配置 - 具体实现在advanced_api.go中
// type ConcurrencyConfig struct { ... } // 实际定义在advanced_api.go

// ===============================
// 选项模式接口
// ===============================

// Option 通用选项接口
type Option interface {
	Apply(config interface{}) error
	GetDescription() string
}

// AdvancedOption 高级转换器选项 - 具体实现在advanced_api.go中
// type AdvancedOption func(*AdvancedConverter) // 实际定义在advanced_api.go

// ConcurrencyOption 并发选项 - ConcurrencyConfig定义在advanced_api.go中
// type ConcurrencyOption func(*ConcurrencyConfig) // 实际定义在advanced_api.go

// ValidationOption 校验选项 - ValidationConfig定义在advanced_api.go中
// type ValidationOption func(*ValidationConfig) // 实际定义在advanced_api.go

// DesensitizationOption 脱敏选项 - DesensitizationConfig定义在advanced_api.go中
// type DesensitizationOption func(*DesensitizationConfig) // 实际定义在advanced_api.go

// ===============================
// 扩展配置结构
// ===============================

// DesensitizationConfig 脱敏配置 - 具体实现在advanced_api.go中
// type DesensitizationConfig struct { ... } // 实际定义在advanced_api.go

// ===============================
// 常量定义
// ===============================

// ConverterType 转换器类型
type ConverterType string

const (
	BidiConverterType          ConverterType = "bidi"
	EnhancedConverterType      ConverterType = "enhanced"
	SafeConverterType          ConverterType = "safe"
	ValidatingConverterType    ConverterType = "validating"
	DesensitizingConverterType ConverterType = "desensitizing"
	ConcurrentConverterType    ConverterType = "concurrent"
	AdvancedConverterType      ConverterType = "advanced"
)

// Feature 功能特性
type Feature string

const (
	ValidationFeature      Feature = "validation"
	DesensitizationFeature Feature = "desensitization"
	ConcurrencyFeature     Feature = "concurrency"
	SafeAccessFeature      Feature = "safe_access"
	MetricsFeature         Feature = "metrics"
	PluginFeature          Feature = "plugin"
)

// ===============================
// 错误接口 - 具体实现在各自的文件中
// ===============================

// ConversionError 转换错误接口 - 具体实现在safe_converter.go
// type ConversionError interface { ... } // 实际定义在safe_converter.go

// ValidationError 校验错误接口 - 具体实现在validator.go
// type ValidationError interface { ... } // 实际定义在validator.go

// DesensitizationError 脱敏错误接口 - 如需要可在相应文件中实现
// type DesensitizationError interface { ... } // 可在desensitization相关文件中定义
