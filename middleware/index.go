/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-10-15 17:30:08
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-10-17 19:05:08
 * @FilePath: \go-rpc-gateway\middleware\index.go
 * @Description: middleware 根包再导出层 - 保持拆包前的全部公开 API 入口不变
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

// Package middleware 是网关中间件的统一入口
//
// 实现按职责拆分到 core / resilience / observability / security / i18n 五个子包，
// 本文件把子包的公开 API 原样再导出到根包，因此 middleware.XxxInterceptor 这类
// 既有调用方式无需任何改动；类型使用别名，方法集与类型身份完全等同于子包中的定义
//
// 每个符号的文档注释与实现放在一起，位于对应子包中，
// 例如 go doc github.com/kamalyes/go-rpc-gateway/middleware/observability Tracing
//
// [EN] Package middleware is the single entry point for the gateway middlewares.
// The implementation is split by responsibility into the core, resilience, observability,
// security and i18n subpackages; this file re-exports their public API verbatim into the
// root package, so existing call sites such as middleware.XxxInterceptor need no change.
// Types are re-exported as aliases, so method sets and type identity are exactly the
// definitions in the subpackages. Each symbol's documentation lives next to its
// implementation in the owning subpackage
package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/kamalyes/go-rpc-gateway/middleware/core"
	"github.com/kamalyes/go-rpc-gateway/middleware/i18n"
	"github.com/kamalyes/go-rpc-gateway/middleware/observability"
	"github.com/kamalyes/go-rpc-gateway/middleware/resilience"
	mwsecurity "github.com/kamalyes/go-rpc-gateway/middleware/security"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	gcmiddleware "github.com/kamalyes/go-rpc-gateway/config/middleware"
	"github.com/kamalyes/go-rpc-gateway/errors"
	goi18n "github.com/kamalyes/go-toolbox/pkg/i18n"
	"github.com/kamalyes/go-toolbox/pkg/sign"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
)

// ============================================================================
// core（核心层：中间件类型、请求上下文、响应包装、Recovery、路径规范化与动态提供器）
// [EN] Core layer: middleware types, request context, response writer, recovery, path normalizer and dynamic providers
// ============================================================================

// 类型别名：别名与子包中的定义是同一类型，完整保留其方法集
// [EN] Type aliases: an alias is the very same type as the subpackage definition and keeps its full method set
type (
	ContextBuilder           = core.ContextBuilder
	DynamicRateLimitProvider = core.DynamicRateLimitProvider
	DynamicRateLimitResult   = core.DynamicRateLimitResult
	DynamicSignatureProvider = core.DynamicSignatureProvider
	GRPCInterceptor          = core.GRPCInterceptor
	HTTPMiddleware           = core.HTTPMiddleware
	MiddlewareFunc           = core.MiddlewareFunc
	PathNormalizer           = core.PathNormalizer
	RateLimitDecision        = core.RateLimitDecision
	RequestCommonMeta        = core.RequestCommonMeta
	ResolvedSignature        = core.ResolvedSignature
	ResponseWriter           = core.ResponseWriter
	SignatureValidator       = core.SignatureValidator
)

// 函数再导出：薄包装，签名与子包中的定义逐字一致
// [EN] Re-exported functions: thin wrappers whose signatures match the subpackage definitions verbatim

func ChainFunc(middlewares ...MiddlewareFunc) MiddlewareFunc {
	return core.ChainFunc(middlewares...)
}

func ForwardOutgoingContext(r *http.Request) context.Context {
	return core.ForwardOutgoingContext(r)
}

func GetAcceptLanguageFromMeta(ctx context.Context) string {
	return core.GetAcceptLanguageFromMeta(ctx)
}

func GetAccessKey(ctx context.Context) string {
	return core.GetAccessKey(ctx)
}

func GetAgentLineID(ctx context.Context) string {
	return core.GetAgentLineID(ctx)
}

func GetAppID(ctx context.Context) string {
	return core.GetAppID(ctx)
}

