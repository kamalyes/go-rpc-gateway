/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-10-09 21:02:37
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-10-13 18:30:09
 * @FilePath: \go-rpc-gateway\cpool\grpc\balancer_test.go
 * @Description: gRPC 客户端 P2C + EWMA 负载均衡测试
 *
 * Copyright (c) 2026 by kamalyes 501893067@qq.com, All Rights Reserved.
 */

package grpc

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// testSubConn 测试用 SubConn
// balancer.SubConn 带有 internal.EnforceSubConnEmbedding 这个强制嵌入标记，包外无法从零实现，
// 必须按 gRPC 的要求嵌入接口本身（这里嵌入的是 nil，因为 Picker 只把 SubConn 当句柄传递，
// 不会调用它的任何方法）
type testSubConn struct {
	balancer.SubConn
	name string
}

// buildTestPicker 用给定的 (addr -> SubConn) 集合构造 Picker，并返回 addr -> 节点 的索引
func buildTestPicker(t *testing.T, pb *p2cPickerBuilder, scs map[string]balancer.SubConn) (*p2cPicker, map[string]*p2cNode) {
	t.Helper()

	info := base.PickerBuildInfo{ReadySCs: make(map[balancer.SubConn]base.SubConnInfo, len(scs))}
	for addr, sc := range scs {
		info.ReadySCs[sc] = base.SubConnInfo{Address: resolver.Address{Addr: addr}}
	}

	picker, ok := pb.Build(info).(*p2cPicker)
	require.True(t, ok, "Build 应返回 *p2cPicker")

	nodes := make(map[string]*p2cNode, len(picker.nodes))
	for _, node := range picker.nodes {
		nodes[node.addr] = node
	}
	require.Len(t, nodes, len(scs))
	return picker, nodes
}

// newTestPickerBuilder 创建互不共享状态的 PickerBuilder
func newTestPickerBuilder() *p2cPickerBuilder {
	return &p2cPickerBuilder{nodes: make(map[balancer.SubConn]*p2cNode)}
}

// setEWMA 直接写入节点的 EWMA 与「已有样本」标记，用于构造确定性的选择场景
func setEWMA(node *p2cNode, lag time.Duration) {
	node.ewma.Store(uint64(lag))
	node.lastUpdate.Store(time.Now().UnixNano())
}

// pickCounts 执行 n 次选择并统计分布
//
// 每次选择后只归还在途计数，不调用 Done：Done 会用「本次 Pick 到归还之间的真实耗时」
// 去更新 EWMA，几微秒的真实耗时会立刻冲掉测试预设的延迟，断言也就无从谈起
// 归还在途计数是必须的，否则在途数会单调累加，选择偏好会从「按延迟」漂移成「按累计次数」
//
// [EN] After each pick only the in-flight slot is released, without calling Done: Done would
// overwrite the EWMA with the few microseconds actually spent in the loop and destroy the
// scenario under test. Releasing the in-flight slot is required, otherwise the counter grows
// monotonically and the preference drifts from "by latency" to "by pick count"
func pickCounts(t *testing.T, picker *p2cPicker, n int) map[string]int {
	t.Helper()

	byConn := make(map[balancer.SubConn]*p2cNode, len(picker.nodes))
	for _, node := range picker.nodes {
		byConn[node.sc] = node
	}

	counts := make(map[string]int, len(picker.nodes))
	for i := 0; i < n; i++ {
		result, err := picker.Pick(balancer.PickInfo{FullMethodName: testMethod})
		require.NoError(t, err)
		node := byConn[result.SubConn]
		require.NotNil(t, node)
		counts[node.addr]++
		node.inflight.Add(-1)
	}
	return counts
}

// ==================== 注册与策略名归一 ====================

func TestP2CBalancerRegistered(t *testing.T) {
	builder := balancer.Get(P2CBalancerName)
	require.NotNil(t, builder, "init 应已把 p2c_ewma 注册进 gRPC balancer 注册表")
	assert.Equal(t, P2CBalancerName, builder.Name())

	// round_robin 仍然可用（默认策略）
	assert.NotNil(t, balancer.Get(RoundRobinBalancerName))

	// 别名不进注册表，由 ResolveLoadBalancePolicy 归一后再交给 gRPC
	assert.Nil(t, balancer.Get(P2CBalancerAlias))
}

