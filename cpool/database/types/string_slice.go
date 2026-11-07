/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-08 15:00:00
 * @FilePath: \go-rpc-gateway\cpool\database\types\string_slice.go
 * @Description: StringSlice - 字符串切片，支持数据库 JSON 序列化和分隔符解析
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package types

import (
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/kamalyes/go-toolbox/pkg/serializer"
)

// StringSlice 字符串切片，支持数据库 JSON 序列化
type StringSlice []string

func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = []string{}
		return nil
	}
	b, err := toBytes(value)
	if err != nil {
		return fmt.Errorf("failed to unmarshal StringSlice: %w", err)
	}
	if len(b) == 0 {
		*s = []string{}
		return nil
	}
	return serializer.JSONUnmarshal(b, s)
}

func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return serializer.JSONMarshal(s)
}

// Contains 检查是否包含指定元素
func (s StringSlice) Contains(item string) bool {
	for _, v := range s {
		if v == item {
			return true
		}
	}
	return false
}

// FromDelimitedString 从分隔符字符串创建 StringSlice
// 支持多种分隔符: 分号(;)、换行符(\n)、逗号(,)
// 自动去除空白字符和空字符串
func (s *StringSlice) FromDelimitedString(input string, delimiters ...string) StringSlice {
	if input == "" {
		*s = StringSlice{}
		return *s
	}

	// 默认分隔符: 换行符和分号
	if len(delimiters) == 0 {
		delimiters = []string{"\n", ";"}
	}

	// 使用多个分隔符递归分割
	items := []string{input}
	for _, delimiter := range delimiters {
		temp := make([]string, 0)
		for _, item := range items {
			parts := strings.Split(item, delimiter)
			temp = append(temp, parts...)
		}
		items = temp
	}

	// 去重和清理
	seen := make(map[string]bool)
	final := make([]string, 0)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			final = append(final, item)
		}
	}

	*s = StringSlice(final)
	return *s
}

// ParseStringSlice 从混合格式解析字符串切片
// 支持数组或分隔字符串 (分号;换行符\n)
func ParseStringSlice(input []string) StringSlice {
	if len(input) == 0 {
		return StringSlice{}
	}

	var allItems []string
	for _, item := range input {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		// 检查是否包含分隔符
		if strings.Contains(item, ";") || strings.Contains(item, "\n") {
			var result StringSlice
			parsed := result.FromDelimitedString(item)
			allItems = append(allItems, parsed...)
		} else {
			// 单个值
			allItems = append(allItems, item)
		}
	}

	// 去重和清理
	seen := make(map[string]bool)
	final := make([]string, 0)
	for _, item := range allItems {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			final = append(final, item)
		}
	}

	return StringSlice(final)
}

// IndexOf 查找元素索引，未找到返回 -1
func (s StringSlice) IndexOf(item string) int {
	for i, v := range s {
		if v == item {
			return i
		}
	}
	return -1
}

// Append 追加元素
func (s *StringSlice) Append(items ...string) StringSlice {
	*s = append(*s, items...)
	return *s
}

// Remove 移除指定元素（第一个匹配的）
func (s *StringSlice) Remove(item string) StringSlice {
	for i, v := range *s {
		if v == item {
			*s = append((*s)[:i], (*s)[i+1:]...)
			break
		}
	}
	return *s
}

// RemoveAt 移除指定索引的元素
func (s *StringSlice) RemoveAt(index int) StringSlice {
	if index >= 0 && index < len(*s) {
		*s = append((*s)[:index], (*s)[index+1:]...)
	}
	return *s
}

// Filter 过滤元素
func (s StringSlice) Filter(fn func(string) bool) StringSlice {
	result := make(StringSlice, 0)
	for _, v := range s {
		if fn(v) {
			result = append(result, v)
		}
	}
	return result
}

// Map 映射转换
func (s StringSlice) Map(fn func(string) string) StringSlice {
	result := make(StringSlice, len(s))
	for i, v := range s {
		result[i] = fn(v)
	}
	return result
}

// Unique 去重
func (s StringSlice) Unique() StringSlice {
	seen := make(map[string]bool)
	result := make(StringSlice, 0)
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// Join 连接为字符串
func (s StringSlice) Join(sep string) string {
	if len(s) == 0 {
		return ""
	}
	result := s[0]
	for i := 1; i < len(s); i++ {
		result += sep + s[i]
	}
	return result
}

// Clone 克隆一个新的切片
func (s StringSlice) Clone() StringSlice {
	clone := make(StringSlice, len(s))
	copy(clone, s)
	return clone
}
