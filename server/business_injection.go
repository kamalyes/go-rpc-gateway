/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-13 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 15:30:32
 * @FilePath: \go-rpc-gateway\server\business_injection.go
 * @Description: 业务服务注入管理器 - 为go-rpc-gateway提供业务服务注入能力
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"context"
	"reflect"
	"sync"

	"github.com/kamalyes/go-logger"
	"github.com/kamalyes/go-rpc-gateway/cpool"
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// BusinessServiceProvider 业务服务提供者接口
type BusinessServiceProvider interface {
	// GetServiceName 获取服务名称
	GetServiceName() string

	// RegisterGRPCServices 注册gRPC服务
	RegisterGRPCServices(grpcServer *grpc.Server)

	// Start 启动服务
	Start() error

	// Stop 停止服务
	Stop() error

	// IsRunning 检查服务运行状态
	IsRunning() bool
}

// BusinessConfigAdapter 业务配置适配器接口
type BusinessConfigAdapter interface {
	// GetServiceName 获取服务名称
	GetServiceName() string

	// GetEnvironment 获取环境
	GetEnvironment() string

	// IsDebug 是否调试模式
	IsDebug() bool

	// 其他配置获取方法...
}

// BusinessInjectionManager 业务服务注入管理器
type BusinessInjectionManager struct {
	logger logger.ILogger

	// 注册的业务服务
	services map[string]BusinessServiceProvider

	// 健康检查服务器
	healthServer *health.Server

	// 连接池管理器
	poolManager cpool.PoolManager

	// 状态管理
	isRunning bool
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewBusinessInjectionManager 创建业务服务注入管理器
func NewBusinessInjectionManager() *BusinessInjectionManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &BusinessInjectionManager{
		logger:       global.GetLogger(),
		services:     make(map[string]BusinessServiceProvider),
		healthServer: health.NewServer(),
		poolManager:  nil,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// RegisterBusinessService 注册业务服务
func (bim *BusinessInjectionManager) RegisterBusinessService(name string, service BusinessServiceProvider) error {
	bim.mu.Lock()
	defer bim.mu.Unlock()

	if _, exists := bim.services[name]; exists {
		return errors.NewErrorf(errors.ErrCodeConflict, "业务服务 %s 已存在", name)
	}

	bim.services[name] = service
	bim.logger.InfoKV("业务服务注册成功",
		"service_name", name,
		"service_type", reflect.TypeOf(service).String())

	return nil
}

// UnregisterBusinessService 注销业务服务
func (bim *BusinessInjectionManager) UnregisterBusinessService(name string) error {
	bim.mu.Lock()
	defer bim.mu.Unlock()

	service, exists := bim.services[name]
	if !exists {
		return errors.NewErrorf(errors.ErrCodeResourceNotFound, "业务服务 %s 不存在", name)
	}

	// 停止服务
	if service.IsRunning() {
		if err := service.Stop(); err != nil {
			bim.logger.Error("停止业务服务失败", "service_name", name, "error", err)
		}
	}

	delete(bim.services, name)
	bim.logger.Info("业务服务注销成功", "service_name", name)

	return nil
}

// GetBusinessService 获取业务服务
func (bim *BusinessInjectionManager) GetBusinessService(name string) (BusinessServiceProvider, error) {
	bim.mu.RLock()
	defer bim.mu.RUnlock()

	service, exists := bim.services[name]
	if !exists {
		return nil, errors.NewErrorf(errors.ErrCodeResourceNotFound, "业务服务 %s 不存在", name)
	}

	return service, nil
}

// ListBusinessServices 列出所有业务服务
func (bim *BusinessInjectionManager) ListBusinessServices() map[string]BusinessServiceProvider {
	bim.mu.RLock()
	defer bim.mu.RUnlock()

	result := make(map[string]BusinessServiceProvider)
	for name, service := range bim.services {
		result[name] = service
	}

	return result
}

// RegisterAllGRPCServices 注册所有业务服务的gRPC服务
func (bim *BusinessInjectionManager) RegisterAllGRPCServices(grpcServer *grpc.Server) {
	bim.mu.RLock()
	services := make(map[string]BusinessServiceProvider)
	for name, service := range bim.services {
		services[name] = service
	}
	bim.mu.RUnlock()

	// 注册健康检查服务
	grpc_health_v1.RegisterHealthServer(grpcServer, bim.healthServer)

	// 注册所有业务服务
	for name, service := range services {
		bim.logger.Info("注册业务服务gRPC接口", "service_name", name)
		service.RegisterGRPCServices(grpcServer)

		// 设置健康状态
		bim.healthServer.SetServingStatus(name, grpc_health_v1.HealthCheckResponse_SERVING)
	}

	// 设置总体健康状态
	bim.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	bim.logger.Info("所有业务服务gRPC接口注册完成", "service_count", len(services))
}

// StartAllBusinessServices 启动所有业务服务
func (bim *BusinessInjectionManager) StartAllBusinessServices() error {
	bim.mu.Lock()
	defer bim.mu.Unlock()

	if bim.isRunning {
		return errors.NewError(errors.ErrCodeConflict, "业务服务管理器已在运行")
	}

	// 启动所有业务服务
	for name, service := range bim.services {
		if !service.IsRunning() {
			if err := service.Start(); err != nil {
				bim.logger.Error("启动业务服务失败", "service_name", name, "error", err)
				// 继续启动其他服务
			} else {
				bim.logger.Info("业务服务启动成功", "service_name", name)
			}
		}
	}

	bim.isRunning = true
	bim.logger.Info("业务服务管理器启动完成", "service_count", len(bim.services))

	return nil
}

// StopAllBusinessServices 停止所有业务服务
func (bim *BusinessInjectionManager) StopAllBusinessServices() error {
	bim.mu.Lock()
	defer bim.mu.Unlock()

	if !bim.isRunning {
		return nil
	}

	// 取消上下文
	bim.cancel()

	// 停止所有业务服务
	for name, service := range bim.services {
		if service.IsRunning() {
			if err := service.Stop(); err != nil {
				bim.logger.Error("停止业务服务失败", "service_name", name, "error", err)
			} else {
				bim.logger.Info("业务服务停止成功", "service_name", name)
			}
		}
	}

	// 设置健康状态为不可用
	for name := range bim.services {
		bim.healthServer.SetServingStatus(name, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	}
	bim.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	bim.isRunning = false
	bim.logger.Info("业务服务管理器停止完成")

	return nil
}

// IsRunning 检查管理器运行状态
func (bim *BusinessInjectionManager) IsRunning() bool {
	bim.mu.RLock()
	defer bim.mu.RUnlock()
	return bim.isRunning
}

// GetHealthServer 获取健康检查服务器
func (bim *BusinessInjectionManager) GetHealthServer() *health.Server {
	return bim.healthServer
}

// GetPoolManager 获取连接池管理器
func (bim *BusinessInjectionManager) GetPoolManager() cpool.PoolManager {
	return bim.poolManager
}

// GetServiceCount 获取已注册服务数量
func (bim *BusinessInjectionManager) GetServiceCount() int {
	bim.mu.RLock()
	defer bim.mu.RUnlock()
	return len(bim.services)
}

// GetServiceStatus 获取所有服务状态
func (bim *BusinessInjectionManager) GetServiceStatus() map[string]bool {
	bim.mu.RLock()
	defer bim.mu.RUnlock()

	status := make(map[string]bool)
	for name, service := range bim.services {
		status[name] = service.IsRunning()
	}

	return status
}
