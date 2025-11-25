/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-24 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-24 22:20:00
 * @FilePath: \go-rpc-gateway\pbmo\enum_mapper.go
 * @Description: 枚举类型映射工具 - 提供 protobuf 枚举与其他枚举类型的转换（支持泛型）
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"fmt"
	"reflect"
)

// EnumMapper 枚举映射器（int32 版本，兼容旧代码）
type EnumMapper struct {
	mappings map[int32]int32 // sourceValue -> targetValue
	reverse  map[int32]int32 // targetValue -> sourceValue
}

// NewEnumMapper 创建枚举映射器
func NewEnumMapper() *EnumMapper {
	return &EnumMapper{
		mappings: make(map[int32]int32),
		reverse:  make(map[int32]int32),
	}
}

// AddMapping 添加映射关系
// source: 源枚举值, target: 目标枚举值
func (em *EnumMapper) AddMapping(source, target int32) *EnumMapper {
	em.mappings[source] = target
	em.reverse[target] = source
	return em
}

// AddMappings 批量添加映射关系（源枚举值和目标枚举值一一对应）
func (em *EnumMapper) AddMappings(pairs [][2]int32) *EnumMapper {
	for _, pair := range pairs {
		em.AddMapping(pair[0], pair[1])
	}
	return em
}

// Map 映射源枚举值到目标枚举值
func (em *EnumMapper) Map(source int32, defaultValue int32) int32 {
	if target, exists := em.mappings[source]; exists {
		return target
	}
	return defaultValue
}

// ReverseMap 反向映射（从目标枚举值到源枚举值）
func (em *EnumMapper) ReverseMap(target int32, defaultValue int32) int32 {
	if source, exists := em.reverse[target]; exists {
		return source
	}
	return defaultValue
}

// MapEnum 通用枚举映射方法（使用反射）
func (em *EnumMapper) MapEnum(sourceEnum interface{}, targetEnumType reflect.Type, defaultValue interface{}) (interface{}, error) {
	sourceVal := reflect.ValueOf(sourceEnum)

	// 获取源枚举的整数值
	var sourceInt int32
	switch sourceVal.Kind() {
	case reflect.Int, reflect.Int32, reflect.Int64:
		sourceInt = int32(sourceVal.Int())
	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		sourceInt = int32(sourceVal.Uint())
	default:
		return defaultValue, fmt.Errorf("unsupported source enum type: %v", sourceVal.Kind())
	}

	// 映射到目标值
	targetInt := em.Map(sourceInt, 0)

	// 如果没有找到映射且提供了默认值，使用默认值
	if targetInt == 0 && defaultValue != nil {
		defaultVal := reflect.ValueOf(defaultValue)
		if defaultVal.Kind() == reflect.Int32 {
			targetInt = int32(defaultVal.Int())
		}
	}

	// 转换为目标枚举类型
	targetVal := reflect.New(targetEnumType).Elem()
	targetVal.SetInt(int64(targetInt))

	return targetVal.Interface(), nil
}

// ConvertEnum 便捷方法：转换枚举（自动推断类型）
func ConvertEnum[S, T any](mapper *EnumMapper, source S, defaultValue T) T {
	sourceInt := int32(reflect.ValueOf(source).Int())
	targetInt := mapper.Map(sourceInt, int32(reflect.ValueOf(defaultValue).Int()))

	var result T
	reflect.ValueOf(&result).Elem().SetInt(int64(targetInt))
	return result
}

// ========== 泛型枚举映射器 ==========

// GenericEnumMapper 泛型枚举映射器
// S: 源枚举类型, T: 目标枚举类型（T 也必须是 comparable 以支持反向映射）
type GenericEnumMapper[S comparable, T comparable] struct {
	mappings     map[S]T // sourceValue -> targetValue
	reverse      map[T]S // targetValue -> sourceValue
	defaultValue T       // 默认值
}

// NewGenericEnumMapper 创建泛型枚举映射器
func NewGenericEnumMapper[S comparable, T comparable](defaultValue T) *GenericEnumMapper[S, T] {
	return &GenericEnumMapper[S, T]{
		mappings:     make(map[S]T),
		reverse:      make(map[T]S),
		defaultValue: defaultValue,
	}
}

// Register 注册映射关系（支持链式调用）
func (m *GenericEnumMapper[S, T]) Register(source S, target T) *GenericEnumMapper[S, T] {
	m.mappings[source] = target
	m.reverse[target] = source
	return m
}

// RegisterBatch 批量注册映射关系
func (m *GenericEnumMapper[S, T]) RegisterBatch(pairs map[S]T) *GenericEnumMapper[S, T] {
	for source, target := range pairs {
		m.Register(source, target)
	}
	return m
}

// Map 映射源枚举到目标枚举
func (m *GenericEnumMapper[S, T]) Map(source S) T {
	if target, exists := m.mappings[source]; exists {
		return target
	}
	return m.defaultValue
}

// MapWithDefault 映射源枚举到目标枚举（指定默认值）
func (m *GenericEnumMapper[S, T]) MapWithDefault(source S, defaultValue T) T {
	if target, exists := m.mappings[source]; exists {
		return target
	}
	return defaultValue
}

// ReverseMap 反向映射（从目标枚举到源枚举）
func (m *GenericEnumMapper[S, T]) ReverseMap(target T) (S, bool) {
	source, exists := m.reverse[target]
	return source, exists
}

// MapSlice 批量映射切片
func (m *GenericEnumMapper[S, T]) MapSlice(sources []S) []T {
	results := make([]T, 0, len(sources))
	for _, source := range sources {
		results = append(results, m.Map(source))
	}
	return results
}

// ========== 自动枚举转换器（类似 pbmo 自动转换） ==========

// AutoEnumConverter 自动枚举转换器
// 使用方式：
//
//	converter := pbmo.NewAutoEnumConverter[ProtoEnum, WSEnum](defaultValue)
//	converter.AutoRegister(map[ProtoEnum]WSEnum{...})
//	result := converter.Convert(source)  // 自动转换
type AutoEnumConverter[S comparable, T comparable] struct {
	mapper *GenericEnumMapper[S, T]
}

// NewAutoEnumConverter 创建自动枚举转换器
func NewAutoEnumConverter[S comparable, T comparable](defaultValue T) *AutoEnumConverter[S, T] {
	return &AutoEnumConverter[S, T]{
		mapper: NewGenericEnumMapper[S, T](defaultValue),
	}
}

// AutoRegister 自动批量注册映射关系（支持 map 和 slice）
func (ac *AutoEnumConverter[S, T]) AutoRegister(mappings map[S]T) *AutoEnumConverter[S, T] {
	ac.mapper.RegisterBatch(mappings)
	return ac
}

// Convert 自动转换（核心方法，像 pbmo 一样简单）
func (ac *AutoEnumConverter[S, T]) Convert(source S) T {
	return ac.mapper.Map(source)
}

// ConvertBack 反向转换
func (ac *AutoEnumConverter[S, T]) ConvertBack(target T) (S, bool) {
	return ac.mapper.ReverseMap(target)
}

// ConvertSlice 批量转换
func (ac *AutoEnumConverter[S, T]) ConvertSlice(sources []S) []T {
	return ac.mapper.MapSlice(sources)
}

// ConvertWithDefault 带默认值转换
func (ac *AutoEnumConverter[S, T]) ConvertWithDefault(source S, defaultValue T) T {
	return ac.mapper.MapWithDefault(source, defaultValue)
}
