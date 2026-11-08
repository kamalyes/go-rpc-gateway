/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-11
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-23 11:57:37
 * @FilePath: \go-rpc-gateway\middleware\observability\logging.go
 * @Description: 统一日志中间件 - 支持 HTTP 和 gRPC
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package observability

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gwconfig "github.com/kamalyes/go-rpc-gateway/config/core"
	gcmiddleware "github.com/kamalyes/go-rpc-gateway/config/middleware"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware/core"
	"github.com/kamalyes/go-toolbox/pkg/desensitize"
	"github.com/kamalyes/go-toolbox/pkg/matcher"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// 预计算日志消息常量，避免每次请求字符串拼接
var (
	logMsgHTTPRequestInfo  = "🚀 " + constants.LogMsgHTTPRequest
	logMsgHTTPRequestError = "❌ " + constants.LogMsgHTTPRequest
	logMsgHTTPRequestWarn  = "⚠️ " + constants.LogMsgHTTPRequest
	logMsgHTTPRequestOK    = "✅ " + constants.LogMsgHTTPRequest
	logMsgHTTPRequestSkip  = "⚠️ " + constants.LogMsgHTTPRequestSkip
)

// protojsonOptionsCache 缓存 protobuf JSON 序列化选项，避免每次请求创建新对象
// 配置热重载时通过 ResetProtojsonOptions 重置
var protojsonOptionsCache atomic.Pointer[protojson.MarshalOptions]

// ResetProtojsonOptions 重置缓存的 protojson 选项（配置热重载时调用）
func ResetProtojsonOptions() {
	protojsonOptionsCache.Store(nil)
}

// getProtojsonOptions 从全局配置 Gateway.JSON 获取 protobuf JSON 序列化选项
// 与 API 响应的序列化配置保持一致，支持配置文件修改和热重载
// 注意：DiscardUnknown 属于反序列化选项，序列化时不适用，故不传入
func getProtojsonOptions() protojson.MarshalOptions {
	if cached := protojsonOptionsCache.Load(); cached != nil {
		return *cached
	}

	var opts protojson.MarshalOptions
	cfg := global.GetGatewayConfig()
	if cfg != nil && cfg.JSON != nil {
		j := cfg.JSON
		opts = protojson.MarshalOptions{
			UseProtoNames:   j.UseProtoNames,
			EmitUnpopulated: j.EmitUnpopulated,
			Multiline:       false,
		}
	} else {
		// 回退：使用 go-config 的默认 JSON 配置
		d := gwconfig.DefaultJSON()
		opts = protojson.MarshalOptions{
			UseProtoNames:   d.UseProtoNames,
			EmitUnpopulated: d.EmitUnpopulated,
			Multiline:       false,
		}
	}

	protojsonOptionsCache.Store(&opts)
	return opts
}

// RequestLogger 统一的请求日志记录器
type RequestLogger struct {
	config *gcmiddleware.Logging
	ctx    context.Context
}

// NewRequestLogger 创建请求日志记录器
func NewRequestLogger(ctx context.Context) *RequestLogger {
	config := getLoggingConfig()
	return &RequestLogger{
		config: config,
		ctx:    ctx,
	}
}

// LogFields 日志字段构建器
type LogFields struct {
	fields []any
}

// NewLogFields 创建日志字段构建器
func NewLogFields() *LogFields {
	return &LogFields{fields: make([]any, 0, 32)}
}

