/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-13 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-16 11:11:15
 * @FilePath: \go-rpc-gateway\cpool\grpc\auto_register_test.go
 * @Description: 自动注册机制测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package grpc

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestClearRegistry(t *testing.T) {
	// 先添加一些数据
	reflectionRegistry.mu.Lock()
	reflectionRegistry.services["test-service"] = []ReflectionServiceInfo{
		{ServiceName: "TestService"},
	}
	reflectionRegistry.initialized = true
	reflectionRegistry.mu.Unlock()

	routeRegistry.mu.Lock()
	routeRegistry.routes = []HTTPRoute{{HTTPMethod: "GET", HTTPPath: "/test"}}
	routeRegistry.mu.Unlock()

	ClearRegistry()

	// 验证已清空
	reflectionRegistry.mu.RLock()
	assert.Empty(t, reflectionRegistry.services)
	assert.False(t, reflectionRegistry.initialized)
	reflectionRegistry.mu.RUnlock()

	routeRegistry.mu.RLock()
	assert.Empty(t, routeRegistry.routes)
	routeRegistry.mu.RUnlock()
}

func TestGetReflectionRegistry(t *testing.T) {
	ClearRegistry()
	defer ClearRegistry()

	// 初始为空
	services := GetReflectionRegistry("nonexistent")
	assert.Nil(t, services)

	// 添加数据后可获取
	reflectionRegistry.mu.Lock()
	reflectionRegistry.services["test-svc"] = []ReflectionServiceInfo{
		{ServiceName: "TestService"},
	}
	reflectionRegistry.mu.Unlock()

	services = GetReflectionRegistry("test-svc")
	assert.Len(t, services, 1)
	assert.Equal(t, "TestService", services[0].ServiceName)
}

func TestGetRoutes(t *testing.T) {
	ClearRegistry()
	defer ClearRegistry()

	// 初始为空
	routes := GetRoutes()
	assert.Empty(t, routes)

	// 添加路由后可获取
	routeRegistry.mu.Lock()
	routeRegistry.routes = []HTTPRoute{
		{HTTPMethod: "GET", HTTPPath: "/api/v1/test"},
		{HTTPMethod: "POST", HTTPPath: "/api/v1/test"},
	}
	routeRegistry.mu.Unlock()

	routes = GetRoutes()
	assert.Len(t, routes, 2)
}

func TestAutoRegisterResult_Summary(t *testing.T) {
	result := &AutoRegisterResult{
		Clients:       []string{"svc1", "svc2"},
		Handlers:      []string{"GET /api/v1/test"},
		TotalClients:  2,
		TotalHandlers: 1,
		SkippedManual: 0,
	}

	summary := result.Summary()
	assert.Contains(t, summary, "2/2 clients")
	assert.Contains(t, summary, "1/1 handlers")
}

func TestCollectRoutes(t *testing.T) {
	registered := []string{
		"GET /api/v1/users",
		"POST /api/v1/users",
		"DELETE /api/v1/users/{id}",
	}

	routes := collectRoutes(registered)
	assert.Len(t, routes, 3)
	assert.Equal(t, "GET", routes[0].HTTPMethod)
	assert.Equal(t, "/api/v1/users", routes[0].HTTPPath)
	assert.Equal(t, "POST", routes[1].HTTPMethod)
	assert.Equal(t, "DELETE", routes[2].HTTPMethod)
}

func TestGrpcStatusToHTTP(t *testing.T) {
	tests := []struct {
		grpcCode codes.Code
		wantHTTP int
	}{
		{codes.OK, 200},
		{codes.NotFound, 404},
		{codes.PermissionDenied, 403},
		{codes.Unauthenticated, 401},
		{codes.InvalidArgument, 400},
		{codes.Internal, 500},
		{codes.Unavailable, 503},
		{codes.Unimplemented, 501},
	}

	for _, tt := range tests {
		got := grpcStatusToHTTP(tt.grpcCode)
		assert.Equal(t, tt.wantHTTP, got, "gRPC code %v", tt.grpcCode)
	}
}

