/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\aggregate\merge_test.go
 * @Description: goctl 风格规范的聚合保真测试 - securityDefinitions、顶层 x- 扩展、参数约束
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package aggregate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	goswagger "github.com/kamalyes/go-rpc-gateway/config/swagger"
	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	"github.com/kamalyes/go-toolbox/pkg/logger"
)

// goctlStyleSpec 模拟 goctl（go-zero）生成风格的服务规范：
// 全 inline schema（无 definitions）、BearerAuth securityDefinitions、
// 顶层 x- 工具链元数据、操作级 x-integration-* 联调元数据、
// body 参数 example、字段 required/pattern/minLength/maxLength 约束
const goctlStyleSpec = `{
  "swagger": "2.0",
  "info": {"title": "demo-api", "version": "1.0"},
  "schemes": ["https"],
  "host": "api.example.com",
  "basePath": "",
  "securityDefinitions": {
    "BearerAuth": {"type": "apiKey", "name": "Authorization", "in": "header"}
  },
  "x-date": "2026-11-06 13:15:08",
  "x-goctl-version": "1.10.1",
  "tags": [{"name": "认证登录", "description": "注册、登录、验证码、token 刷新、登出和密码管理。"}],
  "paths": {
    "/api/auth/mfa/recovery/verify": {
      "post": {
        "summary": "校验 MFA 找回验证码",
        "operationId": "verifyMfaRecoveryCode",
        "tags": ["认证登录"],
        "security": [{"BearerAuth": []}],
        "x-integration-page": "auth/recovery",
        "x-integration-status": "stable",
        "x-integration-owner": "kamalyes",
        "x-integration-module": "auth",
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {
              "type": "object",
              "required": ["code", "identity"],
              "properties": {
                "code": {"type": "string", "description": "6 位 MFA 找回验证码"},
                "identity": {"type": "string", "minLength": 1, "maxLength": 128},
                "phone_region_code": {"type": "string", "pattern": "^[A-Z]{2}$"}
              }
            },
            "example": {"identity": "user@example.com", "code": "051924"}
          }
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`

// newGoctlTestAggregator 从临时文件加载 goctl 风格规范并执行 merge 聚合
func newGoctlTestAggregator(t *testing.T) *Aggregator {
	t.Helper()

	dir := t.TempDir()
	specPath := filepath.Join(dir, "demo-api.json")
	if err := os.WriteFile(specPath, []byte(goctlStyleSpec), 0o600); err != nil {
		t.Fatalf("写入测试规范失败: %v", err)
	}

	config := &goswagger.Swagger{
		Aggregate: &goswagger.AggregateConfig{
			Enabled: true,
			Mode:    constants.AggregateModeMerge,
			Services: []*goswagger.ServiceSpec{
				{Name: "demo-api", SpecPath: specPath, Enabled: true},
			},
		},
	}

	aggregator := NewAggregator(config, logger.NewEmptyLogger())
	if err := aggregator.LoadAll(); err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	return aggregator
}

func getOperation(t *testing.T, spec map[string]interface{}, path, method string) map[string]interface{} {
	t.Helper()

	paths, ok := spec[constants.FieldPaths].(map[string]interface{})
	if !ok {
		t.Fatal("聚合规范缺少 paths")
	}
	pathItem, ok := paths[path].(map[string]interface{})
	if !ok {
		t.Fatalf("聚合规范缺少路径 %s", path)
	}
	op, ok := pathItem[method].(map[string]interface{})
	if !ok {
		t.Fatalf("聚合规范路径 %s 缺少 %s 操作", path, method)
	}
	return op
}

func TestMergeTopLevelSecurityDefinitions(t *testing.T) {
	spec := newGoctlTestAggregator(t).GetAggregatedSpecMap()

	definitions, ok := spec[constants.FieldSecurityDefs].(map[string]interface{})
	if !ok {
		t.Fatal("聚合规范缺少 securityDefinitions，merge 模式下 BearerAuth 会全部悬空")
	}
	bearer, ok := definitions["BearerAuth"].(map[string]interface{})
	if !ok {
		t.Fatal("securityDefinitions 缺少 BearerAuth")
	}
	if bearer["name"] != "Authorization" || bearer["in"] != "header" {
		t.Errorf("BearerAuth = %v, want name=Authorization in=header", bearer)
	}
}

