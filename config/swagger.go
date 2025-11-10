/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 22:20:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 22:20:00
 * @FilePath: \go-rpc-gateway\config\swagger.go
 * @Description: Swagger配置定义
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */
package config

// SwaggerConfig Swagger配置
// [EN] Swagger configuration
type SwaggerConfig struct {
	Enabled     bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`             // 是否启用Swagger [EN] Whether to enable Swagger
	JSONPath    string `mapstructure:"json_path" yaml:"json_path" json:"json_path"`       // Swagger JSON文件路径 [EN] Swagger JSON file path
	UIPath      string `mapstructure:"ui_path" yaml:"ui_path" json:"ui_path"`             // Swagger UI路由路径 [EN] Swagger UI route path
	Title       string `mapstructure:"title" yaml:"title" json:"title"`                   // 文档标题 [EN] Documentation title
	Description string `mapstructure:"description" yaml:"description" json:"description"` // 文档描述 [EN] Documentation description
}
