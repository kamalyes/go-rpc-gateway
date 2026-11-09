/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\errors\base.go
 * @Description: 错误注册 - 注册所有错误类型及其默认消息模板
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package errors

import "github.com/kamalyes/go-toolbox/pkg/errorx"

// init 注册所有错误类型及其默认消息模板
func init() {
	// ==================== Swagger 中间件错误 ====================
	errorx.RegisterError(ErrTypeConfigNil, "Swagger配置不能为空")
	errorx.RegisterError(ErrTypeAggregateDisabled, "聚合功能未启用")
	errorx.RegisterError(ErrTypeSpecNotInitialized, "Swagger规范未初始化")
	errorx.RegisterError(ErrTypeServiceNotFound, "服务 %s 不存在")
	errorx.RegisterError(ErrTypeDocumentNotFound, "文档 %s 不存在")
	errorx.RegisterError(ErrTypeDocumentNameEmpty, "独立文档名称不能为空")
	errorx.RegisterError(ErrTypeDocumentDuplicate, "独立文档 %s 重复配置")
	errorx.RegisterError(ErrTypeDocumentSourceEmpty, "文档来源不能为空")
	errorx.RegisterError(ErrTypeDocumentNoSources, "文档 %s 未配置 sources")
	errorx.RegisterError(ErrTypeDocumentNoService, "文档 %s 存在未配置 service 的 source")
	errorx.RegisterError(ErrTypeLoadFailed, "加载Swagger规范失败: %v")
	errorx.RegisterError(ErrTypeAggregateFailed, "聚合规范失败: %v")
	errorx.RegisterError(ErrTypeWatcherAlreadyStart, "文件监听器已启动")
	errorx.RegisterError(ErrTypeWatcherStartFailed, "启动文件监听器失败: %v")
	errorx.RegisterError(ErrTypeWatcherStopFailed, "停止文件监听器失败: %v")
	errorx.RegisterError(ErrTypeWatcherNoFiles, "没有可监听的文件")
	errorx.RegisterError(ErrTypeSerializeFailed, "序列化JSON失败: %v")
	errorx.RegisterError(ErrTypeInvalidFileFormat, "不支持的文件格式: %s")
	errorx.RegisterError(ErrTypeNoServicesConfig, "没有配置聚合服务")
	errorx.RegisterError(ErrTypeAuthConfigInvalid, "Swagger认证配置无效: %v")

	// ==================== 加载器错误 ====================
	errorx.RegisterError(ErrTypeLoaderFileNotFound, "Swagger文件未找到: %s")
	errorx.RegisterError(ErrTypeLoaderReadFailed, "读取Swagger文件失败: %s")
	errorx.RegisterError(ErrTypeLoaderParseFailed, "解析Swagger文件失败: %s")
	errorx.RegisterError(ErrTypeLoaderHTTPFailed, "HTTP请求失败: %s")
}
