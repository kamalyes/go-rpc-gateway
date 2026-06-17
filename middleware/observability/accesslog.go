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
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/kamalyes/go-rpc-gateway/constants"
	sqltypes "github.com/kamalyes/go-rpc-gateway/cpool/database/types"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-rpc-gateway/middleware/core"
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

var (
	accessLogMu       sync.RWMutex
	accessLogHandlers []AccessLogHandler
	accessLogCount    atomic.Int32 // 活跃钩子计数，避免 HasAccessLogHandlers 遍历
)

// RegisterAccessLog 注册访问日志钩子，返回反注册函数
func RegisterAccessLog(h AccessLogHandler) (unregister func()) {
	if h == nil {
		return func() {}
	}
	accessLogMu.Lock()
	accessLogHandlers = append(accessLogHandlers, h)
	idx := len(accessLogHandlers) - 1
	accessLogMu.Unlock()
	accessLogCount.Add(1)

	var once sync.Once
	return func() {
		once.Do(func() {
			accessLogMu.Lock()
			accessLogHandlers[idx] = nil
			accessLogMu.Unlock()
			accessLogCount.Add(-1)
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
	accessLogMu.RLock()
	hs := accessLogHandlers
	accessLogMu.RUnlock()

	for _, h := range hs {
		if h == nil {
			continue
		}
		func() {
			defer func() { _ = recover() }()
			h(ctx, rec)
		}()
	}
}

// captureAccessLog 捕获 HTTP 请求/响应快照，构建访问日志记录并派发给已注册钩子
// 仅在 HasAccessLogHandlers() 为 true 时调用，零钩子场景零开销
func captureAccessLog(ctx context.Context, r *http.Request, rw *core.ResponseWriter, start time.Time, duration time.Duration, reqBody, respBody []byte, slowThreshold time.Duration) {
	meta := core.GetRequestCommonMeta(ctx)
	// 走 activeMasker 而不是直接读 global.DATAMASKER：后者只由网关的完整初始化链赋值，
	// 而中间件是导出的、可以被单独使用 activeMasker 在它为 nil 时给一个兜底脱敏器，
	// 这里直接解引用等于把那层保护绕过去
	// [EN] activeMasker rather than a direct read of global.DATAMASKER: that variable is only
	// assigned by the gateway's full init chain while the middlewares are exported and usable on
	// their own. activeMasker supplies a fallback masker when it is nil, and dereferencing the
	// variable here walks straight around that protection.
	masker := activeMasker()

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
		Body:         sqltypes.CompressedText(masker.Mask(reqBody)),
		UserAgent:    r.Header.Get(constants.HeaderUserAgent),
		Source:       mathx.IfEmpty(r.Header.Get(constants.HeaderXSource), meta.Domain),
		StatusCode:   int32(rw.StatusCode()),
		Status:       http.StatusText(rw.StatusCode()),
		Response:     sqltypes.CompressedText(masker.Mask(respBody)),
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

	raw, err := json.Marshal(map[string][]string(header))
	if err != nil {
		return nil
	}

	// 解析目标是 map[string]any 而不是 map[string][]string：敏感键名命中时，脱敏器把
	// 整个值替换成一个掩码字符串，于是 {"Authorization":["Bearer x"]} 出来是
	// {"Authorization":"***REDACTED***"} —— 数组变成了字符串，按 []string 解析会直接失败
	// 失败的后果不是「少脱敏」而是整列丢失，所以这一层形状归一是必须的
	//
	// [EN] The unmarshal target is map[string]any rather than map[string][]string: on a
	// sensitive key the masker replaces the whole value with a mask string, so
	// {"Authorization":["Bearer x"]} comes back as {"Authorization":"***REDACTED***"} — an array
	// turned into a string, which fails outright when parsed as []string. The failure would cost
	// the entire column rather than merely some masking, so normalising the shape is required.
	var masked map[string]any
	if err := json.Unmarshal([]byte(activeHeaderMasker().Mask(raw)), &masked); err != nil {
		return nil
	}

	out := make(map[string][]string, len(masked))
	for name, value := range masked {
		switch v := value.(type) {
		case string:
			out[name] = []string{v}
		case []any:
			values := make([]string, 0, len(v))
			for _, item := range v {
				if s, ok := item.(string); ok {
					values = append(values, s)
				}
			}
			out[name] = values
		}
	}
	return out
}
