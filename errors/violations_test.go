/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-02-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-02-08 00:00:00
 * @FilePath: \go-rpc-gateway\errors\violations_test.go
 * @Description: 违规项收集器的测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package errors

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestViolationsCollectsInsteadOfFailingFast 收集而不是遇错即返
//
// 一次提交里有三处填错时，逐条返回会让使用者改一处、提交一次、再被拒一次。
// 这条测试钉住「一整轮问题一次说完」这个行为
//
// [EN] TestViolationsCollectsInsteadOfFailingFast: collect rather than fail fast.
//
// With three mistakes in one submission, returning one at a time makes the user fix one,
// submit, and be rejected again. This pins the behaviour of stating a whole round at once
func TestViolationsCollectsInsteadOfFailingFast(t *testing.T) {
	v := NewViolations()
	assert.False(t, v.Has(), "新建的收集器不该有违规 | a fresh collector holds nothing")

	v.Add("name", "required", "名称必填")
	v.Add("price", "gt", "价格必须大于 0")
	v.Addf("tiers", 2, "minAmount", "gte", "下限不能小于上一档")

	require.True(t, v.Has())
	items := v.Items()
	require.Len(t, items, 3, "三处问题应当一次收齐 | all three should be collected at once")
	assert.Equal(t, "tiers[2].minAmount", items[2].Field,
		"集合内的字段路径要带下标，前端才能定位到具体那一行 | an indexed path is how the frontend finds the row")
}

// TestIndexedFieldFormatIsStable 字段路径格式必须稳定
//
// 前端按这个路径定位到具体那一行的那个输入框。格式一变，所有按它定位的界面
// 一起失效，而后端这边没有任何报错
//
// [EN] TestIndexedFieldFormatIsStable: the field path format has to stay stable.
//
// The frontend locates one input in one row by this path. Change the format and every UI
// relying on it breaks at once, with nothing on the backend to report it
func TestIndexedFieldFormatIsStable(t *testing.T) {
	assert.Equal(t, "tiers[0].minAmount", IndexedField("tiers", 0, "minAmount"))
	assert.Equal(t, "items[12].qty", IndexedField("items", 12, "qty"))
}

// TestErrReturnsABizError Err 返回的是走本包构造的业务错误
//
// 不自己拼 status：本仓库的错误统一由 ToGRPCError 构造、由网关包络。绕过它
// 拼出来的错误不会被 i18n 与脱敏中间件处理到，而同一个接口会有两种错误形状
//
// [EN] TestErrReturnsABizError: Err returns an error built by this package.
//
// It does not hand-roll a status: this repository's errors are built by ToGRPCError and
// enveloped by the gateway. One built around it would bypass the i18n and redaction
// middleware, and the endpoint would have two error shapes
func TestErrReturnsABizError(t *testing.T) {
	v := NewViolations()
	v.Add("name", "required", "名称必填")

	// 全局日志器未初始化时也必须能用：单元测试与「只用本包、不起整个网关」的
	// 场景都没有它，而那时 panic 会把一个本该返回错误的调用变成崩溃
	// [EN] It must work with no global logger initialized: unit tests and setups that use
	// this package without starting the gateway have none, and panicking there would turn a
	// call that should return an error into a crash
	err := v.Err(context.Background(), "error.validation.failed")
	require.Error(t, err, "有违规时必须返回错误 | an error is required when violations exist")
}
