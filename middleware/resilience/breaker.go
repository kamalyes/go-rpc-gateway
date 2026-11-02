/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 15:15:05
 * @FilePath: \go-rpc-gateway\middleware\resilience\breaker.go
 * @Description: 熔断器核心 - 断路器状态机与统计，以及 HTTP 中间件与 gRPC 拦截器适配
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package resilience

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	breakerconfig "github.com/kamalyes/go-config/pkg/breaker"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware/core"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// breakerOpenResp 预分配的熔断 503 JSON 响应体（避免每次拒绝请求时 map 分配 + json 反射编码）
var breakerOpenResp = []byte(`{"code":503,"message":"Service temporarily unavailable (circuit breaker open)","success":false}` + "\n")

// BreakerState 熔断器状态
type BreakerState string

const (
	BreakerClosed   BreakerState = "closed"    // 关闭，正常工作
	BreakerOpen     BreakerState = "open"      // 打开，拒绝请求
	BreakerHalfOpen BreakerState = "half_open" // 半开，尝试恢复
)

// Breaker 断路器 - 核心业务逻辑
// 状态用 atomic.Value 存储，计数字段用原子操作，热路径（Allow/RecordSuccess/RecordFailure）完全无锁
// mu 仅保护状态转换复合操作（lastFailureTime/lastStateChangeAt 及转换原子性）
type Breaker struct {
	mu                sync.Mutex
	state             atomic.Value // BreakerState
	failureThreshold  int
	successThreshold  int
	timeout           time.Duration
	volumeThreshold   int
	failureCount      atomic.Int32 // 连续失败计数
	successCount      atomic.Int32 // 半开状态连续成功计数
	totalRequests     atomic.Int64
	failedRequests    atomic.Int64
	lastFailureTime   time.Time // mu 保护，仅 Open 超时判断读取
	lastStateChangeAt time.Time // mu 保护
}

// NewBreaker 创建断路器
func NewBreaker(failureThreshold, successThreshold, volumeThreshold int, timeout time.Duration) *Breaker {
	b := &Breaker{
		failureThreshold:  failureThreshold,
		successThreshold:  successThreshold,
		volumeThreshold:   volumeThreshold,
		timeout:           timeout,
		lastStateChangeAt: time.Now(),
	}
	b.state.Store(BreakerClosed)
	return b
}

// currentState 无锁读取当前状态（atomic.Value 保证读原子性）
func (b *Breaker) currentState() BreakerState {
	if v := b.state.Load(); v != nil {
		return v.(BreakerState)
	}
	return BreakerClosed
}

// Allow 检查是否允许请求
// Closed/HalfOpen 常态无锁放行，仅 Open 状态升级写锁检查超时恢复
func (b *Breaker) Allow() bool {
	// 快速路径：非 Open 状态直接放行（无锁）
	if b.currentState() != BreakerOpen {
		return true
	}

	// Open 状态：需要写锁检查超时并可能转换状态
	b.mu.Lock()
	defer b.mu.Unlock()

	// 二次检查（可能在升级写锁前被其他 goroutine 转换）
	if b.currentState() != BreakerOpen {
		return true
	}

	// 超时后尝试恢复，转为半开
	if time.Since(b.lastFailureTime) > b.timeout {
		b.transitionToLocked(BreakerHalfOpen)
		return true
	}
	return false
}

// RecordSuccess 记录成功
// Closed 常态仅原子清零失败计数，完全无锁
func (b *Breaker) RecordSuccess() {
	b.totalRequests.Add(1)

	if b.currentState() == BreakerClosed {
		b.failureCount.Store(0)
		return
	}

	// 慢路径：HalfOpen 需维护连续成功计数并可能恢复闭合
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failureCount.Store(0)
	if b.currentState() == BreakerHalfOpen {
		if b.successCount.Add(1) >= int32(b.successThreshold) {
			b.transitionToLocked(BreakerClosed)
		}
	}
}