// isBlank 判断字符串是否只由空白构成
//
// 逐字节只认 ASCII 的空格、制表、换行、回车，与 go-argus 的 IsEmptyValue 对字符串的
// 判定逐项等价 —— 刻意不用 strings.TrimSpace：后者按 unicode.IsSpace 判，
// 会把不换行空格 U+00A0、全角空格 U+3000、行分隔符 U+2028 以及 \v \f 也算空白，
// 换过去会让这些值从日志里消失 差分测试 TestIsBlankMatchesIfEmpty 锁住这个等价关系
//
// 改造前这里调 mathx.IfEmpty(value, "")，它走 reflect.ValueOf(val) ——
// 每次调用一次反射加一次装箱，实测 27 ns 与 1 次分配
// 访问日志每条约 16 个字段，也就是每请求 16 次无谓的反射
//
// [EN] Reports whether a string consists only of whitespace
// Only ASCII space, tab, newline and carriage return count, byte by byte, matching
// go-argus's IsEmptyValue for strings exactly
// strings.TrimSpace is deliberately avoided: it judges by unicode.IsSpace and would
// also treat U+00A0, U+3000, U+2028, \v and \f as blank, which would silently drop
// such values from the logs; TestIsBlankMatchesIfEmpty locks the equivalence down
// This replaces mathx.IfEmpty(value, ""), which went through reflect.ValueOf on every
// call for one reflection plus one boxing allocation, measured at 27 ns, repeated for
// roughly fourteen fields per access-log line
func isBlank(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' {
			return false
		}
	}
	return true
}

// Add 添加字符串字段，值为空或只含空白时跳过
func (lf *LogFields) Add(key, value string) *LogFields {
	if key == "" || isBlank(value) {
		return lf
	}
	lf.fields = append(lf.fields, key, value)
	return lf
}

// AddValue 添加任意类型字段
func (lf *LogFields) AddValue(key string, value any) *LogFields {
	if key == "" || value == nil {
		return lf
	}
	lf.fields = append(lf.fields, key, value)
	return lf
}

// AddRequestContext 添加请求上下文信息
func (lf *LogFields) AddRequestContext(ctx context.Context) *LogFields {
	requestCommonMeta := core.GetRequestCommonMeta(ctx)

	return lf.
		Add(constants.LogFieldTraceID, requestCommonMeta.TraceID).
		Add(constants.LogFieldRequestID, requestCommonMeta.RequestID).
		Add(constants.LogFieldAuthorization, requestCommonMeta.Authorization).
		Add(constants.LogFieldID, requestCommonMeta.ID).
		Add(constants.LogFieldUserID, requestCommonMeta.UserID).
		Add(constants.LogFieldDomain, requestCommonMeta.Domain).
		Add(constants.LogFieldRoleCode, requestCommonMeta.RoleCode).
		Add(constants.LogFieldTenantID, requestCommonMeta.TenantID).
		Add(constants.LogFieldTenantCode, requestCommonMeta.TenantCode).
		Add(constants.LogFieldSessionID, requestCommonMeta.SessionID).
		Add(constants.LogFieldTimezone, requestCommonMeta.Timezone).
		Add(constants.LogFieldAppID, requestCommonMeta.AppID).
		Add(constants.LogFieldDeviceID, requestCommonMeta.DeviceID).
		Add(constants.LogFieldAppVersion, requestCommonMeta.AppVersion).
		Add(constants.LogFieldPlatformID, requestCommonMeta.PlatformID).
		Add(constants.LogFieldPlatformCode, requestCommonMeta.PlatformCode).
		Add(constants.LogFieldRegionID, requestCommonMeta.RegionID).
		Add(constants.LogFieldRegionCode, requestCommonMeta.RegionCode).
		Add(constants.LogFieldIPAddress, requestCommonMeta.IPAddress).
		Add(constants.LogFieldXNsID, requestCommonMeta.XNsID)
}

// AddSlow 添加慢请求标记 🐌
func (lf *LogFields) AddSlow(duration, threshold time.Duration) *LogFields {
	if duration > threshold {
		return lf.AddValue(constants.LogFieldSlowRequest, true)
	}
	return lf
}

// splitFullMethod 将 gRPC FullMethod（"/pkg.ServiceName/MethodName"）拆分为 service 与 method
func splitFullMethod(fullMethod string) (service, method string) {
	// 去掉前导 '/'
	fullMethod = strings.TrimPrefix(fullMethod, "/")
	if i := strings.LastIndex(fullMethod, "/"); i >= 0 {
		return fullMethod[:i], fullMethod[i+1:]
	}
	return "", fullMethod
}

