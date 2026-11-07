/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-03-04 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-03-04 00:00:00
 * @FilePath: \go-rpc-gateway\config\errors.go
 * @Description: 统一错误定义和管理
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package config

import (
	"errors"
	"fmt"
)

// 预定义的基础错误（无参数的错误）
var (
	// 环境管理相关
	ErrCallbackIDEmpty = errors.New("回调ID不能为空")
	ErrConfigEmpty     = errors.New("配置为空")
	ErrNoConfigPath    = errors.New("未指定配置路径或搜索选项")

	// 管理器状态相关
	ErrManagerRunning     = errors.New("集成配置管理器已在运行")
	ErrManagerNotRunning  = errors.New("集成配置管理器未运行")
	ErrReloaderRunning    = errors.New("热更新器已经在运行")
	ErrReloaderNotRunning = errors.New("热更新器未运行")
)

// 环境管理相关错误

// ErrCallbackIDExists 回调ID已存在错误
func ErrCallbackIDExists(id string) error {
	return fmt.Errorf("回调ID %s 已存在", id)
}

// ErrCallbackIDNotFound 回调ID不存在错误
func ErrCallbackIDNotFound(id string) error {
	return fmt.Errorf("回调ID %s 不存在", id)
}

// ErrEnvTypeRegistered 环境类型已注册错误
func ErrEnvTypeRegistered(env EnvironmentType) error {
	return fmt.Errorf("环境类型 %s 已注册", env)
}

// ErrEnvVarNotSet 环境变量未设置错误
func ErrEnvVarNotSet(key string) error {
	return fmt.Errorf("环境变量 %s 未设置", key)
}

// 配置类型相关错误

// ErrConfigTypeMismatch 配置类型不匹配错误
func ErrConfigTypeMismatch(expected, actual any) error {
	return fmt.Errorf("配置类型不匹配: 期望 %T, 实际 %T", expected, actual)
}

// 模块配置相关错误

// ErrModuleNotFound 模块不存在错误
func ErrModuleNotFound(moduleName string) error {
	return fmt.Errorf("模块 %s 不存在", moduleName)
}

// ErrModuleConfigEmpty 模块默认配置为空错误
func ErrModuleConfigEmpty(moduleName string) error {
	return fmt.Errorf("模块 %s 的默认配置为空", moduleName)
}

// ErrModuleDefaultFuncNil 模块默认函数为空错误
func ErrModuleDefaultFuncNil(moduleName string) error {
	return fmt.Errorf("模块 %s 的默认函数为空", moduleName)
}

// ErrPartialModuleFailed 部分模块配置生成失败错误
func ErrPartialModuleFailed(successCount, failCount int) error {
	return fmt.Errorf("部分模块配置生成失败: 成功 %d, 失败 %d", successCount, failCount)
}

// ErrModuleValidationFailed 模块配置验证失败错误
func ErrModuleValidationFailed(failedModules []string) error {
	return fmt.Errorf("以下模块配置验证失败: %v", failedModules)
}

// 任务配置相关错误

// ErrInvalidTimezone 无效的时区错误
func ErrInvalidTimezone(timezone string, err error) error {
	return fmt.Errorf("无效的时区: %s, 错误: %w", timezone, err)
}

// ErrCronSpecEmpty 任务cron_spec为空错误
func ErrCronSpecEmpty(taskName string) error {
	return fmt.Errorf("任务[%s]的cron_spec不能为空", taskName)
}

// ErrTimeoutInvalid 任务timeout无效错误
func ErrTimeoutInvalid(taskName string) error {
	return fmt.Errorf("任务[%s]的timeout不能小于0", taskName)
}

// ErrPriorityOutOfRange 任务priority超出范围错误
func ErrPriorityOutOfRange(taskName string) error {
	return fmt.Errorf("任务[%s]的priority必须在0-99999之间", taskName)
}

// ErrMaxConcurrentInvalid 任务max_concurrent无效错误
func ErrMaxConcurrentInvalid(taskName string) error {
	return fmt.Errorf("任务[%s]的max_concurrent不能小于0", taskName)
}

// ErrBreakerMaxFailuresInvalid 熔断器max_failures无效错误
func ErrBreakerMaxFailuresInvalid(taskName string) error {
	return fmt.Errorf("任务[%s]的熔断器max_failures必须大于0", taskName)
}

// ErrBreakerResetTimeoutInvalid 熔断器reset_timeout无效错误
func ErrBreakerResetTimeoutInvalid(taskName string) error {
	return fmt.Errorf("任务[%s]的熔断器reset_timeout必须大于0", taskName)
}

// ErrBreakerHalfOpenSuccessesInvalid 熔断器half_open_successes无效错误
func ErrBreakerHalfOpenSuccessesInvalid(taskName string) error {
	return fmt.Errorf("任务[%s]的熔断器half_open_successes必须大于0", taskName)
}

// 文件操作相关错误

// ErrReadConfigFile 读取配置文件失败错误
func ErrReadConfigFile(err error) error {
	return fmt.Errorf("读取配置文件失败: %w", err)
}

// ErrUnmarshalConfig 解析配置失败错误
func ErrUnmarshalConfig(err error) error {
	return fmt.Errorf("解析配置失败: %w", err)
}

// ErrCreateHotReloader 创建热更新器失败错误
func ErrCreateHotReloader(err error) error {
	return fmt.Errorf("创建热更新器失败: %w", err)
}

// ErrStartHotReloader 启动热更新器失败错误
func ErrStartHotReloader(err error) error {
	return fmt.Errorf("启动热更新器失败: %w", err)
}

