/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-13 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 14:30:00
 * @FilePath: \go-rpc-gateway\pbmo\safe_converter.go
 * @Description: 安全转换器 - 集成 go-toolbox/safe 模块
 * 职责：安全的字段访问和转换，避免 nil panic
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"fmt"
	"reflect"

	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-toolbox/pkg/safe"
)

// SafeConverter 安全转换器
// 使用 SafeAccess 进行链式安全访问，避免 nil panic
type SafeConverter struct {
	*BidiConverter
	safeMode bool // 启用安全模式
}

// NewSafeConverter 创建安全转换器
func NewSafeConverter(pbType, modelType interface{}) *SafeConverter {
	return &SafeConverter{
		BidiConverter: NewBidiConverter(pbType, modelType),
		safeMode:      true,
	}
}

// SafeConvertPBToModel 安全的 PB -> Model 转换
// 自动处理 nil 值，避免 panic
func (sc *SafeConverter) SafeConvertPBToModel(pb interface{}, modelPtr interface{}) error {
	// 使用 SafeAccess 包装 pb，避免 nil panic
	if pb == nil {
		return NewConversionError("pb message cannot be nil", "SafeConvertPBToModel", "", "")
	}

	// 基础转换
	if err := sc.BidiConverter.ConvertPBToModel(pb, modelPtr); err != nil {
		return err
	}

	return nil
}

// SafeConvertModelToPB 安全的 Model -> PB 转换
func (sc *SafeConverter) SafeConvertModelToPB(model interface{}, pbPtr interface{}) error {
	if model == nil {
		return NewConversionError("model cannot be nil", "SafeConvertModelToPB", "", "")
	}

	// 基础转换
	if err := sc.BidiConverter.ConvertModelToPB(model, pbPtr); err != nil {
		return err
	}

	return nil
}

// SafeFieldAccess 安全字段访问
// 使用链式调用访问嵌套字段，避免 nil panic
func (sc *SafeConverter) SafeFieldAccess(obj interface{}, fieldNames ...string) *safe.SafeAccess {
	sa := safe.Safe(obj)
	for _, name := range fieldNames {
		sa = sa.Field(name)
	}
	return sa
}

// SafeBatchConvert 安全的批量转换
// 返回详细的转换结果，包括失败原因
type SafeBatchResult struct {
	SuccessCount int
	FailureCount int
	Results      []SafeConvertItem
	Duration     int64 // 毫秒
}

// SafeConvertItem 单个转换结果
type SafeConvertItem struct {
	Index   int
	Success bool
	Value   interface{}
	Error   error
}

// SafeBatchConvertPBToModel 安全批量 PB -> Model 转换
// 返回详细的转换结果，不因为单个失败而中断
func (sc *SafeConverter) SafeBatchConvertPBToModel(
	pbs interface{},
	modelsPtr interface{},
) *SafeBatchResult {
	result := &SafeBatchResult{
		Results: make([]SafeConvertItem, 0),
	}

	// 参数检查
	pbsVal := reflect.ValueOf(pbs)
	if pbsVal.Kind() == reflect.Ptr {
		pbsVal = pbsVal.Elem()
	}

	if pbsVal.Kind() != reflect.Slice {
		return result
	}

	modelsVal := reflect.ValueOf(modelsPtr)
	if modelsVal.Kind() != reflect.Ptr {
		return result
	}

	modelsVal = modelsVal.Elem()
	modelType := modelsVal.Type().Elem()
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	models := reflect.MakeSlice(modelsVal.Type(), pbsVal.Len(), pbsVal.Len())

	for i := 0; i < pbsVal.Len(); i++ {
		pb := pbsVal.Index(i)
		item := SafeConvertItem{Index: i}

		// 使用 SafeAccess 检查 nil
		sa := safe.Safe(pb.Interface())
		if !sa.IsValid() {
			item.Success = false
			item.Error = errors.NewErrorf(errors.ErrCodeItemNil, "item %d", i)
			result.FailureCount++
			result.Results = append(result.Results, item)
			continue
		}

		model := models.Index(i)
		if modelType.Kind() == reflect.Ptr {
			modelPtr := reflect.New(modelType)
			if err := sc.BidiConverter.ConvertPBToModel(pb.Interface(), modelPtr.Interface()); err != nil {
				item.Success = false
				item.Error = err
				result.FailureCount++
			} else {
				item.Success = true
				item.Value = modelPtr.Interface()
				result.SuccessCount++
			}
			model.Set(reflect.ValueOf(item.Value))
		} else {
			// 为非指针类型创建一个新的值
			modelPtr := reflect.New(modelType)
			if err := sc.BidiConverter.ConvertPBToModel(pb.Interface(), modelPtr.Interface()); err != nil {
				item.Success = false
				item.Error = err
				result.FailureCount++
			} else {
				item.Success = true
				// 如果目标切片类型是指针，设置指针；否则设置值
				if modelsVal.Type().Elem().Kind() == reflect.Ptr {
					item.Value = modelPtr.Interface()
					model.Set(modelPtr)
				} else {
					item.Value = modelPtr.Elem().Interface()
					model.Set(modelPtr.Elem())
				}
				result.SuccessCount++
			}
		}

		result.Results = append(result.Results, item)
	}

	modelsVal.Set(models)
	return result
}

