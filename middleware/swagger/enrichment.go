/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\enrichment.go
 * @Description: UI 规范增强 - 面向内置 Swagger UI 的展示层增强（联调元数据、参数速览、示例值）
 *
 * 设计原则：原始 spec 永远不被修改（前端联调平台直接消费 x- 扩展）；
 * 仅当请求带 view=ui 查询参数时，服务端把 x-integration-* 联调元数据、
 * 参数 x-example、body 参数 example 等信息以 Markdown 形式追加到
 * operation 的 description 中，由 Swagger UI 原生渲染，无需前端 JS。
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
)

// integrationLabels 联调元数据扩展字段的展示名称
var integrationLabels = map[string]string{
	"x-integration-page":     "联调页面",
	"x-integration-status":   "联调状态",
	"x-integration-scenario": "使用场景",
	"x-integration-owner":    "负责人",
	"x-integration-module":   "所属模块",
}

// httpMethods Swagger Path Item 中的合法 HTTP 方法
var httpMethods = map[string]struct{}{
	"get": {}, "put": {}, "post": {}, "delete": {},
	"options": {}, "head": {}, "patch": {},
}

// isUIView 判断请求是否来自内置 Swagger UI（specURL 带 view=ui 查询参数）
func isUIView(r *http.Request) bool {
	return r != nil && r.URL.Query().Get("view") == "ui"
}

// serveSpecJSON 输出规范 JSON；UI 视图（view=ui）时输出面向展示的增强版本
func (m *Middleware) serveSpecJSON(w http.ResponseWriter, r *http.Request, jsonData []byte) {
	if isUIView(r) {
		enriched, err := enrichSwaggerJSONForUI(jsonData)
		if err != nil {
			m.logger.Warn("生成 UI 增强规范失败，回退原始规范: %v", err)
		} else {
			jsonData = enriched
		}
	}
	w.Write(jsonData)
}

// enrichSwaggerJSONForUI 将规范 JSON 转换为面向 UI 展示的增强版本
func enrichSwaggerJSONForUI(data []byte) ([]byte, error) {
	var spec map[string]interface{}
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("解析规范失败: %w", err)
	}
	enrichSpecForUI(spec)
	return json.MarshalIndent(spec, "", "  ")
}

// enrichSpecForUI 就地增强规范中每个操作的展示信息
func enrichSpecForUI(spec map[string]interface{}) {
	paths, ok := spec[constants.FieldPaths].(map[string]interface{})
	if !ok {
		return
	}

	for _, pathItem := range paths {
		pathMap, ok := pathItem.(map[string]interface{})
		if !ok {
			continue
		}
		for method, operation := range pathMap {
			if !isHTTPMethod(method) {
				continue
			}
			opMap, ok := operation.(map[string]interface{})
			if !ok {
				continue
			}
			section := buildOperationUISection(opMap)
			if section == "" {
				continue
			}
			appendDescription(opMap, section)
		}
	}
}

// buildOperationUISection 构建单个操作的 UI 增强内容（Markdown）
func buildOperationUISection(opMap map[string]interface{}) string {
	sections := make([]string, 0, 4)

	if table := buildIntegrationTable(opMap); table != "" {
		sections = append(sections, table)
	}
	if table := buildParamOverviewTable(opMap); table != "" {
		sections = append(sections, table)
	}
	if body := buildBodyParamSection(opMap); body != "" {
		sections = append(sections, body)
	}
	if examples := buildScenarioExamples(opMap); examples != "" {
		sections = append(sections, examples)
	}

	return strings.Join(sections, "\n\n")
}

