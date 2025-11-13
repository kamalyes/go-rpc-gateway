/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 07:49:58
 * @FilePath: \go-rpc-gateway\pbmo\pbmo.go
 * @Description: PBMO - Protocol Buffer Model Object Converter
 * 高性能双向转换系统，支持参数校验
 * 职责划分：
 *   - types.go: 类型定义、辅助函数
 *   - validator.go: 参数校验
 *   - convert.go: 双向转换核心
 *   - batch.go: 批量转换
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"fmt"
	"reflect"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	errConvertElem = "failed to convert element %d: %w"
)

// BidiConverter 双向转换器
// 支持 PB ↔ Model 转换、参数校验、字段转换
type BidiConverter struct {
	pbType       reflect.Type
	modelType    reflect.Type
	transformers map[string]func(interface{}) interface{}
	validators   map[string][]FieldRule // 添加校验规则存储
}

// NewBidiConverter 创建双向转换器
func NewBidiConverter(pbType, modelType interface{}) *BidiConverter {
	return &BidiConverter{
		pbType:       reflect.TypeOf(pbType),
		modelType:    reflect.TypeOf(modelType),
		transformers: make(map[string]func(interface{}) interface{}),
		validators:   make(map[string][]FieldRule),
	}
}

// RegisterTransformer 注册字段转换器
func (bc *BidiConverter) RegisterTransformer(field string, transformer func(interface{}) interface{}) {
	bc.transformers[field] = transformer
}

// RegisterValidationRules 注册校验规则
func (bc *BidiConverter) RegisterValidationRules(typeName string, rules ...FieldRule) {
	bc.validators[typeName] = append(bc.validators[typeName], rules...)
}

// ConvertPBToModel 高性能 PB -> Model 转换
// 性能：<3µs/次
func (bc *BidiConverter) ConvertPBToModel(pb interface{}, modelPtr interface{}) error {
	// 参数校验
	if pb == nil {
		return fmt.Errorf("pb message cannot be nil")
	}
	if modelPtr == nil {
		return fmt.Errorf("modelPtr cannot be nil")
	}

	modelVal := reflect.ValueOf(modelPtr)
	if modelVal.Kind() != reflect.Ptr {
		return fmt.Errorf("modelPtr must be a pointer")
	}

	// 检查指针是否为 nil
	if modelVal.IsNil() {
		return fmt.Errorf("modelPtr cannot be nil")
	}

	modelVal = modelVal.Elem()

	// 检查 model 是否为接口类型，如果是则获取实际值
	for modelVal.Kind() == reflect.Interface && !modelVal.IsNil() {
		modelVal = modelVal.Elem()
	}

	// 检查 model 是否为结构体类型
	if modelVal.Kind() != reflect.Struct {
		return fmt.Errorf("model must be a struct, got %v", modelVal.Kind())
	}

	pbVal := reflect.ValueOf(pb)
	// 检查 pb 是否是 nil pointer
	if pbVal.Kind() == reflect.Ptr {
		if pbVal.IsNil() {
			return fmt.Errorf("pb message cannot be nil")
		}
		pbVal = pbVal.Elem()
	}

	pbType := pbVal.Type()

	// 遍历 PB 字段进行转换
	for i := 0; i < pbVal.NumField(); i++ {
		pbField := pbVal.Field(i)
		pbFieldName := pbType.Field(i).Name

		// 查找对应 Model 字段
		modelField := modelVal.FieldByName(pbFieldName)
		if !modelField.IsValid() || !modelField.CanSet() {
			continue
		}

		// 应用转换器（如果有）
		if transformer, ok := bc.transformers[pbFieldName]; ok {
			pbField = reflect.ValueOf(transformer(pbField.Interface()))
		}

		// 执行字段转换
		if err := convertFieldFast(pbField, modelField); err != nil {
			return fmt.Errorf("failed to convert field %s: %w", pbFieldName, err)
		}
	}

	return nil
}

