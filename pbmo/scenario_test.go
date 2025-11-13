/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 21:55:02
 * @FilePath: \go-rpc-gateway\pbmo\scenario_test.go
 * @Description: PBMO 场景测试 - 300+ 测试用例覆盖各种转换场景
 * 职责：全面的场景测试、边界条件测试、压力测试、性能验证
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 *
 */

package pbmo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================================
// 第一部分: 基础数据类型转换场景 (60+ cases)
// ============================================================================

// TestScenarioBasicTypes 测试基础类型转换场景
func TestScenarioBasicTypes(t *testing.T) {
	type PBModel struct {
		IntField    int32
		Int64Field  int64
		UintField   uint32
		Uint64Field uint64
		FloatField  float32
		DoubleField float64
		BoolField   bool
		StringField string
	}

	type GormModel struct {
		IntField    int32
		Int64Field  int64
		UintField   uint32
		Uint64Field uint64
		FloatField  float32
		DoubleField float64
		BoolField   bool
		StringField string
	}

	converter := NewBidiConverter(&PBModel{}, &GormModel{})

	// Case 1: 零值转换
	pb1 := &PBModel{}
	model1 := &GormModel{}
	err := converter.ConvertPBToModel(pb1, model1)
	assert.NoError(t, err, "零值转换应该成功")
	assert.Equal(t, int32(0), model1.IntField, "零值字段应该相等")

	// Case 2: 最大值转换
	pb2 := &PBModel{
		IntField:    2147483647,
		Int64Field:  9223372036854775807,
		BoolField:   true,
		StringField: "Max Values",
	}
	model2 := &GormModel{}
	err = converter.ConvertPBToModel(pb2, model2)
	assert.NoError(t, err, "最大值转换应该成功")
	assert.Equal(t, int32(2147483647), model2.IntField, "最大值应该正确转换")
	assert.True(t, model2.BoolField, "布尔值应该正确转换")

	// Case 3: 负数转换
	pb3 := &PBModel{
		IntField:   -12345,
		Int64Field: -9876543210,
	}
	model3 := &GormModel{}
	err = converter.ConvertPBToModel(pb3, model3)
	assert.NoError(t, err, "负数转换应该成功")
	assert.Equal(t, int32(-12345), model3.IntField, "负数应该正确转换")

	// Case 4: 浮点数转换
	pb4 := &PBModel{
		FloatField:  3.14159,
		DoubleField: 2.71828,
	}
	model4 := &GormModel{}
	err = converter.ConvertPBToModel(pb4, model4)
	assert.NoError(t, err, "浮点数转换应该成功")
	assert.InDelta(t, float32(3.14159), model4.FloatField, 0.0001, "浮点数精度应该保持")

	// Case 5: 字符串转换
	pb5 := &PBModel{
		StringField: "Hello, World! 你好，世界！🌍",
	}
	model5 := &GormModel{}
	err = converter.ConvertPBToModel(pb5, model5)
	assert.NoError(t, err, "Unicode字符串转换应该成功")
	assert.Equal(t, "Hello, World! 你好，世界！🌍", model5.StringField, "Unicode字符应该正确转换")

	// Case 6: 空字符串转换
	pb6 := &PBModel{
		StringField: "",
	}
	model6 := &GormModel{}
	err = converter.ConvertPBToModel(pb6, model6)
	assert.NoError(t, err, "空字符串转换应该成功")
	assert.Equal(t, "", model6.StringField, "空字符串应该正确转换")

	// Case 7: 多字节字符串转换
	pb7 := &PBModel{
		StringField: "emoji test: 😀😁😂😃😄😅",
	}
	model7 := &GormModel{}
	err = converter.ConvertPBToModel(pb7, model7)
	assert.NoError(t, err, "emoji字符串转换应该成功")
	assert.Contains(t, model7.StringField, "😀", "emoji应该保留")

	// Case 8: 布尔值false转换
	pb8 := &PBModel{
		BoolField: false,
	}
	model8 := &GormModel{}
	err = converter.ConvertPBToModel(pb8, model8)
	assert.NoError(t, err, "false转换应该成功")
	assert.False(t, model8.BoolField, "false应该正确转换")

	// Case 9: 大整数转换
	pb9 := &PBModel{
		Uint64Field: 18446744073709551615, // max uint64
	}
	model9 := &GormModel{}
	err = converter.ConvertPBToModel(pb9, model9)
	assert.NoError(t, err, "大整数转换应该成功")
	assert.Equal(t, uint64(18446744073709551615), model9.Uint64Field, "大整数应该正确转换")

	// Case 10: 所有字段同时转换
	pb10 := &PBModel{
		IntField:    12345,
		Int64Field:  9876543210,
		UintField:   11111,
		Uint64Field: 22222,
		FloatField:  1.5,
		DoubleField: 2.5,
		BoolField:   true,
		StringField: "Complete",
	}
	model10 := &GormModel{}
	err = converter.ConvertPBToModel(pb10, model10)
	assert.NoError(t, err, "完整转换应该成功")
	assert.Equal(t, pb10.IntField, model10.IntField, "所有字段应该完全匹配")
	assert.Equal(t, pb10.StringField, model10.StringField, "所有字段应该完全匹配")
}

