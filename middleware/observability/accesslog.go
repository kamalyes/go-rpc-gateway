/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-30 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-31 01:15:00
 * @FilePath: \go-rpc-gateway\middleware\observability\accesslog.go
 * @Description: 访问日志 - AccessRecord 模型 + 钩子注册/派发 + HTTP 快照构建
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package observability

import (
	"context"
	"database/sql/driver"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/kamalyes/go-rpc-gateway/constants"
	sqltypes "github.com/kamalyes/go-rpc-gateway/cpool/database/types"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware/core"
	"github.com/kamalyes/go-toolbox/pkg/desensitize"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/netx"
)

// AccessLogKind 访问日志来源类型
type AccessLogKind string

const (
	AccessLogKindHTTP       AccessLogKind = "http"        // HTTP 入口请求
	AccessLogKindGRPCServer AccessLogKind = "grpc_server" // gRPC Server 调用（预留）
	AccessLogKindGRPCClient AccessLogKind = "grpc_client" // gRPC Client 出站调用（预留）
)

// Value 实现 driver.Valuer：ClickHouse 驱动的 String 列仅识别内置 string 类型，命名类型需显式转换
func (k AccessLogKind) Value() (driver.Value, error) {
	return string(k), nil
}

// AccessRecord 一次请求/响应的结构化快照（访问日志）
// 字段与运营日志详情页一一对应：请求时间、响应时间、响应时长、用户、
// 租户、IP、操作结果、路径、headers、query、body、response
// gorm tag 仅保留 column/default/comment，不含任何 DB 专属类型——多 DB 适配由 go-sqlbuilder 方言层负责
type AccessRecord struct {
	ID           string                     `gorm:"column:id;comment:日志ID" json:"id"`
	Kind         AccessLogKind              `gorm:"column:kind;default:'';comment:来源类型(http/grpc_server/grpc_client)" json:"kind"`
	RequestTime  time.Time                  `gorm:"column:request_time;comment:请求时间" json:"requestTime"`
	ResponseTime time.Time                  `gorm:"column:response_time;comment:响应时间" json:"responseTime"`
	DurationMS   int64                      `gorm:"column:duration_ms;default:0;comment:响应时长(毫秒)" json:"durationMs"`
	Service      string                     `gorm:"column:service;default:'';comment:服务名(网关或下游服务)" json:"service"`
	Method       string                     `gorm:"column:method;default:'';comment:HTTP Method / gRPC FullMethod" json:"method"`
	Path         string                     `gorm:"column:path;default:'';comment:请求路径" json:"path"`
	Query        string                     `gorm:"column:query;default:'';comment:URL Query 参数" json:"query"`
	Headers      sqltypes.CompressedTextMap `gorm:"column:headers;comment:请求头" json:"headers"`
	Body         sqltypes.CompressedText    `gorm:"column:body;default:'';comment:请求体" json:"body"`
	UserAgent    string                     `gorm:"column:user_agent;default:'';comment:User-Agent" json:"userAgent"`
	Source       string                     `gorm:"column:source;default:'';comment:请求来源(X-Source，回退Domain)" json:"source"`
	StatusCode   int32                      `gorm:"column:status_code;default:0;comment:HTTP 状态码" json:"statusCode"`
	Status       string                     `gorm:"column:status;default:'';comment:状态描述(gRPC Code / HTTP Status)" json:"status"`
	Error        string                     `gorm:"column:error;default:'';comment:错误信息" json:"error"`
	Response     sqltypes.CompressedText    `gorm:"column:response;default:'';comment:响应体" json:"response"`
	Bytes        int64                      `gorm:"column:bytes;default:0;comment:响应字节数" json:"bytes"`
	Slow         bool                       `gorm:"column:slow;default:0;comment:慢请求标记" json:"slow"`
	Success      bool                       `gorm:"column:success;default:0;comment:操作结果(1=成功)" json:"success"`
	TraceID      string                     `gorm:"column:trace_id;default:'';comment:跟踪ID" json:"traceID"`
	RequestID    string                     `gorm:"column:request_id;default:'';comment:请求ID" json:"requestID"`
	UserID       string                     `gorm:"column:user_id;default:'';comment:用户ID" json:"userID"`
	Account      string                     `gorm:"column:account;default:'';comment:账号(UserName，回退UserID)" json:"account"`
	TenantID     string                     `gorm:"column:tenant_id;default:'';comment:租户ID" json:"tenantID"`
	ClientIP     string                     `gorm:"column:client_ip;default:'';comment:客户端IP" json:"clientIP"`
}

