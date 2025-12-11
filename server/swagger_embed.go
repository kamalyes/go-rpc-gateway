/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-26 12:15:00
 * @FilePath: \go-rpc-gateway\server\swagger_embed.go
 * @Description: Swagger 文件嵌入和访问工具
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package server

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"path/filepath"
	"strings"
)

// SwaggerFileProvider 提供访问 Swagger 文件的接口
type SwaggerFileProvider interface {
	// GetSwaggerFiles 获取所有可用的 Swagger 文件
	GetSwaggerFiles() map[string][]byte
	// GetSwaggerFile 获取指定的 Swagger 文件内容
	GetSwaggerFile(path string) ([]byte, error)
}

// EmbeddedSwaggerProvider 嵌入式 Swagger 文件提供器
type EmbeddedSwaggerProvider struct {
	files map[string][]byte
}

// NewEmbeddedSwaggerProvider 创建嵌入式提供器
func NewEmbeddedSwaggerProvider(files map[string][]byte) *EmbeddedSwaggerProvider {
	return &EmbeddedSwaggerProvider{
		files: files,
	}
}

// GetSwaggerFiles 获取所有文件
func (p *EmbeddedSwaggerProvider) GetSwaggerFiles() map[string][]byte {
	return p.files
}

// GetSwaggerFile 获取指定文件
func (p *EmbeddedSwaggerProvider) GetSwaggerFile(path string) ([]byte, error) {
	if content, exists := p.files[path]; exists {
		return content, nil
	}
	return nil, fmt.Errorf("swagger文件不存在: %s", path)
}

// LoadEndpointsFromProvider 从 Swagger 文件提供器加载端点信息
func (ec *EndpointCollector) LoadEndpointsFromProvider(provider SwaggerFileProvider) error {
	files := provider.GetSwaggerFiles()

	loadedCount := 0
	for filePath, content := range files {
		// 只处理 .swagger.yaml 文件
		if !strings.HasSuffix(filePath, ".swagger.yaml") {
			continue
		}

		if err := ec.LoadEndpointsFromYAMLContent(content); err != nil {
			// 记录错误但继续处理其他文件
			continue
		}
		loadedCount++
	}

	if loadedCount == 0 {
		return fmt.Errorf("未找到有效的 swagger 文件")
	}

	return nil
}

// LoadEndpointsFromYAMLContent 从 YAML 内容加载端点信息
func (ec *EndpointCollector) LoadEndpointsFromYAMLContent(yamlContent []byte) error {
	// 使用现有的 YAML 解析逻辑
	var swaggerDoc map[string]interface{}
	if err := yaml.Unmarshal(yamlContent, &swaggerDoc); err != nil {
		return fmt.Errorf("解析YAML失败: %v", err)
	}

	return ec.CollectFromSwagger(swaggerDoc)
}

// GetSwaggerFilesByPattern 按模式获取文件
func (p *EmbeddedSwaggerProvider) GetSwaggerFilesByPattern(pattern string) map[string][]byte {
	result := make(map[string][]byte)

	for filePath, content := range p.files {
		matched, err := filepath.Match(pattern, filepath.Base(filePath))
		if err == nil && matched {
			result[filePath] = content
		}
	}

	return result
}