func TestNewP2CBalancerBuilder_Isolated(t *testing.T) {
	a, b := NewP2CBalancerBuilder(), NewP2CBalancerBuilder()
	require.NotNil(t, a)
	require.NotNil(t, b)
	assert.Equal(t, P2CBalancerName, a.Name())
	assert.NotSame(t, a, b)
}

func TestResolveLoadBalancePolicy(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", RoundRobinBalancerName},
		{"   ", RoundRobinBalancerName},
		{"round_robin", "round_robin"},
		{"p2c", P2CBalancerName},
		{"P2C", P2CBalancerName},
		{"p2c_ewma", P2CBalancerName},
		{" P2C_EWMA ", P2CBalancerName},
		{"pick_first", "pick_first"},
		{"not_a_policy", "not_a_policy"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			assert.Equal(t, c.want, ResolveLoadBalancePolicy(c.in))
		})
	}

	// 归一后的名字必须真的在注册表里，否则 gRPC 会静默回落到 pick_first
	assert.NotNil(t, balancer.Get(ResolveLoadBalancePolicy("p2c")))
	assert.NotNil(t, balancer.Get(ResolveLoadBalancePolicy("")))
}

// ==================== Picker 构造 ====================

func TestP2CPickerBuilder_NoReadySubConns(t *testing.T) {
	picker := newTestPickerBuilder().Build(base.PickerBuildInfo{})
	require.NotNil(t, picker)

	_, err := picker.Pick(balancer.PickInfo{})
	assert.ErrorIs(t, err, balancer.ErrNoSubConnAvailable)
}

func TestP2CPicker_SingleSubConn(t *testing.T) {
	sc := &testSubConn{name: "only"}
	picker, nodes := buildTestPicker(t, newTestPickerBuilder(), map[string]balancer.SubConn{"10.0.0.1:9000": sc})

	for i := 0; i < 10; i++ {
		result, err := picker.Pick(balancer.PickInfo{})
		require.NoError(t, err)
		assert.Same(t, sc, result.SubConn)
		require.NotNil(t, result.Done)
		result.Done(balancer.DoneInfo{BytesSent: true, BytesReceived: true})
	}
	assert.EqualValues(t, 0, nodes["10.0.0.1:9000"].inflight.Load())
}

func TestP2CPicker_DoneReleasesInflight(t *testing.T) {
	picker, nodes := buildTestPicker(t, newTestPickerBuilder(), map[string]balancer.SubConn{
		"10.0.0.1:9000": &testSubConn{name: "a"},
	})
	node := nodes["10.0.0.1:9000"]

	result, err := picker.Pick(balancer.PickInfo{})
	require.NoError(t, err)
	assert.EqualValues(t, 1, node.inflight.Load(), "Pick 之后在途数应为 1")

	result.Done(balancer.DoneInfo{BytesSent: true, BytesReceived: true})
	assert.EqualValues(t, 0, node.inflight.Load())
	assert.Greater(t, node.ewma.Load(), uint64(0), "完成的调用应留下延迟样本")
}

// ==================== 选择偏好 ====================

// TestP2CPicker_PrefersLowLatency 断言选择确实偏向低延迟节点
//
// 三个节点的 EWMA 为 1ms / 10ms / 100ms，在途数全为 0，于是 load 排序完全由延迟决定
// P2C 每次等概率抽两个不同节点，三种组合各占 1/3：
//
//	(1ms,10ms) -> 1ms    (1ms,100ms) -> 1ms    (10ms,100ms) -> 10ms
//
// 因此期望份额是 1ms 约 2/3、10ms 约 1/3、100ms 恰好 0——这不是「大概偏向」，
// 而是可以算出来的确定分布，断言区间按 n=3000 下约 ±7 个标准差取，不会 flaky
// 100ms 节点为 0 的前提是强制探测不会被触发：newP2CNode 把 lastPick 初始化为建 Picker 的时刻，
// 而 P2CForcePickInterval 是 3s，远长于本测试的执行时间
//
// [EN] With EWMAs of 1ms / 10ms / 100ms and no in-flight requests, the score order is fixed
// by latency alone. P2C samples each of the three unordered pairs with probability 1/3, so
// the expected shares are exactly 2/3, 1/3 and 0 — a computed distribution rather than a
// vague "prefers faster". The bounds are roughly seven standard deviations wide at n=3000.
// The zero share holds because force-probing cannot fire here: newP2CNode initializes
// lastPick to the build time and P2CForcePickInterval is 3s
func TestP2CPicker_PrefersLowLatency(t *testing.T) {
	const picks = 3000

	picker, nodes := buildTestPicker(t, newTestPickerBuilder(), map[string]balancer.SubConn{
		"fast:9000": &testSubConn{name: "fast"},
		"mid:9000":  &testSubConn{name: "mid"},
		"slow:9000": &testSubConn{name: "slow"},
	})
	setEWMA(nodes["fast:9000"], 1*time.Millisecond)
	setEWMA(nodes["mid:9000"], 10*time.Millisecond)
	setEWMA(nodes["slow:9000"], 100*time.Millisecond)

	// load 的大小关系是选择偏好的前提，先单独断言一次
	assert.Less(t, nodes["fast:9000"].load(), nodes["mid:9000"].load())
	assert.Less(t, nodes["mid:9000"].load(), nodes["slow:9000"].load())

	counts := pickCounts(t, picker, picks)
	fast := float64(counts["fast:9000"]) / picks
	mid := float64(counts["mid:9000"]) / picks

	t.Logf("fast=%.3f mid=%.3f slow=%.3f", fast, mid, float64(counts["slow:9000"])/picks)
	assert.InDelta(t, 2.0/3.0, fast, 0.06, "最快的节点应拿到约 2/3 的流量")
	assert.InDelta(t, 1.0/3.0, mid, 0.06, "中等节点应拿到约 1/3 的流量")
	assert.Zero(t, counts["slow:9000"], "最慢的节点在任一组合里都不会胜出")
}

