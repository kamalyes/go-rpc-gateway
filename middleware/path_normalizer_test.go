/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-02-27 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-02-27 15:30:00
 * @FilePath: \go-rpc-gateway\middleware\path_normalizer_test.go
 * @Description: 智能路径规范化器测试
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSmartPathNormalizer_Learning 测试学习功能
func TestSmartPathNormalizer_Learning(t *testing.T) {
	type step struct {
		input    string
		expected string
		reason   string
	}

	tests := map[string][]step{
		"单段动态参数": {
			{"/api/resource/123", "/api/resource/123", "首次访问"},
			{"/api/resource/567", "/api/resource/:param", "触发动态"},
			{"/api/resource/789", "/api/resource/:param", "已动态"},
		},
		"多段动态参数": {
			{"/api/resource/123/items/567", "/api/resource/123/items/567", "首次访问"},
			{"/api/resource/789/items/567", "/api/resource/:param/items/567", "位置2动态"},
			{"/api/resource/123/items/789", "/api/resource/:param/items/:param", "位置4动态"},
		},
		"嵌套资源路径": {
			{"/v1/containers/container-a/data", "/v1/containers/container-a/data", "首次访问"},
			{"/v1/containers/container-b/data", "/v1/containers/:param/data", "位置2动态"},
			{"/v1/containers/container-c/data/file.txt", "/v1/containers/:param/data/file.txt", "扩展路径"},
			{"/v1/containers/container-d/data/photo.jpg", "/v1/containers/:param/data/:param", "位置4动态"},
		},
	}

	for name, steps := range tests {
		t.Run(name, func(t *testing.T) {
			n := newSmartPathNormalizer()
			for i, s := range steps {
				result := n.Normalize(s.input)
				assert.Equal(t, s.expected, result, "步骤%d[%s]: %s", i+1, s.reason, s.input)
			}
		})
	}
}

