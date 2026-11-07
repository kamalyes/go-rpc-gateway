/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\enrichment_test.go
 * @Description: UI 规范增强测试 - 联调元数据表格、参数速览、body 字段表、示例值
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

// newEnrichTestSpec 构建带联调元数据和 body 示例的 goctl 风格规范
func newEnrichTestSpec(t *testing.T) map[string]interface{} {
	t.Helper()

	raw := `{
  "swagger": "2.0",
  "info": {"title": "demo-api", "version": "1.0"},
  "paths": {
    "/api/auth/mfa/recovery/verify": {
      "post": {
        "summary": "校验 MFA 找回验证码",
        "description": "原描述",
        "x-integration-page": "auth/recovery",
        "x-integration-owner": "kamalyes",
        "x-integration-status": "stable",
        "x-examples": {
          "matched": {"code": 0, "data": {"identity_status": "matched"}}
        },
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {
              "type": "object",
              "required": ["code", "identity"],
              "properties": {
                "code": {"type": "string", "description": "6 位验证码"},
                "identity": {"type": "string", "minLength": 1, "maxLength": 128},
                "phone_region_code": {"type": "string"}
              }
            },
            "example": {"identity": "user@example.com", "code": "051924"}
          },
          {
            "name": "page",
            "in": "query",
            "type": "integer",
            "required": false,
            "description": "页码",
            "x-example": "1"
          }
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`

	var spec map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("解析测试规范失败: %v", err)
	}
	return spec
}

func getEnrichedDescription(t *testing.T, spec map[string]interface{}) string {
	t.Helper()

	paths, _ := spec[constants.FieldPaths].(map[string]interface{})
	pathItem, _ := paths["/api/auth/mfa/recovery/verify"].(map[string]interface{})
	op, _ := pathItem["post"].(map[string]interface{})
	return mustString(op[constants.FieldDescription])
}

func TestEnrichSpecForUIIntegrationTable(t *testing.T) {
	spec := newEnrichTestSpec(t)
	enrichSpecForUI(spec)
	description := getEnrichedDescription(t, spec)

	for _, want := range []string{
		"**联调信息**",
		"联调页面", "auth/recovery",
		"负责人", "kamalyes",
		"联调状态", "stable",
	} {
		if !strings.Contains(description, want) {
			t.Errorf("描述缺少 %q:\n%s", want, description)
		}
	}
	if !strings.HasPrefix(description, "原描述") {
		t.Errorf("原描述被覆盖: %s", description)
	}
}

func TestEnrichSpecForUIBodySection(t *testing.T) {
	spec := newEnrichTestSpec(t)
	enrichSpecForUI(spec)
	description := getEnrichedDescription(t, spec)

	for _, want := range []string{
		"**body**（必填）",
		"| 字段 | 类型 | 必填 | 说明 |",
		"| <code>code</code> | string | 是 | 6 位验证码 |",
		"| <code>phone_region_code</code> | string | 否 |",
		"限制：长度 <code>1~128</code>",
		"示例值：",
		"user@example.com",
		"051924",
	} {
		if !strings.Contains(description, want) {
			t.Errorf("描述缺少 %q:\n%s", want, description)
		}
	}
}

func TestEnrichSpecForUIParamOverviewAndScenario(t *testing.T) {
	spec := newEnrichTestSpec(t)
	enrichSpecForUI(spec)
	description := getEnrichedDescription(t, spec)

	// query 参数速览 + x-example 内联
	if !strings.Contains(description, "| <code>page</code> | query | integer | 否 |") {
		t.Errorf("参数速览缺少 page 行:\n%s", description)
	}
	if !strings.Contains(description, "示例：<code>1</code>") {
		t.Errorf("参数 x-example 未合入说明:\n%s", description)
	}

	// 命名场景示例
	if !strings.Contains(description, "**场景示例 - matched**") {
		t.Errorf("描述缺少场景示例标题:\n%s", description)
	}
	if !strings.Contains(description, "identity_status") {
		t.Errorf("场景示例内容丢失:\n%s", description)
	}
}

