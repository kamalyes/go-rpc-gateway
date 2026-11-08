/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-02-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-02-08 00:00:00
 * @FilePath: \go-rpc-gateway\errors\chain.go
 * @Description: 通用业务错误码的具名构造器与链式构造器
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package errors

import "context"

//===========================  具名构造器 | Named Constructors  ==========================//

// 具名构造器一行直出 gRPC 错误，替代 ToGRPCError(ctx, BizErrCodeXxx) 的两段式写法
//
// 没有它们时，每个服务都会在自己的 service/common 里重新包一层：
//
//	func NotFound(ctx context.Context) error {
//	    return errors.ToGRPCError(ctx, errors.BizErrCodeNotFound)
//	}
//
// 这层包装不含任何业务信息，却要在每个新项目里重写一遍、各自起名、各自漏掉几个码，
// 于是同一个「资源不存在」在不同服务里叫 NotFound、NotFoundErr、ErrNotFound
// 错误码表、映射表与内置文案都在本包，构造器自然也该在本包
//
// [EN] Named constructors return the gRPC error in one call. Without them every service
// re-wraps ToGRPCError in its own service/common: a layer that carries no business
// information, yet gets rewritten, renamed and partially forgotten in every new project,
// so one "not found" ends up as NotFound, NotFoundErr and ErrNotFound across services.
// The code table, the mappings and the builtin messages all live here, so these do too

// InvalidArgument 请求参数不合法
func InvalidArgument(ctx context.Context) error { return ToGRPCError(ctx, BizErrCodeInvalidArgument) }

// Unauthenticated 未认证（缺少或无法识别身份）
func Unauthenticated(ctx context.Context) error { return ToGRPCError(ctx, BizErrCodeUnauthenticated) }

// PermissionDenied 已认证但无权限
func PermissionDenied(ctx context.Context) error {
	return ToGRPCError(ctx, BizErrCodePermissionDenied)
}

// NotFound 资源不存在（通用码）
//
// 优先用实体专属码：服务的 errcodes 里每个实体都有自己的 not_found，响应里带的是
// error.<实体>_not_found，调用方一眼能看出缺的是哪个资源；用这个通用码只会得到
// error.not_found，排查时还要回去翻日志对调用栈
// 它留给「确实没有实体可指」的场合：按某个外部标识查不到任何东西，
// 或新实体还没声明专属码时的过渡写法
//
// [EN] Prefer the entity's own code: the service's errcodes gives every entity a not-found of
// its own, so the response says which resource is missing, where this generic one yields only
// error.not_found and sends whoever is debugging back to the logs. It is for the cases where no
// entity can be named, and as a stopgap before a new entity gets its own code
func NotFound(ctx context.Context) error { return ToGRPCError(ctx, BizErrCodeNotFound) }

// Conflict 与现有资源冲突（唯一键重复、状态不允许等）
func Conflict(ctx context.Context) error { return ToGRPCError(ctx, BizErrCodeConflict) }

// ValidationFailed 校验未通过
func ValidationFailed(ctx context.Context) error {
	return ToGRPCError(ctx, BizErrCodeValidationFailed)
}

// InvalidParameter 参数取值不合法
func InvalidParameter(ctx context.Context) error {
	return ToGRPCError(ctx, BizErrCodeInvalidParameter)
}

// CannotBeSelf 不允许指向自身
func CannotBeSelf(ctx context.Context) error { return ToGRPCError(ctx, BizErrCodeCannotBeSelf) }

// DatabaseError 数据库错误
func DatabaseError(ctx context.Context) error { return ToGRPCError(ctx, BizErrCodeDatabaseError) }

// DatabaseOperationFailed 数据库操作失败
//
// 统一返回它而不是仓储层的原始 error：底层错误里可能带表名、SQL 片段、连接串，
// 那是内部信息日志里记全文，响应里只给错误码与本地化文案
//
// [EN] Return this instead of the repository's raw error: the underlying one can carry table
// names, SQL fragments or connection strings. The full text goes to the log; the response
// carries only the code and its localized message
func DatabaseOperationFailed(ctx context.Context) error {
	return ToGRPCError(ctx, BizErrCodeDatabaseOperationFailed)
}

// ConversionFailed 数据转换失败
func ConversionFailed(ctx context.Context) error {
	return ToGRPCError(ctx, BizErrCodeConversionFailed)
}

// InternalError 服务内部错误
func InternalError(ctx context.Context) error { return ToGRPCError(ctx, BizErrCodeInternalError) }

// RepositoryNotInitialized 仓储未初始化
func RepositoryNotInitialized(ctx context.Context) error {
	return ToGRPCError(ctx, BizErrCodeRepositoryNotInitialized)
}

// UnsupportedOperation 不支持的操作
func UnsupportedOperation(ctx context.Context) error {
	return ToGRPCError(ctx, BizErrCodeUnsupportedOperation)
}

// ExternalServiceError 外部服务错误
func ExternalServiceError(ctx context.Context) error {
	return ToGRPCError(ctx, BizErrCodeExternalServiceError)
}

//===========================  链式构造器 | Chain  ==========================//

// Chain 链式错误构造器，用于需要模板变量或底层原因的场景；
// 服务自注册的错误码（经 Register / RegisterMany 登记过映射的码）也走这里，
// 不必在服务侧再包一层
type Chain struct {
	code     string
	template map[string]interface{}
}

// Of 以错误码开启一条链；code 可以是本包的 BizErrCodeXxx，也可以是服务自注册的码
//
//	return errors.Of(errcodes.QuotaExceeded).
//	    With("limit", limit).
//	    Cause(err).
//	    Err(ctx)
func Of(code string) *Chain {
	return &Chain{code: code}
}

// With 追加一个模板变量
func (c *Chain) With(key string, value interface{}) *Chain {
	if c == nil {
		return nil
	}
	if c.template == nil {
		c.template = make(map[string]interface{}, 4)
	}
	c.template[key] = value
	return c
}

// WithMap 批量追加模板变量；同名键以 data 为准
func (c *Chain) WithMap(data map[string]interface{}) *Chain {
	if c == nil {
		return nil
	}
	for key, value := range data {
		c.With(key, value)
	}
	return c
}

// Cause 把底层错误文案写入 {reason} 占位
//
// cause 为 nil 时不改变链，调用方才可以无条件写 .Cause(err) 而不必先判空
// [EN] A nil cause leaves the chain untouched, so callers can write .Cause(err)
// unconditionally instead of branching on nil first
func (c *Chain) Cause(cause error) *Chain {
	if c == nil || cause == nil {
		return c
	}
	return c.With("reason", cause.Error())
}

// Err 收束为携带 i18n 消息的 gRPC 错误
func (c *Chain) Err(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if len(c.template) == 0 {
		return ToGRPCError(ctx, c.code)
	}
	return ToGRPCErrorWithTemplate(ctx, c.code, c.template)
}

// Message 只取 i18n 解析后的消息文本，不做 gRPC 包装（写日志或塞业务响应体时用）
func (c *Chain) Message(ctx context.Context) string {
	if c == nil {
		return ""
	}
	if len(c.template) == 0 {
		return NewI18nError(ctx, c.code)
	}
	return NewI18nErrorWithTemplate(ctx, c.code, c.template)
}
