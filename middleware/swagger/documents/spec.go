/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\documents\spec.go
 * @Description: 独立子文档构建 - 单文档规范构建、info 字段、顶层字段合并
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package documents

import (
	"encoding/json"
	"strings"
	"time"

	goswagger "github.com/kamalyes/go-rpc-gateway/config/swagger"
	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	serrors "github.com/kamalyes/go-rpc-gateway/middleware/swagger/errors"
	convert "github.com/kamalyes/go-toolbox/pkg/convert"
	errorx "github.com/kamalyes/go-toolbox/pkg/errorx"
	mathx "github.com/kamalyes/go-toolbox/pkg/mathx"
)

// buildSingleDocumentSpec 构建单个独立子文档
func (b *Builder) buildSingleDocumentSpec(document *goswagger.DocumentSpec) (map[string]interface{}, error) {
	if len(document.Sources) == 0 {
		return nil, errorx.NewError(serrors.ErrTypeDocumentNoSources, document.Name)
	}

	result := map[string]interface{}{
		constants.FieldSwagger:  constants.SpecVersion,
		constants.FieldInfo:     b.buildDocumentInfoFromSpec(document),
		constants.FieldConsumes: []string{constants.MimeApplicationJSON},
		constants.FieldProduces: []string{constants.MimeApplicationJSON},
		constants.FieldPaths:    make(map[string]interface{}),
		constants.FieldDefs:     make(map[string]interface{}),
		constants.FieldTags:     make([]interface{}, 0),
	}

	allPaths := result[constants.FieldPaths].(map[string]interface{})
	allDefinitions := result[constants.FieldDefs].(map[string]interface{})
	allTags := make([]interface{}, 0)
	tagNames := make(map[string]bool)

	for _, source := range document.Sources {
		if source == nil {
			continue
		}

		serviceName := strings.TrimSpace(source.Service)
		if serviceName == "" {
			return nil, errorx.NewError(serrors.ErrTypeDocumentNoService, document.Name)
		}

		serviceSpec, exists := b.findNamedSpec(serviceName, "文档", b.serviceSpecs)
		if !exists {
			return nil, b.namedSpecNotFoundError("文档", serviceName, b.serviceSpecs)
		}

		b.mergeDocumentTopLevelFields(result, serviceSpec, serviceName)

		selectedPaths, tagSelection := b.selectDocumentSourcePaths(serviceName, serviceSpec, source)
		for pathName, pathItem := range selectedPaths {
			b.mergeDocumentPathItem(allPaths, pathName, pathItem, serviceName)
		}

		b.mergeDocumentDefinitions(allDefinitions, serviceSpec, selectedPaths, serviceName)
		b.mergeDocumentTags(&allTags, serviceSpec, tagSelection, tagNames)
	}

	result[constants.FieldTags] = allTags
	result[constants.FieldXDocumentInfo] = map[string]interface{}{
		constants.FieldName:     document.Name,
		constants.FieldServices: b.collectDocumentServices(document),
		constants.FieldUpdated:  b.lastUpdated.Format(time.RFC3339),
	}

	return result, nil
}

// buildDocumentInfoFromSpec 从 DocumentSpec 构建 info 字段
func (b *Builder) buildDocumentInfoFromSpec(document *goswagger.DocumentSpec) map[string]interface{} {
	title := mathx.IfNotEmpty(strings.TrimSpace(document.Title), strings.TrimSpace(document.Name))
	description := mathx.IfNotEmpty(strings.TrimSpace(document.Description), "Subset document generated from aggregated services")
	version := mathx.IfNotEmpty(strings.TrimSpace(document.Version), mathx.IfNotEmpty(strings.TrimSpace(b.config.Version), "1.0.0"))

	info := map[string]interface{}{
		constants.FieldTitle:       title,
		constants.FieldDescription: description,
		constants.FieldVersion:     version,
	}

	return info
}

