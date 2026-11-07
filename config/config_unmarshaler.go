/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-27 09:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-27 09:30:00
 * @FilePath: \go-rpc-gateway\config\config_unmarshaler.go
 * @Description: 通用配置反序列化工具
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package config

import (
	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// UnmarshalWithFlexibleNaming 使用灵活的命名匹配策略反序列化配置
// 支持的转换场景：
// 1. 直接匹配（大小写敏感）
// 2. 大小写不敏感匹配
// 3. kebab-case (service-name) -> snake_case (service_name)
// 4. camelCase (serviceName) -> snake_case (service_name)
// 5. PascalCase (ServiceName) -> snake_case (service_name)
//
// 参数:
//
//	v: Viper 实例
//	target: 目标配置结构体指针
//
// 返回:
//
//	error: 反序列化错误
func UnmarshalWithFlexibleNaming(v *viper.Viper, target interface{}) error {
	return v.Unmarshal(target, func(dc *mapstructure.DecoderConfig) {
		dc.WeaklyTypedInput = true
		// 自定义键名匹配：支持多种命名风格的转换
		dc.MatchName = FlexibleMatchName
	})
}

// FlexibleMatchName 灵活的键名匹配函数
// 支持多种命名约定之间的自动转换
//
// 匹配策略（按优先级）:
// 1. 直接匹配（快速路径）
// 2. 大小写不敏感匹配
// 3. kebab-case 转 snake_case
// 4. camelCase/PascalCase 转 snake_case
//
// 参数:
//
//	mapKey: 配置文件中的键名（可能是 kebab-case、camelCase 等）
//	fieldName: 结构体字段的 mapstructure 标签名（通常是 snake_case）
//
// 返回:
//
//	bool: 是否匹配成功
func FlexibleMatchName(mapKey, fieldName string) bool {
	// 1. 直接匹配（快速路径）
	if mapKey == fieldName {
		return true
	}

	// 2. 大小写不敏感匹配
	if len(mapKey) == len(fieldName) {
		matched := true
		for i := 0; i < len(mapKey); i++ {
			// 检查字符是否相同，或者大小写差异为32（ASCII码特性）
			if mapKey[i] != fieldName[i] &&
				mapKey[i] != fieldName[i]+32 &&
				mapKey[i] != fieldName[i]-32 {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}

	// 3. kebab-case 转 snake_case (service-name -> service_name)
	if len(mapKey) > 0 {
		normalizedKey := make([]byte, 0, len(mapKey))
		for i := 0; i < len(mapKey); i++ {
			if mapKey[i] == '-' {
				normalizedKey = append(normalizedKey, '_')
			} else {
				normalizedKey = append(normalizedKey, mapKey[i])
			}
		}
		if string(normalizedKey) == fieldName {
			return true
		}
	}

	// 4. snake_case 转 kebab-case (service_name -> service-name)
	if len(mapKey) > 0 {
		normalizedKey := make([]byte, 0, len(mapKey))
		for i := 0; i < len(mapKey); i++ {
			if mapKey[i] == '_' {
				normalizedKey = append(normalizedKey, '-')
			} else {
				normalizedKey = append(normalizedKey, mapKey[i])
			}
		}
		if string(normalizedKey) == fieldName {
			return true
		}
	}

	// 5. camelCase/PascalCase 转 snake_case (serviceName/ServiceName -> service_name)
	if len(mapKey) > 0 && len(fieldName) > 0 {
		snakeCase := make([]byte, 0, len(mapKey)+5)
		for i := 0; i < len(mapKey); i++ {
			ch := mapKey[i]
			// 如果是大写字母且不是第一个字符，添加下划线
			if ch >= 'A' && ch <= 'Z' {
				if i > 0 {
					snakeCase = append(snakeCase, '_')
				}
				snakeCase = append(snakeCase, ch+32) // 转小写
			} else {
				snakeCase = append(snakeCase, ch)
			}
		}
		if string(snakeCase) == fieldName {
			return true
		}
	}

	return false
}
