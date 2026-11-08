/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-02-08 01:26:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2024-05-08 02:00:18
 * @FilePath: \go-rpc-gateway\cpool\grpc\watcher_test.go
 * @Description: 服务发版感知测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package grpc

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gwruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// =============================================================================
// 测试辅助
// =============================================================================

// syntheticPkgSeq 保证每个合成 proto 包名在同一个测试二进制里唯一
// protoregistry.GlobalFiles 没有 Unregister，重名注册会失败
var syntheticPkgSeq atomic.Int64

// registerSyntheticHTTPService 在 GlobalFiles 中注册一个带 google.api.http 注解的服务
//
// 合成而不是复用现成的 proto：reflection 能发现的 grpc 自带服务（health / channelz）
// 都没有 http 注解，extractHTTPRoutes 对它们返回 0 条路由，于是整条注册链路根本不会
// 碰到 mux —— 用它们测并发注册等于什么都没测
// [EN] Synthesized rather than reused: gRPC's own discoverable services carry no http
// annotations, so route extraction yields nothing and the mux is never written — which would
// make a concurrency test prove nothing
func registerSyntheticHTTPService(t *testing.T, methodCount int) (fullName string, paths []string) {
	t.Helper()

	pkg := fmt.Sprintf("watchertest%d", syntheticPkgSeq.Add(1))
	svc := &descriptorpb.ServiceDescriptorProto{Name: proto.String("Svc")}
	for i := 0; i < methodCount; i++ {
		path := fmt.Sprintf("/%s/m%d", pkg, i)
		opts := &descriptorpb.MethodOptions{}
		proto.SetExtension(opts, annotations.E_Http, &annotations.HttpRule{
			Pattern: &annotations.HttpRule_Get{Get: path},
		})
		svc.Method = append(svc.Method, &descriptorpb.MethodDescriptorProto{
			Name:       proto.String(fmt.Sprintf("M%d", i)),
			InputType:  proto.String("." + pkg + ".Req"),
			OutputType: proto.String("." + pkg + ".Resp"),
			Options:    opts,
		})
		paths = append(paths, path)
	}

	fdp := &descriptorpb.FileDescriptorProto{
		Name:       proto.String(pkg + "/svc.proto"),
		Package:    proto.String(pkg),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"google/api/annotations.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("Req")},
			{Name: proto.String("Resp")},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{svc},
	}

	fd, err := protodesc.NewFile(fdp, protoregistry.GlobalFiles)
	require.NoError(t, err)
	require.NoError(t, protoregistry.GlobalFiles.RegisterFile(fd))

	return pkg + ".Svc", paths
}

// startReflectionServerAt 在指定地址启动一个只提供 reflection 的 gRPC 服务器，
// 并把给定的服务名注册进去（无需真实实现：ListServices 读的是 GetServiceInfo，
// FileContainingSymbol 从 GlobalFiles 解析）
func startReflectionServerAt(t *testing.T, addr string, serviceNames ...string) (actualAddr string, stop func()) {
	t.Helper()

	lis, err := net.Listen("tcp", addr)
	require.NoError(t, err)

	server := grpc.NewServer()
	reflection.Register(server)
	for _, name := range serviceNames {
		server.RegisterService(&grpc.ServiceDesc{ServiceName: name}, nil)
	}
	go func() { _ = server.Serve(lis) }()

	return lis.Addr().String(), server.Stop
}

// dialTestConn 建立到测试服务器的连接并放入连接池，测试结束时清理
func dialTestConn(t *testing.T, addr string, poolNames ...string) *grpc.ClientConn {
	t.Helper()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	for _, name := range poolNames {
		PutConn(name, conn)
	}

	t.Cleanup(func() {
		connPoolMu.Lock()
		for _, name := range poolNames {
			delete(connPool, name)
		}
		connPoolMu.Unlock()
		conn.Close()
	})

	return conn
}

// nudgeConn 后台周期性调用 Connect()，模拟生产环境里 HealthChecker 的作用
// （health.go 的周期检查每 3s 会 Connect + ResetConnectBackoff）
// 没有它，空闲的 ClientConn 在对端消失后不一定自行重连，测试会变得不确定
func nudgeConn(t *testing.T, conn *grpc.ClientConn) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				conn.ResetConnectBackoff()
				conn.Connect()
			}
		}
	}()
	t.Cleanup(func() { close(done) })
}

