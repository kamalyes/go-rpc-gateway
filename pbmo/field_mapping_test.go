/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-01-24 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-01-24 10:00:00
 * @FilePath: \go-rpc-gateway\pbmo\field_mapping_test.go
 * @Description: 字段映射功能测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============= 测试用结构体定义 =============

// ThirdPartyClient 模拟第三方库的结构体（无法修改源码）
type ThirdPartyClient struct {
	ID          string
	UserID      string
	NodeID      string
	ClientIP    string
	ConnectedAt time.Time
	LastSeen    time.Time
	Status      int
}

// ClientProto 模拟Proto生成的结构体
type ClientProto struct {
	ClientId    string
	UserId      string
	NodeId      string
	ClientIp    string
	ConnectTime string
	LastSeen    string
	Status      int
}

// CustomModel 自定义结构体（支持pbmo tag）
type CustomModel struct {
	ID       string `json:"id" pbmo:"ClientId"`
	UserID   string `json:"user_id" pbmo:"UserId"`
	NodeID   string `json:"node_id" pbmo:"NodeId"`
	ClientIP string `json:"client_ip" pbmo:"ClientIp"`
	Status   int    `json:"status"`
}

// CustomProto 对应的Proto结构
type CustomProto struct {
	ClientId string
	UserId   string
	NodeId   string
	ClientIp string
	Status   int
}

// ============= 测试用例 =============

// TestFieldMapping_WithFieldMapping 测试链式调用方式的字段映射
func TestFieldMapping_WithFieldMapping(t *testing.T) {
	// 创建转换器并配置字段映射
	converter := NewBidiConverter(
		&ClientProto{},
		&ThirdPartyClient{},
	).WithAutoTimeConversion(false).
		WithFieldMapping("ID", "ClientId").
		WithFieldMapping("UserID", "UserId").
		WithFieldMapping("NodeID", "NodeId").
		WithFieldMapping("ClientIP", "ClientIp")

	// 准备测试数据
	client := &ThirdPartyClient{
		ID:       "conn_123",
		UserID:   "user_456",
		NodeID:   "node_01",
		ClientIP: "192.168.1.1",
		Status:   1,
	}

	// Model -> Proto
	proto := &ClientProto{}
	err := converter.ConvertModelToPB(client, proto)
	assert.NoError(t, err, "ConvertModelToPB should not return error")

	// 验证映射结果
	assert.Equal(t, "conn_123", proto.ClientId, "ClientId should match")
	assert.Equal(t, "user_456", proto.UserId, "UserId should match")
	assert.Equal(t, "node_01", proto.NodeId, "NodeId should match")
	assert.Equal(t, "192.168.1.1", proto.ClientIp, "ClientIp should match")
	assert.Equal(t, 1, proto.Status, "Status should match")

	t.Logf("✅ WithFieldMapping 转换成功: %+v", proto)
}

// TestFieldMapping_RegisterFieldMapping 测试批量注册方式
func TestFieldMapping_RegisterFieldMapping(t *testing.T) {
	converter := NewBidiConverter(
		&ClientProto{},
		&ThirdPartyClient{},
	).WithAutoTimeConversion(false)

	// 批量注册字段映射
	converter.RegisterFieldMapping(map[string]string{
		"ID":       "ClientId",
		"UserID":   "UserId",
		"NodeID":   "NodeId",
		"ClientIP": "ClientIp",
	})

	// 准备测试数据
	client := &ThirdPartyClient{
		ID:       "conn_789",
		UserID:   "user_012",
		NodeID:   "node_02",
		ClientIP: "10.0.0.1",
		Status:   2,
	}

	// Model -> Proto
	proto := &ClientProto{}
	err := converter.ConvertModelToPB(client, proto)
	assert.NoError(t, err, "ConvertModelToPB should not return error")

	// 验证映射结果
	assert.Equal(t, "conn_789", proto.ClientId, "ClientId should match")
	assert.Equal(t, "user_012", proto.UserId, "UserId should match")
	assert.Equal(t, "node_02", proto.NodeId, "NodeId should match")
	assert.Equal(t, "10.0.0.1", proto.ClientIp, "ClientIp should match")

	t.Logf("✅ RegisterFieldMapping 转换成功: %+v", proto)
}

