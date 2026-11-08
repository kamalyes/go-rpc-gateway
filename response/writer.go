/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 10:11:54
 * @FilePath: \go-rpc-gateway\response\writer.go
 * @Description: HTTP响应写入核心函数
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package response

import (
	"encoding/json"
	"net/http"

	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	commonapis "github.com/kamalyes/go-rpc-gateway/proto"
	"github.com/kamalyes/go-toolbox/pkg/httpx"
)

// encodeJSONTo 把 v 编码进 w
//
// 这里原先挂着一个 sync.Pool 池化 json.Encoder，实测它一点东西都没省：
//
//	encoder := jsonEncoderPool.Get().(*json.Encoder)
//	*encoder = *json.NewEncoder(w)   // 照样 new 了一个，再整体拷贝覆盖池里那个
//
// json.Encoder 的目标 writer 是构造时绑定的、没有重绑接口，所以换目标只能靠
// 「新建一个再按值拷过去」—— 等于每次调用都要走一遍 NewEncoder，
// 池子只是在它之上额外加了一次 Get/Put 与一次结构体拷贝
// 按值拷贝 json.Encoder 还越过了它的 API 契约：标准库从未承诺这个结构体可以拷贝，
// 其内部状态（err、缩进缓冲）的拷贝语义不在任何保证范围内
//
// 既然池化买不到东西，就按最简形式写 实测三种写法在同一个二进制里对比：
//
//	形状    改造前(池+拷贝)   池化 buffer+encoder   本实现(无池)
//	small   242.5 ns/160 B    242.4 ns/160 B        236.0 ns/160 B
//	large   6685 ns/3668 B    6552 ns/3668 B        6738 ns/3666 B
//	        (均 6 / 102 次分配，-benchtime=2s -count=3)
//
// 三者分配次数完全相同，小响应上无池反而略快 —— 说明 NewEncoder 那次分配压根没发生：
// 编译器能看出 *json.NewEncoder(w) 的结果被立即解引用拷贝、不逃逸，于是栈上分配了
// 也就是说这个池子从一开始就在解决一个不存在的问题
// 对应基准见 writer_test.go 的 BenchmarkWriteJSONResponse / Legacy / NoPool
//
// [EN] A sync.Pool of json.Encoder used to sit here and, measured, saved nothing. Encoder
// binds its target writer at construction with no way to rebind, so retargeting meant
// allocating a fresh one and copying it over the pooled one by value — NewEncoder ran on
// every call anyway and the pool merely added a Get/Put round trip and a struct copy on top.
// Copying a json.Encoder by value also steps outside its API contract: the standard library
// never promised the struct is copyable.
//
// Since pooling buys nothing, this is written in its simplest form. Three variants measured
// in one binary (see BenchmarkWriteJSONResponse / Legacy / NoPool in writer_test.go) came out
// at identical allocation counts, with the pool-free form marginally faster on small bodies:
// the NewEncoder allocation never happened in the first place, because the compiler sees that
// the result of *json.NewEncoder(w) is dereferenced and copied immediately without escaping,
// and puts it on the stack. The pool was solving a problem that did not exist.
func encodeJSONTo(w http.ResponseWriter, httpStatus int, v any, errMsg string) {
	w.Header().Set(constants.HeaderContentType, httpx.ContentTypeApplicationJSON)
	w.WriteHeader(httpStatus)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		// global.LOGGER 可能尚未初始化（程序化构造、启动早期、单测），
		// 这里原先裸调 global.LOGGER.WithError 会在错误路径上直接 panic，
		// 把一次「响应编码失败」放大成一次崩溃
		// [EN] global.LOGGER may be uninitialised (programmatic construction, early startup,
		// unit tests); the previous bare call panicked on the error path, turning a failed
		// response encoding into a crash
		if global.LOGGER != nil {
			global.LOGGER.WithError(err).ErrorMsg(errMsg)
		}
	}
}

// WriteResult 写入标准化Result响应
func WriteResult(w http.ResponseWriter, httpStatus int, result *commonapis.Result) {
	encodeJSONTo(w, httpStatus, result, "Failed to encode Result response")
}

// WriteJSONResponse 写入自定义JSON响应
func WriteJSONResponse(w http.ResponseWriter, httpStatus int, data any) {
	encodeJSONTo(w, httpStatus, data, "Failed to encode JSON response")
}
