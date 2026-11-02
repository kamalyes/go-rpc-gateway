/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-10-08 21:20:03
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-10-13 17:55:28
 * @FilePath: \go-rpc-gateway\cpool\grpc\breaker_test.go
 * @Description: gRPC 客户端熔断测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package grpc

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	breakerconfig "github.com/kamalyes/go-config/pkg/breaker"
	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	testTarget    = "passthrough:///127.0.0.1:19001"
	testTargetAlt = "passthrough:///127.0.0.1:19002"
	testMethod    = "/user.v1.UserService/GetUser"
	testMethodAlt = "/user.v1.UserService/ListUsers"
	testMethodOrd = "/order.v1.OrderService/GetOrder"
)

// newTestClientConn 创建一个懒连接的 *grpc.ClientConn（grpc.NewClient 不会立即拨号）
// 拦截器只用到 cc.Target()，不会产生任何网络活动
func newTestClientConn(t *testing.T, target string) *grpc.ClientConn {
	t.Helper()
	cc, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cc.Close() })
	return cc
}

// testBreakerCfg 测试用熔断配置：3 次失败打开，2 次成功闭合，恢复等待 timeout
func testBreakerCfg(timeout time.Duration) *breakerconfig.CircuitBreaker {
	return &breakerconfig.CircuitBreaker{
		ModuleName:       "test_client_breaker",
		Enabled:          true,
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          int64(timeout),
		VolumeThreshold:  1,
	}
}

// countingInvoker 返回一个计数的 UnaryInvoker，固定返回 err
func countingInvoker(calls *int32, err error) grpc.UnaryInvoker {
	return func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		atomic.AddInt32(calls, 1)
		return err
	}
}

// countingStreamer 返回一个计数的 Streamer，固定返回 err
func countingStreamer(calls *int32, err error) grpc.Streamer {
	return func(context.Context, *grpc.StreamDesc, *grpc.ClientConn, string, ...grpc.CallOption) (grpc.ClientStream, error) {
		atomic.AddInt32(calls, 1)
		return nil, err
	}
}

// ==================== 构造与开关 ====================

func TestNewClientBreaker_DisabledReturnsNil(t *testing.T) {
	cfg := testBreakerCfg(time.Second)
	cfg.Enabled = false
	assert.Nil(t, NewClientBreaker(cfg, BreakerKeyTargetMethod))
}

func TestNewClientBreaker_NilConfigUsesClientDefault(t *testing.T) {
	cb := NewClientBreaker(nil, "")
	require.NotNil(t, cb)
	// 默认粒度是 target+method
	assert.Equal(t, BreakerKeyTargetMethod, cb.KeyMode())
	// 默认配置 prevention-paths 为空，表示保护全部方法（不像服务端默认值 /api/ 那样永远不生效）
	assert.NotNil(t, cb.BreakerFor(testTarget, testMethod))
}

func TestNewClientBreaker_InvalidKeyModeFallsBack(t *testing.T) {
	cb := NewClientBreaker(testBreakerCfg(time.Second), ClientBreakerKeyMode("bogus"))
	require.NotNil(t, cb)
	assert.Equal(t, BreakerKeyTargetMethod, cb.KeyMode())
}

func TestNewClientBreakerWithManager_NilManager(t *testing.T) {
	assert.Nil(t, NewClientBreakerWithManager(nil, testBreakerCfg(time.Second), BreakerKeyTarget))
}

func TestNewClientBreakerWithManager_SharesManager(t *testing.T) {
	cfg := testBreakerCfg(time.Second)
	mgr := middleware.NewBreakerManager(cfg)
	cb := NewClientBreakerWithManager(mgr, cfg, BreakerKeyTargetMethod)
	require.NotNil(t, cb)
	assert.Same(t, mgr, cb.Manager())

	// 客户端 key 带 target，服务端 key 是裸方法名，共享 manager 时不会互相覆盖
	require.NotNil(t, cb.BreakerFor(testTarget, testMethod))
	serverBreaker := mgr.GetBreaker(testMethod)
	clientBreaker := cb.BreakerFor(testTarget, testMethod)
	assert.NotSame(t, serverBreaker, clientBreaker)
	assert.Len(t, mgr.GetAllBreakers(), 2)
}

