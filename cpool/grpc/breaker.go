/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-10-06 17:11:30
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-10-11 21:28:05
 * @FilePath: \go-rpc-gateway\cpool\grpc\breaker.go
 * @Description: gRPC 客户端熔断 - 复用 middleware 的 Breaker 状态机，在调用下游前就地快速失败
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package grpc

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gcmiddleware "github.com/kamalyes/go-rpc-gateway/config/middleware"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// ============================================================================
// 为什么客户端也要熔断
// [EN] Why the client side needs its own breaker
// ============================================================================
//
// 服务端熔断（middleware.UnaryServerBreakerInterceptor）保护的是「自己被打爆」：
// 请求打进来，处理不过来就拒绝
// 客户端熔断保护的是「被下游拖死」：下游挂了还一直发请求，每个调用都要等到
// DeadlineExceeded 才返回，期间占着调用方的 goroutine、连接和上游请求的超时预算，
// 一个慢下游足以把整个网关的并发吃满——这正是 go-zero zrpc 把 breaker 的主战场
// 放在客户端拦截器的原因
// 两者方向相反，不能互相替代：服务端熔断不会让调用方少等一秒，
// 客户端熔断也不会在自己过载时保护自己
//
// [EN] The server-side breaker protects a service from being overwhelmed by inbound
// traffic. This one protects the caller from a dead dependency: without it every call
// to a broken downstream has to wait for its deadline, holding a goroutine, a stream
// and part of the inbound request's timeout budget the whole time, so a single slow
// dependency can saturate the gateway. That is exactly why go-zero's zrpc puts its
// breaker in the client interceptor. The two directions are not interchangeable.

// ============================================================================
// 熔断 key 的粒度
// [EN] Breaker key granularity
// ============================================================================

// ClientBreakerKeyMode 客户端熔断 key 的粒度
//
// 「下游整体挂掉」和「下游某个方法挂掉」是两种不同的故障模式，粒度选错会出事：
//
//   - 只按 method：同一个方法名在不同下游（灰度集群、多环境、同名方法的不同服务）
//     会共用一个断路器，A 环境挂掉会把 B 环境的调用一起熔断——误伤
//     本仓库的 BreakerManager 可以被多个客户端共享，这个风险是真实存在的
//   - 只按 target：下游整体挂掉时熔断最快（所有方法的失败累加到同一个计数器），
//     但一个慢方法（比如某个大报表查询总是超时）会把该下游所有健康方法一起打死——误伤
//   - 按 target+method（默认）：故障隔离在方法级，一个方法坏掉不影响其它方法；
//     下游整体挂掉时每个方法各自累计失败并独立打开，代价是「打开」比 target 粒度慢一点
//     （每个方法都要自己攒够 failure-threshold），但主力流量的方法会立刻打开，
//     低频方法本来也贡献不了多少阻塞
//
// 还有一点容易混淆：这里的 target 是拨号目标（cc.Target()，即 go-config 里配的 endpoint），
// 不是负载均衡后面的某个具体后端实例
// 「单个后端实例是否变慢/变坏」是 p2c 负载均衡器的职责（见 balancer.go），
// 熔断器管的是「这个逻辑下游还值不值得调用」，两层分工不要混在一起
//
// [EN] "the whole dependency is down" and "one method of it is down" are different
// failure modes, and the wrong key granularity makes the breaker either useless or
// harmful. Keying on the method alone merges distinct downstreams that happen to expose
// the same method name (canary clusters, several environments, a shared BreakerManager);
// keying on the target alone trips fastest when the dependency dies as a whole, but one
// chronically slow method takes every healthy method of that dependency down with it.
// target+method (the default) isolates failures per method; when the dependency dies as
// a whole every method trips independently, slightly later but without collateral damage.
// Note that target here is the dial target (cc.Target()), not an individual backend behind
// load balancing: per-backend latency is the p2c balancer's job (balancer.go), while the
// breaker decides whether this logical dependency is worth calling at all
type ClientBreakerKeyMode string

