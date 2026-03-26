/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-25 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-25 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger_documents.go
 * @Description: Swagger 独立子文档实现
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	goswagger "github.com/kamalyes/go-config/pkg/swagger"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	commonpb "github.com/kamalyes/go-rpc-gateway/proto"
	"github.com/kamalyes/go-toolbox/pkg/convert"
	"github.com/kamalyes/go-toolbox/pkg/mathx"
)

const (
	swaggerFieldHost                = "host"
	swaggerFieldSchemes             = "schemes"
	swaggerFieldSecurity            = "security"
	swaggerFieldSecurityDefinitions = "securityDefinitions"
	swaggerFieldExternalDocs        = "externalDocs"
	defaultDocumentDescription      = "Subset document generated from aggregated services"
)

var swaggerOperationMethods = map[string]struct{}{
	"get":     {},
	"put":     {},
	"post":    {},
	"delete":  {},
	"options": {},
	"head":    {},
	"patch":   {},
}

type documentSummary struct {
	Name        string
	Title       string
	Description string
	Version     string
	Services    []string
}

// handleDocumentJSON 处理独立文档 JSON 请求
func (s *SwaggerMiddleware) handleDocumentJSON(w http.ResponseWriter, r *http.Request) {
	writeSwaggerJSONHeaders(w)
	if handleSwaggerOptions(w, r) {
		return
	}

	if !s.IsAggregateEnabled() {
		writeSwaggerError(w, http.StatusNotFound, commonpb.StatusCode_NotFound, "聚合功能未启用")
		return
	}

	documentName := s.extractSwaggerEntityName(r.URL.Path, constants.SwaggerPathDocumentPrefix)
	if documentName == "" {
		writeSwaggerError(w, http.StatusBadRequest, commonpb.StatusCode_InvalidArgument, "文档名称不能为空")
		return
	}

	jsonData, err := s.GetDocumentSpec(documentName)
	if err != nil {
		global.LOGGER.Error("获取独立文档 %s 的规范失败: %v", documentName, err)
		writeSwaggerError(w, http.StatusNotFound, commonpb.StatusCode_NotFound, fmt.Sprintf("文档 %s 的规范不存在", documentName))
		return
	}

	w.Write(jsonData)
}

// handleDocumentUI 处理独立文档 UI 请求
func (s *SwaggerMiddleware) handleDocumentUI(w http.ResponseWriter, r *http.Request) {
	writeSwaggerHTMLHeaders(w)
	if handleSwaggerOptions(w, r) {
		return
	}

	if !s.IsAggregateEnabled() {
		http.Error(w, "聚合功能未启用", http.StatusNotFound)
		return
	}

	documentName := s.extractSwaggerEntityName(r.URL.Path, constants.SwaggerPathDocumentPrefix)
	if documentName == "" {
		http.Error(w, "文档名称不能为空", http.StatusBadRequest)
		return
	}

	spec, exists := s.findNamedSpec(documentName, "文档", s.documentSpecs)
	if !exists {
		http.Error(w, fmt.Sprintf("文档 %s 不存在", documentName), http.StatusNotFound)
		return
	}

	w.Write([]byte(s.generateDocumentSwaggerUI(documentName, s.resolveSwaggerSpecTitle(spec, documentName))))
}

// handleDocumentsIndex 处理独立文档列表页
func (s *SwaggerMiddleware) handleDocumentsIndex(w http.ResponseWriter, _ *http.Request) {
	if !s.IsAggregateEnabled() {
		writeSwaggerError(w, http.StatusNotFound, commonpb.StatusCode_NotFound, "聚合功能未启用")
		return
	}

	writeSwaggerHTMLHeaders(w)
	w.Write([]byte(s.buildDocumentsHTML()))
}

// generateDocumentSwaggerUI 生成独立文档 Swagger UI 页面
func (s *SwaggerMiddleware) generateDocumentSwaggerUI(documentName, title string) string {
	return s.generateScopedSwaggerUI(
		title,
		title,
		"独立子文档 API 视图",
		fmt.Sprintf("%s/documents/%s.json", s.config.UIPath, documentName),
		s.commonSwaggerUIActions(),
	)
}

