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
 * operation 的 description 中，由 Swagger UI 原生渲染，无需前端 JS
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
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

// enumLinePattern 匹配 protoc-gen-openapiv2 追加在说明末尾的枚举清单行（" - ENUM_NAME: 含义"）
var enumLinePattern = regexp.MustCompile(`^-\s+[A-Za-z0-9_.]+\s*:`)

// maxEnumChips 可选值列表在速览表里最多列出的个数，超出部分折叠成省略号
const maxEnumChips = 12

// maxRefDepth $ref 展开的最大层数，防止自引用定义把展开转成死循环
const maxRefDepth = 8

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

	res := newRefResolver(spec)

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
			section := buildOperationUISection(opMap, res)
			if section == "" {
				continue
			}
			appendDescription(opMap, section)
		}
	}
}

// buildOperationUISection 构建单个操作的 UI 增强内容（Markdown）
func buildOperationUISection(opMap map[string]interface{}, res *refResolver) string {
	sections := make([]string, 0, 4)

	if table := buildIntegrationTable(opMap); table != "" {
		sections = append(sections, table)
	}
	if table := buildParamOverviewTable(opMap, res); table != "" {
		sections = append(sections, table)
	}
	if body := buildBodyParamSection(opMap, res); body != "" {
		sections = append(sections, body)
	}
	if examples := buildScenarioExamples(opMap); examples != "" {
		sections = append(sections, examples)
	}

	for i, section := range sections {
		sections[i] = strings.TrimRight(section, "\n")
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
		b.WriteString("| ")
		b.WriteString(integrationLabels[key])
		b.WriteString(" | ")
		b.WriteString(markdownCell(mustString(opMap[key])))
		b.WriteString(" |\n")
	}
	return b.String()
}

// buildParamOverviewTable 渲染非 body 参数的速览表格（query/path/header）
func buildParamOverviewTable(opMap map[string]interface{}, res *refResolver) string {
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
			markdownCode(name),
			markdownCell(mustString(p["in"])),
			schemaTypeText(p, res),
			requiredLabel(p),
			describeCell(mustString(p[constants.FieldDescription]), p, p["x-example"], res),
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
func buildBodyParamSection(opMap map[string]interface{}, res *refResolver) string {
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
		b.WriteString(title)
		b.WriteString("\n\n")

		schema, _ := p["schema"].(map[string]interface{})
		if table := buildBodyFieldTable(schema, res); table != "" {
			b.WriteString(table)
		}

		if example := formatExampleJSON(p["example"]); example != "" {
			b.WriteString("\n\n示例值：\n\n")
			b.WriteString(example)
		}

		sections = append(sections, strings.TrimRight(b.String(), "\n"))
	}

	return strings.Join(sections, "\n\n")
}