const (
	// BreakerKeyTargetMethod 按「拨号目标 + 全限定方法名」熔断（默认，故障隔离到方法级）
	BreakerKeyTargetMethod ClientBreakerKeyMode = "target_method"
	// BreakerKeyTarget 按「拨号目标」熔断（下游整体挂掉时打开最快，但会误伤健康方法）
	BreakerKeyTarget ClientBreakerKeyMode = "target"
	// BreakerKeyMethod 按「全限定方法名」熔断（与 go-zero 的 key 一致，多下游场景会互相干扰）
	BreakerKeyMethod ClientBreakerKeyMode = "method"
)

// ParseClientBreakerKeyMode 把配置里的裸字符串解析成熔断 key 粒度
//
// go-config 的 GRPCClient.BreakerKeyMode 是 string 而不是枚举（配置库不引入消费方的类型），
// 所以取值校验落在本仓库这一侧
// 返回 (粒度, 取值是否认得)：认不出来时第一个返回值仍是默认粒度 BreakerKeyTargetMethod，
// 第二个返回值为 false，由调用方决定要不要告警——解析函数本身不打日志，
// 这样它在测试与非网关进程里都能用，与 ResolveLoadBalancePolicy 把「未注册策略」
// 交给调用方 balancer.Get 去发现是同一套分工
// 空串算「没配」，是认得的取值：使用方自建 GRPCClient 时留空很常见，不该为此告警
// 大小写与首尾空白都归一，YAML 里写成 "Target_Method" 或 " target " 都认
//
// [EN] Parses the raw configuration string into a key granularity. go-config's
// GRPCClient.BreakerKeyMode is a plain string rather than an enum — a config library does not
// take on a consumer's types — so validating the value is this repository's job. The second
// return value reports whether the spelling was recognized; the first is always usable,
// defaulting to BreakerKeyTargetMethod. The parser itself never logs, so it stays usable in
// tests and outside a gateway process, the same split as ResolveLoadBalancePolicy leaving the
// "unregistered policy" discovery to its caller. An empty string means "not configured" and
// counts as recognized, since leaving it unset is common when building a GRPCClient by hand.
// Case and surrounding whitespace are normalized
func ParseClientBreakerKeyMode(raw string) (ClientBreakerKeyMode, bool) {
	switch mode := ClientBreakerKeyMode(strings.ToLower(strings.TrimSpace(raw))); mode {
	case BreakerKeyTargetMethod, BreakerKeyTarget, BreakerKeyMethod:
		return mode, true
	case "":
		return BreakerKeyTargetMethod, true
	default:
		return BreakerKeyTargetMethod, false
	}
}