// TestP2CPicker_PrefersLowInflight 断言在延迟相同时选择偏向在途请求更少的节点
//
// 两个节点 EWMA 都是 5ms，其中一个预置 4 个在途请求：
// load = sqrt(ewma+1) * (inflight+1)，忙节点是闲节点的 5 倍，两节点场景下每次都要比较，
// 所以闲节点应拿到全部流量
//
// [EN] Same EWMA on both nodes, one carrying four in-flight requests. Since
// load = sqrt(ewma+1) * (inflight+1), the busy node scores five times higher, and with only
// two nodes every pick compares them, so the idle node must take all the traffic
func TestP2CPicker_PrefersLowInflight(t *testing.T) {
	const picks = 500

	picker, nodes := buildTestPicker(t, newTestPickerBuilder(), map[string]balancer.SubConn{
		"idle:9000": &testSubConn{name: "idle"},
		"busy:9000": &testSubConn{name: "busy"},
	})
	setEWMA(nodes["idle:9000"], 5*time.Millisecond)
	setEWMA(nodes["busy:9000"], 5*time.Millisecond)
	nodes["busy:9000"].inflight.Store(4)

	assert.Equal(t, nodes["idle:9000"].load()*5, nodes["busy:9000"].load())

	counts := pickCounts(t, picker, picks)
	assert.Equal(t, picks, counts["idle:9000"], "空闲节点应拿到全部流量")
	assert.Zero(t, counts["busy:9000"])
	assert.EqualValues(t, 4, nodes["busy:9000"].inflight.Load(), "预置的在途数不应被改动")
}

// TestP2CPicker_InflightOutweighsStaleLatency 在途请求数足以盖过轻微的延迟优势
// 否则一个「延迟统计略低但已经堆满请求」的节点会被持续打流量
func TestP2CPicker_InflightOutweighsStaleLatency(t *testing.T) {
	const picks = 500

	picker, nodes := buildTestPicker(t, newTestPickerBuilder(), map[string]balancer.SubConn{
		"loaded:9000": &testSubConn{name: "loaded"},
		"spare:9000":  &testSubConn{name: "spare"},
	})
	// loaded 的延迟更低（4ms vs 9ms，开方后 2000 vs 3000），但手上压了 2 个请求
	setEWMA(nodes["loaded:9000"], 4*time.Millisecond)
	setEWMA(nodes["spare:9000"], 9*time.Millisecond)
	nodes["loaded:9000"].inflight.Store(2)

	counts := pickCounts(t, picker, picks)
	assert.Equal(t, picks, counts["spare:9000"], "在途数 3 倍于对手时，延迟优势不足以补偿")
}

