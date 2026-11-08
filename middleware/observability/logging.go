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
	tblogger "github.com/kamalyes/go-toolbox/pkg/logger"
	"github.com/kamalyes/go-toolbox/pkg/matcher"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// 预计算日志消息常量，避免每次请求字符串拼接
var (
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

// maxHeaderMaskSize 请求头脱敏时允许的报文上限
//
// 取 64 KiB：请求头的真实上限由 http.Server.MaxHeaderBytes 决定（标准库默认 1 MiB，
// 实际部署远小于此），64 KiB 足够容纳一份带 Cookie 的浏览器请求头而不被截断
//
// [EN] 64 KiB: the real ceiling on request headers is http.Server.MaxHeaderBytes (1 MiB by
// default in the standard library, far less in practice), and 64 KiB holds a browser's headers
// with cookies without truncation
const maxHeaderMaskSize = 64 << 10

// headerMaskerEntry 按配置缓存的请求头脱敏器
type headerMaskerEntry struct {
	src    *gcmiddleware.Logging
	masker *desensitize.DataMasker
}

var headerMaskerCache atomic.Pointer[headerMaskerEntry]

// activeHeaderMasker 请求头专用的脱敏器，按配置建一次
//
// 为什么不直接复用 global.DATAMASKER：它的 MaxBodySize 来自 logging.max-body-size
// （默认 2048），那是给请求体定的尺寸 一份带 Cookie 的浏览器请求头很容易超过 2 KiB，
// 超了就会在脱敏入口被截断成非法 JSON，于是整表退回文本脱敏，结果既不是 JSON、
// 也没法再解析回 map —— 只有敏感键名规则是要共用的，尺寸不是
//
// 缓存按配置指针判失效，与 request_attr_index 的做法一致：热重载换了配置就重建
//
// [EN] Not simply reusing global.DATAMASKER because its MaxBodySize comes from
// logging.max-body-size (2048 by default), a size meant for request bodies. A browser's headers
// with cookies easily exceed 2 KiB, and once truncated at the masking entry point they are no
// longer valid JSON, so the whole map falls back to text masking and can no longer be parsed
// back into a map. What should be shared is the sensitive-key rules, not the size.
//
// The cache is invalidated by config pointer identity, the same way request_attr_index does it:
// a hot reload replacing the config triggers a rebuild.
func activeHeaderMasker() *desensitize.DataMasker {
	cfg := getLoggingConfig()
	if entry := headerMaskerCache.Load(); entry != nil && entry.src == cfg {
		return entry.masker
	}
	masker := desensitize.NewMasker(&desensitize.MaskerConfig{
		SensitiveKeys: cfg.SensitiveKeys,
		SensitiveMask: cfg.SensitiveMask,
		MaxBodySize:   maxHeaderMaskSize,
	})
	headerMaskerCache.Store(&headerMaskerEntry{src: cfg, masker: masker})
	return masker
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
//
// 开关在请求路径上判，而不是在构造期判掉
//
// 原先这里先读一次配置，Enabled 为 false 就直接返回透传、根本不装这一层；装上之后又
// 再也不看 reqConfig.Enabled 于是两个方向的热重载都是失效的：启动时关着的，之后在
// 配置里打开不会生效（这一层压根没装）；启动时开着的，之后在配置里关掉也停不下来
// （装上的那一层不看开关）注释写着「每次请求读取一次配置，支持热重载」，
// 唯独 Enabled 这个开关被排除在外
//
// 代价是关着日志时每个请求多一次透传调用与一次 getLoggingConfig（几次指针解引用），
// 换来的是运行期真的能开关 —— 出故障时临时关掉日志降载是实际会用到的操作
//
// [EN] The switch is evaluated on the request path rather than decided at construction.
//
// This used to read the config once, return a pass-through when Enabled was false and never
// install the layer at all; once installed it never looked at reqConfig.Enabled again. Hot
// reload was therefore broken in both directions: enabling it later had no effect because the
// layer was never installed, and disabling it later had no effect because the installed layer
// ignored the switch. The comment said "the config is read once per request to support hot
// reload" while that one switch was the exception.
//
// The cost is one pass-through call and one getLoggingConfig (a few pointer dereferences) per
// request while logging is off, in exchange for the switch actually working at runtime —
// turning logging off to shed load during an incident is a real operation.
func LoggingMiddleware() core.HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := r.Context()
			// 每次请求读取一次配置，支持热重载且避免后续重复读取
			reqConfig := getLoggingConfig()

			if !reqConfig.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// 跳过路径检查
			if isSkipPath(reqConfig, r.URL.Path) {
				wrapped := core.NewResponseWriter(w)
				// Release 必须走 defer：next.ServeHTTP panic 时（由上游 Recovery 兜住）
				// 裸调的那一行永远不会执行，这个 writer 就再也回不到池里
				// 下面主路径本来就是 defer，这里漏了
				// [EN] Release must be deferred: when next.ServeHTTP panics (caught upstream by
				// Recovery) a bare call never runs and the writer never returns to the pool.
				// The main path below already defers it; this branch did not
				defer wrapped.Release()
				next.ServeHTTP(wrapped, r)
				if wrapped.StatusCode() >= 400 {
					logHTTPError(ctx, r, wrapped, time.Since(start))
				}
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

// logLevelEnabled 报告当前日志器等级下这条日志会不会真的被写出
//
// 为什么调用方要先问一句：日志库的等级判定在 logWithContextKV 的入口，
// 也就是**调用方已经把所有字段装箱、拼好切片之后**才判
// 于是 level=warn 的生产配置下，每个 2xx 请求照样要为一条永不写出的 info 日志付出：
// 一次 LogFields 分配（cap 32 的 []any，512 字节）、每个动态字符串值一次装箱
// （访问日志常态 10 个，profile 里 LogFields.Add 单独占 10 次/请求）、一个 RequestLogger
// —— 它们全部白付
//
// 装箱本身去不掉 —— InfoContextKV(ctx, msg, kv ...any) 的签名决定了
// 每个动态值都要在堆上留一份 16 字节的接口数据 能去掉的是「这条日志根本不会被写出时，整段构建都不做」
//
// 未知等级返回 true：宁可多写一条日志，也不要因为常量改名静默丢日志
//
// [EN] Reports whether a line at this level would actually be written.
//
// Why callers ask first: the library checks the level at the entry of logWithContextKV, that
// is *after* the caller has boxed every field and assembled the slice. Under a level=warn
// production configuration every 2xx request therefore still pays for an info line that will
// never be written: one LogFields allocation (a []any with capacity 32, 512 bytes), one boxing
// per dynamic string value (ten in a typical access log, which is what LogFields.Add's ten
// allocations per request in the profile are) and one RequestLogger — all of it wasted.
//
// The boxing itself cannot be removed: the signature InfoContextKV(ctx, msg, kv ...any) forces
// a 16-byte interface payload on the heap for every dynamic value. What can be removed is
// building any of it when the line will not be written.
//
// An unknown level returns true: better an extra log line than silently losing logs because a
// constant was renamed
func logLevelEnabled(level string) bool {
	lg := global.LOGGER
	if lg == nil {
		return false
	}
	var target tblogger.LogLevel
	switch level {
	case constants.LogLevelInfo:
		target = tblogger.INFO
	case constants.LogLevelWarn:
		target = tblogger.WARN
	case constants.LogLevelError:
		target = tblogger.ERROR
	default:
		return true
	}
	return lg.GetLevel().IsEnabled(target)
}

// httpLogLevelFor 按状态码给出日志等级与消息
//
// 从 logHTTPRequest 里提出来，因为等级要在构建字段**之前**定下来 —— 先知道等级，
// 才能判断这条日志会不会被写出，从而决定要不要构建字段
//
// [EN] Extracted from logHTTPRequest because the level must be known *before* the fields are
// built: only then can the caller tell whether the line will be written at all
func httpLogLevelFor(status int) (level, message string) {
	switch {
	case status >= 500:
		return constants.LogLevelError, logMsgHTTPRequestError
	case status >= 400:
		return constants.LogLevelWarn, logMsgHTTPRequestWarn
	default:
		return constants.LogLevelInfo, logMsgHTTPRequestOK
	}
}

// logHTTPRequest 记录 HTTP 请求
func logHTTPRequest(ctx context.Context, r *http.Request, rw *core.ResponseWriter, start time.Time, duration time.Duration, config *gcmiddleware.Logging, reqBody []byte, hasAccessLog bool) {
	level, message := httpLogLevelFor(rw.StatusCode())
	emit := logLevelEnabled(level)

	// 两个消费者都不需要报文时连包装都不建 maskedBody.get() 对 nil 接收者返回空串，
	// 所以 captureAccessLog 那条路上传 nil 也是安全的（它只在 hasAccessLog 时被调用）
	// [EN] The wrappers are not built when neither consumer needs them; maskedBody.get()
	// returns "" for a nil receiver, so passing nil down the captureAccessLog path is safe
	var maskedReq, maskedResp *maskedBody
	if emit || hasAccessLog {
		maskedReq = newMaskedBody(reqBody)
		maskedResp = newMaskedBody(rw.GetBody())
	}

	if emit {
		logHTTPRequestFields(ctx, r, rw, duration, config, reqBody, level, message, maskedReq, maskedResp)
	}

	// 派发访问日志钩子 等级把运行时日志过滤掉不影响这里：
	// 审计落库与运行时日志是两个独立的消费者
	// [EN] Dispatching the access-log hook is unaffected by the runtime log being filtered out:
	// audit persistence and the runtime log are independent consumers
	if hasAccessLog {
		captureAccessLog(ctx, r, rw, start, duration, maskedReq, maskedResp, time.Duration(config.SlowHTTPThreshold)*time.Millisecond)
	}
}

// logHTTPRequestFields 构建并写出运行时访问日志
// 只在 logLevelEnabled 为真时被调用，见 logLevelEnabled 的说明
//
// [EN] Builds and writes the runtime access log; only called when logLevelEnabled is true
func logHTTPRequestFields(ctx context.Context, r *http.Request, rw *core.ResponseWriter, duration time.Duration, config *gcmiddleware.Logging, reqBody []byte, level, message string, maskedReq, maskedResp *maskedBody) {
	logger := NewRequestLogger(ctx)

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

	// 请求体与响应体
	//
	// 必须判 EnableRequest / EnableResponse，不能只判「捕获到的字节数 > 0」：
	// 捕获的条件是 EnableXxx || hasAccessLog（见 LoggingMiddleware），
	// 于是「注册了审计钩子 + 保持默认 EnableResponse:false」这个完全正常的生产配置下，
	// 原先的判断让响应体照样写进运行时日志 —— 运维显式关掉的东西并没有关掉
	//
	// 两个开关管的只是**运行时日志**；访问日志钩子拿到的 AccessRecord 仍然带着报文，
	// 审计钩子正是靠它落库的，这一点由 TestAccessLogStillReceivesBodyWhenRuntimeLogOff 守住
	//
	// 顺带去掉了关着开关时白付的那次 masker.Mask：它内部是
	// json.Unmarshal 到 any 再 Marshal 回去的完整往返，profile 里占访问日志层
	// 全部分配的 24%，而这些分配的结果本来就不该出现在日志里
	//
	// [EN] EnableRequest / EnableResponse must be tested rather than only "captured length
	// > 0": capture is enabled by EnableXxx || hasAccessLog, so with an audit hook registered
	// and the default EnableResponse:false the previous condition still wrote the response
	// body into the runtime log — what the operator explicitly turned off was not off.
	//
	// The two flags govern the *runtime log* only; the access-log hook's AccessRecord still
	// carries the bodies, which is how an audit hook persists them, locked down by
	// TestAccessLogStillReceivesBodyWhenRuntimeLogOff.
	//
	// This also drops the masker.Mask call that was paid for while the flag was off: it is a
	// full json.Unmarshal-to-any plus Marshal round trip internally, measured at 24% of all
	// allocations in the access-log layer, producing a result that was not meant to be logged
	// 报文的脱敏结果包成 maskedBody，让运行时日志与下面的访问日志共用一次计算：
	// 两者原先各自对同一份字节调一次 masker.Mask，而 Mask 内部是
	// json.Unmarshal 到 any 再 Marshal 回去的完整往返，同样的输入算了两遍
	// [EN] The masked bodies are wrapped so the runtime log and the access log below share one
	// computation: each used to call masker.Mask on the same bytes, and Mask is a full
	// json.Unmarshal-to-any plus Marshal round trip internally
	if config.EnableRequest && len(reqBody) > 0 &&
		isLoggableContentType(config, r.Header.Get(constants.HeaderContentType)) {
		fields.Add(constants.LogFieldRequest, maskedReq.get())
	}

	if config.EnableResponse {
		if respBody := rw.GetBody(); len(respBody) > 0 &&
			isLoggableContentType(config, rw.Header().Get(constants.HeaderContentType)) {
			fields.Add(constants.LogFieldResponse, maskedResp.get())
		}
	}

	logger.Log(level, message, fields)
}

// logHTTPError 记录跳过路径的错误 🚫
func logHTTPError(ctx context.Context, r *http.Request, rw *core.ResponseWriter, duration time.Duration) {
	if !logLevelEnabled(constants.LogLevelWarn) {
		return
	}
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

	// 等级只取决于 err，可以在构建字段之前判掉 这两个函数除了写日志没有别的副作用，
	// 所以直接返回是安全的
	// [EN] The level depends only on err, so it can be decided before the fields are built;
	// these functions have no side effect other than logging, so returning early is safe
	if !logLevelEnabled(grpcLogLevelFor(err)) {
		return
	}

	config := getLoggingConfig()
	logger := NewRequestLogger(ctx)
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
			fields.Add(constants.LogFieldRequest, maskBody(marshalProto(req)))
		}
		logger.Log(constants.LogLevelError, "❌ "+errMsg, fields)
	} else {
		fields.Add(constants.LogFieldStatus, "OK")
		if captureReq && req != nil {
			fields.Add(constants.LogFieldRequest, maskBody(marshalProto(req)))
		}
		if captureResp && resp != nil {
			fields.Add(constants.LogFieldResponse, maskBody(marshalProto(resp)))
		}
		logger.Log(constants.LogLevelInfo, "✅ "+successMsg, fields)
	}
}

// grpcLogLevelFor gRPC 调用的日志等级：出错写 error，否则 info
// [EN] The level for a gRPC call: error on failure, info otherwise
func grpcLogLevelFor(err error) string {
	if err != nil {
		return constants.LogLevelError
	}
	return constants.LogLevelInfo
}

// logGRPCStreamCommon 记录 gRPC 流式调用（服务端/客户端共享）
func logGRPCStreamCommon(ctx context.Context, component, fullMethod, serviceName string, isClientStream, isServerStream bool, err error, duration time.Duration, start time.Time, peerAddr, successMsg, errMsg string) {
	if global.LOGGER == nil {
		return
	}

	if !logLevelEnabled(grpcLogLevelFor(err)) {
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
