/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-10-07 18:35:21
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-10-11 21:07:52
 * @FilePath: \go-rpc-gateway\cpool\grpc\balancer.go
 * @Description: gRPC 客户端 P2C + EWMA 负载均衡策略，按延迟与在途请求数择优选择后端
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package grpc

import (
	"math"
	"math/rand/v2"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kamalyes/go-rpc-gateway/middleware"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/status"
)

// ============================================================================
// 策略名与可调参数
// [EN] Policy names and tunables
// ============================================================================

const (
	// P2CBalancerName P2C + EWMA 策略名，填进 load-balance-policy 即生效
	// gRPC 的 balancer 注册表按小写名查找，这里必须保持全小写
	P2CBalancerName = "p2c_ewma"

	// P2CBalancerAlias 简写别名，配置里写 p2c 也能用（由 ResolveLoadBalancePolicy 归一）
	P2CBalancerAlias = "p2c"

	// RoundRobinBalancerName grpc-go 内置轮询策略名，仍是本仓库的默认值
	RoundRobinBalancerName = roundrobin.Name
)

const (
	// P2CDecayTime EWMA 的衰减时间常数
	//
	// 权重按 w = exp(-Δt / P2CDecayTime) 计算（Δt 为距上次采样的间隔）：
	// 两次采样间隔越长，旧值权重越小，新样本说话越算数
	// 用时间而不是固定系数做衰减，是因为各后端的 QPS 差别很大：固定系数下
	// 低频后端的统计值会陈旧得离谱，而时间衰减天然把「很久以前的一次测量」看轻
	//
	// [EN] Weight is w = exp(-Δt / P2CDecayTime) where Δt is the gap since the previous
	// sample, so a longer gap discounts history more. Decaying by time rather than by a
	// fixed coefficient matters because backends see very different request rates: with a
	// fixed coefficient a low-traffic backend would keep a hopelessly stale estimate
	P2CDecayTime = 600 * time.Millisecond

	// P2CErrorPenalty 失败调用额外计入的虚拟延迟
	//
	// 不另开一份成功率 EWMA，而是把失败折算成延迟惩罚喂给同一个 EWMA：
	// 「连接被拒绝」这类失败往往 1 微秒就返回，若按真实延迟记录，一个彻底挂掉的后端
	// 反而会成为延迟最低、最受偏爱的节点——这是 P2C 实现里最经典的坑
	// 失败判定复用 middleware.IsBreakerFailureCode，与熔断器用同一张表，
	// 参数错误之类的客户端自身问题不会污染后端的负载估计
	//
	// [EN] Instead of a second EWMA for success rate, a failure is folded into the latency
	// estimate as a penalty. Failures such as "connection refused" return in microseconds,
	// so recording their real latency would make a completely dead backend look like the
	// fastest and most attractive one — the classic P2C pitfall. What counts as a failure
	// is delegated to middleware.IsBreakerFailureCode, the same table the breaker uses, so
	// client-caused codes such as InvalidArgument never pollute a backend's load estimate
	P2CErrorPenalty = 250 * time.Millisecond

	// P2CForcePickInterval 强制探测间隔
	//
	// EWMA 只在请求结束时更新：一个节点因为一次抖动被判为高负载后，如果从此不再被选中，
	// 它的统计值就永远停在那次抖动上，再也没有机会证明自己已经恢复——永久饿死
	// 因此抽到的两个节点里，若落败的那个已经超过该间隔没被选中，就强制选它探一次
	//
	// [EN] The EWMA is only updated when a request completes, so a node judged slow after a
	// single hiccup would keep that estimate forever if it is never picked again — permanent
	// starvation. When the loser of a comparison has not been picked for longer than this
	// interval, it is picked anyway as a probe
	P2CForcePickInterval = 3 * time.Second
)

// init 注册 p2c 策略
//
// 注册本身不改变任何行为：只有配置里显式选择了这个名字才会被用上，
// 默认依然是 round_robin（见 buildDialOptions）
//
// [EN] Registering is inert: the policy is used only when its name is selected explicitly,
// and the default remains round_robin (see buildDialOptions)
func init() {
	balancer.Register(NewP2CBalancerBuilder())
}