func TestMergeTopLevelSchemesHostAndXExtensions(t *testing.T) {
	spec := newGoctlTestAggregator(t).GetAggregatedSpecMap()

	if spec["host"] != "api.example.com" {
		t.Errorf("host = %v, want api.example.com", spec["host"])
	}
	if spec["x-date"] != "2026-11-06 13:15:08" {
		t.Errorf("x-date = %v, 顶层工具链元数据丢失", spec["x-date"])
	}
	if spec["x-goctl-version"] != "1.10.1" {
		t.Errorf("x-goctl-version = %v, 顶层工具链元数据丢失", spec["x-goctl-version"])
	}

	schemes := toStringSlice(spec[constants.FieldSchemes])
	if len(schemes) != 1 || schemes[0] != "https" {
		t.Errorf("schemes = %v, want [https]", schemes)
	}
}

func TestMergePreservesGoctlOperationFeatures(t *testing.T) {
	spec := newGoctlTestAggregator(t).GetAggregatedSpecMap()
	op := getOperation(t, spec, "/api/auth/mfa/recovery/verify", "post")

	// 操作级联调元数据
	if op["x-integration-owner"] != "kamalyes" {
		t.Errorf("x-integration-owner = %v, 联调元数据丢失", op["x-integration-owner"])
	}
	if op["x-integration-status"] != "stable" {
		t.Errorf("x-integration-status = %v, 联调元数据丢失", op["x-integration-status"])
	}

	// 操作级 security
	security, _ := op[constants.FieldSecurity].([]interface{})
	if len(security) == 0 {
		t.Error("操作级 security 丢失，Swagger UI Authorize 失效")
	}

	// body 参数：example、required、pattern、minLength
	params, _ := op[constants.FieldParameters].([]interface{})
	if len(params) != 1 {
		t.Fatalf("parameters 数量 = %d, want 1", len(params))
	}
	body, _ := params[0].(map[string]interface{})

	example, _ := body["example"].(map[string]interface{})
	if example == nil || example["code"] != "051924" {
		t.Errorf("body example = %v, 示例值丢失", body["example"])
	}

	schema, _ := body["schema"].(map[string]interface{})
	properties, _ := schema["properties"].(map[string]interface{})
	identity, _ := properties["identity"].(map[string]interface{})
	if identity["minLength"] != float64(1) || identity["maxLength"] != float64(128) {
		t.Errorf("identity 约束 = minLength:%v maxLength:%v, 约束丢失", identity["minLength"], identity["maxLength"])
	}
	region, _ := properties["phone_region_code"].(map[string]interface{})
	if region["pattern"] != "^[A-Z]{2}$" {
		t.Errorf("phone_region_code pattern = %v, 约束丢失", region["pattern"])
	}

	required, _ := schema["required"].([]interface{})
	if len(required) != 2 {
		t.Errorf("required = %v, want [code identity]", required)
	}
}

func TestMergePreservesTagsWithDescription(t *testing.T) {
	spec := newGoctlTestAggregator(t).GetAggregatedSpecMap()

	tags, _ := spec[constants.FieldTags].([]interface{})
	if len(tags) == 0 {
		t.Fatal("聚合规范缺少 tags")
	}
	tag, _ := tags[0].(map[string]interface{})
	if tag[constants.FieldName] != "认证登录" {
		t.Errorf("tag name = %v, want 认证登录", tag[constants.FieldName])
	}
	if tag[constants.FieldDescription] == "" {
		t.Error("tag description 丢失")
	}
}

// TestGoctlStyleSpecRoundTrip 验证聚合结果可正常序列化（无非法类型）
func TestGoctlStyleSpecRoundTrip(t *testing.T) {
	data, err := newGoctlTestAggregator(t).GetAggregatedSpec()
	if err != nil {
		t.Fatalf("GetAggregatedSpec failed: %v", err)
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("聚合结果无法反序列化: %v", err)
	}
	if spec[constants.FieldSecurityDefs] == nil {
		t.Error("序列化后的聚合规范缺少 securityDefinitions")
	}
}