// buildDocumentsHTML 构建独立文档列表 HTML 页面
func (s *SwaggerMiddleware) buildDocumentsHTML() string {
	summaries := s.collectDocumentSummaries()

	html := `<!DOCTYPE html>
<html lang="` + constants.SwaggerHTMLLangZH + `">
<head>
    <meta charset="` + constants.SwaggerHTMLCharset + `">
    <meta name="viewport" content="` + constants.HTMLMetaViewport + `">
    <title>` + s.config.Title + ` - 独立文档列表</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
            background: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .top-actions {
            text-align: center;
            margin: 30px 0;
            padding: 20px;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .documents-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
            gap: 20px;
        }
        .document-card {
            background: white;
            padding: 24px;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }
        .document-name {
            font-size: 1.35em;
            font-weight: 600;
            color: #2c3e50;
            margin-bottom: 10px;
        }
        .document-desc {
            color: #666;
            margin-bottom: 15px;
            line-height: 1.5;
        }
        .document-version {
            display: inline-block;
            background: #e8f5e9;
            color: #2e7d32;
            padding: 4px 12px;
            border-radius: 12px;
            font-size: 0.85em;
            font-weight: 500;
            margin-bottom: 12px;
        }
        .document-services {
            color: #555;
            margin-bottom: 16px;
            line-height: 1.5;
        }
        .document-actions {
            display: flex;
            gap: 10px;
        }
        .btn {
            padding: 8px 16px;
            border: none;
            border-radius: 4px;
            text-decoration: none;
            font-size: 0.9em;
            font-weight: 500;
            cursor: pointer;
        }
        .btn-primary {
            background-color: #1976d2;
            color: white;
        }
        .btn-secondary {
            background-color: #f5f5f5;
            color: #555;
            border: 1px solid #ddd;
        }
        .empty {
            background: white;
            padding: 32px;
            border-radius: 8px;
            text-align: center;
            color: #666;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>` + s.config.Title + `</h1>
        <p>按 path + method 组合出来的独立 Swagger 子文档</p>
    </div>

    <div class="top-actions">
        <a href="` + s.config.UIPath + `" class="btn btn-primary">查看聚合文档</a>
        <a href="` + s.config.UIPath + `/services" class="btn btn-secondary">查看服务列表</a>
        <a href="` + s.config.UIPath + `/aggregate.json" class="btn btn-secondary">下载聚合 JSON</a>
    </div>`

	if len(summaries) == 0 {
		html += `
    <div class="empty">
        当前没有可用的独立文档，请检查 swagger.aggregate.documents 配置。
    </div>
</body>
</html>`
		return html
	}

	html += `
    <div class="documents-grid">`

	for _, summary := range summaries {
		html += `
        <div class="document-card">
            <div class="document-name">` + summary.Title + `</div>`

		if summary.Description != "" {
			html += `<div class="document-desc">` + summary.Description + `</div>`
		}

		if summary.Version != "" {
			html += `<div class="document-version">v` + summary.Version + `</div>`
		}

		if len(summary.Services) > 0 {
			html += `<div class="document-services">来源服务: ` + strings.Join(summary.Services, ", ") + `</div>`
		}

		html += `
            <div class="document-actions">
                <a href="` + s.config.UIPath + `/documents/` + summary.Name + `" class="btn btn-primary">查看文档</a>
                <a href="` + s.config.UIPath + `/documents/` + summary.Name + `.json" class="btn btn-secondary">下载 JSON</a>
            </div>
        </div>`
	}

	html += `
    </div>
</body>
</html>`

	return html
}

// buildDocumentSpecs 构建全部独立子文档
func (s *SwaggerMiddleware) buildDocumentSpecs() error {
	s.documentSpecs = make(map[string]map[string]interface{})

	if s.config.Aggregate == nil || len(s.config.Aggregate.Documents) == 0 {
		return nil
	}

	seenDocuments := make(map[string]bool)
	for _, document := range s.config.Aggregate.Documents {
		if document == nil || !document.Enabled {
			continue
		}

		documentName := strings.TrimSpace(document.Name)
		if documentName == "" {
			return fmt.Errorf("独立文档名称不能为空")
		}

		if seenDocuments[documentName] {
			return fmt.Errorf("独立文档 %s 重复配置", documentName)
		}
		seenDocuments[documentName] = true

		spec, err := s.buildSingleDocumentSpec(document)
		if err != nil {
			return fmt.Errorf("构建独立文档 %s 失败: %w", documentName, err)
		}

		s.documentSpecs[documentName] = spec
		global.LOGGER.Info("✅ 独立文档 %s 构建完成，路径数: %d", documentName, len(spec[constants.SwaggerFieldPaths].(map[string]interface{})))
	}

	return nil
}