// AccessLogHandler 访问日志钩子函数
// 约定：必须非阻塞（如写入内存缓冲/channel），框架已带 recover 保护
type AccessLogHandler func(ctx context.Context, rec *AccessRecord)

// accessLogEntry 包一层指针，使反注册可以按身份从快照里移除
//
// 函数值本身除了与 nil 比较之外不可比较，没法在切片里定位；包成指针后
// unregister 持有自己的 entry，按指针相等删除即可
// [EN] Function values are only comparable to nil, so they cannot be located in a slice.
// Wrapping them in a pointer lets unregister hold its own entry and remove it by identity
type accessLogEntry struct {
	handler AccessLogHandler
}

// 访问日志钩子用写时复制（copy-on-write）快照保存：
//
//   - DispatchAccessLog 在请求热路径上，Load 一个原子指针即可遍历，无锁无分配；
//     旧实现先读切片头再解锁遍历，与反注册的 accessLogHandlers[idx] = nil 是数据竞争
//   - 注册/反注册都是低频操作，复制一个只含指针的小切片成本可以忽略
//   - 反注册真正把 entry 从当前快照里删掉，而不是在原槽位写 nil：
//     反复「注册 → 反注册」的插件式调用方不会让切片和遍历成本无界增长
//
// 注意快照内容在发布后不再修改，所以派发时拿到的切片可以安全地长时间持有
// [EN] Copy-on-write snapshot of access-log hooks:
//   - DispatchAccessLog is on the request hot path and only needs one atomic Load, with no
//     lock and no allocation; the old code read the slice header, unlocked and then ranged,
//     which raced with the unregister write of accessLogHandlers[idx] = nil
//   - register/unregister are rare, so copying a tiny slice of pointers is negligible
//   - unregister actually removes the entry from the current snapshot instead of writing nil
//     into its slot, so repeated register/unregister cycles cannot grow the slice forever
//
// The snapshot is never mutated after publication, so a slice handed to dispatch is safe to
// keep iterating even while a concurrent register/unregister publishes a new one
var (
	accessLogEntries atomic.Pointer[[]*accessLogEntry]
	accessLogCount   atomic.Int32 // 活跃钩子计数，避免 HasAccessLogHandlers 遍历
)

func init() {
	empty := make([]*accessLogEntry, 0)
	accessLogEntries.Store(&empty)
}

// RegisterAccessLog 注册访问日志钩子，返回反注册函数
func RegisterAccessLog(h AccessLogHandler) (unregister func()) {
	if h == nil {
		return func() {}
	}

	entry := &accessLogEntry{handler: h}
	for {
		old := accessLogEntries.Load()
		next := make([]*accessLogEntry, len(*old)+1)
		copy(next, *old)
		next[len(*old)] = entry
		if accessLogEntries.CompareAndSwap(old, &next) {
			accessLogCount.Add(1)
			break
		}
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			for {
				old := accessLogEntries.Load()
				idx := -1
				for i, e := range *old {
					if e == entry {
						idx = i
						break
					}
				}
				if idx < 0 {
					return // 已被移除（理论上 once 已拦住，留作状态不一时的兜底）
				}

				next := make([]*accessLogEntry, 0, len(*old)-1)
				next = append(next, (*old)[:idx]...)
				next = append(next, (*old)[idx+1:]...)
				if accessLogEntries.CompareAndSwap(old, &next) {
					accessLogCount.Add(-1)
					return
				}
			}
		})
	}
}

// HasAccessLogHandlers 是否已注册访问日志钩子（无锁 atomic 读取，热路径零开销）
func HasAccessLogHandlers() bool {
	return accessLogCount.Load() > 0
}

