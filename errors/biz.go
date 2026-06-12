/*
 * @Author: kamalyes 501893@qq.com
 * @Date: 2026-06-12 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-06-12 13:15:28
 * @FilePath: \go-rpc-gateway\errors\biz.go
 * @Description: 业务错误码到 gRPC 状态码的可注册映射
 *
 * 各微服务在 bootstrap 阶段调用 RegisterBizCodeMap 注册自己的映射规则，
 * 运行时通过 ToGRPCError / MapBizCodeToGRPCCode 自动查找。
 * 内置通用规则：_not_found 后缀自动映射为 NotFound。
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package errors

import (
	"context"
	"encoding/json"
	"strings"

	goi18n "github.com/kamalyes/go-i18n"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// bizCodeRegistry 业务错误码到 gRPC 状态码的注册表
var bizCodeRegistry = make(map[string]codes.Code)

// RegisterBizCodeMap 批量注册业务错误码到 gRPC 状态码的映射
// 应在服务 bootstrap 阶段调用，仅执行一次
func RegisterBizCodeMap(m map[string]codes.Code) {
	for k, v := range m {
		bizCodeRegistry[k] = v
	}
}

// RegisterBizCode 注册单个业务错误码映射
func RegisterBizCode(bizCode string, grpcCode codes.Code) {
	bizCodeRegistry[bizCode] = grpcCode
}

// MapBizCodeToGRPCCode 将业务错误码映射到 gRPC 状态码
// 查找顺序：
//  1. 注册表中的精确匹配
//  2. 内置规则：_not_found 后缀 → NotFound
//  3. 兜底：Internal
func MapBizCodeToGRPCCode(bizCode string) codes.Code {
	// 1. 精确匹配
	if c, ok := bizCodeRegistry[bizCode]; ok {
		return c
	}

	// 2. 内置规则：_not_found 后缀
	if strings.HasSuffix(bizCode, "_not_found") {
		return codes.NotFound
	}

	// 3. 兜底
	return codes.Internal
}

// ToGRPCError 将业务错误码转换为标准 gRPC 错误
// 自动通过注册表查找 gRPC 状态码，并通过 i18n 获取翻译消息
func ToGRPCError(ctx context.Context, bizCode string) error {
	message := resolveI18nMessage(ctx, bizCode)
	grpcCode := MapBizCodeToGRPCCode(bizCode)
	return status.Error(grpcCode, message)
}

// ToGRPCErrorWithTemplate 将带模板数据的业务错误码转换为标准 gRPC 错误
// 支持 i18n 模板变量替换，例如 error.hello → "你好 {name}"
func ToGRPCErrorWithTemplate(ctx context.Context, bizCode string, templateData map[string]interface{}) error {
	message := resolveI18nMessageWithTemplate(ctx, bizCode, templateData)
	grpcCode := MapBizCodeToGRPCCode(bizCode)
	return status.Error(grpcCode, message)
}

// NewI18nError 创建国际化错误响应（返回纯消息字符串）
// 使用 i18n 键获取翻译消息，翻译失败则返回 bizCode 本身
func NewI18nError(ctx context.Context, bizCode string) string {
	return resolveI18nMessage(ctx, bizCode)
}

// NewI18nErrorWithTemplate 创建带模板数据的国际化错误响应
func NewI18nErrorWithTemplate(ctx context.Context, bizCode string, templateData map[string]interface{}) string {
	return resolveI18nMessageWithTemplate(ctx, bizCode, templateData)
}

// ExtractRpcErrorMsg 从错误字符串中提取 JSON 格式的 msg 字段
// 当 gRPC 返回的错误信息包含 JSON 结构时，提取其中的 msg 字段
// 如果不是 JSON 格式则返回原始错误字符串
func ExtractRpcErrorMsg(errStr string) string {
	start := strings.Index(errStr, "{")
	end := strings.LastIndex(errStr, "}")

	if start == -1 || end == -1 || start >= end {
		return errStr
	}

	jsonStr := errStr[start : end+1]

	var errData struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &errData); err != nil {
		return errStr
	}

	if errData.Msg != "" {
		return errData.Msg
	}

	return errStr
}

// resolveI18nMessage 解析 i18n 消息
func resolveI18nMessage(ctx context.Context, key string) string {
	message := goi18n.GetMsgByKey(ctx, key)
	if message == "" || message == key {
		return key
	}
	return message
}

// resolveI18nMessageWithTemplate 解析带模板数据的 i18n 消息
func resolveI18nMessageWithTemplate(ctx context.Context, key string, templateData map[string]interface{}) string {
	message := goi18n.GetMsgWithMap(ctx, key, templateData)
	if message == "" || message == key {
		return key
	}
	return message
}