// freshMuxFn 返回一个 MuxFn（NewServeMux 是变参函数，不能直接当 func() *ServeMux 用）
func freshMuxFn() func() *gwruntime.ServeMux {
	mux := gwruntime.NewServeMux()
	return func() *gwruntime.ServeMux { return mux }
}

// =============================================================================
// 契约指纹
// =============================================================================

// TestComputeFileHash_Deterministic 同一个 FileDescriptorProto 多次取哈希必须一致
// protobuf 不保证 Marshal 字节稳定，哈希必须走 Deterministic，
// 否则契约没变也会被判定为变更，热注册会被反复触发
func TestComputeFileHash_Deterministic(t *testing.T) {
	fdp := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("determinism/a.proto"),
		Package: proto.String("determinism"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{Name: proto.String("M1")},
			{Name: proto.String("M2")},
			{Name: proto.String("M3")},
		},
	}

	first, err := computeFileHash(fdp)
	require.NoError(t, err)

	for i := 0; i < 50; i++ {
		got, err := computeFileHash(fdp)
		require.NoError(t, err)
		require.Equal(t, first, got, "第 %d 次哈希与首次不一致", i)
	}
}

// TestContractFingerprint 指纹对契约变化敏感、对无关变化稳定
func TestContractFingerprint(t *testing.T) {
	fileA := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("fp/a.proto"),
		Package: proto.String("fp"),
		Syntax:  proto.String("proto3"),
	}
	fileB := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("fp/b.proto"),
		Package: proto.String("fp"),
		Syntax:  proto.String("proto3"),
	}

	svcs := []ReflectionServiceInfo{{ServiceName: "fp.A"}}
	base := contractFingerprint(svcs, map[string]*descriptorpb.FileDescriptorProto{"fp/a.proto": fileA})
	require.NotEmpty(t, base)

	t.Run("相同输入指纹相同", func(t *testing.T) {
		again := contractFingerprint(svcs, map[string]*descriptorpb.FileDescriptorProto{"fp/a.proto": fileA})
		assert.Equal(t, base, again)
	})

	t.Run("服务列表顺序不影响指纹", func(t *testing.T) {
		two := []ReflectionServiceInfo{{ServiceName: "fp.A"}, {ServiceName: "fp.B"}}
		reversed := []ReflectionServiceInfo{{ServiceName: "fp.B"}, {ServiceName: "fp.A"}}
		files := map[string]*descriptorpb.FileDescriptorProto{"fp/a.proto": fileA}
		assert.Equal(t, contractFingerprint(two, files), contractFingerprint(reversed, files))
	})

	t.Run("新增服务指纹变化", func(t *testing.T) {
		more := []ReflectionServiceInfo{{ServiceName: "fp.A"}, {ServiceName: "fp.B"}}
		got := contractFingerprint(more, map[string]*descriptorpb.FileDescriptorProto{"fp/a.proto": fileA})
		assert.NotEqual(t, base, got)
	})

	t.Run("新增文件指纹变化", func(t *testing.T) {
		got := contractFingerprint(svcs, map[string]*descriptorpb.FileDescriptorProto{
			"fp/a.proto": fileA,
			"fp/b.proto": fileB,
		})
		assert.NotEqual(t, base, got)
	})

	t.Run("文件内容变化指纹变化", func(t *testing.T) {
		changed := proto.Clone(fileA).(*descriptorpb.FileDescriptorProto)
		changed.MessageType = []*descriptorpb.DescriptorProto{{Name: proto.String("NewMsg")}}
		got := contractFingerprint(svcs, map[string]*descriptorpb.FileDescriptorProto{"fp/a.proto": changed})
		assert.NotEqual(t, base, got)
	})
}

// TestAddRoutes_Dedup 重复路由只记一次
// 不去重的话，一个反复重连的下游服务会让 routeRegistry 无界增长
func TestAddRoutes_Dedup(t *testing.T) {
	ClearRegistry()
	t.Cleanup(ClearRegistry)

	routes := []HTTPRoute{
		{HTTPMethod: "GET", HTTPPath: "/dedup/a"},
		{HTTPMethod: "POST", HTTPPath: "/dedup/b"},
	}

	for i := 0; i < 5; i++ {
		addRoutes(routes)
	}

	got := GetRoutes()
	assert.Len(t, got, 2, "重复注册同一批路由不应增长 routeRegistry")

	addRoutes([]HTTPRoute{{HTTPMethod: "GET", HTTPPath: "/dedup/c"}})
	assert.Len(t, GetRoutes(), 3, "新路由仍应被记录")
}