func TestDefaultClientBreakerRegistry(t *testing.T) {
	t.Cleanup(DisableClientBreaker)

	DisableClientBreaker()
	assert.Nil(t, GetDefaultClientBreaker())

	cb := EnableClientBreaker(testBreakerCfg(time.Second), BreakerKeyTarget)
	require.NotNil(t, cb)
	assert.Same(t, cb, GetDefaultClientBreaker())

	DisableClientBreaker()
	assert.Nil(t, GetDefaultClientBreaker())

	// 配置里关掉熔断时，Enable 等价于不启用
	disabled := testBreakerCfg(time.Second)
	disabled.Enabled = false
	assert.Nil(t, EnableClientBreaker(disabled, BreakerKeyTarget))
	assert.Nil(t, GetDefaultClientBreaker())
}

// testClientCfg 测试用客户端配置，默认不开熔断（与 go-config 的默认值一致）
func testClientCfg() *gwconfig.GRPCClient {
	return gwconfig.DefaultGRPCClient("user-service", []string{"127.0.0.1:19001"})
}

// TestBuildClientInterceptors_BreakerSwitch 验证开关确实作用于拨号时构建的拦截器链
// 默认（未启用）时链里只有 RequestContext / 日志 / 健康检查三项，与改动前完全一致
func TestBuildClientInterceptors_BreakerSwitch(t *testing.T) {
	t.Cleanup(DisableClientBreaker)

	cfg := testClientCfg()
	require.False(t, cfg.EnableBreaker, "go-config 的默认值应是不开客户端熔断")

	DisableClientBreaker()
	unary, stream := buildClientInterceptors(cfg, "user-service", nil)
	assert.Len(t, unary, 3, "未启用熔断时不应往链里加任何东西")
	assert.Len(t, stream, 3)

	EnableClientBreaker(testBreakerCfg(time.Second), BreakerKeyTargetMethod)
	unary, stream = buildClientInterceptors(cfg, "user-service", nil)
	require.Len(t, unary, 4, "启用后熔断拦截器应追加在链尾")
	require.Len(t, stream, 4)

	// 链尾那一项确实是熔断拦截器：构造一个已打开的断路器，断言它拒绝调用而不触达 invoker
	cb := GetDefaultClientBreaker()
	require.NotNil(t, cb)
	cc := newTestClientConn(t, testTarget)
	breaker := cb.BreakerFor(cc.Target(), testMethod)
	require.NotNil(t, breaker)

	var calls int32
	downstreamErr := status.Error(codes.Unavailable, "down")
	for i := 0; i < 3; i++ {
		assert.Error(t, unary[3](context.Background(), testMethod, nil, nil, cc, countingInvoker(&calls, downstreamErr)))
	}
	require.Equal(t, middleware.BreakerOpen, breaker.GetState())
	assert.Error(t, unary[3](context.Background(), testMethod, nil, nil, cc, countingInvoker(&calls, nil)))
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

// ==================== 配置驱动的开关 ====================

// TestBuildClientInterceptors_ConfigDrivenBreaker 熔断由每服务配置开启，不再依赖包级函数
func TestBuildClientInterceptors_ConfigDrivenBreaker(t *testing.T) {
	t.Cleanup(DisableClientBreaker)
	DisableClientBreaker()

	cfg := testClientCfg()
	cfg.EnableBreaker = true
	cfg.BreakerKeyMode = string(BreakerKeyTarget)

	unary, stream := buildClientInterceptors(cfg, "user-service", nil)
	require.Len(t, unary, 4, "enable-breaker 为 true 时熔断拦截器应入链")
	require.Len(t, stream, 4)

	// 包级实例仍然是空的：配置驱动的熔断器是每服务一个，不会注册成包级默认
	assert.Nil(t, GetDefaultClientBreaker())

	cfg.EnableBreaker = false
	unary, stream = buildClientInterceptors(cfg, "user-service", nil)
	assert.Len(t, unary, 3, "enable-breaker 为 false 时不应入链")
	assert.Len(t, stream, 3)
}

// TestResolveClientBreaker_ConfigDriven 配置里的 key 粒度要落到熔断器上，非法值回落默认
func TestResolveClientBreaker_ConfigDriven(t *testing.T) {
	t.Cleanup(DisableClientBreaker)
	DisableClientBreaker()

	assert.Nil(t, resolveClientBreaker(nil, "user-service"), "没有配置也没有包级实例时不应入链")
	assert.Nil(t, resolveClientBreaker(testClientCfg(), "user-service"), "enable-breaker 默认 false")

	cases := map[string]ClientBreakerKeyMode{
		string(BreakerKeyTarget):       BreakerKeyTarget,
		string(BreakerKeyMethod):       BreakerKeyMethod,
		string(BreakerKeyTargetMethod): BreakerKeyTargetMethod,
		"":                             BreakerKeyTargetMethod,
		"bogus":                        BreakerKeyTargetMethod, // 非法值回落默认并告警
	}
	for raw, want := range cases {
		cfg := testClientCfg()
		cfg.EnableBreaker = true
		cfg.BreakerKeyMode = raw
		cb := resolveClientBreaker(cfg, "user-service")
		require.NotNil(t, cb, "breaker-key-mode=%q", raw)
		assert.Equal(t, want, cb.KeyMode(), "breaker-key-mode=%q", raw)
	}
}

// TestResolveClientBreaker_RegisteredWinsOverConfig 包级实例优先于每服务配置
// 包级实例能带自定义阈值、也能与服务端共享 BreakerManager，这两件事 YAML 写不出来，
// 所以配置不可能是它的超集，只能让它优先（同时会打一条 Warn）
func TestResolveClientBreaker_RegisteredWinsOverConfig(t *testing.T) {
	t.Cleanup(DisableClientBreaker)

	registered := EnableClientBreaker(testBreakerCfg(time.Second), BreakerKeyTarget)
	require.NotNil(t, registered)

	cfg := testClientCfg()
	cfg.EnableBreaker = true
	cfg.BreakerKeyMode = string(BreakerKeyMethod)

	assert.Same(t, registered, resolveClientBreaker(cfg, "user-service"))

	// 配置没开熔断时，包级实例同样生效：它是代码里显式注册的，不该被一个默认值关掉
	cfg.EnableBreaker = false
	assert.Same(t, registered, resolveClientBreaker(cfg, "user-service"))
}

// TestParseClientBreakerKeyMode 裸字符串的解析：认得的取值、空串与非法值
func TestParseClientBreakerKeyMode(t *testing.T) {
	known := map[string]ClientBreakerKeyMode{
		"target_method": BreakerKeyTargetMethod,
		"target":        BreakerKeyTarget,
		"method":        BreakerKeyMethod,
		"":              BreakerKeyTargetMethod, // 没配就是默认，不算写错
		" TARGET ":      BreakerKeyTarget,       // 大小写与空白都归一
		"Target_Method": BreakerKeyTargetMethod,
	}
	for raw, want := range known {
		mode, ok := ParseClientBreakerKeyMode(raw)
		assert.True(t, ok, "raw=%q 应被认得", raw)
		assert.Equal(t, want, mode, "raw=%q", raw)
	}

	for _, raw := range []string{"bogus", "targetmethod", "target-method", "methods"} {
		mode, ok := ParseClientBreakerKeyMode(raw)
		assert.False(t, ok, "raw=%q 应被判为非法", raw)
		assert.Equal(t, BreakerKeyTargetMethod, mode, "非法值应回落默认粒度, raw=%q", raw)
	}
}

// TestBuildDialOptions_LoadBalancePolicy 负载均衡策略归一与未注册策略的兜底
func TestBuildDialOptions_LoadBalancePolicy(t *testing.T) {
	t.Cleanup(DisableClientBreaker)
	DisableClientBreaker()

	// 默认配置就是 round_robin，保持既有默认行为
	clientCfg := gwconfig.DefaultGRPCClient("user-service", []string{"127.0.0.1:19001"})
	assert.True(t, clientCfg.EnableLoadBalance)
	assert.Equal(t, RoundRobinBalancerName, clientCfg.LoadBalancePolicy)
	assert.NotEmpty(t, BuildDialOptions(clientCfg, "user-service", nil))

	// p2c / 未注册策略都不会让构建失败（未注册的会被换成 round_robin 并告警）
	for _, policy := range []string{P2CBalancerAlias, P2CBalancerName, "not_a_policy", ""} {
		cfg := clientCfg.Clone()
		cfg.LoadBalancePolicy = policy
		assert.NotEmpty(t, BuildDialOptions(cfg, "user-service", nil), "policy=%q", policy)
	}
}

// ==================== 熔断 key 粒度 ====================

func TestClientBreakerKey(t *testing.T) {
	cases := []struct {
		mode ClientBreakerKeyMode
		want string
	}{
		{BreakerKeyTargetMethod, testTarget + testMethod},
		{BreakerKeyTarget, testTarget},
		{BreakerKeyMethod, testMethod},
	}
	for _, c := range cases {
		t.Run(string(c.mode), func(t *testing.T) {
			cb := NewClientBreaker(testBreakerCfg(time.Second), c.mode)
			require.NotNil(t, cb)
			assert.Equal(t, c.want, cb.BreakerKey(testTarget, testMethod))
		})
	}
}

// TestClientBreakerFor_TargetMethodIsolation target+method 粒度下方法之间、下游之间都互相隔离
func TestClientBreakerFor_TargetMethodIsolation(t *testing.T) {
	cb := NewClientBreaker(testBreakerCfg(time.Second), BreakerKeyTargetMethod)
	require.NotNil(t, cb)

	a := cb.BreakerFor(testTarget, testMethod)
	b := cb.BreakerFor(testTarget, testMethodAlt)
	c := cb.BreakerFor(testTargetAlt, testMethod)
	require.NotNil(t, a)
	assert.NotSame(t, a, b, "同一下游的不同方法必须是不同断路器")
	assert.NotSame(t, a, c, "不同下游的同名方法必须是不同断路器")

	// 缓存命中返回同一实例
	assert.Same(t, a, cb.BreakerFor(testTarget, testMethod))
	assert.Len(t, cb.Manager().GetAllBreakers(), 3)
}

// TestClientBreakerFor_TargetMode target 粒度下同一下游的所有方法共用一个断路器
func TestClientBreakerFor_TargetMode(t *testing.T) {
	cb := NewClientBreaker(testBreakerCfg(time.Second), BreakerKeyTarget)
	require.NotNil(t, cb)

	a := cb.BreakerFor(testTarget, testMethod)
	b := cb.BreakerFor(testTarget, testMethodAlt)
	c := cb.BreakerFor(testTargetAlt, testMethod)
	require.NotNil(t, a)
	assert.Same(t, a, b)
	assert.NotSame(t, a, c)
	assert.Len(t, cb.Manager().GetAllBreakers(), 2)
}

// TestClientBreakerFor_MethodMode method 粒度下不同下游的同名方法会共用断路器（go-zero 的 key 语义）
func TestClientBreakerFor_MethodMode(t *testing.T) {
	cb := NewClientBreaker(testBreakerCfg(time.Second), BreakerKeyMethod)
	require.NotNil(t, cb)

	a := cb.BreakerFor(testTarget, testMethod)
	b := cb.BreakerFor(testTargetAlt, testMethod)
	require.NotNil(t, a)
	assert.Same(t, a, b, "method 粒度下不同下游会互相影响，这是该模式的已知代价")
	assert.Len(t, cb.Manager().GetAllBreakers(), 1)
}

// TestClientBreakerFor_ExcludesHealthAndReflection 健康检查与反射必须永不被熔断
func TestClientBreakerFor_ExcludesHealthAndReflection(t *testing.T) {
	cb := NewClientBreaker(testBreakerCfg(time.Second), BreakerKeyTargetMethod)
	require.NotNil(t, cb)

	for _, method := range DefaultClientBreakerExcludeMethods {
		assert.Nil(t, cb.BreakerFor(testTarget, method), "%s 不应被保护", method)
	}
	// 排除结果同样被缓存，不会在 manager 里留下断路器
	assert.Empty(t, cb.Manager().GetAllBreakers())
}

// TestClientBreakerFor_ScopedPreventionPaths 配置了 prevention-paths 时按 gRPC 方法名前缀收窄保护范围
func TestClientBreakerFor_ScopedPreventionPaths(t *testing.T) {
	cfg := testBreakerCfg(time.Second)
	cfg.PreventionPaths = []string{"/user.v1.UserService/"}
	cfg.ExcludePaths = []string{testMethodAlt}

	cb := NewClientBreaker(cfg, BreakerKeyTargetMethod)
	require.NotNil(t, cb)

	assert.NotNil(t, cb.BreakerFor(testTarget, testMethod), "前缀命中的方法应被保护")
	assert.Nil(t, cb.BreakerFor(testTarget, testMethodAlt), "exclude-paths 命中的方法不应被保护")
	assert.Nil(t, cb.BreakerFor(testTarget, testMethodOrd), "前缀未命中的方法不应被保护")
}

// ==================== Unary 拦截器 ====================

func TestUnaryClientBreakerInterceptor_NilBreakerPassThrough(t *testing.T) {
	cc := newTestClientConn(t, testTarget)
	var calls int32
	interceptor := UnaryClientBreakerInterceptor(nil)

	require.NoError(t, interceptor(context.Background(), testMethod, nil, nil, cc, countingInvoker(&calls, nil)))
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestUnaryClientBreakerInterceptor_UnprotectedMethodPassThrough(t *testing.T) {
	cc := newTestClientConn(t, testTarget)
	cb := NewClientBreaker(testBreakerCfg(time.Second), BreakerKeyTargetMethod)
	interceptor := UnaryClientBreakerInterceptor(cb)

	var calls int32
	health := DefaultClientBreakerExcludeMethods[0]
	downstreamErr := status.Error(codes.Unavailable, "boom")
	for i := 0; i < 10; i++ {
		assert.Error(t, interceptor(context.Background(), health, nil, nil, cc, countingInvoker(&calls, downstreamErr)))
	}
	// 排除的方法失败 10 次也不会被熔断，调用全部打到了下游
	assert.Equal(t, int32(10), atomic.LoadInt32(&calls))
	assert.Empty(t, cb.Manager().GetAllBreakers())
}

// TestUnaryClientBreakerInterceptor_FailureClassification 失败分类：只有下游真坏了才计入失败
func TestUnaryClientBreakerInterceptor_FailureClassification(t *testing.T) {
	cases := []struct {
		code          codes.Code
		shouldBeFault bool
	}{
		// 计入失败
		{codes.Unknown, true},
		{codes.DeadlineExceeded, true},
		{codes.Internal, true},
		{codes.Unavailable, true},
		{codes.DataLoss, true},
		// 不计入失败：客户端自身问题或业务语义
		{codes.OK, false},
		{codes.Canceled, false},
		{codes.InvalidArgument, false},
		{codes.NotFound, false},
		{codes.AlreadyExists, false},
		{codes.PermissionDenied, false},
		{codes.Unauthenticated, false},
		{codes.FailedPrecondition, false},
		{codes.Aborted, false},
		{codes.OutOfRange, false},
		{codes.ResourceExhausted, false},
		{codes.Unimplemented, false},
	}

	cc := newTestClientConn(t, testTarget)
	for _, c := range cases {
		t.Run(c.code.String(), func(t *testing.T) {
			cb := NewClientBreaker(testBreakerCfg(time.Second), BreakerKeyTargetMethod)
			interceptor := UnaryClientBreakerInterceptor(cb)

			var downstreamErr error
			if c.code != codes.OK {
				downstreamErr = status.Error(c.code, "downstream")
			}
			var calls int32
			for i := 0; i < 3; i++ {
				err := interceptor(context.Background(), testMethod, nil, nil, cc, countingInvoker(&calls, downstreamErr))
				assert.Equal(t, c.code, status.Code(err))
			}
			require.Equal(t, int32(3), atomic.LoadInt32(&calls))

			snap := cb.BreakerFor(testTarget, testMethod).Snapshot()
			assert.EqualValues(t, 3, snap.TotalRequests)
			if c.shouldBeFault {
				assert.EqualValues(t, 3, snap.FailedRequests)
				assert.Equal(t, middleware.BreakerOpen, snap.State, "3 次故障应打开断路器")
			} else {
				assert.EqualValues(t, 0, snap.FailedRequests)
				assert.Equal(t, middleware.BreakerClosed, snap.State)
			}
		})
	}
}

// TestUnaryClientBreakerInterceptor_OpenRejectsWithoutCalling 打开后就地快速失败，不再触达下游
func TestUnaryClientBreakerInterceptor_OpenRejectsWithoutCalling(t *testing.T) {
	cc := newTestClientConn(t, testTarget)
	cb := NewClientBreaker(testBreakerCfg(time.Minute), BreakerKeyTargetMethod)
	interceptor := UnaryClientBreakerInterceptor(cb)

	var calls int32
	downstreamErr := status.Error(codes.Unavailable, "downstream down")
	for i := 0; i < 3; i++ {
		assert.Error(t, interceptor(context.Background(), testMethod, nil, nil, cc, countingInvoker(&calls, downstreamErr)))
	}
	require.Equal(t, int32(3), atomic.LoadInt32(&calls))
	require.Equal(t, middleware.BreakerOpen, cb.BreakerFor(testTarget, testMethod).GetState())

	// 打开期间：拒绝不触达下游
	for i := 0; i < 20; i++ {
		err := interceptor(context.Background(), testMethod, nil, nil, cc, countingInvoker(&calls, nil))
		require.Error(t, err)
		assert.Equal(t, codes.Unavailable, status.Code(err), "熔断拒绝沿用 ErrCodeCircuitBreakerOpen(4003) 的 Unavailable 映射")
		assert.Contains(t, status.Convert(err).Message(), "client-side circuit breaker")
	}
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls), "打开期间不应再调用下游")

	// 拒绝不计入统计，否则会自我强化永远出不来
	snap := cb.BreakerFor(testTarget, testMethod).Snapshot()
	assert.EqualValues(t, 3, snap.TotalRequests)
	assert.EqualValues(t, 3, snap.FailedRequests)
}

