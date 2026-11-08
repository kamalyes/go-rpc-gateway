/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-08 00:00:00
 * @FilePath: \go-rpc-gateway\errors\convert_test.go
 * @Description: 转换错误收口测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package errors

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type convModel struct{ Value string }
type convPB struct{ Value string }

func okConv(m *convModel) (*convPB, error) { return &convPB{Value: m.Value}, nil }
func okConvs(ms []*convModel) ([]*convPB, error) {
	out := make([]*convPB, 0, len(ms))
	for _, m := range ms {
		out = append(out, &convPB{Value: m.Value})
	}
	return out, nil
}

// 转换库的原始错误：带字段名与类型名，正是不该外泄的那种
// [EN] The library's raw error, carrying field and type names — exactly what must not leak
func failConv(*convModel) (*convPB, error) {
	return nil, stderrors.New("字段 Value: 期望 *timestamppb.Timestamp 类型")
}
func failConvs([]*convModel) ([]*convPB, error) {
	return nil, stderrors.New("字段 Value 转换失败: 类型不匹配")
}

func TestConvertSuccess(t *testing.T) {
	pb, err := Convert(context.Background(), &convModel{Value: "v"}, okConv)
	require.NoError(t, err)
	require.NotNil(t, pb)
	assert.Equal(t, "v", pb.Value)
}

func TestConvertSliceSuccess(t *testing.T) {
	pbs, err := ConvertSlice(context.Background(),
		[]*convModel{{Value: "a"}, {Value: "b"}}, okConvs)
	require.NoError(t, err)
	require.Len(t, pbs, 2)
	assert.Equal(t, "a", pbs[0].Value)
}

// 失败时必须换成本包的错误，而且不能把转换库的文案带出去
// [EN] A failure must become this package's error, carrying none of the library's text
func TestConvertFailureIsMappedAndDoesNotLeak(t *testing.T) {
	ctx := context.Background()
	for name, call := range map[string]func() error{
		"Convert": func() error {
			_, err := Convert(ctx, &convModel{}, failConv)
			return err
		},
		"ConvertSlice": func() error {
			_, err := ConvertSlice(ctx, []*convModel{{}}, failConvs)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := call()
			require.Error(t, err)

			st, ok := status.FromError(err)
			require.True(t, ok, "必须是 gRPC status 错误 | must be a gRPC status error")
			assert.Equal(t, codes.Internal, st.Code())
			assert.Equal(t, "Data conversion failed", st.Message(),
				"没有 locales 时走内置文案 | the builtin copy applies without locales")

			assert.NotContains(t, st.Message(), "timestamppb",
				"转换库的类型名不能漏给调用方 | the library's type names must not leak")
			assert.NotContains(t, st.Message(), "字段",
				"转换库的字段名不能漏给调用方 | the library's field names must not leak")
		})
	}
}

// 本包不 import 任何转换库：转换函数由调用方注入，这条测试用普通函数验证这一点
// —— 它能过就说明签名不依赖某个具体库
// [EN] This package imports no conversion library: the function is injected, and this test
// passing with a plain function is what shows the signature is library-agnostic
func TestConverterAcceptsAnyMatchingFunction(t *testing.T) {
	var single Converter[convModel, convPB] = okConv
	var batch SliceConverter[convModel, convPB] = okConvs

	pb, err := Convert(context.Background(), &convModel{Value: "z"}, single)
	require.NoError(t, err)
	assert.Equal(t, "z", pb.Value)

	pbs, err := ConvertSlice(context.Background(), []*convModel{{Value: "z"}}, batch)
	require.NoError(t, err)
	require.Len(t, pbs, 1)
}
