/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-22 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-22 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\aggregate\endpoint_test.go
 * @Description: 端点提取功能测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package aggregate

import "testing"

// 测试用 swagger YAML 数据
var testSwaggerYAML = []byte(`swagger: "2.0"
info:
  title: Test Service
  version: "1.0"
paths:
  /api/users:
    get:
      operationId: User_List
      summary: List all users
      tags:
        - user
        - read
    post:
      operationId: User_Create
      summary: Create a user
      tags:
        - user
  /api/users/{id}:
    get:
      operationId: User_Get
      summary: Get user by ID
      tags:
        - user
    delete:
      operationId: User_Delete
      summary: Delete user
  /api/items:
    get:
      summary: List items (no operationId)
      tags:
        - item
`)

var testSwaggerJSON = []byte(`{
  "swagger": "2.0",
  "info": {"title": "Item Service", "version": "1.0"},
  "paths": {
    "/api/items": {
      "get": {
        "operationId": "Item_List",
        "summary": "List items",
        "tags": ["item"]
      }
    }
  }
}`)

func TestExtractEndpointsFromEmbed_Basic(t *testing.T) {
	files := map[string][]byte{
		"proto/user/user_service.swagger.yaml": testSwaggerYAML,
	}

	endpoints, err := ExtractEndpointsFromEmbed(files)
	if err != nil {
		t.Fatalf("ExtractEndpointsFromEmbed failed: %v", err)
	}

	if len(endpoints) != 5 {
		t.Fatalf("expected 5 endpoints, got %d", len(endpoints))
	}

	// 排序后顺序固定：path 字母序 + method 字母序
	// /api/items: get (无 operationId)
	// /api/users: get(User_List), post(User_Create)
	// /api/users/{id}: delete(User_Delete), get(User_Get)
	expected := []struct {
		path        string
		method      string
		operationID string
		summary     string
	}{
		{"/api/items", "GET", "", "List items (no operationId)"},
		{"/api/users", "GET", "User_List", "List all users"},
		{"/api/users", "POST", "User_Create", "Create a user"},
		{"/api/users/{id}", "DELETE", "User_Delete", "Delete user"},
		{"/api/users/{id}", "GET", "User_Get", "Get user by ID"},
	}

	for i, exp := range expected {
		ep := endpoints[i]
		if ep.Path != exp.path {
			t.Errorf("endpoints[%d].Path = %q, want %q", i, ep.Path, exp.path)
		}
		if ep.Method != exp.method {
			t.Errorf("endpoints[%d].Method = %q, want %q", i, ep.Method, exp.method)
		}
		if ep.OperationID != exp.operationID {
			t.Errorf("endpoints[%d].OperationID = %q, want %q", i, ep.OperationID, exp.operationID)
		}
		if ep.Summary != exp.summary {
			t.Errorf("endpoints[%d].Summary = %q, want %q", i, ep.Summary, exp.summary)
		}
	}

	// 验证 User_List 的 tags
	if len(endpoints[1].Tags) != 2 || endpoints[1].Tags[0] != "user" || endpoints[1].Tags[1] != "read" {
		t.Errorf("User_List Tags = %v, want [user read]", endpoints[1].Tags)
	}
}

func TestExtractEndpointsFromEmbed_ServiceName(t *testing.T) {
	files := map[string][]byte{
		"proto/tenant/tenant_service.swagger.yaml": testSwaggerYAML,
	}

	endpoints, err := ExtractEndpointsFromEmbed(files)
	if err != nil {
		t.Fatalf("ExtractEndpointsFromEmbed failed: %v", err)
	}

	if len(endpoints) == 0 {
		t.Fatal("expected at least one endpoint")
	}

	// serviceNameFromFilename: tenant_service -> TenantService
	if endpoints[0].ServiceName != "TenantService" {
		t.Errorf("ServiceName = %q, want TenantService", endpoints[0].ServiceName)
	}
}

func TestExtractEndpointsFromEmbed_SourceFile(t *testing.T) {
	files := map[string][]byte{
		"proto/user/user_service.swagger.yaml": testSwaggerYAML,
	}

	endpoints, err := ExtractEndpointsFromEmbed(files)
	if err != nil {
		t.Fatalf("ExtractEndpointsFromEmbed failed: %v", err)
	}

	if endpoints[0].SourceFile != "proto/user/user_service.swagger.yaml" {
		t.Errorf("SourceFile = %q, want proto/user/user_service.swagger.yaml", endpoints[0].SourceFile)
	}
}

func TestExtractEndpointsFromEmbed_MultipleFiles(t *testing.T) {
	files := map[string][]byte{
		"proto/user/user_service.swagger.yaml": testSwaggerYAML,
		"proto/item/item_service.swagger.json": testSwaggerJSON,
	}

	endpoints, err := ExtractEndpointsFromEmbed(files)
	if err != nil {
		t.Fatalf("ExtractEndpointsFromEmbed failed: %v", err)
	}

	// YAML 有 5 个端点，JSON 有 1 个端点
	if len(endpoints) != 6 {
		t.Fatalf("expected 6 endpoints, got %d", len(endpoints))
	}
}