// DispatchAccessLog 同步派发访问日志到所有已注册钩子（单个 hook panic 不影响其他 hook 与主流程）
func DispatchAccessLog(ctx context.Context, rec *AccessRecord) {
	if rec == nil {
		return
	}

	for _, entry := range *accessLogEntries.Load() {
		if entry == nil || entry.handler == nil {
			continue
		}
		func() {
			defer func() { _ = recover() }()
			entry.handler(ctx, rec)
		}()
	}
}

// captureAccessLog 捕获 HTTP 请求/响应快照，构建访问日志记录并派发给已注册钩子
// 仅在 HasAccessLogHandlers() 为 true 时调用，零钩子场景零开销
func captureAccessLog(ctx context.Context, r *http.Request, rw *core.ResponseWriter, start time.Time, duration time.Duration, reqBody, respBody *maskedBody, slowThreshold time.Duration) {
	meta := core.GetRequestCommonMeta(ctx)
	serviceName := ""
	if cfg := global.GetGatewayConfig(); cfg != nil {
		serviceName = cfg.Name
	}

	rec := &AccessRecord{
		ID:           uuid.NewString(),
		Kind:         AccessLogKindHTTP,
		Service:      serviceName,
		RequestTime:  start,
		ResponseTime: start.Add(duration),
		DurationMS:   duration.Milliseconds(),
		Method:       r.Method,
		Path:         r.URL.Path,
		Query:        r.URL.RawQuery,
		Headers:      maskHeaders(r.Header),
		Body:         sqltypes.CompressedText(reqBody.get()),
		UserAgent:    r.Header.Get(constants.HeaderUserAgent),
		Source:       mathx.IfEmpty(r.Header.Get(constants.HeaderXSource), meta.Domain),
		StatusCode:   int32(rw.StatusCode()),
		Status:       http.StatusText(rw.StatusCode()),
		Response:     sqltypes.CompressedText(respBody.get()),
		Bytes:        rw.BytesWritten(),
		Slow:         duration > slowThreshold,
		Success:      rw.StatusCode() == http.StatusOK,
		TraceID:      meta.TraceID,
		RequestID:    meta.RequestID,
		UserID:       meta.UserID,
		Account:      mathx.IfEmpty(meta.UserName, meta.UserID),
		TenantID:     meta.TenantID,
		ClientIP:     netx.GetClientIP(r),
	}

	DispatchAccessLog(ctx, rec)
}