// TestUnaryClientBreakerInterceptor_OnlyFailingMethodIsCut target+method 粒度下坏方法不会误伤健康方法
func TestUnaryClientBreakerInterceptor_OnlyFailingMethodIsCut(t *testing.T) {
	cc := newTestClientConn(t, testTarget)
	cb := NewClientBreaker(testBreakerCfg(time.Minute), BreakerKeyTargetMethod)
	interceptor := UnaryClientBreakerInterceptor(cb)

	var calls int32
	downstreamErr := status.Error(codes.DeadlineExceeded, "slow method")
	for i := 0; i < 3; i++ {
		assert.Error(t, interceptor(context.Background(), testMethod, nil, nil, cc, countingInvoker(&calls, downstreamErr)))
	}
	assert.Equal(t, middleware.BreakerOpen, cb.BreakerFor(testTarget, testMethod).GetState())

	// 同一下游的另一个方法照常可用
	var okCalls int32
	require.NoError(t, interceptor(context.Background(), testMethodAlt, nil, nil, cc, countingInvoker(&okCalls, nil)))
	assert.Equal(t, int32(1), atomic.LoadInt32(&okCalls))
	assert.Equal(t, middleware.BreakerClosed, cb.BreakerFor(testTarget, testMethodAlt).GetState())
}