// ErrStopHotReloader 停止热更新器失败错误
func ErrStopHotReloader(err error) error {
	return fmt.Errorf("停止热更新器失败: %w", err)
}

// ErrCreateWatcher 创建文件监控器失败错误
func ErrCreateWatcher(err error) error {
	return fmt.Errorf("创建文件监控器失败: %w", err)
}

// ErrGetAbsPath 获取配置文件绝对路径失败错误
func ErrGetAbsPath(err error) error {
	return fmt.Errorf("获取配置文件绝对路径失败: %w", err)
}

// ErrAddWatcher 添加配置目录监控失败错误
func ErrAddWatcher(err error) error {
	return fmt.Errorf("添加配置目录监控失败: %w", err)
}

// ErrCreateOutputDir 创建输出目录失败错误
func ErrCreateOutputDir(err error) error {
	return fmt.Errorf("创建输出目录失败: %w", err)
}

// ErrGenerateYAML 生成YAML配置失败错误
func ErrGenerateYAML(err error) error {
	return fmt.Errorf("生成YAML配置失败: %w", err)
}

// ErrGenerateJSON 生成JSON配置失败错误
func ErrGenerateJSON(err error) error {
	return fmt.Errorf("生成JSON配置失败: %w", err)
}

// ErrMarshalYAML 序列化YAML失败错误
func ErrMarshalYAML(err error) error {
	return fmt.Errorf("序列化YAML失败（可能包含未标记为 yaml:\"-\" 的函数字段）: %w", err)
}

// ErrWriteYAML 写入YAML文件失败错误
func ErrWriteYAML(err error) error {
	return fmt.Errorf("写入YAML文件失败: %w", err)
}

// ErrMarshalJSON 序列化JSON失败错误
func ErrMarshalJSON(err error) error {
	return fmt.Errorf("序列化JSON失败: %w", err)
}

// ErrWriteJSON 写入JSON文件失败错误
func ErrWriteJSON(err error) error {
	return fmt.Errorf("写入JSON文件失败: %w", err)
}

// 配置发现相关错误

// ErrSearchPathNotExist 搜索路径不存在错误
func ErrSearchPathNotExist(path string) error {
	return fmt.Errorf("搜索路径不存在: %s", path)
}

// ErrConfigFileNotFound 未找到配置文件错误
func ErrConfigFileNotFound(suggestedPath string) error {
	return fmt.Errorf("未找到存在的配置文件，建议创建: %s", suggestedPath)
}

// ErrNoConfigCandidate 未找到任何配置文件候选错误
var ErrNoConfigCandidate = errors.New("未找到任何配置文件候选")

// ErrCreateDir 创建目录失败错误
func ErrCreateDir(err error) error {
	return fmt.Errorf("创建目录失败: %w", err)
}

// ErrGenerateDefaultConfig 生成默认配置失败错误
func ErrGenerateDefaultConfig(err error) error {
	return fmt.Errorf("生成默认配置失败: %w", err)
}

// ErrWriteConfigFile 写入配置文件失败错误
func ErrWriteConfigFile(err error) error {
	return fmt.Errorf("写入配置文件失败: %w", err)
}

// ErrScanDir 扫描目录失败错误
func ErrScanDir(err error) error {
	return fmt.Errorf("扫描目录失败: %w", err)
}

// ErrUnsupportedFormat 不支持的配置文件格式错误
func ErrUnsupportedFormat(extension string) error {
	return fmt.Errorf("不支持的配置文件格式: %s", extension)
}

// 配置构建相关错误

// ErrResolveConfigPath 解析配置路径失败错误
func ErrResolveConfigPath(err error) error {
	return fmt.Errorf("解析配置路径失败: %w", err)
}

// ErrCreateManager 创建集成配置管理器失败错误
func ErrCreateManager(err error) error {
	return fmt.Errorf("创建集成配置管理器失败: %w", err)
}

// ErrStartManager 启动管理器失败错误
func ErrStartManager(err error) error {
	return fmt.Errorf("启动管理器失败: %w", err)
}

// ErrFindConfigByPattern 按模式查找配置文件失败错误
func ErrFindConfigByPattern(err error) error {
	return fmt.Errorf("按模式查找配置文件失败: %w", err)
}

// ErrNoMatchingConfig 未找到匹配模式的配置文件错误
func ErrNoMatchingConfig(pattern string) error {
	return fmt.Errorf("未找到匹配模式 '%s' 的配置文件", pattern)
}

// ErrModuleConfigSerializeFailed 模块配置序列化失败错误
func ErrModuleConfigSerializeFailed(moduleName string, err error) error {
	return fmt.Errorf("模块 %s 的配置序列化失败: %w", moduleName, err)
}

// ErrModulesValidationFailed 模块配置验证失败错误（带模块列表）
func ErrModulesValidationFailed(modules string) error {
	return fmt.Errorf("以下模块配置验证失败: %s", modules)
}

// ErrDiscoverConfigFiles 发现配置文件失败错误
func ErrDiscoverConfigFiles(err error) error {
	return fmt.Errorf("发现配置文件失败: %w", err)
}

// ErrAutoDiscoverConfigFiles 自动发现配置文件失败错误
func ErrAutoDiscoverConfigFiles(err error) error {
	return fmt.Errorf("自动发现配置文件失败: %w", err)
}

// 回调管理相关错误

// ErrCallbackFuncNil 回调函数不能为空错误
var ErrCallbackFuncNil = errors.New("回调函数不能为空")
