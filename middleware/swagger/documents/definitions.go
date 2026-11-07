/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\documents\definitions.go
 * @Description: 独立子文档定义合并 - 递归合并 definitions、引用收集
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package documents

import (
	"strings"

	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	mathx "github.com/kamalyes/go-toolbox/pkg/mathx"
)

// mergeDocumentDefinitions 递归合并独立文档所需 definitions
func (b *Builder) mergeDocumentDefinitions(targetDefinitions map[string]interface{}, serviceSpec map[string]interface{}, selectedPaths map[string]map[string]interface{}, serviceName string) {
	sourceDefinitions, ok := serviceSpec[constants.FieldDefs].(map[string]interface{})
	if !ok || len(sourceDefinitions) == 0 {
		return
	}

	queued := make(map[string]bool)
	processed := make(map[string]bool)
	queue := make([]string, 0)

	for _, pathItem := range selectedPaths {
		enqueueDefinitionRefs(pathItem, queued, &queue)
	}

	for len(queue) > 0 {
		definitionName := queue[0]
		queue = queue[1:]

		if processed[definitionName] {
			continue
		}
		processed[definitionName] = true

		definition, exists := sourceDefinitions[definitionName]
		if !exists {
			b.logger.Warn("独立文档引用的 definition 不存在: service=%s, definition=%s", serviceName, definitionName)
			continue
		}

		clonedDefinition := mathx.ConvertMapKeysToString(definition)
		if existingDefinition, exists := targetDefinitions[definitionName]; exists {
			b.checkDefinitionConsistency(definitionName, existingDefinition, clonedDefinition, serviceName)
		} else {
			targetDefinitions[definitionName] = clonedDefinition
		}

		enqueueDefinitionRefs(clonedDefinition, queued, &queue)
	}

	// 补充所有枚举类型定义，即使未被 $ref 引用
	// 枚举定义可能仅作为内联 query 参数使用（如 enumsRefreshInterval），
	// 不被 $ref 引用但仍需要包含在输出中供客户端生成器使用
	for defName, definition := range sourceDefinitions {
		if processed[defName] {
			continue
		}
		clonedDefinition := mathx.ConvertMapKeysToString(definition)
		if isEnumDefinition(clonedDefinition) {
			if _, exists := targetDefinitions[defName]; !exists {
				targetDefinitions[defName] = clonedDefinition
				b.logger.Debug("补充枚举类型定义: service=%s, definition=%s", serviceName, defName)
			}
			processed[defName] = true
		}
	}
}

// isEnumDefinition 检查定义是否为枚举类型（type=string 且有非空 enum 数组）
func isEnumDefinition(definition interface{}) bool {
	defMap, ok := definition.(map[string]interface{})
	if !ok {
		return false
	}
	typeValue, ok := defMap["type"].(string)
	if !ok || typeValue != "string" {
		return false
	}
	enumValue, exists := defMap["enum"]
	if !exists {
		return false
	}
	enumSlice, ok := enumValue.([]interface{})
	return ok && len(enumSlice) > 0
}

// enqueueDefinitionRefs 从对象中提取 definition 引用并入队
func enqueueDefinitionRefs(obj interface{}, queued map[string]bool, queue *[]string) {
	refs := make(map[string]struct{})
	collectDefinitionRefs(obj, refs)

	for refName := range refs {
		if queued[refName] {
			continue
		}
		queued[refName] = true
		*queue = append(*queue, refName)
	}
}

// collectDefinitionRefs 递归收集 #/definitions 引用
func collectDefinitionRefs(obj interface{}, refs map[string]struct{}) {
	switch value := obj.(type) {
	case map[string]interface{}:
		if refName := extractDefinitionName(value); refName != "" {
			refs[refName] = struct{}{}
		}
		for _, nested := range value {
			collectDefinitionRefs(nested, refs)
		}
	case []interface{}:
		for _, nested := range value {
			collectDefinitionRefs(nested, refs)
		}
	}
}

// extractDefinitionName 提取 definition 名称
func extractDefinitionName(value map[string]interface{}) string {
	refValue, ok := value[constants.FieldRef].(string)
	if !ok || !strings.HasPrefix(refValue, constants.PathDefinitions) {
		return ""
	}

	return strings.TrimPrefix(refValue, constants.PathDefinitions)
}