// TestUnaryClientBreakerInterceptor_HalfOpenRecovery 半开恢复：超时后放行探测，连续成功则闭合
func TestUnaryClientBreakerInterceptor_HalfOpenRecovery(t *testing.T) {
	const recoverTimeout = 80 * time.Millisecond

	cc := newTestClientConn(t, testTarget)
	cb := NewClientBreaker(testBreakerCfg(recoverTimeout), BreakerKeyTargetMethod)
	interceptor := UnaryClientBreakerInterceptor(cb)
	breaker := cb.BreakerFor(testTarget, testMethod)
	require.NotNil(t, breaker)

	var calls int32
	downstreamErr := status.Error(codes.Unavailable, "down")
	for i := 0; i < 3; i++ {
		assert.Error(t, interceptor(context.Background(), testMethod, nil, nil, cc, countingInvoker(&calls, downstreamErr)))
	}
	require.Equal(t, middleware.BreakerOpen, breaker.GetState())

	// 未到恢复时间：继续拒绝
	assert.Error(t, interceptor(context.Background(), testMethod, nil, nil, cc, countingInvoker(&calls, nil)))
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))

	// 等过恢复时间（留足余量，避免调度抖动导致 flaky）
	time.Sleep(recoverTimeout * 3)

	// 第一次探测成功 -> 半开，成功阈值为 2 所以还没闭合
	require.NoError(t, interceptor(context.Background(), testMethod, nil, nil, cc, countingInvoker(&calls, nil)))
	assert.Equal(t, int32(4), atomic.LoadInt32(&calls))
	assert.Equal(t, middleware.BreakerHalfOpen, breaker.GetState())

	// 第二次成功 -> 闭合
	require.NoError(t, interceptor(context.Background(), testMethod, nil, nil, cc, countingInvoker(&calls, nil)))
	assert.Equal(t, int32(5), atomic.LoadInt32(&calls))
	assert.Equal(t, middleware.BreakerClosed, breaker.GetState())
}

