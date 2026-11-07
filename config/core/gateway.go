/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 15:17:09
 * @FilePath: \go-rpc-gateway\config\core\gateway.go
 * @Description: Gateway网关统一配置模块
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package core

import (
	"time"

	"github.com/kamalyes/go-rpc-gateway/config/cache"
	"github.com/kamalyes/go-rpc-gateway/config/database"
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-rpc-gateway/config/kafka"
	"github.com/kamalyes/go-rpc-gateway/config/middleware"
	gcmiddleware "github.com/kamalyes/go-rpc-gateway/config/middleware"
	"github.com/kamalyes/go-rpc-gateway/config/queue"
	"github.com/kamalyes/go-rpc-gateway/config/smtp"
	"github.com/kamalyes/go-rpc-gateway/config/swagger"
	"github.com/kamalyes/go-rpc-gateway/config/tsdb"
	"github.com/kamalyes/go-toolbox/pkg/convert"
	"github.com/kamalyes/go-toolbox/pkg/osx"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
	"github.com/kamalyes/go-toolbox/pkg/types"
)

// Gateway网关统一配置
type Gateway struct {
	ModuleName     string                   `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`             // 模块名称
	Name           string                   `mapstructure:"name" yaml:"name" json:"name"`                                 // 网关名称
	Enabled        bool                     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                        // 是否启用网关
	Debug          bool                     `mapstructure:"debug" yaml:"debug" json:"debug"`                              // 是否启用调试模式
	Version        string                   `mapstructure:"version" yaml:"version" json:"version"`                        // 版本号
	Environment    string                   `mapstructure:"environment" yaml:"environment" json:"environment"`            // 环境 (dev, test, prod)
	BuildTime      string                   `mapstructure:"build-time" yaml:"build-time" json:"buildTime"`                // 构建时间
	BuildUser      string                   `mapstructure:"build-user" yaml:"build-user" json:"buildUser"`                // 构建用户
	GoVersion      string                   `mapstructure:"go-version" yaml:"go-version" json:"goVersion"`                // Go版本
	GitCommit      string                   `mapstructure:"git-commit" yaml:"git-commit" json:"gitCommit"`                // Git提交哈希
	GitBranch      string                   `mapstructure:"git-branch" yaml:"git-branch" json:"gitBranch"`                // Git分支
	GitTag         string                   `mapstructure:"git-tag" yaml:"git-tag" json:"gitTag"`                         // Git标签
	JSON           *JSON                    `mapstructure:"json" yaml:"json" json:"json"`                                 // JSON序列化配置
	HTTPServer     *HTTPServer              `mapstructure:"http" yaml:"http" json:"http"`                                 // HTTP服务器配置（主监听器）
	Listeners      []*Listener              `mapstructure:"listeners" yaml:"listeners" json:"listeners"`                  // 命名监听器列表（支持多端口，如 Ops/Tenant 分离）
	GRPC           *GRPC                    `mapstructure:"grpc" yaml:"grpc" json:"grpc"`                                 // GRPC配置
	Cache          *cache.Cache             `mapstructure:"cache" yaml:"cache" json:"cache"`                              // 缓存配置(包含Redis)
	Database       *database.Database       `mapstructure:"database" yaml:"database" json:"database"`                     // 数据库统一配置
	ClickHouse     *tsdb.ClickHouse         `mapstructure:"clickhouse" yaml:"clickhouse" json:"clickhouse"`               // ClickHouse数据库配置
	Kafka          *kafka.Kafka             `mapstructure:"kafka" yaml:"kafka" json:"kafka"`                              // Kafka配置
	Mqtt           *queue.Mqtt              `mapstructure:"mqtt" yaml:"mqtt" json:"mqtt"`                                 // MQTT配置
	Nats           *queue.Nats              `mapstructure:"nats" yaml:"nats" json:"nats"`                                 // NATS配置
	Smtp           *smtp.Smtp               `mapstructure:"smtp" yaml:"smtp" json:"smtp"`                                 // SMTP邮件服务配置
	Health         *Health                  `mapstructure:"health" yaml:"health" json:"health"`                           // 健康检查配置
	Monitoring     *gcmiddleware.Monitoring `mapstructure:"monitoring" yaml:"monitoring" json:"monitoring"`               // 监控配置
	Security       *gcmiddleware.Security   `mapstructure:"security" yaml:"security" json:"security"`                     // 安全配置
	RequestContext *RequestContext          `mapstructure:"request-context" yaml:"request-context" json:"requestContext"` // 统一请求提取配置
	Middleware     *middleware.Middleware   `mapstructure:"middleware" yaml:"middleware" json:"middleware"`               // 中间件配置
	CORS           *gcmiddleware.Cors       `mapstructure:"cors" yaml:"cors" json:"cors"`                                 // CORS配置
	Swagger        *swagger.Swagger         `mapstructure:"swagger" yaml:"swagger" json:"swagger"`                        // Swagger配置
	Banner         *Banner                  `mapstructure:"banner" yaml:"banner" json:"banner"`                           // Banner配置
	RateLimit      *gcmiddleware.RateLimit  `mapstructure:"rate-limit" yaml:"rate-limit" json:"rateLimit"`                // 限流配置
	Extensions     map[string]any           `mapstructure:"extensions" yaml:"extensions" json:"extensions"`               // 扩展配置字段，供第三方自定义使用
}

