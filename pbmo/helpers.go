/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 07:49:58
 * @FilePath: \go-rpc-gateway\pbmo\helpers.go
 * @Description: 类型定义和辅助函数
 * 职责：共用类型定义、反射辅助函数、类型判断
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"reflect"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// Converter 转换器接口
type Converter interface {
	ToPB() interface{}
	FromPB(interface{}) error
	Validate() error
}

// 常用类型缓存
var (
	timeType         = reflect.TypeOf(time.Time{})
	timestampPtrType = reflect.TypeOf((*timestamppb.Timestamp)(nil))
)

// isZeroValue 判断是否为零值
func isZeroValue(v reflect.Value) bool {
	return v.Interface() == reflect.Zero(v.Type()).Interface()
}

// isNumeric 判断是否为数值类型
func isNumeric(v reflect.Value) bool {
	kind := v.Kind()
	return kind >= reflect.Int && kind <= reflect.Float64
}

// getNumericValue 获取数值
func getNumericValue(v reflect.Value) float64 {
	kind := v.Kind()
	if kind >= reflect.Int && kind <= reflect.Int64 {
		return float64(v.Int())
	}
	if kind >= reflect.Uint && kind <= reflect.Uint64 {
		return float64(v.Uint())
	}
	return v.Float()
}

// isIntegerType 判断是否为整数类型
func isIntegerType(t reflect.Type) bool {
	kind := t.Kind()
	return (kind >= reflect.Int && kind <= reflect.Int64) ||
		(kind >= reflect.Uint && kind <= reflect.Uint64)
}

// isSignedInt 判断是否为有符号整数
func isSignedInt(kind reflect.Kind) bool {
	return kind >= reflect.Int && kind <= reflect.Int64
}

// isUnsignedInt 判断是否为无符号整数
func isUnsignedInt(kind reflect.Kind) bool {
	return kind >= reflect.Uint && kind <= reflect.Uint64
}

// isFloatType 判断是否为浮点数类型
func isFloatType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

// getTypeName 获取类型名称
func getTypeName(t reflect.Type) string {
	if t == nil {
		return "nil"
	}
	if t.Kind() == reflect.Ptr {
		return "*" + t.Elem().Name()
	}
	return t.Name()
}
