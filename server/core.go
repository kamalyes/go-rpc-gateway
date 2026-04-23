/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2024-11-07 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-05 19:48:21
 * @FilePath: \go-rpc-gateway\server\core.go
 * @Description: 核心组件初始化模块，集成企业级组件和go-logger
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package server

import (
	"github.com/kamalyes/go-rpc-gateway/errors"
	"github.com/kamalyes/go-rpc-gateway/global"
)

// initCore 初始化核心组件，集成企业级组件
func (s *Server) initCore() error {
	if global.POOL_MANAGER == nil {
		return errors.NewError(errors.ErrCodeInternalServerError, "global POOL_MANAGER is not initialized, ensure InitializerChain has run")
	}

	// 将连接池管理器保存到服务器中
	s.poolManager = global.POOL_MANAGER

	// 初始化端点收集器
	s.endpointCollector = NewEndpointCollector()
	global.LOGGER.InfoMsg("✅ 端点收集器已初始化")

	return nil
}