// Default 创建默认Gateway配置
func Default() *Gateway {
	return &Gateway{
		ModuleName:     "gateway",
		Name:           "Go RPC Gateway",
		Enabled:        true,
		Debug:          true,
		Version:        "v1.0.0",
		Environment:    "dev",
		BuildTime:      time.Now().Format(time.RFC3339),
		BuildUser:      "kamalyes",
		GoVersion:      "1.25.1",
		GitCommit:      osx.HashUnixMicroCipherText(),
		GitBranch:      "master",
		GitTag:         "v1.0.0",
		JSON:           DefaultJSON(),
		HTTPServer:     DefaultHTTPServer(),
		Listeners:      []*Listener{}, // 默认无额外监听器
		GRPC:           DefaultGRPC(),
		Cache:          cache.Default(),
		Database:       database.DefaultDatabaseConfig(),
		Kafka:          kafka.Default(),
		Mqtt:           queue.Default(),
		Nats:           queue.DefaultNatsPtr(),
		Smtp:           smtp.Default(),
		Health:         DefaultHealth(),
		Monitoring:     gcmiddleware.DefaultMonitoring(),
		Security:       gcmiddleware.DefaultSecurity(),
		RequestContext: DefaultRequestContext(),
		Middleware:     middleware.Default(),
		CORS:           gcmiddleware.DefaultCors(),
		RateLimit:      gcmiddleware.DefaultRateLimit(),
		Swagger:        swagger.Default(),
		Banner:         DefaultBanner(),
		Extensions:     make(map[string]any),
	}
}

// ToYAML 导出Gateway配置为YAML格式
func (c *Gateway) ToYAML() (string, error) {
	return internal.ExportConfigToYAML(c)
}

// ToJSON 导出Gateway配置为JSON格式
func (c *Gateway) ToJSON() (string, error) {
	return internal.ExportConfigToJSON(c)
}

// GenerateDefaultYAML 生成默认Gateway配置的YAML
func GenerateDefaultYAML() (string, error) {
	return internal.GenerateYAMLFromDefault(func() any {
		return Default()
	})
}

// GenerateDefaultJSON 生成默认Gateway配置的JSON
func GenerateDefaultJSON() (string, error) {
	return internal.GenerateJSONFromDefault(func() any {
		return Default()
	})
}

// Get 返回配置接口
func (c *Gateway) Get() any {
	return c
}

// Set 设置配置数据
func (c *Gateway) Set(data any) {
	if cfg, ok := data.(*Gateway); ok {
		*c = *cfg
	}
}

// Clone 返回配置的副本
func (c *Gateway) Clone() internal.Configurable {
	var cloned Gateway
	if err := syncx.DeepCopy(&cloned, c); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Gateway{}
	}
	return &cloned
}

