/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 17:55:10
 * @FilePath: \go-rpc-gateway\middleware\manager.go
 * @Description: 中间件管理器
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	gcmiddleware "github.com/kamalyes/go-rpc-gateway/config/middleware"
	gwswagger "github.com/kamalyes/go-rpc-gateway/config/swagger"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware/core"
	"github.com/kamalyes/go-rpc-gateway/middleware/i18n"
	"github.com/kamalyes/go-rpc-gateway/middleware/observability"
	"github.com/kamalyes/go-rpc-gateway/middleware/resilience"
	"github.com/kamalyes/go-rpc-gateway/middleware/security"
	swaggerMiddleware "github.com/kamalyes/go-rpc-gateway/middleware/swagger"
	"google.golang.org/grpc"
)

// managerCloseTimeout 回收后台资源时等待的上限
//
// 只有链路追踪的 Shutdown 需要时间：它要把缓冲里还没导出的 span 刷到后端
// 给上限而不是无限等，是因为热重载不能被一个连不上的 collector 卡住
//
// [EN] Only tracing's Shutdown takes time, flushing spans still buffered for export. It is
// bounded rather than unbounded because a hot reload must not hang on an unreachable collector.
const managerCloseTimeout = 5 * time.Second

// Manager 中间件管理器 - 使用 go-config 的 middleware 配置
type Manager struct {
	cfg                     *gwconfig.Gateway
	metricsManager          *MetricsManager
	tracingManager          *TracingManager
	rateLimiter             RateLimiter
	dynamicRateLimit        DynamicRateLimitProvider
	dynamicSignature        DynamicSignatureProvider
	i18nManager             *I18nManager
	swaggerMiddleware       *swaggerMiddleware.Middleware
	breakerManager          *BreakerManager  // 熔断器管理器（用于指标采集）
	shedderManager          *ShedderManager  // 自适应降载管理器（用于指标采集）
	preRateLimitMiddlewares []HTTPMiddleware // 在限流之前执行的自定义中间件（如认证）
}

// normalizeConfigSections 返回一份各配置段均非 nil 的浅拷贝
//
// NewManager 与 Manager 的各个方法直接解引用配置段（cfg.Middleware.Tracing.Enabled、
// cfg.Monitoring.Metrics.Enabled、cfg.Swagger.Enabled …）。完整的默认配置满足这个前提，
// 但「调用方手工拼的配置」和「YAML 里缺整段」都不满足 —— 一处缺段就是启动期空指针，
// 而 NewManager 位于启动与热重载的必经路径上
//
// 缺段在这里统一补成零值（零值的 Enabled 均为 false，等价于未启用），
// 而不是要求每个调用点重复判空。只复制被补字段的外层结构，不改调用方的 cfg；
// 原有指针共享，热重载语义不变
// [EN] NewManager and Manager methods dereference config sections directly
// (cfg.Middleware.Tracing.Enabled, cfg.Monitoring.Metrics.Enabled, cfg.Swagger.Enabled, ...).
// Full default configs satisfy that, but hand-built configs and YAML with a missing section
// do not, and one missing section is a nil dereference on the startup/hot-reload path.
// Missing sections are filled with zero values here (Enabled defaults to false, i.e. off)
// instead of repeating nil checks at every call site. Only the structs whose fields are
// filled are copied; the caller's cfg is untouched and shared pointers keep reload semantics
func normalizeConfigSections(cfg *gwconfig.Gateway) *gwconfig.Gateway {
	clone := *cfg
	cfg = &clone

	if cfg.Middleware == nil {
		cfg.Middleware = &gcmiddleware.Middleware{}
	} else {
		mw := *cfg.Middleware
		cfg.Middleware = &mw
	}
	if cfg.Middleware.Tracing == nil {
		cfg.Middleware.Tracing = &gcmiddleware.Tracing{}
	}
	if cfg.Middleware.I18N == nil {
		cfg.Middleware.I18N = &gcmiddleware.I18N{}
	}
	if cfg.Middleware.Logging == nil {
		cfg.Middleware.Logging = &gcmiddleware.Logging{}
	}
	if cfg.Middleware.Recovery == nil {
		cfg.Middleware.Recovery = &gcmiddleware.Recovery{}
	}
	if cfg.Middleware.Signature == nil {
		cfg.Middleware.Signature = &gcmiddleware.Signature{}
	}
	if cfg.Middleware.CircuitBreaker == nil {
		cfg.Middleware.CircuitBreaker = &gcmiddleware.CircuitBreaker{}
	}

	if cfg.Monitoring == nil {
		cfg.Monitoring = &gcmiddleware.Monitoring{}
	} else {
		mon := *cfg.Monitoring
		cfg.Monitoring = &mon
	}
	if cfg.Monitoring.Metrics == nil {
		cfg.Monitoring.Metrics = &gcmiddleware.MonitoringMetrics{}
	}

	if cfg.Security == nil {
		cfg.Security = &gcmiddleware.Security{}
	} else {
		sec := *cfg.Security
		cfg.Security = &sec
	}
	if cfg.Security.CSP == nil {
		cfg.Security.CSP = &gcmiddleware.CSP{}
	}

	if cfg.Swagger == nil {
		cfg.Swagger = &gwswagger.Swagger{}
	}
	if cfg.RateLimit == nil {
		cfg.RateLimit = &gcmiddleware.RateLimit{}
	}
	if cfg.CORS == nil {
		cfg.CORS = &gcmiddleware.Cors{}
	}

	return cfg
}