// RecordFailure 记录失败
// Closed 常态仅原子更新计数，达到阈值时才升级写锁完成状态转换
func (b *Breaker) RecordFailure() {
	b.totalRequests.Add(1)
	b.failedRequests.Add(1)
	b.successCount.Store(0)

	if b.currentState() == BreakerClosed {
		// 快速路径：未达阈值直接返回（绝大多数失败请求走此分支）
		if b.failureCount.Add(1) < int32(b.failureThreshold) ||
			b.totalRequests.Load() < int64(b.volumeThreshold) {
			return
		}
		// 达到阈值：升级写锁完成状态转换（double-check）
		b.mu.Lock()
		b.lastFailureTime = time.Now()
		if b.currentState() == BreakerClosed &&
			b.failureCount.Load() >= int32(b.failureThreshold) &&
			b.totalRequests.Load() >= int64(b.volumeThreshold) {
			b.transitionToLocked(BreakerOpen)
		}
		b.mu.Unlock()
		return
	}

	// 慢路径：HalfOpen 失败立即重新熔断
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failureCount.Add(1)
	b.lastFailureTime = time.Now()

	switch b.currentState() {
	case BreakerClosed: // 竞态兜底：快速路径读取状态后可能已被转换
		if b.failureCount.Load() >= int32(b.failureThreshold) &&
			b.totalRequests.Load() >= int64(b.volumeThreshold) {
			b.transitionToLocked(BreakerOpen)
		}
	case BreakerHalfOpen:
		b.transitionToLocked(BreakerOpen)
	}
}

// transitionToLocked 转换状态（调用者必须持有 mu 写锁）
func (b *Breaker) transitionToLocked(newState BreakerState) {
	oldState := b.currentState()
	b.state.Store(newState)
	b.lastStateChangeAt = time.Now()

	// Closed 和 HalfOpen 重置计数器
	if newState == BreakerClosed || newState == BreakerHalfOpen {
		b.failureCount.Store(0)
		b.successCount.Store(0)
	}

	// 状态转换非热路径，日志在锁内可接受
	if global.LOGGER != nil {
		fields := map[string]interface{}{
			"old_state": oldState,
			"new_state": newState,
		}
		if newState == BreakerOpen {
			fields["failure_count"] = b.failureCount.Load()
			fields["total_requests"] = b.totalRequests.Load()
			global.LOGGER.WithFields(fields).WarnMsg("Circuit breaker opened due to high failure rate")
		} else {
			global.LOGGER.WithFields(fields).InfoMsg("Circuit breaker state changed")
		}
	}
}

// GetState 获取当前状态
func (b *Breaker) GetState() BreakerState {
	return b.currentState()
}

// GetStats 获取统计信息（基于 Snapshot，避免 map 分配）
func (b *Breaker) GetStats() BreakerSnapshot {
	return b.Snapshot()
}

// BreakerSnapshot 熔断器快照（强类型，便于指标采集）
type BreakerSnapshot struct {
	State          BreakerState
	TotalRequests  int64
	FailedRequests int64
	FailureCount   int32
	SuccessCount   int32
}

// Snapshot 获取强类型快照（全原子读取，无锁）
func (b *Breaker) Snapshot() BreakerSnapshot {
	return BreakerSnapshot{
		State:          b.currentState(),
		TotalRequests:  b.totalRequests.Load(),
		FailedRequests: b.failedRequests.Load(),
		FailureCount:   b.failureCount.Load(),
		SuccessCount:   b.successCount.Load(),
	}
}

// Reset 重置断路器
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state.Store(BreakerClosed)
	b.failureCount.Store(0)
	b.successCount.Store(0)
	b.totalRequests.Store(0)
	b.failedRequests.Store(0)
	b.lastStateChangeAt = time.Now()

	if global.LOGGER != nil {
		global.LOGGER.InfoMsg("Circuit breaker reset")
	}
}

// ============================================================================
// BreakerManager 断路器管理器 - 管理多个路径的断路器实例
// ============================================================================

// BreakerManager 断路器管理器 - 直接持有 go-config 的 breaker.CircuitBreaker 配置
type BreakerManager struct {
	mu             sync.RWMutex
	breakers       map[string]*Breaker
	config         *breakerconfig.CircuitBreaker
	excludePathSet map[string]struct{} // 排除路径集合，O(1) 查找替代线性扫描
}

// NewBreakerManager 创建断路器管理器（直接接收 go-config 配置对象）
func NewBreakerManager(cfg *breakerconfig.CircuitBreaker) *BreakerManager {
	cfg = mathx.IF(cfg == nil, breakerconfig.Default(), cfg)
	// 预构建排除路径集合，热路径 O(1) 查找
	excludeSet := make(map[string]struct{}, len(cfg.ExcludePaths))
	for _, p := range cfg.ExcludePaths {
		excludeSet[p] = struct{}{}
	}
	return &BreakerManager{
		breakers:       make(map[string]*Breaker),
		config:         cfg,
		excludePathSet: excludeSet,
	}
}

