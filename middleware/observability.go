/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 15:05:06
 * @FilePath: \go-rpc-gateway\middleware\observability.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// HTTPMiddleware HTTP 中间件接口
type HTTPMiddleware func(http.Handler) http.Handler

// GRPCInterceptor gRPC 拦截器类型
type GRPCInterceptor = grpc.UnaryServerInterceptor

// HTTPMetricsMiddleware HTTP 监控中间件
func HTTPMetricsMiddleware(metricsManager *MetricsManager) HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		if metricsManager == nil {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// 处理请求
			next.ServeHTTP(w, r)

			// 记录指标
			duration := time.Since(start)
			metricsManager.RecordHTTPRequest(duration)
		})
	}
}

// HTTPTracingMiddleware HTTP 链路追踪中间件
func HTTPTracingMiddleware(tracingManager *TracingManager) HTTPMiddleware {
	return func(next http.Handler) http.Handler {
		if tracingManager == nil || tracingManager.GetTracer() == nil {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := tracingManager.GetTracer().Start(r.Context(), fmt.Sprintf("%s %s", r.Method, r.URL.Path))
			defer span.End()

			// 添加属性
			span.SetAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("http.user_agent", r.UserAgent()),
			)

			// 处理请求
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GRPCMetricsInterceptor gRPC 监控拦截器
func GRPCMetricsInterceptor(metricsManager *MetricsManager) GRPCInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if metricsManager == nil {
			return handler(ctx, req)
		}

		start := time.Now()

		// 处理请求
		resp, err := handler(ctx, req)

		// 记录指标
		duration := time.Since(start)
		metricsManager.RecordGRPCRequest(duration)

		return resp, err
	}
}

// GRPCTracingInterceptor gRPC 链路追踪拦截器
func GRPCTracingInterceptor(tracingManager *TracingManager) GRPCInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if tracingManager == nil || tracingManager.GetTracer() == nil {
			return handler(ctx, req)
		}

		ctx, span := tracingManager.GetTracer().Start(ctx, info.FullMethod)
		defer span.End()

		// 添加属性
		span.SetAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", info.FullMethod),
		)

		// 从 metadata 中获取额外信息
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if userAgent := md.Get("user-agent"); len(userAgent) > 0 {
				span.SetAttributes(attribute.String("user_agent", userAgent[0]))
			}
		}

		// 处理请求
		return handler(ctx, req)
	}
}
