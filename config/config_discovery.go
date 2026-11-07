/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-12 16:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-13 10:54:19
 * @FilePath: \go-rpc-gateway\config\config_discovery.go
 * @Description: 配置文件自动发现工具
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ConfigDiscovery 配置文件发现器
type ConfigDiscovery struct {
	// 支持的配置文件扩展名，按优先级排序
	SupportedExtensions []string
	// 默认配置文件名（不含扩展名）
	DefaultNames []string
	// 环境特定的配置文件前缀
	EnvPrefixes map[EnvironmentType][]string
}

// ConfigFileInfo 配置文件信息
type ConfigFileInfo struct {
	Path        string          `json:"path"`        // 文件完整路径
	Name        string          `json:"name"`        // 文件名（含扩展名）
	BaseName    string          `json:"base_name"`   // 文件名（不含扩展名）
	Extension   string          `json:"extension"`   // 文件扩展名
	Environment EnvironmentType `json:"environment"` // 环境类型
	Priority    int             `json:"priority"`    // 优先级（数字越小优先级越高）
	Exists      bool            `json:"exists"`      // 文件是否存在
}

// NewConfigDiscovery 创建配置文件发现器
func NewConfigDiscovery() *ConfigDiscovery {
	return &ConfigDiscovery{
		SupportedExtensions: DefaultSupportedExtensions,
		DefaultNames:        DefaultConfigNames,
		EnvPrefixes:         DefaultEnvPrefixes,
	}
}

// DiscoverConfigFiles 在指定目录中发现配置文件
func (cd *ConfigDiscovery) DiscoverConfigFiles(searchPath string, env EnvironmentType) ([]*ConfigFileInfo, error) {
	var configFiles []*ConfigFileInfo

	// 确保搜索路径存在
	if _, err := os.Stat(searchPath); os.IsNotExist(err) {
		return nil, ErrSearchPathNotExist(searchPath)
	}

	// 生成可能的配置文件名
	candidates := cd.generateConfigCandidates(env)

	// 在搜索路径中查找配置文件
	for _, candidate := range candidates {
		for _, ext := range cd.SupportedExtensions {
			fileName := candidate.BaseName + ext
			fullPath := filepath.Join(searchPath, fileName)

			info := &ConfigFileInfo{
				Path:        fullPath,
				Name:        fileName,
				BaseName:    candidate.BaseName,
				Extension:   ext,
				Environment: candidate.Environment,
				Priority:    candidate.Priority,
				Exists:      false,
			}

			// 检查文件是否存在
			if _, err := os.Stat(fullPath); err == nil {
				info.Exists = true
			}

			configFiles = append(configFiles, info)
		}
	}

	// 按优先级排序（优先级数字越小越靠前）
	sort.Slice(configFiles, func(i, j int) bool {
		if configFiles[i].Priority != configFiles[j].Priority {
			return configFiles[i].Priority < configFiles[j].Priority
		}
		// 如果优先级相同，优先选择存在的文件
		if configFiles[i].Exists != configFiles[j].Exists {
			return configFiles[i].Exists
		}
		// 如果都存在或都不存在，按扩展名优先级排序
		return cd.getExtensionPriority(configFiles[i].Extension) < cd.getExtensionPriority(configFiles[j].Extension)
	})

	return configFiles, nil
}

// FindBestConfigFile 找到最合适的配置文件
func (cd *ConfigDiscovery) FindBestConfigFile(searchPath string, env EnvironmentType) (*ConfigFileInfo, error) {
	configFiles, err := cd.DiscoverConfigFiles(searchPath, env)
	if err != nil {
		return nil, err
	}

	// 首先尝试找到存在的文件
	for _, info := range configFiles {
		if info.Exists {
			return info, nil
		}
	}

	// 如果没有找到存在的文件，返回优先级最高的候选文件
	if len(configFiles) > 0 {
		return configFiles[0], ErrConfigFileNotFound(configFiles[0].Path)
	}

	return nil, ErrNoConfigCandidate
}

// FindConfigFileByPattern 根据模式查找配置文件
func (cd *ConfigDiscovery) FindConfigFileByPattern(searchPath, pattern string, env EnvironmentType) ([]*ConfigFileInfo, error) {
	configFiles, err := cd.DiscoverConfigFiles(searchPath, env)
	if err != nil {
		return nil, err
	}

	var matchedFiles []*ConfigFileInfo
	pattern = strings.ToLower(pattern)

	for _, info := range configFiles {
		if info.Exists && strings.Contains(strings.ToLower(info.BaseName), pattern) {
			matchedFiles = append(matchedFiles, info)
		}
	}

	return matchedFiles, nil
}

// ScanDirectory 扫描目录中的所有配置文件
func (cd *ConfigDiscovery) ScanDirectory(searchPath string) ([]*ConfigFileInfo, error) {
	var allFiles []*ConfigFileInfo

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(info.Name()))
		if cd.isSupportedExtension(ext) {
			baseName := strings.TrimSuffix(info.Name(), ext)
			env := cd.detectEnvironment(baseName)

			fileInfo := &ConfigFileInfo{
				Path:        path,
				Name:        info.Name(),
				BaseName:    baseName,
				Extension:   ext,
				Environment: env,
				Priority:    cd.calculatePriority(baseName, env),
				Exists:      true,
			}

			allFiles = append(allFiles, fileInfo)
		}

		return nil
	})

	if err != nil {
		return nil, ErrScanDir(err)
	}

	// 按优先级排序
	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].Priority < allFiles[j].Priority
	})

	return allFiles, nil
}