func TestEnrichSpecForUISkipsPlainOperations(t *testing.T) {
	spec := map[string]interface{}{
		constants.FieldPaths: map[string]interface{}{
			"/health": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":   "健康检查",
					"responses": map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
				},
			},
		},
	}

	enrichSpecForUI(spec)

	paths := spec[constants.FieldPaths].(map[string]interface{})
	op := paths["/health"].(map[string]interface{})["get"].(map[string]interface{})
	if _, exists := op[constants.FieldDescription]; exists {
		t.Error("无元数据的操作不应被注入描述")
	}
}

func TestEnrichSwaggerJSONForUIRoundTrip(t *testing.T) {
	input := []byte(`{"swagger":"2.0","paths":{"/a":{"get":{"summary":"s","x-integration-owner":"kamalyes"}}}}`)

	output, err := enrichSwaggerJSONForUI(input)
	if err != nil {
		t.Fatalf("enrichSwaggerJSONForUI failed: %v", err)
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(output, &spec); err != nil {
		t.Fatalf("增强结果无法反序列化: %v", err)
	}
	paths := spec[constants.FieldPaths].(map[string]interface{})
	op := paths["/a"].(map[string]interface{})["get"].(map[string]interface{})
	if !strings.Contains(mustString(op[constants.FieldDescription]), "kamalyes") {
		t.Error("增强结果缺少联调元数据")
	}
	// x- 扩展保留（前端联调平台仍可直接消费）
	if op["x-integration-owner"] != "kamalyes" {
		t.Error("增强结果不应删除 x- 扩展")
	}
}

func TestIsUIView(t *testing.T) {
	if isUIView(nil) {
		t.Error("nil 请求不应判定为 UI 视图")
	}
}

// newBilingualSpec 构建一份贴近真实 proto 产物的规范：
// 说明是「中文 | [EN] English」双语、枚举清单由 protoc-gen-openapiv2 追加在末尾、
// body 走 $ref 而不是内联 schema这三样正是把速览表打散的原因
//
// [EN] A spec shaped like the real protoc-gen-openapiv2 output: bilingual "中文 | [EN] English"
// descriptions, the enum listing it appends to the text, and a body that is a bare $ref rather
// than an inline schema. Those three are what used to tear the overview table apart.
func newBilingualSpec(t *testing.T) map[string]interface{} {
	t.Helper()

	raw := `{
  "swagger": "2.0",
  "paths": {
    "/v1/billing/entitlement-catalog": {
      "get": {
        "summary": "权益目录列表 | Entitlement catalog list",
        "parameters": [
          {
            "name": "group",
            "in": "query",
            "type": "string",
            "required": false,
            "description": "按分组过滤（不传则全部）| [EN] Filter by group; all when unset\n\n - ENTITLEMENT_GROUP_UNSPECIFIED: 未指定 | [EN] Unspecified\n - ENTITLEMENT_GROUP_TRANSLATION: 翻译 | [EN] Translation\n - ENTITLEMENT_GROUP_DATA: 数据 | [EN] Data",
            "enum": ["ENTITLEMENT_GROUP_UNSPECIFIED", "ENTITLEMENT_GROUP_TRANSLATION", "ENTITLEMENT_GROUP_DATA"],
            "default": "ENTITLEMENT_GROUP_UNSPECIFIED"
          }
        ],
        "x-integration-owner": "kamalyes"
      },
      "put": {
        "summary": "更新权益目录项",
        "x-integration-owner": "kamalyes",
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {"$ref": "#/definitions/UpdateBody"}
          }
        ]
      }
    }
  },
  "definitions": {
    "UpdateBody": {
      "type": "object",
      "required": ["remark"],
      "properties": {
        "group": {"$ref": "#/definitions/enumsEntitlementGroup", "title": "所属分组 | [EN] Group"},
        "remark": {"type": "string", "title": "变更说明（必填）| [EN] Change note", "minLength": 2, "maxLength": 200},
        "names": {"type": "array", "items": {"type": "string"}, "title": "名称 | [EN] Names"},
        "meta": {"$ref": "#/definitions/SaveMeta", "title": "保存元信息 | [EN] Save metadata"},
        "code": {"type": "string", "title": "权益标识 | [EN] Key", "pattern": "^[a-z]+(_[a-z]+)*$"}
      }
    },
    "SaveMeta": {"type": "object", "properties": {"remark": {"type": "string"}}},
    "enumsEntitlementGroup": {
      "type": "string",
      "enum": ["ENTITLEMENT_GROUP_UNSPECIFIED", "ENTITLEMENT_GROUP_TRANSLATION"],
      "description": " - ENTITLEMENT_GROUP_UNSPECIFIED: 未指定 | [EN] Unspecified\n - ENTITLEMENT_GROUP_TRANSLATION: 翻译 | [EN] Translation",
      "title": "功能权益分组\n[EN] Entitlement group"
    }
  }
}`

	var spec map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("解析测试规范失败: %v", err)
	}
	return spec
}

