/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-17 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-17 21:33:55
 * @FilePath: \go-rpc-gateway\errors\formatter.go
 * @Description: 错误和消息格式化工具
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package errors

import (
	"fmt"
)

// FormatError 格式化错误消息
func FormatError(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// FormatMessage 格式化普通消息
func FormatMessage(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// FormatInitError 格式化初始化错误消息
func FormatInitError(component string, err error) string {
	return fmt.Sprintf("初始化%s失败: %v", component, err)
}

// FormatStartupError 格式化启动错误消息
func FormatStartupError(service string, err error) string {
	return fmt.Sprintf("启动%s失败: %v", service, err)
}

// FormatConfigError 格式化配置错误消息
func FormatConfigError(operation string, err error) string {
	return fmt.Sprintf("%s失败: %v", operation, err)
}

// FormatConnectionInfo 格式化连接信息
func FormatConnectionInfo(service string, endpoint string) string {
	return fmt.Sprintf("🌐 %s端点: %s", service, endpoint)
}

// FormatConfigUpdateInfo 格式化配置更新信息
func FormatConfigUpdateInfo(name string) string {
	return fmt.Sprintf("📋 配置已更新: %s", name)
}

// FormatEnvironmentChangeInfo 格式化环境变更信息
func FormatEnvironmentChangeInfo(oldEnv, newEnv string) string {
	return fmt.Sprintf("🌍 环境变更: %s -> %s", oldEnv, newEnv)
}

// FormatServiceInfo 格式化服务信息
func FormatServiceInfo(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// FormatShutdownInfo 格式化关闭信息
func FormatShutdownInfo(signal string) string {
	return fmt.Sprintf("\n🛑 收到信号 %s，开始优雅关闭...", signal)
}

// FormatStopError 格式化停止错误消息
func FormatStopError(err error) string {
	return fmt.Sprintf("❌ 停止服务时发生错误: %v", err)
}

// FormatPanicError 格式化 panic 错误消息
func FormatPanicError(operation string, err interface{}) string {
	return fmt.Sprintf("%s失败: %v", operation, err)
}