// DefaultClientBreakerExcludeMethods 默认不受客户端熔断保护的方法
//
// 探测与发现类调用必须排除，它们是「用来判断下游好没好」的信号，不是业务流量，
// 把信号本身熔断掉，就只剩一个永远无法被证伪的结论
//
//   - 反射：AutoRegister / RediscoverAndRegisterService 通过连接池里的同一条 ClientConn
//     开 ServerReflectionInfo 流来发现下游服务，而「下游刚恢复」正是最需要重新发现的时刻
//     如果这条流被熔断拒绝，服务就再也注册不回来——形成「熔断导致发现失败，
//     发现失败导致继续熔断」的死锁
//     v1 与 v1alpha 两个版本都排除：服务端只实现 v1alpha 时客户端会回落到它
//   - 健康检查：本仓库的 HealthChecker 走的是 ClientConn 连通性状态（GetState /
//     WaitForStateChange），并不发 Health RPC，所以这条排除是给「应用自己用 healthpb
//     轮询下游」的场景留的保险
//     顺带说明：gRPC 自身的子连接健康检查（base.Config{HealthCheck: true} 配合 service
//     config 的 healthCheckConfig）用的是 transport 直连流（newNonRetryClientStream），
//     根本不经过客户端拦截器，无论排不排除都不受熔断影响
//
// [EN] Probe and discovery calls must never be gated by the breaker: they are the signal
// used to decide whether a dependency is healthy, and cutting off the signal leaves only a
// conclusion that can never be falsified. Reflection matters concretely here — AutoRegister
// and RediscoverAndRegisterService open a ServerReflectionInfo stream on the very same
// pooled ClientConn, and "the dependency just came back" is exactly when rediscovery is
// needed, so a rejected stream would deadlock the service into never being registered again
// (both v1 and v1alpha are excluded, since a client falls back to v1alpha against older
// servers). The health methods are excluded as insurance for applications that poll a
// dependency through healthpb themselves: this repository's HealthChecker uses ClientConn
// connectivity state rather than a Health RPC, and gRPC's own per-SubConn health checking
// uses a direct transport stream that never passes through client interceptors anyway
var DefaultClientBreakerExcludeMethods = []string{
	"/grpc.health.v1.Health/Check",
	"/grpc.health.v1.Health/Watch",
	"/grpc.reflection.v1.ServerReflection/ServerReflectionInfo",
	"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
}

// clientBreakerOpenErr 预构建的客户端熔断拒绝错误
//
// 错误码沿用 errors.ErrCodeCircuitBreakerOpen(4003)：它的三态映射（HTTP 503 /
// codes.Unavailable）已在 docs/ERRORS.md 登记，熔断在客户端还是服务端触发并不改变
// 「服务暂时不可用，稍后重试」这个语义，调用方的处理方式也完全一样，没有必要再占一个错误码
// Details 里写明是客户端侧拒绝，用来区分「我没去调下游」和「下游拒绝了我」
// 与服务端一样预构建成包级变量：被熔断期间请求量最大，拒绝路径不应该再分配对象
//
// [EN] Reuses errors.ErrCodeCircuitBreakerOpen(4003); its three-way mapping (HTTP 503 /
// codes.Unavailable) is already registered in docs/ERRORS.md, and whether the breaker
// tripped on the client or the server side does not change the meaning for the caller
// ("temporarily unavailable, retry later"), so a second error code would buy nothing. The
// detail string distinguishes "I did not call the dependency" from "the dependency
// rejected me". Pre-built like its server-side counterpart: the reject path carries the
// most traffic exactly while the breaker is open, so it must not allocate
var clientBreakerOpenErr = errors.NewError(
	errors.ErrCodeCircuitBreakerOpen,
	errors.WithDetails("downstream call rejected by client-side circuit breaker"),
).ToGRPCError()

// DefaultClientBreakerConfig 客户端熔断的默认配置
//
// 与 gcmiddleware.DefaultCircuitBreaker() 的差别有两处，都是客户端语义决定的：
//   - PreventionPaths 为空：服务端默认值是 []string{"/api/"}，那是 HTTP 路径前缀，
//     放到 gRPC 侧永远匹配不上 /pkg.Service/Method，断路器会「启用了但从不生效」
//     客户端语义是「开了就保护这个下游的全部方法」，所以用空列表表示全保护
//   - Timeout 取 5s 而非 30s：客户端熔断打开期间调用方是完全看不到下游的，
//     恢复探测迟 30s 意味着下游已经好了还要再瞎 30s，客户端对误判的容忍度比服务端低
//
// [EN] Differs from gcmiddleware.DefaultCircuitBreaker() in two client-specific ways. PreventionPaths is
// empty: the server default of "/api/" is an HTTP path prefix that can never match a gRPC
// method name, which would leave the breaker enabled but permanently inert, whereas the
// client-side meaning of "enabled" is "protect every method of this dependency". And the
// recovery timeout is 5s rather than 30s, because while a client breaker is open the caller
// is blind to the dependency, so waiting 30s after it recovered is far more costly here
func DefaultClientBreakerConfig() *gcmiddleware.CircuitBreaker {
	return &gcmiddleware.CircuitBreaker{
		ModuleName:       "grpc_client_breaker",
		Enabled:          true,
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          int64(5 * time.Second),
		VolumeThreshold:  10,
		PreventionPaths:  nil,
		ExcludePaths:     append([]string(nil), DefaultClientBreakerExcludeMethods...),
	}
}