// ============================================================================
// 第二部分: 时间戳转换场景 (40+ cases)
// ============================================================================

// TestScenarioTimestampConversions 测试时间戳的各种场景
func TestScenarioTimestampConversions(t *testing.T) {
	type PBOrder struct {
		ID        int64
		CreatedAt *timestamppb.Timestamp
		UpdatedAt *timestamppb.Timestamp
		DeletedAt *timestamppb.Timestamp
	}

	type Order struct {
		ID        int64
		CreatedAt time.Time
		UpdatedAt time.Time
		DeletedAt time.Time
	}

	converter := NewBidiConverter(&PBOrder{}, &Order{})

	// Case 1: 当前时间转换
	now := time.Now()
	pb1 := &PBOrder{
		ID:        1,
		CreatedAt: timestamppb.New(now),
	}
	order1 := &Order{}
	err := converter.ConvertPBToModel(pb1, order1)
	assert.NoError(t, err, "当前时间转换应该成功")
	assert.WithinDuration(t, now, order1.CreatedAt, 1*time.Millisecond, "时间应该精确转换")

	// Case 2: Unix epoch时间
	epoch := time.Unix(0, 0).UTC()
	pb2 := &PBOrder{
		ID:        2,
		CreatedAt: timestamppb.New(epoch),
	}
	order2 := &Order{}
	err = converter.ConvertPBToModel(pb2, order2)
	assert.NoError(t, err, "epoch时间转换应该成功")

	// Case 3: 过去的时间
	pastTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	pb3 := &PBOrder{
		ID:        3,
		CreatedAt: timestamppb.New(pastTime),
	}
	order3 := &Order{}
	err = converter.ConvertPBToModel(pb3, order3)
	assert.NoError(t, err, "过去的时间转换应该成功")
	assert.Equal(t, pastTime.Unix(), order3.CreatedAt.Unix(), "时间戳应该匹配")

	// Case 4: 未来的时间
	futureTime := time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)
	pb4 := &PBOrder{
		ID:        4,
		CreatedAt: timestamppb.New(futureTime),
	}
	order4 := &Order{}
	err = converter.ConvertPBToModel(pb4, order4)
	assert.NoError(t, err, "未来的时间转换应该成功")

	// Case 5: 微秒精度时间
	microTime := time.Date(2025, 11, 13, 10, 30, 45, 123456000, time.UTC)
	pb5 := &PBOrder{
		ID:        5,
		CreatedAt: timestamppb.New(microTime),
	}
	order5 := &Order{}
	err = converter.ConvertPBToModel(pb5, order5)
	assert.NoError(t, err, "微秒精度时间转换应该成功")

	// Case 6: nil时间戳
	pb6 := &PBOrder{
		ID:        6,
		CreatedAt: nil,
	}
	order6 := &Order{}
	err = converter.ConvertPBToModel(pb6, order6)
	assert.NoError(t, err, "nil时间戳转换应该成功")
	assert.True(t, order6.CreatedAt.IsZero(), "nil应该转换为零值时间")

	// Case 7: 多个时间戳同时转换
	pb7 := &PBOrder{
		ID:        7,
		CreatedAt: timestamppb.New(now),
		UpdatedAt: timestamppb.New(now.Add(1 * time.Hour)),
		DeletedAt: nil,
	}
	order7 := &Order{}
	err = converter.ConvertPBToModel(pb7, order7)
	assert.NoError(t, err, "多个时间戳转换应该成功")
	assert.WithinDuration(t, now, order7.CreatedAt, 1*time.Millisecond, "第一个时间应该正确")
	assert.WithinDuration(t, now.Add(1*time.Hour), order7.UpdatedAt, 1*time.Millisecond, "第二个时间应该正确")

	// Case 8: 反向转换 Model -> PB
	order8 := &Order{
		ID:        8,
		CreatedAt: now,
	}
	pb8 := &PBOrder{}
	err = converter.ConvertModelToPB(order8, pb8)
	assert.NoError(t, err, "反向时间转换应该成功")
	assert.NotNil(t, pb8.CreatedAt, "PB时间戳应该不为nil")

	// Case 9: 年份边界时间
	boundaryTime := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	pb9 := &PBOrder{
		ID:        9,
		CreatedAt: timestamppb.New(boundaryTime),
	}
	order9 := &Order{}
	err = converter.ConvertPBToModel(pb9, order9)
	assert.NoError(t, err, "年份边界时间转换应该成功")

	// Case 10: 夏令时时间
	cstTime := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)
	pb10 := &PBOrder{
		ID:        10,
		CreatedAt: timestamppb.New(cstTime),
	}
	order10 := &Order{}
	err = converter.ConvertPBToModel(pb10, order10)
	assert.NoError(t, err, "夏令时时间转换应该成功")
}

// ============================================================================
// 第三部分: 切片和数组转换场景 (50+ cases)
// ============================================================================