func GetAppVersion(ctx context.Context) string {
	return core.GetAppVersion(ctx)
}

func GetAuthorization(ctx context.Context) string {
	return core.GetAuthorization(ctx)
}

func GetDeviceID(ctx context.Context) string {
	return core.GetDeviceID(ctx)
}

func GetDomain(ctx context.Context) string {
	return core.GetDomain(ctx)
}

func GetFamilyId(ctx context.Context) string {
	return core.GetFamilyId(ctx)
}

func GetForwardedHost(ctx context.Context) string {
	return core.GetForwardedHost(ctx)
}

func GetID(ctx context.Context) string {
	return core.GetID(ctx)
}

func GetIPAddress(ctx context.Context) string {
	return core.GetIPAddress(ctx)
}

func GetJti(ctx context.Context) string {
	return core.GetJti(ctx)
}

func GetNonce(ctx context.Context) string {
	return core.GetNonce(ctx)
}

func GetPlatformCode(ctx context.Context) string {
	return core.GetPlatformCode(ctx)
}

func GetPlatformID(ctx context.Context) string {
	return core.GetPlatformID(ctx)
}

func GetPushToken(ctx context.Context) string {
	return core.GetPushToken(ctx)
}

func GetRegionCode(ctx context.Context) string {
	return core.GetRegionCode(ctx)
}

func GetRegionID(ctx context.Context) string {
	return core.GetRegionID(ctx)
}

func GetRequestCommonMeta(ctx context.Context) *RequestCommonMeta {
	return core.GetRequestCommonMeta(ctx)
}

func GetRequestID(ctx context.Context) string {
	return core.GetRequestID(ctx)
}

func GetRoleCode(ctx context.Context) string {
	return core.GetRoleCode(ctx)
}

func GetSessionID(ctx context.Context) string {
	return core.GetSessionID(ctx)
}

func GetSignature(ctx context.Context) string {
	return core.GetSignature(ctx)
}

func GetTenantCode(ctx context.Context) string {
	return core.GetTenantCode(ctx)
}

func GetTenantID(ctx context.Context) string {
	return core.GetTenantID(ctx)
}

func GetTimestamp(ctx context.Context) string {
	return core.GetTimestamp(ctx)
}

func GetTimezone(ctx context.Context) string {
	return core.GetTimezone(ctx)
}

func GetToken(ctx context.Context) string {
	return core.GetToken(ctx)
}

func GetTraceID(ctx context.Context) string {
	return core.GetTraceID(ctx)
}

func GetUserAgent(ctx context.Context) string {
	return core.GetUserAgent(ctx)
}

func GetUserID(ctx context.Context) string {
	return core.GetUserID(ctx)
}

func GetUserName(ctx context.Context) string {
	return core.GetUserName(ctx)
}

func GetXNsID(ctx context.Context) string {
	return core.GetXNsID(ctx)
}

func NewContextBuilder(ctx context.Context) *ContextBuilder {
	return core.NewContextBuilder(ctx)
}

func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return core.NewResponseWriter(w)
}

func RecoveryMiddleware(cfg *gcmiddleware.Recovery) HTTPMiddleware {
	return core.RecoveryMiddleware(cfg)
}

func RequestContextMiddleware() HTTPMiddleware {
	return core.RequestContextMiddleware()
}

func StreamClientRequestContextInterceptor() grpc.StreamClientInterceptor {
	return core.StreamClientRequestContextInterceptor()
}

func StreamServerRequestContextInterceptor() grpc.StreamServerInterceptor {
	return core.StreamServerRequestContextInterceptor()
}

func UnaryClientRequestContextInterceptor() grpc.UnaryClientInterceptor {
	return core.UnaryClientRequestContextInterceptor()
}

func UnaryServerRequestContextInterceptor() grpc.UnaryServerInterceptor {
	return core.UnaryServerRequestContextInterceptor()
}

func WithAcceptLanguage(ctx context.Context, language string) context.Context {
	return core.WithAcceptLanguage(ctx, language)
}