// ============================================================================
// cowMap 写时复制字符串映射
// [EN] Copy-on-write string map
// ============================================================================

// cowMap 只读快照 + 写时复制的 string -> V 映射
//
// 热路径（每次 RPC 一次）需要按 target 和 method 查断路器，要求零分配：
// sync.Map 的 Load(key any) 会把 string 装箱成 interface{}，每次调用多一次堆分配；
// RWMutex + map 虽然不分配，但高并发下 RLock 的原子操作会成为共享缓存行热点
// key 集合（下游地址、生成代码里的方法名）是有限且很快收敛的，写时复制正好合适：
// 读路径一次 atomic.Load 加一次 map 索引，完全无锁无分配，写只发生在每个 key 的首次访问
//
// [EN] The hot path needs a breaker lookup per RPC and must not allocate. sync.Map boxes a
// string key into an interface{} on every Load, costing an allocation per call, while an
// RWMutex-guarded map does not allocate but turns its reader count into a contended cache
// line. The key space (dial targets, generated method names) is small and converges almost
// immediately, which is the ideal case for copy-on-write: reads are one atomic load plus one
// map index, and a write happens only the first time a key is seen
type cowMap[V any] struct {
	mu      sync.Mutex
	entries atomic.Pointer[map[string]V]
}

// load 无锁读取，返回值与「是否存在」
func (c *cowMap[V]) load(key string) (V, bool) {
	if m := c.entries.Load(); m != nil {
		v, ok := (*m)[key]
		return v, ok
	}
	var zero V
	return zero, false
}

