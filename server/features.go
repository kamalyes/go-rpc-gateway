/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 13:59:00
 * @FilePath: \go-rpc-gateway\server\features.go
 * @Description: 统一的功能特性注册管理器
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	gohealth "github.com/kamalyes/go-config/pkg/health"
	gojaeger "github.com/kamalyes/go-config/pkg/jaeger"
	gomonitoring "github.com/kamalyes/go-config/pkg/monitoring"
	gopprof "github.com/kamalyes/go-config/pkg/pprof"
	goswagger "github.com/kamalyes/go-config/pkg/swagger"
	"github.com/kamalyes/go-rpc-gateway/errors"
)

// FeatureType 功能特性类型
type FeatureType string

const (
	FeatureSwagger    FeatureType = "swagger"
	FeatureMonitoring FeatureType = "monitoring"
	FeatureTracing    FeatureType = "tracing"
	FeatureHealth     FeatureType = "health"
	FeaturePProf      FeatureType = "pprof"
	FeatureWSC        FeatureType = "wsc" // WebSocket通信
)

// FeatureEnabler 功能启用器接口
type FeatureEnabler interface {
	// Enable 启用功能，使用配置中的默认设置
	Enable() error

	// EnableWithConfig 使用自定义配置启用功能
	EnableWithConfig(config interface{}) error

	// IsEnabled 检查功能是否已启用
	IsEnabled() bool

	// GetType 获取功能类型
	GetType() FeatureType
}

// FeatureManager 功能管理器
type FeatureManager struct {
	server   *Server
	enablers map[FeatureType]FeatureEnabler
}

// NewFeatureManager 创建功能管理器
func NewFeatureManager(s *Server) *FeatureManager {
	fm := &FeatureManager{
		server:   s,
		enablers: make(map[FeatureType]FeatureEnabler),
	}

	// 注册所有内置功能
	fm.registerBuiltinFeatures()

	return fm
}

// registerBuiltinFeatures 注册内置功能
func (fm *FeatureManager) registerBuiltinFeatures() {
	fm.enablers[FeatureSwagger] = &SwaggerFeature{server: fm.server}
	fm.enablers[FeatureMonitoring] = &MonitoringFeature{server: fm.server}
	fm.enablers[FeatureHealth] = &HealthFeature{server: fm.server}
	fm.enablers[FeaturePProf] = &PProfFeature{server: fm.server}
	fm.enablers[FeatureTracing] = &TracingFeature{server: fm.server}
	fm.enablers[FeatureWSC] = &WSCFeature{server: fm.server}
}

// Enable 启用指定功能（使用配置中的默认设置）
func (fm *FeatureManager) Enable(feature FeatureType) error {
	enabler, exists := fm.enablers[feature]
	if !exists {
		return errors.NewErrorf(errors.ErrCodeFeatureNotRegistered, "feature %s not registered", feature)
	}
	return enabler.Enable()
}

// EnableWithConfig 使用自定义配置启用功能
func (fm *FeatureManager) EnableWithConfig(feature FeatureType, config interface{}) error {
	enabler, exists := fm.enablers[feature]
	if !exists {
		return errors.NewErrorf(errors.ErrCodeFeatureNotRegistered, "feature %s not registered", feature)
	}
	return enabler.EnableWithConfig(config)
}

// IsEnabled 检查功能是否已启用
func (fm *FeatureManager) IsEnabled(feature FeatureType) bool {
	enabler, exists := fm.enablers[feature]
	if !exists {
		return false
	}
	return enabler.IsEnabled()
}

// EnableAll 启用所有在配置中标记为启用的功能
func (fm *FeatureManager) EnableAll() error {
	for _, enabler := range fm.enablers {
		if err := enabler.Enable(); err != nil {
			return errors.WrapWithContext(err, errors.ErrCodeFeatureEnableFailed)
		}
	}
	return nil
}

// SwaggerFeature Swagger功能实现
type SwaggerFeature struct {
	server  *Server
	enabled bool
}

// Enable 启用Swagger（使用配置中的设置）
func (f *SwaggerFeature) Enable() error {
	if f.server.config.Swagger.Enabled {
		return f.EnableWithConfig(&f.server.config.Swagger)
	}

	// 如果配置中未启用，使用默认配置
	defaultConfig := goswagger.Default()
	return f.EnableWithConfig(defaultConfig)
}

// EnableWithConfig 使用自定义配置启用Swagger
func (f *SwaggerFeature) EnableWithConfig(config interface{}) error {
	swaggerConfig, ok := config.(*goswagger.Swagger)
	if !ok {
		return errors.ErrInvalidConfigType.WithDetails("expected *goswagger.Swagger")
	}

	if err := f.server.EnableSwaggerWithConfig(swaggerConfig); err != nil {
		return err
	}

	f.enabled = true
	return nil
}

// IsEnabled 检查Swagger是否已启用
func (f *SwaggerFeature) IsEnabled() bool {
	return f.enabled
}

// GetType 获取功能类型
func (f *SwaggerFeature) GetType() FeatureType {
	return FeatureSwagger
}

// MonitoringFeature Monitoring功能实现
type MonitoringFeature struct {
	server  *Server
	enabled bool
}

