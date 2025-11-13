/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 21:55:02
 * @FilePath: \go-rpc-gateway\pbmo\comprehensive_51_100_test.go
 * @Description: 综合场景测试 - 第二批50条 (51-100)
 * 复杂嵌套结构、数组、切片、Map等场景
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================================
// 第二批: 50条新场景测试 (51-100)
// 嵌套结构、数组、切片、Map、接口等复杂类型
// ============================================================================

// TestComprehensive51_100 包含复杂嵌套结构场景
func TestComprehensive51_100(t *testing.T) {
	// 定义复杂嵌套结构
	type Address struct {
		Street string
		City   string
		Zip    string
	}

	type Contact struct {
		Email  string
		Phone  string
		Mobile string
	}

	type Person struct {
		ID        int64
		Name      string
		Age       int32
		Address   *Address
		Contact   *Contact
		Active    bool
		Salary    float64
		CreatedAt *timestamppb.Timestamp
	}

	type PBAddress struct {
		Street string
		City   string
		Zip    string
	}

	type PBContact struct {
		Email  string
		Phone  string
		Mobile string
	}

	type PBPerson struct {
		ID        int64
		Name      string
		Age       int32
		Address   *PBAddress
		Contact   *PBContact
		Active    bool
		Salary    float64
		CreatedAt *timestamppb.Timestamp
	}

	personConverter := NewBidiConverter(&PBPerson{}, &Person{})

	// ========== Case 51-55: 简单嵌套结构 ==========

	// Case 51: 完整嵌套结构
	pb51 := &PBPerson{
		ID:   51,
		Name: "Alice",
		Age:  28,
		Address: &PBAddress{
			Street: "123 Main St",
			City:   "New York",
			Zip:    "10001",
		},
		Contact: &PBContact{
			Email:  "alice@example.com",
			Phone:  "212-555-0001",
			Mobile: "212-555-0101",
		},
		Active:    true,
		Salary:    75000.00,
		CreatedAt: timestamppb.Now(),
	}
	model51 := &Person{}
	err := personConverter.ConvertPBToModel(pb51, model51)
	assert.NoError(t, err, "Case 51: 完整嵌套结构转换应成功")
	assert.Equal(t, int64(51), model51.ID, "Case 51: ID应相等")
	assert.Equal(t, "Alice", model51.Name, "Case 51: Name应相等")
	assert.NotNil(t, model51.Address, "Case 51: Address不应为nil")
	assert.Equal(t, "123 Main St", model51.Address.Street, "Case 51: Address.Street应相等")
	assert.NotNil(t, model51.Contact, "Case 51: Contact不应为nil")
	assert.Equal(t, "alice@example.com", model51.Contact.Email, "Case 51: Contact.Email应相等")

	// Case 52: 仅部分嵌套字段填充
	pb52 := &PBPerson{
		ID:   52,
		Name: "Bob",
		Age:  35,
		Address: &PBAddress{
			Street: "456 Oak Ave",
			City:   "Los Angeles",
		},
		Contact: &PBContact{
			Email: "bob@example.com",
		},
		Active: true,
	}
	model52 := &Person{}
	err = personConverter.ConvertPBToModel(pb52, model52)
	assert.NoError(t, err, "Case 52: 部分嵌套字段转换应成功")
	assert.Equal(t, "456 Oak Ave", model52.Address.Street, "Case 52: Address.Street应相等")
	assert.Empty(t, model52.Address.Zip, "Case 52: Address.Zip应为空")
	assert.Equal(t, "bob@example.com", model52.Contact.Email, "Case 52: Contact.Email应相等")
	assert.Empty(t, model52.Contact.Phone, "Case 52: Contact.Phone应为空")

	// Case 53: nil嵌套结构
	pb53 := &PBPerson{
		ID:      53,
		Name:    "Charlie",
		Age:     42,
		Address: nil,
		Contact: nil,
		Active:  false,
	}
	model53 := &Person{}
	err = personConverter.ConvertPBToModel(pb53, model53)
	assert.NoError(t, err, "Case 53: nil嵌套结构转换应成功")
	assert.Nil(t, model53.Address, "Case 53: Address应为nil")
	assert.Nil(t, model53.Contact, "Case 53: Contact应为nil")
	assert.False(t, model53.Active, "Case 53: Active应为false")

	// Case 54: 空嵌套结构
	pb54 := &PBPerson{
		ID:   54,
		Name: "Diana",
		Address: &PBAddress{
			Street: "",
			City:   "",
			Zip:    "",
		},
		Contact: &PBContact{
			Email:  "",
			Phone:  "",
			Mobile: "",
		},
	}
	model54 := &Person{}
	err = personConverter.ConvertPBToModel(pb54, model54)
	assert.NoError(t, err, "Case 54: 空嵌套结构转换应成功")
	assert.NotNil(t, model54.Address, "Case 54: Address不应为nil")
	assert.Empty(t, model54.Address.Street, "Case 54: Address.Street应为空")
	assert.NotNil(t, model54.Contact, "Case 54: Contact不应为nil")
	assert.Empty(t, model54.Contact.Email, "Case 54: Contact.Email应为空")

	// Case 55: Unicode字符在嵌套字段中
	pb55 := &PBPerson{
		ID:   55,
		Name: "王小明",
		Age:  30,
		Address: &PBAddress{
			Street: "北京市朝阳区建国路1号",
			City:   "北京",
			Zip:    "100022",
		},
		Contact: &PBContact{
			Email:  "wangxiaoming@example.cn",
			Phone:  "010-1234-5678",
			Mobile: "138-1234-5678",
		},
		Active: true,
	}
	model55 := &Person{}
	err = personConverter.ConvertPBToModel(pb55, model55)
	assert.NoError(t, err, "Case 55: Unicode嵌套字段转换应成功")
	assert.Equal(t, "王小明", model55.Name, "Case 55: 中文Name应相等")
	assert.Equal(t, "北京市朝阳区建国路1号", model55.Address.Street, "Case 55: 中文Address应相等")

	// ========== Case 56-60: 数组场景 ==========

	type Company struct {
		ID        int32
		Name      string
		Employees []*Person
		Managers  []*Person
	}

	type PBCompany struct {
		ID        int32
		Name      string
		Employees []*PBPerson
		Managers  []*PBPerson
	}

	companyConverter := NewBidiConverter(&PBCompany{}, &Company{})

	// Case 56: 空员工数组
	pb56 := &PBCompany{
		ID:        56,
		Name:      "TechCorp",
		Employees: []*PBPerson{},
		Managers:  []*PBPerson{},
	}
	model56 := &Company{}
	err = companyConverter.ConvertPBToModel(pb56, model56)
	assert.NoError(t, err, "Case 56: 空数组转换应成功")
	assert.Empty(t, model56.Employees, "Case 56: Employees应为空")
	assert.Empty(t, model56.Managers, "Case 56: Managers应为空")

	// Case 57: 单个元素数组
	pb57 := &PBCompany{
		ID:   57,
		Name: "StartupXYZ",
		Employees: []*PBPerson{
			{
				ID:     1001,
				Name:   "Eve",
				Age:    25,
				Active: true,
			},
		},
		Managers: []*PBPerson{},
	}
	model57 := &Company{}
	err = companyConverter.ConvertPBToModel(pb57, model57)
	assert.NoError(t, err, "Case 57: 单元素数组转换应成功")
	assert.Len(t, model57.Employees, 1, "Case 57: Employees长度应为1")
	assert.Equal(t, "Eve", model57.Employees[0].Name, "Case 57: 第一个员工名字应相等")

	// Case 58: 多个元素数组
	pb58 := &PBCompany{
		ID:   58,
		Name: "BigCorp",
		Employees: []*PBPerson{
			{ID: 1001, Name: "Frank", Age: 30, Active: true},
			{ID: 1002, Name: "Grace", Age: 28, Active: true},
			{ID: 1003, Name: "Henry", Age: 35, Active: false},
			{ID: 1004, Name: "Ivy", Age: 26, Active: true},
			{ID: 1005, Name: "Jack", Age: 32, Active: true},
		},
		Managers: []*PBPerson{
			{ID: 2001, Name: "Karen", Age: 45, Active: true},
			{ID: 2002, Name: "Liam", Age: 50, Active: true},
		},
	}
	model58 := &Company{}
	err = companyConverter.ConvertPBToModel(pb58, model58)
	assert.NoError(t, err, "Case 58: 多元素数组转换应成功")
	assert.Len(t, model58.Employees, 5, "Case 58: Employees长度应为5")
	assert.Len(t, model58.Managers, 2, "Case 58: Managers长度应为2")
	assert.Equal(t, "Frank", model58.Employees[0].Name, "Case 58: 第一个员工应为Frank")
	assert.Equal(t, "Karen", model58.Managers[0].Name, "Case 58: 第一个经理应为Karen")

	// Case 59: 大数组（100个元素）
	pb59 := &PBCompany{
		ID:        59,
		Name:      "MassiveCorp",
		Employees: make([]*PBPerson, 100),
	}
	for i := 0; i < 100; i++ {
		pb59.Employees[i] = &PBPerson{
			ID:     int64(1000 + i),
			Name:   "Employee" + string(rune(i)),
			Age:    int32(20 + (i % 45)),
			Active: i%2 == 0,
		}
	}
	model59 := &Company{}
	err = companyConverter.ConvertPBToModel(pb59, model59)
	assert.NoError(t, err, "Case 59: 大数组转换应成功")
	assert.Len(t, model59.Employees, 100, "Case 59: Employees长度应为100")
	assert.Equal(t, int64(1050), model59.Employees[50].ID, "Case 59: 第50个员工ID应为1050")

	// Case 60: nil数组
	pb60 := &PBCompany{
		ID:        60,
		Name:      "NilCorp",
		Employees: nil,
		Managers:  nil,
	}
	model60 := &Company{}
	err = companyConverter.ConvertPBToModel(pb60, model60)
	assert.NoError(t, err, "Case 60: nil数组转换应成功")
	assert.Nil(t, model60.Employees, "Case 60: Employees应为nil")

	// ========== Case 61-65: 切片场景 ==========

	type Department struct {
		ID       int32
		Name     string
		Tags     []string
		Codes    []int32
		Budgets  []float64
		Flags    []bool
		Metadata map[string]string
	}

	type PBDepartment struct {
		ID       int32
		Name     string
		Tags     []string
		Codes    []int32
		Budgets  []float64
		Flags    []bool
		Metadata map[string]string
	}

	deptConverter := NewBidiConverter(&PBDepartment{}, &Department{})

	// Case 61: 空切片
	pb61 := &PBDepartment{
		ID:      61,
		Name:    "HR",
		Tags:    []string{},
		Codes:   []int32{},
		Budgets: []float64{},
		Flags:   []bool{},
	}
	model61 := &Department{}
	err = deptConverter.ConvertPBToModel(pb61, model61)
	assert.NoError(t, err, "Case 61: 空切片转换应成功")
	assert.Empty(t, model61.Tags, "Case 61: Tags应为空")
	assert.Empty(t, model61.Codes, "Case 61: Codes应为空")

	// Case 62: 字符串切片
	pb62 := &PBDepartment{
		ID:   62,
		Name: "Engineering",
		Tags: []string{"backend", "frontend", "devops", "ml", "qa"},
	}
	model62 := &Department{}
	err = deptConverter.ConvertPBToModel(pb62, model62)
	assert.NoError(t, err, "Case 62: 字符串切片转换应成功")
	assert.Len(t, model62.Tags, 5, "Case 62: Tags长度应为5")
	assert.Equal(t, "backend", model62.Tags[0], "Case 62: 第一个tag应为backend")
	assert.Equal(t, "qa", model62.Tags[4], "Case 62: 最后一个tag应为qa")

	// Case 63: 数值切片（整数）
	pb63 := &PBDepartment{
		ID:    63,
		Name:  "Finance",
		Codes: []int32{100, 200, 300, -100, -200, 0, 999999},
	}
	model63 := &Department{}
	err = deptConverter.ConvertPBToModel(pb63, model63)
	assert.NoError(t, err, "Case 63: 整数切片转换应成功")
	assert.Len(t, model63.Codes, 7, "Case 63: Codes长度应为7")
	assert.Equal(t, int32(100), model63.Codes[0], "Case 63: 第一个code应为100")
	assert.Equal(t, int32(999999), model63.Codes[6], "Case 63: 最后一个code应为999999")

	// Case 64: 浮点数切片
	pb64 := &PBDepartment{
		ID:      64,
		Name:    "Operations",
		Budgets: []float64{10000.50, 25000.75, 50000.00, -5000.25, 0.01},
	}
	model64 := &Department{}
	err = deptConverter.ConvertPBToModel(pb64, model64)
	assert.NoError(t, err, "Case 64: 浮点数切片转换应成功")
	assert.Len(t, model64.Budgets, 5, "Case 64: Budgets长度应为5")
	assert.InDelta(t, 10000.50, model64.Budgets[0], 0.01, "Case 64: 第一个budget应接近")
	assert.InDelta(t, -5000.25, model64.Budgets[3], 0.01, "Case 64: 负数budget应接近")

	// Case 65: 布尔值切片
	pb65 := &PBDepartment{
		ID:    65,
		Name:  "Marketing",
		Flags: []bool{true, false, true, true, false, false, true},
	}
	model65 := &Department{}
	err = deptConverter.ConvertPBToModel(pb65, model65)
	assert.NoError(t, err, "Case 65: 布尔值切片转换应成功")
	assert.Len(t, model65.Flags, 7, "Case 65: Flags长度应为7")
	assert.True(t, model65.Flags[0], "Case 65: 第一个flag应为true")
	assert.False(t, model65.Flags[1], "Case 65: 第二个flag应为false")

	// ========== Case 66-70: 时间戳复杂场景 ==========

	type Timeline struct {
		ID        int32
		EventName string
		Events    []*timestamppb.Timestamp
		StartTime *timestamppb.Timestamp
		EndTime   *timestamppb.Timestamp
	}

	type PBTimeline struct {
		ID        int32
		EventName string
		Events    []*timestamppb.Timestamp
		StartTime *timestamppb.Timestamp
		EndTime   *timestamppb.Timestamp
	}

	timelineConverter := NewBidiConverter(&PBTimeline{}, &Timeline{})

	// Case 66: 时间戳数组
	now := time.Now()
	pb66 := &PBTimeline{
		ID:        66,
		EventName: "ProjectTimeline",
		Events: []*timestamppb.Timestamp{
			timestamppb.New(now.Add(-7 * 24 * time.Hour)),
			timestamppb.New(now),
			timestamppb.New(now.Add(7 * 24 * time.Hour)),
			timestamppb.New(now.Add(14 * 24 * time.Hour)),
		},
		StartTime: timestamppb.New(now.Add(-30 * 24 * time.Hour)),
		EndTime:   timestamppb.New(now.Add(90 * 24 * time.Hour)),
	}
	model66 := &Timeline{}
	err = timelineConverter.ConvertPBToModel(pb66, model66)
	assert.NoError(t, err, "Case 66: 时间戳数组转换应成功")
	assert.Len(t, model66.Events, 4, "Case 66: Events长度应为4")
	assert.NotNil(t, model66.StartTime, "Case 66: StartTime不应为nil")
	assert.NotNil(t, model66.EndTime, "Case 66: EndTime不应为nil")

	// Case 67: 单个时间戳
	pb67 := &PBTimeline{
		ID:        67,
		EventName: "Deadline",
		Events: []*timestamppb.Timestamp{
			timestamppb.Now(),
		},
		StartTime: timestamppb.Now(),
	}
	model67 := &Timeline{}
	err = timelineConverter.ConvertPBToModel(pb67, model67)
	assert.NoError(t, err, "Case 67: 单个时间戳转换应成功")
	assert.Len(t, model67.Events, 1, "Case 67: Events长度应为1")

	// Case 68: 空时间戳数组
	pb68 := &PBTimeline{
		ID:        68,
		EventName: "EmptyTimeline",
		Events:    []*timestamppb.Timestamp{},
	}
	model68 := &Timeline{}
	err = timelineConverter.ConvertPBToModel(pb68, model68)
	assert.NoError(t, err, "Case 68: 空时间戳数组转换应成功")
	assert.Empty(t, model68.Events, "Case 68: Events应为空")

	// Case 69: nil时间戳
	pb69 := &PBTimeline{
		ID:        69,
		EventName: "NoTime",
		Events:    nil,
		StartTime: nil,
		EndTime:   nil,
	}
	model69 := &Timeline{}
	err = timelineConverter.ConvertPBToModel(pb69, model69)
	assert.NoError(t, err, "Case 69: nil时间戳转换应成功")
	assert.Nil(t, model69.Events, "Case 69: Events应为nil")
	assert.Nil(t, model69.StartTime, "Case 69: StartTime应为nil")

	// Case 70: 混合nil和非nil时间戳
	pb70 := &PBTimeline{
		ID:        70,
		EventName: "MixedTime",
		Events: []*timestamppb.Timestamp{
			timestamppb.Now(),
			nil,
			timestamppb.Now(),
		},
		StartTime: timestamppb.Now(),
		EndTime:   nil,
	}
	model70 := &Timeline{}
	err = timelineConverter.ConvertPBToModel(pb70, model70)
	assert.NoError(t, err, "Case 70: 混合时间戳转换应成功")
	assert.Len(t, model70.Events, 3, "Case 70: Events长度应为3")

	// ========== Case 71-75: Map/字典场景 ==========

	type Configuration struct {
		ID    int32
		Name  string
		Props map[string]string
		Vals  map[string]int32
		Rates map[string]float64
	}

	type PBConfiguration struct {
		ID    int32
		Name  string
		Props map[string]string
		Vals  map[string]int32
		Rates map[string]float64
	}

	configConverter := NewBidiConverter(&PBConfiguration{}, &Configuration{})

	// Case 71: 空Map
	pb71 := &PBConfiguration{
		ID:    71,
		Name:  "EmptyConfig",
		Props: map[string]string{},
		Vals:  map[string]int32{},
		Rates: map[string]float64{},
	}
	model71 := &Configuration{}
	err = configConverter.ConvertPBToModel(pb71, model71)
	assert.NoError(t, err, "Case 71: 空Map转换应成功")
	assert.Empty(t, model71.Props, "Case 71: Props应为空")

	// Case 72: 字符串Map
	pb72 := &PBConfiguration{
		ID:   72,
		Name: "StringConfig",
		Props: map[string]string{
			"theme":     "dark",
			"language":  "en-US",
			"timezone":  "UTC",
			"encoding":  "UTF-8",
			"delimiter": ",",
		},
	}
	model72 := &Configuration{}
	err = configConverter.ConvertPBToModel(pb72, model72)
	assert.NoError(t, err, "Case 72: 字符串Map转换应成功")
	assert.Len(t, model72.Props, 5, "Case 72: Props长度应为5")
	assert.Equal(t, "dark", model72.Props["theme"], "Case 72: theme应为dark")
	assert.Equal(t, "UTF-8", model72.Props["encoding"], "Case 72: encoding应为UTF-8")

	// Case 73: 整数Map
	pb73 := &PBConfiguration{
		ID:   73,
		Name: "IntConfig",
		Vals: map[string]int32{
			"max_connections": 1000,
			"timeout":         30,
			"max_retries":     3,
			"buffer_size":     65536,
			"thread_count":    -1,
			"port":            8080,
		},
	}
	model73 := &Configuration{}
	err = configConverter.ConvertPBToModel(pb73, model73)
	assert.NoError(t, err, "Case 73: 整数Map转换应成功")
	assert.Len(t, model73.Vals, 6, "Case 73: Vals长度应为6")
	assert.Equal(t, int32(1000), model73.Vals["max_connections"], "Case 73: max_connections应为1000")
	assert.Equal(t, int32(-1), model73.Vals["thread_count"], "Case 73: 负数应保持")

	// Case 74: 浮点数Map
	pb74 := &PBConfiguration{
		ID:   74,
		Name: "FloatConfig",
		Rates: map[string]float64{
			"cpu_threshold":    0.85,
			"memory_threshold": 0.90,
			"disk_threshold":   0.95,
			"interest_rate":    0.035,
			"tax_rate":         0.25,
		},
	}
	model74 := &Configuration{}
	err = configConverter.ConvertPBToModel(pb74, model74)
	assert.NoError(t, err, "Case 74: 浮点数Map转换应成功")
	assert.Len(t, model74.Rates, 5, "Case 74: Rates长度应为5")
	assert.InDelta(t, 0.85, model74.Rates["cpu_threshold"], 0.001, "Case 74: cpu_threshold应接近")

	// Case 75: nil Map
	pb75 := &PBConfiguration{
		ID:    75,
		Name:  "NilConfig",
		Props: nil,
		Vals:  nil,
		Rates: nil,
	}
	model75 := &Configuration{}
	err = configConverter.ConvertPBToModel(pb75, model75)
	assert.NoError(t, err, "Case 75: nil Map转换应成功")
	assert.Nil(t, model75.Props, "Case 75: Props应为nil")

	// ========== Case 76-80: 多层嵌套结构 ==========

	type Project struct {
		ID          int32
		Name        string
		Manager     *Person
		Team        []*Person
		Budget      float64
		Location    *Address
		CreatedTime *timestamppb.Timestamp
	}

	type PBProject struct {
		ID          int32
		Name        string
		Manager     *PBPerson
		Team        []*PBPerson
		Budget      float64
		Location    *PBAddress
		CreatedTime *timestamppb.Timestamp
	}

	projectConverter := NewBidiConverter(&PBProject{}, &Project{})

	// Case 76: 完整多层嵌套
	pb76 := &PBProject{
		ID:     76,
		Name:   "AI Platform",
		Budget: 5000000.00,
		Manager: &PBPerson{
			ID:   100,
			Name: "Maria",
			Age:  40,
			Contact: &PBContact{
				Email: "maria@company.com",
			},
		},
		Team: []*PBPerson{
			{
				ID:   101,
				Name: "TeamMember1",
				Age:  30,
				Contact: &PBContact{
					Email: "member1@company.com",
				},
			},
			{
				ID:   102,
				Name: "TeamMember2",
				Age:  28,
				Contact: &PBContact{
					Email: "member2@company.com",
				},
			},
		},
		Location: &PBAddress{
			Street: "100 Tech Park",
			City:   "San Francisco",
			Zip:    "94105",
		},
		CreatedTime: timestamppb.Now(),
	}
	model76 := &Project{}
	err = projectConverter.ConvertPBToModel(pb76, model76)
	assert.NoError(t, err, "Case 76: 多层嵌套转换应成功")
	assert.Equal(t, "AI Platform", model76.Name, "Case 76: 项目名应相等")
	assert.NotNil(t, model76.Manager, "Case 76: Manager不应为nil")
	assert.Equal(t, "Maria", model76.Manager.Name, "Case 76: Manager名字应相等")
	assert.Len(t, model76.Team, 2, "Case 76: Team长度应为2")
	assert.Equal(t, "San Francisco", model76.Location.City, "Case 76: 城市应相等")

	// Case 77: 部分nil嵌套
	pb77 := &PBProject{
		ID:     77,
		Name:   "DataPipeline",
		Budget: 2000000.00,
		Manager: &PBPerson{
			ID:   200,
			Name: "Nathan",
			Age:  35,
		},
		Team:     nil,
		Location: nil,
	}
	model77 := &Project{}
	err = projectConverter.ConvertPBToModel(pb77, model77)
	assert.NoError(t, err, "Case 77: 部分nil嵌套转换应成功")
	assert.NotNil(t, model77.Manager, "Case 77: Manager不应为nil")
	assert.Nil(t, model77.Team, "Case 77: Team应为nil")
	assert.Nil(t, model77.Location, "Case 77: Location应为nil")

	// Case 78: 空Team数组
	pb78 := &PBProject{
		ID:     78,
		Name:   "Research",
		Budget: 1000000.00,
		Manager: &PBPerson{
			ID:   300,
			Name: "Olivia",
			Age:  38,
		},
		Team: []*PBPerson{},
		Location: &PBAddress{
			Street: "Lab Building",
			City:   "Boston",
		},
	}
	model78 := &Project{}
	err = projectConverter.ConvertPBToModel(pb78, model78)
	assert.NoError(t, err, "Case 78: 空Team数组转换应成功")
	assert.Empty(t, model78.Team, "Case 78: Team应为空数组")

	// Case 79: 大Team数组（20人）
	pb79 := &PBProject{
		ID:     79,
		Name:   "CloudInfra",
		Budget: 10000000.00,
		Manager: &PBPerson{
			ID:   400,
			Name: "Patricia",
			Age:  45,
		},
		Team: make([]*PBPerson, 20),
	}
	for i := 0; i < 20; i++ {
		pb79.Team[i] = &PBPerson{
			ID:     int64(500 + i),
			Name:   "Member" + string(rune(i)),
			Age:    int32(25 + i),
			Active: true,
		}
	}
	model79 := &Project{}
	err = projectConverter.ConvertPBToModel(pb79, model79)
	assert.NoError(t, err, "Case 79: 大Team数组转换应成功")
	assert.Len(t, model79.Team, 20, "Case 79: Team长度应为20")

	// Case 80: 往返转换多层嵌套
	originalProject := &Project{
		ID:   80,
		Name: "TestProject",
		Manager: &Person{
			ID:   500,
			Name: "Quinn",
			Age:  32,
			Contact: &Contact{
				Email: "quinn@example.com",
			},
		},
		Team: []*Person{
			{ID: 501, Name: "Alice", Age: 26},
		},
		Location: &Address{
			Street: "500 Main",
			City:   "Seattle",
		},
		Budget: 3500000,
	}
	pbProject := &PBProject{}
	err = projectConverter.ConvertModelToPB(originalProject, pbProject)
	assert.NoError(t, err, "Case 80: 多层嵌套反向转换应成功")
	assert.Equal(t, "TestProject", pbProject.Name, "Case 80: 反向Name应相等")
	assert.Equal(t, "Quinn", pbProject.Manager.Name, "Case 80: 反向Manager名字应相等")

	// ========== Case 81-85: 特殊字符和编码 ==========

	type Document struct {
		ID       int32
		Title    string
		Content  string
		Tags     []string
		Encoding string
	}

	type PBDocument struct {
		ID       int32
		Title    string
		Content  string
		Tags     []string
		Encoding string
	}

	docConverter := NewBidiConverter(&PBDocument{}, &Document{})

	// Case 81: SQL注入风格内容
	pb81 := &PBDocument{
		ID:       81,
		Title:    "Database Test",
		Content:  "'; DROP TABLE users; -- SELECT * FROM admin WHERE 1=1",
		Tags:     []string{"security", "injection", "sql"},
		Encoding: "utf-8",
	}
	model81 := &Document{}
	err = docConverter.ConvertPBToModel(pb81, model81)
	assert.NoError(t, err, "Case 81: SQL风格内容转换应成功")
	assert.Equal(t, pb81.Content, model81.Content, "Case 81: Content应完全相等")

	// Case 82: XSS风格内容
	pb82 := &PBDocument{
		ID:       82,
		Title:    "XSS Test",
		Content:  "<script>alert('XSS')</script><img src=x onerror=alert('XSS')>",
		Tags:     []string{"security", "xss"},
		Encoding: "utf-8",
	}
	model82 := &Document{}
	err = docConverter.ConvertPBToModel(pb82, model82)
	assert.NoError(t, err, "Case 82: XSS风格内容转换应成功")
	assert.Equal(t, pb82.Content, model82.Content, "Case 82: Content应完全相等")

	// Case 83: JSON格式内容
	pb83 := &PBDocument{
		ID:       83,
		Title:    "JSON Config",
		Content:  `{"key": "value", "number": 123, "nested": {"bool": true}}`,
		Tags:     []string{"json", "config"},
		Encoding: "utf-8",
	}
	model83 := &Document{}
	err = docConverter.ConvertPBToModel(pb83, model83)
	assert.NoError(t, err, "Case 83: JSON内容转换应成功")
	assert.Equal(t, pb83.Content, model83.Content, "Case 83: JSON Content应相等")

	// Case 84: 正则表达式内容
	pb84 := &PBDocument{
		ID:       84,
		Title:    "Regex Pattern",
		Content:  `^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`,
		Tags:     []string{"regex", "email"},
		Encoding: "utf-8",
	}
	model84 := &Document{}
	err = docConverter.ConvertPBToModel(pb84, model84)
	assert.NoError(t, err, "Case 84: 正则内容转换应成功")
	assert.Equal(t, pb84.Content, model84.Content, "Case 84: 正则Content应相等")

	// Case 85: 混合编码测试
	pb85 := &PBDocument{
		ID:       85,
		Title:    "多语言内容",
		Content:  "English 中文 日本語 한국어 العربية עברית",
		Tags:     []string{"multilingual", "utf-8", "i18n"},
		Encoding: "utf-8",
	}
	model85 := &Document{}
	err = docConverter.ConvertPBToModel(pb85, model85)
	assert.NoError(t, err, "Case 85: 多语言内容转换应成功")
	assert.Equal(t, pb85.Content, model85.Content, "Case 85: 多语言Content应相等")

	// ========== Case 86-90: 随机复杂组合 ==========

	// Case 86: 随机复杂1
	pb86 := &PBCompany{
		ID:   86,
		Name: "RandomCorp1",
		Employees: []*PBPerson{
			{ID: 8601, Name: "R1", Age: 25, Active: true},
			{ID: 8602, Name: "R2", Age: 30, Active: false},
		},
	}
	model86 := &Company{}
	err = companyConverter.ConvertPBToModel(pb86, model86)
	assert.NoError(t, err, "Case 86: 随机复杂1转换应成功")

	// Case 87: 随机复杂2
	pb87 := &PBTimeline{
		ID:        87,
		EventName: "Random2",
		Events: []*timestamppb.Timestamp{
			timestamppb.Now(),
			timestamppb.Now(),
		},
	}
	model87 := &Timeline{}
	err = timelineConverter.ConvertPBToModel(pb87, model87)
	assert.NoError(t, err, "Case 87: 随机复杂2转换应成功")

	// Case 88: 随机复杂3
	pb88 := &PBConfiguration{
		ID:   88,
		Name: "Random3",
		Props: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}
	model88 := &Configuration{}
	err = configConverter.ConvertPBToModel(pb88, model88)
	assert.NoError(t, err, "Case 88: 随机复杂3转换应成功")

	// Case 89: 随机复杂4
	pb89 := &PBDepartment{
		ID:   89,
		Name: "Random4",
		Tags: []string{"tag1", "tag2", "tag3"},
	}
	model89 := &Department{}
	err = deptConverter.ConvertPBToModel(pb89, model89)
	assert.NoError(t, err, "Case 89: 随机复杂4转换应成功")

	// Case 90: 随机复杂5
	pb90 := &PBDocument{
		ID:      90,
		Title:   "Random5",
		Content: "Some content",
		Tags:    []string{"random", "content"},
	}
	model90 := &Document{}
	err = docConverter.ConvertPBToModel(pb90, model90)
	assert.NoError(t, err, "Case 90: 随机复杂5转换应成功")

	// ========== Case 91-100: 综合压力测试 ==========

	// Case 91-95: 边界值组合
	for i := 91; i <= 95; i++ {
		pb := &PBCompany{
			ID:        int32(i),
			Name:      "BoundaryTest" + string(rune(i)),
			Employees: make([]*PBPerson, i-90),
		}
		model := &Company{}
		err := companyConverter.ConvertPBToModel(pb, model)
		assert.NoError(t, err, "Case %d: 边界转换应成功", i)
	}

	// Case 96-100: 最终综合测试
	for i := 96; i <= 100; i++ {
		pb := &PBProject{
			ID:     int32(i),
			Name:   "FinalTest" + string(rune(i)),
			Budget: float64(i) * 1000000,
			Manager: &PBPerson{
				ID:   int64(i * 1000),
				Name: "Manager" + string(rune(i)),
				Age:  int32(30 + (i % 20)),
			},
		}
		model := &Project{}
		err := projectConverter.ConvertPBToModel(pb, model)
		assert.NoError(t, err, "Case %d: 最终综合转换应成功", i)
		assert.Equal(t, pb.Name, model.Name, "Case %d: Name应相等", i)
	}
}
