/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\aggregate\selector.go
 * @Description: 聚合选择器模式 - 按服务维度提供独立 API 规范视图
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package aggregate

import (
	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
)

// createSelectorSpec 创建选择器规范
// 选择器模式不合并 paths/definitions，而是通过 x-service-selector 字段提供服务列表
// 前端可根据选择器信息按需加载单个服务的规范
func (a *Aggregator) createSelectorSpec() error {
	a.aggregatedSpec = a.newBaseSpec()
	a.aggregatedSpec[constants.FieldXServiceSelector] = map[string]interface{}{
		constants.FieldEnabled:  true,
		constants.FieldServices: a.buildServicesSummary(),
	}

	a.logger.Info("选择器规范创建完成")
	return nil
}
