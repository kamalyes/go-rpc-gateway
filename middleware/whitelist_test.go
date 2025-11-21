/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-21 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-21 16:29:23
 * @FilePath: \engine-im-service\go-rpc-gateway\middleware\whitelist_test.go
 * @Description: 白名单中间件测试
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kamalyes/go-toolbox/pkg/netx"

	"github.com/stretchr/testify/assert"
)

// TestGetClientIP 测试客户端 IP 提取
func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name         string
		setupRequest func() *http.Request
		expectedIP   string
		description  string
	}{
		{
			name: "X-Forwarded-For单个IP",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-For", "192.168.1.100")
				return req
			},
			expectedIP:  "192.168.1.100",
			description: "应该从 X-Forwarded-For 提取第一个IP",
		},
		{
			name: "X-Forwarded-For多个IP",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.1, 172.16.0.1")
				return req
			},
			expectedIP:  "192.168.1.100",
			description: "应该从 X-Forwarded-For 提取第一个IP（客户端真实IP）",
		},
		{
			name: "X-Real-IP",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Real-IP", "203.0.113.1")
				return req
			},
			expectedIP:  "203.0.113.1",
			description: "应该从 X-Real-IP 提取IP",
		},
		{
			name: "X-Forwarded-For优先级高于X-Real-IP",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-For", "192.168.1.100")
				req.Header.Set("X-Real-IP", "10.0.0.1")
				return req
			},
			expectedIP:  "192.168.1.100",
			description: "X-Forwarded-For 优先级应该高于 X-Real-IP",
		},
		{
			name: "RemoteAddr",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "198.51.100.1:54321"
				return req
			},
			expectedIP:  "198.51.100.1",
			description: "应该从 RemoteAddr 提取IP（去除端口）",
		},
		{
			name: "RemoteAddr仅IP",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "198.51.100.2"
				return req
			},
			expectedIP:  "198.51.100.2",
			description: "应该处理没有端口的 RemoteAddr",
		},
		{
			name: "IPv6地址",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Forwarded-For", "2001:db8::1")
				return req
			},
			expectedIP:  "2001:db8::1",
			description: "应该支持 IPv6 地址",
		},
		{
			name: "IPv6地址带端口",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "[2001:db8::1]:8080"
				return req
			},
			expectedIP:  "2001:db8::1",
			description: "应该从 IPv6 地址中提取IP（去除端口）",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			ip := netx.GetClientIP(req)
			assert.Equal(t, tt.expectedIP, ip, tt.description)
		})
	}
}