// TestGetRoutes_ReturnsCopy 调用方修改返回值不应污染注册表
func TestGetRoutes_ReturnsCopy(t *testing.T) {
	ClearRegistry()
	t.Cleanup(ClearRegistry)

	addRoutes([]HTTPRoute{{HTTPMethod: "GET", HTTPPath: "/copy/a"}})

	got := GetRoutes()
	require.Len(t, got, 1)
	got[0].HTTPPath = "/mutated"

	assert.Equal(t, "/copy/a", GetRoutes()[0].HTTPPath)
}

// =============================================================================
// reconcileService
// =============================================================================

// TestReconcileService_FingerprintGate 契约未变时第二次 reconcile 应直接返回
//
// 这是整套感知机制能被高频触发的前提：无条件重注册会让 ServeMux.handlers
// 每轮都压进一条永不被命中的旧 handler
func TestReconcileService_FingerprintGate(t *testing.T) {
	ClearRegistry()
	t.Cleanup(ClearRegistry)

	svcName, paths := registerSyntheticHTTPService(t, 2)
	addr, stop := startReflectionServerAt(t, "localhost:0", svcName)
	defer stop()

	const poolName = "gate-svc"
	conn := dialTestConn(t, addr, poolName)
	waitForConnReady(t, conn, 10*time.Second)

	mux := gwruntime.NewServeMux()
	ctx := context.Background()

	first, err := reconcileService(ctx, mux, poolName)
	require.NoError(t, err)
	assert.True(t, first.Changed, "首次 reconcile 应判定为变更")
	assert.NotEmpty(t, first.Fingerprint)
	assert.Len(t, first.Routes, len(paths), "应注册合成服务的全部路由")

	second, err := reconcileService(ctx, mux, poolName)
	require.NoError(t, err)
	assert.False(t, second.Changed, "契约未变时不应重复注册")
	assert.Equal(t, first.Fingerprint, second.Fingerprint)
	assert.Empty(t, second.Routes)
}

// TestReconcileService_FingerprintNotStoredOnFailure discovery 失败不应留下指纹
// 否则服务恢复后上报同样的契约会被当成"已注册"而永久跳过
func TestReconcileService_FingerprintNotStoredOnFailure(t *testing.T) {
	ClearRegistry()
	t.Cleanup(ClearRegistry)

	const poolName = "fail-svc"
	dialTestConn(t, findFreePort(t), poolName) // 端口无人监听

	_, err := reconcileService(context.Background(), gwruntime.NewServeMux(), poolName)
	require.Error(t, err)

	_, ok := loadFingerprint(poolName)
	assert.False(t, ok, "discovery 失败不应记录指纹")
}

// TestRegisterAndTrackRoutes_Concurrent 并发注册不应 panic
//
// 复现的是修复前的真实崩溃路径：onRecover 是在 HealthChecker 周期检查的
// 「每服务一个 goroutine」里调用的，两个下游服务同时恢复，就是两个 goroutine
// 同时往同一个 ServeMux 的 handlers map 上写 —— Go map 并发写直接打挂进程
func TestRegisterAndTrackRoutes_Concurrent(t *testing.T) {
	ClearRegistry()
	t.Cleanup(ClearRegistry)

	const services = 8
	type svcCase struct {
		poolName string
		svcName  string
		paths    []string
	}

	var cases []svcCase
	var allServiceNames []string
	for i := 0; i < services; i++ {
		svcName, paths := registerSyntheticHTTPService(t, 4)
		cases = append(cases, svcCase{
			poolName: fmt.Sprintf("conc-svc-%d", i),
			svcName:  svcName,
			paths:    paths,
		})
		allServiceNames = append(allServiceNames, svcName)
	}

	addr, stop := startReflectionServerAt(t, "localhost:0", allServiceNames...)
	defer stop()

	poolNames := make([]string, 0, len(cases))
	for _, c := range cases {
		poolNames = append(poolNames, c.poolName)
	}
	conn := dialTestConn(t, addr, poolNames...)
	waitForConnReady(t, conn, 10*time.Second)

	mux := gwruntime.NewServeMux()
	ctx := context.Background()

	var wg sync.WaitGroup
	var registered atomic.Int64
	start := make(chan struct{})
	for _, c := range cases {
		wg.Add(1)
		go func(c svcCase) {
			defer wg.Done()
			<-start // 尽量让所有 goroutine 同时冲进注册
			routes := registerAndTrackRoutes(ctx, mux, c.poolName, []ReflectionServiceInfo{{ServiceName: c.svcName}})
			registered.Add(int64(len(routes)))
		}(c)
	}
	close(start)
	wg.Wait()

	wantRoutes := 0
	for _, c := range cases {
		wantRoutes += len(c.paths)
	}
	assert.Equal(t, int64(wantRoutes), registered.Load(), "并发注册不应丢路由")
	assert.Len(t, GetRoutes(), wantRoutes)
}

