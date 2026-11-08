/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\errors\code.go
 * @Description: 错误码定义 - Swagger 中间件与加载器的全部错误类型常量
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package errors

import "github.com/kamalyes/go-toolbox/pkg/errorx"

// ==================== Swagger 中间件错误类型（7000-7099） ====================

const (
	ErrTypeConfigNil errorx.ErrorType = 7000 + iota
	ErrTypeAggregateDisabled
	ErrTypeSpecNotInitialized
	ErrTypeServiceNotFound
	ErrTypeDocumentNotFound
	ErrTypeDocumentNameEmpty
	ErrTypeDocumentDuplicate
	ErrTypeDocumentSourceEmpty
	ErrTypeDocumentNoSources
	ErrTypeDocumentNoService
	ErrTypeLoadFailed
	ErrTypeAggregateFailed
	ErrTypeWatcherAlreadyStart
	ErrTypeWatcherStartFailed
	ErrTypeWatcherStopFailed
	ErrTypeWatcherNoFiles
	ErrTypeSerializeFailed
	ErrTypeInvalidFileFormat
	ErrTypeNoServicesConfig
	ErrTypeAuthConfigInvalid
)

// ==================== 加载器错误类型（7100-7199） ====================

const (
	ErrTypeLoaderFileNotFound errorx.ErrorType = 7100 + iota
	ErrTypeLoaderReadFailed
	ErrTypeLoaderParseFailed
	ErrTypeLoaderHTTPFailed
)