// TestPathPrefixRule 测试路径前缀规则
func TestPathPrefixRule(t *testing.T) {
	rule := &PathPrefixRule{
		prefix:      "/api/",
		description: "API 路径",
		priority:    100,
	}

	tests := []struct {
		method   string
		path     string
		expected bool
	}{
		{"GET", "/api/users", true},
		{"POST", "/api/orders", true},
		{"GET", "/api/", true},
		{"GET", "/app/users", false},
		{"GET", "/api", false}, // 不匹配，缺少尾部斜杠
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := rule.Match(tt.method, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExactPathRule 测试精确路径规则
func TestExactPathRule(t *testing.T) {
	rule := &ExactPathRule{
		method:      "POST",
		path:        "/v1/install",
		description: "安装接口",
		priority:    50,
	}

	tests := []struct {
		method   string
		path     string
		expected bool
	}{
		{"POST", "/v1/install", true},
		{"POST", "/V1/INSTALL", true}, // 大小写不敏感
		{"GET", "/v1/install", false},
		{"POST", "/v1/install/", false},
		{"POST", "/v2/install", false},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			result := rule.Match(tt.method, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIPRule 测试 IP 规则
func TestIPRule(t *testing.T) {
	rule := &IPRule{
		allowedIPs:  []string{"192.168.1.100", "10.0.0.1"},
		description: "允许的IP列表",
		priority:    10,
	}

	tests := []struct {
		name     string
		clientIP string
		expected bool
	}{
		{"允许的IP1", "192.168.1.100", true},
		{"允许的IP2", "10.0.0.1", true},
		{"不允许的IP", "192.168.1.101", false},
		{"不允许的IP2", "203.0.113.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.MatchWithIP(tt.clientIP)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCIDRRule 测试 CIDR 规则
func TestCIDRRule(t *testing.T) {
	_, cidrNet, _ := net.ParseCIDR("192.168.1.0/24")
	rule := &CIDRRule{
		allowedNets: []*net.IPNet{cidrNet},
		description: "内网IP段",
		priority:    10,
	}

	tests := []struct {
		name     string
		clientIP string
		expected bool
	}{
		{"网段内IP1", "192.168.1.1", true},
		{"网段内IP2", "192.168.1.100", true},
		{"网段内IP3", "192.168.1.254", true},
		{"网段外IP", "192.168.2.1", false},
		{"完全不同IP", "10.0.0.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rule.MatchWithIP(tt.clientIP)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestWhitelistManager 测试白名单管理器
func TestWhitelistManager(t *testing.T) {
	manager := NewWhitelistManager()

	// 添加规则
	NewRuleBuilder(manager).
		AddPathPrefix("/api/", "API").
		AddExactPath("POST", "/v1/install", "安装").
		AddPathPrefix("/health", "健康检查").
		Build()

	tests := []struct {
		method   string
		path     string
		expected bool
	}{
		{"GET", "/api/users", true},
		{"POST", "/api/orders", true},
		{"POST", "/v1/install", true},
		{"GET", "/v1/install", false},
		{"GET", "/health", true},
		{"GET", "/healthz", false},
		{"GET", "/app/users", false},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			result := manager.IsWhitelisted(tt.method, tt.path)
			assert.Equal(t, tt.expected, result, "路径: %s %s", tt.method, tt.path)
		})
	}
}

// TestWhitelistManagerWithIP 测试带 IP 的白名单管理器
func TestWhitelistManagerWithIP(t *testing.T) {
	manager := NewWhitelistManager()

	// 添加规则
	NewRuleBuilder(manager).
		AddPathPrefix("/public/", "公开资源").
		AddIP([]string{"192.168.1.100"}, "特定IP").
		AddCIDR([]string{"10.0.0.0/8"}, "内网").
		Build()

	tests := []struct {
		method   string
		path     string
		clientIP string
		expected bool
	}{
		{"GET", "/public/image.png", "any", true},      // 路径匹配，不看IP
		{"GET", "/admin/users", "192.168.1.100", true}, // IP匹配
		{"GET", "/admin/users", "10.0.0.5", true},      // CIDR匹配
		{"GET", "/admin/users", "203.0.113.1", false},  // IP不匹配
		{"GET", "/public/data", "203.0.113.1", true},   // 路径匹配
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path+" from "+tt.clientIP, func(t *testing.T) {
			result := manager.IsWhitelistedWithIP(tt.method, tt.path, tt.clientIP)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRulePriority 测试规则优先级
func TestRulePriority(t *testing.T) {
	manager := NewWhitelistManager()

	// 添加不同优先级的规则
	manager.Register(&ExactPathRule{
		method:      "GET",
		path:        "/test",
		description: "精确匹配",
		priority:    1, // 高优先级
	})

	manager.Register(&PathPrefixRule{
		prefix:      "/test",
		description: "前缀匹配",
		priority:    100, // 低优先级
	})

	// 验证规则按优先级排序
	rules := manager.GetRules()
	assert.Equal(t, 2, len(rules))
	assert.Equal(t, 1, rules[0].Priority())
	assert.Equal(t, 100, rules[1].Priority())
}

// BenchmarkGetClientIP 性能测试
func BenchmarkGetClientIP(b *testing.B) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		netx.GetClientIP(req)
	}
}

// BenchmarkWhitelistMatch 性能测试
func BenchmarkWhitelistMatch(b *testing.B) {
	manager := NewWhitelistManager()
	NewRuleBuilder(manager).
		AddPathPrefix("/api/", "API").
		AddPathPrefix("/public/", "公开").
		AddExactPath("POST", "/v1/install", "安装").
		Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.IsWhitelisted("GET", "/api/users")
	}
}