// =============================================================================
// ServiceWatcher
// =============================================================================

// TestNewServiceWatcher_NilSafe 参数不全时返回 nil，且 nil 上的方法不 panic
func TestNewServiceWatcher_NilSafe(t *testing.T) {
	assert.Nil(t, NewServiceWatcher(ServiceWatchOptions{Services: []string{"a"}}), "MuxFn 为空应返回 nil")
	assert.Nil(t, NewServiceWatcher(ServiceWatchOptions{MuxFn: freshMuxFn()}), "服务列表为空应返回 nil")

	var w *ServiceWatcher
	assert.NotPanics(t, func() {
		w.Start(context.Background())
		w.Trigger("whatever")
		w.Stop()
	})
}

// TestServiceWatcher_StopReleasesGoroutines Stop 必须收干所有 goroutine
//
// HealthChecker.StartPeriodicCheck 当初就是因为没有退出路径，每个调用过它的测试
// 都留下一个永久 goroutine，并跨测试边界继续探测、制造竞态。新加的监视器不能重犯
func TestServiceWatcher_StopReleasesGoroutines(t *testing.T) {
	before := runtime.NumGoroutine()

	w := NewServiceWatcher(ServiceWatchOptions{
		MuxFn:    freshMuxFn(),
		Services: []string{"absent-a", "absent-b", "absent-c"},
		Interval: 50 * time.Millisecond,
	})
	require.NotNil(t, w)

	w.Start(context.Background())
	time.Sleep(200 * time.Millisecond)
	assert.Greater(t, runtime.NumGoroutine(), before, "监视器应已起 goroutine")

	w.Stop()
	w.Stop() // 可重复调用

	// Stop 内部已 wg.Wait，这里只给 runtime 一点收敛时间
	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	assert.LessOrEqual(t, runtime.NumGoroutine(), before, "Stop 后不应残留 goroutine")
}

// TestServiceWatcher_StartIsIdempotent 重复 Start 只生效一次
func TestServiceWatcher_StartIsIdempotent(t *testing.T) {
	w := NewServiceWatcher(ServiceWatchOptions{
		MuxFn:    freshMuxFn(),
		Services: []string{"absent"},
	})
	require.NotNil(t, w)
	t.Cleanup(w.Stop)

	w.Start(context.Background())
	after := runtime.NumGoroutine()
	w.Start(context.Background())
	time.Sleep(100 * time.Millisecond)

	assert.LessOrEqual(t, runtime.NumGoroutine(), after, "重复 Start 不应再起 goroutine")
}