// loadOrCreate 读取 key，缺失时调用 create 生成并写时复制写回
// create 在锁内执行，且只会对每个 key 执行一次（double-check）
func (c *cowMap[V]) loadOrCreate(key string, create func() V) V {
	if v, ok := c.load(key); ok {
		return v
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	old := c.entries.Load()
	if old != nil {
		if v, ok := (*old)[key]; ok {
			return v
		}
	}

	size := 1
	if old != nil {
		size += len(*old)
	}
	next := make(map[string]V, size)
	if old != nil {
		for k, v := range *old {
			next[k] = v
		}
	}
	v := create()
	next[key] = v
	c.entries.Store(&next)
	return v
}

// size 返回当前条目数（诊断与测试用）
func (c *cowMap[V]) size() int {
	if m := c.entries.Load(); m != nil {
		return len(*m)
	}
	return 0
}

// ============================================================================
// ClientBreaker 客户端熔断器
// [EN] Client-side breaker
// ============================================================================

// ClientBreaker 客户端熔断器
//
// 不自带状态机：三态转换、阈值判定、半开恢复全部复用 middleware.Breaker 与
// middleware.BreakerManager，这里只负责两件客户端特有的事——算出熔断 key，
// 以及把「哪些方法要保护」的判定结果缓存起来
//
// [EN] Holds no state machine of its own: the three states, the thresholds and the
// half-open recovery all come from middleware.Breaker / middleware.BreakerManager. This
// type only does the two client-specific things — deriving the breaker key and memoizing
// which methods are protected
type ClientBreaker struct {
	manager *middleware.BreakerManager
	keyMode ClientBreakerKeyMode

	// scoped 为 true 表示配置了 prevention-paths，保护范围交给 manager.IsPathProtected 前缀匹配；
	// 为 false 表示「除排除项外全部保护」
	scoped   bool
	excluded map[string]struct{}

	// targets: target -> (method -> *Breaker)，值为 nil 表示该方法判定为不保护
	// 两级结构避免拼接 target+method 字符串（那会在每次 RPC 上多一次分配）
	targets cowMap[*cowMap[*middleware.Breaker]]
}

// NewClientBreaker 创建客户端熔断器
//
// cfg 为 nil 时取 DefaultClientBreakerConfig()；cfg.Enabled 为 false 时返回 nil，
// 拦截器遇到 nil 直接直通——「关闭」就是不存在，不需要在热路径上再判一次开关
// keyMode 为空时取 BreakerKeyTargetMethod
//
// [EN] A nil cfg falls back to DefaultClientBreakerConfig(); a disabled cfg returns nil and
// the interceptors pass through, so "off" costs nothing beyond one nil check. An empty
// keyMode defaults to BreakerKeyTargetMethod
func NewClientBreaker(cfg *gcmiddleware.CircuitBreaker, keyMode ClientBreakerKeyMode) *ClientBreaker {
	if cfg == nil {
		cfg = DefaultClientBreakerConfig()
	}
	if !cfg.Enabled {
		return nil
	}
	return NewClientBreakerWithManager(middleware.NewBreakerManager(cfg), cfg, keyMode)
}

// NewClientBreakerWithManager 用已有的 BreakerManager 创建客户端熔断器
//
// 用于与服务端熔断共享一个 manager，让 /breaker 这类诊断端点一次看到两侧的全部断路器
// 共享是安全的：客户端 key 带着 target（形如 user-svc:9000/user.v1.UserService/GetUser），
// 服务端 key 是裸方法名（形如 /user.v1.UserService/GetUser），不会撞在一起
// cfg 必须是构造 manager 用的那份配置，ClientBreaker 要从里面读保护范围
//
// [EN] Lets the client share one manager with the server side so a diagnostic endpoint can
// list every breaker at once. Sharing is safe because client keys carry the target while
// server keys are bare method names, so they cannot collide. cfg must be the same config the
// manager was built from, since the protection scope is read from it
func NewClientBreakerWithManager(manager *middleware.BreakerManager, cfg *gcmiddleware.CircuitBreaker, keyMode ClientBreakerKeyMode) *ClientBreaker {
	if manager == nil {
		return nil
	}
	if cfg == nil {
		cfg = DefaultClientBreakerConfig()
	}
	// 这里的回落是静默的，且刻意保持静默：keyMode 已经是枚举类型，走到 default 只可能是
	// 编程时硬写了一个不存在的值，不是配置写错
	// 来自 YAML 的裸字符串请先过 ParseClientBreakerKeyMode，由它告诉调用方取值认不认得，
	// 配置写错要有提示，构造函数打日志则会污染每一次测试与非网关进程的输出
	//
	// [EN] This fallback is silent on purpose: keyMode is already a typed enum, so reaching
	// default means a hard-coded bogus value in code, not a configuration typo. Raw strings
	// from YAML go through ParseClientBreakerKeyMode, which reports whether the spelling was
	// recognized — a misconfiguration deserves a warning, while a constructor that logs would
	// pollute every test and every non-gateway process
	switch keyMode {
	case BreakerKeyTarget, BreakerKeyMethod, BreakerKeyTargetMethod:
	default:
		keyMode = BreakerKeyTargetMethod
	}

	// 排除集合 = 配置里的 exclude-paths + 健康检查/反射（后者无条件排除，理由见
	// DefaultClientBreakerExcludeMethods）
	excluded := make(map[string]struct{}, len(cfg.ExcludePaths)+len(DefaultClientBreakerExcludeMethods))
	for _, p := range cfg.ExcludePaths {
		excluded[p] = struct{}{}
	}
	for _, p := range DefaultClientBreakerExcludeMethods {
		excluded[p] = struct{}{}
	}

	return &ClientBreaker{
		manager:  manager,
		keyMode:  keyMode,
		scoped:   len(cfg.PreventionPaths) > 0,
		excluded: excluded,
	}
}

// KeyMode 返回熔断 key 粒度
func (cb *ClientBreaker) KeyMode() ClientBreakerKeyMode {
	return cb.keyMode
}

// Manager 返回底层 BreakerManager，用于采集快照、重置断路器等诊断操作
func (cb *ClientBreaker) Manager() *middleware.BreakerManager {
	return cb.manager
}

// BreakerKey 按当前粒度拼出熔断 key（诊断与测试用，热路径不走这里）
func (cb *ClientBreaker) BreakerKey(target, method string) string {
	switch cb.keyMode {
	case BreakerKeyTarget:
		return target
	case BreakerKeyMethod:
		return method
	default:
		// method 以 / 开头，直接相接即可得到 user-svc:9000/user.v1.UserService/GetUser
		return target + method
	}
}

// isProtected 判定某个方法是否需要熔断保护
//
// 配置了 prevention-paths 时交给 manager.IsPathProtected 做前缀匹配
// （此时前缀要按 gRPC 全限定方法名写，例如 /user.v1.UserService/ 保护该服务全部方法）；
// 没配时「除排除项外全部保护」
// 这个判定含线性前缀扫描，结果由 BreakerFor 缓存，每个方法只算一次
//
// [EN] With prevention-paths configured the decision is delegated to
// manager.IsPathProtected (prefixes must then be written as gRPC method names, e.g.
// /user.v1.UserService/ to cover every method of that service); without them everything but
// the exclusion set is protected. The check includes a linear prefix scan, so BreakerFor
// memoizes it per method
func (cb *ClientBreaker) isProtected(method string) bool {
	if _, ok := cb.excluded[method]; ok {
		return false
	}
	if cb.scoped {
		return cb.manager.IsPathProtected(method)
	}
	return true
}

// BreakerFor 返回 (target, method) 对应的断路器，返回 nil 表示该方法不受保护
//
// 结果（含 nil）按 target/method 两级缓存，之后每次 RPC 只是两次原子读加两次 map 索引
//
// [EN] Returns the breaker for this (target, method) pair, or nil when the method is not
// protected. Both outcomes are memoized, so a subsequent RPC costs two atomic loads and two
// map lookups
func (cb *ClientBreaker) BreakerFor(target, method string) *middleware.Breaker {
	if cb == nil {
		return nil
	}

	methods := cb.targets.loadOrCreate(target, func() *cowMap[*middleware.Breaker] {
		return &cowMap[*middleware.Breaker]{}
	})

	return methods.loadOrCreate(method, func() *middleware.Breaker {
		if !cb.isProtected(method) {
			return nil
		}
		return cb.manager.GetBreaker(cb.BreakerKey(target, method))
	})
}

// recordClientBreakerResult 按下游返回的 error 记录成功/失败
//
// 失败判定直接复用 middleware.IsBreakerFailureCode，与服务端共用一张表：
// {Unknown, DeadlineExceeded, Internal, Unavailable, DataLoss} 算故障，
// InvalidArgument / NotFound / PermissionDenied / ResourceExhausted / Unimplemented 不算
// 客户端侧还有两个本地语义值得留意，它们落在同一张表里也是对的：
//   - Canceled：调用方自己取消（上游断开、ctx 取消），下游没坏，不记失败
//   - DeadlineExceeded：记失败，这是客户端熔断最主要的触发信号
//     （注意它也包含「调用方给的超时本来就太短」这种情况，这与 go-zero 的取舍一致：
//     从调用方视角看，一个总是等不到结果的下游就该被熔断）
//
// [EN] Classification is delegated to middleware.IsBreakerFailureCode so both sides share a
// single table. Two client-local cases are worth noting and both fall out correctly:
// Canceled (the caller gave up, the dependency is fine) is not a failure, while
// DeadlineExceeded is the primary trigger for a client breaker — including the case where
// the caller's own deadline was simply too tight, which matches go-zero's trade-off: from
// the caller's point of view a dependency that never answers in time should be cut off
func recordClientBreakerResult(b *middleware.Breaker, err error) {
	if err != nil && middleware.IsBreakerFailureCode(status.Code(err)) {
		b.RecordFailure()
		return
	}
	b.RecordSuccess()
}

// clientTarget 安全读取拨号目标
// 正常由 gRPC 调用时 cc 不会为 nil，这里兜一层是为了让拦截器在 mock 场景下也不会 panic
func clientTarget(cc *grpc.ClientConn) string {
	if cc == nil {
		return ""
	}
	return cc.Target()
}

// UnaryClientBreakerInterceptor 客户端一元熔断拦截器
//
// cb 为 nil 时直通（未启用熔断的默认情况）
// 熔断打开时立即返回预构建的 codes.Unavailable 错误，不发起网络调用、不等超时、不打日志
// （状态转换日志由 middleware.Breaker 在转换时打一次，打开期间逐次拒绝再打日志只会刷屏）
// 被拒绝的调用不计入统计：那是本地决策，不是下游的表现，计进去会自我强化永远出不来
//
// 在拦截器链里应当放在最靠近 invoker 的位置（见 buildDialOptions）：
// 这样熔断统计只反映真实的下游调用结果，健康检查拦截器的本地短路不会被当成下游故障，
// 同时被熔断拒绝的调用仍会经过日志拦截器，运维看得见
//
// [EN] A nil cb passes through. An open breaker returns the pre-built codes.Unavailable
// error immediately — no network call, no waiting for the deadline, no logging (the state
// transition is logged once by middleware.Breaker; logging every rejection would only flood
// the log while the breaker is open). Rejected calls are not recorded, because a local
// decision is not evidence about the dependency and counting it would be self-reinforcing.
// In the chain this interceptor belongs closest to the invoker (see buildDialOptions): its
// statistics then reflect only real downstream outcomes, the health interceptor's local
// short-circuit is not mistaken for a downstream failure, and rejected calls still pass
// through the logging interceptor so they stay visible
func UnaryClientBreakerInterceptor(cb *ClientBreaker) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if cb == nil {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		breaker := cb.BreakerFor(clientTarget(cc), method)
		if breaker == nil {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		if !breaker.Allow() {
			return clientBreakerOpenErr
		}

		err := invoker(ctx, method, req, reply, cc, opts...)
		recordClientBreakerResult(breaker, err)
		return err
	}
}

// StreamClientBreakerInterceptor 客户端流式熔断拦截器
//
// 判定逻辑与 Unary 一致，计数粒度是「一条流的建立」而不是「一条消息」：
// 建流时 Allow() 一次，并按建流是否成功记一次结果
// 流建起来之后的消息收发不再经过拦截器，流内错误也就不会计入熔断统计——这是 gRPC
// 客户端拦截器的固有边界（StreamClientInterceptor 只在建流时被调用一次），
// 与服务端 Stream 拦截器「一条流记一次」的粒度刻意保持一致
// 代价要说清楚：长连接流在建立之后的异常是熔断器看不到的，依赖这种流的服务
// 应当同时依赖健康检查
//
// [EN] Same classification as the unary interceptor, but the unit of accounting is stream
// establishment: Allow() is checked once and the outcome of creating the stream is recorded
// once. Messages exchanged afterwards never reach the interceptor, so errors inside a live
// stream do not feed the breaker — an inherent boundary of gRPC client interceptors, which
// are invoked exactly once per stream, kept deliberately consistent with the server-side
// stream interceptor. The cost is explicit: failures after establishment are invisible here,
// so services that rely on long-lived streams should also rely on health checking
func StreamClientBreakerInterceptor(cb *ClientBreaker) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		if cb == nil {
			return streamer(ctx, desc, cc, method, opts...)
		}

		breaker := cb.BreakerFor(clientTarget(cc), method)
		if breaker == nil {
			return streamer(ctx, desc, cc, method, opts...)
		}

		if !breaker.Allow() {
			return nil, clientBreakerOpenErr
		}

		cs, err := streamer(ctx, desc, cc, method, opts...)
		recordClientBreakerResult(breaker, err)
		return cs, err
	}
}

