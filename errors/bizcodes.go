/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-02-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-02-08 00:00:00
 * @FilePath: \go-rpc-gateway\errors\bizcodes.go
 * @Description: 通用业务错误码常量
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package errors

// 通用业务错误码
//
// 这批常量原先住在独立的 go-bizerror 仓库里。拆出去的理由是当时 bizerror 若引用
// 网关类型，就会被 global -> cpool 这条边拖进 MySQL、Redis、ClickHouse 等几百个包；
// 错误码是最底层的契约，不该背着连接池走。
//
// 现在那条边已经断了（见 violations.go 的 ViolationLogger），errors 包只依赖
// pb 类型与 i18n，于是「错误码定义」与「错误码到 gRPC / 网关码的映射」可以住在一起。
// 分成两个仓库的代价是实打实的：两份 ToGRPCError、两套注册表、两张内置文案表，
// 外加一个专门把两边粘起来的 gwx 子模块 —— 而那个子模块最终没有任何调用方。
//
// [EN] These constants used to live in a separate go-bizerror repository.
//
// It was split out because referencing the gateway's types dragged it, via global -> cpool,
// into MySQL, Redis, ClickHouse and several hundred other packages — and an error-code
// contract should not carry a connection pool. That edge is now cut (see ViolationLogger in
// violations.go): this package depends only on the pb types and i18n, so the code definitions
// and the mapping from them to gRPC and gateway codes can live together. Two repositories cost
// two ToGRPCError implementations, two registries, two builtin-message tables and a bridge
// submodule to glue them — a submodule that ended up with no callers at all.
//
// 命名保留 BizErrCode 前缀：与同包的 ErrCode*（网关层错误码）区分开，
// 一个是业务语义串、一个是网关数值码，混用过的人都知道那有多难查
// [EN] The BizErrCode prefix is kept to separate these business-semantic strings from the
// ErrCode* numeric gateway codes in this same package
const (
	//===========================  通用错误 | Common  ==========================//

	BizErrCodeInvalidArgument         = "error.invalid_argument"
	BizErrCodeUnauthenticated         = "error.unauthenticated"
	BizErrCodePermissionDenied        = "error.permission_denied"
	BizErrCodeNotFound                = "error.not_found"
	BizErrCodeConflict                = "error.conflict"
	BizErrCodeValidationFailed        = "error.validation_failed"
	BizErrCodeInvalidParameter        = "error.invalid_parameter"
	BizErrCodeCannotBeSelf            = "error.cannot_be_self"
	BizErrCodeDatabaseError           = "error.database_error"
	BizErrCodeDatabaseOperationFailed = "error.database_operation_failed"
	BizErrCodeConversionFailed        = "error.conversion_failed"

	//===========================  系统内部错误 | Internal  ==========================//

	BizErrCodeInternalError            = "error.internal"
	BizErrCodeRepositoryNotInitialized = "error.repository_not_initialized"
	BizErrCodeUnsupportedOperation     = "error.unsupported_operation"
	BizErrCodeExternalServiceError     = "error.external_service_error"
)