func WithAccessKey(ctx context.Context, accessKey string) context.Context {
	return core.WithAccessKey(ctx, accessKey)
}

func WithAgentLineID(ctx context.Context, agentLineID string) context.Context {
	return core.WithAgentLineID(ctx, agentLineID)
}

func WithAppID(ctx context.Context, appID string) context.Context {
	return core.WithAppID(ctx, appID)
}

func WithAppVersion(ctx context.Context, appVersion string) context.Context {
	return core.WithAppVersion(ctx, appVersion)
}

func WithAuthorization(ctx context.Context, authorization string) context.Context {
	return core.WithAuthorization(ctx, authorization)
}

func WithDeviceID(ctx context.Context, deviceID string) context.Context {
	return core.WithDeviceID(ctx, deviceID)
}

func WithDomain(ctx context.Context, domain string) context.Context {
	return core.WithDomain(ctx, domain)
}

func WithFamilyId(ctx context.Context, familyId string) context.Context {
	return core.WithFamilyId(ctx, familyId)
}

func WithForwardedHost(ctx context.Context, host string) context.Context {
	return core.WithForwardedHost(ctx, host)
}

func WithID(ctx context.Context, id string) context.Context {
	return core.WithID(ctx, id)
}

func WithIPAddress(ctx context.Context, ipAddress string) context.Context {
	return core.WithIPAddress(ctx, ipAddress)
}

func WithJti(ctx context.Context, jti string) context.Context {
	return core.WithJti(ctx, jti)
}

func WithNonce(ctx context.Context, nonce string) context.Context {
	return core.WithNonce(ctx, nonce)
}

func WithPlatformCode(ctx context.Context, platformCode string) context.Context {
	return core.WithPlatformCode(ctx, platformCode)
}

func WithPlatformID(ctx context.Context, platformID string) context.Context {
	return core.WithPlatformID(ctx, platformID)
}

func WithPushToken(ctx context.Context, pushToken string) context.Context {
	return core.WithPushToken(ctx, pushToken)
}

func WithRegionCode(ctx context.Context, regionCode string) context.Context {
	return core.WithRegionCode(ctx, regionCode)
}

func WithRegionID(ctx context.Context, regionID string) context.Context {
	return core.WithRegionID(ctx, regionID)
}

func WithRequestCommonMeta(ctx context.Context, requestCommonMeta *RequestCommonMeta) context.Context {
	return core.WithRequestCommonMeta(ctx, requestCommonMeta)
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return core.WithRequestID(ctx, requestID)
}

func WithRoleCode(ctx context.Context, roleCode string) context.Context {
	return core.WithRoleCode(ctx, roleCode)
}

func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return core.WithSessionID(ctx, sessionID)
}

func WithSignature(ctx context.Context, signature string) context.Context {
	return core.WithSignature(ctx, signature)
}

func WithTenantCode(ctx context.Context, tenantCode string) context.Context {
	return core.WithTenantCode(ctx, tenantCode)
}

func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return core.WithTenantID(ctx, tenantID)
}

func WithTimestamp(ctx context.Context, timestamp string) context.Context {
	return core.WithTimestamp(ctx, timestamp)
}

func WithTimezone(ctx context.Context, timezone string) context.Context {
	return core.WithTimezone(ctx, timezone)
}

func WithToken(ctx context.Context, token string) context.Context {
	return core.WithToken(ctx, token)
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return core.WithTraceID(ctx, traceID)
}

func WithUserAgent(ctx context.Context, userAgent string) context.Context {
	return core.WithUserAgent(ctx, userAgent)
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return core.WithUserID(ctx, userID)
}

func WithUserName(ctx context.Context, userName string) context.Context {
	return core.WithUserName(ctx, userName)
}

func WithXNsID(ctx context.Context, xNsID string) context.Context {
	return core.WithXNsID(ctx, xNsID)
}

