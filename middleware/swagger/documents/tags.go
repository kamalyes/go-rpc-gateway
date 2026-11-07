/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\documents\tags.go
 * @Description: 独立子文档标签合并 - 标签去重、操作标签收集
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package documents

import (
	"sort"

	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	convert "github.com/kamalyes/go-toolbox/pkg/convert"
	mathx "github.com/kamalyes/go-toolbox/pkg/mathx"
)

// mergeDocumentTags 合并独立文档需要的 tags
func (b *Builder) mergeDocumentTags(target *[]interface{}, serviceSpec map[string]interface{}, selectedTagNames map[string]struct{}, tagNames map[string]bool) {
	if len(selectedTagNames) == 0 {
		return
	}

	foundTags := make(map[string]bool, len(selectedTagNames))
	if sourceTags, ok := serviceSpec[constants.FieldTags].([]interface{}); ok {
		for _, tag := range sourceTags {
			tagMap, ok := tag.(map[string]interface{})
			if !ok {
				continue
			}

			tagName := convert.MustString(tagMap[constants.FieldName])
			if tagName == "" {
				continue
			}

			if _, selected := selectedTagNames[tagName]; !selected {
				continue
			}

			foundTags[tagName] = true
			b.addUniqueTag(tagName, mathx.ConvertMapKeysToString(tagMap), target, tagNames)
		}
	}

	missing := make([]string, 0)
	for tagName := range selectedTagNames {
		if !foundTags[tagName] {
			missing = append(missing, tagName)
		}
	}
	sort.Strings(missing)

	for _, tagName := range missing {
		b.addUniqueTag(tagName, map[string]interface{}{constants.FieldName: tagName}, target, tagNames)
	}
}

// addUniqueTag 添加唯一标签（通用去重逻辑）
func (b *Builder) addUniqueTag(tagKey string, tag interface{}, allTags *[]interface{}, tagSet map[string]bool) bool {
	if tagKey == "" || tagSet[tagKey] {
		return false
	}
	tagSet[tagKey] = true
	*allTags = append(*allTags, tag)
	return true
}

// collectOperationTagNames 收集已选 operation 使用到的 tag
func (b *Builder) collectOperationTagNames(selectedPaths map[string]map[string]interface{}) map[string]struct{} {
	tagNames := make(map[string]struct{})

	for _, pathItem := range selectedPaths {
		for method, operation := range pathItem {
			if !isSwaggerOperationMethod(method) {
				continue
			}

			operationMap, ok := operation.(map[string]interface{})
			if !ok {
				continue
			}

			for _, tagName := range toStringSlice(operationMap[constants.FieldTags]) {
				if tagName != "" {
					tagNames[tagName] = struct{}{}
				}
			}
		}
	}

	return tagNames
}

// toStringSlice 将任意字符串数组类型转换为 []string
func toStringSlice(value interface{}) []string {
	switch actual := value.(type) {
	case nil:
		return nil
	case []string:
		return actual
	case []interface{}:
		result := make([]string, 0, len(actual))
		for _, v := range actual {
			if s, ok := v.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}