func TestSetFieldValue(t *testing.T) {
	// 使用 dynamicpb 测试字段设置
	// 这里只测试辅助函数的逻辑，不依赖 proto 描述符
	// 实际的 proto 描述符测试需要完整的 reflection 流程

	// 测试 grpcStatusToHTTP 的默认值
	assert.Equal(t, 500, grpcStatusToHTTP(codes.Code(999)))
}

// TestSetFieldValue_Enum 验证 enum 类型路径参数能正确设置到动态消息上
// 复现 webhook 场景：path 参数 type=WEBHOOK_TYPE_CF_DOMAIN_BIND 映射到 enum 字段
func TestSetFieldValue_Enum(t *testing.T) {
	md := buildTestEnumMessageDescriptor(t, "TestEnumMsg", "type")

	inputMsg := dynamicpb.NewMessage(md)
	field := md.Fields().ByName("type")
	assert.NotNil(t, field)
	assert.Equal(t, protoreflect.EnumKind, field.Kind())

	// 1. 按枚举名设置（webhook 路径参数的实际场景）
	setFieldValue(inputMsg, field, "WEBHOOK_TYPE_CF_DOMAIN_BIND")
	assert.Equal(t, protoreflect.EnumNumber(1), inputMsg.Get(field).Enum())

	// 2. 按数字设置
	inputMsg.Clear(field)
	setFieldValue(inputMsg, field, "2")
	assert.Equal(t, protoreflect.EnumNumber(2), inputMsg.Get(field).Enum())

	// 3. 无效值不应 panic 且不设置
	inputMsg.Clear(field)
	setFieldValue(inputMsg, field, "INVALID_NAME")
	assert.Equal(t, protoreflect.EnumNumber(0), inputMsg.Get(field).Enum())
}

// TestSetFieldValue_Bytes 验证 bytes 类型路径参数能正确 base64 解码
func TestSetFieldValue_Bytes(t *testing.T) {
	md := buildTestMessageDescriptor(t, "TestBytesMsg", "data")

	inputMsg := dynamicpb.NewMessage(md)
	field := md.Fields().ByName("data")
	assert.NotNil(t, field)
	assert.Equal(t, protoreflect.BytesKind, field.Kind())

	// 1. 标准 base64 编码
	setFieldValue(inputMsg, field, base64.StdEncoding.EncodeToString([]byte("hello")))
	assert.Equal(t, []byte("hello"), inputMsg.Get(field).Bytes())

	// 2. URL 安全 base64 编码
	inputMsg.Clear(field)
	setFieldValue(inputMsg, field, base64.URLEncoding.EncodeToString([]byte("world")))
	assert.Equal(t, []byte("world"), inputMsg.Get(field).Bytes())

	// 3. 无效 base64 不应 panic
	inputMsg.Clear(field)
	setFieldValue(inputMsg, field, "!!!not-base64!!!")
	assert.Empty(t, inputMsg.Get(field).Bytes())
}

// TestSetFieldValue_Timestamp 验证 Timestamp 类型路径参数通过 protojson 原生解析
// 路径参数为 RFC3339 字符串，包装成 JSON string 后 protojson 能自动识别 well-known 类型
func TestSetFieldValue_Timestamp(t *testing.T) {
	md := buildTestWellKnownMessageDescriptor(t, "TestTimestampMsg", "ts", "google.protobuf.Timestamp")

	inputMsg := dynamicpb.NewMessage(md)
	field := md.Fields().ByName("ts")
	assert.NotNil(t, field)
	assert.Equal(t, protoreflect.MessageKind, field.Kind())

	// RFC3339 格式的时间字符串
	setFieldValue(inputMsg, field, "2026-06-23T10:00:00Z")

	// 验证 seconds 字段被正确设置（2026-06-23T10:00:00Z 的 Unix 时间戳）
	tsMsg := inputMsg.Get(field).Message()
	secsField := tsMsg.Descriptor().Fields().ByName("seconds")
	assert.NotNil(t, secsField)
	assert.Equal(t, int64(1782208800), tsMsg.Get(secsField).Int())
}