// AddGRPCMeta 添加 gRPC 调用元数据字段（与 go-grpc-middleware/v2 logging 输出对齐）
// component: "server" 或 "client"；fullMethod: gRPC FullMethod；methodType: "unary" 或 "stream"
// start: 调用开始时间；peerAddr: 对端地址（空则忽略）
func (lf *LogFields) AddGRPCMeta(ctx context.Context, component, fullMethod, methodType string, start time.Time, peerAddr string) *LogFields {
	service, method := splitFullMethod(fullMethod)
	lf.AddValue(constants.LogFieldGRPCComponent, component).
		Add(constants.LogFieldGRPCFullMethod, fullMethod).
		Add(constants.LogFieldGRPCService, service).
		Add(constants.LogFieldGRPCMethod, method).
		Add(constants.LogFieldGRPCMethodType, methodType).
		Add(constants.LogFieldGRPCStartTime, start.Format(time.RFC3339Nano)).
		Add(constants.LogFieldProtocol, "grpc")
	if deadline, ok := ctx.Deadline(); ok {
		lf.Add(constants.LogFieldGRPCRequestDeadline, deadline.Format(time.RFC3339Nano))
	}
	if peerAddr != "" {
		lf.Add(constants.LogFieldPeerAddress, peerAddr)
	}
	return lf
}

// peerFromContext 从上下文提取对端地址
func peerFromContext(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		return p.Addr.String()
	}
	return ""
}

// Build 构建字段列表
func (lf *LogFields) Build() []any {
	return lf.fields
}

// Log 记录日志
func (rl *RequestLogger) Log(level string, message string, fields *LogFields) {
	if global.LOGGER == nil {
		return
	}

	fieldList := fields.Build()

	switch level {
	case "info":
		global.LOGGER.InfoContextKV(rl.ctx, message, fieldList...)
	case "warn":
		global.LOGGER.WarnContextKV(rl.ctx, message, fieldList...)
	case "error":
		global.LOGGER.ErrorContextKV(rl.ctx, message, fieldList...)
	}
}

// getLoggingConfig 获取日志配置
func getLoggingConfig() *gcmiddleware.Logging {
	if cfg := global.GetGatewayConfig(); cfg != nil &&
		cfg.Middleware != nil &&
		cfg.Middleware.Logging != nil {
		return cfg.Middleware.Logging
	}
	return gcmiddleware.DefaultLogging()
}

// hasPrefixFold 前缀大小写不敏感匹配（零分配，替代 strings.ToLower+HasPrefix）
func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// isLoggableContentType 检查 Content-Type 是否可记录
func isLoggableContentType(config *gcmiddleware.Logging, contentType string) bool {
	if contentType == "" {
		return true
	}

	for _, prefix := range config.LoggableContentTypes {
		if hasPrefixFold(contentType, prefix) {
			return true
		}
	}
	return false
}

// isSkipPath 检查是否为跳过路径
// skip 项支持 glob 通配符（* 可匹配 /，? 匹配单字符），无通配符时退化为精确匹配
// 示例："/v1/audit-logs*" 覆盖 "/v1/audit-logs" 与 "/v1/audit-logs/{id}"
func isSkipPath(config *gcmiddleware.Logging, path string) bool {
	for _, skip := range config.SkipPaths {
		if matcher.MatchPathGlob(path, skip) {
			return true
		}
	}
	return false
}

// ============================================================================
// HTTP 日志中间件
// ============================================================================

// fallbackMasker global.DATAMASKER 为 nil 时用的兜底脱敏器
// 只在第一次用到时建一次
//
// [EN] The masker used when global.DATAMASKER is nil, built once on first use
var (
	fallbackMaskerOnce sync.Once
	fallbackMasker     *desensitize.DataMasker
)