// TestP2CPicker_ForcePicksStarvedNode 长时间未被选中的节点会被强制探测一次
func TestP2CPicker_ForcePicksStarvedNode(t *testing.T) {
	picker, nodes := buildTestPicker(t, newTestPickerBuilder(), map[string]balancer.SubConn{
		"fast:9000":    &testSubConn{name: "fast"},
		"starved:9000": &testSubConn{name: "starved"},
	})
	setEWMA(nodes["fast:9000"], 1*time.Millisecond)
	setEWMA(nodes["starved:9000"], 500*time.Millisecond)

	// 把饿死节点的「上次被选中」推到强制探测间隔之外
	nodes["starved:9000"].lastPick.Store(time.Now().Add(-2 * P2CForcePickInterval).UnixNano())

	first, err := picker.Pick(balancer.PickInfo{})
	require.NoError(t, err)
	assert.Same(t, nodes["starved:9000"].sc, first.SubConn, "超过探测间隔未被选中的节点应被强制选中")
	nodes["starved:9000"].inflight.Add(-1)

	// 探测之后 lastPick 已刷新，下一次回到按负载选择
	second, err := picker.Pick(balancer.PickInfo{})
	require.NoError(t, err)
	assert.Same(t, nodes["fast:9000"].sc, second.SubConn)
	nodes["fast:9000"].inflight.Add(-1)
}

// ==================== EWMA 与负载打分 ====================

func TestP2CNode_Load(t *testing.T) {
	node := newP2CNode(&testSubConn{}, "a:9000", 0, time.Now().UnixNano())

	// 全新节点：ewma = 0，load = (inflight+1)
	assert.EqualValues(t, 1, node.load())
	node.inflight.Store(3)
	assert.EqualValues(t, 4, node.load())

	// 1ms = 1e6ns，开方得 1000
	node.inflight.Store(0)
	node.ewma.Store(uint64(time.Millisecond))
	assert.EqualValues(t, 1000, node.load())
	node.inflight.Store(2)
	assert.EqualValues(t, 3000, node.load())

	// 在途数被 Done 减成负数时（理论上不该发生）不能让 load 反向变小
	node.inflight.Store(-5)
	assert.EqualValues(t, 1000, node.load())
}

func TestP2CNode_ObserveEWMA(t *testing.T) {
	t.Run("首个样本直接作为初值", func(t *testing.T) {
		node := newP2CNode(&testSubConn{}, "a", 0, 0)
		node.observe(1000, int64(10*time.Millisecond))
		assert.EqualValues(t, 10*time.Millisecond, node.ewma.Load())
	})

	t.Run("间隔很短时旧值主导", func(t *testing.T) {
		node := newP2CNode(&testSubConn{}, "a", 0, 0)
		now := int64(1_000_000_000)
		node.observe(now, int64(10*time.Millisecond))
		// Δt 为衰减常数的万分之一，新样本权重约 0.0001
		node.observe(now+int64(P2CDecayTime)/10000, int64(20*time.Millisecond))

		ewma := time.Duration(node.ewma.Load())
		assert.Greater(t, ewma, 10*time.Millisecond)
		assert.Less(t, ewma, 10200*time.Microsecond)
	})

	t.Run("间隔很长时新样本主导", func(t *testing.T) {
		node := newP2CNode(&testSubConn{}, "a", 0, 0)
		now := int64(1_000_000_000)
		node.observe(now, int64(10*time.Millisecond))
		// Δt 为衰减常数的 10 倍，旧值权重 exp(-10) 约 4.5e-5
		node.observe(now+int64(P2CDecayTime)*10, int64(20*time.Millisecond))

		ewma := time.Duration(node.ewma.Load())
		assert.Greater(t, ewma, 19900*time.Microsecond)
		assert.LessOrEqual(t, ewma, 20*time.Millisecond)
	})

	t.Run("负样本与时间回拨不会算出异常值", func(t *testing.T) {
		node := newP2CNode(&testSubConn{}, "a", 0, 0)
		node.observe(2000, int64(10*time.Millisecond))
		node.observe(1000, -5) // 时间回拨 + 负延迟
		assert.LessOrEqual(t, node.ewma.Load(), uint64(10*time.Millisecond))
	})
}

