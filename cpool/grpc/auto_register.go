/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-06-13 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-16 11:15:51
 * @FilePath: \go-rpc-gateway\cpool\grpc\auto_register.go
 * @Description: gRPC 服务自动注册 - 基于 gRPC Server Reflection，业务零胶水代码
 *
 * 工作流程:
 *   1. Gateway 连接配置中的 gRPC server
 *   2. 通过 gRPC Server Reflection 自动获取服务列表和 FileDescriptorProto
 *   3. 解析 google.api.http annotation，提取 HTTP 路由
 *   4. 动态注册 HTTP handler 到 runtime.ServeMux
 *
 * 前提: gRPC server 需要启用 reflection (reflection.Register(server))
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package grpc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/grpc-ecosystem/grpc-gateway/v2/utilities"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	gwglobal "github.com/kamalyes/go-rpc-gateway/global"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// =============================================================================
// 类型定义
// =============================================================================

// ReflectionServiceInfo 通过 reflection 获取的单个服务信息
type ReflectionServiceInfo struct {
	ServiceName string // 完整服务名，如 "km.access_control.AuthService"
}

// HTTPRoute 描述一个 HTTP 路由映射
type HTTPRoute struct {
	ServiceName string // gRPC 服务全名，如 "km.access_control.AuthService"
	MethodName  string // gRPC 方法名，如 "Login"
	HTTPMethod  string // HTTP 方法，如 "POST"
	HTTPPath    string // HTTP 路径，如 "/api/v1/auth/login"
	BodyField   string // 请求体映射字段，"*" 表示整个 body
}

// AutoRegisterResult 自动注册结果
type AutoRegisterResult struct {
	Clients       []string // 成功连接的服务名列表
	Handlers      []string // 成功注册的 HTTP 路由列表
	TotalClients  int      // 配置的客户端总数
	TotalHandlers int      // 发现的 handler 总数
	SkippedManual int      // 跳过的手动注册数
}

// Summary 返回结果摘要
func (r *AutoRegisterResult) Summary() string {
	return fmt.Sprintf("AutoRegister: %d/%d clients, %d/%d handlers initialized (%d manual skipped)",
		len(r.Clients), r.TotalClients, len(r.Handlers), r.TotalHandlers, r.SkippedManual)
}

// =============================================================================
// 全局状态
// =============================================================================

// reflectionRegistry 存储 reflection 发现的服务和 proto 文件
var reflectionRegistry = &struct {
	mu          sync.RWMutex
	services    map[string][]ReflectionServiceInfo // serviceName -> services
	files       *protoregistry.Files               // proto 文件注册表（指向 GlobalFiles）
	initialized bool
}{
	services: make(map[string][]ReflectionServiceInfo),
	files:    protoregistry.GlobalFiles,
}

// routeRegistry 存储已注册的 HTTP 路由
var routeRegistry = &struct {
	mu     sync.RWMutex
	routes []HTTPRoute
}{}

// defaultJSONPb 默认 JSON 序列化器（protojson.MarshalOptions 为只读配置，并发安全）
var defaultJSONPb = &runtime.JSONPb{
	MarshalOptions: protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: true,
	},
}

// GetReflectionRegistry 获取已发现的服务列表
func GetReflectionRegistry(serviceName string) []ReflectionServiceInfo {
	reflectionRegistry.mu.RLock()
	defer reflectionRegistry.mu.RUnlock()
	return reflectionRegistry.services[serviceName]
}

// GetRoutes 获取所有已注册的 HTTP 路由
func GetRoutes() []HTTPRoute {
	routeRegistry.mu.RLock()
	defer routeRegistry.mu.RUnlock()
	return routeRegistry.routes
}

// ClearRegistry 清空 reflection 发现的服务和路由（主要用于测试，不清空 GlobalFiles）
func ClearRegistry() {
	reflectionRegistry.mu.Lock()
	reflectionRegistry.services = make(map[string][]ReflectionServiceInfo)
	reflectionRegistry.initialized = false
	reflectionRegistry.mu.Unlock()

	routeRegistry.mu.Lock()
	routeRegistry.routes = nil
	routeRegistry.mu.Unlock()
}

// =============================================================================
// 连接管理
// =============================================================================

