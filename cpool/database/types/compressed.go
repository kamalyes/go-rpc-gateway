/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-30 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-30 00:00:00
 * @FilePath: \go-rpc-gateway\cpool\database\types\compressed.go
 * @Description: 数据库压缩存储类型——字段级透明 gzip 压缩，多 DB 方言适配
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package types

import (
	"database/sql/driver"
	"fmt"

	"github.com/kamalyes/go-toolbox/pkg/serializer"
	"github.com/kamalyes/go-toolbox/pkg/zipx"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// =================== CompressedText（压缩文本） ===================

// CompressedText 压缩文本——写入时 gzip 压缩，读取时透明解压
// 兼容历史未压缩数据：非 GZIP: 前缀的数据直接返回原值
// [EN] Compressed text—gzip on write, transparent decompress on read; backward-compatible with plain text

type CompressedText string

// Value 实现 driver.Valuer：空值返回 nil，非空值 gzip 压缩并添加 GZIP: 前缀
func (c CompressedText) Value() (driver.Value, error) {
	if c == "" {
		return nil, nil
	}
	return zipx.GzipCompressWithPrefix([]byte(c))
}

// Scan 实现 sql.Scanner：透明解压（兼容未压缩历史数据）
func (c *CompressedText) Scan(value interface{}) error {
	if value == nil {
		*c = ""
		return nil
	}
	b, err := toBytes(value)
	if err != nil {
		return fmt.Errorf("compressed text scan: %w", err)
	}
	if len(b) == 0 {
		*c = ""
		return nil
	}
	data, err := zipx.GzipSmartDecompress(b)
	if err != nil {
		return fmt.Errorf("compressed text decompress: %w", err)
	}
	*c = CompressedText(data)
	return nil
}

// GormDataType 实现 schema.GormDataTypeInterface：声明逻辑数据类型为 string
// 必须实现：否则 gorm schema 解析时 DataType 为空，字段会被误判为关联关系（parseRelation 报错）
func (CompressedText) GormDataType() string {
	return "string"
}

// GormDBDataType 按方言返回列类型：ClickHouse→String，MySQL→LONGTEXT，其他→TEXT
func (CompressedText) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "mysql":
		return "LONGTEXT"
	case "sqlite", "postgres":
		return "TEXT"
	default:
		return "String" // ClickHouse（gorm clickhouse driver 默认映射）
	}
}

// String 返回底层字符串值
func (c CompressedText) String() string {
	return string(c)
}

// =================== CompressedTextMap（压缩 Map） ===================

// CompressedTextMap 压缩键值映射——JSON 序列化后 gzip 压缩存储
// 适用于大体积结构化文本，读取时透明解压并反序列化
// [EN] Compressed key-value map—JSON marshal then gzip compress; transparent decompress on read

type CompressedTextMap map[string][]string

// Value 实现 driver.Valuer：nil 返回 nil；非 nil 先 JSON 序列化再 gzip 压缩
func (c CompressedTextMap) Value() (driver.Value, error) {
	if c == nil {
		return nil, nil
	}
	data, err := serializer.JSONMarshal(c)
	if err != nil {
		return nil, fmt.Errorf("compressed map marshal: %w", err)
	}
	return zipx.GzipCompressWithPrefix(data)
}

// Scan 实现 sql.Scanner：透明解压后 JSON 反序列化
func (c *CompressedTextMap) Scan(value interface{}) error {
	if value == nil {
		*c = nil
		return nil
	}
	b, err := toBytes(value)
	if err != nil {
		return fmt.Errorf("compressed map scan: %w", err)
	}
	if len(b) == 0 {
		*c = nil
		return nil
	}
	data, err := zipx.GzipSmartDecompress(b)
	if err != nil {
		return fmt.Errorf("compressed map decompress: %w", err)
	}
	if len(data) == 0 {
		*c = nil
		return nil
	}
	return serializer.JSONUnmarshal(data, c)
}

// GormDataType 实现 schema.GormDataTypeInterface：声明逻辑数据类型为 string
// 必须实现：Map 底层类型无法被 gorm 推导 DataType，空 DataType 会被误判为关联关系导致解析失败
func (CompressedTextMap) GormDataType() string {
	return "string"
}

// GormDBDataType 按方言返回列类型：全部返回 TEXT/String（map 压缩为 JSON 字符串存储）
func (CompressedTextMap) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "mysql":
		return "LONGTEXT"
	case "sqlite", "postgres":
		return "TEXT"
	default:
		return "String"
	}
}

// ToMap 转为原生 map[string][]string（零拷贝，底层类型相同）
func (c CompressedTextMap) ToMap() map[string][]string {
	return map[string][]string(c)
}

// FromMap 从原生 map[string][]string 创建（零拷贝）
func CompressedTextMapFromMap(m map[string][]string) CompressedTextMap {
	return CompressedTextMap(m)
}
