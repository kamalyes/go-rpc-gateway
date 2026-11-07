/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-08 15:15:19
 * @FilePath: \go-rpc-gateway\cpool\database\types\map_any.go
 * @Description: MapAny - 任意类型 Map，支持数据库 JSON 序列化
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/kamalyes/go-toolbox/pkg/convert"
	"github.com/kamalyes/go-toolbox/pkg/serializer"
	"google.golang.org/protobuf/types/known/structpb"
)

// MapAny 任意类型的 Map，支持数据库 JSON 序列化
type MapAny map[string]any

func (m *MapAny) Scan(value interface{}) error {
	if value == nil {
		*m = make(MapAny)
		return nil
	}
	b, err := toBytes(value)
	if err != nil {
		return fmt.Errorf("failed to unmarshal MapAny: %w", err)
	}
	if len(b) == 0 {
		*m = make(MapAny)
		return nil
	}
	return serializer.JSONUnmarshal(b, m)
}

func (m MapAny) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return serializer.JSONMarshal(m)
}

// Get 获取指定 key 的值，支持默认值
func (m MapAny) Get(key string, defaultValue ...any) any {
	if val, ok := m[key]; ok {
		return val
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return nil
}

// GetString 获取字符串值
func (m MapAny) GetString(key string, defaultValue ...string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return ""
}

// GetInt 获取整数值
func (m MapAny) GetInt(key string, defaultValue ...int) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int32:
			return int(v)
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return 0
}

// GetInt32 获取 int32 值
func (m MapAny) GetInt32(key string, defaultValue ...int32) int32 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int32:
			return v
		case int:
			return int32(v)
		case int64:
			return int32(v)
		case float64:
			return int32(v)
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return 0
}

// GetBool 获取布尔值
func (m MapAny) GetBool(key string, defaultValue ...bool) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return false
}

// GetMap 获取 MapAny 值
func (m MapAny) GetMap(key string, defaultValue ...MapAny) MapAny {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case MapAny:
			return v
		case map[string]interface{}:
			return MapAny(v)
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return MapAny{}
}

// Set 设置值
func (m MapAny) Set(key string, value any) MapAny {
	m[key] = value
	return m
}

// Has 检查 key 是否存在
func (m MapAny) Has(key string) bool {
	_, ok := m[key]
	return ok
}

// Delete 删除指定 key
func (m MapAny) Delete(key string) MapAny {
	delete(m, key)
	return m
}

// Keys 获取所有 key
func (m MapAny) Keys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Values 获取所有 value
func (m MapAny) Values() []any {
	values := make([]any, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// Merge 合并另一个 Map
func (m MapAny) Merge(other MapAny) MapAny {
	for k, v := range other {
		m[k] = v
	}
	return m
}

// Clone 克隆一个新的 Map
func (m MapAny) Clone() MapAny {
	clone := make(MapAny, len(m))
	for k, v := range m {
		clone[k] = v
	}
	return clone
}

// MapAnyToStruct 将 MapAny 转换为 structpb.Struct
func MapAnyToStruct(payload MapAny) *structpb.Struct {
	if len(payload) == 0 {
		result, _ := structpb.NewStruct(map[string]interface{}{})
		return result
	}

	// 将 MapAny 转换为 map[string]interface{}，并处理嵌套的 MapAny
	converted := convertMapAnyToInterfaceMap(payload)
	result, err := structpb.NewStruct(converted)
	if err != nil {
		return &structpb.Struct{}
	}
	return result
}

// convertMapAnyToInterfaceMap 将 MapAny 转换为 map[string]interface{}，处理嵌套的 MapAny
func convertMapAnyToInterfaceMap(m MapAny) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = convertValue(v)
	}
	return result
}

// convertValue 转换单个值，处理各种嵌套类型
func convertValue(v interface{}) interface{} {
	switch val := v.(type) {
	case MapAny:
		return convertMapAnyToInterfaceMap(val)
	case map[string]interface{}:
		return convertInterfaceMap(val)
	case []interface{}:
		// 处理数组中的嵌套 MapAny
		convertedSlice := make([]interface{}, len(val))
		for i, item := range val {
			convertedSlice[i] = convertValue(item)
		}
		return convertedSlice
	default:
		return v
	}
}

// convertInterfaceMap 递归处理 map[string]interface{} 中的嵌套 MapAny
func convertInterfaceMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = convertValue(v)
	}
	return result
}

// StructToMapAny 将 structpb.Struct 转换为 MapAny
func StructToMapAny(payload *structpb.Struct) MapAny {
	if payload == nil {
		return MapAny{}
	}
	return MapAny(payload.AsMap())
}

// StructToMapString 将 structpb.Struct 转换为 map[string]string
// 标量值直接格式化为字符串，复合类型（嵌套 struct/list）序列化为 JSON 字符串
func StructToMapString(payload *structpb.Struct) map[string]string {
	if payload == nil {
		return nil
	}
	m := payload.AsMap()
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = convert.MustString(v)
	}
	return result
}

// MapAnyToJSONString 将 MapAny 转换为 JSON 字符串
func MapAnyToJSONString(payload MapAny) string {
	if len(payload) == 0 {
		return "{}"
	}
	b, err := serializer.JSONMarshal(payload)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// StructToMapAny 将 Go struct 转换为 MapAny
// 使用 JSON 序列化/反序列化实现，支持任意 struct 类型
func StructToMapAnyFromStruct(v interface{}) MapAny {
	if v == nil {
		return MapAny{}
	}

	// 先序列化为 JSON
	jsonBytes, err := serializer.JSONMarshal(v)
	if err != nil {
		return MapAny{}
	}

	// 再反序列化为 MapAny
	var result MapAny
	if err := serializer.JSONUnmarshal(jsonBytes, &result); err != nil {
		return MapAny{}
	}

	return result
}

// MapAnyToStructTarget 将 MapAny 转换为 Go struct
// target 必须是指针类型
func MapAnyToStructTarget(m MapAny, target interface{}) error {
	if len(m) == 0 {
		return nil
	}

	// 先序列化为 JSON
	jsonBytes, err := serializer.JSONMarshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal MapAny: %w", err)
	}

	// 再反序列化为 struct，使用标准库 json.Unmarshal 处理任意类型
	return json.Unmarshal(jsonBytes, target)
}

// Len 获取 Map 长度
func (m MapAny) Len() int {
	return len(m)
}

// IsEmpty 判断 Map 是否为空
func (m MapAny) IsEmpty() bool {
	return len(m) == 0
}

// Filter 过滤键值对
func (m MapAny) Filter(fn func(key string, value any) bool) MapAny {
	result := make(MapAny)
	for k, v := range m {
		if fn(k, v) {
			result[k] = v
		}
	}
	return result
}

// Map 映射转换
func (m MapAny) Map(fn func(key string, value any) (string, any)) MapAny {
	result := make(MapAny)
	for k, v := range m {
		newKey, newValue := fn(k, v)
		result[newKey] = newValue
	}
	return result
}

// Each 遍历所有键值对
func (m MapAny) Each(fn func(key string, value any)) {
	for k, v := range m {
		fn(k, v)
	}
}