// activeMasker 取可用的脱敏器，全局未初始化时退回默认配置的实例
//
// global.DATAMASKER 只由网关的完整初始化链赋值，而 LoggingMiddleware 是导出的、
// 可以单独使用 改造前这里直接 masker.Mask(...)，拿到 nil 就是空指针崩溃 ——
// 崩在日志路径上，被最外层 Recovery 兜成 500，于是每个带 body 的请求都变成 500
// 且一行日志都没有，现场只看得到 500
//
// 日志不该把请求打挂，所以这里兜底而不是报错
//
// [EN] Returns a usable masker, falling back to a default-configured one when the
// global is unset
// global.DATAMASKER is only assigned by the gateway's full init chain while
// LoggingMiddleware is exported and usable on its own; calling Mask on the nil global
// panicked inside the logging path, which the outermost Recovery turned into a 500, so
// every request with a body became a 500 with no log line to explain it
// Logging must not take a request down, hence a fallback rather than an error
func activeMasker() *desensitize.DataMasker {
	if m := global.DATAMASKER; m != nil {
		return m
	}
	fallbackMaskerOnce.Do(func() {
		fallbackMasker = desensitize.NewMasker()
	})
	return fallbackMasker
}

// prefixedBody 把「已读进内存的前缀」与「尚未读取的剩余」拼成一个 ReadCloser
// Close 转交给原始 body，否则连接不会被正确释放
//
// 自己实现而不是 io.MultiReader(bytes.NewReader(prefix), rest)：后者要分配
// bytes.Reader、multiReader 结构体、以及它内部那个 reader 切片，共三次；
// 这里只有 prefixedBody 自己一次 alloc profile 里那三项曾合计占链路前三层分配的 10%
//
// 也不用 io.NopCloser 包一层：那样原始 body 永远不会被 Close，
// 对 HTTP/1.1 的 keep-alive 连接会泄漏
//
// [EN] prefixedBody joins the in-memory prefix with the unread remainder as one ReadCloser
// Hand-written rather than io.MultiReader(bytes.NewReader(prefix), rest), which allocates
// a bytes.Reader, a multiReader and its internal reader slice, three in all, against one
// here; those three together accounted for 10% of the first three layers' allocations
// io.NopCloser is also avoided, since it would leave the original body unclosed and leak
// keep-alive connections on HTTP/1.1
type prefixedBody struct {
	prefix []byte
	off    int
	rest   io.ReadCloser
}

func (b *prefixedBody) Read(p []byte) (int, error) {
	if b.off < len(b.prefix) {
		n := copy(p, b.prefix[b.off:])
		b.off += n
		return n, nil
	}
	if b.rest == nil {
		return 0, io.EOF
	}
	return b.rest.Read(p)
}

func (b *prefixedBody) Close() error {
	if b.rest == nil {
		return nil
	}
	return b.rest.Close()
}