// NewManager 创建中间件管理器 - 使用全局 GATEWAY 配置
func NewManager(cfg *gwconfig.Gateway) (*Manager, error) {
	if cfg == nil {
		return nil, errors.NewErrorf(errors.ErrCodeMiddlewareError, "gateway config is nil")
	}

	cfg = normalizeConfigSections(cfg)

	var err error
	manager := &Manager{
		cfg: cfg,
	}

	// 初始化监控管理器（使用 monitoring 配置）
	if cfg.Monitoring.Metrics.Enabled {
		manager.metricsManager = observability.NewMetricsManager(cfg.Monitoring)
	}

	// 初始化链路追踪管理器
	if cfg.Middleware.Tracing.Enabled {
		manager.tracingManager, err = observability.NewTracingManager(cfg.Middleware.Tracing)
		if err != nil {
			return nil, errors.NewErrorf(errors.ErrCodeTracingError, "failed to init tracing manager: %v", err)
		}
	}

	// 初始化i18n管理器
	if cfg.Middleware.I18N.Enabled {
		manager.i18nManager, err = i18n.NewI18nManager(cfg.Middleware.I18N)
		if err != nil {
			return nil, errors.NewErrorf(errors.ErrCodeMiddlewareError, "failed to init i18n manager: %v", err)
		}
	}

	// 初始化 Swagger 中间件
	if cfg.Swagger.Enabled {
		manager.swaggerMiddleware = swaggerMiddleware.NewMiddleware(cfg.Swagger,
			swaggerMiddleware.WithLogger(global.LOGGER),
		)
		if err != nil {
			return nil, errors.NewErrorf(errors.ErrCodeMiddlewareError, "failed to init swagger middleware: %v", err)
		}
		global.LOGGER.Info("Swagger文档中间件已初始化 [ui_path=%s, enabled=%v]",
			cfg.Swagger.UIPath, true)
	}

	// 初始化限流器（如果启用）
	if cfg.RateLimit.Enabled {
		// 根据策略选择限流器实现
		switch cfg.RateLimit.Strategy {
		case gcmiddleware.StrategyTokenBucket:
			manager.rateLimiter = resilience.NewTokenBucketLimiter(cfg.RateLimit)
		case gcmiddleware.StrategySlidingWindow:
			if global.REDIS != nil {
				manager.rateLimiter = resilience.NewSlidingWindowLimiter(cfg.RateLimit)
			} else {
				manager.rateLimiter = resilience.NewTokenBucketLimiter(cfg.RateLimit) // 降级到令牌桶
				global.LOGGER.Warn("Redis不可用，限流器降级为令牌桶模式")
			}
		case gcmiddleware.StrategyFixedWindow:
			manager.rateLimiter = resilience.NewFixedWindowLimiter(cfg.RateLimit)
		default:
			manager.rateLimiter = resilience.NewTokenBucketLimiter(cfg.RateLimit)
		}

		var rps, burst int
		if cfg.RateLimit.GlobalLimit != nil {
			rps = cfg.RateLimit.GlobalLimit.RequestsPerSecond
			burst = cfg.RateLimit.GlobalLimit.BurstSize
		}
		global.LOGGER.Info("限流器已初始化 [strategy=%s, rps=%d, burst=%d, enabled=%v]",
			cfg.RateLimit.Strategy, rps, burst, true)
	}

	// 初始化熔断器管理器（直接传入 go-config 的 CircuitBreaker 配置）
	if cfg.Middleware.CircuitBreaker.Enabled {
		manager.breakerManager = resilience.NewBreakerManager(cfg.Middleware.CircuitBreaker)
		global.LOGGER.Info("熔断器管理器已初始化 [failure_threshold=%d, timeout=%d, enabled=%v]",
			cfg.Middleware.CircuitBreaker.FailureThreshold, cfg.Middleware.CircuitBreaker.Timeout, true)
	}

	// 初始化自适应降载管理器（直接传入 go-config 的 Shedder 配置，与熔断同一套写法）
	// 配置缺省（没有 middleware.shedder 子树且使用方自建了 Middleware 配置）时视为未启用
	if shedderCfg := cfg.Middleware.Shedder; shedderCfg != nil && shedderCfg.Enabled {
		manager.shedderManager = resilience.NewShedderManager(shedderCfg)
		effective := manager.shedderManager.Config()
		global.LOGGER.Info("自适应降载管理器已初始化 [cpu_threshold=%d, max_concurrency=%d, window_ms=%d, buckets=%d, cooldown_ms=%d, cpu_source=%s, enabled=%v]",
			effective.CPUThreshold, effective.MaxConcurrency, effective.WindowMS,
			effective.WindowBuckets, effective.CooldownMS, resilience.CPUUsageSource(), true)
	}

	return manager, nil
}

