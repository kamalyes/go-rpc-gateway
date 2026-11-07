/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\argus.go
 * @Description: argus 校验标签对标 - 把 proto 注释里的 validate 标签翻译成速览表的「必填」与「限制」
 *
 * 设计原则：表格里写的规则必须和运行时真正执行的规则是同一份
 * 网关的参数校验由 go-toolbox/pkg/argus 执行（见 middleware/i18n/struct_tag_validator.go），
 * 规则写在 proto 注释的 `@inject_tag: validate:"..."` 里，由 protoc-go-inject-tag 注入到
 * pb 生成代码的结构体标签上protoc-gen-openapiv2 不认识这个标签：既不把它翻成 OpenAPI 的
 * minimum/maxLength，也不把它从注释里摘掉于是只读 OpenAPI 字段的表格有三处对不上：
 *
 *  1. 「必填」列只看 OpenAPI 的 required 列表，而 proto3 字段几乎从不出现在那里，
 *     于是带 validate:"required" 的字段一律显示成「否」
 *  2. 「限制」列只读 minLength/maximum 这些，proto 产物里基本是空的，
 *     真正在跑的 gte=1,lte=30 一个字都看不到
 *  3. @inject_tag 指令原文留在说明列里，把一行字段说明撑成半屏指令
 *
 * [EN] The rules printed in the table must be the same rules that run at request time.
 *
 * Parameter validation in this gateway is executed by go-toolbox/pkg/argus (see
 * middleware/i18n/struct_tag_validator.go). The rules are written in proto comments as
 * `@inject_tag: validate:"..."` and injected onto the generated struct tags by
 * protoc-go-inject-tag. protoc-gen-openapiv2 knows nothing about that tag: it neither
 * translates it into OpenAPI minimum/maxLength nor strips it from the comment. A table that
 * reads only OpenAPI fields therefore disagreed with the runtime in three places:
 *
 *  1. The 必填 column read only OpenAPI's required list, where proto3 fields almost never
 *     appear, so every validate:"required" field was documented as optional.
 *  2. The 限制 column read only minLength/maximum and friends, which proto output rarely
 *     carries, so the gte=1,lte=30 that actually runs was nowhere to be seen.
 *  3. The raw @inject_tag directive stayed in the description column, stretching a one-line
 *     field note into half a screen of directives.
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"regexp"
	"strings"
)

// injectTagDirective 匹配 protoc-go-inject-tag 的整条指令（含其后所有 key:"value" 标签）
//
// 一条指令可以带多个标签（validate 之外常见的还有 json、gorm），整条都要从说明里摘掉
// [EN] One directive may carry several tags — json and gorm alongside validate are common —
// and the whole directive has to come out of the description.
var injectTagDirective = regexp.MustCompile(`@inject_tag:\s*(?:[A-Za-z_][A-Za-z0-9_]*:"[^"]*"\s*)+`)

// validateTagValue 取 validate:"..." 里的规则串
var validateTagValue = regexp.MustCompile(`validate:"([^"]*)"`)

// validateRule 一条 argus 校验规则：名字与可选参数（gte=1 → {gte, 1}）
type validateRule struct {
	name  string
	param string
}

// argusTag 一个字段上解析出来的 argus 规则，以及把规则里引用的字段名换成文档字段名的改名表
type argusTag struct {
	rules []validateRule
	namer *fieldNamer
}

// has 判断是否带某条规则
func (t argusTag) has(name string) bool {
	for _, rule := range t.rules {
		if rule.name == name {
			return true
		}
	}
	return false
}

// find 取某条规则，不存在时第二个返回值为 false
func (t argusTag) find(name string) (validateRule, bool) {
	for _, rule := range t.rules {
		if rule.name == name {
			return rule, true
		}
	}
	return validateRule{}, false
}

// splitValidateDoc 把说明里的 inject_tag 指令摘出来，返回干净的说明与解析好的规则
//
// 裸写 validate:"..."（没有 @inject_tag: 前缀）的也认：有人手写 pb.go 或用别的注入工具，
// 指令同样不该出现在接口文档里
//
// [EN] A bare validate:"..." with no @inject_tag: prefix is recognised too: some fields are
// hand-written or injected by another tool, and the directive has no place in API docs either way.
func splitValidateDoc(doc string) (string, []validateRule) {
	directive := injectTagDirective.FindString(doc)
	if directive == "" {
		directive = validateTagValue.FindString(doc)
	}
	if directive == "" {
		return doc, nil
	}

	text := strings.TrimSpace(strings.Replace(doc, directive, "", 1))
	match := validateTagValue.FindStringSubmatch(directive)
	if match == nil {
		return text, nil
	}
	return text, parseValidateTag(match[1])
}

