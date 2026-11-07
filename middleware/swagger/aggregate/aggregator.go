/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\aggregate\aggregator.go
 * @Description: 聚合核心 - Aggregator 结构定义、构造函数、聚合模式分发
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package aggregate

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	goswagger "github.com/kamalyes/go-rpc-gateway/config/swagger"
	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	serrors "github.com/kamalyes/go-rpc-gateway/middleware/swagger/errors"
	format "github.com/kamalyes/go-rpc-gateway/middleware/swagger/format"
	errorx "github.com/kamalyes/go-toolbox/pkg/errorx"
	logger "github.com/kamalyes/go-toolbox/pkg/logger"
	mathx "github.com/kamalyes/go-toolbox/pkg/mathx"
)

// ServiceSpecConfig 服务规范配置接口，解耦对 goswagger.ServiceSpec 的直接依赖
type ServiceSpecConfig interface {
	GetName() string
	GetBasePath() string
	GetTags() []string
}

// serviceSpecAdapter 将 *goswagger.ServiceSpec 适配为 ServiceSpecConfig 接口
type serviceSpecAdapter struct {
	*goswagger.ServiceSpec
}

// GetName 返回服务名称
func (a *serviceSpecAdapter) GetName() string {
	return a.ServiceSpec.Name
}

// GetBasePath 返回服务基础路径
func (a *serviceSpecAdapter) GetBasePath() string {
	return a.ServiceSpec.BasePath
}

// GetTags 返回服务标签
func (a *serviceSpecAdapter) GetTags() []string {
	return a.ServiceSpec.Tags
}

// wrapServiceSpec 将 *goswagger.ServiceSpec 包装为 ServiceSpecConfig
func wrapServiceSpec(s *goswagger.ServiceSpec) ServiceSpecConfig {
	return &serviceSpecAdapter{ServiceSpec: s}
}

// Aggregator 聚合器，负责加载、预处理、合并多个服务的 Swagger 规范
type Aggregator struct {
	config         *goswagger.Swagger
	serviceSpecs   map[string]map[string]interface{}
	aggregatedSpec map[string]interface{}
	lastUpdated    time.Time
	httpClient     *http.Client
	logger         logger.ILogger
}

// NewAggregator 创建聚合器
func NewAggregator(config *goswagger.Swagger, logger logger.ILogger) *Aggregator {
	return &Aggregator{
		config:       config,
		serviceSpecs: make(map[string]map[string]interface{}),
		httpClient:   &http.Client{Timeout: constants.DefaultTimeout},
		logger:       logger,
	}
}

// LoadAll 加载所有服务的 Swagger 规范并执行聚合
func (a *Aggregator) LoadAll() error {
	if a.config.Aggregate == nil || len(a.config.Aggregate.Services) == 0 {
		return errorx.NewError(serrors.ErrTypeNoServicesConfig)
	}

	a.serviceSpecs = make(map[string]map[string]interface{})
	a.aggregatedSpec = nil
	a.lastUpdated = time.Now()

	a.logger.Info("开始加载所有服务规范，总计 %d 个服务", len(a.config.Aggregate.Services))

	loadedServices := make(map[string]bool)
	for i, service := range a.config.Aggregate.Services {
		a.loadSingleService(i, service, loadedServices)
	}

	if err := a.aggregateSpecs(); err != nil {
		return errorx.NewError(serrors.ErrTypeAggregateFailed, err)
	}

	if a.aggregatedSpec != nil {
		a.aggregatedSpec[constants.FieldXAggregateInfo] = a.buildServicesInfo()
	}

	a.logger.Info("所有服务规范加载完成，共 %d 个服务", len(a.serviceSpecs))
	return nil
}

// aggregateSpecs 根据配置的聚合模式执行规范聚合
func (a *Aggregator) aggregateSpecs() error {
	if len(a.serviceSpecs) == 0 {
		return fmt.Errorf("没有加载的服务规范")
	}

	switch strings.ToLower(a.config.Aggregate.Mode) {
	case constants.AggregateModeMerge:
		return a.mergeAllSpecs()
	case constants.AggregateModeSelector:
		return a.createSelectorSpec()
	default:
		return fmt.Errorf("不支持的聚合模式: %s", a.config.Aggregate.Mode)
	}
}

