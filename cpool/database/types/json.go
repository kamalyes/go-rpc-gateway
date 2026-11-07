/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-11 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-08 15:05:00
 * @FilePath: \go-rpc-gateway\cpool\database\types\json.go
 * @Description: JSON - 泛型 JSON 类型，支持任意可序列化类型的数据库存储
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package types

import (
	"database/sql/driver"
	"fmt"
	"reflect"

	"github.com/kamalyes/go-toolbox/pkg/serializer"
)

// JSON 泛型 JSON 类型，支持任意可序列化类型
type JSON[T any] struct {
	Data T
}

// Len 获取数据长度（仅对切片类型有效）
func (j JSON[T]) Len() int {
	rv := reflect.ValueOf(j.Data)
	if rv.Kind() == reflect.Slice {
		return rv.Len()
	}
	return 0
}

// IsEmpty 判断是否为空（切片长度为0或数据为零值）
func (j JSON[T]) IsEmpty() bool {
	rv := reflect.ValueOf(j.Data)
	if rv.Kind() == reflect.Slice {
		return rv.Len() == 0
	}
	return rv.IsZero()
}

// Append 追加元素（仅对切片类型有效）
func (j *JSON[T]) Append(items ...any) {
	rv := reflect.ValueOf(&j.Data)
	if rv.Elem().Kind() != reflect.Slice {
		return
	}
	for _, item := range items {
		rv.Elem().Set(reflect.Append(rv.Elem(), reflect.ValueOf(item)))
	}
}

// Clone 克隆数据
func (j JSON[T]) Clone() JSON[T] {
	rv := reflect.ValueOf(j.Data)
	clone := reflect.New(rv.Type()).Elem()
	clone.Set(rv)
	return JSON[T]{Data: clone.Interface().(T)}
}

func (j *JSON[T]) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, err := toBytes(value)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	if len(b) == 0 {
		return nil
	}
	return serializer.JSONUnmarshal(b, &j.Data)
}

func (j JSON[T]) Value() (driver.Value, error) {
	return serializer.JSONMarshal(j.Data)
}

// Get 获取数据
func (j *JSON[T]) Get() T {
	return j.Data
}

// Set 设置数据
func (j *JSON[T]) Set(data T) {
	j.Data = data
}

// IsNil 判断 Data 是否为 nil
// 仅对指针、接口、切片、map、channel、函数类型有效（通过反射安全判断）
// 值类型（struct、int、string 等）永远返回 false
func (j JSON[T]) IsNil() bool {
	rv := reflect.ValueOf(j.Data)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return rv.IsNil()
	default:
		return false
	}
}

// IsPresent 判断 Data 是否非 nil（IsNil 的反向）
func (j JSON[T]) IsPresent() bool {
	return !j.IsNil()
}

// GetOrZero 获取数据，如果 Data 为 nil 则返回 T 的零值
func (j JSON[T]) GetOrZero() T {
	if j.IsNil() {
		var zero T
		return zero
	}
	return j.Data
}

// GetOrDefault 获取数据，如果 Data 为 nil 则返回 defaultValue
func (j JSON[T]) GetOrDefault(defaultValue T) T {
	if j.IsNil() {
		return defaultValue
	}
	return j.Data
}

// IfPresent 当 Data 非 nil 时执行回调（Java Optional 风格）
// 用于消除 `if x.Data != nil { ... }` 这类样板代码
func (j JSON[T]) IfPresent(fn func(T)) {
	if !j.IsNil() {
		fn(j.Data)
	}
}

// ToOptional 将 JSON[T] 转换为 Optional[T]，便于使用 MapOpt/FlatMapOpt 等单子操作
// Data 为 nil 时返回空 Optional
func (j JSON[T]) ToOptional() Optional[T] {
	if j.IsNil() {
		return Optional[T]{}
	}
	return Optional[T]{value: j.Data, present: true}
}

// MapJSON 当 Data 非 nil 时应用转换函数，返回 *U
// Data 为 nil 时返回 nil；fn 内部可返回 nil 主动中断链路
// 适用于链式访问嵌套指针字段，配合 MapPtr 实现深层取值
// （命名为 MapJSON 是为避免与 slice.go 中 Slice 的 Map 函数冲突）
//
// 示例（单层）：
//
//	fb := types.MapJSON(agentLine.Tracking, func(t *TrackingConfig) *FacebookConfig {
//	    return t.Facebook
//	})
//	if fb != nil {
//	    params.TrackID = fb.PixelId
//	}
//
// 注：Go 方法不支持额外类型参数，因此以泛型函数形式提供
func MapJSON[T any, U any](j JSON[T], fn func(T) *U) *U {
	if j.IsNil() {
		return nil
	}
	return fn(j.Data)
}

// MapPtr 对普通指针 *T 应用转换函数，返回 *U
// ptr 为 nil 时返回 nil；fn 接收 *T（指针）便于调用指针接收者方法
// fn 内部可返回 nil 主动中断链路
// 用于在 MapJSON 之后继续链式穿透嵌套指针字段
//
// 示例（深层链式：Tracking → Facebook → PixelId）：
//
//	fb := types.MapJSON(agentLine.Tracking, func(t *TrackingConfig) *FacebookConfig {
//	    return t.Facebook
//	})
//	pixel := types.MapPtr(fb, func(f *FacebookConfig) *string {
//	    return types.Ptr(f.PixelId)
//	})
//	if pixel != nil {
//	    params.TrackID = *pixel
//	}
func MapPtr[T any, U any](ptr *T, fn func(*T) *U) *U {
	if ptr == nil {
		return nil
	}
	return fn(ptr)
}

// Ptr 取值的地址，便于在 Map/MapPtr 的回调中返回字段的指针
//
// 示例：
//
//	types.MapPtr(fb, func(f *FacebookConfig) *string {
//	    return types.Ptr(f.PixelId)
//	})
func Ptr[T any](v T) *T {
	return &v
}
