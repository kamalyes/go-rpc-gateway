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

// modelAnchorPrefix Models 区块里每个定义的 DOM id 前缀
//
// Swagger UI 给每个 definition 渲染 <div id="model-Xxx">，所以 #model-Xxx 是现成的锚点，
// 不需要我们另造一套编号页面里的 JS 只是在此之上补「展开 Models 区块 + 展开这一条 +
// 高亮落点」，JS 不生效时浏览器仍会按原生锚点滚过去
//
// [EN] Swagger UI renders every definition as <div id="model-Xxx">, so #model-Xxx is an anchor
// that already exists and needs no numbering scheme of our own. The page script only adds
// "expand the Models section, expand this entry, highlight where we landed" on top of it; with
// the script disabled the browser still scrolls there through the plain anchor.
const modelAnchorPrefix = "model-"

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
	params := paramList(opMap)
	// 改名表按同一个操作的全部参数建：条件必填规则引用的是同级字段
	// [EN] The rename table covers all parameters of the operation: conditional required rules
	// reference sibling fields.
	namer := newFieldNamer(paramNames(params))

	rows := make([]string, 0)
	for _, p := range params {
		if mustString(p["in"]) == "body" {
			continue
		}
		name := mustString(p[constants.FieldName])
		if name == "" {
			continue
		}
		doc, rules := splitValidateDoc(mustString(p[constants.FieldDescription]))
		tag := argusTag{rules: rules, namer: namer}
		required, _ := p["required"].(bool)
		row := fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			markdownCode(name),
			markdownCell(mustString(p["in"])),
			schemaTypeText(p, res),
			requiredCell(required, tag),
			describeCell(doc, p, p["x-example"], res, tag),
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
		schema, _ := p["schema"].(map[string]interface{})

		// 标题带上 body 自己的定义名并做成锚点：顶层结构体同样是「想点进去看字段」的那一个
		//
		// 链接必须写在 ** ** 之内：挪到外面，strong 就多了个兄弟元素，
		// 描述区 `p > strong:only-child` 的色条规则不再命中，标题会矮下去一截
		//
		// [EN] The body's own definition name goes into the heading as an anchor: the top-level
		// struct is just as much a "click through to the fields" target as the nested ones.
		//
		// The link has to sit inside the ** ** pair. Outside it the strong gains an element
		// sibling, the description area's `p > strong:only-child` accent-bar rule stops matching,
		// and the heading visibly flattens.
		title := "**body"
		if _, name := res.resolve(schema); name != "" {
			title += " " + modelLink(name)
		}
		title += "**"
		if required, _ := p["required"].(bool); required {
			title += "（必填）"
		}
		b.WriteString(title)
		b.WriteString("\n\n")

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

	namer := newFieldNamer(names)

	var b strings.Builder
	b.WriteString("| 字段 | 类型 | 必填 | 说明 |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, name := range names {
		prop, ok := properties[name].(map[string]interface{})
		if !ok {
			continue
		}
		doc, rules := splitValidateDoc(propertyDoc(prop))
		tag := argusTag{rules: rules, namer: namer}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			markdownCode(name),
			schemaTypeText(prop, res),
			requiredCell(requiredSet[name], tag),
			describeCell(doc, prop, nil, res, tag),
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

// paramNames 取参数名列表，用于建立 argus 规则引用字段的改名表
func paramNames(params []map[string]interface{}) []string {
	names := make([]string, 0, len(params))
	for _, p := range params {
		if name := mustString(p[constants.FieldName]); name != "" {
			names = append(names, name)
		}
	}
	return names
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
func describeCell(doc string, schema map[string]interface{}, example interface{}, res *refResolver, tag argusTag) string {
	resolved, _ := res.resolve(schema)

	parts := make([]string, 0, 6)
	if lead := leadParagraph(doc); lead != "" {
		parts = append(parts, markdownCell(lead))
	}
	// 条件必填单独成行：「必填」列只容得下「条件」两个字，容不下条件本身
	// [EN] The condition gets its own line: the 必填 column has room for the word 条件, not for
	// the condition itself.
	for _, condition := range requiredConditions(tag) {
		parts = append(parts, "必填条件："+condition)
	}
	values := enumValues(resolved)
	if len(values) == 0 {
		// 没有 OpenAPI enum 时看 argus 的 oneof：string 字段的可选值常常只写在标签里
		// [EN] With no OpenAPI enum, fall back to argus oneof: for string fields the allowed
		// values often live only in the tag.
		values = foldEnumChips(argusEnumValues(tag))
	}
	if len(values) > 0 {
		parts = append(parts, "可选值："+joinCode(values))
	}
	if def := scalarString(firstNonNil(schema["default"], resolved["default"])); def != "" {
		parts = append(parts, "默认值："+markdownCode(def))
	}
	if limit := constraintText(schema, resolved, tag, res); limit != "" {
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
	return foldEnumChips(values)
}

// foldEnumChips 超过 maxEnumChips 个就折叠尾部
func foldEnumChips(values []string) []string {
	if len(values) > maxEnumChips {
		return append(values[:maxEnumChips:maxEnumChips], fmt.Sprintf("…共 %d 项", len(values)))
	}
	return values
}

// constraintText 汇总「限制」列：argus 标签优先，没有标签时才回落到 OpenAPI 原生约束
//
// 为什么 argus 优先：它是请求真正会被卡住的那一道（见 middleware/i18n/struct_tag_validator.go），
// OpenAPI 的 minimum/maxLength 只是文档声明，而 proto 产物里基本是空的
// 两者都存在且对不上时，按运行时的那一份写，否则文档会把人骗进一个必定 400 的请求
//
// [EN] argus wins because it is the gate a request actually fails at (see
// middleware/i18n/struct_tag_validator.go), while OpenAPI's minimum/maxLength is a declaration
// that proto output rarely carries at all. When both exist and disagree, the document follows
// the one that runs — otherwise it talks the reader into a request that is certain to 400.
func constraintText(schema, resolved map[string]interface{}, tag argusTag, res *refResolver) string {
	if items := argusConstraints(tag, compareScope(schema, res)); len(items) > 0 {
		return strings.Join(items, "；")
	}
	return openAPIConstraintText(schema, resolved)
}

// openAPIConstraintText 汇总 OpenAPI 原生的长度、取值范围、元素个数与格式约束
func openAPIConstraintText(schema, resolved map[string]interface{}) string {
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

// schemaTypeText 推断类型展示值，展开 $ref 后取底层类型，对象则渲染成可跳转 Models 区块的定义名
//
// 为什么标量定义不给链接：枚举这类被引用的定义展开后就是 string/integer，底层类型已是它的
// 全部信息，可选值也由 describeCell 单独列出，再挂个链接只是让人多点一次
//
// [EN] Scalar definitions get no link: an enum-style definition resolves to string or integer,
// which is all the information it carries, and its allowed values are already listed separately
// by describeCell. A link there would only cost the reader one more click.
func schemaTypeText(schema map[string]interface{}, res *refResolver) string {
	if mustString(schema[constants.FieldRef]) != "" {
		resolved, name := res.resolve(schema)
		if typ := mustString(resolved["type"]); typ != "" && typ != "object" {
			return escapeHTML(typ)
		}
		if name != "" {
			return modelLink(name)
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

	switch typ {
	case "array":
		if items, ok := schema["items"].(map[string]interface{}); ok {
			// 尖括号必须转义：description 走的是 html:true 的渲染器，
			// 原样的 <string> 会被当成未知标签整段吞掉，只剩下 "array"
			// [EN] The angle brackets must be escaped: descriptions go through the renderer with
			// html:true, where a literal <string> is taken for an unknown tag and swallowed whole,
			// leaving a bare "array" behind.
			return "array&lt;" + schemaTypeText(items, res) + "&gt;"
		}
	case "object":
		// protobuf 的 map 字段落在 additionalProperties 上，不看这一层只能显示成 object，
		// 值是结构体时连个可点的入口都没有（additionalProperties 为 true 时断言失败，照常显示 object）
		//
		// [EN] protobuf map fields land on additionalProperties. Without this branch they can
		// only render as "object", and a struct-valued map offers nothing to click through. A
		// bare `additionalProperties: true` fails the assertion and still renders as "object".
		if additional, ok := schema["additionalProperties"].(map[string]interface{}); ok {
			return "map&lt;string, " + schemaTypeText(additional, res) + "&gt;"
		}
	}
	return escapeHTML(typ)
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
	return strings.ReplaceAll(escapeInline(s), "\n", "<br>")
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
	return "<code>" + escapeInline(s) + "</code>"
}

// modelLink 把定义名渲染成指向 Models 区块的锚点，点一下就跳到该结构体的字段列表
//
// 为什么只留 href：Swagger UI 的描述要过一遍 DOMPurify，默认配置的 FORBID_ATTR 里有
// class 和 style，写上去也会被摘掉所以配色只能挂在 a[href^="#model-"] 这个属性选择器上，
// 页面 JS 也只能按 href 前缀认出这些链接
//
// 为什么不再套 <code>：这个链接要嵌进 array<...>、map<string, ...>，外面已经有一层尖括号，
// 再叠一层代码块底色会让一格类型压三层背景，等宽与配色改由上面那个选择器给
//
// [EN] Only href survives: Swagger UI runs descriptions through DOMPurify, whose default config
// lists class and style in FORBID_ATTR, so both are stripped. That is why the colours hang off
// an `a[href^="#model-"]` attribute selector and why the page script can only recognise these
// links by their href prefix.
//
// No <code> wrapper either: the link nests inside array<...> and map<string, ...>, which already
// contribute a layer of brackets, and a code chip would stack a third background into one type
// cell. The monospace face and colour come from that same selector instead.
func modelLink(name string) string {
	text := escapeInline(name)
	// 定义名直接进 href 属性：规范可能来自远端服务，引号要一并实体化，
	// 不能让一个定义名提前把属性闭合
	// [EN] The name goes straight into the href, and specs can come from a remote service, so
	// quotes are escaped too: a definition name must not be able to close the attribute early.
	return `<a href="#` + modelAnchorPrefix + strings.ReplaceAll(text, `"`, "&quot;") + `">` + text + `</a>`
}

// joinCode 把一组取值渲染成顿号分隔的行内代码
func joinCode(values []string) string {
	chips := make([]string, 0, len(values))
	for _, value := range values {
		chips = append(chips, markdownCode(value))
	}
	return strings.Join(chips, "、")
}

// escapeHTML 转义 HTML 特殊字符（描述走 html:true 渲染，未转义的尖括号会被当成标签丢掉）
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	return strings.ReplaceAll(s, ">", "&gt;")
}

// escapeInline 转义进表格的行内内容：HTML 特殊字符实体化，竖线也走实体，免得把单元格切开
func escapeInline(s string) string {
	return strings.ReplaceAll(escapeHTML(s), "|", "&#124;")
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
