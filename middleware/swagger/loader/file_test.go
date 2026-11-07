/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-22 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-22 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\loader\file_test.go
 * @Description: LoadSpecFromBytes 函数测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package loader

import "testing"

const testYAML = `swagger: "2.0"
info:
  title: Test API
  version: "1.0"
paths:
  /api/users:
    get:
      operationId: User_List
      summary: List users
      tags:
        - user
    post:
      operationId: User_Create
      summary: Create user
  /api/users/{id}:
    delete:
      operationId: User_Delete
      summary: Delete user
`

const testJSON = `{
  "swagger": "2.0",
  "info": {
    "title": "Test API",
    "version": "1.0"
  },
  "paths": {
    "/api/items": {
      "get": {
        "operationId": "Item_List",
        "summary": "List items"
      }
    }
  }
}`

func TestLoadSpecFromBytes_YAML(t *testing.T) {
	spec, err := LoadSpecFromBytes("test.swagger.yaml", []byte(testYAML))
	if err != nil {
		t.Fatalf("LoadSpecFromBytes yaml failed: %v", err)
	}

	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("spec has no paths")
	}
	if _, exists := paths["/api/users"]; !exists {
		t.Error("missing path /api/users")
	}
	if _, exists := paths["/api/users/{id}"]; !exists {
		t.Error("missing path /api/users/{id}")
	}
}

func TestLoadSpecFromBytes_JSON(t *testing.T) {
	spec, err := LoadSpecFromBytes("test.swagger.json", []byte(testJSON))
	if err != nil {
		t.Fatalf("LoadSpecFromBytes json failed: %v", err)
	}

	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("spec has no paths")
	}
	if _, exists := paths["/api/items"]; !exists {
		t.Error("missing path /api/items")
	}
}

func TestLoadSpecFromBytes_UnknownExt_FallbackToJSONThenYAML(t *testing.T) {
	// 无扩展名时先尝试 JSON，失败再尝试 YAML
	spec, err := LoadSpecFromBytes("test", []byte(testYAML))
	if err != nil {
		t.Fatalf("LoadSpecFromBytes with unknown ext failed: %v", err)
	}
	if _, ok := spec["paths"]; !ok {
		t.Error("spec has no paths")
	}
}

func TestLoadSpecFromBytes_EmptyData(t *testing.T) {
	_, err := LoadSpecFromBytes("test.yaml", []byte{})
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestLoadSpecFromBytes_InvalidYAML(t *testing.T) {
	_, err := LoadSpecFromBytes("test.yaml", []byte("{{invalid yaml"))
	if err == nil {
		t.Error("expected error for invalid yaml")
	}
}

func TestLoadSpecFromBytes_InvalidJSON(t *testing.T) {
	_, err := LoadSpecFromBytes("test.json", []byte("{invalid json"))
	if err == nil {
		t.Error("expected error for invalid json")
	}
}
