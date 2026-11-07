/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\argus_test.go
 * @Description: argus 校验标签对标测试 - 必填、限制、比较维度与指令剔除
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"encoding/json"
	"strings"
	"testing"

	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
)

// newTrialSettingsSpec 按 protoc-gen-openapiv2 的真实产物构建规范
//
// 对应的 proto（字段规则写在 @inject_tag 里，由 argus 在网关拦截器里执行）：
//
//	bool  enabled              = 1; // 是否启用试用 | [EN] Whether the trial is offered
//	string plan_id             = 2; // 试用套餐…… @inject_tag: validate:"required_if=Enabled true"
//	int32  days                = 3; // 试用天数…… @inject_tag: validate:"required,gte=1,lte=30"
//	int64  agent_seat_limit    = 4; // 试用期内含成员席位…… @inject_tag: validate:"required,gte=1"
//	int64  translation_char_cap= 7; // 试用期翻译字符上限…… @inject_tag: validate:"required,gt=0"
//	common.AdminSaveMeta meta  = 9; // 保存元信息…… @inject_tag: validate:"required"
//
// 两处必须照抄产物、不能图省事：
//   - trailing 注释连 @inject_tag 指令一起进 title
//   - int64/uint64 被映射成 {"type":"string","format":"int64"}，而不是 integer
//
// [EN] Shaped like the real protoc-gen-openapiv2 output. Two details are copied faithfully
// rather than simplified: the trailing comment carries the @inject_tag directive into `title`,
// and int64/uint64 are mapped to {"type":"string","format":"int64"} rather than integer.
func newTrialSettingsSpec(t *testing.T) map[string]interface{} {
	t.Helper()

	raw := `{
  "swagger": "2.0",
  "paths": {
    "/v1/billing/trial-settings": {
      "post": {
        "summary": "保存免费试用设置 | [EN] Save trial settings",
        "x-integration-owner": "kamalyes",
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {"$ref": "#/definitions/billingTrialSettingsSaveRequest"}
          }
        ]
      }
    }
  },
  "definitions": {
    "billingTrialSettingsSaveRequest": {
      "type": "object",
      "properties": {
        "enabled": {"type": "boolean", "title": "是否启用试用 | [EN] Whether the trial is offered"},
        "plan_id": {"type": "string", "title": "试用套餐（启用时必填，须为在售付费套餐）| [EN] Trial plan, required when enabled @inject_tag: validate:\"required_if=Enabled true\""},
        "days": {"type": "integer", "format": "int32", "title": "试用天数 | [EN] Trial length @inject_tag: validate:\"required,gte=1,lte=30\""},
        "agent_seat_limit": {"type": "string", "format": "int64", "title": "试用期内含成员席位 | [EN] Agent seats @inject_tag: validate:\"required,gte=1\""},
        "hosted_account_limit": {"type": "string", "format": "int64", "title": "试用期内含云托管账号 | [EN] Hosted accounts @inject_tag: validate:\"gte=0\""},
        "translation_char_cap": {"type": "string", "format": "int64", "title": "试用期翻译字符上限 | [EN] Translation character cap @inject_tag: validate:\"required,gt=0\""},
        "meta": {"$ref": "#/definitions/commonAdminSaveMeta", "title": "保存元信息 | [EN] Save metadata @inject_tag: validate:\"required\""},
        "version": {"type": "integer", "format": "int32", "title": "行版本 | [EN] Row version @inject_tag: validate:\"gte=0\""}
      }
    },
    "commonAdminSaveMeta": {"type": "object", "properties": {"remark": {"type": "string"}}}
  }
}`

	var spec map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("解析测试规范失败: %v", err)
	}
	return spec
}

func trialDescription(t *testing.T, spec map[string]interface{}) string {
	t.Helper()

	paths, _ := spec[constants.FieldPaths].(map[string]interface{})
	pathItem, _ := paths["/v1/billing/trial-settings"].(map[string]interface{})
	op, _ := pathItem["post"].(map[string]interface{})
	return mustString(op[constants.FieldDescription])
}