// LoggingMiddleware HTTP 日志中间件
func LoggingMiddleware() core.HTTPMiddleware {
	config := getLoggingConfig()
	if !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := r.Context()
			// 每次请求读取一次配置，支持热重载且避免后续重复读取
			reqConfig := getLoggingConfig()

			// 跳过路径检查
			if isSkipPath(reqConfig, r.URL.Path) {
				wrapped := core.NewResponseWriter(w)
				next.ServeHTTP(wrapped, r)
				if wrapped.StatusCode() >= 400 {
					logHTTPError(ctx, r, wrapped, time.Since(start))
				}
				wrapped.Release()
				return
			}

			// 捕获请求体：运行时日志开启 或 存在访问日志钩子 时读取，避免访问日志 body 受运行时日志开关影响而丢失
			//
			// 只读日志真正用得到的前 MaxBodySize 字节，剩余部分不进内存直通 handler
			//
			// 改造前是 io.ReadAll(r.Body) 无界读，再整段塞进 bytes.Buffer 交给 handler，
			// 而日志侧 masker.Mask 本来就只取前 MaxBodySize 字节（默认 2048）
			// 也就是说为了记 2 KB，一个 50 MB 的上传要分配 50 MB 捕获加 50 MB 缓冲，
			// 其中 99.99% 立刻被丢弃 —— 这既是吞吐开销，也是一条很容易触发的内存放大
			//
			// io.MultiReader 把已读的前缀与未读的剩余拼回去，handler 看到的仍是完整请求体；
			// 日志输出与改造前逐字节相同，因为 Mask 在同一个位置截断
			//
			// [EN] Only the first MaxBodySize bytes the log can actually use are read, and the
			// remainder streams to the handler without entering memory
			// This used to be an unbounded io.ReadAll plus a full bytes.Buffer copy, while
			// masker.Mask keeps just the first MaxBodySize bytes (2048 by default): logging
			// 2 KB of a 50 MB upload allocated 50 MB twice and discarded 99.99% of it, which
			// is both throughput cost and an easily triggered memory amplification
			// io.MultiReader rejoins the read prefix with the untouched remainder so the
			// handler still sees the whole body, and the log output is byte-for-byte what it
			// was, because Mask truncates at the same point
			hasAccessLog := HasAccessLogHandlers()
			needCaptureBody := reqConfig.EnableRequest || hasAccessLog
			captureLimit := reqConfig.MaxBodySize
			if captureLimit <= 0 {
				captureLimit = core.DefaultBodyCaptureLimit
			}
			var reqBody []byte
			if needCaptureBody && r.Body != nil {
				original := r.Body
				var err error
				reqBody, err = io.ReadAll(io.LimitReader(original, int64(captureLimit)))
				if err != nil && global.LOGGER != nil {
					global.LOGGER.ErrorContextKV(ctx, "❌ Failed to read request body",
						"path", r.URL.Path,
						"method", r.Method,
						"error", err)
				}
				r.Body = &prefixedBody{prefix: reqBody, rest: original}
			}

			// 包装响应：运行时日志开启 或 存在访问日志钩子 时启用响应体捕获
			wrapped := core.NewResponseWriter(w)
			if reqConfig.EnableResponse || hasAccessLog {
				wrapped.EnableBodyCaptureLimit(captureLimit)
			}
			defer wrapped.Release()

			// 执行请求
			next.ServeHTTP(wrapped, r)

			// 记录日志
			logHTTPRequest(ctx, r, wrapped, start, time.Since(start), reqConfig, reqBody, hasAccessLog)
		})
	}
}

// logHTTPRequest 记录 HTTP 请求
func logHTTPRequest(ctx context.Context, r *http.Request, rw *core.ResponseWriter, start time.Time, duration time.Duration, config *gcmiddleware.Logging, reqBody []byte, hasAccessLog bool) {
	logger := NewRequestLogger(ctx)
	masker := activeMasker()

	fields := NewLogFields().
		Add(constants.LogFieldMethod, r.Method).
		Add(constants.LogFieldPath, r.URL.Path).
		AddValue(constants.LogFieldStatus, rw.StatusCode()).
		AddValue(constants.LogFieldBytes, rw.BytesWritten()).
		AddValue(constants.LogFieldDuration, duration.Milliseconds()).
		Add(constants.LogFieldIP, core.GetRequestCommonMeta(ctx).IPAddress).
		Add(constants.LogFieldUserAgent, r.Header.Get(constants.HeaderUserAgent)).
		AddSlow(duration, time.Duration(config.SlowHTTPThreshold)*time.Millisecond).
		AddRequestContext(ctx)

	// 请求参数
	if config.EnableRequest && r.URL.RawQuery != "" {
		fields.Add(constants.LogFieldQuery, r.URL.RawQuery)
	}

	// 请求体
	if len(reqBody) > 0 && isLoggableContentType(config, r.Header.Get(constants.HeaderContentType)) {
		fields.Add(constants.LogFieldRequest, masker.Mask(reqBody))
	}

	// 响应体
	if respBody := rw.GetBody(); len(respBody) > 0 && isLoggableContentType(config, rw.Header().Get(constants.HeaderContentType)) {
		fields.Add(constants.LogFieldResponse, masker.Mask(respBody))
	}

	level := constants.LogLevelInfo
	message := logMsgHTTPRequestInfo
	if rw.StatusCode() >= 500 {
		level = constants.LogLevelError
		message = logMsgHTTPRequestError
	} else if rw.StatusCode() >= 400 {
		level = constants.LogLevelWarn
		message = logMsgHTTPRequestWarn
	} else {
		message = logMsgHTTPRequestOK
	}

	logger.Log(level, message, fields)

	// 派发访问日志钩子
	if hasAccessLog {
		captureAccessLog(ctx, r, rw, start, duration, reqBody, rw.GetBody(), time.Duration(config.SlowHTTPThreshold)*time.Millisecond)
	}
}

