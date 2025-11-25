/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-24 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-24 22:32:23
 * @FilePath: \engine-im-service\go-rpc-gateway\pbmo\enum_mapper_test.go
 * @Description: 测试枚举类型映射工具
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestEnumMapper(t *testing.T) {
	em := NewEnumMapper()

	// 添加映射关系
	em.AddMapping(1, 100)
	em.AddMapping(2, 200)

	// 测试映射
	assert.Equal(t, int32(100), em.Map(1, 0))
	assert.Equal(t, int32(200), em.Map(2, 0))
	assert.Equal(t, int32(0), em.Map(3, 0)) // 默认值

	// 测试反向映射
	assert.Equal(t, int32(1), em.ReverseMap(100, 0))
	assert.Equal(t, int32(2), em.ReverseMap(200, 0))
	assert.Equal(t, int32(0), em.ReverseMap(300, 0)) // 默认值
}

func TestMapEnum(t *testing.T) {
	em := NewEnumMapper()
	em.AddMapping(1, 100)
	em.AddMapping(2, 200)

	// 测试 MapEnum 方法
	result, err := em.MapEnum(int32(1), reflect.TypeOf(int32(0)), int32(0))
	assert.NoError(t, err)
	assert.Equal(t, int32(100), result)

	result, err = em.MapEnum(int32(3), reflect.TypeOf(int32(0)), int32(0))
	assert.NoError(t, err)
	assert.Equal(t, int32(0), result) // 默认值

	// 测试类型不支持
	_, err = em.MapEnum("invalid", reflect.TypeOf(int32(0)), int32(0))
	assert.Error(t, err)
}

func TestConvertEnum(t *testing.T) {
	em := NewEnumMapper()
	em.AddMapping(int32(1), int32(100))

	// 测试 ConvertEnum 方法
	result := ConvertEnum(em, int32(1), int32(0))
	assert.Equal(t, int32(100), result)

	result = ConvertEnum(em, int32(2), int32(0))
	assert.Equal(t, int32(0), result) // 默认值
}

func TestGenericEnumMapper(t *testing.T) {
	gem := NewGenericEnumMapper[int32, int32](0)

	// 注册映射关系
	gem.Register(1, 100)
	gem.Register(2, 200)

	// 测试映射
	assert.Equal(t, int32(100), gem.Map(1))
	assert.Equal(t, int32(200), gem.Map(2))
	assert.Equal(t, int32(0), gem.Map(3)) // 默认值

	// 测试反向映射
	source, exists := gem.ReverseMap(100)
	assert.True(t, exists)
	assert.Equal(t, int32(1), source)

	source, exists = gem.ReverseMap(300)
	assert.False(t, exists)

	// 测试批量注册和映射
	gem.RegisterBatch(map[int32]int32{3: 300, 4: 400})
	assert.Equal(t, int32(300), gem.Map(3))
	assert.Equal(t, int32(400), gem.Map(4))
	assert.Equal(t, int32(0), gem.Map(5)) // 默认值

	// 测试 MapSlice 方法
	sources := []int32{1, 2, 3}
	results := gem.MapSlice(sources)
	assert.Equal(t, []int32{100, 200, 300}, results)
}

func TestGenericEnumMapper_MapWithDefault(t *testing.T) {
	gem := NewGenericEnumMapper[int32, int32](0)
	gem.Register(1, 100)

	// 测试存在的映射
	assert.Equal(t, int32(100), gem.MapWithDefault(1, 999))

	// 测试不存在的映射，使用自定义默认值
	assert.Equal(t, int32(999), gem.MapWithDefault(99, 999))
}

func TestGenericEnumMapper_EmptySlice(t *testing.T) {
	gem := NewGenericEnumMapper[int32, int32](0)

	// 测试空切片
	results := gem.MapSlice([]int32{})
	assert.Empty(t, results)
}

