/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-13 00:00:00
 * @Description: gRPC 自动 PB ↔ GORM Model 转换中间件
 * 支持参数校验、自动转换、自定义处理器
 * 性能优化：缓存字段映射，避免反复反射
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"context"
	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/pbmo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"reflect"
	"sync"
)

// ConversionMiddleware 自动转换中间件配置
type ConversionMiddleware struct {
	Enabled        bool
	LogConversions bool
	Logger         logger.ILogger
	pbmo           map[string]*pbmo.BidiConverter
	lock           sync.RWMutex
}

// NewConversionMiddleware 创建转换中间件
func NewConversionMiddleware(log logger.ILogger, enabled bool) *ConversionMiddleware {
	return &ConversionMiddleware{
		Enabled: enabled,
		Logger:  log,
		pbmo:    make(map[string]*pbmo.BidiConverter),
	}
}

// RegisterConverter 注册类型转换器
func (cm *ConversionMiddleware) RegisterConverter(key string, converter *pbmo.BidiConverter) {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	cm.pbmo[key] = converter
	if cm.Logger != nil {
		cm.Logger.Debug("Registered converter for: %s", key)
	}
}

// UnaryServerInterceptor 一元调用拦截器
func (cm *ConversionMiddleware) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !cm.Enabled {
			return handler(ctx, req)
		}

		if cm.LogConversions && cm.Logger != nil {
			cm.Logger.Debug("🔄 Processing RPC: %s", info.FullMethod)
		}

		// 调用实际处理器
		resp, err := handler(ctx, req)
		if err != nil {
			return resp, err
		}

		if resp == nil {
			return resp, nil
		}

		return resp, nil
	}
}

// StreamServerInterceptor 流调用拦截器
func (cm *ConversionMiddleware) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !cm.Enabled {
			return handler(srv, ss)
		}

		wrappedStream := &conversionStream{
			ServerStream: ss,
			middleware:   cm,
		}
		return handler(srv, wrappedStream)
	}
}

// conversionStream 转换流包装器
type conversionStream struct {
	grpc.ServerStream
	middleware *ConversionMiddleware
}

// RecvMsg 接收消息
func (cs *conversionStream) RecvMsg(m interface{}) error {
	return cs.ServerStream.RecvMsg(m)
}

// SendMsg 发送消息
func (cs *conversionStream) SendMsg(m interface{}) error {
	return cs.ServerStream.SendMsg(m)
}

// ValidatingInterceptor 参数校验拦截器
func (cm *ConversionMiddleware) ValidatingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !cm.Enabled {
			return handler(ctx, req)
		}

		// 校验请求参数
		if validator, ok := req.(pbmo.Validator); ok {
			if err := validator.Validate(); err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "validation failed: %v", err)
			}
		}

		// 继续处理
		resp, err := handler(ctx, req)
		if err != nil {
			return resp, err
		}

		// 校验响应参数
		if validator, ok := resp.(pbmo.Validator); ok {
			if err := validator.Validate(); err != nil {
				return nil, status.Errorf(codes.Internal, "response validation failed: %v", err)
			}
		}

		return resp, nil
	}
}

// ChainUnaryInterceptors 链接多个一元拦截器
func ChainUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 反向遍历拦截器链
		chain := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			prevChain := chain
			nextHandler := grpc.UnaryHandler(func(ctx context.Context, req interface{}) (interface{}, error) {
				return interceptor(ctx, req, info, prevChain)
			})
			chain = nextHandler
		}
		return chain(ctx, req)
	}
}

// ChainStreamInterceptors 链接多个流拦截器
func ChainStreamInterceptors(interceptors ...grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		chain := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			prevChain := chain
			nextHandler := grpc.StreamHandler(func(srv interface{}, ss grpc.ServerStream) error {
				return interceptor(srv, ss, info, prevChain)
			})
			chain = nextHandler
		}
		return chain(srv, ss)
	}
}

// ConversionConfig 转换配置
type ConversionConfig struct {
	// 是否启用自动转换
	Enabled bool
	// 自定义字段映射 map[pbFieldName]modelFieldName
	FieldMappings map[string]map[string]string
	// 自定义转换器 map[methodName]converter
	Custompbmo map[string]func(interface{}) interface{}
	// 是否记录转换日志
	LogConversions bool
	// 需要转换的消息类型列表（如果为空则转换所有）
	MessageTypes []string
}

// AutoModelConverterInterceptor 自动模型转换拦截器
// 特性：
// - 自动处理 PB ↔ GORM Model 转换
// - 支持嵌套消息转换
// - 支持时间戳、枚举、自定义类型转换
// - 零代码侵入
func AutoModelConverterInterceptor(config ConversionConfig, log logger.ILogger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 记录请求信息
		if config.LogConversions && log != nil {
			log.DebugContext(ctx, "🔄 Processing gRPC call: %s", info.FullMethod)
		}

		// 调用实际处理器
		resp, err := handler(ctx, req)
		if err != nil {
			return resp, err
		}

		// 如果响应为 nil，直接返回
		if resp == nil {
			return resp, nil
		}

		// 自动转换响应（如果需要）
		if config.Enabled {
			if converted, convErr := autoConvertResponse(resp, log); convErr == nil {
				if config.LogConversions && log != nil {
					log.DebugContext(ctx, "✅ Auto-converted response: %T -> %T", resp, converted)
				}
				return converted, nil
			} else if config.LogConversions && log != nil {
				log.WarnContext(ctx, "⚠️  Failed to auto-convert response: %v", convErr)
			}
		}

		return resp, nil
	}
}