// TestEnrichAlignsRequiredWithArgus 「必填」列必须对标 argus，而不是 OpenAPI 的 required 列表
//
// 回归点：proto3 字段几乎从不出现在 OpenAPI 的 required 列表里，于是带 validate:"required"
// 的字段一律显示成「否」——文档说可以不传，网关直接 400
//
// [EN] Regression: proto3 fields almost never appear in OpenAPI's required list, so every
// validate:"required" field was documented as optional. The document said the field could be
// omitted; the gateway answered 400.
func TestEnrichAlignsRequiredWithArgus(t *testing.T) {
	spec := newTrialSettingsSpec(t)
	enrichSpecForUI(spec)
	description := trialDescription(t, spec)

	for _, want := range []string{
		// validate:"required" → 是
		"| <code>days</code> | integer | 是 |",
		"| <code>translation_char_cap</code> | string | 是 |",
		// 对象引用上的 required 也认
		`| <code>meta</code> | <a href="#model-commonAdminSaveMeta">commonAdminSaveMeta</a> | 是 |`,
		// 没有标签、也不在 required 列表里 → 否
		"| <code>enabled</code> | boolean | 否 |",
		// 只有 gte=0，不是 required → 否
		"| <code>version</code> | integer | 否 |",
		// required_if 这类 OpenAPI 表达不了的，列里写「条件」，条件本身进说明
		"| <code>plan_id</code> | string | 条件 |",
		"必填条件：当 <code>enabled</code> = <code>true</code> 时必填",
	} {
		if !strings.Contains(description, want) {
			t.Errorf("字段表缺少 %q:\n%s", want, description)
		}
	}
}

// TestEnrichAlignsConstraintsWithArgus 「限制」列必须把真正在跑的规则写出来
func TestEnrichAlignsConstraintsWithArgus(t *testing.T) {
	spec := newTrialSettingsSpec(t)
	enrichSpecForUI(spec)
	description := trialDescription(t, spec)

	for _, want := range []string{
		// gte + lte 合成闭区间
		"限制：取值 <code>1~30</code>",
		// 单边 gte
		"限制：取值 ≥ <code>1</code>",
		"限制：取值 ≥ <code>0</code>",
		// gt 是开区间，不能写成 ≥
		"限制：取值 > <code>0</code>",
	} {
		if !strings.Contains(description, want) {
			t.Errorf("字段表缺少 %q:\n%s", want, description)
		}
	}

	// required 本身不是「限制」，不该重复出现在限制里
	if strings.Contains(description, "限制：required") {
		t.Errorf("required 被当成限制渲染:\n%s", description)
	}
}

// TestEnrichDropsInjectTagDirective @inject_tag 指令是给 protoc-go-inject-tag 看的，不该进接口文档
func TestEnrichDropsInjectTagDirective(t *testing.T) {
	spec := newTrialSettingsSpec(t)
	enrichSpecForUI(spec)
	description := trialDescription(t, spec)

	for _, leak := range []string{"@inject_tag", `validate:"`, "required_if=Enabled"} {
		if strings.Contains(description, leak) {
			t.Errorf("说明列残留指令原文 %q:\n%s", leak, description)
		}
	}
	// 指令摘掉了，说明正文要留着
	if !strings.Contains(description, "试用天数") || !strings.Contains(description, "Trial length") {
		t.Errorf("摘指令时把说明正文一起删了:\n%s", description)
	}

	// 列数不能被条件说明里的符号切开
	for _, row := range tableRows(description) {
		if cells := strings.Count(row, "|") - 1; cells != 2 && cells != 4 {
			t.Errorf("表格行列数错位（%d 列）: %s", cells, row)
		}
	}
}

