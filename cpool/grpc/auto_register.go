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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/grpc-ecosystem/grpc-gateway/v2/utilities"
	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	gwglobal "github.com/kamalyes/go-rpc-gateway/global"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection/grpc_reflection_v1"
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
	mu         sync.RWMutex
	services   map[string][]ReflectionServiceInfo // serviceName -> services
	files      *protoregistry.Files               // proto 文件注册表（指向 GlobalFiles）
	fileHashes map[string]string                  // fileName -> SHA256 hex，用于检测版本不一致

	// fingerprints serviceName -> 契约指纹，只在该服务注册成功后写入
	//
	// 这个"只在成功后写入"是个不变式：指纹命中 ⟺ 当前 mux 里已经有这份契约对应的路由
	// 若在 discovery 成功、注册失败时也写入，服务下次上报同样的指纹就会被当成"已注册"
	// 而跳过，路由永久缺失
	// [EN] Written only after a successful registration, so a fingerprint hit always implies
	// the routes for that exact contract are already in the mux
	fingerprints map[string]string

	// pending serviceName -> 已算出但尚未注册完成的指纹
	//
	// 启动路径是分两步的：DiscoverAllServices 只发现（此时 mux 还没建），
	// 随后 RegisterDynamicHandlers 才注册指纹必须在第二步成功后才能生效，
	// 否则一旦注册失败，指纹已经落库，后续 reconcile 会认为"已注册"而永久跳过
	// [EN] Startup discovers before the mux exists and registers afterwards, so the
	// fingerprint may only take effect once the second step succeeds
	pending map[string]string

	// stale serviceName -> 已检测到 proto 变更但无法热加载的信息
	//
	// registerFileDescriptors 发现同名文件内容变了只能打一条日志，然后把哈希基准更新掉
	// 以免重复告警 —— 代价是"当前跑的是旧契约"这个事实在下一轮比对后就查不到了
	// 日志会滚走，这个状态不会：留在这里供健康检查/运维接口读取，
	// 静默提供旧契约比直接报错更危险
	// [EN] A changed file cannot be hot-reloaded (GlobalFiles has no Unregister), and the hash
	// baseline is updated to avoid repeating the warning — which would otherwise erase the fact
	// that this process serves an outdated contract. Kept here because logs roll away
	stale map[string]StaleInfo

	initialized bool
}{
	services:     make(map[string][]ReflectionServiceInfo),
	files:        protoregistry.GlobalFiles,
	fileHashes:   make(map[string]string),
	fingerprints: make(map[string]string),
	pending:      make(map[string]string),
	stale:        make(map[string]StaleInfo),
}

// StaleInfo 描述一个服务被检测到、但网关无法热加载的 proto 变更
type StaleInfo struct {
	ServiceName string    // 服务名（配置中的 key）
	Files       []string  // 内容已变更的 proto 文件
	DetectedAt  time.Time // 首次检测到的时间
	Count       int       // 累计检测次数
}

// routeRegistry 存储已注册的 HTTP 路由
//
// seen 不是洁癖：每次 reconcile 都无条件 append 的话，一个反复重连的下游服务
// 会让这个切片无界增长（mux 侧没有同样的问题，ServeMux.Handle 是前插，
// 重复注册会被新的那条盖住）
// [EN] seen keeps the slice from growing without bound when a flapping service is
// re-registered repeatedly; the mux itself is fine because Handle prepends
var routeRegistry = &struct {
	mu     sync.RWMutex
	routes []HTTPRoute
	seen   map[string]struct{} // "METHOD PATH" -> 已记录
}{
	seen: make(map[string]struct{}),
}

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
// 返回副本：调用方拿到切片后，动态注册仍可能继续向 routeRegistry 追加
// [EN] Returns a copy, since dynamic registration may keep appending after the caller returns
func GetRoutes() []HTTPRoute {
	routeRegistry.mu.RLock()
	defer routeRegistry.mu.RUnlock()
	if len(routeRegistry.routes) == 0 {
		return nil
	}
	out := make([]HTTPRoute, len(routeRegistry.routes))
	copy(out, routeRegistry.routes)
	return out
}