// ============================================================================
// resilience（韧性层：熔断、限流、自适应降载）
// [EN] Resilience layer: circuit breaker, rate limiting and adaptive load shedding
// ============================================================================

// 类型别名：别名与子包中的定义是同一类型，完整保留其方法集
// [EN] Type aliases: an alias is the very same type as the subpackage definition and keeps its full method set
type (
	Breaker              = resilience.Breaker
	BreakerHealthStatus  = resilience.BreakerHealthStatus
	BreakerManager       = resilience.BreakerManager
	BreakerSnapshot      = resilience.BreakerSnapshot
	BreakerState         = resilience.BreakerState
	FixedWindowLimiter   = resilience.FixedWindowLimiter
	RateLimiter          = resilience.RateLimiter
	Shedder              = resilience.Shedder
	ShedderManager       = resilience.ShedderManager
	ShedderSnapshot      = resilience.ShedderSnapshot
	SlidingWindowLimiter = resilience.SlidingWindowLimiter
	TokenBucketLimiter   = resilience.TokenBucketLimiter
)

// 常量再导出：常量带有子包中声明的类型
// [EN] Re-exported constants, carrying the type declared in the subpackage
const (
	BreakerClosed   = resilience.BreakerClosed
	BreakerHalfOpen = resilience.BreakerHalfOpen
	BreakerOpen     = resilience.BreakerOpen
)

// 函数再导出：薄包装，签名与子包中的定义逐字一致
// [EN] Re-exported functions: thin wrappers whose signatures match the subpackage definitions verbatim

func BreakerHTTPMiddleware(manager *BreakerManager) func(http.Handler) http.Handler {
	return resilience.BreakerHTTPMiddleware(manager)
}

func CPUUsage() (int64, bool) {
	return resilience.CPUUsage()
}

func CPUUsageSource() string {
	return resilience.CPUUsageSource()
}

func GRPCRateLimitStreamInterceptor(config *gcmiddleware.RateLimit, limiter RateLimiter) grpc.StreamServerInterceptor {
	return resilience.GRPCRateLimitStreamInterceptor(config, limiter)
}

func GRPCRateLimitUnaryInterceptor(config *gcmiddleware.RateLimit, limiter RateLimiter) grpc.UnaryServerInterceptor {
	return resilience.GRPCRateLimitUnaryInterceptor(config, limiter)
}

func IsBreakerFailureCode(code codes.Code) bool {
	return resilience.IsBreakerFailureCode(code)
}

func NewBreaker(failureThreshold, successThreshold, volumeThreshold int, timeout time.Duration) *Breaker {
	return resilience.NewBreaker(failureThreshold, successThreshold, volumeThreshold, timeout)
}

func NewBreakerManager(cfg *gcmiddleware.CircuitBreaker) *BreakerManager {
	return resilience.NewBreakerManager(cfg)
}

func NewFixedWindowLimiter(config *gcmiddleware.RateLimit) *FixedWindowLimiter {
	return resilience.NewFixedWindowLimiter(config)
}

func NewShedder(cfg *gcmiddleware.Shedder) *Shedder {
	return resilience.NewShedder(cfg)
}

func NewShedderManager(cfg *gcmiddleware.Shedder) *ShedderManager {
	return resilience.NewShedderManager(cfg)
}

func NewShedderWithCPUFunc(cfg *gcmiddleware.Shedder, cpuUsageFn func() (int64, bool)) *Shedder {
	return resilience.NewShedderWithCPUFunc(cfg, cpuUsageFn)
}

func NewSlidingWindowLimiter(config *gcmiddleware.RateLimit) *SlidingWindowLimiter {
	return resilience.NewSlidingWindowLimiter(config)
}

func NewTokenBucketLimiter(cfg *gcmiddleware.RateLimit) *TokenBucketLimiter {
	return resilience.NewTokenBucketLimiter(cfg)
}

func RateLimitMiddleware(config *gcmiddleware.RateLimit) HTTPMiddleware {
	return resilience.RateLimitMiddleware(config)
}

