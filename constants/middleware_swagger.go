/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-13 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-13 20:03:21
 * @FilePath: \go-rpc-gateway\constants\middleware_swagger.go
 * @Description: Swagger相关常量定义
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package constants

import "time"

// Swagger 路径相关常量
const (
	SwaggerJSONPath      = "/swagger.json"
	SwaggerServicesPath  = "/services"
	SwaggerAggregatePath = "/aggregate.json"
	SwaggerDebugPath     = "/debug/services"
	SwaggerIndexHTML     = "/index.html"
	SwaggerJSONExt       = ".json"
)

// Swagger 规范相关常量
const (
	SwaggerVersion         = "2.0"
	SwaggerPathDefinitions = "#/definitions/"
	SwaggerFieldRef        = "$ref"
	SwaggerFieldPaths      = "paths"
	SwaggerFieldDefs       = "definitions"
	SwaggerFieldTags       = "tags"
	SwaggerFieldParameters = "parameters"
)

// Swagger 聚合模式常量
const (
	SwaggerAggregateModeMerge    = "merge"
	SwaggerAggregateModeSelector = "selector"
)

// Swagger 文件格式常量
const (
	FileExtYAML = ".yaml"
	FileExtYML  = ".yml"
	FileExtJSON = ".json"
	MimeYAML    = "yaml"
	MimeYML     = "yml"
)

// Swagger 默认配置常量
const (
	DefaultSwaggerTimeout         = 30 * time.Second
	DefaultSwaggerRefreshInterval = 5 * time.Minute
)

// Swagger 字段名常量
const (
	SwaggerFieldInfo             = "info"
	SwaggerFieldSwagger          = "swagger"
	SwaggerFieldConsumes         = "consumes"
	SwaggerFieldProduces         = "produces"
	SwaggerFieldBasePath         = "basePath"
	SwaggerFieldXAggregateInfo   = "x-aggregate-info"
	SwaggerFieldXServiceSelector = "x-service-selector"
	SwaggerFieldServices         = "services"
	SwaggerFieldName             = "name"
	SwaggerFieldDescription      = "description"
	SwaggerFieldVersion          = "version"
	SwaggerFieldEnabled          = "enabled"
	SwaggerFieldMode             = "mode"
	SwaggerFieldUpdated          = "updated"
	SwaggerFieldCount            = "count"
	SwaggerFieldTitle            = "title"
	SwaggerFieldContact          = "contact"
	SwaggerFieldLicense          = "license"
	SwaggerFieldEmail            = "email"
	SwaggerFieldURL              = "url"
)

// Swagger 调试信息字段常量
const (
	SwaggerFieldTotalServices      = "total_services"
	SwaggerFieldLoadedServices     = "loaded_services"
	SwaggerFieldConfiguredServices = "configured_services"
	SwaggerFieldTimestamp          = "timestamp"
	SwaggerFieldSpecPath           = "spec_path"
)

// HTML 相关常量
const (
	SwaggerHTMLLangEN  = "en"
	SwaggerHTMLLangZH  = "zh-CN"
	SwaggerHTMLCharset = "UTF-8"
)

// 路径分隔符常量
const (
	SwaggerPathSeparator      = "/"
	SwaggerPathServicePrefix  = "/services/"
	SwaggerPathUnderscoreChar = "_"
	SwaggerPathHyphenChar     = "-"
)

// Swagger UI 配置常量
const (
	SwaggerUITemplateName = "swagger"
	SwaggerUILayout       = "StandaloneLayout"
	SwaggerUIDomID        = "#swagger-ui"
)

// JSON 格式化常量
const (
	JSONIndentPrefix = ""
	JSONIndentValue  = "  "
)

// HTML Meta 标签常量
const (
	HTMLMetaViewport = "width=device-width, initial-scale=1.0"
	HTMLIconSizes32  = "32x32"
	HTMLIconSizes16  = "16x16"
)

// Swagger UI JavaScript 常量
const (
	SwaggerUIBundleVar    = "SwaggerUIBundle"
	SwaggerUIPresetVar    = "SwaggerUIStandalonePreset"
	SwaggerUIWindowOnload = "window.onload"
	SwaggerUIDeepLinking  = "true"
)