// UpdateConfig 更新中间件管理器配置
//
// 做法是用新配置整建一个管理器再换上去，所以「跨配置活下来的东西」必须逐项带过去，
// 「被换下来的东西」必须逐项回收 —— 这两件事各自漏过一项：
//
//  1. preRateLimitMiddlewares 没有被带过去 它的文档用途就是放认证中间件
//     （见 AddPreRateLimitMiddleware），丢掉等于热重载之后认证从 HTTP 链上消失，
//     而且不报错、不告警，表现是重载后接口突然不需要凭据
//  2. 旧管理器的后台资源没有被回收，每次热重载泄一份，见 closeResources
//
// 另外原先会先对「即将被丢弃的那个」swaggerMiddleware 调一次 UpdateConfig
// 那个调用没有任何持久效果，因为紧接着 *m = *next 就把它整个换掉了；更糟的是
// swagger 的 UpdateConfig 内部会重启文件监听，也就是在一个马上要扔掉的实例上
// 又起了一个 fsnotify 监听 现在改为直接由 NewManager 建好新的，旧的交给 closeResources
//
// *m = *next 这种整体覆盖之所以目前是安全的：Manager 的每个方法都只在建链时读字段、
// 把值捕获进闭包，没有任何方法在返回的 handler 内部再读 m，而建链发生在
// server 侧持有 s.mu 的重载流程里 哪天有方法改成在请求路径上懒读 m 的字段，
// 这里就必须换成换指针而不是覆盖结构体
//
// [EN] The approach is to build a whole new manager from the new config and swap it in, so
// everything that must survive the config change has to be carried over item by item, and
// everything being replaced has to be reclaimed item by item. One item was missed on each side:
//
//  1. preRateLimitMiddlewares was not carried over. Its documented purpose is authentication
//     middleware (see AddPreRateLimitMiddleware), so losing it means authentication silently
//     disappears from the HTTP chain after a reload, with no error and no warning — the symptom
//     is endpoints suddenly not requiring credentials.
//  2. The old manager's background resources were not reclaimed, leaking one set per reload;
//     see closeResources.
//
// There also used to be an UpdateConfig call on the swaggerMiddleware that was about to be
// discarded. It had no lasting effect, since *m = *next replaced it immediately, and it was
// actively harmful: swagger's UpdateConfig restarts the file watcher, so it started an fsnotify
// watcher on an instance that was being thrown away. NewManager now builds the new one and
// closeResources takes the old one.
//
// Overwriting the struct wholesale is safe today because every Manager method reads its fields
// only while building the chain and captures the values into a closure; none of them read m
// inside the returned handler, and chain building happens inside the server's reload path while
// it holds s.mu. If a method ever starts reading m's fields lazily on the request path, this
// has to become a pointer swap rather than a struct overwrite.
func (m *Manager) UpdateConfig(cfg *gwconfig.Gateway) error {
	if m == nil {
		return nil
	}

	next, err := NewManager(cfg)
	if err != nil {
		return err
	}

	// 跨配置保留：动态提供器与限流前自定义中间件
	// [EN] Carried across the config change: dynamic providers and pre-rate-limit middlewares
	next.dynamicRateLimit = m.dynamicRateLimit
	next.dynamicSignature = m.dynamicSignature
	next.preRateLimitMiddlewares = m.preRateLimitMiddlewares

	old := *m
	*m = *next

	// 先换上新的再回收旧的：顺序反过来会在新配置生效之前就关掉还在服务的资源
	// [EN] Swap first, reclaim second: the other order closes resources still in service
	old.closeResources()
	return nil
}

