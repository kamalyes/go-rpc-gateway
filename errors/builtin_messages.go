/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-02-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-02-08 00:00:00
 * @FilePath: \go-rpc-gateway\errors\builtin_messages.go
 * @Description: 通用错误码的内置兜底文案
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package errors

// builtinMessages 内置通用错误码文案（语言 → 错误码 → 文案）
//
// 服务侧 locales/*.json 的同名 key 优先，这里只在缺失时兜底。
//
// 为什么必须有这张表：在它存在之前，resolveI18nMessage 查不到就把 key 本身当消息返回，
// 于是一个还没写 locales 的新服务，前端收到的 msg 就是字面量 "error.not_found"。
// 那不是错误消息，那是内部标识符漏到了界面上
//
// [EN] Builtin copy for the common error codes, keyed by language then code.
//
// A service's own locales/*.json wins; this table only fills gaps. It has to exist because
// without it resolveI18nMessage returned the key itself on a miss, so a new service with no
// locales yet handed the frontend the literal string "error.not_found" — not an error message
// but an internal identifier leaking into the UI
var builtinMessages = map[string]map[string]string{
	"en": {
		BizErrCodeInvalidArgument:          "Invalid argument",
		BizErrCodeUnauthenticated:          "Unauthenticated",
		BizErrCodePermissionDenied:         "Permission denied",
		BizErrCodeNotFound:                 "Resource not found",
		BizErrCodeConflict:                 "Resource conflict",
		BizErrCodeValidationFailed:         "Validation failed",
		BizErrCodeInvalidParameter:         "Invalid request parameter: {reason}",
		BizErrCodeCannotBeSelf:             "Cannot perform this operation on yourself",
		BizErrCodeDatabaseError:            "Database error",
		BizErrCodeDatabaseOperationFailed:  "Database operation failed",
		BizErrCodeConversionFailed:         "Data conversion failed",
		BizErrCodeInternalError:            "Internal error",
		BizErrCodeRepositoryNotInitialized: "Repository not initialized",
		BizErrCodeUnsupportedOperation:     "Unsupported operation",
		BizErrCodeExternalServiceError:     "External service error",
	},
	"zh": {
		BizErrCodeInvalidArgument:          "无效的参数",
		BizErrCodeUnauthenticated:          "未认证",
		BizErrCodePermissionDenied:         "权限不足",
		BizErrCodeNotFound:                 "资源未找到",
		BizErrCodeConflict:                 "资源冲突",
		BizErrCodeValidationFailed:         "数据校验失败",
		BizErrCodeInvalidParameter:         "请求参数无效：{reason}",
		BizErrCodeCannotBeSelf:             "不能对自身执行该操作",
		BizErrCodeDatabaseError:            "数据库错误",
		BizErrCodeDatabaseOperationFailed:  "数据库操作失败",
		BizErrCodeConversionFailed:         "数据转换失败",
		BizErrCodeInternalError:            "内部错误",
		BizErrCodeRepositoryNotInitialized: "仓库未初始化",
		BizErrCodeUnsupportedOperation:     "不支持的操作",
		BizErrCodeExternalServiceError:     "外部服务错误",
	},
}

// BuiltinMessage 查询内置文案：指定语言未收录时回退 en，仍无返回空串
//
// 导出而不是私有：调用方需要能判断「这个码有没有内置文案」，
// 用返回空串表达「没有」，比让调用方自己再维护一张表要好
//
// [EN] Looks up the builtin copy, falling back to en and then to an empty string.
//
// Exported rather than private so callers can tell whether a code has builtin copy at all;
// an empty return says "none", which beats making every caller keep its own table
func BuiltinMessage(language, errorCode string) string {
	if language == "" {
		language = "en"
	}
	if msg, ok := builtinMessages[language][errorCode]; ok {
		return msg
	}
	return builtinMessages["en"][errorCode]
}