// ============================================================================
// 包级默认熔断器 - 编程式入口，优先于每服务配置
// [EN] Package-level default: the programmatic entry point, which wins over per-service config
// ============================================================================
//
// 常规开启方式是每服务配置（grpc.clients.<name>.enable-breaker / breaker-key-mode，
// 接线见 client.go 的 resolveClientBreaker）
// 这组包级函数仍然保留，它们能表达 YAML 表达不了的两件事：
//
//   - 自定义阈值：失败/成功阈值、恢复超时、volume-threshold、prevention-paths、
//     exclude-paths 全都只能从 gcmiddleware.CircuitBreaker 来，GRPCClient 上只有
//     「开不开」和「key 粒度」两个字段
//   - 与服务端熔断共享一个 BreakerManager（NewClientBreakerWithManager），
//     让一个诊断端点一次看到两侧的全部断路器
//
// 因为配置永远不可能是它们的超集，所以两者同时存在时以包级实例为准（并打一条 Warn）
//
// [EN] The ordinary way to turn the client breaker on is per-service configuration
// (grpc.clients.<name>.enable-breaker / breaker-key-mode, wired in client.go's
// resolveClientBreaker). These package-level functions are kept because they express two
// things YAML cannot: custom thresholds — failure/success thresholds, recovery timeout,
// volume threshold, prevention/exclude paths all come from gcmiddleware.CircuitBreaker,
// while GRPCClient carries only an on/off flag and a key granularity — and sharing one
// BreakerManager with the server side so a single diagnostic endpoint sees every breaker.
// Config can therefore never be a superset of them, which is why a registered package-level
// instance takes precedence (with a warning) when both are present