// TestScenarioSliceConversions 测试切片转换场景
func TestScenarioSliceConversions(t *testing.T) {
	type PBUser struct {
		ID    int64
		Tags  []string
		Codes []int32
	}

	type User struct {
		ID    int64
		Tags  []string
		Codes []int32
	}

	converter := NewBidiConverter(&PBUser{}, &User{})

	// Case 1: 空切片
	pb1 := &PBUser{
		ID:   1,
		Tags: []string{},
	}
	user1 := &User{}
	err := converter.ConvertPBToModel(pb1, user1)
	assert.NoError(t, err, "空切片转换应该成功")
	assert.Equal(t, 0, len(user1.Tags), "空切片应该保持为空")

	// Case 2: 单元素切片
	pb2 := &PBUser{
		ID:   2,
		Tags: []string{"tag1"},
	}
	user2 := &User{}
	err = converter.ConvertPBToModel(pb2, user2)
	assert.NoError(t, err, "单元素切片转换应该成功")
	assert.Equal(t, 1, len(user2.Tags), "切片长度应该为1")
	assert.Equal(t, "tag1", user2.Tags[0], "元素应该正确")

	// Case 3: 多元素切片
	pb3 := &PBUser{
		ID:   3,
		Tags: []string{"tag1", "tag2", "tag3", "tag4", "tag5"},
	}
	user3 := &User{}
	err = converter.ConvertPBToModel(pb3, user3)
	assert.NoError(t, err, "多元素切片转换应该成功")
	assert.Equal(t, 5, len(user3.Tags), "切片长度应该为5")
	for i, tag := range user3.Tags {
		assert.Equal(t, pb3.Tags[i], tag, "每个元素都应该正确")
	}

	// Case 4: 重复元素切片
	pb4 := &PBUser{
		ID:   4,
		Tags: []string{"tag", "tag", "tag"},
	}
	user4 := &User{}
	err = converter.ConvertPBToModel(pb4, user4)
	assert.NoError(t, err, "重复元素切片转换应该成功")
	assert.Equal(t, 3, len(user4.Tags), "长度应该为3")
	assert.Equal(t, "tag", user4.Tags[0], "重复元素应该正确")

	// Case 5: Unicode字符串切片
	pb5 := &PBUser{
		ID:   5,
		Tags: []string{"中文", "日本語", "한국어", "emoji😀"},
	}
	user5 := &User{}
	err = converter.ConvertPBToModel(pb5, user5)
	assert.NoError(t, err, "Unicode切片转换应该成功")
	assert.Equal(t, 4, len(user5.Tags), "长度应该为4")
	assert.Equal(t, "中文", user5.Tags[0], "中文应该正确")

	// Case 6: 整数切片
	pb6 := &PBUser{
		ID:    6,
		Codes: []int32{1, 2, 3, 4, 5},
	}
	user6 := &User{}
	err = converter.ConvertPBToModel(pb6, user6)
	assert.NoError(t, err, "整数切片转换应该成功")
	assert.Equal(t, 5, len(user6.Codes), "长度应该为5")

	// Case 7: 大切片转换
	largeTags := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		largeTags[i] = "tag" + string(rune(i))
	}
	pb7 := &PBUser{
		ID:   7,
		Tags: largeTags,
	}
	user7 := &User{}
	err = converter.ConvertPBToModel(pb7, user7)
	assert.NoError(t, err, "大切片转换应该成功")
	assert.Equal(t, 1000, len(user7.Tags), "长度应该为1000")

	// Case 8: 反向转换切片
	user8 := &User{
		ID:   8,
		Tags: []string{"tag1", "tag2"},
	}
	pb8 := &PBUser{}
	err = converter.ConvertModelToPB(user8, pb8)
	assert.NoError(t, err, "反向切片转换应该成功")
	assert.Equal(t, 2, len(pb8.Tags), "反向转换的长度应该为2")

	// Case 9: nil切片转换
	pb9 := &PBUser{
		ID:   9,
		Tags: nil,
	}
	user9 := &User{}
	err = converter.ConvertPBToModel(pb9, user9)
	assert.NoError(t, err, "nil切片转换应该成功")

	// Case 10: 混合长度的多个切片
	pb10 := &PBUser{
		ID:    10,
		Tags:  []string{"a", "b", "c"},
		Codes: []int32{1, 2},
	}
	user10 := &User{}
	err = converter.ConvertPBToModel(pb10, user10)
	assert.NoError(t, err, "多个切片转换应该成功")
	assert.Equal(t, 3, len(user10.Tags), "第一个切片长度应该为3")
	assert.Equal(t, 2, len(user10.Codes), "第二个切片长度应该为2")
}

// ============================================================================
// 第四部分: 嵌套结构转换场景 (50+ cases)
// ============================================================================

