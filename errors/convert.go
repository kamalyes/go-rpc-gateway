/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-08 00:00:00
 * @FilePath: \go-rpc-gateway\errors\convert.go
 * @Description: 把转换库的失败收口成本包的错误，不依赖任何具体转换库
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package errors

import "context"

// Converter 一次 Model → PB 转换
//
// 为什么是函数类型而不是接口：Go 的接口方法不能带类型参数，写不出
// `interface { ToPB[M, PB](*M) (*PB, error) }`。泛型函数类型是单方法接口在这里的等价物，
// 由调用方把具体实现传进来，本包因此不需要知道转换库是谁
//
// 这件事的由来：这两个函数先前是 go-bizerror/pbx 子模块，并入时我把它做成了
// go-rpc-gateway/pbx 包，于是网关 go.mod 里多了一条 pbmo 依赖 —— 为了两个
// 各五行的包装函数。改成注入之后，网关对转换库零依赖，调用点还是一行
//
// [EN] One Model-to-PB conversion.
//
// A function type rather than an interface because Go's interface methods cannot take type
// parameters: `interface { ToPB[M, PB](*M) (*PB, error) }` is not expressible. A generic
// function type is the single-method interface's equivalent here, with the caller supplying the
// implementation, so this package never needs to know which conversion library is in use.
//
// How this came about: these two functions were the go-bizerror/pbx submodule, and absorbing
// them as a go-rpc-gateway/pbx package added a pbmo requirement to the gateway's go.mod — for
// two five-line wrappers. Injected instead, the gateway depends on no conversion library and
// the call site is still one line
type Converter[M any, PB any] func(*M) (*PB, error)

// SliceConverter 一次批量 Model → PB 转换
// [EN] One batch Model-to-PB conversion
type SliceConverter[M any, PB any] func([]*M) ([]*PB, error)

// Convert 执行转换，失败时换成 error.conversion_failed 的 gRPC 错误
//
//	func (m *TenantModel) ToPB(ctx context.Context) (*commonpb.TenantInfo, error) {
//	    return errors.Convert(ctx, m, pbmo.ToPB[TenantModel, commonpb.TenantInfo])
//	}
//
// 为什么不把转换库的错误原样返回：那些错误带着字段名、类型名与反射细节，
// 直接返给调用方等于把模型内部结构漏出去，而且不过 i18n —— 客户端拿到的是
// 一句英文的反射错误，不是一条可本地化的业务错误
//
// [EN] Runs the conversion, replacing a failure with the gRPC error for
// error.conversion_failed.
//
// The library's own error is not passed through because it carries field names, type names and
// reflection details: returning one verbatim leaks the model's internals and skips i18n,
// handing the client an English reflection error instead of a localizable business one
func Convert[M any, PB any](ctx context.Context, m *M, conv Converter[M, PB]) (*PB, error) {
	pb, err := conv(m)
	if err != nil {
		return nil, ConversionFailed(ctx)
	}
	return pb, nil
}

// ConvertSlice 批量转换，失败时整批失败
//
// 不跳过坏的那条：部分成功的列表对调用方是更糟的结果 —— 它拿到一个看起来正常、
// 实际少了几条的响应，而少了哪几条无从得知
//
// [EN] Converts a batch, failing the whole batch rather than skipping the bad element: a
// partially successful list is the worse outcome, a response that looks fine while silently
// missing rows with no way to tell which
func ConvertSlice[M any, PB any](ctx context.Context, items []*M, conv SliceConverter[M, PB]) ([]*PB, error) {
	pbs, err := conv(items)
	if err != nil {
		return nil, ConversionFailed(ctx)
	}
	return pbs, nil
}