// TestSmartPathNormalizer_StaticAndQuery 测试静态路径和查询参数（合并）
func TestSmartPathNormalizer_StaticAndQuery(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		// 静态路径
		{"根路径", "/", "/"},
		{"健康检查", "/health", "/health"},
		{"指标", "/metrics", "/metrics"},
		{"就绪检查", "/ready", "/ready"},

		// 查询参数处理
		{"移除查询参数", "/api/users?page=1&limit=10", "/api/users"},
		{"带动态参数和查询", "/api/user/123?include=posts", "/api/user/123"},

		// 边界情况
		{"空段-双斜杠", "/api//user", "/api//user"},
		{"尾部斜杠", "/api/user/", "/api/user/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := newSmartPathNormalizer() // 每个测试用例独立实例
			result := n.Normalize(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSmartPathNormalizer_Cache 测试缓存和LRU淘汰（合并）
func TestSmartPathNormalizer_Cache(t *testing.T) {
	t.Run("缓存命中", func(t *testing.T) {
		n := newSmartPathNormalizer()
		path := "/api/user/123"

		// 第一次访问
		n.Normalize(path)
		_, exists := n.cache[path]
		assert.True(t, exists, "路径应该被缓存")

		// 第二次访问（缓存命中）
		result := n.Normalize(path)
		assert.Equal(t, "/api/user/123", result)
	})

	t.Run("LRU淘汰", func(t *testing.T) {
		n := newSmartPathNormalizer()
		n.maxCache = 3

		// 填满缓存
		n.Normalize("/api/test/a")
		n.Normalize("/api/test/b")
		n.Normalize("/api/test/c")
		assert.Equal(t, 3, len(n.cache))

		// 触发淘汰
		n.Normalize("/api/test/d")
		assert.LessOrEqual(t, len(n.cache), 3)
	})
}

// TestSmartPathNormalizer_MaxValues 测试值数量限制
func TestSmartPathNormalizer_MaxValues(t *testing.T) {
	n := newSmartPathNormalizer()

	steps := []struct {
		input    string
		expected string
		reason   string
	}{
		{"/api/user/1", "/api/user/1", "第1个值，记录"},
		{"/api/user/2", "/api/user/:param", "第2个值，触发动态"},
		{"/api/user/3", "/api/user/:param", "已标记为动态"},
		{"/api/user/4", "/api/user/:param", "已标记为动态"},
		{"/api/user/5", "/api/user/:param", "已标记为动态"},
		{"/api/user/6", "/api/user/:param", "已标记为动态"},
	}

	for i, step := range steps {
		result := n.Normalize(step.input)
		assert.Equal(t, step.expected, result, "步骤 %d [%s]: 输入=%s", i+1, step.reason, step.input)
	}
}

// TestSmartPathNormalizer_DifferentPrefixes 测试不同前缀独立学习
func TestSmartPathNormalizer_DifferentPrefixes(t *testing.T) {
	n := newSmartPathNormalizer()

	steps := []struct {
		input    string
		expected string
		reason   string
	}{
		{"/api/v1/users/123", "/api/v1/users/123", "v1首次访问"},
		{"/api/v1/users/567", "/api/v1/users/:param", "v1位置3触发动态"},
		{"/api/v2/users/789", "/api/:param/users/789", "v2出现，位置1触发动态"},
		{"/api/v2/users/101", "/api/:param/users/:param", "位置3触发动态"},
	}

	for i, step := range steps {
		result := n.Normalize(step.input)
		assert.Equal(t, step.expected, result, "步骤 %d [%s]: 输入=%s", i+1, step.reason, step.input)
	}
}

// TestSmartPathNormalizer_ConcurrentAccess 测试并发安全
func TestSmartPathNormalizer_ConcurrentAccess(t *testing.T) {
	n := newSmartPathNormalizer()
	n.Normalize("/api/resource/1")
	n.Normalize("/api/resource/2")

	done := make(chan bool, 10)
	for i := range 10 {
		go func(id int) {
			defer func() { done <- true }()
			path := "/api/resource/" + string(rune('a'+id))
			result := n.Normalize(path)
			assert.NotEmpty(t, result)
		}(i)
	}

	for range 10 {
		<-done
	}
}

// TestSmartPathNormalizer_SingletonScenario 测试单例模式（所有路径共享学习状态）
func TestSmartPathNormalizer_SingletonScenario(t *testing.T) {
	n := newSmartPathNormalizer()

	steps := []struct {
		input    string
		expected string
		reason   string
	}{
		// 第一组：items 路由
		{"/api/items/item-001/status", "/api/items/item-001/status", "items首次访问"},
		{"/api/items/item-002/status", "/api/items/:param/status", "items位置2触发动态"},

		// 第二组：entities 路由（触发 /api 位置1 变动态）
		{"/api/entities/entity-001", "/api/:param/entity-001", "entities出现，位置1触发动态"},
		{"/api/entities/entity-002", "/api/:param/:param", "位置2触发动态"},

		// 第三组：collections 路由（位置1已动态）
		{"/api/collections/coll-001/members/mem-001/perms", "/api/:param/:param/members/mem-001/perms", "collections出现，位置1已动态"},
		{"/api/collections/coll-002/members/mem-002/perms", "/api/:param/:param/members/:param/perms", "位置4触发动态"},

		// 第四组：repos 路由
		{"/api/repos/repo-a/files", "/api/:param/:param/:param", "repos出现，位置1-2已动态"},
		{"/api/repos/repo-c/files/doc.txt", "/api/:param/:param/:param/doc.txt", "扩展路径"},
		{"/api/repos/repo-d/files/img.jpg", "/api/:param/:param/:param/:param", "位置4触发动态"},
	}

	for i, step := range steps {
		result := n.Normalize(step.input)
		assert.Equal(t, step.expected, result, "步骤 %d [%s]: 输入=%s", i+1, step.reason, step.input)
	}
}

// BenchmarkSmartPathNormalizer_Normalize 性能基准测试
func BenchmarkSmartPathNormalizer_Normalize(b *testing.B) {
	n := newSmartPathNormalizer()
	n.Normalize("/api/user/1")
	n.Normalize("/api/user/2")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n.Normalize("/api/user/123")
	}
}

// BenchmarkSmartPathNormalizer_CacheHit 缓存命中性能测试
func BenchmarkSmartPathNormalizer_CacheHit(b *testing.B) {
	n := newSmartPathNormalizer()
	path := "/api/user/123"
	n.Normalize("/api/user/1")
	n.Normalize("/api/user/2")
	n.Normalize(path)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n.Normalize(path)
	}
}

// BenchmarkSmartPathNormalizer_NewPath 新路径性能测试
func BenchmarkSmartPathNormalizer_NewPath(b *testing.B) {
	n := newSmartPathNormalizer()
	n.Normalize("/api/user/1")
	n.Normalize("/api/user/2")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := "/api/user/" + string(rune('a'+(i%26)))
		n.Normalize(path)
	}
}