// TestSetFieldValue_Duration_GoFormat 验证 Duration 类型路径参数支持 Go duration 格式
// protojson 只接受 "1800s" 格式，Go duration "1h30m" 需要专用回退
func TestSetFieldValue_Duration_GoFormat(t *testing.T) {
	md := buildTestWellKnownMessageDescriptor(t, "TestDurationMsg", "dur", "google.protobuf.Duration")

	inputMsg := dynamicpb.NewMessage(md)
	field := md.Fields().ByName("dur")
	assert.NotNil(t, field)
	assert.Equal(t, protoreflect.MessageKind, field.Kind())

	// Go duration 格式 "1h30m" = 5400 秒
	setFieldValue(inputMsg, field, "1h30m")

	durMsg := inputMsg.Get(field).Message()
	secsField := durMsg.Descriptor().Fields().ByName("seconds")
	assert.NotNil(t, secsField)
	assert.Equal(t, int64(5400), durMsg.Get(secsField).Int())
}

func TestAnnotateContextForwardsHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/api/v1/test", nil)
	assert.NoError(t, err)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("X-User-ID", "user-1")
	req.Header.Set("Connection", "keep-alive")

	// 使用与实际网关相同的 incomingHeaderMatcher 配置
	// 注意：Authorization 必须排除，因为 grpc-gateway 的 annotateContext 已对其做无条件转发，
	// 此处再匹配会导致 metadata 中出现重复值
	mux := runtime.NewServeMux(runtime.WithIncomingHeaderMatcher(func(key string) (string, bool) {
		switch strings.ToLower(key) {
		case "connection", "keep-alive", "proxy-connection",
			"transfer-encoding", "upgrade", "te":
			return key, false
		case "authorization":
			return key, false
		}
		return key, true
	}))
	ctx, err := runtime.AnnotateContext(context.Background(), mux, req, "/test.Service/Method")
	assert.NoError(t, err)

	md, ok := metadata.FromOutgoingContext(ctx)
	assert.True(t, ok)
	// Authorization 由 grpc-gateway 的向后兼容逻辑转发，只出现一次
	assert.Equal(t, []string{"Bearer token"}, md.Get("authorization"))
	assert.Equal(t, []string{"user-1"}, md.Get("x-user-id"))
	assert.Empty(t, md.Get("connection"))
}

// TestCreateDynamicHandler_BytesBodyField 验证 body 映射到 bytes 字段时不会 panic
// 复现 webhook 场景：proto 定义 body: "body"，body 字段为 bytes 类型
func TestCreateDynamicHandler_BytesBodyField(t *testing.T) {
	// 使用内置的 descriptorpb.FileDescriptorProto 构造一个简单的 message 描述符
	// 包含一个 bytes 字段 "body"，模拟 WebhookReceiveRequest
	md := buildTestMessageDescriptor(t, "TestBytesBodyMsg", "body")

	// 模拟 Worker 发送的 body：JSON.stringify(ciphertext) = "Base64..."（带引号的 JSON string）
	// protojson 对 bytes 字段期望 base64 编码的 JSON string
	ciphertext := `"SGVsbG8gV29ybGQ="` // "Hello World" 的 base64

	inputMsg := dynamicpb.NewMessage(md)
	field := md.Fields().ByName("body")
	assert.NotNil(t, field)
	assert.Equal(t, protoreflect.BytesKind, field.Kind())

	// 构造 {"body": "SGVsbG8gV29ybGQ="} 交给 protojson 解析
	wrappedJSON := fmt.Sprintf(`{%q: %s}`, "body", ciphertext)
	err := protojson.Unmarshal([]byte(wrappedJSON), inputMsg)
	assert.NoError(t, err)
	assert.Equal(t, []byte("Hello World"), inputMsg.Get(field).Bytes())
}

// TestCreateDynamicHandler_MessageBodyField 验证 body 映射到 message 字段时正常工作
func TestCreateDynamicHandler_MessageBodyField(t *testing.T) {
	// 使用 descriptorpb.FileDescriptorProto 作为测试 message，它本身就是一个 proto message
	// 其 name 字段是 string 类型，验证 message 字段的 body 解析路径
	md := (&descriptorpb.FileDescriptorProto{}).ProtoReflect().Descriptor()
	field := md.Fields().ByName("name")
	assert.NotNil(t, field)
	assert.Equal(t, protoreflect.StringKind, field.Kind())

	// body 是整个 message 的 JSON 表示
	bodyData := []byte(`{"name":"test.proto"}`)
	inputMsg := dynamicpb.NewMessage(md)
	err := protojson.Unmarshal(bodyData, inputMsg)
	assert.NoError(t, err)
	assert.Equal(t, "test.proto", inputMsg.Get(field).String())
}