// TestP2CNode_DoneErrorPenalty 失败调用要吃延迟惩罚
// 否则「连接被拒绝」这种 1 微秒就返回的失败会让挂掉的节点成为最受偏爱的节点
func TestP2CNode_DoneErrorPenalty(t *testing.T) {
	t.Run("下游故障计入惩罚", func(t *testing.T) {
		node := newP2CNode(&testSubConn{}, "a", 0, time.Now().UnixNano())
		node.inflight.Add(1)
		node.done(time.Now(), balancer.DoneInfo{Err: status.Error(codes.Unavailable, "refused"), BytesSent: true})

		assert.EqualValues(t, 0, node.inflight.Load())
		assert.GreaterOrEqual(t, time.Duration(node.ewma.Load()), P2CErrorPenalty)
	})

	t.Run("客户端自身错误不计惩罚", func(t *testing.T) {
		node := newP2CNode(&testSubConn{}, "a", 0, time.Now().UnixNano())
		node.inflight.Add(1)
		node.done(time.Now(), balancer.DoneInfo{
			Err:           status.Error(codes.InvalidArgument, "bad arg"),
			BytesSent:     true,
			BytesReceived: true,
		})

		assert.EqualValues(t, 0, node.inflight.Load())
		assert.Less(t, time.Duration(node.ewma.Load()), 50*time.Millisecond)
	})

	t.Run("请求没真正发出时不记样本", func(t *testing.T) {
		seed := uint64(7 * time.Millisecond)
		node := newP2CNode(&testSubConn{}, "a", seed, time.Now().UnixNano())
		node.inflight.Add(1)
		node.done(time.Now(), balancer.DoneInfo{})

		assert.EqualValues(t, 0, node.inflight.Load())
		assert.Equal(t, seed, node.ewma.Load(), "零值 DoneInfo 不应被当成 0 延迟的成功")
		assert.EqualValues(t, 0, node.lastUpdate.Load())
	})
}

// ==================== Picker 重建时的状态保持 ====================

// TestP2CPickerBuilder_CarriesOverAndPrunes Picker 重建保留 EWMA，离开 Ready 的节点被回收
//
// base 在任一 SubConn 进入/离开 Ready 时都会重建 Picker，若统计存在 Picker 里，
// 每次后端抖动都会把全部历史清零，p2c 就退化成随机选择
func TestP2CPickerBuilder_CarriesOverAndPrunes(t *testing.T) {
	pb := newTestPickerBuilder()
	scA := &testSubConn{name: "a"}
	scB := &testSubConn{name: "b"}

	_, nodes1 := buildTestPicker(t, pb, map[string]balancer.SubConn{"a:9000": scA, "b:9000": scB})
	setEWMA(nodes1["a:9000"], 7*time.Millisecond)
	setEWMA(nodes1["b:9000"], 70*time.Millisecond)
	nodes1["b:9000"].inflight.Store(1) // b 还有在途请求

	// b 离开 Ready
	_, nodes2 := buildTestPicker(t, pb, map[string]balancer.SubConn{"a:9000": scA})
	assert.Same(t, nodes1["a:9000"], nodes2["a:9000"], "仍在 Ready 的节点应被原样保留")
	assert.EqualValues(t, 7*time.Millisecond, nodes2["a:9000"].ewma.Load())
	assert.Len(t, pb.nodes, 1, "离开 Ready 的节点应被回收，map 不随后端更替增长")

	// b 以新的 SubConn 重新加入：新节点在途数从 0 起算，
	// 老节点的 Done 回调减的是那个已被丢弃的对象，不会把新节点减成负数
	scB2 := &testSubConn{name: "b2"}
	_, nodes3 := buildTestPicker(t, pb, map[string]balancer.SubConn{"a:9000": scA, "b:9000": scB2})
	assert.NotSame(t, nodes1["b:9000"], nodes3["b:9000"])
	assert.EqualValues(t, 0, nodes3["b:9000"].inflight.Load())

	nodes1["b:9000"].done(time.Now(), balancer.DoneInfo{BytesSent: true, BytesReceived: true})
	assert.EqualValues(t, 0, nodes3["b:9000"].inflight.Load())
	assert.EqualValues(t, 0, nodes1["b:9000"].inflight.Load())
}