// parseValidateTag 按逗号拆规则
//
// 反斜杠转义的逗号不算分隔符，且反斜杠本身留在参数里——这是 argus 自己的口径，
// 文档照抄它，免得两边对同一个标签解析出不同结果
//
// [EN] A backslash-escaped comma is not a separator and the backslash stays in the parameter.
// That is argus's own reading of the tag, and the docs copy it so the two cannot disagree about
// what a given tag means.
func parseValidateTag(tag string) []validateRule {
	items := splitEscaped(tag, ',')
	rules := make([]validateRule, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		name, param := item, ""
		if idx := strings.Index(item, "="); idx >= 0 {
			name, param = item[:idx], item[idx+1:]
		}
		rules = append(rules, validateRule{name: strings.TrimSpace(name), param: strings.TrimSpace(param)})
	}
	return rules
}

// splitEscaped 按 sep 切分，`\sep` 不算分隔符
func splitEscaped(s string, sep byte) []string {
	parts := make([]string, 0, 4)
	start, escaped := 0, false
	for i := 0; i < len(s); i++ {
		switch {
		case escaped:
			escaped = false
		case s[i] == '\\':
			escaped = true
		case s[i] == sep:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

// ==================== 引用字段改名 ====================

// fieldNamer 把规则里引用的 Go 字段名换成文档里的字段名
//
// argus 规则引用的是 Go 字段名（required_if=Enabled true），而表格列的是 JSON 名
// （enabled）直接把 Enabled 印出去，读文档的人得自己猜这是哪个参数
//
// [EN] argus rules reference Go field names (required_if=Enabled true) while the table lists
// JSON names (enabled). Printing Enabled as-is leaves the reader guessing which parameter it means.
type fieldNamer struct {
	byNormalized map[string]string
}

// newFieldNamer 以同级字段名建立改名表
func newFieldNamer(names []string) *fieldNamer {
	byNormalized := make(map[string]string, len(names))
	for _, name := range names {
		byNormalized[normalizeFieldName(name)] = name
	}
	return &fieldNamer{byNormalized: byNormalized}
}

// display 返回文档里该用的字段名；找不到同级字段时原样返回
func (n *fieldNamer) display(name string) string {
	if n == nil {
		return name
	}
	if found, ok := n.byNormalized[normalizeFieldName(name)]; ok {
		return found
	}
	return name
}

// normalizeFieldName 去掉大小写与下划线差异，让 AgentSeatLimit 能对上 agent_seat_limit
func normalizeFieldName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "_", ""))
}

// ==================== 必填 ====================

// requiredConditionRules 条件必填族规则 → 条件说明的句式
//
// 这些是 argus 相对 OpenAPI 的「特殊属性」：OpenAPI 的 required 只有「是」和「否」，
// 表达不了「另一个字段等于某值时才必填」所以「必填」列只写「条件」，
// 具体条件由 requiredCondition 渲染成一句人话放进说明列
//
// [EN] These are the argus rules OpenAPI has no room for: its `required` is a plain yes or no
// and cannot say "required only when another field holds a given value". The 必填 column
// therefore says just 条件 and requiredCondition spells the condition out in the description.
var requiredConditionRules = map[string]struct{}{
	"required_if":          {},
	"required_unless":      {},
	"required_with":        {},
	"required_with_all":    {},
	"required_without":     {},
	"required_without_all": {},
}

// requiredCell 渲染「必填」列：argus 标签优先，没有标签时才回落到 OpenAPI 的 required
//
// 为什么 argus 优先：它是请求真正会被卡住的那一道，OpenAPI 的 required 只是文档声明
// [EN] argus wins because it is the gate a request actually fails at; OpenAPI's required is
// only a declaration in the document.
func requiredCell(schemaRequired bool, tag argusTag) string {
	if tag.has("required") {
		return "是"
	}
	for _, rule := range tag.rules {
		if _, ok := requiredConditionRules[rule.name]; ok {
			return "条件"
		}
	}
	// omitempty 是明确声明「可以不传」，比 OpenAPI 的 required 更贴近运行时
	if tag.has("omitempty") || tag.has("omitzero") || tag.has("omitnil") {
		return "否"
	}
	if schemaRequired {
		return "是"
	}
	return "否"
}