// TestUnaryClientBreakerInterceptor_HalfOpenFailureReopens 半开期间再失败立即重新熔断
func TestUnaryClientBreakerInterceptor_HalfOpenFailureReopens(t *testing.T) {
	const recoverTimeout = 80 * time.Millisecond

	cc := newTestClientConn(t, testTarget)
	cb := NewClientBreaker(testBreakerCfg(recoverTimeout), BreakerKeyTargetMethod)
	interceptor := UnaryClientBreakerInterceptor(cb)
	breaker := cb.BreakerFor(testTarget, testMethod)

	var calls int32
	downstreamErr := status.Error(codes.Internal, "down")
	for i := 0; i < 3; i++ {
		assert.Error(t, interceptor(context.Background(), testMethod, nil, nil, cc, countingInvoker(&calls, downstreamErr)))
	}
	require.Equal(t, middleware.BreakerOpen, breaker.GetState())

	time.Sleep(recoverTimeout * 3)

	// 半开探测再次失败 -> 立刻回到打开
	assert.Error(t, interceptor(context.Background(), testMethod, nil, nil, cc, countingInvoker(&calls, downstreamErr)))
	assert.Equal(t, middleware.BreakerOpen, breaker.GetState())

	// 并且马上又开始拒绝
	before := atomic.LoadInt32(&calls)
	assert.Error(t, interceptor(context.Background(), testMethod, nil, nil, cc, countingInvoker(&calls, nil)))
	assert.Equal(t, before, atomic.LoadInt32(&calls))
}

