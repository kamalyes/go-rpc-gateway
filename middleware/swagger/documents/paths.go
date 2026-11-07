/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\documents\paths.go
 * @Description: 独立子文档路径选择 - include/exclude 路径与方法筛选
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package documents

import (
	"strings"

	goswagger "github.com/kamalyes/go-rpc-gateway/config/swagger"
	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	mathx "github.com/kamalyes/go-toolbox/pkg/mathx"
)

// swaggerOperationMethods Swagger 规范中合法的 HTTP 操作方法
var swaggerOperationMethods = map[string]struct{}{
	"get":     {},
	"put":     {},
	"post":    {},
	"delete":  {},
	"options": {},
	"head":    {},
	"patch":   {},
}

// selectDocumentSourcePaths 选出单个 source 对应的 path/method 子集
func (b *Builder) selectDocumentSourcePaths(serviceName string, serviceSpec map[string]interface{}, source *goswagger.DocumentSource) (map[string]map[string]interface{}, map[string]struct{}) {
	selected := make(map[string]map[string]interface{})

	paths, ok := serviceSpec[constants.FieldPaths].(map[string]interface{})
	if !ok {
		return selected, map[string]struct{}{}
	}

	includeSelectors := source.GetEffectiveInclude()
	if len(includeSelectors) == 0 {
		for pathName, pathValue := range paths {
			pathItem, ok := pathValue.(map[string]interface{})
			if !ok {
				continue
			}
			if cloned := b.cloneSelectedPathItem(pathItem, nil); cloned != nil {
				selected[pathName] = cloned
			}
		}
	} else {
		for _, selector := range includeSelectors {
			if selector == nil || selector.Path == "" {
				continue
			}

			pathValue, exists := paths[selector.Path]
			if !exists {
				b.logger.Warn("独立文档 include 未匹配到路径: service=%s, path=%s", serviceName, selector.Path)
				continue
			}

			pathItem, ok := pathValue.(map[string]interface{})
			if !ok {
				continue
			}

			cloned := b.cloneSelectedPathItem(pathItem, selector.Methods)
			if cloned == nil {
				b.logger.Warn("独立文档 include 未匹配到方法: service=%s, path=%s, methods=%v", serviceName, selector.Path, selector.Methods)
				continue
			}

			b.mergeDocumentPathItemSelection(selected, selector.Path, cloned)
		}
	}

	for _, selector := range source.Exclude {
		if selector == nil || selector.Path == "" {
			continue
		}
		b.removeDocumentPathSelection(selected, selector)
	}

	return selected, b.collectOperationTagNames(selected)
}

// cloneSelectedPathItem 复制 path item 并只保留指定 method
func (b *Builder) cloneSelectedPathItem(pathItem map[string]interface{}, methods []string) map[string]interface{} {
	methodSet := normalizeHTTPMethods(methods)
	selectAllMethods := len(methodSet) == 0

	cloned := make(map[string]interface{})
	methodCount := 0

	for key, value := range pathItem {
		if isSwaggerOperationMethod(key) {
			if !selectAllMethods && !methodSet[strings.ToLower(key)] {
				continue
			}
			cloned[key] = mathx.ConvertMapKeysToString(value)
			methodCount++
			continue
		}

		cloned[key] = mathx.ConvertMapKeysToString(value)
	}

	if methodCount == 0 {
		return nil
	}

	return cloned
}

// mergeDocumentPathItem 将选中的 path item 合并到文档结果
func (b *Builder) mergeDocumentPathItem(target map[string]interface{}, pathName string, newPathItem map[string]interface{}, serviceName string) {
	if existingValue, exists := target[pathName]; exists {
		existingPathItem, ok := existingValue.(map[string]interface{})
		if !ok {
			target[pathName] = newPathItem
			return
		}

		for key, value := range newPathItem {
			if isSwaggerOperationMethod(key) {
				if _, methodExists := existingPathItem[key]; methodExists {
					b.logger.Warn("独立文档路径 %s 的方法 %s 重复定义 (来自: %s)，保留首次加载的定义", pathName, key, serviceName)
					continue
				}
				existingPathItem[key] = value
				continue
			}

			if _, exists := existingPathItem[key]; !exists {
				existingPathItem[key] = value
			}
		}
		return
	}

	target[pathName] = newPathItem
}

// mergeDocumentPathItemSelection 合并同一路径的 include 结果
func (b *Builder) mergeDocumentPathItemSelection(target map[string]map[string]interface{}, pathName string, newPathItem map[string]interface{}) {
	if existingPathItem, exists := target[pathName]; exists {
		for key, value := range newPathItem {
			if _, exists := existingPathItem[key]; !exists {
				existingPathItem[key] = value
			}
		}
		return
	}

	target[pathName] = newPathItem
}

// removeDocumentPathSelection 在已选结果上应用 exclude
func (b *Builder) removeDocumentPathSelection(selected map[string]map[string]interface{}, selector *goswagger.DocumentPathSelector) {
	pathItem, exists := selected[selector.Path]
	if !exists {
		return
	}

	methodSet := normalizeHTTPMethods(selector.Methods)
	if len(methodSet) == 0 {
		delete(selected, selector.Path)
		return
	}

	for method := range methodSet {
		delete(pathItem, method)
	}

	if !pathItemHasOperations(pathItem) {
		delete(selected, selector.Path)
	}
}

// isSwaggerOperationMethod 判断是否是 Swagger Path Item 中的 HTTP 方法
func isSwaggerOperationMethod(method string) bool {
	_, exists := swaggerOperationMethods[strings.ToLower(method)]
	return exists
}

// normalizeHTTPMethods 将 HTTP 方法标准化为小写集合
func normalizeHTTPMethods(methods []string) map[string]bool {
	result := make(map[string]bool, len(methods))
	for _, method := range methods {
		normalized := strings.ToLower(strings.TrimSpace(method))
		if normalized == "" {
			continue
		}
		result[normalized] = true
	}
	return result
}

// pathItemHasOperations 判断 path item 是否还包含 HTTP operation
func pathItemHasOperations(pathItem map[string]interface{}) bool {
	for key := range pathItem {
		if isSwaggerOperationMethod(key) {
			return true
		}
	}
	return false
}