// initAllConnections 为所有配置的 gRPC 客户端初始化连接
// 返回成功建立连接的服务名列表
func initAllConnections(ctx context.Context, healthChecker *HealthChecker, clients map[string]*gwconfig.GRPCClient) []string {
	var connected []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	for serviceName, clientCfg := range clients {
		// 跳过已存在的连接
		if _, ok := GetConn(serviceName); ok {
			connected = append(connected, serviceName)
			continue
		}

		wg.Add(1)
		go func(name string, cfg *gwconfig.GRPCClient) {
			defer wg.Done()

			if cfg == nil || len(cfg.Endpoints) == 0 {
				return
			}

			endpoint := cfg.Endpoints[0]
			creds := buildTLSConfig(cfg, name)
			dialOpts := buildDialOptions(cfg, name, creds, healthChecker)

			conn, err := grpc.NewClient(endpoint, dialOpts...)
			if err != nil {
				gwglobal.LOGGER.WarnContext(ctx, "⚠️  %s 创建连接失败: %v", name, err)
				return
			}

			if healthChecker != nil {
				healthChecker.Register(name, conn, endpoint)
			}

			PutConn(name, conn)
			gwglobal.LOGGER.InfoContext(ctx, "✅ %s 连接已建立 -> %s", name, endpoint)

			mu.Lock()
			connected = append(connected, name)
			mu.Unlock()
		}(serviceName, clientCfg)
	}

	wg.Wait()
	sort.Strings(connected)
	return connected
}

// =============================================================================
// Reflection 服务发现
// =============================================================================

// registerFileDescriptors 注册未在 GlobalFiles 中的 FileDescriptorProto
// 业务 import 的 proto 包已在 GlobalFiles 中，跳过；只注册 reflection 获取的新文件
// 多趟注册解决文件间依赖顺序问题（被依赖的文件需先注册），用 recover 防 panic
// 返回新注册数和跳过数
func registerFileDescriptors(fileDescriptors map[string]*descriptorpb.FileDescriptorProto) (registered, skipped int) {
	if len(fileDescriptors) == 0 {
		return
	}

	// 第一趟：过滤已在 GlobalFiles 中的文件
	remaining := make(map[string]*descriptorpb.FileDescriptorProto, len(fileDescriptors))
	for name, fdp := range fileDescriptors {
		if _, err := protoregistry.GlobalFiles.FindFileByPath(name); err == nil {
			skipped++
			continue
		}
		remaining[name] = fdp
	}

	// 后续趟：反复尝试注册，每趟注册依赖已满足的文件，直到无进展
	for len(remaining) > 0 {
		progress := false
		for name, fdp := range remaining {
			ok := func() (succeeded bool) {
				defer func() { _ = recover() }()
				fd, err := protodesc.NewFile(fdp, protoregistry.GlobalFiles)
				if err != nil {
					return false
				}
				if err := protoregistry.GlobalFiles.RegisterFile(fd); err != nil {
					return false
				}
				// 同时注册类型到 GlobalTypes，使 grpc-gateway 的 PopulateFieldFromPath
				// 等函数能通过 protoregistry.GlobalTypes.FindEnumByName 查找到动态加载的枚举
				registerFileTypes(fd)
				return true
			}()
			if ok {
				registered++
				delete(remaining, name)
				progress = true
			}
		}
		if !progress {
			// 剩余文件存在无法解析的依赖，跳过并记录
			for name := range remaining {
				gwglobal.LOGGER.Warn("registerFileDescriptors: 文件 %s 依赖未注册，跳过", name)
			}
			break
		}
	}
	return
}

// registerFileTypes 将文件描述符中的所有 message 和 enum 类型注册到 protoregistry.GlobalTypes
// 使 grpc-gateway 的 PopulateFieldFromPath / PopulateQueryParameters 能通过全局类型注册表
// 查找到动态加载的枚举类型（parseField 的 EnumKind 分支依赖 protoregistry.GlobalTypes.FindEnumByName）
func registerFileTypes(fd protoreflect.FileDescriptor) {
	for i := 0; i < fd.Messages().Len(); i++ {
		registerMessageType(fd.Messages().Get(i))
	}
	for i := 0; i < fd.Enums().Len(); i++ {
		registerEnumType(fd.Enums().Get(i))
	}
}

// registerMessageType 递归注册 message 类型及其嵌套类型
func registerMessageType(md protoreflect.MessageDescriptor) {
	// 跳过已注册的类型（如业务层 import 的 proto 包已自动注册）
	if _, err := protoregistry.GlobalTypes.FindMessageByName(md.FullName()); err == nil {
		return
	}
	if err := protoregistry.GlobalTypes.RegisterMessage(dynamicpb.NewMessageType(md)); err != nil {
		// 已注册或冲突时忽略
		return
	}
	// 递归注册嵌套 message 和 enum
	for i := 0; i < md.Messages().Len(); i++ {
		registerMessageType(md.Messages().Get(i))
	}
	for i := 0; i < md.Enums().Len(); i++ {
		registerEnumType(md.Enums().Get(i))
	}
}

