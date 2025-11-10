/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:37:03
 * @FilePath: \go-rpc-gateway\config\health.go
 * @Description:
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package config

// HealthConfig 健康检查配置 - 集成 go-core 的健康检查
type HealthConfig struct {
	Enabled  bool                   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Path     string                 `mapstructure:"path" yaml:"path" json:"path"`
	Checks   map[string]HealthCheck `mapstructure:"checks" yaml:"checks" json:"checks"`
	Response HealthResponseConfig   `mapstructure:"response" yaml:"response" json:"response"`
}

// HealthCheck 健康检查项配置
type HealthCheck struct {
	Enabled    bool        `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Name       string      `mapstructure:"name" yaml:"name" json:"name"`
	Timeout    string      `mapstructure:"timeout" yaml:"timeout" json:"timeout"`
	Connection interface{} `mapstructure:"connection" yaml:"connection" json:"connection"` // 可以是各种连接配置
	Endpoint   string      `mapstructure:"endpoint" yaml:"endpoint" json:"endpoint"`
}

// HealthResponseConfig 健康检查响应配置
type HealthResponseConfig struct {
	IncludeDetails    bool              `mapstructure:"include_details" yaml:"include_details" json:"include_details"`
	IncludeSystemInfo bool              `mapstructure:"include_system_info" yaml:"include_system_info" json:"include_system_info"`
	CustomFields      map[string]string `mapstructure:"custom_fields" yaml:"custom_fields" json:"custom_fields"`
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	Enabled  bool              `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Path     string            `mapstructure:"path" yaml:"path" json:"path"`
	Interval int               `mapstructure:"interval" yaml:"interval" json:"interval"`
	Timeout  int               `mapstructure:"timeout" yaml:"timeout" json:"timeout"` // 健康检查超时时间(秒)
	Redis    RedisHealthConfig `mapstructure:"redis" yaml:"redis" json:"redis"`       // Redis健康检查配置
	MySQL    MySQLHealthConfig `mapstructure:"mysql" yaml:"mysql" json:"mysql"`       // MySQL健康检查配置
}

// RedisHealthConfig Redis健康检查配置
type RedisHealthConfig struct {
	Enabled  bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Host     string `mapstructure:"host" yaml:"host" json:"host"`
	Port     int    `mapstructure:"port" yaml:"port" json:"port"`
	Password string `mapstructure:"password" yaml:"password" json:"password"`
	Database int    `mapstructure:"database" yaml:"database" json:"database"`
	Timeout  int    `mapstructure:"timeout" yaml:"timeout" json:"timeout"` // 连接超时时间(秒)
}

// MySQLHealthConfig MySQL健康检查配置
type MySQLHealthConfig struct {
	Enabled  bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Host     string `mapstructure:"host" yaml:"host" json:"host"`
	Port     int    `mapstructure:"port" yaml:"port" json:"port"`
	Username string `mapstructure:"username" yaml:"username" json:"username"`
	Password string `mapstructure:"password" yaml:"password" json:"password"`
	Database string `mapstructure:"database" yaml:"database" json:"database"`
	Timeout  int    `mapstructure:"timeout" yaml:"timeout" json:"timeout"` // 连接超时时间(秒)
}
