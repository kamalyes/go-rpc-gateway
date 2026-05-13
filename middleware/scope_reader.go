/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-29 10:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-13 23:26:55
 * @FilePath: \go-rpc-gateway\middleware\scope_reader.go
 * @Description: 从请求上下文读取作用域数据
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package middleware

import "context"

// ContextScopeReader 将 go-rpc-gateway 的请求上下文适配为外部作用域读取接口
type ContextScopeReader struct{}

// GetDomain 从 context 获取 Domain
func (ContextScopeReader) GetDomain(ctx context.Context) string {
	return GetDomain(ctx)
}

// GetTenantID 从 context 获取 TenantID
func (ContextScopeReader) GetTenantID(ctx context.Context) string {
	return GetTenantID(ctx)
}

// GetRoleCode 从 context 获取 RoleCode
func (ContextScopeReader) GetRoleCode(ctx context.Context) string {
	return GetRoleCode(ctx)
}