// TestFieldMapping_StructTag 测试struct tag方式的字段映射
func TestFieldMapping_StructTag(t *testing.T) {
	// 创建转换器（会自动读取pbmo tag）
	converter := NewBidiConverter(
		&CustomProto{},
		&CustomModel{},
	)

	// 准备测试数据
	model := &CustomModel{
		ID:       "model_123",
		UserID:   "user_456",
		NodeID:   "node_01",
		ClientIP: "172.16.0.1",
		Status:   3,
	}

	// Model -> Proto
	proto := &CustomProto{}
	err := converter.ConvertModelToPB(model, proto)
	assert.NoError(t, err, "ConvertModelToPB should not return error")

	// 验证映射结果
	assert.Equal(t, "model_123", proto.ClientId, "ClientId should match")
	assert.Equal(t, "user_456", proto.UserId, "UserId should match")
	assert.Equal(t, "node_01", proto.NodeId, "NodeId should match")
	assert.Equal(t, "172.16.0.1", proto.ClientIp, "ClientIp should match")
	assert.Equal(t, 3, proto.Status, "Status should match")

	t.Logf("✅ Struct Tag 映射成功: %+v", proto)
}

// TestFieldMapping_ReverseConversion 测试反向转换（Proto -> Model）
func TestFieldMapping_ReverseConversion(t *testing.T) {
	converter := NewBidiConverter(
		&ClientProto{},
		&ThirdPartyClient{},
	).WithAutoTimeConversion(false).
		WithFieldMapping("ID", "ClientId").
		WithFieldMapping("UserID", "UserId").
		WithFieldMapping("NodeID", "NodeId").
		WithFieldMapping("ClientIP", "ClientIp")

	// 准备Proto数据
	proto := &ClientProto{
		ClientId: "conn_999",
		UserId:   "user_888",
		NodeId:   "node_03",
		ClientIp: "192.168.2.1",
		Status:   5,
	}

	// Proto -> Model
	client := &ThirdPartyClient{}
	err := converter.ConvertPBToModel(proto, client)
	assert.NoError(t, err, "ConvertPBToModel should not return error")

	// 验证反向映射结果
	assert.Equal(t, "conn_999", client.ID, "ID should match")
	assert.Equal(t, "user_888", client.UserID, "UserID should match")
	assert.Equal(t, "node_03", client.NodeID, "NodeID should match")
	assert.Equal(t, "192.168.2.1", client.ClientIP, "ClientIP should match")
	assert.Equal(t, 5, client.Status, "Status should match")

	t.Logf("✅ 反向转换成功: %+v", client)
}

// TestFieldMapping_MixedMappings 测试混合使用（tag + 手动注册）
func TestFieldMapping_MixedMappings(t *testing.T) {
	// 创建转换器（会读取pbmo tag）
	converter := NewBidiConverter(
		&CustomProto{},
		&CustomModel{},
	).WithFieldMapping("Status", "StatusCode") // 手动覆盖Status字段映射

	// 准备测试数据
	model := &CustomModel{
		ID:       "mix_123",
		UserID:   "user_mix",
		NodeID:   "node_mix",
		ClientIP: "127.0.0.1",
		Status:   99,
	}

	proto := &CustomProto{}
	err := converter.ConvertModelToPB(model, proto)

	// 由于手动映射了Status->StatusCode，而CustomProto没有StatusCode字段
	// 所以Status字段不会被转换，但其他字段应该正常转换
	assert.NoError(t, err, "ConvertModelToPB should not return error")

	// 验证tag定义的映射仍然有效
	assert.Equal(t, "mix_123", proto.ClientId, "ClientId should match")
	assert.Equal(t, "user_mix", proto.UserId, "UserId should match")

	t.Logf("✅ 混合映射测试成功: %+v", proto)
}

// TestFieldMapping_WithTimeConversion 测试时间字段映射
func TestFieldMapping_WithTimeConversion(t *testing.T) {
	converter := NewBidiConverter(
		&ClientProto{},
		&ThirdPartyClient{},
	).WithAutoTimeConversion(false).
		WithFieldMapping("ID", "ClientId").
		WithFieldMapping("UserID", "UserId").
		WithFieldMapping("ConnectedAt", "ConnectTime")

	now := time.Now()
	client := &ThirdPartyClient{
		ID:          "time_test",
		UserID:      "user_time",
		ConnectedAt: now,
	}

	proto := &ClientProto{}
	err := converter.ConvertModelToPB(client, proto)
	assert.NoError(t, err, "ConvertModelToPB should not return error")

	// 验证ID和UserID映射正常
	assert.Equal(t, "time_test", proto.ClientId, "ClientId should match")
	assert.Equal(t, "user_time", proto.UserId, "UserId should match")

	// 注意：ConnectTime是string类型，time.Time不会自动转换为string
	// 需要在业务代码中手动处理时间格式转换
	t.Logf("✅ 时间字段映射测试完成")
}

