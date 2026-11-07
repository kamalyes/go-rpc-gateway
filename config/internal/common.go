/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2024-11-07 23:55:15
 * @FilePath: \go-rpc-gateway\config\internal\common.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package internal

import (
	"encoding/json"
	"sync"

	validator "github.com/kamalyes/go-toolbox/pkg/argus"
	"gopkg.in/yaml.v3"
)

// Configurable 接口定义了配置的基本行为
type Configurable interface {
	Get() interface{}
	Set(data interface{})
	Clone() Configurable
	Validate() error
}

// Exportable 接口定义了配置的导出功能
type Exportable interface {
	ToYAML() (string, error)
	ToJSON() (string, error)
}

// Hookable 接口定义了配置的钩子函数
// 实现此接口的配置可以在加载前后执行自定义操作
type Hookable interface {
	BeforeLoad() error // 配置加载前的钩子，可用于设置默认值或预处理
	AfterLoad() error  // 配置加载后的钩子，可用于计算衍生字段或后处理
}

// CallBeforeLoad 安全调用 BeforeLoad 钩子
func CallBeforeLoad(cfg interface{}) error {
	if hookable, ok := cfg.(Hookable); ok {
		return hookable.BeforeLoad()
	}
	return nil
}

// CallAfterLoad 安全调用 AfterLoad 钩子
func CallAfterLoad(cfg interface{}) error {
	if hookable, ok := cfg.(Hookable); ok {
		return hookable.AfterLoad()
	}
	return nil
}

var mu sync.Mutex // 用于保护实例的创建

// 全局验证器实例
var validate *validator.Validate

// 初始化验证器
func init() {
	validate = validator.New()
}

// Validate 方法，通用验证
func ValidateStruct(c interface{}) error {
	return validate.Struct(c)
}

// 验证额外函数
func ValidateExtra(extraFunc func() error) error {
	if extraFunc != nil {
		return extraFunc()
	}
	return nil
}

// LockFunc 是一个公共方法，用于锁定和解锁
func LockFunc(fn func()) {
	mu.Lock()
	defer mu.Unlock()
	fn()
}

// ExportToYAML 通用YAML导出函数
func ExportToYAML(config interface{}) (string, error) {
	yamlData, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(yamlData), nil
}

// ExportToJSON 通用JSON导出函数
func ExportToJSON(config interface{}) (string, error) {
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

// ExportConfigToYAML 导出Configurable对象为YAML
func ExportConfigToYAML(config Configurable) (string, error) {
	return ExportToYAML(config.Get())
}

// ExportConfigToJSON 导出Configurable对象为JSON
func ExportConfigToJSON(config Configurable) (string, error) {
	return ExportToJSON(config.Get())
}

// GenerateYAMLFromDefault 从默认配置生成YAML
func GenerateYAMLFromDefault(defaultFunc func() interface{}) (string, error) {
	config := defaultFunc()
	return ExportToYAML(config)
}

// GenerateJSONFromDefault 从默认配置生成JSON
func GenerateJSONFromDefault(defaultFunc func() interface{}) (string, error) {
	config := defaultFunc()
	return ExportToJSON(config)
}
