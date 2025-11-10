/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-10 11:36:08
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:36:42
 * @FilePath: \go-rpc-gateway\config\banner.go
 * @Description: 
 * 
 * Copyright (c) 2025 by kamalyes, All Rights Reserved. 
 */
package config

// BannerConfig 启动横幅配置 - 使用 go-toolbox 的模板功能
type BannerConfig struct {
	Enabled        bool              `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Template       string            `mapstructure:"template" yaml:"template" json:"template"`
	Colors         BannerColorConfig `mapstructure:"colors" yaml:"colors" json:"colors"`
	ShowSystemInfo bool              `mapstructure:"show_system_info" yaml:"show_system_info" json:"show_system_info"`
	ShowMiddleware bool              `mapstructure:"show_middleware" yaml:"show_middleware" json:"show_middleware"`
	ShowRoutes     bool              `mapstructure:"show_routes" yaml:"show_routes" json:"show_routes"`
	CustomFields   map[string]string `mapstructure:"custom_fields" yaml:"custom_fields" json:"custom_fields"`
}

// BannerColorConfig 横幅颜色配置
type BannerColorConfig struct {
	Enabled bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Title   string `mapstructure:"title" yaml:"title" json:"title"`
	Info    string `mapstructure:"info" yaml:"info" json:"info"`
	Warning string `mapstructure:"warning" yaml:"warning" json:"warning"`
	Error   string `mapstructure:"error" yaml:"error" json:"error"`
}