func bilingualDescription(t *testing.T, spec map[string]interface{}, method string) string {
	t.Helper()

	paths, _ := spec[constants.FieldPaths].(map[string]interface{})
	pathItem, _ := paths["/v1/billing/entitlement-catalog"].(map[string]interface{})
	op, _ := pathItem[method].(map[string]interface{})
	return mustString(op[constants.FieldDescription])
}

// tableRows 返回描述里所有以 | 开头的表格行
func tableRows(description string) []string {
	rows := make([]string, 0, 8)
	for _, line := range strings.Split(description, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			rows = append(rows, strings.TrimSpace(line))
		}
	}
	return rows
}

// TestEnrichKeepsTableColumnsAligned 说明里的竖线不得把单元格切开
//
// 回归点：双语注释每条都带 `|`，之前用 `\|` 转义，Swagger UI 的渲染器不认，
// 一行说明会被切成七八列、还把反斜杠显示出来
func TestEnrichKeepsTableColumnsAligned(t *testing.T) {
	spec := newBilingualSpec(t)
	enrichSpecForUI(spec)
	description := bilingualDescription(t, spec, "get")

	if strings.Contains(description, `\|`) {
		t.Errorf("说明里仍有反斜杠转义的竖线:\n%s", description)
	}

	for _, row := range tableRows(description) {
		cells := strings.Count(row, "|") - 1
		if cells != 2 && cells != 5 {
			t.Errorf("表格行列数错位（%d 列）: %s", cells, row)
		}
	}
}

// TestEnrichRendersEnumAsAllowedValues 枚举清单折成「可选值」而不是灌进说明
func TestEnrichRendersEnumAsAllowedValues(t *testing.T) {
	spec := newBilingualSpec(t)
	enrichSpecForUI(spec)
	description := bilingualDescription(t, spec, "get")

	for _, want := range []string{
		"可选值：<code>ENTITLEMENT_GROUP_UNSPECIFIED</code>、",
		"<code>ENTITLEMENT_GROUP_DATA</code>",
		"默认值：<code>ENTITLEMENT_GROUP_UNSPECIFIED</code>",
		"按分组过滤（不传则全部）",
	} {
		if !strings.Contains(description, want) {
			t.Errorf("描述缺少 %q:\n%s", want, description)
		}
	}

	// protoc 追加的枚举正文不应再出现在说明列里（否则同一份内容渲染两遍）
	if strings.Contains(description, "- ENTITLEMENT_GROUP_DATA: 数据") {
		t.Errorf("枚举清单原文未从说明列剔除:\n%s", description)
	}
}

// TestEnrichResolvesRefBody body 是 $ref 时也要渲染出字段表
//
// 回归点：之前只读内联 schema.properties，$ref 的 body 一个字段都列不出来，
// 整个 body 区块只剩一行 **body**（必填）
func TestEnrichResolvesRefBody(t *testing.T) {
	spec := newBilingualSpec(t)
	enrichSpecForUI(spec)
	description := bilingualDescription(t, spec, "put")

	for _, want := range []string{
		"| 字段 | 类型 | 必填 | 说明 |",
		"<code>remark</code>",
		"限制：长度 <code>2~200</code>",
		"<code>code</code>",
		"格式 <code>^[a-z]+(_[a-z]+)*$</code>",
		// 枚举字段展开到底层类型并带出可选值
		"可选值：<code>ENTITLEMENT_GROUP_UNSPECIFIED</code>",
		// 对象引用渲染成可点的 Models 锚点
		`<a href="#model-SaveMeta">SaveMeta</a>`,
	} {
		if !strings.Contains(description, want) {
			t.Errorf("body 字段表缺少 %q:\n%s", want, description)
		}
	}

	// required 只认 definitions 里的 required 列表
	if !strings.Contains(description, "<code>remark</code> | string | 是 |") {
		t.Errorf("required 字段未标记为必填:\n%s", description)
	}
}