// generateConfigCandidates 生成配置文件候选列表
func (cd *ConfigDiscovery) generateConfigCandidates(env EnvironmentType) []struct {
	BaseName    string
	Environment EnvironmentType
	Priority    int
} {
	var candidates []struct {
		BaseName    string
		Environment EnvironmentType
		Priority    int
	}

	priority := 0

	// 1. 环境特定的配置文件（最高优先级）
	if prefixes, exists := cd.EnvPrefixes[env]; exists {
		for _, prefix := range prefixes {
			for _, name := range cd.DefaultNames {
				candidates = append(candidates, struct {
					BaseName    string
					Environment EnvironmentType
					Priority    int
				}{
					BaseName:    fmt.Sprintf("%s-%s", name, prefix),
					Environment: env,
					Priority:    priority,
				})
				priority++

				candidates = append(candidates, struct {
					BaseName    string
					Environment EnvironmentType
					Priority    int
				}{
					BaseName:    fmt.Sprintf("%s.%s", name, prefix),
					Environment: env,
					Priority:    priority,
				})
				priority++
			}
		}
	}

	// 2. 通用配置文件（较低优先级）
	for _, name := range cd.DefaultNames {
		candidates = append(candidates, struct {
			BaseName    string
			Environment EnvironmentType
			Priority    int
		}{
			BaseName:    name,
			Environment: EnvDevelopment, // 默认环境
			Priority:    priority,
		})
		priority++
	}

	return candidates
}

// calculatePriority 计算文件优先级
func (cd *ConfigDiscovery) calculatePriority(baseName string, env EnvironmentType) int {
	priority := 1000 // 基础优先级

	// 环境匹配度
	if prefixes, exists := cd.EnvPrefixes[env]; exists {
		for i, prefix := range prefixes {
			if strings.Contains(strings.ToLower(baseName), prefix) {
				priority -= (100 - i*10) // 环境匹配的文件优先级更高
				break
			}
		}
	}

	// 默认名称匹配度
	for i, name := range cd.DefaultNames {
		if strings.Contains(strings.ToLower(baseName), name) {
			priority -= (50 - i*5) // 默认名称匹配的文件优先级更高
			break
		}
	}

	return priority
}

// detectEnvironment 从文件名检测环境
// 约定配置文件名形如 name-<env>（如 config-dev、application-prod），
// 环境段为按分隔符切分后的最后一个 token，按完整相等匹配，
// 避免 "co"-"colombia"、"at"-"austria" 等短前缀因子串包含而误命中。
func (cd *ConfigDiscovery) detectEnvironment(baseName string) EnvironmentType {
	lowerName := strings.ToLower(baseName)

	// 按分隔符切分，取最后一个 token 作为环境段
	tokens := strings.FieldsFunc(lowerName, func(r rune) bool {
		return r == '-' || r == '_' || r == '.' || r == ' '
	})
	if len(tokens) == 0 {
		return EnvDevelopment
	}
	last := tokens[len(tokens)-1]

	for env, prefixes := range cd.EnvPrefixes {
		for _, prefix := range prefixes {
			if last == prefix {
				return env
			}
		}
	}

	return EnvDevelopment // 默认环境
}

// isSupportedExtension 检查是否为支持的扩展名
func (cd *ConfigDiscovery) isSupportedExtension(ext string) bool {
	for _, supported := range cd.SupportedExtensions {
		if ext == supported {
			return true
		}
	}
	return false
}

// getExtensionPriority 获取扩展名优先级
func (cd *ConfigDiscovery) getExtensionPriority(ext string) int {
	for i, supported := range cd.SupportedExtensions {
		if ext == supported {
			return i
		}
	}
	return len(cd.SupportedExtensions)
}

// 全局配置发现器实例
var globalConfigDiscovery *ConfigDiscovery
var globalConfigDiscoveryOnce sync.Once

// GetGlobalConfigDiscovery 获取全局配置发现器
func GetGlobalConfigDiscovery() *ConfigDiscovery {
	globalConfigDiscoveryOnce.Do(func() {
		globalConfigDiscovery = NewConfigDiscovery()
	})
	return globalConfigDiscovery
}

// 便利函数

// DiscoverConfig 发现配置文件（便利函数）
func DiscoverConfig(searchPath string, env EnvironmentType) ([]*ConfigFileInfo, error) {
	return GetGlobalConfigDiscovery().DiscoverConfigFiles(searchPath, env)
}

// FindBestConfig 找到最佳配置文件（便利函数）
func FindBestConfig(searchPath string, env EnvironmentType) (*ConfigFileInfo, error) {
	return GetGlobalConfigDiscovery().FindBestConfigFile(searchPath, env)
}