// registerEnumType 注册 enum 类型
func registerEnumType(ed protoreflect.EnumDescriptor) {
	if _, err := protoregistry.GlobalTypes.FindEnumByName(ed.FullName()); err == nil {
		return
	}
	if err := protoregistry.GlobalTypes.RegisterEnum(dynamicpb.NewEnumType(ed)); err != nil {
		return
	}
}

// discoverSingleClient 发现单个客户端的服务（不注册 FileDescriptorProto，由调用方负责）
func discoverSingleClient(ctx context.Context, serviceName string) ([]ReflectionServiceInfo, map[string]*descriptorpb.FileDescriptorProto, error) {
	conn, ok := GetConn(serviceName)
	if !ok {
		return nil, nil, fmt.Errorf("服务 %s 未建立连接", serviceName)
	}

	services, fileCache, err := discoverServices(ctx, conn)
	if err != nil {
		return nil, nil, fmt.Errorf("发现服务 %s 失败: %w", serviceName, err)
	}

	return services, fileCache, nil
}

// DiscoverAllServices 遍历所有已配置的 gRPC 客户端，通过 reflection 发现服务
//
// 业务层 import proto 包时，文件已自动注册到 protoregistry.GlobalFiles（如 access_control）
// 但未被 import 的 proto 包（如 core 服务的 app/client/game 等）不在 GlobalFiles 中
// 因此需要从 reflection 获取 FileDescriptorProto 并注册，用 recover 防 panic（已注册的会冲突）
func DiscoverAllServices(ctx context.Context, clients map[string]*gwconfig.GRPCClient) {
	// Phase 1: 并发 discovery 所有服务，收集服务名和 FileDescriptorProto
	allFileDescriptors := map[string]*descriptorpb.FileDescriptorProto{} // fileName -> fdp
	servicesMap := map[string][]ReflectionServiceInfo{}                  // serviceName -> services
	var mu sync.Mutex
	var wg sync.WaitGroup

	for serviceName := range clients {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			services, fileCache, err := discoverSingleClient(ctx, name)
			if err != nil {
				gwglobal.LOGGER.WarnContext(ctx, "reflection: %v", err)
				return
			}

			mu.Lock()
			servicesMap[name] = services
			for k, v := range fileCache {
				allFileDescriptors[k] = v
			}
			mu.Unlock()

			gwglobal.LOGGER.InfoContext(ctx, "✅ reflection: 服务 %s 发现 %d 个 gRPC 服务", name, len(services))
		}(serviceName)
	}

	wg.Wait()

	// Phase 2: 注册未在 GlobalFiles 中的 FileDescriptorProto
	registered, skipped := registerFileDescriptors(allFileDescriptors)

	reflectionRegistry.mu.Lock()
	reflectionRegistry.services = servicesMap
	reflectionRegistry.files = protoregistry.GlobalFiles
	reflectionRegistry.initialized = true
	reflectionRegistry.mu.Unlock()

	gwglobal.LOGGER.InfoContext(ctx, "✅ reflection: 新注册 %d 个 proto 文件，跳过 %d 个已注册文件，发现 %d 个服务组", registered, skipped, len(servicesMap))
}