// TestServiceWatcher_DetectsRedeployedContract 端到端：下游发版新增服务后被感知
//
// 这是整件事的验收点。流程完全对应生产：
//  1. 旧版服务在线，网关完成首次发现与注册（建立契约指纹基准）
//  2. 服务下线、新版上线（同端口，多了一个带 HTTP 注解的服务）
//  3. 连接重建 → watchConn 捕获 Ready 上升沿 → reconcile 发现指纹变化 → 注册新路由
//
// 注意这里 Interval 为 0，即关闭兜底轮询：断言的是事件路径本身有效，
// 而不是"等够 60s 总会发现"
func TestServiceWatcher_DetectsRedeployedContract(t *testing.T) {
	ClearRegistry()
	t.Cleanup(ClearRegistry)

	oldSvc, oldPaths := registerSyntheticHTTPService(t, 2)
	newSvc, newPaths := registerSyntheticHTTPService(t, 3)

	// 1. 旧版上线
	addr, stopOld := startReflectionServerAt(t, "localhost:0", oldSvc)

	const poolName = "redeploy-svc"
	conn := dialTestConn(t, addr, poolName)
	waitForConnReady(t, conn, 10*time.Second)
	nudgeConn(t, conn)

	mux := gwruntime.NewServeMux()
	baseline, err := reconcileService(context.Background(), mux, poolName)
	require.NoError(t, err)
	require.True(t, baseline.Changed)
	require.Len(t, baseline.Routes, len(oldPaths))

	// 2. 启动监视（仅连接事件，不轮询）
	changes := make(chan []string, 4)
	w := NewServiceWatcher(ServiceWatchOptions{
		MuxFn:       func() *gwruntime.ServeMux { return mux },
		Services:    []string{poolName},
		Interval:    0,
		MinInterval: 50 * time.Millisecond,
		OnChange: func(_ string, routes []string) {
			changes <- routes
		},
	})
	require.NotNil(t, w)
	w.Start(context.Background())
	t.Cleanup(w.Stop)

	// 让 watchConn 先完成初始状态采样，避免把启动瞬间当成上升沿
	time.Sleep(300 * time.Millisecond)

	// 3. 发版：旧版下线，新版带着多出来的服务上线（同端口）
	stopOld()
	_, stopNew := startReflectionServerAt(t, addr, oldSvc, newSvc)
	defer stopNew()

	select {
	case routes := <-changes:
		assert.Len(t, routes, len(oldPaths)+len(newPaths), "应注册新旧两个服务的全部路由")
	case <-time.After(30 * time.Second):
		t.Fatal("发版后 30s 内未感知到契约变更")
	}

	// 新版的路由确实进了注册表
	registry := make(map[string]struct{})
	for _, r := range GetRoutes() {
		registry[r.HTTPPath] = struct{}{}
	}
	for _, p := range newPaths {
		assert.Contains(t, registry, p, "新服务的路由 %s 应已注册", p)
	}
}

// TestServiceWatcher_TriggerReconciles 外部 Trigger 应立即触发一次 reconcile
// 这是发布侧想做到秒级感知时该用的入口
func TestServiceWatcher_TriggerReconciles(t *testing.T) {
	ClearRegistry()
	t.Cleanup(ClearRegistry)

	svcName, paths := registerSyntheticHTTPService(t, 2)
	addr, stop := startReflectionServerAt(t, "localhost:0", svcName)
	defer stop()

	const poolName = "trigger-svc"
	conn := dialTestConn(t, addr, poolName)
	waitForConnReady(t, conn, 10*time.Second)

	mux := gwruntime.NewServeMux()
	changes := make(chan []string, 4)
	w := NewServiceWatcher(ServiceWatchOptions{
		MuxFn:       func() *gwruntime.ServeMux { return mux },
		Services:    []string{poolName},
		Interval:    0,
		MinInterval: 50 * time.Millisecond,
		OnChange:    func(_ string, routes []string) { changes <- routes },
	})
	require.NotNil(t, w)
	w.Start(context.Background())
	t.Cleanup(w.Stop)

	w.Trigger(poolName)

	select {
	case routes := <-changes:
		assert.Len(t, routes, len(paths))
	case <-time.After(20 * time.Second):
		t.Fatal("Trigger 后 20s 内未完成 reconcile")
	}
}

