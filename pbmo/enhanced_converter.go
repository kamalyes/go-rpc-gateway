/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-14 09:45:09
 * @FilePath: \go-rpc-gateway\pbmo\enhanced_converter.go
 * @Description: 增强的双向转换器 - 集成错误处理和日志
 * 职责：高级转换功能、自动错误处理、日志记录、性能监控
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"fmt"
	"reflect"
	"time"

	"github.com/kamalyes/go-logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// EnhancedBidiConverter 增强的双向转换器
// 自动处理错误、记录日志、性能监控
type EnhancedBidiConverter struct {
	*BidiConverter
	logger        logger.ILogger
	errorHandler  *ConversionErrorHandler
	enableMetrics bool
	metrics       *ConversionMetrics
}

// ConversionMetrics 转换性能指标
type ConversionMetrics struct {
	TotalConversions      int64
	SuccessfulConversions int64
	FailedConversions     int64
	TotalDuration         time.Duration
	AverageDuration       time.Duration
	LastError             error
}

// NewEnhancedBidiConverter 创建增强的双向转换器
func NewEnhancedBidiConverter(pbType, modelType interface{}, log logger.ILogger) *EnhancedBidiConverter {
	return &EnhancedBidiConverter{
		BidiConverter: NewBidiConverter(pbType, modelType),
		logger:        log,
		errorHandler:  NewConversionErrorHandler(log),
		enableMetrics: true,
		metrics: &ConversionMetrics{
			TotalConversions:      0,
			SuccessfulConversions: 0,
			FailedConversions:     0,
		},
	}
}

// ConvertPBToModelWithLog 带日志的 PB -> Model 转换
func (ebc *EnhancedBidiConverter) ConvertPBToModelWithLog(pb interface{}, modelPtr interface{}) error {
	start := time.Now()
	ebc.metrics.TotalConversions++

	pbType := getTypeName(reflect.TypeOf(pb))
	modelType := getTypeName(reflect.TypeOf(modelPtr))

	ebc.errorHandler.LogConversionStart(pbType, modelType)

	// 执行转换
	err := ebc.BidiConverter.ConvertPBToModel(pb, modelPtr)

	duration := time.Since(start)
	ebc.updateMetrics(duration, err)

	if err != nil {
		ebc.metrics.FailedConversions++
		ebc.metrics.LastError = err

		if ebc.logger != nil {
			ebc.logger.Error("❌ PB->Model conversion failed (%s->%s) in %v: %v",
				pbType, modelType, duration, err)
		}
		return ebc.errorHandler.HandleConversionError(err, pbType+"->"+modelType)
	}

	ebc.metrics.SuccessfulConversions++
	ebc.errorHandler.LogConversionSuccess(pbType, modelType)

	if ebc.logger != nil {
		ebc.logger.Debug("⏱️  PB->Model conversion completed in %v", duration)
	}

	return nil
}

// ConvertModelToPBWithLog 带日志的 Model -> PB 转换
func (ebc *EnhancedBidiConverter) ConvertModelToPBWithLog(model interface{}, pbPtr interface{}) error {
	start := time.Now()
	ebc.metrics.TotalConversions++

	modelType := getTypeName(reflect.TypeOf(model))
	pbType := getTypeName(reflect.TypeOf(pbPtr))

	ebc.errorHandler.LogConversionStart(modelType, pbType)

	// 执行转换
	err := ebc.BidiConverter.ConvertModelToPB(model, pbPtr)

	duration := time.Since(start)
	ebc.updateMetrics(duration, err)

	if err != nil {
		ebc.metrics.FailedConversions++
		ebc.metrics.LastError = err

		if ebc.logger != nil {
			ebc.logger.Error("❌ Model->PB conversion failed (%s->%s) in %v: %v",
				modelType, pbType, duration, err)
		}
		return ebc.errorHandler.HandleConversionError(err, modelType+"->"+pbType)
	}

	ebc.metrics.SuccessfulConversions++
	ebc.errorHandler.LogConversionSuccess(modelType, pbType)

	if ebc.logger != nil {
		ebc.logger.Debug("⏱️  Model->PB conversion completed in %v", duration)
	}

	return nil
}

