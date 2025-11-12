/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 01:02:08
 * @FilePath: \go-rpc-gateway\middleware\grpc_interceptors.go
 * @Description: gRPC 拦截器管理器
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"runtime/debug"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/validator"
	logger "github.com/kamalyes/go-logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const TraceIDKey = "trace_id"

// InterceptorManager gRPC 拦截器管理器
type InterceptorManager struct {
	logger        *logger.Logger
	serverMetrics *grpc_prometheus.ServerMetrics
	clientMetrics *grpc_prometheus.ClientMetrics
	panicCounter  prometheus.Counter
}

// NewInterceptorManager 创建拦截器管理器
func NewInterceptorManager(
	log *logger.Logger,
	serverMetrics *grpc_prometheus.ServerMetrics,
	clientMetrics *grpc_prometheus.ClientMetrics,
	panicCounter prometheus.Counter,
) *InterceptorManager {
	return &InterceptorManager{
		logger:        log,
		serverMetrics: serverMetrics,
		clientMetrics: clientMetrics,
		panicCounter:  panicCounter,
	}
}

// ServerOptions 返回服务器拦截器选项
func (im *InterceptorManager) ServerOptions() []grpc.ServerOption {
	// 创建日志拦截器
	loggingInterceptor := interceptorLogger(im.logger)

	// 创建恢复处理器
	recoveryHandler := im.panicRecoveryHandler()

	// Exemplar 提取器（关联 trace 和 metrics）
	exemplarFromContext := func(ctx context.Context) prometheus.Labels {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return prometheus.Labels{TraceIDKey: span.TraceID().String()}
		}
		return nil
	}

	return []grpc.ServerOption{
		// OpenTelemetry 追踪
		grpc.StatsHandler(otelgrpc.NewServerHandler()),

		// Unary 拦截器链
		grpc.ChainUnaryInterceptor(
			// 1. Metrics（最外层，记录所有请求）
			im.serverMetrics.UnaryServerInterceptor(
				grpc_prometheus.WithExemplarFromContext(exemplarFromContext),
			),

			// 2. Validator（验证请求）
			grpc_validator.UnaryServerInterceptor(),

			// 3. Logging（记录请求日志）
			grpc_logging.UnaryServerInterceptor(
				loggingInterceptor,
				grpc_logging.WithFieldsFromContext(logTraceID),
			),

			// 4. Recovery（最内层，捕获 panic）
			grpc_recovery.UnaryServerInterceptor(
				grpc_recovery.WithRecoveryHandler(recoveryHandler),
			),
		),

		// Stream 拦截器链
		grpc.ChainStreamInterceptor(
			// 1. Metrics
			im.serverMetrics.StreamServerInterceptor(
				grpc_prometheus.WithExemplarFromContext(exemplarFromContext),
			),

			// 2. Validator
			grpc_validator.StreamServerInterceptor(),

			// 3. Logging
			grpc_logging.StreamServerInterceptor(
				loggingInterceptor,
				grpc_logging.WithFieldsFromContext(logTraceID),
			),

			// 4. Recovery
			grpc_recovery.StreamServerInterceptor(
				grpc_recovery.WithRecoveryHandler(recoveryHandler),
			),
		),
	}
}

// ClientDialOptions 返回客户端拨号选项
func (im *InterceptorManager) ClientDialOptions() []grpc.DialOption {
	loggingInterceptor := interceptorLogger(im.logger)

	exemplarFromContext := func(ctx context.Context) prometheus.Labels {
		if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
			return prometheus.Labels{TraceIDKey: span.TraceID().String()}
		}
		return nil
	}

	return []grpc.DialOption{
		// OpenTelemetry 追踪
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),

		// Unary 拦截器链
		grpc.WithChainUnaryInterceptor(
			// 1. Metrics
			im.clientMetrics.UnaryClientInterceptor(
				grpc_prometheus.WithExemplarFromContext(exemplarFromContext),
			),

			// 2. Validator
			grpc_validator.UnaryClientInterceptor(),

			// 3. Logging
			grpc_logging.UnaryClientInterceptor(
				loggingInterceptor,
				grpc_logging.WithFieldsFromContext(logTraceID),
			),
		),

		// Stream 拦截器链
		grpc.WithChainStreamInterceptor(
			// 1. Metrics
			im.clientMetrics.StreamClientInterceptor(
				grpc_prometheus.WithExemplarFromContext(exemplarFromContext),
			),

			// 2. Logging
			grpc_logging.StreamClientInterceptor(
				loggingInterceptor,
				grpc_logging.WithFieldsFromContext(logTraceID),
			),
		),
	}
}

// panicRecoveryHandler 创建 panic 恢复处理器
func (im *InterceptorManager) panicRecoveryHandler() grpc_recovery.RecoveryHandlerFunc {
	return func(p any) error {
		// 增加 panic 计数器
		if im.panicCounter != nil {
			im.panicCounter.Inc()
		}

		// 记录 panic 详情
		im.logger.ErrorKV("recovered from panic",
			"panic", p,
			"stack", string(debug.Stack()))

		// 返回内部错误
		return status.Errorf(codes.Internal, "internal server error: %v", p)
	}
}

// logTraceID 从上下文中提取 trace ID 用于日志
var logTraceID = func(ctx context.Context) grpc_logging.Fields {
	if span := trace.SpanContextFromContext(ctx); span.IsSampled() {
		return grpc_logging.Fields{TraceIDKey, span.TraceID().String()}
	}
	return nil
}

// interceptorLogger 创建拦截器日志器
func interceptorLogger(l *logger.Logger) grpc_logging.Logger {
	return grpc_logging.LoggerFunc(func(ctx context.Context, lvl grpc_logging.Level, msg string, fields ...any) {
		switch lvl {
		case grpc_logging.LevelDebug:
			l.WithContext(ctx).DebugKV(msg, fields...)
		case grpc_logging.LevelInfo:
			l.WithContext(ctx).InfoKV(msg, fields...)
		case grpc_logging.LevelWarn:
			l.WithContext(ctx).WarnKV(msg, fields...)
		case grpc_logging.LevelError:
			l.WithContext(ctx).ErrorKV(msg, fields...)
		default:
			l.WithContext(ctx).InfoKV(msg, fields...)
		}
	})
}