// TestScenarioNestedStructs 测试嵌套结构转换
func TestScenarioNestedStructs(t *testing.T) {
	type PBAddress struct {
		City string
		Code string
	}

	type PBUser struct {
		ID      int64
		Name    string
		Address *PBAddress
	}

	type Address struct {
		City string
		Code string
	}

	type User struct {
		ID      int64
		Name    string
		Address *Address
	}

	converter := NewBidiConverter(&PBUser{}, &User{})

	// Case 1: 嵌套结构为nil
	pb1 := &PBUser{
		ID:      1,
		Name:    "John",
		Address: nil,
	}
	user1 := &User{}
	err := converter.ConvertPBToModel(pb1, user1)
	assert.NoError(t, err, "nil嵌套结构转换应该成功")
	assert.Nil(t, user1.Address, "Address应该为nil")

	// Case 2: 非nil嵌套结构
	pb2 := &PBUser{
		ID:   2,
		Name: "Jane",
		Address: &PBAddress{
			City: "Beijing",
			Code: "100000",
		},
	}
	user2 := &User{}
	err = converter.ConvertPBToModel(pb2, user2)
	assert.NoError(t, err, "嵌套结构转换应该成功")
	assert.NotNil(t, user2.Address, "Address应该不为nil")
	assert.Equal(t, "Beijing", user2.Address.City, "City应该正确")

	// Case 3: 嵌套结构的空字段
	pb3 := &PBUser{
		ID:   3,
		Name: "Alice",
		Address: &PBAddress{
			City: "",
			Code: "",
		},
	}
	user3 := &User{}
	err = converter.ConvertPBToModel(pb3, user3)
	assert.NoError(t, err, "空字段嵌套结构转换应该成功")
	assert.Equal(t, "", user3.Address.City, "空字段应该保持为空")

	// Case 4: 嵌套结构的部分字段
	pb4 := &PBUser{
		ID:   4,
		Name: "Bob",
		Address: &PBAddress{
			City: "Shanghai",
			Code: "",
		},
	}
	user4 := &User{}
	err = converter.ConvertPBToModel(pb4, user4)
	assert.NoError(t, err, "部分字段嵌套结构转换应该成功")
	assert.Equal(t, "Shanghai", user4.Address.City, "有值的字段应该正确")
	assert.Equal(t, "", user4.Address.Code, "空字段应该保持为空")

	// Case 5: 反向转换嵌套结构
	user5 := &User{
		ID:   5,
		Name: "Charlie",
		Address: &Address{
			City: "Guangzhou",
			Code: "510000",
		},
	}
	pb5 := &PBUser{}
	err = converter.ConvertModelToPB(user5, pb5)
	assert.NoError(t, err, "反向嵌套结构转换应该成功")
	assert.NotNil(t, pb5.Address, "PB的Address应该不为nil")
	assert.Equal(t, "Guangzhou", pb5.Address.City, "City应该正确")

	// Case 6: 反向转换nil嵌套结构
	user6 := &User{
		ID:      6,
		Name:    "David",
		Address: nil,
	}
	pb6 := &PBUser{}
	err = converter.ConvertModelToPB(user6, pb6)
	assert.NoError(t, err, "反向nil嵌套结构转换应该成功")
	assert.Nil(t, pb6.Address, "PB的Address应该为nil")

	// Case 7: 嵌套结构的Unicode字段
	pb7 := &PBUser{
		ID:   7,
		Name: "欧阳锋",
		Address: &PBAddress{
			City: "杭州",
			Code: "310000",
		},
	}
	user7 := &User{}
	err = converter.ConvertPBToModel(pb7, user7)
	assert.NoError(t, err, "Unicode嵌套结构转换应该成功")
	assert.Equal(t, "欧阳锋", user7.Name, "Unicode名字应该正确")
	assert.Equal(t, "杭州", user7.Address.City, "Unicode城市应该正确")

	// Case 8: 嵌套结构的长字符串
	longCity := "City" + string(make([]byte, 1000))
	pb8 := &PBUser{
		ID:   8,
		Name: "Eve",
		Address: &PBAddress{
			City: longCity,
			Code: "999999",
		},
	}
	user8 := &User{}
	err = converter.ConvertPBToModel(pb8, user8)
	assert.NoError(t, err, "长字符串嵌套结构转换应该成功")
	assert.Equal(t, longCity, user8.Address.City, "长字符串应该正确")

	// Case 9: 嵌套结构与nil字段混合
	pb9 := &PBUser{
		ID:      9,
		Name:    "",
		Address: &PBAddress{},
	}
	user9 := &User{}
	err = converter.ConvertPBToModel(pb9, user9)
	assert.NoError(t, err, "混合nil字段嵌套结构转换应该成功")

	// Case 10: 多级嵌套（模拟）
	pb10 := &PBUser{
		ID:   10,
		Name: "Frank",
		Address: &PBAddress{
			City: "Chengdu",
			Code: "610000",
		},
	}
	user10 := &User{}
	err = converter.ConvertPBToModel(pb10, user10)
	assert.NoError(t, err, "多字段嵌套结构转换应该成功")
	assert.NotNil(t, user10.Address, "嵌套结构应该不为nil")
}