// TestEnrichEscapesAngleBracketsInType 复合类型的尖括号必须转义
//
// 描述走 html:true 的渲染器，原样的 array<string> 会被当成未知标签吞掉，只剩 array
func TestEnrichEscapesAngleBracketsInType(t *testing.T) {
	spec := newBilingualSpec(t)
	enrichSpecForUI(spec)
	description := bilingualDescription(t, spec, "put")

	if !strings.Contains(description, "array&lt;string&gt;") {
		t.Errorf("数组类型未转义尖括号:\n%s", description)
	}
	if strings.Contains(description, "array<string>") {
		t.Errorf("描述里仍有未转义的尖括号:\n%s", description)
	}
}

// TestMarkdownCodeKeepsPipesIntact 行内代码里的竖线既不能切表格，也不能漏成实体原文
func TestMarkdownCodeKeepsPipesIntact(t *testing.T) {
	got := markdownCode("^(a|b)$")
	if strings.Contains(got, "`") {
		t.Errorf("行内代码不应使用反引号（反引号内实体不解析）: %s", got)
	}
	if strings.Contains(strings.TrimPrefix(got, "<code>"), "|") {
		t.Errorf("行内代码里的竖线未转义: %s", got)
	}
	if want := "<code>^(a&#124;b)$</code>"; got != want {
		t.Errorf("markdownCode = %s, want %s", got, want)
	}
}

// newStructRefSpec 构建一份把结构体引用用全的规范：对象引用、对象数组、protobuf map、
// 枚举引用、悬空引用各一个字段，用来固定类型列的渲染方式
//
// [EN] A spec that exercises every shape of struct reference — a plain object ref, an array of
// objects, a protobuf map, an enum ref and a dangling ref — one field each, to pin down how the
// type column renders them.
func newStructRefSpec(t *testing.T) map[string]interface{} {
	t.Helper()

	raw := `{
  "swagger": "2.0",
  "paths": {
    "/v1/items": {
      "post": {
        "summary": "保存条目",
        "x-integration-owner": "kamalyes",
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {"$ref": "#/definitions/SaveItemBody"}
          }
        ]
      }
    }
  },
  "definitions": {
    "SaveItemBody": {
      "type": "object",
      "properties": {
        "detail": {"$ref": "#/definitions/ItemDetail", "title": "明细"},
        "tags": {"type": "array", "items": {"$ref": "#/definitions/ItemTag"}, "title": "标签"},
        "labels": {"type": "object", "additionalProperties": {"$ref": "#/definitions/commonLocalizedText"}, "title": "多语言名称"},
        "extra": {"type": "object", "additionalProperties": true, "title": "自由扩展"},
        "status": {"$ref": "#/definitions/enumsItemStatus", "title": "状态"},
        "missing": {"$ref": "#/definitions/NotDeclared", "title": "引用了不存在的定义"},
        "names": {"type": "array", "items": {"type": "string"}, "title": "名称"}
      }
    },
    "ItemDetail": {"type": "object", "properties": {"remark": {"type": "string"}}},
    "ItemTag": {"type": "object", "properties": {"key": {"type": "string"}}},
    "commonLocalizedText": {"type": "object", "properties": {"zh": {"type": "string"}}},
    "enumsItemStatus": {"type": "string", "enum": ["ITEM_STATUS_ON", "ITEM_STATUS_OFF"]}
  }
}`

	var spec map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("解析测试规范失败: %v", err)
	}
	return spec
}

