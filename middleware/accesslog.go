/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-30 00:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-31 01:15:00
 * @FilePath: \go-rpc-gateway\middleware\accesslog.go
 * @Description: 访问日志 - AccessRecord 模型（gorm tag 供 ClickHouse 自动建表）+ 钩子注册/派发 + HTTP 快照构建
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"context"
	"database/sql/driver"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	sqltypes "github.com/kamalyes/go-sqlbuilder/types"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/netx"
	"gorm.io/gorm"
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

// BeforeCreate gorm 钩子：写入前补全默认值
// 由 BaseRepository.CreateBatch → gorm.CreateInBatches 触发（含异步批量写入路径）
func (m *AccessRecord) BeforeCreate(tx *gorm.DB) error {
	m.ID = mathx.IfEmpty(m.ID, uuid.NewString())
	m.RequestTime = mathx.IfNotZero(m.RequestTime, time.Now())
	return nil
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
func captureAccessLog(ctx context.Context, r *http.Request, rw *ResponseWriter, start time.Time, duration time.Duration, reqBody, respBody []byte, slowThreshold time.Duration) {
	meta := GetRequestCommonMeta(ctx)
	masker := global.DATAMASKER

	serviceName := ""
	if global.GATEWAY != nil {
		serviceName = global.GATEWAY.Name
	}

	rec := &AccessRecord{
		Kind:         AccessLogKindHTTP,
		Service:      serviceName,
		RequestTime:  start,
		ResponseTime: start.Add(duration),
		DurationMS:   duration.Milliseconds(),
		Method:       r.Method,
		Path:         r.URL.Path,
		Query:        r.URL.RawQuery,
		Headers:      sqltypes.CompressedTextMap(r.Header),
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
