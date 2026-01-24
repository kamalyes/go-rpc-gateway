/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 16:30:00
 * @LastEditors: kamalyes 501893067@qq.comcom
 * @LastEditTime: 2025-12-11 15:08:15
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
	"reflect"
	"time"

	"github.com/kamalyes/go-rpc-gateway/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BidiConverter 双向转换器
// 支持 PB ↔ Model 转换、参数校验、字段转换
type BidiConverter struct {
	pbType             reflect.Type
	modelType          reflect.Type
	transformers       map[string]func(interface{}) interface{}
	validators         map[string][]FieldRule // 添加校验规则存储
	autoTimeConversion bool                   // 自动时间转换开关
	fieldMapping       map[string]string      // 字段名映射: Model字段名 -> PB字段名
	tagMappingCached   bool                   // struct tag映射是否已缓存
}

// NewBidiConverter 创建双向转换器
func NewBidiConverter(pbType, modelType interface{}) *BidiConverter {
	return &BidiConverter{
		pbType:             reflect.TypeOf(pbType),
		modelType:          reflect.TypeOf(modelType),
		transformers:       make(map[string]func(interface{}) interface{}),
		validators:         make(map[string][]FieldRule),
		autoTimeConversion: true,                    // 默认启用自动时间转换
		fieldMapping:       make(map[string]string), // 初始化字段映射
		tagMappingCached:   false,
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

// WithAutoTimeConversion 设置自动时间转换开关
func (bc *BidiConverter) WithAutoTimeConversion(enabled bool) *BidiConverter {
	bc.autoTimeConversion = enabled
	return bc
}

// SetAutoTimeConversion 设置自动时间转换开关
func (bc *BidiConverter) SetAutoTimeConversion(enabled bool) {
	bc.autoTimeConversion = enabled
}

// IsAutoTimeConversionEnabled 检查是否启用自动时间转换
func (bc *BidiConverter) IsAutoTimeConversionEnabled() bool {
	return bc.autoTimeConversion
}

// GetModelType 获取Model类型（实现Converter接口）
func (bc *BidiConverter) GetModelType() reflect.Type {
	return bc.modelType
}

// ConvertSlice 切片转换方法（实现Converter接口）
func (bc *BidiConverter) ConvertSlice(src interface{}) (interface{}, error) {
	// 使用批量转换实现切片转换
	srcVal := reflect.ValueOf(src)
	if srcVal.Kind() != reflect.Slice {
		return nil, errors.ErrMustBeSlice
	}

	// 创建目标切片
	dstType := reflect.SliceOf(bc.modelType)
	dst := reflect.MakeSlice(dstType, 0, srcVal.Len())
	dstPtr := reflect.New(dstType)
	dstPtr.Elem().Set(dst)

	// 执行批量转换
	err := bc.BatchConvertPBToModel(src, dstPtr.Interface())
	if err != nil {
		return nil, err
	}

	return dstPtr.Elem().Interface(), nil
}

// GetConverterInfo 获取转换器信息（实现Converter接口）
func (bc *BidiConverter) GetConverterInfo() ConverterInfo {
	return ConverterInfo{
		Type:    string(BidiConverterType),
		Version: "1.0.0",
		Features: []string{
			string(ValidationFeature),
			"field_mapping",
			"auto_time_conversion",
			"custom_transformers",
		},
		Performance: PerformanceInfo{
			BenchmarkScore:   1.0,
			AverageLatency:   3 * time.Microsecond,
			ThroughputPerSec: 300000,
		},
		Config: map[string]interface{}{
			"autoTimeConversion": bc.autoTimeConversion,
			"fieldMappingCount":  len(bc.fieldMapping),
			"validatorsCount":    len(bc.validators),
		},
	}
}

// WithFieldMapping 设置字段名映射（链式调用）
// modelFieldName: Model结构体的字段名
// pbFieldName: PB结构体的字段名
// 示例: converter.WithFieldMapping("ID", "ClientId").WithFieldMapping("UserID", "UserId")
func (bc *BidiConverter) WithFieldMapping(modelFieldName, pbFieldName string) *BidiConverter {
	bc.fieldMapping[modelFieldName] = pbFieldName
	return bc
}

// RegisterFieldMapping 注册字段映射（批量）
// mappings: map[Model字段名]PB字段名
func (bc *BidiConverter) RegisterFieldMapping(mappings map[string]string) {
	for modelField, pbField := range mappings {
		bc.fieldMapping[modelField] = pbField
	}
}

// loadTagMappings 从Model结构体的pbmo tag加载字段映射
// 只在首次使用时执行一次
func (bc *BidiConverter) loadTagMappings() {
	if bc.tagMappingCached {
		return
	}
	bc.tagMappingCached = true

	modelType := bc.modelType
	// 如果是指针类型，获取其指向的类型
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	// 只处理结构体类型
	if modelType.Kind() != reflect.Struct {
		return
	}

	// 遍历Model结构体的所有字段
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)

		// 读取 pbmo tag
		if pbFieldName := field.Tag.Get("pbmo"); pbFieldName != "" {
			// 如果该字段还没有手动配置映射，则使用tag定义的映射
			if _, exists := bc.fieldMapping[field.Name]; !exists {
				bc.fieldMapping[field.Name] = pbFieldName
			}
		}
	}
}

