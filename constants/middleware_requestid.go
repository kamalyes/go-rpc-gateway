/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:40:02
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:40:10
 * @FilePath: \go-rpc-gateway\constants\middleware_requestid.go
 * @Description: 请求ID中间件相关常量
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

// 请求ID生成策略常量
const (
	RequestIDGeneratorUUID      = "uuid"
	RequestIDGeneratorSnowflake = "snowflake"
	RequestIDGeneratorTimestamp = "timestamp"
	RequestIDGeneratorCustom    = "custom"
)

// 默认请求ID配置
const (
	// 默认生成策略
	RequestIDDefaultGenerator = RequestIDGeneratorUUID

	// 默认请求头
	RequestIDDefaultHeader = HeaderXRequestID

	// 默认响应头
	RequestIDDefaultResponseHeader = HeaderXRequestID

	// 默认是否启用
	RequestIDDefaultEnabled = true

	// 默认长度（用于自定义生成器）
	RequestIDDefaultLength = 32
)

// 雪花算法配置
const (
	// 默认数据中心ID
	RequestIDSnowflakeDefaultDatacenterID = 1

	// 默认工作节点ID
	RequestIDSnowflakeDefaultWorkerID = 1
)

// 请求ID格式常量
const (
	// UUID v4 格式
	RequestIDFormatUUIDv4 = "uuid-v4"

	// 时间戳+随机数格式
	RequestIDFormatTimestamp = "timestamp-random"

	// 简短格式
	RequestIDFormatShort = "short"

	// 自定义格式
	RequestIDFormatCustom = "custom"
)

// 请求ID验证模式
const (
	RequestIDValidationStrict = "strict"
	RequestIDValidationLoose  = "loose"
	RequestIDValidationNone   = "none"
)
