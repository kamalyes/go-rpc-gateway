/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-15 13:10:50
 * @FilePath: \go-rpc-gateway\global\extensions.go
 * @Description: Gateway Extensions 扩展配置读取工具
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package global

import (
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-toolbox/pkg/osx"
	"github.com/kamalyes/go-toolbox/pkg/types"
)

// GetExtensionAs 获取指定类型的扩展配置（泛型版本）
// 支持类型：string, bool, 数值类型, []byte, map[string]any, []any
//
// 示例:
//
//	str, ok := global.GetExtensionAs[string]("api-key")
//	num, ok := global.GetExtensionAs[int]("max-retry")
//	flag, ok := global.GetExtensionAs[bool]("enabled")
func GetExtensionAs[T types.Convertible](key string) (T, bool) {
	if GATEWAY == nil {
		var zero T
		return zero, false
	}
	return gwconfig.GetExtensionAs[T](GATEWAY, key)
}

// GetEnvOrExtension 先读环境变量，为空则回退到 gateway extensions 配置（字符串类型）
// 适用于敏感配置（如密钥）：环境变量优先，yaml extensions 作为默认值
func GetEnvOrExtension(envKey, extKey string) string {
	if val := osx.Getenv(envKey, ""); val != "" {
		return val
	}
	if s, ok := GetExtensionAs[string](extKey); ok {
		return s
	}
	return ""
}