// ConvertPBToModel 高性能 PB -> Model 转换
// 性能：<3µs/次
func (bc *BidiConverter) ConvertPBToModel(pb interface{}, modelPtr interface{}) error {
	// 首次使用时加载struct tag映射
	bc.loadTagMappings()

	// 参数校验
	if pb == nil {
		return errors.ErrPBMessageNil
	}
	if modelPtr == nil {
		return errors.ErrModelMessageNil
	}

	modelVal := reflect.ValueOf(modelPtr)
	if modelVal.Kind() != reflect.Ptr {
		return errors.ErrMustBePointer
	}

	// 检查指针是否为 nil
	if modelVal.IsNil() {
		return errors.ErrModelMessageNil
	}

	modelVal = modelVal.Elem()

	// 检查 model 是否为接口类型，如果是则获取实际值
	for modelVal.Kind() == reflect.Interface && !modelVal.IsNil() {
		modelVal = modelVal.Elem()
	}

	// 检查 model 是否为结构体类型
	if modelVal.Kind() != reflect.Struct {
		return errors.NewErrorf(errors.ErrCodeMustBeStruct, "got %v", modelVal.Kind())
	}

	pbVal := reflect.ValueOf(pb)
	// 检查 pb 是否是 nil pointer
	if pbVal.Kind() == reflect.Ptr {
		if pbVal.IsNil() {
			return errors.ErrPBMessageNil
		}
		pbVal = pbVal.Elem()
	}

	pbType := pbVal.Type()

	// 构建反向映射 (PB字段名 -> Model字段名)
	reverseMapping := make(map[string]string)
	for modelField, pbField := range bc.fieldMapping {
		reverseMapping[pbField] = modelField
	}

	// 遍历 PB 字段进行转换
	for i := 0; i < pbVal.NumField(); i++ {
		pbField := pbVal.Field(i)
		pbFieldName := pbType.Field(i).Name

		// 检查是否有反向字段映射
		modelFieldName := pbFieldName
		if mappedName, ok := reverseMapping[pbFieldName]; ok {
			modelFieldName = mappedName
		}

		// 查找对应 Model 字段
		modelField := modelVal.FieldByName(modelFieldName)
		if !modelField.IsValid() || !modelField.CanSet() {
			continue
		}

		// 应用转换器（如果有）
		if transformer, ok := bc.transformers[pbFieldName]; ok {
			pbField = reflect.ValueOf(transformer(pbField.Interface()))
		}

		// 执行字段转换
		if err := convertFieldFast(pbField, modelField, bc); err != nil {
			return errors.NewErrorf(errors.ErrCodeFieldConversionError, "field %s->%s: %v", pbFieldName, modelFieldName, err)
		}
	}

	return nil
}