// NewP2CBalancerBuilder 创建一个独立的 P2C balancer.Builder
//
// 包的 init 已经注册了一个全局实例，这里导出构造函数是为了测试与特殊场景能拿到
// 互不共享 EWMA 状态的实例
//
// 实现基于 balancer/base：base 负责「为每个解析出的地址建 SubConn、维护连通状态、
// 只把 Ready 的 SubConn 交给 Picker」，自定义策略只需实现 base.PickerBuilder
// 选这套 API 是因为在 grpc-go v1.83.2 里 base.NewBalancerBuilder / base.PickerBuilder /
// base.NewErrPicker 均未废弃；被废弃的是 balancer.Balancer.UpdateSubConnState（已由
// NewSubConnOptions.StateListener 取代）、SubConn.UpdateAddresses、base.ErrPicker 等，
// 本文件一个都没用到——base 内部已经用 StateListener 建 SubConn
//
// [EN] The package init already registers one global instance; this constructor is exported
// so tests and special cases can obtain instances that do not share EWMA state. The
// implementation builds on balancer/base, which owns SubConn creation, connectivity
// tracking and handing only READY SubConns to the picker. That API was chosen because in
// grpc-go v1.83.2 base.NewBalancerBuilder, base.PickerBuilder and base.NewErrPicker are all
// current; what is deprecated is Balancer.UpdateSubConnState (superseded by
// NewSubConnOptions.StateListener), SubConn.UpdateAddresses and base.ErrPicker, none of
// which are used here — base itself already creates SubConns with a StateListener
func NewP2CBalancerBuilder() balancer.Builder {
	return base.NewBalancerBuilder(
		P2CBalancerName,
		&p2cPickerBuilder{nodes: make(map[balancer.SubConn]*p2cNode)},
		// 与 round_robin 保持一致：服务配置里声明了 healthCheckConfig 时启用子连接健康检查
		base.Config{HealthCheck: true},
	)
}

// ResolveLoadBalancePolicy 把配置里的负载均衡策略名归一成 gRPC 注册表里的名字
//
// 空值回落到 round_robin（保持既有默认行为），p2c / p2c_ewma 都指向 P2CBalancerName，
// 其余名字原样返回，交给调用方校验是否已注册
//
// [EN] Normalizes the configured policy name: empty falls back to round_robin (preserving
// the existing default), both p2c and p2c_ewma map to P2CBalancerName, anything else is
// returned unchanged for the caller to validate
func ResolveLoadBalancePolicy(policy string) string {
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case "":
		return RoundRobinBalancerName
	case P2CBalancerAlias, P2CBalancerName:
		return P2CBalancerName
	default:
		return strings.ToLower(strings.TrimSpace(policy))
	}
}

// ============================================================================
// p2cNode 单个后端的负载统计
// [EN] Per-backend load statistics
// ============================================================================

// p2cNode 单个后端（SubConn）的负载统计
// 全部字段用原子类型，选择路径与统计更新路径都无锁，与 middleware.Breaker 的做法一致
type p2cNode struct {
	sc   balancer.SubConn
	addr string

	ewma       atomic.Uint64 // 延迟的指数加权移动平均，单位纳秒
	inflight   atomic.Int64  // 在途请求数
	lastUpdate atomic.Int64  // 上次更新 ewma 的时刻，UnixNano，0 表示还没有样本
	lastPick   atomic.Int64  // 上次被选中的时刻，UnixNano
}

// newP2CNode 创建节点
// seedEWMA 为新节点的初始 EWMA（取现存节点的均值，理由见 p2cPickerBuilder.Build）
func newP2CNode(sc balancer.SubConn, addr string, seedEWMA uint64, nowNanos int64) *p2cNode {
	n := &p2cNode{sc: sc, addr: addr}
	n.ewma.Store(seedEWMA)
	// lastPick 初始化为当前时刻，避免新节点一上来就满足强制探测条件
	n.lastPick.Store(nowNanos)
	return n
}

// load 计算节点的负载打分，越小越优
//
//	load = sqrt(ewma + 1) * (inflight + 1)
//
// 两个因子分别对应两种信息：ewma 是「这个后端有多慢」（已经发生的事实），
// inflight 是「这个后端手上压了多少活」（即将发生的事实）
// 只看延迟会在后端刚变慢但统计还没跟上时过度打流量，只看在途数则完全无视处理能力差异，
// 两者相乘才能同时表达「慢」和「忙」
//
// sqrt 用来压缩延迟的量纲：延迟在毫秒级就是 1e6 纳秒，不开方的话延迟项会彻底压倒
// 在途数项，一次长尾抖动足以让节点被排除很久；开方后 1ms 与 4ms 的差距是 2 倍而不是
// 4 倍，在途数仍有发言权
// +1 保证新节点（ewma = 0）与空闲节点（inflight = 0）不会算出 0 分而被无条件偏爱
//
// [EN] Score is sqrt(ewma+1) * (inflight+1), lower is better. The two factors carry
// different information: the EWMA says how slow the backend has been (what already
// happened), the in-flight count says how much work it is already holding (what is about to
// happen). Latency alone over-sends to a backend that just got slow but whose estimate has
// not caught up; in-flight alone ignores capacity differences entirely; multiplied, the
// score expresses both "slow" and "busy". The square root compresses the latency scale —
// milliseconds are 1e6 nanoseconds, so without it the latency term would swamp the
// in-flight term and one tail hiccup would sideline a node for a long time. The +1 terms
// keep a fresh node (ewma 0) or an idle node (inflight 0) from scoring zero and being
// preferred unconditionally
func (n *p2cNode) load() uint64 {
	lag := n.ewma.Load()
	inflight := n.inflight.Load()
	if inflight < 0 {
		inflight = 0
	}
	return uint64(math.Sqrt(float64(lag)+1)) * uint64(inflight+1)
}