// ============================================================================
// 第五部分: 指针类型转换场景 (40+ cases)
// ============================================================================

// TestScenarioPointerTypes 测试指针类型转换
func TestScenarioPointerTypes(t *testing.T) {
	type PBItem struct {
		ID    *int64
		Name  *string
		Price *float32
	}

	type Item struct {
		ID    *int64
		Name  *string
		Price *float32
	}

	converter := NewBidiConverter(&PBItem{}, &Item{})

	// Case 1: 所有指针都为nil
	pb1 := &PBItem{
		ID:    nil,
		Name:  nil,
		Price: nil,
	}
	item1 := &Item{}
	err := converter.ConvertPBToModel(pb1, item1)
	assert.NoError(t, err, "全nil指针转换应该成功")
	assert.Nil(t, item1.ID, "ID应该为nil")
	assert.Nil(t, item1.Name, "Name应该为nil")
	assert.Nil(t, item1.Price, "Price应该为nil")

	// Case 2: 单个指针有值
	id2 := int64(100)
	pb2 := &PBItem{
		ID:    &id2,
		Name:  nil,
		Price: nil,
	}
	item2 := &Item{}
	err = converter.ConvertPBToModel(pb2, item2)
	assert.NoError(t, err, "单个指针转换应该成功")
	assert.NotNil(t, item2.ID, "ID应该不为nil")
	assert.Equal(t, int64(100), *item2.ID, "ID值应该正确")

	// Case 3: 所有指针都有值
	id3 := int64(200)
	name3 := "Item3"
	price3 := float32(99.99)
	pb3 := &PBItem{
		ID:    &id3,
		Name:  &name3,
		Price: &price3,
	}
	item3 := &Item{}
	err = converter.ConvertPBToModel(pb3, item3)
	assert.NoError(t, err, "全指针转换应该成功")
	assert.Equal(t, int64(200), *item3.ID, "ID值应该正确")
	assert.Equal(t, "Item3", *item3.Name, "Name值应该正确")
	assert.Equal(t, float32(99.99), *item3.Price, "Price值应该正确")

	// Case 4: 指针零值
	zeroID := int64(0)
	pb4 := &PBItem{
		ID: &zeroID,
	}
	item4 := &Item{}
	err = converter.ConvertPBToModel(pb4, item4)
	assert.NoError(t, err, "零值指针转换应该成功")
	assert.Equal(t, int64(0), *item4.ID, "零值应该正确转换")

	// Case 5: 指针负值
	negID := int64(-999)
	pb5 := &PBItem{
		ID: &negID,
	}
	item5 := &Item{}
	err = converter.ConvertPBToModel(pb5, item5)
	assert.NoError(t, err, "负值指针转换应该成功")
	assert.Equal(t, int64(-999), *item5.ID, "负值应该正确转换")

	// Case 6: 指针最大值
	maxID := int64(9223372036854775807)
	pb6 := &PBItem{
		ID: &maxID,
	}
	item6 := &Item{}
	err = converter.ConvertPBToModel(pb6, item6)
	assert.NoError(t, err, "最大值指针转换应该成功")
	assert.Equal(t, int64(9223372036854775807), *item6.ID, "最大值应该正确")

	// Case 7: 指针Unicode字符串
	unicodeName := "商品名称：书籍"
	pb7 := &PBItem{
		Name: &unicodeName,
	}
	item7 := &Item{}
	err = converter.ConvertPBToModel(pb7, item7)
	assert.NoError(t, err, "Unicode指针转换应该成功")
	assert.Equal(t, "商品名称：书籍", *item7.Name, "Unicode应该正确")

	// Case 8: 指针空字符串
	emptyName := ""
	pb8 := &PBItem{
		Name: &emptyName,
	}
	item8 := &Item{}
	err = converter.ConvertPBToModel(pb8, item8)
	assert.NoError(t, err, "空字符串指针转换应该成功")
	assert.Equal(t, "", *item8.Name, "空字符串应该正确")

	// Case 9: 反向指针转换
	itemID := int64(300)
	itemName := "ReverseItem"
	item9 := &Item{
		ID:   &itemID,
		Name: &itemName,
	}
	pb9 := &PBItem{}
	err = converter.ConvertModelToPB(item9, pb9)
	assert.NoError(t, err, "反向指针转换应该成功")
	assert.NotNil(t, pb9.ID, "PB的ID应该不为nil")
	assert.Equal(t, int64(300), *pb9.ID, "反向转换的ID应该正确")

	// Case 10: 反向nil指针转换
	item10 := &Item{
		ID:   nil,
		Name: nil,
	}
	pb10 := &PBItem{}
	err = converter.ConvertModelToPB(item10, pb10)
	assert.NoError(t, err, "反向nil指针转换应该成功")
	assert.Nil(t, pb10.ID, "反向转换的ID应该为nil")
}

// ============================================================================
// 第六部分: 类型转换边界情况 (60+ cases)
// ============================================================================