func RateLimitMiddlewareWithProvider(config *gcmiddleware.RateLimit, provider DynamicRateLimitProvider) HTTPMiddleware {
	return resilience.RateLimitMiddlewareWithProvider(config, provider)
}

func ShedderHTTPMiddleware(manager *ShedderManager) func(http.Handler) http.Handler {
	return resilience.ShedderHTTPMiddleware(manager)
}

func StreamServerBreakerInterceptor(manager *BreakerManager) grpc.StreamServerInterceptor {
	return resilience.StreamServerBreakerInterceptor(manager)
}

func StreamServerShedderInterceptor(manager *ShedderManager) grpc.StreamServerInterceptor {
	return resilience.StreamServerShedderInterceptor(manager)
}

func UnaryServerBreakerInterceptor(manager *BreakerManager) grpc.UnaryServerInterceptor {
	return resilience.UnaryServerBreakerInterceptor(manager)
}

func UnaryServerShedderInterceptor(manager *ShedderManager) grpc.UnaryServerInterceptor {
	return resilience.UnaryServerShedderInterceptor(manager)
}

// ============================================================================
// observability（可观测层：链路追踪、日志、访问日志、指标、pprof、健康检查）
// [EN] Observability layer: tracing, logging, access log, metrics, pprof and health checks
// ============================================================================

// 类型别名：别名与子包中的定义是同一类型，完整保留其方法集
// [EN] Type aliases: an alias is the very same type as the subpackage definition and keeps its full method set
type (
	AccessHeaders       = observability.AccessHeaders
	AccessLogCodec      = observability.AccessLogCodec
	AccessLogHandler    = observability.AccessLogHandler
	AccessLogKind       = observability.AccessLogKind
	AccessRecord        = observability.AccessRecord
	AccessText          = observability.AccessText
	BreakerStat         = observability.BreakerStat
	GzipAccessLogCodec  = observability.GzipAccessLogCodec
	HTTPMetrics         = observability.HTTPMetrics
	HealthCheckResult   = observability.HealthCheckResult
	HealthChecker       = observability.HealthChecker
	HealthManager       = observability.HealthManager
	HealthStatus        = observability.HealthStatus
	LogFields           = observability.LogFields
	MetricsManager      = observability.MetricsManager
	MySQLChecker        = observability.PostgreSQLChecker // 兼容旧名
	PlainAccessLogCodec = observability.PlainAccessLogCodec
	PostgreSQLChecker   = observability.PostgreSQLChecker
	PProfServer         = observability.PProfServer
	RedisChecker        = observability.RedisChecker
	RequestLogger       = observability.RequestLogger
	ShedderStat         = observability.ShedderStat
	TracingManager      = observability.TracingManager
)

// 常量再导出：常量带有子包中声明的类型
// [EN] Re-exported constants, carrying the type declared in the subpackage
const (
	AccessLogKindGRPCClient = observability.AccessLogKindGRPCClient
	AccessLogKindGRPCServer = observability.AccessLogKindGRPCServer
	AccessLogKindHTTP       = observability.AccessLogKindHTTP
)

// 函数再导出：薄包装，签名与子包中的定义逐字一致
// [EN] Re-exported functions: thin wrappers whose signatures match the subpackage definitions verbatim

func DispatchAccessLog(ctx context.Context, rec *AccessRecord) {
	observability.DispatchAccessLog(ctx, rec)
}

func ExemplarFromContext(ctx context.Context) prometheus.Labels {
	return observability.ExemplarFromContext(ctx)
}

func GRPCMetricsInterceptor(metricsManager *MetricsManager) GRPCInterceptor {
	return observability.GRPCMetricsInterceptor(metricsManager)
}

func GRPCTracingInterceptor(tracingManager *TracingManager) GRPCInterceptor {
	return observability.GRPCTracingInterceptor(tracingManager)
}

func GRPCTracingStreamInterceptor(tracingManager *TracingManager) grpc.StreamServerInterceptor {
	return observability.GRPCTracingStreamInterceptor(tracingManager)
}