// ValidateWithLog 带日志的参数校验
func (ebc *EnhancedBidiConverter) ValidateWithLog(validator FieldValidator, data interface{}) error {
	start := time.Now()

	dataType := getTypeName(reflect.TypeOf(data))
	if ebc.logger != nil {
		ebc.logger.Debug("🔍 Validating %s", dataType)
	}

	// 执行校验
	err := validator.Validate(data)

	duration := time.Since(start)

	if err != nil {
		if ebc.logger != nil {
			ebc.logger.Warn("⚠️  Validation failed for %s in %v: %v", dataType, duration, err)
		}
		return ebc.errorHandler.HandleValidationError(err)
	}

	ebc.errorHandler.LogValidationSuccess(dataType)
	if ebc.logger != nil {
		ebc.logger.Debug("⏱️  Validation completed in %v", duration)
	}

	return nil
}

// BatchConvertWithErrorCollection 批量转换 - 收集所有错误
type BatchConversionResult struct {
	SuccessCount int
	FailureCount int
	Errors       []error
	Duration     time.Duration
}

// ConvertPBToModelBatchSafe 安全的批量 PB->Model 转换
// 继续处理即使有单个项目失败，收集所有错误
func (ebc *EnhancedBidiConverter) ConvertPBToModelBatchSafe(
	pbs interface{},
	modelsPtr interface{},
) *BatchConversionResult {
	start := time.Now()
	result := &BatchConversionResult{
		Errors: make([]error, 0),
	}

	pbsVal := reflect.ValueOf(pbs)
	if pbsVal.Kind() == reflect.Ptr {
		pbsVal = pbsVal.Elem()
	}

	if pbsVal.Kind() != reflect.Slice {
		err := fmt.Errorf("pbs must be a slice")
		result.Errors = append(result.Errors, err)
		result.Duration = time.Since(start)
		return result
	}

	modelsVal := reflect.ValueOf(modelsPtr)
	if modelsVal.Kind() != reflect.Ptr {
		err := fmt.Errorf("modelsPtr must be a pointer")
		result.Errors = append(result.Errors, err)
		result.Duration = time.Since(start)
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
		model := models.Index(i)

		if modelType.Kind() == reflect.Ptr {
			modelPtr := reflect.New(modelType)
			if err := ebc.BidiConverter.ConvertPBToModel(pb.Interface(), modelPtr.Interface()); err != nil {
				result.FailureCount++
				result.Errors = append(result.Errors, err)

				if ebc.logger != nil {
					ebc.logger.Warn("⚠️  Batch item %d conversion failed: %v", i, err)
				}
				continue
			}
			model.Set(modelPtr)
		} else {
			if err := ebc.BidiConverter.ConvertPBToModel(pb.Interface(), model.Addr().Interface()); err != nil {
				result.FailureCount++
				result.Errors = append(result.Errors, err)

				if ebc.logger != nil {
					ebc.logger.Warn("⚠️  Batch item %d conversion failed: %v", i, err)
				}
				continue
			}
		}

		result.SuccessCount++
	}

	modelsVal.Set(models)
	result.Duration = time.Since(start)

	if ebc.logger != nil {
		ebc.logger.Info("📦 Batch conversion completed: %d success, %d failures in %v",
			result.SuccessCount, result.FailureCount, result.Duration)
	}

	return result
}

// updateMetrics 更新性能指标
func (ebc *EnhancedBidiConverter) updateMetrics(duration time.Duration, err error) {
	ebc.metrics.TotalDuration += duration

	if ebc.metrics.TotalConversions > 0 {
		ebc.metrics.AverageDuration = ebc.metrics.TotalDuration / time.Duration(ebc.metrics.TotalConversions)
	}
}

// GetMetrics 获取性能指标
func (ebc *EnhancedBidiConverter) GetMetrics() *ConversionMetrics {
	return ebc.metrics
}

// ResetMetrics 重置指标
func (ebc *EnhancedBidiConverter) ResetMetrics() {
	ebc.metrics = &ConversionMetrics{}
}

// GetGRPCErrorFromConversion 从转换结果获取 gRPC 错误
func GetGRPCErrorFromConversion(err error) error {
	if err == nil {
		return nil
	}

	grpcCode, msg := ErrorToGRPCStatus(err)
	return status.Errorf(grpcCode, msg)
}

// GetGRPCErrorFromValidation 从校验结果获取 gRPC 错误
func GetGRPCErrorFromValidation(validationErr error) error {
	if validationErr == nil {
		return nil
	}

	return status.Errorf(codes.InvalidArgument, "validation failed: %v", validationErr)
}
