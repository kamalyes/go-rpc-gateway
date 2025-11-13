/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 16:30:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 09:05:15
 * @FilePath: \go-rpc-gateway\pbmo\error_handler.go
 * @Description: 转换错误处理和日志模块
 * 职责：转换失败处理、参数校验失败、日志记录、gRPC状态码映射
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package pbmo

import (
	"fmt"

	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ConversionErrorHandler 转换错误处理器
type ConversionErrorHandler struct {
	logger logger.ILogger
}

// NewConversionErrorHandler 创建错误处理器
func NewConversionErrorHandler(log logger.ILogger) *ConversionErrorHandler {
	return &ConversionErrorHandler{
		logger: log,
	}
}

// HandleConversionError 处理转换错误
// 将转换错误转换为 gRPC 状态错误并记录日志
func (ceh *ConversionErrorHandler) HandleConversionError(err error, conversionType string) error {
	if err == nil {
		return nil
	}

	if ceh.logger != nil {
		ceh.logger.Error("❌ Conversion failed [%s]: %v", conversionType, err)
	}

	// 返回 gRPC 错误
	return status.Errorf(codes.Internal, "failed to convert %s: %v", conversionType, err)
}

// HandleValidationError 处理参数校验错误
// 返回 InvalidArgument 状态码
func (ceh *ConversionErrorHandler) HandleValidationError(validationErr error) error {
	if validationErr == nil {
		return nil
	}

	if ceh.logger != nil {
		ceh.logger.Warn("⚠️  Validation failed: %v", validationErr)
	}

	// 返回 gRPC InvalidArgument 错误
	return status.Errorf(codes.InvalidArgument, "validation failed: %v", validationErr)
}

// HandleBatchConversionError 处理批量转换错误
// 记录详细的转换失败信息
func (ceh *ConversionErrorHandler) HandleBatchConversionError(index int, err error) error {
	if err == nil {
		return nil
	}

	if ceh.logger != nil {
		ceh.logger.Error("❌ Batch conversion failed at index %d: %v", index, err)
	}

	return status.Errorf(codes.Internal, "batch conversion failed at index %d: %v", index, err)
}

// LogConversionStart 记录转换开始
func (ceh *ConversionErrorHandler) LogConversionStart(srcType, dstType string) {
	if ceh.logger != nil {
		ceh.logger.Debug("🔄 Converting %s -> %s", srcType, dstType)
	}
}

// LogConversionSuccess 记录转换成功
func (ceh *ConversionErrorHandler) LogConversionSuccess(srcType, dstType string) {
	if ceh.logger != nil {
		ceh.logger.Debug("✅ Successfully converted %s -> %s", srcType, dstType)
	}
}

// LogValidationSuccess 记录校验成功
func (ceh *ConversionErrorHandler) LogValidationSuccess(dataType string) {
	if ceh.logger != nil {
		ceh.logger.Debug("✅ Validation passed for %s", dataType)
	}
}

// ErrorToGRPCStatus 将错误转换为 gRPC 状态码
func ErrorToGRPCStatus(err error) (codes.Code, string) {
	if err == nil {
		return codes.OK, ""
	}

	errMsg := err.Error()

	// 根据错误内容判断状态码
	switch {
	case IsValidationError(err):
		return codes.InvalidArgument, fmt.Sprintf("validation error: %v", err)
	case IsConversionError(err):
		return codes.Internal, fmt.Sprintf("conversion error: %v", err)
	case IsNilError(err):
		return codes.InvalidArgument, "nil value provided"
	case IsTypeError(err):
		return codes.Internal, fmt.Sprintf("type error: %v", err)
	default:
		return codes.Internal, errMsg
	}
}

// IsValidationError 判断是否为校验错误
func IsValidationError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(ValidationErrors)
	return ok
}

// IsConversionError 判断是否为转换错误
func IsConversionError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return errMsg[:12] == "failed to convert" || errMsg[:13] == "batch conversion"
}

// IsNilError 判断是否为 nil 错误
func IsNilError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return errMsg == "pb message cannot be nil" ||
		errMsg == "modelPtr cannot be nil" ||
		errMsg == "model cannot be nil" ||
		errMsg == "pbPtr cannot be nil"
}

// IsTypeError 判断是否为类型错误
func IsTypeError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return errMsg[:12] == "must be a pointer" || errMsg[:16] == "cannot convert"
}

// ToErrorCode 将错误转换为框架错误码
func ToErrorCode(err error) errors.ErrorCode {
	if err == nil {
		return errors.ErrCodeOK
	}

	switch {
	case IsValidationError(err):
		return errors.ErrCodeInvalidParameter
	case IsConversionError(err):
		return errors.ErrCodeInternalServerError
	case IsNilError(err):
		return errors.ErrCodeMissingParameter
	case IsTypeError(err):
		return errors.ErrCodeInternalServerError
	default:
		return errors.ErrCodeInternal
	}
}

// ConversionErrorContext 转换操作的错误上下文
type ConversionErrorContext struct {
	Operation   string // 操作类型：PBToModel, ModelToPB, Validation
	SourceType  string // 源类型
	TargetType  string // 目标类型
	FieldName   string // 字段名（如果是字段级错误）
	Index       int    // 批量转换中的索引
	OriginalErr error  // 原始错误
	Logger      logger.ILogger
}

// Error 返回错误信息
func (cec *ConversionErrorContext) Error() string {
	switch cec.Operation {
	case "PBToModel":
		if cec.FieldName != "" {
			return fmt.Sprintf("failed to convert PB field %s from %s to %s: %v",
				cec.FieldName, cec.SourceType, cec.TargetType, cec.OriginalErr)
		}
		return fmt.Sprintf("failed to convert PB %s to model %s: %v",
			cec.SourceType, cec.TargetType, cec.OriginalErr)

	case "ModelToPB":
		if cec.FieldName != "" {
			return fmt.Sprintf("failed to convert model field %s from %s to PB %s: %v",
				cec.FieldName, cec.SourceType, cec.TargetType, cec.OriginalErr)
		}
		return fmt.Sprintf("failed to convert model %s to PB %s: %v",
			cec.SourceType, cec.TargetType, cec.OriginalErr)

	case "Validation":
		return fmt.Sprintf("validation failed for %s: %v", cec.SourceType, cec.OriginalErr)

	case "Batch":
		return fmt.Sprintf("batch conversion failed at index %d (%s -> %s): %v",
			cec.Index, cec.SourceType, cec.TargetType, cec.OriginalErr)

	default:
		return fmt.Sprintf("conversion error: %v", cec.OriginalErr)
	}
}

// Log 记录错误
func (cec *ConversionErrorContext) Log() {
	if cec.Logger != nil {
		cec.Logger.Error("❌ %s", cec.Error())
	}
}

// ToGRPCError 转换为 gRPC 错误
func (cec *ConversionErrorContext) ToGRPCError() error {
	grpcCode := codes.Internal

	if cec.Operation == "Validation" {
		grpcCode = codes.InvalidArgument
	}

	return status.Errorf(grpcCode, cec.Error())
}
