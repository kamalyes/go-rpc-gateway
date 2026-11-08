/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-02-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-02-08 00:00:00
 * @FilePath: \go-rpc-gateway\errors\bizmessage_test.go
 * @Description: 内置文案、原因注入与注册表并发安全测试
 *
 * Copyright (c) 2026 by kamalyes 501893067@qq.com, All Rights Reserved.
 */
package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestBuiltinMessage(t *testing.T) {
	assert.Equal(t, "资源未找到", BuiltinMessage("zh", BizErrCodeNotFound))
	assert.Equal(t, "Resource not found", BuiltinMessage("en", BizErrCodeNotFound))
	// 语言为空时按 en | [EN] empty language means en
	assert.Equal(t, "Resource not found", BuiltinMessage("", BizErrCodeNotFound))
	// 未收录的语言回退 en | [EN] an unlisted language falls back to en
	assert.Equal(t, "Resource not found", BuiltinMessage("fr", BizErrCodeNotFound))
	// 未收录的错误码返回空串，由调用方决定兜底
	// [EN] an unlisted code returns empty so the caller decides the fallback
	assert.Empty(t, BuiltinMessage("zh", "error.nobody_wrote_this"))
}

// ctx 里没有 i18n 上下文时回退内置文案，这是「新服务零配置即有可读错误消息」的保证
//
// 在内置表加入之前，这里返回的是 "error.not_found" —— 一个内部标识符，
// 而不是错误消息
//
// [EN] With no i18n context the builtin copy is used; this is what makes a zero-config
// service produce readable errors. Before the table existed this returned the literal
// "error.not_found" — an internal identifier, not a message
func TestNewI18nError_FallsBackToBuiltin(t *testing.T) {
	assert.Equal(t, "Resource not found", NewI18nError(context.Background(), BizErrCodeNotFound))
}

// 内置文案也没有时回退错误码本身，返回值永不为空
// [EN] Without builtin copy either, the code itself is returned — never an empty string
func TestNewI18nError_UnknownCodeKeepsCodeReadable(t *testing.T) {
	const code = "error.totally_unknown"
	assert.Equal(t, code, NewI18nError(context.Background(), code))
}

func TestNewI18nErrorWithTemplate_FillsBuiltinPlaceholder(t *testing.T) {
	msg := NewI18nErrorWithTemplate(context.Background(), BizErrCodeInvalidParameter,
		map[string]interface{}{"reason": "字段 age 必须为正数"})
	// 内置 en 文案是 "Invalid request parameter: {reason}"，占位符应被填充
	// [EN] the builtin en copy carries {reason}, which must be filled
	assert.Contains(t, msg, "字段 age 必须为正数")
	assert.NotContains(t, msg, "{reason}")
}

// 未收录的错误码配模板时仍回退错误码本身，不能回退成带花括号的模板串
// [EN] An unlisted code with template data still falls back to the code, never to a string
// still carrying braces
func TestNewI18nErrorWithTemplate_UnknownCode(t *testing.T) {
	const code = "error.no_builtin_copy"
	msg := NewI18nErrorWithTemplate(context.Background(), code, map[string]interface{}{"reason": "x"})
	assert.Equal(t, code, msg)
}

func TestToGRPCErrorWithCause(t *testing.T) {
	// cause 为 nil 时退化为 ToGRPCError | [EN] a nil cause degrades to ToGRPCError
	plain := ToGRPCErrorWithCause(context.Background(), BizErrCodeNotFound, nil)
	st, _ := status.FromError(plain)
	assert.Equal(t, "Resource not found", st.Message())

	// cause 非 nil 时注入 {reason} | [EN] a non-nil cause fills {reason}
	withCause := ToGRPCErrorWithCause(context.Background(), BizErrCodeInvalidParameter,
		stderrors.New("连接被重置"))
	st2, _ := status.FromError(withCause)
	assert.Equal(t, codes.InvalidArgument, st2.Code())
	assert.Contains(t, st2.Message(), "连接被重置")
}

func TestExtractRpcErrorMsg(t *testing.T) {
	assert.Empty(t, ExtractRpcErrorMsg(nil))
	assert.Equal(t, "boom", ExtractRpcErrorMsg(status.Error(codes.Internal, "boom")))

	// 被 %w 包裹的 status：要拿到内层干净消息，而不是带 "rpc error: code = ..." 信封的串
	// [EN] A status wrapped with %w must yield the inner message, not the enveloped text
	wrapped := fmt.Errorf("调用下游失败: %w", status.Error(codes.NotFound, "缺少用户"))
	assert.Equal(t, "缺少用户", ExtractRpcErrorMsg(wrapped))

	// 非 status 错误回退 error 文案并去空白 | [EN] a plain error falls back to its trimmed text
	assert.Equal(t, "普通错误", ExtractRpcErrorMsg(stderrors.New("  普通错误  ")))
}