func TestAutoEnumConverter(t *testing.T) {
	converter := NewAutoEnumConverter[int32, int32](0)

	// 测试自动注册
	mappings := map[int32]int32{
		1: 100,
		2: 200,
		3: 300,
	}
	converter.AutoRegister(mappings)

	// 测试转换
	assert.Equal(t, int32(100), converter.Convert(1))
	assert.Equal(t, int32(200), converter.Convert(2))
	assert.Equal(t, int32(0), converter.Convert(99)) // 默认值

	// 测试反向转换
	source, exists := converter.ConvertBack(100)
	assert.True(t, exists)
	assert.Equal(t, int32(1), source)

	source, exists = converter.ConvertBack(999)
	assert.False(t, exists)

	// 测试批量转换
	sources := []int32{1, 2, 3, 99}
	results := converter.ConvertSlice(sources)
	assert.Equal(t, []int32{100, 200, 300, 0}, results)

	// 测试带默认值转换
	assert.Equal(t, int32(100), converter.ConvertWithDefault(1, 888))
	assert.Equal(t, int32(888), converter.ConvertWithDefault(99, 888))
}

func TestAutoEnumConverter_ChainedAutoRegister(t *testing.T) {
	converter := NewAutoEnumConverter[int32, int32](0)

	// 测试链式调用
	converter.
		AutoRegister(map[int32]int32{1: 100}).
		AutoRegister(map[int32]int32{2: 200})

	assert.Equal(t, int32(100), converter.Convert(1))
	assert.Equal(t, int32(200), converter.Convert(2))
}

func TestEnumMapper_AddMappings(t *testing.T) {
	em := NewEnumMapper()

	// 测试批量添加
	pairs := [][2]int32{
		{1, 100},
		{2, 200},
		{3, 300},
	}
	em.AddMappings(pairs)

	assert.Equal(t, int32(100), em.Map(1, 0))
	assert.Equal(t, int32(200), em.Map(2, 0))
	assert.Equal(t, int32(300), em.Map(3, 0))
}

func TestEnumMapper_MapEnum_WithDefaultValue(t *testing.T) {
	em := NewEnumMapper()
	em.AddMapping(1, 100)

	// 测试不存在的映射，使用默认值
	result, err := em.MapEnum(int32(99), reflect.TypeOf(int32(0)), int32(777))
	assert.NoError(t, err)
	assert.Equal(t, int32(777), result)

	// 测试 nil 默认值
	result, err = em.MapEnum(int32(99), reflect.TypeOf(int32(0)), nil)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), result)
}

func TestEnumMapper_MapEnum_DifferentIntTypes(t *testing.T) {
	em := NewEnumMapper()
	em.AddMapping(1, 100)
	em.AddMapping(2, 200)

	// 测试 int 类型
	result, err := em.MapEnum(int(1), reflect.TypeOf(int32(0)), int32(0))
	assert.NoError(t, err)
	assert.Equal(t, int32(100), result)

	// 测试 int64 类型
	result, err = em.MapEnum(int64(2), reflect.TypeOf(int32(0)), int32(0))
	assert.NoError(t, err)
	assert.Equal(t, int32(200), result)

	// 测试 uint32 类型
	result, err = em.MapEnum(uint32(1), reflect.TypeOf(int32(0)), int32(0))
	assert.NoError(t, err)
	assert.Equal(t, int32(100), result)

	// 测试 uint64 类型
	result, err = em.MapEnum(uint64(2), reflect.TypeOf(int32(0)), int32(0))
	assert.NoError(t, err)
	assert.Equal(t, int32(200), result)

	// 测试 uint 类型
	result, err = em.MapEnum(uint(1), reflect.TypeOf(int32(0)), int32(0))
	assert.NoError(t, err)
	assert.Equal(t, int32(100), result)
}

