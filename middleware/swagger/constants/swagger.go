/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\constants\swagger.go
 * @Description: Swagger 规范常量 - 版本号、规范字段名、聚合模式
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package constants

// ==================== Swagger 规范版本 ====================

const (
	SpecVersion = "2.0"
)

// ==================== Swagger 规范字段名 ====================

const (
	FieldRef        = "$ref"
	FieldPaths      = "paths"
	FieldDefs       = "definitions"
	FieldTags       = "tags"
	FieldParameters = "parameters"
	PathDefinitions = "#/definitions/"
)

// ==================== Swagger 规范通用字段 ====================

const (
	FieldInfo             = "info"
	FieldSwagger          = "swagger"
	FieldConsumes         = "consumes"
	FieldProduces         = "produces"
	FieldBasePath         = "basePath"
	FieldSchemes          = "schemes"
	FieldSecurity         = "security"
	FieldSecurityDefs     = "securityDefinitions"
	FieldXAggregateInfo   = "x-aggregate-info"
	FieldXDocumentInfo    = "x-document-info"
	FieldXServiceSelector = "x-service-selector"
	FieldServices         = "services"
	FieldDocuments        = "documents"
	FieldName             = "name"
	FieldDescription      = "description"
	FieldVersion          = "version"
	FieldEnabled          = "enabled"
	FieldMode             = "mode"
	FieldUpdated          = "updated"
	FieldCount            = "count"
	FieldTitle            = "title"
	FieldContact          = "contact"
	FieldLicense          = "license"
	FieldEmail            = "email"
	FieldURL              = "url"
)

// ==================== 聚合模式 ====================

const (
	AggregateModeMerge    = "merge"
	AggregateModeSelector = "selector"
)