// maskHeaders 请求头脱敏后再入库
//
// 改造前这里是 sqltypes.CompressedTextMap(r.Header)：请求体与响应体都过了 masker.Mask，
// 唯独请求头原样入库 —— 也就是 Authorization、Cookie、X-Token、签名头全以明文落在审计库里，
// 而审计库恰恰是会被导出、被下游分析、保留期最长的那一份数据
//
// 走脱敏器的 JSON 结构化路径而不是自己维护一份敏感头名单：脱敏器内置的键名规则表已经覆盖
// authorization / cookie / set_cookie / token / signature / api_key / session 这些，
// 键名规范化时把 - _ . 都当分隔符，所以 HTTP 头的写法（X-Token、Set-Cookie）本来就能命中；
// 用户在 logging.sensitive-keys 里追加的键名也一并生效 自己再列一份必然和它漂移
//
// 顺带去掉一处别名：CompressedTextMap(r.Header) 是类型转换，不复制底层 map，
// 等于把请求的 header map 直接交给可能异步消费的钩子现在返回的是新建的 map
//
// 脱敏后解析不回来时返回 nil：审计日志里宁可缺这一列，也不能漏出凭据
//
// [EN] This used to be sqltypes.CompressedTextMap(r.Header): request and response bodies both
// went through masker.Mask while the headers went in verbatim, so Authorization, Cookie,
// X-Token and the signature headers all landed in the audit store in clear — and the audit
// store is precisely the data that gets exported, analysed downstream and retained longest.
//
// It goes through the masker's structured JSON path rather than keeping a private list of
// sensitive header names: the masker's built-in key table already covers authorization, cookie,
// set_cookie, token, signature, api_key and session, and its key normalisation treats -, _ and
// . as separators, so HTTP spellings such as X-Token and Set-Cookie match as they are. Keys the
// user adds under logging.sensitive-keys apply too. A second list would inevitably drift.
//
// It also removes an aliasing issue: CompressedTextMap(r.Header) is a type conversion and does
// not copy the underlying map, handing the request's own header map to hooks that may consume
// it asynchronously. The returned map is freshly built.
//
// On a parse failure it returns nil: better to miss this column in the audit log than to leak
// credentials into it.
func maskHeaders(header http.Header) sqltypes.CompressedTextMap {
	if len(header) == 0 {
		return nil
	}

	mask := headerMaskText()
	extra := headerExtraSensitiveKeys()

	// 所有头的值共用一块平坦底层数组
	//
	// 原先是每个头各 append(make([]string, 0, len(values)), values...) 一次，于是分配次数随头数线性增长：
	// 7 个请求头的常态形状下 maskHeaders 一共 8 次分配，memprofile 里 accesslog.go 的复制那一行单独占到 5.9 次/请求现在是 1 次 map 加 1 次数组，与头数无关
	//
	// cap 取 sum(max(len(values), 1)) 是严格上界：KeepAsIs 分支写 len(values) 个，
	// 擦除分支无论原有几个值都只写 1 个掩码（值为空切片的键也要占这 1 个，
	// 所以每个键至少留一个位置）于是 append 永不扩容 ——
	// 这个前提必须成立：一旦扩容，先前交出去的子切片会继续指向旧数组，
	// 后面的头再也改不动它们，表现为「审计库里某些头拿到的是过期值」
	// TestMaskHeadersSharesOneBackingArray 闸住它
	//
	// 子切片用三下标形式把 cap 掐到 len：记录会交给钩子异步持有，
	// 不掐的话钩子对某个头的值 append 一下，写进去的东西会落到相邻头的值上
	//
	// [EN] All header values share one flat backing array. It used to be one
	// append(make(...), values...) per header, so the allocation count grew linearly with the
	// header count: eight allocations for the common seven-header shape, of which the copy line
	// alone was 5.9 per request in the memory profile. It is now one map plus one array,
	// independent of the header count.
	//
	// The capacity sum(max(len(values), 1)) is a strict upper bound -- KeepAsIs writes
	// len(values) entries, every other action writes exactly one mask regardless (a key with an
	// empty value slice still needs that one slot) -- so append never grows the array. That
	// must hold: on a reallocation the sub-slices already handed out keep pointing at the old
	// array and no longer track later writes, which would surface as stale header values in the
	// audit store. TestMaskHeadersSharesOneBackingArray locks it down.
	//
	// The three-index form caps the sub-slices at their length: the record is held
	// asynchronously by hooks, and without it a hook appending to one header's values would
	// overwrite the next header's
	total := 0
	for _, values := range header {
		if n := len(values); n > 0 {
			total += n
		} else {
			total++
		}
	}

	out := make(map[string][]string, len(header))
	buf := make([]string, 0, total)
	for name, values := range header {
		start := len(buf)
		switch keyMaskAction(name, extra) {
		case desensitize.KeepAsIs:
			// 复制而不是共享底层数组：记录会被钩子异步持有，
			// 而 http.Header 的切片在请求结束后可能被 net/http 复用
			// [EN] Copied rather than shared: the record may be held asynchronously by a hook
			// while net/http can reuse the header's slices once the request is over
			buf = append(buf, values...)

		default:
			// 非 KeepAsIs 的动作一律整体擦除，包括 MaskPhone / MaskEmail /
			// MaskFingerprint / MaskEncrypt
			//
			// 这不是偷懒，而是刻意与改造前的 JSON 往返路径逐字保持一致。
			// 那条路径上请求头的值是 JSON 数组，而库里的 maskTextValue 对任何
			// 非字符串值一律返回完整掩码 —— 于是部分遮蔽的动作在**请求头上**
			// 从来就是整体擦除，那是现有行为，不是疏漏
			//
			// 为什么不"顺手改好"成逐值部分遮蔽：请求头上的部分遮蔽会往审计库里
			// 写入比今天更多的 PII（手机号末 4 位、邮箱域名与首字符）。
			// 审计库是留存最久、导出最多的那份数据，在脱敏路径上扩大泄露面
			// 需要的是一个明确的产品决定，而不是一次性能优化的副作用。
			// TestMaskHeadersMatchesLegacyJSONPath 闸住这个等价关系；
			// 真要改成部分遮蔽，改的是那条测试的期望值，而不是悄悄改行为
			//
			// MaskEncrypt 另有一层原因：它本应是确定性加密（可关联、持密钥可还原），
			// 但要走 DataMasker 未导出的 encryptValue，拿不到。方向必须是
			// 「擦得比该擦的多」而不是「少」
			//
			// [EN] Every action other than KeepAsIs wipes the value entirely, including
			// MaskPhone, MaskEmail, MaskFingerprint and MaskEncrypt.
			//
			// This deliberately matches the previous JSON round-trip path byte for byte. On
			// that path a header's value is a JSON array, and the library's maskTextValue
			// returns the full mask for any non-string value — so the partial-masking actions
			// have always been outright wipes *on headers*. That is the existing behaviour,
			// not an oversight.
			//
			// Why it is not quietly "improved" into per-value partial masking: on headers that
			// would write more PII into the audit store than today (a phone's last four digits,
			// an email's domain and first character). The audit store is the data retained
			// longest and exported most, and widening the exposure surface on a redaction path
			// needs an explicit product decision rather than being a side effect of a
			// performance change. TestMaskHeadersMatchesLegacyJSONPath locks the equivalence
			// down: switching to partial masking means changing that test's expectations, not
			// changing the behaviour silently.
			//
			// MaskEncrypt has a further reason: it should be deterministic encryption but needs
			// DataMasker's unexported encryptValue, which is unreachable. The direction must be
			// "redact more than needed" rather than less
			buf = append(buf, mask)
		}
		out[name] = buf[start:len(buf):len(buf)]
	}
	return out
}