// requiredCondition 把条件必填/条件禁传规则渲染成一句人话
func requiredCondition(rule validateRule, namer *fieldNamer) string {
	switch rule.name {
	case "required_if":
		return "当 " + fieldValuePairs(rule.param, namer) + " 时必填"
	case "required_unless":
		return "除 " + fieldValuePairs(rule.param, namer) + " 外必填"
	case "required_with":
		return "当 " + fieldNames(rule.param, namer) + " 任一非空时必填"
	case "required_with_all":
		return "当 " + fieldNames(rule.param, namer) + " 全部非空时必填"
	case "required_without":
		return "当 " + fieldNames(rule.param, namer) + " 任一为空时必填"
	case "required_without_all":
		return "当 " + fieldNames(rule.param, namer) + " 全部为空时必填"
	case "excluded_if":
		return "当 " + fieldValuePairs(rule.param, namer) + " 时禁止传入"
	case "excluded_unless":
		return "除 " + fieldValuePairs(rule.param, namer) + " 外禁止传入"
	case "excluded_with":
		return "当 " + fieldNames(rule.param, namer) + " 任一非空时禁止传入"
	case "excluded_with_all":
		return "当 " + fieldNames(rule.param, namer) + " 全部非空时禁止传入"
	case "excluded_without":
		return "当 " + fieldNames(rule.param, namer) + " 任一为空时禁止传入"
	case "excluded_without_all":
		return "当 " + fieldNames(rule.param, namer) + " 全部为空时禁止传入"
	}
	return ""
}

// requiredConditions 收集字段上所有条件必填/条件禁传的说明
func requiredConditions(tag argusTag) []string {
	items := make([]string, 0, 2)
	for _, rule := range tag.rules {
		if text := requiredCondition(rule, tag.namer); text != "" {
			items = append(items, text)
		}
	}
	return items
}

// fieldValuePairs 渲染「字段 值 [字段 值]...」参数：成对出现，多对之间是 AND
func fieldValuePairs(param string, namer *fieldNamer) string {
	parts := strings.Fields(param)
	pairs := make([]string, 0, len(parts)/2+1)
	for i := 0; i+1 < len(parts); i += 2 {
		pairs = append(pairs, markdownCode(namer.display(parts[i]))+" = "+markdownCode(parts[i+1]))
	}
	if len(pairs) == 0 {
		return markdownCode(param)
	}
	return strings.Join(pairs, " 且 ")
}

// fieldNames 渲染「字段 字段 ...」参数
func fieldNames(param string, namer *fieldNamer) string {
	parts := strings.Fields(param)
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		names = append(names, markdownCode(namer.display(part)))
	}
	if len(names) == 0 {
		return markdownCode(param)
	}
	return strings.Join(names, "、")
}

// ==================== 限制 ====================