// GetStaleServices 返回检测到 proto 契约变更、但网关无法热加载的服务
//
// 这些服务当前走的仍是网关进程启动时加载的旧 descriptor：新增/删除字段、改类型
// 对网关不可见，必须重启网关进程才能生效暴露出来是为了让这件事可被监控，
// 而不是只躺在一条滚走的日志里
// [EN] These services still resolve against the descriptors loaded at process start;
// only a restart picks up the new ones. Exposed so it can be monitored
func GetStaleServices() []StaleInfo {
	reflectionRegistry.mu.RLock()
	defer reflectionRegistry.mu.RUnlock()
	if len(reflectionRegistry.stale) == 0 {
		return nil
	}
	out := make([]StaleInfo, 0, len(reflectionRegistry.stale))
	for _, info := range reflectionRegistry.stale {
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ServiceName < out[j].ServiceName })
	return out
}

// ClearRegistry 清空 reflection 发现的服务和路由（主要用于测试，不清空 GlobalFiles）
func ClearRegistry() {
	reflectionRegistry.mu.Lock()
	reflectionRegistry.services = make(map[string][]ReflectionServiceInfo)
	reflectionRegistry.fileHashes = make(map[string]string)
	reflectionRegistry.fingerprints = make(map[string]string)
	reflectionRegistry.pending = make(map[string]string)
	reflectionRegistry.stale = make(map[string]StaleInfo)
	reflectionRegistry.initialized = false
	reflectionRegistry.mu.Unlock()

	routeRegistry.mu.Lock()
	routeRegistry.routes = nil
	routeRegistry.seen = make(map[string]struct{})
	routeRegistry.mu.Unlock()
}

// =============================================================================
// 注册串行化与契约指纹
// =============================================================================

// muxRegisterMu 串行化所有对 runtime.ServeMux 的注册
//
// ServeMux.handlers 是 map[string][]handler，Handle/HandlePath 直接读写它，没有任何
// 互斥保护 —— RegisterDynamicHandlers 的 Phase 2 已经为此串行化过但漏了另一条路：
// onRecover 是在 HealthChecker 的周期检查里「每服务一个 goroutine」的上下文中调用的
// （health.go 的 go func(...) 内 callback(name)），两个下游服务同时恢复，就是两个
// goroutine 同时往同一个 mux 上 HandlePath —— Go map 的并发写，直接 panic 掉整个进程
// ServiceWatcher 又加入了第三条触发路径，所以把三条全部收口到这把锁上
//
// 只锁注册本身，不锁 discovery：reflection 要走网络，把它圈进来会让一个慢服务
// 卡住其他服务的注册
// [EN] Serializes every write to the ServeMux. Phase 2 of RegisterDynamicHandlers already
// did this locally, but the onRecover path did not: it runs inside the health checker's
// per-service goroutines, so two services recovering together wrote the same map
// concurrently and panicked the process. Only registration is inside the lock —
// discovery does network I/O and would make one slow service block the others
var muxRegisterMu sync.Mutex

// addRoutes 把新注册的路由并入 routeRegistry，按 "METHOD PATH" 去重
func addRoutes(routes []HTTPRoute) {
	if len(routes) == 0 {
		return
	}
	routeRegistry.mu.Lock()
	defer routeRegistry.mu.Unlock()
	if routeRegistry.seen == nil {
		routeRegistry.seen = make(map[string]struct{}, len(routes))
	}
	for _, route := range routes {
		key := route.HTTPMethod + " " + route.HTTPPath
		if _, dup := routeRegistry.seen[key]; dup {
			continue
		}
		routeRegistry.seen[key] = struct{}{}
		routeRegistry.routes = append(routeRegistry.routes, route)
	}
}

// registerAndTrackRoutes 在 muxRegisterMu 保护下注册某个服务的路由并记账
func registerAndTrackRoutes(ctx context.Context, mux *runtime.ServeMux, serviceName string, svcInfos []ReflectionServiceInfo) []string {
	muxRegisterMu.Lock()
	registered := registerClientHandlers(ctx, mux, serviceName, svcInfos)
	muxRegisterMu.Unlock()

	addRoutes(collectRoutes(registered))
	return registered
}

