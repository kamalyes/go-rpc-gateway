/*
 * @Author: kamalyes 501893@qq.com
 * @Date: 2026-06-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-12 20:01:58
 * @FilePath: \go-rpc-gateway\errors\biz_test.go
 * @Description: 业务错误码映射测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package errors

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// resetRegistry 重置注册表（测试用）
func resetRegistry() {
	bizCodeRegistry = make(map[string]codes.Code)
}

func TestRegisterBizCode(t *testing.T) {
	resetRegistry()

	RegisterBizCode("error.item_not_available", codes.FailedPrecondition)

	assert.Equal(t, codes.FailedPrecondition, MapBizCodeToGRPCCode("error.item_not_available"))
}

func TestRegisterBizCodeMap(t *testing.T) {
	resetRegistry()

	RegisterBizCodeMap(map[string]codes.Code{
		"error.item_conflict":  codes.AlreadyExists,
		"error.item_forbidden": codes.PermissionDenied,
		"error.item_bad_input": codes.InvalidArgument,
	})

	assert.Equal(t, codes.AlreadyExists, MapBizCodeToGRPCCode("error.item_conflict"))
	assert.Equal(t, codes.PermissionDenied, MapBizCodeToGRPCCode("error.item_forbidden"))
	assert.Equal(t, codes.InvalidArgument, MapBizCodeToGRPCCode("error.item_bad_input"))
}

func TestMapBizCodeToGRPCCode_NotFoundSuffix(t *testing.T) {
	resetRegistry()

	// 内置规则：_not_found 后缀自动映射为 NotFound
	assert.Equal(t, codes.NotFound, MapBizCodeToGRPCCode("error.user_not_found"))
	assert.Equal(t, codes.NotFound, MapBizCodeToGRPCCode("error.order_not_found"))
	assert.Equal(t, codes.NotFound, MapBizCodeToGRPCCode("error.resource_not_found"))
}

func TestMapBizCodeToGRPCCode_RegistryOverridesSuffix(t *testing.T) {
	resetRegistry()

	// 注册表优先级高于内置规则
	RegisterBizCode("error.item_not_found", codes.InvalidArgument)
	assert.Equal(t, codes.InvalidArgument, MapBizCodeToGRPCCode("error.item_not_found"))
}

func TestMapBizCodeToGRPCCode_FallbackInternal(t *testing.T) {
	resetRegistry()

	// 未注册且不匹配内置规则，兜底 Internal
	assert.Equal(t, codes.Internal, MapBizCodeToGRPCCode("error.unknown_error"))
	assert.Equal(t, codes.Internal, MapBizCodeToGRPCCode("error.something_went_wrong"))
}

func TestToGRPCError(t *testing.T) {
	resetRegistry()

	RegisterBizCode("error.item_bad_input", codes.InvalidArgument)

	ctx := context.Background()
	err := ToGRPCError(ctx, "error.item_bad_input")

	assert.NotNil(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestToGRPCError_NotFoundSuffix(t *testing.T) {
	resetRegistry()

	ctx := context.Background()
	err := ToGRPCError(ctx, "error.item_not_found")

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestToGRPCError_Fallback(t *testing.T) {
	resetRegistry()

	ctx := context.Background()
	err := ToGRPCError(ctx, "error.unknown_error")

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestToGRPCErrorWithTemplate(t *testing.T) {
	resetRegistry()

	RegisterBizCode("error.item_not_found", codes.NotFound)

	ctx := context.Background()
	err := ToGRPCErrorWithTemplate(ctx, "error.item_not_found", map[string]interface{}{
		"name": "test-item",
	})

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestNewI18nError(t *testing.T) {
	resetRegistry()

	ctx := context.Background()
	// 无 i18n 翻译时返回 key 本身
	msg := NewI18nError(ctx, "error.test_code")
	assert.Equal(t, "error.test_code", msg)
}

func TestNewI18nErrorWithTemplate(t *testing.T) {
	resetRegistry()

	ctx := context.Background()
	msg := NewI18nErrorWithTemplate(ctx, "error.test_code", map[string]interface{}{
		"key": "value",
	})
	assert.Equal(t, "error.test_code", msg)
}

func TestExtractRpcErrorMsg_WithJSON(t *testing.T) {
	result := ExtractRpcErrorMsg(`rpc error: code = NotFound desc = {"code":5,"msg":"资源未找到"}`)
	assert.Equal(t, "资源未找到", result)
}

func TestExtractRpcErrorMsg_WithEmptyMsg(t *testing.T) {
	result := ExtractRpcErrorMsg(`rpc error: code = Internal desc = {"code":13,"msg":""}`)
	assert.Equal(t, `rpc error: code = Internal desc = {"code":13,"msg":""}`, result)
}

func TestExtractRpcErrorMsg_NoJSON(t *testing.T) {
	result := ExtractRpcErrorMsg("simple error message")
	assert.Equal(t, "simple error message", result)
}

func TestExtractRpcErrorMsg_InvalidJSON(t *testing.T) {
	result := ExtractRpcErrorMsg(`rpc error: {invalid json}`)
	assert.Equal(t, `rpc error: {invalid json}`, result)
}

func TestExtractRpcErrorMsg_EmptyString(t *testing.T) {
	result := ExtractRpcErrorMsg("")
	assert.Equal(t, "", result)
}

func TestExtractRpcErrorMsg_NoBraces(t *testing.T) {
	result := ExtractRpcErrorMsg("no braces here")
	assert.Equal(t, "no braces here", result)
}

func TestRegisterBizCodeMap_Override(t *testing.T) {
	resetRegistry()

	RegisterBizCode("error.override_test", codes.InvalidArgument)
	assert.Equal(t, codes.InvalidArgument, MapBizCodeToGRPCCode("error.override_test"))

	// 后注册覆盖先注册
	RegisterBizCode("error.override_test", codes.PermissionDenied)
	assert.Equal(t, codes.PermissionDenied, MapBizCodeToGRPCCode("error.override_test"))
}

func TestMapBizCodeToGRPCCode_EmptyString(t *testing.T) {
	resetRegistry()

	assert.Equal(t, codes.Internal, MapBizCodeToGRPCCode(""))
}