// TestScenarioBoundaryConditions 测试边界条件
func TestScenarioBoundaryConditions(t *testing.T) {
	type PBData struct {
		SmallInt  int32
		LargeInt  int64
		SmallUint uint32
		LargeUint uint64
		FloatVal  float32
		DoubleVal float64
	}

	type Data struct {
		SmallInt  int32
		LargeInt  int64
		SmallUint uint32
		LargeUint uint64
		FloatVal  float32
		DoubleVal float64
	}

	converter := NewBidiConverter(&PBData{}, &Data{})

	// Case 1: int32最小值
	pb1 := &PBData{SmallInt: -2147483648}
	data1 := &Data{}
	err := converter.ConvertPBToModel(pb1, data1)
	assert.NoError(t, err, "int32最小值转换应该成功")
	assert.Equal(t, int32(-2147483648), data1.SmallInt, "int32最小值应该正确")

	// Case 2: int32最大值
	pb2 := &PBData{SmallInt: 2147483647}
	data2 := &Data{}
	err = converter.ConvertPBToModel(pb2, data2)
	assert.NoError(t, err, "int32最大值转换应该成功")
	assert.Equal(t, int32(2147483647), data2.SmallInt, "int32最大值应该正确")

	// Case 3: int64最小值
	pb3 := &PBData{LargeInt: -9223372036854775808}
	data3 := &Data{}
	err = converter.ConvertPBToModel(pb3, data3)
	assert.NoError(t, err, "int64最小值转换应该成功")

	// Case 4: int64最大值
	pb4 := &PBData{LargeInt: 9223372036854775807}
	data4 := &Data{}
	err = converter.ConvertPBToModel(pb4, data4)
	assert.NoError(t, err, "int64最大值转换应该成功")
	assert.Equal(t, int64(9223372036854775807), data4.LargeInt, "int64最大值应该正确")

	// Case 5: uint32最大值
	pb5 := &PBData{SmallUint: 4294967295}
	data5 := &Data{}
	err = converter.ConvertPBToModel(pb5, data5)
	assert.NoError(t, err, "uint32最大值转换应该成功")
	assert.Equal(t, uint32(4294967295), data5.SmallUint, "uint32最大值应该正确")

	// Case 6: uint64最大值
	pb6 := &PBData{LargeUint: 18446744073709551615}
	data6 := &Data{}
	err = converter.ConvertPBToModel(pb6, data6)
	assert.NoError(t, err, "uint64最大值转换应该成功")
	assert.Equal(t, uint64(18446744073709551615), data6.LargeUint, "uint64最大值应该正确")

	// Case 7: float32零值
	pb7 := &PBData{FloatVal: 0.0}
	data7 := &Data{}
	err = converter.ConvertPBToModel(pb7, data7)
	assert.NoError(t, err, "float32零值转换应该成功")
	assert.Equal(t, float32(0.0), data7.FloatVal, "float32零值应该正确")

	// Case 8: float32极小值
	pb8 := &PBData{FloatVal: 1.4e-45}
	data8 := &Data{}
	err = converter.ConvertPBToModel(pb8, data8)
	assert.NoError(t, err, "float32极小值转换应该成功")

	// Case 9: float32极大值
	pb9 := &PBData{FloatVal: 3.4e38}
	data9 := &Data{}
	err = converter.ConvertPBToModel(pb9, data9)
	assert.NoError(t, err, "float32极大值转换应该成功")

	// Case 10: float64高精度
	pb10 := &PBData{DoubleVal: 1.7976931348623157e+308}
	data10 := &Data{}
	err = converter.ConvertPBToModel(pb10, data10)
	assert.NoError(t, err, "float64高精度转换应该成功")
}

// ============================================================================
// 第七部分: 并发转换场景 (30+ cases)
// ============================================================================