// stoppableLimiter 需要显式停止的限流器
//
// 用可选接口而不是对 *FixedWindowLimiter 做类型断言：限流器是按策略选出来的，
// 以后新增一种自带后台协程的实现时，只要它有 Stop 就会被一并回收，不用改这里
//
// [EN] An optional interface rather than a type assertion on *FixedWindowLimiter: the limiter
// is chosen by strategy, so a future implementation with its own background goroutine gets
// reclaimed as long as it has Stop, without touching this code
type stoppableLimiter interface {
	Stop()
}

// Close 回收管理器持有的后台资源
//
// 热重载和进程关停都要走这里 不回收的话，每换一次配置就在进程里留下一份：
//
//   - TracingManager 的 TracerProvider 带 BatchSpanProcessor 的常驻协程与一条导出连接
//     Shutdown 在 observability/tracing.go 里一直是写好的，但在这次改动之前没有任何调用方
//   - FixedWindowLimiter 在构造函数里就 go cleanup()，Stop 同样是写好了没人调
//   - Swagger 中间件开着文件监听时持有一个 fsnotify 实例，也就是一组 OS 文件描述符
//     加一个 watchLoop 协程 这一项最危险：inotify 实例数在 Linux 上是进程级 rlimit，
//     反复热重载会把它耗光，之后任何需要监听文件的功能都起不来
//
// 返回值只聚合第一个错误：回收是尽力而为，一项失败不应让其余项不被回收
//
// [EN] Both hot reload and process shutdown go through here. Without it every config change
// leaves one more set behind in the process:
//
//   - TracingManager's TracerProvider carries a BatchSpanProcessor goroutine and an exporter
//     connection. Shutdown has always been written in observability/tracing.go but had no
//     caller at all before this change.
//   - FixedWindowLimiter starts go cleanup() in its constructor, and its Stop was likewise
//     written and never called.
//   - The swagger middleware with file watching holds an fsnotify instance — a set of OS file
//     descriptors — plus a watchLoop goroutine. This is the dangerous one: the number of inotify
//     instances is a per-process rlimit on Linux, so repeated reloads exhaust it and afterwards
//     anything that needs to watch a file cannot start.
//
// Only the first error is returned: reclaiming is best effort and one failure must not stop
// the remaining items from being reclaimed.
func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	return m.closeResources()
}

// closeResources 回收这一份管理器快照持有的后台资源
func (m *Manager) closeResources() error {
	if m == nil {
		return nil
	}

	var firstErr error
	keep := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if m.tracingManager != nil {
		// Shutdown 会把还没导出的 span 刷出去，给它一个有界的时间窗
		// [EN] Shutdown flushes spans still pending export, so give it a bounded window
		ctx, cancel := context.WithTimeout(context.Background(), managerCloseTimeout)
		keep(m.tracingManager.Shutdown(ctx))
		cancel()
	}

	if limiter, ok := m.rateLimiter.(stoppableLimiter); ok && limiter != nil {
		limiter.Stop()
	}

	if m.swaggerMiddleware != nil {
		keep(m.swaggerMiddleware.DisableFileWatcher())
	}

	return firstErr
}

// HTTPMetricsMiddleware HTTP 监控中间件
func (m *Manager) HTTPMetricsMiddleware() MiddlewareFunc {
	return observability.HTTPMetricsMiddleware(m.metricsManager)
}

// HTTPTracingMiddleware HTTP 链路追踪中间件
func (m *Manager) HTTPTracingMiddleware() MiddlewareFunc {
	return observability.Tracing(m.tracingManager)
}

// GRPCMetricsInterceptor gRPC 监控拦截器
func (m *Manager) GRPCMetricsInterceptor() GRPCInterceptor {
	return observability.GRPCMetricsInterceptor(m.metricsManager)
}

// GRPCTracingInterceptor gRPC 链路追踪拦截器
func (m *Manager) GRPCTracingInterceptor() GRPCInterceptor {
	return observability.GRPCTracingInterceptor(m.tracingManager)
}