// ==================== Stream 拦截器 ====================

func TestStreamClientBreakerInterceptor_NilBreakerPassThrough(t *testing.T) {
	cc := newTestClientConn(t, testTarget)
	var calls int32
	interceptor := StreamClientBreakerInterceptor(nil)

	_, err := interceptor(context.Background(), &grpc.StreamDesc{}, cc, testMethod, countingStreamer(&calls, nil))
	assert.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestStreamClientBreakerInterceptor_OpenRejects(t *testing.T) {
	cc := newTestClientConn(t, testTarget)
	cb := NewClientBreaker(testBreakerCfg(time.Minute), BreakerKeyTargetMethod)
	interceptor := StreamClientBreakerInterceptor(cb)

	var calls int32
	downstreamErr := status.Error(codes.Unavailable, "cannot create stream")
	for i := 0; i < 3; i++ {
		_, err := interceptor(context.Background(), &grpc.StreamDesc{}, cc, testMethod, countingStreamer(&calls, downstreamErr))
		assert.Error(t, err)
	}
	require.Equal(t, int32(3), atomic.LoadInt32(&calls))
	require.Equal(t, middleware.BreakerOpen, cb.BreakerFor(testTarget, testMethod).GetState())

	_, err := interceptor(context.Background(), &grpc.StreamDesc{}, cc, testMethod, countingStreamer(&calls, nil))
	require.Error(t, err)
	assert.Equal(t, codes.Unavailable, status.Code(err))
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls), "打开期间不应再建流")
}