// buildIntegrationTable 渲染 x-integration-* 联调元数据表格
func buildIntegrationTable(opMap map[string]interface{}) string {
	keys := make([]string, 0, len(integrationLabels))
	for key := range opMap {
		if _, ok := integrationLabels[key]; ok {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("**联调信息**\n\n")
	b.WriteString("| 项目 | 值 |\n| --- | --- |\n")
	for _, key := range keys {
		b.WriteString("| " + integrationLabels[key] + " | " + markdownCell(mustString(opMap[key])) + " |\n")
	}
	return b.String()
}

// buildParamOverviewTable 渲染非 body 参数的速览表格（query/path/header）
func buildParamOverviewTable(opMap map[string]interface{}) string {
	rows := make([]string, 0)
	for _, p := range paramList(opMap) {
		if mustString(p["in"]) == "body" {
			continue
		}
		name := mustString(p[constants.FieldName])
		if name == "" {
			continue
		}
		row := fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			markdownCell(name),
			markdownCell(mustString(p["in"])),
			markdownCell(paramType(p)),
			requiredLabel(p),
			markdownCell(paramDescription(p)),
		)
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("**参数速览**\n\n")
	b.WriteString("| 参数 | 位置 | 类型 | 必填 | 说明 |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, row := range rows {
		b.WriteString(row)
	}
	return b.String()
}

// buildBodyParamSection 渲染 body 参数：字段表格 + 示例值
func buildBodyParamSection(opMap map[string]interface{}) string {
	var sections []string

	for _, p := range paramList(opMap) {
		if mustString(p["in"]) != "body" {
			continue
		}

		var b strings.Builder
		title := "**body**"
		if required, _ := p["required"].(bool); required {
			title += "（必填）"
		}
		b.WriteString(title + "\n\n")

		schema, _ := p["schema"].(map[string]interface{})
		if table := buildBodyFieldTable(schema); table != "" {
			b.WriteString(table)
		}

		if example := formatExampleJSON(p["example"]); example != "" {
			b.WriteString("\n\n示例值：\n\n" + example)
		}

		sections = append(sections, strings.TrimRight(b.String(), "\n"))
	}

	return strings.Join(sections, "\n\n")
}

// buildBodyFieldTable 渲染 body schema 顶层字段表格（字段/类型/必填/说明）
func buildBodyFieldTable(schema map[string]interface{}) string {
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok || len(properties) == 0 {
		return ""
	}

	requiredSet := toStringSet(schema["required"])

	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("| 字段 | 类型 | 必填 | 说明 |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, name := range names {
		prop, ok := properties[name].(map[string]interface{})
		if !ok {
			continue
		}
		required := "否"
		if requiredSet[name] {
			required = "是"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			markdownCell(name),
			markdownCell(schemaType(prop)),
			required,
			markdownCell(mustString(prop[constants.FieldDescription])),
		))
	}
	return b.String()
}

// buildScenarioExamples 渲染操作级 x-examples 命名场景示例
func buildScenarioExamples(opMap map[string]interface{}) string {
	examples, ok := opMap["x-examples"].(map[string]interface{})
	if !ok || len(examples) == 0 {
		return ""
	}

	names := make([]string, 0, len(examples))
	for name := range examples {
		names = append(names, name)
	}
	sort.Strings(names)

	sections := make([]string, 0, len(names))
	for _, name := range names {
		if formatted := formatExampleJSON(examples[name]); formatted != "" {
			sections = append(sections, fmt.Sprintf("**场景示例 - %s**\n\n%s", name, formatted))
		}
	}
	return strings.Join(sections, "\n\n")
}

// appendDescription 把增强内容追加到操作描述（原描述保留在前）
func appendDescription(opMap map[string]interface{}, section string) {
	existing := strings.TrimSpace(mustString(opMap[constants.FieldDescription]))
	if existing == "" {
		opMap[constants.FieldDescription] = section
		return
	}
	opMap[constants.FieldDescription] = existing + "\n\n---\n\n" + section
}

// paramList 提取操作参数列表中的 map 形式参数
func paramList(opMap map[string]interface{}) []map[string]interface{} {
	params, _ := opMap[constants.FieldParameters].([]interface{})
	result := make([]map[string]interface{}, 0, len(params))
	for _, p := range params {
		if param, ok := p.(map[string]interface{}); ok {
			result = append(result, param)
		}
	}
	return result
}

// paramDescription 拼接参数说明与 x-example 示例
func paramDescription(param map[string]interface{}) string {
	desc := mustString(param[constants.FieldDescription])
	if example, exists := param["x-example"]; exists && example != nil {
		exampleStr := mustString(example)
		if exampleStr != "" {
			if desc != "" {
				desc += "。"
			}
			desc += "示例：" + exampleStr
		}
	}
	return desc
}

// paramType 推断参数类型展示值
func paramType(param map[string]interface{}) string {
	if typ := mustString(param["type"]); typ != "" {
		return typ
	}
	if _, ok := param["schema"]; ok {
		return "object"
	}
	return "string"
}

// schemaType 推断 schema 字段类型展示值
func schemaType(prop map[string]interface{}) string {
	typ := mustString(prop["type"])
	if typ == "" {
		typ = "string"
	}
	if typ == "array" {
		if items, ok := prop["items"].(map[string]interface{}); ok {
			if itemType := mustString(items["type"]); itemType != "" {
				return "array<" + itemType + ">"
			}
		}
	}
	return typ
}

// requiredLabel 返回参数必填标记
func requiredLabel(param map[string]interface{}) string {
	if required, _ := param["required"].(bool); required {
		return "是"
	}
	return "否"
}

// formatExampleJSON 将示例值序列化为 JSON 代码块，无法序列化时返回空
func formatExampleJSON(example interface{}) string {
	if example == nil {
		return ""
	}
	data, err := json.MarshalIndent(example, "", "  ")
	if err != nil {
		return ""
	}
	return "```json\n" + string(data) + "\n```"
}

// toStringSet 将字符串数组转为集合
func toStringSet(value interface{}) map[string]bool {
	items, _ := value.([]interface{})
	result := make(map[string]bool, len(items))
	for _, item := range items {
		result[mustString(item)] = true
	}
	return result
}

// markdownCell 转义 Markdown 表格单元格内容
func markdownCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	return strings.ReplaceAll(s, "\n", "<br>")
}

// mustString 将接口值安全转换为字符串
func mustString(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

// isHTTPMethod 判断字段是否为 HTTP 方法
func isHTTPMethod(method string) bool {
	_, ok := httpMethods[method]
	return ok
}
