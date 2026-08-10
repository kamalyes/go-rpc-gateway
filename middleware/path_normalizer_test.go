/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-02-27 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-02-27 15:30:00
 * @FilePath: \go-rpc-gateway\middleware\path_normalizer_test.go
 * @Description: 路径规范化器测试（内容特征检测 + 前缀学习）
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"testing"

	"github.com/kamalyes/go-config/pkg/monitoring"

	"github.com/stretchr/testify/assert"
)

// TestSmartPathNormalizer_ContentDetection 测试内容特征检测（无状态，立即生效）
// 每个用例使用独立 normalizer，无学习干扰
func TestSmartPathNormalizer_ContentDetection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// 纯数字 → 动态
		{"纯数字", "/api/resource/123", "/api/resource/:param"},
		{"长数字", "/api/resource/99999", "/api/resource/:param"},

		// UUID → 动态
		{"UUID", "/api/resource/550e8400-e29b-41d4-a716-446655440000", "/api/resource/:param"},

		// 字母+数字混合 → 动态
		{"混合字母数字", "/api/resource/abc123", "/api/resource/:param"},
		{"字母+数字+连字符", "/api/resource/user-001", "/api/resource/:param"},

		// 纯字母 → 保留（首次访问，无学习）
		{"纯字母资源名", "/api/users/list", "/api/users/list"},
		{"纯字母角色名", "/api/roles/admin", "/api/roles/admin"},

		// 版本前缀 → 保留
		{"版本前缀v1", "/v1/users/123", "/v1/users/:param"},
		{"版本前缀v2", "/v2/roles/456", "/v2/roles/:param"},
		{"版本前缀v10", "/v10/policies/789", "/v10/policies/:param"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := newSmartPathNormalizer(monitoring.DefaultStaticPaths())
			result := n.Normalize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSmartPathNormalizer_PrefixLearning 测试前缀学习（有状态，逐段学习）
// 验证注释中描述的 bucket 学习流程
func TestSmartPathNormalizer_PrefixLearning(t *testing.T) {
	n := newSmartPathNormalizer(monitoring.DefaultStaticPaths())

	steps := []struct {
		input    string
		expected string
		reason   string
	}{
		// 步骤1: 首次访问 my-bucket，纯字母 → 学习记录，保留
		{"/v1/buckets/my-bucket/objects", "/v1/buckets/my-bucket/objects",
			"首次: my-bucket 纯字母→记录，buckets/objects 唯一→保留"},

		// 步骤2: 第二个 bucket 名 → 触发动态
		{"/v1/buckets/your-bucket/objects", "/v1/buckets/:param/objects",
			"学习: your-bucket 是第2个值→位置2标记动态"},

		// 步骤3: 已知动态，直接替换
		{"/v1/buckets/another-bucket/objects", "/v1/buckets/:param/objects",
			"已知动态: 位置2已标记→直接:param"},

		// 步骤4: 深层路径，位置4首次记录
		{"/v1/buckets/my-bucket/objects/file.txt", "/v1/buckets/:param/objects/file.txt",
			"深层: 位置2已动态→:param, 位置4 file.txt 首次→记录"},

		// 步骤5: 位置4第二个值 → 触发动态
		{"/v1/buckets/my-bucket/objects/photo.jpg", "/v1/buckets/:param/objects/:param",
			"深层学习: photo.jpg 是位置4第2个值→标记动态"},
	}

	for i, s := range steps {
		result := n.Normalize(s.input)
		assert.Equal(t, s.expected, result, "步骤%d [%s]: 输入=%s", i+1, s.reason, s.input)
	}
}

// TestSmartPathNormalizer_ResourceCollapse 测试多资源名在同一位置的学习坍缩
// 不同资源名出现在同一位置时，学习会将该位置标记为动态
func TestSmartPathNormalizer_ResourceCollapse(t *testing.T) {
	n := newSmartPathNormalizer(monitoring.DefaultStaticPaths())

	steps := []struct {
		input    string
		expected string
		reason   string
	}{
		// 第一个资源名 "users" → 保留
		{"/v1/users/123", "/v1/users/:param",
			"users 首次→保留, 123 数字→:param"},

		// 第二个资源名 "roles" → 位置1标记动态
		{"/v1/roles/456", "/v1/:param/:param",
			"roles 是位置1第2个值→动态, 456 数字→:param"},

		// 第三个资源名 "policies" → 已知动态
		{"/v1/policies/789", "/v1/:param/:param",
			"位置1已知动态→:param, 789 数字→:param"},
	}

	for i, s := range steps {
		result := n.Normalize(s.input)
		assert.Equal(t, s.expected, result, "步骤%d [%s]: 输入=%s", i+1, s.reason, s.input)
	}
}

// TestSmartPathNormalizer_StaticAndQuery 测试静态路径和查询参数
func TestSmartPathNormalizer_StaticAndQuery(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"根路径", "/", "/"},
		{"健康检查", "/health", "/health"},
		{"指标", "/metrics", "/metrics"},
		{"就绪检查", "/ready", "/ready"},
		{"移除查询参数", "/api/users?page=1&limit=10", "/api/users"},
		{"带动态参数和查询", "/api/user/123?include=posts", "/api/user/:param"},
		{"空段-双斜杠", "/api//user", "/api//user"},
		{"尾部斜杠", "/api/user/", "/api/user/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := newSmartPathNormalizer(monitoring.DefaultStaticPaths())
			result := n.Normalize(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSmartPathNormalizer_Cache 测试缓存和LRU淘汰
func TestSmartPathNormalizer_Cache(t *testing.T) {
	t.Run("缓存命中", func(t *testing.T) {
		n := newSmartPathNormalizer(monitoring.DefaultStaticPaths())
		path := "/api/user/123"

		n.Normalize(path)
		_, exists := n.cache[path]
		assert.True(t, exists, "路径应该被缓存")

		result := n.Normalize(path)
		assert.Equal(t, "/api/user/:param", result)
	})

	t.Run("LRU淘汰", func(t *testing.T) {
		n := newSmartPathNormalizer(monitoring.DefaultStaticPaths())
		n.maxCache = 3

		n.Normalize("/api/test/a")
		n.Normalize("/api/test/b")
		n.Normalize("/api/test/c")
		assert.Equal(t, 3, len(n.cache))

		n.Normalize("/api/test/d")
		assert.LessOrEqual(t, len(n.cache), 3)
	})
}

// TestSmartPathNormalizer_DifferentPrefixes 测试不同前缀独立学习（版本前缀保留）
func TestSmartPathNormalizer_DifferentPrefixes(t *testing.T) {
	n := newSmartPathNormalizer(monitoring.DefaultStaticPaths())

	steps := []struct {
		input    string
		expected string
		reason   string
	}{
		{"/api/v1/users/123", "/api/v1/users/:param", "v1保留(内容检测), users首次→保留, 123→:param"},
		{"/api/v1/users/567", "/api/v1/users/:param", "v1保留, users已知→保留, 567→:param"},
		{"/api/v2/users/789", "/api/v2/users/:param", "v2保留(内容检测), users首次于/v2→保留, 789→:param"},
		{"/api/v2/users/101", "/api/v2/users/:param", "v2保留, users已知→保留, 101→:param"},
	}

	for i, step := range steps {
		result := n.Normalize(step.input)
		assert.Equal(t, step.expected, result, "步骤 %d [%s]: 输入=%s", i+1, step.reason, step.input)
	}
}

// TestSmartPathNormalizer_ConcurrentAccess 测试并发安全
func TestSmartPathNormalizer_ConcurrentAccess(t *testing.T) {
	n := newSmartPathNormalizer(monitoring.DefaultStaticPaths())
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

// TestSmartPathNormalizer_SingletonScenario 测试多资源路径（内容检测+学习混合）
func TestSmartPathNormalizer_SingletonScenario(t *testing.T) {
	n := newSmartPathNormalizer(monitoring.DefaultStaticPaths())

	steps := []struct {
		input    string
		expected string
		reason   string
	}{
		// items 路由（item-001 含数字→内容检测立即 :param）
		{"/api/items/item-001/status", "/api/items/:param/status",
			"items首次→保留, item-001混合→:param, status首次→保留"},
		{"/api/items/item-002/status", "/api/items/:param/status",
			"items已知→保留, item-002混合→:param, status已知→保留"},

		// entities 出现 → 位置2第2个值 → 标记动态
		{"/api/entities/entity-001", "/api/:param/:param",
			"entities是位置2第2个值→:param, entity-001混合→:param"},
		{"/api/entities/entity-002", "/api/:param/:param",
			"位置2已知动态→:param, entity-002混合→:param"},

		// collections 路由（位置2已知动态）
		{"/api/collections/coll-001/members/mem-001/perms", "/api/:param/:param/members/:param/perms",
			"位置2动态→:param, coll-001混合→:param, members首次→保留, mem-001混合→:param, perms首次→保留"},
		{"/api/collections/coll-002/members/mem-002/perms", "/api/:param/:param/members/:param/perms",
			"同上, members/perms已知→保留"},

		// repos 路由（repo-a 纯字母→学习，首次保留）
		{"/api/repos/repo-a/files", "/api/:param/repo-a/files",
			"位置2动态→:param, repo-a纯字母首次→保留, files首次→保留"},
		// repo-c 第2个值 → 位置3标记动态；files 与 members 同位置 → 也标记动态
		{"/api/repos/repo-c/files/doc123", "/api/:param/:param/:param/:param",
			"位置2动态→:param, repo-c是位置3第2个值→:param, files与members同位置第2个值→:param, doc123混合→:param"},
	}

	for i, step := range steps {
		result := n.Normalize(step.input)
		assert.Equal(t, step.expected, result, "步骤 %d [%s]: 输入=%s", i+1, step.reason, step.input)
	}
}

// BenchmarkSmartPathNormalizer_Normalize 性能基准测试
func BenchmarkSmartPathNormalizer_Normalize(b *testing.B) {
	n := newSmartPathNormalizer(monitoring.DefaultStaticPaths())
	n.Normalize("/api/user/1") // 预热缓存

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n.Normalize("/api/user/123")
	}
}

// BenchmarkSmartPathNormalizer_CacheHit 缓存命中性能测试
func BenchmarkSmartPathNormalizer_CacheHit(b *testing.B) {
	n := newSmartPathNormalizer(monitoring.DefaultStaticPaths())
	path := "/api/user/123"
	n.Normalize(path) // 预热缓存

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n.Normalize(path)
	}
}

// BenchmarkSmartPathNormalizer_NewPath 新路径性能测试
func BenchmarkSmartPathNormalizer_NewPath(b *testing.B) {
	n := newSmartPathNormalizer(monitoring.DefaultStaticPaths())
	n.Normalize("/api/user/1") // 预热

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := "/api/user/" + string(rune('a'+(i%26)))
		n.Normalize(path)
	}
}
