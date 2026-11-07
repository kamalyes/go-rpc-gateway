/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 18:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-11 18:00:00
 * @FilePath: \go-rpc-gateway\config\middleware\jaeger.go
 * @Description: Jaeger配置模块
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// Jaeger Jaeger配置
type Jaeger struct {
	ModuleName  string            `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`    // 模块名称
	Enabled     bool              `mapstructure:"enabled" yaml:"enabled" json:"enabled"`               // 是否启用Jaeger
	Endpoint    string            `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint"`            // Jaeger端点
	ServiceName string            `mapstructure:"service-name" yaml:"service-name" json:"serviceName"` // 服务名称
	SampleRate  float64           `mapstructure:"sample-rate" yaml:"sample-rate" json:"sampleRate"`    // 采样率
	Agent       *Agent            `mapstructure:"agent" yaml:"agent" json:"agent"`                     // Agent配置
	Collector   *Collector        `mapstructure:"collector" yaml:"collector" json:"collector"`         // Collector配置
	Sampling    *Sampling         `mapstructure:"sampling" yaml:"sampling" json:"sampling"`            // 采样配置
	Tags        map[string]string `mapstructure:"tags" yaml:"tags" json:"tags"`                        // 全局标签
}

// Agent Agent配置
type Agent struct {
	Host string `mapstructure:"host" yaml:"host" json:"host"` // Agent主机
	Port int    `mapstructure:"port" yaml:"port" json:"port"` // Agent端口
}

// Collector Collector配置
type Collector struct {
	Endpoint string `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint"` // Collector端点
	Username string `mapstructure:"username" yaml:"username" json:"username"` // 用户名
	Password string `mapstructure:"password" yaml:"password" json:"password"` // 密码
}

// Sampling 采样配置
type Sampling struct {
	Type               string              `mapstructure:"type" yaml:"type" json:"type"`                                                 // 采样类型
	Param              float64             `mapstructure:"param" yaml:"param" json:"param"`                                              // 采样参数
	MaxTracesPerSecond int                 `mapstructure:"max-traces-per-second" yaml:"max-traces-per-second" json:"maxTracesPerSecond"` // 每秒最大追踪数
	OperationSampling  []OperationSampling `mapstructure:"operation-sampling" yaml:"operation-sampling" json:"operationSampling"`        // 操作采样
}

// OperationSampling 操作采样配置
type OperationSampling struct {
	Operation             string  `mapstructure:"operation" yaml:"operation" json:"operation"`                                       // 操作名称
	MaxTracesPerSecond    int     `mapstructure:"max-traces-per-second" yaml:"max-traces-per-second" json:"maxTracesPerSecond"`      // 每秒最大追踪数
	ProbabilisticSampling float64 `mapstructure:"probabilistic-sampling" yaml:"probabilistic-sampling" json:"probabilisticSampling"` // 概率采样
}

// DefaultJaeger 创建默认Jaeger配置
func DefaultJaeger() *Jaeger {
	return &Jaeger{
		ModuleName:  "jaeger",
		Enabled:     false,
		Endpoint:    "http://localhost:14268/api/traces",
		ServiceName: "go-rpc-gateway",
		SampleRate:  0.1,
		Agent: &Agent{
			Host: "localhost",
			Port: 6832,
		},
		Collector: &Collector{
			Endpoint: "http://localhost:14268/api/traces",
			Username: "jaeger_user",
			Password: "jaeger_password",
		},
		Sampling: &Sampling{
			Type:               "probabilistic",
			Param:              0.1,
			MaxTracesPerSecond: 100,
			OperationSampling:  []OperationSampling{},
		},
		Tags: make(map[string]string),
	}
}

// Get 返回配置接口
func (j *Jaeger) Get() interface{} {
	return j
}

// Set 设置配置数据
func (j *Jaeger) Set(data interface{}) {
	if cfg, ok := data.(*Jaeger); ok {
		*j = *cfg
	}
}

// Clone 返回配置的副本
func (j *Jaeger) Clone() internal.Configurable {
	var cloned Jaeger
	if err := syncx.DeepCopy(&cloned, j); err != nil {
		// 如果深拷贝失败，返回空配置
		return &Jaeger{}
	}
	return &cloned
}

// Validate 验证配置
func (j *Jaeger) Validate() error {
	return internal.ValidateStruct(j)
}

// WithModuleName 设置模块名称
func (j *Jaeger) WithModuleName(moduleName string) *Jaeger {
	j.ModuleName = moduleName
	return j
}

// WithEnabled 设置是否启用
func (j *Jaeger) WithEnabled(enabled bool) *Jaeger {
	j.Enabled = enabled
	return j
}

// WithEndpoint 设置端点
func (j *Jaeger) WithEndpoint(endpoint string) *Jaeger {
	j.Endpoint = endpoint
	return j
}

// WithServiceName 设置服务名称
func (j *Jaeger) WithServiceName(serviceName string) *Jaeger {
	j.ServiceName = serviceName
	return j
}

// WithSampleRate 设置采样率
func (j *Jaeger) WithSampleRate(sampleRate float64) *Jaeger {
	j.SampleRate = sampleRate
	return j
}

// WithAgent 设置Agent配置
func (j *Jaeger) WithAgent(host string, port int) *Jaeger {
	if j.Agent == nil {
		j.Agent = &Agent{}
	}
	j.Agent.Host = host
	j.Agent.Port = port
	return j
}

// WithCollector 设置Collector配置
func (j *Jaeger) WithCollector(endpoint, username, password string) *Jaeger {
	if j.Collector == nil {
		j.Collector = &Collector{}
	}
	j.Collector.Endpoint = endpoint
	j.Collector.Username = username
	j.Collector.Password = password
	return j
}

// WithSampling 设置采样配置
func (j *Jaeger) WithSampling(samplingType string, param float64, maxTracesPerSecond int) *Jaeger {
	if j.Sampling == nil {
		j.Sampling = &Sampling{}
	}
	j.Sampling.Type = samplingType
	j.Sampling.Param = param
	j.Sampling.MaxTracesPerSecond = maxTracesPerSecond
	return j
}

// AddOperationSampling 添加操作采样
func (j *Jaeger) AddOperationSampling(operation string, maxTracesPerSecond int, probabilisticSampling float64) *Jaeger {
	if j.Sampling == nil {
		j.Sampling = &Sampling{}
	}
	j.Sampling.OperationSampling = append(j.Sampling.OperationSampling, OperationSampling{
		Operation:             operation,
		MaxTracesPerSecond:    maxTracesPerSecond,
		ProbabilisticSampling: probabilisticSampling,
	})
	return j
}

// AddTag 添加全局标签
func (j *Jaeger) AddTag(key, value string) *Jaeger {
	if j.Tags == nil {
		j.Tags = make(map[string]string)
	}
	j.Tags[key] = value
	return j
}

// Enable 启用Jaeger
func (j *Jaeger) Enable() *Jaeger {
	j.Enabled = true
	return j
}

// Disable 禁用Jaeger
func (j *Jaeger) Disable() *Jaeger {
	j.Enabled = false
	return j
}

// IsEnabled 检查是否启用
func (j *Jaeger) IsEnabled() bool {
	return j.Enabled
}