// SafeBatchConvertModelToPB 安全批量 Model -> PB 转换
func (sc *SafeConverter) SafeBatchConvertModelToPB(
	models interface{},
	pbsPtr interface{},
) *SafeBatchResult {
	result := &SafeBatchResult{
		Results: make([]SafeConvertItem, 0),
	}

	modelsVal := reflect.ValueOf(models)
	if modelsVal.Kind() == reflect.Ptr {
		modelsVal = modelsVal.Elem()
	}

	if modelsVal.Kind() != reflect.Slice {
		return result
	}

	pbsVal := reflect.ValueOf(pbsPtr)
	if pbsVal.Kind() != reflect.Ptr {
		return result
	}

	pbsVal = pbsVal.Elem()
	pbType := pbsVal.Type().Elem()
	if pbType.Kind() == reflect.Ptr {
		pbType = pbType.Elem()
	}

	pbs := reflect.MakeSlice(pbsVal.Type(), modelsVal.Len(), modelsVal.Len())

	for i := 0; i < modelsVal.Len(); i++ {
		model := modelsVal.Index(i)
		item := SafeConvertItem{Index: i}

		// 使用 SafeAccess 检查 nil
		sa := safe.Safe(model.Interface())
		if !sa.IsValid() {
			item.Success = false
			item.Error = errors.NewErrorf(errors.ErrCodeItemNil, "item %d", i)
			result.FailureCount++
			result.Results = append(result.Results, item)
			continue
		}

		pb := pbs.Index(i)
		if pbType.Kind() == reflect.Ptr {
			pbPtr := reflect.New(pbType)
			if err := sc.BidiConverter.ConvertModelToPB(model.Interface(), pbPtr.Interface()); err != nil {
				item.Success = false
				item.Error = err
				result.FailureCount++
			} else {
				item.Success = true
				item.Value = pbPtr.Interface()
				result.SuccessCount++
			}
			pb.Set(reflect.ValueOf(item.Value))
		} else {
			if err := sc.BidiConverter.ConvertModelToPB(model.Interface(), pb.Addr().Interface()); err != nil {
				item.Success = false
				item.Error = err
				result.FailureCount++
			} else {
				item.Success = true
				item.Value = pb.Interface()
				result.SuccessCount++
			}
		}

		result.Results = append(result.Results, item)
	}

	pbsVal.Set(pbs)
	return result
}

// ConversionError 转换错误类型
type ConversionError struct {
	Message    string
	Operation  string
	SourceType string
	TargetType string
}

// NewConversionError 创建转换错误
func NewConversionError(msg, operation, source, target string) *ConversionError {
	return &ConversionError{
		Message:    msg,
		Operation:  operation,
		SourceType: source,
		TargetType: target,
	}
}

// Error 实现 error 接口
func (e *ConversionError) Error() string {
	return fmt.Sprintf("[%s] %s (from %s to %s)", e.Operation, e.Message, e.SourceType, e.TargetType)
}

// Is 支持 errors.Is 检查
func (e *ConversionError) Is(target error) bool {
	_, ok := target.(*ConversionError)
	return ok
}