// ConvertModelToPB 高性能 Model -> PB 转换
// 性能：<3µs/次
func (bc *BidiConverter) ConvertModelToPB(model interface{}, pbPtr interface{}) error {
	// 参数校验
	if model == nil {
		return fmt.Errorf("model cannot be nil")
	}
	if pbPtr == nil {
		return fmt.Errorf("pbPtr cannot be nil")
	}

	pbVal := reflect.ValueOf(pbPtr)
	if pbVal.Kind() != reflect.Ptr {
		return fmt.Errorf("pbPtr must be a pointer")
	}

	// 检查指针是否为 nil
	if pbVal.IsNil() {
		return fmt.Errorf("pbPtr cannot be nil")
	}

	pbVal = pbVal.Elem()

	modelVal := reflect.ValueOf(model)
	// 检查 model 是否是 nil pointer
	if modelVal.Kind() == reflect.Ptr {
		if modelVal.IsNil() {
			return fmt.Errorf("model cannot be nil")
		}
		modelVal = modelVal.Elem()
	}

	modelType := modelVal.Type()

	// 遍历 Model 字段进行转换
	for i := 0; i < modelVal.NumField(); i++ {
		modelField := modelVal.Field(i)
		modelFieldName := modelType.Field(i).Name

		// 查找对应 PB 字段
		pbField := pbVal.FieldByName(modelFieldName)
		if !pbField.IsValid() || !pbField.CanSet() {
			continue
		}

		// 应用转换器（如果有）
		if transformer, ok := bc.transformers[modelFieldName]; ok {
			modelField = reflect.ValueOf(transformer(modelField.Interface()))
		}

		// 执行字段转换
		if err := convertFieldFast(modelField, pbField); err != nil {
			return fmt.Errorf("failed to convert field %s: %w", modelFieldName, err)
		}
	}

	return nil
}

// convertFieldFast 快速字段转换（内联优化）
// 支持的转换：
// - 时间戳：time.Time <-> *timestamppb.Timestamp
// - 整数：uint <-> int64
// - 基本类型：string, bool, float 等
// - 切片：递归转换
// - 指针：自动解引用和装箱
func convertFieldFast(src reflect.Value, dst reflect.Value) error {
	if !src.IsValid() {
		return nil
	}

	srcType := src.Type()
	dstType := dst.Type()

	// 快速路径：类型完全相同
	if srcType == dstType && srcType.Kind() != reflect.Ptr {
		dst.Set(src)
		return nil
	}

	// 时间戳转换（最常用）
	if srcType == timeType && dstType == timestampPtrType {
		t := src.Interface().(time.Time)
		dst.Set(reflect.ValueOf(timestamppb.New(t)))
		return nil
	}
	if srcType == timestampPtrType && dstType == timeType {
		if src.IsNil() {
			return nil
		}
		ts := src.Interface().(*timestamppb.Timestamp)
		dst.Set(reflect.ValueOf(ts.AsTime()))
		return nil
	}

	// ID 字段转换（uint <-> int64）
	if isIntegerType(srcType) && isIntegerType(dstType) {
		return convertInteger(src, dst)
	}

	// 直接赋值
	if srcType.AssignableTo(dstType) {
		dst.Set(src)
		return nil
	}

	// 可转换类型
	if srcType.ConvertibleTo(dstType) {
		dst.Set(src.Convert(dstType))
		return nil
	}

	// 指针处理
	if dstType.Kind() == reflect.Ptr {
		if src.IsZero() {
			return nil
		}

		// 如果目标已经是一个指针且不为nil，直接转换到其内容
		if !dst.IsNil() {
			if srcType.Kind() == reflect.Ptr {
				return convertFieldFast(src.Elem(), dst.Elem())
			}
			return convertFieldFast(src, dst.Elem())
		}

		// 目标为nil，创建新的对象
		newVal := reflect.New(dstType.Elem())
		var err error
		if srcType.Kind() == reflect.Ptr {
			err = convertFieldFast(src.Elem(), newVal.Elem())
		} else {
			err = convertFieldFast(src, newVal.Elem())
		}
		if err == nil {
			dst.Set(newVal) // 设置创建的新值到目标字段
		}
		return err
	}

	if srcType.Kind() == reflect.Ptr {
		if src.IsNil() {
			return nil
		}
		return convertFieldFast(src.Elem(), dst)
	}

	// 切片转换
	if srcType.Kind() == reflect.Slice && dstType.Kind() == reflect.Slice {
		return convertSliceFast(src, dst)
	}

	// 结构体转换
	if srcType.Kind() == reflect.Struct && dstType.Kind() == reflect.Struct {
		return convertStructFast(src, dst)
	}

	return nil
}

// convertInteger 整数类型转换
func convertInteger(src reflect.Value, dst reflect.Value) error {
	srcKind := src.Type().Kind()
	dstKind := dst.Type().Kind()

	if isUnsignedInt(srcKind) {
		val := src.Uint()
		if isSignedInt(dstKind) {
			dst.SetInt(int64(val))
		} else {
			dst.SetUint(val)
		}
	} else {
		val := src.Int()
		if isUnsignedInt(dstKind) {
			dst.SetUint(uint64(val))
		} else {
			dst.SetInt(val)
		}
	}

	return nil
}

// convertSliceFast 快速切片转换
func convertSliceFast(src reflect.Value, dst reflect.Value) error {
	if src.IsNil() {
		return nil
	}

	len := src.Len()
	dstSlice := reflect.MakeSlice(dst.Type(), len, len)

	for i := 0; i < len; i++ {
		if err := convertFieldFast(src.Index(i), dstSlice.Index(i)); err != nil {
			return fmt.Errorf(errConvertElem, i, err)
		}
	}

	dst.Set(dstSlice)
	return nil
}