// discoverServices 通过 gRPC Server Reflection 发现单个 gRPC server 的所有服务
// 返回服务列表和所有相关的 FileDescriptorProto（含依赖文件）
// 使用流水线模式：批量发送所有 FileContainingSymbol 请求后批量接收响应，将 N 次 RTT 降为 1 次
func discoverServices(ctx context.Context, conn *grpc.ClientConn) ([]ReflectionServiceInfo, map[string]*descriptorpb.FileDescriptorProto, error) {
	client := grpc_reflection_v1alpha.NewServerReflectionClient(conn)
	stream, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("创建 reflection stream 失败: %w", err)
	}
	defer stream.CloseSend()

	// 1. 获取服务列表
	if err := stream.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_ListServices{},
	}); err != nil {
		return nil, nil, fmt.Errorf("发送 ListServices 请求失败: %w", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, nil, fmt.Errorf("接收 ListServices 响应失败: %w", err)
	}

	listResp, ok := resp.MessageResponse.(*grpc_reflection_v1alpha.ServerReflectionResponse_ListServicesResponse)
	if !ok {
		return nil, nil, fmt.Errorf("意外的响应类型: %T", resp.MessageResponse)
	}

	var serviceNames []string
	for _, svc := range listResp.ListServicesResponse.Service {
		// 跳过 reflection 自身的服务
		if strings.HasPrefix(svc.Name, "grpc.reflection") {
			continue
		}
		serviceNames = append(serviceNames, svc.Name)
	}
	sort.Strings(serviceNames)

	if len(serviceNames) == 0 {
		return nil, nil, nil
	}

	gwglobal.LOGGER.InfoContext(ctx, "🔍 reflection 发现 %d 个服务: %v", len(serviceNames), serviceNames)

	// 2. 流水线获取所有服务的 FileDescriptorProto（批量发送 + 批量接收）
	fileCache := map[string]*descriptorpb.FileDescriptorProto{}
	failed := make(map[string]bool, len(serviceNames))

	// 批量发送所有 FileContainingSymbol 请求
	for _, svcName := range serviceNames {
		if err := stream.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
			MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: svcName,
			},
		}); err != nil {
			failed[svcName] = true
			gwglobal.LOGGER.WarnContext(ctx, "发送 FileContainingSymbol 请求失败 %s: %v", svcName, err)
		}
	}

	// 批量接收所有响应（与发送顺序一一对应）
	for _, svcName := range serviceNames {
		if failed[svcName] {
			continue
		}
		resp, err := stream.Recv()
		if err != nil {
			failed[svcName] = true
			gwglobal.LOGGER.WarnContext(ctx, "接收 FileContainingSymbol 响应失败 %s: %v", svcName, err)
			continue
		}
		if err := parseFileDescriptorResponse(resp, fileCache); err != nil {
			failed[svcName] = true
			gwglobal.LOGGER.WarnContext(ctx, "解析 FileDescriptor 失败 %s: %v", svcName, err)
		}
	}

	// 构建结果（跳过失败的服务）
	result := make([]ReflectionServiceInfo, 0, len(serviceNames))
	for _, svcName := range serviceNames {
		if !failed[svcName] {
			result = append(result, ReflectionServiceInfo{ServiceName: svcName})
		}
	}

	return result, fileCache, nil
}

// parseFileDescriptorResponse 解析 ServerReflectionResponse 中的 FileDescriptorProto，写入 cache
func parseFileDescriptorResponse(resp *grpc_reflection_v1alpha.ServerReflectionResponse, cache map[string]*descriptorpb.FileDescriptorProto) error {
	fdResp, ok := resp.MessageResponse.(*grpc_reflection_v1alpha.ServerReflectionResponse_FileDescriptorResponse)
	if !ok {
		return fmt.Errorf("意外的响应类型: %T", resp.MessageResponse)
	}

	if len(fdResp.FileDescriptorResponse.FileDescriptorProto) == 0 {
		return fmt.Errorf("空的 FileDescriptorProto")
	}

	for _, raw := range fdResp.FileDescriptorResponse.FileDescriptorProto {
		fdp := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(raw, fdp); err != nil {
			continue
		}
		if fdp.GetName() == "" {
			continue
		}
		if _, exists := cache[fdp.GetName()]; !exists {
			cache[fdp.GetName()] = fdp
		}
	}
	return nil
}

// =============================================================================
// 动态 HTTP Handler 注册
// =============================================================================

// registerClientHandlers 注册单个客户端的所有 HTTP 路由
func registerClientHandlers(ctx context.Context, mux *runtime.ServeMux, serviceName string, svcInfos []ReflectionServiceInfo) []string {
	var registered []string

	// 获取该服务的 gRPC 连接
	conn, ok := GetConn(serviceName)
	if !ok {
		gwglobal.LOGGER.WarnContext(ctx, "动态注册: 服务 %s 未建立连接，跳过", serviceName)
		return nil
	}

	for _, svcInfo := range svcInfos {
		// 查找 ServiceDescriptor
		fullName := protoreflect.FullName(svcInfo.ServiceName)
		desc, err := protoregistry.GlobalFiles.FindDescriptorByName(fullName)
		if err != nil {
			gwglobal.LOGGER.WarnContext(ctx, "查找 ServiceDescriptor %s 失败: %v", fullName, err)
			continue
		}

		svcDesc, ok := desc.(protoreflect.ServiceDescriptor)
		if !ok {
			continue
		}

		// 提取 HTTP 路由
		routes := extractHTTPRoutes(svcDesc)
		if len(routes) == 0 {
			gwglobal.LOGGER.DebugContext(ctx, "服务 %s 无 HTTP 路由，跳过", fullName)
			continue
		}

		// 注册每个路由
		for _, route := range routes {
			if err := registerSingleRoute(mux, conn, svcDesc, route); err != nil {
				gwglobal.LOGGER.WarnContext(ctx, "注册路由 %s %s 失败: %v", route.HTTPMethod, route.HTTPPath, err)
				continue
			}
			registered = append(registered, fmt.Sprintf("%s %s", route.HTTPMethod, route.HTTPPath))
		}
	}

	return registered
}

