/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\documents\spec_fidelity_test.go
 * @Description: goctl 风格规范的独立文档保真测试 - securityDefinitions、顶层 x- 透传、参数约束
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package documents

import (
	"encoding/json"
	"testing"
	"time"

	goswagger "github.com/kamalyes/go-rpc-gateway/config/swagger"
	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	"github.com/kamalyes/go-toolbox/pkg/logger"
)

// goctlStyleServiceSpec goctl（go-zero）生成风格的服务规范
const goctlStyleServiceSpec = `{
  "swagger": "2.0",
  "info": {"title": "demo-api", "version": "1.0"},
  "schemes": ["https"],
  "host": "api.example.com",
  "securityDefinitions": {
    "BearerAuth": {"type": "apiKey", "name": "Authorization", "in": "header"}
  },
  "x-date": "2026-11-06 13:15:08",
  "x-goctl-version": "1.10.1",
  "tags": [{"name": "认证登录", "description": "注册、登录、验证码。"}],
  "paths": {
    "/api/auth/mfa/recovery/verify": {
      "post": {
        "summary": "校验 MFA 找回验证码",
        "operationId": "verifyMfaRecoveryCode",
        "tags": ["认证登录"],
        "x-integration-owner": "kamalyes",
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
              "required": ["code"],
              "properties": {
                "code": {"type": "string", "minLength": 6, "maxLength": 6},
                "phone_region_code": {"type": "string", "pattern": "^[A-Z]{2}$"}
              }
            },
            "example": {"identity": "user@example.com", "code": "051924"}
          },
          {
            "name": "page",
            "in": "query",
            "type": "integer",
            "x-example": "1",
            "x-nullable": true
          }
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`

func newGoctlDocumentBuilder(t *testing.T) *Builder {
	t.Helper()

	var serviceSpec map[string]interface{}
	if err := json.Unmarshal([]byte(goctlStyleServiceSpec), &serviceSpec); err != nil {
		t.Fatalf("解析测试规范失败: %v", err)
	}

	config := &goswagger.Swagger{
		Aggregate: &goswagger.AggregateConfig{
			Enabled: true,
			Mode:    "merge",
			Documents: []*goswagger.DocumentSpec{
				{
					Name:    "auth",
					Title:   "认证登录",
					Enabled: true,
					Sources: []*goswagger.DocumentSource{
						{Service: "demo-api"},
					},
				},
			},
		},
	}

	builder := NewBuilder(
		config,
		map[string]map[string]interface{}{"demo-api": serviceSpec},
		time.Now(),
		logger.NewEmptyLogger(),
	)
	if err := builder.BuildAll(); err != nil {
		t.Fatalf("BuildAll failed: %v", err)
	}
	return builder
}

func getDocumentSpec(t *testing.T, builder *Builder) map[string]interface{} {
	t.Helper()

	data, err := builder.GetDocumentSpec("auth")
	if err != nil {
		t.Fatalf("GetDocumentSpec failed: %v", err)
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("文档规范反序列化失败: %v", err)
	}
	return spec
}

func TestDocumentMergesSecurityDefinitions(t *testing.T) {
	spec := getDocumentSpec(t, newGoctlDocumentBuilder(t))

	definitions, ok := spec[constants.FieldSecurityDefs].(map[string]interface{})
	if !ok {
		t.Fatal("独立文档缺少 securityDefinitions，Authorize 按钮失效")
	}
	bearer, ok := definitions["BearerAuth"].(map[string]interface{})
	if !ok {
		t.Fatal("securityDefinitions 缺少 BearerAuth")
	}
	if bearer["name"] != "Authorization" {
		t.Errorf("BearerAuth.name = %v, want Authorization", bearer["name"])
	}
}

func TestDocumentPassthroughTopLevelXExtensions(t *testing.T) {
	spec := getDocumentSpec(t, newGoctlDocumentBuilder(t))

	if spec["x-date"] != "2026-11-06 13:15:08" {
		t.Errorf("x-date = %v, 顶层工具链元数据未透传", spec["x-date"])
	}
	if spec["x-goctl-version"] != "1.10.1" {
		t.Errorf("x-goctl-version = %v, 顶层工具链元数据未透传", spec["x-goctl-version"])
	}
	if spec["host"] != "api.example.com" {
		t.Errorf("host = %v, want api.example.com", spec["host"])
	}
}

func TestDocumentPreservesOperationFeatures(t *testing.T) {
	spec := getDocumentSpec(t, newGoctlDocumentBuilder(t))

	paths, _ := spec[constants.FieldPaths].(map[string]interface{})
	if len(paths) != 1 {
		t.Fatalf("paths 数量 = %d, want 1", len(paths))
	}
	pathItem, _ := paths["/api/auth/mfa/recovery/verify"].(map[string]interface{})
	op, _ := pathItem["post"].(map[string]interface{})

	if op["x-integration-owner"] != "kamalyes" {
		t.Errorf("x-integration-owner = %v, 联调元数据丢失", op["x-integration-owner"])
	}

	examples, _ := op["x-examples"].(map[string]interface{})
	if len(examples) != 1 {
		t.Errorf("x-examples = %v, 场景示例丢失", op["x-examples"])
	}

	params, _ := op[constants.FieldParameters].([]interface{})
	if len(params) != 2 {
		t.Fatalf("parameters 数量 = %d, want 2", len(params))
	}

	// body 参数
	body, _ := params[0].(map[string]interface{})
	example, _ := body["example"].(map[string]interface{})
	if example == nil || example["code"] != "051924" {
		t.Errorf("body example = %v, 示例值丢失", body["example"])
	}
	schema, _ := body["schema"].(map[string]interface{})
	properties, _ := schema["properties"].(map[string]interface{})
	region, _ := properties["phone_region_code"].(map[string]interface{})
	if region["pattern"] != "^[A-Z]{2}$" {
		t.Errorf("pattern = %v, 约束丢失", region["pattern"])
	}

	// query 参数的 x-example / x-nullable
	page, _ := params[1].(map[string]interface{})
	if page["x-example"] != "1" {
		t.Errorf("x-example = %v, 参数示例丢失", page["x-example"])
	}
	if nullable, _ := page["x-nullable"].(bool); !nullable {
		t.Errorf("x-nullable = %v, 可空标记丢失", page["x-nullable"])
	}
}

func TestDocumentContainsXDocumentInfo(t *testing.T) {
	spec := getDocumentSpec(t, newGoctlDocumentBuilder(t))

	info, ok := spec[constants.FieldXDocumentInfo].(map[string]interface{})
	if !ok {
		t.Fatal("独立文档缺少 x-document-info")
	}
	if info[constants.FieldName] != "auth" {
		t.Errorf("x-document-info.name = %v, want auth", info[constants.FieldName])
	}
}