// mergeDocumentTopLevelFields 合并独立文档需要的顶层字段
func (b *Builder) mergeDocumentTopLevelFields(target, source map[string]interface{}, serviceName string) {
	b.mergeUniqueStringField(target, constants.FieldConsumes, source[constants.FieldConsumes])
	b.mergeUniqueStringField(target, constants.FieldProduces, source[constants.FieldProduces])
	b.mergeUniqueStringField(target, constants.FieldSchemes, source[constants.FieldSchemes])
	b.mergeUniqueInterfaceField(target, constants.FieldSecurity, source[constants.FieldSecurity])
	b.mergeSecurityDefinitions(target, source, serviceName)

	for _, field := range []string{constants.FieldBasePath, "host", "externalDocs"} {
		if _, exists := target[field]; exists {
			continue
		}
		if value, exists := source[field]; exists {
			target[field] = mathx.ConvertMapKeysToString(value)
		}
	}

	// 透传源规范顶层的 x- 扩展字段（如 x-date 等工具链元数据）
	for field, value := range source {
		if !strings.HasPrefix(field, "x-") {
			continue
		}
		if _, exists := target[field]; exists {
			continue
		}
		target[field] = mathx.ConvertMapKeysToString(value)
	}
}

// mergeUniqueStringField 合并字符串数组字段并去重
func (b *Builder) mergeUniqueStringField(target map[string]interface{}, field string, source interface{}) {
	sourceValues := toStringSlice(source)
	if len(sourceValues) == 0 {
		return
	}

	merged := append(toStringSlice(target[field]), sourceValues...)
	merged = mathx.FilterSliceByFunc(merged, func(value string) bool {
		return value != ""
	})
	target[field] = mathx.SliceUniq(merged)
}

// mergeUniqueInterfaceField 合并顶层对象数组字段并去重
func (b *Builder) mergeUniqueInterfaceField(target map[string]interface{}, field string, source interface{}) {
	sourceValues, _ := mathx.ConvertMapKeysToString(source).([]interface{})
	if len(sourceValues) == 0 {
		return
	}

	existing, _ := mathx.ConvertMapKeysToString(target[field]).([]interface{})
	seen := make(map[string]bool, len(existing)+len(sourceValues))
	merged := make([]interface{}, 0, len(existing)+len(sourceValues))

	for _, value := range existing {
		key := convert.MustString(value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, value)
	}

	for _, value := range sourceValues {
		key := convert.MustString(value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, mathx.ConvertMapKeysToString(value))
	}

	target[field] = merged
}

// mergeSecurityDefinitions 合并 securityDefinitions
func (b *Builder) mergeSecurityDefinitions(target, source map[string]interface{}, serviceName string) {
	sourceDefinitions, ok := source[constants.FieldSecurityDefs].(map[string]interface{})
	if !ok || len(sourceDefinitions) == 0 {
		return
	}

	targetDefinitions, ok := target[constants.FieldSecurityDefs].(map[string]interface{})
	if !ok {
		targetDefinitions = make(map[string]interface{})
		target[constants.FieldSecurityDefs] = targetDefinitions
	}

	for defName, definition := range sourceDefinitions {
		clonedDefinition := mathx.ConvertMapKeysToString(definition)
		if existingDefinition, exists := targetDefinitions[defName]; exists {
			b.checkDefinitionConsistency(defName, existingDefinition, clonedDefinition, serviceName)
			continue
		}
		targetDefinitions[defName] = clonedDefinition
	}
}

// checkDefinitionConsistency 检查定义一致性
func (b *Builder) checkDefinitionConsistency(defName string, existingDef, newDef interface{}, serviceName string) {
	existingJSON, err := json.Marshal(existingDef)
	if err != nil {
		b.logger.Warn("序列化现有定义 %s 失败，跳过一致性检查: %v", defName, err)
		return
	}
	newJSON, err := json.Marshal(newDef)
	if err != nil {
		b.logger.Warn("序列化新定义 %s 失败，跳过一致性检查: %v", defName, err)
		return
	}
	if string(existingJSON) != string(newJSON) {
		b.logger.Warn("类型 %s 在不同服务中定义不一致！当前使用第一个定义，忽略 %s 的定义", defName, serviceName)
	}
}