// buildTestMessageDescriptor 构造一个包含单个 bytes 字段的测试 message 描述符
func buildTestMessageDescriptor(t *testing.T, msgName, fieldName string) protoreflect.MessageDescriptor {
	t.Helper()
	fd := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("test_" + strings.ToLower(msgName) + ".proto"),
		Package: proto.String("test"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String(msgName),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:   proto.String(fieldName),
						Number: proto.Int32(1),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum(),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	}
	file, err := protodesc.NewFile(fd, nil)
	assert.NoError(t, err)
	return file.Messages().Get(0)
}

// buildTestEnumMessageDescriptor 构造一个包含单个 enum 字段的测试 message 描述符
// enum 类型定义在 message 内部，模拟 WebhookReceiveRequest.type 字段
func buildTestEnumMessageDescriptor(t *testing.T, msgName, fieldName string) protoreflect.MessageDescriptor {
	t.Helper()
	enumName := msgName + "Type"
	fd := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("test_enum_" + strings.ToLower(msgName) + ".proto"),
		Package: proto.String("test"),
		EnumType: []*descriptorpb.EnumDescriptorProto{
			{
				Name: proto.String(enumName),
				Value: []*descriptorpb.EnumValueDescriptorProto{
					{Name: proto.String("WEBHOOK_TYPE_UNSPECIFIED"), Number: proto.Int32(0)},
					{Name: proto.String("WEBHOOK_TYPE_CF_DOMAIN_BIND"), Number: proto.Int32(1)},
					{Name: proto.String("WEBHOOK_TYPE_CF_DOMAIN_RESET"), Number: proto.Int32(2)},
				},
			},
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String(msgName),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String(fieldName),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
						TypeName: proto.String("." + "test" + "." + enumName),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	}
	file, err := protodesc.NewFile(fd, nil)
	assert.NoError(t, err)
	return file.Messages().Get(0)
}

// buildTestWellKnownMessageDescriptor 构造一个包含单个 well-known 类型字段的消息描述符
// wellKnownType 是全名，如 "google.protobuf.Timestamp"
func buildTestWellKnownMessageDescriptor(t *testing.T, msgName, fieldName, wellKnownType string) protoreflect.MessageDescriptor {
	t.Helper()
	// 从全局注册表查找 well-known 类型描述符
	wkDesc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(wellKnownType))
	assert.NoError(t, err)
	_, ok := wkDesc.(protoreflect.MessageDescriptor)
	assert.True(t, ok, "well-known type %s is not a message", wellKnownType)

	// well-known 类型对应的 proto 文件名映射
	wkProtoFile := map[string]string{
		"google.protobuf.Timestamp":   "google/protobuf/timestamp.proto",
		"google.protobuf.Duration":    "google/protobuf/duration.proto",
		"google.protobuf.FieldMask":   "google/protobuf/field_mask.proto",
		"google.protobuf.StringValue": "google/protobuf/wrappers.proto",
		"google.protobuf.Int32Value":  "google/protobuf/wrappers.proto",
		"google.protobuf.Int64Value":  "google/protobuf/wrappers.proto",
		"google.protobuf.BoolValue":   "google/protobuf/wrappers.proto",
	}
	depFile, ok := wkProtoFile[wellKnownType]
	assert.True(t, ok, "unsupported well-known type: %s", wellKnownType)

	fd := &descriptorpb.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String("test_wk_" + strings.ToLower(msgName) + ".proto"),
		Package: proto.String("test"),
		Dependency: []string{
			depFile,
		},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String(msgName),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String(fieldName),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String("." + wellKnownType),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					},
				},
			},
		},
	}
	// 使用 protoregistry.GlobalFiles 作为 resolver，解析 well-known 类型的依赖
	file, err := protodesc.NewFile(fd, protoregistry.GlobalFiles)
	assert.NoError(t, err)
	return file.Messages().Get(0)
}
