/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-11
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-11 00:00:00
 * @FilePath: \go-rpc-gateway\constants\metadata.go
 * @Description: gRPC Metadata 和日志字段常量定义
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

// ============================================================================
// gRPC Metadata 键常量
// ============================================================================

// gRPC Metadata 键常量（小写，符合 gRPC metadata 规范）
const (
	MetadataTraceID           = "x-trace-id"
	MetadataRequestID         = "x-request-id"
	MetadataID                = "x-id"
	MetadataUserID            = "x-user-id"
	MetadataTenantID          = "x-tenant-id"
	MetadataTenantCode        = "x-tenant-code"
	MetadataSessionID         = "x-session-id"
	MetadataTimezone          = "x-timezone"
	MetadataIPAddress         = "x-ip-address"
	MetadataAppID             = "x-app-id"
	MetadataDeviceID          = "x-device-id"
	MetadataAppVersion        = "x-app-version"
	MetadataPlatformID        = "x-platform-id"
	MetadataPlatformCode      = "x-platform-code"
	MetadataRegionID          = "x-region-id"
	MetadataRegionCode        = "x-region-code"
	MetadataNonce             = "x-nonce"
	MetadataXNsID             = "x-ns-id"
	MetadataGrpcMetadataXNsID = "grpc-metadata-x-ns-id"
)

// ============================================================================
// 通用日志字段常量（所有中间件共用）
// ============================================================================

// 上下文相关字段
const (
	LogFieldTraceID           = "trace_id"
	LogFieldRequestID         = "request_id"
	LogFieldID                = "id"
	LogFieldUserID            = "user_id"
	LogFieldTenantID          = "tenant_id"
	LogFieldTenantCode        = "tenant_code"
	LogFieldSessionID         = "session_id"
	LogFieldTimezone          = "timezone"
	LogFieldAppID             = "app_id"
	LogFieldDeviceID          = "device_id"
	LogFieldAppVersion        = "app_version"
	LogFieldPlatform          = "platform"
	LogFieldPlatformID        = "platform_id"
	LogFieldPlatformCode      = "platform_code"
	LogFieldRegionID          = "region_id"
	LogFieldRegionCode        = "region_code"
	LogFieldIPAddress         = "ip_address"
	LogFieldNonce             = "nonce"
	LogFieldXNsID             = "x_ns_id"
	LogFieldGrpcMetadataXNsId = "grpc_metadata_x_ns_id"
)

// 请求相关字段
const (
	LogFieldMethod         = "method"
	LogFieldPath           = "path"
	LogFieldQuery          = "query"
	LogFieldRequest        = "request"
	LogFieldResponse       = "response"
	LogFieldUserAgent      = "user_agent"
	LogFieldRemoteAddr     = "remote_addr"
	LogFieldIP             = "ip"
	LogFieldClientIP       = "client_ip"
	LogFieldProtectionPath = "protection_path"
	LogFieldStatusCode     = "status_code"
	LogFieldHTTPMethod     = "http_method"
	LogFieldHTTPPath       = "http_path"
	LogFieldResponseSize   = "response_size"
	LogFieldRequestSize    = "request_size"
	LogFieldSlowRequest    = "slow_request"
	LogFieldReferer        = "referer"
	LogFieldHost           = "host"
	LogFieldProtocol       = "protocol"
	LogFieldScheme         = "scheme"
	LogFieldLatency        = "latency_ms"
	LogFieldClientStream   = "client_stream"
	LogFieldServerStream   = "server_stream"
)

// 性能和状态相关字段
const (
	LogFieldDuration   = "duration_ms"
	LogFieldStatus     = "status"
	LogFieldBytes      = "bytes"
	LogFieldError      = "error"
	LogFieldStackTrace = "stack_trace"
)

// 日志级别常量
const (
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
	LogLevelDebug = "debug"
)

// 日志消息常量
const (
	LogMsgHTTPRequest        = "HTTP Request"
	LogMsgHTTPRequestSkip    = "HTTP Request (Skip Path)"
	LogMsgGRPCRequest        = "gRPC Request"
	LogMsgGRPCRequestError   = "gRPC Request Error"
	LogMsgGRPCStream         = "gRPC Stream"
	LogMsgGRPCStreamError    = "gRPC Stream Error"
	LogMsgPanicRecovered     = "PANIC Recovered"
	LogMsgWriteResponseError = "写入panic响应失败"
)

// 其他常量
const (
	MsgInternalError = "服务器内部错误"
)