// TestP2CPickerBuilder_SeedsNewNodeWithClusterMean 新节点以集群均值起步，避免冷启动惊群
func TestP2CPickerBuilder_SeedsNewNodeWithClusterMean(t *testing.T) {
	pb := newTestPickerBuilder()
	scA := &testSubConn{name: "a"}
	scB := &testSubConn{name: "b"}

	_, nodes1 := buildTestPicker(t, pb, map[string]balancer.SubConn{"a:9000": scA, "b:9000": scB})
	setEWMA(nodes1["a:9000"], 10*time.Millisecond)
	setEWMA(nodes1["b:9000"], 20*time.Millisecond)

	scC := &testSubConn{name: "c"}
	_, nodes2 := buildTestPicker(t, pb, map[string]balancer.SubConn{
		"a:9000": scA, "b:9000": scB, "c:9000": scC,
	})
	assert.EqualValues(t, 15*time.Millisecond, nodes2["c:9000"].ewma.Load(), "新节点初值应为现存节点均值")
	assert.EqualValues(t, 0, nodes2["c:9000"].lastUpdate.Load(), "种子不算真实样本")

	// 全新集群没有历史可参考，种子为 0
	fresh := newTestPickerBuilder()
	_, nodes3 := buildTestPicker(t, fresh, map[string]balancer.SubConn{"x:9000": &testSubConn{name: "x"}})
	assert.EqualValues(t, 0, nodes3["x:9000"].ewma.Load())
}

// ==================== 并发 ====================

// TestP2CPicker_Concurrent 并发选择 + 完成回调（配合 -race）
// 只断言不变量（在途数归零、选择必有结果），不对分布做时序断言，避免 flaky
func TestP2CPicker_Concurrent(t *testing.T) {
	const (
		goroutines = 32
		perG       = 100
	)

	picker, nodes := buildTestPicker(t, newTestPickerBuilder(), map[string]balancer.SubConn{
		"a:9000": &testSubConn{name: "a"},
		"b:9000": &testSubConn{name: "b"},
		"c:9000": &testSubConn{name: "c"},
		"d:9000": &testSubConn{name: "d"},
	})

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				result, err := picker.Pick(balancer.PickInfo{FullMethodName: testMethod})
				if err != nil {
					t.Error(err)
					return
				}
				if (g+i)%7 == 0 {
					result.Done(balancer.DoneInfo{Err: status.Error(codes.Unavailable, "x"), BytesSent: true})
				} else {
					result.Done(balancer.DoneInfo{BytesSent: true, BytesReceived: true})
				}
			}
		}(g)
	}
	wg.Wait()

	for addr, node := range nodes {
		assert.EqualValues(t, 0, node.inflight.Load(), "%s 的在途计数应归零", addr)
	}
}

// TestP2CBalancer_EndToEndThroughClientConn 端到端验证策略真的能在活的 ClientConn 里工作
//
// 前面的测试都是直接驱动 Picker，证明不了「gRPC 会接受这个策略名并真的用上我们的 Picker」
// 这里注册一个带独立统计的测试专用策略名，用它拨号到一个真实但没注册任何服务的 gRPC server：
//   - 调用返回 Unimplemented 说明请求确实经过我们的 Picker 打到了后端
//     （若负载均衡阶段就失败，拿到的会是 Unavailable 或超时）
//   - PickerBuilder 里留下了节点与延迟样本，说明用的正是我们的 Picker 而不是 pick_first
//
// [EN] The other tests drive the picker directly and cannot show that gRPC accepts the
// policy name and actually routes through our picker. This one registers a test-only policy
// name backed by its own statistics and dials a real gRPC server with no services
// registered: an Unimplemented reply proves the request reached the backend through our
// picker (a load-balancing failure would surface as Unavailable or a timeout), and the
// node plus latency sample left behind in the PickerBuilder proves the picker was ours
func TestP2CBalancer_EndToEndThroughClientConn(t *testing.T) {
	const policyName = "p2c_ewma_e2e_test"

	pb := newTestPickerBuilder()
	balancer.Register(base.NewBalancerBuilder(policyName, pb, base.Config{HealthCheck: true}))
	require.NotNil(t, balancer.Get(policyName))

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	cc, err := grpc.NewClient("passthrough:///"+lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"`+policyName+`"}`),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cc.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = cc.Invoke(ctx, "/p2c.e2e.Probe/Ping", &emptypb.Empty{}, &emptypb.Empty{}, grpc.WaitForReady(true))
	require.Error(t, err)
	assert.Equal(t, codes.Unimplemented, status.Code(err), "请求应已打到后端，只是服务端没注册该服务")

	pb.mu.Lock()
	defer pb.mu.Unlock()
	require.Len(t, pb.nodes, 1, "应为该后端建过一个节点")
	for _, node := range pb.nodes {
		assert.Equal(t, lis.Addr().String(), node.addr)
		assert.EqualValues(t, 0, node.inflight.Load(), "调用结束后在途数应归零")
		assert.Greater(t, node.ewma.Load(), uint64(0), "应留下延迟样本，证明走的是我们的 Done 回调")
	}
}