// defaultClientBreaker 包级默认客户端熔断器，零值（nil）表示不启用
// 用 atomic.Pointer 而不是 RWMutex：拨号时读一次，配置时写一次，不需要互斥语义
var defaultClientBreaker atomic.Pointer[ClientBreaker]

// SetDefaultClientBreaker 设置包级默认客户端熔断器，传 nil 即关闭
//
// 只影响之后新建的连接：拦截器链在 grpc.NewClient 时就固定下来了，
// 已经建好并缓存在连接池里的连接不会追加拦截器
// 因此应当在初始化 gRPC 客户端之前调用（网关初始化阶段）
//
// [EN] Affects only connections created afterwards: the interceptor chain is fixed when
// grpc.NewClient is called, so connections already cached in the pool keep theirs. Call it
// before initializing any gRPC client
func SetDefaultClientBreaker(cb *ClientBreaker) {
	defaultClientBreaker.Store(cb)
}

// GetDefaultClientBreaker 返回包级默认客户端熔断器，未启用时为 nil
func GetDefaultClientBreaker() *ClientBreaker {
	return defaultClientBreaker.Load()
}

// EnableClientBreaker 按配置创建并注册包级默认客户端熔断器，返回创建出的实例
// cfg.Enabled 为 false 时等价于 DisableClientBreaker()，返回 nil
func EnableClientBreaker(cfg *gcmiddleware.CircuitBreaker, keyMode ClientBreakerKeyMode) *ClientBreaker {
	cb := NewClientBreaker(cfg, keyMode)
	SetDefaultClientBreaker(cb)
	return cb
}

// DisableClientBreaker 关闭包级默认客户端熔断器（之后新建的连接不再带熔断拦截器）
func DisableClientBreaker() {
	SetDefaultClientBreaker(nil)
}
