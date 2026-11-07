/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\constants\render.go
 * @Description: 渲染常量 - HTML 元信息、Swagger UI 配置、JSON 序列化、调试字段
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package constants

import "time"

// ==================== HTML 渲染常量 ====================

const (
	HTMLLangEN       = "en"
	HTMLLangZH       = "zh-CN"
	HTMLCharset      = "UTF-8"
	HTMLMetaViewport = "width=device-width, initial-scale=1.0"
	HTMLIconSizes32  = "32x32"
	HTMLIconSizes16  = "16x16"
)

// ==================== Swagger UI 配置 ====================

const (
	UILayout = "StandaloneLayout"
	UIDomID  = "#swagger-ui"
)

// ==================== JSON 序列化 ====================

const (
	JSONIndentPrefix = ""
	JSONIndentValue  = "  "
)

// ==================== 调试信息字段 ====================

const (
	FieldTotalServices      = "total_services"
	FieldLoadedServices     = "loaded_services"
	FieldConfiguredServices = "configured_services"
	FieldTimestamp          = "timestamp"
	FieldSpecPath           = "spec_path"
)

// ==================== 默认超时与刷新间隔 ====================

const (
	DefaultTimeout         = 30 * time.Second
	DefaultRefreshInterval = 5 * time.Minute
)
