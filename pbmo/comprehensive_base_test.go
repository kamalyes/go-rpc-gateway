/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-13 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 21:21:52
 * @FilePath: \go-rpc-gateway\pbmo\assert_test.go
 * @Description: pbmo 模块的 assert 校验测试
 * 测试内容：转换、校验、错误处理、SafeConverter 等核心功能
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestBidiConverterBasic 测试基础双向转换
func TestBidiConverterBasic(t *testing.T) {
	type SimpleModel struct {
		ID   int64
		Name string
	}

	type SimplePB struct {
		ID   int64
		Name string
	}

	converter := NewBidiConverter(&SimplePB{}, &SimpleModel{})

	// 测试 PB -> Model
	pb := &SimplePB{ID: 1, Name: "test"}
	var model SimpleModel

	err := converter.ConvertPBToModel(pb, &model)
	assert.NoError(t, err, "ConvertPBToModel should not return error")
	assert.Equal(t, int64(1), model.ID, "ID should be 1")
	assert.Equal(t, "test", model.Name, "Name should be 'test'")

	// 测试 Model -> PB
	var pbResult SimplePB
	err = converter.ConvertModelToPB(&model, &pbResult)
	assert.NoError(t, err, "ConvertModelToPB should not return error")
	assert.Equal(t, int64(1), pbResult.ID, "ID should be 1")
	assert.Equal(t, "test", pbResult.Name, "Name should be 'test'")
}

// TestBidiConverterNilHandling 测试 nil 处理
func TestBidiConverterNilHandling(t *testing.T) {
	type Model struct {
		ID int64
	}

	type PB struct {
		ID int64
	}

	converter := NewBidiConverter(&PB{}, &Model{})

	// nil pb 应该返回错误
	var model Model
	err := converter.ConvertPBToModel(nil, &model)
	assert.Error(t, err, "ConvertPBToModel should return error for nil pb")

	// nil modelPtr 应该返回错误
	pb := &PB{ID: 1}
	err = converter.ConvertPBToModel(pb, nil)
	assert.Error(t, err, "ConvertPBToModel should return error for nil modelPtr")
}

// TestTimestampConversion 测试时间戳转换
func TestTimestampConversion(t *testing.T) {
	type Model struct {
		CreatedAt time.Time
	}

	type PB struct {
		CreatedAt *timestamppb.Timestamp
	}

	converter := NewBidiConverter(&PB{}, &Model{})

	// PB -> Model
	now := time.Now().UTC()
	pb := &PB{CreatedAt: timestamppb.New(now)}
	var model Model

	err := converter.ConvertPBToModel(pb, &model)
	assert.NoError(t, err, "Timestamp conversion should not return error")
	assert.True(t, model.CreatedAt.Equal(now), "Converted time should match original")

	// Model -> PB
	var pbResult PB
	err = converter.ConvertModelToPB(&model, &pbResult)
	assert.NoError(t, err, "Timestamp conversion should not return error")
	assert.NotNil(t, pbResult.CreatedAt, "Timestamp should not be nil")
}

// TestIntegerConversion 测试整数转换
func TestIntegerConversion(t *testing.T) {
	type Model struct {
		Count uint
	}

	type PB struct {
		Count int64
	}

	converter := NewBidiConverter(&PB{}, &Model{})

	// PB -> Model: int64 -> uint
	pb := &PB{Count: 42}
	var model Model

	err := converter.ConvertPBToModel(pb, &model)
	assert.NoError(t, err, "Integer conversion should not return error")
	assert.Equal(t, uint(42), model.Count, "Count should be 42")
}

// TestSliceConversion 测试切片转换
func TestSliceConversion(t *testing.T) {
	type Model struct {
		IDs []int64
	}

	type PB struct {
		IDs []int64
	}

	converter := NewBidiConverter(&PB{}, &Model{})

	// PB -> Model
	pb := &PB{IDs: []int64{1, 2, 3}}
	var model Model

	err := converter.ConvertPBToModel(pb, &model)
	assert.NoError(t, err, "Slice conversion should not return error")
	assert.Len(t, model.IDs, 3, "Slice should have 3 elements")
	assert.Equal(t, []int64{1, 2, 3}, model.IDs, "Slice elements should match")
}