// argusFormatLabels 无参格式类规则 → 中文说明
//
// 只给名字（「邮箱」而不是 email）：这一列是给人扫读的，规则原文在 proto 里
// 这张表不是照 go-playground/validator 抄的——argus 自带一批它没有的规则
// （cn_phone / cn_idcard / strong_password / 图片格式等），漏掉就等于把真实约束藏起来
// argus_test.go 里有一个用例拿 argus 的注册表逐条对账，新增规则不会被悄悄漏掉
//
// [EN] Labels only, so the column stays scannable; the raw rule lives in the proto.
//
// This table is not copied from go-playground/validator: argus ships a batch of rules it does
// not have (cn_phone, cn_idcard, strong_password, the image formats), and leaving them out hides
// real constraints. A test in argus_test.go reconciles this file against argus's own registry
// rule by rule, so a newly added rule cannot slip through unnoticed.
var argusFormatLabels = map[string]string{
	"email":             "邮箱",
	"e164":              "E.164 手机号",
	"cn_phone":          "中国大陆手机号",
	"cn_idcard":         "中国大陆身份证号（15 或 18 位）",
	"cn_idcard_strict":  "中国大陆身份证号（含校验位）",
	"url":               "URL",
	"uri":               "URI",
	"http_url":          "HTTP URL",
	"https_url":         "HTTPS URL",
	"url_encoded":       "URL 编码",
	"json":              "JSON",
	"uuid":              "UUID",
	"uuid3":             "UUID v3",
	"uuid4":             "UUID v4",
	"uuid5":             "UUID v5",
	"uuid_rfc4122":      "UUID（RFC 4122）",
	"uuid3_rfc4122":     "UUID v3（RFC 4122）",
	"uuid4_rfc4122":     "UUID v4（RFC 4122）",
	"uuid5_rfc4122":     "UUID v5（RFC 4122）",
	"base32":            "Base32",
	"base64":            "Base64",
	"base64url":         "Base64 URL",
	"base64rawurl":      "Base64 RawURL（无填充）",
	"hexadecimal":       "十六进制",
	"hexcolor":          "十六进制颜色",
	"rgb":               "RGB 颜色",
	"rgba":              "RGBA 颜色",
	"hsl":               "HSL 颜色",
	"hsla":              "HSLA 颜色",
	"ip":                "IP",
	"ip_addr":           "IP",
	"ipv4":              "IPv4",
	"ipv6":              "IPv6",
	"cidr":              "CIDR",
	"cidrv4":            "IPv4 CIDR",
	"cidrv6":            "IPv6 CIDR",
	"mac":               "MAC 地址",
	"hostname":          "主机名",
	"hostname_rfc1123":  "主机名（RFC 1123）",
	"hostname_port":     "主机名:端口",
	"fqdn":              "FQDN",
	"port":              "端口",
	"semver":            "语义化版本",
	"cron":              "cron 表达式",
	"timezone":          "时区",
	"bcp47":             "BCP 47 语言标记",
	"mongodb":           "MongoDB ObjectID",
	"credit_card":       "银行卡号",
	"luhn_checksum":     "Luhn 校验",
	"isbn10":            "ISBN-10",
	"isbn13":            "ISBN-13",
	"issn":              "ISSN",
	"bic":               "BIC/SWIFT",
	"eth_addr":          "以太坊地址",
	"btc_addr":          "比特币地址",
	"datauri":           "Data URI",
	"html":              "HTML 标签",
	"html_encoded":      "HTML 编码",
	"latitude":          "纬度",
	"longitude":         "经度",
	"filepath":          "文件路径",
	"dirpath":           "目录路径",
	"file":              "已存在的文件",
	"dir":               "已存在的目录",
	"alpha":             "仅字母",
	"alphaspace":        "仅字母与空格",
	"alphanum":          "仅字母数字",
	"alphanumspace":     "仅字母数字与空格",
	"alphaunicode":      "仅 Unicode 字母",
	"alphanumunicode":   "仅 Unicode 字母数字",
	"ascii":             "仅 ASCII",
	"printascii":        "仅可打印 ASCII",
	"multibyte":         "含多字节字符",
	"lowercase":         "全小写",
	"uppercase":         "全大写",
	"upper_alpha":       "纯大写字母",
	"lower_alpha":       "纯小写字母",
	"word_chars":        "仅字母、数字与下划线",
	"digits":            "纯数字",
	"int_or_float":      "整数或最多 2 位小数",
	"integer":           "整数",
	"boolean":           "布尔值",
	"number":            "数字",
	"numeric":           "数值",
	"strong_password":   "强密码（8 位以上，含大小写字母、数字与特殊字符）",
	"dns_rfc1035_label": "DNS 标签（RFC 1035）",
}

// argusPlainLabels 本身就是一句话的规则：不加「格式」前缀，直接当限制写
var argusPlainLabels = map[string]string{
	"has_special":     "须含特殊字符",
	"has_double_byte": "须含双字节字符",
	"empty_line":      "须为空白行",
	"unique":          "元素不可重复",
	"dive":            "逐元素校验",
}

// argusImageLabels 图片格式规则 → 中文说明
//
// 这组规则校验的是字段的字节内容（PNG 还会查 IEND 尾块完整性），OpenAPI 侧只能看到
// type: string / format: byte，完全表达不了
//
// [EN] These check the field's raw bytes — the PNG rule even verifies the trailing IEND chunk —
// which OpenAPI, seeing only type: string with format: byte, cannot express at all.
var argusImageLabels = map[string]string{
	"png":  "PNG",
	"jpg":  "JPEG",
	"jpeg": "JPEG",
	"gif":  "GIF",
	"webp": "WebP",
	"bmp":  "BMP",
	"tiff": "TIFF",
	"ico":  "ICO 图标",
	"avif": "AVIF",
	"heic": "HEIC/HEIF",
	"svg":  "SVG",
}