// Enable 启用Monitoring（使用配置中的设置）
func (f *MonitoringFeature) Enable() error {
	if f.server.config.Monitoring.Enabled {
		return f.EnableWithConfig(&f.server.config.Monitoring)
	}

	// 如果配置中未启用，使用默认配置
	defaultConfig := gomonitoring.Default()
	return f.EnableWithConfig(defaultConfig)
}

// EnableWithConfig 使用自定义配置启用Monitoring
func (f *MonitoringFeature) EnableWithConfig(config interface{}) error {
	_, ok := config.(*gomonitoring.Monitoring)
	if !ok {
		return errors.ErrInvalidConfigType.WithDetails("expected *gomonitoring.Monitoring")
	}

	if err := f.server.EnableMonitoringWithConfig(); err != nil {
		return err
	}

	f.enabled = true
	return nil
}

// IsEnabled 检查Monitoring是否已启用
func (f *MonitoringFeature) IsEnabled() bool {
	return f.enabled
}

// GetType 获取功能类型
func (f *MonitoringFeature) GetType() FeatureType {
	return FeatureMonitoring
}

// HealthFeature Health功能实现
type HealthFeature struct {
	server  *Server
	enabled bool
}

// Enable 启用Health（使用配置中的设置）
func (f *HealthFeature) Enable() error {
	if f.server.config.Health.Enabled {
		return f.EnableWithConfig(&f.server.config.Health)
	}

	// 如果配置中未启用，使用默认配置
	defaultConfig := gohealth.Default()
	return f.EnableWithConfig(defaultConfig)
}

// EnableWithConfig 使用自定义配置启用Health
func (f *HealthFeature) EnableWithConfig(config interface{}) error {
	_, ok := config.(*gohealth.Health)
	if !ok {
		return errors.ErrInvalidConfigType.WithDetails("expected *gohealth.Health")
	}

	if err := f.server.EnableHealthWithConfig(); err != nil {
		return err
	}

	f.enabled = true
	return nil
}

// IsEnabled 检查Health是否已启用
func (f *HealthFeature) IsEnabled() bool {
	return f.enabled
}

// GetType 获取功能类型
func (f *HealthFeature) GetType() FeatureType {
	return FeatureHealth
}

// PProfFeature PProf功能实现
type PProfFeature struct {
	server  *Server
	enabled bool
}

// Enable 启用PProf（使用配置中的设置）
func (f *PProfFeature) Enable() error {
	if f.server.config.Middleware.PProf.Enabled {
		return f.EnableWithConfig(&f.server.config.Middleware.PProf)
	}

	// 如果配置中未启用，使用默认配置
	defaultConfig := gopprof.Default()
	return f.EnableWithConfig(defaultConfig)
}

// EnableWithConfig 使用自定义配置启用PProf
func (f *PProfFeature) EnableWithConfig(config interface{}) error {
	_, ok := config.(*gopprof.PProf)
	if !ok {
		return errors.ErrInvalidConfigType.WithDetails("expected *gopprof.PProf")
	}

	if err := f.server.EnablePProfWithConfig(); err != nil {
		return err
	}

	f.enabled = true
	return nil
}

// IsEnabled 检查PProf是否已启用
func (f *PProfFeature) IsEnabled() bool {
	return f.enabled
}

// GetType 获取功能类型
func (f *PProfFeature) GetType() FeatureType {
	return FeaturePProf
}

// TracingFeature Tracing功能实现
type TracingFeature struct {
	server  *Server
	enabled bool
}

// Enable 启用Tracing（使用配置中的设置）
func (f *TracingFeature) Enable() error {
	if f.server.config.Monitoring.Jaeger.Enabled {
		return f.EnableWithConfig(&f.server.config.Monitoring.Jaeger)
	}

	// 如果配置中未启用，使用默认配置
	defaultConfig := gojaeger.Default()
	return f.EnableWithConfig(defaultConfig)
}

// EnableWithConfig 使用自定义配置启用Tracing
func (f *TracingFeature) EnableWithConfig(config interface{}) error {
	_, ok := config.(*gojaeger.Jaeger)
	if !ok {
		return errors.ErrInvalidConfigType.WithDetails("expected *gojaeger.Jaeger")
	}

	if err := f.server.EnableTracingWithConfig(); err != nil {
		return err
	}

	f.enabled = true
	return nil
}

// IsEnabled 检查Tracing是否已启用
func (f *TracingFeature) IsEnabled() bool {
	return f.enabled
}

// GetType 获取功能类型
func (f *TracingFeature) GetType() FeatureType {
	return FeatureTracing
}

// WSCFeature WebSocket通信功能实现
type WSCFeature struct {
	server  *Server
	enabled bool
}

// Enable 启用WSC（使用配置中的设置）
func (f *WSCFeature) Enable() error {
	if f.server.config.WSC != nil && f.server.config.WSC.Enabled {
		return f.EnableWithConfig(f.server.config.WSC)
	}

	// 如果配置中未启用，跳过
	return nil
}

// EnableWithConfig 使用自定义配置启用WSC
func (f *WSCFeature) EnableWithConfig(config interface{}) error {
	if err := f.server.EnableWSCWithConfig(config); err != nil {
		return err
	}

	f.enabled = true
	return nil
}

// IsEnabled 检查WSC是否已启用
func (f *WSCFeature) IsEnabled() bool {
	return f.enabled
}

// GetType 获取功能类型
func (f *WSCFeature) GetType() FeatureType {
	return FeatureWSC
}