// GRPCTracingStreamInterceptor gRPC 流式链路追踪拦截器
func (m *Manager) GRPCTracingStreamInterceptor() grpc.StreamServerInterceptor {
	return observability.GRPCTracingStreamInterceptor(m.tracingManager)
}

// GetTracingManager 获取链路追踪管理器（供 WSC 等非中间件链组件使用）
func (m *Manager) GetTracingManager() *TracingManager {
	return m.tracingManager
}

// GRPCStructTagValidatorInterceptor gRPC struct tag 参数校验拦截器
// 配合 protoc-go-inject-tag 在 pb 字段上注入的 `validate:"..."` 标签生效。
func (m *Manager) GRPCStructTagValidatorInterceptor() GRPCInterceptor {
	return i18n.StructTagValidatorUnaryInterceptor()
}

// GRPCStructTagValidatorStreamInterceptor gRPC 流式 struct tag 参数校验拦截器
func (m *Manager) GRPCStructTagValidatorStreamInterceptor() grpc.StreamServerInterceptor {
	return i18n.StructTagValidatorStreamInterceptor()
}

// GRPCGatewayStructTagValidatorMiddleware grpc-gateway HTTP 层 struct tag 参数校验中间件
// 用于本地 Handler 模式（RegisterXxxHandlerServer），HTTP 请求绕过 gRPC 拦截器链，
// 需要通过此中间件在 gateway 层拦截校验
func (m *Manager) GRPCGatewayStructTagValidatorMiddleware() runtime.Middleware {
	return i18n.StructTagValidatorGatewayMiddleware()
}

// CORSMiddleware CORS 中间件
func (m *Manager) CORSMiddleware() MiddlewareFunc {
	return MiddlewareFunc(security.CORSMiddleware(m.cfg.CORS))
}

// RecoveryMiddleware 恢复中间件
func (m *Manager) RecoveryMiddleware() MiddlewareFunc {
	return MiddlewareFunc(core.RecoveryMiddleware(m.cfg.Middleware.Recovery))
}

// RequestContextMiddlewareFunc 统一的请求上下文中间件
// 负责 trace_id、request_id 等的注入，使用 go-logger 的统一管理
func (m *Manager) RequestContextMiddlewareFunc() MiddlewareFunc {
	return MiddlewareFunc(core.RequestContextMiddleware())
}

// SCPMiddleware 安全中间件 - 从配置读取 CSP 策略
func (m *Manager) SCPMiddleware() MiddlewareFunc {
	return MiddlewareFunc(security.SCPMiddleware(m.cfg.Security.CSP))
}

// RateLimitMiddleware 限流中间件
func (m *Manager) RateLimitMiddleware() MiddlewareFunc {
	return MiddlewareFunc(resilience.RateLimitMiddlewareWithOptions(m.cfg.RateLimit, m.rateLimiter, m.dynamicRateLimit, m.metricsManager))
}

// AddPreRateLimitMiddleware 添加在限流之前执行的自定义中间件（如认证）。
// 认证中间件在此处执行后，UserID 会注入 context，限流中间件即可正确读取。
func (m *Manager) AddPreRateLimitMiddleware(mw ...HTTPMiddleware) {
	m.preRateLimitMiddlewares = append(m.preRateLimitMiddlewares, mw...)
}

// GRPCRateLimitUnaryInterceptor gRPC 一元限流拦截器（未启用限流时返回 nil）
func (m *Manager) GRPCRateLimitUnaryInterceptor() grpc.UnaryServerInterceptor {
	if !m.cfg.RateLimit.Enabled {
		return nil
	}
	return resilience.GRPCRateLimitUnaryInterceptor(m.cfg.RateLimit, m.rateLimiter)
}

// GRPCRateLimitStreamInterceptor gRPC 流式限流拦截器（未启用限流时返回 nil）
func (m *Manager) GRPCRateLimitStreamInterceptor() grpc.StreamServerInterceptor {
	if !m.cfg.RateLimit.Enabled {
		return nil
	}
	return resilience.GRPCRateLimitStreamInterceptor(m.cfg.RateLimit, m.rateLimiter)
}

// LoggingMiddleware HTTP日志中间件
func (m *Manager) LoggingMiddleware() MiddlewareFunc {
	return MiddlewareFunc(observability.LoggingMiddleware())
}

