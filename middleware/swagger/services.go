/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\services.go
 * @Description: 服务规范 API - 单服务 JSON、聚合 JSON、调试端点
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	goswagger "github.com/kamalyes/go-rpc-gateway/config/swagger"
	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	serrors "github.com/kamalyes/go-rpc-gateway/middleware/swagger/errors"
	errorx "github.com/kamalyes/go-toolbox/pkg/errorx"
	safe "github.com/kamalyes/go-toolbox/pkg/safe"
)

// handleServiceJSON 处理单个服务的 Swagger JSON 请求
func (m *Middleware) handleServiceJSON(w http.ResponseWriter, r *http.Request) {
	writeJSONHeaders(w)
	if handleOptions(w, r) {
		return
	}

	if !m.IsAggregateEnabled() {
		m.errorResponseFn(w, http.StatusNotFound, "聚合功能未启用")
		return
	}

	serviceName := m.extractEntityName(r.URL.Path, constants.PathServicePrefix)
	if serviceName == "" {
		m.errorResponseFn(w, http.StatusBadRequest, "服务名称不能为空")
		return
	}

	jsonData, err := m.aggregator.GetServiceSpec(serviceName)
	if err != nil {
		m.errorResponseFn(w, http.StatusNotFound, fmt.Sprintf("服务 %s 不存在", serviceName))
		return
	}

	m.serveSpecJSON(w, r, jsonData)
}

// handleAggregatedJSON 处理聚合的 Swagger JSON 请求
func (m *Middleware) handleAggregatedJSON(w http.ResponseWriter, r *http.Request) {
	writeJSONHeaders(w)
	if handleOptions(w, r) {
		return
	}

	if !m.IsAggregateEnabled() {
		m.errorResponseFn(w, http.StatusNotFound, "聚合功能未启用")
		return
	}

	jsonData, err := m.aggregator.GetAggregatedSpec()
	if err != nil {
		m.logger.Error("获取聚合Swagger规范失败: %v", err)
		m.errorResponseFn(w, http.StatusInternalServerError, "获取聚合规范失败")
		return
	}

	m.serveSpecJSON(w, r, jsonData)
}

// handleServicesIndex 处理服务列表页面
func (m *Middleware) handleServicesIndex(w http.ResponseWriter, _ *http.Request) {
	if !m.IsAggregateEnabled() {
		m.errorResponseFn(w, http.StatusNotFound, "聚合功能未启用")
		return
	}

	aggregatedSpec, err := m.aggregator.GetAggregatedSpec()
	if err != nil {
		m.errorResponseFn(w, http.StatusInternalServerError, "获取服务列表失败")
		return
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(aggregatedSpec, &spec); err != nil {
		m.errorResponseFn(w, http.StatusInternalServerError, "解析服务信息失败")
		return
	}

	writeHTMLHeaders(w)
	w.Write([]byte(m.buildServicesHTML(spec)))
}

// handleServicesDebug 处理服务调试信息端点
func (m *Middleware) handleServicesDebug(w http.ResponseWriter, r *http.Request) {
	if !m.IsAggregateEnabled() {
		m.errorResponseFn(w, http.StatusNotFound, "聚合功能未启用")
		return
	}

	writeJSONHeaders(w)

	serviceSpecs := m.aggregator.GetServiceSpecs()

	debugInfo := map[string]interface{}{
		constants.FieldTotalServices:      len(serviceSpecs),
		constants.FieldLoadedServices:     make([]map[string]interface{}, 0),
		constants.FieldConfiguredServices: make([]map[string]interface{}, 0),
		constants.FieldTimestamp:          time.Now().Format(time.RFC3339),
	}

	for serviceName := range serviceSpecs {
		debugInfo[constants.FieldLoadedServices] = append(debugInfo[constants.FieldLoadedServices].([]map[string]interface{}), map[string]interface{}{
			constants.FieldName: serviceName,
			constants.FieldURL:  fmt.Sprintf("%s/services/%s", m.config.UIPath, serviceName),
		})
	}

	safeAggregate := safe.New(m.config.Aggregate)
	if safeAggregate.Field("Enabled").Bool(false) {
		servicesVal := safeAggregate.Field("Services").Value()
		if services, ok := servicesVal.([]*goswagger.ServiceSpec); ok {
			for _, service := range services {
				debugInfo[constants.FieldConfiguredServices] = append(debugInfo[constants.FieldConfiguredServices].([]map[string]interface{}), map[string]interface{}{
					constants.FieldName:     service.Name,
					constants.FieldEnabled:  service.Enabled,
					constants.FieldSpecPath: service.SpecPath,
					constants.FieldURL:      service.URL,
				})
			}
		}
	}

	jsonData, err := json.MarshalIndent(debugInfo, constants.JSONIndentPrefix, constants.JSONIndentValue)
	if err != nil {
		m.errorResponseFn(w, http.StatusInternalServerError, "序列化调试信息失败")
		return
	}

	w.Write(jsonData)
}

// GetServiceSpec 获取单个服务的规范 JSON
func (m *Middleware) GetServiceSpec(serviceName string) ([]byte, error) {
	if !m.config.IsAggregateEnabled() {
		return nil, errorx.NewError(serrors.ErrTypeAggregateDisabled)
	}
	return m.aggregator.GetServiceSpec(serviceName)
}

// GetAggregatedSpec 获取聚合后的 Swagger 规范 JSON
func (m *Middleware) GetAggregatedSpec() ([]byte, error) {
	if !m.config.IsAggregateEnabled() {
		return nil, errorx.NewError(serrors.ErrTypeAggregateDisabled)
	}
	return m.aggregator.GetAggregatedSpec()
}
