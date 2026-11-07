/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-22 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-22 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\aggregate\endpoint.go
 * @Description: 端点提取 - 从已加载的服务规范或内嵌 Swagger 数据中提取结构化接口端点列表
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package aggregate

import (
	"fmt"
	"sort"
	"strings"

	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	loader "github.com/kamalyes/go-rpc-gateway/middleware/swagger/loader"
	convert "github.com/kamalyes/go-toolbox/pkg/convert"
)

// Endpoint 表示从 Swagger 规范中提取的接口端点
type Endpoint struct {
	SourceFile  string   // 来源文件名（如 proto/tenant/tenant_service.swagger.yaml）
	ServiceName string   // 来源服务名（文件名去掉扩展名和下划线后的 PascalCase）
	Path        string   // API 路径
	Method      string   // HTTP 方法（大写，如 GET、POST）
	OperationID string   // operationId
	Summary     string   // 接口摘要
	Tags        []string // 标签列表
}

// ExtractEndpoints 从已加载的服务规范中提取所有端点
// 返回按服务名排序的端点列表，同一 operationId 只保留首次出现的端点
func (a *Aggregator) ExtractEndpoints() []Endpoint {
	if len(a.serviceSpecs) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	endpoints := make([]Endpoint, 0)

	for _, serviceName := range a.getSortedServiceNames() {
		spec := a.serviceSpecs[serviceName]
		extractEndpointsFromSpec(spec, serviceName, "", seen, &endpoints)
	}

	return endpoints
}

// ExtractEndpointsFromEmbed 从内嵌的 Swagger 文件数据中提取所有端点
// swaggerFiles: 文件名 -> 文件内容的映射（GetSwaggerFiles() 的返回值）
func ExtractEndpointsFromEmbed(swaggerFiles map[string][]byte) ([]Endpoint, error) {
	if len(swaggerFiles) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{})
	endpoints := make([]Endpoint, 0)

	// 按文件名排序保证输出稳定
	filenames := make([]string, 0, len(swaggerFiles))
	for name := range swaggerFiles {
		filenames = append(filenames, name)
	}
	sort.Strings(filenames)

	for _, filename := range filenames {
		data := swaggerFiles[filename]
		if len(data) == 0 {
			continue
		}

		spec, err := loader.LoadSpecFromBytes(filename, data)
		if err != nil {
			return nil, fmt.Errorf("解析 swagger %s 失败: %w", filename, err)
		}

		serviceName := serviceNameFromFilename(filename)
		extractEndpointsFromSpec(spec, serviceName, filename, seen, &endpoints)
	}

	return endpoints, nil
}

// extractEndpointsFromSpec 从单个 spec 中提取端点，按 path 和 method 排序保证输出稳定
func extractEndpointsFromSpec(spec map[string]interface{}, serviceName, sourceFile string, seen map[string]struct{}, endpoints *[]Endpoint) {
	paths, ok := spec[constants.FieldPaths].(map[string]interface{})
	if !ok {
		return
	}

	// 按 path 排序
	sortedPaths := make([]string, 0, len(paths))
	for path := range paths {
		sortedPaths = append(sortedPaths, path)
	}
	sort.Strings(sortedPaths)

	for _, path := range sortedPaths {
		pathData := paths[path]
		pathItem, ok := pathData.(map[string]interface{})
		if !ok {
			continue
		}

		// 按 method 排序
		sortedMethods := make([]string, 0, len(pathItem))
		for method := range pathItem {
			sortedMethods = append(sortedMethods, method)
		}
		sort.Strings(sortedMethods)

		for _, method := range sortedMethods {
			operation := pathItem[method]
			normalized := constants.NormalizeHTTPMethod(method)
			if normalized == "" {
				continue
			}

			opMap, ok := operation.(map[string]interface{})
			if !ok {
				continue
			}

			operationID := convert.MustString(opMap["operationId"])
			summary := convert.MustString(opMap["summary"])
			tags := extractStringSlice(opMap["tags"])

			if operationID != "" {
				if _, exists := seen[operationID]; exists {
					continue
				}
				seen[operationID] = struct{}{}
			}

			*endpoints = append(*endpoints, Endpoint{
				SourceFile:  filepathToSlash(sourceFile),
				ServiceName: serviceName,
				Path:        path,
				Method:      normalized,
				OperationID: operationID,
				Summary:     summary,
				Tags:        tags,
			})
		}
	}
}

// serviceNameFromFilename 从 swagger 文件名推导服务名
// 规则：取文件名部分，去掉 .swagger.yaml/.swagger.json 后缀，按下划线分割后 PascalCase 拼接
// 例如：proto/tenant/tenant_service.swagger.yaml -> TenantService
func serviceNameFromFilename(filename string) string {
	base := filename
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	if idx := strings.LastIndex(base, "\\"); idx >= 0 {
		base = base[idx+1:]
	}
	base = strings.TrimSuffix(base, ".swagger.yaml")
	base = strings.TrimSuffix(base, ".swagger.json")
	base = strings.TrimSuffix(base, ".yaml")
	base = strings.TrimSuffix(base, ".json")

	parts := strings.Split(base, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}

// filepathToSlash 将路径分隔符统一为 /
func filepathToSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

// extractStringSlice 从 interface{} 中提取字符串切片
func extractStringSlice(value interface{}) []string {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		return result
	default:
		s := strings.TrimSpace(convert.MustString(value))
		if s == "" {
			return nil
		}
		return []string{s}
	}
}
