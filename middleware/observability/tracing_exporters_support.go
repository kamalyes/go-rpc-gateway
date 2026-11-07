//go:build otel

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-05 22:15:32
 * @FilePath: \go-rpc-gateway\middleware\observability\tracing_exporters_support.go
 * @Description: 链路追踪导出器接入（otel 构建标签）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// tracing_exporters_support 仅在启用 otel 构建标签（-tags otel）时编译，
// 提供 zipkin / otlphttp / otlpgrpc 三类导出器。
// otel exporters（含 otlptracegrpc/http 的 gRPC+HTTP 客户端、zipkin-go）
// 约 3MB，自部署无 collector 的服务默认不编译进生产二进制，
// 追踪中间件本身仍可用（span 不上报）

package observability

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	gcmiddleware "github.com/kamalyes/go-rpc-gateway/config/middleware"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/zipkin"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// createExporter 创建导出器
func createExporter(cfg *gcmiddleware.Tracing) (sdktrace.SpanExporter, error) {
	switch cfg.ExporterType {
	case constants.TracingExporterZipkin:
		return zipkin.New(cfg.ExporterEndpoint)
	case constants.TracingExporterOTLPHTTP, constants.TracingExporterOTLP:
		return newOTLPHTTPExporter(cfg)
	case constants.TracingExporterOTLPGRPC:
		return newOTLPGRPCExporter(cfg)
	case constants.TracingExporterConsole, constants.TracingExporterNoop:
		fallthrough
	default:
		return nil, nil
	}
}

// newOTLPHTTPExporter 创建 OTLP HTTP 导出器
// endpoint 支持完整 URL（如 http://host:port/api/default），会自动拆分为 host 与 URL path，
// 并追加 /v1/traces 作为 traces 路径（与 OpenTelemetry Collector otlphttp 行为一致）
func newOTLPHTTPExporter(cfg *gcmiddleware.Tracing) (sdktrace.SpanExporter, error) {
	host, urlPath, schemeIsHTTP := parseOTLPHTTPAddress(cfg.ExporterEndpoint)

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(host),
		otlptracehttp.WithURLPath(urlPath),
	}
	// http 协议或显式配置 insecure 时跳过 TLS
	if schemeIsHTTP || cfg.ExporterTLSInsecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	if len(cfg.ExporterHeaders) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.ExporterHeaders))
	}

	if global.LOGGER != nil {
		global.LOGGER.DebugMsg(fmt.Sprintf("[tracing] OTLP HTTP exporter: endpoint=%s path=%s insecure=%v", host, urlPath, schemeIsHTTP || cfg.ExporterTLSInsecure))
	}

	return otlptracehttp.New(context.Background(), opts...)
}

// newOTLPGRPCExporter 创建 OTLP gRPC 导出器
// endpoint 仅需 host:port（如 127.0.0.1:5081），TLS 由 exporter-tls-insecure 控制
func newOTLPGRPCExporter(cfg *gcmiddleware.Tracing) (sdktrace.SpanExporter, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.ExporterEndpoint),
	}
	if cfg.ExporterTLSInsecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	if len(cfg.ExporterHeaders) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(cfg.ExporterHeaders))
	}

	if global.LOGGER != nil {
		global.LOGGER.DebugMsg(fmt.Sprintf("[tracing] OTLP gRPC exporter: endpoint=%s insecure=%v", cfg.ExporterEndpoint, cfg.ExporterTLSInsecure))
	}

	return otlptracegrpc.New(context.Background(), opts...)
}

// parseOTLPHTTPAddress 将 OTLP HTTP endpoint 解析为 host、urlPath 及是否为 http(明文)协议
// 支持两种输入：
//   - 完整 URL：http://127.0.0.1:30017/api/default → host=127.0.0.1:30017, path=/api/default/v1/traces
//   - host:port：127.0.0.1:30017 → host=127.0.0.1:30017, path=/v1/traces
func parseOTLPHTTPAddress(rawEndpoint string) (host, urlPath string, schemeIsHTTP bool) {
	rawEndpoint = strings.TrimSpace(rawEndpoint)
	if strings.HasPrefix(rawEndpoint, "http://") || strings.HasPrefix(rawEndpoint, "https://") {
		schemeIsHTTP = strings.HasPrefix(rawEndpoint, "http://")
		if u, err := url.Parse(rawEndpoint); err == nil {
			host = u.Host
			urlPath = strings.TrimRight(u.Path, "/")
		}
	}
	if host == "" {
		host = rawEndpoint
	}
	// 确保包含 traces 路径后缀，避免重复拼接
	if urlPath == "" {
		urlPath = "/v1/traces"
	} else if !strings.HasSuffix(urlPath, "/v1/traces") {
		urlPath = urlPath + "/v1/traces"
	}
	return
}