// RegisterDynamicHandlers 基于 reflection 结果动态注册 HTTP handler
// Phase 1: 并行提取所有服务的 HTTP 路由（CPU 密集，无共享状态）
// Phase 2: 串行注册到 runtime.ServeMux
//
// runtime.ServeMux.handlers 是 map[string][]handler，Handle/HandlePath 对其直接读写，
// 无任何互斥保护，并发调用会触发 Go map 的 concurrent write panic，因此必须串行注册
func RegisterDynamicHandlers(
	ctx context.Context,
	mux *runtime.ServeMux,
	clients map[string]*gwconfig.GRPCClient,
) ([]string, error) {
	reflectionRegistry.mu.RLock()
	if !reflectionRegistry.initialized {
		reflectionRegistry.mu.RUnlock()
		return nil, fmt.Errorf("reflection registry 未初始化")
	}
	services := reflectionRegistry.services
	reflectionRegistry.mu.RUnlock()

	// Phase 1: 并行提取所有服务的 HTTP 路由
	type svcRoutes struct {
		conn    *grpc.ClientConn
		svcDesc protoreflect.ServiceDescriptor
		routes  []HTTPRoute
	}

	ch := make(chan svcRoutes)
	var wg sync.WaitGroup

	for serviceName, svcInfos := range services {
		conn, ok := GetConn(serviceName)
		if !ok {
			gwglobal.LOGGER.WarnContext(ctx, "动态注册: 服务 %s 未建立连接，跳过", serviceName)
			continue
		}

		for _, svcInfo := range svcInfos {
			wg.Add(1)
			go func(svcName string, conn *grpc.ClientConn) {
				defer wg.Done()

				fullName := protoreflect.FullName(svcName)
				desc, err := protoregistry.GlobalFiles.FindDescriptorByName(fullName)
				if err != nil {
					gwglobal.LOGGER.WarnContext(ctx, "查找 ServiceDescriptor %s 失败: %v", fullName, err)
					return
				}

				svcDesc, ok := desc.(protoreflect.ServiceDescriptor)
				if !ok {
					return
				}

				routes := extractHTTPRoutes(svcDesc)
				if len(routes) == 0 {
					return
				}

				ch <- svcRoutes{conn: conn, svcDesc: svcDesc, routes: routes}
			}(svcInfo.ServiceName, conn)
		}
	}

	// 后台关闭 channel，使收集循环能正常退出
	go func() {
		wg.Wait()
		close(ch)
	}()

	var allRoutes []svcRoutes
	for sr := range ch {
		allRoutes = append(allRoutes, sr)
	}

	// Phase 2: 串行注册到 mux（ServeMux.handlers 为 map[string][]handler，非线程安全）
	var registered []string
	var newRoutes []HTTPRoute
	for _, sr := range allRoutes {
		for _, route := range sr.routes {
			if err := registerSingleRoute(mux, sr.conn, sr.svcDesc, route); err != nil {
				gwglobal.LOGGER.WarnContext(ctx, "注册路由 %s %s 失败: %v", route.HTTPMethod, route.HTTPPath, err)
				continue
			}
			registered = append(registered, fmt.Sprintf("%s %s", route.HTTPMethod, route.HTTPPath))
			newRoutes = append(newRoutes, route)
		}
	}

	// 缓存路由
	routeRegistry.mu.Lock()
	routeRegistry.routes = append(routeRegistry.routes, newRoutes...)
	routeRegistry.mu.Unlock()

	sort.Strings(registered)
	return registered, nil
}

