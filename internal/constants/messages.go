/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-07 00:00:00
 * @FilePath: \go-rpc-gateway\internal\constants\messages.go
 * @Description: 消息常量定义
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package constants

// 响应消息常量
const (
	// 通用响应
	MsgSuccess      = "success"
	MsgError        = "error"
	MsgFailed       = "failed"
	MsgNotFound     = "not found"
	MsgUnauthorized = "unauthorized"
	MsgForbidden    = "forbidden"
	MsgBadRequest   = "bad request"
	
	// 业务响应
	MsgInternalServerError   = "Internal server error"
	MsgTooManyRequests      = "Too Many Requests"
	MsgRateLimitExceeded    = "Rate limit exceeded"
	MsgInvalidSignature     = "Invalid signature"
	MsgSignatureExpired     = "Signature expired"
	MsgMissingRequiredFields = "Missing required fields"
	MsgRequestTimeout       = "Request timeout"
	MsgServiceUnavailable   = "Service unavailable"
	MsgBadGateway          = "Bad gateway"
	MsgGatewayTimeout      = "Gateway timeout"
	
	// 验证相关
	MsgValidationFailed     = "Validation failed"
	MsgInvalidParameter     = "Invalid parameter"
	MsgParameterMissing     = "Parameter missing"
	MsgParameterTooLong     = "Parameter too long"
	MsgParameterTooShort    = "Parameter too short"
	MsgInvalidFormat        = "Invalid format"
	MsgInvalidEmail         = "Invalid email format"
	MsgInvalidPhone         = "Invalid phone number"
	MsgInvalidURL           = "Invalid URL format"
	
	// 认证相关
	MsgTokenExpired         = "Token expired"
	MsgTokenInvalid         = "Token invalid"
	MsgTokenMissing         = "Token missing"
	MsgPermissionDenied     = "Permission denied"
	MsgAccountNotFound      = "Account not found"
	MsgAccountDisabled      = "Account disabled"
	MsgPasswordIncorrect    = "Password incorrect"
	MsgLoginRequired        = "Login required"
	
	// 数据操作相关
	MsgCreateSuccess        = "Created successfully"
	MsgUpdateSuccess        = "Updated successfully"
	MsgDeleteSuccess        = "Deleted successfully"
	MsgQuerySuccess         = "Query successful"
	MsgCreateFailed         = "Create failed"
	MsgUpdateFailed         = "Update failed"
	MsgDeleteFailed         = "Delete failed"
	MsgQueryFailed          = "Query failed"
	MsgDataNotFound         = "Data not found"
	MsgDataExists           = "Data already exists"
	MsgDataCorrupted        = "Data corrupted"
	
	// 系统相关
	MsgSystemBusy           = "System busy, please try again later"
	MsgMaintenanceMode      = "System is under maintenance"
	MsgSystemError          = "System error"
	MsgConfigurationError   = "Configuration error"
	MsgDatabaseError        = "Database error"
	MsgNetworkError         = "Network error"
	MsgTimeout              = "Operation timeout"
	
	// 文件操作相关
	MsgFileUploadSuccess    = "File uploaded successfully"
	MsgFileUploadFailed     = "File upload failed"
	MsgFileNotFound         = "File not found"
	MsgFileTypeNotSupported = "File type not supported"
	MsgFileTooLarge         = "File too large"
	MsgFileCorrupted        = "File corrupted"
	
	// 缓存相关
	MsgCacheHit             = "Cache hit"
	MsgCacheMiss            = "Cache miss"
	MsgCacheExpired         = "Cache expired"
	MsgCacheError           = "Cache error"
)

// JSON 响应模板
const (
	JSONSuccessTemplate = `{"success": true, "message": "%s", "data": %s}`
	JSONErrorTemplate   = `{"success": false, "error": "%s", "message": "%s"}`
	JSONSimpleError     = `{"error": "%s"}`
	JSONSimpleSuccess   = `{"success": true}`
	JSONRateLimitError  = `{"error": "Too Many Requests", "message": "Rate limit exceeded"}`
	JSONInternalError   = `{"error": "Internal server error"}`
	JSONUnauthorized    = `{"error": "Unauthorized", "message": "Authentication required"}`
	JSONForbidden       = `{"error": "Forbidden", "message": "Permission denied"}`
	JSONNotFound        = `{"error": "Not Found", "message": "Resource not found"}`
	JSONBadRequest      = `{"error": "Bad Request", "message": "Invalid request"}`
)