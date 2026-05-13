/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-15 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-15 00:00:00
 * @FilePath: \go-rpc-gateway\global\idgen.go
 * @Description: 统一ID生成器
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */
package global

import (
	"github.com/kamalyes/go-toolbox/pkg/idgen"
	"github.com/kamalyes/go-toolbox/pkg/osx"
	"github.com/kamalyes/go-toolbox/pkg/safe"
)

// defaultSnowflakeShortIDLength 默认 Snowflake 短 ID 长度
const defaultSnowflakeShortIDLength = 8

var (
	snowflakeWorkerID     = osx.GetWorkerIdForSnowflake()                                         // 当前进程 Snowflake 使用的 workerID
	snowflakeDatacenterID = osx.GetDatacenterId()                                                 // 当前进程 Snowflake 使用的 datacenterID
	snowflakeGen          = idgen.NewSnowflakeGenerator(snowflakeWorkerID, snowflakeDatacenterID) // Snowflake ID生成器
)

// NewSnowflakeID 生成一个新的短 ID 字符串
func NewSnowflakeID() string {
	return NewSnowflakeIDWithLength(defaultSnowflakeShortIDLength)
}

// NewSnowflakeID12 生成一个新的 12 位短 ID 字符串
func NewSnowflakeID12() string {
	return NewSnowflakeIDWithLength(12)
}

// NewSnowflakeIDWithLength 生成指定长度的短 ID 字符串
func NewSnowflakeIDWithLength(length int) string {
	return safe.ShortHashWithLength(snowflakeGen.GenerateRequestID(), length)
}

// GetSnowflakeWorkerID 获取当前进程 Snowflake 使用的 workerID
func GetSnowflakeWorkerID() int64 {
	return snowflakeWorkerID
}

// GetSnowflakeDatacenterID 获取当前进程 Snowflake 使用的 datacenterID
func GetSnowflakeDatacenterID() int64 {
	return snowflakeDatacenterID
}