// GetBreaker 获取或创建断路器
func (m *BreakerManager) GetBreaker(path string) *Breaker {
	m.mu.RLock()
	if breaker, exists := m.breakers[path]; exists {
		m.mu.RUnlock()
		return breaker
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// 再次检查（double-check lock）
	if breaker, exists := m.breakers[path]; exists {
		return breaker
	}

	// 从配置创建断路器（timeout 为纳秒时间戳，转换为 time.Duration）
	var duration time.Duration
	if m.config.Timeout > 0 {
		duration = time.Duration(m.config.Timeout)
	}

	breaker := NewBreaker(m.config.FailureThreshold, m.config.SuccessThreshold, m.config.VolumeThreshold, duration)
	m.breakers[path] = breaker
	return breaker
}

// GetAllBreakers 获取所有断路器
func (m *BreakerManager) GetAllBreakers() map[string]*Breaker {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Breaker)
	for path, breaker := range m.breakers {
		result[path] = breaker
	}
	return result
}

// GetAllBreakerSnapshots 获取所有断路器的强类型快照（用于指标采集）
func (m *BreakerManager) GetAllBreakerSnapshots() map[string]BreakerSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]BreakerSnapshot, len(m.breakers))
	for path, breaker := range m.breakers {
		result[path] = breaker.Snapshot()
	}
	return result
}

// ResetBreaker 重置特定路径的断路器
func (m *BreakerManager) ResetBreaker(path string) bool {
	m.mu.RLock()
	breaker, exists := m.breakers[path]
	m.mu.RUnlock()

	if !exists {
		return false
	}

	breaker.Reset()
	return true
}

// ResetAllBreakers 重置所有断路器
func (m *BreakerManager) ResetAllBreakers() {
	m.mu.RLock()
	breakers := make([]*Breaker, 0, len(m.breakers))
	for _, breaker := range m.breakers {
		breakers = append(breakers, breaker)
	}
	m.mu.RUnlock()

	for _, breaker := range breakers {
		breaker.Reset()
	}
}

// GetStats 获取所有断路器的统计信息
// GetStats 获取所有断路器统计（等价于 GetAllBreakerSnapshots，保留以兼容旧 API）
func (m *BreakerManager) GetStats() map[string]BreakerSnapshot {
	return m.GetAllBreakerSnapshots()
}

// IsPathProtected 检查路径是否需要保护（排除路径 O(1) map 查找，保护路径前缀匹配）
func (m *BreakerManager) IsPathProtected(path string) bool {
	// 排除列表：O(1) map 查找
	if _, excluded := m.excludePathSet[path]; excluded {
		return false
	}

	// 保护列表：前缀匹配
	for _, preventionPath := range m.config.PreventionPaths {
		if len(path) >= len(preventionPath) && path[:len(preventionPath)] == preventionPath {
			return true
		}
	}

	return false
}

// BreakerHealthStatus 熔断器健康状态（强类型，替代 map[string]interface{}）
type BreakerHealthStatus struct {
	Total    int  // 断路器总数
	Open     int  // 打开数量
	HalfOpen int  // 半开数量
	Closed   int  // 关闭数量
	Healthy  bool // 是否健康（无打开的断路器）
}

// CountByState 单次遍历统计各状态断路器数量（替代三次独立遍历）
func (m *BreakerManager) CountByState() BreakerHealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := BreakerHealthStatus{Total: len(m.breakers)}
	for _, breaker := range m.breakers {
		switch breaker.GetState() {
		case BreakerOpen:
			status.Open++
		case BreakerHalfOpen:
			status.HalfOpen++
		case BreakerClosed:
			status.Closed++
		}
	}
	status.Healthy = status.Open == 0
	return status
}

// CountOpenBreakers 统计打开的断路器数量
func (m *BreakerManager) CountOpenBreakers() int {
	return m.CountByState().Open
}

// CountHalfOpenBreakers 统计半开的断路器数量
func (m *BreakerManager) CountHalfOpenBreakers() int {
	return m.CountByState().HalfOpen
}

// CountClosedBreakers 统计关闭的断路器数量
func (m *BreakerManager) CountClosedBreakers() int {
	return m.CountByState().Closed
}

// GetHealthStatus 获取健康状态（返回强类型结构体，单次遍历）
func (m *BreakerManager) GetHealthStatus() BreakerHealthStatus {
	return m.CountByState()
}

// ============================================================================
// HTTP 中间件适配
// ============================================================================

