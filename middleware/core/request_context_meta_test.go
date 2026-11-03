/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-03 18:02:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-03 18:20:00
 * @FilePath: \\go-rpc-gateway\\middleware\\core\\request_context_meta_test.go
 * @Description: WithRequestCommonMeta 单层 context 同时应答两个键的测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package core

import (
	"context"
	"testing"

	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-toolbox/pkg/contextx"
	"github.com/stretchr/testify/assert"
)

// TestWithRequestCommonMetaServesTraceIDKey 合并成一层后，go-logger 的 trace_id 兜底键必须照旧可读
//
// 改造前是两层 context：一层存 meta，一层单独存 logger.ContextKeyTraceID
// 合并成 requestMetaContext 后 trace_id 改为从 meta 现算，这个等价关系没有测试就会悄悄失效 ——
// 失效的表现是日志里 trace_id 变空，而所有现有测试照样绿
//
// [EN] After merging into one layer, go-logger's trace id fallback key must still resolve
// This equivalence is what the merge rests on, and without a test it can break silently:
// the symptom is an empty trace_id in logs while every existing test stays green
func TestWithRequestCommonMetaServesTraceIDKey(t *testing.T) {
	meta := &RequestCommonMeta{TraceID: "trace-xyz", RequestID: "req-xyz"}
	ctx := WithRequestCommonMeta(context.Background(), meta)

	assert.Equal(t, "trace-xyz", contextx.GetValue[string](ctx, logger.ContextKeyTraceID),
		"go-logger 的兜底键读不到 trace_id")
	assert.Same(t, meta, GetRequestCommonMeta(ctx), "meta 自身的键读不到")

	// 外层单独覆盖 trace_id 时，外层应当赢（与两层写法的顺序语义一致）
	// [EN] An outer layer overriding the trace id must win, as it did with two layers
	outer := contextx.WithValue(ctx, logger.ContextKeyTraceID, "override")
	assert.Equal(t, "override", contextx.GetValue[string](outer, logger.ContextKeyTraceID))

	// 其余键照常穿透到父 context
	// [EN] Other keys still pass through to the parent
	parent := contextx.WithValue(context.Background(), "unrelated", "kept")
	assert.Equal(t, "kept", contextx.GetValue[string](WithRequestCommonMeta(parent, meta), "unrelated"))
}

// TestWithRequestCommonMetaNilSafe nil 的 ctx 与 nil 的 meta 都不得 panic
// [EN] A nil ctx and a nil meta must both be safe
func TestWithRequestCommonMetaNilSafe(t *testing.T) {
	assert.NotPanics(t, func() {
		//nolint:staticcheck // 刻意传 nil ctx，验证兜底
		ctx := WithRequestCommonMeta(nil, &RequestCommonMeta{TraceID: "t"})
		assert.Equal(t, "t", contextx.GetValue[string](ctx, logger.ContextKeyTraceID))
	})
	assert.NotPanics(t, func() {
		// 用 Equal 而不是 Same：context.Background() 返回的是结构体值而非指针，Same 只收指针
		// [EN] Equal rather than Same: context.Background() is a struct value, not a pointer
		base := context.Background()
		assert.Equal(t, base, WithRequestCommonMeta(base, nil), "meta 为 nil 时应原样返回父 context")
	})
}

// TestRequestMetaContextUncomparableKey context 的键允许是不可比较类型，查找不得 panic
// 用 switch key 做 == 比较会在这里崩，用类型断言不会
//
// [EN] Context keys may be uncomparable and lookup must not panic
// A switch comparing keys with == crashes here; type assertions do not
func TestRequestMetaContextUncomparableKey(t *testing.T) {
	type uncomparable struct{ fn func() }
	ctx := WithRequestCommonMeta(context.Background(), &RequestCommonMeta{TraceID: "t"})
	assert.NotPanics(t, func() {
		assert.Nil(t, ctx.Value(uncomparable{fn: func() {}}))
	})
}