// buildBodyFieldTable 渲染 body schema 顶层字段表格（字段/类型/必填/说明）
func buildBodyFieldTable(schema map[string]interface{}, res *refResolver) string {
	resolved, _ := res.resolve(schema)
	properties, ok := resolved["properties"].(map[string]interface{})
	if !ok || len(properties) == 0 {
		return ""
	}

	requiredSet := toStringSet(resolved["required"])

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
			markdownCode(name),
			schemaTypeText(prop, res),
			required,
			describeCell(propertyDoc(prop), prop, nil, res),
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

// ==================== $ref 解析 ====================

// refResolver 按 definitions 展开 $ref
//
// body 参数几乎都写成 "schema": {"$ref": "#/definitions/XxxBody"}不展开的话
// buildBodyFieldTable 拿不到 properties，整个 body 区块就只剩一个孤零零的标题
// 枚举字段同理：底层类型和可选值都在被引用的定义里，不展开就只能显示成 object
//
// [EN] Body parameters are almost always a bare $ref, so without resolution the field table
// finds no properties and the body section degenerates into a lone heading. Enum fields are the
// same: both the underlying type and the allowed values live in the referenced definition, and
// an unresolved one can only be rendered as "object".
type refResolver struct {
	definitions map[string]interface{}
}

// newRefResolver 以规范的 definitions 建立解析器
func newRefResolver(spec map[string]interface{}) *refResolver {
	definitions, _ := spec[constants.FieldDefs].(map[string]interface{})
	return &refResolver{definitions: definitions}
}

// resolve 展开 schema 上的 $ref，返回被指向的定义与定义名；非引用原样返回
func (r *refResolver) resolve(schema map[string]interface{}) (map[string]interface{}, string) {
	name := ""
	if schema == nil {
		return nil, name
	}
	for depth := 0; depth < maxRefDepth; depth++ {
		ref := mustString(schema[constants.FieldRef])
		if ref == "" || r == nil || r.definitions == nil {
			break
		}
		next := refName(ref)
		target, ok := r.definitions[next].(map[string]interface{})
		if !ok {
			break
		}
		name, schema = next, target
	}
	return schema, name
}

// refName 取 $ref 的末段定义名
func refName(ref string) string {
	if idx := strings.LastIndex(ref, "/"); idx >= 0 {
		return ref[idx+1:]
	}
	return ref
}

// ==================== 单元格内容 ====================

// describeCell 组装「说明」列：首段说明 + 可选值 + 默认值 + 限制 + 示例
//
// 为什么只取首段：proto 注释里常跟着整段设计背景（为什么要带 version、改了类型会
// 怎样），几百字塞进一个单元格会把表格撑成一面墙速览表只负责「一眼看清」，
// 完整说明 Swagger UI 自己的 Parameters / Model 区块就在下面，一字不少
//
// [EN] Only the first paragraph: proto comments often carry a full design rationale behind the
// label, and several hundred characters in one cell turn the table into a wall. This table is
// the quick glance; Swagger UI's own Parameters and Model blocks sit right below it with the
// complete text, so nothing is actually lost.
func describeCell(doc string, schema map[string]interface{}, example interface{}, res *refResolver) string {
	resolved, _ := res.resolve(schema)

	parts := make([]string, 0, 5)
	if lead := leadParagraph(doc); lead != "" {
		parts = append(parts, markdownCell(lead))
	}
	if values := enumValues(resolved); len(values) > 0 {
		parts = append(parts, "可选值："+joinCode(values))
	}
	if def := scalarString(firstNonNil(schema["default"], resolved["default"])); def != "" {
		parts = append(parts, "默认值："+markdownCode(def))
	}
	if limit := constraintText(schema, resolved); limit != "" {
		parts = append(parts, "限制："+limit)
	}
	if ex := scalarString(example); ex != "" {
		parts = append(parts, "示例："+markdownCode(ex))
	}
	return strings.Join(parts, "<br>")
}

// propertyDoc 取字段说明：title 是一行标签，description 往往是整段背景，速览优先用前者
func propertyDoc(prop map[string]interface{}) string {
	if title := mustString(prop[constants.FieldTitle]); strings.TrimSpace(title) != "" {
		return title
	}
	return mustString(prop[constants.FieldDescription])
}

// leadParagraph 取说明的首个自然段，并剔除 protoc-gen-openapiv2 追加的枚举清单行
//
// 枚举清单（" - ENTITLEMENT_GROUP_DATA: 数据"）会被单独渲染成「可选值」一行，
// 留在正文里只会让同一份内容出现两次
//
// [EN] The enum listing protoc-gen-openapiv2 appends is rendered separately as the allowed-values
// line, so leaving it in the prose would print the same content twice.
func leadParagraph(doc string) string {
	for _, block := range strings.Split(strings.TrimSpace(doc), "\n\n") {
		lines := make([]string, 0, 4)
		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || enumLinePattern.MatchString(line) {
				continue
			}
			lines = append(lines, line)
		}
		if len(lines) > 0 {
			return strings.Join(lines, "\n")
		}
	}
	return ""
}

// enumValues 取枚举可选值，超过 maxEnumChips 个折叠尾部
func enumValues(schema map[string]interface{}) []string {
	items, _ := schema["enum"].([]interface{})
	values := make([]string, 0, len(items))
	for _, item := range items {
		if value := scalarString(item); value != "" {
			values = append(values, value)
		}
	}
	if len(values) > maxEnumChips {
		return append(values[:maxEnumChips:maxEnumChips], fmt.Sprintf("…共 %d 项", len(values)))
	}
	return values
}

// constraintText 汇总长度、取值范围、元素个数与格式约束
func constraintText(schema, resolved map[string]interface{}) string {
	pick := func(key string) interface{} { return firstNonNil(schema[key], resolved[key]) }

	items := make([]string, 0, 4)
	if pattern := mustString(pick("pattern")); pattern != "" {
		items = append(items, "格式 "+markdownCode(pattern))
	}
	if text := rangeText(pick("minLength"), pick("maxLength")); text != "" {
		items = append(items, "长度 "+text)
	}
	if text := rangeText(pick("minimum"), pick("maximum")); text != "" {
		items = append(items, "取值 "+text)
	}
	if text := rangeText(pick("minItems"), pick("maxItems")); text != "" {
		items = append(items, "元素数 "+text)
	}
	return strings.Join(items, "；")
}

// rangeText 把上下界渲染成区间文本，单边缺失时退化为 ≥ / ≤
func rangeText(min, max interface{}) string {
	low, high := scalarString(min), scalarString(max)
	switch {
	case low != "" && high != "" && low == high:
		return markdownCode(low)
	case low != "" && high != "":
		return markdownCode(low + "~" + high)
	case low != "":
		return "≥ " + markdownCode(low)
	case high != "":
		return "≤ " + markdownCode(high)
	}
	return ""
}