func HTTPMetricsMiddleware(m *MetricsManager) MiddlewareFunc {
	return observability.HTTPMetricsMiddleware(m)
}

func HTTPTracingMiddleware(tracingManager *TracingManager) HTTPMiddleware {
	return observability.HTTPTracingMiddleware(tracingManager)
}

func HasAccessLogHandlers() bool {
	return observability.HasAccessLogHandlers()
}

func LogError(ctx context.Context, err error, message string, fields ...attribute.KeyValue) {
	observability.LogError(ctx, err, message, fields...)
}

func LogInfo(ctx context.Context, message string, fields ...attribute.KeyValue) {
	observability.LogInfo(ctx, message, fields...)
}

func LoggingMiddleware() HTTPMiddleware {
	return observability.LoggingMiddleware()
}

func NewHealthManager() *HealthManager {
	return observability.NewHealthManager()
}

func NewLogFields() *LogFields {
	return observability.NewLogFields()
}

func NewMetricsManager(cfg *gcmiddleware.Monitoring) *MetricsManager {
	return observability.NewMetricsManager(cfg)
}

func NewMySQLChecker(timeout time.Duration) *MySQLChecker {
	return observability.NewPostgreSQLChecker(timeout)
}

func NewPostgreSQLChecker(timeout time.Duration) *PostgreSQLChecker {
	return observability.NewPostgreSQLChecker(timeout)
}

func NewPostgreSQLCheckerWithDB(db *gorm.DB, timeout time.Duration) *PostgreSQLChecker {
	return observability.NewPostgreSQLCheckerWithDB(db, timeout)
}

func NewPProfServer(cfg *gcmiddleware.PProf) *PProfServer {
	return observability.NewPProfServer(cfg)
}

func NewRedisChecker(timeout time.Duration) *RedisChecker {
	return observability.NewRedisChecker(timeout)
}

func NewRedisCheckerWithClient(client redis.UniversalClient, timeout time.Duration) *RedisChecker {
	return observability.NewRedisCheckerWithClient(client, timeout)
}

func NewRequestLogger(ctx context.Context) *RequestLogger {
	return observability.NewRequestLogger(ctx)
}

func NewTracingManager(cfg *gcmiddleware.Tracing) (*TracingManager, error) {
	return observability.NewTracingManager(cfg)
}

func RegisterAccessLog(h AccessLogHandler) (unregister func()) {
	return observability.RegisterAccessLog(h)
}

func ResetProtojsonOptions() {
	observability.ResetProtojsonOptions()
}

func SetAccessLogCodec(codec AccessLogCodec) (restore func()) {
	return observability.SetAccessLogCodec(codec)
}

func StartPProfServer(cfg *gcmiddleware.PProf) error {
	return observability.StartPProfServer(cfg)
}

func StreamClientLoggingInterceptor(serviceName string) grpc.StreamClientInterceptor {
	return observability.StreamClientLoggingInterceptor(serviceName)
}

func StreamServerLoggingInterceptor() grpc.StreamServerInterceptor {
	return observability.StreamServerLoggingInterceptor()
}

func Tracing(manager *TracingManager) MiddlewareFunc {
	return observability.Tracing(manager)
}

func UnaryClientLoggingInterceptor(serviceName string) grpc.UnaryClientInterceptor {
	return observability.UnaryClientLoggingInterceptor(serviceName)
}

func UnaryServerLoggingInterceptor() grpc.UnaryServerInterceptor {
	return observability.UnaryServerLoggingInterceptor()
}

// ============================================================================
// security（安全层：安全头与 CORS、签名、Nonce、白名单、时间戳、作用域读取）
// [EN] Security layer: security headers and CORS, signature, nonce, whitelist, timestamp and scope reader
// ============================================================================