// argusCrossFieldLabels 跨字段比较规则 → 关系说明
//
// 这是 argus 另一组 OpenAPI 完全没有对应物的属性：约束挂在「另一个字段的取值」上
// cs 系列（eqcsfield 等）比的是跨结构体的字段，句式一样，差别只在查找路径
//
// [EN] Another group of argus rules OpenAPI has no counterpart for: the constraint hangs off
// another field's value. The cs variants (eqcsfield and friends) compare against a field in
// another struct; the sentence is the same and only the lookup path differs.
var argusCrossFieldLabels = map[string]string{
	"eqfield":     "须等于",
	"nefield":     "须不等于",
	"gtfield":     "须大于",
	"gtefield":    "须不小于",
	"ltfield":     "须小于",
	"ltefield":    "须不大于",
	"eqcsfield":   "须等于",
	"necsfield":   "须不等于",
	"gtcsfield":   "须大于",
	"gtecsfield":  "须不小于",
	"ltcsfield":   "须小于",
	"ltecsfield":  "须不大于",
	"afterfield":  "须晚于",
	"beforefield": "须早于",
}

// argusFieldLenOps fieldlen 的比较符参数 → 关系说明
//
// fieldlen 的参数是 `FieldName` 或 `op:FieldName`（op 取 eq/ne/gt/gte/lt/lte，默认 eq），
// 比的是两个字段的长度。不解析这个前缀，`gt:Tags` 会被说成「长度须等于 tags」——正好说反
//
// [EN] fieldlen takes `FieldName` or `op:FieldName` (op one of eq/ne/gt/gte/lt/lte, eq by
// default) and compares the two fields' lengths. Without parsing that prefix, `gt:Tags` reads as
// "length must equal tags" — the exact opposite.
var argusFieldLenOps = map[string]string{
	"eq":  "须等于",
	"ne":  "须不等于",
	"gt":  "须大于",
	"gte": "须不小于",
	"lt":  "须小于",
	"lte": "须不大于",
}

// argusBoundRules 由 argusConstraints 合并成区间的规则，不单独渲染
var argusBoundRules = map[string]struct{}{
	"gte": {}, "lte": {}, "gt": {}, "lt": {},
	"min": {}, "max": {}, "len": {},
}

// argusSilentRules 不进「限制」列的规则：只影响校验流程，或已在别的列体现
//
// keys / endkeys 是映射键规则的起止标记，structonly / nostructlevel 控制是否进入结构体，
// oneof / oneofci 已渲染成「可选值」，required 系列在「必填」列
//
// [EN] keys and endkeys delimit map-key rules, structonly and nostructlevel control whether
// validation descends into a struct, oneof and oneofci are already rendered as allowed values,
// and the required family lives in the 必填 column.
var argusSilentRules = map[string]struct{}{
	"required": {}, "omitempty": {}, "omitzero": {}, "omitnil": {},
	"isdefault": {}, "structonly": {}, "nostructlevel": {},
	"keys": {}, "endkeys": {},
	"oneof": {}, "oneofci": {},
}