// schemaTypeText 推断类型展示值，展开 $ref 后取底层类型，对象则显示定义名
func schemaTypeText(schema map[string]interface{}, res *refResolver) string {
	if mustString(schema[constants.FieldRef]) != "" {
		resolved, name := res.resolve(schema)
		if typ := mustString(resolved["type"]); typ != "" && typ != "object" {
			return escapeHTML(typ)
		}
		if name != "" {
			return markdownCode(name)
		}
		return "object"
	}

	typ := mustString(schema["type"])
	if typ == "" {
		inner, ok := schema["schema"].(map[string]interface{})
		if !ok {
			return "string"
		}
		return schemaTypeText(inner, res)
	}

	if typ == "array" {
		if items, ok := schema["items"].(map[string]interface{}); ok {
			// 尖括号必须转义：description 走的是 html:true 的渲染器，
			// 原样的 <string> 会被当成未知标签整段吞掉，只剩下 "array"
			// [EN] The angle brackets must be escaped: descriptions go through the renderer with
			// html:true, where a literal <string> is taken for an unknown tag and swallowed whole,
			// leaving a bare "array" behind.
			return "array&lt;" + stripCode(schemaTypeText(items, res)) + "&gt;"
		}
	}
	return escapeHTML(typ)
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

// ==================== Markdown / HTML 转义 ====================

// markdownCell 转义 Markdown 表格单元格内容
func markdownCell(s string) string {
	// 竖线用 HTML 实体而不是反斜杠转义
	//
	// `\|` 这种转义只在部分 markdown 实现里生效，Swagger UI 用的渲染器不认：
	// 竖线照样把单元格切开，反斜杠还原样显示出来后果在双语注释的项目里尤其明显——
	// 这些 proto 注释统一写成「中文 | [EN] English」，于是每一条说明都带竖线，
	// 整张表会错位成一行横跨五列、中间散着 `\` 的样子
	// &#124; 是实体引用，渲染器在解析表格结构之前就不会把它当分隔符
	//
	// [EN] Pipes become an HTML entity rather than a backslash escape, which only some markdown
	// implementations honour and the one Swagger UI ships does not: the pipe still splits the
	// cell and the backslash shows up verbatim. It matters here because these proto comments are
	// written as "中文 | [EN] English", so each description carries pipes and the whole table
	// collapses into one row spanning five columns with stray backslashes in it.
	s = strings.ReplaceAll(escapeHTML(s), "|", "&#124;")
	return strings.ReplaceAll(s, "\n", "<br>")
}

// markdownCode 把名称、枚举值、正则等渲染成行内代码（等宽 + 主题色），让它们在整行说明里一眼可辨
//
// 用 <code> 标签而不是反引号：反引号里的内容不参与实体解析，`a|b` 这样的正则
// 写成 `a&#124;b` 会把实体原样显示出来，写成 `a|b` 又会把单元格切开——两条路都不通
// 标签形式下实体由浏览器解析，竖线既不切表格也能正常显示
//
// [EN] A <code> tag rather than backticks: a code span does not resolve entities, so a pattern
// like a|b renders the literal text a&#124;b when escaped and splits the cell when it is not —
// neither way works. Inside a tag the browser resolves the entity, so the pipe neither breaks
// the table nor disappears.
func markdownCode(s string) string {
	if s == "" {
		return ""
	}
	return "<code>" + strings.ReplaceAll(escapeHTML(s), "|", "&#124;") + "</code>"
}

// joinCode 把一组取值渲染成顿号分隔的行内代码
func joinCode(values []string) string {
	chips := make([]string, 0, len(values))
	for _, value := range values {
		chips = append(chips, markdownCode(value))
	}
	return strings.Join(chips, "、")
}

// stripCode 去掉 markdownCode 包裹的标签，用于嵌进 array<...> 这类复合类型
func stripCode(s string) string {
	s = strings.TrimPrefix(s, "<code>")
	return strings.TrimSuffix(s, "</code>")
}

// escapeHTML 转义 HTML 特殊字符（描述走 html:true 渲染，未转义的尖括号会被当成标签丢掉）
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	return strings.ReplaceAll(s, ">", "&gt;")
}

// ==================== 取值辅助 ====================

// firstNonNil 返回第一个非 nil 的取值
func firstNonNil(values ...interface{}) interface{} {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

// scalarString 将标量取值转成展示字符串，整数不带小数尾巴
func scalarString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case json.Number:
		return v.String()
	case float64:
		if v == math.Trunc(v) && math.Abs(v) < 1e15 {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	}
	return ""
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