// keyMaskAction 给出一个键名对应的脱敏动作（请求头名、JSON 报文里的键名都走这里）
//
// 判定与 DataMasker.actionFor 逐项等价：先查内置规则表（ActionForKeyLoose，已导出，
// 零分配），再按配置里的额外敏感键做不区分大小写的子串匹配
//
// 刻意不在网关侧复制任何规则：内置表仍然只有 go-toolbox 那一份，
// 库里新增规则这里自动跟上 —— 复制一份规则表是脱敏逻辑最常见的漏擦来源
//
// 返回的是精确动作而不是「要不要擦」，尽管目前唯一的调用方 maskHeaders 把所有
// 非 KeepAsIs 一律整体擦除（理由见那里：与改造前的 JSON 往返路径保持等价）
// 这么分工有两个原因：等价关系能被 TestHeaderKeyActionMatchesMaskerActionFor 按动作逐项断言，
// 比断言一个布尔值严得多；真要把请求头改成逐值部分遮蔽，改的是 maskHeaders 的分支，
// 这里不用动
//
// [EN] Equivalent to DataMasker.actionFor item by item: the built-in table first via the
// exported, zero-allocation ActionForKeyLoose, then a case-insensitive substring match against
// the configured extra keys.
//
// No rule is duplicated on the gateway side: the built-in table remains the single copy inside
// go-toolbox, so a rule added there is picked up here automatically — duplicating a redaction
// rule table is the most common way redaction starts missing things.
//
// It returns the precise action rather than a yes-or-no even though its only caller,
// maskHeaders, wipes every non-KeepAsIs value entirely (its own comment explains why: byte-for
// -byte equivalence with the previous JSON round trip). The split earns its keep twice over:
// the equivalence is asserted action by action, which is far stricter than asserting a boolean,
// and moving headers to per-value partial masking would change a branch in maskHeaders without
// touching this function
func keyMaskAction(name string, extra []string) desensitize.KeyAction {
	if action := desensitize.ActionForKeyLoose(name); action != desensitize.KeepAsIs {
		return action
	}
	for _, key := range extra {
		if containsFold(name, key) {
			// 配置里追加的敏感键一律整体擦除，与 DataMasker.actionFor 一致
			// [EN] Configured extra keys wipe entirely, matching DataMasker.actionFor
			return desensitize.WipeValue
		}
	}
	return desensitize.KeepAsIs
}

