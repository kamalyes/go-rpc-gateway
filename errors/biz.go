/*
 * @Author: kamalyes 501893@qq.com
 * @Date: 2026-06-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-12 13:15:28
 * @FilePath: \go-rpc-gateway\errors\biz.go
 * @Description: 业务错误码到 gRPC 状态码 / 网关错误码的可注册映射
 *
 * 各微服务在 bootstrap 阶段调用 RegisterMany 注册自己的映射规则，
 * 运行时通过 ToGRPCError / Mapping 自动查找。
 * 内置通用规则：_not_found 后缀自动映射为 NotFound。
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package errors

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"strings"
	"sync"

	goi18n "github.com/kamalyes/go-toolbox/pkg/i18n"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//===========================  错误码映射 | Code Mapping  ==========================//

// CodeMapping 一个业务错误码对应的 gRPC 状态码与网关错误码
//
// 两者一起注册而不是分两张表：同一个业务码的 gRPC 码和网关码是一个决定的两面，
// 分开存就会出现只改了一半的状态，而那种不一致只在线上某个具体接口上才看得见
//
// [EN] The gRPC status code and gateway code one business code maps to.
//
// They are registered together rather than in two tables because they are two faces of one
// decision; splitting them allows a half-updated state that only shows up on one live endpoint
type CodeMapping struct {
	GRPC    codes.Code // gRPC 状态码 | [EN] gRPC status code
	Gateway ErrorCode  // 网关错误码 | [EN] gateway error code
}

var (
	// 读写锁不是可选的：RegisterMany 在 bootstrap 期写入，而 MapBizCodeToGRPCCode
	// 在每个请求的错误路径上读。裸 map 在这两者重叠时是 data race，
	// 且 Go 运行时对并发读写 map 是直接 fatal，不是返回错误
	// [EN] The lock is not optional: RegisterMany writes during bootstrap while
	// MapBizCodeToGRPCCode reads on every request's error path. A bare map is a data race
	// there, and the Go runtime makes concurrent map read/write a fatal error, not an error value
	mappingMu sync.RWMutex
	mappings  = map[string]CodeMapping{}
)

func init() {
	registerDefaults()
}

// registerDefaults 装入通用错误码的默认映射，新服务零注册即可用
//
// 单独成函数而不是写在 init 里：测试需要清空注册表验证兜底规则，
// 清空后必须能把默认值放回去，否则后续测试看到的是一张空表
//
// [EN] Separate from init because tests clear the registry to exercise the fallback rules and
// must be able to put the defaults back; otherwise later tests see an empty table
func registerDefaults() {
	RegisterMany(map[string]CodeMapping{
		BizErrCodeInternalError:            {codes.Internal, ErrCodeInternalServerError},
		BizErrCodeInvalidArgument:          {codes.InvalidArgument, ErrCodeBadRequest},
		BizErrCodeValidationFailed:         {codes.InvalidArgument, ErrCodeBadRequest},
		BizErrCodeInvalidParameter:         {codes.InvalidArgument, ErrCodeBadRequest},
		BizErrCodeUnauthenticated:          {codes.Unauthenticated, ErrCodeUnauthorized},
		BizErrCodePermissionDenied:         {codes.PermissionDenied, ErrCodeForbidden},
		BizErrCodeNotFound:                 {codes.NotFound, ErrCodeNotFound},
		BizErrCodeConflict:                 {codes.AlreadyExists, ErrCodeConflict},
		BizErrCodeCannotBeSelf:             {codes.FailedPrecondition, ErrCodeBadRequest},
		BizErrCodeUnsupportedOperation:     {codes.Unimplemented, ErrCodeInternalServerError},
		BizErrCodeDatabaseError:            {codes.Internal, ErrCodeInternalServerError},
		BizErrCodeDatabaseOperationFailed:  {codes.Internal, ErrCodeInternalServerError},
		BizErrCodeConversionFailed:         {codes.Internal, ErrCodeInternalServerError},
		BizErrCodeExternalServiceError:     {codes.Internal, ErrCodeInternalServerError},
		BizErrCodeRepositoryNotInitialized: {codes.Internal, ErrCodeInternalServerError},
	})
}

// Register 注册单个业务错误码映射（幂等，后注册覆盖先注册；空错误码忽略）
// [EN] Registers one mapping; idempotent, last write wins, empty code ignored
func Register(bizCode string, mapping CodeMapping) {
	if bizCode == "" {
		return
	}
	mappingMu.Lock()
	defer mappingMu.Unlock()
	mappings[bizCode] = mapping
}

// RegisterMany 批量注册业务错误码映射（服务专属码在启动期集中注入）
// [EN] Registers a batch of mappings; a service injects its own codes at startup
func RegisterMany(batch map[string]CodeMapping) {
	for bizCode, mapping := range batch {
		Register(bizCode, mapping)
	}
}

// RegisterBizCode 只注册 gRPC 状态码，网关码按 gRPC 码推导
//
// 保留这个入口是为了不打断既有调用方；新代码用 Register，它能把网关码一并说清
// [EN] Kept so existing callers keep compiling; new code uses Register, which states the
// gateway code too
func RegisterBizCode(bizCode string, grpcCode codes.Code) {
	Register(bizCode, CodeMapping{GRPC: grpcCode, Gateway: gatewayCodeForGRPC(grpcCode)})
}

// RegisterBizCodeMap 批量注册 gRPC 状态码，网关码按 gRPC 码推导
// [EN] Batch form of RegisterBizCode
func RegisterBizCodeMap(m map[string]codes.Code) {
	for bizCode, grpcCode := range m {
		RegisterBizCode(bizCode, grpcCode)
	}
}

// Mapping 查询错误码映射：显式注册优先，其次 _not_found 后缀规则，兜底 Internal
// [EN] Explicit registration wins, then the _not_found suffix rule, then Internal
func Mapping(bizCode string) CodeMapping {
	mappingMu.RLock()
	mapping, ok := mappings[bizCode]
	mappingMu.RUnlock()
	if ok {
		return mapping
	}
	if bizCode == BizErrCodeNotFound || strings.HasSuffix(bizCode, "_not_found") {
		return CodeMapping{codes.NotFound, ErrCodeNotFound}
	}
	return CodeMapping{codes.Internal, ErrCodeInternalServerError}
}

// MapBizCodeToGRPCCode 业务错误码映射为 gRPC 状态码
// [EN] Maps a business code to a gRPC status code
func MapBizCodeToGRPCCode(bizCode string) codes.Code {
	return Mapping(bizCode).GRPC
}

// MapBizCodeToGatewayCode 业务错误码映射为网关错误码
// [EN] Maps a business code to a gateway error code
func MapBizCodeToGatewayCode(bizCode string) ErrorCode {
	return Mapping(bizCode).Gateway
}

// gatewayCodeForGRPC 由 gRPC 状态码推导网关错误码，供只给了 gRPC 码的注册入口使用
// [EN] Derives a gateway code from a gRPC one, for the registration entry points that only
// supply the latter
func gatewayCodeForGRPC(grpcCode codes.Code) ErrorCode {
	switch grpcCode {
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange:
		return ErrCodeBadRequest
	case codes.Unauthenticated:
		return ErrCodeUnauthorized
	case codes.PermissionDenied:
		return ErrCodeForbidden
	case codes.NotFound:
		return ErrCodeNotFound
	case codes.AlreadyExists, codes.Aborted:
		return ErrCodeConflict
	case codes.ResourceExhausted:
		return ErrCodeTooManyRequests
	case codes.Unavailable:
		return ErrCodeServiceUnavailable
	case codes.DeadlineExceeded:
		return ErrCodeGatewayTimeout
	default:
		return ErrCodeInternalServerError
	}
}

//===========================  错误构造 | Error Construction  ==========================//

// ToGRPCError 将业务错误码转换为携带 i18n 消息的 gRPC 错误
// [EN] Turns a business code into a gRPC error carrying its i18n message
func ToGRPCError(ctx context.Context, bizCode string) error {
	return status.Error(MapBizCodeToGRPCCode(bizCode), resolveI18nMessage(ctx, bizCode))
}

// ToGRPCErrorWithTemplate 将业务错误码 + 模板变量转换为 gRPC 错误
// [EN] Same, with template data filled into the message's placeholders
func ToGRPCErrorWithTemplate(ctx context.Context, bizCode string, templateData map[string]interface{}) error {
	return status.Error(MapBizCodeToGRPCCode(bizCode), resolveI18nMessageWithTemplate(ctx, bizCode, templateData))
}

// ToGRPCErrorWithCause 将业务错误码转换为 gRPC 错误，并把底层错误信息注入 {reason} 占位
//
// cause 为 nil 时退化为 ToGRPCError —— 调用方不必自己判空，
// 否则「包装一个可能为 nil 的 err」这件事会在每个调用点重写一遍
//
// [EN] Injects the underlying error's text into the {reason} placeholder, degrading to
// ToGRPCError when cause is nil so callers do not rewrite that nil check at every call site
func ToGRPCErrorWithCause(ctx context.Context, bizCode string, cause error) error {
	if cause == nil {
		return ToGRPCError(ctx, bizCode)
	}
	return ToGRPCErrorWithTemplate(ctx, bizCode, map[string]interface{}{"reason": cause.Error()})
}

// NewI18nError 按错误码解析当前请求语言的消息文案
// [EN] Resolves the message for a code in the request's language
func NewI18nError(ctx context.Context, bizCode string) string {
	return resolveI18nMessage(ctx, bizCode)
}

// NewI18nErrorWithTemplate 按错误码 + 模板变量解析消息文案
// [EN] Same, with template data
func NewI18nErrorWithTemplate(ctx context.Context, bizCode string, templateData map[string]interface{}) string {
	return resolveI18nMessageWithTemplate(ctx, bizCode, templateData)
}

//===========================  文案解析 | Message Resolution  ==========================//

// resolveI18nMessage 解析顺序：服务 locales > 包内置文案 > 错误码本身
//
// 中间那层是后加的。在它存在之前查不到就直接返回 key，于是一个还没写 locales 的
// 新服务，前端收到的 msg 就是 "error.not_found" 这样的内部标识符
//
// [EN] Resolution order: the service's locales, then this package's builtin copy, then the
// code itself. The middle layer was added because without it a service with no locales yet
// handed the frontend internal identifiers like "error.not_found"
func resolveI18nMessage(ctx context.Context, key string) string {
	message := goi18n.GetMsgByKey(ctx, key)
	if message == "" || message == key {
		message = BuiltinMessage(goi18n.GetLanguage(ctx), key)
	}
	if message == "" {
		return key
	}
	return message
}

// resolveI18nMessageWithTemplate 同上，并套用模板变量
//
// 内置文案兜底时同样要套模板：内置表里存的是带占位符的模板（如
// "Invalid request parameter: {reason}"），若直接返回，服务 locales 未覆盖该错误码时
// 调用方就会收到字面量 {reason} —— 而内置文案存在的理由恰恰是让新服务零配置也有可读消息
//
// [EN] The builtin fallback must have the template applied too: the table stores templates
// with placeholders, so returning one verbatim would hand the caller a literal {reason}
// precisely in the zero-config case the table exists for
func resolveI18nMessageWithTemplate(ctx context.Context, key string, templateData map[string]interface{}) string {
	message := goi18n.GetMsgWithMap(ctx, key, templateData)
	if message == "" || message == key {
		if builtin := BuiltinMessage(goi18n.GetLanguage(ctx), key); builtin != "" {
			message = goi18n.FormatWithTemplateData(builtin, templateData)
		} else {
			message = ""
		}
	}
	if message == "" {
		return key
	}
	return message
}

//===========================  错误消息提取 | Message Extraction  ==========================//

// ExtractRpcErrorMsg 从 RPC 错误中提取可读消息（无 status 时回退 error 文案）
//
// 用 errors.As 直接找链上最内层实现 GRPCStatus 的那个错误，而不是 status.FromError
//
// 原因：grpc-go 的 status.FromError 在遇到被包裹的 status 错误时，虽然能 unwrap
// 拿到内层 status，却会把外层 error 的完整文案覆盖进 Message
// 于是 fmt.Errorf("调用下游失败: %w", status.Error(codes.NotFound, "缺少用户"))
// 提取出来是 "调用下游失败: rpc error: code = NotFound desc = 缺少用户" ——
// 把 gRPC 的信封文本漏给了调用方，而这个函数的用途恰恰是拿到干净的 "缺少用户"
//
// [EN] Uses errors.As to reach the innermost error implementing GRPCStatus rather than
// status.FromError: when grpc-go unwraps a wrapped status error it overwrites Message with the
// outer error's full text, so the extracted message leaks the "rpc error: code = ..." envelope
// — exactly what this helper exists to strip
func ExtractRpcErrorMsg(err error) string {
	if err == nil {
		return ""
	}
	var withStatus interface{ GRPCStatus() *status.Status }
	if stderrors.As(err, &withStatus) {
		if st := withStatus.GRPCStatus(); st != nil {
			return st.Message()
		}
	}
	return strings.TrimSpace(err.Error())
}

// ExtractEnvelopeMsg 从错误字符串中提取网关包络 {code,msg} 里的 msg 字段
//
// 处理的是已经被网关包成 JSON、又被塞回字符串的那种错误文案，
// 与 ExtractRpcErrorMsg 的输入不同：那个收 error，这个收字符串
//
// [EN] Pulls msg out of a gateway {code,msg} envelope that has been flattened back into a
// string. Its input differs from ExtractRpcErrorMsg's: that one takes an error, this a string
func ExtractEnvelopeMsg(errStr string) string {
	start := strings.Index(errStr, "{")
	end := strings.LastIndex(errStr, "}")
	if start == -1 || end == -1 || start >= end {
		return errStr
	}
	var errData struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal([]byte(errStr[start:end+1]), &errData); err != nil {
		return errStr
	}
	if errData.Msg != "" {
		return errData.Msg
	}
	return errStr
}