// TestValidatorRequired 测试必填字段校验
func TestValidatorRequired(t *testing.T) {
	type User struct {
		Name string
		Age  int
	}

	validator := NewFieldValidator()
	validator.RegisterRules("User",
		FieldRule{
			Name:     "Name",
			Required: true,
		},
	)

	// 空的 name 应该校验失败
	user := &User{Name: "", Age: 18}
	err := validator.Validate(user)
	assert.Error(t, err, "Validation should fail for empty Name")

	// 有效的数据应该通过
	user.Name = "Alice"
	err = validator.Validate(user)
	assert.NoError(t, err, "Validation should pass for valid Name")
}

// TestValidatorMinMax 测试最小/最大值校验
func TestValidatorMinMax(t *testing.T) {
	type Age struct {
		Value int
	}

	validator := NewFieldValidator()
	validator.RegisterRules("Age",
		FieldRule{
			Name: "Value",
			Min:  0,
			Max:  150,
		},
	)

	// 小于最小值
	age := &Age{Value: -1}
	err := validator.Validate(age)
	assert.Error(t, err, "Validation should fail for negative age")

	// 超过最大值
	age.Value = 200
	err = validator.Validate(age)
	assert.Error(t, err, "Validation should fail for age > 150")

	// 有效范围
	age.Value = 25
	err = validator.Validate(age)
	assert.NoError(t, err, "Validation should pass for valid age")
}