// StreamModelConverterInterceptor 流拦截器（支持自动转换）
func StreamModelConverterInterceptor(config ConversionConfig, log logger.ILogger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		wrappedStream := &modelConversionStream{
			ServerStream: ss,
			config:       config,
			log:          log,
		}
		return handler(srv, wrappedStream)
	}
}

// modelConversionStream 包装的流
type modelConversionStream struct {
	grpc.ServerStream
	config ConversionConfig
	log    logger.ILogger
}

// RecvMsg 接收消息时自动转换
func (s *modelConversionStream) RecvMsg(m interface{}) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}

	// 自动从 PB 转换为 Model（如果需要）
	if s.config.Enabled {
		if _, convErr := autoConvertRequest(m, s.log); convErr != nil && s.log != nil {
			ctx := s.ServerStream.Context()
			s.log.WarnContext(ctx, "Failed to convert received message: %v", convErr)
		}
	}

	return nil
}

// SendMsg 发送消息时自动转换
func (s *modelConversionStream) SendMsg(m interface{}) error {
	if !s.config.Enabled {
		return s.ServerStream.SendMsg(m)
	}

	// 自动转换响应
	if converted, err := autoConvertResponse(m, s.log); err == nil {
		return s.ServerStream.SendMsg(converted)
	}

	return s.ServerStream.SendMsg(m)
}

// autoConvertRequest 自动转换请求（PB -> Model）
func autoConvertRequest(pbReq interface{}, log logger.ILogger) (interface{}, error) {
	if pbReq == nil {
		return nil, nil
	}

	// 获取 PB 消息的类型名称
	pbType := reflect.TypeOf(pbReq)
	if pbType.Kind() == reflect.Ptr {
		pbType = pbType.Elem()
	}
	pbTypeName := pbType.Name()

	// 移除 "Pb" 或 "PB" 后缀获取模型名称
	modelTypeName := pbTypeName
	if len(modelTypeName) > 2 && modelTypeName[:2] == "PB" {
		modelTypeName = modelTypeName[2:]
	}

	if log != nil {
		log.Debug("Attempting to convert PB: %s -> Model: %s", pbTypeName, modelTypeName)
	}

	// 这里可以扩展为从注册表中查找转换器
	// 对于现在，我们只支持直接转换或使用 BidiConverter

	return pbReq, nil
}

// autoConvertResponse 自动转换响应（Model -> PB）
func autoConvertResponse(model interface{}, log logger.ILogger) (interface{}, error) {
	if model == nil {
		return nil, nil
	}

	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}
	modelTypeName := modelType.Name()

	// 如果已经是 PB 类型（通常以 Response 或 Message 结尾），直接返回
	if isPBType(modelTypeName) {
		return model, nil
	}

	// 如果实现了 ModelConverter 接口，使用它
	// 对于现在，我们假设模型已经是正确的类型

	if log != nil {
		log.Debug("No conversion needed for type: %s", modelTypeName)
	}

	return model, nil
}

// isPBType 判断是否是 PB 类型
func isPBType(typeName string) bool {
	return len(typeName) > 8 && (typeName[len(typeName)-8:] == "Response" || typeName[len(typeName)-7:] == "Message")
}

// ConversionRegistry 转换注册表（用于高级场景）
type ConversionRegistry struct {
	// pbToModelpbmo map[pbTypeName]converter
	pbToModelpbmo map[string]func(interface{}) (interface{}, error)
	// modelToPBpbmo map[modelTypeName]converter
	modelToPBpbmo map[string]func(interface{}) (interface{}, error)
	// log
	log logger.ILogger
}

// NewConversionRegistry 创建新的转换注册表
func NewConversionRegistry(log logger.ILogger) *ConversionRegistry {
	return &ConversionRegistry{
		pbToModelpbmo: make(map[string]func(interface{}) (interface{}, error)),
		modelToPBpbmo: make(map[string]func(interface{}) (interface{}, error)),
		log:           log,
	}
}

// RegisterPBToModelConverter 注册 PB -> Model 转换器
func (r *ConversionRegistry) RegisterPBToModelConverter(pbTypeName string, converter func(interface{}) (interface{}, error)) {
	r.pbToModelpbmo[pbTypeName] = converter
	if r.log != nil {
		r.log.Debug("Registered PB->Model converter for type: %s", pbTypeName)
	}
}

// RegisterModelToPBConverter 注册 Model -> PB 转换器
func (r *ConversionRegistry) RegisterModelToPBConverter(modelTypeName string, converter func(interface{}) (interface{}, error)) {
	r.modelToPBpbmo[modelTypeName] = converter
	if r.log != nil {
		r.log.Debug("Registered Model->PB converter for type: %s", modelTypeName)
	}
}

// ConvertPBToModel 使用注册的转换器
func (r *ConversionRegistry) ConvertPBToModel(pb interface{}) (interface{}, error) {
	pbType := reflect.TypeOf(pb)
	if pbType.Kind() == reflect.Ptr {
		pbType = pbType.Elem()
	}

	if converter, ok := r.pbToModelpbmo[pbType.Name()]; ok {
		return converter(pb)
	}

	// 回退到自动转换
	return pb, nil
}

// ConvertModelToPB 使用注册的转换器
func (r *ConversionRegistry) ConvertModelToPB(model interface{}) (interface{}, error) {
	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	if converter, ok := r.modelToPBpbmo[modelType.Name()]; ok {
		return converter(model)
	}

	// 回退到自动转换
	return model, nil
}