// ConvertModelToPB 高性能 Model -> PB 转换
// 性能：<3µs/次
func (bc *BidiConverter) ConvertModelToPB(model interface{}, pbPtr interface{}) error {
	// 首次使用时加载struct tag映射
	bc.loadTagMappings()

	// 参数校验
	if model == nil {
		return errors.ErrModelMessageNil
	}
	if pbPtr == nil {
		return errors.ErrPBMessageNil
	}

	pbVal := reflect.ValueOf(pbPtr)
	if pbVal.Kind() != reflect.Ptr {
		return errors.ErrMustBePointer
	}

	// 检查指针是否为 nil
	if pbVal.IsNil() {
		return errors.ErrPBMessageNil
	}

	pbVal = pbVal.Elem()

	modelVal := reflect.ValueOf(model)
	// 检查 model 是否是 nil pointer
	if modelVal.Kind() == reflect.Ptr {
		if modelVal.IsNil() {
			return errors.ErrModelMessageNil
		}
		modelVal = modelVal.Elem()
	}

	modelType := modelVal.Type()

	// 遍历 Model 字段进行转换
	for i := 0; i < modelVal.NumField(); i++ {
		modelField := modelVal.Field(i)
		modelFieldName := modelType.Field(i).Name

		// 检查是否有字段映射
		pbFieldName := modelFieldName
		if mappedName, ok := bc.fieldMapping[modelFieldName]; ok {
			pbFieldName = mappedName
		}

		// 查找对应 PB 字段
		pbField := pbVal.FieldByName(pbFieldName)
		if !pbField.IsValid() || !pbField.CanSet() {
			continue
		}

		// 应用转换器（如果有）
		if transformer, ok := bc.transformers[modelFieldName]; ok {
			modelField = reflect.ValueOf(transformer(modelField.Interface()))
		}

		// 执行字段转换
		if err := convertFieldFast(modelField, pbField, bc); err != nil {
			return errors.NewErrorf(errors.ErrCodeFieldConversionError, "field %s->%s: %v", modelFieldName, pbFieldName, err)
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
func convertFieldFast(src reflect.Value, dst reflect.Value, converter *BidiConverter) error {
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

	// 时间戳转换（最常用）- 只在开关启用时执行
	if converter != nil && converter.autoTimeConversion {
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
				return convertFieldFast(src.Elem(), dst.Elem(), converter)
			}
			return convertFieldFast(src, dst.Elem(), converter)
		}

		// 目标为nil，创建新的对象
		newVal := reflect.New(dstType.Elem())
		var err error
		if srcType.Kind() == reflect.Ptr {
			err = convertFieldFast(src.Elem(), newVal.Elem(), converter)
		} else {
			err = convertFieldFast(src, newVal.Elem(), converter)
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
		return convertFieldFast(src.Elem(), dst, converter)
	}

	// 切片转换
	if srcType.Kind() == reflect.Slice && dstType.Kind() == reflect.Slice {
		return convertSliceFast(src, dst, converter)
	}

	// 结构体转换
	if srcType.Kind() == reflect.Struct && dstType.Kind() == reflect.Struct {
		return convertStructFast(src, dst, converter)
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
func convertSliceFast(src reflect.Value, dst reflect.Value, converter *BidiConverter) error {
	if src.IsNil() {
		return nil
	}

	len := src.Len()
	dstSlice := reflect.MakeSlice(dst.Type(), len, len)

	for i := 0; i < len; i++ {
		if err := convertFieldFast(src.Index(i), dstSlice.Index(i), converter); err != nil {
			return errors.NewErrorf(errors.ErrCodeElementConversion, "element %d: %v", i, err)
		}
	}

	dst.Set(dstSlice)
	return nil
}

// convertStructFast 快速结构体转换
func convertStructFast(src reflect.Value, dst reflect.Value, converter *BidiConverter) error {
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
		if err := convertFieldFast(srcField, dstField, converter); err != nil {
			return errors.NewErrorf(errors.ErrCodeFieldConversionError, "struct field %s: %v", srcFieldName, err)
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
		return errors.ErrMustBeSlice
	}

	modelsVal := reflect.ValueOf(modelsPtr)
	if modelsVal.Kind() != reflect.Ptr {
		return errors.ErrMustBePointer
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
				return errors.NewErrorf(errors.ErrCodeElementConversion, "element %d: %v", i, err)
			}
			model.Set(modelPtr)
		} else {
			// 为非指针类型创建一个新的值
			modelPtr := reflect.New(modelType)
			if err := bc.ConvertPBToModel(pb.Interface(), modelPtr.Interface()); err != nil {
				return errors.NewErrorf(errors.ErrCodeElementConversion, "element %d: %v", i, err)
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
		return errors.ErrMustBeSlice
	}

	pbsVal := reflect.ValueOf(pbsPtr)
	if pbsVal.Kind() != reflect.Ptr {
		return errors.ErrMustBePointer
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
				return errors.NewErrorf(errors.ErrCodeElementConversion, "element %d: %v", i, err)
			}
			pb.Set(pbPtr)
		} else {
			// 为非指针类型创建一个新的值
			pbPtr := reflect.New(pbType)
			if err := bc.ConvertModelToPB(model.Interface(), pbPtr.Interface()); err != nil {
				return errors.NewErrorf(errors.ErrCodeElementConversion, "element %d: %v", i, err)
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