// TestValidatorPattern 测试正则表达式校验
func TestValidatorPattern(t *testing.T) {
	type Email struct {
		Value string
	}

	validator := NewFieldValidator()
	validator.RegisterRules("Email",
		FieldRule{
			Name:    "Value",
			Pattern: `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
		},
	)

	// 无效邮箱
	email := &Email{Value: "invalid"}
	err := validator.Validate(email)
	assert.Error(t, err, "Validation should fail for invalid email")

	// 有效邮箱
	email.Value = "test@example.com"
	err = validator.Validate(email)
	assert.NoError(t, err, "Validation should pass for valid email")
}

// TestSafeConverterBasic 测试安全转换器基础功能
func TestSafeConverterBasic(t *testing.T) {
	type Model struct {
		ID   int64
		Name string
	}

	type PB struct {
		ID   int64
		Name string
	}

	converter := NewSafeConverter(&PB{}, &Model{})

	// 有效的转换
	pb := &PB{ID: 1, Name: "test"}
	var model Model

	err := converter.SafeConvertPBToModel(pb, &model)
	assert.NoError(t, err, "SafeConvertPBToModel should not return error")
	assert.Equal(t, int64(1), model.ID, "ID should be 1")
	assert.Equal(t, "test", model.Name, "Name should be 'test'")

	// nil pb 应该返回 ConversionError
	err = converter.SafeConvertPBToModel(nil, &model)
	assert.Error(t, err, "SafeConvertPBToModel should return error for nil pb")

	convErr, ok := err.(*ConversionError)
	assert.True(t, ok, "Error should be of type *ConversionError")
	assert.Equal(t, "SafeConvertPBToModel", convErr.Operation, "Operation should match")
}

// TestSafeFieldAccess 测试安全字段访问
func TestSafeFieldAccess(t *testing.T) {
	type Address struct {
		City    string
		Country string
	}

	type Profile struct {
		Address *Address
	}

	type User struct {
		Name    string
		Profile *Profile
	}

	converter := NewSafeConverter(nil, nil)

	// 有效的嵌套对象
	user := &User{
		Name: "Alice",
		Profile: &Profile{
			Address: &Address{
				City:    "Beijing",
				Country: "China",
			},
		},
	}

	// 访问深层字段
	city := converter.SafeFieldAccess(user, "Profile", "Address", "City").String("Unknown")
	assert.Equal(t, "Beijing", city, "City should be 'Beijing'")

	// 访问不存在的中间字段（nil）
	user2 := &User{
		Name:    "Bob",
		Profile: nil,
	}

	city2 := converter.SafeFieldAccess(user2, "Profile", "Address", "City").String("Unknown")
	assert.Equal(t, "Unknown", city2, "City should default to 'Unknown'")
}

// TestSafeBatchConvert 测试安全批量转换
func TestSafeBatchConvert(t *testing.T) {
	type Model struct {
		ID int64
	}

	type PB struct {
		ID int64
	}

	converter := NewSafeConverter(&PB{}, &Model{})

	// 有效的批量数据
	pbs := []*PB{
		{ID: 1},
		{ID: 2},
		{ID: 3},
	}

	var models []*Model
	result := converter.SafeBatchConvertPBToModel(pbs, &models)

	assert.Equal(t, 3, result.SuccessCount, "Should have 3 successful conversions")
	assert.Equal(t, 0, result.FailureCount, "Should have 0 failed conversions")
	assert.Len(t, result.Results, 3, "Should have 3 results")

	// 检查转换结果
	for i, item := range result.Results {
		assert.True(t, item.Success, "Item %d should succeed", i)
		assert.Equal(t, i, item.Index, "Item %d index mismatch", i)
	}
}

// TestConversionErrorType 测试转换错误类型
func TestConversionErrorType(t *testing.T) {
	err := NewConversionError(
		"field type mismatch",
		"ConvertPBToModel",
		"*pb.User",
		"*User",
	)

	// 检查错误消息格式
	expectedMsg := "[ConvertPBToModel] field type mismatch (from *pb.User to *User)"
	assert.Equal(t, expectedMsg, err.Error(), "Error message mismatch")

	// 检查错误字段
	assert.Equal(t, "ConvertPBToModel", err.Operation, "Operation mismatch")
	assert.Equal(t, "*pb.User", err.SourceType, "SourceType mismatch")
	assert.Equal(t, "*User", err.TargetType, "TargetType mismatch")

	// 检查 Is 方法
	var convErr *ConversionError
	assert.True(t, errors.Is(err, convErr), "errors.Is should recognize ConversionError")
}

// TestBatchConvertPBToModel 测试批量 PB -> Model 转换
func TestBatchConvertPBToModel(t *testing.T) {
	type Model struct {
		ID   int64
		Name string
	}

	type PB struct {
		ID   int64
		Name string
	}

	converter := NewBidiConverter(&PB{}, &Model{})

	pbs := []*PB{
		{ID: 1, Name: "Alice"},
		{ID: 2, Name: "Bob"},
	}

	var models []*Model
	err := converter.BatchConvertPBToModel(pbs, &models)
	assert.NoError(t, err, "BatchConvertPBToModel should not return error")
	assert.Len(t, models, 2, "Should have 2 models")
	assert.Equal(t, "Alice", models[0].Name, "First model name should be 'Alice'")
}

// TestBatchConvertModelToPB 测试批量 Model -> PB 转换
func TestBatchConvertModelToPB(t *testing.T) {
	type Model struct {
		ID   int64
		Name string
	}

	type PB struct {
		ID   int64
		Name string
	}

	converter := NewBidiConverter(&PB{}, &Model{})

	models := []*Model{
		{ID: 1, Name: "Alice"},
		{ID: 2, Name: "Bob"},
	}

	var pbs []*PB
	err := converter.BatchConvertModelToPB(models, &pbs)
	assert.NoError(t, err, "BatchConvertModelToPB should not return error")
	assert.Len(t, pbs, 2, "Should have 2 pbs")
	assert.Equal(t, "Alice", pbs[0].Name, "First pb name should be 'Alice'")
}

// TestFieldTransformer 测试字段转换器
func TestFieldTransformer(t *testing.T) {
	type Model struct {
		Status string
	}

	type PB struct {
		Status int32
	}

	converter := NewBidiConverter(&PB{}, &Model{})

	// 注册转换器
	converter.RegisterTransformer("Status", func(v interface{}) interface{} {
		if code, ok := v.(int32); ok {
			switch code {
			case 1:
				return "active"
			case 2:
				return "inactive"
			}
		}
		return "unknown"
	})

	pb := &PB{Status: 1}
	var model Model

	err := converter.ConvertPBToModel(pb, &model)
	assert.NoError(t, err, "Transformer conversion should not return error")
	assert.Equal(t, "active", model.Status, "Status should be 'active'")
}

// TestPointerConversion 测试指针转换
func TestPointerConversion(t *testing.T) {
	type Model struct {
		Name *string
	}

	type PB struct {
		Name *string
	}

	converter := NewBidiConverter(&PB{}, &Model{})

	name := "test"
	pb := &PB{Name: &name}
	var model Model

	err := converter.ConvertPBToModel(pb, &model)
	assert.NoError(t, err, "Pointer conversion should not return error")
	assert.NotNil(t, model.Name, "Name pointer should not be nil")
	assert.Equal(t, "test", *model.Name, "Name value should be 'test'")
}

// TestValidatorStringLength 测试字符串长度校验
func TestValidatorStringLength(t *testing.T) {
	type Username struct {
		Value string
	}

	validator := NewFieldValidator()
	validator.RegisterRules("Username",
		FieldRule{
			Name:   "Value",
			MinLen: 3,
			MaxLen: 20,
		},
	)

	// 太短
	user := &Username{Value: "ab"}
	err := validator.Validate(user)
	assert.Error(t, err, "Validation should fail for short string")

	// 太长
	user.Value = "abcdefghijklmnopqrstuvwxyz"
	err = validator.Validate(user)
	assert.Error(t, err, "Validation should fail for long string")

	// 有效长度
	user.Value = "alice"
	err = validator.Validate(user)
	assert.NoError(t, err, "Validation should pass for valid length")
}

// TestIsIntegerType 测试整数类型判断
func TestIsIntegerType(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{"int", int(5), true},
		{"int64", int64(5), true},
		{"uint", uint(5), true},
		{"uint64", uint64(5), true},
		{"string", "5", false},
		{"float64", float64(5), false},
	}

	for _, test := range tests {
		result := isIntegerType(reflect.TypeOf(test.value))
		assert.Equal(t, test.expected, result, "isIntegerType(%s) mismatch", test.name)
	}
}

// TestGetTypeName 测试类型名称获取
func TestGetTypeName(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"nil type", nil, "nil"},
		{"string type", "test", "string"},
		{"int type", 42, "int"},
		{"pointer type", (*int)(nil), "*int"},
	}

	for _, test := range tests {
		result := getTypeName(reflect.TypeOf(test.value))
		assert.Equal(t, test.expected, result, "getTypeName(%s) mismatch", test.name)
	}
}

// BenchmarkOptimizedConverterPBToModel 优化转换器性能测试
func BenchmarkOptimizedConverterPBToModel(b *testing.B) {
	type Model struct {
		ID        int64
		Name      string
		Email     string
		Phone     string
		Age       int
		Active    bool
		Balance   float64
		CreatedAt time.Time
	}

	type PB struct {
		ID        int64
		Name      string
		Email     string
		Phone     string
		Age       int
		Active    bool
		Balance   float64
		CreatedAt time.Time
	}

	converter := NewOptimizedBidiConverter(&PB{}, &Model{})
	pb := &PB{
		ID:        1,
		Name:      "Alice",
		Email:     "alice@example.com",
		Phone:     "+1234567890",
		Age:       30,
		Active:    true,
		Balance:   1000.50,
		CreatedAt: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var model Model
		_ = converter.ConvertPBToModel(pb, &model)
	}
}

// BenchmarkUltraFastConverterPBToModel 极速转换器性能测试
func BenchmarkUltraFastConverterPBToModel(b *testing.B) {
	type Model struct {
		ID        int64
		Name      string
		Email     string
		Phone     string
		Age       int
		Active    bool
		Balance   float64
		CreatedAt time.Time
	}

	type PB struct {
		ID        int64
		Name      string
		Email     string
		Phone     string
		Age       int
		Active    bool
		Balance   float64
		CreatedAt time.Time
	}

	converter := NewUltraFastConverter(&PB{}, &Model{})
	pb := &PB{
		ID:        1,
		Name:      "Alice",
		Email:     "alice@example.com",
		Phone:     "+1234567890",
		Age:       30,
		Active:    true,
		Balance:   1000.50,
		CreatedAt: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var model Model
		_ = converter.ConvertPBToModel(pb, &model)
	}
}

// BenchmarkBidiConverterModelToPB 模型转PB性能测试
func BenchmarkBidiConverterModelToPB(b *testing.B) {
	type Model struct {
		ID        int64
		Name      string
		Email     string
		Phone     string
		Age       int
		Active    bool
		Balance   float64
		CreatedAt time.Time
	}

	type PB struct {
		ID        int64
		Name      string
		Email     string
		Phone     string
		Age       int
		Active    bool
		Balance   float64
		CreatedAt time.Time
	}

	converter := NewBidiConverter(&PB{}, &Model{})
	model := &Model{
		ID:        1,
		Name:      "Alice",
		Email:     "alice@example.com",
		Phone:     "+1234567890",
		Age:       30,
		Active:    true,
		Balance:   1000.50,
		CreatedAt: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var pb PB
		_ = converter.ConvertModelToPB(model, &pb)
	}
}

// BenchmarkOptimizedConverterModelToPB 优化转换器 Model->PB 性能测试
func BenchmarkOptimizedConverterModelToPB(b *testing.B) {
	type Model struct {
		ID        int64
		Name      string
		Email     string
		Phone     string
		Age       int
		Active    bool
		Balance   float64
		CreatedAt time.Time
	}

	type PB struct {
		ID        int64
		Name      string
		Email     string
		Phone     string
		Age       int
		Active    bool
		Balance   float64
		CreatedAt time.Time
	}

	converter := NewOptimizedBidiConverter(&PB{}, &Model{})
	model := &Model{
		ID:        1,
		Name:      "Alice",
		Email:     "alice@example.com",
		Phone:     "+1234567890",
		Age:       30,
		Active:    true,
		Balance:   1000.50,
		CreatedAt: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var pb PB
		_ = converter.ConvertModelToPB(model, &pb)
	}
}

// BenchmarkUltraFastConverterModelToPB 极速转换器 Model->PB 性能测试
func BenchmarkUltraFastConverterModelToPB(b *testing.B) {
	type Model struct {
		ID        int64
		Name      string
		Email     string
		Phone     string
		Age       int
		Active    bool
		Balance   float64
		CreatedAt time.Time
	}

	type PB struct {
		ID        int64
		Name      string
		Email     string
		Phone     string
		Age       int
		Active    bool
		Balance   float64
		CreatedAt time.Time
	}

	converter := NewUltraFastConverter(&PB{}, &Model{})
	model := &Model{
		ID:        1,
		Name:      "Alice",
		Email:     "alice@example.com",
		Phone:     "+1234567890",
		Age:       30,
		Active:    true,
		Balance:   1000.50,
		CreatedAt: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var pb PB
		_ = converter.ConvertModelToPB(model, &pb)
	}
}

// BenchmarkBatchConvert100Items 批量转换性能测试（100项）
func BenchmarkBatchConvert100Items(b *testing.B) {
	type Model struct {
		ID   int64
		Name string
	}

	type PB struct {
		ID   int64
		Name string
	}

	converter := NewBidiConverter(&PB{}, &Model{})

	pbs := make([]*PB, 100)
	for i := 0; i < 100; i++ {
		pbs[i] = &PB{ID: int64(i), Name: "test"}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var models []*Model
		_ = converter.BatchConvertPBToModel(pbs, &models)
	}
}

// BenchmarkOptimizedBatchConvert100Items 优化批量转换性能测试（100项）
func BenchmarkOptimizedBatchConvert100Items(b *testing.B) {
	type Model struct {
		ID   int64
		Name string
	}

	type PB struct {
		ID   int64
		Name string
	}

	converter := NewOptimizedBidiConverter(&PB{}, &Model{})

	pbs := make([]*PB, 100)
	for i := 0; i < 100; i++ {
		pbs[i] = &PB{ID: int64(i), Name: "test"}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var models []*Model
		_ = converter.BatchConvertPBToModel(pbs, &models)
	}
}

// BenchmarkBatchConvert1000Items 批量转换性能测试（1000项）
func BenchmarkBatchConvert1000Items(b *testing.B) {
	type Model struct {
		ID   int64
		Name string
	}

	type PB struct {
		ID   int64
		Name string
	}

	converter := NewBidiConverter(&PB{}, &Model{})

	pbs := make([]*PB, 1000)
	for i := 0; i < 1000; i++ {
		pbs[i] = &PB{ID: int64(i), Name: "test"}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var models []*Model
		_ = converter.BatchConvertPBToModel(pbs, &models)
	}
}

// BenchmarkOptimizedBatchConvert1000Items 优化批量转换性能测试（1000项）
func BenchmarkOptimizedBatchConvert1000Items(b *testing.B) {
	type Model struct {
		ID   int64
		Name string
	}

	type PB struct {
		ID   int64
		Name string
	}

	converter := NewOptimizedBidiConverter(&PB{}, &Model{})

	pbs := make([]*PB, 1000)
	for i := 0; i < 1000; i++ {
		pbs[i] = &PB{ID: int64(i), Name: "test"}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var models []*Model
		_ = converter.BatchConvertPBToModel(pbs, &models)
	}
}

// BenchmarkValidatorRequired 必填字段校验性能测试
func BenchmarkValidatorRequired(b *testing.B) {
	type User struct {
		Name  string
		Email string
		Age   int
	}

	validator := NewFieldValidator()
	validator.RegisterRules("User",
		FieldRule{Name: "Name", Required: true},
		FieldRule{Name: "Email", Required: true},
	)

	user := &User{Name: "Alice", Email: "alice@example.com", Age: 30}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(user)
	}
}

// BenchmarkValidatorPattern 正则校验性能测试
func BenchmarkValidatorPattern(b *testing.B) {
	type Email struct {
		Value string
	}

	validator := NewFieldValidator()
	validator.RegisterRules("Email",
		FieldRule{
			Name:    "Value",
			Pattern: `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
		},
	)

	email := &Email{Value: "test@example.com"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(email)
	}
}