// SignatureMiddleware 签名验证中间件
func (m *Manager) SignatureMiddleware() MiddlewareFunc {
	return MiddlewareFunc(security.SignatureMiddlewareWithProvider(m.cfg.Middleware.Signature, m.dynamicSignature))
}

// SetDynamicSignatureProvider 设置动态签名提供器
func (m *Manager) SetDynamicSignatureProvider(provider DynamicSignatureProvider) {
	m.dynamicSignature = provider
}

// SetDynamicRateLimitProvider 设置动态限流提供器
func (m *Manager) SetDynamicRateLimitProvider(provider DynamicRateLimitProvider) {
	m.dynamicRateLimit = provider
}

// TimestampMiddleware 时间戳验证中间件
func (m *Manager) TimestampMiddleware() MiddlewareFunc {
	return MiddlewareFunc(security.TimestampMiddleware(m.cfg.Middleware.Signature))
}

// NonceMiddleware 防重放验证中间件
func (m *Manager) NonceMiddleware() MiddlewareFunc {
	return MiddlewareFunc(security.NonceMiddleware(m.cfg.Middleware.Signature))
}

// I18nMiddleware 国际化中间件
// 由配置驱动：启用时使用 NewManager 中创建的 i18nManager，未启用时直通
func (m *Manager) I18nMiddleware() MiddlewareFunc {
	if !m.cfg.Middleware.I18N.Enabled {
		return MiddlewareFunc(core.NoopMiddleware)
	}
	return i18n.I18nWithManager(m.i18nManager)
}

// GRPCUnaryI18nInterceptor 返回 gRPC 服务端 i18n 一元调用拦截器（未启用时返回 nil）
func (m *Manager) GRPCUnaryI18nInterceptor() grpc.UnaryServerInterceptor {
	if !m.cfg.Middleware.I18N.Enabled {
		return nil
	}
	return i18n.UnaryServerI18nInterceptor(m.i18nManager)
}

// GRPCStreamI18nInterceptor 返回 gRPC 服务端 i18n 流式调用拦截器（未启用时返回 nil）
func (m *Manager) GRPCStreamI18nInterceptor() grpc.StreamServerInterceptor {
	if !m.cfg.Middleware.I18N.Enabled {
		return nil
	}
	return i18n.StreamServerI18nInterceptor(m.i18nManager)
}

// BreakerMiddleware 熔断中间件
// 由配置驱动：启用时使用 NewManager 中创建的共享 breakerManager（便于指标采集），未启用时直通
func (m *Manager) BreakerMiddleware() MiddlewareFunc {
	if !m.cfg.Middleware.CircuitBreaker.Enabled {
		return MiddlewareFunc(core.NoopMiddleware)
	}
	return MiddlewareFunc(resilience.BreakerHTTPMiddleware(m.breakerManager))
}

// GRPCBreakerUnaryInterceptor gRPC 一元熔断拦截器（未启用熔断时返回 nil）
// 与 HTTP 熔断中间件共用 NewManager 中创建的同一个 breakerManager，
// 因此同一方法在 HTTP 与 gRPC 两侧共享状态与统计，指标采集也只有一份数据源
//
// [EN] Unary breaker interceptor, nil when the breaker is disabled. It shares the single
// breakerManager created in NewManager with the HTTP middleware, so state, statistics and
// metrics collection all come from one source
func (m *Manager) GRPCBreakerUnaryInterceptor() grpc.UnaryServerInterceptor {
	if !m.cfg.Middleware.CircuitBreaker.Enabled || m.breakerManager == nil {
		return nil
	}
	return resilience.UnaryServerBreakerInterceptor(m.breakerManager)
}

// GRPCBreakerStreamInterceptor gRPC 流式熔断拦截器（未启用熔断时返回 nil）
//
// [EN] Stream breaker interceptor, nil when the breaker is disabled
func (m *Manager) GRPCBreakerStreamInterceptor() grpc.StreamServerInterceptor {
	if !m.cfg.Middleware.CircuitBreaker.Enabled || m.breakerManager == nil {
		return nil
	}
	return resilience.StreamServerBreakerInterceptor(m.breakerManager)
}

// GetBreakerManager 获取熔断器管理器（供 server 包注入指标采集函数）
func (m *Manager) GetBreakerManager() *BreakerManager {
	return m.breakerManager
}