// observe 用一个新样本更新 EWMA
//
// w = exp(-Δt / P2CDecayTime)，新值 = 旧值 * w + 样本 * (1 - w)
// 首个样本直接作为初值，避免被 0 或种子值拖着慢慢爬
//
// 并发下 Load/Store 之间可能插入另一次更新而丢掉一个样本，这是刻意的：
// 对统计量而言丢个别样本无实质影响，换来热路径上不需要 CAS 重试或加锁
//
// [EN] Updates the EWMA with one sample using w = exp(-Δt / P2CDecayTime). The very first
// sample becomes the value outright, so the estimate does not have to crawl up from zero or
// from the seed. Concurrent updates may lose an individual sample between the load and the
// store; that is deliberate, since losing one sample does not meaningfully move an average
// and it keeps the hot path free of CAS retries and locks
func (n *p2cNode) observe(nowNanos, sampleNanos int64) {
	if sampleNanos < 0 {
		sampleNanos = 0
	}

	last := n.lastUpdate.Swap(nowNanos)
	prev := n.ewma.Load()
	if last == 0 || prev == 0 {
		n.ewma.Store(uint64(sampleNanos))
		return
	}

	td := nowNanos - last
	if td < 0 {
		td = 0
	}
	w := math.Exp(-float64(td) / float64(P2CDecayTime))
	n.ewma.Store(uint64(float64(prev)*w + float64(sampleNanos)*(1-w)))
}

// done 请求结束时回收在途计数并记录延迟样本
//
// di 为零值（gRPC 在 SubConn 实际不可用、请求没有真正发出时会这样回调）时只回收在途数，
// 不记录样本：那次「0 延迟的成功」是假的，喂进 EWMA 会让坏节点显得极快
//
// [EN] Releases the in-flight slot and records a latency sample. When di is the zero value —
// which gRPC does when the SubConn turned out to be unusable and nothing was actually sent —
// only the in-flight count is released and no sample is recorded, because that fake
// zero-latency success would make a broken node look extremely fast
func (n *p2cNode) done(start time.Time, di balancer.DoneInfo) {
	n.inflight.Add(-1)

	if di.Err == nil && !di.BytesSent && !di.BytesReceived {
		return
	}

	now := time.Now()
	sample := now.Sub(start)
	if di.Err != nil && middleware.IsBreakerFailureCode(status.Code(di.Err)) {
		sample += P2CErrorPenalty
	}
	n.observe(now.UnixNano(), int64(sample))
}

// ============================================================================
// p2cPickerBuilder / p2cPicker
// ============================================================================

// p2cPickerBuilder 按 base 的要求构造 Picker，并跨 Picker 重建保存 EWMA 状态
//
// base 在任一 SubConn 进入/离开 Ready 时都会重建 Picker，所以统计状态必须放在
// Builder 上而不是 Picker 里，否则每次后端抖动都会把全部历史清空
// nodes 按 SubConn 身份索引：SubConn 由 gRPC 为每个地址创建，天然唯一且不跨连接复用
//
// [EN] base rebuilds the picker whenever any SubConn enters or leaves READY, so the
// statistics must live on the builder rather than in the picker, otherwise every hiccup
// would wipe all history. Nodes are indexed by SubConn identity, which gRPC creates per
// address and never shares across ClientConns
type p2cPickerBuilder struct {
	mu    sync.Mutex
	nodes map[balancer.SubConn]*p2cNode
}