// argusRuleText 渲染单条限制类规则
//
// 返回值的三种组合：
//   - (text, true)：这条规则就渲染成 text
//   - ("", true)：认识它，但由别处渲染（必填列 / 可选值 / 区间合并）
//   - ("", false)：本文件不认识，交给调用方原样印出来
//
// 规则名只在这里判一次，argusRuleKnown 与对账测试都走同一条路径，不另留一份清单
//
// [EN] Rule names are matched in exactly one place: argusRuleKnown and the reconciliation test
// both go through this function rather than keeping a second list that could drift.
func argusRuleText(rule validateRule, namer *fieldNamer) (string, bool) {
	if _, ok := argusSilentRules[rule.name]; ok {
		return "", true
	}
	if _, ok := argusBoundRules[rule.name]; ok {
		return "", true
	}
	if _, ok := requiredConditionRules[rule.name]; ok {
		return "", true // 条件必填单独渲染成「必填条件」
	}
	if strings.HasPrefix(rule.name, "excluded_") {
		return "", true // 条件禁传同上
	}

	if label, ok := argusPlainLabels[rule.name]; ok {
		return label, true
	}
	if label, ok := argusCrossFieldLabels[rule.name]; ok {
		return label + " " + markdownCode(namer.display(rule.param)), true
	}
	if label, ok := argusImageLabels[rule.name]; ok {
		return "图片格式 " + label, true
	}
	if label, ok := argusFormatLabels[rule.name]; ok {
		return "格式 " + label, true
	}

	switch rule.name {
	case "eq", "eq_ignore_case":
		return "须等于 " + markdownCode(rule.param), true
	case "ne", "ne_ignore_case":
		return "不得等于 " + markdownCode(rule.param), true
	case "noneof", "noneofci":
		return "不得为 " + joinCode(strings.Fields(rule.param)), true
	case "contains":
		return "须包含 " + markdownCode(rule.param), true
	case "containsany":
		return "须含任一字符 " + markdownCode(rule.param), true
	case "containsrune":
		return "须包含字符 " + markdownCode(rule.param), true
	case "excludes":
		return "不得包含 " + markdownCode(rule.param), true
	case "excludesall":
		return "不得含任一字符 " + markdownCode(rule.param), true
	case "excludesrune":
		return "不得包含字符 " + markdownCode(rule.param), true
	case "startswith":
		return "须以 " + markdownCode(rule.param) + " 开头", true
	case "endswith":
		return "须以 " + markdownCode(rule.param) + " 结尾", true
	case "startsnotwith":
		return "不得以 " + markdownCode(rule.param) + " 开头", true
	case "endsnotwith":
		return "不得以 " + markdownCode(rule.param) + " 结尾", true
	case "datetime":
		return "时间格式 " + markdownCode(rule.param), true
	case "after":
		return "须晚于 " + markdownCode(rule.param), true
	case "before":
		return "须早于 " + markdownCode(rule.param), true
	case "fieldcontains":
		return "须包含字段 " + markdownCode(namer.display(rule.param)) + " 的值", true
	case "fieldexcludes":
		return "不得包含字段 " + markdownCode(namer.display(rule.param)) + " 的值", true
	case "fieldlen":
		return fieldLenText(rule.param, namer), true
	case "image":
		// 参数是空格分隔的格式白名单，不传则接受任意支持的格式
		if formats := strings.Fields(rule.param); len(formats) > 0 {
			return "图片格式 " + joinCode(formats), true
		}
		return "须为图片", true
	case "range":
		// range=Start|End：参数本身用 | 分隔（逗号会被当规则分隔符）
		if bounds := strings.SplitN(rule.param, "|", 2); len(bounds) == 2 {
			return "须落在 " + markdownCode(namer.display(bounds[0])) +
				" ~ " + markdownCode(namer.display(bounds[1])) + " 之间", true
		}
		return "区间 " + markdownCode(rule.param), true
	}
	return "", false
}

// fieldLenText 渲染 fieldlen：长度与另一个字段的长度比较
func fieldLenText(param string, namer *fieldNamer) string {
	op, target := "eq", param
	if idx := strings.Index(param, ":"); idx >= 0 {
		if _, ok := argusFieldLenOps[param[:idx]]; ok {
			op, target = param[:idx], param[idx+1:]
		}
	}
	return "长度" + argusFieldLenOps[op] + " " + markdownCode(namer.display(target)) + " 的长度"
}

// argusRuleKnown 这条规则名是否有专门的渲染
//
// 只给对账测试用：拿 argus 的注册表逐条问过来，没人认的规则会让测试红掉，
// 而不是在文档里悄悄退化成一段标签原文
//
// [EN] For the reconciliation test only: every name in argus's registry is asked here, so a rule
// nobody renders turns the test red instead of quietly degrading into raw tag text in the docs.
func argusRuleKnown(name string) bool {
	_, known := argusRuleText(validateRule{name: name, param: "Placeholder"}, nil)
	return known
}