// ShedderMiddleware 自适应降载中间件
// 由配置驱动：启用时使用 NewManager 中创建的共享 shedderManager（便于指标采集），未启用时直通
//
// [EN] Adaptive load-shedding middleware, sharing the shedderManager created in NewManager
// so that metrics come from one source; a pass-through when shedding is disabled
func (m *Manager) ShedderMiddleware() MiddlewareFunc {
	if m.shedderManager == nil {
		return MiddlewareFunc(core.NoopMiddleware)
	}
	return MiddlewareFunc(resilience.ShedderHTTPMiddleware(m.shedderManager))
}

// GRPCShedderUnaryInterceptor gRPC 一元降载拦截器（未启用降载时返回 nil）
// 与 HTTP 降载中间件共用 NewManager 中创建的同一个 shedderManager，
// 因此 HTTP 与 gRPC 两侧的飞行请求数、响应时间滑窗合并统计——降载判定的是整个进程的负载
//
// [EN] Unary shedding interceptor, nil when shedding is disabled. It shares the single
// shedderManager created in NewManager with the HTTP middleware, so in-flight counts and the
// response-time window are pooled across both transports: shedding judges whole-process load
func (m *Manager) GRPCShedderUnaryInterceptor() grpc.UnaryServerInterceptor {
	if m.shedderManager == nil {
		return nil
	}
	return resilience.UnaryServerShedderInterceptor(m.shedderManager)
}

// GRPCShedderStreamInterceptor gRPC 流式降载拦截器（未启用降载时返回 nil）
//
// [EN] Stream shedding interceptor, nil when shedding is disabled
func (m *Manager) GRPCShedderStreamInterceptor() grpc.StreamServerInterceptor {
	if m.shedderManager == nil {
		return nil
	}
	return resilience.StreamServerShedderInterceptor(m.shedderManager)
}

// GetShedderManager 获取降载管理器（供 server 包注册指标采集器）
//
// [EN] Returns the shedding manager, for the server package to register its collector
func (m *Manager) GetShedderManager() *ShedderManager {
	return m.shedderManager
}

// MetricsHandler 返回监控指标处理器（使用 MetricsManager 的独立 registry）
func (m *Manager) MetricsHandler() http.Handler {
	if m.metricsManager == nil {
		return http.NotFoundHandler()
	}
	return m.metricsManager.Handler()
}

// GetMetricsManager 获取监控管理器（供 server 包注入扩展指标采集函数）
func (m *Manager) GetMetricsManager() *MetricsManager {
	return m.metricsManager
}

// SwaggerHandler 返回 Swagger 文档处理器
func (m *Manager) SwaggerHandler() http.Handler {
	if m.swaggerMiddleware == nil {
		return http.NotFoundHandler()
	}
	return m.swaggerMiddleware
}

// GetSwaggerPaths 获取 Swagger 路由路径
func (m *Manager) GetSwaggerPaths() []string {
	if m.swaggerMiddleware == nil || !m.cfg.Swagger.Enabled {
		return nil
	}
	return m.swaggerMiddleware.GetSwaggerPaths()
}