// BenchmarkSafeConverterBasic 安全转换器性能测试
func BenchmarkSafeConverterBasic(b *testing.B) {
	type Model struct {
		ID   int64
		Name string
	}

	type PB struct {
		ID   int64
		Name string
	}

	converter := NewSafeConverter(&PB{}, &Model{})
	pb := &PB{ID: 1, Name: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var model Model
		_ = converter.SafeConvertPBToModel(pb, &model)
	}
}

// BenchmarkSafeFieldAccessSingleLevel 单层字段访问性能测试
func BenchmarkSafeFieldAccessSingleLevel(b *testing.B) {
	type User struct {
		Name string
	}

	converter := NewSafeConverter(nil, nil)
	user := &User{Name: "Alice"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = converter.SafeFieldAccess(user, "Name").String("default")
	}
}

// BenchmarkSafeFieldAccessMultiLevel 多层字段访问性能测试
func BenchmarkSafeFieldAccessMultiLevel(b *testing.B) {
	type Address struct {
		City string
	}

	type Profile struct {
		Address *Address
	}

	type User struct {
		Profile *Profile
	}

	converter := NewSafeConverter(nil, nil)
	user := &User{
		Profile: &Profile{
			Address: &Address{City: "Beijing"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = converter.SafeFieldAccess(user, "Profile", "Address", "City").String("Unknown")
	}
}

// BenchmarkSafeBatchConvert100Items 安全批量转换性能测试（100项）
func BenchmarkSafeBatchConvert100Items(b *testing.B) {
	type Model struct {
		ID int64
	}

	type PB struct {
		ID int64
	}

	converter := NewSafeConverter(&PB{}, &Model{})

	pbs := make([]*PB, 100)
	for i := 0; i < 100; i++ {
		pbs[i] = &PB{ID: int64(i)}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var models []*Model
		_ = converter.SafeBatchConvertPBToModel(pbs, &models)
	}
}

// BenchmarkConversionErrorCreation 转换错误创建性能测试
func BenchmarkConversionErrorCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewConversionError(
			"test error",
			"TestOperation",
			"*pb.Model",
			"*Model",
		)
	}
}