// logHTTPError 记录跳过路径的错误 🚫
func logHTTPError(ctx context.Context, r *http.Request, rw *core.ResponseWriter, duration time.Duration) {
	logger := NewRequestLogger(ctx)
	fields := NewLogFields().
		Add(constants.LogFieldPath, r.URL.Path).
		AddValue(constants.LogFieldStatus, rw.StatusCode()).
		AddValue(constants.LogFieldDuration, duration.Milliseconds()).
		AddRequestContext(ctx)

	logger.Log(constants.LogLevelWarn, logMsgHTTPRequestSkip, fields)
}

// ============================================================================
// gRPC 日志拦截器
// ============================================================================

// UnaryServerLoggingInterceptor gRPC 一元调用日志拦截器
func UnaryServerLoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		logGRPCUnaryCommon(ctx, "server", info.FullMethod, "", req, resp, err, time.Since(start), start, peerFromContext(ctx),
			constants.LogMsgGRPCRequest, constants.LogMsgGRPCRequestError)
		return resp, err
	}
}

// StreamServerLoggingInterceptor gRPC 流式调用日志拦截器
func StreamServerLoggingInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		logGRPCStreamCommon(ss.Context(), "server", info.FullMethod, "", info.IsClientStream, info.IsServerStream, err, time.Since(start), start, peerFromContext(ss.Context()),
			constants.LogMsgGRPCStream, constants.LogMsgGRPCStreamError)
		return err
	}
}

// logGRPCUnaryCommon 记录 gRPC 一元调用（服务端/客户端共享）
// component: "server" 或 "client"；serviceName: 客户端配置的服务名（服务端为空）
func logGRPCUnaryCommon(ctx context.Context, component, fullMethod, serviceName string, req, resp any, err error, duration time.Duration, start time.Time, peerAddr, successMsg, errMsg string) {
	if global.LOGGER == nil {
		return
	}

	config := getLoggingConfig()
	logger := NewRequestLogger(ctx)
	masker := activeMasker()
	captureReq := config.EnableRequest
	captureResp := config.EnableResponse

	fields := NewLogFields().
		AddGRPCMeta(ctx, component, fullMethod, "unary", start, peerAddr).
		AddValue(constants.LogFieldGRPCTimeMS, duration.Milliseconds()).
		AddSlow(duration, time.Duration(config.SlowGRPCThreshold)*time.Millisecond).
		AddRequestContext(ctx)
	if serviceName != "" {
		fields.Add(constants.LogFieldService, serviceName)
	}

	if err != nil {
		st, _ := status.FromError(err)
		fields.Add(constants.LogFieldStatus, st.Code().String()).Add(constants.LogFieldError, st.Message())
		if captureReq && req != nil {
			fields.Add(constants.LogFieldRequest, masker.Mask(marshalProto(req)))
		}
		logger.Log(constants.LogLevelError, "❌ "+errMsg, fields)
	} else {
		fields.Add(constants.LogFieldStatus, "OK")
		if captureReq && req != nil {
			fields.Add(constants.LogFieldRequest, masker.Mask(marshalProto(req)))
		}
		if captureResp && resp != nil {
			fields.Add(constants.LogFieldResponse, masker.Mask(marshalProto(resp)))
		}
		logger.Log(constants.LogLevelInfo, "✅ "+successMsg, fields)
	}
}