// TestServiceWatcher_HealthFilterSkipsPollOnly 健康过滤只作用于兜底轮询
//
// 健康状态由另一条路径以固定间隔刷新，必然滞后于连接事件。若用它过滤事件，
// 连接刚 Ready 时投递的信号会被消费后丢弃，真正的变更要等到下一轮轮询才被发现
func TestServiceWatcher_HealthFilterSkipsPollOnly(t *testing.T) {
	ClearRegistry()
	t.Cleanup(ClearRegistry)

	svcName, paths := registerSyntheticHTTPService(t, 1)
	addr, stop := startReflectionServerAt(t, "localhost:0", svcName)
	defer stop()

	const poolName = "healthfilter-svc"
	conn := dialTestConn(t, addr, poolName)
	waitForConnReady(t, conn, 10*time.Second)

	mux := gwruntime.NewServeMux()
	changes := make(chan []string, 4)
	w := NewServiceWatcher(ServiceWatchOptions{
		MuxFn:       func() *gwruntime.ServeMux { return mux },
		Services:    []string{poolName},
		Interval:    50 * time.Millisecond, // 轮询开着，但一律被健康过滤挡掉
		MinInterval: 20 * time.Millisecond,
		IsHealthy:   func(string) bool { return false },
		OnChange:    func(_ string, routes []string) { changes <- routes },
	})
	require.NotNil(t, w)
	w.Start(context.Background())
	t.Cleanup(w.Stop)

	// 轮询被挡住：一段时间内不应有任何 reconcile 结果
	select {
	case <-changes:
		t.Fatal("服务被判定为不健康时，兜底轮询不应执行 reconcile")
	case <-time.After(500 * time.Millisecond):
	}

	// 事件触发不受健康过滤影响
	w.Trigger(poolName)
	select {
	case routes := <-changes:
		assert.Len(t, routes, len(paths))
	case <-time.After(20 * time.Second):
		t.Fatal("事件触发被健康过滤挡掉了，信号丢失")
	}
}

// TestServiceWatcher_BackoffOnFailure 持续失败时退避间隔应递增且有上限
func TestServiceWatcher_BackoffOnFailure(t *testing.T) {
	w := NewServiceWatcher(ServiceWatchOptions{
		MuxFn:       freshMuxFn(),
		Services:    []string{"absent"},
		Interval:    time.Second,
		MinInterval: 10 * time.Millisecond,
	})
	require.NotNil(t, w)

	// 抖动是 ±10%，用宽松区间断言量级即可
	assert.InDelta(t, float64(time.Second), float64(w.nextInterval(0)), float64(150*time.Millisecond))
	assert.InDelta(t, float64(4*time.Second), float64(w.nextInterval(2)), float64(600*time.Millisecond))

	capped := w.nextInterval(watchMaxBackoffSteps)
	assert.InDelta(t, float64(16*time.Second), float64(capped), float64(2*time.Second))
}

// TestWatchedServiceNames 只取出配置完整的服务
func TestWatchedServiceNames(t *testing.T) {
	names := WatchedServiceNames(nil)
	assert.Empty(t, names)
}

// TestGetStaleServices_Empty 无 stale 记录时返回 nil
func TestGetStaleServices_Empty(t *testing.T) {
	ClearRegistry()
	t.Cleanup(ClearRegistry)
	assert.Nil(t, GetStaleServices())
}

// TestMarkStale 记录无法热加载的 proto 变更
// 这个状态必须可查：日志会滚走，而"网关正在提供旧契约"是个需要被监控的事实
func TestMarkStale(t *testing.T) {
	ClearRegistry()
	t.Cleanup(ClearRegistry)

	markStale("stale-svc", []string{"b.proto", "a.proto"})
	markStale("stale-svc", []string{"a.proto"})

	got := GetStaleServices()
	require.Len(t, got, 1)
	assert.Equal(t, "stale-svc", got[0].ServiceName)
	assert.Equal(t, 2, got[0].Count, "重复检测应累加次数")
	assert.False(t, got[0].DetectedAt.IsZero())
}

// TestServiceWatcher_ConcurrentStartStop Start 与 Stop 并发时不应泄漏 goroutine
//
// 这条路径的真实来源是启动流程与关闭流程分属不同 goroutine：Stop 已经进入 wg.Wait
// 之后 Start 再执行 wg.Add，属于 WaitGroup 未定义行为，且新起的 goroutine 用的是
// 已被取消的 ctx —— 没人收，也永远不会停
func TestServiceWatcher_ConcurrentStartStop(t *testing.T) {
	before := runtime.NumGoroutine()

	for i := 0; i < 20; i++ {
		w := NewServiceWatcher(ServiceWatchOptions{
			MuxFn:       freshMuxFn(),
			Services:    []string{"absent-a", "absent-b"},
			Interval:    10 * time.Millisecond,
			MinInterval: 5 * time.Millisecond,
		})
		require.NotNil(t, w)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); w.Start(context.Background()) }()
		go func() { defer wg.Done(); w.Stop() }()
		wg.Wait()

		// 收尾：无论 Start 是否抢在 Stop 之前，最终都不应再有 goroutine 在跑
		w.Stop()
	}

	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	assert.LessOrEqual(t, runtime.NumGoroutine(), before, "并发 Start/Stop 不应残留 goroutine")
}