// BreakerHTTPMiddleware 创建熔断 HTTP 中间件（使用共享的 BreakerManager）
// 熔断器打开时返回 503 JSON 错误响应（使用预分配的响应体，零分配）
func BreakerHTTPMiddleware(manager *BreakerManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 检查路径是否需要保护
			if !manager.IsPathProtected(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// 获取对应路径的断路器
			breaker := manager.GetBreaker(r.URL.Path)

			// 检查断路器状态
			if !breaker.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write(breakerOpenResp)
				return
			}

			// 使用统一的 ResponseWriter 捕获状态码
			rw := core.NewResponseWriter(w)
			defer rw.Release()

			// 调用下一个处理器
			next.ServeHTTP(rw, r)

			// 根据响应状态码记录成功或失败
			if rw.IsServerError() {
				breaker.RecordFailure()
			} else {
				breaker.RecordSuccess()
			}
		})
	}
}

// ============================================================================
// gRPC 拦截器适配
// [EN] gRPC interceptor adapters
// ============================================================================

// breakerOpenGRPCErr 预构建的熔断拒绝错误（与 HTTP 侧 breakerOpenResp 同理，避免每次拒绝都构造对象）
//
// 遵循仓库错误规范：熔断属于框架层保护，错误码取 errors.ErrCodeCircuitBreakerOpen(4003)，
// 经 AppError.ToGRPCError() 得到 codes.Unavailable（三态映射见 docs/ERRORS.md），
// 与 HTTP 侧返回的 503 语义一致；不用裸 status.Errorf 是为了让错误码体系保持单一来源
// 这里不走 errors.ToGRPCError(ctx, bizCode) 的 i18n 路径：那条路径是给各微服务注册的
// 业务错误码用的（biz.go），熔断拒绝不是业务语义，也不该在被熔断时再付一次翻译查表的代价
//
// [EN] Pre-built rejection error, mirroring breakerOpenResp on the HTTP side so that a
// rejected call allocates nothing. It goes through the repository's error system rather
// than a bare status.Errorf: ErrCodeCircuitBreakerOpen(4003) maps to codes.Unavailable,
// matching the 503 the HTTP middleware returns. The i18n path (errors.ToGRPCError with a
// biz code) is deliberately not used — that one is for per-service business codes, while
// a breaker rejection is a framework-level decision and must not pay a lookup per reject
var breakerOpenGRPCErr = errors.NewError(errors.ErrCodeCircuitBreakerOpen, "").ToGRPCError()

// IsBreakerFailureCode 判定一个 gRPC status code 是否算作「下游故障」
//
// 只有下游真的坏了才应该计入失败，否则客户端自己的错误会把健康的下游熔断掉
// 判定为故障（等价于 HTTP 5xx，与 BreakerHTTPMiddleware 的 IsServerError 对齐）：
//   - Unknown            handler panic 或返回了非 status 错误，客户端看到的就是 500
//   - DeadlineExceeded   下游超时，熔断器存在的首要信号
//   - Internal           下游内部错误
//   - Unavailable        下游不可用，最典型的熔断场景
//   - DataLoss           不可恢复的数据损坏
//
// 不计入故障（客户端自身问题或业务语义，等价于 HTTP 4xx）：
//   - Canceled           调用方主动取消，下游还活着
//   - InvalidArgument / FailedPrecondition / OutOfRange  参数与业务前置条件不满足
//   - NotFound / AlreadyExists / Aborted                 资源状态问题
//   - PermissionDenied / Unauthenticated                 鉴权问题
//   - ResourceExhausted  限流/配额背压；计入会形成「流量高峰→限流拒绝→熔断打开」的正反馈
//   - Unimplemented      方法不存在是确定性的契约错误，重试永远失败，熔断只会把
//     Unimplemented 换成信息量更少的 Unavailable，掩盖版本/部署不一致
//     （这是对「5xx 即故障」的一处有意偏离：grpc-gateway 把它映射成 HTTP 501）
//
// 注意：判定只看客户端最终收到的 status code，即 status.Code(err)
// 若 service 直接返回 *errors.AppError 而没有调用 .ToGRPCError()，gRPC 对外给出的是
// Unknown，这里也就按 Unknown 记为故障——与客户端视角保持一致，同时暴露这种用法问题
//
// [EN] Decides whether a gRPC status code counts as a downstream failure. Only codes that
// mean "the dependency is broken" are counted (Unknown, DeadlineExceeded, Internal,
// Unavailable, DataLoss) — the same set that maps to HTTP 5xx, keeping this consistent with
// the HTTP middleware's IsServerError check. Client-caused codes are not counted, otherwise
// a stream of bad requests would trip a breaker in front of a perfectly healthy service.
// ResourceExhausted is excluded on purpose: counting it would create a traffic-spike →
// rate-limit-reject → breaker-open feedback loop. Unimplemented is excluded even though it
// maps to HTTP 501, because a missing method is a deterministic contract error that retrying
// never fixes, and tripping the breaker would only replace it with a vaguer Unavailable.
// Classification uses status.Code(err), i.e. exactly what the client receives
func IsBreakerFailureCode(code codes.Code) bool {
	switch code {
	case codes.Unknown, codes.DeadlineExceeded, codes.Internal, codes.Unavailable, codes.DataLoss:
		return true
	default:
		return false
	}
}

