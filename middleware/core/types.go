/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 07:49:58
 * @FilePath: \go-rpc-gateway\middleware\core\types.go
 * @Description: 中间件类型定义
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package core

import (
	"net/http"

	"google.golang.org/grpc"
)

// MiddlewareFunc 中间件函数类型
// [EN] MiddlewareFunc is the HTTP middleware function type
type MiddlewareFunc func(http.Handler) http.Handler

// HTTPMiddleware HTTP 中间件接口
// [EN] HTTPMiddleware is the HTTP middleware signature shared by every subpackage
type HTTPMiddleware func(http.Handler) http.Handler

// GRPCInterceptor gRPC 拦截器类型
// [EN] GRPCInterceptor is the gRPC unary server interceptor type
type GRPCInterceptor = grpc.UnaryServerInterceptor

// NoopMiddleware 直通中间件，未启用时复用此变量避免重复创建闭包
// [EN] NoopMiddleware is the pass-through middleware reused when a feature is disabled, avoiding repeated closure allocation
var NoopMiddleware HTTPMiddleware = func(next http.Handler) http.Handler { return next }

// ChainFunc 创建中间件链函数
// [EN] ChainFunc composes the given middlewares into a single middleware chain
func ChainFunc(middlewares ...MiddlewareFunc) MiddlewareFunc {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}