// 类型别名：别名与子包中的定义是同一类型，完整保留其方法集
// [EN] Type aliases: an alias is the very same type as the subpackage definition and keeps its full method set
type (
	CIDRRule           = mwsecurity.CIDRRule
	CommonRules        = mwsecurity.CommonRules
	ContextScopeReader = mwsecurity.ContextScopeReader
	CustomRule         = mwsecurity.CustomRule
	ExactPathRule      = mwsecurity.ExactPathRule
	HMACValidator      = mwsecurity.HMACValidator
	IPRule             = mwsecurity.IPRule
	IPWhitelistRule    = mwsecurity.IPWhitelistRule
	MethodRule         = mwsecurity.MethodRule
	PathGlobRule       = mwsecurity.PathGlobRule
	PathPrefixRule     = mwsecurity.PathPrefixRule
	PathSuffixRule     = mwsecurity.PathSuffixRule
	RSAValidator       = mwsecurity.RSAValidator
	RegexRule          = mwsecurity.RegexRule
	RuleBuilder        = mwsecurity.RuleBuilder
	WhitelistManager   = mwsecurity.WhitelistManager
	WhitelistRule      = mwsecurity.WhitelistRule
)

// 函数再导出：薄包装，签名与子包中的定义逐字一致
// [EN] Re-exported functions: thin wrappers whose signatures match the subpackage definitions verbatim

func CORSMiddleware(corsConfig *gcmiddleware.Cors) HTTPMiddleware {
	return mwsecurity.CORSMiddleware(corsConfig)
}

func CSRFProtectionMiddleware(enabled bool) HTTPMiddleware {
	return mwsecurity.CSRFProtectionMiddleware(enabled)
}

func CSRFTokenHandler(w http.ResponseWriter, r *http.Request) {
	mwsecurity.CSRFTokenHandler(w, r)
}

func DefaultWhitelistManager() *WhitelistManager {
	return mwsecurity.DefaultWhitelistManager()
}

func IPWhitelistMiddleware(allowedIPs []string) HTTPMiddleware {
	return mwsecurity.IPWhitelistMiddleware(allowedIPs)
}

func IsWhitelisted(method, path string) bool {
	return mwsecurity.IsWhitelisted(method, path)
}

func IsWhitelistedWithIP(method, path, clientIP string) bool {
	return mwsecurity.IsWhitelistedWithIP(method, path, clientIP)
}

func NewHMACValidator(algorithm sign.HashCryptoFunc) (*HMACValidator, error) {
	return mwsecurity.NewHMACValidator(algorithm)
}

func NewRSAValidator(publicKeyPEM []byte) (*RSAValidator, error) {
	return mwsecurity.NewRSAValidator(publicKeyPEM)
}

func NewRuleBuilder(manager *WhitelistManager) *RuleBuilder {
	return mwsecurity.NewRuleBuilder(manager)
}

func NewWhitelistManager() *WhitelistManager {
	return mwsecurity.NewWhitelistManager()
}

func NonceMiddleware(config *gcmiddleware.Signature) HTTPMiddleware {
	return mwsecurity.NonceMiddleware(config)
}

func PathProtectionMiddleware(pathPrefix string, cfg *gcmiddleware.ServiceProtection) HTTPMiddleware {
	return mwsecurity.PathProtectionMiddleware(pathPrefix, cfg)
}

func RegisterWhitelistRule(rule WhitelistRule) {
	mwsecurity.RegisterWhitelistRule(rule)
}

func SCPMiddleware(cspConfig *gcmiddleware.CSP) HTTPMiddleware {
	return mwsecurity.SCPMiddleware(cspConfig)
}

func SignatureMiddleware(config *gcmiddleware.Signature) HTTPMiddleware {
	return mwsecurity.SignatureMiddleware(config)
}

func SignatureMiddlewareWithProvider(config *gcmiddleware.Signature, provider DynamicSignatureProvider) HTTPMiddleware {
	return mwsecurity.SignatureMiddlewareWithProvider(config, provider)
}

func SignatureMiddlewareWithValidator(config *gcmiddleware.Signature, validator SignatureValidator) HTTPMiddleware {
	return mwsecurity.SignatureMiddlewareWithValidator(config, validator)
}

