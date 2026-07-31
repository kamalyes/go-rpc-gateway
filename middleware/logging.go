/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-11
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-23 11:57:37
 * @FilePath: \go-rpc-gateway\middleware\logging.go
 * @Description: 统一日志中间件 - 支持 HTTP 和 gRPC
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	gwconfig "github.com/kamalyes/go-config/pkg/gateway"
	"github.com/kamalyes/go-config/pkg/logging"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/kamalyes/go-toolbox/pkg/matcher"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
	"github.com/kamalyes/go-toolbox/pkg/netx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// getProtojsonOptions 从全局配置 Gateway.JSON 获取 protobuf JSON 序列化选项
// 与 API 响应的序列化配置保持一致，支持配置文件修改和热重载
// 注意：DiscardUnknown 属于反序列化选项，序列化时不适用，故不传入
func getProtojsonOptions() protojson.MarshalOptions {
	if global.GATEWAY != nil && global.GATEWAY.JSON != nil {
		j := global.GATEWAY.JSON
		return protojson.MarshalOptions{
			UseProtoNames:   j.UseProtoNames,
			EmitUnpopulated: j.EmitUnpopulated,
			Multiline:       false,
		}
	}
	// 回退：使用 go-config 的默认 JSON 配置
	d := gwconfig.DefaultJSON()
	return protojson.MarshalOptions{
		UseProtoNames:   d.UseProtoNames,
		EmitUnpopulated: d.EmitUnpopulated,
		Multiline:       false,
	}
}

// RequestLogger 统一的请求日志记录器
type RequestLogger struct {
	config *logging.Logging
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

// Add 添加字符串字段
func (lf *LogFields) Add(key, value string) *LogFields {
	value = mathx.IfEmpty(value, "")
	if key == "" || value == "" {
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
	requestCommonMeta := GetRequestCommonMeta(ctx)

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
func getLoggingConfig() *logging.Logging {
	if global.GATEWAY != nil &&
		global.GATEWAY.Middleware != nil &&
		global.GATEWAY.Middleware.Logging != nil {
		return global.GATEWAY.Middleware.Logging
	}
	return logging.Default()
}

// isLoggableContentType 检查 Content-Type 是否可记录
func isLoggableContentType(config *logging.Logging, contentType string) bool {
	if contentType == "" {
		return true
	}

	contentType = strings.ToLower(contentType)

	for _, prefix := range config.LoggableContentTypes {
		if strings.HasPrefix(contentType, prefix) {
			return true
		}
	}
	return false
}

// isSkipPath 检查是否为跳过路径
// skip 项支持 glob 通配符（* 可匹配 /，? 匹配单字符），无通配符时退化为精确匹配
// 示例："/v1/audit-logs*" 覆盖 "/v1/audit-logs" 与 "/v1/audit-logs/{id}"
func isSkipPath(config *logging.Logging, path string) bool {
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

// LoggingMiddleware HTTP 日志中间件
func LoggingMiddleware() HTTPMiddleware {
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
				wrapped := NewResponseWriter(w)
				next.ServeHTTP(wrapped, r)
				if wrapped.StatusCode() >= 400 {
					logHTTPError(ctx, r, wrapped, time.Since(start))
				}
				wrapped.Release()
				return
			}

			// 捕获请求体：运行时日志开启 或 存在访问日志钩子 时读取，避免访问日志 body 受运行时日志开关影响而丢失
			needCaptureBody := reqConfig.EnableRequest || HasAccessLogHandlers()
			var reqBody []byte
			if needCaptureBody && r.Body != nil {
				var err error
				reqBody, err = io.ReadAll(r.Body)
				if err != nil && global.LOGGER != nil {
					global.LOGGER.ErrorContextKV(ctx, "❌ Failed to read request body",
						"path", r.URL.Path,
						"method", r.Method,
						"error", err)
				}
				r.Body = io.NopCloser(bytes.NewBuffer(reqBody))
			}

			// 包装响应：运行时日志开启 或 存在访问日志钩子 时启用响应体捕获
			wrapped := NewResponseWriter(w)
			if reqConfig.EnableResponse || HasAccessLogHandlers() {
				wrapped.EnableBodyCapture()
			}
			defer wrapped.Release()

			// 执行请求
			next.ServeHTTP(wrapped, r)

			// 记录日志
			logHTTPRequest(ctx, r, wrapped, start, time.Since(start), reqConfig, reqBody)
		})
	}
}

// logHTTPRequest 记录 HTTP 请求
func logHTTPRequest(ctx context.Context, r *http.Request, rw *ResponseWriter, start time.Time, duration time.Duration, config *logging.Logging, reqBody []byte) {
	logger := NewRequestLogger(ctx)
	masker := global.DATAMASKER

	fields := NewLogFields().
		Add(constants.LogFieldMethod, r.Method).
		Add(constants.LogFieldPath, r.URL.Path).
		AddValue(constants.LogFieldStatus, rw.StatusCode()).
		AddValue(constants.LogFieldBytes, rw.BytesWritten()).
		AddValue(constants.LogFieldDuration, duration.Milliseconds()).
		Add(constants.LogFieldIP, netx.GetClientIP(r)).
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
	message := "🚀 " + constants.LogMsgHTTPRequest
	if rw.StatusCode() >= 500 {
		level = constants.LogLevelError
		message = "❌ " + constants.LogMsgHTTPRequest
	} else if rw.StatusCode() >= 400 {
		level = constants.LogLevelWarn
		message = "⚠️ " + constants.LogMsgHTTPRequest
	} else {
		message = "✅ " + constants.LogMsgHTTPRequest
	}

	logger.Log(level, message, fields)

	// 派发访问日志钩子
	if HasAccessLogHandlers() {
		captureAccessLog(ctx, r, rw, start, duration, reqBody, rw.GetBody(), time.Duration(config.SlowHTTPThreshold)*time.Millisecond)
	}
}

// logHTTPError 记录跳过路径的错误 🚫
func logHTTPError(ctx context.Context, r *http.Request, rw *ResponseWriter, duration time.Duration) {
	logger := NewRequestLogger(ctx)
	fields := NewLogFields().
		Add(constants.LogFieldPath, r.URL.Path).
		AddValue(constants.LogFieldStatus, rw.StatusCode()).
		AddValue(constants.LogFieldDuration, duration.Milliseconds()).
		AddRequestContext(ctx)

	logger.Log(constants.LogLevelWarn, "⚠️ "+constants.LogMsgHTTPRequestSkip, fields)
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
	masker := global.DATAMASKER
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
