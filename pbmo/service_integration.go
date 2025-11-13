/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 21:08:42
 * @FilePath: \go-rpc-gateway\pbmo\service_integration.go
 * @Description: gRPC 服务集成适配器
 * 职责：自动转换拦截、参数校验拦截、错误处理
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"github.com/kamalyes/go-logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ServiceIntegration gRPC 服务集成工具
type ServiceIntegration struct {
	converter      *EnhancedBidiConverter
	validator      *FieldValidator
	logger         logger.ILogger
	errorHandler   *ConversionErrorHandler
}

// NewServiceIntegration 创建服务集成工具
func NewServiceIntegration(
	pbType, modelType interface{},
	log logger.ILogger,
) *ServiceIntegration {
	return &ServiceIntegration{
		converter:    NewEnhancedBidiConverter(pbType, modelType, log),
		validator:    NewFieldValidator(),
		logger:       log,
		errorHandler: NewConversionErrorHandler(log),
	}
}

// ConvertAndValidatePBToModel 转换并校验 PB -> Model
func (si *ServiceIntegration) ConvertAndValidatePBToModel(
	pb interface{},
	modelPtr interface{},
) error {
	// 1. 转换
	if err := si.converter.ConvertPBToModelWithLog(pb, modelPtr); err != nil {
		return err
	}

	// 2. 校验
	if err := si.converter.ValidateWithLog(*si.validator, modelPtr); err != nil {
		return err
	}

	return nil
}

// ConvertAndValidateModelToPB 转换并校验 Model -> PB
func (si *ServiceIntegration) ConvertAndValidateModelToPB(
	model interface{},
	pbPtr interface{},
) error {
	// 1. 校验源模型
	if err := si.converter.ValidateWithLog(*si.validator, model); err != nil {
		return err
	}

	// 2. 转换
	if err := si.converter.ConvertModelToPBWithLog(model, pbPtr); err != nil {
		return err
	}

	return nil
}

// BatchConvertSafe 安全的批量转换（继续处理失败项）
func (si *ServiceIntegration) BatchConvertSafe(
	pbs interface{},
	modelsPtr interface{},
) (*BatchConversionResult, error) {
	result := si.converter.ConvertPBToModelBatchSafe(pbs, modelsPtr)

	if len(result.Errors) > 0 && result.FailureCount > 0 {
		// 返回部分成功结果和错误信息
		errMsg := "partial batch conversion: some items failed"
		return result, status.Errorf(codes.Internal, errMsg)
	}

	return result, nil
}

// HandleError 处理错误并返回 gRPC 状态
func (si *ServiceIntegration) HandleError(err error, operationType string) error {
	if err == nil {
		return nil
	}

	if si.logger != nil {
		si.logger.Error("❌ Operation %s failed: %v", operationType, err)
	}

	grpcCode, msg := ErrorToGRPCStatus(err)
	return status.Errorf(grpcCode, msg)
}

// HandleValidationErrorWithDetails 处理校验错误并返回详细信息
func (si *ServiceIntegration) HandleValidationErrorWithDetails(validationErr error) error {
	if validationErr == nil {
		return nil
	}

	if si.logger != nil {
		si.logger.Warn("⚠️  Validation error: %v", validationErr)
	}

	// 如果是 ValidationErrors 类型，返回详细的错误信息
	if validErrors, ok := validationErr.(ValidationErrors); ok {
		if len(validErrors) > 0 {
			return status.Errorf(codes.InvalidArgument,
				"validation failed: %v", validErrors.Error())
		}
	}

	return status.Errorf(codes.InvalidArgument, "validation error: %v", validationErr)
}

// RegisterValidationRules 注册校验规则
func (si *ServiceIntegration) RegisterValidationRules(
	structName string,
	rules ...FieldRule,
) {
	si.validator.RegisterRules(structName, rules...)
}

// RegisterTransformer 注册字段转换器
func (si *ServiceIntegration) RegisterTransformer(
	field string,
	transformer func(interface{}) interface{},
) {
	si.converter.RegisterTransformer(field, transformer)
}

// GetMetrics 获取转换性能指标
func (si *ServiceIntegration) GetMetrics() *ConversionMetrics {
	return si.converter.GetMetrics()
}

// ReportMetrics 报告性能指标
func (si *ServiceIntegration) ReportMetrics() {
	metrics := si.GetMetrics()
	if si.logger != nil && metrics.TotalConversions > 0 {
		successRate := float64(metrics.SuccessfulConversions) / float64(metrics.TotalConversions) * 100
		si.logger.Info("📊 Conversion Metrics: Total=%d, Success=%d, Failed=%d, SuccessRate=%.2f%%, AvgDuration=%v",
			metrics.TotalConversions,
			metrics.SuccessfulConversions,
			metrics.FailedConversions,
			successRate,
			metrics.AverageDuration,
		)
	}
}