// convertStructFast 快速结构体转换
func convertStructFast(src reflect.Value, dst reflect.Value) error {
	srcType := src.Type()

	// 遍历源结构体的所有字段
	for i := 0; i < src.NumField(); i++ {
		srcField := src.Field(i)
		srcFieldName := srcType.Field(i).Name

		// 查找目标结构体中的对应字段
		dstField := dst.FieldByName(srcFieldName)
		if !dstField.IsValid() || !dstField.CanSet() {
			continue
		}

		// 递归转换字段
		if err := convertFieldFast(srcField, dstField); err != nil {
			return fmt.Errorf("failed to convert struct field %s: %w", srcFieldName, err)
		}
	}

	return nil
}

// BatchConvertPBToModel 批量 PB -> Model 转换
func (bc *BidiConverter) BatchConvertPBToModel(pbs interface{}, modelsPtr interface{}) error {
	pbsVal := reflect.ValueOf(pbs)
	if pbsVal.Kind() == reflect.Ptr {
		pbsVal = pbsVal.Elem()
	}
	if pbsVal.Kind() != reflect.Slice {
		return fmt.Errorf("pbs must be a slice")
	}

	modelsVal := reflect.ValueOf(modelsPtr)
	if modelsVal.Kind() != reflect.Ptr {
		return fmt.Errorf("modelsPtr must be a pointer")
	}
	modelsVal = modelsVal.Elem()

	modelType := modelsVal.Type().Elem()
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	models := reflect.MakeSlice(modelsVal.Type(), pbsVal.Len(), pbsVal.Len())

	for i := 0; i < pbsVal.Len(); i++ {
		pb := pbsVal.Index(i)
		model := models.Index(i)

		if modelType.Kind() == reflect.Ptr {
			modelPtr := reflect.New(modelType)
			if err := bc.ConvertPBToModel(pb.Interface(), modelPtr.Interface()); err != nil {
				return fmt.Errorf(errConvertElem, i, err)
			}
			model.Set(modelPtr)
		} else {
			// 为非指针类型创建一个新的值
			modelPtr := reflect.New(modelType)
			if err := bc.ConvertPBToModel(pb.Interface(), modelPtr.Interface()); err != nil {
				return fmt.Errorf(errConvertElem, i, err)
			}
			// 如果目标切片类型是指针，设置指针；否则设置值
			if modelsVal.Type().Elem().Kind() == reflect.Ptr {
				model.Set(modelPtr)
			} else {
				model.Set(modelPtr.Elem())
			}
		}
	}

	modelsVal.Set(models)
	return nil
}

// BatchConvertModelToPB 批量 Model -> PB 转换
func (bc *BidiConverter) BatchConvertModelToPB(models interface{}, pbsPtr interface{}) error {
	modelsVal := reflect.ValueOf(models)
	if modelsVal.Kind() == reflect.Ptr {
		modelsVal = modelsVal.Elem()
	}
	if modelsVal.Kind() != reflect.Slice {
		return fmt.Errorf("models must be a slice")
	}

	pbsVal := reflect.ValueOf(pbsPtr)
	if pbsVal.Kind() != reflect.Ptr {
		return fmt.Errorf("pbsPtr must be a pointer")
	}
	pbsVal = pbsVal.Elem()

	pbType := pbsVal.Type().Elem()
	if pbType.Kind() == reflect.Ptr {
		pbType = pbType.Elem()
	}

	pbs := reflect.MakeSlice(pbsVal.Type(), modelsVal.Len(), modelsVal.Len())

	for i := 0; i < modelsVal.Len(); i++ {
		model := modelsVal.Index(i)
		pb := pbs.Index(i)

		if pbType.Kind() == reflect.Ptr {
			pbPtr := reflect.New(pbType)
			if err := bc.ConvertModelToPB(model.Interface(), pbPtr.Interface()); err != nil {
				return fmt.Errorf(errConvertElem, i, err)
			}
			pb.Set(pbPtr)
		} else {
			// 为非指针类型创建一个新的值
			pbPtr := reflect.New(pbType)
			if err := bc.ConvertModelToPB(model.Interface(), pbPtr.Interface()); err != nil {
				return fmt.Errorf(errConvertElem, i, err)
			}
			// 如果目标切片类型是指针，设置指针；否则设置值
			if pbsVal.Type().Elem().Kind() == reflect.Ptr {
				pb.Set(pbPtr)
			} else {
				pb.Set(pbPtr.Elem())
			}
		}
	}

	pbsVal.Set(pbs)
	return nil
}