// BenchmarkFieldTransformerConversion 字段转换器性能测试
func BenchmarkFieldTransformerConversion(b *testing.B) {
	type Model struct {
		Status string
	}

	type PB struct {
		Status int32
	}

	converter := NewBidiConverter(&PB{}, &Model{})
	converter.RegisterTransformer("Status", func(v interface{}) interface{} {
		if code, ok := v.(int32); ok {
			switch code {
			case 1:
				return "active"
			case 2:
				return "inactive"
			}
		}
		return "unknown"
	})

	pb := &PB{Status: 1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var model Model
		_ = converter.ConvertPBToModel(pb, &model)
	}
}

// BenchmarkTimestampConversion 时间戳转换性能测试
func BenchmarkTimestampConversion(b *testing.B) {
	type Model struct {
		CreatedAt time.Time
	}

	type PB struct {
		CreatedAt *timestamppb.Timestamp
	}

	converter := NewBidiConverter(&PB{}, &Model{})
	now := time.Now().UTC()
	pb := &PB{CreatedAt: timestamppb.New(now)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var model Model
		_ = converter.ConvertPBToModel(pb, &model)
	}
}

// BenchmarkComplexStructConversion 复杂结构体转换性能测试
func BenchmarkComplexStructConversion(b *testing.B) {
	type Address struct {
		Street string
		City   string
		Zip    string
	}

	type Contact struct {
		Email   string
		Phone   string
		Address Address
	}

	type Person struct {
		ID      int64
		Name    string
		Age     int
		Contact Contact
	}

	type PBAddress struct {
		Street string
		City   string
		Zip    string
	}

	type PBContact struct {
		Email   string
		Phone   string
		Address PBAddress
	}

	type PBPerson struct {
		ID      int64
		Name    string
		Age     int
		Contact PBContact
	}

	converter := NewBidiConverter(&PBPerson{}, &Person{})
	pb := &PBPerson{
		ID:   1,
		Name: "Alice",
		Age:  30,
		Contact: PBContact{
			Email: "alice@example.com",
			Phone: "+1234567890",
			Address: PBAddress{
				Street: "123 Main St",
				City:   "Beijing",
				Zip:    "100000",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var person Person
		_ = converter.ConvertPBToModel(pb, &person)
	}
}

// BenchmarkValidatorMultipleRules 多规则校验性能测试
func BenchmarkValidatorMultipleRules(b *testing.B) {
	type User struct {
		Name     string
		Email    string
		Age      int
		Username string
		Phone    string
	}

	validator := NewFieldValidator()
	validator.RegisterRules("User",
		FieldRule{Name: "Name", Required: true, MinLen: 2, MaxLen: 50},
		FieldRule{Name: "Email", Required: true, Pattern: `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`},
		FieldRule{Name: "Age", Min: 18, Max: 120},
		FieldRule{Name: "Username", Required: true, MinLen: 3, MaxLen: 20},
		FieldRule{Name: "Phone", MinLen: 10, MaxLen: 15},
	)

	user := &User{
		Name:     "Alice",
		Email:    "alice@example.com",
		Age:      30,
		Username: "alice123",
		Phone:    "+1234567890",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(user)
	}
}
