/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-02-27 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-02-27 15:00:00
 * @FilePath: \go-rpc-gateway\middleware\path_normalizer.go
 * @Description: 智能路径规范化器 - 自动学习动态参数模式
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package middleware

import (
	"strings"
	"sync"
)

// PathNormalizer 路径规范化器接口
type PathNormalizer interface {
	Normalize(path string) string
}

// smartPathNormalizer 智能路径规范化器（基于前缀匹配自动学习）
// 核心思想：使用前缀作为上下文，逐段学习每个位置的值
//
// 示例流程：
// 1. /v1/buckets/my-bucket/objects
//   - 前缀 "/v1/buckets" 位置2: 记录 ["my-bucket"]
//   - 结果: /v1/buckets/my-bucket/objects
//
// 2. /v1/buckets/your-bucket/objects
//   - 前缀 "/v1/buckets" 位置2: 发现 ["my-bucket", "your-bucket"]
//   - 判定位置2为动态参数
//   - 结果: /v1/buckets/:param/objects
//
// 3. /v1/buckets/another-bucket/objects
//   - 前缀 "/v1/buckets" 位置2: 已知是动态
//   - 结果: /v1/buckets/:param/objects
//
// 4. /v1/buckets/my-bucket/objects/file.txt
//   - 前缀 "/v1/buckets" 位置2: 已知是动态 -> :param
//   - 前缀 "/v1/buckets/:param/objects" 位置4: 记录 ["file.txt"]
//   - 结果: /v1/buckets/:param/objects/file.txt
//
// 5. /v1/buckets/my-bucket/objects/photo.jpg
//   - 前缀 "/v1/buckets/:param/objects" 位置4: 发现 ["file.txt", "photo.jpg"]
//   - 判定位置4为动态参数
//   - 结果: /v1/buckets/:param/objects/:param
type smartPathNormalizer struct {
	mu            sync.RWMutex                // 保护并发访问
	cache         map[string]string           // 原始路径 -> 规范化路径
	pathStructure map[string]map[int][]string // 前缀key -> {位置索引 -> 该位置的值列表}
	staticPaths   map[string]bool             // 静态路径集合（快速查找）
	maxCache      int                         // 最大缓存数量
	maxValues     int                         // 每个位置最多记录多少个不同值
}

// newSmartPathNormalizer 创建智能路径规范化器
func newSmartPathNormalizer(staticPaths []string) *smartPathNormalizer {
	// 构建静态路径集合
	staticPathMap := make(map[string]bool, len(staticPaths))
	for _, path := range staticPaths {
		staticPathMap[path] = true
	}

	return &smartPathNormalizer{
		cache:         make(map[string]string, 1000),
		pathStructure: make(map[string]map[int][]string, 200),
		staticPaths:   staticPathMap,
		maxCache:      1000, // 缓存最多 1000 个路径
		maxValues:     5,    // 每个位置最多记录 5 个不同值，超过则判定为动态参数
	}
}

// Normalize 规范化路径（基于前缀匹配自动学习）
func (n *smartPathNormalizer) Normalize(path string) string {
	// 1. 检查缓存（读锁）
	n.mu.RLock()
	if normalized, ok := n.cache[path]; ok {
		n.mu.RUnlock()
		return normalized
	}
	n.mu.RUnlock()

	// 2. 移除查询参数
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	// 3. 静态路径直接返回
	if n.staticPaths[path] {
		n.addToCache(path, path)
		return path
	}

	// 4. 智能规范化（基于前缀学习）
	normalized := n.smartNormalize(path)

	// 5. 缓存结果
	n.addToCache(path, normalized)

	return normalized
}

// smartNormalize 智能规范化（逐段学习路径结构）
func (n *smartPathNormalizer) smartNormalize(path string) string {
	parts := strings.Split(path, "/")
	normalized := make([]string, len(parts))

	// 逐段分析，使用前缀作为上下文
	for i, part := range parts {
		if part == "" {
			normalized[i] = ""
			continue
		}

		// 构建前缀 key（用于区分不同的路径结构）
		prefixKey := n.buildPrefixKey(normalized[:i])

		// 使用写锁保护结构修改
		n.mu.Lock()

		// 确保该前缀的结构存在
		if _, exists := n.pathStructure[prefixKey]; !exists {
			n.pathStructure[prefixKey] = make(map[int][]string)
		}

		// 检查该位置是否已经判定为动态参数
		if n.isDynamicPositionUnsafe(prefixKey, i) {
			n.mu.Unlock()
			normalized[i] = ":param"
			continue
		}

		// 记录该位置的值
		n.recordValueUnsafe(prefixKey, i, part)

		// 检查该位置的值是否多样化（判定为动态参数）
		if n.shouldBeDynamicUnsafe(prefixKey, i) {
			normalized[i] = ":param"
		} else {
			normalized[i] = part
		}

		n.mu.Unlock()
	}

	return strings.Join(normalized, "/")
}

// buildPrefixKey 构建前缀 key（用于区分不同的路径上下文）
// 注意：使用已规范化的前缀，这样可以正确处理嵌套的动态参数
// 例如：["", "v1", "buckets", ":param"] -> "/v1/buckets/:param"
func (n *smartPathNormalizer) buildPrefixKey(normalizedPrefix []string) string {
	if len(normalizedPrefix) == 0 {
		return "/"
	}
	return strings.Join(normalizedPrefix, "/")
}

// recordValueUnsafe 记录某个位置出现的值（不加锁，调用者需持有锁）
func (n *smartPathNormalizer) recordValueUnsafe(prefixKey string, position int, value string) {
	values := n.pathStructure[prefixKey][position]

	// 检查是否已存在
	for _, v := range values {
		if v == value {
			return
		}
	}

	// 限制记录数量，防止内存泄漏
	if len(values) < n.maxValues {
		n.pathStructure[prefixKey][position] = append(values, value)
	}
}

// shouldBeDynamicUnsafe 判断某个位置是否应该是动态参数（不加锁）
func (n *smartPathNormalizer) shouldBeDynamicUnsafe(prefixKey string, position int) bool {
	values := n.pathStructure[prefixKey][position]
	return len(values) >= 2
}

// isDynamicPositionUnsafe 检查某个位置是否已经被判定为动态参数（不加锁）
func (n *smartPathNormalizer) isDynamicPositionUnsafe(prefixKey string, position int) bool {
	values := n.pathStructure[prefixKey][position]
	return len(values) >= n.maxValues
}

// addToCache 添加到缓存（LRU 淘汰）
func (n *smartPathNormalizer) addToCache(original, normalized string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.cache) >= n.maxCache {
		// 简单的淘汰策略：清空一半缓存
		for k := range n.cache {
			delete(n.cache, k)
			if len(n.cache) < n.maxCache/2 {
				break
			}
		}
	}
	n.cache[original] = normalized
}
