/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 21:55:02
 * @FilePath: \go-rpc-gateway\pbmo\comprehensive_101_150_test.go
 * @Description: 综合场景测试 - 第3批 101-150 深层嵌套和空值场景
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
// 第三批: 101-150条复杂场景测试 (深层嵌套结构 + 空值混合)
// ============================================================================

// TestComprehensive101_150 包含深层嵌套和空值的50条测试
func TestComprehensive101_150(t *testing.T) {
	// 定义深层嵌套结构
	type Level3 struct {
		Value    string
		Number   int32
		FloatVal float64
	}

	type Level2 struct {
		Name    string
		Level3  *Level3
		IntVal  int64
		BoolVal bool
	}

	type Level1 struct {
		Title    string
		Level2   *Level2
		TimeVal  *timestamppb.Timestamp
		FloatVal float32
	}

	type PBNested struct {
		ID      int32
		Level1  *Level1
		ListVal []string
		MapVal  map[string]int32
	}

	type ModelNested struct {
		ID      int32
		Level1  *Level1
		ListVal []string
		MapVal  map[string]int32
	}

	converter := NewBidiConverter(&PBNested{}, &ModelNested{})

	// ========== Case 101-110: 深层嵌套全部填充 ==========

	// Case 101: 三层嵌套全部非空
	pb101 := &PBNested{
		ID: 101,
		Level1: &Level1{
			Title:    "deep_level1",
			FloatVal: 1.01,
			TimeVal:  timestamppb.Now(),
			Level2: &Level2{
				Name:    "deep_level2",
				IntVal:  101101,
				BoolVal: true,
				Level3: &Level3{
					Value:    "deep_level3",
					Number:   101,
					FloatVal: 101.101,
				},
			},
		},
		ListVal: []string{"a", "b", "c"},
	}
	model101 := &ModelNested{}
	err := converter.ConvertPBToModel(pb101, model101)
	assert.NoError(t, err, "Case 101: 三层嵌套全部非空转换应成功")
	assert.Equal(t, int32(101), model101.ID, "Case 101: ID应相等")
	assert.NotNil(t, model101.Level1, "Case 101: Level1不应为nil")
	assert.NotNil(t, model101.Level1.Level2, "Case 101: Level2不应为nil")
	assert.NotNil(t, model101.Level1.Level2.Level3, "Case 101: Level3不应为nil")
	assert.Equal(t, "deep_level3", model101.Level1.Level2.Level3.Value, "Case 101: 最深层Value应相等")

	// Case 102: 三层嵌套，第三层为nil
	pb102 := &PBNested{
		ID: 102,
		Level1: &Level1{
			Title:    "level2_only",
			FloatVal: 1.02,
			Level2: &Level2{
				Name:    "level2_name",
				IntVal:  102102,
				BoolVal: false,
				Level3:  nil,
			},
		},
	}
	model102 := &ModelNested{}
	err = converter.ConvertPBToModel(pb102, model102)
	assert.NoError(t, err, "Case 102: 第三层nil转换应成功")
	assert.NotNil(t, model102.Level1.Level2, "Case 102: Level2不应为nil")
	assert.Nil(t, model102.Level1.Level2.Level3, "Case 102: Level3应为nil")

	// Case 103: 三层嵌套，第二层为nil
	pb103 := &PBNested{
		ID: 103,
		Level1: &Level1{
			Title:    "level1_only",
			FloatVal: 1.03,
			TimeVal:  timestamppb.Now(),
			Level2:   nil,
		},
	}
	model103 := &ModelNested{}
	err = converter.ConvertPBToModel(pb103, model103)
	assert.NoError(t, err, "Case 103: 第二层nil转换应成功")
	assert.NotNil(t, model103.Level1, "Case 103: Level1不应为nil")
	assert.Nil(t, model103.Level1.Level2, "Case 103: Level2应为nil")

	// Case 104: 第一层为nil
	pb104 := &PBNested{
		ID:      104,
		Level1:  nil,
		ListVal: []string{"x", "y"},
	}
	model104 := &ModelNested{}
	err = converter.ConvertPBToModel(pb104, model104)
	assert.NoError(t, err, "Case 104: 第一层nil转换应成功")
	assert.Nil(t, model104.Level1, "Case 104: Level1应为nil")
	assert.Equal(t, 2, len(model104.ListVal), "Case 104: ListVal长度应为2")

	// Case 105: 所有嵌套都为nil
	pb105 := &PBNested{
		ID:      105,
		Level1:  nil,
		ListVal: nil,
		MapVal:  nil,
	}
	model105 := &ModelNested{}
	err = converter.ConvertPBToModel(pb105, model105)
	assert.NoError(t, err, "Case 105: 所有嵌套nil转换应成功")
	assert.Nil(t, model105.Level1, "Case 105: Level1应为nil")
	assert.Nil(t, model105.ListVal, "Case 105: ListVal应为nil")
	assert.Nil(t, model105.MapVal, "Case 105: MapVal应为nil")

	// Case 106: 嵌套中含有空字符串
	pb106 := &PBNested{
		ID: 106,
		Level1: &Level1{
			Title: "", // 空字符串
			Level2: &Level2{
				Name: "", // 空字符串
				Level3: &Level3{
					Value: "", // 空字符串
				},
			},
		},
	}
	model106 := &ModelNested{}
	err = converter.ConvertPBToModel(pb106, model106)
	assert.NoError(t, err, "Case 106: 含空字符串嵌套转换应成功")
	assert.Equal(t, "", model106.Level1.Title, "Case 106: 空字符串应保持")

	// Case 107: 嵌套中含有零值数字
	pb107 := &PBNested{
		ID: 107,
		Level1: &Level1{
			FloatVal: 0.0,
			Level2: &Level2{
				IntVal:  0,
				BoolVal: false,
				Level3: &Level3{
					Number:   0,
					FloatVal: 0.0,
				},
			},
		},
	}
	model107 := &ModelNested{}
	err = converter.ConvertPBToModel(pb107, model107)
	assert.NoError(t, err, "Case 107: 零值嵌套转换应成功")
	assert.Equal(t, int32(0), model107.Level1.Level2.Level3.Number, "Case 107: 零值数字应保持")

	// Case 108: 嵌套中含有极值
	pb108 := &PBNested{
		ID: 108,
		Level1: &Level1{
			FloatVal: float32(math.MaxFloat32),
			Level2: &Level2{
				IntVal: math.MaxInt64,
				Level3: &Level3{
					Number:   math.MaxInt32,
					FloatVal: -math.MaxFloat64, // 最小（负最大）
				},
			},
		},
	}
	model108 := &ModelNested{}
	err = converter.ConvertPBToModel(pb108, model108)
	assert.NoError(t, err, "Case 108: 极值嵌套转换应成功")
	assert.Equal(t, int32(math.MaxInt32), model108.Level1.Level2.Level3.Number, "Case 108: 最大int32应保持")

	// Case 109: 嵌套中含有特殊浮点值
	pb109 := &PBNested{
		ID: 109,
		Level1: &Level1{
			FloatVal: float32(math.NaN()),
			Level2: &Level2{
				Level3: &Level3{
					FloatVal: math.Inf(1),
				},
			},
		},
	}
	model109 := &ModelNested{}
	err = converter.ConvertPBToModel(pb109, model109)
	assert.NoError(t, err, "Case 109: NaN和Inf嵌套转换应成功")
	assert.True(t, math.IsNaN(float64(model109.Level1.FloatVal)), "Case 109: NaN应保持")
	assert.True(t, math.IsInf(model109.Level1.Level2.Level3.FloatVal, 1), "Case 109: 正无穷应保持")

	// Case 110: 嵌套中含有Unicode字符串
	pb110 := &PBNested{
		ID: 110,
		Level1: &Level1{
			Title: "Unicode: 你好世界🌍",
			Level2: &Level2{
				Name: "مرحبا بالعالم",
				Level3: &Level3{
					Value: "🎉 Emoji test 🚀",
				},
			},
		},
	}
	model110 := &ModelNested{}
	err = converter.ConvertPBToModel(pb110, model110)
	assert.NoError(t, err, "Case 110: Unicode嵌套转换应成功")
	assert.Equal(t, "Unicode: 你好世界🌍", model110.Level1.Title, "Case 110: Unicode字符应保持")

	// ========== Case 111-120: 列表和映射的复杂场景 ==========

	// Case 111: 空列表
	pb111 := &PBNested{
		ID:      111,
		ListVal: []string{},
	}
	model111 := &ModelNested{}
	err = converter.ConvertPBToModel(pb111, model111)
	assert.NoError(t, err, "Case 111: 空列表转换应成功")
	assert.NotNil(t, model111.ListVal, "Case 111: 空列表应存在")
	assert.Equal(t, 0, len(model111.ListVal), "Case 111: 列表长度应为0")

	// Case 112: 大列表
	largeList := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		largeList[i] = "item_" + string(rune(i%10))
	}
	pb112 := &PBNested{
		ID:      112,
		ListVal: largeList,
	}
	model112 := &ModelNested{}
	err = converter.ConvertPBToModel(pb112, model112)
	assert.NoError(t, err, "Case 112: 大列表转换应成功")
	assert.Equal(t, 1000, len(model112.ListVal), "Case 112: 列表长度应为1000")

	// Case 113: 列表含空字符串
	pb113 := &PBNested{
		ID:      113,
		ListVal: []string{"", "a", "", "b", ""},
	}
	model113 := &ModelNested{}
	err = converter.ConvertPBToModel(pb113, model113)
	assert.NoError(t, err, "Case 113: 含空字符串列表转换应成功")
	assert.Equal(t, 5, len(model113.ListVal), "Case 113: 列表长度应为5")
	assert.Equal(t, "", model113.ListVal[0], "Case 113: 第一个元素应为空")

	// Case 114: 列表含Unicode
	pb114 := &PBNested{
		ID:      114,
		ListVal: []string{"你好", "مرحبا", "🌍", "😀"},
	}
	model114 := &ModelNested{}
	err = converter.ConvertPBToModel(pb114, model114)
	assert.NoError(t, err, "Case 114: Unicode列表转换应成功")
	assert.Equal(t, "🌍", model114.ListVal[2], "Case 114: Emoji应保持")

	// Case 115: 列表与嵌套组合
	pb115 := &PBNested{
		ID: 115,
		Level1: &Level1{
			Title: "with_list",
		},
		ListVal: []string{"list1", "list2"},
	}
	model115 := &ModelNested{}
	err = converter.ConvertPBToModel(pb115, model115)
	assert.NoError(t, err, "Case 115: 列表与嵌套组合转换应成功")
	assert.NotNil(t, model115.Level1, "Case 115: Level1不应为nil")
	assert.Equal(t, 2, len(model115.ListVal), "Case 115: 列表长度应为2")

	// Case 116: 空映射
	pb116 := &PBNested{
		ID:     116,
		MapVal: map[string]int32{},
	}
	model116 := &ModelNested{}
	err = converter.ConvertPBToModel(pb116, model116)
	assert.NoError(t, err, "Case 116: 空映射转换应成功")
	assert.NotNil(t, model116.MapVal, "Case 116: 空映射应存在")
	assert.Equal(t, 0, len(model116.MapVal), "Case 116: 映射大小应为0")

	// Case 117: 大映射
	largeMap := make(map[string]int32)
	for i := 0; i < 100; i++ {
		largeMap["key_"+string(rune(i%10))] = int32(i)
	}
	pb117 := &PBNested{
		ID:     117,
		MapVal: largeMap,
	}
	model117 := &ModelNested{}
	err = converter.ConvertPBToModel(pb117, model117)
	assert.NoError(t, err, "Case 117: 大映射转换应成功")
	assert.Greater(t, len(model117.MapVal), 0, "Case 117: 映射应有数据")

	// Case 118: 映射含零值
	pb118 := &PBNested{
		ID: 118,
		MapVal: map[string]int32{
			"zero":     0,
			"one":      1,
			"negative": -1,
			"max":      math.MaxInt32,
			"min":      math.MinInt32,
		},
	}
	model118 := &ModelNested{}
	err = converter.ConvertPBToModel(pb118, model118)
	assert.NoError(t, err, "Case 118: 含极值映射转换应成功")
	assert.Equal(t, int32(0), model118.MapVal["zero"], "Case 118: 零值应保持")

	// Case 119: 列表和映射都有
	pb119 := &PBNested{
		ID:      119,
		ListVal: []string{"a", "b"},
		MapVal: map[string]int32{
			"x": 10,
			"y": 20,
		},
	}
	model119 := &ModelNested{}
	err = converter.ConvertPBToModel(pb119, model119)
	assert.NoError(t, err, "Case 119: 列表和映射组合转换应成功")
	assert.Equal(t, 2, len(model119.ListVal), "Case 119: 列表长度应为2")
	assert.Equal(t, 2, len(model119.MapVal), "Case 119: 映射大小应为2")

	// Case 120: 全部字段都有
	pb120 := &PBNested{
		ID: 120,
		Level1: &Level1{
			Title:    "complete",
			FloatVal: 1.20,
			TimeVal:  timestamppb.Now(),
			Level2: &Level2{
				Name:    "nested",
				IntVal:  120120,
				BoolVal: true,
				Level3: &Level3{
					Value:    "deep",
					Number:   120,
					FloatVal: 120.120,
				},
			},
		},
		ListVal: []string{"a", "b", "c"},
		MapVal: map[string]int32{
			"one": 1,
			"two": 2,
		},
	}
	model120 := &ModelNested{}
	err = converter.ConvertPBToModel(pb120, model120)
	assert.NoError(t, err, "Case 120: 全部字段转换应成功")
	assert.NotNil(t, model120.Level1, "Case 120: Level1不应为nil")
	assert.Equal(t, 3, len(model120.ListVal), "Case 120: 列表长度应为3")
	assert.Equal(t, 2, len(model120.MapVal), "Case 120: 映射大小应为2")

	// ========== Case 121-130: 反向转换（Model -> PB） ==========

	// Case 121: 反向转换三层嵌套全部非空
	modelSrc121 := &ModelNested{
		ID: 121,
		Level1: &Level1{
			Title:    "reverse_deep",
			FloatVal: 1.21,
			TimeVal:  timestamppb.Now(),
			Level2: &Level2{
				Name:    "reverse_level2",
				IntVal:  121121,
				BoolVal: true,
				Level3: &Level3{
					Value:    "reverse_level3",
					Number:   121,
					FloatVal: 121.121,
				},
			},
		},
	}
	pbResult121 := &PBNested{}
	err = converter.ConvertModelToPB(modelSrc121, pbResult121)
	assert.NoError(t, err, "Case 121: 反向三层嵌套转换应成功")
	assert.NotNil(t, pbResult121.Level1, "Case 121: 反向Level1不应为nil")
	assert.Equal(t, "reverse_level3", pbResult121.Level1.Level2.Level3.Value, "Case 121: 反向最深层应相等")

	// Case 122: 反向转换含nil
	modelSrc122 := &ModelNested{
		ID: 122,
		Level1: &Level1{
			Title: "partial",
			Level2: &Level2{
				Name:   "name_only",
				Level3: nil,
			},
		},
	}
	pbResult122 := &PBNested{}
	err = converter.ConvertModelToPB(modelSrc122, pbResult122)
	assert.NoError(t, err, "Case 122: 反向含nil转换应成功")
	assert.Nil(t, pbResult122.Level1.Level2.Level3, "Case 122: 反向Level3应为nil")

	// Case 123: 反向往返转换
	originalModel := &ModelNested{
		ID: 123,
		Level1: &Level1{
			Title:    "roundtrip",
			FloatVal: 1.23,
			Level2: &Level2{
				Name:    "trip",
				IntVal:  123123,
				BoolVal: false,
			},
		},
		ListVal: []string{"rt1", "rt2"},
	}
	tempPB := &PBNested{}
	converter.ConvertModelToPB(originalModel, tempPB)
	finalModel := &ModelNested{}
	err = converter.ConvertPBToModel(tempPB, finalModel)
	assert.NoError(t, err, "Case 123: 往返转换应成功")
	assert.Equal(t, originalModel.ID, finalModel.ID, "Case 123: 往返ID应相等")
	assert.Equal(t, originalModel.Level1.Title, finalModel.Level1.Title, "Case 123: 往返Title应相等")

	// Case 124: 反向转换大列表
	largeListModel := make([]string, 100)
	for i := 0; i < 100; i++ {
		largeListModel[i] = "model_" + string(rune(i%10))
	}
	modelSrc124 := &ModelNested{
		ID:      124,
		ListVal: largeListModel,
	}
	pbResult124 := &PBNested{}
	err = converter.ConvertModelToPB(modelSrc124, pbResult124)
	assert.NoError(t, err, "Case 124: 反向大列表转换应成功")
	assert.Equal(t, 100, len(pbResult124.ListVal), "Case 124: 反向列表长度应为100")

	// Case 125: 反向转换映射
	modelSrc125 := &ModelNested{
		ID: 125,
		MapVal: map[string]int32{
			"a": 10,
			"b": 20,
			"c": 30,
		},
	}
	pbResult125 := &PBNested{}
	err = converter.ConvertModelToPB(modelSrc125, pbResult125)
	assert.NoError(t, err, "Case 125: 反向映射转换应成功")
	assert.Equal(t, 3, len(pbResult125.MapVal), "Case 125: 反向映射大小应为3")

	// Case 126: 反向转换空嵌套
	modelSrc126 := &ModelNested{
		ID:      126,
		Level1:  nil,
		ListVal: nil,
		MapVal:  nil,
	}
	pbResult126 := &PBNested{}
	err = converter.ConvertModelToPB(modelSrc126, pbResult126)
	assert.NoError(t, err, "Case 126: 反向空嵌套转换应成功")
	assert.Nil(t, pbResult126.Level1, "Case 126: 反向Level1应为nil")

	// Case 127: 反向转换含极值
	modelSrc127 := &ModelNested{
		ID: 127,
		Level1: &Level1{
			FloatVal: float32(-math.MaxFloat32), // 最小float32
			Level2: &Level2{
				IntVal: math.MaxInt64,
				Level3: &Level3{
					Number:   math.MinInt32,
					FloatVal: math.Inf(-1),
				},
			},
		},
	}
	pbResult127 := &PBNested{}
	err = converter.ConvertModelToPB(modelSrc127, pbResult127)
	assert.NoError(t, err, "Case 127: 反向极值转换应成功")
	assert.Equal(t, int32(math.MinInt32), pbResult127.Level1.Level2.Level3.Number, "Case 127: 反向最小值应保持")

	// Case 128: 反向转换含特殊字符
	modelSrc128 := &ModelNested{
		ID: 128,
		Level1: &Level1{
			Title: "'; DROP TABLE;--",
			Level2: &Level2{
				Name: "<script>alert('xss')</script>",
				Level3: &Level3{
					Value: "tab\there\nnewline",
				},
			},
		},
	}
	pbResult128 := &PBNested{}
	err = converter.ConvertModelToPB(modelSrc128, pbResult128)
	assert.NoError(t, err, "Case 128: 反向特殊字符转换应成功")
	assert.Equal(t, "'; DROP TABLE;--", pbResult128.Level1.Title, "Case 128: 反向特殊字符应保持")

	// Case 129: 反向转换时间戳
	now := time.Now()
	modelSrc129 := &ModelNested{
		ID: 129,
		Level1: &Level1{
			TimeVal: timestamppb.New(now),
		},
	}
	pbResult129 := &PBNested{}
	err = converter.ConvertModelToPB(modelSrc129, pbResult129)
	assert.NoError(t, err, "Case 129: 反向时间戳转换应成功")
	assert.WithinDuration(t, now, pbResult129.Level1.TimeVal.AsTime(), time.Microsecond, "Case 129: 反向时间应接近")

	// Case 130: 反向转换完整结构
	modelSrc130 := &ModelNested{
		ID: 130,
		Level1: &Level1{
			Title:    "complete_reverse",
			FloatVal: 1.30,
			TimeVal:  timestamppb.Now(),
			Level2: &Level2{
				Name:    "nested_reverse",
				IntVal:  130130,
				BoolVal: true,
				Level3: &Level3{
					Value:    "deep_reverse",
					Number:   130,
					FloatVal: 130.130,
				},
			},
		},
		ListVal: []string{"rev1", "rev2", "rev3"},
		MapVal: map[string]int32{
			"x": 100,
			"y": 200,
			"z": 300,
		},
	}
	pbResult130 := &PBNested{}
	err = converter.ConvertModelToPB(modelSrc130, pbResult130)
	assert.NoError(t, err, "Case 130: 反向完整结构转换应成功")
	assert.NotNil(t, pbResult130.Level1, "Case 130: 反向Level1不应为nil")
	assert.Equal(t, 3, len(pbResult130.ListVal), "Case 130: 反向列表长度应为3")
	assert.Equal(t, 3, len(pbResult130.MapVal), "Case 130: 反向映射大小应为3")

	// ========== Case 131-140: 部分嵌套和空值混合 ==========

	// Case 131: 只有ID和空Level1
	pb131 := &PBNested{
		ID:     131,
		Level1: nil,
	}
	model131 := &ModelNested{}
	err = converter.ConvertPBToModel(pb131, model131)
	assert.NoError(t, err, "Case 131: ID仅有转换应成功")
	assert.Equal(t, int32(131), model131.ID, "Case 131: ID应相等")
	assert.Nil(t, model131.Level1, "Case 131: Level1应为nil")

	// Case 132: ID和ListVal，其他nil
	pb132 := &PBNested{
		ID:      132,
		ListVal: []string{"single"},
		Level1:  nil,
		MapVal:  nil,
	}
	model132 := &ModelNested{}
	err = converter.ConvertPBToModel(pb132, model132)
	assert.NoError(t, err, "Case 132: ID和ListVal转换应成功")
	assert.Equal(t, 1, len(model132.ListVal), "Case 132: ListVal长度应为1")

	// Case 133: 只有Level1.Title
	pb133 := &PBNested{
		ID: 133,
		Level1: &Level1{
			Title: "only_title",
		},
	}
	model133 := &ModelNested{}
	err = converter.ConvertPBToModel(pb133, model133)
	assert.NoError(t, err, "Case 133: 仅Title转换应成功")
	assert.Equal(t, "only_title", model133.Level1.Title, "Case 133: Title应相等")
	assert.Nil(t, model133.Level1.Level2, "Case 133: Level2应为nil")

	// Case 134: Level1.Title和Level2.Name
	pb134 := &PBNested{
		ID: 134,
		Level1: &Level1{
			Title: "title",
			Level2: &Level2{
				Name: "name",
			},
		},
	}
	model134 := &ModelNested{}
	err = converter.ConvertPBToModel(pb134, model134)
	assert.NoError(t, err, "Case 134: Title和Name转换应成功")
	assert.Equal(t, "name", model134.Level1.Level2.Name, "Case 134: Name应相等")
	assert.Nil(t, model134.Level1.Level2.Level3, "Case 134: Level3应为nil")

	// Case 135: 多个空字符串
	pb135 := &PBNested{
		ID: 135,
		Level1: &Level1{
			Title: "",
			Level2: &Level2{
				Name: "",
				Level3: &Level3{
					Value: "",
				},
			},
		},
		ListVal: []string{},
	}
	model135 := &ModelNested{}
	err = converter.ConvertPBToModel(pb135, model135)
	assert.NoError(t, err, "Case 135: 多个空字符串转换应成功")
	assert.Equal(t, "", model135.Level1.Title, "Case 135: 空Title应保持")
	assert.Equal(t, 0, len(model135.ListVal), "Case 135: 空ListVal长度应为0")

	// Case 136: 混合空和非空字符串
	pb136 := &PBNested{
		ID: 136,
		Level1: &Level1{
			Title: "title", // 非空
			Level2: &Level2{
				Name: "", // 空
				Level3: &Level3{
					Value: "value", // 非空
				},
			},
		},
	}
	model136 := &ModelNested{}
	err = converter.ConvertPBToModel(pb136, model136)
	assert.NoError(t, err, "Case 136: 混合空字符串转换应成功")
	assert.Equal(t, "title", model136.Level1.Title, "Case 136: 非空Title应保持")
	assert.Equal(t, "", model136.Level1.Level2.Name, "Case 136: 空Name应保持")

	// Case 137: Level1存在但所有子字段为零值
	pb137 := &PBNested{
		ID: 137,
		Level1: &Level1{
			Title:    "",
			FloatVal: 0,
			TimeVal:  nil,
			Level2: &Level2{
				Name:    "",
				IntVal:  0,
				BoolVal: false,
				Level3: &Level3{
					Value:    "",
					Number:   0,
					FloatVal: 0,
				},
			},
		},
	}
	model137 := &ModelNested{}
	err = converter.ConvertPBToModel(pb137, model137)
	assert.NoError(t, err, "Case 137: 全零值嵌套转换应成功")
	assert.NotNil(t, model137.Level1.Level2.Level3, "Case 137: Level3应存在但为零值")
	assert.Equal(t, int32(0), model137.Level1.Level2.Level3.Number, "Case 137: Number应为0")

	// Case 138: ListVal含多个相同元素
	pb138 := &PBNested{
		ID:      138,
		ListVal: []string{"same", "same", "same", "same"},
	}
	model138 := &ModelNested{}
	err = converter.ConvertPBToModel(pb138, model138)
	assert.NoError(t, err, "Case 138: 相同元素列表转换应成功")
	assert.Equal(t, 4, len(model138.ListVal), "Case 138: 列表长度应为4")
	assert.Equal(t, "same", model138.ListVal[0], "Case 138: 元素应相同")

	// Case 139: MapVal含重复值
	pb139 := &PBNested{
		ID: 139,
		MapVal: map[string]int32{
			"a": 100,
			"b": 100,
			"c": 100,
		},
	}
	model139 := &ModelNested{}
	err = converter.ConvertPBToModel(pb139, model139)
	assert.NoError(t, err, "Case 139: 重复值映射转换应成功")
	assert.Equal(t, int32(100), model139.MapVal["a"], "Case 139: 值应相等")
	assert.Equal(t, int32(100), model139.MapVal["b"], "Case 139: 值应相等")

	// Case 140: 混合所有类型的复杂场景
	pb140 := &PBNested{
		ID: 140,
		Level1: &Level1{
			Title:    "complex_mix",
			FloatVal: float32(math.Inf(1)),
			TimeVal:  timestamppb.Now(),
			Level2: &Level2{
				Name:    "你好世界",
				IntVal:  math.MaxInt64,
				BoolVal: false,
				Level3: &Level3{
					Value:    "🎉",
					Number:   math.MinInt32,
					FloatVal: math.NaN(),
				},
			},
		},
		ListVal: []string{"", "a", "", "😀", ""},
		MapVal: map[string]int32{
			"":     0,
			"zero": 0,
			"min":  math.MinInt32,
			"max":  math.MaxInt32,
		},
	}
	model140 := &ModelNested{}
	err = converter.ConvertPBToModel(pb140, model140)
	assert.NoError(t, err, "Case 140: 复杂混合转换应成功")
	assert.True(t, math.IsInf(float64(model140.Level1.FloatVal), 1), "Case 140: Inf应保持")
	assert.Equal(t, "你好世界", model140.Level1.Level2.Name, "Case 140: Unicode应保持")
	assert.True(t, math.IsNaN(model140.Level1.Level2.Level3.FloatVal), "Case 140: NaN应保持")

	// ========== Case 141-150: 边界和压力测试 ==========

	// Case 141: 最深嵌套+最大值
	pb141 := &PBNested{
		ID: 141,
		Level1: &Level1{
			Title:    "max_nested",
			FloatVal: float32(math.MaxFloat32),
			Level2: &Level2{
				IntVal:  math.MaxInt64,
				BoolVal: true,
				Level3: &Level3{
					Number:   math.MaxInt32,
					FloatVal: math.MaxFloat64,
				},
			},
		},
	}
	model141 := &ModelNested{}
	err = converter.ConvertPBToModel(pb141, model141)
	assert.NoError(t, err, "Case 141: 最深嵌套最大值转换应成功")
	assert.Greater(t, model141.Level1.Level2.Level3.Number, int32(0), "Case 141: 最大值应为正")

	// Case 142: 最深嵌套+最小值
	pb142 := &PBNested{
		ID: 142,
		Level1: &Level1{
			Title:    "min_nested",
			FloatVal: float32(math.SmallestNonzeroFloat32),
			Level2: &Level2{
				IntVal:  math.MinInt64,
				BoolVal: false,
				Level3: &Level3{
					Number:   math.MinInt32,
					FloatVal: -math.MaxFloat64,
				},
			},
		},
	}
	model142 := &ModelNested{}
	err = converter.ConvertPBToModel(pb142, model142)
	assert.NoError(t, err, "Case 142: 最深嵌套最小值转换应成功")
	assert.Less(t, model142.Level1.Level2.Level3.Number, int32(0), "Case 142: 最小值应为负")

	// Case 143: 大量列表元素
	hugeList := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		hugeList[i] = "huge_" + string(rune(i%100))
	}
	pb143 := &PBNested{
		ID:      143,
		ListVal: hugeList,
	}
	model143 := &ModelNested{}
	err = converter.ConvertPBToModel(pb143, model143)
	assert.NoError(t, err, "Case 143: 大量列表元素转换应成功")
	assert.Equal(t, 10000, len(model143.ListVal), "Case 143: 列表应有10000个元素")

	// Case 144: 大量映射元素
	hugeMap := make(map[string]int32)
	for i := 0; i < 1000; i++ {
		hugeMap["key_"+string(rune(i%10))] = int32(i)
	}
	pb144 := &PBNested{
		ID:     144,
		MapVal: hugeMap,
	}
	model144 := &ModelNested{}
	err = converter.ConvertPBToModel(pb144, model144)
	assert.NoError(t, err, "Case 144: 大量映射元素转换应成功")
	assert.Greater(t, len(model144.MapVal), 0, "Case 144: 映射应有数据")

	// Case 145: 深层嵌套+大列表
	pb145 := &PBNested{
		ID: 145,
		Level1: &Level1{
			Title: "deep_with_list",
			Level2: &Level2{
				Name: "nested",
				Level3: &Level3{
					Value: "deep",
				},
			},
		},
		ListVal: hugeList[:100], // 使用100个元素
	}
	model145 := &ModelNested{}
	err = converter.ConvertPBToModel(pb145, model145)
	assert.NoError(t, err, "Case 145: 深层嵌套加大列表转换应成功")
	assert.Equal(t, 100, len(model145.ListVal), "Case 145: 列表应有100个元素")

	// Case 146: 深层嵌套+大映射
	pb146 := &PBNested{
		ID: 146,
		Level1: &Level1{
			Title: "deep_with_map",
			Level2: &Level2{
				Name: "with_map",
				Level3: &Level3{
					Value: "mapped",
				},
			},
		},
		MapVal: hugeMap,
	}
	model146 := &ModelNested{}
	err = converter.ConvertPBToModel(pb146, model146)
	assert.NoError(t, err, "Case 146: 深层嵌套加大映射转换应成功")
	assert.Greater(t, len(model146.MapVal), 0, "Case 146: 映射应有数据")

	// Case 147: 所有nil引用
	pb147 := &PBNested{
		ID:      147,
		Level1:  nil,
		ListVal: nil,
		MapVal:  nil,
	}
	model147 := &ModelNested{}
	err = converter.ConvertPBToModel(pb147, model147)
	assert.NoError(t, err, "Case 147: 所有nil转换应成功")
	assert.Nil(t, model147.Level1, "Case 147: Level1应为nil")

	// Case 148: 所有空引用
	pb148 := &PBNested{
		ID: 148,
		Level1: &Level1{
			Level2: &Level2{
				Level3: &Level3{},
			},
		},
		ListVal: []string{},
		MapVal:  map[string]int32{},
	}
	model148 := &ModelNested{}
	err = converter.ConvertPBToModel(pb148, model148)
	assert.NoError(t, err, "Case 148: 所有空引用转换应成功")
	assert.NotNil(t, model148.Level1.Level2.Level3, "Case 148: Level3应存在")

	// Case 149: 往返+修改测试
	originalPB := &PBNested{
		ID: 149,
		Level1: &Level1{
			Title:    "original",
			FloatVal: 1.49,
			Level2: &Level2{
				Name:   "data",
				IntVal: 149149,
				Level3: &Level3{
					Value:  "value",
					Number: 149,
				},
			},
		},
	}
	tempModel := &ModelNested{}
	converter.ConvertPBToModel(originalPB, tempModel)
	tempModel.Level1.Title = "modified"
	tempModel.Level1.FloatVal = 9.49
	roundTripPB := &PBNested{}
	err = converter.ConvertModelToPB(tempModel, roundTripPB)
	assert.NoError(t, err, "Case 149: 往返修改转换应成功")
	assert.Equal(t, "modified", roundTripPB.Level1.Title, "Case 149: 修改应保持")

	// Case 150: 压力测试综合场景
	pb150 := &PBNested{
		ID: 150,
		Level1: &Level1{
			Title:    "压力测试 🎉 Stress Test",
			FloatVal: 1.50,
			TimeVal:  timestamppb.Now(),
			Level2: &Level2{
				Name:    "مرحبا 你好 🌍",
				IntVal:  math.MaxInt64,
				BoolVal: true,
				Level3: &Level3{
					Value:    "'; DROP TABLE; --\n\t",
					Number:   math.MinInt32,
					FloatVal: math.Inf(1),
				},
			},
		},
		ListVal: []string{"", "a", "你好", "🎉", "", "sql'; --"},
		MapVal: map[string]int32{
			"zero":    0,
			"":        -1,
			"Unicode": 150,
			"Max":     math.MaxInt32,
			"Min":     math.MinInt32,
		},
	}
	model150 := &ModelNested{}
	err = converter.ConvertPBToModel(pb150, model150)
	assert.NoError(t, err, "Case 150: 压力测试综合场景转换应成功")
	assert.Equal(t, int32(150), model150.ID, "Case 150: ID应相等")
	assert.NotNil(t, model150.Level1.Level2.Level3, "Case 150: 深层应存在")
	assert.Equal(t, 5, len(model150.MapVal), "Case 150: 映射大小应为5")
}