// Build 由 base 在 Ready 集合变化时调用
func (pb *p2cPickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	if len(info.ReadySCs) == 0 {
		// 没有可用后端：返回错误 Picker，gRPC 会阻塞或快速失败（取决于 WaitForReady）
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}

	pb.mu.Lock()
	defer pb.mu.Unlock()

	nowNanos := time.Now().UnixNano()
	seed := pb.seedEWMALocked(info)

	// kept 只保留本轮 Ready 的节点：离开 Ready 的节点连同统计一起丢弃，
	// map 不会随后端更替无限增长
	// 正在被丢弃的节点若还有在途请求，它们的 Done 回调仍持有旧节点对象，
	// 减到的是那个已经没人读的对象，不会把新节点的在途数减成负数
	//
	// [EN] Only nodes that are READY in this round are kept, so the map cannot grow without
	// bound as backends come and go. In-flight requests on a dropped node still hold a
	// reference to the old object through their Done closure, so they decrement that
	// orphan rather than driving a fresh node's counter negative
	kept := make(map[balancer.SubConn]*p2cNode, len(info.ReadySCs))
	nodes := make([]*p2cNode, 0, len(info.ReadySCs))
	for sc, scInfo := range info.ReadySCs {
		node, ok := pb.nodes[sc]
		if !ok {
			node = newP2CNode(sc, scInfo.Address.Addr, seed, nowNanos)
		}
		kept[sc] = node
		nodes = append(nodes, node)
	}
	pb.nodes = kept

	return &p2cPicker{nodes: nodes}
}

// seedEWMALocked 计算新节点的初始 EWMA：现存且有样本的节点的均值
//
// 新节点若以 ewma = 0 起步，load 只剩 (inflight+1)，在其他节点动辄上千分的对比下
// 会赢下几乎每一次抽样，刚扩容或刚恢复的后端会被瞬间打爆（典型的冷启动惊群）
// 用集群均值作为先验，新节点既不会被偏爱也不会被冷落，几个请求之后就用自己的真实数据接管
// 没有任何历史可参考时返回 0，此时所有节点都是新的，谁都不占优势
//
// [EN] Seeds a new node with the mean EWMA of the nodes that already have samples. Starting
// from zero would leave its score at (inflight+1) and let it win nearly every comparison
// against nodes scoring in the thousands, so a freshly scaled-out or just-recovered backend
// would be hammered instantly — the classic cold-start thundering herd. The cluster mean is
// a neutral prior: a few requests later the node's own measurements take over. With no
// history at all the seed is 0, which is fair because every node is then new
func (pb *p2cPickerBuilder) seedEWMALocked(info base.PickerBuildInfo) uint64 {
	var sum, count uint64
	for sc := range info.ReadySCs {
		node, ok := pb.nodes[sc]
		if !ok || node.lastUpdate.Load() == 0 {
			continue
		}
		sum += node.ewma.Load()
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / count
}

// p2cPicker 一次 Picker 快照，nodes 构造后只读
type p2cPicker struct {
	nodes []*p2cNode
}

// Pick 选择一个后端
//
// 只做原子操作与常数次比较，不阻塞、不加锁——Picker.Pick 位于每个 RPC 的关键路径上
func (p *p2cPicker) Pick(balancer.PickInfo) (balancer.PickResult, error) {
	var chosen *p2cNode
	start := time.Now()

	switch len(p.nodes) {
	case 0:
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	case 1:
		chosen = p.nodes[0]
	default:
		chosen = p.choose(start.UnixNano())
	}

	chosen.inflight.Add(1)
	chosen.lastPick.Store(start.UnixNano())

	return balancer.PickResult{
		SubConn: chosen.sc,
		Done:    func(di balancer.DoneInfo) { chosen.done(start, di) },
	}, nil
}

// choose P2C：随机抽两个不同的节点，取负载打分较低的那个
//
// 为什么是「抽两个比较」而不是「全局选最优」：全局最优会让所有调用方在同一时刻
// 一致地扑向同一个节点（它的统计还没反映出压力），下一刻又一起扑向另一个，产生羊群振荡
// 随机抽两个把这种一致性打散，而「两个里选较好的」又足以把流量显著拉向健康节点，
// 代价只有两次原子读
//
// [EN] P2C samples two distinct nodes at random and takes the one with the lower score.
// Picking the global optimum would make every caller stampede onto the same node at the same
// moment (before its statistics reflect the pressure) and then onto the next one, producing
// herd oscillation. Random sampling breaks that correlation, while "best of two" still pulls
// traffic strongly toward healthy nodes — for the price of two atomic loads
func (p *p2cPicker) choose(nowNanos int64) *p2cNode {
	count := len(p.nodes)

	// 抽两个不同下标：j 在 count-1 个候选里抽，再对 i 做位移，保证 i != j 且各无序对等概率
	i := rand.IntN(count)
	j := rand.IntN(count - 1)
	if j >= i {
		j++
	}

	winner, loser := p.nodes[i], p.nodes[j]
	if winner.load() > loser.load() {
		winner, loser = loser, winner
	}

	// 强制探测：落败方太久没被选中，其 EWMA 已失去时效，给它一次机会
	if nowNanos-loser.lastPick.Load() > int64(P2CForcePickInterval) {
		return loser
	}
	return winner
}
