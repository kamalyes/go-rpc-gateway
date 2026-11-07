/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\aggregate\preprocess.go
 * @Description: 服务规范预处理 - BasePath 更新、标签注入
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package aggregate

import (
	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	convert "github.com/kamalyes/go-toolbox/pkg/convert"
)

// preprocessServiceSpec 预处理服务规范
// 包括更新 BasePath 和为操作添加服务标签
func (a *Aggregator) preprocessServiceSpec(spec map[string]interface{}, service ServiceSpecConfig) {
	if service.GetBasePath() != "" {
		a.updatePathsWithBasePath(spec, service.GetBasePath())
	}
	a.addServiceTagsToOperations(spec, service)
}

// updatePathsWithBasePath 更新路径的 BasePath
func (a *Aggregator) updatePathsWithBasePath(spec map[string]interface{}, basePath string) {
	if _, ok := spec[constants.FieldPaths].(map[string]interface{}); ok {
		spec[constants.FieldBasePath] = basePath
		a.logger.Debug("更新服务BasePath: %s", basePath)
	}
}

// addServiceTagsToOperations 为操作添加服务标签
// 只在配置了额外标签时才添加，否则保持原始标签不变
func (a *Aggregator) addServiceTagsToOperations(spec map[string]interface{}, service ServiceSpecConfig) {
	if len(service.GetTags()) == 0 {
		a.logger.Debug("服务 %s 未配置额外标签，保持原始标签", service.GetName())
		return
	}

	paths, ok := spec[constants.FieldPaths].(map[string]interface{})
	if !ok {
		return
	}

	serviceTags := a.buildServiceTags(service)
	a.processAllPaths(paths, serviceTags)
}

// buildServiceTags 构建服务标签列表
func (a *Aggregator) buildServiceTags(service ServiceSpecConfig) []interface{} {
	tags := service.GetTags()
	if len(tags) == 0 {
		return nil
	}
	serviceTags := make([]interface{}, len(tags))
	for i, tag := range tags {
		serviceTags[i] = tag
	}
	return serviceTags
}

// processAllPaths 处理所有路径，添加标签
func (a *Aggregator) processAllPaths(paths map[string]interface{}, serviceTags []interface{}) {
	for pathName, pathData := range paths {
		pathMap, ok := pathData.(map[string]interface{})
		if !ok {
			continue
		}
		a.processPathMethods(pathName, pathMap, serviceTags)
	}
}

// processPathMethods 处理单个路径下的所有 HTTP 方法
func (a *Aggregator) processPathMethods(pathName string, pathMap map[string]interface{}, serviceTags []interface{}) {
	for method, operation := range pathMap {
		opMap, ok := operation.(map[string]interface{})
		if !ok {
			continue
		}
		a.mergeOperationTags(pathName, method, opMap, serviceTags)
	}
}

// mergeOperationTags 合并操作的标签
func (a *Aggregator) mergeOperationTags(pathName, method string, opMap map[string]interface{}, serviceTags []interface{}) {
	existingTags := a.extractExistingTags(opMap)
	mergedTags := a.mergeOperationTagsLists(existingTags, serviceTags)

	opMap[constants.FieldTags] = mergedTags
	a.logger.Debug("路径 %s %s: 原始标签%v + 额外标签%v → 最终%v",
		method, pathName, existingTags, serviceTags, mergedTags)
}

// extractExistingTags 提取现有标签
func (a *Aggregator) extractExistingTags(opMap map[string]interface{}) []interface{} {
	if tags, exists := opMap[constants.FieldTags]; exists {
		if tagList, ok := tags.([]interface{}); ok {
			return tagList
		}
	}
	return nil
}

// mergeOperationTagsLists 合并两个操作标签列表并去重
func (a *Aggregator) mergeOperationTagsLists(existingTags, newTags []interface{}) []interface{} {
	if len(existingTags) == 0 {
		return newTags
	}
	if len(newTags) == 0 {
		return existingTags
	}

	allTags := make([]interface{}, 0, len(existingTags)+len(newTags))
	allTags = append(allTags, existingTags...)
	allTags = append(allTags, newTags...)

	seen := make(map[string]bool, len(allTags))
	result := make([]interface{}, 0, len(allTags))

	for _, tag := range allTags {
		tagStr := convert.MustString(tag)
		if tagStr != "" && !seen[tagStr] {
			seen[tagStr] = true
			result = append(result, tag)
		}
	}

	return result
}