// argusConstraints 把限制类规则渲染成「限制」列的条目
//
// scope 是比较维度（取值 / 长度 / 元素数）：gte=1 对数值是「取值 ≥ 1」，
// 对字符串是「长度 ≥ 1」，argus 两种都按字段的 Go 类型来，所以维度必须由调用方定
//
// [EN] scope is the dimension being compared (value, length, element count): gte=1 means
// "value ≥ 1" for a number and "length ≥ 1" for a string. argus decides by the field's Go type,
// so the caller has to supply the dimension.
func argusConstraints(tag argusTag, scope string) []string {
	var (
		items            []string
		minIncl, maxIncl string
		minExcl, maxExcl string
		exact            string
		hasExact         bool
	)

	for _, rule := range tag.rules {
		switch rule.name {
		case "gte", "min":
			minIncl = rule.param
			continue
		case "lte", "max":
			maxIncl = rule.param
			continue
		case "gt":
			minExcl = rule.param
			continue
		case "lt":
			maxExcl = rule.param
			continue
		case "len":
			exact, hasExact = rule.param, true
			continue
		}

		text, known := argusRuleText(rule, tag.namer)
		switch {
		case !known:
			// 没收录的规则原样印出来，宁可丑一点也不要让一条真实约束消失
			// [EN] Unknown rules are printed verbatim: better ugly than a real constraint vanishing.
			items = append(items, markdownCode(ruleText(rule)))
		case text != "":
			items = append(items, text)
		}
	}

	if bounded := boundsText(scope, minIncl, maxIncl, minExcl, maxExcl); bounded != "" {
		items = append([]string{bounded}, items...)
	}
	if hasExact {
		items = append([]string{scope + " " + markdownCode(exact)}, items...)
	}
	return items
}

// boundsText 把上下界合成一条区间文本
func boundsText(scope, minIncl, maxIncl, minExcl, maxExcl string) string {
	switch {
	case minIncl != "" && maxIncl != "":
		return scope + " " + markdownCode(minIncl+"~"+maxIncl)
	case minIncl != "":
		return scope + " ≥ " + markdownCode(minIncl)
	case maxIncl != "":
		return scope + " ≤ " + markdownCode(maxIncl)
	}

	parts := make([]string, 0, 2)
	if minExcl != "" {
		parts = append(parts, "> "+markdownCode(minExcl))
	}
	if maxExcl != "" {
		parts = append(parts, "< "+markdownCode(maxExcl))
	}
	if len(parts) == 0 {
		return ""
	}
	return scope + " " + strings.Join(parts, " 且 ")
}

// ruleText 还原规则原文（name 或 name=param）
func ruleText(rule validateRule) string {
	if rule.param == "" {
		return rule.name
	}
	return rule.name + "=" + rule.param
}

// argusEnumValues 取 oneof / oneofci 的可选值，供「可选值」一行使用
func argusEnumValues(tag argusTag) []string {
	for _, name := range []string{"oneof", "oneofci"} {
		if rule, ok := tag.find(name); ok {
			if values := strings.Fields(rule.param); len(values) > 0 {
				return values
			}
		}
	}
	return nil
}

// ==================== 比较维度 ====================

// numericFormats OpenAPI 2.0 里会被映射成 string 的数值 format
//
// 这是最容易把文档写反的一处：protoc-gen-openapiv2 把 int64/uint64 产成
// {"type":"string","format":"int64"}（JSON 里确实是带引号的数字），
// 只看 type 就会把 gte=1 说成「长度 ≥ 1」——而 argus 校验的是 Go 结构体字段，
// 那里是 int64，比的是数值大小
//
// [EN] This is the easiest place to get the docs backwards: protoc-gen-openapiv2 emits int64 and
// uint64 as {"type":"string","format":"int64"} — the JSON really does carry a quoted number — so
// reading `type` alone turns gte=1 into "length ≥ 1". argus validates the Go struct field, which
// is an int64, and compares magnitudes.
var numericFormats = map[string]struct{}{
	"int32": {}, "int64": {}, "uint32": {}, "uint64": {},
	"float": {}, "double": {}, "decimal": {},
}

// compareScope 判断比较类规则作用在哪个维度上
func compareScope(schema map[string]interface{}, res *refResolver) string {
	resolved, _ := res.resolve(schema)

	typ := mustString(schema["type"])
	if typ == "" {
		typ = mustString(resolved["type"])
	}
	format := mustString(schema["format"])
	if format == "" {
		format = mustString(resolved["format"])
	}

	if _, ok := numericFormats[format]; ok {
		return "取值"
	}
	switch typ {
	case "integer", "number":
		return "取值"
	case "array":
		return "元素数"
	case "object":
		return "字段数"
	}
	return "长度"
}