// TestEnrichLinksStructTypesToModels 结构体类型渲染成指向 Models 区块的锚点
//
// 回归点：对象引用过去只渲染成 <code>SaveMeta</code>，想看它有哪些字段只能自己滚到页面底部的
// Models 区块里按名字翻；对象数组与 protobuf 的 map 字段更是连定义名都看不到，一律显示成
// array 或 object
//
// [EN] Regression: an object reference used to render as <code>SaveMeta</code>, so seeing its
// fields meant scrolling to the Models section at the foot of the page and hunting for the name.
// Arrays of objects and protobuf map fields did not even show the definition name — they came
// out as a bare "array" or "object".
func TestEnrichLinksStructTypesToModels(t *testing.T) {
	spec := newStructRefSpec(t)
	enrichSpecForUI(spec)

	paths, _ := spec[constants.FieldPaths].(map[string]interface{})
	pathItem, _ := paths["/v1/items"].(map[string]interface{})
	op, _ := pathItem["post"].(map[string]interface{})
	description := mustString(op[constants.FieldDescription])

	for _, want := range []string{
		// body 自己的定义名也可点；链接写在 ** ** 之内，色条规则才还命中
		"**body " + `<a href="#model-SaveItemBody">SaveItemBody</a>` + "**（必填）",
		// 对象引用
		`| <code>detail</code> | <a href="#model-ItemDetail">ItemDetail</a> |`,
		// 对象数组：链接嵌在尖括号里，尖括号仍是实体
		`array&lt;<a href="#model-ItemTag">ItemTag</a>&gt;`,
		// protobuf map：值类型是结构体时同样给入口
		`map&lt;string, <a href="#model-commonLocalizedText">commonLocalizedText</a>&gt;`,
		// 枚举引用展开到底层类型，不给链接（可选值已单独列出）
		"| <code>status</code> | string | 否 |",
		"可选值：<code>ITEM_STATUS_ON</code>",
		// additionalProperties: true 不是 map<>，照常显示 object
		"| <code>extra</code> | object | 否 |",
		// 悬空引用在 definitions 里找不到落点，不能生成死链接
		"| <code>missing</code> | object | 否 |",
		// 标量数组不受影响
		"array&lt;string&gt;",
	} {
		if !strings.Contains(description, want) {
			t.Errorf("body 字段表缺少 %q:\n%s", want, description)
		}
	}

	if strings.Contains(description, `#model-NotDeclared`) {
		t.Errorf("悬空引用生成了死链接:\n%s", description)
	}
	// 链接不再套 <code>：它要嵌进 array<>、map<> 里，叠一层代码块底色会让一格类型压三层背景
	if strings.Contains(description, "<code><a") || strings.Contains(description, "</a></code>") {
		t.Errorf("类型链接被 <code> 包裹:\n%s", description)
	}
}

// TestModelLinkEscapesName 定义名直接进 href，引号与竖线都不能原样留下
//
// 规范可以来自远端服务：引号不实体化会把 href 属性提前闭合，竖线不实体化会把表格切开
//
// [EN] Specs can come from a remote service: an unescaped quote would close the href attribute
// early, and an unescaped pipe would split the table cell.
func TestModelLinkEscapesName(t *testing.T) {
	if got, want := modelLink("SaveMeta"), `<a href="#model-SaveMeta">SaveMeta</a>`; got != want {
		t.Errorf("modelLink = %s, want %s", got, want)
	}

	got := modelLink(`a"|b`)
	if want := `<a href="#model-a&quot;&#124;b">a"&#124;b</a>`; got != want {
		t.Errorf("modelLink = %s, want %s", got, want)
	}
}

// TestLeadParagraphDropsEnumLines 首段提取应跳过枚举清单与后续设计背景段落
func TestLeadParagraphDropsEnumLines(t *testing.T) {
	doc := "行版本 | [EN] Row version\n带上它服务端才做 CAS\n\n这里是一整段设计背景，不进速览表\n\n - A: 甲\n - B: 乙"
	got := leadParagraph(doc)
	want := "行版本 | [EN] Row version\n带上它服务端才做 CAS"
	if got != want {
		t.Errorf("leadParagraph = %q, want %q", got, want)
	}

	if only := leadParagraph(" - A: 甲\n - B: 乙"); only != "" {
		t.Errorf("纯枚举清单应得到空首段，实际 %q", only)
	}
}
