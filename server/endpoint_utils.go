/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-26 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-26 12:10:55
 * @FilePath: \go-rpc-gateway\server\endpoint_utils.go
 * @Description: API端点信息聚合工具 - 纯工具方法，无业务侵入
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package server

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// EndpointInfo API端点信息
type EndpointInfo struct {
	Method      string   `json:"method"`       // HTTP方法
	Path        string   `json:"path"`         // 路径
	Summary     string   `json:"summary"`      // 描述
	OperationID string   `json:"operation_id"` // 操作ID
	Tags        []string `json:"tags"`         // 标签
}

// EndpointResponse 端点信息响应
type EndpointResponse struct {
	EndpointInfos []EndpointInfo `json:"endpoint_infos"`
}

// EndpointCollector 端点信息收集器
type EndpointCollector struct {
	mu        sync.RWMutex
	endpoints []EndpointInfo
}

// NewEndpointCollector 创建新的端点收集器
func NewEndpointCollector() *EndpointCollector {
	return &EndpointCollector{
		endpoints: make([]EndpointInfo, 0),
	}
}

// AddEndpoint 添加端点信息
func (ec *EndpointCollector) AddEndpoint(endpoint EndpointInfo) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	// 检查是否已存在相同的端点
	for i, existing := range ec.endpoints {
		if existing.Method == endpoint.Method && existing.Path == endpoint.Path {
			// 更新已存在的端点
			ec.endpoints[i] = endpoint
			return
		}
	}

	// 添加新端点
	ec.endpoints = append(ec.endpoints, endpoint)
}

// GetAllEndpoints 获取所有端点信息
func (ec *EndpointCollector) GetAllEndpoints() []EndpointInfo {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	// 创建副本
	result := make([]EndpointInfo, len(ec.endpoints))
	copy(result, ec.endpoints)

	// 按路径和方法排序
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path == result[j].Path {
			return result[i].Method < result[j].Method
		}
		return result[i].Path < result[j].Path
	})

	return result
}

// Clear 清空所有端点
func (ec *EndpointCollector) Clear() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.endpoints = ec.endpoints[:0]
}

// GenerateEndpointInfo 生成端点信息的工具方法
// 用户需要提供完整的信息，不做任何业务假设
func GenerateEndpointInfo(method, path, summary, operationID string, tags []string) EndpointInfo {
	if tags == nil {
		tags = []string{}
	}

	return EndpointInfo{
		Method:      strings.ToUpper(method),
		Path:        path,
		Summary:     summary,
		OperationID: operationID,
		Tags:        tags,
	}
}

// LoadEndpointsFromSwaggerFile 从单个Swagger YAML文件加载端点信息
func (ec *EndpointCollector) LoadEndpointsFromSwaggerFile(filePath string) error {
	// 读取YAML文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %v", err)
	}

	// 解析YAML
	var swaggerDoc map[string]interface{}
	if err := yaml.Unmarshal(data, &swaggerDoc); err != nil {
		return fmt.Errorf("解析YAML失败: %v", err)
	}

	// 从解析的YAML中提取端点信息
	return ec.CollectFromSwagger(swaggerDoc)
}

// LoadEndpointsFromSwaggerFiles 从Swagger YAML文件批量加载端点信息
func (ec *EndpointCollector) LoadEndpointsFromSwaggerFiles(swaggerDir string) error {
	var files []string

	// 递归查找所有子目录中的swagger文件
	err := filepath.Walk(swaggerDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".swagger.yaml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("遍历目录失败: %v", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("在目录 %s 中未找到 .swagger.yaml 文件", swaggerDir)
	}

	for _, file := range files {
		if err := ec.LoadEndpointsFromSwaggerFile(file); err != nil {
			continue // 跳过错误的文件，继续处理其他文件
		}
	}

	return nil
}

// CollectFromSwagger 从Swagger数据收集端点信息
func (ec *EndpointCollector) CollectFromSwagger(swaggerData map[string]interface{}) error {
	paths, ok := swaggerData["paths"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("swagger数据中没有paths字段")
	}

	for path, pathData := range paths {
		pathMap, ok := pathData.(map[string]interface{})
		if !ok {
			continue
		}

		for method, methodData := range pathMap {
			method = strings.ToUpper(method)
			if !isValidHTTPMethod(method) {
				continue
			}

			methodMap, ok := methodData.(map[string]interface{})
			if !ok {
				continue
			}

			endpoint := EndpointInfo{
				Method: method,
				Path:   path,
			}

			// 提取summary
			if summary, ok := methodMap["summary"].(string); ok {
				endpoint.Summary = summary
			}

			// 提取operationId
			if operationID, ok := methodMap["operationId"].(string); ok {
				endpoint.OperationID = operationID
			}

			// 提取tags
			if tags, ok := methodMap["tags"].([]interface{}); ok {
				for _, tag := range tags {
					if tagStr, ok := tag.(string); ok {
						endpoint.Tags = append(endpoint.Tags, tagStr)
					}
				}
			}

			if endpoint.Tags == nil {
				endpoint.Tags = []string{}
			}

			ec.AddEndpoint(endpoint)
		}
	}

	return nil
}

// ToJSON 将端点信息转换为JSON格式
func (ec *EndpointCollector) ToJSON() ([]byte, error) {
	endpoints := ec.GetAllEndpoints()
	response := EndpointResponse{
		EndpointInfos: endpoints,
	}
	return json.Marshal(response)
}

// CreateHTTPHandler 创建HTTP处理器（工具方法）
func (ec *EndpointCollector) CreateHTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		jsonData, err := ec.ToJSON()
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)
	}
}

// isValidHTTPMethod 检查是否为有效的HTTP方法
func isValidHTTPMethod(method string) bool {
	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}
	for _, validMethod := range validMethods {
		if method == validMethod {
			return true
		}
	}
	return false
}