// RediscoverAndRegisterService 重新发现并注册单个服务
// 用于服务恢复健康后，重新通过 reflection 发现服务并注册 HTTP 路由
func RediscoverAndRegisterService(ctx context.Context, mux *runtime.ServeMux, serviceName string) error {
	gwglobal.LOGGER.InfoContext(ctx, "🔄 服务 %s 恢复健康，重新发现并注册...", serviceName)

	// 1. 重新发现服务
	services, fileCache, err := discoverSingleClient(ctx, serviceName)
	if err != nil {
		return fmt.Errorf("重新发现服务 %s 失败: %w", serviceName, err)
	}

	// 2. 注册 FileDescriptorProto
	registered, skipped := registerFileDescriptors(fileCache)
	gwglobal.LOGGER.InfoContext(ctx, "✅ reflection: 服务 %s 新注册 %d 个 proto 文件，跳过 %d 个已注册文件，发现 %d 个 gRPC 服务",
		serviceName, registered, skipped, len(services))

	// 3. 更新 reflectionRegistry
	reflectionRegistry.mu.Lock()
	reflectionRegistry.services[serviceName] = services
	reflectionRegistry.files = protoregistry.GlobalFiles
	reflectionRegistry.initialized = true
	reflectionRegistry.mu.Unlock()

	// 4. 注册 HTTP 路由
	registeredRoutes := registerClientHandlers(ctx, mux, serviceName, services)
	if len(registeredRoutes) > 0 {
		routeRegistry.mu.Lock()
		routeRegistry.routes = append(routeRegistry.routes, collectRoutes(registeredRoutes)...)
		routeRegistry.mu.Unlock()
		gwglobal.LOGGER.InfoContext(ctx, "✅ 服务 %s 重新注册 %d 个 HTTP 路由: %v", serviceName, len(registeredRoutes), registeredRoutes)
	} else {
		gwglobal.LOGGER.InfoContext(ctx, "✅ 服务 %s 无 HTTP 路由需要注册", serviceName)
	}

	return nil
}

// collectRoutes 将 "METHOD PATH" 字符串列表转换为 HTTPRoute 列表
func collectRoutes(registered []string) []HTTPRoute {
	var routes []HTTPRoute
	for _, r := range registered {
		parts := strings.SplitN(r, " ", 2)
		if len(parts) == 2 {
			routes = append(routes, HTTPRoute{
				HTTPMethod: parts[0],
				HTTPPath:   parts[1],
			})
		}
	}
	return routes
}

// extractHTTPRoutes 从 proto ServiceDescriptor 中提取 HTTP 路由
func extractHTTPRoutes(svcDesc protoreflect.ServiceDescriptor) []HTTPRoute {
	var routes []HTTPRoute

	methods := svcDesc.Methods()
	for i := 0; i < methods.Len(); i++ {
		method := methods.Get(i)
		fullName := string(method.FullName())

		// 获取 google.api.http annotation
		opts := method.Options()
		if opts == nil {
			continue
		}

		httpRule := proto.GetExtension(opts, annotations.E_Http).(*annotations.HttpRule)
		if httpRule == nil {
			continue
		}

		route := HTTPRoute{
			ServiceName: string(svcDesc.FullName()),
			MethodName:  string(method.Name()),
			BodyField:   httpRule.GetBody(),
		}

		// 解析 HTTP 方法和路径
		switch pattern := httpRule.Pattern.(type) {
		case *annotations.HttpRule_Get:
			route.HTTPMethod = "GET"
			route.HTTPPath = pattern.Get
		case *annotations.HttpRule_Put:
			route.HTTPMethod = "PUT"
			route.HTTPPath = pattern.Put
		case *annotations.HttpRule_Post:
			route.HTTPMethod = "POST"
			route.HTTPPath = pattern.Post
		case *annotations.HttpRule_Delete:
			route.HTTPMethod = "DELETE"
			route.HTTPPath = pattern.Delete
		case *annotations.HttpRule_Patch:
			route.HTTPMethod = "PATCH"
			route.HTTPPath = pattern.Patch
		case *annotations.HttpRule_Custom:
			route.HTTPMethod = pattern.Custom.GetKind()
			route.HTTPPath = pattern.Custom.GetPath()
		default:
			continue
		}

		if route.HTTPPath == "" {
			continue
		}

		// 处理 additional_bindings
		for _, binding := range httpRule.GetAdditionalBindings() {
			extraRoute := route
			switch pattern := binding.Pattern.(type) {
			case *annotations.HttpRule_Get:
				extraRoute.HTTPMethod = "GET"
				extraRoute.HTTPPath = pattern.Get
			case *annotations.HttpRule_Put:
				extraRoute.HTTPMethod = "PUT"
				extraRoute.HTTPPath = pattern.Put
			case *annotations.HttpRule_Post:
				extraRoute.HTTPMethod = "POST"
				extraRoute.HTTPPath = pattern.Post
			case *annotations.HttpRule_Delete:
				extraRoute.HTTPMethod = "DELETE"
				extraRoute.HTTPPath = pattern.Delete
			case *annotations.HttpRule_Patch:
				extraRoute.HTTPMethod = "PATCH"
				extraRoute.HTTPPath = pattern.Patch
			case *annotations.HttpRule_Custom:
				extraRoute.HTTPMethod = pattern.Custom.GetKind()
				extraRoute.HTTPPath = pattern.Custom.GetPath()
			}
			if extraRoute.HTTPPath != "" {
				extraRoute.BodyField = binding.GetBody()
				routes = append(routes, extraRoute)
			}
		}

		routes = append(routes, route)
		gwglobal.LOGGER.Debug("📍 路由: %s %s -> %s.%s", route.HTTPMethod, route.HTTPPath, fullName, route.MethodName)
	}

	return routes
}

