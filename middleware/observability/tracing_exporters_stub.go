//go:build !otel

/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-05 22:15:32
 * @FilePath: \go-rpc-gateway\middleware\observability\tracing_exporters_stub.go
 * @Description: 链路追踪导出器空实现
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

// tracing_exporters_stub 在未启用 otel 构建标签时提供 createExporter 的空实现，
// zipkin / otlptracehttp / otlptracegrpc 导出器不进二进制；
// 追踪中间件本身仍可用（provider 无 batcher，span 采样但不导出）
// 需要导出器支持时使用 -tags otel 构建

package observability

import (
	"fmt"

	gcmiddleware "github.com/kamalyes/go-rpc-gateway/config/middleware"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// createExporter 追踪导出器未编译进二进制
// 配置了 zipkin/otlphttp/otlpgrpc 时退化为无导出器（与 console/noop 同路径）
func createExporter(cfg *gcmiddleware.Tracing) (sdktrace.SpanExporter, error) {
	if cfg.Enabled &&
		cfg.ExporterType != constants.TracingExporterConsole &&
		cfg.ExporterType != constants.TracingExporterNoop &&
		global.LOGGER != nil {
		global.LOGGER.WarnMsg(fmt.Sprintf("[tracing] exporter %s not compiled in; rebuild with -tags otel to enable exporters", cfg.ExporterType))
	}
	return nil, nil
}