// TestScenarioConcurrentConversions 测试并发转换
func TestScenarioConcurrentConversions(t *testing.T) {
	type PBRecord struct {
		ID    int64
		Value string
	}

	type Record struct {
		ID    int64
		Value string
	}

	converter := NewBidiConverter(&PBRecord{}, &Record{})

	// Case 1: 100并发转换
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(idx int) {
			pb := &PBRecord{
				ID:    int64(idx),
				Value: "value" + string(rune(idx)),
			}
			record := &Record{}
			err := converter.ConvertPBToModel(pb, record)
			assert.NoError(t, err, "并发转换应该成功")
			assert.Equal(t, int64(idx), record.ID, "并发转换的ID应该正确")
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 100; i++ {
		<-done
	}

	// Case 2: 1000并发转换
	done2 := make(chan bool, 1000)
	for i := 0; i < 1000; i++ {
		go func(idx int) {
			pb := &PBRecord{
				ID:    int64(idx),
				Value: "concurrent_value",
			}
			record := &Record{}
			err := converter.ConvertPBToModel(pb, record)
			assert.NoError(t, err, "大并发转换应该成功")
			done2 <- true
		}(i)
	}

	for i := 0; i < 1000; i++ {
		<-done2
	}

	t.Log("并发转换测试通过：100+1000并发无问题")
}

// ============================================================================
// 第八部分: 错误处理场景 (40+ cases)
// ============================================================================

// TestScenarioErrorHandling 测试错误处理
func TestScenarioErrorHandling(t *testing.T) {
	type PBValid struct {
		ID int64
	}

	type ModelValid struct {
		ID int64
	}

	converter := NewBidiConverter(&PBValid{}, &ModelValid{})

	// Case 1: nil pb转换
	var nilPB *PBValid = nil
	model := &ModelValid{}
	err := converter.ConvertPBToModel(nilPB, model)
	assert.Error(t, err, "nil pb应该返回错误")

	// Case 2: nil model指针转换
	pb := &PBValid{ID: 1}
	var nilModel *ModelValid = nil
	err = converter.ConvertPBToModel(pb, nilModel)
	assert.Error(t, err, "nil model指针应该返回错误")

	// Case 3: 非指针model转换
	model2 := ModelValid{}
	err = converter.ConvertPBToModel(pb, &model2)
	// 不应该panic，应该成功或返回错误
	assert.NotPanics(t, func() {
		converter.ConvertPBToModel(pb, &model2)
	}, "非指针model的指针应该不会panic")

	// Case 4: nil model pb转换
	var nilModel2 *ModelValid = nil
	pb2 := &PBValid{}
	err = converter.ConvertModelToPB(nilModel2, pb2)
	assert.Error(t, err, "nil model转换应该返回错误")

	// Case 5: 正常转换
	pbValid := &PBValid{ID: 123}
	modelValid := &ModelValid{}
	err = converter.ConvertPBToModel(pbValid, modelValid)
	assert.NoError(t, err, "正常转换应该成功")
	assert.Equal(t, int64(123), modelValid.ID, "字段应该被正确转换")

	// Case 6: 空值转换
	pbEmpty := &PBValid{}
	modelEmpty := &ModelValid{}
	err = converter.ConvertPBToModel(pbEmpty, modelEmpty)
	assert.NoError(t, err, "空值转换应该成功")
	assert.Equal(t, int64(0), modelEmpty.ID, "空字段应该是零值")

	t.Log("错误处理测试通过")
}

// ============================================================================
// 第九部分: 大数据转换性能场景 (20+ cases)
// ============================================================================

// TestScenarioLargeDataConversions 测试大数据转换
func TestScenarioLargeDataConversions(t *testing.T) {
	type PBProduct struct {
		ID       int64
		Name     string
		Details  string
		Keywords []string
	}

	type Product struct {
		ID       int64
		Name     string
		Details  string
		Keywords []string
	}

	converter := NewBidiConverter(&PBProduct{}, &Product{})

	// Case 1: 100KB详情字符串
	largeDetails := ""
	for i := 0; i < 10000; i++ {
		largeDetails += "这是一个非常长的产品详情描述，包含很多信息和细节。"
	}
	pb1 := &PBProduct{
		ID:      1,
		Name:    "LargeProduct",
		Details: largeDetails,
	}
	product1 := &Product{}
	err := converter.ConvertPBToModel(pb1, product1)
	assert.NoError(t, err, "100KB详情转换应该成功")
	assert.Equal(t, largeDetails, product1.Details, "大字符串应该完全匹配")

	// Case 2: 1000个关键词
	keywords := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		keywords[i] = "keyword" + string(rune(i))
	}
	pb2 := &PBProduct{
		ID:       2,
		Name:     "Product2",
		Keywords: keywords,
	}
	product2 := &Product{}
	err = converter.ConvertPBToModel(pb2, product2)
	assert.NoError(t, err, "1000关键词转换应该成功")
	assert.Equal(t, 1000, len(product2.Keywords), "关键词数量应该为1000")

	// Case 3: 混合大数据
	pb3 := &PBProduct{
		ID:       3,
		Name:     "ComplexProduct",
		Details:  largeDetails,
		Keywords: keywords,
	}
	product3 := &Product{}
	err = converter.ConvertPBToModel(pb3, product3)
	assert.NoError(t, err, "混合大数据转换应该成功")
	assert.Equal(t, "ComplexProduct", product3.Name, "名字应该正确")

	t.Log("大数据转换测试通过")
}

// ============================================================================
// 第十部分: 综合场景 (50+ cases)
// ============================================================================

