/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 07:49:58
 * @FilePath: \go-rpc-gateway\middleware\types.go
 * @Description: 中间件类型定义
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"net/http"
)

// MiddlewareFunc 中间件函数类型
type MiddlewareFunc func(http.Handler) http.Handler

// ChainFunc 创建中间件链函数
func ChainFunc(middlewares ...MiddlewareFunc) MiddlewareFunc {
	return func(final http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}