// recordBreakerResult 按 handler 返回的 error 记录成功/失败（Unary 与 Stream 共用）
// err 为 nil 时 status.Code 返回 codes.OK，直接计入成功
//
// [EN] Records the outcome of a call on the breaker; shared by the unary and stream
// interceptors. status.Code returns codes.OK for a nil error, which counts as a success
func recordBreakerResult(b *Breaker, err error) {
	if IsBreakerFailureCode(status.Code(err)) {
		b.RecordFailure()
		return
	}
	b.RecordSuccess()
}

// UnaryServerBreakerInterceptor 创建熔断 gRPC 一元拦截器（复用共享的 BreakerManager）
//
// 与 HTTP 中间件共用同一套 Breaker 状态机和 BreakerManager 实例，差别只在熔断 key：
// HTTP 用 r.URL.Path，gRPC 用 info.FullMethod（形如 /user.v1.UserService/GetUser）
// 因此 prevention-paths / exclude-paths 需要按 gRPC 全限定方法名配置，
// 前缀匹配对 /user.v1.UserService/ 这样的前缀同样生效，可一次保护某个服务的所有方法
// 熔断打开时返回预构建的 codes.Unavailable 错误，不执行 handler，也不打日志
// （状态转换本身已在 transitionToLocked 中记日志，避免打开期间日志刷屏）
// manager 为 nil 时直通，便于未启用熔断的场景安全调用
//
// [EN] Unary breaker interceptor, reusing the very same Breaker state machine and
// BreakerManager instance as the HTTP middleware; only the key differs — the HTTP side keys
// on r.URL.Path, this one on info.FullMethod such as /user.v1.UserService/GetUser, so
// prevention-paths and exclude-paths must be configured with fully qualified gRPC method
// names (prefix matching still works, e.g. /user.v1.UserService/ protects every method of
// that service). An open breaker returns the pre-built codes.Unavailable error without
// invoking the handler and without logging — state transitions are already logged in
// transitionToLocked, so logging each reject would only flood the log while the breaker is
// open. A nil manager passes through, so callers that disabled the breaker stay safe
func UnaryServerBreakerInterceptor(manager *BreakerManager) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 未启用或方法不需要保护：直通，不创建断路器
		if manager == nil || !manager.IsPathProtected(info.FullMethod) {
			return handler(ctx, req)
		}

		// 获取该方法对应的断路器
		breaker := manager.GetBreaker(info.FullMethod)

		// 断路器打开：尽早拒绝，不触达 handler
		if !breaker.Allow() {
			return nil, breakerOpenGRPCErr
		}

		resp, err := handler(ctx, req)
		recordBreakerResult(breaker, err)
		return resp, err
	}
}

// StreamServerBreakerInterceptor 创建熔断 gRPC 流式拦截器（复用共享的 BreakerManager）
//
// 判定逻辑与 Unary 完全一致，但计数粒度是「一条流」而不是「一条消息」：
// 建流时检查一次 Allow()，流结束（handler 返回）时按返回的 status code 记一次结果
// 这与限流 Stream 拦截器「建流时限流一次」的粒度保持一致；长连接流在结束前不贡献统计，
// 这是刻意的取舍——流内每条消息都记一次会让单条异常长流主导整个方法的熔断统计
//
// [EN] Stream breaker interceptor. The classification is identical to the unary one, but
// the unit of accounting is a stream rather than a message: Allow() is checked once when the
// stream is established and the outcome is recorded once when the handler returns, matching
// the granularity of the stream rate-limit interceptor. A long-lived stream therefore
// contributes no statistics until it ends — a deliberate trade-off, since recording every
// message would let one unusually long stream dominate the method's breaker statistics
func StreamServerBreakerInterceptor(manager *BreakerManager) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if manager == nil || !manager.IsPathProtected(info.FullMethod) {
			return handler(srv, ss)
		}

		breaker := manager.GetBreaker(info.FullMethod)

		if !breaker.Allow() {
			return breakerOpenGRPCErr
		}

		err := handler(srv, ss)
		recordBreakerResult(breaker, err)
		return err
	}
}