// registerSingleRoute 注册单个 HTTP 路由
func registerSingleRoute(
	mux *runtime.ServeMux,
	conn *grpc.ClientConn,
	svcDesc protoreflect.ServiceDescriptor,
	route HTTPRoute,
) error {
	// 获取输入和输出消息类型
	methodDesc := svcDesc.Methods().ByName(protoreflect.Name(route.MethodName))
	if methodDesc == nil {
		return fmt.Errorf("方法 %s 不存在", route.MethodName)
	}

	inputType := methodDesc.Input()
	outputType := methodDesc.Output()

	// 构造 gRPC 方法全名
	fullMethodName := fmt.Sprintf("/%s/%s", svcDesc.FullName(), route.MethodName)

	// 注册到 runtime.ServeMux
	handler := createDynamicHandler(mux, conn, fullMethodName, inputType, outputType, route)

	return mux.HandlePath(route.HTTPMethod, route.HTTPPath, handler)
}

// createDynamicHandler 创建动态 HTTP handler
func createDynamicHandler(
	mux *runtime.ServeMux,
	conn *grpc.ClientConn,
	fullMethodName string,
	inputType protoreflect.MessageDescriptor,
	outputType protoreflect.MessageDescriptor,
	route HTTPRoute,
) runtime.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		ctx := r.Context()

		// 1. 创建输入消息
		inputMsg := dynamicpb.NewMessage(inputType)

		// 2. 从请求体填充字段（先填充 body，再填充 path/query，避免 body 覆盖路径参数）
		if route.BodyField != "" && r.Body != nil {
			bodyData, err := io.ReadAll(r.Body)
			if err != nil {
				writeError(w, http.StatusBadRequest, "读取请求体失败")
				return
			}
			defer r.Body.Close()

			if len(bodyData) > 0 {
				if route.BodyField == "*" {
					// 整个 body 映射到消息
					if err := protojson.Unmarshal(bodyData, inputMsg); err != nil {
						writeError(w, http.StatusBadRequest, fmt.Sprintf("解析请求体失败: %v", err))
						return
					}
				} else {
					// body 映射到特定字段
					field := inputType.Fields().ByName(protoreflect.Name(route.BodyField))
					if field != nil {
						if field.Kind() == protoreflect.MessageKind {
							// message 类型字段：body 是该 message 的 JSON 表示
							fieldMsg := dynamicpb.NewMessage(field.Message())
							if err := protojson.Unmarshal(bodyData, fieldMsg); err != nil {
								writeError(w, http.StatusBadRequest, fmt.Sprintf("解析请求体字段失败: %v", err))
								return
							}
							inputMsg.Set(field, protoreflect.ValueOfMessage(fieldMsg))
						} else {
							// scalar/bytes/enum 类型字段：body 是该字段的 JSON 值
							// 构造 {"field": <body>} 交给 protojson 解析，bytes 字段会自动 base64 解码
							wrappedJSON := fmt.Sprintf(`{%q: %s}`, route.BodyField, bodyData)
							if err := protojson.Unmarshal([]byte(wrappedJSON), inputMsg); err != nil {
								writeError(w, http.StatusBadRequest, fmt.Sprintf("解析请求体字段失败: %v", err))
								return
							}
						}
					}
				}
			}
		}

		// 3. 从路径参数填充字段（body 之后，确保路径参数不被覆盖）
		//    直接使用 grpc-gateway 的 PopulateFieldFromPath，支持全部 18 种 Kind
		//    （含 enum/bytes/message/well-known 类型），与静态生成的 gateway 代码行为完全一致
		for paramName, paramValue := range pathParams {
			if err := runtime.PopulateFieldFromPath(inputMsg, paramName, paramValue); err != nil {
				gwglobal.LOGGER.WarnContext(ctx, "路径参数 %s=%s 填充失败: %v", paramName, paramValue, err)
			}
		}

		// 4. 从查询参数填充字段（使用 grpc-gateway 的 PopulateQueryParameters 支持嵌套字段如 page_request.page）
		if err := r.ParseForm(); err == nil {
			if err := runtime.PopulateQueryParameters(inputMsg, r.Form, &utilities.DoubleArray{Encoding: map[string]int{}}); err != nil {
				gwglobal.LOGGER.WarnContext(ctx, "解析 query 参数失败: %v", err)
			}
		}

		// 5. 使用 AnnotateContext 构建带 metadata 的 context
		// AnnotateContext 通过 mux 的 incomingHeaderMatcher 正确映射 HTTP header 到 gRPC metadata，
		// 确保 middleware 注入的 payload 信息（如 user_id, domain 等）能正确传递到下游 gRPC 服务
		// 相比简化的 ForwardOutgoingContext，AnnotateContext 还处理了 header 校验、二进制 header、
		// timeout 传播等关键逻辑，与 grpc-gateway 静态生成的 handler 行为一致
		annotatedCtx, err := runtime.AnnotateContext(ctx, mux, r, fullMethodName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("构建 gRPC context 失败: %v", err))
			return
		}

		// 6. 调用 gRPC 方法
		outputMsg := dynamicpb.NewMessage(outputType)
		err = conn.Invoke(annotatedCtx, fullMethodName, inputMsg, outputMsg)
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				httpStatus := grpcStatusToHTTP(st.Code())
				writeError(w, httpStatus, st.Message())
			} else {
				writeError(w, http.StatusInternalServerError, fmt.Sprintf("gRPC 调用失败: %v", err))
			}
			return
		}

		// 8. 序列化响应（复用 package 级 marshaler，避免每次请求创建）
		w.Header().Set("Content-Type", "application/json")
		data, err := defaultJSONPb.Marshal(outputMsg)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("序列化响应失败: %v", err))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(data)
	}
}

