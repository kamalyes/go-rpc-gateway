/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-05-22 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-05-22 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\constants\http_test.go
 * @Description: HTTP 方法标准化函数测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package constants

import (
	"net/http"
	"testing"
)

func TestNormalizeHTTPMethod(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"get", http.MethodGet},
		{"GET", http.MethodGet},
		{"Get", http.MethodGet},
		{"post", http.MethodPost},
		{"PUT", http.MethodPut},
		{"delete", http.MethodDelete},
		{"patch", http.MethodPatch},
		{"head", http.MethodHead},
		{"options", http.MethodOptions},
		{"trace", http.MethodTrace},
		{"connect", http.MethodConnect},
		{"", ""},
		{"invalid", ""},
		{"GETX", ""},
	}

	for _, tt := range tests {
		result := NormalizeHTTPMethod(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeHTTPMethod(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestIsValidHTTPMethod(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"GET", true},
		{"get", true},
		{"Post", true},
		{"", false},
		{"INVALID", false},
		{"getx", false},
	}

	for _, tt := range tests {
		result := IsValidHTTPMethod(tt.input)
		if result != tt.expected {
			t.Errorf("IsValidHTTPMethod(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestValidHTTPMethodsContainsAllMethods(t *testing.T) {
	expected := []string{
		http.MethodGet,
		http.MethodPut,
		http.MethodPost,
		http.MethodDelete,
		http.MethodOptions,
		http.MethodHead,
		http.MethodPatch,
		http.MethodTrace,
		http.MethodConnect,
	}
	if len(ValidHTTPMethods) != len(expected) {
		t.Fatalf("ValidHTTPMethods has %d entries, want %d", len(ValidHTTPMethods), len(expected))
	}

	methodSet := make(map[string]bool, len(ValidHTTPMethods))
	for _, m := range ValidHTTPMethods {
		methodSet[m] = true
	}

	for _, m := range expected {
		if !methodSet[m] {
			t.Errorf("ValidHTTPMethods missing %q", m)
		}
	}
}