// contractFingerprint 计算一个服务当前 proto 契约的指纹
//
// 输入是 reflection 这一轮返回的全部文件名 + 各自内容哈希，外加服务名列表，
// 所以新增/删除文件、改字段、给已有 service 加一个 RPC 或一条 google.api.http 注解，
// 都会让指纹变化指纹相同就代表这一轮什么都不用做 —— 稳态下 reconcile 的成本
// 到此为止：一次 reflection 往返 + 若干次 SHA256，不碰 mux、不写注册表、不打日志
// [EN] Fingerprints the contract from the file hashes plus the service list, so any proto
// change moves it. An unchanged fingerprint ends the reconcile right there: in steady state
// the cost is one reflection round trip plus a few SHA256s, with no mux or registry writes
func contractFingerprint(svcInfos []ReflectionServiceInfo, fileCache map[string]*descriptorpb.FileDescriptorProto) string {
	names := make([]string, 0, len(fileCache))
	for name := range fileCache {
		names = append(names, name)
	}
	sort.Strings(names)

	h := sha256.New()
	for _, name := range names {
		hash, err := computeFileHash(fileCache[name])
		if err != nil {
			// 算不出哈希就让指纹不可比：返回空串，调用方按"有变化"处理，
			// 宁可多注册一次，也不要因为一个文件出错而永久跳过变更检测
			// [EN] An unhashable file makes the fingerprint incomparable: re-register rather
			// than risk skipping real changes forever
			return ""
		}
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write([]byte(hash))
		h.Write([]byte{0})
	}

	svcNames := make([]string, 0, len(svcInfos))
	for _, info := range svcInfos {
		svcNames = append(svcNames, info.ServiceName)
	}
	sort.Strings(svcNames)
	for _, name := range svcNames {
		h.Write([]byte(name))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// loadFingerprint 读取某服务上次注册成功时的契约指纹
func loadFingerprint(serviceName string) (string, bool) {
	reflectionRegistry.mu.RLock()
	defer reflectionRegistry.mu.RUnlock()
	fp, ok := reflectionRegistry.fingerprints[serviceName]
	return fp, ok
}

// storeFingerprint 记录某服务注册成功后的契约指纹
// 空指纹（计算失败）不记录，避免把"不可比"固化成"已注册"
func storeFingerprint(serviceName, fingerprint string) {
	if fingerprint == "" {
		return
	}
	reflectionRegistry.mu.Lock()
	reflectionRegistry.fingerprints[serviceName] = fingerprint
	reflectionRegistry.mu.Unlock()
}

// stagePendingFingerprint 暂存启动期算出的指纹，等注册成功后由 commitPendingFingerprints 提交
func stagePendingFingerprint(serviceName, fingerprint string) {
	if fingerprint == "" {
		return
	}
	reflectionRegistry.mu.Lock()
	if reflectionRegistry.pending == nil {
		reflectionRegistry.pending = make(map[string]string)
	}
	reflectionRegistry.pending[serviceName] = fingerprint
	reflectionRegistry.mu.Unlock()
}

// commitPendingFingerprints 把暂存的指纹提升为已注册指纹
func commitPendingFingerprints(serviceNames []string) {
	reflectionRegistry.mu.Lock()
	defer reflectionRegistry.mu.Unlock()
	for _, name := range serviceNames {
		if fp, ok := reflectionRegistry.pending[name]; ok {
			reflectionRegistry.fingerprints[name] = fp
			delete(reflectionRegistry.pending, name)
		}
	}
}

// markStale 记录某服务存在无法热加载的 proto 变更
func markStale(serviceName string, files []string) {
	if len(files) == 0 {
		return
	}
	sort.Strings(files)
	reflectionRegistry.mu.Lock()
	defer reflectionRegistry.mu.Unlock()
	info, exists := reflectionRegistry.stale[serviceName]
	if !exists {
		info = StaleInfo{ServiceName: serviceName, DetectedAt: time.Now()}
	}
	info.Files = files
	info.Count++
	reflectionRegistry.stale[serviceName] = info
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
// 返回新注册数、跳过数和内容已变更（无法热加载）的文件名列表
//
// 版本不一致检测：通过比较 FileDescriptorProto 的 SHA256 哈希判断 reflection 获取的
// proto 文件是否与上次注册的版本相同若不一致，说明服务发版后 proto 定义已变更，
// 但 protoregistry.GlobalFiles/GlobalTypes 不可变（无 Unregister API），运行时无法
// 热替换，只能告警提示重启网关进程以加载新版本
func registerFileDescriptors(fileDescriptors map[string]*descriptorpb.FileDescriptorProto) (registered, skipped int, mismatched []string) {
	if len(fileDescriptors) == 0 {
		return
	}

	// 预计算所有文件的哈希，用于版本一致性检测
	type fileEntry struct {
		fdp  *descriptorpb.FileDescriptorProto
		hash string // 空串表示计算失败（降级为原逻辑，不检测）
	}
	entries := make(map[string]*fileEntry, len(fileDescriptors))
	for name, fdp := range fileDescriptors {
		hash, err := computeFileHash(fdp)
		if err != nil {
			gwglobal.LOGGER.Warn("registerFileDescriptors: 计算文件 %s 哈希失败: %v", name, err)
		}
		entries[name] = &fileEntry{fdp: fdp, hash: hash}
	}

	// 第一趟：过滤已在 GlobalFiles 中的文件，同时检测版本不一致
	remaining := make(map[string]*descriptorpb.FileDescriptorProto, len(entries))
	reflectionRegistry.mu.Lock()
	for name, entry := range entries {
		if _, err := protoregistry.GlobalFiles.FindFileByPath(name); err == nil {
			// 文件已在 GlobalFiles 中
			storedHash, hasStored := reflectionRegistry.fileHashes[name]
			if entry.hash != "" {
				if hasStored && storedHash != entry.hash {
					// 版本不一致：已注册的 proto 文件与 reflection 获取的版本不同
					mismatched = append(mismatched, name)
					gwglobal.LOGGER.Error("⚠️ proto 文件版本不一致: %s (已注册: %.8s, 当前: %.8s)，GlobalFiles 不可变，请重启网关以加载新版本",
						name, storedHash, entry.hash)
				}
				// 更新哈希基准（即使不一致也更新，避免重复告警同一文件）
				reflectionRegistry.fileHashes[name] = entry.hash
			}
			skipped++
			continue
		}
		remaining[name] = entry.fdp
		// 预存哈希，注册成功后即为后续比对的基准版本
		if entry.hash != "" {
			reflectionRegistry.fileHashes[name] = entry.hash
		}
	}
	reflectionRegistry.mu.Unlock()

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

// computeFileHash 计算 FileDescriptorProto 的内容哈希，用于版本一致性检测
// 对序列化后的二进制内容取 SHA256，能感知字段增删改、类型变更等任何 proto 定义变化
//
// 必须用 Deterministic：protobuf 不保证 Marshal 输出字节稳定（同一消息两次序列化
// 可以得到不同字节序），哈希拿它做输入就会凭空"变更"，把整套热注册反复触发一遍
// [EN] Deterministic is required: protobuf does not guarantee byte-stable output, and a
// non-stable hash input would report phantom changes and re-trigger registration forever
func computeFileHash(fdp *descriptorpb.FileDescriptorProto) (string, error) {
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(fdp)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
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

			// 指纹要在合并进 allFileDescriptors 之前按服务各自算，
			// 合并后就分不清哪个文件属于哪个服务了
			// [EN] Fingerprint per service before merging, which loses file ownership
			stagePendingFingerprint(name, contractFingerprint(services, fileCache))

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
	registered, skipped, mismatched := registerFileDescriptors(allFileDescriptors)

	reflectionRegistry.mu.Lock()
	reflectionRegistry.services = servicesMap
	reflectionRegistry.files = protoregistry.GlobalFiles
	reflectionRegistry.initialized = true
	reflectionRegistry.mu.Unlock()

	if len(mismatched) > 0 {
		gwglobal.LOGGER.ErrorContext(ctx, "⚠️ reflection: 检测到 %d 个 proto 文件版本不一致，请重启网关以加载新版本: %v", len(mismatched), mismatched)
	}
	gwglobal.LOGGER.InfoContext(ctx, "✅ reflection: 新注册 %d 个 proto 文件，跳过 %d 个已注册文件，发现 %d 个服务组", registered, skipped, len(servicesMap))
}

// discoverServices 通过 gRPC Server Reflection 发现单个 gRPC server 的所有服务
// 返回服务列表和所有相关的 FileDescriptorProto（含依赖文件）
// 使用流水线模式：批量发送所有 FileContainingSymbol 请求后批量接收响应，将 N 次 RTT 降为 1 次
func discoverServices(ctx context.Context, conn *grpc.ClientConn) ([]ReflectionServiceInfo, map[string]*descriptorpb.FileDescriptorProto, error) {
	client := grpc_reflection_v1.NewServerReflectionClient(conn)
	stream, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("创建 reflection stream 失败: %w", err)
	}
	defer stream.CloseSend()

	// 1. 获取服务列表
	if sendErr := stream.Send(&grpc_reflection_v1.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_ListServices{},
	}); sendErr != nil {
		// gRPC 规范：Send 返回 io.EOF 表示流已被服务端终止，需调用 Recv 获取真实错误
		if sendErr == io.EOF {
			if _, recvErr := stream.Recv(); recvErr != nil && recvErr != io.EOF {
				sendErr = recvErr
			}
		}
		return nil, nil, fmt.Errorf("发送 ListServices 请求失败: %w", sendErr)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, nil, fmt.Errorf("接收 ListServices 响应失败: %w", err)
	}

	listResp, ok := resp.MessageResponse.(*grpc_reflection_v1.ServerReflectionResponse_ListServicesResponse)
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
		if sendErr := stream.Send(&grpc_reflection_v1.ServerReflectionRequest{
			MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: svcName,
			},
		}); sendErr != nil {
			failed[svcName] = true
			gwglobal.LOGGER.WarnContext(ctx, "发送 FileContainingSymbol 请求失败 %s: %v", svcName, sendErr)
		}
	}

	// 批量接收所有响应（与发送顺序一一对应）
	for _, svcName := range serviceNames {
		if failed[svcName] {
			continue
		}
		fdResp, recvErr := stream.Recv()
		if recvErr != nil {
			failed[svcName] = true
			gwglobal.LOGGER.WarnContext(ctx, "接收 FileContainingSymbol 响应失败 %s: %v", svcName, recvErr)
			continue
		}
		if parseErr := parseFileDescriptorResponse(fdResp, fileCache); parseErr != nil {
			failed[svcName] = true
			gwglobal.LOGGER.WarnContext(ctx, "解析 FileDescriptor 失败 %s: %v", svcName, parseErr)
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
func parseFileDescriptorResponse(resp *grpc_reflection_v1.ServerReflectionResponse, cache map[string]*descriptorpb.FileDescriptorProto) error {
	fdResp, ok := resp.MessageResponse.(*grpc_reflection_v1.ServerReflectionResponse_FileDescriptorResponse)
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

	// 本轮处理到的服务，注册走完后提交其暂存指纹，使 ServiceWatcher 的
	// 首次兜底轮询不会把启动时已注册的路由重复注册一遍
	// [EN] Collected so their staged fingerprints can be committed after registration,
	// which keeps the watcher's first poll from re-registering what startup already did
	handledServices := make([]string, 0, len(services))

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
		handledServices = append(handledServices, serviceName)

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
	// 锁是包级的 muxRegisterMu，而不是本函数内的局部串行：并发的另一端是
	// onRecover 与 ServiceWatcher，它们注册的是同一个 mux
	// [EN] The lock is package-level because the other side of the race is onRecover and
	// ServiceWatcher registering into the same mux, not another call to this function
	var registered []string
	var newRoutes []HTTPRoute
	muxRegisterMu.Lock()
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
	muxRegisterMu.Unlock()

	// 缓存路由
	addRoutes(newRoutes)
	commitPendingFingerprints(handledServices)

	sort.Strings(registered)
	return registered, nil
}

// RediscoverAndRegisterService 重新发现并注册单个服务
// 用于服务恢复健康后，重新通过 reflection 发现服务并注册 HTTP 路由
// 包含重试机制（3 次），避免服务刚恢复时 reflection 瞬态失败导致路由永久丢失
func RediscoverAndRegisterService(ctx context.Context, mux *runtime.ServeMux, serviceName string) error {
	gwglobal.LOGGER.InfoContext(ctx, "🔄 服务 %s 恢复健康，重新发现并注册...", serviceName)
	_, err := reconcileServiceWithRetry(ctx, mux, serviceName, 3)
	return err
}

// reconcileServiceWithRetry 带重试的 reconcile
// 重试只针对 discovery 失败：服务刚恢复时 reflection 可能瞬态失败，
// 不重试就意味着这一版的新路由永久缺失
// [EN] Retries only cover discovery failures; without them a transient reflection error
// right after recovery would drop this version's new routes for good
func reconcileServiceWithRetry(ctx context.Context, mux *runtime.ServeMux, serviceName string, maxAttempts int) (reconcileOutcome, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			delay := time.Duration(attempt) * time.Second
			gwglobal.LOGGER.ErrorContext(ctx, "🔄 服务 %s 重新发现第 %d 次尝试（等待 %v）...", serviceName, attempt, delay)
			select {
			case <-ctx.Done():
				return reconcileOutcome{}, ctx.Err()
			case <-time.After(delay):
			}
		}
		outcome, err := reconcileService(ctx, mux, serviceName)
		if err == nil {
			return outcome, nil
		}
		lastErr = err
		gwglobal.LOGGER.ErrorContext(ctx, "⚠️ 服务 %s 重新发现第 %d/%d 次失败: %v", serviceName, attempt, maxAttempts, err)
	}
	return reconcileOutcome{}, fmt.Errorf("服务 %s 重新发现失败（已重试 %d 次）: %w", serviceName, maxAttempts, lastErr)
}

// reconcileOutcome 一次 reconcile 的结果
type reconcileOutcome struct {
	Changed     bool     // 契约指纹变化并完成了重注册
	Fingerprint string   // 本轮契约指纹（空串表示不可比）
	Stale       []string // 检测到内容变更但无法热加载的 proto 文件
	Routes      []string // 本轮注册的 HTTP 路由
}

// reconcileService 把单个服务的 proto 契约与网关当前的注册状态对齐
//
// 先取指纹再决定做不做事，是这条路径能被高频调用的前提：契约没变就在第 2 步返回，
// 既不碰 mux 也不写注册表无条件重注册会有两个实际代价 ——
// ServeMux.Handle 是前插，每次重复注册都会在 handlers 切片里压进一条永不被命中的
// 旧 handler（一个反复重连的服务就是一条稳定增长的内存泄漏）；而重复的 reflection
// 注册日志会把真正的变更淹掉
// [EN] Fingerprinting first is what makes this path safe to call often: an unchanged contract
// returns at step 2 without touching the mux or the registries. Unconditional re-registration
// leaks: Handle prepends, so each repeat buries another handler that will never be reached
func reconcileService(ctx context.Context, mux *runtime.ServeMux, serviceName string) (reconcileOutcome, error) {
	// 1. 重新发现服务
	services, fileCache, err := discoverSingleClient(ctx, serviceName)
	if err != nil {
		return reconcileOutcome{}, fmt.Errorf("重新发现服务 %s 失败: %w", serviceName, err)
	}

	// 2. 指纹比对：契约未变则直接返回
	fingerprint := contractFingerprint(services, fileCache)
	if fingerprint != "" {
		if prev, ok := loadFingerprint(serviceName); ok && prev == fingerprint {
			return reconcileOutcome{Changed: false, Fingerprint: fingerprint}, nil
		}
	}

	outcome := reconcileOutcome{Changed: true, Fingerprint: fingerprint}

	// 3. 注册 FileDescriptorProto
	registered, skipped, mismatched := registerFileDescriptors(fileCache)
	if len(mismatched) > 0 {
		outcome.Stale = mismatched
		markStale(serviceName, mismatched)
		gwglobal.LOGGER.ErrorContext(ctx, "⚠️ reflection: 服务 %s 检测到 %d 个 proto 文件版本不一致，请重启网关以加载新版本: %v",
			serviceName, len(mismatched), mismatched)
	}
	gwglobal.LOGGER.InfoContext(ctx, "✅ reflection: 服务 %s 新注册 %d 个 proto 文件，跳过 %d 个已注册文件，发现 %d 个 gRPC 服务",
		serviceName, registered, skipped, len(services))

	// 4. 更新 reflectionRegistry
	reflectionRegistry.mu.Lock()
	reflectionRegistry.services[serviceName] = services
	reflectionRegistry.files = protoregistry.GlobalFiles
	reflectionRegistry.initialized = true
	reflectionRegistry.mu.Unlock()

	// 5. 注册 HTTP 路由
	outcome.Routes = registerAndTrackRoutes(ctx, mux, serviceName, services)
	if len(outcome.Routes) > 0 {
		gwglobal.LOGGER.InfoContext(ctx, "✅ 服务 %s 重新注册 %d 个 HTTP 路由: %v", serviceName, len(outcome.Routes), outcome.Routes)
	} else {
		gwglobal.LOGGER.InfoContext(ctx, "✅ 服务 %s 无 HTTP 路由需要注册", serviceName)
	}

	// 6. 指纹只在注册走完后才记录，保证"指纹命中 ⟹ 路由已在 mux 里"
	storeFingerprint(serviceName, fingerprint)
	return outcome, nil
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
				writeError(w, codes.InvalidArgument, "failed to read request body")
				return
			}
			defer r.Body.Close()

			if len(bodyData) > 0 {
				if route.BodyField == "*" {
					// 整个 body 映射到消息
					if err := protojson.Unmarshal(bodyData, inputMsg); err != nil {
						writeError(w, codes.InvalidArgument, fmt.Sprintf("failed to parse request body: %v", err))
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
								writeError(w, codes.InvalidArgument, fmt.Sprintf("failed to parse request body field: %v", err))
								return
							}
							inputMsg.Set(field, protoreflect.ValueOfMessage(fieldMsg))
						} else {
							// scalar/bytes/enum 类型字段：body 是该字段的 JSON 值
							// 构造 {"field": <body>} 交给 protojson 解析，bytes 字段会自动 base64 解码
							wrappedJSON := fmt.Sprintf(`{%q: %s}`, route.BodyField, bodyData)
							if err := protojson.Unmarshal([]byte(wrappedJSON), inputMsg); err != nil {
								writeError(w, codes.InvalidArgument, fmt.Sprintf("failed to parse request body field: %v", err))
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
			writeError(w, codes.Internal, fmt.Sprintf("failed to build gRPC context: %v", err))
			return
		}

		// 6. 调用 gRPC 方法
		outputMsg := dynamicpb.NewMessage(outputType)
		err = conn.Invoke(annotatedCtx, fullMethodName, inputMsg, outputMsg)
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				writeError(w, st.Code(), st.Message())
			} else {
				writeError(w, codes.Internal, fmt.Sprintf("gRPC call failed: %v", err))
			}
			return
		}

		// 8. 序列化响应（复用 package 级 marshaler，避免每次请求创建）
		w.Header().Set("Content-Type", "application/json")
		data, err := defaultJSONPb.Marshal(outputMsg)
		if err != nil {
			writeError(w, codes.Internal, fmt.Sprintf("failed to marshal response: %v", err))
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

// writeError 写入错误响应，统一使用 gRPC 标准格式 {"code":N,"error":"...","status":"..."}
func writeError(w http.ResponseWriter, code codes.Code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(grpcStatusToHTTP(code))
	fmt.Fprintf(w, `{"code":%d,"error":%q,"status":%q}`, code, message, code.String())
}
