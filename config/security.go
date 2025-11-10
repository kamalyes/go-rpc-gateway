/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-10 11:25:00
 * @FilePath: \go-rpc-gateway\config\security.go
 * @Description: 安全配置模块
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package config

// SecurityConfig 安全配置
type SecurityConfig struct {
	TLS      TLSConfig      `mapstructure:"tls" yaml:"tls" json:"tls"`
	Security SecurityPolicy `mapstructure:"policy" yaml:"policy" json:"policy"`
}

// TLSConfig TLS配置
type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	CertFile string `mapstructure:"cert_file" yaml:"cert_file" json:"cert_file"`
	KeyFile  string `mapstructure:"key_file" yaml:"key_file" json:"key_file"`
	CAFile   string `mapstructure:"ca_file" yaml:"ca_file" json:"ca_file"`
}

// SecurityPolicy 安全策略
type SecurityPolicy struct {
	EnableCSRFProtection  bool              `mapstructure:"enable_csrf_protection" yaml:"enable_csrf_protection" json:"enable_csrf_protection"`
	EnableXSSProtection   bool              `mapstructure:"enable_xss_protection" yaml:"enable_xss_protection" json:"enable_xss_protection"`
	ContentTypeNoSniff    bool              `mapstructure:"content_type_no_sniff" yaml:"content_type_no_sniff" json:"content_type_no_sniff"`
	FrameOptions          string            `mapstructure:"frame_options" yaml:"frame_options" json:"frame_options"`
	Enabled               bool              `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Headers               map[string]string `mapstructure:"headers" yaml:"headers" json:"headers"`
	XSSProtection         bool              `mapstructure:"xss_protection" yaml:"xss_protection" json:"xss_protection"`
	ContentSecurityPolicy string            `mapstructure:"content_security_policy" yaml:"content_security_policy" json:"content_security_policy"`
	ReferrerPolicy        string            `mapstructure:"referrer_policy" yaml:"referrer_policy" json:"referrer_policy"`
	HSTSMaxAge            int               `mapstructure:"hsts_max_age" yaml:"hsts_max_age" json:"hsts_max_age"`
}
