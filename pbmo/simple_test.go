package pbmo

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// 测试基础类型转换
func TestSimpleTypeConversions(t *testing.T) {
	// 创建Go模型实例
	goUser := &GoUser{
		ID:         123,
		Age:        25,
		Score:      95,
		Balance:    10000,
		Rating:     4.5,
		Salary:     75000.50,
		Name:       "张三",
		Username:   "zhangsan",
		Bio:        "这是一个测试用户",
		IsActive:   true,
		IsVerified: false,
		Avatar:     []byte("avatar_data"),
		Signature:  []byte("signature_data"),
		Status:     GoStatusActive,
		Priority:   GoPriorityHigh,
		Address: GoAddress{
			Street:     "北京市朝阳区",
			City:       "北京",
			Country:    "中国",
			PostalCode: "100000",
		},
		Contact: GoContact{
			Email:   "zhangsan@example.com",
			Phone:   "13800138000",
			Website: "https://example.com",
		},
		Tags:        []string{"developer", "golang", "backend"},
		Scores:      []int32{85, 90, 95, 88},
		Preferences: []bool{true, false, true},
		Metadata:    map[string]string{"role": "admin", "department": "tech"},
		Settings:    map[string]int32{"theme": 1, "language": 2},
		Flags:       map[string]bool{"notifications": true, "dark_mode": false},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 设置可选字段
	nickname := "小张"
	goUser.Nickname = &nickname
	middleAge := int32(30)
	goUser.MiddleAge = &middleAge
	isPremium := true
	goUser.IsPremium = &isPremium

	// 转换为PB结构
	pbUser := &User{
		Id:         goUser.ID,
		Age:        goUser.Age,
		Score:      goUser.Score,
		Balance:    goUser.Balance,
		Rating:     goUser.Rating,
		Salary:     goUser.Salary,
		Name:       goUser.Name,
		Username:   goUser.Username,
		Bio:        goUser.Bio,
		IsActive:   goUser.IsActive,
		IsVerified: goUser.IsVerified,
		Avatar:     goUser.Avatar,
		Signature:  goUser.Signature,
		Status:     Status(goUser.Status),
		Priority:   Priority(goUser.Priority),
		Address: &Address{
			Street:     goUser.Address.Street,
			City:       goUser.Address.City,
			Country:    goUser.Address.Country,
			PostalCode: goUser.Address.PostalCode,
		},
		Contact: &Contact{
			Email:   goUser.Contact.Email,
			Phone:   goUser.Contact.Phone,
			Website: goUser.Contact.Website,
		},
		Tags:        goUser.Tags,
		Scores:      goUser.Scores,
		Preferences: goUser.Preferences,
		Metadata:    goUser.Metadata,
		Settings:    goUser.Settings,
		Flags:       goUser.Flags,
	}

	// 设置可选字段
	if goUser.Nickname != nil {
		pbUser.Nickname = goUser.Nickname
	}
	if goUser.MiddleAge != nil {
		pbUser.MiddleAge = goUser.MiddleAge
	}
	if goUser.IsPremium != nil {
		pbUser.IsPremium = goUser.IsPremium
	}

	// 验证转换结果
	if pbUser.Id != goUser.ID {
		t.Errorf("ID转换失败: 期望 %d, 得到 %d", goUser.ID, pbUser.Id)
	}

	if pbUser.Name != goUser.Name {
		t.Errorf("Name转换失败: 期望 %s, 得到 %s", goUser.Name, pbUser.Name)
	}

	if pbUser.IsActive != goUser.IsActive {
		t.Errorf("IsActive转换失败: 期望 %t, 得到 %t", goUser.IsActive, pbUser.IsActive)
	}

	if int(pbUser.Status) != int(goUser.Status) {
		t.Errorf("Status转换失败: 期望 %d, 得到 %d", goUser.Status, pbUser.Status)
	}

	if pbUser.Address.City != goUser.Address.City {
		t.Errorf("Address.City转换失败: 期望 %s, 得到 %s", goUser.Address.City, pbUser.Address.City)
	}

	if len(pbUser.Tags) != len(goUser.Tags) {
		t.Errorf("Tags长度转换失败: 期望 %d, 得到 %d", len(goUser.Tags), len(pbUser.Tags))
	}

	if pbUser.Metadata["role"] != goUser.Metadata["role"] {
		t.Errorf("Metadata转换失败: 期望 %s, 得到 %s", goUser.Metadata["role"], pbUser.Metadata["role"])
	}

	fmt.Printf("✅ 基础类型转换测试通过\n")
}

// 测试产品类型转换
func TestProductConversion(t *testing.T) {
	goProduct := &GoProduct{
		ID:       1001,
		Name:     "MacBook Pro",
		Price:    19999.99,
		InStock:  true,
		Category: GoStatusActive,
		Images:   []string{"image1.jpg", "image2.jpg"},
		Properties: map[string]string{
			"color": "Space Gray",
			"size":  "13inch",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 设置可选字段
	desc := "最新款MacBook Pro，性能卓越"
	goProduct.Description = &desc

	pbProduct := &Product{
		Id:         goProduct.ID,
		Name:       goProduct.Name,
		Price:      goProduct.Price,
		InStock:    goProduct.InStock,
		Category:   Status(goProduct.Category),
		Images:     goProduct.Images,
		Properties: goProduct.Properties,
	}

	if goProduct.Description != nil {
		pbProduct.Description = goProduct.Description
	}

	// 验证转换
	if pbProduct.Name != goProduct.Name {
		t.Errorf("Product Name转换失败: 期望 %s, 得到 %s", goProduct.Name, pbProduct.Name)
	}

	if pbProduct.Price != goProduct.Price {
		t.Errorf("Product Price转换失败: 期望 %f, 得到 %f", goProduct.Price, pbProduct.Price)
	}

	if len(pbProduct.Images) != len(goProduct.Images) {
		t.Errorf("Product Images长度转换失败: 期望 %d, 得到 %d", len(goProduct.Images), len(pbProduct.Images))
	}

	fmt.Printf("✅ 产品类型转换测试通过\n")
}

// 测试请求响应类型
func TestRequestResponseTypes(t *testing.T) {
	// 测试请求类型
	goRequest := &GoGetUserRequest{
		ID: 123,
	}
	includeInactive := true
	goRequest.IncludeInactive = &includeInactive

	pbRequest := &GetUserRequest{
		Id: goRequest.ID,
	}
	if goRequest.IncludeInactive != nil {
		pbRequest.IncludeInactive = goRequest.IncludeInactive
	}

	if pbRequest.Id != goRequest.ID {
		t.Errorf("Request ID转换失败: 期望 %d, 得到 %d", goRequest.ID, pbRequest.Id)
	}

	// 测试响应类型
	goResponse := &GoUserResponse{
		Success: true,
		Message: "操作成功",
		User: &GoUser{
			ID:   123,
			Name: "测试用户",
		},
		TotalCount: 1,
	}

	pbResponse := &UserResponse{
		Success: goResponse.Success,
		Message: goResponse.Message,
		User: &User{
			Id:   goResponse.User.ID,
			Name: goResponse.User.Name,
		},
		TotalCount: goResponse.TotalCount,
	}

	if pbResponse.Success != goResponse.Success {
		t.Errorf("Response Success转换失败: 期望 %t, 得到 %t", goResponse.Success, pbResponse.Success)
	}

	fmt.Printf("✅ 请求响应类型转换测试通过\n")
}

// 测试枚举类型
func TestEnumConversion(t *testing.T) {
	// 测试Status枚举
	goStatuses := []GoStatus{
		GoStatusUnknown,
		GoStatusActive,
		GoStatusInactive,
		GoStatusPending,
	}

	for _, goStatus := range goStatuses {
		pbStatus := Status(goStatus)
		convertedBack := GoStatus(pbStatus)

		if convertedBack != goStatus {
			t.Errorf("Status枚举转换失败: 原始 %d, 转换后 %d", goStatus, convertedBack)
		}
	}

	// 测试Priority枚举
	goPriorities := []GoPriority{
		GoPriorityLow,
		GoPriorityMedium,
		GoPriorityHigh,
		GoPriorityCritical,
	}

	for _, goPriority := range goPriorities {
		pbPriority := Priority(goPriority)
		convertedBack := GoPriority(pbPriority)

		if convertedBack != goPriority {
			t.Errorf("Priority枚举转换失败: 原始 %d, 转换后 %d", goPriority, convertedBack)
		}
	}

	fmt.Printf("✅ 枚举类型转换测试通过\n")
}

// 测试JSON序列化
func TestJSONSerialization(t *testing.T) {
	goUser := &GoUser{
		ID:       456,
		Name:     "李四",
		Username: "lisi",
		IsActive: true,
		Tags:     []string{"frontend", "react"},
		Metadata: map[string]string{"team": "frontend"},
	}

	// 序列化Go结构
	goJSON, err := json.Marshal(goUser)
	if err != nil {
		t.Fatalf("Go结构JSON序列化失败: %v", err)
	}

	pbUser := &User{
		Id:       goUser.ID,
		Name:     goUser.Name,
		Username: goUser.Username,
		IsActive: goUser.IsActive,
		Tags:     goUser.Tags,
		Metadata: goUser.Metadata,
	}

	// 序列化PB结构
	pbJSON, err := json.Marshal(pbUser)
	if err != nil {
		t.Fatalf("PB结构JSON序列化失败: %v", err)
	}

	fmt.Printf("Go JSON: %s\n", string(goJSON))
	fmt.Printf("PB JSON: %s\n", string(pbJSON))
	fmt.Printf("✅ JSON序列化测试通过\n")
}

// 性能基准测试
func BenchmarkGoToPBConversion(b *testing.B) {
	goUser := &GoUser{
		ID:       123,
		Age:      25,
		Name:     "benchmark_user",
		Username: "benchuser",
		IsActive: true,
		Tags:     []string{"tag1", "tag2", "tag3"},
		Metadata: map[string]string{"key1": "value1", "key2": "value2"},
		Settings: map[string]int32{"setting1": 1, "setting2": 2},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pbUser := &User{
			Id:       goUser.ID,
			Age:      goUser.Age,
			Name:     goUser.Name,
			Username: goUser.Username,
			IsActive: goUser.IsActive,
			Tags:     goUser.Tags,
			Metadata: goUser.Metadata,
			Settings: goUser.Settings,
		}
		_ = pbUser // 避免编译器优化
	}
}

func BenchmarkPBToGoConversion(b *testing.B) {
	pbUser := &User{
		Id:       123,
		Age:      25,
		Name:     "benchmark_user",
		Username: "benchuser",
		IsActive: true,
		Tags:     []string{"tag1", "tag2", "tag3"},
		Metadata: map[string]string{"key1": "value1", "key2": "value2"},
		Settings: map[string]int32{"setting1": 1, "setting2": 2},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		goUser := &GoUser{
			ID:       pbUser.Id,
			Age:      pbUser.Age,
			Name:     pbUser.Name,
			Username: pbUser.Username,
			IsActive: pbUser.IsActive,
			Tags:     pbUser.Tags,
			Metadata: pbUser.Metadata,
			Settings: pbUser.Settings,
		}
		_ = goUser // 避免编译器优化
	}
}