func TestExtractEndpointsFromEmbed_DedupByOperationID(t *testing.T) {
	// 两个文件有相同的 operationId，应该只保留第一个
	// 注意：/api/items GET 没有 operationId，不会被去重，所以两个文件各产生一个
	files := map[string][]byte{
		"proto/a/a_service.swagger.yaml": testSwaggerYAML,
		"proto/b/b_service.swagger.yaml": testSwaggerYAML, // 完全相同的内容
	}

	endpoints, err := ExtractEndpointsFromEmbed(files)
	if err != nil {
		t.Fatalf("ExtractEndpointsFromEmbed failed: %v", err)
	}

	// 4 个有 operationId 的去重为 4，1 个无 operationId 的各保留 = 4 + 2 = 6
	if len(endpoints) != 6 {
		t.Errorf("expected 6 endpoints (4 deduped + 2 no-operationId), got %d", len(endpoints))
	}

	// 验证有 operationId 的不重复
	opIDCount := make(map[string]int)
	for _, ep := range endpoints {
		if ep.OperationID != "" {
			opIDCount[ep.OperationID]++
		}
	}
	for id, count := range opIDCount {
		if count > 1 {
			t.Errorf("operationId %q appeared %d times, should be deduplicated", id, count)
		}
	}
}

func TestExtractEndpointsFromEmbed_NoOperationID(t *testing.T) {
	// /api/items GET 没有 operationId，仍然应该被提取
	files := map[string][]byte{
		"proto/user/user_service.swagger.yaml": testSwaggerYAML,
	}

	endpoints, err := ExtractEndpointsFromEmbed(files)
	if err != nil {
		t.Fatalf("ExtractEndpointsFromEmbed failed: %v", err)
	}

	// 找到没有 operationId 的端点
	var found bool
	for _, ep := range endpoints {
		if ep.Path == "/api/items" && ep.Method == "GET" && ep.OperationID == "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected endpoint with no operationId to be extracted")
	}
}

func TestExtractEndpointsFromEmbed_EmptyInput(t *testing.T) {
	endpoints, err := ExtractEndpointsFromEmbed(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if endpoints != nil {
		t.Errorf("expected nil, got %v", endpoints)
	}

	endpoints, err = ExtractEndpointsFromEmbed(map[string][]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if endpoints != nil {
		t.Errorf("expected nil, got %v", endpoints)
	}
}

func TestExtractEndpointsFromEmbed_SkipsEmptyData(t *testing.T) {
	files := map[string][]byte{
		"proto/empty/empty_service.swagger.yaml": {},
		"proto/user/user_service.swagger.yaml":   testSwaggerYAML,
	}

	endpoints, err := ExtractEndpointsFromEmbed(files)
	if err != nil {
		t.Fatalf("ExtractEndpointsFromEmbed failed: %v", err)
	}
	if len(endpoints) != 5 {
		t.Errorf("expected 5 endpoints (skipping empty file), got %d", len(endpoints))
	}
}

func TestExtractEndpointsFromEmbed_InvalidData(t *testing.T) {
	files := map[string][]byte{
		"proto/bad/bad_service.swagger.yaml": []byte("{{invalid"),
	}

	_, err := ExtractEndpointsFromEmbed(files)
	if err == nil {
		t.Error("expected error for invalid swagger data")
	}
}

func TestExtractEndpointsFromEmbed_IgnoresNonOperationMethods(t *testing.T) {
	yamlWithParameters := []byte(`swagger: "2.0"
info:
  title: Test
  version: "1.0"
paths:
  /api/test:
    parameters:
      - name: id
        in: query
    get:
      operationId: Test_Get
      summary: Get test
`)

	files := map[string][]byte{
		"proto/test/test.swagger.yaml": yamlWithParameters,
	}

	endpoints, err := ExtractEndpointsFromEmbed(files)
	if err != nil {
		t.Fatalf("ExtractEndpointsFromEmbed failed: %v", err)
	}

	// 只有 GET 方法应该被提取，parameters 不是操作方法
	if len(endpoints) != 1 {
		t.Errorf("expected 1 endpoint, got %d", len(endpoints))
	}
	if endpoints[0].OperationID != "Test_Get" {
		t.Errorf("OperationID = %q, want Test_Get", endpoints[0].OperationID)
	}
}

// ==================== 辅助函数测试 ====================

func TestServiceNameFromFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"proto/tenant/tenant_service.swagger.yaml", "TenantService"},
		{"proto/access_control/user_service.swagger.yaml", "UserService"},
		{"proto/game/game.swagger.yaml", "Game"},
		{"proto/system/system_service.swagger.json", "SystemService"},
		{"simple.yaml", "Simple"},
		{"multi_word_name.swagger.yaml", "MultiWordName"},
		{"path\\with\\backslash_service.swagger.yaml", "BackslashService"},
	}

	for _, tt := range tests {
		result := serviceNameFromFilename(tt.input)
		if result != tt.expected {
			t.Errorf("serviceNameFromFilename(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFilepathToSlash(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"path/to/file", "path/to/file"},
		{"path\\to\\file", "path/to/file"},
		{"mixed/path\\file", "mixed/path/file"},
		{"nopath", "nopath"},
	}

	for _, tt := range tests {
		result := filepathToSlash(tt.input)
		if result != tt.expected {
			t.Errorf("filepathToSlash(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestExtractStringSlice(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected []string
	}{
		{nil, nil},
		{[]string{"a", "b"}, []string{"a", "b"}},
		{[]interface{}{"a", "b", "c"}, []string{"a", "b", "c"}},
		{[]interface{}{"a", "", "c"}, []string{"a", "c"}},
		{"single", []string{"single"}},
		{"", nil},
	}

	for _, tt := range tests {
		result := extractStringSlice(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("extractStringSlice(%v) = %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("extractStringSlice(%v)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}