// TestCompareScopeFollowsGoType 比较维度要按 argus 看到的 Go 类型定，不是按 JSON 编码定
//
// 这是最容易把文档写反的一处：protoc-gen-openapiv2 把 int64 产成
// {"type":"string","format":"int64"}，只看 type 就会把 gte=1 说成「长度 ≥ 1」，
// 而 argus 校验的是 Go 结构体字段（int64），比的是数值大小
//
// [EN] The easiest place to get the docs backwards: protoc-gen-openapiv2 emits int64 as
// {"type":"string","format":"int64"}, so reading `type` alone turns gte=1 into "length ≥ 1",
// while argus validates the Go field — an int64 — and compares magnitudes.
func TestCompareScopeFollowsGoType(t *testing.T) {
	res := newRefResolver(map[string]interface{}{})

	cases := []struct {
		name   string
		schema map[string]interface{}
		want   string
	}{
		{"int64 产成 string", map[string]interface{}{"type": "string", "format": "int64"}, "取值"},
		{"uint64 产成 string", map[string]interface{}{"type": "string", "format": "uint64"}, "取值"},
		{"int32 仍是 integer", map[string]interface{}{"type": "integer", "format": "int32"}, "取值"},
		{"浮点", map[string]interface{}{"type": "number", "format": "double"}, "取值"},
		{"真字符串", map[string]interface{}{"type": "string"}, "长度"},
		{"bytes 走长度", map[string]interface{}{"type": "string", "format": "byte"}, "长度"},
		{"数组比个数", map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}}, "元素数"},
	}
	for _, c := range cases {
		if got := compareScope(c.schema, res); got != c.want {
			t.Errorf("%s: compareScope = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestSplitValidateDoc 指令剔除与规则解析
func TestSplitValidateDoc(t *testing.T) {
	doc := `试用天数 | [EN] Trial length @inject_tag: validate:"required,gte=1,lte=30"`
	text, rules := splitValidateDoc(doc)

	if want := "试用天数 | [EN] Trial length"; text != want {
		t.Errorf("text = %q, want %q", text, want)
	}
	if len(rules) != 3 {
		t.Fatalf("规则数 = %d, want 3: %+v", len(rules), rules)
	}
	if rules[1].name != "gte" || rules[1].param != "1" {
		t.Errorf("rules[1] = %+v, want {gte 1}", rules[1])
	}

	// 一条指令带多个标签时整条都要摘掉
	multi := `备注 @inject_tag: json:"remark" validate:"required,max=200"`
	text, rules = splitValidateDoc(multi)
	if text != "备注" {
		t.Errorf("多标签指令未整条摘除: %q", text)
	}
	if len(rules) != 2 {
		t.Errorf("规则数 = %d, want 2: %+v", len(rules), rules)
	}

	// 没有指令时原样返回
	if text, rules = splitValidateDoc("只是说明"); text != "只是说明" || rules != nil {
		t.Errorf("无指令的说明被改动: %q %+v", text, rules)
	}

	// 转义逗号不算分隔符（与 argus 的解析口径一致，反斜杠留在参数里）
	rules = parseValidateTag(`contains=a\,b,required`)
	if len(rules) != 2 || rules[0].param != `a\,b` {
		t.Errorf("转义逗号解析错误: %+v", rules)
	}
}

// TestArgusConstraintsSpecialAttributes argus 相对 OpenAPI 的特殊属性要给出人话说明
//
// 跨字段比较、条件必填、逐元素校验这些 OpenAPI 完全没有对应物，
// 不翻译就只能把标签原文丢给读文档的人
//
// [EN] Cross-field comparisons, conditional requirements and per-element validation have no
// OpenAPI counterpart at all; without translation the reader is left with the raw tag.
func TestArgusConstraintsSpecialAttributes(t *testing.T) {
	namer := newFieldNamer([]string{"start_at", "end_at", "contact_email", "tags"})

	cases := []struct {
		tag   string
		scope string
		want  string
	}{
		// 跨字段比较：参数是 Go 字段名，要换成文档里的字段名
		{"gtefield=StartAt", "取值", "须不小于 <code>start_at</code>"},
		{"ltfield=EndAt", "取值", "须小于 <code>end_at</code>"},
		{"eqfield=ContactEmail", "长度", "须等于 <code>contact_email</code>"},
		// 格式类给中文名
		{"email", "长度", "格式 邮箱"},
		{"e164", "长度", "格式 E.164 手机号"},
		{"uuid4", "长度", "格式 UUID v4"},
		{"datetime=2006-01-02", "长度", "时间格式 <code>2006-01-02</code>"},
		// 集合类
		{"unique", "元素数", "元素不可重复"},
		{"dive", "元素数", "逐元素校验"},
		{"noneof=a b", "长度", "不得为 <code>a</code>、<code>b</code>"},
		// 字符串内容
		{"startswith=sk-", "长度", "须以 <code>sk-</code> 开头"},
		{"excludes=@", "长度", "不得包含 <code>@</code>"},
		// len 是等值，不是区间
		{"len=6", "长度", "长度 <code>6</code>"},
		// range 的参数用 | 分隔（逗号会被当规则分隔符）
		{"range=StartAt|EndAt", "取值", "须落在 <code>start_at</code> ~ <code>end_at</code> 之间"},
		// 没收录的规则原样印出来，不能静默消失
		{"someday_new_rule=7", "长度", "<code>someday_new_rule=7</code>"},
	}

	for _, c := range cases {
		tag := argusTag{rules: parseValidateTag(c.tag), namer: namer}
		got := strings.Join(argusConstraints(tag, c.scope), "；")
		if !strings.Contains(got, c.want) {
			t.Errorf("%s → %q, 缺少 %q", c.tag, got, c.want)
		}
	}
}

// TestRequiredConditionsCoverWholeFamily 条件必填族每条都要有说明，不能只认 required_if
func TestRequiredConditionsCoverWholeFamily(t *testing.T) {
	namer := newFieldNamer([]string{"notify_type", "email", "phone"})

	cases := map[string]string{
		"required_if=NotifyType email":         "当 <code>notify_type</code> = <code>email</code> 时必填",
		"required_unless=NotifyType sms":       "除 <code>notify_type</code> = <code>sms</code> 外必填",
		"required_with=Email":                  "当 <code>email</code> 任一非空时必填",
		"required_with_all=Email Phone":        "当 <code>email</code>、<code>phone</code> 全部非空时必填",
		"required_without=Phone":               "当 <code>phone</code> 任一为空时必填",
		"required_without_all=Email Phone":     "当 <code>email</code>、<code>phone</code> 全部为空时必填",
		"excluded_if=NotifyType sms":           "当 <code>notify_type</code> = <code>sms</code> 时禁止传入",
		"required_if=NotifyType email Phone 1": "当 <code>notify_type</code> = <code>email</code> 且 <code>phone</code> = <code>1</code> 时必填",
	}

	for tagText, want := range cases {
		tag := argusTag{rules: parseValidateTag(tagText), namer: namer}
		got := strings.Join(requiredConditions(tag), "；")
		if got != want {
			t.Errorf("%s → %q, want %q", tagText, got, want)
		}
	}
}

// TestRequiredCellPrefersArgus 必填列的取值优先级
func TestRequiredCellPrefersArgus(t *testing.T) {
	cases := []struct {
		name           string
		schemaRequired bool
		tag            string
		want           string
	}{
		{"标签 required 压过 schema", false, "required", "是"},
		{"条件必填", false, "required_if=Enabled true", "条件"},
		{"omitempty 明确可不传", true, "omitempty", "否"},
		{"没有标签时用 schema", true, "", "是"},
		{"两边都没有", false, "", "否"},
		{"gte 不是必填", false, "gte=0", "否"},
	}
	for _, c := range cases {
		tag := argusTag{rules: parseValidateTag(c.tag), namer: newFieldNamer(nil)}
		if got := requiredCell(c.schemaRequired, tag); got != c.want {
			t.Errorf("%s: requiredCell = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestArgusOneofFeedsAllowedValues string 字段的可选值常常只写在 oneof 标签里，OpenAPI 侧没有 enum
func TestArgusOneofFeedsAllowedValues(t *testing.T) {
	spec := map[string]interface{}{
		constants.FieldPaths: map[string]interface{}{
			"/v1/notify": map[string]interface{}{
				"post": map[string]interface{}{
					"x-integration-owner": "kamalyes",
					constants.FieldParameters: []interface{}{
						map[string]interface{}{
							constants.FieldName:        "channel",
							"in":                       "query",
							"type":                     "string",
							constants.FieldDescription: `通知渠道 @inject_tag: validate:"required,oneof=email sms webhook"`,
						},
					},
				},
			},
		},
	}

	enrichSpecForUI(spec)
	paths := spec[constants.FieldPaths].(map[string]interface{})
	op := paths["/v1/notify"].(map[string]interface{})["post"].(map[string]interface{})
	description := mustString(op[constants.FieldDescription])

	for _, want := range []string{
		"| <code>channel</code> | query | string | 是 |",
		"可选值：<code>email</code>、<code>sms</code>、<code>webhook</code>",
		"通知渠道",
	} {
		if !strings.Contains(description, want) {
			t.Errorf("参数速览缺少 %q:\n%s", want, description)
		}
	}
	// oneof 已渲染成可选值，不该再出现在限制里
	if strings.Contains(description, "限制：<code>oneof") {
		t.Errorf("oneof 被重复渲染进限制:\n%s", description)
	}
}

// TestOpenAPIConstraintsStillUsedWithoutTag 没有 argus 标签时，OpenAPI 原生约束照旧渲染
func TestOpenAPIConstraintsStillUsedWithoutTag(t *testing.T) {
	spec := newBilingualSpec(t)
	enrichSpecForUI(spec)
	description := bilingualDescription(t, spec, "put")

	for _, want := range []string{
		"限制：长度 <code>2~200</code>",
		"格式 <code>^[a-z]+(_[a-z]+)*$</code>",
	} {
		if !strings.Contains(description, want) {
			t.Errorf("OpenAPI 原生约束丢失 %q:\n%s", want, description)
		}
	}
}
