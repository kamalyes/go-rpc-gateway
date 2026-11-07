/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-08 15:00:00
 * @FilePath: \go-rpc-gateway\cpool\database\types\slice.go
 * @Description: Slice - 泛型切片类型，支持数据库 JSON 序列化
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package types

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/kamalyes/go-toolbox/pkg/serializer"
)

// Slice 泛型切片类型
type Slice[T any] []T

func (s *Slice[T]) Scan(value interface{}) error {
	if value == nil {
		*s = []T{}
		return nil
	}
	b, err := toBytes(value)
	if err != nil {
		return fmt.Errorf("failed to unmarshal Slice: %w", err)
	}
	if len(b) == 0 {
		*s = []T{}
		return nil
	}
	b = normalizeSliceJSON(b)
	if err := serializer.JSONUnmarshal(b, s); err != nil {
		// protojson 反序列化失败时回退到标准 json
		// 兼容旧格式数据（如 Timestamp 对象格式、枚举数字值等 generated proto json tag 可解析的格式）
		return json.Unmarshal(b, s)
	}
	return nil
}

func normalizeSliceJSON(data []byte) []byte {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] == '[' || bytes.EqualFold(data, []byte("null")) {
		return data
	}
	wrapped := make([]byte, 0, len(data)+2)
	wrapped = append(wrapped, '[')
	wrapped = append(wrapped, data...)
	wrapped = append(wrapped, ']')
	return wrapped
}

func (s Slice[T]) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return serializer.JSONMarshal(s)
}

// Len 获取长度
func (s Slice[T]) Len() int {
	return len(s)
}

// Append 追加元素
func (s *Slice[T]) Append(items ...T) Slice[T] {
	*s = append(*s, items...)
	return *s
}

// Filter 过滤元素
func (s Slice[T]) Filter(fn func(T) bool) Slice[T] {
	result := make(Slice[T], 0)
	for _, v := range s {
		if fn(v) {
			result = append(result, v)
		}
	}
	return result
}

// Map 映射转换
func Map[T any, R any](s Slice[T], fn func(T) R) Slice[R] {
	result := make(Slice[R], len(s))
	for i, v := range s {
		result[i] = fn(v)
	}
	return result
}

// Clone 克隆切片
func (s Slice[T]) Clone() Slice[T] {
	clone := make(Slice[T], len(s))
	copy(clone, s)
	return clone
}
