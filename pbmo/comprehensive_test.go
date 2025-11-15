/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 09:45:02
 * @FilePath: \go-rpc-gateway\pbmo\comprehensive_test.go
 * @Description: 综合场景测试 - 300+ 复杂测试用例，覆盖所有类型
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================================
// 第一批: 50条复杂场景测试 (基础类型组合 + 边界值 + 特殊值)
// ============================================================================

// TestComprehensive1_50 包含复杂场景的前50条测试
func TestComprehensive1_50(t *testing.T) {
	type PBSimple struct {
		IntVal    int32
		Int64Val  int64
		UintVal   uint32
		Uint64Val uint64
		FloatVal  float32
		DoubleVal float64
		BoolVal   bool
		StringVal string
		BytesVal  []byte
		TimeVal   *timestamppb.Timestamp
	}

	type ModelSimple struct {
		IntVal    int32
		Int64Val  int64
		UintVal   uint32
		Uint64Val uint64
		FloatVal  float32
		DoubleVal float64
		BoolVal   bool
		StringVal string
		BytesVal  []byte
		TimeVal   *timestamppb.Timestamp
	}

	converter := NewBidiConverter(&PBSimple{}, &ModelSimple{})

	// ========== Case 1-5: int32 边界值组合 ==========

	// Case 1: int32最小值与其他类型组合
	pb1 := &PBSimple{
		IntVal:    math.MinInt32,
		Int64Val:  math.MinInt64,
		UintVal:   0,
		Uint64Val: 0,
		FloatVal:  -3.14,
		DoubleVal: -2.718,
		BoolVal:   false,
		StringVal: "min_value_combo",
	}
	model1 := &ModelSimple{}
	err := converter.ConvertPBToModel(pb1, model1)
	assert.NoError(t, err, "Case 1: int32最小值组合转换应成功")
	assert.Equal(t, int32(math.MinInt32), model1.IntVal, "Case 1: int32最小值应相等")
	assert.Equal(t, int64(math.MinInt64), model1.Int64Val, "Case 1: int64最小值应相等")
	assert.InDelta(t, float32(-3.14), model1.FloatVal, 0.01, "Case 1: float32应相等")
	assert.InDelta(t, -2.718, model1.DoubleVal, 0.001, "Case 1: float64应在误差范围内")

	// Case 2: int32最大值与其他类型组合
	pb2 := &PBSimple{
		IntVal:    math.MaxInt32,
		Int64Val:  math.MaxInt64,
		UintVal:   math.MaxUint32,
		Uint64Val: math.MaxUint64,
		FloatVal:  3.14,
		DoubleVal: 2.718,
		BoolVal:   true,
		StringVal: "max_value_combo",
	}
	model2 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb2, model2)
	assert.NoError(t, err, "Case 2: int32最大值组合转换应成功")
	assert.Equal(t, int32(math.MaxInt32), model2.IntVal, "Case 2: int32最大值应相等")
	assert.Equal(t, int64(math.MaxInt64), model2.Int64Val, "Case 2: int64最大值应相等")
	assert.Equal(t, uint32(math.MaxUint32), model2.UintVal, "Case 2: uint32最大值应相等")
	assert.Equal(t, uint64(math.MaxUint64), model2.Uint64Val, "Case 2: uint64最大值应相等")

	// Case 3: 零值组合（所有字段都是零值）
	pb3 := &PBSimple{
		IntVal:    0,
		Int64Val:  0,
		UintVal:   0,
		Uint64Val: 0,
		FloatVal:  0.0,
		DoubleVal: 0.0,
		BoolVal:   false,
		StringVal: "",
		BytesVal:  []byte{},
		TimeVal:   nil,
	}
	model3 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb3, model3)
	assert.NoError(t, err, "Case 3: 零值组合转换应成功")
	assert.Equal(t, int32(0), model3.IntVal, "Case 3: int32零值应为0")
	assert.Equal(t, int64(0), model3.Int64Val, "Case 3: int64零值应为0")
	assert.False(t, model3.BoolVal, "Case 3: bool零值应为false")
	assert.Empty(t, model3.StringVal, "Case 3: string零值应为空")

	// Case 4: 负数与正数混合
	pb4 := &PBSimple{
		IntVal:    -100,
		Int64Val:  100,
		UintVal:   50,
		Uint64Val: 200,
		FloatVal:  -1.5,
		DoubleVal: 2.5,
		BoolVal:   true,
		StringVal: "mixed_sign_values",
	}
	model4 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb4, model4)
	assert.NoError(t, err, "Case 4: 负数与正数混合转换应成功")
	assert.Equal(t, int32(-100), model4.IntVal, "Case 4: 负数int32应相等")
	assert.Equal(t, int64(100), model4.Int64Val, "Case 4: 正数int64应相等")
	assert.Equal(t, float32(-1.5), model4.FloatVal, "Case 4: 负数float32应相等")
	assert.InDelta(t, 2.5, model4.DoubleVal, 0.001, "Case 4: 正数float64应相等")

	// Case 5: 特殊浮点值（无穷大、NaN）
	pb5 := &PBSimple{
		IntVal:    1,
		FloatVal:  float32(math.Inf(1)),
		DoubleVal: math.Inf(-1),
		StringVal: "special_float_values",
	}
	model5 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb5, model5)
	assert.NoError(t, err, "Case 5: 特殊浮点值转换应成功")
	assert.True(t, math.IsInf(float64(model5.FloatVal), 1), "Case 5: float32正无穷应保持")
	assert.True(t, math.IsInf(model5.DoubleVal, -1), "Case 5: float64负无穷应保持")

	// ========== Case 6-10: uint64 超大值场景 ==========

	// Case 6: uint64最大值单独转换
	pb6 := &PBSimple{
		Uint64Val: math.MaxUint64,
		StringVal: "max_uint64_single",
	}
	model6 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb6, model6)
	assert.NoError(t, err, "Case 6: uint64最大值单独转换应成功")
	assert.Equal(t, uint64(math.MaxUint64), model6.Uint64Val, "Case 6: uint64最大值应保持")

	// Case 7: uint32与uint64交叉验证
	pb7 := &PBSimple{
		UintVal:   math.MaxUint32,
		Uint64Val: uint64(math.MaxUint32) + 1,
		StringVal: "uint32_uint64_cross",
	}
	model7 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb7, model7)
	assert.NoError(t, err, "Case 7: uint32与uint64交叉转换应成功")
	assert.Equal(t, uint32(math.MaxUint32), model7.UintVal, "Case 7: uint32应等于最大值")
	assert.Greater(t, model7.Uint64Val, uint64(model7.UintVal), "Case 7: uint64应大于uint32最大值")

	// Case 8: int64与uint64交界转换
	pb8 := &PBSimple{
		Int64Val:  math.MaxInt64,
		Uint64Val: math.MaxInt64 + 1,
		StringVal: "int64_uint64_boundary",
	}
	model8 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb8, model8)
	assert.NoError(t, err, "Case 8: int64与uint64交界转换应成功")
	assert.Equal(t, int64(math.MaxInt64), model8.Int64Val, "Case 8: int64最大值应保持")
	assert.Equal(t, uint64(math.MaxInt64)+1, model8.Uint64Val, "Case 8: uint64应超过int64最大值")

	// Case 9: 超大uint64与多个字段组合
	pb9 := &PBSimple{
		IntVal:    1,
		UintVal:   math.MaxUint32,
		Uint64Val: math.MaxUint64,
		FloatVal:  1.23,
		BoolVal:   true,
		StringVal: "large_uint64_combo",
	}
	model9 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb9, model9)
	assert.NoError(t, err, "Case 9: 超大uint64与多字段组合转换应成功")
	assert.Equal(t, uint64(math.MaxUint64), model9.Uint64Val, "Case 9: uint64最大值应保持")

	// Case 10: 精确uint64分界线
	pb10 := &PBSimple{
		Uint64Val: 9223372036854775808, // MaxInt64 + 1
		StringVal: "precise_uint64_boundary",
	}
	model10 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb10, model10)
	assert.NoError(t, err, "Case 10: 精确uint64分界线转换应成功")
	assert.Equal(t, uint64(9223372036854775808), model10.Uint64Val, "Case 10: 分界线值应精确保持")

	// ========== Case 11-15: 浮点数精度与特殊值 ==========

	// Case 11: float32精度极限
	pb11 := &PBSimple{
		FloatVal:  1.23456789,
		StringVal: "float32_precision",
	}
	model11 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb11, model11)
	assert.NoError(t, err, "Case 11: float32精度转换应成功")
	assert.InDelta(t, 1.23456789, model11.FloatVal, 0.0001, "Case 11: float32精度应在可接受范围")

	// Case 12: float64高精度
	pb12 := &PBSimple{
		DoubleVal: 1.23456789012345,
		StringVal: "float64_precision",
	}
	model12 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb12, model12)
	assert.NoError(t, err, "Case 12: float64精度转换应成功")
	assert.InDelta(t, 1.23456789012345, model12.DoubleVal, 1e-10, "Case 12: float64精度应保持")

	// Case 13: 极小正数（接近零）
	pb13 := &PBSimple{
		FloatVal:  1e-6,
		DoubleVal: 1e-15,
		StringVal: "very_small_positives",
	}
	model13 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb13, model13)
	assert.NoError(t, err, "Case 13: 极小正数转换应成功")
	assert.Greater(t, model13.FloatVal, float32(0), "Case 13: float32应大于零")
	assert.Greater(t, model13.DoubleVal, 0.0, "Case 13: float64应大于零")

	// Case 14: 极大浮点数
	pb14 := &PBSimple{
		FloatVal:  float32(1.7e37),
		DoubleVal: 1.7e307,
		StringVal: "very_large_floats",
	}
	model14 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb14, model14)
	assert.NoError(t, err, "Case 14: 极大浮点数转换应成功")
	assert.Greater(t, model14.FloatVal, float32(0), "Case 14: float32应为正数")
	assert.Greater(t, model14.DoubleVal, 0.0, "Case 14: float64应为正数")

	// Case 15: 浮点NaN值处理
	pb15 := &PBSimple{
		FloatVal:  float32(math.NaN()),
		DoubleVal: math.NaN(),
		StringVal: "nan_values",
	}
	model15 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb15, model15)
	assert.NoError(t, err, "Case 15: NaN值转换应成功")
	assert.True(t, math.IsNaN(float64(model15.FloatVal)), "Case 15: float32 NaN应保持")
	assert.True(t, math.IsNaN(model15.DoubleVal), "Case 15: float64 NaN应保持")

	// ========== Case 16-20: 时间戳复杂场景 ==========

	// Case 16: Unix epoch时间
	pb16 := &PBSimple{
		TimeVal:   timestamppb.New(time.Unix(0, 0)),
		StringVal: "unix_epoch",
	}
	model16 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb16, model16)
	assert.NoError(t, err, "Case 16: Unix epoch时间转换应成功")
	assert.NotNil(t, model16.TimeVal, "Case 16: 时间不应为nil")
	assert.Equal(t, int64(0), model16.TimeVal.GetSeconds(), "Case 16: 秒数应为0")

	// Case 17: 未来时间戳（2099年）
	futureTime := time.Date(2099, 12, 31, 23, 59, 59, 999999999, time.UTC)
	pb17 := &PBSimple{
		TimeVal:   timestamppb.New(futureTime),
		StringVal: "future_timestamp",
	}
	model17 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb17, model17)
	assert.NoError(t, err, "Case 17: 未来时间戳转换应成功")
	assert.NotNil(t, model17.TimeVal, "Case 17: 时间不应为nil")
	assert.WithinDuration(t, futureTime, model17.TimeVal.AsTime(), time.Microsecond, "Case 17: 时间应接近")

	// Case 18: 过去时间戳（1970年前）
	pastTime := time.Date(1950, 1, 1, 0, 0, 0, 0, time.UTC)
	pb18 := &PBSimple{
		TimeVal:   timestamppb.New(pastTime),
		StringVal: "past_timestamp",
	}
	model18 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb18, model18)
	assert.NoError(t, err, "Case 18: 过去时间戳转换应成功")
	assert.NotNil(t, model18.TimeVal, "Case 18: 时间不应为nil")
	assert.WithinDuration(t, pastTime, model18.TimeVal.AsTime(), time.Second, "Case 18: 时间应接近")

	// Case 19: 纳秒精度时间
	nanoTime := time.Date(2025, 6, 15, 12, 30, 45, 123456789, time.UTC)
	pb19 := &PBSimple{
		TimeVal:   timestamppb.New(nanoTime),
		StringVal: "nano_precision_time",
	}
	model19 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb19, model19)
	assert.NoError(t, err, "Case 19: 纳秒精度时间转换应成功")
	assert.NotNil(t, model19.TimeVal, "Case 19: 时间不应为nil")
	nanosecond := model19.TimeVal.GetNanos()
	assert.Equal(t, int32(123456789), nanosecond, "Case 19: 纳秒部分应保持精度")

	// Case 20: nil时间戳
	pb20 := &PBSimple{
		TimeVal:   nil,
		StringVal: "nil_timestamp",
	}
	model20 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb20, model20)
	assert.NoError(t, err, "Case 20: nil时间戳转换应成功")
	assert.Nil(t, model20.TimeVal, "Case 20: 时间应保持为nil")

	// ========== Case 21-25: 字符串复杂场景 ==========

	// Case 21: 空字符串
	pb21 := &PBSimple{
		StringVal: "",
		IntVal:    1,
	}
	model21 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb21, model21)
	assert.NoError(t, err, "Case 21: 空字符串转换应成功")
	assert.Empty(t, model21.StringVal, "Case 21: 字符串应为空")

	// Case 22: 超长字符串（1MB）
	longString := ""
	for i := 0; i < 1024*1024/10; i++ {
		longString += "0123456789"
	}
	pb22 := &PBSimple{
		StringVal: longString,
		IntVal:    2,
	}
	model22 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb22, model22)
	assert.NoError(t, err, "Case 22: 超长字符串转换应成功")
	assert.Equal(t, longString, model22.StringVal, "Case 22: 超长字符串应完全相等")

	// Case 23: Unicode字符串
	pb23 := &PBSimple{
		StringVal: "你好世界🌍 مرحبا بالعالم 🎉 مرجبا",
		IntVal:    3,
	}
	model23 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb23, model23)
	assert.NoError(t, err, "Case 23: Unicode字符串转换应成功")
	assert.Equal(t, pb23.StringVal, model23.StringVal, "Case 23: Unicode字符应保持")

	// Case 24: 特殊字符字符串（控制字符、空白）
	pb24 := &PBSimple{
		StringVal: "tab\there\nnewline\rcarriage\x00null",
		IntVal:    4,
	}
	model24 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb24, model24)
	assert.NoError(t, err, "Case 24: 特殊字符字符串转换应成功")
	assert.Equal(t, pb24.StringVal, model24.StringVal, "Case 24: 特殊字符应保持")

	// Case 25: SQL注入风格字符串
	pb25 := &PBSimple{
		StringVal: "'; DROP TABLE users; --",
		IntVal:    5,
	}
	model25 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb25, model25)
	assert.NoError(t, err, "Case 25: SQL风格字符串转换应成功")
	assert.Equal(t, pb25.StringVal, model25.StringVal, "Case 25: SQL风格字符串应完全保持")

	// ========== Case 26-30: 字节数组场景 ==========

	// Case 26: 空字节数组
	pb26 := &PBSimple{
		BytesVal: []byte{},
		IntVal:   1,
	}
	model26 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb26, model26)
	assert.NoError(t, err, "Case 26: 空字节数组转换应成功")
	assert.Empty(t, model26.BytesVal, "Case 26: 字节数组应为空")

	// Case 27: 单字节数组
	pb27 := &PBSimple{
		BytesVal: []byte{255},
		IntVal:   2,
	}
	model27 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb27, model27)
	assert.NoError(t, err, "Case 27: 单字节数组转换应成功")
	assert.Equal(t, []byte{255}, model27.BytesVal, "Case 27: 单字节应保持")

	// Case 28: 所有字节值（0-255）
	allBytes := make([]byte, 256)
	for i := 0; i < 256; i++ {
		allBytes[i] = byte(i)
	}
	pb28 := &PBSimple{
		BytesVal: allBytes,
		IntVal:   3,
	}
	model28 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb28, model28)
	assert.NoError(t, err, "Case 28: 所有字节值转换应成功")
	assert.Equal(t, allBytes, model28.BytesVal, "Case 28: 所有字节应保持")

	// Case 29: 大字节数组（10MB）
	largeBytes := make([]byte, 10*1024*1024)
	for i := 0; i < len(largeBytes); i++ {
		largeBytes[i] = byte(i % 256)
	}
	pb29 := &PBSimple{
		BytesVal: largeBytes,
		IntVal:   4,
	}
	model29 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb29, model29)
	assert.NoError(t, err, "Case 29: 大字节数组转换应成功")
	assert.Equal(t, len(largeBytes), len(model29.BytesVal), "Case 29: 字节数组长度应保持")
	assert.Equal(t, largeBytes, model29.BytesVal, "Case 29: 大字节数组应完全相等")

	// Case 30: 二进制格式字节
	pb30 := &PBSimple{
		BytesVal: []byte{0x00, 0xFF, 0x01, 0xFE, 0xDE, 0xAD, 0xBE, 0xEF},
		IntVal:   5,
	}
	model30 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb30, model30)
	assert.NoError(t, err, "Case 30: 二进制字节转换应成功")
	assert.Equal(t, pb30.BytesVal, model30.BytesVal, "Case 30: 二进制字节应保持")

	// ========== Case 31-35: 布尔值与多字段组合 ==========

	// Case 31: true值组合
	pb31 := &PBSimple{
		BoolVal:   true,
		IntVal:    100,
		FloatVal:  3.14,
		StringVal: "all_true",
	}
	model31 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb31, model31)
	assert.NoError(t, err, "Case 31: true值组合转换应成功")
	assert.True(t, model31.BoolVal, "Case 31: bool应为true")
	assert.Equal(t, int32(100), model31.IntVal, "Case 31: int应相等")

	// Case 32: false值组合
	pb32 := &PBSimple{
		BoolVal:   false,
		IntVal:    -100,
		FloatVal:  -3.14,
		StringVal: "all_false",
	}
	model32 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb32, model32)
	assert.NoError(t, err, "Case 32: false值组合转换应成功")
	assert.False(t, model32.BoolVal, "Case 32: bool应为false")

	// Case 33: 布尔值与极值组合
	pb33 := &PBSimple{
		BoolVal:   true,
		IntVal:    math.MaxInt32,
		Int64Val:  math.MinInt64,
		UintVal:   math.MaxUint32,
		StringVal: "bool_extremes",
	}
	model33 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb33, model33)
	assert.NoError(t, err, "Case 33: 布尔与极值组合转换应成功")
	assert.True(t, model33.BoolVal, "Case 33: bool应为true")
	assert.Equal(t, int32(math.MaxInt32), model33.IntVal, "Case 33: int32应为最大值")

	// Case 34: 多个布尔标志位转换
	pb34 := &PBSimple{
		BoolVal:   false,
		IntVal:    0,
		FloatVal:  0,
		StringVal: "",
	}
	model34 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb34, model34)
	assert.NoError(t, err, "Case 34: 多零值组合转换应成功")
	assert.False(t, model34.BoolVal, "Case 34: bool应为false")
	assert.Equal(t, int32(0), model34.IntVal, "Case 34: int应为0")

	// Case 35: 布尔值与时间戳组合
	pb35 := &PBSimple{
		BoolVal:   true,
		TimeVal:   timestamppb.Now(),
		StringVal: "bool_timestamp",
	}
	model35 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb35, model35)
	assert.NoError(t, err, "Case 35: 布尔与时间戳组合转换应成功")
	assert.True(t, model35.BoolVal, "Case 35: bool应为true")
	assert.NotNil(t, model35.TimeVal, "Case 35: 时间不应为nil")

	// ========== Case 36-40: 混合类型复杂场景 ==========

	// Case 36: 所有字段非零值
	now := time.Now()
	pb36 := &PBSimple{
		IntVal:    42,
		Int64Val:  9223372036854775800,
		UintVal:   4294967290,
		Uint64Val: 18446744073709551610,
		FloatVal:  3.14159,
		DoubleVal: 2.71828,
		BoolVal:   true,
		StringVal: "all_fields_filled",
		BytesVal:  []byte{1, 2, 3, 4, 5},
		TimeVal:   timestamppb.New(now),
	}
	model36 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb36, model36)
	assert.NoError(t, err, "Case 36: 所有字段转换应成功")
	assert.Equal(t, pb36.IntVal, model36.IntVal, "Case 36: IntVal应相等")
	assert.Equal(t, pb36.StringVal, model36.StringVal, "Case 36: StringVal应相等")
	assert.Equal(t, pb36.BytesVal, model36.BytesVal, "Case 36: BytesVal应相等")

	// Case 37: 反向转换（Model -> PB）
	modelSrc := &ModelSimple{
		IntVal:    99,
		Int64Val:  999,
		UintVal:   999,
		Uint64Val: 9999,
		FloatVal:  9.99,
		DoubleVal: 99.99,
		BoolVal:   true,
		StringVal: "reverse_conversion",
		BytesVal:  []byte{9, 9, 9},
		TimeVal:   timestamppb.Now(),
	}
	pbResult := &PBSimple{}
	err = converter.ConvertModelToPB(modelSrc, pbResult)
	assert.NoError(t, err, "Case 37: 反向转换应成功")
	assert.Equal(t, modelSrc.IntVal, pbResult.IntVal, "Case 37: 反向IntVal应相等")
	assert.Equal(t, modelSrc.StringVal, pbResult.StringVal, "Case 37: 反向StringVal应相等")

	// Case 38: 往返转换（PB -> Model -> PB）
	originalPB := &PBSimple{
		IntVal:    777,
		StringVal: "round_trip",
		FloatVal:  7.77,
		BoolVal:   true,
	}
	tempModel := &ModelSimple{}
	converter.ConvertPBToModel(originalPB, tempModel)
	roundTripPB := &PBSimple{}
	err = converter.ConvertModelToPB(tempModel, roundTripPB)
	assert.NoError(t, err, "Case 38: 往返转换应成功")
	assert.Equal(t, originalPB.IntVal, roundTripPB.IntVal, "Case 38: 往返IntVal应相等")
	assert.Equal(t, originalPB.StringVal, roundTripPB.StringVal, "Case 38: 往返StringVal应相等")

	// Case 39: 部分字段转换（只设置部分字段）
	pb39 := &PBSimple{
		IntVal:    39,
		StringVal: "partial",
	}
	model39 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb39, model39)
	assert.NoError(t, err, "Case 39: 部分字段转换应成功")
	assert.Equal(t, int32(39), model39.IntVal, "Case 39: 设置的字段应相等")
	assert.Equal(t, "partial", model39.StringVal, "Case 39: 设置的StringVal应保持")

	// Case 40: 最小值与最大值交替
	pb40 := &PBSimple{
		IntVal:    math.MinInt32,
		Int64Val:  math.MaxInt64,
		UintVal:   0,
		Uint64Val: math.MaxUint64,
		FloatVal:  float32(math.Inf(-1)),
		DoubleVal: math.Inf(1),
		BoolVal:   false,
		StringVal: "min_max_alt",
	}
	model40 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb40, model40)
	assert.NoError(t, err, "Case 40: 最小最大值交替转换应成功")
	assert.Equal(t, int32(math.MinInt32), model40.IntVal, "Case 40: min int32应保持")
	assert.Equal(t, int64(math.MaxInt64), model40.Int64Val, "Case 40: max int64应保持")
	assert.True(t, math.IsInf(float64(model40.FloatVal), -1), "Case 40: 负无穷应保持")

	// ========== Case 41-45: 类型边界与溢出场景 ==========

	// Case 41: uint32边界值
	pb41 := &PBSimple{
		UintVal:   math.MaxUint32,
		StringVal: "uint32_boundary",
	}
	model41 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb41, model41)
	assert.NoError(t, err, "Case 41: uint32边界转换应成功")
	assert.Equal(t, uint32(math.MaxUint32), model41.UintVal, "Case 41: uint32应保持最大值")

	// Case 42: int64边界值
	pb42 := &PBSimple{
		Int64Val:  math.MinInt64,
		StringVal: "int64_min_boundary",
	}
	model42 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb42, model42)
	assert.NoError(t, err, "Case 42: int64最小边界转换应成功")
	assert.Equal(t, int64(math.MinInt64), model42.Int64Val, "Case 42: int64应保持最小值")

	// Case 43: float32边界
	pb43 := &PBSimple{
		FloatVal:  float32(3.40282e38),
		StringVal: "float32_boundary",
	}
	model43 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb43, model43)
	assert.NoError(t, err, "Case 43: float32边界转换应成功")
	assert.Greater(t, model43.FloatVal, float32(0), "Case 43: float32应为正数")

	// Case 44: float64边界
	pb44 := &PBSimple{
		DoubleVal: 1.79769e308,
		StringVal: "float64_boundary",
	}
	model44 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb44, model44)
	assert.NoError(t, err, "Case 44: float64边界转换应成功")
	assert.Greater(t, model44.DoubleVal, 0.0, "Case 44: float64应为正数")

	// Case 45: 所有边界值组合
	pb45 := &PBSimple{
		IntVal:    math.MaxInt32,
		Int64Val:  math.MinInt64,
		UintVal:   math.MaxUint32,
		Uint64Val: math.MaxUint64,
		FloatVal:  float32(math.Inf(1)),
		DoubleVal: math.Inf(-1),
		StringVal: "all_boundaries",
	}
	model45 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb45, model45)
	assert.NoError(t, err, "Case 45: 所有边界值转换应成功")
	assert.Equal(t, int32(math.MaxInt32), model45.IntVal, "Case 45: max int32应保持")
	assert.Equal(t, int64(math.MinInt64), model45.Int64Val, "Case 45: min int64应保持")

	// ========== Case 46-50: 综合压力测试 ==========

	// Case 46: 随机组合1
	pb46 := &PBSimple{
		IntVal:    46,
		Int64Val:  4646,
		UintVal:   46,
		Uint64Val: 464646,
		FloatVal:  4.6,
		DoubleVal: 46.46,
		BoolVal:   false,
		StringVal: "case_46_random",
		BytesVal:  []byte{46},
	}
	model46 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb46, model46)
	assert.NoError(t, err, "Case 46: 随机组合1转换应成功")

	// Case 47: 随机组合2
	pb47 := &PBSimple{
		IntVal:    -47,
		Int64Val:  -4747,
		FloatVal:  -4.7,
		DoubleVal: -47.47,
		BoolVal:   true,
		StringVal: "case_47_random",
	}
	model47 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb47, model47)
	assert.NoError(t, err, "Case 47: 随机组合2转换应成功")

	// Case 48: 随机组合3
	pb48 := &PBSimple{
		UintVal:   48,
		Uint64Val: 484848,
		FloatVal:  0.48,
		StringVal: "case_48_random",
		TimeVal:   timestamppb.Now(),
	}
	model48 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb48, model48)
	assert.NoError(t, err, "Case 48: 随机组合3转换应成功")

	// Case 49: 随机组合4
	pb49 := &PBSimple{
		IntVal:    49,
		FloatVal:  49.49,
		DoubleVal: 4949.49,
		BoolVal:   true,
		BytesVal:  []byte{49, 49, 49},
		StringVal: "case_49_random",
	}
	model49 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb49, model49)
	assert.NoError(t, err, "Case 49: 随机组合4转换应成功")

	// Case 50: 随机组合5（接近满值）
	pb50 := &PBSimple{
		IntVal:    50,
		Int64Val:  5000,
		UintVal:   500,
		Uint64Val: 50000,
		FloatVal:  5.0,
		DoubleVal: 50.0,
		BoolVal:   false,
		StringVal: "case_50_final",
		BytesVal:  []byte{50, 0, 50},
		TimeVal:   timestamppb.Now(),
	}
	model50 := &ModelSimple{}
	err = converter.ConvertPBToModel(pb50, model50)
	assert.NoError(t, err, "Case 50: 随机组合5转换应成功")
	assert.Equal(t, int32(50), model50.IntVal, "Case 50: IntVal应相等")
	assert.Equal(t, "case_50_final", model50.StringVal, "Case 50: StringVal应相等")
}