// logGRPCStreamCommon 记录 gRPC 流式调用（服务端/客户端共享）
func logGRPCStreamCommon(ctx context.Context, component, fullMethod, serviceName string, isClientStream, isServerStream bool, err error, duration time.Duration, start time.Time, peerAddr, successMsg, errMsg string) {
	if global.LOGGER == nil {
		return
	}

	config := getLoggingConfig()
	logger := NewRequestLogger(ctx)

	fields := NewLogFields().
		AddGRPCMeta(ctx, component, fullMethod, "stream", start, peerAddr).
		AddValue(constants.LogFieldGRPCTimeMS, duration.Milliseconds()).
		AddValue(constants.LogFieldClientStream, isClientStream).
		AddValue(constants.LogFieldServerStream, isServerStream).
		AddSlow(duration, time.Duration(config.SlowStreamThreshold)*time.Millisecond).
		AddRequestContext(ctx)
	if serviceName != "" {
		fields.Add(constants.LogFieldService, serviceName)
	}

	if err != nil {
		st, _ := status.FromError(err)
		fields.Add(constants.LogFieldStatus, st.Code().String()).Add(constants.LogFieldError, st.Message())
		logger.Log(constants.LogLevelError, "❌ "+errMsg, fields)
	} else {
		fields.Add(constants.LogFieldStatus, "OK")
		logger.Log(constants.LogLevelInfo, "📊 "+successMsg, fields)
	}
}

// ============================================================================
// gRPC 客户端日志拦截器
// ============================================================================

// UnaryClientLoggingInterceptor gRPC Client 一元调用日志拦截器
// 记录客户端发起的 gRPC 一元调用，包含服务名、方法、耗时、状态以及请求/响应摘要
func UnaryClientLoggingInterceptor(serviceName string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		logGRPCUnaryCommon(ctx, "client", method, serviceName, req, reply, err, time.Since(start), start, cc.Target(),
			constants.LogMsgGRPCClientRequest, constants.LogMsgGRPCClientRequestErr)
		return err
	}
}

// StreamClientLoggingInterceptor gRPC Client 流式调用日志拦截器
// 记录客户端建立的 gRPC 流，包含服务名、方法、流方向、耗时和状态
// 注意：仅记录流的建立阶段，流上的 SendMsg/RecvMsg 不在此拦截器范围内
func StreamClientLoggingInterceptor(serviceName string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		start := time.Now()
		stream, err := streamer(ctx, desc, cc, method, opts...)
		logGRPCStreamCommon(ctx, "client", method, serviceName, desc.ClientStreams, desc.ServerStreams, err, time.Since(start), start, cc.Target(),
			constants.LogMsgGRPCClientStream, constants.LogMsgGRPCClientStreamError)
		return stream, err
	}
}

// marshalProto 序列化 protobuf 消息
// 优先使用 protojson（基于 protobuf descriptor，比 encoding/json 反射快 3-5x），
// 序列化选项来自全局配置 Gateway.JSON，与 API 响应保持一致；
// 非 proto.Message 类型回退到 json.Marshal
func marshalProto(data any) []byte {
	if data == nil {
		return nil
	}
	// 优先走 protojson 快速路径
	if msg, ok := data.(proto.Message); ok {
		b, err := getProtojsonOptions().Marshal(msg)
		if err == nil {
			return b
		}
	}
	// 回退：非 protobuf 消息用标准 json
	jsonBytes, _ := json.Marshal(data)
	return jsonBytes
}