// GetAggregatedSpec 获取聚合后的 Swagger 规范 JSON
func (a *Aggregator) GetAggregatedSpec() ([]byte, error) {
	if !a.config.IsAggregateEnabled() {
		return nil, errorx.NewError(serrors.ErrTypeAggregateDisabled)
	}
	if a.aggregatedSpec == nil {
		return nil, errorx.NewError(serrors.ErrTypeSpecNotInitialized)
	}
	return format.MarshalSpec(a.aggregatedSpec)
}

// GetServiceSpec 获取单个服务的规范 JSON
func (a *Aggregator) GetServiceSpec(serviceName string) ([]byte, error) {
	if !a.config.IsAggregateEnabled() {
		return nil, errorx.NewError(serrors.ErrTypeAggregateDisabled)
	}

	spec, exists := a.findNamedSpec(serviceName, "服务", a.serviceSpecs)
	if !exists {
		return nil, a.namedSpecNotFoundError("服务", serviceName, a.serviceSpecs)
	}

	return format.MarshalSpec(spec)
}

// GetServiceSpecs 获取所有已加载的服务规范
func (a *Aggregator) GetServiceSpecs() map[string]map[string]interface{} {
	return a.serviceSpecs
}

// GetAggregatedSpecMap 获取聚合规范 map（供 documents 模块使用）
func (a *Aggregator) GetAggregatedSpecMap() map[string]interface{} {
	return a.aggregatedSpec
}

// GetLastUpdated 获取最后更新时间
func (a *Aggregator) GetLastUpdated() time.Time {
	return a.lastUpdated
}

// getSortedServiceNames 获取排序后的服务名列表
func (a *Aggregator) getSortedServiceNames() []string {
	serviceNames := make([]string, 0, len(a.serviceSpecs))
	for name := range a.serviceSpecs {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)
	return serviceNames
}

// findNamedSpec 按名称查找规范，支持忽略大小写、标准化名称、包含匹配
func (a *Aggregator) findNamedSpec(specName, specKind string, specs map[string]map[string]interface{}) (map[string]interface{}, bool) {
	if spec, exists := specs[specName]; exists {
		return spec, true
	}

	for actualName, actualSpec := range specs {
		if strings.EqualFold(specName, actualName) {
			a.logger.Info("通过忽略大小写匹配找到%s: %s -> %s", specKind, specName, actualName)
			return actualSpec, true
		}
	}

	for actualName, actualSpec := range specs {
		if strings.Contains(strings.ToLower(specName), strings.ToLower(actualName)) {
			a.logger.Info("通过包含匹配找到%s: %s -> %s", specKind, specName, actualName)
			return actualSpec, true
		}
	}

	return nil, false
}

// namedSpecNotFoundError 构造命名规范未找到的错误
func (a *Aggregator) namedSpecNotFoundError(specKind, specName string, specs map[string]map[string]interface{}) error {
	availableSpecs := make([]string, 0, len(specs))
	for name := range specs {
		availableSpecs = append(availableSpecs, name)
	}
	sort.Strings(availableSpecs)

	errMsg := fmt.Sprintf("%s %s 不存在。可用%s: [%s]", specKind, specName, specKind, strings.Join(availableSpecs, ", "))
	a.logger.Error(errMsg)
	return fmt.Errorf("%s", errMsg)
}

// processAndStoreSpec 预处理并存储服务规范
func (a *Aggregator) processAndStoreSpec(service *goswagger.ServiceSpec, spec map[string]interface{}) error {
	a.preprocessServiceSpec(spec, wrapServiceSpec(service))

	convertedSpec := mathx.ConvertMapKeysToString(spec)
	if convertedMap, ok := convertedSpec.(map[string]interface{}); ok {
		a.serviceSpecs[service.Name] = convertedMap
	} else {
		return fmt.Errorf("转换服务规范失败: 无法转换为map[string]interface{}")
	}
	return nil
}