func TimestampMiddleware(config *gcmiddleware.Signature) HTTPMiddleware {
	return mwsecurity.TimestampMiddleware(config)
}

// ============================================================================
// i18n（国际化层：多语言消息与结构体标签校验）
// [EN] Internationalization layer: multilingual messages and struct tag validation
// ============================================================================

// 类型别名：别名与子包中的定义是同一类型，完整保留其方法集
// [EN] Type aliases: an alias is the very same type as the subpackage definition and keeps its full method set
type (
	I18nManager    = i18n.I18nManager
	LocalizedError = i18n.LocalizedError
)

// 函数再导出：薄包装，签名与子包中的定义逐字一致
// [EN] Re-exported functions: thin wrappers whose signatures match the subpackage definitions verbatim

func GetLanguage(ctx context.Context) string {
	return i18n.GetLanguage(ctx)
}

func GetMsgByKey(ctx context.Context, key string) string {
	return i18n.GetMsgByKey(ctx, key)
}

func GetMsgWithMap(ctx context.Context, key string, maps map[string]any) string {
	return i18n.GetMsgWithMap(ctx, key, maps)
}

func I18n() MiddlewareFunc {
	return i18n.I18n()
}

func I18nFromContext(ctx context.Context) *goi18n.Context {
	return i18n.I18nFromContext(ctx)
}

func I18nWithManager(manager *goi18n.Manager, detect ...*gcmiddleware.I18N) MiddlewareFunc {
	return i18n.I18nWithManager(manager, detect...)
}

func NewFileMessageLoader(localesPath string) *goi18n.FileLoader {
	return i18n.NewFileMessageLoader(localesPath)
}

func NewI18nManager(config *gcmiddleware.I18N) (*goi18n.Manager, error) {
	return i18n.NewI18nManager(config)
}

func NewJSONMessageLoader(messagesJSON string) (*goi18n.JSONLoader, error) {
	return i18n.NewJSONMessageLoader(messagesJSON)
}

func NewLocalizedAppError(ctx context.Context, code errors.ErrorCode, key string, args ...any) *errors.AppError {
	return i18n.NewLocalizedAppError(ctx, code, key, args...)
}

func NewLocalizedAppErrorWithMap(ctx context.Context, code errors.ErrorCode, key string, templateData map[string]any) *errors.AppError {
	return i18n.NewLocalizedAppErrorWithMap(ctx, code, key, templateData)
}

func NewLocalizedError(ctx context.Context, key string, args ...any) *LocalizedError {
	return i18n.NewLocalizedError(ctx, key, args...)
}

func RegisterGatewayMessageType(method string, pathPattern string, newMsg func() any) {
	i18n.RegisterGatewayMessageType(method, pathPattern, newMsg)
}

func SetLanguage(ctx context.Context, language string) context.Context {
	return i18n.SetLanguage(ctx, language)
}

func StreamServerI18nInterceptor(manager *goi18n.Manager) grpc.StreamServerInterceptor {
	return i18n.StreamServerI18nInterceptor(manager)
}

func StructTagValidatorGatewayMiddleware() runtime.Middleware {
	return i18n.StructTagValidatorGatewayMiddleware()
}

func StructTagValidatorStreamInterceptor() grpc.StreamServerInterceptor {
	return i18n.StructTagValidatorStreamInterceptor()
}

func StructTagValidatorUnaryInterceptor() GRPCInterceptor {
	return i18n.StructTagValidatorUnaryInterceptor()
}

func T(ctx context.Context, key string, args ...any) string {
	return i18n.T(ctx, key, args...)
}

func TWithMap(ctx context.Context, key string, templateData map[string]any) string {
	return i18n.TWithMap(ctx, key, templateData)
}

func UnaryServerI18nInterceptor(manager *goi18n.Manager) grpc.UnaryServerInterceptor {
	return i18n.UnaryServerI18nInterceptor(manager)
}
