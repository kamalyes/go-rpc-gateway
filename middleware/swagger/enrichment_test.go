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
		"| code | string | 是 | 6 位验证码 |",
		"| phone_region_code | string | 否 |",
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
	if !strings.Contains(description, "| page | query | integer | 否 |") {
		t.Errorf("参数速览缺少 page 行:\n%s", description)
	}
	if !strings.Contains(description, "示例：1") {
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