// buildSingleDocumentSpec 构建单个独立子文档
func (s *SwaggerMiddleware) buildSingleDocumentSpec(document *goswagger.DocumentSpec) (map[string]interface{}, error) {
	if len(document.Sources) == 0 {
		return nil, fmt.Errorf("文档 %s 未配置 sources", document.Name)
	}

	result := map[string]interface{}{
		constants.SwaggerFieldSwagger:  constants.SwaggerVersion,
		constants.SwaggerFieldInfo:     s.buildDocumentInfo(document),
		constants.SwaggerFieldConsumes: []string{constants.MimeApplicationJSON},
		constants.SwaggerFieldProduces: []string{constants.MimeApplicationJSON},
		constants.SwaggerFieldPaths:    make(map[string]interface{}),
		constants.SwaggerFieldDefs:     make(map[string]interface{}),
		constants.SwaggerFieldTags:     make([]interface{}, 0),
	}

	allPaths := result[constants.SwaggerFieldPaths].(map[string]interface{})
	allDefinitions := result[constants.SwaggerFieldDefs].(map[string]interface{})
	allTags := make([]interface{}, 0)
	tagNames := make(map[string]bool)

	for _, source := range document.Sources {
		if source == nil {
			continue
		}

		serviceName := strings.TrimSpace(source.Service)
		if serviceName == "" {
			return nil, fmt.Errorf("文档 %s 存在未配置 service 的 source", document.Name)
		}

		serviceSpec, exists := s.findNamedSpec(serviceName, "文档", s.serviceSpecs)
		if !exists {
			return nil, s.namedSpecNotFoundError("文档", serviceName, s.serviceSpecs)
		}

		s.mergeDocumentTopLevelFields(result, serviceSpec, serviceName)

		selectedPaths, tagSelection := s.selectDocumentSourcePaths(serviceName, serviceSpec, source)
		for pathName, pathItem := range selectedPaths {
			s.mergeDocumentPathItem(allPaths, pathName, pathItem, serviceName)
		}

		s.mergeDocumentDefinitions(allDefinitions, serviceSpec, selectedPaths, serviceName)
		s.mergeDocumentTags(&allTags, serviceSpec, tagSelection, tagNames)
	}

	result[constants.SwaggerFieldTags] = allTags
	result[constants.SwaggerFieldXDocumentInfo] = map[string]interface{}{
		constants.SwaggerFieldName:     document.Name,
		constants.SwaggerFieldServices: s.collectDocumentServices(document),
		constants.SwaggerFieldUpdated:  s.lastUpdated.Format(time.RFC3339),
	}

	return result, nil
}

// buildDocumentInfo 构建独立文档 info 字段
func (s *SwaggerMiddleware) buildDocumentInfo(document *goswagger.DocumentSpec) map[string]interface{} {
	title := mathx.IfNotEmpty(strings.TrimSpace(document.Title), strings.TrimSpace(document.Name))
	description := mathx.IfNotEmpty(strings.TrimSpace(document.Description), defaultDocumentDescription)
	version := mathx.IfNotEmpty(strings.TrimSpace(document.Version), mathx.IfNotEmpty(strings.TrimSpace(s.config.Version), "1.0.0"))

	info := map[string]interface{}{
		constants.SwaggerFieldTitle:       title,
		constants.SwaggerFieldDescription: description,
		constants.SwaggerFieldVersion:     version,
	}

	if contact := s.buildContactInfo(); contact != nil {
		info[constants.SwaggerFieldContact] = contact
	}
	if license := s.buildLicenseInfo(); license != nil {
		info[constants.SwaggerFieldLicense] = license
	}

	return info
}

// mergeDocumentTopLevelFields 合并独立文档需要的顶层字段
func (s *SwaggerMiddleware) mergeDocumentTopLevelFields(target, source map[string]interface{}, serviceName string) {
	s.mergeUniqueStringField(target, constants.SwaggerFieldConsumes, source[constants.SwaggerFieldConsumes])
	s.mergeUniqueStringField(target, constants.SwaggerFieldProduces, source[constants.SwaggerFieldProduces])
	s.mergeUniqueStringField(target, swaggerFieldSchemes, source[swaggerFieldSchemes])
	s.mergeUniqueInterfaceField(target, swaggerFieldSecurity, source[swaggerFieldSecurity])
	s.mergeSecurityDefinitions(target, source, serviceName)

	for _, field := range []string{constants.SwaggerFieldBasePath, swaggerFieldHost, swaggerFieldExternalDocs} {
		if _, exists := target[field]; exists {
			continue
		}
		if value, exists := source[field]; exists {
			target[field] = mathx.ConvertMapKeysToString(value)
		}
	}
}