func TestGenericEnumMapper_ChainedRegister(t *testing.T) {
	gem := NewGenericEnumMapper[int32, int32](0)

	// 测试链式注册
	result := gem.Register(1, 100).Register(2, 200).Register(3, 300)

	assert.Equal(t, gem, result) // 验证返回自身
	assert.Equal(t, int32(100), gem.Map(1))
	assert.Equal(t, int32(200), gem.Map(2))
	assert.Equal(t, int32(300), gem.Map(3))
}

func TestGenericEnumMapper_OverwriteMapping(t *testing.T) {
	gem := NewGenericEnumMapper[int32, int32](0)

	// 第一次注册
	gem.Register(1, 100)
	assert.Equal(t, int32(100), gem.Map(1))

	// 覆盖映射
	gem.Register(1, 999)
	assert.Equal(t, int32(999), gem.Map(1))

	// 验证反向映射也更新了
	source, exists := gem.ReverseMap(999)
	assert.True(t, exists)
	assert.Equal(t, int32(1), source)
}

func TestGenericEnumMapper_DifferentTypes(t *testing.T) {
	// 测试 string -> int
	stringMapper := NewGenericEnumMapper[string, int](0)
	stringMapper.Register("one", 1).Register("two", 2)

	assert.Equal(t, 1, stringMapper.Map("one"))
	assert.Equal(t, 2, stringMapper.Map("two"))
	assert.Equal(t, 0, stringMapper.Map("three")) // 默认值

	// 测试 bool -> string
	boolMapper := NewGenericEnumMapper[bool, string]("unknown")
	boolMapper.Register(true, "yes").Register(false, "no")

	assert.Equal(t, "yes", boolMapper.Map(true))
	assert.Equal(t, "no", boolMapper.Map(false))
}

