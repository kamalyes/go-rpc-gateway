/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-25 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-25 00:00:00
 * @FilePath: \go-rpc-gateway\cpool\database\types\optional.go
 * @Description: Optional[T] - 通用可选值容器，支持 nil-safe 链式操作
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package types

// Optional[T] 通用可选值容器，支持 nil-safe 链式操作
//
// 设计要点：
//   - 内部用 present 标记区分"空"与"零值"，对值类型（int/struct）也能正确表达缺失
//   - 对指针类型 T=*V 同样适用，present=false 优先判定为空
//   - 提供 Map / FlatMap / Filter 等单子操作，便于深层嵌套字段的链式访问
//
// 与 JSON[T] 的关系：JSON[T] 主要服务于数据库读写，Optional[T] 服务于业务层 nil-safe 访问
// 通过 JSON[T].ToOptional() 进行桥接
type Optional[T any] struct {
	value   T
	present bool
}

// Of 创建一个非空 Optional
func Of[T any](v T) Optional[T] {
	return Optional[T]{value: v, present: true}
}

// OfNullable 接受指针 *T，nil 时返回空 Optional；非 nil 时解引用装入 Optional[T]
// 适用于"从指针提取值"场景
func OfNullable[T any](v *T) Optional[T] {
	if v == nil {
		return Optional[T]{}
	}
	return Optional[T]{value: *v, present: true}
}

// OfNullablePtr 接受指针 *T，nil 时返回空 Optional[*T]；非 nil 时装入指针本身
// 与 OfNullable 的区别：OfNullable 解引用得到 Optional[T]，OfNullablePtr 保留 Optional[*T]
// 适用于"链式穿透多层可空指针字段"场景（配合 FlatMapOpt）
func OfNullablePtr[T any](v *T) Optional[*T] {
	if v == nil {
		return Optional[*T]{}
	}
	return Optional[*T]{value: v, present: true}
}

// Empty 创建空 Optional
func Empty[T any]() Optional[T] {
	return Optional[T]{}
}

// IsPresent 是否有值
func (o Optional[T]) IsPresent() bool {
	return o.present
}

// IsNil 是否为空（IsPresent 的反向）
func (o Optional[T]) IsNil() bool {
	return !o.present
}

// Get 返回值的指针；空 Optional 返回 nil
func (o Optional[T]) Get() *T {
	if !o.present {
		return nil
	}
	return &o.value
}

// GetOrZero 获取值，空 Optional 返回 T 的零值
func (o Optional[T]) GetOrZero() T {
	if !o.present {
		var zero T
		return zero
	}
	return o.value
}

// GetOrDefault 获取值，空 Optional 返回 defaultValue
func (o Optional[T]) GetOrDefault(defaultValue T) T {
	if !o.present {
		return defaultValue
	}
	return o.value
}

// OrElse 与 GetOrDefault 等价（Java Optional 习惯命名）
func (o Optional[T]) OrElse(defaultValue T) T {
	return o.GetOrDefault(defaultValue)
}

// IfPresent 有值时执行回调，无值时跳过
func (o Optional[T]) IfPresent(fn func(T)) {
	if o.present {
		fn(o.value)
	}
}

// IfPresentOrElse 有值时执行 action，无值时执行 emptyAction
func (o Optional[T]) IfPresentOrElse(action func(T), emptyAction func()) {
	if o.present {
		action(o.value)
	} else {
		emptyAction()
	}
}

// Filter 谓词为 false（或空 Optional）时返回空 Optional
func (o Optional[T]) Filter(pred func(T) bool) Optional[T] {
	if !o.present || !pred(o.value) {
		return Optional[T]{}
	}
	return o
}

// MapOpt 转换为 Optional[U]，fn 返回 *U 便于主动中断链路（返回 nil 即视为空）
// 注意：Go 方法不支持额外类型参数，因此单子操作以泛型函数形式提供
//
// 示例：
//
//	fb := types.MapOpt(types.OfNullable(tracking), func(t *TrackingConfig) *FacebookConfig {
//	    return t.Facebook
//	})
func MapOpt[T any, U any](o Optional[T], fn func(T) *U) Optional[U] {
	if !o.present {
		return Optional[U]{}
	}
	r := fn(o.value)
	if r == nil {
		return Optional[U]{}
	}
	return Optional[U]{value: *r, present: true}
}

// FlatMapOpt 单子绑定，fn 返回 Optional[U]
func FlatMapOpt[T any, U any](o Optional[T], fn func(T) Optional[U]) Optional[U] {
	if !o.present {
		return Optional[U]{}
	}
	return fn(o.value)
}