// TestFieldMapping_Priority 测试映射优先级（手动 > tag）
func TestFieldMapping_Priority(t *testing.T) {
	type TagModel struct {
		FieldA string `pbmo:"TargetA"`
		FieldB string `pbmo:"TargetB"`
	}

	type ProtoType struct {
		TargetA   string
		TargetB   string
		TargetNew string
	}

	// 创建转换器，手动覆盖FieldA的映射
	converter := NewBidiConverter(
		&ProtoType{},
		&TagModel{},
	).WithFieldMapping("FieldA", "TargetNew") // 覆盖tag定义的映射

	model := &TagModel{
		FieldA: "value_a",
		FieldB: "value_b",
	}

	proto := &ProtoType{}
	err := converter.ConvertModelToPB(model, proto)
	assert.NoError(t, err, "ConvertModelToPB should not return error")

	// FieldA应该映射到TargetNew（手动配置覆盖tag）
	assert.Equal(t, "value_a", proto.TargetNew, "TargetNew should match (manual mapping overrides tag)")

	// FieldB应该映射到TargetB（使用tag定义）
	assert.Equal(t, "value_b", proto.TargetB, "TargetB should match (from struct tag)")

	// TargetA应该为空（因为FieldA被重定向到TargetNew）
	assert.Empty(t, proto.TargetA, "TargetA should be empty (field redirected)")

	t.Logf("✅ 映射优先级测试通过: 手动配置 > struct tag")
}

// TestFieldMapping_EmptyMapping 测试无映射情况
func TestFieldMapping_EmptyMapping(t *testing.T) {
	type SimpleModel struct {
		Name string
		Age  int
	}

	type SimpleProto struct {
		Name string
		Age  int
	}

	// 不配置任何映射
	converter := NewBidiConverter(
		&SimpleProto{},
		&SimpleModel{},
	)

	model := &SimpleModel{
		Name: "张三",
		Age:  25,
	}

	proto := &SimpleProto{}
	err := converter.ConvertModelToPB(model, proto)
	assert.NoError(t, err, "ConvertModelToPB should not return error")

	// 字段名相同，应该正常转换
	assert.Equal(t, "张三", proto.Name, "Name should match")
	assert.Equal(t, 25, proto.Age, "Age should match")

	t.Logf("✅ 无映射转换成功（字段名匹配）")
}

// BenchmarkFieldMapping_WithMapping 性能测试：使用字段映射
func BenchmarkFieldMapping_WithMapping(b *testing.B) {
	converter := NewBidiConverter(
		&ClientProto{},
		&ThirdPartyClient{},
	).WithAutoTimeConversion(false).
		WithFieldMapping("ID", "ClientId").
		WithFieldMapping("UserID", "UserId").
		WithFieldMapping("NodeID", "NodeId").
		WithFieldMapping("ClientIP", "ClientIp")

	client := &ThirdPartyClient{
		ID:       "bench_id",
		UserID:   "bench_user",
		NodeID:   "bench_node",
		ClientIP: "1.2.3.4",
		Status:   1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proto := &ClientProto{}
		_ = converter.ConvertModelToPB(client, proto)
	}
}

// BenchmarkFieldMapping_WithoutMapping 性能测试：无映射（基准）
func BenchmarkFieldMapping_WithoutMapping(b *testing.B) {
	type DirectModel struct {
		ClientId string
		UserId   string
		NodeId   string
		ClientIp string
		Status   int
	}

	type DirectProto struct {
		ClientId string
		UserId   string
		NodeId   string
		ClientIp string
		Status   int
	}

	converter := NewBidiConverter(
		&DirectProto{},
		&DirectModel{},
	)

	model := &DirectModel{
		ClientId: "bench_id",
		UserId:   "bench_user",
		NodeId:   "bench_node",
		ClientIp: "1.2.3.4",
		Status:   1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proto := &DirectProto{}
		_ = converter.ConvertModelToPB(model, proto)
	}
}