// TestServiceWatcher_StopBeforeStart Stop 先于 Start 调用时，Start 不应再起 goroutine
// 否则没有 goroutine 会去消费取消信号，监视器就成了永久泄漏
func TestServiceWatcher_StopBeforeStart(t *testing.T) {
	before := runtime.NumGoroutine()

	w := NewServiceWatcher(ServiceWatchOptions{
		MuxFn:    freshMuxFn(),
		Services: []string{"absent"},
		Interval: 50 * time.Millisecond,
	})
	require.NotNil(t, w)

	w.Stop()
	w.Start(context.Background())
	time.Sleep(200 * time.Millisecond)

	assert.LessOrEqual(t, runtime.NumGoroutine(), before, "Stop 之后 Start 不应起 goroutine")
}

// startHangingReflectionServer 启动一个接受 stream 后永不回包的 reflection 服务器
// 模拟"TCP 连得上但 reflection 不响应"：线程池打满、死锁、中间件吞包都长这样
func startHangingReflectionServer(t *testing.T) string {
	t.Helper()

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	block := make(chan struct{})
	server := grpc.NewServer(grpc.StreamInterceptor(
		func(_ any, _ grpc.ServerStream, _ *grpc.StreamServerInfo, _ grpc.StreamHandler) error {
			<-block // 永不回包，直到测试结束
			return nil
		}))
	reflection.Register(server)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(func() {
		close(block)
		server.Stop()
	})

	return lis.Addr().String()
}

// TestReconcileService_HonorsDeadline reconcile 必须在 ctx 超时时自行返回
//
// 这是 worker 不会被卡死的根据：reflection 走 stream，对端连着但不回包时，
// 没有 deadline 的 Recv 会一直阻塞。worker 是每服务一个 goroutine，
// 它卡住就意味着这个服务从此既收不到事件也不再轮询 —— 静默失联
func TestReconcileService_HonorsDeadline(t *testing.T) {
	ClearRegistry()
	t.Cleanup(ClearRegistry)

	const poolName = "deadline-svc"
	conn := dialTestConn(t, startHangingReflectionServer(t), poolName)
	waitForConnReady(t, conn, 10*time.Second)

	const timeout = 400 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	_, err := reconcileService(ctx, gwruntime.NewServeMux(), poolName)
	elapsed := time.Since(start)

	require.Error(t, err, "对端不回包时 reconcile 应返回错误而不是阻塞")
	assert.GreaterOrEqual(t, elapsed, timeout, "不应早于 deadline 返回")
	assert.Less(t, elapsed, 5*time.Second, "应在 deadline 附近返回，实际耗时 %v", elapsed)
}

// TestServiceWatcher_StopCancelsInFlightReconcile Stop 应打断正在进行的 reconcile
// 不打断的话 Stop 会一直卡在 wg.Wait 上，拖慢（甚至挂住）整个网关的关闭流程
func TestServiceWatcher_StopCancelsInFlightReconcile(t *testing.T) {
	ClearRegistry()
	t.Cleanup(ClearRegistry)

	const poolName = "hang-svc"
	conn := dialTestConn(t, startHangingReflectionServer(t), poolName)
	waitForConnReady(t, conn, 10*time.Second)

	// ReconcileTimeout 故意设得远长于本测试的等待，确保收住 Stop 的是 ctx 取消
	// 而不是超时（超时本身由 TestReconcileService_HonorsDeadline 覆盖）
	w := NewServiceWatcher(ServiceWatchOptions{
		MuxFn:            freshMuxFn(),
		Services:         []string{poolName},
		Interval:         0,
		MinInterval:      20 * time.Millisecond,
		ReconcileTimeout: 10 * time.Minute,
	})
	require.NotNil(t, w)
	w.Start(context.Background())

	w.Trigger(poolName)
	time.Sleep(300 * time.Millisecond) // 让 reconcile 真的进到 Recv 上

	done := make(chan struct{})
	go func() {
		w.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Stop 未能打断正在进行的 reconcile")
	}
}