// =============================================================================
// 自动注册入口
// =============================================================================

// AutoRegister 一站式自动注册：连接 gRPC server + reflection 发现服务 + 动态注册 HTTP handler
//
// 业务层只需调用此函数，无需写任何注册代码
// 前提: gRPC server 需要启用 reflection (reflection.Register(server))
func AutoRegister(
	ctx context.Context,
	healthChecker *HealthChecker,
	mux *runtime.ServeMux,
	clients map[string]*gwconfig.GRPCClient,
) *AutoRegisterResult {
	// 1. 初始化所有配置的 gRPC 连接并发现服务
	clientNames := InitConnectionsAndDiscover(ctx, healthChecker, clients)

	// 2. 动态注册 HTTP handler
	handlerNames := []string{}
	if len(clientNames) > 0 {
		dynamicHandlers, err := RegisterDynamicHandlers(ctx, mux, clients)
		if err != nil {
			gwglobal.LOGGER.Warn("gRPC reflection 动态注册失败: %v", err)
		} else {
			handlerNames = dynamicHandlers
			gwglobal.LOGGER.Info("✅ gRPC reflection 动态注册 %d 个 HTTP handler", len(handlerNames))
		}
	}

	return &AutoRegisterResult{
		Clients:       clientNames,
		Handlers:      handlerNames,
		TotalClients:  len(clients),
		TotalHandlers: len(handlerNames),
		SkippedManual: 0,
	}
}

// InitConnectionsAndDiscover 初始化所有配置的 gRPC 连接并通过 reflection 发现服务
// 不注册 HTTP handler，handler 注册由调用方负责（支持重放）
func InitConnectionsAndDiscover(ctx context.Context, healthChecker *HealthChecker, clients map[string]*gwconfig.GRPCClient) []string {
	// 1. 初始化所有配置的 gRPC 连接
	clientNames := initAllConnections(ctx, healthChecker, clients)

	// 2. 通过 gRPC Server Reflection 自动发现服务
	if len(clientNames) > 0 {
		gwglobal.LOGGER.Info("🔍 开始通过 gRPC Server Reflection 自动发现服务...")
		DiscoverAllServices(ctx, clients)
	}

	return clientNames
}

// =============================================================================
// 辅助函数
// =============================================================================

// grpcStatusToHTTP 将 gRPC 状态码转换为 HTTP 状态码
func grpcStatusToHTTP(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return 499
	case codes.Unknown:
		return http.StatusInternalServerError
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.FailedPrecondition:
		return http.StatusBadRequest
	case codes.Aborted:
		return http.StatusConflict
	case codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Internal:
		return http.StatusInternalServerError
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DataLoss:
		return http.StatusInternalServerError
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprintf(w, `{"error":%q}`, message)
}
