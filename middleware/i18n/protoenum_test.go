/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-02-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-02-08 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\i18n\protoenum_test.go
 * @Description: protoenum 规则在本包的校验器上确实生效
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package i18n

import (
	"testing"

	"github.com/kamalyes/go-toolbox/pkg/argus/rule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/descriptorpb"
)

// TestProtoEnumRuleReachesTheSharedValidator protoenum 必须对本包的校验器生效
//
// 本包的校验器是一个 sync.Once 的包私有单例，没有导出入口，所以规则只能走 argus
// 的包级规则表注册。这条测试守的就是这条链路：规则表 → 建校验计划 → 单例校验。
//
// 链路断了会怎样：argus 对未知规则名 fail-closed，带 protoenum 标签的字段会一律
// 校验失败。那个方向是刻意选的（立刻全挂比悄悄不校验好），但本测试要确保正常
// 情况下它是通的 —— 否则每个服务的枚举字段都会拒掉合法请求
//
// [EN] TestProtoEnumRuleReachesTheSharedValidator: protoenum must apply to this package's
// validator.
//
// That validator is a sync.Once package-private singleton with no exported accessor, so the
// rule can only arrive through argus's package-level table. This test guards that chain:
// rule table, plan building, singleton validation.
//
// A broken chain fails closed, since argus rejects unknown rule names, and every
// protoenum-tagged field would reject everything. That direction is deliberate — failing
// loudly beats silently not validating — but this test ensures the normal path works, or
// every service's enum fields would reject valid requests
func TestProtoEnumRuleReachesTheSharedValidator(t *testing.T) {
	_, registered := rule.BuiltinRules["protoenum"]
	require.True(t, registered, "protoenum 未注册进 argus 规则表 | the rule is not in argus's table")

	type request struct {
		Kind descriptorpb.FieldDescriptorProto_Type `validate:"required,protoenum"`
	}
	v := getStructTagValidator()

	require.NoError(t, v.Struct(&request{Kind: descriptorpb.FieldDescriptorProto_TYPE_STRING}),
		"合法枚举值被本包的校验器拒绝 | a valid enum value was rejected by this package's validator")
	assert.Error(t, v.Struct(&request{Kind: descriptorpb.FieldDescriptorProto_Type(9999)}),
		"协议里没声明的数字通过了校验 | a number the contract never declared passed validation")
	assert.Error(t, v.Struct(&request{}), "零值通过了 required | the zero value passed required")
}
