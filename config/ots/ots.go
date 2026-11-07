/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 11:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 11:30:00
 * @FilePath: \go-rpc-gateway\config\ots\ots.go
 * @Description: 阿里云 Tablestore（OTS）配置
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package ots

import (
	"fmt"

	"github.com/kamalyes/go-rpc-gateway/config/internal"
	"github.com/kamalyes/go-toolbox/pkg/syncx"
)

// OTS 阿里云 Tablestore 配置
//
// Endpoint 使用 Tablestore 实例对应的公网或 VPC 地址，不带协议前缀：
//
//	https://<instance>.cn-hangzhou.ots.aliyuncs.com   -> <instance>.cn-hangzhou.ots.aliyuncs.com
//	VPC 地址同理，换成 vpc 域名
//
// SecurityToken 仅在使用 STS 临时凭据时填写，与 AccessKeyID/AccessKeySecret 配套使用
//
// [EN] OTS is the Alibaba Cloud Tablestore configuration. Endpoint carries no scheme prefix
// (use `<instance>.cn-hangzhou.ots.aliyuncs.com`, not the https:// form). SecurityToken is
// only set for STS temporary credentials and is used together with the access key pair.
type OTS struct {
	Enabled         bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`                                      // 是否启用
	Endpoint        string `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint" validate:"required"`               // 实例端点，不带协议前缀
	InstanceName    string `mapstructure:"instance-name" yaml:"instance-name" json:"instanceName" validate:"required"` // 实例名称
	AccessKeyID     string `mapstructure:"access-key-id" yaml:"access-key-id" json:"accessKeyId"`                      // 访问密钥 ID
	AccessKeySecret string `mapstructure:"access-key-secret" yaml:"access-key-secret" json:"accessKeySecret"`          // 访问密钥
	SecurityToken   string `mapstructure:"security-token" yaml:"security-token" json:"securityToken"`                  // STS 临时令牌
	HTTPTimeout     int    `mapstructure:"http-timeout" yaml:"http-timeout" json:"httpTimeout" validate:"min=0"`       // 请求超时（秒）
	RetryTimes      int    `mapstructure:"retry-times" yaml:"retry-times" json:"retryTimes" validate:"min=0"`          // 最大重试次数
	MaxIdleConns    int    `mapstructure:"max-idle-conns" yaml:"max-idle-conns" json:"maxIdleConns" validate:"min=0"`  // 最大空闲连接数
	ModuleName      string `mapstructure:"module-name" yaml:"module-name" json:"moduleName"`                           // 模块名称
}

// NewOTS 创建一个新的 OTS 配置实例
func NewOTS(opt *OTS) *OTS {
	var instance *OTS
	internal.LockFunc(func() {
		instance = opt
	})
	return instance
}

// Clone 返回 OTS 配置的副本
func (o *OTS) Clone() internal.Configurable {
	var cloned OTS
	if err := syncx.DeepCopy(&cloned, o); err != nil {
		return &OTS{}
	}
	return &cloned
}

// Get 返回 OTS 配置的所有字段
func (o *OTS) Get() interface{} {
	return o
}

// Set 更新 OTS 配置的字段
func (o *OTS) Set(data interface{}) {
	if configData, ok := data.(*OTS); ok {
		*o = *configData
	}
}

// Validate 校验 OTS 配置
// 启用时才要求端点与实例名，未启用的配置允许留空
func (o *OTS) Validate() error {
	if !o.Enabled {
		return nil
	}
	if o.Endpoint == "" {
		return fmt.Errorf("Tablestore endpoint 不能为空")
	}
	if o.InstanceName == "" {
		return fmt.Errorf("Tablestore 实例名称不能为空")
	}
	if (o.AccessKeyID == "") != (o.AccessKeySecret == "") {
		return fmt.Errorf("Tablestore access-key-id 与 access-key-secret 必须同时配置")
	}
	if o.HTTPTimeout < 0 || o.RetryTimes < 0 || o.MaxIdleConns < 0 {
		return fmt.Errorf("Tablestore 超时、重试与连接数不能为负数")
	}
	return internal.ValidateStruct(o)
}

// DefaultOTS 返回默认 OTS 配置
func DefaultOTS() OTS {
	return OTS{
		Enabled:      false,
		HTTPTimeout:  10,
		RetryTimes:   3,
		MaxIdleConns: 100,
		ModuleName:   "ots",
	}
}

// Default 返回默认 OTS 配置的指针，支持链式调用
func Default() *OTS {
	config := DefaultOTS()
	return &config
}

// WithEnabled 设置是否启用
func (o *OTS) WithEnabled(enabled bool) *OTS {
	o.Enabled = enabled
	return o
}

// WithEndpoint 设置实例端点
func (o *OTS) WithEndpoint(endpoint string) *OTS {
	o.Endpoint = endpoint
	return o
}

// WithInstanceName 设置实例名称
func (o *OTS) WithInstanceName(instanceName string) *OTS {
	o.InstanceName = instanceName
	return o
}

// WithCredentials 设置访问凭据
func (o *OTS) WithCredentials(accessKeyID, accessKeySecret, securityToken string) *OTS {
	o.AccessKeyID = accessKeyID
	o.AccessKeySecret = accessKeySecret
	o.SecurityToken = securityToken
	return o
}

// WithHTTPTimeout 设置请求超时（秒）
func (o *OTS) WithHTTPTimeout(seconds int) *OTS {
	o.HTTPTimeout = seconds
	return o
}

// WithRetryTimes 设置最大重试次数
func (o *OTS) WithRetryTimes(times int) *OTS {
	o.RetryTimes = times
	return o
}

// WithMaxIdleConns 设置最大空闲连接数
func (o *OTS) WithMaxIdleConns(conns int) *OTS {
	o.MaxIdleConns = conns
	return o
}

// WithModuleName 设置模块名称
func (o *OTS) WithModuleName(moduleName string) *OTS {
	o.ModuleName = moduleName
	return o
}