// RegisterBizCode 只给 gRPC 码时，网关码必须被推导出来而不是留零值
//
// 留零值等于 ErrCodeOK，那会让一个错误在网关包络里显示为成功
//
// [EN] When only a gRPC code is supplied the gateway code must be derived, not left at its
// zero value — which is ErrCodeOK and would render an error as a success in the envelope
func TestRegisterBizCode_DerivesGatewayCode(t *testing.T) {
	resetRegistry()

	cases := map[codes.Code]ErrorCode{
		codes.InvalidArgument:   ErrCodeBadRequest,
		codes.Unauthenticated:   ErrCodeUnauthorized,
		codes.PermissionDenied:  ErrCodeForbidden,
		codes.NotFound:          ErrCodeNotFound,
		codes.AlreadyExists:     ErrCodeConflict,
		codes.ResourceExhausted: ErrCodeTooManyRequests,
		codes.Unavailable:       ErrCodeServiceUnavailable,
		codes.DeadlineExceeded:  ErrCodeGatewayTimeout,
		codes.Unknown:           ErrCodeInternalServerError,
	}
	for grpcCode, want := range cases {
		bizCode := "error.derive_" + grpcCode.String()
		RegisterBizCode(bizCode, grpcCode)
		assert.Equal(t, grpcCode, MapBizCodeToGRPCCode(bizCode))
		assert.Equal(t, want, MapBizCodeToGatewayCode(bizCode), "gRPC %s 的网关码推导不对", grpcCode)
		assert.NotEqual(t, ErrCodeOK, MapBizCodeToGatewayCode(bizCode), "网关码不能是 OK")
	}
}

// 空错误码不得写入注册表，否则一次手滑的 Register("", ...) 会污染查询
// [EN] An empty code must not enter the registry, or one slipped Register("", ...) poisons lookups
func TestRegister_IgnoresEmptyCode(t *testing.T) {
	resetRegistry()
	Register("", CodeMapping{codes.NotFound, ErrCodeNotFound})

	mappingMu.RLock()
	_, present := mappings[""]
	mappingMu.RUnlock()
	assert.False(t, present)
	assert.Equal(t, codes.Internal, MapBizCodeToGRPCCode(""))
}

// 注册表在 bootstrap 期被写、在每个请求的错误路径上被读
//
// 这个测试要配 -race 跑才有意义：注册表原先是裸 map，并发读写时 Go 运行时直接
// fatal（"concurrent map read and map write"），不是返回错误 —— 整个进程挂掉，
// 而触发条件是「启动还没注册完就来了第一个出错请求」这种只在线上才碰到的时序
//
// [EN] The registry is written during bootstrap and read on every request's error path.
// This test matters under -race: the registry used to be a bare map, and the Go runtime makes
// a concurrent map read/write fatal rather than an error — the whole process dies, triggered
// by a timing only production hits: the first failing request arriving mid-registration
func TestRegistryIsConcurrencySafe(t *testing.T) {
	resetRegistry()

	var wg sync.WaitGroup
	const n = 50

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			Register(fmt.Sprintf("error.concurrent_%d", i),
				CodeMapping{codes.FailedPrecondition, ErrCodeBadRequest})
		}
	}()

	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < n; i++ {
				_ = MapBizCodeToGRPCCode(fmt.Sprintf("error.concurrent_%d", i))
				_ = MapBizCodeToGatewayCode(BizErrCodeNotFound)
			}
		}()
	}

	wg.Wait()
	require.Equal(t, codes.FailedPrecondition, MapBizCodeToGRPCCode("error.concurrent_0"))
}

// 默认映射覆盖全部通用码，且网关码与 gRPC 码一致
// [EN] The defaults cover every common code, with gateway and gRPC codes agreeing
func TestMapping_Defaults(t *testing.T) {
	resetRegistry()

	assert.Equal(t, CodeMapping{codes.NotFound, ErrCodeNotFound}, Mapping(BizErrCodeNotFound))
	assert.Equal(t, CodeMapping{codes.Unauthenticated, ErrCodeUnauthorized}, Mapping(BizErrCodeUnauthenticated))
	assert.Equal(t, CodeMapping{codes.AlreadyExists, ErrCodeConflict}, Mapping(BizErrCodeConflict))
	assert.Equal(t, CodeMapping{codes.Internal, ErrCodeInternalServerError}, Mapping(BizErrCodeInternalError))

	// 未注册的码：_not_found 后缀走 NotFound，其余兜底 Internal
	// [EN] Unregistered: the _not_found suffix yields NotFound, everything else Internal
	assert.Equal(t, CodeMapping{codes.NotFound, ErrCodeNotFound}, Mapping("error.widget_not_found"))
	assert.Equal(t, CodeMapping{codes.Internal, ErrCodeInternalServerError}, Mapping("error.who_knows"))
}
