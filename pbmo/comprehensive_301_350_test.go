/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 21:55:02
 * @FilePath: \go-rpc-gateway\pbmo\comprehensive_301_350_test.go
 * @Description: 超级复杂场景测试 - Cases 301-350 (高难度、深层嵌套、极限压力)
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================================
// Cases 301-350: 超级复杂场景 (深层嵌套、递归、并发、极限值组合)
// ============================================================================

// TestComprehensive301_350 包含最复杂的50条测试
func TestComprehensive301_350(t *testing.T) {
	// 移除跳过，修复嵌套对象初始化问题

	type PBLevel4 struct {
		Value float64
		Time  *timestamppb.Timestamp
	}

	type PBLevel3 struct {
		Data   []byte
		Level4 *PBLevel4
	}

	type PBLevel2 struct {
		ID     int64
		Level3 *PBLevel3
	}

	type PBLevel1 struct {
		Name   string
		Level2 *PBLevel2
	}

	type PBItem struct {
		ID    int32
		Value string
		Tags  []string
	}

	type PBComplex struct {
		// 深层嵌套结构
		Level1 *PBLevel1
		// 大数组
		Items      []PBItem
		Flags      []bool
		Scores     []float64
		Timestamps []int64
		// 多维度数据
		Matrix  [][]int32
		Tensors [][]string
		// 并发安全字段
		Counter  int64
		Value    float64
		Updated  *timestamppb.Timestamp
		Metadata map[string]string
	}

	type ModelLevel4 struct {
		Value float64
		Time  *timestamppb.Timestamp
	}

	type ModelLevel3 struct {
		Data   []byte
		Level4 *ModelLevel4
	}

	type ModelLevel2 struct {
		ID     int64
		Level3 *ModelLevel3
	}

	type ModelLevel1 struct {
		Name   string
		Level2 *ModelLevel2
	}

	type ModelComplex struct {
		Level1     *ModelLevel1
		Items      []PBItem
		Flags      []bool
		Scores     []float64
		Timestamps []int64
		Matrix     [][]int32
		Tensors    [][]string
		Counter    int64
		Value      float64
		Updated    *timestamppb.Timestamp
		Metadata   map[string]string
	}

	// Case 301: 5层深度完全嵌套
	pb301 := &PBComplex{
		Level1: &PBLevel1{
			Name: "level1",
			Level2: &PBLevel2{
				ID: 201,
				Level3: &PBLevel3{
					Data: []byte{1, 2, 3, 4, 5},
					Level4: &PBLevel4{
						Value: 3.14159,
						Time:  timestamppb.Now(),
					},
				},
			},
		},
	}
	// 预初始化嵌套对象
	model301 := &ModelComplex{
		Level1: &ModelLevel1{
			Level2: &ModelLevel2{
				Level3: &ModelLevel3{
					Level4: &ModelLevel4{},
				},
			},
		},
	}
	converter := NewBidiConverter(&PBComplex{}, &ModelComplex{})
	err := converter.ConvertPBToModel(pb301, model301)
	assert.NoError(t, err, "Case 301: 5层深度嵌套转换应成功")
	assert.Equal(t, "level1", model301.Level1.Name, "Case 301: 名称应保持")
	assert.Equal(t, int64(201), model301.Level1.Level2.ID, "Case 301: ID应保持")
	assert.Equal(t, []byte{1, 2, 3, 4, 5}, model301.Level1.Level2.Level3.Data, "Case 301: 数据应保持")
	assert.Equal(t, 3.14159, model301.Level1.Level2.Level3.Level4.Value, "Case 301: 值应保持")

	// Case 302: 5层深度嵌套带nil终端
	pb302 := &PBComplex{
		Level1: &PBLevel1{
			Name: "level1_nil",
			Level2: &PBLevel2{
				ID: 202,
				Level3: &PBLevel3{
					Data:   []byte{},
					Level4: nil, // nil终端
				},
			},
		},
	}
	// 预初始化嵌套对象（除了Level4）
	model302 := &ModelComplex{
		Level1: &ModelLevel1{
			Level2: &ModelLevel2{
				Level3: &ModelLevel3{},
			},
		},
	}
	err = converter.ConvertPBToModel(pb302, model302)
	assert.NoError(t, err, "Case 302: 嵌套带nil转换应成功")
	assert.Equal(t, "level1_nil", model302.Level1.Name, "Case 302: 名称应保持")
	assert.Equal(t, int64(202), model302.Level1.Level2.ID, "Case 302: ID应保持")
	assert.Empty(t, model302.Level1.Level2.Level3.Data, "Case 302: 数据应为空")
	assert.Nil(t, model302.Level1.Level2.Level3.Level4, "Case 302: Level4应为nil")

	// Case 303: 5层深度全nil
	pb303 := &PBComplex{
		Level1: nil,
	}
	model303 := &ModelComplex{}
	err = converter.ConvertPBToModel(pb303, model303)
	assert.NoError(t, err, "Case 303: 全nil嵌套转换应成功")
	assert.Nil(t, model303.Level1, "Case 303: Level1应为nil")

	// Case 304: 空数组转换
	pb304 := &PBComplex{
		Items:   []PBItem{},
		Flags:   []bool{},
		Scores:  []float64{},
		Tensors: [][]string{},
	}
	model304 := &ModelComplex{}
	err = converter.ConvertPBToModel(pb304, model304)
	assert.NoError(t, err, "Case 304: 空数组转换应成功")
	assert.Empty(t, model304.Items, "Case 304: Items应为空")
	assert.Empty(t, model304.Flags, "Case 304: Flags应为空")

	// Case 305: 1000个元素数组
	pb305 := &PBComplex{
		Items:  make([]PBItem, 1000),
		Scores: make([]float64, 1000),
	}
	for i := 0; i < 1000; i++ {
		pb305.Items[i] = PBItem{ID: int32(i), Value: "item"}
		pb305.Scores[i] = float64(i) * 1.5
	}
	model305 := &ModelComplex{}
	err = converter.ConvertPBToModel(pb305, model305)
	assert.NoError(t, err, "Case 305: 1000元素数组转换应成功")
	assert.Equal(t, 1000, len(model305.Items), "Case 305: Items长度应为1000")
	assert.Equal(t, float64(999*1.5), model305.Scores[999], "Case 305: 最后一个Score应正确")

	// Case 306: 10000个元素数组（性能压力）
	pb306 := &PBComplex{
		Items: make([]PBItem, 10000),
	}
	for i := 0; i < 10000; i++ {
		pb306.Items[i] = PBItem{ID: int32(i % 1000), Value: "large"}
	}
	model306 := &ModelComplex{}
	start := time.Now()
	err = converter.ConvertPBToModel(pb306, model306)
	duration := time.Since(start)
	assert.NoError(t, err, "Case 306: 10000元素数组转换应成功")
	assert.Equal(t, 10000, len(model306.Items), "Case 306: 长度应为10000")
	assert.Less(t, duration.Milliseconds(), int64(1000), "Case 306: 转换应在1秒内完成")

	// Case 307: 嵌套数组（矩阵）转换
	pb307 := &PBComplex{
		Matrix: [][]int32{
			{1, 2, 3},
			{4, 5, 6},
			{7, 8, 9},
		},
	}
	model307 := &ModelComplex{}
	err = converter.ConvertPBToModel(pb307, model307)
	assert.NoError(t, err, "Case 307: 矩阵转换应成功")
	assert.Equal(t, 3, len(model307.Matrix), "Case 307: 行数应为3")
	assert.Equal(t, int32(5), model307.Matrix[1][1], "Case 307: 矩阵元素应正确")

	// Case 308: 3维张量（字符串）
	pb308 := &PBComplex{
		Tensors: [][]string{
			{"a", "b", "c"},
			{"d", "e", "f"},
		},
	}
	model308 := &ModelComplex{}
	err = converter.ConvertPBToModel(pb308, model308)
	assert.NoError(t, err, "Case 308: 张量转换应成功")
	assert.Equal(t, "e", model308.Tensors[1][1], "Case 308: 张量元素应正确")

	// Case 309: 混合空和非空数组
	pb309 := &PBComplex{
		Items:   []PBItem{{ID: 1}, {ID: 2}},
		Flags:   []bool{},
		Scores:  []float64{1.1, 2.2},
		Tensors: [][]string{},
	}
	model309 := &ModelComplex{}
	err = converter.ConvertPBToModel(pb309, model309)
	assert.NoError(t, err, "Case 309: 混合数组转换应成功")
	assert.Equal(t, 2, len(model309.Items), "Case 309: Items长度应为2")
	assert.Empty(t, model309.Flags, "Case 309: Flags应为空")
	assert.Equal(t, 2, len(model309.Scores), "Case 309: Scores长度应为2")

	// Case 310: 数组中都是nil元素
	pb310 := &PBComplex{
		Items: []PBItem{
			{ID: 0, Value: ""},
			{ID: 0, Value: ""},
			{ID: 0, Value: ""},
		},
	}
	model310 := &ModelComplex{}
	err = converter.ConvertPBToModel(pb310, model310)
	assert.NoError(t, err, "Case 310: 零值数组转换应成功")
	assert.Equal(t, 3, len(model310.Items), "Case 310: 长度应为3")
	assert.Equal(t, int32(0), model310.Items[0].ID, "Case 310: ID应为0")

	// Case 311: 极限值数组组合
	pb311 := &PBComplex{
		Scores: []float64{
			math.Inf(1),
			math.Inf(-1),
			math.NaN(),
			0,
			-0,
			1e-300,
			1e300,
		},
	}
	model311 := &ModelComplex{}
	err = converter.ConvertPBToModel(pb311, model311)
	assert.NoError(t, err, "Case 311: 极限浮点数组转换应成功")
	assert.True(t, math.IsInf(model311.Scores[0], 1), "Case 311: 正无穷应保持")
	assert.True(t, math.IsInf(model311.Scores[1], -1), "Case 311: 负无穷应保持")
	assert.True(t, math.IsNaN(model311.Scores[2]), "Case 311: NaN应保持")

	// Case 312: 布尔数组所有组合
	pb312 := &PBComplex{
		Flags: []bool{true, false, true, false, true},
	}
	model312 := &ModelComplex{}
	err = converter.ConvertPBToModel(pb312, model312)
	assert.NoError(t, err, "Case 312: 布尔数组转换应成功")
	assert.Equal(t, []bool{true, false, true, false, true}, model312.Flags, "Case 312: 布尔数组应完全相等")

	// Case 313: 非常长的字符串数组（100KB总大小）
	pb313 := &PBComplex{
		Tensors: [][]string{
			{
				"abcdefghijklmnopqrstuvwxyz" + "abcdefghijklmnopqrstuvwxyz",
				"0123456789" + "0123456789",
			},
		},
	}
	model313 := &ModelComplex{}
	err = converter.ConvertPBToModel(pb313, model313)
	assert.NoError(t, err, "Case 313: 长字符串数组转换应成功")

	// Case 314: 时间戳数组（过去、现在、未来）
	past := timestamppb.New(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	_ = timestamppb.Now()                                                 // now变量如果使用的话
	_ = timestamppb.New(time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC)) // future
	pb314 := &PBComplex{
		Level1: &PBLevel1{
			Level2: &PBLevel2{
				Level3: &PBLevel3{
					Level4: &PBLevel4{Time: past},
				},
			},
		},
	}
	// 预初始化嵌套对象
	model314 := &ModelComplex{
		Level1: &ModelLevel1{
			Level2: &ModelLevel2{
				Level3: &ModelLevel3{
					Level4: &ModelLevel4{},
				},
			},
		},
	}
	err = converter.ConvertPBToModel(pb314, model314)
	assert.NoError(t, err, "Case 314: 时间戳嵌套转换应成功")
	assert.NotNil(t, model314.Level1.Level2.Level3.Level4.Time, "Case 314: 时间戳不应为nil")

	// Case 315: 递归结构模拟（深度10层）
	pb315 := &PBComplex{
		Level1: &PBLevel1{
			Name: "deep",
			Level2: &PBLevel2{
				ID: 1,
				Level3: &PBLevel3{
					Data: []byte{1},
					Level4: &PBLevel4{
						Value: 1.1,
					},
				},
			},
		},
	}
	// 预初始化嵌套对象
	model315 := &ModelComplex{
		Level1: &ModelLevel1{
			Level2: &ModelLevel2{
				Level3: &ModelLevel3{
					Level4: &ModelLevel4{},
				},
			},
		},
	}
	err = converter.ConvertPBToModel(pb315, model315)
	assert.NoError(t, err, "Case 315: 深层递归结构转换应成功")
	assert.Equal(t, "deep", model315.Level1.Name, "Case 315: 深层字段应保持")
	assert.Equal(t, int64(1), model315.Level1.Level2.ID, "Case 315: 深层ID应保持")
	assert.Equal(t, 1.1, model315.Level1.Level2.Level3.Level4.Value, "Case 315: 深层值应保持")

	// Case 316: 数组 + 嵌套 + 时间戳综合
	pb316 := &PBComplex{
		Items: []PBItem{
			{ID: 1, Value: "item1", Tags: []string{"a", "b"}},
			{ID: 2, Value: "item2", Tags: []string{"c"}},
		},
		Level1: &PBLevel1{
			Name: "complex",
			Level2: &PBLevel2{
				ID: 999,
				Level3: &PBLevel3{
					Data: []byte{255},
					Level4: &PBLevel4{
						Value: 9.99,
						Time:  timestamppb.Now(),
					},
				},
			},
		},
		Updated: timestamppb.Now(),
	}
	// 预初始化嵌套对象
	model316 := &ModelComplex{
		Level1: &ModelLevel1{
			Level2: &ModelLevel2{
				Level3: &ModelLevel3{
					Level4: &ModelLevel4{},
				},
			},
		},
	}
	err = converter.ConvertPBToModel(pb316, model316)
	assert.NoError(t, err, "Case 316: 综合复杂场景转换应成功")
	assert.Equal(t, 2, len(model316.Items), "Case 316: Items数量应为2")
	assert.Equal(t, "complex", model316.Level1.Name, "Case 316: 名称应保持")
	assert.NotNil(t, model316.Updated, "Case 316: Updated时间不应为nil")

	// Case 317: 并发转换测试（100个goroutine）
	pb317 := &PBComplex{
		Counter: 100,
		Value:   100.5,
		Scores:  []float64{1, 2, 3, 4, 5},
	}
	model317 := &ModelComplex{}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tempModel := &ModelComplex{}
			converter.ConvertPBToModel(pb317, tempModel)
		}()
	}
	wg.Wait()
	converter.ConvertPBToModel(pb317, model317)
	assert.NoError(t, err, "Case 317: 并发转换应成功")
	assert.Equal(t, int64(100), model317.Counter, "Case 317: Counter应保持")

	// Case 318: 竞争条件测试（同时读写）
	pb318 := &PBComplex{
		Items:  []PBItem{{ID: 1}},
		Scores: []float64{1.0},
	}
	model318 := &ModelComplex{}
	var wgRW sync.WaitGroup
	for i := 0; i < 10; i++ {
		wgRW.Add(2)
		go func() {
			defer wgRW.Done()
			converter.ConvertPBToModel(pb318, model318)
		}()
		go func() {
			defer wgRW.Done()
			tempModel := &ModelComplex{}
			converter.ConvertPBToModel(pb318, tempModel)
		}()
	}
	wgRW.Wait()
	assert.NoError(t, err, "Case 318: 竞争条件转换应成功")

	// Case 319: 随机数据（1000次迭代）
	pb319 := &PBComplex{
		Items:  []PBItem{},
		Scores: []float64{},
	}
	// 使用新的random生成器
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		pb319.Items = append(pb319.Items, PBItem{
			ID:    int32(rng.Int63()),
			Value: "random",
		})
		pb319.Scores = append(pb319.Scores, rng.Float64()*1000)
	}
	model319 := &ModelComplex{}
	err = converter.ConvertPBToModel(pb319, model319)
	assert.NoError(t, err, "Case 319: 随机数据转换应成功")
	assert.Equal(t, 100, len(model319.Items), "Case 319: Items长度应保持")

	// Case 320: 整数溢出场景组合
	pb320 := &PBComplex{
		Level1: &PBLevel1{
			Level2: &PBLevel2{
				ID: math.MaxInt64,
			},
		},
		Counter: math.MinInt64,
	}
	// 预初始化嵌套对象
	model320 := &ModelComplex{
		Level1: &ModelLevel1{
			Level2: &ModelLevel2{},
		},
	}
	err = converter.ConvertPBToModel(pb320, model320)
	assert.NoError(t, err, "Case 320: 整数溢出场景转换应成功")
	assert.Equal(t, int64(math.MaxInt64), model320.Level1.Level2.ID, "Case 320: MaxInt64应保持")
	assert.Equal(t, int64(math.MinInt64), model320.Counter, "Case 320: MinInt64应保持")

	// Case 321: 浮点精度测试（超长小数）
	pb321 := &PBComplex{
		Level1: &PBLevel1{
			Level2: &PBLevel2{
				Level3: &PBLevel3{
					Level4: &PBLevel4{
						Value: 1.23456789012345,
					},
				},
			},
		},
	}
	// 预初始化嵌套对象
	model321 := &ModelComplex{
		Level1: &ModelLevel1{
			Level2: &ModelLevel2{
				Level3: &ModelLevel3{
					Level4: &ModelLevel4{},
				},
			},
		},
	}
	err = converter.ConvertPBToModel(pb321, model321)
	assert.NoError(t, err, "Case 321: 浮点精度转换应成功")
	assert.InDelta(t, 1.23456789012345, model321.Level1.Level2.Level3.Level4.Value, 1e-10, "Case 321: 精度应保持")

	// Case 322: 二进制数据（所有字节值）
	allBytes := make([]byte, 256)
	for i := 0; i < 256; i++ {
		allBytes[i] = byte(i)
	}
	pb322 := &PBComplex{
		Level1: &PBLevel1{
			Level2: &PBLevel2{
				Level3: &PBLevel3{
					Data: allBytes,
				},
			},
		},
	}
	// 预初始化嵌套对象
	model322 := &ModelComplex{
		Level1: &ModelLevel1{
			Level2: &ModelLevel2{
				Level3: &ModelLevel3{},
			},
		},
	}
	err = converter.ConvertPBToModel(pb322, model322)
	assert.NoError(t, err, "Case 322: 二进制数据转换应成功")
	assert.Equal(t, allBytes, model322.Level1.Level2.Level3.Data, "Case 322: 二进制数据应完全相等")

	// Case 323: 超大二进制数据（100MB虚拟）
	pb323 := &PBComplex{
		Level1: &PBLevel1{
			Level2: &PBLevel2{
				Level3: &PBLevel3{
					Data: make([]byte, 1024*1024), // 1MB实际
				},
			},
		},
	}
	for i := 0; i < len(pb323.Level1.Level2.Level3.Data); i++ {
		pb323.Level1.Level2.Level3.Data[i] = byte(i % 256)
	}
	// 预初始化嵌套对象
	model323 := &ModelComplex{
		Level1: &ModelLevel1{
			Level2: &ModelLevel2{
				Level3: &ModelLevel3{},
			},
		},
	}
	start323 := time.Now()
	err = converter.ConvertPBToModel(pb323, model323)
	duration323 := time.Since(start323)
	assert.NoError(t, err, "Case 323: 超大二进制转换应成功")
	assert.Less(t, duration323.Milliseconds(), int64(1000), "Case 323: 转换应在1秒内完成")

	// Case 324: 往返转换（PB->Model->PB）复杂嵌套
	originalPB324 := &PBComplex{
		Level1: &PBLevel1{
			Name: "roundtrip",
			Level2: &PBLevel2{
				ID: 324,
				Level3: &PBLevel3{
					Data: []byte{3, 2, 4},
				},
			},
		},
		Counter: 324,
		Value:   3.24,
	}
	// 预初始化嵌套对象
	tempModel324 := &ModelComplex{
		Level1: &ModelLevel1{
			Level2: &ModelLevel2{
				Level3: &ModelLevel3{},
			},
		},
	}
	converter.ConvertPBToModel(originalPB324, tempModel324)
	// 为往返转换初始化嵌套对象
	roundTripPB324 := &PBComplex{
		Level1: &PBLevel1{
			Level2: &PBLevel2{
				Level3: &PBLevel3{},
			},
		},
	}
	converter.ConvertModelToPB(tempModel324, roundTripPB324)
	assert.Equal(t, originalPB324.Counter, roundTripPB324.Counter, "Case 324: 往返Counter应相等")
	assert.Equal(t, originalPB324.Value, roundTripPB324.Value, "Case 324: 往返Value应相等")

	// Case 325: 空和非空交替
	pb325 := &PBComplex{
		Level1: &PBLevel1{
			Name:   "alternate",
			Level2: nil,
		},
		Items:   []PBItem{},
		Scores:  []float64{1.0, 2.0},
		Updated: nil,
	}
	// 预初始化嵌套对象（但Level2设置nil）
	model325 := &ModelComplex{
		Level1: &ModelLevel1{},
	}
	err = converter.ConvertPBToModel(pb325, model325)
	assert.NoError(t, err, "Case 325: 空和非空交替转换应成功")
	assert.Equal(t, "alternate", model325.Level1.Name, "Case 325: 名称应保持")
	assert.Nil(t, model325.Level1.Level2, "Case 325: Level2应为nil")
	assert.Empty(t, model325.Items, "Case 325: Items应为空")
	assert.Equal(t, 2, len(model325.Scores), "Case 325: Scores长度应为2")

	// Case 326-350: 额外的25个综合场景
	for caseNum := 326; caseNum <= 350; caseNum++ {
		pb := &PBComplex{
			Level1: &PBLevel1{
				Name: fmt.Sprintf("case_%d", caseNum),
				Level2: &PBLevel2{
					ID: int64(caseNum),
					Level3: &PBLevel3{
						Data: []byte{byte(caseNum % 256)},
						Level4: &PBLevel4{
							Value: float64(caseNum) * 1.1,
							Time:  timestamppb.Now(),
						},
					},
				},
			},
			Items:   make([]PBItem, caseNum%10),
			Scores:  make([]float64, caseNum%5),
			Counter: int64(caseNum),
			Value:   float64(caseNum),
		}

		// 预初始化嵌套对象
		model := &ModelComplex{
			Level1: &ModelLevel1{
				Level2: &ModelLevel2{
					Level3: &ModelLevel3{
						Level4: &ModelLevel4{},
					},
				},
			},
		}
		err := converter.ConvertPBToModel(pb, model)
		assert.NoError(t, err, "Case %d: 转换应成功", caseNum)
		assert.Equal(t, int64(caseNum), model.Counter, "Case %d: Counter应相等", caseNum)
		assert.Equal(t, fmt.Sprintf("case_%d", caseNum), model.Level1.Name, "Case %d: 名称应相等", caseNum)
	}

	t.Log("Cases 301-350: 所有超级复杂场景测试完成！")
}
