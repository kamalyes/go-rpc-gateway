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
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
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
	// 测试 grpcStatusToHTTP 的默认值
	assert.Equal(t, 500, grpcStatusToHTTP(codes.Code(999)))
}

// TestPopulateFieldFromPath_Enum 验证 enum 类型路径参数能通过 grpc-gateway 的 PopulateFieldFromPath 正确设置
// 复现 webhook 场景：path 参数 type=WEBHOOK_TYPE_CF_DOMAIN_BIND 映射到 enum 字段
// 前置条件：enum 类型必须注册到 protoregistry.GlobalTypes（由 registerFileTypes 完成）
func TestPopulateFieldFromPath_Enum(t *testing.T) {
	md := buildTestEnumMessageDescriptor(t, "TestEnumMsg", "type")
	// 注册类型到 GlobalTypes，使 PopulateFieldFromPath 能查找枚举
	registerFileTypes(md.ParentFile())

	inputMsg := dynamicpb.NewMessage(md)
	field := md.Fields().ByName("type")
	assert.NotNil(t, field)
	assert.Equal(t, protoreflect.EnumKind, field.Kind())

	// 1. 按枚举名设置（webhook 路径参数的实际场景）
	err := runtime.PopulateFieldFromPath(inputMsg, "type", "WEBHOOK_TYPE_CF_DOMAIN_BIND")
	assert.NoError(t, err)
	assert.Equal(t, protoreflect.EnumNumber(1), inputMsg.Get(field).Enum())

	// 2. 按数字设置
	inputMsg.Clear(field)
	err = runtime.PopulateFieldFromPath(inputMsg, "type", "2")
	assert.NoError(t, err)
	assert.Equal(t, protoreflect.EnumNumber(2), inputMsg.Get(field).Enum())

	// 3. 无效值应返回 error
	inputMsg.Clear(field)
	err = runtime.PopulateFieldFromPath(inputMsg, "type", "INVALID_NAME")
	assert.Error(t, err)
}

// TestPopulateFieldFromPath_Bytes 验证 bytes 类型路径参数能通过 PopulateFieldFromPath 正确 base64 解码
func TestPopulateFieldFromPath_Bytes(t *testing.T) {
	md := buildTestMessageDescriptor(t, "TestBytesMsg", "data")

	inputMsg := dynamicpb.NewMessage(md)
	field := md.Fields().ByName("data")
	assert.NotNil(t, field)
	assert.Equal(t, protoreflect.BytesKind, field.Kind())

	// 1. 标准 base64 编码
	err := runtime.PopulateFieldFromPath(inputMsg, "data", base64.StdEncoding.EncodeToString([]byte("hello")))
	assert.NoError(t, err)
	assert.Equal(t, []byte("hello"), inputMsg.Get(field).Bytes())

	// 2. URL 安全 base64 编码
	inputMsg.Clear(field)
	err = runtime.PopulateFieldFromPath(inputMsg, "data", base64.URLEncoding.EncodeToString([]byte("world")))
	assert.NoError(t, err)
	assert.Equal(t, []byte("world"), inputMsg.Get(field).Bytes())
}

// TestPopulateFieldFromPath_Timestamp 验证 Timestamp 类型路径参数通过 PopulateFieldFromPath 解析
// grpc-gateway 的 parseMessage 对 Timestamp 用 time.Parse(RFC3339Nano)
func TestPopulateFieldFromPath_Timestamp(t *testing.T) {
	md := buildTestWellKnownMessageDescriptor(t, "TestTimestampMsg", "ts", "google.protobuf.Timestamp")

	inputMsg := dynamicpb.NewMessage(md)
	field := md.Fields().ByName("ts")
	assert.NotNil(t, field)
	assert.Equal(t, protoreflect.MessageKind, field.Kind())

	// RFC3339 格式的时间字符串
	err := runtime.PopulateFieldFromPath(inputMsg, "ts", "2026-06-23T10:00:00Z")
	assert.NoError(t, err)

	// 验证 seconds 字段被正确设置
	tsMsg := inputMsg.Get(field).Message()
	secsField := tsMsg.Descriptor().Fields().ByName("seconds")
	assert.NotNil(t, secsField)
	assert.Equal(t, int64(1782208800), tsMsg.Get(secsField).Int())
}