// containsFold 报告 s 是否包含 substr，ASCII 大小写不敏感，不分配
//
// 与 go-toolbox 的同名内部函数逐项等价（它未导出）：请求头名与配置里的字段名都是
// ASCII，故只折叠 A-Z
// 不走 strings.Contains(strings.ToLower(s), strings.ToLower(substr))：那是每个
// 请求头两次字符串复制
//
// [EN] Reports whether s contains substr, ASCII case-insensitively, without allocating.
// Equivalent to go-toolbox's unexported function of the same name; header names and configured
// field names are ASCII, so only A-Z is folded. The ToLower form would copy two strings per
// header
func containsFold(s, substr string) bool {
	n := len(substr)
	if n == 0 {
		return true
	}
	if n > len(s) {
		return false
	}
	first := lowerASCII(substr[0])
	for i := 0; i+n <= len(s); i++ {
		if lowerASCII(s[i]) != first {
			continue
		}
		match := true
		for j := 1; j < n; j++ {
			if lowerASCII(s[i+j]) != lowerASCII(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// lowerASCII 把 A-Z 折叠为小写，其余字节原样返回
func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// headerMaskText 掩码文本，与 DataMasker.getMask 的取法一致
func headerMaskText() string {
	if cfg := getLoggingConfig(); cfg != nil && cfg.SensitiveMask != "" {
		return cfg.SensitiveMask
	}
	return defaultSensitiveMask
}

// headerExtraSensitiveKeys 配置里追加的敏感字段名
func headerExtraSensitiveKeys() []string {
	if cfg := getLoggingConfig(); cfg != nil {
		return cfg.SensitiveKeys
	}
	return nil
}

// defaultSensitiveMask 与 go-toolbox desensitize 的默认掩码一致
//
// 该常量在库里未导出，这里取同值 TestHeaderMaskTextMatchesMasker 用脱敏器的实际
// 输出闸住这个等价关系，库里改了默认值测试会失败
//
// [EN] The library's constant is unexported, so the same value is used here;
// TestHeaderMaskTextMatchesMasker locks the equivalence against the masker's actual output, so
// the test fails if the library's default changes
const defaultSensitiveMask = "***"

// maskedBody 对同一份字节只脱敏一次的记忆化包装
//
// 为什么需要：运行时日志（logHTTPRequest）与访问日志（captureAccessLog）是同一个请求里
// 的两个消费者，它们原先各自对**同一份** reqBody 调一次 activeMasker().Mask，
// 而 Mask 内部是 json.Unmarshal 到 any 再 Marshal 回去的完整往返 ——
// 同样的字节、同一个脱敏器、同样的结果，算了两遍
//
// 走 activeMasker 而不是直接读 global.DATAMASKER：后者只由网关的完整初始化链赋值，
// 而中间件是导出的、可以被单独使用 activeMasker 在它为 nil 时给一个兜底脱敏器，
// 直接解引用等于把那层保护绕过去
//
// 不做并发保护：一个 maskedBody 只在单个请求的处理路径内使用，不跨 goroutine
// 钩子拿到的是 AccessRecord 里已经取出的字符串，不是这个包装本身
//
// [EN] Memoises the masking of one body so it happens at most once.
//
// Why: the runtime log and the access log are two consumers within one request, and each used
// to call activeMasker().Mask on the *same* reqBody, while Mask internally is a full
// json.Unmarshal-to-any plus Marshal round trip — the same bytes through the same masker for
// the same result, computed twice.
//
// activeMasker rather than a direct read of global.DATAMASKER: that variable is only assigned
// by the gateway's full init chain while the middlewares are exported and usable on their own,
// and activeMasker supplies a fallback masker when it is nil.
//
// No concurrency guard: a maskedBody lives inside a single request's handling path and never
// crosses goroutines; a hook receives the already-extracted string in the AccessRecord, not
// this wrapper
type maskedBody struct {
	raw  []byte
	val  string
	done bool
}

// newMaskedBody 包装一份待脱敏的字节
func newMaskedBody(raw []byte) *maskedBody { return &maskedBody{raw: raw} }

// get 返回脱敏后的文本，首次调用时计算
func (m *maskedBody) get() string {
	if m == nil {
		return ""
	}
	if !m.done {
		m.val = maskBody(m.raw)
		m.done = true
	}
	return m.val
}
