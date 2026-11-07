/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\aggregate\spec.go
 * @Description: 聚合规范骨架 - 基础规范创建、引用路径修复
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package aggregate

import (
	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
)

// newBaseSpec 创建聚合规范骨架，供 merge 和 selector 模式复用
func (a *Aggregator) newBaseSpec() map[string]interface{} {
	return map[string]interface{}{
		constants.FieldSwagger:        constants.SpecVersion,
		constants.FieldInfo:           a.buildAggregateInfo(),
		constants.FieldConsumes:       []string{constants.MimeApplicationJSON},
		constants.FieldProduces:       []string{constants.MimeApplicationJSON},
		constants.FieldPaths:          make(map[string]interface{}),
		constants.FieldDefs:           make(map[string]interface{}),
		constants.FieldXAggregateInfo: a.buildServicesInfo(),
	}
}

// fixReferences 修复聚合规范中的所有引用路径
func (a *Aggregator) fixReferences() error {
	return a.fixReferencesInObject(a.aggregatedSpec)
}

// fixReferencesInObject 递归修复对象中的引用
func (a *Aggregator) fixReferencesInObject(obj interface{}) error {
	switch v := obj.(type) {
	case map[string]interface{}:
		return a.fixReferencesInMap(v)
	case []interface{}:
		return a.fixReferencesInSlice(v)
	}
	return nil
}

// fixReferencesInMap 修复 map 中的引用
func (a *Aggregator) fixReferencesInMap(m map[string]interface{}) error {
	for _, value := range m {
		if err := a.fixReferencesInObject(value); err != nil {
			return err
		}
	}
	return nil
}

// fixReferencesInSlice 修复 slice 中的引用
func (a *Aggregator) fixReferencesInSlice(slice []interface{}) error {
	for _, item := range slice {
		if err := a.fixReferencesInObject(item); err != nil {
			return err
		}
	}
	return nil
}