// GetMiddlewares 获取中间件链（完全基于配置驱动）
func (m *Manager) GetMiddlewares() []MiddlewareFunc {
	var middlewares []MiddlewareFunc

	// 1. Recovery 中间件（始终启用，最先执行）
	middlewares = append(middlewares, m.RecoveryMiddleware())

	// 2. 链路追踪中间件（必须在 RequestContext 之前）
	// 原因：先创建 OTel span，后续 RequestContextMiddleware 的 extractOrGenerateTraceID
	// 才能从 span context 中提取 OTel trace_id，确保日志、gRPC metadata、OTel 后端
	// 使用同一个 trace_id，实现全链路打通；若顺序颠倒，Logging 等中间件会记录
	// RequestContext 自行生成的随机 trace_id，与 OTel trace_id 不一致
	if m.cfg.Middleware.Tracing.Enabled && m.tracingManager != nil {
		middlewares = append(middlewares, m.HTTPTracingMiddleware())
	}

	// 3. Context 追踪中间件（始终启用）
	middlewares = append(middlewares, m.RequestContextMiddlewareFunc())

	// 4. 日志中间件（始终装上，开关由中间件在请求路径上判）
	//
	// 不在这里按 Enabled 决定装不装：装不装是建链时一次性的决定，而建链只在启动与
	// 「HTTP/Swagger 运行期配置变了」时发生（见 gateway.go 的 httpRuntimeChanged，
	// 它只比 HTTPServer 与 Swagger 两棵子树，不含 Middleware）
	// 于是启动时关着日志，之后在配置里打开根本不会生效 —— 这一层从来没被装上
	// 现在恒定装上，由 LoggingMiddleware 每请求读一次 Enabled，两个方向都能热切
	//
	// [EN] Enabled is deliberately not consulted here. Whether a layer is installed is decided
	// once while the chain is built, and the chain is only built at startup and when the HTTP or
	// Swagger runtime config changes (see httpRuntimeChanged in gateway.go, which compares only
	// the HTTPServer and Swagger subtrees and not Middleware). So logging that was off at
	// startup could never be switched on later: the layer was never installed. It is now always
	// installed and LoggingMiddleware reads Enabled once per request, so both directions work.
	middlewares = append(middlewares, m.LoggingMiddleware())

	// 5. 国际化中间件（根据配置）
	if m.cfg.Middleware.I18N.Enabled {
		middlewares = append(middlewares, m.I18nMiddleware())
	}

	// 6. 监控中间件（根据配置）
	if m.cfg.Monitoring.Metrics.Enabled && m.metricsManager != nil {
		middlewares = append(middlewares, m.HTTPMetricsMiddleware())
	}

	// 6.5. 限流前自定义中间件（如认证，需要在限流之前注入 UserID）
	for _, mw := range m.preRateLimitMiddlewares {
		middlewares = append(middlewares, MiddlewareFunc(mw))
	}

	// 7. 限流中间件（根据配置）
	if m.cfg.RateLimit.Enabled && m.rateLimiter != nil {
		middlewares = append(middlewares, m.RateLimitMiddleware())
	}

	// 7.5. 自适应降载中间件（根据配置，夹在限流与熔断之间）
	// 位置理由（与 server/grpc.go 的 gRPC 链完全一致，详见 docs/SHEDDER.md）：
	//  1. 必须在限流之后——限流是确定性的配额判定，被限流拒绝的请求根本不会触达 handler，
	//     放在降载之前可以让它们既不占用飞行计数、也不把「拒绝得极快」的响应时间
	//     掺进滑窗，否则最小平均 RT 会被压低，承载上限被低估，反过来加剧降载
	//  2. 必须在熔断之前（即熔断在降载内层）——降载拒绝返回 503/codes.Unavailable，
	//     而 Unavailable 正是熔断计入失败的码；若降载在熔断内层，熔断器会把降载的
	//     自我保护拒绝当成下游故障，形成「过载→降载拒绝→熔断打开」的正反馈，
	//     哪怕负载已经退潮也要等熔断超时才能恢复 顺序反过来后熔断器永远看不到降载拒绝
	//  3. 两者语义本就不同：降载回答「本进程现在还扛得住吗」，熔断回答「下游还活着吗」，
	//     前者是全局自保、应当更早拒绝，后者需要贴近 handler 才能观测到真实健康度
	if m.shedderManager != nil {
		middlewares = append(middlewares, m.ShedderMiddleware())
	}

	// 8. 熔断中间件（根据配置）
	if m.cfg.Middleware.CircuitBreaker.Enabled {
		middlewares = append(middlewares, m.BreakerMiddleware())
	}

	// 9. 安全中间件（根据配置）
	if m.cfg.Security.CSP.Enabled {
		middlewares = append(middlewares, m.SCPMiddleware())
	}

	// 10. CORS 中间件（根据配置）
	if m.cfg.CORS.Enabled {
		middlewares = append(middlewares, m.CORSMiddleware())
	}

	// 11. 签名验证中间件
	if m.cfg.Middleware.Signature.Enabled {
		middlewares = append(middlewares, m.TimestampMiddleware())
		middlewares = append(middlewares, m.NonceMiddleware())
		middlewares = append(middlewares, m.SignatureMiddleware())
	}

	return middlewares
}

// HTTPMiddleware 应用HTTP中间件链
func (m *Manager) HTTPMiddleware(handler http.Handler) http.Handler {
	middlewares := m.GetMiddlewares()
	return ApplyMiddlewares(handler, middlewares...)
}

// UnaryServerInterceptor 返回gRPC一元拦截器
func (m *Manager) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return observability.UnaryServerLoggingInterceptor()
}

// StreamServerInterceptor 返回gRPC流拦截器
func (m *Manager) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return observability.StreamServerLoggingInterceptor()
}

// ApplyMiddlewares 应用中间件链到处理器
func ApplyMiddlewares(handler http.Handler, middlewares ...MiddlewareFunc) http.Handler {
	// 倒序应用中间件
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] == nil {
			continue
		}
		handler = middlewares[i](handler)
	}
	return handler
}