// Validate 验证配置
func (c *Gateway) Validate() error {
	if err := internal.ValidateStruct(c); err != nil {
		return err
	}

	// 验证子配置
	if c.JSON != nil {
		if err := c.JSON.Validate(); err != nil {
			return err
		}
	}
	if c.Cache != nil {
		if err := c.Cache.Validate(); err != nil {
			return err
		}
	}
	if c.Database != nil {
		if err := c.Database.Validate(); err != nil {
			return err
		}
	}
	// Kafka是值类型，不需要nil检查
	if err := c.Kafka.Validate(); err != nil {
		return err
	}
	if c.Mqtt != nil {
		if err := c.Mqtt.Validate(); err != nil {
			return err
		}
	}
	if c.Nats != nil {
		if err := c.Nats.Validate(); err != nil {
			return err
		}
	}
	if c.Smtp != nil {
		if err := c.Smtp.Validate(); err != nil {
			return err
		}
	}
	if c.Health != nil {
		if err := c.Health.Validate(); err != nil {
			return err
		}
	}
	if c.Monitoring != nil {
		if err := c.Monitoring.Validate(); err != nil {
			return err
		}
	}
	if c.Security != nil {
		if err := c.Security.Validate(); err != nil {
			return err
		}
	}
	if c.RequestContext != nil {
		if err := c.RequestContext.Validate(); err != nil {
			return err
		}
	}
	if c.Middleware != nil {
		if err := c.Middleware.Validate(); err != nil {
			return err
		}
	}
	if c.CORS != nil {
		if err := c.CORS.Validate(); err != nil {
			return err
		}
	}
	if c.RateLimit != nil {
		if err := c.RateLimit.Validate(); err != nil {
			return err
		}
	}
	if c.Swagger != nil {
		if err := c.Swagger.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// WithModuleName 设置模块名称
func (c *Gateway) WithModuleName(moduleName string) *Gateway {
	c.ModuleName = moduleName
	return c
}

// WithName 设置网关名称
func (c *Gateway) WithName(name string) *Gateway {
	c.Name = name
	return c
}

// WithEnabled 设置是否启用网关
func (c *Gateway) WithEnabled(enabled bool) *Gateway {
	c.Enabled = enabled
	return c
}

// WithDebug 设置调试模式
func (c *Gateway) WithDebug(debug bool) *Gateway {
	c.Debug = debug
	return c
}

// WithVersion 设置版本号
func (c *Gateway) WithVersion(version string) *Gateway {
	c.Version = version
	return c
}

// WithEnvironment 设置环境
func (c *Gateway) WithEnvironment(environment string) *Gateway {
	c.Environment = environment
	return c
}

// WithServer 设置服务器配置
func (c *Gateway) WithServer(host string, port int) *Gateway {
	if c.HTTPServer == nil {
		c.HTTPServer = &HTTPServer{}
	}
	c.HTTPServer.Host = host
	c.HTTPServer.Port = port
	return c
}

// WithTLS 设置TLS配置
func (c *Gateway) WithTLS(enabled bool, certFile, keyFile, caFile string) *Gateway {
	if c.HTTPServer == nil {
		c.HTTPServer = &HTTPServer{}
	}
	if c.HTTPServer.TLS == nil {
		c.HTTPServer.TLS = &TLS{}
	}
	c.HTTPServer.EnableTls = enabled
	c.HTTPServer.TLS.CertFile = certFile
	c.HTTPServer.TLS.KeyFile = keyFile
	c.HTTPServer.TLS.CAFile = caFile
	return c
}

// WithBanner 设置Banner配置
func (c *Gateway) WithBanner(enabled bool, title, description, author, email, website string) *Gateway {
	if c.Banner == nil {
		c.Banner = &Banner{}
	}
	c.Banner.Enabled = enabled
	c.Banner.Title = title
	c.Banner.Description = description
	c.Banner.Author = author
	c.Banner.Email = email
	c.Banner.Website = website
	return c
}

// WithHealth 设置健康检查配置
func (c *Gateway) WithHealth(cfg *Health) *Gateway {
	c.Health = cfg
	return c
}

// WithMonitoring 设置监控配置
func (c *Gateway) WithMonitoring(cfg *gcmiddleware.Monitoring) *Gateway {
	c.Monitoring = cfg
	return c
}

// WithSecurity 设置安全配置
func (c *Gateway) WithSecurity(cfg *gcmiddleware.Security) *Gateway {
	c.Security = cfg
	return c
}

// WithRequestContext 设置统一请求提取配置
func (c *Gateway) WithRequestContext(cfg *RequestContext) *Gateway {
	c.RequestContext = cfg
	return c
}

// WithMiddleware 设置中间件配置
func (c *Gateway) WithMiddleware(cfg *middleware.Middleware) *Gateway {
	c.Middleware = cfg
	return c
}

// WithCORS 设置CORS配置
func (c *Gateway) WithCORS(cfg *gcmiddleware.Cors) *Gateway {
	c.CORS = cfg
	return c
}

// WithSwagger 设置Swagger配置
func (c *Gateway) WithSwagger(cfg *swagger.Swagger) *Gateway {
	c.Swagger = cfg
	return c
}

// EnableTLS 启用TLS
func (c *Gateway) EnableTLS() *Gateway {
	if c.HTTPServer == nil {
		c.HTTPServer = &HTTPServer{}
	}
	c.HTTPServer.EnableTls = true
	return c
}

// EnableBanner 启用Banner
func (c *Gateway) EnableBanner() *Gateway {
	if c.Banner == nil {
		c.Banner = &Banner{}
	}
	c.Banner.Enabled = true
	return c
}

// EnableHealth 启用健康检查
func (c *Gateway) EnableHealth() *Gateway {
	if c.Health != nil {
		c.Health.Enable()
	}
	return c
}

// EnableMonitoring 启用监控
func (c *Gateway) EnableMonitoring() *Gateway {
	if c.Monitoring != nil {
		c.Monitoring.Enable()
	}
	return c
}

// EnableMiddleware 启用中间件
func (c *Gateway) EnableMiddleware() *Gateway {
	if c.Middleware != nil {
		c.Middleware.Enable()
	}
	return c
}

// EnableSwagger 启用Swagger
func (c *Gateway) EnableSwagger() *Gateway {
	if c.Swagger != nil {
		c.Swagger.Enable()
	}
	return c
}

// WithListeners 设置命名监听器列表
func (c *Gateway) WithListeners(listeners []*Listener) *Gateway {
	c.Listeners = listeners
	return c
}

// AddListener 添加命名监听器
func (c *Gateway) AddListener(listener *Listener) *Gateway {
	if c.Listeners == nil {
		c.Listeners = []*Listener{}
	}
	c.Listeners = append(c.Listeners, listener)
	return c
}

// GetListener 按名称获取监听器
func (c *Gateway) GetListener(name string) *Listener {
	for _, l := range c.Listeners {
		if l.Name == name {
			return l
		}
	}
	return nil
}

// Enable 启用网关
func (c *Gateway) Enable() *Gateway {
	c.Enabled = true
	return c
}

// Disable 禁用网关
func (c *Gateway) Disable() *Gateway {
	c.Enabled = false
	return c
}

// IsEnabled 检查是否启用
func (c *Gateway) IsEnabled() bool {
	return c.Enabled
}

// SetExtension 设置扩展配置
func (c *Gateway) SetExtension(key string, value any) *Gateway {
	if c.Extensions == nil {
		c.Extensions = make(map[string]any)
	}
	c.Extensions[key] = value
	return c
}

// GetExtension 获取扩展配置
func (c *Gateway) GetExtension(key string) (any, bool) {
	if c.Extensions == nil {
		return nil, false
	}
	value, exists := c.Extensions[key]
	return value, exists
}

// GetExtensionAs 获取指定类型的扩展配置（泛型版本）
// 使用 go-toolbox 的 convert.MustConvertTo 自动进行类型转换
//
// 支持的类型：
//   - string, bool
//   - int, int8, int16, int32, int64
//   - uint, uint8, uint16, uint32, uint64
//   - float32, float64
//   - []byte, map[string]any, []any
//
// 示例:
//
//	str, ok := GetExtensionAs[string](gw, "api-key")
//	num, ok := GetExtensionAs[int](gw, "max-retry")
//	flag, ok := GetExtensionAs[bool](gw, "enabled")
func GetExtensionAs[T types.Convertible](c *Gateway, key string) (T, bool) {
	var zero T

	value, exists := c.GetExtension(key)
	if !exists {
		return zero, false
	}

	// 直接使用 convert.MustConvertTo 进行类型转换
	return convert.MustConvertTo[T](value)
}

// GetExtensionMap 获取 map 类型的扩展配置
func (c *Gateway) GetExtensionMap(key string) (map[string]any, bool) {
	return GetExtensionAs[map[string]any](c, key)
}

// HasExtension 检查是否存在指定的扩展配置
func (c *Gateway) HasExtension(key string) bool {
	_, exists := c.GetExtension(key)
	return exists
}

// DeleteExtension 删除扩展配置
func (c *Gateway) DeleteExtension(key string) *Gateway {
	if c.Extensions != nil {
		delete(c.Extensions, key)
	}
	return c
}

// GetAllExtensions 获取所有扩展配置（深拷贝副本）
func (c *Gateway) GetAllExtensions() map[string]any {
	if c.Extensions == nil {
		return make(map[string]any)
	}
	// 使用深拷贝返回副本，避免外部修改影响内部数据
	var result map[string]any
	if err := syncx.DeepCopy(&result, &c.Extensions); err != nil {
		// 如果深拷贝失败，返回空 map
		return make(map[string]any)
	}
	return result
}