func TestStreamClientBreakerInterceptor_ClientErrorNotCounted(t *testing.T) {
	cc := newTestClientConn(t, testTarget)
	cb := NewClientBreaker(testBreakerCfg(time.Minute), BreakerKeyTargetMethod)
	interceptor := StreamClientBreakerInterceptor(cb)

	var calls int32
	clientErr := status.Error(codes.InvalidArgument, "bad request")
	for i := 0; i < 10; i++ {
		_, err := interceptor(context.Background(), &grpc.StreamDesc{}, cc, testMethod, countingStreamer(&calls, clientErr))
		assert.Error(t, err)
	}
	assert.Equal(t, int32(10), atomic.LoadInt32(&calls))
	assert.Equal(t, middleware.BreakerClosed, cb.BreakerFor(testTarget, testMethod).GetState())
}

// ==================== 并发 ====================

// TestUnaryClientBreakerInterceptor_Concurrent 并发下 key 缓存与统计不出错（配合 -race）
// 不对「打开/闭合」做时序断言，只断言不变量，避免 flaky
func TestUnaryClientBreakerInterceptor_Concurrent(t *testing.T) {
	cc := newTestClientConn(t, testTarget)
	cb := NewClientBreaker(testBreakerCfg(time.Minute), BreakerKeyTargetMethod)
	interceptor := UnaryClientBreakerInterceptor(cb)

	const (
		goroutines = 32
		perG       = 50
	)
	methods := []string{testMethod, testMethodAlt, testMethodOrd}
	var calls int32
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			method := methods[g%len(methods)]
			for i := 0; i < perG; i++ {
				_ = interceptor(context.Background(), method, nil, nil, cc, countingInvoker(&calls, nil))
			}
		}(g)
	}
	wg.Wait()

	assert.Equal(t, int32(goroutines*perG), atomic.LoadInt32(&calls))
	// 每个方法一个断路器，不会因为并发首次访问而重复创建
	assert.Len(t, cb.Manager().GetAllBreakers(), len(methods))

	var total int64
	for _, snap := range cb.Manager().GetAllBreakerSnapshots() {
		total += snap.TotalRequests
		assert.Equal(t, middleware.BreakerClosed, snap.State)
	}
	assert.EqualValues(t, goroutines*perG, total)
}

// TestCowMap_LoadOrCreateOnce 写时复制映射：并发首次访问只创建一次
func TestCowMap_LoadOrCreateOnce(t *testing.T) {
	var m cowMap[*int]
	var created int32

	const goroutines = 64
	results := make([]*int, goroutines)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = m.loadOrCreate("k", func() *int {
				atomic.AddInt32(&created, 1)
				v := 42
				return &v
			})
		}(i)
	}
	wg.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&created))
	assert.Equal(t, 1, m.size())
	for i := 1; i < goroutines; i++ {
		assert.Same(t, results[0], results[i])
	}

	// 缓存 nil 值同样算「已存在」，不会反复触发 create
	var nilMap cowMap[*int]
	var nilCreated int32
	for i := 0; i < 3; i++ {
		v := nilMap.loadOrCreate("absent", func() *int { atomic.AddInt32(&nilCreated, 1); return nil })
		assert.Nil(t, v)
	}
	assert.Equal(t, int32(1), atomic.LoadInt32(&nilCreated))
}

func TestClientTarget_NilConn(t *testing.T) {
	assert.Equal(t, "", clientTarget(nil))
}