// mergeUniqueStringField 合并字符串数组字段并去重
func (s *SwaggerMiddleware) mergeUniqueStringField(target map[string]interface{}, field string, source interface{}) {
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
func (s *SwaggerMiddleware) mergeUniqueInterfaceField(target map[string]interface{}, field string, source interface{}) {
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
func (s *SwaggerMiddleware) mergeSecurityDefinitions(target, source map[string]interface{}, serviceName string) {
	sourceDefinitions, ok := source[swaggerFieldSecurityDefinitions].(map[string]interface{})
	if !ok || len(sourceDefinitions) == 0 {
		return
	}

	targetDefinitions, ok := target[swaggerFieldSecurityDefinitions].(map[string]interface{})
	if !ok {
		targetDefinitions = make(map[string]interface{})
		target[swaggerFieldSecurityDefinitions] = targetDefinitions
	}

	for defName, definition := range sourceDefinitions {
		clonedDefinition := mathx.ConvertMapKeysToString(definition)
		if existingDefinition, exists := targetDefinitions[defName]; exists {
			s.checkDefinitionConsistency(defName, existingDefinition, clonedDefinition, serviceName)
			continue
		}
		targetDefinitions[defName] = clonedDefinition
	}
}

// selectDocumentSourcePaths 选出单个 source 对应的 path/method 子集
func (s *SwaggerMiddleware) selectDocumentSourcePaths(serviceName string, serviceSpec map[string]interface{}, source *goswagger.DocumentSource) (map[string]map[string]interface{}, map[string]struct{}) {
	selected := make(map[string]map[string]interface{})

	paths, ok := serviceSpec[constants.SwaggerFieldPaths].(map[string]interface{})
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
			if cloned := s.cloneSelectedPathItem(pathItem, nil); cloned != nil {
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
				global.LOGGER.Warn("独立文档 include 未匹配到路径: service=%s, path=%s", serviceName, selector.Path)
				continue
			}

			pathItem, ok := pathValue.(map[string]interface{})
			if !ok {
				continue
			}

			cloned := s.cloneSelectedPathItem(pathItem, selector.Methods)
			if cloned == nil {
				global.LOGGER.Warn("独立文档 include 未匹配到方法: service=%s, path=%s, methods=%v", serviceName, selector.Path, selector.Methods)
				continue
			}

			s.mergeDocumentPathItemSelection(selected, selector.Path, cloned)
		}
	}

	for _, selector := range source.Exclude {
		if selector == nil || selector.Path == "" {
			continue
		}
		s.removeDocumentPathSelection(selected, selector)
	}

	return selected, s.collectOperationTagNames(selected)
}