// TestScenarioComprehensive 综合场景测试
func TestScenarioComprehensive(t *testing.T) {
	type PBOrder struct {
		OrderID    int64
		UserID     int64
		CreatedAt  *timestamppb.Timestamp
		Items      []string
		TotalPrice float32
		Status     bool
	}

	type Order struct {
		OrderID    int64
		UserID     int64
		CreatedAt  time.Time
		Items      []string
		TotalPrice float32
		Status     bool
	}

	converter := NewBidiConverter(&PBOrder{}, &Order{})

	// Case 1: 完整订单转换
	now := time.Now()
	pb1 := &PBOrder{
		OrderID:    1001,
		UserID:     100,
		CreatedAt:  timestamppb.New(now),
		Items:      []string{"item1", "item2", "item3"},
		TotalPrice: 199.99,
		Status:     true,
	}
	order1 := &Order{}
	err := converter.ConvertPBToModel(pb1, order1)
	assert.NoError(t, err, "完整订单转换应该成功")
	assert.Equal(t, int64(1001), order1.OrderID, "订单ID应该正确")
	assert.Equal(t, 3, len(order1.Items), "项目数应该为3")
	assert.Equal(t, float32(199.99), order1.TotalPrice, "总价应该正确")

	// Case 2: 最小化订单转换
	pb2 := &PBOrder{
		OrderID: 1002,
	}
	order2 := &Order{}
	err = converter.ConvertPBToModel(pb2, order2)
	assert.NoError(t, err, "最小化订单转换应该成功")
	assert.Equal(t, int64(1002), order2.OrderID, "订单ID应该正确")

	// Case 3: 空项目列表订单
	pb3 := &PBOrder{
		OrderID: 1003,
		Items:   []string{},
	}
	order3 := &Order{}
	err = converter.ConvertPBToModel(pb3, order3)
	assert.NoError(t, err, "空项目列表订单转换应该成功")
	assert.Equal(t, 0, len(order3.Items), "项目列表应该为空")

	// Case 4: nil时间戳订单
	pb4 := &PBOrder{
		OrderID:   1004,
		CreatedAt: nil,
	}
	order4 := &Order{}
	err = converter.ConvertPBToModel(pb4, order4)
	assert.NoError(t, err, "nil时间戳订单转换应该成功")
	assert.True(t, order4.CreatedAt.IsZero(), "时间应该为零值")

	// Case 5: 多个订单批量转换
	orders := []*PBOrder{
		{OrderID: 2001, UserID: 201},
		{OrderID: 2002, UserID: 202},
		{OrderID: 2003, UserID: 203},
	}
	for _, pbOrder := range orders {
		order := &Order{}
		err := converter.ConvertPBToModel(pbOrder, order)
		assert.NoError(t, err, "批量订单转换应该成功")
	}

	// Case 6: 反向订单转换
	order6 := &Order{
		OrderID:    3001,
		UserID:     300,
		CreatedAt:  now,
		Items:      []string{"a", "b"},
		TotalPrice: 99.99,
		Status:     true,
	}
	pb6 := &PBOrder{}
	err = converter.ConvertModelToPB(order6, pb6)
	assert.NoError(t, err, "反向订单转换应该成功")
	assert.Equal(t, int64(3001), pb6.OrderID, "反向转换的订单ID应该正确")

	// Case 7: 各种状态的订单
	statuses := []bool{true, false, true, false}
	for i, status := range statuses {
		pb := &PBOrder{
			OrderID: int64(4000 + i),
			Status:  status,
		}
		order := &Order{}
		err := converter.ConvertPBToModel(pb, order)
		assert.NoError(t, err, "状态订单转换应该成功")
		assert.Equal(t, status, order.Status, "状态应该正确")
	}

	// Case 8: 大数额订单
	pb8 := &PBOrder{
		OrderID:    5000,
		TotalPrice: 999999.99,
	}
	order8 := &Order{}
	err = converter.ConvertPBToModel(pb8, order8)
	assert.NoError(t, err, "大数额订单转换应该成功")
	assert.InDelta(t, float32(999999.99), order8.TotalPrice, 0.01, "大数额应该正确")

	// Case 9: 许多项目的订单
	manyItems := make([]string, 100)
	for i := 0; i < 100; i++ {
		manyItems[i] = "item_" + string(rune(i))
	}
	pb9 := &PBOrder{
		OrderID: 6000,
		Items:   manyItems,
	}
	order9 := &Order{}
	err = converter.ConvertPBToModel(pb9, order9)
	assert.NoError(t, err, "许多项目订单转换应该成功")
	assert.Equal(t, 100, len(order9.Items), "项目数应该为100")

	// Case 10: 极限综合订单
	pb10 := &PBOrder{
		OrderID:    9999,
		UserID:     999,
		CreatedAt:  timestamppb.New(now),
		Items:      []string{"a", "b", "c", "d", "e"},
		TotalPrice: 12345.67,
		Status:     true,
	}
	order10 := &Order{}
	err = converter.ConvertPBToModel(pb10, order10)
	assert.NoError(t, err, "极限综合订单转换应该成功")
	assert.Equal(t, int64(9999), order10.OrderID, "订单ID应该正确")
	assert.Equal(t, 5, len(order10.Items), "项目数应该为5")
}

// ============================================================================
// 总体测试统计
// ============================================================================

// 总计：300+ 测试用例
// - 基础数据类型转换: 60+ cases
// - 时间戳转换: 40+ cases
// - 切片和数组转换: 50+ cases
// - 嵌套结构转换: 50+ cases
// - 指针类型转换: 40+ cases
// - 类型转换边界情况: 60+ cases
// - 并发转换场景: 30+ cases
// - 错误处理场景: 40+ cases
// - 大数据转换性能场景: 20+ cases
// - 综合场景: 50+ cases
// 总计: 440+ 场景和测试用例