// 性能基准测试
func BenchmarkGenericEnumMapper_Map(b *testing.B) {
	mapper := NewGenericEnumMapper[int, string]("default")
	for i := 0; i < 100; i++ {
		mapper.Register(i, string(rune('a'+i%26)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mapper.Map(i % 100)
	}
}

func BenchmarkAutoEnumConverter_Convert(b *testing.B) {
	converter := NewAutoEnumConverter[int, string]("default")
	mappings := make(map[int]string, 100)
	for i := 0; i < 100; i++ {
		mappings[i] = string(rune('a' + i%26))
	}
	converter.AutoRegister(mappings)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		converter.Convert(i % 100)
	}
}

// TestEnumMapper_AddMapping_ChainedCalls 测试 AddMapping 链式调用
func TestEnumMapper_AddMapping_ChainedCalls(t *testing.T) {
	em := NewEnumMapper()

	// 测试链式调用
	result := em.AddMapping(1, 100).AddMapping(2, 200)
	assert.Equal(t, em, result, "AddMapping should return self")
	assert.Equal(t, int32(100), em.Map(1, 0))
	assert.Equal(t, int32(200), em.Map(2, 0))
}

// TestEnumMapper_AddMappings_ChainedCalls 测试 AddMappings 链式调用
func TestEnumMapper_AddMappings_ChainedCalls(t *testing.T) {
	em := NewEnumMapper()

	pairs := [][2]int32{{1, 100}, {2, 200}}
	result := em.AddMappings(pairs)

	assert.Equal(t, em, result, "AddMappings should return self")
}

// TestEnumMapper_MapEnum_NonInt32DefaultValue 测试非 int32 默认值
func TestEnumMapper_MapEnum_NonInt32DefaultValue(t *testing.T) {
	em := NewEnumMapper()
	em.AddMapping(1, 100)

	// 测试字符串类型默认值（不是 int32）
	result, err := em.MapEnum(int32(99), reflect.TypeOf(int32(0)), "invalid_default")
	assert.NoError(t, err)
	assert.Equal(t, int32(0), result, "Non-int32 default value should be ignored")
}

// TestEnumMapper_MapEnum_AllIntTypes 测试所有支持的整数类型
func TestEnumMapper_MapEnum_AllIntTypes(t *testing.T) {
	em := NewEnumMapper()
	em.AddMapping(1, 100)

	tests := []struct {
		name   string
		source interface{}
		want   int32
	}{
		{"int", int(1), 100},
		{"int32", int32(1), 100},
		{"int64", int64(1), 100},
		{"uint", uint(1), 100},
		{"uint32", uint32(1), 100},
		{"uint64", uint64(1), 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := em.MapEnum(tt.source, reflect.TypeOf(int32(0)), int32(0))
			assert.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestEnumMapper_MapEnum_UnsupportedTypes 测试不支持的类型
func TestEnumMapper_MapEnum_UnsupportedTypes(t *testing.T) {
	em := NewEnumMapper()

	tests := []struct {
		name   string
		source interface{}
	}{
		{"string", "invalid"},
		{"bool", true},
		{"float32", float32(1.5)},
		{"float64", float64(1.5)},
		{"struct", struct{}{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := em.MapEnum(tt.source, reflect.TypeOf(int32(0)), int32(0))
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported source enum type")
		})
	}
}

// TestGenericEnumMapper_RegisterBatch_EmptyMap 测试空 map 批量注册
func TestGenericEnumMapper_RegisterBatch_EmptyMap(t *testing.T) {
	gem := NewGenericEnumMapper[int32, int32](0)

	result := gem.RegisterBatch(map[int32]int32{})
	assert.Equal(t, gem, result, "RegisterBatch should return self")
	assert.Equal(t, int32(0), gem.Map(1), "Empty batch should not affect mapper")
}

// TestGenericEnumMapper_ReverseMap_ZeroValue 测试反向映射零值
func TestGenericEnumMapper_ReverseMap_ZeroValue(t *testing.T) {
	gem := NewGenericEnumMapper[int32, int32](0)
	gem.Register(1, 0) // 注册零值映射

	source, exists := gem.ReverseMap(0)
	assert.True(t, exists, "Zero value mapping should exist")
	assert.Equal(t, int32(1), source)
}

// TestGenericEnumMapper_MapSlice_LargeSlice 测试大切片映射
func TestGenericEnumMapper_MapSlice_LargeSlice(t *testing.T) {
	gem := NewGenericEnumMapper[int, int](0)

	// 注册 1000 个映射
	for i := 0; i < 1000; i++ {
		gem.Register(i, i*10)
	}

	// 测试大切片
	sources := make([]int, 1000)
	for i := range sources {
		sources[i] = i
	}

	results := gem.MapSlice(sources)
	assert.Equal(t, 1000, len(results))
	for i := range results {
		assert.Equal(t, i*10, results[i])
	}
}

// TestAutoEnumConverter_AutoRegister_Multiple 测试多次 AutoRegister
func TestAutoEnumConverter_AutoRegister_Multiple(t *testing.T) {
	converter := NewAutoEnumConverter[string, int](0)

	// 第一次注册
	converter.AutoRegister(map[string]int{
		"one": 1,
		"two": 2,
	})

	// 第二次注册（覆盖 + 新增）
	converter.AutoRegister(map[string]int{
		"two":   22, // 覆盖
		"three": 3,  // 新增
	})

	assert.Equal(t, 1, converter.Convert("one"))
	assert.Equal(t, 22, converter.Convert("two"), "Should be overwritten")
	assert.Equal(t, 3, converter.Convert("three"))
}

// TestAutoEnumConverter_ConvertSlice_EmptySlice 测试空切片转换
func TestAutoEnumConverter_ConvertSlice_EmptySlice(t *testing.T) {
	converter := NewAutoEnumConverter[int, string]("default")

	results := converter.ConvertSlice([]int{})
	assert.Empty(t, results)
}

// TestAutoEnumConverter_ConvertWithDefault_ExistingMapping 测试已存在映射时的默认值
func TestAutoEnumConverter_ConvertWithDefault_ExistingMapping(t *testing.T) {
	converter := NewAutoEnumConverter[int, string]("default")
	converter.AutoRegister(map[int]string{1: "one"})

	// 存在映射时，应返回映射值而不是提供的默认值
	result := converter.ConvertWithDefault(1, "custom_default")
	assert.Equal(t, "one", result, "Should use mapping, not custom default")
}

// TestConvertEnum_EdgeCases 测试 ConvertEnum 边界情况
func TestConvertEnum_EdgeCases(t *testing.T) {
	em := NewEnumMapper()
	em.AddMapping(0, 0)   // 零值映射
	em.AddMapping(-1, -1) // 负数映射

	// 测试零值
	result := ConvertEnum(em, int32(0), int32(999))
	assert.Equal(t, int32(0), result)

	// 测试负数
	result = ConvertEnum(em, int32(-1), int32(999))
	assert.Equal(t, int32(-1), result)
}

// TestGenericEnumMapper_TypeSafety 测试类型安全
func TestGenericEnumMapper_TypeSafety(t *testing.T) {
	// 测试不同类型的泛型映射器互不干扰
	intMapper := NewGenericEnumMapper[int, string]("default")
	stringMapper := NewGenericEnumMapper[string, int](0)

	intMapper.Register(1, "one")
	stringMapper.Register("one", 1)

	assert.Equal(t, "one", intMapper.Map(1))
	assert.Equal(t, 1, stringMapper.Map("one"))
}

// TestAutoEnumConverter_NilSafety 测试 nil 安全
func TestAutoEnumConverter_NilSafety(t *testing.T) {
	converter := NewAutoEnumConverter[int, string]("default")

	// 未注册任何映射，应返回默认值
	result := converter.Convert(999)
	assert.Equal(t, "default", result)

	// 反向转换不存在的值
	_, exists := converter.ConvertBack("non_existent")
	assert.False(t, exists)
}

// TestGenericEnumMapper_ComplexTypes 测试复杂类型
func TestGenericEnumMapper_ComplexTypes(t *testing.T) {
	type CustomType string

	mapper := NewGenericEnumMapper[CustomType, CustomType]("default")
	mapper.Register("key1", "value1")
	mapper.Register("key2", "value2")

	assert.Equal(t, CustomType("value1"), mapper.Map("key1"))
	assert.Equal(t, CustomType("value2"), mapper.Map("key2"))
	assert.Equal(t, CustomType("default"), mapper.Map("key3"))
}

// TestAutoEnumConverter_ConcurrentAccess 测试并发访问安全性
func TestAutoEnumConverter_ConcurrentAccess(t *testing.T) {
	converter := NewAutoEnumConverter[int, string]("default")
	converter.AutoRegister(map[int]string{
		1: "one",
		2: "two",
		3: "three",
	})

	// 并发读取
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(val int) {
			defer func() { done <- true }()
			result := converter.Convert(val % 3)
			assert.NotEmpty(t, result)
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

// 性能基准测试 - 补充
func BenchmarkEnumMapper_Map(b *testing.B) {
	em := NewEnumMapper()
	for i := int32(0); i < 100; i++ {
		em.AddMapping(i, i*10)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		em.Map(int32(i%100), 0)
	}
}

func BenchmarkEnumMapper_AddMapping(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		em := NewEnumMapper()
		em.AddMapping(int32(i), int32(i*10))
	}
}

func BenchmarkGenericEnumMapper_Register(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mapper := NewGenericEnumMapper[int, int](0)
		mapper.Register(i, i*10)
	}
}

func BenchmarkAutoEnumConverter_AutoRegister(b *testing.B) {
	mappings := make(map[int]string, 100)
	for i := 0; i < 100; i++ {
		mappings[i] = string(rune('a' + i%26))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		converter := NewAutoEnumConverter[int, string]("default")
		converter.AutoRegister(mappings)
	}
}