// cloneSelectedPathItem 复制 path item 并只保留指定 method
func (s *SwaggerMiddleware) cloneSelectedPathItem(pathItem map[string]interface{}, methods []string) map[string]interface{} {
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
func (s *SwaggerMiddleware) mergeDocumentPathItem(target map[string]interface{}, pathName string, newPathItem map[string]interface{}, serviceName string) {
	if existingValue, exists := target[pathName]; exists {
		existingPathItem, ok := existingValue.(map[string]interface{})
		if !ok {
			target[pathName] = newPathItem
			return
		}

		for key, value := range newPathItem {
			if isSwaggerOperationMethod(key) {
				if _, methodExists := existingPathItem[key]; methodExists {
					global.LOGGER.Warn("⚠️  独立文档路径 %s 的方法 %s 重复定义 (来自: %s)，保留首次加载的定义", pathName, key, serviceName)
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
func (s *SwaggerMiddleware) mergeDocumentPathItemSelection(target map[string]map[string]interface{}, pathName string, newPathItem map[string]interface{}) {
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
func (s *SwaggerMiddleware) removeDocumentPathSelection(selected map[string]map[string]interface{}, selector *goswagger.DocumentPathSelector) {
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

// mergeDocumentDefinitions 递归合并独立文档所需 definitions
func (s *SwaggerMiddleware) mergeDocumentDefinitions(targetDefinitions map[string]interface{}, serviceSpec map[string]interface{}, selectedPaths map[string]map[string]interface{}, serviceName string) {
	sourceDefinitions, ok := serviceSpec[constants.SwaggerFieldDefs].(map[string]interface{})
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
			global.LOGGER.Warn("独立文档引用的 definition 不存在: service=%s, definition=%s", serviceName, definitionName)
			continue
		}

		clonedDefinition := mathx.ConvertMapKeysToString(definition)
		if existingDefinition, exists := targetDefinitions[definitionName]; exists {
			s.checkDefinitionConsistency(definitionName, existingDefinition, clonedDefinition, serviceName)
		} else {
			targetDefinitions[definitionName] = clonedDefinition
		}

		enqueueDefinitionRefs(clonedDefinition, queued, &queue)
	}
}

// mergeDocumentTags 合并独立文档需要的 tags
func (s *SwaggerMiddleware) mergeDocumentTags(target *[]interface{}, serviceSpec map[string]interface{}, selectedTagNames map[string]struct{}, tagNames map[string]bool) {
	if len(selectedTagNames) == 0 {
		return
	}

	foundTags := make(map[string]bool, len(selectedTagNames))
	if sourceTags, ok := serviceSpec[constants.SwaggerFieldTags].([]interface{}); ok {
		for _, tag := range sourceTags {
			tagMap, ok := tag.(map[string]interface{})
			if !ok {
				continue
			}

			tagName := convert.MustString(tagMap[constants.SwaggerFieldName])
			if tagName == "" {
				continue
			}

			if _, selected := selectedTagNames[tagName]; !selected {
				continue
			}

			foundTags[tagName] = true
			s.addUniqueTag(tagName, mathx.ConvertMapKeysToString(tagMap), target, tagNames)
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
		s.addUniqueTag(tagName, map[string]interface{}{constants.SwaggerFieldName: tagName}, target, tagNames)
	}
}

// collectOperationTagNames 收集已选 operation 使用到的 tag
func (s *SwaggerMiddleware) collectOperationTagNames(selectedPaths map[string]map[string]interface{}) map[string]struct{} {
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

			for _, tagName := range toStringSlice(operationMap[constants.SwaggerFieldTags]) {
				if tagName != "" {
					tagNames[tagName] = struct{}{}
				}
			}
		}
	}

	return tagNames
}

// GetDocumentSpec 获取独立文档规范
func (s *SwaggerMiddleware) GetDocumentSpec(documentName string) ([]byte, error) {
	if !s.config.IsAggregateEnabled() {
		return nil, fmt.Errorf("聚合模式未启用")
	}

	spec, exists := s.findNamedSpec(documentName, "文档", s.documentSpecs)
	if !exists {
		return nil, s.namedSpecNotFoundError("文档", documentName, s.documentSpecs)
	}

	return s.serializeServiceSpec(spec)
}

// collectDocumentSummaries 收集文档列表页所需摘要
func (s *SwaggerMiddleware) collectDocumentSummaries() []documentSummary {
	summaries := make([]documentSummary, 0)

	if s.config.Aggregate == nil {
		return summaries
	}

	for _, document := range s.config.Aggregate.Documents {
		if document == nil || !document.Enabled {
			continue
		}

		if _, exists := s.documentSpecs[document.Name]; !exists {
			continue
		}

		title := mathx.IfNotEmpty(strings.TrimSpace(document.Title), document.Name)
		description := mathx.IfNotEmpty(strings.TrimSpace(document.Description), defaultDocumentDescription)
		version := mathx.IfNotEmpty(strings.TrimSpace(document.Version), strings.TrimSpace(s.config.Version))

		summaries = append(summaries, documentSummary{
			Name:        document.Name,
			Title:       title,
			Description: description,
			Version:     version,
			Services:    s.collectDocumentServices(document),
		})
	}

	return summaries
}

// collectDocumentServices 收集独立文档引用的服务名
func (s *SwaggerMiddleware) collectDocumentServices(document *goswagger.DocumentSpec) []string {
	seen := make(map[string]bool)
	services := make([]string, 0, len(document.Sources))

	for _, source := range document.Sources {
		if source == nil {
			continue
		}

		serviceName := strings.TrimSpace(source.Service)
		if serviceName == "" || seen[serviceName] {
			continue
		}

		seen[serviceName] = true
		services = append(services, serviceName)
	}

	return services
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
	refValue, ok := value[constants.SwaggerFieldRef].(string)
	if !ok || !strings.HasPrefix(refValue, constants.SwaggerPathDefinitions) {
		return ""
	}

	return strings.TrimPrefix(refValue, constants.SwaggerPathDefinitions)
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

// toStringSlice 将任意字符串数组类型转换为 []string
func toStringSlice(value interface{}) []string {
	switch actual := value.(type) {
	case nil:
		return nil
	case []string:
		return mathx.FilterSliceByFunc(actual, func(item string) bool {
			return item != ""
		})
	case []interface{}:
		result := make([]string, 0, len(actual))
		for _, item := range actual {
			result = append(result, convert.MustString(item))
		}
		return mathx.FilterSliceByFunc(result, func(item string) bool {
			return item != ""
		})
	case string:
		if actual == "" {
			return nil
		}
		return []string{actual}
	default:
		return nil
	}
}
