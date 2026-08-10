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

	"github.com/kamalyes/go-argus/validate"
)

// PathNormalizer 路径规范化器接口
type PathNormalizer interface {
	Normalize(path string) string
}

// smartPathNormalizer 智能路径规范化器（内容特征检测 + 前缀匹配自动学习）
//
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
	mu            sync.RWMutex
	cache         map[string]string           // 原始路径 -> 规范化路径
	staticPaths   map[string]bool             // 静态路径集合（快速查找）
	pathStructure map[string]map[int][]string // 前缀 -> 位置 -> 观察到的值（nil 表示已动态）
	maxValues     int                         // 触发动态判定的最大不同值数
	maxCache      int                         // 最大缓存数量
}

// newSmartPathNormalizer 创建智能路径规范化器
func newSmartPathNormalizer(staticPaths []string) *smartPathNormalizer {
	staticPathMap := make(map[string]bool, len(staticPaths))
	for _, path := range staticPaths {
		staticPathMap[path] = true
	}

	return &smartPathNormalizer{
		cache:         make(map[string]string, 1000),
		staticPaths:   staticPathMap,
		pathStructure: make(map[string]map[int][]string),
		maxValues:     2, // 同一位置出现 2 个不同值即判定为动态
		maxCache:      1000,
	}
}

// Normalize 规范化路径（内容特征检测 + 前缀学习）
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

	// 4. 混合规范化（内容特征检测 + 前缀学习）
	normalized := n.smartNormalize(path)

	// 5. 缓存结果
	n.addToCache(path, normalized)

	return normalized
}

// smartNormalize 混合规范化：先内容特征检测（无状态），再前缀学习（有状态）
//
// 对每个路径段：
//   - 优先内容特征检测：数字/UUID/字母+数字混合 → 立即 :param
//   - 其次前缀学习：纯字母段通过前缀上下文记录值，达到阈值后 → :param
func (n *smartPathNormalizer) smartNormalize(path string) string {
	parts := strings.Split(path, "/")

	n.mu.Lock()
	defer n.mu.Unlock()

	prefix := ""
	for i, part := range parts {
		if part == "" {
			continue
		}

		// 阶段一：内容特征检测（立即判定，无需学习）
		if isDynamicSegment(part) {
			parts[i] = ":param"
			prefix += "/:param"
			continue
		}

		// 版本前缀 v1/v2/v10 → 确定静态，保留并跳过学习
		if len(part) >= 2 && part[0] == 'v' && validate.IsDigits(part[1:]) {
			prefix += "/" + part
			continue
		}

		// 阶段二：前缀学习（纯字母段通过上下文学习）
		posValues, exists := n.pathStructure[prefix]
		if !exists {
			posValues = make(map[int][]string)
			n.pathStructure[prefix] = posValues
		}

		// 该位置已标记为动态（nil slice 表示动态）
		if vals, ok := posValues[i]; ok && vals == nil {
			parts[i] = ":param"
			prefix += "/:param"
			continue
		}

		// 记录值并检查是否达到阈值
		found := false
		for _, v := range posValues[i] {
			if v == part {
				found = true
				break
			}
		}
		if !found {
			posValues[i] = append(posValues[i], part)
			if len(posValues[i]) >= n.maxValues {
				posValues[i] = nil // 标记该位置为动态
				parts[i] = ":param"
				prefix += "/:param"
				continue
			}
		}

		prefix += "/" + part
	}

	return strings.Join(parts, "/")
}

// isDynamicSegment 判断路径段是否为动态参数（基于内容特征）
//
// 判定为动态（满足任一）：
//   - 纯数字（如 123, 999）
//   - UUID 格式（如 550e8400-e29b-41d4-a716-446655440000）
//   - 同时包含字母和数字（如 abc123, user-001, item-002）
//
// 保留为静态（交由前缀学习判定）：
//   - 版本前缀（如 v1, v2, v10）
//   - 纯字母（如 users, roles, my-bucket）
func isDynamicSegment(s string) bool {
	if s == "" {
		return false
	}

	// 版本前缀 v1/v2/v10 → 静态
	if len(s) >= 2 && s[0] == 'v' && validate.IsDigits(s[1:]) {
		return false
	}

	// 纯数字 → 动态（ID: 123, 999）
	if validate.IsDigits(s) {
		return true
	}

	// UUID 格式 → 动态
	if validate.IsUUID(s) {
		return true
	}

	// 同时包含字母和数字 → 动态（abc123, user-001, item-002）
	hasDigit, hasAlpha := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			hasDigit = true
		} else if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			hasAlpha = true
		}
	}
	return hasDigit && hasAlpha
}

// addToCache 添加到缓存（简单淘汰策略）
func (n *smartPathNormalizer) addToCache(original, normalized string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.cache) >= n.maxCache {
		for k := range n.cache {
			delete(n.cache, k)
			if len(n.cache) < n.maxCache/2 {
				break
			}
		}
	}
	n.cache[original] = normalized
}