// TestPopulateFieldFromPath_Duration 验证 Duration 类型路径参数通过 PopulateFieldFromPath 解析
// grpc-gateway 的 parseMessage 对 Duration 用 time.ParseDuration（Go duration 格式）
func TestPopulateFieldFromPath_Duration(t *testing.T) {
	md := buildTestWellKnownMessageDescriptor(t, "TestDurationMsg", "dur", "google.protobuf.Duration")

	inputMsg := dynamicpb.NewMessage(md)
	field := md.Fields().ByName("dur")
	assert.NotNil(t, field)
	assert.Equal(t, protoreflect.MessageKind, field.Kind())

	// Go duration 格式 "1h30m" = 5400 秒
	err := runtime.PopulateFieldFromPath(inputMsg, "dur", "1h30m")
	assert.NoError(t, err)

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

// =============================================================================
// discoverServices 测试（EOF 处理 + reflection 集成）
// =============================================================================

// TestDiscoverServices_Success 正常 reflection 发现
func TestDiscoverServices_Success(t *testing.T) {
	addr, stop := startTestGRPCServer(t)
	defer stop()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	waitForConnReady(t, conn, 5*time.Second)

	services, files, err := discoverServices(context.Background(), conn)
	assert.NoError(t, err)
	// 服务器仅注册了 reflection，reflection 服务会被跳过 → 空列表
	assert.Empty(t, services)
	assert.Empty(t, files)
}

// TestDiscoverServices_ConnectionRefused 连接不可达时的错误处理
func TestDiscoverServices_ConnectionRefused(t *testing.T) {
	conn, err := grpc.NewClient("localhost:59999", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	_, _, err = discoverServices(context.Background(), conn)
	assert.Error(t, err)
	// 错误应包含 reflection stream 创建或发送失败的信息
	errMsg := err.Error()
	assert.True(t,
		strings.Contains(errMsg, "reflection") || strings.Contains(errMsg, "ListServices") || strings.Contains(errMsg, "Unavailable"),
		"错误应包含 reflection 相关信息, got: %s", errMsg)
}

// TestDiscoverServices_ServerRejectsStream 服务端拒绝 stream 时的 EOF 处理
// 验证：Send 返回 EOF 时调用 Recv 获取真实错误，而非丢失为纯 EOF
func TestDiscoverServices_ServerRejectsStream(t *testing.T) {
	rejectErr := status.Error(codes.PermissionDenied, "reflection denied")
	addr, stop := startTestGRPCServerWithStreamInterceptor(t, rejectErr)
	defer stop()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	waitForConnReady(t, conn, 5*time.Second)

	_, _, err = discoverServices(context.Background(), conn)
	assert.Error(t, err)
	// 关键验证：错误不应是纯 EOF，应包含服务端返回的真实状态
	errMsg := err.Error()
	assert.Contains(t, errMsg, "PermissionDenied",
		"Send 返回 EOF 时应通过 Recv 获取真实错误，而非丢失为纯 EOF, got: %s", errMsg)
	assert.NotContains(t, errMsg, "发送 ListServices 请求失败: EOF",
		"错误不应是纯 EOF，应包含真实 gRPC 状态")
}

// TestDiscoverServices_ServerShutdown 服务端关闭后的错误处理
func TestDiscoverServices_ServerShutdown(t *testing.T) {
	addr, stop := startTestGRPCServer(t)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	waitForConnReady(t, conn, 5*time.Second)

	// 先验证正常工作
	_, _, err = discoverServices(context.Background(), conn)
	assert.NoError(t, err, "服务器运行时 discovery 应成功")

	// 停止服务器
	stop()

	// 等待 ClientConn 检测到连接断开
	time.Sleep(2 * time.Second)

	// 再次调用应失败，错误应包含有意义的信息（非纯 EOF）
	_, _, err = discoverServices(context.Background(), conn)
	assert.Error(t, err)
	errMsg := err.Error()
	// 错误应包含 Unavailable 或 EOF 或 reflection 相关信息
	assert.True(t,
		strings.Contains(errMsg, "Unavailable") ||
			strings.Contains(errMsg, "EOF") ||
			strings.Contains(errMsg, "reflection") ||
			strings.Contains(errMsg, "ListServices"),
		"服务器关闭后应返回有意义的错误, got: %s", errMsg)
}

// TestDiscoverServices_ContextCanceled 上下文取消时的错误处理
func TestDiscoverServices_ContextCanceled(t *testing.T) {
	addr, stop := startTestGRPCServer(t)
	defer stop()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	waitForConnReady(t, conn, 5*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, _, err = discoverServices(ctx, conn)
	assert.Error(t, err)
}

// TestDiscoverServices_MultipleServers 多个服务器的 reflection 发现
func TestDiscoverServices_MultipleServers(t *testing.T) {
	addr1, stop1 := startTestGRPCServer(t)
	defer stop1()

	addr2, stop2 := startTestGRPCServer(t)
	defer stop2()

	// 服务器1
	conn1, err := grpc.NewClient(addr1, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn1.Close()
	waitForConnReady(t, conn1, 5*time.Second)

	// 服务器2
	conn2, err := grpc.NewClient(addr2, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn2.Close()
	waitForConnReady(t, conn2, 5*time.Second)

	// 分别 discovery
	_, _, err1 := discoverServices(context.Background(), conn1)
	_, _, err2 := discoverServices(context.Background(), conn2)
	assert.NoError(t, err1)
	assert.NoError(t, err2)
}

// TestDiscoverServices_RetryAfterRecovery 服务恢复后 discovery 可再次成功
func TestDiscoverServices_RetryAfterRecovery(t *testing.T) {
	freePort := findFreePort(t)

	conn, err := grpc.NewClient(freePort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	// 初始：端口未监听 → discovery 失败
	_, _, err = discoverServices(context.Background(), conn)
	assert.Error(t, err, "端口未监听时 discovery 应失败")

	// 启动服务器
	lis, err := net.Listen("tcp", freePort)
	require.NoError(t, err)
	server := grpc.NewServer()
	reflection.Register(server)
	go server.Serve(lis)
	defer server.Stop()

	// 等待 ClientConn 自动重连
	waitForConnReady(t, conn, 15*time.Second)

	// 恢复后 discovery 应成功
	_, _, err = discoverServices(context.Background(), conn)
	assert.NoError(t, err, "服务恢复后 discovery 应成功")
}
